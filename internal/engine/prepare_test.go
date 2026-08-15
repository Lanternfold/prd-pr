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
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/preflight"
	"github.com/lanternfold/prd-pr/internal/state"
)

type recordingWorker struct {
	calls int
	inner cursor.Fake
}

func (r *recordingWorker) Run(ctx context.Context, req cursor.Request) cursor.Result {
	r.calls++
	return r.inner.Run(ctx, req)
}

func TestPrepareValidProject(t *testing.T) {
	root := fixtureRepo(t)
	rec := &recordingWorker{inner: cursor.Fake{ClaimSuccess: true, WriteRel: "mutated.txt", WriteBody: "nope\n"}}
	before := productFiles(t, root)
	eng := engine.New(engine.Options{Worker: rec, NewID: seqID()})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason != "" {
		t.Fatalf("refused: %s", res.Execution.RefusalReason)
	}
	if res.Execution.Invoked || rec.calls != 0 {
		t.Fatalf("worker invoked: calls=%d invoked=%t", rec.calls, res.Execution.Invoked)
	}
	if res.Execution.VerifiedSuccess || res.Execution.WorkerClaimedSuccess {
		t.Fatalf("%+v", res.Execution)
	}
	if res.Execution.PacketRef == "" || res.Execution.Baseline.SHA == "" {
		t.Fatalf("%+v", res.Execution)
	}
	if _, err := os.Stat(filepath.Join(root, res.Execution.PacketRef)); err != nil {
		t.Fatal(err)
	}
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentState != state.StatePrepared {
		t.Fatalf("state=%s", snap.CurrentState)
	}
	after := productFiles(t, root)
	if extra := extraProductFiles(before, after); len(extra) != 0 {
		t.Fatalf("product files mutated: %v", extra)
	}
	if _, err := os.Stat(filepath.Join(root, "mutated.txt")); err == nil {
		t.Fatal("worker wrote a product file")
	}
}

func TestPrepareExplicitPRDInsideAndOutsideRoot(t *testing.T) {
	root := fixtureRepo(t)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()})
	inside := filepath.Join(root, "PRD.md")
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PRDPath: inside, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason != "" {
		t.Fatalf("in-root PRD refused: %s", res.Execution.RefusalReason)
	}

	outside := t.TempDir()
	copyPRD(t, outside, "minimal_valid.md")
	root2 := fixtureRepo(t)
	res, err = engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()}).Prepare(context.Background(), engine.Request{
		ProductRoot: root2, PRDPath: filepath.Join(outside, "PRD.md"), PhaseID: "P1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason == "" {
		t.Fatal("PRD outside product root must be refused")
	}
}

