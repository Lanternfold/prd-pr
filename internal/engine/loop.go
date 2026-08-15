package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/knowledge"
	"github.com/lanternfold/prd-pr/internal/preflight"
	"github.com/lanternfold/prd-pr/internal/repair"
	"github.com/lanternfold/prd-pr/internal/review"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

type PhaseResult struct {
	Execution        Execution
	Verification     testeng.Result
	Incident         repair.Incident
	Human            *human.Request
	Completed        bool
	Waiting          bool
	ProjectCompleted bool
	Phases           []string
}

// RunPhase is the headless sequential loop for one phase: worker, verify, review, bounded repair.
func (e *Engine) RunPhase(ctx context.Context, req Request) (PhaseResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Mode == "" {
		req.Mode = preflight.ModeHeadless
	}
	out := PhaseResult{}
	run, err := e.Run(ctx, req)
	if err != nil {
		return out, err
	}
	out.Execution = run.Execution
	out.Human = run.Human
	out.Waiting = run.WaitingForHuman
	out.ProjectCompleted = run.ProjectCompleted
	if run.Execution.PhaseID != "" {
		out.Phases = []string{run.Execution.PhaseID}
	}
	if run.WaitingForHuman || run.ProjectCompleted {
		return out, nil
	}
	if run.Execution.RefusalReason != "" && !run.Execution.Invoked {
		return out, nil
	}
	v, err := e.Verify(ctx, req)
	if err != nil {
		return out, err
	}
	out.Verification = v
	if v.VerifiedSuccess {
		if waiting, h := e.deliveryWaiting(run.Execution.ProductRoot); waiting {
			out.Waiting = true
			out.Human = h
			return out, nil
		}
		out.Completed = true
		return out, nil
	}
	if v.Status == testeng.StatusManual || (v.ManualVerificationRequired && v.TestsPass) {
		h, herr := e.RequestHuman(ctx, req.ProductRoot, human.Request{
			Kind:      human.KindManualAC,
			Reason:    "manual_acceptance",
			Phase:     v.PhaseID,
			Task:      v.TaskID,
			Attempted: v.Reason,
			Needed:    "Confirm the required manual acceptance criteria. Do not paste secrets.",
			Urgency:   human.UrgencyNormal,
		})
		out.Human = &h
		out.Waiting = true
		return out, herr
	}

	for i := 0; i < repair.MaxAttempts; i++ {
		rev, err := e.Review(ctx, req)
		if err != nil {
			return out, err
		}
		if !rev.Repair {
			h, herr := e.RequestHuman(ctx, req.ProductRoot, humanFromReview(rev, v))
			out.Human = &h
			out.Waiting = true
			return out, herr
		}
		rp, err := e.PrepareRepair(ctx, req)
		if err != nil {
			h, _ := e.RequestHuman(ctx, req.ProductRoot, human.Request{
				Kind: human.KindRepairFail, Reason: err.Error(), Needed: "human inspection", Urgency: human.UrgencyHigh,
			})
			out.Human = &h
			out.Waiting = true
			return out, nil
		}
		wres := e.applyRepairWorker(ctx, run.Execution.ProductRoot, rp)
		wres.VerifiedSuccess = false
		v, err = e.Verify(ctx, req)
		if err != nil {
			return out, err
		}
		out.Verification = v
		store, err := state.Open(run.Execution.ProductRoot)
		if err != nil {
			return out, err
		}
		guard, err := store.Lock()
		if err != nil {
			return out, err
		}
		inc := e.recordRepairAttempt(guard, run.Execution.ProductRoot, rp, v, wres)
		_ = guard.Unlock()
		out.Incident = inc
		if v.VerifiedSuccess {
			out.Completed = true
			return out, nil
		}
		if inc.Exhausted || !repair.CanAttempt(inc, e.cfg()) {
			h, _ := e.RequestHuman(ctx, req.ProductRoot, human.Request{
				Kind:      human.KindRepairFail,
				Reason:    "repeated_repair_failure",
				Phase:     inc.PhaseID,
				Task:      inc.TaskID,
				Attempted: fmt.Sprintf("%d attempts", len(inc.Attempts)),
				Needed:    inc.HumanAction,
				Urgency:   human.UrgencyHigh,
			})
			out.Human = &h
			out.Waiting = true
			return out, nil
		}
	}
	return out, nil
}

