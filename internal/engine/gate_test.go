package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestCommitRefusedBeforeVerify(t *testing.T) {
	root := goGitFixture(t, true)
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CommitVerified(context.Background(), root, "x"); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("commit before verify: %v", err)
	}
}

func TestCommitRefusedAfterFailedVerify(t *testing.T) {
	root := goGitFixture(t, false)
	cfg := config.Defaults()
	cfg.AutoCommit = false
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.CommitVerified(context.Background(), root, "x"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "verif") {
		t.Fatalf("commit after failed verify: %v", err)
	}
}

func TestPRRefusedBeforeVerify(t *testing.T) {
	root := goGitFixture(t, true)
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	eng := engine.New(engine.Options{
		AllowSelf: true,
		Config:    cfg,
		GH:        &vcs.GHClient{LookPath: func(string) (string, error) { return "/bin/gh", nil }},
	})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.OpenMilestonePR(context.Background(), root, "t", "b"); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("PR before verify: %v", err)
	}
}

func TestCommitAndPRAllowedAfterVerified(t *testing.T) {
	root := goGitFixture(t, true)
	run(t, root, "git", "remote", "add", "origin", initBare(t))
	cfg := config.Defaults()
	cfg.AutoCommit = false
	cfg.GitHubEnabled = true
	var ghArgs []string
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "extra.txt", WriteBody: "verified extra\n"},
		NewID:     seqID(),
		AllowSelf: true,
		Config:    cfg,
		GH: &vcs.GHClient{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(_ context.Context, _ string, args ...string) (string, error) {
				ghArgs = append(ghArgs, args...)
				if len(args) > 0 && args[0] == "auth" {
					return "ok", nil
				}
				if len(args) >= 2 && args[1] == "list" {
					return "[]", nil
				}
				if len(args) >= 2 && args[1] == "view" {
					return `{"number":1,"url":"https://example.com/pull/1","headRefName":"main","baseRefName":"main","headRefOid":"abc","state":"OPEN"}`, nil
				}
				return "https://example.com/pull/1", nil
			},
		},
	})
	if _, err := eng.Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !vres.VerifiedSuccess {
		t.Fatalf("need verified: %+v", vres)
	}
	sha, err := eng.CommitVerified(context.Background(), root, "prdpr: verified changes")
	if err != nil || sha == "" {
		t.Fatalf("commit after verify: %v %s", err, sha)
	}
	pr, err := eng.OpenMilestonePR(context.Background(), root, "t", "b")
	if err != nil || pr.Skipped || pr.URL == "" {
		t.Fatalf("PR after verify: %+v %v", pr, err)
	}
	for _, a := range ghArgs {
		if a == "merge" {
			t.Fatal(ghArgs)
		}
	}
}

func TestAllPhasesCompletedDoesNotReplayFirstPhase(t *testing.T) {
	root := goGitFixture(t, true)
	cfg := config.Defaults()
	cfg.AutoCommit = false
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	vres, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root})
	if err != nil || !vres.VerifiedSuccess {
		t.Fatalf("verify: %v %+v", err, vres)
	}
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ProjectCompleted {
		t.Fatalf("expected PROJECT_COMPLETED, got refusal=%q packet=%s", res.Execution.RefusalReason, res.Execution.PacketRef)
	}
	if res.Execution.PacketRef != "" {
		t.Fatal("must not prepare a new packet")
	}
	st, err := state.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if snap.ProjectStatus != state.StatusProjectCompleted || snap.CurrentState != state.StateCompleted {
		t.Fatalf("status=%s state=%s", snap.ProjectStatus, snap.CurrentState)
	}
	if _, err := os.Stat(filepath.Join(root, res.Execution.PacketRef)); res.Execution.PacketRef != "" && err == nil {
		t.Fatal("unexpected packet")
	}
}
