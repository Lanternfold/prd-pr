package vcs_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestInitAndInitialCommitIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "PRD.md"), []byte("# p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := vcs.Default()
	ctx := context.Background()
	if err := c.Init(ctx, root, "main"); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	jail, err := fsguard.New(root)
	if err != nil {
		t.Fatal(err)
	}
	sha1, err := c.InitialCommit(ctx, root, "chore: initialize product", jail)
	if err != nil || sha1 == "" {
		t.Fatalf("%s %v", sha1, err)
	}
	sha2, err := c.InitialCommit(ctx, root, "chore: initialize product", jail)
	if err != nil {
		t.Fatal(err)
	}
	if sha1 != sha2 {
		t.Fatalf("duplicate initial commit %s vs %s", sha1, sha2)
	}
	msg := runOut(t, root, "git", "log", "--pretty=%s")
	if strings.Count(msg, "chore: initialize product") != 1 {
		t.Fatalf("log=%s", msg)
	}
}

func TestAddRemoteDoesNotOverwrite(t *testing.T) {
	root := initRepo(t)
	c := vcs.Default()
	ctx := context.Background()
	if err := c.AddRemoteIfMissing(ctx, root, "origin", "https://example.com/a.git"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddRemoteIfMissing(ctx, root, "origin", "https://example.com/b.git"); err != nil {
		t.Fatal(err)
	}
	if u := c.RemoteURL(ctx, root, "origin"); u != "https://example.com/a.git" {
		t.Fatalf("url=%s", u)
	}
}

func TestPushRefusesForce(t *testing.T) {
	root := initRepo(t)
	var seen [][]string
	c := &vcs.Client{
		LookPath: exec.LookPath,
		Git: func(ctx context.Context, dir string, args ...string) (string, error) {
			seen = append(seen, append([]string{}, args...))
			return vcs.Default().Git(ctx, dir, args...)
		},
	}
	if err := c.Push(context.Background(), root, "origin", "+main"); err == nil {
		t.Fatal("expected force-ref refusal")
	}
	err := c.Push(context.Background(), root, "origin", "main")
	if err == nil {
		t.Fatal("push without remote should fail")
	}
	for _, args := range seen {
		for _, a := range args {
			if a == "--force" || a == "-f" || a == "--force-with-lease" {
				t.Fatalf("force push args %v", args)
			}
		}
	}
}

func TestEnsureBranchDoesNotRewrite(t *testing.T) {
	root := initRepo(t)
	c := vcs.Default()
	ctx := context.Background()
	if err := c.EnsureBranch(ctx, root, "topic"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("on-topic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jail, _ := fsguard.New(root)
	if _, err := c.Commit(ctx, root, "feat: topic change", []string{"hello.txt"}, jail); err != nil {
		t.Fatal(err)
	}
	topic := runOut(t, root, "git", "rev-parse", "topic")
	if err := c.EnsureBranch(ctx, root, "main"); err != nil && runOut(t, root, "git", "branch", "--show-current") == "" {
		t.Fatal(err)
	}
	_ = c.EnsureBranch(ctx, root, "main")
	if err := c.EnsureBranch(ctx, root, "topic"); err != nil {
		t.Fatal(err)
	}
	if got := runOut(t, root, "git", "rev-parse", "topic"); got != topic {
		t.Fatalf("rewrote topic %s -> %s", topic, got)
	}
}

func TestCheckpointMessageDeterministic(t *testing.T) {
	a := vcs.CheckpointMessage("P1", "Implement Add")
	b := vcs.CheckpointMessage("P1", "Implement Add")
	if a != b || a != "feat: implement P1 Implement Add" {
		t.Fatalf("%q", a)
	}
	if vcs.CheckpointMessage("P3", "fix overflow") != "fix: implement P3 fix overflow" {
		t.Fatal(vcs.CheckpointMessage("P3", "fix overflow"))
	}
}

func TestInitialCommitSkipsSecretsAndProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env"), []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".project"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".project", "state.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := vcs.Default()
	ctx := context.Background()
	if err := c.Init(ctx, root, "main"); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	jail, _ := fsguard.New(root)
	if _, err := c.InitialCommit(ctx, root, "chore: initialize product", jail); err != nil {
		t.Fatal(err)
	}
	files := runOut(t, root, "git", "ls-tree", "-r", "--name-only", "HEAD")
	if strings.Contains(files, ".env") || strings.Contains(files, ".project") {
		t.Fatalf("committed secrets or metadata:\n%s", files)
	}
	if !strings.Contains(files, "ok.txt") {
		t.Fatalf("missing ok.txt:\n%s", files)
	}
}

func runOut(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return strings.TrimSpace(string(out))
}
