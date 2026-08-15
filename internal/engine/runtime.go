package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/apprun"
	"github.com/lanternfold/prd-pr/internal/diagnose"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/llm"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/repair"
	"github.com/lanternfold/prd-pr/internal/review"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func (e *Engine) runtimeStarter() apprun.Starter {
	if e.opts.Runtime != nil {
		return e.opts.Runtime
	}
	return apprun.ProcStarter{Runner: e.opts.ProcRunner}
}

// StartRuntime starts the local application after verified project completion.
func (e *Engine) StartRuntime(ctx context.Context, root string) (apprun.Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, _, err := identifyRoot(root)
	if err != nil {
		return apprun.Report{Error: err.Error()}, nil
	}
	store, err := state.Open(root)
	if err != nil {
		return apprun.Report{Error: err.Error()}, nil
	}
	g, err := store.Lock()
	if err != nil {
		return apprun.Report{}, err
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return apprun.Report{Error: err.Error()}, nil
	}
	return e.startRuntimeLocked(ctx, g, st, root)
}

func (e *Engine) startRuntimeLocked(ctx context.Context, g *state.Guard, st state.State, root string) (apprun.Report, error) {
	def := apprun.Load(root)
	if def.Kind == "" {
		def = apprun.ForType(st.ProjectType)
	}
	if def.Kind == "" || def.Kind == apprun.KindNone {
		rep := apprun.Report{Kind: apprun.KindNone, Skipped: true, Reason: "no local application runtime for this project type"}
		st.Runtime.Status = state.RuntimeSkipped
		st.Runtime.Kind = def.Kind
		_ = g.AppendEvent(state.Event{Kind: state.KindResult, Name: state.EventRuntimeSkipped, Payload: state.Payload(map[string]string{"reason": rep.Reason})})
		_ = g.Save(st)
		return rep, nil
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindIntent,
		Name:    state.EventRuntimeStarted,
		RunID:   st.CurrentRunID,
		Payload: state.Payload(map[string]string{"kind": def.Kind, "command": def.Command}),
	})
	rep := e.runtimeStarter().Start(ctx, root, def)
	st.Runtime.Kind = def.Kind
	st.Runtime.Command = strings.TrimSpace(def.Command + " " + strings.Join(def.Args, " "))
	st.Runtime.URL = firstNonEmpty(rep.URL, def.URL)
	st.Runtime.Ready = rep.Ready
	st.Runtime.LastError = firstNonEmpty(rep.Error, rep.Reason)
	if rep.Skipped {
		st.Runtime.Status = state.RuntimeSkipped
		_ = g.AppendEvent(state.Event{Kind: state.KindResult, Name: state.EventRuntimeSkipped, Payload: state.Payload(map[string]string{"reason": rep.Reason})})
		_ = g.Save(st)
		return rep, nil
	}
	if rep.Ready {
		st.Runtime.Status = state.RuntimeReady
		if def.KeepAlive {
			st.Runtime.Status = state.RuntimeRunning
		}
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventRuntimeReady,
			Payload: state.Payload(map[string]any{"url": st.Runtime.URL, "keep_alive": def.KeepAlive}),
		})
		_ = g.Save(st)
		return rep, nil
	}
	st.Runtime.Status = state.RuntimeFailed
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventRuntimeFailed,
		Payload: state.Payload(map[string]string{"error": st.Runtime.LastError, "stdout": clipLog(rep.Stdout), "stderr": clipLog(rep.Stderr)}),
	})
	_ = g.Save(st)
	return rep, nil
}

func clipLog(s string) string {
	if len(s) > 2000 {
		return s[:2000]
	}
	return s
}

func isInfraRuntime(rep apprun.Report) bool {
	msg := strings.ToLower(rep.Error + " " + rep.Stderr)
	for _, n := range []string{"executable file not found", "no such file", "network", "connection refused", "temporary failure", "rate limit"} {
		if strings.Contains(msg, n) && strings.Contains(msg, "go") && strings.Contains(msg, "not found") {
			return true
		}
		if strings.Contains(msg, "executable file not found") || strings.Contains(msg, "network is unreachable") {
			return true
		}
	}
	return false
}

