package graph

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/prd"
)

func TestSingleNode(t *testing.T) {
	g := FromSpecs([]Spec{{ID: "P0", Name: "only"}})
	if g.HasErrors() {
		t.Fatalf("%v", g.Diagnostics)
	}
	if len(g.Nodes) != 1 || len(g.Edges) != 0 {
		t.Fatalf("nodes=%d edges=%d", len(g.Nodes), len(g.Edges))
	}
	if got := g.SequentialOrder(); strings.Join(ids(got), ",") != "P0" {
		t.Fatalf("order=%v", got)
	}
	if got := g.Ready(); strings.Join(ids(got), ",") != "P0" {
		t.Fatalf("ready=%v", got)
	}
}

func TestAllCompletedHasNoReadyPhase(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1", Status: StatusCompleted},
		{ID: "P2", Deps: []NodeID{"P1"}, Status: StatusCompleted},
	})
	if !g.AllCompleted() {
		t.Fatal("expected all completed")
	}
	if len(g.Ready()) != 0 {
		t.Fatalf("ready=%v", g.Ready())
	}
	if join(g.SequentialOrder()) != "P1,P2" {
		t.Fatalf("DAG order must remain %v", g.SequentialOrder())
	}
}

func TestLinearGraph(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1", Name: "a"},
		{ID: "P2", Name: "b", Deps: []NodeID{"P1"}},
		{ID: "P3", Name: "c", Deps: []NodeID{"P2"}},
		{ID: "P4", Name: "d", Deps: []NodeID{"P3"}},
	})
	if g.HasErrors() {
		t.Fatal(g.Diagnostics)
	}
	if join(g.SequentialOrder()) != "P1,P2,P3,P4" {
		t.Fatalf("order=%v", g.SequentialOrder())
	}
	if join(g.Ready()) != "P1" {
		t.Fatalf("ready=%v", g.Ready())
	}
	if join(g.Blocked()) != "P2,P3,P4" {
		t.Fatalf("blocked=%v", g.Blocked())
	}
}

func TestBranchingAndMerge(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P1"}},
		{ID: "P4", Deps: []NodeID{"P2", "P3"}},
	})
	if g.HasErrors() {
		t.Fatal(g.Diagnostics)
	}
	order := join(g.SequentialOrder())
	if order != "P1,P2,P3,P4" {
		t.Fatalf("order=%s", order)
	}
	if join(g.Affected("P1")) != "P2,P3,P4" {
		t.Fatalf("affected P1=%v", g.Affected("P1"))
	}
}

func TestIndependentNodes(t *testing.T) {
	g := FromSpecs([]Spec{{ID: "P2"}, {ID: "P0"}, {ID: "P1"}})
	if join(g.SequentialOrder()) != "P0,P1,P2" {
		t.Fatalf("order=%v (numeric ID tie-break)", g.SequentialOrder())
	}
	if join(g.Ready()) != "P0,P1,P2" {
		t.Fatalf("ready=%v", g.Ready())
	}
	if join(g.Independent()) != "P0,P1,P2" {
		t.Fatalf("independent=%v", g.Independent())
	}
	if !hasCode(g, CodeNoExplicitDeps) {
		t.Fatalf("expected INFO no explicit deps: %v", g.Diagnostics)
	}
}

func TestDuplicateNodeID(t *testing.T) {
	g := FromSpecs([]Spec{{ID: "P2"}, {ID: "P2"}})
	if !hasCode(g, CodeDuplicateNode) {
		t.Fatalf("%v", g.Diagnostics)
	}
}

func TestUnknownDependency(t *testing.T) {
	g := FromSpecs([]Spec{{ID: "P1", Deps: []NodeID{"P9"}}})
	if !hasCode(g, CodeUnknownDependency) {
		t.Fatalf("%v", g.Diagnostics)
	}
}

func TestSelfDependency(t *testing.T) {
	g := FromSpecs([]Spec{{ID: "P1", Deps: []NodeID{"P1"}}})
	if !hasCode(g, CodeSelfDependency) {
		t.Fatalf("%v", g.Diagnostics)
	}
}

func TestSimpleCycle(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1", Deps: []NodeID{"P3"}},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P2"}},
	})
	if !hasCode(g, CodeCycle) {
		t.Fatalf("%v", g.Diagnostics)
	}
	msg := cycleMsg(g)
	if !strings.Contains(msg, "→") || !strings.Contains(msg, "P1") {
		t.Fatalf("cycle message should identify path: %q", msg)
	}
	if g.SequentialOrder() != nil {
		t.Fatal("topo order must be empty on cycle")
	}
}

