package engine

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/plan"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/preflight"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/subagent"
)

const cursorAgentCheckID = "machine.cursor_agent"

// Prepare builds a deterministic task packet and Git baseline without invoking a worker.
func (e *Engine) Prepare(ctx context.Context, req Request) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Mode == "" {
		req.Mode = preflight.ModeInteractive
	}
	root, _, err := e.ensureWorkspace(ctx, req.ProductRoot)
	if err != nil {
		return refused("", err.Error()), nil
	}
	if !e.opts.AllowSelf && isOrchestratorRepo(root) {
		return refused(root, "refusing to prepare a coding task against the PRD→PR orchestrator repository"), nil
	}

	store, err := state.Open(root)
	if err != nil {
		return refused(root, err.Error()), nil
	}
	if _, err := store.Init(); err != nil {
		return refused(root, err.Error()), nil
	}
	g, err := store.Lock()
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = g.Unlock() }()

	st, err := g.Load()
	if err != nil {
		return refused(root, err.Error()), nil
	}
	st, err = e.recoverLocked(ctx, g, st)
	if err != nil {
		return Result{}, err
	}
	return e.prepareLocked(ctx, g, st, req, root)
}

func (e *Engine) prepareLocked(ctx context.Context, g *state.Guard, st state.State, req Request, root string) (Result, error) {
	prdPath := req.PRDPath
	if prdPath == "" {
		prdPath = filepath.Join(root, "PRD.md")
	}

	var doc *prd.Document
	if parsed, perr := prd.ParseFile(prdPath); perr == nil && parsed != nil && !parsed.HasErrors() {
		doc = parsed
	}
	var err error
	st, err = e.bootstrapRepo(ctx, g, st, root, doc)
	if err != nil {
		return e.persistPrepareRefusal(g, st, root, "", "repository bootstrap: "+err.Error())
	}

	mode, _ := preflight.NormalizeMode(req.Mode)
	worker := preflight.WorkerSession
	if e.fakeWorker() {
		worker = preflight.WorkerFake
	} else if mode == preflight.ModeHeadless {
		worker = preflight.WorkerCursor
	}
	rep := e.checker().Run(ctx, preflight.Request{ProductRoot: root, PRDPath: prdPath, Mode: mode, Worker: worker})
	if reason := prepareBlockingReason(rep, preflight.RequireCursorAgent(mode, worker)); reason != "" {
		return e.persistPrepareRefusal(g, st, root, "", "preflight blocked: "+reason)
	}
	if rep.PRDPath != "" {
		prdPath = rep.PRDPath
	}

	doc, err = prd.ParseFile(prdPath)
	if err != nil {
		return e.persistPrepareRefusal(g, st, root, "", "cannot read PRD: "+err.Error())
	}
	if doc.HasErrors() {
		return e.persistPrepareRefusal(g, st, root, "", "PRD is invalid")
	}

	gr := graph.FromDocument(doc)
	if gr.HasErrors() {
		return e.persistPrepareRefusal(g, st, root, "", "graph is invalid: "+graphError(gr))
	}
	if existing := loadGraph(root); existing != nil && !existing.HasErrors() {
		mergeGraphStatus(gr, existing)
	}
	gr.Refresh()
	if raw, err := gr.Marshal(); err == nil {
		_ = g.WriteFile(graphFile, append(raw, '\n'))
	}

	phaseID := req.PhaseID
	if phaseID == "" {
		phaseID = firstReadyPhase(gr)
		if phaseID == "" {
			if gr.AllCompleted() {
				return e.persistProjectCompleted(ctx, g, st, root)
			}
			return e.persistPrepareRefusal(g, st, root, "", "no ready phase")
		}
	}

	runID := "run_" + e.opts.NewID()
	st.CurrentRunID = runID
	st.CurrentPhaseID = string(phaseID)
	st, err = e.ensureExecutionBranch(ctx, g, st, root)
	if err != nil {
		return e.persistPrepareRefusal(g, st, root, string(phaseID), "branch: "+err.Error())
	}

	baseline, _, err := e.opts.Git.EstablishBaseline(ctx, root)
	if err != nil {
		return e.persistPrepareRefusal(g, st, root, string(phaseID), "git baseline: "+err.Error())
	}

	taskID := "task_" + e.opts.NewID()
	pkt, err := plan.DeterministicPlanner{}.Packet(plan.Input{
		Document:    doc,
		PhaseID:     phaseID,
		ProjectID:   st.ProjectID,
		TaskID:      taskID,
		ProductRoot: root,
	})
	if err != nil {
		return e.persistPrepareRefusal(g, st, root, string(phaseID), err.Error())
	}
	packetRel := filepath.Join(packetsDir, taskID+".json")
	packetBytes, err := packet.Marshal(pkt)
	if err != nil {
		return e.persistPrepareRefusal(g, st, root, string(phaseID), err.Error())
	}
	if err := g.WriteFile(packetRel, packetBytes); err != nil {
		return e.persistPrepareRefusal(g, st, root, string(phaseID), err.Error())
	}

	fc := human.Forecast{}
	if e.cfg().GitHubEnabled {
		fc.ExpectedReasons = append(fc.ExpectedReasons, "GitHub credential if gh is missing")
		fc.ExpectedCount++
		fc.ExpectedMinutes = 1
	}
	if raw, err := json.MarshalIndent(fc, "", "  "); err == nil {
		_ = g.WriteFile(forecastFile, append(raw, '\n'))
	}
	sub := subagent.Decide(subagent.Input{Complexity: "low"})
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventSubagentDecision,
		RunID:   runID,
		PhaseID: string(phaseID),
		Payload: state.Payload(map[string]string{"choice": sub.Choice, "reason": sub.Reason}),
	})

	ex := Execution{
		SchemaVersion:        1,
		RunID:                runID,
		TaskID:               taskID,
		ProjectID:            st.ProjectID,
		PhaseID:              string(phaseID),
		ProductRoot:          root,
		Baseline:             baseline,
		PacketRef:            packetRel,
		Invoked:              false,
		WorkerClaimedSuccess: false,
		ClaimedDone:          false,
		VerifiedSuccess:      false,
		RecordedAt:           e.opts.Now().UTC().Format(time.RFC3339Nano),
		Subagent:             sub.Choice,
	}
	if err := writeExecution(g, ex); err != nil {
		return Result{}, err
	}
	if err := g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventPrepared,
		RunID:   runID,
		PhaseID: string(phaseID),
		Payload: state.Payload(map[string]string{
			"task_id":  taskID,
			"packet":   packetRel,
			"baseline": baseline.SHA,
			"state":    state.StatePrepared,
		}),
	}); err != nil {
		return Result{}, err
	}
	st.CurrentRunID = runID
	st.CurrentPhaseID = string(phaseID)
	st.CurrentState = state.StatePrepared
	st.CurrentCommit = baseline.SHA
	if err := g.Save(st); err != nil {
		return Result{}, err
	}
	return Result{Execution: ex, Packet: pkt}, nil
}

