package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lanternfold/prd-pr/internal/preflight"
)

func TestPreflightJSONReady(t *testing.T) {
	root := cliGitPRD(t)
	rt := preflightRuntime("git", "cursor-agent", "go")
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "preflight", "--json", root}, stdout, stderr, rt)
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var r preflight.Report
	if err := json.Unmarshal(stdout.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != preflight.OverallReady {
		t.Fatalf("status=%s blocking=%v", r.Status, r.Blocking)
	}
}

func TestPreflightBlockedMissingAgent(t *testing.T) {
	root := cliGitPRD(t)
	rt := preflightRuntime("git")
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "preflight", root}, stdout, stderr, rt)
	if code != exitError {
		t.Fatalf("exit %d want 1 stdout=%q", code, stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "PRD→PR PREFLIGHT") || !strings.Contains(out, "BLOCKED") {
		t.Fatalf("stdout=%s", out)
	}
	if strings.Contains(out, "✓") && !strings.Contains(out, "Cursor Agent") {
		t.Fatalf("expected cursor agent section:\n%s", out)
	}
}

func TestPreflightUsage(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "preflight", "--bogus"}, stdout, stderr, preflightRuntime("git"))
	if code != exitUsage {
		t.Fatalf("exit %d", code)
	}
}

func TestDoctorStillReportsGitAndNewTools(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "doctor"}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	for _, s := range []string{"Git:", "available", "Cursor editor:", "Cursor Agent:", "GitHub CLI:"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %q in\n%s", s, out)
		}
	}
}

func preflightRuntime(bins ...string) Runtime {
	rt := testRuntime()
	set := map[string]string{}
	for _, b := range bins {
		set[b] = "/fake/" + b
	}
	rt.LookPath = func(file string) (string, error) {
		if p, ok := set[file]; ok {
			return p, nil
		}
		return "", os.ErrNotExist
	}
	rt.Now = func() time.Time { return time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC) }
	return rt
}

func cliGitPRD(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "minimal_valid.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "--template=")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	runGit("add", "PRD.md")
	runGit("commit", "-m", "init")
	return root
}
