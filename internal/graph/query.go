package graph

import "fmt"

// Refresh recomputes READY/BLOCKED from dependency statuses.
// RUNNING, COMPLETED, FAILED, and WAITING_FOR_HUMAN are left unchanged.
func (g *Graph) Refresh() {
	if g == nil {
		return
	}
	g.rebuildIndex()
	cyclic := false
	for _, d := range g.Diagnostics {
		if d.Code == CodeCycle {
			cyclic = true
			break
		}
	}
	for i := range g.Nodes {
		st := g.Nodes[i].Status
		switch st {
		case StatusCompleted, StatusFailed, StatusRunning, StatusWaitingForHuman:
			g.Nodes[i].BlockedReasons = nil
			continue
		}
		g.Nodes[i].BlockedReasons = nil
		if cyclic {
			g.Nodes[i].Status = StatusBlocked
			g.Nodes[i].BlockedReasons = []BlockReason{{Code: BlockGraphInvalid}}
			continue
		}
		reasons := g.blockReasons(g.Nodes[i])
		if len(reasons) == 0 {
			g.Nodes[i].Status = StatusReady
			continue
		}
		g.Nodes[i].Status = StatusBlocked
		g.Nodes[i].BlockedReasons = reasons
	}
}

func (g *Graph) blockReasons(n Node) []BlockReason {
	var reasons []BlockReason
	for _, depID := range n.Dependencies {
		dep, ok := g.Node(depID)
		if !ok {
			reasons = append(reasons, BlockReason{Code: BlockGraphInvalid, Dependency: depID})
			continue
		}
		switch dep.Status {
		case StatusCompleted:
			continue
		case StatusFailed:
			reasons = append(reasons, BlockReason{Code: BlockDepFailed, Dependency: depID})
		case StatusBlocked:
			reasons = append(reasons, BlockReason{Code: BlockDepBlocked, Dependency: depID})
		case StatusWaitingForHuman:
			reasons = append(reasons, BlockReason{Code: BlockDepWaiting, Dependency: depID})
		default:
			reasons = append(reasons, BlockReason{Code: BlockDepIncomplete, Dependency: depID})
		}
	}
	return reasons
}

// SetStatus updates a node and refreshes READY/BLOCKED.
func (g *Graph) SetStatus(id NodeID, st Status) error {
	if g == nil || g.index == nil {
		return fmt.Errorf("graph is nil")
	}
	i, ok := g.index[id]
	if !ok {
		return fmt.Errorf("%s: unknown node %s", CodeUnknownNode, id)
	}
	g.Nodes[i].Status = st
	g.Refresh()
	return nil
}

// SequentialOrder is a deterministic topological order. Empty if the graph has a cycle.
func (g *Graph) SequentialOrder() []NodeID {
	if g == nil {
		return nil
	}
	for _, d := range g.Diagnostics {
		if d.Code == CodeCycle {
			return nil
		}
	}
	return g.kahn()
}

func (g *Graph) kahn() []NodeID {
	indeg := map[NodeID]int{}
	for _, n := range g.Nodes {
		indeg[n.ID] = 0
	}
	for _, e := range g.Edges {
		indeg[e.To]++
	}
	var ready []NodeID
	for _, id := range g.sortedIDs() {
		if indeg[id] == 0 {
			ready = append(ready, id)
		}
	}
	var order []NodeID
	adj := g.outAdj()
	for len(ready) > 0 {
		sortIDs(ready)
		u := ready[0]
		ready = ready[1:]
		order = append(order, u)
		for _, v := range adj[u] {
			indeg[v]--
			if indeg[v] == 0 {
				ready = append(ready, v)
			}
		}
	}
	if len(order) != len(g.Nodes) {
		return nil
	}
	return order
}

// AllCompleted reports whether every phase node is COMPLETED.
func (g *Graph) AllCompleted() bool {
	if g == nil || len(g.Nodes) == 0 {
		return false
	}
	for _, n := range g.Nodes {
		if n.Status != StatusCompleted {
			return false
		}
	}
	return true
}

// Ready returns nodes whose dependencies have completed successfully.
func (g *Graph) Ready() []NodeID {
	if g == nil {
		return nil
	}
	var ids []NodeID
	for _, n := range g.Nodes {
		if n.Status == StatusReady {
			ids = append(ids, n.ID)
		}
	}
	sortIDs(ids)
	return ids
}

// Blocked returns nodes that cannot run, with structured reasons on the node.
func (g *Graph) Blocked() []NodeID {
	if g == nil {
		return nil
	}
	var ids []NodeID
	for _, n := range g.Nodes {
		if n.Status == StatusBlocked {
			ids = append(ids, n.ID)
		}
	}
	sortIDs(ids)
	return ids
}

// Independent returns nodes with no incoming or outgoing edges (parallel-isolated).
func (g *Graph) Independent() []NodeID {
	if g == nil {
		return nil
	}
	var ids []NodeID
	for _, n := range g.Nodes {
		if len(n.Dependencies) == 0 && len(n.Dependents) == 0 {
			ids = append(ids, n.ID)
		}
	}
	sortIDs(ids)
	return ids
}

// ParallelCandidates is the ready set. V1 does not execute them concurrently.
func (g *Graph) ParallelCandidates() []NodeID {
	return g.Ready()
}

// Affected returns strict descendants of id in topological order.
func (g *Graph) Affected(id NodeID) []NodeID {
	if g == nil {
		return nil
	}
	if _, ok := g.index[id]; !ok {
		return nil
	}
	seen := map[NodeID]bool{}
	adj := g.outAdj()
	var walk func(NodeID)
	walk = func(u NodeID) {
		for _, v := range adj[u] {
			if seen[v] {
				continue
			}
			seen[v] = true
			walk(v)
		}
	}
	walk(id)
	order := g.SequentialOrder()
	if len(order) == 0 {
		ids := make([]NodeID, 0, len(seen))
		for id := range seen {
			ids = append(ids, id)
		}
		sortIDs(ids)
		return ids
	}
	var out []NodeID
	for _, n := range order {
		if seen[n] {
			out = append(out, n)
		}
	}
	return out
}

// Rewind calculates which nodes must be replayed if origin changes.
func (g *Graph) Rewind(id NodeID) (RewindPlan, error) {
	if g == nil {
		return RewindPlan{}, fmt.Errorf("graph is nil")
	}
	if _, ok := g.index[id]; !ok {
		return RewindPlan{}, fmt.Errorf("%s: unknown node %s", CodeUnknownNode, id)
	}
	affected := g.Affected(id)
	replay := append([]NodeID{id}, affected...)
	return RewindPlan{Origin: id, Affected: affected, ReplayOrder: replay}, nil
}
