package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/apprun"
	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/llm"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func TestOpenFromPRDCreatesStudioProjectAndPrepares(t *testing.T) {
	gitAuthor(t)
	studio := fakeStudio(t)
	prdPath := copyNamedPRD(t, t.TempDir(), "pass_complete.md")
	cfg := config.Defaults()
	cfg.StudioRoot = studio
	cfg.GitHubEnabled = false
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Home: t.TempDir(), Cwd: t.TempDir()})
	res, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil || res.WaitingForHuman || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v human=%v", err, res.Execution, res.Human)
	}
	if res.ProjectType != "go_library" {
		t.Fatalf("type=%s", res.ProjectType)
	}
	dest := filepath.Join(studio, "Tools", "notes-library")
	if res.ProjectLocation != dest {
		t.Fatalf("loc=%s want %s", res.ProjectLocation, dest)
	}
	if _, err := os.Stat(filepath.Join(dest, "PRD.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatal(err)
	}
	rulesPath := filepath.Join(dest, ".cursor", "rules", "prdpr-engineering.mdc")
	rules, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rules), "PRD→PR execution policy") || !strings.Contains(string(rules), "do not grant terminal permissions") {
		t.Fatalf("bootstrapped rules missing execution policy:\n%s", rules)
	}
	if _, err := os.Stat(filepath.Join(dest, ".cursor", "permissions.json")); err == nil {
		t.Fatal("bootstrap must not write .cursor/permissions.json")
	}
	if res.Execution.PacketRef == "" || res.Execution.Baseline.SHA == "" {
		t.Fatalf("%+v", res.Execution)
	}
	st := loadState(t, dest)
	if st.ProjectType != "go_library" || st.ProjectLocation != dest {
		t.Fatalf("%+v", st)
	}
	// idempotent
	log1 := gitLog(t, dest)
	res2, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil || res2.Execution.RefusalReason != "" {
		t.Fatalf("second: %v %+v", err, res2.Execution)
	}
	log2 := gitLog(t, dest)
	if strings.Join(log1, "\n") != strings.Join(log2, "\n") {
		t.Fatalf("history changed %v vs %v", log1, log2)
	}
}

func TestOpenFromPRDConflictUnrelatedDirectory(t *testing.T) {
	gitAuthor(t)
	studio := fakeStudio(t)
	dest := filepath.Join(studio, "Tools", "notes-library")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "other.txt"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prdPath := copyNamedPRD(t, t.TempDir(), "pass_complete.md")
	cfg := config.Defaults()
	cfg.StudioRoot = studio
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), Config: cfg, Home: t.TempDir(), Cwd: t.TempDir()})
	res, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !res.WaitingForHuman || res.Human == nil || res.Human.Kind != human.KindDirectoryConflict {
		t.Fatalf("%+v", res)
	}
}

func TestOpenFromPRDAmbiguousStudioAsksHuman(t *testing.T) {
	prdPath := copyNamedPRD(t, t.TempDir(), "pass_complete.md")
	cfg := config.Defaults()
	cfg.StudioRoot = t.TempDir() // not a Studio
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), Config: cfg, Home: t.TempDir(), Cwd: t.TempDir()})
	res, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !res.WaitingForHuman || res.Human == nil || res.Human.Kind != human.KindStudioPlacement {
		t.Fatalf("%+v", res)
	}
}

func TestOpenFromPRDGitHubCreateAndReuse(t *testing.T) {
	gitAuthor(t)
	studio := fakeStudio(t)
	prdPath := copyNamedPRD(t, t.TempDir(), "pass_complete.md")
	var creates, posts int
	cfg := config.Defaults()
	cfg.StudioRoot = studio
	cfg.GitHubEnabled = true
	cfg.GitHubOwner = "example"
	cfg.GitHubRepo = "notes-library"
	eng := engine.New(engine.Options{
		Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Home: t.TempDir(), Cwd: t.TempDir(),
		GH: fakeGH(&creates, &posts),
	})
	res, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, res.Execution)
	}
	if creates != 1 || posts != 1 {
		t.Fatalf("creates=%d posts=%d", creates, posts)
	}
	st := loadState(t, res.ProjectLocation)
	if st.Repository.GitHubStatus != state.GitHubCreated || st.Repository.RulesetStatus != state.RulesetCreated {
		t.Fatalf("%+v", st.Repository)
	}
	res2, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil || res2.Execution.RefusalReason != "" {
		t.Fatalf("reuse %v %+v", err, res2.Execution)
	}
	if creates != 1 || posts != 1 {
		t.Fatalf("recreated creates=%d posts=%d", creates, posts)
	}
}

