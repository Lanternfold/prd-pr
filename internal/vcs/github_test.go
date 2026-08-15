package vcs

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestOpenPRSkippedWithoutGH(t *testing.T) {
	g := &GHClient{LookPath: func(string) (string, error) { return "", os.ErrNotExist }}
	pr, err := g.OpenPR(context.Background(), t.TempDir(), "t", "b", "main")
	if err != nil {
		t.Fatal(err)
	}
	if !pr.Skipped {
		t.Fatalf("%+v", pr)
	}
}

func TestOpenPRDoesNotMerge(t *testing.T) {
	var args []string
	g := &GHClient{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(_ context.Context, _ string, a ...string) (string, error) {
			args = append(args, a...)
			if len(a) >= 2 && a[1] == "list" {
				return "[]", nil
			}
			if len(a) >= 2 && a[1] == "view" {
				return `{"number":1,"url":"https://example.com/pull/1","headRefName":"feat","baseRefName":"main","headRefOid":"abc","state":"OPEN"}`, nil
			}
			return "https://example.com/pull/1", nil
		},
	}
	pr, err := g.OpenPR(context.Background(), ".", "title", "body", "main")
	if err != nil || pr.URL == "" || pr.Skipped {
		t.Fatalf("%+v %v", pr, err)
	}
	if pr.Number != "1" {
		t.Fatalf("number=%s", pr.Number)
	}
	for _, a := range args {
		if a == "merge" {
			t.Fatal(args)
		}
	}
}

func TestEnsurePRReusesExisting(t *testing.T) {
	var creates int
	g := &GHClient{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(_ context.Context, _ string, a ...string) (string, error) {
			if len(a) >= 2 && a[1] == "list" {
				return `[{"number":7,"url":"https://example.com/pull/7","headRefName":"prdpr/p","baseRefName":"main","headRefOid":"def","state":"OPEN"}]`, nil
			}
			if len(a) >= 2 && a[1] == "create" {
				creates++
			}
			return "", fmt.Errorf("unexpected %v", a)
		},
	}
	pr, err := g.EnsurePR(context.Background(), ".", "t", "b", "main", "prdpr/p")
	if err != nil || pr.Number != "7" || !pr.Reused || creates != 0 {
		t.Fatalf("%+v creates=%d err=%v", pr, creates, err)
	}
}

func TestCreateRepoDefaultsPrivateAndDoesNotGuessName(t *testing.T) {
	var args []string
	g := &GHClient{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(_ context.Context, _ string, a ...string) (string, error) {
			args = a
			return "https://github.com/acme/app", nil
		},
	}
	url, err := g.CreateRepo(context.Background(), RepoSpec{Owner: "acme", Name: "app", Description: "d"})
	if err != nil || url == "" {
		t.Fatalf("%s %v", url, err)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--private") || strings.Contains(joined, "--public") {
		t.Fatalf("visibility args %v", args)
	}
	if _, err := g.CreateRepo(context.Background(), RepoSpec{}); err == nil {
		t.Fatal("must not invent owner/name")
	}
}

func TestParseRepoRef(t *testing.T) {
	o, n := ParseRepoRef("github.com/lanternfold/prd-pr")
	if o != "lanternfold" || n != "prd-pr" {
		t.Fatalf("%s %s", o, n)
	}
	o, n = ParseRepoRef("example/fixture")
	if o != "example" || n != "fixture" {
		t.Fatalf("%s %s", o, n)
	}
	if o, n := ParseRepoRef("fixture"); o != "" || n != "" {
		t.Fatalf("guessed %s/%s", o, n)
	}
}

func TestRepoExistsTreatsViewErrorAsMissing(t *testing.T) {
	g := &GHClient{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(_ context.Context, _ string, args ...string) (string, error) {
			return "", os.ErrNotExist
		},
	}
	ok, _, err := g.RepoExists(context.Background(), "acme", "app")
	if err != nil || ok {
		t.Fatalf("ok=%t err=%v", ok, err)
	}
}