func TestPrepareTmpVsPrivateTmpPRD(t *testing.T) {
	if _, err := os.Stat("/tmp"); err != nil {
		t.Skip("/tmp not present")
	}
	root, err := os.MkdirTemp("/tmp", "prdpr-canon-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	copyPRD(t, root, "minimal_valid.md")
	gitInit(t, root)

	prdPath := filepath.Join(root, "PRD.md")
	alt := prdPath
	if strings.HasPrefix(prdPath, "/private/tmp/") {
		alt = strings.TrimPrefix(prdPath, "/private")
	} else if strings.HasPrefix(prdPath, "/tmp/") {
		alt = filepath.Join("/private", prdPath)
	}
	if _, err := os.Stat(alt); err != nil {
		t.Skip("no /tmp vs /private/tmp alias")
	}
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PRDPath: alt, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason != "" {
		t.Fatalf("aliased in-root PRD refused: %s (root=%s prd=%s)", res.Execution.RefusalReason, root, alt)
	}
}

func TestPrepareMissingPRD(t *testing.T) {
	root := t.TempDir()
	gitInitEmpty(t, root)
	res, err := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()}).Prepare(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked || res.Execution.PacketRef != "" {
		t.Fatalf("%+v", res.Execution)
	}
	if !strings.Contains(strings.ToLower(res.Execution.RefusalReason), "prd") && !strings.Contains(strings.ToLower(res.Execution.RefusalReason), "preflight") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestPrepareInvalidPRD(t *testing.T) {
	root := t.TempDir()
	copyPRD(t, root, "duplicate_phase.md")
	gitInit(t, root)
	res, err := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()}).Prepare(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked {
		t.Fatal("must not prepare invalid PRD")
	}
	if !strings.Contains(res.Execution.RefusalReason, "PRD") && !strings.Contains(strings.ToLower(res.Execution.RefusalReason), "preflight") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestPrepareInvalidGraph(t *testing.T) {
	root := t.TempDir()
	writeString(t, filepath.Join(root, "PRD.md"), cyclicPRD)
	gitInit(t, root)
	res, err := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()}).Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked || res.Packet.TaskID != "" {
		t.Fatalf("%+v", res)
	}
	if !strings.Contains(strings.ToLower(res.Execution.RefusalReason), "graph") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestPrepareBlockedPreflight(t *testing.T) {
	root := fixtureRepo(t)
	env := preflight.DefaultEnv()
	env.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	res, err := engine.New(engine.Options{
		Worker:       panicWorker{},
		NewID:        seqID(),
		PreflightEnv: env,
	}).Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked {
		t.Fatal("must not prepare when preflight is blocked")
	}
	if !strings.Contains(strings.ToLower(res.Execution.RefusalReason), "preflight") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestPrepareGitBaselineFailure(t *testing.T) {
	root := fixtureRepo(t)
	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("d\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()}).Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: prd.PhaseID("P1")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Invoked || res.Execution.PacketRef != "" {
		t.Fatalf("%+v", res.Execution)
	}
	if !strings.Contains(strings.ToLower(res.Execution.RefusalReason), "git") && !strings.Contains(strings.ToLower(res.Execution.RefusalReason), "dirty") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestPreparePacketDeterminism(t *testing.T) {
	root := fixtureRepo(t)
	a, err := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()}).Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || a.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, a.Execution)
	}
	raw1, err := os.ReadFile(filepath.Join(root, a.Execution.PacketRef))
	if err != nil {
		t.Fatal(err)
	}
	p1, err := packet.Unmarshal(raw1)
	if err != nil {
		t.Fatal(err)
	}
	root2 := fixtureRepo(t)
	b, err := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()}).Prepare(context.Background(), engine.Request{ProductRoot: root2, PhaseID: "P1"})
	if err != nil || b.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, b.Execution)
	}
	raw2, err := os.ReadFile(filepath.Join(root2, b.Execution.PacketRef))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := packet.Unmarshal(raw2)
	if err != nil {
		t.Fatal(err)
	}
	if p1.Objective != p2.Objective || p1.PhaseID != p2.PhaseID {
		t.Fatalf("packets diverged: %+v vs %+v", p1, p2)
	}
	if len(p1.Requirements) != len(p2.Requirements) {
		t.Fatalf("requirements %d vs %d", len(p1.Requirements), len(p2.Requirements))
	}
	p1.TaskID, p2.TaskID = "", ""
	p1.ProjectID, p2.ProjectID = "", ""
	p1.ProductRoot, p2.ProductRoot = "", ""
	j1, _ := json.Marshal(p1)
	j2, _ := json.Marshal(p2)
	if string(j1) != string(j2) {
		t.Fatalf("packet fields not deterministic:\n%s\n%s", j1, j2)
	}
}

func TestPrepareDoesNotRequireCursorAgent(t *testing.T) {
	root := fixtureRepo(t)
	env := preflight.DefaultEnv()
	env.LookPath = func(file string) (string, error) {
		if file == "git" {
			return "/usr/bin/git", nil
		}
		return "", os.ErrNotExist
	}
	res, err := engine.New(engine.Options{
		Worker:       panicWorker{},
		NewID:        seqID(),
		PreflightEnv: env,
	}).Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason != "" {
		t.Fatalf("prepare must not require cursor-agent: %s", res.Execution.RefusalReason)
	}
}

type panicWorker struct{}

func (panicWorker) Run(context.Context, cursor.Request) cursor.Result {
	panic("worker must not be invoked during prepare")
}

func productFiles(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".project" || strings.HasPrefix(rel, ".project/") || rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if info.IsDir() && (rel == ".project" || rel == ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		out[rel] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func extraProductFiles(before, after map[string]struct{}) []string {
	var extra []string
	for p := range after {
		if _, ok := before[p]; !ok {
			extra = append(extra, p)
		}
	}
	return extra
}

func gitInit(t *testing.T, root string) {
	t.Helper()
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	run(t, root, "git", "add", "PRD.md")
	run(t, root, "git", "commit", "-m", "init")
}

func gitInitEmpty(t *testing.T, root string) {
	t.Helper()
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "README.md")
	run(t, root, "git", "commit", "-m", "init")
}

func copyPRD(t *testing.T, root, name string) {
	t.Helper()
	src := filepath.Join("..", "prd", "testdata", "prd", name)
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeString(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const cyclicPRD = `# PRD: Cycle

**Product:** Cycle

# 1. Product Overview

Cyclic phases.

# 2. Goals

- Catch cycles

# 3. Phases

P1: One
Objective: one
Dependencies: P2

P2: Two
Objective: two
Dependencies: P1
`
