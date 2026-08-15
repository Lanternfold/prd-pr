package engine

import (
	"os"
	"path/filepath"

	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/prd"
)

func loadGraph(root string) *graph.Graph {
	raw, err := os.ReadFile(filepath.Join(root, graphFile))
	if err != nil {
		return nil
	}
	g, err := graph.Unmarshal(raw)
	if err != nil {
		return nil
	}
	return g
}

func mergeGraphStatus(dst, src *graph.Graph) {
	if dst == nil || src == nil {
		return
	}
	for _, n := range src.Nodes {
		_ = dst.SetStatus(n.ID, n.Status)
	}
	dst.Refresh()
}

func firstReadyPhase(g *graph.Graph) prd.PhaseID {
	if g == nil {
		return ""
	}
	ready := g.Ready()
	if len(ready) > 0 {
		return prd.PhaseID(ready[0])
	}
	return ""
}

func markPhase(g *graph.Graph, id string, st graph.Status) {
	if g == nil || id == "" {
		return
	}
	_ = g.SetStatus(graph.NodeID(id), st)
}

func applyRewind(g *graph.Graph, plan graph.RewindPlan) {
	if g == nil {
		return
	}
	for _, id := range plan.ReplayOrder {
		_ = g.SetStatus(id, graph.StatusPending)
	}
	g.Refresh()
}
