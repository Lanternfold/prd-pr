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
	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/state"
)

func TestPrepareRejectsBlockedPhase(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.RefusalReason == "" || !strings.Contains(res.Execution.RefusalReason, "not READY") {
		t.Fatalf("want BLOCKED/not READY refusal, got %q", res.Execution.RefusalReason)
	}
	if res.Execution.PacketRef != "" && res.Execution.Invoked {
		t.Fatal("must not prepare a blocked phase")
	}
	g := loadTestGraph(t, root)
	n, ok := g.Node("P2")
	if !ok || n.Status != graph.StatusBlocked {
		t.Fatalf("P2 status=%s ok=%t", n.Status, ok)
	}
}

func TestPrepareRejectsUnknownPhase(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P9"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Execution.RefusalReason, "unknown phase") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestPrepareMarksSelectedPhaseRunning(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	eng := engine.New(engine.Options{Worker: panicWorker{}, NewID: seqID(), AllowSelf: true})
	res, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root})
	if err != nil || res.Execution.RefusalReason != "" {
		t.Fatalf("%v %+v", err, res.Execution)
	}
	if res.Execution.PhaseID != "P1" {
		t.Fatalf("phase=%s", res.Execution.PhaseID)
	}
	g := loadTestGraph(t, root)
	p1, _ := g.Node("P1")
	p2, _ := g.Node("P2")
	if p1.Status != graph.StatusRunning {
		t.Fatalf("P1=%s", p1.Status)
	}
	if p2.Status != graph.StatusBlocked {
		t.Fatalf("P2=%s", p2.Status)
	}
}

func TestRunPhaseRejectsBlockedPhase(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	eng := graphWalkEngine(t, root, cursor.Fake{ClaimSuccess: true, WriteRel: "p2.txt", WriteBody: "nope\n"})
	res, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P2"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Completed || res.Execution.Invoked {
		t.Fatalf("blocked phase must not run: %+v", res)
	}
	if !strings.Contains(res.Execution.RefusalReason, "not READY") {
		t.Fatalf("reason=%q", res.Execution.RefusalReason)
	}
}