// RepairRuntime runs bounded runtime repair after a failed local start.
func (e *Engine) RepairRuntime(ctx context.Context, root string) (repair.Packet, apprun.Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, _, err := identifyRoot(root)
	if err != nil {
		return repair.Packet{}, apprun.Report{Error: err.Error()}, nil
	}
	store, err := state.Open(root)
	if err != nil {
		return repair.Packet{}, apprun.Report{}, err
	}
	g, err := store.Lock()
	if err != nil {
		return repair.Packet{}, apprun.Report{}, err
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return repair.Packet{}, apprun.Report{}, err
	}
	rep, err := e.startRuntimeLocked(ctx, g, st, root)
	if err != nil {
		return repair.Packet{}, rep, err
	}
	if rep.Ready || rep.Skipped {
		return repair.Packet{}, rep, nil
	}
	if isInfraRuntime(rep) {
		h := human.Request{
			Kind:    human.KindRuntimeFail,
			Reason:  "infrastructure",
			Needed:  "Local startup failed due to infrastructure: " + firstNonEmpty(rep.Error, rep.Reason) + ". Fix the environment, then prdpr runtime.",
			Urgency: human.UrgencyHigh,
		}
		_, _ = e.requestHumanLocked(g, st, root, h)
		return repair.Packet{}, rep, nil
	}

	inc := loadIncident(root)
	if inc.ID != "" && strings.HasPrefix(inc.ID, "rt_") && !repair.CanAttempt(inc, e.cfg()) {
		inc.Exhausted = true
		inc.HumanAction = runtimeHumanAction(st, rep, inc)
		_ = saveIncident(g, inc)
		_, _ = e.requestHumanLocked(g, st, root, human.Request{
			Kind:      human.KindRuntimeFail,
			Reason:    "runtime_repair_exhausted",
			Needed:    inc.HumanAction,
			Attempted: fmt.Sprintf("%d attempts", len(inc.Attempts)),
			Urgency:   human.UrgencyHigh,
		})
		return repair.Packet{}, rep, fmt.Errorf("runtime repair exhausted")
	}

	diag := diagnose.Report{
		Actionable:      true,
		Classification:  diagnose.ClassProduct,
		Summary:         firstNonEmpty(rep.Error, "application failed to start"),
		ConsumesAttempt: true,
		Confidence:      0.6,
		NeedsLLM:        true,
	}
	if _, ok := e.opts.LLM.(llm.None); !ok && e.opts.LLM != nil {
		if resp, err := e.opts.LLM.Complete(ctx, llm.Request{
			Role:   llm.RoleRuntime,
			System: "Diagnose local app startup failures. No secrets. Do not authorize Git operations.",
			Prompt: "error: " + rep.Error + "\nstderr: " + clipLog(rep.Stderr),
		}); err == nil && strings.TrimSpace(resp.Text) != "" {
			diag.Summary = strings.TrimSpace(resp.Text)
		}
	}
	st, _ = g.Load()
	v := testeng.Result{Status: testeng.StatusFailed, Reason: diag.Summary, RunID: st.CurrentRunID, PhaseID: st.CurrentPhaseID}
	pkt, err := e.prepareRuntimeRepairLocked(g, st, root, diag, v)
	return pkt, rep, err
}

func (e *Engine) prepareRuntimeRepairLocked(g *state.Guard, st state.State, root string, diag diagnose.Report, v testeng.Result) (repair.Packet, error) {
	inc := loadIncident(root)
	if inc.ID == "" || !strings.HasPrefix(inc.ID, "rt_") {
		inc = repair.Incident{
			SchemaVersion: repair.SchemaVersion,
			ID:            "rt_" + e.opts.NewID(),
			PhaseID:       firstNonEmpty(st.CurrentPhaseID, "runtime"),
			TaskID:        firstNonEmpty(st.CurrentRunID, "runtime"),
			RunID:         st.CurrentRunID,
			MaxAttempts:   repair.Max(e.cfg()),
		}
	}
	if !repair.CanAttempt(inc, e.cfg()) {
		inc.Exhausted = true
		inc.HumanAction = runtimeHumanAction(st, apprun.Report{Error: diag.Summary}, inc)
		_ = saveIncident(g, inc)
		_, _ = e.requestHumanLocked(g, st, root, human.Request{
			Kind: human.KindRuntimeFail, Reason: "runtime_repair_exhausted", Needed: inc.HumanAction, Urgency: human.UrgencyHigh,
		})
		return repair.Packet{}, fmt.Errorf("runtime repair exhausted")
	}
	orig := packet.Packet{
		SchemaVersion: packet.SchemaVersion,
		TaskID:        firstNonEmpty(st.CurrentRunID, "runtime"),
		ProjectID:     st.ProjectID,
		PhaseID:       firstNonEmpty(st.CurrentPhaseID, "P1"),
		Objective:     "Repair local application startup so the process becomes ready.",
		ProductRoot:   root,
		Constraints:   []string{"Stay inside product_root.", "Do not execute arbitrary PRD shell.", "Do not weaken Git safety."},
	}
	rev := review.Result{Repair: true, Diagnosis: diag}
	rp := repair.NewPacket(inc, orig, rev, v, clipLog(st.Runtime.LastError))
	rp.ProductRoot = root
	data, err := repair.Marshal(rp)
	if err != nil {
		return repair.Packet{}, err
	}
	rel := filepath.ToSlash(filepath.Join(packetsDir, fmt.Sprintf("repair_%s_%d.json", inc.ID, rp.Attempt)))
	if err := g.WriteFile(rel, data); err != nil {
		return repair.Packet{}, err
	}
	inc.Diagnoses = append(inc.Diagnoses, diag)
	inc = repair.BeginAttempt(inc, e.opts.Now())
	_ = saveIncident(g, inc)
	st.Runtime.IncidentID = inc.ID
	st.Runtime.Attempts = rp.Attempt
	st.CurrentState = state.StateRepairing
	_ = g.Save(st)
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindIntent,
		Name:    state.EventRepairStarted,
		Payload: state.Payload(map[string]any{"incident": inc.ID, "attempt": rp.Attempt, "kind": "runtime"}),
	})
	return rp, nil
}

func runtimeHumanAction(st state.State, rep apprun.Report, inc repair.Incident) string {
	return fmt.Sprintf(
		"Local runtime failed after %d repair attempts.\nStartup: %s\nFailure: %s\nLogs: %s\nRemaining blocker: application did not become ready.\nHuman action: inspect the command, fix the environment or product, then prdpr runtime / prdpr resume.",
		len(inc.Attempts), st.Runtime.Command, firstNonEmpty(rep.Error, st.Runtime.LastError), clipLog(rep.Stderr+"\n"+rep.Stdout),
	)
}
