package graph

import "testing"

func TestTopoRespectsEdges(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P1"}},
		{ID: "P4", Deps: []NodeID{"P2", "P3"}},
	})
	pos := map[NodeID]int{}
	for i, id := range g.SequentialOrder() {
		pos[id] = i
	}
	for _, e := range g.Edges {
		if pos[e.From] >= pos[e.To] {
			t.Fatalf("edge %s → %s violated topo order %v", e.From, e.To, g.SequentialOrder())
		}
	}
}

func TestAffectedHasPath(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P2"}},
		{ID: "P4", Deps: []NodeID{"P1"}},
	})
	adj := g.outAdj()
	for _, id := range g.Affected("P1") {
		if !reachable(adj, "P1", id) {
			t.Fatalf("affected %s is not reachable from P1", id)
		}
	}
	if reachable(adj, "P2", "P4") {
		t.Fatal("P4 must not be reachable from P2")
	}
}

func TestReplayDepsBeforeDependents(t *testing.T) {
	g := FromSpecs([]Spec{
		{ID: "P1"},
		{ID: "P2", Deps: []NodeID{"P1"}},
		{ID: "P3", Deps: []NodeID{"P2"}},
		{ID: "P4", Deps: []NodeID{"P2"}},
	})
	plan, err := g.Rewind("P2")
	if err != nil {
		t.Fatal(err)
	}
	pos := map[NodeID]int{}
	for i, id := range plan.ReplayOrder {
		pos[id] = i
	}
	inReplay := map[NodeID]bool{}
	for _, id := range plan.ReplayOrder {
		inReplay[id] = true
	}
	for _, e := range g.Edges {
		if !inReplay[e.From] || !inReplay[e.To] {
			continue
		}
		if pos[e.From] >= pos[e.To] {
			t.Fatalf("replay order %v violates %s → %s", plan.ReplayOrder, e.From, e.To)
		}
	}
}

func reachable(adj map[NodeID][]NodeID, from, to NodeID) bool {
	seen := map[NodeID]bool{}
	var walk func(NodeID) bool
	walk = func(u NodeID) bool {
		for _, v := range adj[u] {
			if v == to {
				return true
			}
			if seen[v] {
				continue
			}
			seen[v] = true
			if walk(v) {
				return true
			}
		}
		return false
	}
	return walk(from)
}
