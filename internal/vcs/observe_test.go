package vcs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestObserveNotRepository(t *testing.T) {
	root := t.TempDir()
	obs := vcs.Default().Observe(context.Background(), root)
	if obs.State != vcs.StateNotRepository && obs.State != vcs.StateNotInstalled {
		t.Fatalf("state=%s", obs.State)
	}
}

func TestObserveNoCommitsAndCleanDirty(t *testing.T) {
	root := t.TempDir()
	run(t, root, "git", "init", "--template=")
	obs := vcs.Default().Observe(context.Background(), root)
	if obs.State != vcs.StateNoCommits {
		t.Fatalf("state=%s want %s", obs.State, vcs.StateNoCommits)
	}

	root2 := initRepo(t)
	obs = vcs.Default().Observe(context.Background(), root2)
	if obs.State != vcs.StateClean || !obs.HasHEAD {
		t.Fatalf("clean observe=%+v", obs)
	}

	if err := os.WriteFile(filepath.Join(root2, "extra.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	obs = vcs.Default().Observe(context.Background(), root2)
	if obs.State != vcs.StateDirty {
		t.Fatalf("dirty observe=%+v", obs)
	}
}
