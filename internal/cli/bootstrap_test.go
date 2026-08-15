package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPRDOnlyInvocationBootstraps(t *testing.T) {
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
	studio := t.TempDir()
	for _, c := range []string{"Tools", "Products", "Experiments"} {
		if err := os.Mkdir(filepath.Join(studio, c), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PRDPR_STUDIO", studio)
	src, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "contract", "pass_complete.md"))
	if err != nil {
		t.Fatal(err)
	}
	prdPath := filepath.Join(t.TempDir(), "PRD.md")
	if err := os.WriteFile(prdPath, src, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", prdPath}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "prepared: true") || !strings.Contains(out, "product_root:") {
		t.Fatalf("%s", out)
	}
	dest := filepath.Join(studio, "Tools", "notes-library")
	if _, err := os.Stat(filepath.Join(dest, "PRD.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatal(err)
	}
}