// RunGraph walks READY phases sequentially. Each phase uses RunPhase as the inner loop.
func (e *Engine) RunGraph(ctx context.Context, req Request) (PhaseResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Mode == "" {
		req.Mode = preflight.ModeHeadless
	}
	var acc PhaseResult
	seen := map[string]bool{}
	phaseReq := req
	root := strings.TrimSpace(req.ProductRoot)
	if gr := loadGraph(root); gr != nil {
		gr.Refresh()
		if gr.AllCompleted() {
			acc.ProjectCompleted = true
			return acc, nil
		}
		if w := firstWaitingPhase(gr); w != "" {
			acc.Waiting = true
			return acc, nil
		}
	}
	for {
		res, err := e.RunPhase(ctx, phaseReq)
		if res.Execution.PhaseID != "" {
			acc.Phases = append(acc.Phases, res.Execution.PhaseID)
		}
		acc.Execution = res.Execution
		acc.Verification = res.Verification
		acc.Incident = res.Incident
		acc.Human = res.Human
		acc.Completed = res.Completed
		acc.Waiting = res.Waiting
		acc.ProjectCompleted = res.ProjectCompleted
		if err != nil {
			return acc, err
		}
		if res.Waiting || res.ProjectCompleted {
			return acc, nil
		}
		if !res.Completed {
			return acc, nil
		}
		root := res.Execution.ProductRoot
		if root == "" {
			root = req.ProductRoot
		}
		if res.Execution.PhaseID != "" {
			if seen[res.Execution.PhaseID] {
				return acc, nil
			}
			seen[res.Execution.PhaseID] = true
		}
		gr := loadGraph(root)
		if gr == nil {
			return acc, nil
		}
		gr.Refresh()
		if gr.AllCompleted() {
			acc.ProjectCompleted = true
			return acc, nil
		}
		if w := firstWaitingPhase(gr); w != "" {
			acc.Waiting = true
			return acc, nil
		}
		next := firstReadyPhase(gr)
		if next == "" {
			return acc, nil
		}
		phaseReq.ProductRoot = root
		phaseReq.PhaseID = next
		phaseReq.PRDPath = req.PRDPath
		phaseReq.Mode = req.Mode
		phaseReq.PRDOnly = req.PRDOnly
	}
}

func (e *Engine) completeLocked(ctx context.Context, g *state.Guard, st state.State, root string) error {
	ks := knowledge.ProjectStore(root)
	_, _ = ks.Put(knowledge.Entry{
		Category:    "test_command",
		Scope:       knowledge.ScopeProject,
		Observation: "go test ./...",
		Evidence:    "P7 verification",
		Source:      "verify",
		Confidence:  0.8,
	})
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventKnowledgeUpdated,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]string{"category": "test_command"}),
	})
	if e.cfg().AutoCommit {
		ex, _ := loadExecutionFromGuard(g)
		var err error
		st, err = e.checkpointLocked(ctx, g, st, root, ex)
		if err != nil {
			return err
		}
		st, err = e.pushLocked(ctx, g, st, root, false)
		if err != nil {
			return err
		}
		if st.CurrentState == state.StateWaitingForHuman {
			return g.Save(st)
		}
		st, err = e.maybeDeliverLocked(ctx, g, st, root)
		if err != nil {
			return err
		}
		if st.CurrentState == state.StateWaitingForHuman {
			return g.Save(st)
		}
	}
	if gr := loadGraph(root); gr != nil {
		markPhase(gr, st.CurrentPhaseID, graph.StatusCompleted)
		persistGraph(g, gr)
	}
	st.CurrentState = state.StateCompleted
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventPhaseCompleted,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
	})
	if gr := loadGraph(root); gr != nil && gr.AllCompleted() {
		st.ProjectStatus = state.StatusProjectCompleted
		if !e.cfg().AutoCommit {
			var err error
			st, err = e.maybeDeliverLocked(ctx, g, st, root)
			if err != nil {
				return err
			}
			if st.CurrentState == state.StateWaitingForHuman {
				return g.Save(st)
			}
		}
		_, err := e.startRuntimeLocked(ctx, g, st, root)
		return err
	}
	return g.Save(st)
}

