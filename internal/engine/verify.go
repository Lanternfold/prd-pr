package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/repair"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

const (
	verificationFile = ".project/verification.json"
	verificationLogs = ".project/verification"
)

// Verify runs independent P7 verification. It never trusts worker_claimed_success.
func (e *Engine) Verify(ctx context.Context, req Request) (testeng.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, jail, err := identifyRoot(req.ProductRoot)
	if err != nil {
		return testeng.Result{SchemaVersion: testeng.SchemaVersion, Status: testeng.StatusInfrastructure, Reason: err.Error()}, nil
	}

	store, err := state.Open(root)
	if err != nil {
		return testeng.Result{SchemaVersion: testeng.SchemaVersion, Status: testeng.StatusInfrastructure, Reason: err.Error()}, nil
	}
	g, err := store.Lock()
	if err != nil {
		return testeng.Result{}, err
	}
	defer func() { _ = g.Unlock() }()

	st, err := g.Load()
	if err != nil {
		return testeng.Result{
			SchemaVersion: testeng.SchemaVersion,
			Status:        testeng.StatusIncomplete,
			Reason:        "no prepared execution: " + err.Error(),
		}, nil
	}

	ex, err := loadExecution(root)
	if err != nil {
		return e.persistVerify(ctx, g, st, testeng.Result{
			SchemaVersion: testeng.SchemaVersion,
			Status:        testeng.StatusIncomplete,
			Reason:        "missing execution.json: " + err.Error(),
			RunID:         st.CurrentRunID,
			PhaseID:       st.CurrentPhaseID,
		}, root)
	}

	pkt, err := loadPacket(root, ex.PacketRef)
	if err != nil {
		return e.persistVerify(ctx, g, st, testeng.Result{
			SchemaVersion: testeng.SchemaVersion,
			Status:        testeng.StatusIncomplete,
			Reason:        "cannot load task packet: " + err.Error(),
			RunID:         ex.RunID,
			TaskID:        ex.TaskID,
			PhaseID:       ex.PhaseID,
			BaselineSHA:   ex.Baseline.SHA,
		}, root)
	}

	_ = g.AppendEvent(state.Event{
		Kind:    state.KindIntent,
		Name:    state.EventVerificationStarted,
		RunID:   ex.RunID,
		PhaseID: ex.PhaseID,
		Payload: state.Payload(map[string]string{"task_id": ex.TaskID, "packet": ex.PacketRef}),
	})

	st.CurrentState = state.StateVerifying
	_ = g.Save(st)

	te := testeng.New(testeng.Options{
		Git:      e.opts.Git,
		Runner:   e.opts.ProcRunner,
		LookPath: e.opts.LookPath,
		Now:      e.opts.Now,
		Timeout:  e.opts.TestTimeout,
	})
	vres := te.Run(ctx, testeng.Request{
		ProductRoot:     root,
		Packet:          pkt,
		RunID:           ex.RunID,
		TaskID:          ex.TaskID,
		PhaseID:         ex.PhaseID,
		BaselineSHA:     ex.Baseline.SHA,
		Jail:            jail,
		ManualConfirmed: human.ManualConfirmed(root),
	})

	for i := range vres.Checks {
		c := &vres.Checks[i]
		if c.Stdout != "" || c.Stderr != "" {
			base := filepath.Join(verificationLogs, ex.TaskID+"-"+c.ID)
			if c.Stdout != "" {
				rel := filepath.ToSlash(base + ".stdout.txt")
				_ = g.WriteFile(rel, []byte(c.Stdout))
				c.StdoutRef = rel
				c.Stdout = ""
			}
			if c.Stderr != "" {
				rel := filepath.ToSlash(base + ".stderr.txt")
				_ = g.WriteFile(rel, []byte(c.Stderr))
				c.StderrRef = rel
				c.Stderr = ""
			}
		}
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventVerificationTestCompleted,
			RunID:   ex.RunID,
			PhaseID: ex.PhaseID,
			Payload: state.Payload(map[string]any{
				"check_id":  c.ID,
				"outcome":   c.Outcome,
				"exit_code": c.ExitCode,
			}),
		})
	}

	return e.persistVerify(ctx, g, st, vres, root)
}

