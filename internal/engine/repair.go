package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/diagnose"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/repair"
	"github.com/lanternfold/prd-pr/internal/review"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func (e *Engine) PrepareRepair(ctx context.Context, req Request) (repair.Packet, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, _, err := identifyRoot(req.ProductRoot)
	if err != nil {
		return repair.Packet{}, err
	}
	store, err := state.Open(root)
	if err != nil {
		return repair.Packet{}, err
	}
	g, err := store.Lock()
	if err != nil {
		return repair.Packet{}, err
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return repair.Packet{}, err
	}
	inc := loadIncident(root)
	if inc.ID != "" && !repair.CanAttempt(inc, e.cfg()) {
		inc.Exhausted = true
		inc.HumanAction = "Autonomous repair exhausted after 3 attempts."
		_ = saveIncident(g, inc)
		_, _ = e.requestHumanLocked(g, st, root, human.Request{
			Kind:      human.KindRepairFail,
			Reason:    "repeated_repair_failure",
			Phase:     st.CurrentPhaseID,
			Task:      st.CurrentRunID,
			Attempted: fmt.Sprintf("%d repair attempts", len(inc.Attempts)),
			Needed:    inc.HumanAction,
			Urgency:   human.UrgencyHigh,
		})
		return repair.Packet{}, fmt.Errorf("repair exhausted: attempt %d of %d already recorded", len(inc.Attempts), repair.Max(e.cfg()))
	}
	rev, err := e.reviewLocked(ctx, g, st, root)
	if err != nil {
		return repair.Packet{}, err
	}
	st, _ = g.Load()
	pkt, err := e.prepareRepairLocked(g, st, root, rev)
	return pkt, err
}

func (e *Engine) prepareRepairLocked(g *state.Guard, st state.State, root string, rev review.Result) (repair.Packet, error) {
	if !rev.Repair {
		return repair.Packet{}, fmt.Errorf("repair is not recommended")
	}
	ex, err := loadExecution(root)
	if err != nil {
		return repair.Packet{}, err
	}
	orig, err := loadPacket(root, ex.PacketRef)
	if err != nil {
		return repair.Packet{}, err
	}
	v := loadVerification(root)
	inc := loadIncident(root)
	if inc.ID == "" {
		inc = repair.Incident{
			SchemaVersion: repair.SchemaVersion,
			ID:            "inc_" + e.opts.NewID(),
			PhaseID:       ex.PhaseID,
			TaskID:        ex.TaskID,
			RunID:         ex.RunID,
			MaxAttempts:   repair.Max(e.cfg()),
		}
	}
	if !repair.CanAttempt(inc, e.cfg()) {
		inc.Exhausted = true
		inc.HumanAction = "Autonomous repair exhausted after 3 attempts."
		_ = saveIncident(g, inc)
		_, _ = e.requestHumanLocked(g, st, root, human.Request{
			Kind:      human.KindRepairFail,
			Reason:    "repeated_repair_failure",
			Phase:     ex.PhaseID,
			Task:      ex.TaskID,
			Attempted: fmt.Sprintf("%d repair attempts", len(inc.Attempts)),
			Needed:    inc.HumanAction,
			Urgency:   human.UrgencyHigh,
		})
		return repair.Packet{}, fmt.Errorf("repair exhausted")
	}
	if rev.Diagnosis.Origin == diagnose.OriginUpstreamPhase {
		if gr := loadGraph(root); gr != nil {
			if plan, err := repair.RewindTarget(gr, ex.PhaseID, rev.Diagnosis.Origin); err == nil {
				applyRewind(gr, plan)
				if raw, err := gr.Marshal(); err == nil {
					_ = g.WriteFile(graphFile, append(raw, '\n'))
				}
			}
		}
	}
	rp := repair.NewPacket(inc, orig, rev, v, joinDiff(v.ChangedFiles))
	rp.ProductRoot = root
	data, err := repair.Marshal(rp)
	if err != nil {
		return repair.Packet{}, err
	}
	rel := filepath.ToSlash(filepath.Join(packetsDir, fmt.Sprintf("repair_%s_%d.json", inc.ID, rp.Attempt)))
	if err := g.WriteFile(rel, data); err != nil {
		return repair.Packet{}, err
	}
	inc.Diagnoses = append(inc.Diagnoses, rev.Diagnosis)
	inc = repair.BeginAttempt(inc, e.opts.Now())
	_ = saveIncident(g, inc)
	ex.IncidentID = inc.ID
	ex.RepairAttempt = rp.Attempt
	_ = writeExecution(g, ex)
	st.CurrentState = state.StateRepairing
	_ = g.Save(st)
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindIntent,
		Name:    state.EventRepairStarted,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]any{"incident": inc.ID, "attempt": rp.Attempt, "packet": rel}),
	})
	return rp, nil
}

func (e *Engine) applyRepairWorker(ctx context.Context, root string, rp repair.Packet) cursor.Result {
	rel := filepath.ToSlash(filepath.Join(packetsDir, fmt.Sprintf("repair_%s_%d.json", rp.IncidentID, rp.Attempt)))
	task := repair.AsTaskPacket(rp)
	w := e.repairWorker()
	return w.Run(ctx, cursor.Request{
		ProductRoot: root,
		Packet:      task,
		PacketRel:   rel,
		Timeout:     e.opts.Timeout,
	})
}

func saveIncident(g *state.Guard, inc repair.Incident) error {
	raw, err := json.MarshalIndent(inc, "", "  ")
	if err != nil {
		return err
	}
	return g.WriteFile(incidentFile, append(raw, '\n'))
}

func (e *Engine) recordRepairAttempt(g *state.Guard, root string, rp repair.Packet, v testeng.Result, wres cursor.Result) repair.Incident {
	inc := loadIncident(root)
	changed := v.ChangedFiles
	inc = repair.FinishAttempt(inc, rp.Attempt, v.HeadSHA, changed, v.VerifiedSuccess, v.Reason, e.opts.Now())
	if wres.TimedOut && len(inc.Attempts) > 0 {
		inc.Attempts[len(inc.Attempts)-1].Summary = "repair worker timed out"
	}
	_ = saveIncident(g, inc)
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventRepairAttempt,
		RunID:   inc.RunID,
		PhaseID: inc.PhaseID,
		Payload: state.Payload(map[string]any{
			"attempt":  len(inc.Attempts),
			"verified": v.VerifiedSuccess,
			"claimed":  wres.WorkerClaimedSuccess,
		}),
	})
	return inc
}
