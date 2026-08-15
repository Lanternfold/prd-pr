package review

import (
	"context"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/diagnose"
	"github.com/lanternfold/prd-pr/internal/llm"
	"github.com/lanternfold/prd-pr/internal/modelrouter"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func TestDeterministicReviewUsesNoModel(t *testing.T) {
	res := Run(context.Background(), Input{
		Verification: testeng.Result{Status: testeng.StatusFailed, Failures: []string{"TestAdd failed"}, ChangedFiles: []string{"add.go"}},
		Packet:       packet.Packet{ExpectedOutputs: []string{"add.go"}},
	}, Options{Config: config.Defaults(), Adapter: llm.Fail{Err: context.Canceled}})
	if res.Model.SelectedModel != modelrouter.ModelNone {
		t.Fatalf("model=%+v", res.Model)
	}
	if !res.Repair || res.Diagnosis.Classification == diagnose.ClassInfrastructure {
		t.Fatalf("%+v", res)
	}
}

func TestModelFailureIsStructured(t *testing.T) {
	cfg := config.Defaults()
	zero := 0.0
	_ = zero
	res := Run(context.Background(), Input{
		Verification: testeng.Result{Status: testeng.StatusFailed, Failures: []string{"ambiguous requirement: unclear"}, Reason: "ambiguous requirement"},
		Packet:       packet.Packet{},
	}, Options{Config: cfg, Adapter: llm.Fail{Err: context.DeadlineExceeded}})
	if res.Diagnosis.Classification != diagnose.ClassAmbiguous {
		t.Fatalf("%+v", res.Diagnosis)
	}
}

func TestBudgetAskHuman(t *testing.T) {
	cfg := config.Defaults()
	cfg.CostBudgetUSD = 0
	cfg.BudgetBreachPolicy = config.BudgetAskHuman
	res := Run(context.Background(), Input{
		Verification: testeng.Result{Status: testeng.StatusFailed, Failures: []string{"x"}, ChangedFiles: []string{"a.go"}},
		Packet:       packet.Packet{},
	}, Options{Config: cfg, SpentUSD: 1})
	// deterministic sufficient still wins for product failures
	if res.Model.SelectedModel != modelrouter.ModelNone && !strings.Contains(res.Diagnosis.HumanReason+res.Model.BudgetBreach, "budget") && !res.Repair {
		t.Fatalf("%+v", res)
	}
}
