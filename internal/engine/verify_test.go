package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func TestVerifySetsVerifiedSuccess(t *testing.T) {
	root := goGitFixture(t, true)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true})
	prep, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || prep.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, prep.Execution)
	}
	if prep.Execution.VerifiedSuccess {
		t.Fatal("prepare must not set verified_success")
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !vres.VerifiedSuccess || vres.Status != testeng.StatusVerified {
		t.Fatalf("%+v", vres)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".project", "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ex engine.Execution
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatal(err)
	}
	if !ex.VerifiedSuccess {
		t.Fatal("execution.json verified_success")
	}
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentState != state.StateVerified && snap.CurrentState != state.StateCompleted {
		t.Fatalf("state=%s", snap.CurrentState)
	}
	if vres.BaselineSHA == "" || vres.HeadSHA == "" {
		t.Fatalf("missing SHAs %+v", vres)
	}
}

func TestWorkerClaimDoesNotVerify(t *testing.T) {
	root := goGitFixture(t, false)
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "WORKER_FAKE.txt", WriteBody: "claimed\n"},
		NewID:     seqID(),
		AllowSelf: true,
	})
	run, err := eng.Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if !run.Execution.WorkerClaimedSuccess {
		t.Fatal("expected claim")
	}
	if run.Execution.VerifiedSuccess {
		t.Fatal("run must not set verified_success")
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if vres.VerifiedSuccess || vres.Status == testeng.StatusVerified {
		t.Fatalf("claim must not verify: %+v", vres)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".project", "execution.json"))
	var ex engine.Execution
	_ = json.Unmarshal(raw, &ex)
	if ex.VerifiedSuccess {
		t.Fatal("verified_success leaked from claim")
	}
	if !ex.WorkerClaimedSuccess {
		t.Fatal("claimed flag should remain")
	}
}

func TestVerifyChangedAndUntracked(t *testing.T) {
	root := goGitFixture(t, true)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra.txt"), []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range vres.UntrackedFiles {
		if p == "extra.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("untracked=%v changed=%v", vres.UntrackedFiles, vres.ChangedFiles)
	}
}

func TestVerifyIncompleteWithoutPrepare(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := engine.New(engine.Options{AllowSelf: true}).Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != testeng.StatusIncomplete {
		t.Fatalf("status=%s", res.Status)
	}
}

func goGitFixture(t *testing.T, pass bool) string {
	return goGitFixturePRD(t, pass, "auto_verify.md")
}

func TestManualAcceptanceNotVerifiedUntilConfirmed(t *testing.T) {
	root := goGitFixturePRD(t, true, "minimal_valid.md")
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, SkipWait: true})
	if prep, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil || prep.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, prep.Execution)
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if vres.VerifiedSuccess || vres.Status != testeng.StatusManual {
		t.Fatalf("A: want manual not verified, got %+v", vres)
	}
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentState != state.StateWaitingForHuman {
		t.Fatalf("A: state=%s", snap.CurrentState)
	}

	req, err := human.LoadRequest(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.Feedback(root, req.ID, "looks good", human.StatusConfirmed, ""); err != nil {
		t.Fatal(err)
	}
	if err := eng.Resume(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	vres, err = eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !vres.VerifiedSuccess || vres.Status != testeng.StatusVerified {
		t.Fatalf("B: want verified after confirm, got %+v", vres)
	}
}

func TestManualConfirmDoesNotBypassFailedEngineVerify(t *testing.T) {
	root := goGitFixturePRD(t, false, "minimal_valid.md")
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, SkipWait: true})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if vres.VerifiedSuccess || vres.Status == testeng.StatusVerified {
		t.Fatalf("C: tests failed must not verify: %+v", vres)
	}
	if err := eng.Feedback(root, "h_force", "confirm anyway", human.StatusConfirmed, ""); err != nil {
		t.Fatal(err)
	}
	vres, err = eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if vres.VerifiedSuccess || vres.Status != testeng.StatusFailed {
		t.Fatalf("C: confirm must not bypass failed tests: %+v", vres)
	}
}

func goGitFixturePRD(t *testing.T, pass bool, prdName string) string {
	t.Helper()
	root := t.TempDir()
	writeString(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.22\n")
	writeString(t, filepath.Join(root, "add.go"), "package fixture\n\nfunc Add(a, b int) int { return a + b }\n")
	want := "4"
	if !pass {
		want = "5"
	}
	writeString(t, filepath.Join(root, "add_test.go"), "package fixture\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 2) != "+want+" { t.Fatal(Add(2,2)) }\n}\n")
	src := filepath.Join("..", "prd", "testdata", "prd", prdName)
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	writeString(t, filepath.Join(root, "PRD.md"), string(raw))
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-m", "init")
	return root
}