func TestComplexCycle(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P0"},
		{ID: "P1", Deps: []NodeID{"P0", "P4"}},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P2"}},
		{ID: "P4", Deps: []NodeID{"P3"}},
		{ID: "P5", Deps: []NodeID{"P0"}},
	})
	if !hasCode(g, CodeCycle) {
		t.Fatalf("%v", g.Diagnostics)
	}
	if !strings.Contains(cycleMsg(g), "→") {
		t.Fatalf("expected cycle path, got %q", cycleMsg(g))
	}
}

func TestDeterministicTopoTieBreak(t *testing.T) {
	a := FromSpecs([]Spec{
		{ID: "P3", Deps: []NodeID{"P1"}},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P1"},
	})
	b := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P1"}},
	})
	if join(a.SequentialOrder()) != join(b.SequentialOrder()) {
		t.Fatalf("%v vs %v", a.SequentialOrder(), b.SequentialOrder())
	}
	if join(a.SequentialOrder()) != "P1,P2,P3" {
		t.Fatalf("got %v", a.SequentialOrder())
	}
}

func TestReadyBlockedFailedWaiting(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P2"}},
	})
	n, _ := g.Node("P2")
	if len(n.BlockedReasons) == 0 || n.BlockedReasons[0].Code != BlockDepIncomplete {
		t.Fatalf("P2 reasons=%v", n.BlockedReasons)
	}
	n, _ = g.Node("P3")
	if n.Status != StatusBlocked || n.BlockedReasons[0].Code != BlockDepBlocked {
		t.Fatalf("P3 should be blocked by blocked P2: %+v", n)
	}
	if err := g.SetStatus("P1", StatusFailed); err != nil {
		t.Fatal(err)
	}
	n, _ = g.Node("P2")
	if n.Status != StatusBlocked || n.BlockedReasons[0].Code != BlockDepFailed {
		t.Fatalf("failed dep: %+v", n)
	}
	if err := g.SetStatus("P1", StatusWaitingForHuman); err != nil {
		t.Fatal(err)
	}
	n, _ = g.Node("P2")
	if n.BlockedReasons[0].Code != BlockDepWaiting {
		t.Fatalf("waiting dep: %+v", n)
	}
	if err := g.SetStatus("P1", StatusCompleted); err != nil {
		t.Fatal(err)
	}
	if join(g.Ready()) != "P2" {
		t.Fatalf("ready after P1 complete=%v", g.Ready())
	}
}

func TestDownstreamAndRewind(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P2"}},
		{ID: "P4", Deps: []NodeID{"P3"}},
	})
	if join(g.Affected("P2")) != "P3,P4" {
		t.Fatalf("affected=%v", g.Affected("P2"))
	}
	plan, err := g.Rewind("P2")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Origin != "P2" || join(plan.Affected) != "P3,P4" || join(plan.ReplayOrder) != "P2,P3,P4" {
		t.Fatalf("%+v", plan)
	}
}

func TestIndependentBranchUnaffected(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P4", Deps: []NodeID{"P2"}},
		{ID: "P3", Deps: []NodeID{"P1"}},
		{ID: "P5", Deps: []NodeID{"P3"}},
	})
	if join(g.Affected("P2")) != "P4" {
		t.Fatalf("affected P2=%v (must not include P3/P5)", g.Affected("P2"))
	}
	plan, _ := g.Rewind("P2")
	if join(plan.ReplayOrder) != "P2,P4" {
		t.Fatalf("replay=%v", plan.ReplayOrder)
	}
}

func TestSerializationRoundTrip(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
	})
	raw, err := g.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	g2, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if join(g.SequentialOrder()) != join(g2.SequentialOrder()) {
		t.Fatalf("%v vs %v", g.SequentialOrder(), g2.SequentialOrder())
	}
	if !json.Valid(raw) {
		t.Fatal("invalid json")
	}
	raw2, _ := g.Marshal()
	if string(raw) != string(raw2) {
		t.Fatal("marshal not deterministic")
	}
}

func TestRepeatedCalculationIdentical(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P1"}},
	})
	a1, a2 := join(g.Affected("P1")), join(g.Affected("P1"))
	o1, o2 := join(g.SequentialOrder()), join(g.SequentialOrder())
	r1, r2 := join(g.Ready()), join(g.Ready())
	if a1 != a2 || o1 != o2 || r1 != r2 {
		t.Fatal("non-deterministic")
	}
}

func TestInvalidNodeID(t *testing.T) {
	g := FromSpecs([]Spec{{ID: "nope"}})
	if !hasCode(g, CodeInvalidNode) {
		t.Fatalf("%v", g.Diagnostics)
	}
}

