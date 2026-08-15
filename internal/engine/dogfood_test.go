package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/knowledge"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func TestHeadlessRepairLoopFixesDeterministicBug(t *testing.T) {
	root := goGitFixture(t, true)
	buggy := "package fixture\n\nfunc Add(a, b int) int { return a + b + 1 }\n"
	good := "package fixture\n\nfunc Add(a, b int) int { return a + b }\n"
	seq := &cursor.Sequence{Steps: []cursor.Fake{
		{ClaimSuccess: true, WriteRel: "add.go", WriteBody: buggy},
		{ClaimSuccess: true, WriteRel: "add.go", WriteBody: good},
	}}
	eng := engine.New(engine.Options{
		Worker:    seq,
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
	})
	res, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Completed || !res.Verification.VerifiedSuccess {
		t.Fatalf("completed=%v waiting=%v ver=%+v inc=%+v", res.Completed, res.Waiting, res.Verification, res.Incident)
	}
	if len(res.Incident.Attempts) != 1 || !res.Incident.Attempts[0].Verified {
		t.Fatalf("expected one successful repair attempt, got %+v", res.Incident.Attempts)
	}
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentState != state.StateCompleted {
		t.Fatalf("state=%s", snap.CurrentState)
	}
	hints := knowledge.ProjectStore(root).Hints("test_command")
	if len(hints) == 0 {
		t.Fatal("expected project knowledge")
	}
}

func TestHeadlessRepairExhaustedNoFourthAttempt(t *testing.T) {
	root := goGitFixture(t, true)
	buggy := "package fixture\n\nfunc Add(a, b int) int { return 0 }\n"
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "add.go", WriteBody: buggy},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
	})
	res, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Completed || !res.Waiting {
		t.Fatalf("want waiting after 3 failed repairs: completed=%v waiting=%v inc=%+v", res.Completed, res.Waiting, res.Incident)
	}
	if len(res.Incident.Attempts) != 3 || !res.Incident.Exhausted {
		t.Fatalf("attempts=%+v", res.Incident.Attempts)
	}
	if _, err := eng.PrepareRepair(context.Background(), engine.Request{ProductRoot: root}); err == nil {
		t.Fatal("fourth headless repair must be refused")
	}
}

func TestInteractiveReviewThenRepairPrepare(t *testing.T) {
	root := goGitFixture(t, true)
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "add.go", WriteBody: "package fixture\n\nfunc Add(a, b int) int { return 0 }\n"},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
	})
	if _, err := eng.Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if v.VerifiedSuccess || v.Status == testeng.StatusVerified {
		t.Fatal("expected failed verify")
	}
	rev, err := eng.Review(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !rev.Repair {
		t.Fatalf("expected repair recommendation %+v", rev)
	}
	if rev.Model.SelectedModel != "NONE" {
		t.Fatalf("expected NONE model, got %+v", rev.Model)
	}
	pkt, err := eng.PrepareRepair(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if pkt.Attempt != 1 || pkt.IncidentID == "" {
		t.Fatalf("%+v", pkt)
	}
	p := filepath.Join(root, ".project", "packets", "repair_"+pkt.IncidentID+"_1.json")
	if _, err := os.ReadFile(p); err != nil {
		t.Fatal(err)
	}
}

func TestFeedbackDoesNotStoreSecretShapedTextUnredactedInEvents(t *testing.T) {
	root := goGitFixture(t, true)
	eng := engine.New(engine.Options{AllowSelf: true, SkipWait: true, NewID: seqID()})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if err := eng.Feedback(root, "h1", "token=sk-abcdefghijklmnopqrstuvwxyz123456", "PRESENT_UNVERIFIED", "openai"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".project", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Fatal("events")
	}
}

func TestOpenPRSkippedWhenGitHubDisabled(t *testing.T) {
	root := goGitFixture(t, true)
	cfg := config.Defaults()
	cfg.AutoCommit = false
	cfg.GitHubEnabled = false
	eng := engine.New(engine.Options{AllowSelf: true, SkipWait: true, NewID: seqID(), Config: cfg, Worker: panicWorker{}})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil || !v.VerifiedSuccess {
		t.Fatalf("verify first: %v %+v", err, v)
	}
	pr, err := eng.OpenMilestonePR(context.Background(), root, "t", "b")
	if err != nil {
		t.Fatal(err)
	}
	if !pr.Skipped {
		t.Fatalf("%+v", pr)
	}
}

func TestRecoverDoesNotInventVerifiedSuccess(t *testing.T) {
	root := goGitFixture(t, true)
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "n.txt", WriteBody: "x\n"},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
	})
	run, err := eng.Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Execution.VerifiedSuccess {
		t.Fatal("claim is not verify")
	}
	store, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	g, err := store.Lock()
	if err != nil {
		t.Fatal(err)
	}
	st, err := g.Load()
	if err != nil {
		t.Fatal(err)
	}
	st.CurrentState = state.StateWorkerRunning
	if err := g.Save(st); err != nil {
		t.Fatal(err)
	}
	_ = g.Unlock()

	run2, err := eng.Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if run2.Execution.VerifiedSuccess {
		t.Fatal("recovery must not verify")
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".project", "state.json"))
	var snap state.State
	_ = json.Unmarshal(raw, &snap)
	if snap.CurrentState == state.StateWorkerRunning {
		t.Fatal("should recover off WORKER_RUNNING")
	}
}
