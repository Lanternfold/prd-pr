package engine_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestExistingLocalGitRepositoryReused(t *testing.T) {
	root := fixtureRepo(t)
	before := gitLog(t, root)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, res.Execution)
	}
	after := gitLog(t, root)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("history changed: before=%v after=%v", before, after)
	}
}

func TestExistingRemoteReusedNotOverwritten(t *testing.T) {
	root := fixtureRepo(t)
	bare := initBare(t)
	run(t, root, "git", "remote", "add", "origin", bare)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if u := vcs.Default().RemoteURL(context.Background(), root, "origin"); u != bare {
		t.Fatalf("remote overwritten: %s", u)
	}
}

func TestGitHubDisabledLocalOnly(t *testing.T) {
	gitAuthor(t)
	root := t.TempDir()
	writePRD(t, root)
	cfg := config.Defaults()
	cfg.GitHubEnabled = false
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), Config: cfg})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, res.Execution)
	}
	st := loadState(t, root)
	if st.Repository.GitHubStatus != state.GitHubDisabled || st.Repository.PushStatus != state.PushSkipped {
		t.Fatalf("%+v", st.Repository)
	}
	if !strings.Contains(events(t, root), "push_skipped") && !strings.Contains(events(t, root), "repo_skipped") {
		t.Fatalf("expected skip event in %s", events(t, root))
	}
	if gitLog(t, root)[0] != "chore: initialize product" {
		t.Fatalf("log=%v", gitLog(t, root))
	}
}

func TestGitHubUnavailableSkipsRemote(t *testing.T) {
	gitAuthor(t)
	root := t.TempDir()
	writePRD(t, root)
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.GitHubOwner = "example"
	cfg.GitHubRepo = "fixture"
	eng := engine.New(engine.Options{
		Worker: panicWorker{},
		NewID:  seqID(),
		Config: cfg,
		GH:     &vcs.GHClient{LookPath: func(string) (string, error) { return "", os.ErrNotExist }},
	})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, res.Execution)
	}
	st := loadState(t, root)
	if st.Repository.GitHubStatus != state.GitHubUnavailable || st.Repository.PushStatus != state.PushSkipped {
		t.Fatalf("%+v", st.Repository)
	}
}

func TestRepositoryCreationAndInitialPush(t *testing.T) {
	gitAuthor(t)
	root := t.TempDir()
	writePRD(t, root)
	bare := initBare(t)
	var creates int
	var views int
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.GitHubOwner = "example"
	cfg.GitHubRepo = "fixture"
	cfg.GitHubVisibility = config.VisibilityPrivate
	real := vcs.Default()
	eng := engine.New(engine.Options{
		Worker: panicWorker{},
		NewID:  seqID(),
		Config: cfg,
		Git: &vcs.Client{
			LookPath: exec.LookPath,
			Git: func(ctx context.Context, dir string, args ...string) (string, error) {
				if len(args) >= 3 && args[0] == "remote" && args[1] == "add" {
					args = []string{"remote", "add", args[2], bare}
				}
				return real.Git(ctx, dir, args...)
			},
		},
		GH: &vcs.GHClient{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(_ context.Context, _ string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "auth" {
					return "logged in", nil
				}
				if len(args) >= 2 && args[1] == "view" {
					views++
					if creates == 0 {
						return "", fmt.Errorf("not found")
					}
					return "https://github.com/example/fixture", nil
				}
				if len(args) >= 2 && args[1] == "create" {
					creates++
					joined := strings.Join(args, " ")
					if !strings.Contains(joined, "--private") {
						t.Fatalf("expected private: %v", args)
					}
					return "https://github.com/example/fixture", nil
				}
				return "", fmt.Errorf("unexpected %v", args)
			},
		},
	})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, res.Execution)
	}
	if creates != 1 {
		t.Fatalf("creates=%d", creates)
	}
	st := loadState(t, root)
	if st.Repository.GitHubStatus != state.GitHubCreated {
		t.Fatalf("%+v", st.Repository)
	}
	if st.Repository.PushStatus != state.PushPushed {
		t.Fatalf("push=%s events=%s", st.Repository.PushStatus, events(t, root))
	}
	// idempotent
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if creates != 1 {
		t.Fatalf("recreated repo creates=%d views=%d", creates, views)
	}
}