func TestFromDocumentExplicitDepsOnly(t *testing.T) {
	doc := &prd.Document{
		Phases: []prd.Phase{
			{ID: "P1", Name: "one"},
			{ID: "P2", Name: "two"},
		},
	}
	g := FromDocument(doc)
	if len(g.Edges) != 0 {
		t.Fatalf("must not invent edges: %v", g.Edges)
	}
	doc.Phases[1].Dependencies = []prd.PhaseID{"P1"}
	g = FromDocument(doc)
	if len(g.Edges) != 1 || g.Edges[0].From != "P1" || g.Edges[0].To != "P2" {
		t.Fatalf("edges=%v", g.Edges)
	}
}

func TestRepoPRDGraph(t *testing.T) {
	p := filepath.Join("..", "..", "PRD.md")
	doc, err := prd.ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	g := FromDocument(doc)
	if len(g.Nodes) != 14 {
		t.Fatalf("nodes=%d want 14", len(g.Nodes))
	}
	if len(g.Edges) != 26 {
		t.Fatalf("edges=%d want 26 (explicit PRD deps only); edges=%v", len(g.Edges), g.Edges)
	}
	if g.HasErrors() {
		t.Fatalf("PRD graph must be acyclic and valid: %v", g.Diagnostics)
	}
	if cycleMsg(g) != "" {
		t.Fatalf("cycle: %s", cycleMsg(g))
	}

	wantEdges := []Edge{
		{From: "P0", To: "P1"},
		{From: "P1", To: "P2"},
		{From: "P0", To: "P3"},
		{From: "P1", To: "P3"},
		{From: "P0", To: "P4"},
		{From: "P1", To: "P4"},
		{From: "P0", To: "P5"},
		{From: "P5", To: "P6"},
		{From: "P0", To: "P7"},
		{From: "P1", To: "P7"},
		{From: "P0", To: "P8"},
		{From: "P1", To: "P8"},
		{From: "P2", To: "P9"},
		{From: "P4", To: "P9"},
		{From: "P7", To: "P9"},
		{From: "P0", To: "P10"},
		{From: "P0", To: "P11"},
		{From: "P2", To: "P12"},
		{From: "P1", To: "P13"},
		{From: "P2", To: "P13"},
		{From: "P3", To: "P13"},
		{From: "P4", To: "P13"},
		{From: "P5", To: "P13"},
		{From: "P7", To: "P13"},
		{From: "P8", To: "P13"},
		{From: "P9", To: "P13"},
	}
	if len(wantEdges) != 26 {
		t.Fatalf("fixture edge list %d != 26", len(wantEdges))
	}
	for _, e := range wantEdges {
		if !hasEdge(g, e.From, e.To) {
			t.Fatalf("missing explicit edge %s → %s", e.From, e.To)
		}
	}
	if hasEdge(g, "P5", "P4") {
		t.Fatal("must not infer Cursor←Git from numbering; P4 does not depend on P5")
	}
	if hasEdge(g, "P3", "P4") {
		t.Fatal("must not infer Cursor←Preflight as a build edge")
	}

	order := g.SequentialOrder()
	if join(order) != "P0,P1,P2,P3,P4,P5,P6,P7,P8,P9,P10,P11,P12,P13" {
		t.Fatalf("topo=%v", order)
	}
	if join(g.Ready()) != "P0" {
		t.Fatalf("ready=%v want P0", g.Ready())
	}
	blocked := g.Blocked()
	if len(blocked) != 13 {
		t.Fatalf("blocked=%v want 13 downstream nodes", blocked)
	}
	for _, id := range blocked {
		if id == "P0" {
			t.Fatal("P0 must not be blocked")
		}
	}

	if err := g.SetStatus("P0", StatusCompleted); err != nil {
		t.Fatal(err)
	}
	if join(g.Ready()) != "P1,P5,P10,P11" {
		t.Fatalf("parallel after P0=%v want P1,P5,P10,P11", g.Ready())
	}
	if join(g.ParallelCandidates()) != join(g.Ready()) {
		t.Fatal("parallel candidates should match ready set")
	}
}

func ids(in []NodeID) []string {
	out := make([]string, len(in))
	for i, id := range in {
		out[i] = string(id)
	}
	return out
}

func join(in []NodeID) string {
	return strings.Join(ids(in), ",")
}

func hasCode(g *Graph, code string) bool {
	for _, d := range g.Diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

func hasEdge(g *Graph, from, to NodeID) bool {
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			return true
		}
	}
	return false
}

func cycleMsg(g *Graph) string {
	for _, d := range g.Diagnostics {
		if d.Code == CodeCycle {
			return d.Message
		}
	}
	return ""
}
