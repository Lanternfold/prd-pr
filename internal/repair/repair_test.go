package repair

import (
	"testing"
	"time"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/diagnose"
	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/review"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func TestThreeAttemptCap(t *testing.T) {
	cfg := config.Defaults()
	inc := Incident{ID: "i1"}
	now := time.Unix(1, 0)
	for i := 0; i < 3; i++ {
		if !CanAttempt(inc, cfg) {
			t.Fatalf("should allow attempt %d", i+1)
		}
		inc = BeginAttempt(inc, now)
		inc = FinishAttempt(inc, i+1, "sha", nil, false, "fail", now)
	}
	if CanAttempt(inc, cfg) || !inc.Exhausted || inc.MaxAttempts != 3 {
		t.Fatalf("%+v", inc)
	}
	inc2 := Incident{ID: "i1"}
	for i := 0; i < 3; i++ {
		inc2 = RecordAttempt(inc2, "sha", nil, false, "fail", now)
	}
	if CanAttempt(inc2, cfg) || !inc2.Exhausted {
		t.Fatalf("%+v", inc2)
	}
}

func TestRepairPacketBounded(t *testing.T) {
	orig := packet.Packet{
		SchemaVersion: 1, TaskID: "t", ProjectID: "p", PhaseID: "P1",
		Objective: "add", ProductRoot: "/tmp/x", ExpectedOutputs: []string{"add.go"},
	}
	inc := Incident{ID: "i1"}
	rev := review.Result{Diagnosis: diagnose.Report{Summary: "TestAdd failed", RecommendedScope: []string{"add.go"}}}
	p := NewPacket(inc, orig, rev, testeng.Result{Status: testeng.StatusFailed}, "diff")
	if p.Attempt != 1 {
		t.Fatalf("%+v", p)
	}
	if _, err := Marshal(p); err != nil {
		t.Fatal(err)
	}
}

func TestRewindUsesGraph(t *testing.T) {
	g := graph.FromSpecs([]graph.Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []graph.NodeID{"P1"}},
		{ID: "P3"},
	})
	plan, err := RewindTarget(g, "P1", diagnose.OriginLocalPhase)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Affected) != 1 || plan.Affected[0] != "P2" {
		t.Fatalf("%+v", plan)
	}
	if len(plan.ReplayOrder) != 2 {
		t.Fatalf("replay=%v", plan.ReplayOrder)
	}
}
