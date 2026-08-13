package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/state"
)

func TestInitCreatesProjectAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "init", dir}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("init exit %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Initialized") {
		t.Fatalf("stdout=%q", stdout.String())
	}

	statePath := filepath.Join(dir, state.DirName, state.StateFileName)
	eventsPath := filepath.Join(dir, state.DirName, state.EventsFileName)
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(eventsPath); err != nil {
		t.Fatal(err)
	}
	var st state.State
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.SchemaVersion != state.SchemaVersion || st.ProjectID == "" {
		t.Fatalf("state = %+v", st)
	}

	stdout2, stderr2 := new(bytes.Buffer), new(bytes.Buffer)
	code = Main([]string{"prdpr", "init", dir}, stdout2, stderr2, testRuntime())
	if code != exitOK {
		t.Fatalf("second init exit %d stderr=%q", code, stderr2.String())
	}
	if !strings.Contains(stdout2.String(), "already initialized") {
		t.Fatalf("stdout=%q", stdout2.String())
	}
	raw2, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw2) != string(raw) {
		t.Fatal("second init changed state.json")
	}
}

func TestStatusReadsPersistedState(t *testing.T) {
	dir := t.TempDir()
	if code := Main([]string{"prdpr", "init", dir}, new(bytes.Buffer), new(bytes.Buffer), testRuntime()); code != exitOK {
		t.Fatal("init failed")
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "status", dir}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("status exit %d stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	for _, s := range []string{"schema_version:", "project_id:", "project_status:", "PROJECT_CREATED"} {
		if !strings.Contains(out, s) {
			t.Fatalf("status missing %q:\n%s", s, out)
		}
	}
}

func TestStatusNotInitialized(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "status", dir}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stderr.String(), "prdpr init") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestInitInvalidRoot(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "init", filepath.Join(t.TempDir(), "missing")}, stdout, stderr, testRuntime())
	if code != exitError {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "invalid") && !strings.Contains(stderr.String(), "missing") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestInitCwd(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "init"}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit %d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, state.DirName, state.StateFileName)); err != nil {
		t.Fatal(err)
	}
}
