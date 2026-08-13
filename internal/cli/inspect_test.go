package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/prd"
)

func fixturePRD(name string) string {
	return filepath.Join("..", "prd", "testdata", "prd", name)
}

func TestInspectMinimalValid(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "inspect", fixturePRD("minimal_valid.md")}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, s := range []string{"PRD→PR", "Requirements: 2", "Acceptance criteria: 2", "Phases: 1", "Status: VALID"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %q in\n%s", s, out)
		}
	}
}

func TestInspectDuplicateInvalid(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "inspect", fixturePRD("duplicate_req.md")}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("exit %d want %d stdout=%q", code, exitError, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Status: INVALID") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "REQ_DUPLICATE") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestInspectJSON(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "inspect", "--json", fixturePRD("minimal_valid.md")}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	var doc prd.Document
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Requirements) != 2 {
		t.Fatalf("json requirements = %d", len(doc.Requirements))
	}
}

func TestInspectMissingFile(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "inspect", filepath.Join(t.TempDir(), "nope.md")}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "cannot read") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestInspectGraph(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "inspect", "--graph", fixturePRD("minimal_valid.md")}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, s := range []string{"Graph:", "Nodes: 1", "Edges: 0", "Topological order:", "P1", "Ready:", "Cycles:", "0"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %q in\n%s", s, out)
		}
	}
}

func TestInspectUsage(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "inspect"}, stdout, stderr, testRuntime())
	if code != exitUsage {
		t.Fatalf("exit %d", code)
	}
}