func TestOpenFromPRDGitHubAuthRequestsHumanAndResumes(t *testing.T) {
	gitAuthor(t)
	studio := fakeStudio(t)
	prdPath := copyNamedPRD(t, t.TempDir(), "pass_complete.md")
	cfg := config.Defaults()
	cfg.StudioRoot = studio
	cfg.GitHubEnabled = true
	cfg.GitHubOwner = "example"
	cfg.GitHubRepo = "notes-library"
	auth := false
	var creates int
	gh := &vcs.GHClient{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(_ context.Context, _ string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "auth" {
				if !auth {
					return "", os.ErrNotExist
				}
				return "ok", nil
			}
			if len(args) >= 2 && args[1] == "view" {
				if creates == 0 {
					return "", os.ErrNotExist
				}
				return "https://github.com/example/notes-library", nil
			}
			if len(args) >= 2 && args[1] == "create" {
				creates++
				return "https://github.com/example/notes-library", nil
			}
			if len(args) > 0 && args[0] == "api" {
				if strings.Contains(strings.Join(args, " "), "POST") {
					return `{"id":1,"name":"prdpr-baseline"}`, nil
				}
				return "[]", nil
			}
			return "", os.ErrNotExist
		},
	}
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Home: t.TempDir(), Cwd: t.TempDir(), GH: gh, SkipWait: true})
	res, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !res.WaitingForHuman || res.Human == nil || res.Human.Kind != human.KindGitHubAuth {
		t.Fatalf("%+v", res)
	}
	dest := filepath.Join(studio, "Tools", "notes-library")
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatal("git state must be preserved")
	}
	auth = true
	if err := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Home: t.TempDir(), Cwd: t.TempDir(), GH: gh}).Feedback(dest, res.Human.ID, "authenticated", human.StatusConfirmed, ""); err != nil {
		t.Fatal(err)
	}
	if err := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Home: t.TempDir(), Cwd: t.TempDir(), GH: gh, SkipWait: true}).Resume(context.Background(), dest); err != nil {
		t.Fatal(err)
	}
	st := loadState(t, dest)
	if st.Repository.GitHubStatus != state.GitHubCreated && st.Repository.GitHubStatus != state.GitHubExists {
		t.Fatalf("after resume %+v", st.Repository)
	}
}

func TestRulesetConflictAsksHuman(t *testing.T) {
	gitAuthor(t)
	studio := fakeStudio(t)
	prdPath := copyNamedPRD(t, t.TempDir(), "pass_complete.md")
	cfg := config.Defaults()
	cfg.StudioRoot = studio
	cfg.GitHubEnabled = true
	cfg.GitHubOwner = "example"
	cfg.GitHubRepo = "notes-library"
	eng := engine.New(engine.Options{
		Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Home: t.TempDir(), Cwd: t.TempDir(), SkipWait: true,
		GH: &vcs.GHClient{
			LookPath: func(string) (string, error) { return "/bin/gh", nil },
			GH: func(_ context.Context, _ string, args ...string) (string, error) {
				if len(args) > 0 && args[0] == "auth" {
					return "ok", nil
				}
				if len(args) >= 2 && args[1] == "view" {
					return "", os.ErrNotExist
				}
				if len(args) >= 2 && args[1] == "create" {
					return "https://github.com/example/notes-library", nil
				}
				if len(args) > 0 && args[0] == "api" {
					if strings.Contains(strings.Join(args, " "), "POST") {
						t.Fatal("must not overwrite conflicting rulesets")
					}
					return `[{"id":2,"name":"prdpr-baseline","enforcement":"active","rules":[{"type":"pull_request","parameters":{"required_approving_review_count":0}}]}]`, nil
				}
				return "", os.ErrNotExist
			},
		},
	})
	res, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !res.WaitingForHuman || res.Human == nil || res.Human.Kind != human.KindRulesetConflict {
		t.Fatalf("%+v human=%+v", res.Execution, res.Human)
	}
}

