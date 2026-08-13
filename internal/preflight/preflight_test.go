package preflight

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestValidProjectReady(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	env := testEnv(t, "git", "cursor-agent", "go", "gh")
	r := New(env).Run(context.Background(), Request{ProductRoot: root})
	if r.Status != OverallReady {
		t.Fatalf("status=%s blocking=%v", r.Status, r.Blocking)
	}
	assertCheck(t, r, "project.prd", StatusAvailable, false)
	assertCheck(t, r, "project.root", StatusAvailable, false)
	assertCheck(t, r, "project.git", StatusAvailable, false)
	assertCheck(t, r, "machine.cursor_agent", StatusAvailable, false)
	assertCheck(t, r, "project.graph", StatusAvailable, false)
	if r.Repository == nil || r.Repository.State != vcs.StateClean {
		t.Fatalf("repo=%+v", r.Repository)
	}
}

func TestMissingPRD(t *testing.T) {
	root := gitDir(t)
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	if r.Status != OverallBlocked {
		t.Fatalf("status=%s", r.Status)
	}
	assertCheck(t, r, "project.prd", StatusMissing, true)
}

func TestInvalidPRD(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "duplicate_req.md"))
	if err != nil {
		t.Fatal(err)
	}
	root := gitPRD(t, string(src))
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	if r.Status != OverallBlocked {
		t.Fatalf("status=%s", r.Status)
	}
	assertCheck(t, r, "project.prd", StatusError, true)
}

func TestGitBinaryMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), []byte(minimalPRD()), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New(testEnv(t, "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	mg := mustCheck(t, r, "machine.git")
	if !mg.Blocking {
		t.Fatalf("missing git should block: %+v", mg)
	}
	pg := mustCheck(t, r, "project.git")
	if !pg.Blocking || !strings.Contains(pg.Detail, "not installed") {
		t.Fatalf("project git=%+v", pg)
	}
}

func TestNonGitProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), []byte(minimalPRD()), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	if r.Status != OverallBlocked {
		t.Fatalf("status=%s", r.Status)
	}
	c := mustCheck(t, r, "project.git")
	if !c.Blocking || !strings.Contains(c.Detail, "not a repository") {
		t.Fatalf("%+v", c)
	}
}

func TestGitNoCommits(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), []byte(minimalPRD()), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "init", "--template=")
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	c := mustCheck(t, r, "project.git")
	if !c.Blocking || c.Detail != "no commits" {
		t.Fatalf("%+v", c)
	}
}

func TestCleanAndDirtyGit(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	env := testEnv(t, "git", "cursor-agent")
	r := New(env).Run(context.Background(), Request{ProductRoot: root})
	assertCheck(t, r, "project.git", StatusAvailable, false)

	if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = New(env).Run(context.Background(), Request{ProductRoot: root})
	c := mustCheck(t, r, "project.git")
	if c.Status != StatusWarning || c.Blocking {
		t.Fatalf("dirty should warn, not block: %+v", c)
	}
	if r.Status != OverallReady {
		t.Fatalf("dirty tree must not fail preflight, status=%s", r.Status)
	}
}

func TestCursorEditorOnly(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	r := New(testEnv(t, "git", "cursor")).Run(context.Background(), Request{ProductRoot: root})
	assertCheck(t, r, "machine.cursor_editor", StatusAvailable, false)
	c := mustCheck(t, r, "machine.cursor_agent")
	if c.Status != StatusBlocking || !c.Blocking {
		t.Fatalf("agent=%+v", c)
	}
	if r.Status != OverallBlocked {
		t.Fatalf("status=%s", r.Status)
	}
}

func TestCursorAgentAvailableAndMissing(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	assertCheck(t, r, "machine.cursor_agent", StatusAvailable, false)

	r = New(testEnv(t, "git")).Run(context.Background(), Request{ProductRoot: root})
	c := mustCheck(t, r, "machine.cursor_agent")
	if !c.Blocking {
		t.Fatalf("missing agent should block: %+v", c)
	}
}

func TestGitHubCLI(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	r := New(testEnv(t, "git", "cursor-agent", "gh")).Run(context.Background(), Request{ProductRoot: root})
	assertCheck(t, r, "machine.github_cli", StatusAvailable, false)

	r = New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	c := mustCheck(t, r, "machine.github_cli")
	if c.Blocking || c.Status != StatusOptional {
		t.Fatalf("missing gh should be optional: %+v", c)
	}
	if r.Status != OverallReady {
		t.Fatalf("status=%s blocking=%v", r.Status, r.Blocking)
	}
}

func TestOptionalAndRequiredDependencies(t *testing.T) {
	prd := minimalPRD() + `

# Dependencies

OPTIONAL
- gh

BLOCKING
- docker
`
	root := gitPRD(t, prd)
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	var opt, req Check
	for _, c := range r.Checks {
		if strings.Contains(c.Name, "gh") {
			opt = c
		}
		if strings.Contains(c.Name, "docker") {
			req = c
		}
	}
	if opt.Status != StatusOptional || opt.Blocking {
		t.Fatalf("optional gh=%+v", opt)
	}
	if req.Status != StatusBlocking || !req.Blocking {
		t.Fatalf("required docker=%+v", req)
	}
}

