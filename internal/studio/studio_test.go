package studio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFromExplicitRoot(t *testing.T) {
	root := fakeStudio(t)
	lay, err := Discover(root, t.TempDir(), t.TempDir())
	if err != nil || lay.Root != root || !lay.Has("Tools") || !lay.Has("Products") {
		t.Fatalf("%+v %v", lay, err)
	}
}

func TestDiscoverDoesNotGuessMissingStudio(t *testing.T) {
	lay, err := Discover("", t.TempDir(), t.TempDir())
	if err != nil || !lay.Empty() {
		t.Fatalf("expected empty layout, got %+v %v", lay, err)
	}
}

func TestDiscoverEnvOverridesHome(t *testing.T) {
	root := fakeStudio(t)
	t.Setenv("PRDPR_STUDIO", root)
	lay, err := Discover("", t.TempDir(), filepath.Join(t.TempDir(), "not-studio"))
	if err != nil || lay.Root != root {
		t.Fatalf("%+v %v", lay, err)
	}
}

func fakeStudio(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, c := range []string{"Tools", "Products", "Experiments"} {
		if err := os.Mkdir(filepath.Join(root, c), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
