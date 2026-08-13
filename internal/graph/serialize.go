package graph

import "encoding/json"

// Marshal returns deterministic JSON. It does not write .project/.
func (g *Graph) Marshal() ([]byte, error) {
	if g == nil {
		return []byte("null"), nil
	}
	cp := *g
	cp.index = nil
	return json.MarshalIndent(cp, "", "  ")
}

// Unmarshal loads a graph snapshot and refreshes derived fields.
func Unmarshal(data []byte) (*Graph, error) {
	var g Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	if g.SchemaVersion == 0 {
		g.SchemaVersion = SchemaVersion
	}
	g.rebuildIndex()
	g.computeDependents()
	sortIDsOnGraph(&g)
	return &g, nil
}

func sortIDsOnGraph(g *Graph) {
	g.sortStable()
	g.rebuildIndex()
	g.computeDependents()
}
