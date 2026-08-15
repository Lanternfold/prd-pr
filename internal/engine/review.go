package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/lanternfold/prd-pr/internal/cost"
	"github.com/lanternfold/prd-pr/internal/llm"
	"github.com/lanternfold/prd-pr/internal/repair"
	"github.com/lanternfold/prd-pr/internal/review"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func (e *Engine) Review(ctx context.Context, req Request) (review.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, _, err := identifyRoot(req.ProductRoot)
	if err != nil {
		return review.Result{}, err
	}
	store, err := state.Open(root)
	if err != nil {
		return review.Result{}, err
	}
	g, err := store.Lock()
	if err != nil {
		return review.Result{}, err
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return review.Result{}, err
	}
	return e.reviewLocked(ctx, g, st, root)
}

func (e *Engine) reviewLocked(ctx context.Context, g *state.Guard, st state.State, root string) (review.Result, error) {
	st.CurrentState = state.StateReviewing
	_ = g.Save(st)
	_ = g.AppendEvent(state.Event{Kind: state.KindIntent, Name: state.EventReviewStarted, RunID: st.CurrentRunID, PhaseID: st.CurrentPhaseID})

	v := loadVerification(root)
	ex, _ := loadExecution(root)
	pkt, _ := loadPacket(root, ex.PacketRef)
	inc := loadIncident(root)
	in := review.Input{
		Verification:     v,
		Packet:           pkt,
		DiffSummary:      joinDiff(v.ChangedFiles),
		PreviousAttempts: len(inc.Attempts),
	}
	res := review.Run(ctx, in, review.Options{
		Adapter:  e.opts.LLM,
		Config:   e.cfg(),
		SpentUSD: cost.SpentUSD(root),
	})
	raw, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return review.Result{}, err
	}
	if err := g.WriteFile(reviewFile, append(raw, '\n')); err != nil {
		return review.Result{}, err
	}
	_ = cost.Append(root, cost.Line{
		Provider:     "router",
		Model:        res.Model.SelectedModel,
		Purpose:      llm.RoleReview,
		InputTokens:  res.Model.InputTokens,
		OutputTokens: res.Model.OutputTokens,
		EstimatedUSD: res.Model.EstimatedCost,
		RunID:        st.CurrentRunID,
		PhaseID:      st.CurrentPhaseID,
	})
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventCostRecorded,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]any{"model": res.Model.SelectedModel, "usd": res.Model.EstimatedCost}),
	})
	name := state.EventReviewCompleted
	if res.ReviewFailure != "" {
		name = state.EventReviewFailed
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    name,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]any{
			"classification": res.Diagnosis.Classification,
			"repair":         res.Repair,
			"human":          res.Human,
			"model":          res.Model.SelectedModel,
		}),
	})
	if res.Human && !res.Repair {
		st.CurrentState = state.StateWaitingForHuman
	} else if v.Status == testeng.StatusVerified {
		st.CurrentState = state.StateVerified
	} else {
		st.CurrentState = state.StateVerificationFailed
	}
	_ = g.Save(st)
	return res, nil
}

func loadVerification(root string) testeng.Result {
	raw, err := os.ReadFile(filepath.Join(root, verificationFile))
	if err != nil {
		return testeng.Result{Status: testeng.StatusIncomplete, Reason: "missing verification.json"}
	}
	var v testeng.Result
	_ = json.Unmarshal(raw, &v)
	return v
}

func loadIncident(root string) repair.Incident {
	raw, err := os.ReadFile(filepath.Join(root, incidentFile))
	if err != nil {
		return repair.Incident{SchemaVersion: repair.SchemaVersion}
	}
	var inc repair.Incident
	_ = json.Unmarshal(raw, &inc)
	return inc
}

func joinDiff(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	b, _ := json.Marshal(paths)
	return string(b)
}
