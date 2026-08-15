package modelrouter

import (
	"testing"

	"github.com/lanternfold/prd-pr/internal/config"
)

func TestDeterministicWins(t *testing.T) {
	d := Route(Input{DeterministicSufficient: true, TaskComplexity: "high", ExpectedValue: 10})
	if d.SelectedModel != ModelNone {
		t.Fatalf("%+v", d)
	}
}

func TestBudgetAskHuman(t *testing.T) {
	zero := 0.0
	d := Route(Input{
		DeterministicSufficient: false,
		ExpectedValue:           1,
		BudgetRemainingUSD:      &zero,
		BudgetBreachPolicy:      config.BudgetAskHuman,
		FailureType:             "product",
	})
	if d.SelectedModel != ModelNone || d.BudgetBreach != config.BudgetAskHuman {
		t.Fatalf("%+v", d)
	}
}

func TestCheapThenStrong(t *testing.T) {
	d := Route(Input{FailureType: "product", ExpectedValue: 1, EvidenceSize: 100, Role: "review"})
	if d.SelectedModel != ModelCheap {
		t.Fatalf("cheap: %+v", d)
	}
	d = Route(Input{
		FailureType:      "ambiguous",
		ExpectedValue:    1,
		EvidenceSize:     9000,
		PreviousAttempts: 2,
		TaskComplexity:   "high",
		Role:             "diagnosis",
	})
	if d.SelectedModel != ModelStrong {
		t.Fatalf("strong: %+v", d)
	}
}
