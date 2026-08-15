package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/prd"
)

func TestValidatePRDValidAndJSON(t *testing.T) {
	path := filepath.Join("..", "prd", "testdata", "prd", "contract", "pass_complete.md")
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "validate-prd", path}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Status: VALID") {
		t.Fatalf("%s", stdout.String())
	}

	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	code = Main([]string{"prdpr", "validate-prd", "--json", path}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("json exit %d %s", code, stderr.String())
	}
	var res prd.ContractResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Status != prd.ContractValid {
		t.Fatalf("%+v", res)
	}
}

func TestValidatePRDRejected(t *testing.T) {
	path := filepath.Join("..", "prd", "testdata", "prd", "contract", "reject_missing_objective.md")
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "validate-prd", path}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("exit %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "Status: REJECTED") || !strings.Contains(out, "PRD-VAL-002") {
		t.Fatalf("%s", out)
	}
}

func TestValidatePRDUsage(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "validate-prd"}, stdout, stderr, testRuntime())
	if code != exitUsage {
		t.Fatalf("exit %d", code)
	}
}

func TestValidatePRDDoesNotCreateWorkspace(t *testing.T) {
	dir := t.TempDir()
	prdPath := filepath.Join(dir, "PRD.md")
	src, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "contract", "reject_missing_objective.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prdPath, src, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "validate-prd", prdPath}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("exit %d %s %s", code, stderr.String(), stdout.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "PRD.md" {
		t.Fatalf("unexpected files: %v", entries)
	}
}
