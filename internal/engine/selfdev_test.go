package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func TestOrdinaryExecutionAgainstOrchestratorRemainsRefused(t *testing.T) {
	root := orchestratorFixture(t, true, true)
	rec := &recordingWorker{inner: cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "nope\n"}}
	res, err := engine.New(engine.Options{Worker: rec, NewID: seqID()}).Run(context.Background(), engine.Request{
		ProductRoot: root,
		PhaseID:     "P1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked || rec.calls != 0 {
		t.Fatal("ordinary execution must not invoke a coding worker against the orchestrator")
	}
	if !strings.Contains(res.Execution.RefusalReason, "orchestrator") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
	if res.Execution.ExecutionMode == state.ExecutionModeSelfDevelopment {
		t.Fatal("ordinary refusal must not enter SELF_DEVELOPMENT")
	}
}

func TestExplicitSelfDevelopmentAcceptedWhenPreconditionsPass(t *testing.T) {
	root := orchestratorFixture(t, true, true)
	eng := engine.New(engine.Options{
		Worker: cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "ok\n"},
		NewID:  seqID(),
	})
	req := engine.Request{ProductRoot: root, PhaseID: "P1", ExecutionMode: state.ExecutionModeSelfDevelopment}
	prep, err := eng.Prepare(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if prep.Execution.RefusalReason != "" {
		t.Fatalf("prepare refused: %s", prep.Execution.RefusalReason)
	}
	if prep.Execution.Invoked {
		t.Fatal("prepare must not invoke the worker")
	}
	if prep.Execution.ExecutionMode != state.ExecutionModeSelfDevelopment {
		t.Fatalf("mode=%q", prep.Execution.ExecutionMode)
	}
	if !prep.Execution.SelfDevelopment.Authorized {
		t.Fatalf("not authorized: %+v", prep.Execution.SelfDevelopment)
	}
	run, err := eng.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !run.Execution.Invoked {
		t.Fatalf("self-development must use the dedicated path: invoked=%v reason=%s", run.Execution.Invoked, run.Execution.RefusalReason)
	}
	if run.Execution.VerifiedSuccess {
		t.Fatal("worker completion must not constitute successful self-development")
	}
	if run.Execution.SelfDevelopment.Status != state.SelfDevStatusRunning {
		t.Fatalf("status=%s", run.Execution.SelfDevelopment.Status)
	}
}

func TestSelfDevelopmentWithoutExplicitDeclarationIsRefused(t *testing.T) {
	root := orchestratorFixture(t, true, false)
	rec := &recordingWorker{inner: cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "nope\n"}}
	eng := engine.New(engine.Options{Worker: rec, NewID: seqID()})

	res, err := eng.Run(context.Background(), engine.Request{
		ProductRoot:   root,
		PhaseID:       "P1",
		ExecutionMode: state.ExecutionModeSelfDevelopment,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked || rec.calls != 0 {
		t.Fatal("missing PRD declaration must refuse")
	}
	if !strings.Contains(res.Execution.RefusalReason, "Execution mode") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}

	root2 := orchestratorFixture(t, true, true)
	res, err = eng.Run(context.Background(), engine.Request{ProductRoot: root2, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked {
		t.Fatal("PRD declaration without request mode must not enter self-development")
	}
}

func TestSelfDevelopmentAgainstNonOrchestratorIsRefused(t *testing.T) {
	root := fixtureRepo(t)
	writeSelfDevPRD(t, root, true)
	run(t, root, "git", "add", "PRD.md")
	run(t, root, "git", "commit", "-m", "prd")
	rec := &recordingWorker{inner: cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "nope\n"}}
	res, err := engine.New(engine.Options{Worker: rec, NewID: seqID()}).Run(context.Background(), engine.Request{
		ProductRoot:   root,
		PhaseID:       "P1",
		ExecutionMode: state.ExecutionModeSelfDevelopment,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked || rec.calls != 0 {
		t.Fatal("non-orchestrator self-development must be refused")
	}
	if !strings.Contains(res.Execution.RefusalReason, "not the PRD→PR orchestrator") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestSelfDevelopmentStatePersists(t *testing.T) {
	root := orchestratorFixture(t, true, true)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()})
	res, err := eng.Prepare(context.Background(), engine.Request{
		ProductRoot:   root,
		PhaseID:       "P1",
		ExecutionMode: state.ExecutionModeSelfDevelopment,
	})
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, res.Execution)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".project", "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ex engine.Execution
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatal(err)
	}
	if ex.ExecutionMode != state.ExecutionModeSelfDevelopment {
		t.Fatalf("execution mode=%q", ex.ExecutionMode)
	}
	if ex.SelfDevelopment.TargetIdentity != "github.com/lanternfold/prd-pr" {
		t.Fatalf("identity=%q", ex.SelfDevelopment.TargetIdentity)
	}
	if !ex.SelfDevelopment.Authorized || ex.SelfDevelopment.Status != state.SelfDevStatusRunning {
		t.Fatalf("%+v", ex.SelfDevelopment)
	}

	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.ExecutionMode != state.ExecutionModeSelfDevelopment {
		t.Fatalf("state mode=%q", snap.ExecutionMode)
	}
	if snap.SelfDevelopment.TargetIdentity != ex.SelfDevelopment.TargetIdentity {
		t.Fatalf("state identity=%q", snap.SelfDevelopment.TargetIdentity)
	}
}