func TestCrashAfterRemoteCreationDoesNotRecreate(t *testing.T) {
	gitAuthor(t)
	root := t.TempDir()
	writePRD(t, root)
	var creates int
	addFails := true
	real := vcs.Default()
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.GitHubOwner = "example"
	cfg.GitHubRepo = "fixture"
	opts := engine.Options{
		Worker: panicWorker{},
		NewID:  seqID(),
		Config: cfg,
		Git: &vcs.Client{
			LookPath: exec.LookPath,
			Git: func(ctx context.Context, dir string, args ...string) (string, error) {
				if addFails && len(args) >= 2 && args[0] == "remote" && args[1] == "add" {
					return "", fmt.Errorf("crash after create")
				}
				return real.Git(ctx, dir, args...)
			},
		},
		GH: &vcs.GHClient{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(_ context.Context, _ string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "auth" {
					return "ok", nil
				}
				if len(args) >= 2 && args[1] == "view" {
					if creates == 0 {
						return "", fmt.Errorf("missing")
					}
					return "https://github.com/example/fixture", nil
				}
				if len(args) >= 2 && args[1] == "create" {
					creates++
					return "https://github.com/example/fixture", nil
				}
				return "", fmt.Errorf("unexpected %v", args)
			},
		},
	}
	eng := engine.New(opts)
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason == "" {
		t.Fatal("expected remote-add failure to refuse prepare")
	}
	if creates != 1 {
		t.Fatalf("creates=%d", creates)
	}
	addFails = false
	eng = engine.New(opts)
	res, err = eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("resume: %v %+v", err, res.Execution)
	}
	if creates != 1 {
		t.Fatalf("recreated after crash creates=%d", creates)
	}
}

func TestPhaseVerifiedCreatesCommitThenPush(t *testing.T) {
	root := goGitFixture(t, true)
	bare := initBare(t)
	run(t, root, "git", "remote", "add", "origin", bare)
	before := len(gitLog(t, root))
	cfg := config.Defaults()
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "note.txt", WriteBody: "phase note\n"},
		NewID:     seqID(),
		AllowSelf: true,
		Config:    cfg,
		SkipWait:  true,
	})
	res, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || !res.Completed {
		t.Fatalf("%v %+v", err, res)
	}
	log := gitLog(t, root)
	if len(log) != before+1 {
		t.Fatalf("log=%v", log)
	}
	if !strings.HasPrefix(log[0], "feat: implement P1") {
		t.Fatalf("message=%q", log[0])
	}
	st := loadState(t, root)
	if st.Repository.PhaseCheckpoints["P1"] == "" {
		t.Fatalf("%+v", st.Repository)
	}
	if st.Repository.PushStatus != state.PushPushed {
		t.Fatalf("push=%s", st.Repository.PushStatus)
	}
}

func TestFailedVerificationDoesNotCommit(t *testing.T) {
	root := goGitFixture(t, false)
	before := gitLog(t, root)
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "bad.txt", WriteBody: "x\n"},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
	})
	res, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Completed || res.Verification.VerifiedSuccess {
		t.Fatalf("%+v", res)
	}
	after := gitLog(t, root)
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("committed failed work: %v", after)
	}
}

func TestManualAcceptancePendingNoCommit(t *testing.T) {
	root := goGitFixturePRD(t, true, "minimal_valid.md")
	before := gitLog(t, root)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, SkipWait: true})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if v.VerifiedSuccess {
		t.Fatal("manual must not verify")
	}
	if strings.Join(before, "\n") != strings.Join(gitLog(t, root), "\n") {
		t.Fatal("committed while waiting for human")
	}
}

func TestRepairPendingNoCommit(t *testing.T) {
	root := goGitFixture(t, false)
	before := gitLog(t, root)
	cfg := config.Defaults()
	cfg.AutoCommit = true
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "x.txt", WriteBody: "x\n"},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
	})
	if _, err := eng.Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.PrepareRepair(context.Background(), engine.Request{ProductRoot: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CommitVerified(context.Background(), root, "feat: should fail"); err == nil {
		t.Fatal("commit during repair")
	}
	if strings.Join(before, "\n") != strings.Join(gitLog(t, root), "\n") {
		t.Fatal("repair pending created a commit")
	}
}

func TestPushFailureRecorded(t *testing.T) {
	root := goGitFixture(t, true)
	real := vcs.Default()
	cfg := config.Defaults()
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "note.txt", WriteBody: "n\n"},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
		Git: &vcs.Client{
			LookPath: exec.LookPath,
			Git: func(ctx context.Context, dir string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "push" {
					return "", fmt.Errorf("simulated push failure")
				}
				if len(args) >= 2 && args[0] == "remote" && args[1] == "get-url" {
					return "https://github.com/example/fixture.git", nil
				}
				if len(args) >= 2 && args[0] == "rev-parse" && strings.Contains(args[1], "/") {
					return "", fmt.Errorf("no upstream")
				}
				return real.Git(ctx, dir, args...)
			},
		},
	})
	res, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || !res.Completed {
		t.Fatalf("local complete should succeed: %v %+v", err, res)
	}
	st := loadState(t, root)
	if st.Repository.PushStatus != state.PushFailed || st.CurrentState != state.StateCompleted {
		t.Fatalf("state=%s repo=%+v", st.CurrentState, st.Repository)
	}
	if !strings.Contains(events(t, root), "push_failed") {
		t.Fatal(events(t, root))
	}
}