func (e *Engine) persistVerify(ctx context.Context, g *state.Guard, st state.State, vres testeng.Result, root string) (testeng.Result, error) {
	data, err := json.MarshalIndent(vres, "", "  ")
	if err != nil {
		return testeng.Result{}, err
	}
	if err := g.WriteFile(verificationFile, append(data, '\n')); err != nil {
		return testeng.Result{}, err
	}

	ex, _ := loadExecutionFromGuard(g)
	if ex.RunID != "" || ex.TaskID != "" {
		ex.VerifiedSuccess = vres.VerifiedSuccess
		ex.ChangedPaths = vres.ChangedFiles
		ex.RecordedAt = e.opts.Now().UTC().Format(time.RFC3339Nano)
		if vres.Timestamp != "" {
			ex.RecordedAt = vres.Timestamp
		}
		applySelfDevVerification(&ex, &st, vres)
		_ = writeExecution(g, ex)
		if ex.RepairAttempt > 0 {
			inc := loadIncident(root)
			inc = repair.FinishAttempt(inc, ex.RepairAttempt, vres.HeadSHA, vres.ChangedFiles, vres.VerifiedSuccess, vres.Reason, e.opts.Now())
			_ = saveIncident(g, inc)
		}
	}

	evName := state.EventVerificationFailed
	st.CurrentState = state.StateVerificationFailed
	switch vres.Status {
	case testeng.StatusVerified:
		evName = state.EventVerificationPassed
		st.CurrentState = state.StateVerified
	case testeng.StatusManual:
		st.CurrentState = state.StateWaitingForHuman
	case testeng.StatusIncomplete, testeng.StatusUnsupported, testeng.StatusInfrastructure:
		st.CurrentState = state.StateVerificationIncomplete
	}
	if vres.RunID != "" {
		st.CurrentRunID = vres.RunID
	}
	if vres.PhaseID != "" {
		st.CurrentPhaseID = vres.PhaseID
	}
	if vres.HeadSHA != "" {
		st.CurrentCommit = vres.HeadSHA
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    evName,
		RunID:   vres.RunID,
		PhaseID: vres.PhaseID,
		Payload: state.Payload(map[string]any{
			"status":           vres.Status,
			"verified_success": vres.VerifiedSuccess,
			"tests_pass":       vres.TestsPass,
			"manual":           vres.ManualVerificationRequired,
		}),
	})
	if vres.Status == testeng.StatusVerified {
		st.LastKnownGoodCommit = vres.HeadSHA
		if err := e.completeLocked(ctx, g, st, root); err != nil {
			return vres, err
		}
		return vres, nil
	}
	if vres.Status == testeng.StatusManual {
		_, _ = e.requestHumanLocked(g, st, root, human.Request{
			Kind:      human.KindManualAC,
			Reason:    "manual_acceptance",
			Phase:     vres.PhaseID,
			Task:      vres.TaskID,
			Attempted: vres.Reason,
			Needed:    "Confirm the required manual acceptance criteria. Do not paste secrets.",
			Urgency:   human.UrgencyNormal,
		})
		return vres, nil
	}
	if err := g.Save(st); err != nil {
		return vres, err
	}
	return vres, nil
}

func loadExecution(root string) (Execution, error) {
	raw, err := os.ReadFile(filepath.Join(root, executionFile))
	if err != nil {
		return Execution{}, err
	}
	var ex Execution
	if err := json.Unmarshal(raw, &ex); err != nil {
		return Execution{}, err
	}
	return ex, nil
}

func loadExecutionFromGuard(g *state.Guard) (Execution, error) {
	raw, err := os.ReadFile(filepath.Join(g.Jail().Root(), executionFile))
	if err != nil {
		return Execution{}, err
	}
	var ex Execution
	if err := json.Unmarshal(raw, &ex); err != nil {
		return Execution{}, err
	}
	return ex, nil
}

func loadPacket(root, rel string) (packet.Packet, error) {
	if rel == "" {
		return packet.Packet{}, os.ErrNotExist
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return packet.Packet{}, err
	}
	return packet.Unmarshal(raw)
}
