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
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func copyContractPRD(t *testing.T, destDir, name string) string {
	t.Helper()
	src := filepath.Join("..", "prd", "testdata", "prd", "contract", name)
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(destDir, "PRD.md")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRejectedPRDDoesNotMutateWorld(t *testing.T) {
	prdDir := t.TempDir()
	prdPath := copyContractPRD(t, prdDir, "reject_missing_objective.md")
	missingRoot := filepath.Join(t.TempDir(), "must-not-be-created")

	gitCalls := 0
	ghCalls := 0
	worker := &recordingWorker{inner: cursor.Fake{ClaimSuccess: true, WriteRel: "x.go", WriteBody: "package x\n"}}
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	eng := engine.New(engine.Options{
		Worker:    worker,
		AllowSelf: true,
		NewID:     seqID(),
		Config:    cfg,
		Git: &vcs.Client{Git: func(ctx context.Context, dir string, args ...string) (string, error) {
			gitCalls++
			t.Fatalf("git must not run on rejected PRD: %v in %s", args, dir)
			return "", nil
		}},
		GH: &vcs.GHClient{
			LookPath: func(string) (string, error) {
				ghCalls++
				t.Fatal("gh must not be looked up on rejected PRD")
				return "", os.ErrNotExist
			},
			GH: func(ctx context.Context, dir string, args ...string) (string, error) {
				ghCalls++
				t.Fatalf("gh must not run on rejected PRD: %v", args)
				return "", nil
			},
		},
	})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: missingRoot, PRDPath: prdPath, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason != "PRD contract validation rejected" {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
	if res.Contract == nil || res.Contract.Status != prd.ContractRejected {
		t.Fatalf("contract=%+v", res.Contract)
	}
	if worker.calls != 0 {
		t.Fatal("cursor worker invoked")
	}
	if gitCalls != 0 || ghCalls != 0 {
		t.Fatalf("git=%d gh=%d", gitCalls, ghCalls)
	}
	if _, err := os.Stat(missingRoot); !os.IsNotExist(err) {
		t.Fatalf("product directory created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prdDir, ".git")); !os.IsNotExist(err) {
		t.Fatal("git repo created beside PRD")
	}
	if _, err := os.Stat(filepath.Join(prdDir, ".project")); !os.IsNotExist(err) {
		t.Fatal("project state created on rejection")
	}

	runRes, err := eng.Run(context.Background(), engine.Request{ProductRoot: missingRoot, PRDPath: prdPath, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(runRes.Execution.RefusalReason, "PRD contract") {
		t.Fatalf("run reason=%q", runRes.Execution.RefusalReason)
	}
	if worker.calls != 0 {
		t.Fatal("run invoked worker")
	}
	if _, err := os.Stat(missingRoot); !os.IsNotExist(err) {
		t.Fatal("run created product directory")
	}
}

func TestValidPRDMayEnterBootstrap(t *testing.T) {
	root := t.TempDir()
	copyContractPRD(t, root, "pass_complete.md")
	eng := engine.New(engine.Options{Worker: panicWorker{}, AllowSelf: true, NewID: seqID()})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason != "" {
		t.Fatalf("valid PRD must enter bootstrap: %s", res.Execution.RefusalReason)
	}
	if res.Execution.PacketRef == "" || res.Execution.Baseline.SHA == "" {
		t.Fatalf("bootstrap incomplete: %+v", res.Execution)
	}
	if res.Execution.Invoked {
		t.Fatal("prepare must not invoke cursor")
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Fatal("expected git bootstrap")
	}
	if _, err := os.Stat(filepath.Join(root, ".project")); err != nil {
		t.Fatal("expected project state")
	}
}
