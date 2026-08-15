package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/notify"
	"github.com/lanternfold/prd-pr/internal/redact"
	"github.com/lanternfold/prd-pr/internal/state"
)

func (e *Engine) RequestHuman(ctx context.Context, root string, req human.Request) (human.Request, error) {
	root, _, err := identifyRoot(root)
	if err != nil {
		return req, err
	}
	store, err := state.Open(root)
	if err != nil {
		return req, err
	}
	g, err := store.Lock()
	if err != nil {
		return req, err
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return req, err
	}
	return e.requestHumanLocked(g, st, root, req)
}

func (e *Engine) requestHumanLocked(g *state.Guard, st state.State, root string, req human.Request) (human.Request, error) {
	if req.ID == "" {
		req.ID = "h_" + e.opts.NewID()
	}
	if req.Phase == "" {
		req.Phase = st.CurrentPhaseID
	}
	if req.Task == "" {
		req.Task = st.CurrentRunID
	}
	if err := human.WriteRequest(root, req); err != nil {
		return req, err
	}
	st.CurrentState = state.StateWaitingForHuman
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindIntent,
		Name:    state.EventHumanRequested,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]string{"id": req.ID, "kind": req.Kind, "reason": req.Reason}),
	})
	_ = g.Save(st)
	_ = e.opts.Bell.Ring("PRD→PR needs you", redact.String(req.Needed))
	if e.opts.SkipWait {
		return req, nil
	}
	timeout := e.cfg().HumanTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ok := notify.WaitUntil(timeout, func() bool {
		_, err := human.LoadResponse(root)
		return err == nil
	})
	if ok {
		return req, nil
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventHumanTimeout,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]any{"id": req.ID, "optional": req.Optional}),
	})
	if req.Optional {
		st.CurrentState = state.StateVerified
		_ = g.Save(st)
	}
	return req, nil
}

func (e *Engine) Feedback(root, requestID, text, status, credName string) error {
	root, _, err := identifyRoot(root)
	if err != nil {
		return err
	}
	text = redact.String(text)
	res := human.Response{RequestID: requestID, Text: text, Status: status}
	if err := human.WriteResponse(root, res); err != nil {
		return err
	}
	if credName != "" {
		st := human.CredPresentUnverified
		if status == human.CredPresentVerified {
			st = human.CredPresentVerified
		}
		if err := human.RecordCredential(root, credName, st); err != nil {
			return err
		}
	}
	store, err := state.Open(root)
	if err != nil {
		return err
	}
	return store.AppendEvent(state.Event{
		Kind: state.KindResult,
		Name: state.EventHumanResolved,
		Payload: state.Payload(map[string]string{
			"request_id": requestID,
			"status":     status,
			"text":       text,
		}),
	})
}

func (e *Engine) Resume(ctx context.Context, root string) error {
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
	unlocked := false
	defer func() {
		if !unlocked {
			_ = g.Unlock()
		}
	}()
	st, err := g.Load()
	if err != nil {
		return err
	}
	if st.CurrentState != state.StateWaitingForHuman {
		return fmt.Errorf("not waiting for human (state %s)", st.CurrentState)
	}
	if _, err := human.LoadResponse(root); err != nil {
		return fmt.Errorf("no human response yet")
	}
	inc := loadIncident(root)
	baseline := ""
	if ex, err := loadExecution(root); err == nil {
		baseline = ex.Baseline.SHA
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventHumanResolved,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]any{
			"resume":           true,
			"incident":         inc.ID,
			"attempts":         len(inc.Attempts),
			"baseline":         baseline,
			"manual_confirmed": human.ManualConfirmed(root),
		}),
	})
	confirmed := human.ManualConfirmed(root)
	st.CurrentState = state.StateVerificationFailed
	if err := g.Save(st); err != nil {
		return err
	}
	if err := g.Unlock(); err != nil {
		return err
	}
	unlocked = true
	if confirmed {
		_, err := e.Verify(ctx, Request{ProductRoot: root})
		return err
	}
	return nil
}

func (e *Engine) NextCredential(root string, names []string) (human.Request, error) {
	n := human.NextMissingCredential(names, root)
	if n == "" {
		return human.Request{}, nil
	}
	return e.RequestHuman(context.Background(), root, human.Request{
		Kind:       human.KindCredential,
		Reason:     "missing_credential",
		Needed:     "Provide " + n + " (metadata only; do not paste the secret into logs)",
		Credential: n,
		Urgency:    human.UrgencyHigh,
	})
}
