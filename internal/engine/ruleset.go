package engine

import (
	"context"
	"fmt"
	"strconv"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func (e *Engine) ensureRulesetLocked(ctx context.Context, g *state.Guard, st state.State, root string, doc *prd.Document, prdOnly bool) (state.State, error) {
	cfg := e.cfg()
	if !cfg.GitHubEnabled || !cfg.RulesetsEnabled {
		return st, nil
	}
	if st.Repository.Type != state.RepoTypeGitHub {
		return st, nil
	}
	if st.Repository.RulesetStatus == state.RulesetCreated || st.Repository.RulesetStatus == state.RulesetExists {
		return st, nil
	}
	if e.opts.GH == nil || !e.opts.GH.Available() || !e.opts.GH.Authenticated(ctx) {
		if prdOnly {
			return e.rulesetHuman(g, st, "GitHub authentication is required to configure repository rulesets")
		}
		return st, nil
	}
	owner, name := e.repoIdentity(doc)
	if owner == "" || name == "" {
		return st, nil
	}
	existing, err := e.opts.GH.ListRulesets(ctx, owner, name)
	if err != nil {
		if prdOnly {
			return e.rulesetHuman(g, st, "GitHub ruleset lookup failed: "+err.Error())
		}
		return st, nil
	}
	policy := rulesetPolicy(cfg)
	action, conflict, match := vcs.ReconcileRuleset(existing, policy)
	switch action {
	case "skip":
		st.Repository.RulesetStatus = state.RulesetSkipped
	case "reuse":
		st.Repository.RulesetStatus = state.RulesetExists
		st.Repository.RulesetName = match.Name
		if match.ID != 0 {
			st.Repository.RulesetID = strconv.FormatInt(match.ID, 10)
		}
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventRulesetConfigured,
			Payload: state.Payload(map[string]any{"action": "reuse", "name": match.Name, "id": match.ID}),
		})
	case "conflict":
		st.Repository.RulesetStatus = state.RulesetConflict
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventRulesetConflict,
			Payload: state.Payload(map[string]string{"reason": conflict}),
		})
		h := human.Request{
			Kind:    human.KindRulesetConflict,
			Reason:  "ruleset_conflict",
			Needed:  conflict + " Do not weaken GitHub rulesets. Resolve the conflict, then resume.",
			Urgency: human.UrgencyHigh,
		}
		_, _ = e.requestHumanLocked(g, st, root, h)
		st.CurrentState = state.StateWaitingForHuman
		return st, nil
	case "create":
		created, cerr := e.opts.GH.CreateRuleset(ctx, owner, name, policy)
		if cerr != nil {
			if prdOnly {
				return e.rulesetHuman(g, st, "GitHub ruleset creation failed: "+cerr.Error())
			}
			return st, nil
		}
		st.Repository.RulesetStatus = state.RulesetCreated
		st.Repository.RulesetName = firstNonEmpty(created.Name, vcs.BaselineRulesetName)
		if created.ID != 0 {
			st.Repository.RulesetID = strconv.FormatInt(created.ID, 10)
		}
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventRulesetConfigured,
			Payload: state.Payload(map[string]any{"action": "create", "name": st.Repository.RulesetName, "id": created.ID}),
		})
	}
	st.Bootstrap.RulesetDone = st.Repository.RulesetStatus == state.RulesetCreated || st.Repository.RulesetStatus == state.RulesetExists
	return st, nil
}

func (e *Engine) rulesetHuman(g *state.Guard, st state.State, reason string) (state.State, error) {
	root := st.ProductRoot
	h := human.Request{
		Kind:    human.KindGitHubAuth,
		Reason:  "github_ruleset",
		Needed:  reason + ". Restore GitHub access and run prdpr resume.",
		Urgency: human.UrgencyHigh,
	}
	_, _ = e.requestHumanLocked(g, st, root, h)
	st.CurrentState = state.StateWaitingForHuman
	return st, nil
}

func rulesetPolicy(cfg config.Config) vcs.RulesetPolicy {
	approvals := cfg.RequiredApprovals
	if cfg.RequireApproval && approvals < 1 {
		approvals = 1
	}
	return vcs.RulesetPolicy{
		Enabled:           cfg.RulesetsEnabled,
		DefaultBranch:     cfg.Branch(),
		RequirePR:         true,
		AllowForcePush:    cfg.AllowForcePush,
		AllowDeletion:     cfg.AllowBranchDeletion,
		RequiredChecks:    cfg.RequiredChecks,
		RequiredApprovals: approvals,
	}
}

func (e *Engine) ensureRulesetForRoot(ctx context.Context, root string, doc *prd.Document) error {
	store, err := state.Open(root)
	if err != nil {
		return err
	}
	g, err := store.Lock()
	if err != nil {
		return err
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return err
	}
	st, err = e.ensureRulesetLocked(ctx, g, st, root, doc, true)
	if err != nil {
		return err
	}
	if err := g.Save(st); err != nil {
		return err
	}
	if st.CurrentState == state.StateWaitingForHuman {
		return fmt.Errorf("waiting for human")
	}
	return nil
}