func TestSelfDevelopmentCompletesOnlyAfterVerify(t *testing.T) {
	root := orchestratorFixture(t, true, true)
	eng := engine.New(engine.Options{
		Worker: cursor.Fake{ClaimSuccess: true, WriteRel: "claimed.txt", WriteBody: "claimed\n"},
		NewID:  seqID(),
	})
	runRes, err := eng.Run(context.Background(), engine.Request{
		ProductRoot:   root,
		PhaseID:       "P1",
		ExecutionMode: state.ExecutionModeSelfDevelopment,
	})
	if err != nil {
		t.Fatal(err)
	}
	if runRes.Execution.VerifiedSuccess || runRes.Execution.SelfDevelopment.Status != state.SelfDevStatusRunning {
		t.Fatalf("%+v", runRes.Execution.SelfDevelopment)
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !vres.VerifiedSuccess {
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
	if !ex.VerifiedSuccess || ex.SelfDevelopment.Status != state.SelfDevStatusCompleted {
		t.Fatalf("%+v", ex.SelfDevelopment)
	}
}

func TestSelfDevelopmentCannotSucceedWhenVerificationFails(t *testing.T) {
	root := orchestratorFixture(t, false, true)
	eng := engine.New(engine.Options{
		Worker: cursor.Fake{ClaimSuccess: true, WriteRel: "claimed.txt", WriteBody: "claimed\n"},
		NewID:  seqID(),
	})
	run, err := eng.Run(context.Background(), engine.Request{
		ProductRoot:   root,
		PhaseID:       "P1",
		ExecutionMode: state.ExecutionModeSelfDevelopment,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !run.Execution.Invoked || run.Execution.VerifiedSuccess {
		t.Fatalf("%+v", run.Execution)
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if vres.VerifiedSuccess || vres.Status == testeng.StatusVerified {
		t.Fatalf("verification must fail: %+v", vres)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".project", "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ex engine.Execution
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatal(err)
	}
	if ex.VerifiedSuccess {
		t.Fatal("must not claim verified success")
	}
	if ex.SelfDevelopment.Status != state.SelfDevStatusFailed {
		t.Fatalf("status=%s", ex.SelfDevelopment.Status)
	}
	snap := loadState(t, root)
	if snap.SelfDevelopment.Status != state.SelfDevStatusFailed {
		t.Fatalf("state status=%s", snap.SelfDevelopment.Status)
	}
}

func TestSelfDevelopmentNotInferredFromTitleOrCWD(t *testing.T) {
	root := orchestratorFixture(t, true, false)
	src, err := os.ReadFile(filepath.Join(root, "PRD.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := "# PRD: prd-pr SELF_DEVELOPMENT title only\n\n" + string(src)
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "PRD.md")
	run(t, root, "git", "commit", "-m", "title")
	res, err := engine.New(engine.Options{
		Worker: cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "nope\n"},
		NewID:  seqID(),
	}).Run(context.Background(), engine.Request{
		ProductRoot:   root,
		PhaseID:       "P1",
		ExecutionMode: state.ExecutionModeSelfDevelopment,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked {
		t.Fatal("title mention must not declare self-development")
	}
}

func orchestratorFixture(t *testing.T, testsPass, declare bool) string {
	t.Helper()
	root := t.TempDir()
	writeString(t, filepath.Join(root, "go.mod"), "module github.com/lanternfold/prd-pr\n\ngo 1.22\n")
	writeString(t, filepath.Join(root, "add.go"), "package fixture\n\nfunc Add(a, b int) int { return a + b }\n")
	want := "4"
	if !testsPass {
		want = "5"
	}
	writeString(t, filepath.Join(root, "add_test.go"), "package fixture\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 2) != "+want+" { t.Fatal(Add(2,2)) }\n}\n")
	writeSelfDevPRD(t, root, declare)
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-m", "init")
	return root
}

func writeSelfDevPRD(t *testing.T, root string, declare bool) {
	t.Helper()
	src := filepath.Join("..", "prd", "testdata", "prd", "auto_verify.md")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if declare {
		body = strings.Replace(body, "**Repository:** example/fixture\n", "**Repository:** example/fixture\n\nExecution mode: SELF_DEVELOPMENT\n", 1)
	}
	writeString(t, filepath.Join(root, "PRD.md"), body)
}
