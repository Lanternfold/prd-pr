package config

import (
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	c := Defaults()
	if c.HumanTimeout != 30*time.Second {
		t.Fatalf("human timeout = %s", c.HumanTimeout)
	}
	if c.MaxRepairAttempts != 3 {
		t.Fatalf("attempts = %d", c.MaxRepairAttempts)
	}
	if c.BudgetBreachPolicy != BudgetAskHuman {
		t.Fatalf("budget = %s", c.BudgetBreachPolicy)
	}
	if c.GitHubEnabled || c.RemoteVisibility() != VisibilityPrivate {
		t.Fatalf("github defaults %+v vis=%s", c, c.RemoteVisibility())
	}
	if c.InitialMessage() != "chore: initialize product" {
		t.Fatalf("initial=%s", c.InitialMessage())
	}
	if c.AutoMergeEnabled || c.MergeMethodName() != MergeSquash {
		t.Fatalf("automerge defaults enabled=%t method=%s", c.AutoMergeEnabled, c.MergeMethodName())
	}
}
