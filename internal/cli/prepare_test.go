package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareCLI(t *testing.T) {
	root := prepareFixture(t)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "prepare", "--phase", "P1", root}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, s := range []string{"prepared: true", "invoked: false", "verified_success: false", "packet:", "current_state: PREPARED"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %q in\n%s", s, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "WORKER_FAKE.txt")); err == nil {
		t.Fatal("prepare must not run the fake worker")
	}
}

func TestPrepareCLIMissingPRD(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "--template=")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "README.md")
	git("commit", "-m", "init")

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "prepare", root}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "refused:") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if strings.Contains(stdout.String(), "prepared: true") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func prepareFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "minimal_valid.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "--template=")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	git("add", "PRD.md")
	git("commit", "-m", "init")
	return root
}

func TestPrepareCLISelfDevelopmentFlag(t *testing.T) {
	root := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "auto_verify.md"))
	if err != nil {
		t.Fatal(err)
	}
	prdBody := []byte(strings.Replace(string(src), "**Repository:** example/fixture\n", "**Repository:** example/fixture\n\nExecution mode: SELF_DEVELOPMENT\n", 1))
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), prdBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/lanternfold/prd-pr\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "--template=")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	git("add", ".")
	git("commit", "-m", "init")

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "prepare", "--phase", "P1", root}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("ordinary prepare must refuse orchestrator: exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "orchestrator") {
		t.Fatalf("stderr=%q", stderr.String())
	}

	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	code = Main([]string{"prdpr", "prepare", "--self-development", "--phase", "P1", root}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("self-development prepare exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "prepared: true") || !strings.Contains(out, "execution_mode: SELF_DEVELOPMENT") {
		t.Fatalf("stdout=%q", out)
	}
}
