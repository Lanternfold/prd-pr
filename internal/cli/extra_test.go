package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/repair"
	"github.com/lanternfold/prd-pr/internal/state"
)

func TestInteractiveRepairCapViaCLI(t *testing.T) {
	root := verifyFixture(t, false)
	rt := verifyRuntime()
	run := func(args ...string) (string, string, int) {
		t.Helper()
		stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
		code := Main(append([]string{"prdpr"}, args...), stdout, stderr, rt)
		return stdout.String(), stderr.String(), code
	}

	if out, err, code := run("prepare", "--phase", "P1", root); code != exitOK {
		t.Fatalf("prepare %d %s %s", code, err, out)
	}
	out, err, code := run("verify", root)
	if code != exitError {
		t.Fatalf("verify1 %d %s %s", code, err, out)
	}
	if !strings.Contains(out, "verified_success: false") {
		t.Fatalf("verify1 stdout=%s", out)
	}

	for i := 1; i <= 3; i++ {
		out, err, code := run("repair", root)
		if code != exitOK {
			t.Fatalf("repair %d: %d stderr=%s stdout=%s", i, code, err, out)
		}
		if !strings.Contains(out, "attempt: "+itoa(i)) {
			t.Fatalf("repair %d stdout=%s", i, out)
		}
		if !strings.Contains(out, "max_attempts: 3") {
			t.Fatalf("repair %d missing max: %s", i, out)
		}
		if out, err, code := run("verify", root); code != exitError {
			t.Fatalf("verify after repair %d: %d %s %s", i, code, err, out)
		}
	}

	out, err, code = run("repair", root)
	if code != exitError {
		t.Fatalf("fourth repair must be refused, exit %d stdout=%s stderr=%s", code, out, err)
	}
	if !strings.Contains(err, "exhausted") && !strings.Contains(out, "exhausted") {
		t.Fatalf("fourth repair: stdout=%s stderr=%s", out, err)
	}

	raw, rerr := os.ReadFile(filepath.Join(root, ".project", "incident.json"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	var inc repair.Incident
	if jerr := json.Unmarshal(raw, &inc); jerr != nil {
		t.Fatal(jerr)
	}
	if len(inc.Attempts) != 3 || inc.MaxAttempts != 3 || !inc.Exhausted {
		t.Fatalf("incident=%+v", inc)
	}
}

func TestHumanResumeAfterRepairExhaustionViaCLI(t *testing.T) {
	root := verifyFixture(t, false)
	rt := verifyRuntime()
	run := func(args ...string) (string, string, int) {
		t.Helper()
		stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
		code := Main(append([]string{"prdpr"}, args...), stdout, stderr, rt)
		return stdout.String(), stderr.String(), code
	}
	if _, err, code := run("prepare", "--phase", "P1", root); code != exitOK {
		t.Fatalf("prepare %d %s", code, err)
	}
	exBefore := readExecutionJSON(t, root)
	if err := os.WriteFile(filepath.Join(root, "dirty_repair.txt"), []byte("left by failed repair\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := run("verify", root); code != exitError {
		t.Fatalf("verify %d", code)
	}
	for i := 0; i < 3; i++ {
		if out, err, code := run("repair", root); code != exitOK {
			t.Fatalf("repair %d: %d %s %s", i+1, code, err, out)
		}
		if _, _, code := run("verify", root); code != exitError {
			t.Fatalf("verify after repair %d", i+1)
		}
	}
	if _, err, code := run("repair", root); code != exitError {
		t.Fatalf("fourth %d %s", code, err)
	}

	req, err := human.LoadRequest(root)
	if err != nil {
		t.Fatal(err)
	}
	if out, errOut, code := run("feedback", "--request", req.ID, "--text", "inspect and continue", "--status", human.CredPresentUnverified, root); code != exitOK {
		t.Fatalf("feedback %d %s %s", code, errOut, out)
	}
	if out, errOut, code := run("resume", root); code != exitOK {
		t.Fatalf("resume %d %s %s", code, errOut, out)
	}

	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.CurrentState == state.StatePrepared {
		t.Fatal("resume must not reset the incident to PREPARED")
	}

	exAfter := readExecutionJSON(t, root)
	if exAfter["baseline"].(map[string]any)["sha"] != exBefore["baseline"].(map[string]any)["sha"] {
		t.Fatalf("baseline changed: before=%v after=%v", exBefore["baseline"], exAfter["baseline"])
	}

	raw, _ := os.ReadFile(filepath.Join(root, ".project", "incident.json"))
	var inc repair.Incident
	_ = json.Unmarshal(raw, &inc)
	if len(inc.Attempts) != 3 || inc.Exhausted != true {
		t.Fatalf("attempt history lost: %+v", inc)
	}
	if _, err := os.Stat(filepath.Join(root, "dirty_repair.txt")); err != nil {
		t.Fatal("working tree edits must be preserved")
	}
	out, _, code := run("verify", root)
	if code != exitError {
		t.Fatalf("verify after resume should still run, exit %d %s", code, out)
	}
	if !strings.Contains(out, "verified_success: false") {
		t.Fatalf("verify after resume: %s", out)
	}
	raw, _ = os.ReadFile(filepath.Join(root, ".project", "incident.json"))
	_ = json.Unmarshal(raw, &inc)
	if len(inc.Attempts) != 3 {
		t.Fatalf("verify after resume must not add a fourth attempt: %+v", inc)
	}
}

func readExecutionJSON(t *testing.T, root string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".project", "execution.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func itoa(n int) string {
	return []string{"0", "1", "2", "3", "4"}[n]
}
