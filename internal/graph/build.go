package graph

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/lanternfold/prd-pr/internal/prd"
)

// FromDocument builds a phase-level DAG from a parsed PRD.
// Only explicit Phase.Dependencies become edges. Phase numbers are not edges.
func FromDocument(doc *prd.Document) *Graph {
	if doc == nil {
		g := &Graph{SchemaVersion: SchemaVersion}
		g.Diagnostics = append(g.Diagnostics, Diagnostic{
			Severity: SevError,
			Code:     CodeMalformed,
			Message:  "parsed PRD is nil",
		})
		return g
	}
	specs := make([]Spec, 0, len(doc.Phases))
	for _, p := range doc.Phases {
		specs = append(specs, Spec{
			ID:   NodeID(p.ID),
			Name: p.Name,
			Deps: phaseDeps(p),
		})
	}
	g := FromSpecs(specs)
	for i := range g.Nodes {
		if ph, ok := phaseByID(doc, prd.PhaseID(g.Nodes[i].ID)); ok {
			g.Nodes[i].PhaseID = ph.ID
			g.Nodes[i].Source = ph.Source
			if g.Nodes[i].Name == "" {
				g.Nodes[i].Name = ph.Name
			}
		}
	}
	return g
}

func phaseDeps(p prd.Phase) []NodeID {
	out := make([]NodeID, 0, len(p.Dependencies))
	seen := map[NodeID]bool{}
	for _, d := range p.Dependencies {
		id := NodeID(d)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func phaseByID(doc *prd.Document, id prd.PhaseID) (prd.Phase, bool) {
	for _, p := range doc.Phases {
		if p.ID == id {
			return p, true
		}
	}
	return prd.Phase{}, false
}

// FromSpecs constructs a graph from explicit node specs.
func FromSpecs(specs []Spec) *Graph {
	g := &Graph{SchemaVersion: SchemaVersion, Nodes: make([]Node, 0, len(specs))}
	seen := map[NodeID]int{}
	for _, s := range specs {
		id := NodeID(string(s.ID))
		if id == "" {
			g.Diagnostics = append(g.Diagnostics, Diagnostic{
				Severity: SevError,
				Code:     CodeInvalidNode,
				Message:  "node id is empty",
			})
			continue
		}
		if _, err := prd.ParsePhaseID(string(id)); err != nil {
			g.Diagnostics = append(g.Diagnostics, Diagnostic{
				Severity: SevError,
				Code:     CodeInvalidNode,
				Message:  fmt.Sprintf("invalid node identifier %q", id),
				Nodes:    []NodeID{id},
			})
			continue
		}
		if first, dup := seen[id]; dup {
			g.Diagnostics = append(g.Diagnostics, Diagnostic{
				Severity: SevError,
				Code:     CodeDuplicateNode,
				Message:  fmt.Sprintf("duplicate node ID %s (first at spec %d)", id, first),
				Nodes:    []NodeID{id},
			})
			continue
		}
		st := s.Status
		if st == "" {
			st = StatusPending
		}
		seen[id] = len(g.Nodes)
		g.Nodes = append(g.Nodes, Node{
			ID:           id,
			Type:         TypePhase,
			Name:         s.Name,
			Dependencies: nil,
			Dependents:   nil,
			Status:       st,
			PhaseID:      prd.PhaseID(id),
		})
	}
	g.rebuildIndex()

	for _, s := range specs {
		to := NodeID(s.ID)
		if _, ok := g.index[to]; !ok {
			continue
		}
		for _, from := range s.Deps {
			g.addDeclaredEdge(from, to)
		}
	}
	g.finalize()
	return g
}

func (g *Graph) addDeclaredEdge(from, to NodeID) {
	if from == to {
		g.Diagnostics = append(g.Diagnostics, Diagnostic{
			Severity: SevError,
			Code:     CodeSelfDependency,
			Message:  fmt.Sprintf("self-dependency on %s", to),
			Nodes:    []NodeID{to},
		})
		return
	}
	if _, ok := g.index[from]; !ok {
		g.Diagnostics = append(g.Diagnostics, Diagnostic{
			Severity: SevError,
			Code:     CodeUnknownDependency,
			Message:  fmt.Sprintf("unknown dependency %s referenced by %s", from, to),
			Nodes:    []NodeID{to, from},
		})
		return
	}
	if _, ok := g.index[to]; !ok {
		return
	}
	for _, e := range g.Edges {
		if e.From == from && e.To == to {
			return
		}
	}
	g.Edges = append(g.Edges, Edge{From: from, To: to})
	ni := g.index[to]
	g.Nodes[ni].Dependencies = appendUnique(g.Nodes[ni].Dependencies, from)
}

func (g *Graph) finalize() {
	g.sortStable()
	g.rebuildIndex()
	g.computeDependents()
	g.detectCycles()
	g.noteIndependence()
	g.Refresh()
}

func (g *Graph) sortStable() {
	sort.Slice(g.Nodes, func(i, j int) bool { return lessID(g.Nodes[i].ID, g.Nodes[j].ID) })
	sort.Slice(g.Edges, func(i, j int) bool {
		if g.Edges[i].From != g.Edges[j].From {
			return lessID(g.Edges[i].From, g.Edges[j].From)
		}
		return lessID(g.Edges[i].To, g.Edges[j].To)
	})
	for i := range g.Nodes {
		sortIDs(g.Nodes[i].Dependencies)
	}
}

func (g *Graph) computeDependents() {
	deps := map[NodeID][]NodeID{}
	for _, e := range g.Edges {
		deps[e.From] = append(deps[e.From], e.To)
	}
	for i := range g.Nodes {
		d := deps[g.Nodes[i].ID]
		sortIDs(d)
		g.Nodes[i].Dependents = d
	}
}

func (g *Graph) detectCycles() {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[NodeID]int{}
	parent := map[NodeID]NodeID{}
	adj := g.outAdj()
	var cycle []NodeID

	var dfs func(NodeID)
	dfs = func(u NodeID) {
		if cycle != nil {
			return
		}
		color[u] = gray
		for _, v := range adj[u] {
			if cycle != nil {
				return
			}
			if color[v] == white {
				parent[v] = u
				dfs(v)
			} else if color[v] == gray {
				cycle = reconstructCycle(parent, v, u)
			}
		}
		color[u] = black
	}

	ids := g.sortedIDs()
	for _, id := range ids {
		if color[id] == white {
			dfs(id)
		}
	}
	if cycle != nil {
		g.Diagnostics = append(g.Diagnostics, Diagnostic{
			Severity: SevError,
			Code:     CodeCycle,
			Message:  "cycle detected: " + formatCycle(cycle),
			Nodes:    append([]NodeID{}, cycle...),
		})
	}
}

func reconstructCycle(parent map[NodeID]NodeID, start, end NodeID) []NodeID {
	var nodes []NodeID
	seen := map[NodeID]bool{}
	for cur := end; !seen[cur]; {
		nodes = append(nodes, cur)
		seen[cur] = true
		if cur == start {
			break
		}
		next, ok := parent[cur]
		if !ok {
			break
		}
		cur = next
	}
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	return append(nodes, start)
}

func formatCycle(cycle []NodeID) string {
	if len(cycle) == 0 {
		return ""
	}
	s := string(cycle[0])
	for i := 1; i < len(cycle); i++ {
		s += " → " + string(cycle[i])
	}
	return s
}

func (g *Graph) noteIndependence() {
	if len(g.Nodes) > 1 && len(g.Edges) == 0 {
		g.Diagnostics = append(g.Diagnostics, Diagnostic{
			Severity: SevInfo,
			Code:     CodeNoExplicitDeps,
			Message:  "no explicit phase dependencies were declared; nodes are independent and order is by node ID",
		})
	}
	if len(g.Edges) == 0 {
		return
	}
	connected := map[NodeID]bool{}
	for _, e := range g.Edges {
		connected[e.From] = true
		connected[e.To] = true
	}
	for _, n := range g.Nodes {
		if !connected[n.ID] {
			g.Diagnostics = append(g.Diagnostics, Diagnostic{
				Severity: SevInfo,
				Code:     CodeIndependentNode,
				Message:  fmt.Sprintf("node %s has no edges and is independent", n.ID),
				Nodes:    []NodeID{n.ID},
			})
		}
	}
}

func (g *Graph) outAdj() map[NodeID][]NodeID {
	adj := map[NodeID][]NodeID{}
	for _, n := range g.Nodes {
		adj[n.ID] = nil
	}
	for _, e := range g.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	for id := range adj {
		sortIDs(adj[id])
	}
	return adj
}

func (g *Graph) inAdj() map[NodeID][]NodeID {
	adj := map[NodeID][]NodeID{}
	for _, n := range g.Nodes {
		adj[n.ID] = nil
	}
	for _, e := range g.Edges {
		adj[e.To] = append(adj[e.To], e.From)
	}
	for id := range adj {
		sortIDs(adj[id])
	}
	return adj
}

func (g *Graph) sortedIDs() []NodeID {
	ids := make([]NodeID, len(g.Nodes))
	for i, n := range g.Nodes {
		ids[i] = n.ID
	}
	sortIDs(ids)
	return ids
}

func sortIDs(ids []NodeID) {
	sort.Slice(ids, func(i, j int) bool { return lessID(ids[i], ids[j]) })
}

func appendUnique(ids []NodeID, id NodeID) []NodeID {
	for _, x := range ids {
		if x == id {
			return ids
		}
	}
	return append(ids, id)
}

func lessID(a, b NodeID) bool {
	na, oka := phaseNum(a)
	nb, okb := phaseNum(b)
	if oka && okb {
		if na != nb {
			return na < nb
		}
		return a < b
	}
	return a < b
}

func phaseNum(id NodeID) (int, bool) {
	s := string(id)
	if len(s) < 2 || s[0] != 'P' {
		return 0, false
	}
	i := 1
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 1 {
		return 0, false
	}
	n, err := strconv.Atoi(s[1:i])
	if err != nil {
		return 0, false
	}
	if i < len(s) {
		rest := s[i:]
		if len(rest) != 1 || rest[0] < 'A' || rest[0] > 'Z' {
			return 0, false
		}
	}
	return n, true
}
