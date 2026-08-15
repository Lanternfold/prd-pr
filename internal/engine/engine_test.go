package engine_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/prd"
)

func TestFakeWorkerWritesExecutionAndNeverVerifies(t *testing.T) {
	root := fixtureRepo(t)
	eng := engine.New(engine.Options{
		Worker: cursor.Fake{ClaimSuccess: true, WriteRel: "added.txt", WriteBody: "from-fake\n"},
		NewID:  seqID(),
	})
	res, err := eng.Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Execution.Invoked {
		t.Fatalf("refused: %s", res.Execution.RefusalReason)
	}
	if !res.Execution.WorkerClaimedSuccess || res.Execution.VerifiedSuccess {
		t.Fatalf("%+v", res.Execution)
	}
	if len(res.Execution.ChangedPaths) != 1 || res.Execution.ChangedPaths[0] != "added.txt" {
		t.Fatalf("changed=%v", res.Execution.ChangedPaths)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".project", "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ex engine.Execution
	if err := json.Unmarshal(raw, &ex); err != nil {
		t.Fatal(err)
	}
	if ex.VerifiedSuccess || ex.Baseline.SHA == "" {
		t.Fatalf("%+v", ex)
	}
	if _, err := os.Stat(filepath.Join(root, res.Execution.PacketRef)); err != nil {
		t.Fatal(err)
	}
}

func TestRefuseWithoutGitBaseline(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	root := t.TempDir()
	writePRD(t, root)
	eng := engine.New(engine.Options{
		Worker: cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "ok\n"},
		NewID:  seqID(),
	})
	res, err := eng.Run(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Execution.Invoked {
		t.Fatalf("expected bootstrap to create a baseline, refused: %s", res.Execution.RefusalReason)
	}
	if res.Execution.Baseline.SHA == "" {
		t.Fatal("missing baseline after bootstrap")
	}
}

func TestRefuseDirtyTree(t *testing.T) {
	root := fixtureRepo(t)
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("d\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := engine.New(engine.Options{
		Worker: cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "nope\n"},
		NewID:  seqID(),
	}).Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: prd.PhaseID("P1")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked {
		t.Fatal("must not invoke on dirty tree")
	}
}

func TestRefuseOrchestratorRepo(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	res, err := engine.New(engine.Options{
		Worker: cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "nope\n"},
	}).Run(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked {
		t.Fatal("must not invoke against orchestrator repo")
	}
	if !strings.Contains(res.Execution.RefusalReason, "orchestrator") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestRefuseMissingCursorBinary(t *testing.T) {
	root := fixtureRepo(t)
	res, err := engine.New(engine.Options{
		Worker: &cursor.CLI{LookPath: func(string) (string, error) { return "", os.ErrNotExist }},
		NewID:  seqID(),
	}).Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked {
		t.Fatal("must not invoke when agent CLI is missing")
	}
}

func fixtureRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writePRD(t, root)
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	run(t, root, "git", "add", "PRD.md")
	run(t, root, "git", "commit", "-m", "init")
	return root
}

func writePRD(t *testing.T, root string) {
	t.Helper()
	src := filepath.Join("..", "prd", "testdata", "prd", "minimal_valid.md")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func seqID() func() string {
	n := 0
	return func() string {
		n++
		return "id" + itoa(n)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