func (e *Engine) persistPrepareRefusal(g *state.Guard, st state.State, root, phaseID, reason string) (Result, error) {
	ex := Execution{
		SchemaVersion:   1,
		RunID:           st.CurrentRunID,
		ProjectID:       st.ProjectID,
		PhaseID:         phaseID,
		ProductRoot:     root,
		Invoked:         false,
		RefusalReason:   reason,
		VerifiedSuccess: false,
		RecordedAt:      e.opts.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeExecution(g, ex); err != nil {
		return Result{}, err
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventPrepareRefused,
		RunID:   st.CurrentRunID,
		PhaseID: phaseID,
		Payload: state.Payload(map[string]string{"reason": reason}),
	})
	return Result{Execution: ex}, nil
}

func (e *Engine) checker() *preflight.Checker {
	env := e.opts.PreflightEnv
	if env.LookPath == nil && env.Git == nil && env.Now == nil && env.GOOS == "" {
		env = preflight.DefaultEnv()
	}
	if env.Now == nil {
		env.Now = e.opts.Now
	}
	if env.Git == nil {
		env.Git = e.opts.Git
	}
	return preflight.New(env)
}

func prepareBlockingReason(rep *preflight.Report, requireAgent bool) string {
	if rep == nil {
		return ""
	}
	var parts []string
	for _, c := range rep.Checks {
		if c.ID == cursorAgentCheckID && !requireAgent {
			continue
		}
		if c.ID == "execution.mode" {
			continue
		}
		if !c.Blocking && c.Status != preflight.StatusBlocking && c.Status != preflight.StatusError {
			continue
		}
		msg := c.Name
		if c.Detail != "" {
			msg = c.Name + ": " + c.Detail
		}
		parts = append(parts, msg)
	}
	return strings.Join(parts, "; ")
}

func (e *Engine) persistProjectCompleted(ctx context.Context, g *state.Guard, st state.State, root string) (Result, error) {
	st.ProjectStatus = state.StatusProjectCompleted
	st.CurrentState = state.StateCompleted
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventPhaseCompleted,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]string{"project_status": state.StatusProjectCompleted}),
	})
	st, err := e.maybeDeliverLocked(ctx, g, st, root)
	if err != nil {
		return Result{}, err
	}
	if err := g.Save(st); err != nil {
		return Result{}, err
	}
	return Result{
		ProjectCompleted: true,
		Execution: Execution{
			SchemaVersion:   1,
			RunID:           st.CurrentRunID,
			ProjectID:       st.ProjectID,
			PhaseID:         st.CurrentPhaseID,
			ProductRoot:     root,
			Invoked:         false,
			VerifiedSuccess: false,
			RecordedAt:      e.opts.Now().UTC().Format(time.RFC3339Nano),
		},
	}, nil
}

func graphError(g *graph.Graph) string {
	if g == nil {
		return "graph is nil"
	}
	for _, d := range g.Diagnostics {
		if d.Severity == graph.SevError {
			return d.Code + ": " + d.Message
		}
	}
	return "unknown graph error"
}
