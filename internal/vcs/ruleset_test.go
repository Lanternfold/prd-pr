package vcs

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestReconcileRulesetCreateReuseConflict(t *testing.T) {
	p := RulesetPolicy{Enabled: true, RequirePR: true, AllowForcePush: false, AllowDeletion: false, RequiredApprovals: 1}
	action, conflict, _ := ReconcileRuleset(nil, p)
	if action != "create" || conflict != "" {
		t.Fatalf("%s %s", action, conflict)
	}
	ours := Ruleset{Name: BaselineRulesetName, RequirePR: true, ForcePush: "denied", Deletion: "denied", Approvals: 1}
	action, conflict, got := ReconcileRuleset([]Ruleset{ours}, p)
	if action != "reuse" || conflict != "" || got.Name != BaselineRulesetName {
		t.Fatalf("%s %s %+v", action, conflict, got)
	}
	weak := Ruleset{Name: BaselineRulesetName, RequirePR: true, ForcePush: "unknown", Approvals: 0}
	action, conflict, _ = ReconcileRuleset([]Ruleset{weak}, p)
	if action != "conflict" || conflict == "" {
		t.Fatalf("%s %s", action, conflict)
	}
	other := Ruleset{Name: "legacy", ForcePush: "allowed"}
	action, conflict, _ = ReconcileRuleset([]Ruleset{other}, p)
	if action != "conflict" {
		t.Fatalf("%s %s", action, conflict)
	}
}

func TestCreateRulesetUsesPrivateAPIAndIsIdempotentAtClient(t *testing.T) {
	var posts int
	g := &GHClient{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(_ context.Context, _ string, args ...string) (string, error) {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "--method POST") {
				posts++
				return `{"id":9,"name":"prdpr-baseline","target":"branch","enforcement":"active","rules":[{"type":"non_fast_forward"},{"type":"deletion"},{"type":"pull_request","parameters":{"required_approving_review_count":0}}]}`, nil
			}
			if args[0] == "api" {
				return "[]", nil
			}
			return "", os.ErrNotExist
		},
	}
	rs, err := g.CreateRuleset(context.Background(), "acme", "app", RulesetPolicy{Enabled: true, RequirePR: true})
	if err != nil || rs.Name != BaselineRulesetName || posts != 1 {
		t.Fatalf("%+v posts=%d err=%v", rs, posts, err)
	}
	list, err := g.ListRulesets(context.Background(), "acme", "app")
	if err != nil || len(list) != 0 {
		t.Fatalf("%+v %v", list, err)
	}
}

func TestListRulesetsRequiresOwnerName(t *testing.T) {
	g := &GHClient{LookPath: func(string) (string, error) { return "/bin/gh", nil }}
	if _, err := g.ListRulesets(context.Background(), "", "x"); err == nil {
		t.Fatal("expected error")
	}
}
