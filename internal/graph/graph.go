package graph

import "github.com/lanternfold/prd-pr/internal/prd"

const SchemaVersion = 1

type NodeID string

type NodeType string

const TypePhase NodeType = "PHASE"

type Status string

const (
	StatusPending         Status = "PENDING"
	StatusReady           Status = "READY"
	StatusRunning         Status = "RUNNING"
	StatusCompleted       Status = "COMPLETED"
	StatusFailed          Status = "FAILED"
	StatusBlocked         Status = "BLOCKED"
	StatusWaitingForHuman Status = "WAITING_FOR_HUMAN"
)

type BlockCode string

const (
	BlockDepIncomplete BlockCode = "DEPENDENCY_INCOMPLETE"
	BlockDepFailed     BlockCode = "DEPENDENCY_FAILED"
	BlockDepBlocked    BlockCode = "DEPENDENCY_BLOCKED"
	BlockDepWaiting    BlockCode = "DEPENDENCY_WAITING_FOR_HUMAN"
	BlockGraphInvalid  BlockCode = "GRAPH_INVALID"
)

type Severity string

const (
	SevError   Severity = "ERROR"
	SevWarning Severity = "WARNING"
	SevInfo    Severity = "INFO"
)

const (
	CodeDuplicateNode     = "GRAPH_DUPLICATE_NODE"
	CodeUnknownDependency = "GRAPH_UNKNOWN_DEPENDENCY"
	CodeSelfDependency    = "GRAPH_SELF_DEPENDENCY"
	CodeCycle             = "GRAPH_CYCLE"
	CodeInvalidNode       = "GRAPH_INVALID_NODE"
	CodeMalformed         = "GRAPH_MALFORMED"
	CodeIndependentNode   = "GRAPH_INDEPENDENT_NODE"
	CodeNoExplicitDeps    = "GRAPH_NO_EXPLICIT_DEPENDENCIES"
	CodeUnknownNode       = "GRAPH_UNKNOWN_NODE"
)

// Diagnostic is a structured graph validation finding.
type Diagnostic struct {
	Severity  Severity `json:"severity"`
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	Nodes     []NodeID `json:"nodes,omitempty"`
	StartLine int      `json:"start_line,omitempty"`
}

type BlockReason struct {
	Code       BlockCode `json:"code"`
	Dependency NodeID    `json:"dependency,omitempty"`
}

type Node struct {
	ID             NodeID        `json:"id"`
	Type           NodeType      `json:"type"`
	Name           string        `json:"name,omitempty"`
	Dependencies   []NodeID      `json:"dependencies"`
	Dependents     []NodeID      `json:"dependents"`
	Status         Status        `json:"status"`
	BlockedReasons []BlockReason `json:"blocked_reasons,omitempty"`
	PhaseID        prd.PhaseID   `json:"phase_id,omitempty"`
	Source         prd.SourceRef `json:"source,omitempty"`
}

// Edge is "From must complete before To".
type Edge struct {
	From NodeID `json:"from"`
	To   NodeID `json:"to"`
}

// Graph is a phase-level DAG. It does not execute work.
type Graph struct {
	SchemaVersion int          `json:"schema_version"`
	Nodes         []Node       `json:"nodes"`
	Edges         []Edge       `json:"edges"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
	index         map[NodeID]int
}

func (g *Graph) HasErrors() bool {
	if g == nil {
		return false
	}
	for _, d := range g.Diagnostics {
		if d.Severity == SevError {
			return true
		}
	}
	return false
}

func (g *Graph) Node(id NodeID) (Node, bool) {
	if g == nil || g.index == nil {
		return Node{}, false
	}
	i, ok := g.index[id]
	if !ok {
		return Node{}, false
	}
	return g.Nodes[i], true
}

func (g *Graph) rebuildIndex() {
	g.index = make(map[NodeID]int, len(g.Nodes))
	for i, n := range g.Nodes {
		g.index[n.ID] = i
	}
}

// RewindPlan is a calculation only: no Git or filesystem action.
type RewindPlan struct {
	Origin      NodeID   `json:"origin"`
	Affected    []NodeID `json:"affected"`
	ReplayOrder []NodeID `json:"replay_order"`
}

// Spec is a test/construction helper. Dependencies are explicit.
type Spec struct {
	ID     NodeID
	Name   string
	Deps   []NodeID
	Status Status
}
