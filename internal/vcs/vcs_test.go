package vcs_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestEstablishBaselineAndChangedSince(t *testing.T) {
	root := initRepo(t)
	c := vcs.Default()
	base, sn, err := c.EstablishBaseline(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if base.SHA == "" || sn.Dirty {
		t.Fatalf("baseline=%+v snap=%+v", base, sn)
	}

	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jail, err := fsguard.New(root)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := c.ChangedSince(context.Background(), root, base.SHA, jail)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0] != "hello.txt" {
		t.Fatalf("changed=%v", changed)
	}
}

func TestRefuseDirtyTree(t *testing.T) {
	root := initRepo(t)
	if err := os.WriteFile(filepath.Join(root, "extra.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := vcs.Default().EstablishBaseline(context.Background(), root)
	if err == nil {
		t.Fatal("expected dirty refusal")
	}
}

func TestProjectMetaIsNotDirty(t *testing.T) {
	root := initRepo(t)
	if err := os.MkdirAll(filepath.Join(root, ".project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".project", "state.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := vcs.Default().EstablishBaseline(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNoHEAD(t *testing.T) {
	root := t.TempDir()
	run(t, root, "git", "init", "--template=")
	_, _, err := vcs.Default().EstablishBaseline(context.Background(), root)
	if err == nil {
		t.Fatal("expected missing HEAD")
	}
}

func TestOutsideJailChange(t *testing.T) {
	root := initRepo(t)
	c := &vcs.Client{
		LookPath: exec.LookPath,
		Git: func(ctx context.Context, dir string, args ...string) (string, error) {
			if args[0] == "diff" && args[1] == "--name-only" {
				return "../outside.txt", nil
			}
			if args[0] == "ls-files" {
				return "", nil
			}
			return vcs.Default().Git(ctx, dir, args...)
		},
	}
	jail, err := fsguard.New(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.ChangedSince(context.Background(), root, "abc1234", jail)
	if err == nil {
		t.Fatal("expected outside-jail error")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "hello.txt")
	run(t, root, "git", "commit", "-m", "init")
	return root
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

func TestCreateBranchAndCommit(t *testing.T) {
	root := initRepo(t)
	c := vcs.Default()
	if err := c.CreateBranch(context.Background(), root, "prdpr/p1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("next\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jail, err := fsguard.New(root)
	if err != nil {
		t.Fatal(err)
	}
	sha, err := c.Commit(context.Background(), root, "update hello", []string{"hello.txt"}, jail)
	if err != nil || sha == "" {
		t.Fatalf("sha=%s err=%v", sha, err)
	}
}

func TestCommitRefusesOutsidePath(t *testing.T) {
	root := initRepo(t)
	jail, err := fsguard.New(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = vcs.Default().Commit(context.Background(), root, "bad", []string{"../secret"}, jail)
	if err == nil {
		t.Fatal("expected refusal")
	}
}
