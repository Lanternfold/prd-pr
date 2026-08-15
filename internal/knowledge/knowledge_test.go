package knowledge

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPutAndHintsStayInProject(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: filepath.Join(dir, "k"), Now: func() time.Time { return time.Unix(1, 0).UTC() }}
	e, err := s.Put(Entry{Category: "test_command", Scope: ScopeProject, Observation: "go test ./...", Source: "verify"})
	if err != nil {
		t.Fatal(err)
	}
	if e.ID == "" {
		t.Fatal("id")
	}
	hints := s.Hints("test_command")
	if len(hints) != 1 || hints[0] != "go test ./..." {
		t.Fatalf("%v", hints)
	}
	other := ProjectStore(t.TempDir())
	if len(other.Hints("test_command")) != 0 {
		t.Fatal("project knowledge leaked")
	}
}

func TestRejectsSecretFreeButEmptyObservation(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if _, err := s.Put(Entry{Category: "x", Scope: ScopeProject}); err == nil {
		t.Fatal("expected error")
	}
}
