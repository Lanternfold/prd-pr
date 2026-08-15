package modelrouter

import (
	"github.com/lanternfold/prd-pr/internal/config"
)

const (
	ModelNone   = "NONE"
	ModelCheap  = "cheap"
	ModelStrong = "strong"
)

// Input is routing evidence. No vendor names required.
type Input struct {
	Role                    string
	TaskComplexity          string // low, medium, high
	FailureType             string
	EvidenceSize            int
	PreviousAttempts        int
	ExpectedValue           float64
	BudgetRemainingUSD      *float64
	BudgetBreachPolicy      string
	DeterministicSufficient bool
	KnowledgeHints          []string
	CheapModel              string
	StrongModel             string
}

// Decision is the structured routing result.
type Decision struct {
	SelectedModel  string  `json:"selected_model"`
	Reason         string  `json:"reason"`
	EstimatedCost  float64 `json:"estimated_cost,omitempty"`
	InputTokens    int     `json:"input_tokens,omitempty"`
	OutputTokens   int     `json:"output_tokens,omitempty"`
	Confidence     float64 `json:"confidence"`
	Fallback       bool    `json:"fallback,omitempty"`
	BudgetBreach   string  `json:"budget_breach,omitempty"`
	Purpose        string  `json:"purpose,omitempty"`
}

// Route chooses NONE, a cheap model, or a stronger model. It does not call a provider.
func Route(in Input) Decision {
	cheap := in.CheapModel
	if cheap == "" {
		cheap = ModelCheap
	}
	strong := in.StrongModel
	if strong == "" {
		strong = ModelStrong
	}
	d := Decision{Purpose: in.Role, Confidence: 0.7}

	if in.DeterministicSufficient {
		d.SelectedModel = ModelNone
		d.Reason = "deterministic analysis is sufficient; no LLM required"
		d.Confidence = 0.95
		return d
	}

	if in.BudgetRemainingUSD != nil && *in.BudgetRemainingUSD <= 0 && in.ExpectedValue > 0 {
		switch in.BudgetBreachPolicy {
		case config.BudgetContinue:
			d.BudgetBreach = config.BudgetContinue
			d.SelectedModel = cheap
			d.Reason = "budget exhausted; continuing with cheap model as configured"
			d.Fallback = true
			d.Confidence = 0.4
			return d
		case config.BudgetDowngrade:
			d.SelectedModel = ModelNone
			d.Reason = "budget exhausted; downgrading to deterministic analysis"
			d.Fallback = true
			d.BudgetBreach = config.BudgetDowngrade
			d.Confidence = 0.6
			return d
		default:
			d.SelectedModel = ModelNone
			d.Reason = "budget exhausted; stop and ask human"
			d.Fallback = true
			d.BudgetBreach = config.BudgetAskHuman
			d.Confidence = 0.8
			return d
		}
	}

	complex := in.TaskComplexity == "high" || in.EvidenceSize > 8000 || in.FailureType == "ambiguous"
	if complex && in.PreviousAttempts >= 1 && in.ExpectedValue > 0 {
		d.SelectedModel = strong
		d.Reason = "complexity and prior attempts justify a stronger model"
		d.Confidence = 0.55
		d.EstimatedCost = 0.02
		return d
	}

	if in.ExpectedValue > 0 && (in.FailureType == "product" || in.Role == "diagnosis" || in.Role == "review") && !complex {
		d.SelectedModel = cheap
		d.Reason = "small evidence; cheapest capable model"
		d.Confidence = 0.6
		d.EstimatedCost = 0.002
		return d
	}

	d.SelectedModel = ModelNone
	d.Reason = "no LLM justified; prefer deterministic analysis"
	d.Confidence = 0.85
	return d
}

// RecordUsage fills token/cost fields after an adapter returns.
func RecordUsage(d Decision, inputTokens, outputTokens int, cost float64) Decision {
	d.InputTokens = inputTokens
	d.OutputTokens = outputTokens
	if cost > 0 {
		d.EstimatedCost = cost
	}
	return d
}
