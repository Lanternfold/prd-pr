package vcs

import (
	"context"
	"strings"
	"testing"
)

func TestRefuseDestructiveGitInvariants(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"push", "--force", "origin", "main"}, "force"},
		{[]string{"push", "-f", "origin", "main"}, "force"},
		{[]string{"push", "--force-with-lease", "origin", "main"}, "force"},
		{[]string{"reset", "--hard", "HEAD~1"}, "reset"},
		{[]string{"reset", "HEAD~1"}, "reset"},
		{[]string{"rebase", "main"}, "rebase"},
		{[]string{"commit", "--amend", "-m", "x"}, "amend"},
		{[]string{"remote", "set-url", "origin", "https://example.com/other.git"}, "overwrite"},
		{[]string{"filter-branch", "--all"}, "rewrite"},
	}
	for _, tc := range cases {
		err := refuseDestructiveGit(tc.args)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
			t.Fatalf("args %v: %v want %q", tc.args, err, tc.want)
		}
	}
	if err := refuseDestructiveGit([]string{"push", "-u", "origin", "prdpr/x"}); err != nil {
		t.Fatalf("normal push refused: %v", err)
	}
	if err := refuseDestructiveGit([]string{"remote", "add", "origin", "https://example.com/a.git"}); err != nil {
		t.Fatalf("remote add refused: %v", err)
	}
}

func TestProtectedBranchNames(t *testing.T) {
	if !protectedBranch("main") || !protectedBranch("master") || protectedBranch("prdpr/x") {
		t.Fatal("protected branch classification")
	}
}

func TestMergePRRefusesRebase(t *testing.T) {
	g := &GHClient{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(_ context.Context, _ string, args ...string) (string, error) {
			t.Fatalf("github called %v", args)
			return "", nil
		},
	}
	res, err := g.MergePR(context.Background(), ".", "1", "rebase")
	if err != nil {
		t.Fatal(err)
	}
	if res.Merged || res.Reason == "" {
		t.Fatalf("%+v", res)
	}
}
