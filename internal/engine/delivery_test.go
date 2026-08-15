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
	if d := engine.EvaluateMerge(cfg, st, ex, pr, ci.Report{Status: ci.StatusPending}, nil); d.Allow || d.Status != state.PRChecking || !strings.Contains(d.Reason, "PENDING") {
		t.Fatalf("pending ci %+v", d)
	}
	if d := engine.EvaluateMerge(cfg, st, ex, pr, ci.Report{Status: ci.StatusUnknown}, nil); d.Allow || d.Status != state.PRWaitingForMerge || !strings.Contains(d.Reason, "UNKNOWN") {
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

	missing := ci.Report{Status: ci.StatusPassing, Checks: []ci.Check{{Name: "test", Bucket: "pass"}}}
	need := cfg
	need.RequiredChecks = []string{"ci"}
	if d := engine.EvaluateMerge(need, st, ex, pr, missing, nil); d.Allow || !strings.Contains(d.Reason, "missing") {
		t.Fatalf("missing required %+v", d)
	}
	failed := ci.Report{Status: ci.StatusPassing, Checks: []ci.Check{{Name: "ci", Bucket: "fail"}}}
	if d := engine.EvaluateMerge(need, st, ex, pr, failed, nil); d.Allow || !strings.Contains(d.Reason, "failed") {
		t.Fatalf("failed required %+v", d)
	}
	okreq := ci.Report{Status: ci.StatusPassing, Checks: []ci.Check{{Name: "ci", Bucket: "pass"}}}
	if d := engine.EvaluateMerge(need, st, ex, pr, okreq, nil); !d.Allow || d.Status != state.PRReadyToMerge {
		t.Fatalf("required pass %+v", d)
	}
	if d := engine.EvaluateMerge(cfg, st, ex, pr, ci.Report{Status: ci.StatusSkipped}, nil); d.Allow {
		t.Fatalf("skipped ci %+v", d)
	}
	rb := cfg
	rb.MergeMethod = config.MergeRebase
	if d := engine.EvaluateMerge(rb, st, ex, pr, pass, nil); d.Allow || !strings.Contains(d.Reason, "rebase") {
		t.Fatalf("rebase %+v", d)
	}
}

func TestBranchCreatedForGitHubPRWorkflow(t *testing.T) {
	root := goGitFixture(t, true)
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoCommit = false
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, SkipWait: true, Config: cfg})
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
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, SkipWait: true, Config: cfg})
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
		SkipWait:  true,
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
		SkipWait:  true,
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
	got := loadState(t, root)
	if got.Repository.MergeStatus != state.PRMerged {
		t.Fatalf("%+v", got.Repository)
	}
	if got.Repository.PRNumber != "3" || got.Repository.MergeSHA == "" || got.Repository.MergeMethod == "" || got.Repository.MergeAt == "" || got.Repository.MergeBranch == "" {
		t.Fatalf("merge record incomplete %+v", got.Repository)
	}
}

func TestDuplicatePRIsReused(t *testing.T) {
	root := goGitFixture(t, true)
	run(t, root, "git", "remote", "add", "origin", initBare(t))
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoCommit = false
	var creates int
	eng := engine.New(engine.Options{
		Worker:    panicWorker{},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
		GH: &vcs.GHClient{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(_ context.Context, _ string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "auth" {
					return "ok", nil
				}
				if len(args) >= 2 && args[1] == "list" {
					return `[{"number":1,"url":"https://example.com/pull/1","headRefName":"prdpr/x","baseRefName":"main","headRefOid":"abc","state":"OPEN"}]`, nil
				}
				if len(args) >= 2 && args[1] == "create" {
					creates++
				}
				return "", nil
			},
		},
	})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil || !v.VerifiedSuccess {
		t.Fatal(err)
	}
	pr1, err := eng.OpenMilestonePR(context.Background(), root, "t", "b")
	if err != nil || pr1.Number != "1" || !pr1.Reused {
		t.Fatalf("%+v %v", pr1, err)
	}
	pr2, err := eng.OpenMilestonePR(context.Background(), root, "t", "b")
	if err != nil || pr2.Number != "1" || creates != 0 {
		t.Fatalf("duplicate %+v creates=%d err=%v", pr2, creates, err)
	}
	if loadState(t, root).Repository.MergeStatus != state.PROpen {
		t.Fatalf("%+v", loadState(t, root).Repository)
	}
}

func TestInspectChecksPersistsPendingAndUnknown(t *testing.T) {
	root := goGitFixture(t, true)
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	eng := engine.New(engine.Options{
		Worker:    panicWorker{},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
		GH:        &vcs.GHClient{LookPath: func(string) (string, error) { return "", os.ErrNotExist }},
		CI: &ci.Watcher{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(context.Context, string, ...string) (string, error) {
				return `[{"name":"test","state":"PENDING","bucket":"pending"}]`, nil
			},
		},
	})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	st := loadState(t, root)
	st.Repository.PRNumber = "1"
	store, _ := state.Open(root)
	_ = store.Save(st)
	rep := eng.InspectChecks(context.Background(), root)
	if rep.Verdict() != ci.VerdictPending {
		t.Fatalf("%+v", rep)
	}
	if loadState(t, root).Repository.MergeStatus != state.PRChecking {
		t.Fatalf("%+v", loadState(t, root).Repository)
	}
	if loadState(t, root).Repository.ChecksStatus != ci.VerdictPending {
		t.Fatalf("checks=%s", loadState(t, root).Repository.ChecksStatus)
	}
}

