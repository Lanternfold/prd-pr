package engine_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/ci"
	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestEvaluateMergeGates(t *testing.T) {
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoMergeEnabled = true
	cfg.RequireApproval = true
	st := state.State{CurrentState: state.StateCompleted, Repository: state.Repository{FeatureBranch: "prdpr/p", BaseBranch: "main", PRNumber: "1"}}
	ex := engine.Execution{VerifiedSuccess: true}
	pr := vcs.PR{Number: "1", State: "open", Head: "prdpr/p", Base: "main", Mergeable: "MERGEABLE", ReviewDecision: "APPROVED"}
	pass := ci.Report{Status: ci.StatusPassing}
	dec := engine.EvaluateMerge(cfg, st, ex, pr, pass, nil)
	if !dec.Allow {
		t.Fatalf("expected allow %+v", dec)
	}

	off := cfg
	off.AutoMergeEnabled = false
	if d := engine.EvaluateMerge(off, st, ex, pr, pass, nil); d.Allow || d.Status != state.PRWaitingForMerge {
		t.Fatalf("disabled %+v", d)
	}
	if d := engine.EvaluateMerge(cfg, st, ex, pr, ci.Report{Status: ci.StatusFailing}, nil); d.Allow || !strings.Contains(d.Reason, "FAIL") {
		t.Fatalf("fail ci %+v", d)
	}
	if d := engine.EvaluateMerge(cfg, st, ex, pr, ci.Report{Status: ci.StatusPending}, nil); d.Allow || !strings.Contains(d.Reason, "PENDING") {
		t.Fatalf("pending ci %+v", d)
	}
	if d := engine.EvaluateMerge(cfg, st, ex, pr, ci.Report{Status: ci.StatusUnknown}, nil); d.Allow || !strings.Contains(d.Reason, "UNKNOWN") {
		t.Fatalf("unknown ci %+v", d)
	}
	noap := pr
	noap.ReviewDecision = "REVIEW_REQUIRED"
	if d := engine.EvaluateMerge(cfg, st, ex, noap, pass, nil); d.Allow || !strings.Contains(d.Reason, "approval") {
		t.Fatalf("approval %+v", d)
	}
	ex2 := ex
	ex2.VerifiedSuccess = false
	if d := engine.EvaluateMerge(cfg, st, ex2, pr, pass, nil); d.Allow {
		t.Fatal("unverified")
	}
}

func TestBranchCreatedForGitHubPRWorkflow(t *testing.T) {
	root := goGitFixture(t, true)
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoCommit = false
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, res.Execution)
	}
	st := loadState(t, root)
	if st.Repository.FeatureBranch == "" || !strings.HasPrefix(st.Repository.FeatureBranch, "prdpr/") {
		t.Fatalf("%+v", st.Repository)
	}
}

func TestLocalOnlySkipsPRAndMerge(t *testing.T) {
	root := goGitFixture(t, true)
	cfg := config.Defaults()
	cfg.GitHubEnabled = false
	cfg.AutoCommit = false
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil || !v.VerifiedSuccess {
		t.Fatal(err)
	}
	pr, err := eng.OpenMilestonePR(context.Background(), root, "t", "b")
	if err != nil || !pr.Skipped {
		t.Fatalf("%+v %v", pr, err)
	}
	dec, _, err := eng.TryMerge(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allow {
		t.Fatal("local-only must not merge")
	}
}

func TestGitHubUnavailableBlocksPR(t *testing.T) {
	root := goGitFixture(t, true)
	run(t, root, "git", "remote", "add", "origin", initBare(t))
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoCommit = false
	eng := engine.New(engine.Options{
		Worker:    panicWorker{},
		NewID:     seqID(),
		AllowSelf: true,
		Config:    cfg,
		GH:        &vcs.GHClient{LookPath: func(string) (string, error) { return "", os.ErrNotExist }},
	})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil || !v.VerifiedSuccess {
		t.Fatal(err)
	}
	pr, err := eng.OpenMilestonePR(context.Background(), root, "t", "b")
	if err != nil {
		t.Fatal(err)
	}
	if !pr.Skipped || !strings.Contains(pr.Reason, state.GitHubActionBlocked) {
		t.Fatalf("%+v", pr)
	}
}

func TestAutoMergeSuccessAndCleanup(t *testing.T) {
	root := goGitFixture(t, true)
	run(t, root, "git", "remote", "add", "origin", initBare(t))
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoCommit = false
	cfg.AutoMergeEnabled = true
	cfg.DeleteBranchAfterMerge = true
	var merged bool
	real := vcs.Default()
	eng := engine.New(engine.Options{
		Worker:    cursor.Fake{ClaimSuccess: true, WriteRel: "extra.txt", WriteBody: "ok\n"},
		NewID:     seqID(),
		AllowSelf: true,
		Config:    cfg,
		Git: &vcs.Client{
			LookPath: exec.LookPath,
			Git: func(ctx context.Context, dir string, args ...string) (string, error) {
				if len(args) > 0 && (args[0] == "fetch" || (args[0] == "push" && containsArg(args, "--delete"))) {
					return "", nil
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
				if len(args) >= 2 && args[1] == "list" {
					return `[{"number":3,"url":"https://example.com/pull/3","headRefName":"prdpr/x","baseRefName":"main","headRefOid":"abc","state":"OPEN"}]`, nil
				}
				if len(args) >= 2 && args[1] == "view" {
					return `{"number":3,"url":"https://example.com/pull/3","headRefName":"prdpr/x","baseRefName":"main","headRefOid":"abc","state":"OPEN","mergeable":"MERGEABLE","reviewDecision":"APPROVED"}`, nil
				}
				if len(args) >= 2 && args[1] == "merge" {
					merged = true
					return "aabbccd", nil
				}
				return "https://example.com/pull/3", nil
			},
		},
		CI: &ci.Watcher{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(context.Context, string, ...string) (string, error) {
				return `[{"name":"test","state":"SUCCESS","bucket":"pass"}]`, nil
			},
		},
	})
	if _, err := eng.Run(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil || !v.VerifiedSuccess {
		t.Fatal(err)
	}
	st := loadState(t, root)
	st.Repository.FeatureBranch = "prdpr/x"
	st.Repository.BaseBranch = "main"
	st.Repository.PRNumber = "3"
	store, _ := state.Open(root)
	_ = store.Save(st)
	dec, res, err := eng.TryMerge(context.Background(), root)
	if err != nil || !dec.Allow || !merged {
		t.Fatalf("dec=%+v res=%+v err=%v", dec, res, err)
	}
	if loadState(t, root).Repository.MergeStatus != state.MergeMerged {
		t.Fatalf("%+v", loadState(t, root).Repository)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
