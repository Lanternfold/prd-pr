package vcs_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestEnsureOwnedBranchCreateReuseConflict(t *testing.T) {
	root := initRepo(t)
	c := vcs.Default()
	ctx := context.Background()
	name := vcs.FeatureBranchName("proj1")
	br, err := c.EnsureOwnedBranch(ctx, root, name, "", "")
	if err != nil || !br.Exists || br.Reason != "created" {
		t.Fatalf("%+v %v", br, err)
	}
	br2, err := c.EnsureOwnedBranch(ctx, root, name, name, br.SHA)
	if err != nil || br2.Reason != "reused" {
		t.Fatalf("reuse %+v %v", br2, err)
	}

	other := initRepo(t)
	if err := c.EnsureBranch(ctx, other, "unrelated"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "hello.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, other, "git", "add", "hello.txt")
	run(t, other, "git", "commit", "-m", "other work")
	run(t, other, "git", "checkout", "-")
	_, err = c.EnsureOwnedBranch(ctx, other, "unrelated", "", "")
	if err == nil {
		t.Fatal("expected conflict for unrelated branch")
	}
}

func TestDeleteBranchSafeAndRefusesCurrent(t *testing.T) {
	root := initRepo(t)
	c := vcs.Default()
	ctx := context.Background()
	sn, err := c.Inspect(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteBranch(ctx, root, sn.Branch); err == nil {
		t.Fatal("must refuse deleting the current branch")
	}
	if err := c.EnsureBranch(ctx, root, "feature-tmp"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureBranch(ctx, root, sn.Branch); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteBranch(ctx, root, "feature-tmp"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteBranch(ctx, root, "main"); err == nil {
		t.Fatal("must refuse deleting protected/base branch")
	}
}

func TestFeatureBranchNameDeterministic(t *testing.T) {
	if vcs.FeatureBranchName("abc") != vcs.FeatureBranchName("abc") {
		t.Fatal("not deterministic")
	}
	if vcs.FeatureBranchName("abc") != "prdpr/abc" {
		t.Fatal(vcs.FeatureBranchName("abc"))
	}
}
