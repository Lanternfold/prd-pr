package engine

import (
	"os"
	"path/filepath"

	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/state"
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

func persistGraph(g *state.Guard, gr *graph.Graph) {
	if g == nil || gr == nil {
		return
	}
	gr.Refresh()
	raw, err := gr.Marshal()
	if err != nil {
		return
	}
	_ = g.WriteFile(graphFile, append(raw, '\n'))
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

func firstWaitingPhase(g *graph.Graph) string {
	if g == nil {
		return ""
	}
	for _, n := range g.Nodes {
		if n.Status == graph.StatusWaitingForHuman {
			return string(n.ID)
		}
	}
	return ""
}

// selectRunnablePhase returns the next phase the engine may execute.
// A requested id is accepted only when it is READY, or RUNNING for the already-active phase.
func selectRunnablePhase(g *graph.Graph, requested prd.PhaseID, currentPhase string) (prd.PhaseID, string) {
	if g == nil {
		return "", "graph is missing"
	}
	if requested != "" {
		n, ok := g.Node(graph.NodeID(requested))
		if !ok {
			return "", "unknown phase " + string(requested)
		}
		switch n.Status {
		case graph.StatusReady:
			return requested, ""
		case graph.StatusRunning:
			if currentPhase == string(requested) {
				return requested, ""
			}
			return "", "phase " + string(requested) + " is not READY (RUNNING)"
		default:
			return "", "phase " + string(requested) + " is not READY (" + string(n.Status) + ")"
		}
	}
	if currentPhase != "" {
		if n, ok := g.Node(graph.NodeID(currentPhase)); ok {
			switch n.Status {
			case graph.StatusRunning:
				return prd.PhaseID(currentPhase), ""
			case graph.StatusWaitingForHuman:
				return "", "phase " + currentPhase + " is WAITING_FOR_HUMAN"
			}
		}
	}
	if id := firstReadyPhase(g); id != "" {
		return id, ""
	}
	if w := firstWaitingPhase(g); w != "" {
		return "", "phase " + w + " is WAITING_FOR_HUMAN"
	}
	return "", ""
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
