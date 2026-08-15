package engine

import (
	"context"
	"strings"

	"github.com/lanternfold/prd-pr/internal/ci"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func (e *Engine) recoverLocked(ctx context.Context, g *state.Guard, st state.State) (state.State, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	from := st.CurrentState
	switch st.CurrentState {
	case state.StateWorkerRunning:
		if _, err := loadExecutionFromGuard(g); err == nil {
			st.CurrentState = state.StateWorkerClaimedDone
		} else {
			st.CurrentState = state.StateWaitingForHuman
		}
		if err := e.noteRecovered(g, st, from); err != nil {
			return st, err
		}
	case state.StateVerifying:
		st.CurrentState = state.StateWorkerClaimedDone
		if err := e.noteRecovered(g, st, from); err != nil {
			return st, err
		}
	case state.StateVerified:
		if err := e.completeLocked(ctx, g, st, g.Jail().Root()); err != nil {
			return st, err
		}
		st, err := g.Load()
		if err != nil {
			return st, err
		}
		_ = e.noteRecovered(g, st, from)
		st, err = e.reconcileDeliveryLocked(ctx, g, st, g.Jail().Root())
		return st, err
	case state.StateCompleted:
		var err error
		st, err = e.pushLocked(ctx, g, st, g.Jail().Root(), false)
		if err != nil {
			return st, err
		}
		st, err = e.reconcileDeliveryLocked(ctx, g, st, g.Jail().Root())
		if err != nil {
			return st, err
		}
		if err := g.Save(st); err != nil {
			return st, err
		}
		return st, nil
	}
	var err error
	st, err = e.reconcileDeliveryLocked(ctx, g, st, g.Jail().Root())
	if err != nil {
		return st, err
	}
	return st, nil
}

func (e *Engine) reconcileDeliveryLocked(ctx context.Context, g *state.Guard, st state.State, root string) (state.State, error) {
	cfg := e.cfg()
	remote := firstNonEmpty(st.Repository.RemoteName, cfg.Remote())
	base := firstNonEmpty(st.Repository.BaseBranch, cfg.Branch())
	ref := firstNonEmpty(st.Repository.FeatureBranch, st.Repository.Branch, cfg.Branch())
	sha := st.CurrentCommit
	if sha == "" {
		if sn, err := e.opts.Git.Inspect(ctx, root); err == nil {
			sha = sn.HeadSHA
		}
	}
	basePush := cfg.FeatureBranches() && ref != "" && base != "" && ref == base
	if !basePush && e.opts.Git.Pushed(ctx, root, remote, ref, sha) {
		st.Repository.PushStatus = state.PushPushed
	}
	if e.opts.GH == nil || !e.opts.GH.Available() {
		return st, nil
	}
	if !cfg.GitHubEnabled && st.Repository.PRNumber == "" {
		return st, nil
	}
	if !e.opts.GH.Authenticated(ctx) {
		return st, nil
	}
	head := firstNonEmpty(st.Repository.FeatureBranch, st.Repository.Branch)
	if st.Repository.PRNumber == "" {
		if found, err := e.opts.GH.FindOpenPR(ctx, root, head, base); err == nil && found.Number != "" {
			st.Repository.PRNumber = found.Number
			st.Repository.PRURL = found.URL
			st.Repository.PRHead = found.Head
			st.Repository.PRBase = found.Base
			st.Repository.PRSHA = found.SHA
			if st.Repository.MergeStatus != state.PRMerged {
				st.Repository.MergeStatus = state.PROpen
			}
			_ = g.AppendEvent(state.Event{
				Kind:    state.KindResult,
				Name:    state.EventPRReused,
				RunID:   st.CurrentRunID,
				PhaseID: st.CurrentPhaseID,
				Payload: state.Payload(map[string]any{"number": found.Number, "url": found.URL, "recovered": true}),
			})
		}
	}
	if st.Repository.PRNumber == "" {
		return st, nil
	}
	pr, err := e.opts.GH.ViewPR(ctx, root, st.Repository.PRNumber)
	if err != nil || pr.Skipped {
		return st, nil
	}
	if strings.EqualFold(pr.State, "merged") {
		st = e.recordMerged(g, st, pr, vcs.MergeResult{Merged: true, SHA: pr.SHA, Method: firstNonEmpty(st.Repository.MergeMethod, cfg.MergeMethodName())}, cfg)
		st = e.cleanupMergedBranch(ctx, g, st, root, pr, cfg)
		return st, nil
	}
	checks := e.opts.CI.PRChecks(ctx, root, pr.Number)
	st.Repository.ChecksStatus = checks.Verdict()
	if st.Repository.MergeStatus != state.PRMerged {
		switch checks.Verdict() {
		case ci.VerdictPending:
			st.Repository.MergeStatus = state.PRChecking
		default:
			if st.Repository.MergeStatus == "" {
				st.Repository.MergeStatus = state.PROpen
			}
		}
	}
	return st, nil
}

func (e *Engine) noteRecovered(g *state.Guard, st state.State, from string) error {
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventRecovered,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]string{"from": from, "to": st.CurrentState}),
	})
	return g.Save(st)
}

// Reconcile inspects Git/GitHub reality and repairs persisted delivery state without repeating operations.
func (e *Engine) Reconcile(ctx context.Context, root string) error {
	root, _, err := identifyRoot(root)
	if err != nil {
		return err
	}
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
	st, err = e.recoverLocked(ctx, g, st)
	if err != nil {
		return err
	}
	return g.Save(st)
}