func TestCursorRulesPreservedOnBootstrap(t *testing.T) {
	gitAuthor(t)
	studio := fakeStudio(t)
	dest := filepath.Join(studio, "Tools", "notes-library")
	if err := os.MkdirAll(filepath.Join(dest, ".cursor", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := filepath.Join(dest, ".cursor", "rules", "prdpr-engineering.mdc")
	if err := os.WriteFile(custom, []byte("# user rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prdPath := copyNamedPRD(t, t.TempDir(), "pass_complete.md")
	cfg := config.Defaults()
	cfg.StudioRoot = studio
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Home: t.TempDir(), Cwd: t.TempDir()})
	if _, err := eng.OpenFromPRD(context.Background(), prdPath); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(custom)
	if string(b) != "# user rules\n" {
		t.Fatalf("%q", b)
	}
	if _, err := os.Stat(filepath.Join(dest, ".cursor", "permissions.json")); err == nil {
		t.Fatal("bootstrap must not write .cursor/permissions.json")
	}
}

func TestCompletenessBlockingQuestion(t *testing.T) {
	prdPath := copyNamedPRD(t, t.TempDir(), "pass_complete.md")
	cfg := config.Defaults()
	cfg.StudioRoot = fakeStudio(t)
	eng := engine.New(engine.Options{
		Worker: panicWorker{}, NewID: seqID(), Config: cfg, Home: t.TempDir(), Cwd: t.TempDir(),
		LLM: llm.Static{Text: `{"findings":[{"severity":"BLOCKING_QUESTION","topic":"onboarding","question":"What happens on first launch?"}]}`},
	})
	res, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !res.WaitingForHuman || res.Human == nil || res.Human.Kind != human.KindProductQuestion {
		t.Fatalf("%+v", res)
	}
	if _, err := os.Stat(filepath.Join(cfg.StudioRoot, "Tools", "notes-library")); !os.IsNotExist(err) {
		t.Fatal("must not create project before blocking product questions")
	}
}

func TestRuntimeSuccessFailureRepairExhaustion(t *testing.T) {
	gitAuthor(t)
	root := t.TempDir()
	copyNamedPRD(t, root, "pass_complete.md")
	cfg := config.Defaults()
	cfg.GitHubEnabled = false
	fake := &apprun.Fake{Ready: true, URL: "http://127.0.0.1:9"}
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Runtime: fake})
	if _, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"}); err != nil {
		t.Fatal(err)
	}
	st := loadState(t, root)
	st.ProjectType = "go_cli"
	if err := mustSaveState(t, root, st); err != nil {
		t.Fatal(err)
	}
	_ = apprun.Save(root, apprun.Def{Kind: apprun.KindGoRun, Command: "go", Args: []string{"run", "."}})
	rep, err := eng.StartRuntime(context.Background(), root)
	if err != nil || !rep.Ready {
		t.Fatalf("%+v %v", rep, err)
	}

	fail := &apprun.Fake{Ready: false, Error: "boom"}
	engFail := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true, Config: cfg, Runtime: fail, SkipWait: true})
	rep, err = engFail.StartRuntime(context.Background(), root)
	if err != nil || rep.Ready {
		t.Fatalf("%+v %v", rep, err)
	}
	pkt, _, err := engFail.RepairRuntime(context.Background(), root)
	if err != nil || pkt.Attempt != 1 {
		t.Fatalf("repair1 %v %+v", err, pkt)
	}
	pkt, _, err = engFail.RepairRuntime(context.Background(), root)
	if err != nil || pkt.Attempt != 2 {
		t.Fatalf("repair2 %v %+v", err, pkt)
	}
	pkt, _, err = engFail.RepairRuntime(context.Background(), root)
	if err != nil || pkt.Attempt != 3 {
		t.Fatalf("repair3 %v %+v", err, pkt)
	}
	_, _, err = engFail.RepairRuntime(context.Background(), root)
	if err == nil {
		t.Fatal("expected exhaustion")
	}
}

func TestRejectedContractStillBlocksOpenFromPRD(t *testing.T) {
	prdPath := copyNamedPRD(t, t.TempDir(), "reject_missing_objective.md")
	studio := fakeStudio(t)
	cfg := config.Defaults()
	cfg.StudioRoot = studio
	eng := engine.New(engine.Options{Worker: panicWorker{}, Config: cfg, Home: t.TempDir(), Cwd: t.TempDir()})
	res, err := eng.OpenFromPRD(context.Background(), prdPath)
	if err != nil {
		t.Fatal(err)
	}
	if res.Contract == nil || res.Contract.Status != prd.ContractRejected {
		t.Fatalf("%+v", res)
	}
	entries, _ := os.ReadDir(filepath.Join(studio, "Tools"))
	if len(entries) != 0 {
		t.Fatalf("created %v", entries)
	}
}

func fakeStudio(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, c := range []string{"Tools", "Products", "Experiments"} {
		if err := os.Mkdir(filepath.Join(root, c), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func copyNamedPRD(t *testing.T, dir, name string) string {
	t.Helper()
	src := filepath.Join("..", "prd", "testdata", "prd", "contract", name)
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "PRD.md")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeGH(creates, posts *int) *vcs.GHClient {
	return &vcs.GHClient{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(_ context.Context, _ string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "auth" {
				return "ok", nil
			}
			if len(args) >= 2 && args[1] == "view" {
				if *creates == 0 {
					return "", os.ErrNotExist
				}
				return "https://github.com/example/notes-library", nil
			}
			if len(args) >= 2 && args[1] == "create" {
				*creates++
				return "https://github.com/example/notes-library", nil
			}
			if len(args) > 0 && args[0] == "api" {
				if strings.Contains(strings.Join(args, " "), "POST") {
					*posts++
					return `{"id":1,"name":"prdpr-baseline","rules":[{"type":"non_fast_forward"},{"type":"deletion"},{"type":"pull_request"}]}`, nil
				}
				if *posts > 0 {
					return `[{"id":1,"name":"prdpr-baseline","rules":[{"type":"non_fast_forward"},{"type":"deletion"},{"type":"pull_request","parameters":{"required_approving_review_count":0}}]}]`, nil
				}
				return "[]", nil
			}
			return "", os.ErrNotExist
		},
	}
}

func mustSaveState(t *testing.T, root string, st state.State) error {
	t.Helper()
	s, err := state.Open(root)
	if err != nil {
		return err
	}
	return s.Save(st)
}
