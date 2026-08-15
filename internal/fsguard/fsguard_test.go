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

func TestResolveCanonicalizesTmpAlias(t *testing.T) {
	if _, err := os.Stat("/tmp"); err != nil {
		t.Skip("/tmp not present")
	}
	root, err := os.MkdirTemp("/tmp", "prdpr-jail-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	prd := filepath.Join(root, "PRD.md")
	if err := os.WriteFile(prd, []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	j, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := j.Resolve(prd)
	if err != nil {
		t.Fatal(err)
	}
	alt := prd
	if len(prd) >= 13 && prd[:13] == "/private/tmp/" {
		alt = prd[8:]
	} else if len(prd) >= 5 && prd[:5] == "/tmp/" {
		alt = "/private" + prd
	}
	if _, err := os.Stat(alt); err != nil {
		t.Skip("no /tmp vs /private/tmp alias")
	}
	gotAlt, err := j.Resolve(alt)
	if err != nil {
		t.Fatalf("aliased PRD inside root rejected: %v (root=%s alt=%s)", err, j.Root(), alt)
	}
	if got != gotAlt {
		t.Fatalf("canonical mismatch %q vs %q", got, gotAlt)
	}
}

func TestResolveRejectsParentPRDAndOutside(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "app")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "PRD.md"), []byte("# outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), []byte("# inside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	j, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(filepath.Join(root, "PRD.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(filepath.Join("..", "PRD.md")); err == nil {
		t.Fatal("../PRD.md must be rejected")
	}
	if _, err := j.Resolve(filepath.Join(parent, "PRD.md")); err == nil {
		t.Fatal("explicit PRD outside root must be rejected")
	}
}

func TestResolveRejectsSymlinkPRDEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "PRD.md"), []byte("# escape\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "PRD.md"), filepath.Join(root, "PRD.md")); err != nil {
		t.Fatal(err)
	}
	j, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve("PRD.md"); err == nil {
		t.Fatal("symlinked PRD escaping root must be rejected")
	}
	if _, err := j.Resolve(filepath.Join(root, "PRD.md")); err == nil {
		t.Fatal("absolute symlink PRD escaping root must be rejected")
	}
}

func TestSymlinkedWorkspaceRootStaysConfined(t *testing.T) {
	realRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(realRoot, "PRD.md"), []byte("# in\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent := t.TempDir()
	link := filepath.Join(parent, "ws")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Fatal(err)
	}
	j, err := New(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve("PRD.md"); err != nil {
		t.Fatal(err)
	}
	escape := filepath.Join(parent, "PRD.md")
	if err := os.WriteFile(escape, []byte("# out\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Resolve(escape); err == nil {
		t.Fatal("PRD beside symlink root must be rejected")
	}
	if _, err := j.Resolve(filepath.Join(link, "..", "PRD.md")); err == nil {
		t.Fatal("path via symlink parent must be rejected")
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
