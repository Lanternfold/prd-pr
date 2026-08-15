package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/diagnose"
	"github.com/lanternfold/prd-pr/internal/llm"
	"github.com/lanternfold/prd-pr/internal/modelrouter"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

const SchemaVersion = 1

// Result is independent review of P7 evidence.
type Result struct {
	SchemaVersion int                   `json:"schema_version"`
	Diagnosis     diagnose.Report       `json:"diagnosis"`
	Model         modelrouter.Decision  `json:"model"`
	LLMText       string                `json:"llm_text,omitempty"`
	ReviewFailure string                `json:"review_failure,omitempty"`
	Repair        bool                  `json:"recommend_repair"`
	Human         bool                  `json:"recommend_human"`
}

// Options injects router inputs and an optional LLM adapter.
type Options struct {
	Adapter llm.Adapter
	Config  config.Config
	SpentUSD float64
}

// Input is everything review may inspect. It does not trust worker claims.
type Input struct {
	Verification     testeng.Result
	Packet           packet.Packet
	DiffSummary      string
	PreviousAttempts int
}

// Run classifies the failure. It uses an LLM only when the router selects one.
func Run(ctx context.Context, in Input, opts Options) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := opts.Config
	if cfg.MaxRepairAttempts == 0 {
		cfg = config.Defaults()
	}
	diag := diagnose.Classify(in.Verification, in.Packet)
	out := Result{SchemaVersion: SchemaVersion, Diagnosis: diag}

	evidence := len(strings.Join(in.Verification.Failures, " ")) + len(in.DiffSummary)
	remaining := cfg.CostBudgetUSD - opts.SpentUSD
	routeIn := modelrouter.Input{
		Role:                    llm.RoleReview,
		FailureType:             diag.Classification,
		EvidenceSize:            evidence,
		PreviousAttempts:        in.PreviousAttempts,
		ExpectedValue:           1,
		DeterministicSufficient: !diag.NeedsLLM && diag.Classification != diagnose.ClassAmbiguous,
		BudgetRemainingUSD:      &remaining,
		BudgetBreachPolicy:      cfg.BudgetBreachPolicy,
		CheapModel:              cfg.CheapModel,
		StrongModel:             cfg.StrongModel,
	}
	if diag.Classification == diagnose.ClassNone {
		routeIn.DeterministicSufficient = true
		routeIn.ExpectedValue = 0
	}
	dec := modelrouter.Route(routeIn)
	out.Model = dec

	if dec.BudgetBreach == config.BudgetAskHuman {
		out.Human = true
		out.Repair = false
		out.Diagnosis.HumanReason = "cost_budget"
		return out
	}

	if dec.SelectedModel != modelrouter.ModelNone && opts.Adapter != nil {
		res, err := opts.Adapter.Complete(ctx, llm.Request{
			Role:   llm.RoleReview,
			Model:  dec.SelectedModel,
			Prompt: reviewPrompt(in, diag),
		})
		if err != nil {
			out.ReviewFailure = err.Error()
			out.Model.Fallback = true
			out.Model.Reason = "model failed; keeping deterministic diagnosis"
			// do not retry a stronger model
		} else {
			out.LLMText = res.Text
			out.Model = modelrouter.RecordUsage(dec, res.InputTokens, res.OutputTokens, res.CostUSD)
		}
	}

	out.Repair = diag.Actionable && diag.ConsumesAttempt
	out.Human = diag.HumanReason != "" && !out.Repair
	if in.PreviousAttempts >= cfg.MaxRepairAttempts {
		out.Repair = false
		out.Human = true
		out.Diagnosis.HumanReason = "repair_exhausted"
	}
	return out
}

func reviewPrompt(in Input, d diagnose.Report) string {
	return fmt.Sprintf("Classify this verification failure. Diagnosis so far: %s (%s). Failures: %s. Do not claim tests passed.",
		d.Classification, d.Summary, strings.Join(in.Verification.Failures, "; "))
}