func TestRunGraphWalksDependencyChain(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	seq := &cursor.Sequence{Steps: []cursor.Fake{
		{ClaimSuccess: true, WriteRel: "p1.txt", WriteBody: "one\n"},
		{ClaimSuccess: true, WriteRel: "p2.txt", WriteBody: "two\n"},
	}}
	eng := graphWalkEngine(t, root, seq)
	res, err := eng.RunGraph(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Completed || !res.ProjectCompleted {
		t.Fatalf("completed=%v project=%v waiting=%v reason=%q phases=%v", res.Completed, res.ProjectCompleted, res.Waiting, res.Execution.RefusalReason, res.Phases)
	}
	if strings.Join(res.Phases, ",") != "P1,P2" {
		t.Fatalf("phases=%v", res.Phases)
	}
	g := loadTestGraph(t, root)
	if !g.AllCompleted() {
		t.Fatal("graph should be AllCompleted")
	}
	if len(g.Ready()) != 0 {
		t.Fatalf("ready=%v", g.Ready())
	}
}

func TestRunGraphIndependentReadyPhasesAreSequential(t *testing.T) {
	root := graphWalkFixture(t, true, independentPRD)
	prepEng := graphWalkEngine(t, root, panicWorker{})
	prep, err := prepEng.Prepare(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if prep.Execution.RefusalReason != "" {
		t.Fatal(prep.Execution.RefusalReason)
	}
	g := loadTestGraph(t, root)
	p1, _ := g.Node("P1")
	p2, _ := g.Node("P2")
	if p1.Status != graph.StatusRunning || p2.Status != graph.StatusReady {
		t.Fatalf("after prepare P1=%s P2=%s ready=%s", p1.Status, p2.Status, ids(g.Ready()))
	}

	seq := &cursor.Sequence{Steps: []cursor.Fake{
		{ClaimSuccess: true, WriteRel: "a.txt", WriteBody: "a\n"},
		{ClaimSuccess: true, WriteRel: "b.txt", WriteBody: "b\n"},
	}}
	res, err := graphWalkEngine(t, root, seq).RunGraph(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(res.Phases, ",") != "P1,P2" {
		t.Fatalf("want deterministic P1 then P2, got %v waiting=%v reason=%q", res.Phases, res.Waiting, res.Execution.RefusalReason)
	}
	if !res.ProjectCompleted {
		t.Fatalf("want project completed, got %+v", res)
	}
}

func TestPhaseCompletionSelectsNextReady(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	eng := graphWalkEngine(t, root, cursor.Fake{ClaimSuccess: true, WriteRel: "p1.txt", WriteBody: "one\n"})
	res, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || !res.Completed {
		t.Fatalf("p1 %v completed=%v reason=%q", err, res.Completed, res.Execution.RefusalReason)
	}
	g := loadTestGraph(t, root)
	p1, _ := g.Node("P1")
	p2, _ := g.Node("P2")
	if p1.Status != graph.StatusCompleted {
		t.Fatalf("P1=%s", p1.Status)
	}
	if p2.Status != graph.StatusReady {
		t.Fatalf("P2 want READY got %s", p2.Status)
	}
	if join := ids(g.Ready()); join != "P2" {
		t.Fatalf("ready=%s", join)
	}

	eng2 := graphWalkEngine(t, root, cursor.Fake{ClaimSuccess: true, WriteRel: "p2.txt", WriteBody: "two\n"})
	res, err = eng2.RunGraph(context.Background(), engine.Request{ProductRoot: root})
	if err != nil || !res.Completed {
		t.Fatalf("p2 %v %+v", err, res)
	}
	if strings.Join(res.Phases, ",") != "P2" {
		t.Fatalf("next phase=%v", res.Phases)
	}
}

func TestRunGraphAllCompletedStopsScheduling(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	seq := &cursor.Sequence{Steps: []cursor.Fake{
		{ClaimSuccess: true, WriteRel: "p1.txt", WriteBody: "one\n"},
		{ClaimSuccess: true, WriteRel: "p2.txt", WriteBody: "two\n"},
	}}
	eng := graphWalkEngine(t, root, seq)
	res, err := eng.RunGraph(context.Background(), engine.Request{ProductRoot: root})
	if err != nil || !res.ProjectCompleted {
		t.Fatalf("%v %+v", err, res)
	}
	again, err := eng.RunGraph(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if again.Execution.Invoked {
		t.Fatal("must not schedule another phase after AllCompleted")
	}
	if !again.ProjectCompleted {
		t.Fatalf("want project completed, got reason=%q phases=%v", again.Execution.RefusalReason, again.Phases)
	}
	g := loadTestGraph(t, root)
	if !g.AllCompleted() {
		t.Fatal("graph changed after extra walk")
	}
}

func TestRunGraphWaitingForHumanDoesNotAdvance(t *testing.T) {
	root := goGitFixturePRD(t, false, "two_phase.md")
	eng := graphWalkEngine(t, root, cursor.Fake{ClaimSuccess: true, WriteRel: "add.go", WriteBody: "package fixture\n\nfunc Add(a, b int) int { return 0 }\n"})
	res, err := eng.RunGraph(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if res.Completed || !res.Waiting {
		t.Fatalf("want waiting, completed=%v waiting=%v phases=%v reason=%q", res.Completed, res.Waiting, res.Phases, res.Execution.RefusalReason)
	}
	if strings.Join(res.Phases, ",") != "P1" {
		t.Fatalf("must not start P2: %v", res.Phases)
	}
	g := loadTestGraph(t, root)
	p1, _ := g.Node("P1")
	p2, _ := g.Node("P2")
	if p1.Status != graph.StatusWaitingForHuman {
		t.Fatalf("P1=%s", p1.Status)
	}
	if p2.Status != graph.StatusBlocked {
		t.Fatalf("P2=%s", p2.Status)
	}
	snap := loadState(t, root)
	if snap.CurrentState != state.StateWaitingForHuman || snap.CurrentPhaseID != "P1" {
		t.Fatalf("state=%s phase=%s", snap.CurrentState, snap.CurrentPhaseID)
	}

	again, err := eng.RunGraph(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if again.Execution.Invoked || strings.Contains(strings.Join(again.Phases, ","), "P2") && again.Completed {
		t.Fatalf("must not advance while waiting: %+v", again)
	}
}

func TestPrepareRejectsCompletedPhase(t *testing.T) {
	root := goGitFixturePRD(t, true, "two_phase.md")
	eng := graphWalkEngine(t, root, cursor.Fake{ClaimSuccess: true, WriteRel: "p1.txt", WriteBody: "one\n"})
	res, err := eng.RunPhase(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil || !res.Completed {
		t.Fatalf("%v %+v", err, res)
	}
	prep, err := eng.Prepare(context.Background(), engine.Request{ProductRoot: root, PhaseID: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prep.Execution.RefusalReason, "not READY") {
		t.Fatalf("reason=%q", prep.Execution.RefusalReason)
	}
}

func graphWalkEngine(t *testing.T, root string, worker cursor.Worker) *engine.Engine {
	t.Helper()
	cfg := config.Defaults()
	return engine.New(engine.Options{
		Worker:    worker,
		NewID:     seqID(),
		AllowSelf: true,
		SkipWait:  true,
		Config:    cfg,
	})
}

func graphWalkFixture(t *testing.T, pass bool, prdBody string) string {
	t.Helper()
	root := t.TempDir()
	writeString(t, filepath.Join(root, "go.mod"), "module fixture\n\ngo 1.22\n")
	writeString(t, filepath.Join(root, "add.go"), "package fixture\n\nfunc Add(a, b int) int { return a + b }\n")
	want := "4"
	if !pass {
		want = "5"
	}
	writeString(t, filepath.Join(root, "add_test.go"), "package fixture\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 2) != "+want+" { t.Fatal(Add(2,2)) }\n}\n")
	writeString(t, filepath.Join(root, "PRD.md"), prdBody)
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "commit", "-m", "init")
	return root
}

func loadTestGraph(t *testing.T, root string) *graph.Graph {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".project", "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := graph.Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	g.Refresh()
	return g
}

func ids(in []graph.NodeID) string {
	var s []string
	for _, id := range in {
		s = append(s, string(id))
	}
	return strings.Join(s, ",")
}

const independentPRD = `# PRD: Two Libraries

**Status:** Draft
**Product:** Two Libraries
**Owner:** Test
**Repository:** example/fixture

# 1. Product Overview

Two independent helpers.

# 2. Goals

- Ship Add

# 3. Non-Goals

- Multiply

# 4. Requirements

- REQ-001: The app must expose Add(a, b int) int
- REQ-002: Add remains covered

# 5. Acceptance Criteria

- AC-001: must expose Add(a, b int)
- AC-002: must expose Add(a, b int)

# 6. Testing

- TEST-001: unit test Add

# 7. Phases

## P1: Add

Objective: Implement Add
Dependencies:
Requirements: REQ-001
Acceptance Criteria: AC-001
Tests: TEST-001
Definition of Done:
- tests pass

## P2: Keep covered

Objective: Keep Add covered
Dependencies:
Requirements: REQ-002
Acceptance Criteria: AC-002
Tests: TEST-001
Definition of Done:
- tests pass
`
