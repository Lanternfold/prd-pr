package fsguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewRejectsInvalidRoot(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected error for empty root")
	}
	if _, err := New(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected error for missing root")
	}
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(f); err == nil {
		t.Fatal("expected error for file root")
	}
}

func TestResolveStaysInsideRoot(t *testing.T) {
	root := t.TempDir()
	j, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := j.Resolve(filepath.Join(".project", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(j.Root(), ".project", "state.json")
	if got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
	insideAbs, err := j.Resolve(want)
	if err != nil {
		t.Fatal(err)
	}
	if insideAbs != want {
		t.Fatalf("absolute in-root path = %q, want %q", insideAbs, want)
	}
}

func TestResolveRejectsEscape(t *testing.T) {
	root := t.TempDir()
	j, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"..",
		filepath.Join("..", "outside"),
		filepath.Join(".project", "..", "..", "outside"),
		"/etc/passwd",
		filepath.Join(filepath.Dir(root), "sibling"),
	}
	for _, p := range cases {
		if _, err := j.Resolve(p); err == nil {
			t.Fatalf("expected rejection of %q", p)
		}
	}
}

func TestContainsDoesNotMatchPrefixSibling(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "app")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	j, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if j.Contains(filepath.Join(parent, "app-extra", "x")) {
		t.Fatal("prefix sibling must not be inside jail")
	}
}