func (e *Engine) deliveryWaiting(root string) (bool, *human.Request) {
	root = strings.TrimSpace(root)
	if root == "" {
		return false, nil
	}
	store, err := state.Open(root)
	if err != nil {
		return false, nil
	}
	snap, err := store.Load()
	if err != nil || snap.CurrentState != state.StateWaitingForHuman {
		return false, nil
	}
	req, err := human.LoadRequest(root)
	if err != nil {
		return true, nil
	}
	return true, &req
}

func (e *Engine) CommitVerified(ctx context.Context, root, message string) (string, error) {
	root, jail, err := identifyRoot(root)
	if err != nil {
		return "", err
	}
	if _, err := e.requireVerifiedExecution(root); err != nil {
		return "", err
	}
	if message == "" {
		message = "prdpr: verified changes"
	}
	sha, err := e.opts.Git.Commit(ctx, root, message, nil, jail)
	if err != nil {
		return "", err
	}
	store, err := state.Open(root)
	if err == nil {
		if snap, lerr := store.Load(); lerr == nil {
			snap.CurrentCommit = sha
			snap.LastKnownGoodCommit = sha
			if snap.Repository.PhaseCheckpoints == nil {
				snap.Repository.PhaseCheckpoints = map[string]string{}
			}
			if snap.CurrentPhaseID != "" {
				snap.Repository.PhaseCheckpoints[snap.CurrentPhaseID] = sha
			}
			snap.Repository.LatestCheckpointSHA = sha
			snap.Repository.CommitStatus = "checkpointed"
			_ = store.Save(snap)
			_ = store.AppendEvent(state.Event{
				Kind:    state.KindResult,
				Name:    state.EventCommitCreated,
				PhaseID: snap.CurrentPhaseID,
				Payload: state.Payload(map[string]string{"sha": sha, "kind": "phase", "message": message}),
			})
		}
	}
	return sha, nil
}

func (e *Engine) requireVerifiedExecution(root string) (Execution, error) {
	st, err := state.Open(root)
	if err != nil {
		return Execution{}, fmt.Errorf("commit/pr refused: %w", err)
	}
	snap, err := st.Load()
	if err != nil {
		return Execution{}, fmt.Errorf("commit/pr refused: verification has not run")
	}
	ex, err := loadExecution(root)
	if err != nil {
		return Execution{}, fmt.Errorf("commit/pr refused: verification has not run")
	}
	switch snap.CurrentState {
	case state.StateWaitingForHuman:
		return Execution{}, fmt.Errorf("commit/pr refused: human intervention is pending")
	case state.StateRepairing:
		return Execution{}, fmt.Errorf("commit/pr refused: repair is pending")
	case state.StateVerificationFailed:
		return Execution{}, fmt.Errorf("commit/pr refused: verification failed")
	case state.StateVerificationIncomplete:
		return Execution{}, fmt.Errorf("commit/pr refused: verification is incomplete")
	case state.StateVerifying, state.StateReviewing:
		return Execution{}, fmt.Errorf("commit/pr refused: verification has not completed")
	}
	if !ex.VerifiedSuccess {
		return Execution{}, fmt.Errorf("commit/pr refused: verified_success is not true")
	}
	switch snap.CurrentState {
	case state.StateVerified, state.StateCompleted:
		return ex, nil
	default:
		return Execution{}, fmt.Errorf("commit/pr refused: phase is not VERIFIED (state %s)", snap.CurrentState)
	}
}

func (e *Engine) CIStatus(ctx context.Context, root string) interface{} {
	return e.opts.CI.Status(ctx, root)
}

func humanFromReview(rev review.Result, v testeng.Result) human.Request {
	kind := human.KindBlocked
	if rev.Diagnosis.HumanReason == "manual_acceptance" {
		kind = human.KindManualAC
	}
	if rev.Diagnosis.HumanReason == "cost_budget" {
		kind = human.KindBudget
	}
	needed := rev.Diagnosis.HumanReason
	if needed == "" {
		needed = rev.Diagnosis.Summary
	}
	return human.Request{
		Kind:      kind,
		Reason:    rev.Diagnosis.Classification,
		Needed:    needed,
		Attempted: v.Reason,
		Phase:     v.PhaseID,
		Task:      v.TaskID,
		Urgency:   human.UrgencyNormal,
		Optional:  false,
	}
}
