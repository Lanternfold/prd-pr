package vcs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const BaselineRulesetName = "prdpr-baseline"

// Ruleset is a GitHub repository ruleset. It is not a Cursor Rule.
type Ruleset struct {
	ID          int64    `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Enforcement string   `json:"enforcement,omitempty"`
	Target      string   `json:"target,omitempty"`
	Rules       []string `json:"rules,omitempty"`
	ForcePush   string   `json:"force_push,omitempty"`
	Deletion    string   `json:"deletion,omitempty"`
	Approvals   int      `json:"approvals,omitempty"`
	Checks      []string `json:"checks,omitempty"`
	RequirePR   bool     `json:"require_pr,omitempty"`
}

// RulesetPolicy is the engine's desired baseline. Auto-merge is not configured here.
type RulesetPolicy struct {
	Enabled           bool
	DefaultBranch     string
	RequirePR         bool
	AllowForcePush    bool
	AllowDeletion     bool
	RequiredChecks    []string
	RequiredApprovals int
}

type rulesetAPI struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Target      string `json:"target"`
	Enforcement string `json:"enforcement"`
	Rules       []struct {
		Type       string          `json:"type"`
		Parameters json.RawMessage `json:"parameters"`
	} `json:"rules"`
}

func (g *GHClient) ListRulesets(ctx context.Context, owner, name string) ([]Ruleset, error) {
	owner, name = strings.TrimSpace(owner), strings.TrimSpace(name)
	if owner == "" || name == "" {
		return nil, fmt.Errorf("repository owner and name are required")
	}
	if !g.Available() {
		return nil, fmt.Errorf("gh is not available")
	}
	out, err := g.gh()(ctx, "", "api", "repos/"+owner+"/"+name+"/rulesets")
	if err != nil {
		return nil, err
	}
	out = strings.TrimSpace(out)
	if out == "" || out == "[]" {
		return nil, nil
	}
	var raw []rulesetAPI
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		var one rulesetAPI
		if err2 := json.Unmarshal([]byte(out), &one); err2 != nil {
			return nil, fmt.Errorf("decode rulesets: %w", err)
		}
		raw = []rulesetAPI{one}
	}
	var outList []Ruleset
	for _, r := range raw {
		outList = append(outList, parseRuleset(r))
	}
	return outList, nil
}

func (g *GHClient) CreateRuleset(ctx context.Context, owner, name string, policy RulesetPolicy) (Ruleset, error) {
	owner, name = strings.TrimSpace(owner), strings.TrimSpace(name)
	if owner == "" || name == "" {
		return Ruleset{}, fmt.Errorf("repository owner and name are required")
	}
	if !g.Available() {
		return Ruleset{}, fmt.Errorf("gh is not available")
	}
	body, err := json.Marshal(rulesetCreateBody(policy))
	if err != nil {
		return Ruleset{}, err
	}
	tmp, err := os.CreateTemp("", "prdpr-ruleset-*.json")
	if err != nil {
		return Ruleset{}, err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return Ruleset{}, err
	}
	if err := tmp.Close(); err != nil {
		return Ruleset{}, err
	}
	out, err := g.gh()(ctx, "", "api", "--method", "POST", "repos/"+owner+"/"+name+"/rulesets", "--input", tmpName)
	if err != nil {
		return Ruleset{}, err
	}
	var raw rulesetAPI
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return Ruleset{Name: BaselineRulesetName}, nil
	}
	return parseRuleset(raw), nil
}

func parseRuleset(r rulesetAPI) Ruleset {
	rs := Ruleset{ID: r.ID, Name: r.Name, Enforcement: r.Enforcement, Target: r.Target, ForcePush: "unknown", Deletion: "unknown"}
	for _, rule := range r.Rules {
		rs.Rules = append(rs.Rules, rule.Type)
		switch rule.Type {
		case "non_fast_forward":
			rs.ForcePush = "denied"
		case "deletion":
			rs.Deletion = "denied"
		case "pull_request":
			rs.RequirePR = true
			var p struct {
				RequiredApprovingReviewCount int `json:"required_approving_review_count"`
			}
			_ = json.Unmarshal(rule.Parameters, &p)
			rs.Approvals = p.RequiredApprovingReviewCount
		case "required_status_checks":
			var p struct {
				RequiredStatusChecks []struct {
					Context string `json:"context"`
				} `json:"required_status_checks"`
			}
			_ = json.Unmarshal(rule.Parameters, &p)
			for _, c := range p.RequiredStatusChecks {
				if c.Context != "" {
					rs.Checks = append(rs.Checks, c.Context)
				}
			}
		}
	}
	return rs
}

func rulesetCreateBody(p RulesetPolicy) map[string]any {
	branch := strings.TrimSpace(p.DefaultBranch)
	if branch == "" {
		branch = "main"
	}
	rules := []map[string]any{}
	if p.RequirePR {
		approvals := p.RequiredApprovals
		if approvals < 0 {
			approvals = 0
		}
		rules = append(rules, map[string]any{
			"type": "pull_request",
			"parameters": map[string]any{
				"required_approving_review_count":   approvals,
				"dismiss_stale_reviews_on_push":     false,
				"required_review_thread_resolution": false,
				"require_code_owner_review":         false,
				"require_last_push_approval":        false,
			},
		})
	}
	if !p.AllowForcePush {
		rules = append(rules, map[string]any{"type": "non_fast_forward"})
	}
	if !p.AllowDeletion {
		rules = append(rules, map[string]any{"type": "deletion"})
	}
	if len(p.RequiredChecks) > 0 {
		var checks []map[string]string
		for _, c := range p.RequiredChecks {
			c = strings.TrimSpace(c)
			if c != "" {
				checks = append(checks, map[string]string{"context": c})
			}
		}
		if len(checks) > 0 {
			rules = append(rules, map[string]any{
				"type": "required_status_checks",
				"parameters": map[string]any{
					"strict_required_status_checks_policy": true,
					"required_status_checks":               checks,
				},
			})
		}
	}
	return map[string]any{
		"name":        BaselineRulesetName,
		"target":      "branch",
		"enforcement": "active",
		"conditions": map[string]any{
			"ref_name": map[string]any{
				"include": []string{"refs/heads/" + branch},
				"exclude": []string{},
			},
		},
		"rules": rules,
	}
}

// ReconcileRuleset inspects existing rulesets and creates the baseline when missing.
// It never weakens an existing policy. Conflict is returned for the engine to ask a human.
func ReconcileRuleset(existing []Ruleset, policy RulesetPolicy) (action string, conflict string, match Ruleset) {
	if !policy.Enabled {
		return "skip", "", Ruleset{}
	}
	var ours Ruleset
	for _, rs := range existing {
		if rs.Name == BaselineRulesetName {
			ours = rs
			break
		}
	}
	if ours.Name != "" {
		if rulesetConflicts(ours, policy) {
			return "conflict", "existing prdpr-baseline ruleset conflicts with required governance; refusing to overwrite", ours
		}
		return "reuse", "", ours
	}
	for _, rs := range existing {
		if otherConflicts(rs, policy) {
			return "conflict", "an existing repository ruleset conflicts with required governance; refusing to overwrite it", rs
		}
	}
	return "create", "", Ruleset{}
}

func rulesetConflicts(rs Ruleset, p RulesetPolicy) bool {
	if !p.AllowForcePush && rs.ForcePush != "denied" {
		return true
	}
	if !p.AllowDeletion && rs.Deletion != "denied" {
		return true
	}
	if p.RequirePR && !rs.RequirePR && len(rs.Rules) > 0 {
		return true
	}
	if p.RequiredApprovals > 0 && rs.Approvals < p.RequiredApprovals && rs.RequirePR {
		return true
	}
	return false
}

func otherConflicts(rs Ruleset, p RulesetPolicy) bool {
	if rs.Name == BaselineRulesetName {
		return false
	}
	if !p.AllowForcePush && rs.ForcePush == "allowed" {
		return true
	}
	return false
}