func TestCrashAfterCommitBeforePush(t *testing.T) {
	root := goGitFixture(t, true)
	real := vcs.Default()
	failPush := true
	cfg := config.Defaults()
	opts := engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "note.txt", WriteBody: "n\n"},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
		Git: &vcs.Client{
			LookPath: exec.LookPath,
			Git: func(ctx context.Context, dir string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "push" {
					if failPush {
						return "", fmt.Errorf("crash-before-push")
					}
					return "", nil
				}
				if len(args) >= 2 && args[0] == "remote" && args[1] == "get-url" {
					return "https://example.com/r.git", nil
				}
				if len(args) >= 2 && args[0] == "rev-parse" && strings.Contains(args[1], "/") {
					if failPush {
						return "", fmt.Errorf("no upstream")
					}
					sha, _ := real.Git(ctx, dir, "rev-parse", "HEAD")
					return sha, nil
				}
				return real.Git(ctx, dir, args...)
			},
		},
	}
	eng := engine.New(opts)
	if _, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if loadState(t, root).Repository.PushStatus != state.PushFailed {
		t.Fatal(loadState(t, root).Repository)
	}
	failPush = false
	eng = engine.New(opts)
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root}); err != nil {
		t.Fatal(err)
	}
	if loadState(t, root).Repository.PushStatus != state.PushPushed {
		t.Fatalf("%+v events=%s", loadState(t, root).Repository, events(t, root))
	}
}

func TestRepeatedPrepareIsIdempotent(t *testing.T) {
	gitAuthor(t)
	root := t.TempDir()
	writePRD(t, root)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID()})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	first := gitLog(t, root)
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	second := gitLog(t, root)
	if strings.Join(first, "\n") != strings.Join(second, "\n") {
		t.Fatalf("duplicate commits %v vs %v", first, second)
	}
}

func TestHistoryPreservedAndNoForcePush(t *testing.T) {
	root := fixtureRepo(t)
	var seen [][]string
	real := vcs.Default()
	eng := engine.New(engine.Options{
		Worker: panicWorker{},
		NewID:  seqID(),
		Git: &vcs.Client{
			LookPath: exec.LookPath,
			Git: func(ctx context.Context, dir string, args ...string) (string, error) {
				seen = append(seen, append([]string{}, args...))
				return real.Git(ctx, dir, args...)
			},
		},
	})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if gitLog(t, root)[len(gitLog(t, root))-1] != "init" && gitLog(t, root)[len(gitLog(t, root))-1] != "chore: initialize product" {
		if got := gitLog(t, root); got[len(got)-1] != "init" {
			t.Fatalf("lost history %v", got)
		}
	}
	for _, args := range seen {
		for _, a := range args {
			if a == "--force" || a == "-f" || a == "--hard" {
				t.Fatalf("destructive git %v", args)
			}
		}
	}
}

func TestMultiplePhasesLogicalCommits(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	cfg := config.Defaults()
	seq := &cursor.Sequence{Steps: []cursor.Fake{
		{ClaimSuccess: true, WriteRel: "p1.txt", WriteBody: "one\n"},
		{ClaimSuccess: true, WriteRel: "p2.txt", WriteBody: "two\n"},
	}}
	eng := engine.New(engine.Options{Worker: seq, NewID: seqID(), AllowSelf: true, SkipWait: true, Config: cfg})
	r1, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || !r1.Completed {
		t.Fatalf("p1 %v %+v", err, r1)
	}
	r2, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P2"})
	if err != nil || !r2.Completed {
		t.Fatalf("p2 %v %+v", err, r2)
	}
	log := gitLog(t, root)
	var feats []string
	for _, m := range log {
		if strings.HasPrefix(m, "feat: implement P") {
			feats = append(feats, m)
		}
	}
	if len(feats) != 2 {
		t.Fatalf("want 2 phase commits, log=%v", log)
	}
	if feats[0] == feats[1] {
		t.Fatal("messages should be phase-specific")
	}
}

func gitAuthor(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

func gitLog(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "log", "--pretty=%s")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log: %v\n%s", err, out)
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func initBare(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init", "--bare", "--template=")
	return dir
}

func loadState(t *testing.T, root string) state.State {
	t.Helper()
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func events(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".project", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
