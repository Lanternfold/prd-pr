package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFakeWorker(t *testing.T) {
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

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "run", "--worker", "fake", "--phase", "P1", root}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, s := range []string{"worker_claimed_success: true", "verified_success: false", "invoked: true", "WORKER_FAKE.txt"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %q in\n%s", s, out)
		}
	}
}

func TestRunRefusesNonRepo(t *testing.T) {
	root := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "minimal_valid.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "run", "--worker", "fake", root}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "refused:") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if strings.Contains(stdout.String(), "verified_success: true") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}