func TestCredentialsNeverLeak(t *testing.T) {
	secret := "super-secret-value-xyz"
	prd := minimalPRD() + `

# Credential Handling

- credential: GITHUB_TOKEN
`
	root := gitPRD(t, prd)
	env := testEnv(t, "git", "cursor-agent")
	env.LookupEnv = func(key string) (string, bool) {
		if key == "GITHUB_TOKEN" {
			return secret, true
		}
		return "", false
	}
	r := New(env).Run(context.Background(), Request{ProductRoot: root})
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatalf("secret leaked in JSON")
	}
	var buf bytes.Buffer
	if err := Format(&buf, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), secret) {
		t.Fatalf("secret leaked in text")
	}
	found := false
	for _, c := range r.Checks {
		if strings.Contains(c.Name, "GITHUB_TOKEN") && c.Status == StatusWarning {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected present credential warning, checks=%v", r.Checks)
	}

	env.LookupEnv = func(string) (string, bool) { return "", false }
	r = New(env).Run(context.Background(), Request{ProductRoot: root})
	c := mustCheck(t, r, "project.credential.0")
	if !c.Blocking {
		t.Fatalf("absent credential should block: %+v", c)
	}
}

func TestFilesystemBoundary(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte(minimalPRD()), 0o644); err != nil {
		t.Fatal(err)
	}
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{
		ProductRoot: root,
		PRDPath:     outside,
	})
	c := mustCheck(t, r, "project.prd")
	if !c.Blocking || c.Status != StatusError {
		t.Fatalf("%+v", c)
	}
}

func TestValidAndCyclicGraph(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	assertCheck(t, r, "project.graph", StatusAvailable, false)

	cyclic := `# PRD: Cycle

**Product:** Cycle

# 1. Product Overview

cycle

# 2. Goals

- cycle

# 7. Phases

## P1: A
Dependencies: P2

## P2: B
Dependencies: P1
`
	root = gitPRD(t, cyclic)
	r = New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	c := mustCheck(t, r, "project.graph")
	if !c.Blocking || c.Status != StatusError {
		t.Fatalf("cycle=%+v", c)
	}
	if !strings.Contains(strings.ToLower(c.Detail), "cycle") {
		t.Fatalf("detail=%s", c.Detail)
	}
}

func TestDeterministicRepeatedReport(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	env := testEnv(t, "git", "cursor-agent")
	a := New(env).Run(context.Background(), Request{ProductRoot: root})
	b := New(env).Run(context.Background(), Request{ProductRoot: root})
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if !bytes.Equal(ja, jb) {
		t.Fatalf("reports differ\n%s\n%s", ja, jb)
	}
}

func TestMachineCheckCache(t *testing.T) {
	calls := 0
	env := testEnv(t, "git", "cursor-agent")
	look := env.LookPath
	env.LookPath = func(file string) (string, error) {
		if file == "git" {
			calls++
		}
		return look(file)
	}
	c := New(env)
	_ = c.Machine()
	_ = c.Machine()
	if calls == 0 {
		t.Fatal("expected lookPath")
	}
	first := calls
	_ = c.Machine()
	if calls != first {
		t.Fatalf("machine checks were repeated: %d -> %d", first, calls)
	}
}

func TestDoesNotModifyProject(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	before := listAll(t, root)
	_ = New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	after := listAll(t, root)
	if before != after {
		t.Fatalf("preflight modified the tree\nbefore=%s\nafter=%s", before, after)
	}
}

func TestJSONHasNoTerminalMarks(t *testing.T) {
	root := gitPRD(t, minimalPRD())
	r := New(testEnv(t, "git", "cursor-agent")).Run(context.Background(), Request{ProductRoot: root})
	var buf bytes.Buffer
	if err := FormatJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, mark := range []string{"✓", "✗", "⚠", "─"} {
		if strings.Contains(s, mark) {
			t.Fatalf("JSON contains %q", mark)
		}
	}
	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
}

func testEnv(t *testing.T, bins ...string) Env {
	t.Helper()
	set := map[string]string{}
	for _, b := range bins {
		set[b] = "/fake/" + b
	}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	look := func(file string) (string, error) {
		if p, ok := set[file]; ok {
			return p, nil
		}
		return "", errors.New("not found")
	}
	return Env{
		Now:       func() time.Time { return now },
		LookPath:  look,
		LookupEnv: func(string) (string, bool) { return "", false },
		Git:       &vcs.Client{LookPath: look, Git: vcs.Default().Git},
		GOOS:      "darwin",
		GOARCH:    "arm64",
		GoVersion: "go1.26.4",
	}
}

func gitPRD(t *testing.T, markdown string) string {
	t.Helper()
	root := gitDir(t)
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), []byte(markdown), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "PRD.md")
	run(t, root, "git", "commit", "-m", "prd")
	return root
}

func gitDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	return root
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

func mustCheck(t *testing.T, r *Report, id string) Check {
	t.Helper()
	for _, c := range r.Checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("missing check %s in %+v", id, r.Checks)
	return Check{}
}

func assertCheck(t *testing.T, r *Report, id string, status Status, blocking bool) {
	t.Helper()
	c := mustCheck(t, r, id)
	if c.Status != status || c.Blocking != blocking {
		t.Fatalf("%s status=%s blocking=%v want %s %v detail=%s", id, c.Status, c.Blocking, status, blocking, c.Detail)
	}
}

func listAll(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if info.IsDir() {
			return nil
		}
		b.WriteString(rel)
		b.WriteByte('\n')
		return nil
	})
	return b.String()
}

func minimalPRD() string {
	src, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "minimal_valid.md"))
	if err != nil {
		panic(err)
	}
	return string(src)
}