func TestReconcileMergeAndPRWithoutRepeating(t *testing.T) {
	root := goGitFixture(t, true)
	run(t, root, "git", "remote", "add", "origin", initBare(t))
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoCommit = false
	cfg.AutoMergeEnabled = true
	var merges int
	var creates int
	eng := engine.New(engine.Options{
		Worker:    panicWorker{},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
		GH: &vcs.GHClient{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(_ context.Context, _ string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "auth" {
					return "ok", nil
				}
				if len(args) >= 2 && args[1] == "list" {
					return `[{"number":9,"url":"https://example.com/pull/9","headRefName":"prdpr/p","baseRefName":"main","headRefOid":"deadbeef","state":"OPEN"}]`, nil
				}
				if len(args) >= 2 && args[1] == "view" {
					return `{"number":9,"url":"https://example.com/pull/9","headRefName":"prdpr/p","baseRefName":"main","headRefOid":"cafebabe","state":"MERGED","mergeable":"MERGEABLE","reviewDecision":"APPROVED"}`, nil
				}
				if len(args) >= 2 && args[1] == "create" {
					creates++
				}
				if len(args) >= 2 && args[1] == "merge" {
					merges++
				}
				return "", nil
			},
		},
		CI: &ci.Watcher{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(context.Context, string, ...string) (string, error) {
				return `[{"name":"test","state":"SUCCESS","bucket":"pass"}]`, nil
			},
		},
	})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil || !v.VerifiedSuccess {
		t.Fatal(err)
	}
	st := loadState(t, root)
	st.CurrentState = state.StateCompleted
	st.Repository.FeatureBranch = "prdpr/p"
	st.Repository.BaseBranch = "main"
	store, _ := state.Open(root)
	_ = store.Save(st)

	if err := eng.Reconcile(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	got := loadState(t, root)
	if got.Repository.PRNumber != "9" {
		t.Fatalf("did not recover PR %+v", got.Repository)
	}
	if got.Repository.MergeStatus != state.PRMerged || got.Repository.MergeSHA == "" {
		t.Fatalf("did not recover merge %+v", got.Repository)
	}
	if merges != 0 || creates != 0 {
		t.Fatalf("repeated github ops merges=%d creates=%d", merges, creates)
	}

	dec, res, err := eng.TryMerge(context.Background(), root)
	if err != nil || dec.Allow || res.Merged == false && got.Repository.MergeStatus != state.PRMerged {
		t.Fatalf("retry merge dec=%+v res=%+v err=%v", dec, res, err)
	}
	if merges != 0 {
		t.Fatal("merge after crash replayed")
	}
}

func TestAutoMergeDisabledDoesNotMerge(t *testing.T) {
	root := goGitFixture(t, true)
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoCommit = false
	if cfg.AutoMergeEnabled {
		t.Fatal("default must keep AutoMergeEnabled false")
	}
	var merged bool
	eng := engine.New(engine.Options{
		Worker:    panicWorker{},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
		GH: &vcs.GHClient{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(_ context.Context, _ string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "auth" {
					return "ok", nil
				}
				if len(args) >= 2 && args[1] == "view" {
					return `{"number":1,"url":"https://example.com/pull/1","headRefName":"prdpr/p","baseRefName":"main","headRefOid":"abc","state":"OPEN","mergeable":"MERGEABLE"}`, nil
				}
				if len(args) >= 2 && args[1] == "merge" {
					merged = true
				}
				return "", nil
			},
		},
		CI: &ci.Watcher{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(context.Context, string, ...string) (string, error) {
				return `[{"name":"test","state":"SUCCESS","bucket":"pass"}]`, nil
			},
		},
	})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil || !v.VerifiedSuccess {
		t.Fatal(err)
	}
	st := loadState(t, root)
	st.Repository.PRNumber = "1"
	st.Repository.FeatureBranch = "prdpr/p"
	st.Repository.BaseBranch = "main"
	store, _ := state.Open(root)
	_ = store.Save(st)
	dec, _, err := eng.TryMerge(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allow || merged || dec.Status != state.PRWaitingForMerge {
		t.Fatalf("%+v merged=%t", dec, merged)
	}
}

func TestProductWorkDoesNotPushBaseBranch(t *testing.T) {
	root := goGitFixture(t, true)
	run(t, root, "git", "remote", "add", "origin", initBare(t))
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	cfg.AutoCommit = false
	eng := engine.New(engine.Options{
		Worker:    panicWorker{},
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
		GH:        &vcs.GHClient{LookPath: func(string) (string, error) { return "", os.ErrNotExist }},
	})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	if v, err := eng.Verify(context.Background(), engine.Request{ProductRoot: root}); err != nil || !v.VerifiedSuccess {
		t.Fatal(err)
	}
	st := loadState(t, root)
	st.CurrentState = state.StateCompleted
	st.Repository.FeatureBranch = "main"
	st.Repository.BaseBranch = "main"
	st.Repository.Branch = "main"
	st.Repository.PushStatus = state.PushPending
	store, _ := state.Open(root)
	_ = store.Save(st)
	if err := eng.Reconcile(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if loadState(t, root).Repository.PushStatus != state.PushFailed {
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
