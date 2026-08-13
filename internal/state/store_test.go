package state

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitSaveLoadAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir, Options{NewID: func() string { return "proj-fixed" }})

	r1, err := s.Init()
	if err != nil {
		t.Fatal(err)
	}
	if !r1.Created {
		t.Fatal("first init should create")
	}
	if r1.State.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %d", r1.State.SchemaVersion)
	}
	if r1.State.ProjectID != "proj-fixed" {
		t.Fatalf("project_id = %q", r1.State.ProjectID)
	}
	if r1.State.ProjectStatus != StatusCreated {
		t.Fatalf("status = %q", r1.State.ProjectStatus)
	}

	r2, err := s.Init()
	if err != nil {
		t.Fatal(err)
	}
	if r2.Created {
		t.Fatal("second init must not create")
	}
	if r2.State.ProjectID != "proj-fixed" {
		t.Fatalf("project_id changed to %q", r2.State.ProjectID)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProjectID != "proj-fixed" || loaded.CreatedAt != r1.State.CreatedAt {
		t.Fatalf("existing state was not preserved: %+v", loaded)
	}

	if _, err := os.Stat(filepath.Join(dir, DirName, StateFileName)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, DirName, EventsFileName)); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaVersionRejected(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir, Options{})
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, DirName, StateFileName)
	if err := os.WriteFile(path, []byte(`{"schema_version":99,"project_id":"x","product_root":"`+dir+`","project_status":"PROJECT_CREATED"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("want schema error, got %v", err)
	}
	if _, err := s.Init(); err == nil {
		t.Fatal("init must not overwrite unsupported schema")
	}
}

func TestCorruptStateNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir, Options{})
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, DirName, StateFileName)
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("want corrupt error, got %v", err)
	}
	if _, err := s.Init(); err == nil {
		t.Fatal("init must not replace corrupt state")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{not-json" {
		t.Fatalf("corrupt state was modified:\n%s", got)
	}
}

func TestSaveLoadAfterRestart(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir, Options{NewID: func() string { return "p1" }})
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	st.CurrentRunID = "run-1"
	st.CurrentPhaseID = "phase-a"
	st.CurrentState = "PREFLIGHT"
	if err := s.Save(st); err != nil {
		t.Fatal(err)
	}

	s2 := mustOpen(t, dir, Options{})
	got, err := s2.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != "p1" || got.CurrentRunID != "run-1" || got.CurrentPhaseID != "phase-a" || got.CurrentState != "PREFLIGHT" {
		t.Fatalf("resume mismatch: %+v", got)
	}
}

func TestEventsAppendJSONL(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	s := mustOpen(t, dir, Options{Now: func() time.Time { return now }})
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(Event{Kind: KindIntent, Name: EventRunStarted, RunID: "run-1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(Event{Kind: KindResult, Name: EventRunStarted, RunID: "run-1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(Event{Kind: KindIntent, Name: EventRunEnded, RunID: "run-1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendEvent(Event{Kind: KindResult, Name: EventRunEnded, RunID: "run-1"}); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(filepath.Join(dir, DirName, EventsFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []Event
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			t.Fatalf("invalid JSONL line: %s", line)
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(lines) < 6 {
		t.Fatalf("got %d events, want at least 6 (init intent/result + 4 run)", len(lines))
	}
	if lines[0].Kind != KindIntent || lines[0].Name != EventProjectInitialized {
		t.Fatalf("first event = %+v", lines[0])
	}
	if lines[1].Kind != KindResult || lines[1].Name != EventProjectInitialized {
		t.Fatalf("second event = %+v", lines[1])
	}
	names := make([]string, 0, len(lines))
	for _, ev := range lines {
		names = append(names, ev.Kind+":"+ev.Name)
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{"intent:run_started", "result:run_started", "intent:run_ended", "result:run_ended"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %s", want, joined)
		}
	}
}

func TestCrashBeforeRenameKeepsPreviousState(t *testing.T) {
	dir := t.TempDir()
	renames := 0
	s := mustOpen(t, dir, Options{
		NewID: func() string { return "keep-me" },
		Rename: func(oldpath, newpath string) error {
			renames++
			if renames >= 2 {
				return errors.New("simulated crash before rename")
			}
			return os.Rename(oldpath, newpath)
		},
	})
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, DirName, StateFileName))
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	st.CurrentRunID = "should-not-persist"
	if err := s.Save(st); err == nil {
		t.Fatal("expected save to fail before rename")
	}
	after, err := os.ReadFile(filepath.Join(dir, DirName, StateFileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("state.json changed after failed write:\nbefore=%s\nafter=%s", before, after)
	}
	var kept State
	if err := json.Unmarshal(after, &kept); err != nil {
		t.Fatal(err)
	}
	if kept.ProjectID != "keep-me" || kept.CurrentRunID != "" {
		t.Fatalf("previous valid state lost: %+v", kept)
	}
}

func TestInitDoesNotWriteOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "product")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	s := mustOpen(t, root, Options{})
	if _, err := s.Init(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "product" {
			t.Fatalf("unexpected entry outside product root: %s", e.Name())
		}
	}
}

func TestLoadNotInitialized(t *testing.T) {
	s := mustOpen(t, t.TempDir(), Options{})
	if _, err := s.Load(); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("err = %v", err)
	}
}

func mustOpen(t *testing.T, dir string, opts Options) *Store {
	t.Helper()
	s, err := OpenWith(dir, opts)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
