package packet

import (
	"encoding/json"
	"fmt"
	"strings"
)

const SchemaVersion = 1

// Packet is the versioned task file given to a coding worker.
// It is scoped to one phase/task and must not contain the entire PRD or secrets.
type Packet struct {
	SchemaVersion        int      `json:"schema_version"`
	TaskID               string   `json:"task_id"`
	ProjectID            string   `json:"project_id"`
	PhaseID              string   `json:"phase_id"`
	Objective            string   `json:"objective"`
	Requirements         []Item   `json:"requirements"`
	AcceptanceCriteria   []Item   `json:"acceptance_criteria"`
	Constraints          []string `json:"constraints"`
	RelevantArchitecture string   `json:"relevant_architecture,omitempty"`
	RelevantDesign       string   `json:"relevant_design,omitempty"`
	RelevantDecisions    []string `json:"relevant_decisions,omitempty"`
	DesignRefs           []string `json:"design_refs,omitempty"`
	ADRRefs              []string `json:"adr_refs,omitempty"`
	ExpectedOutputs      []string `json:"expected_outputs,omitempty"`
	TestExpectations     []string `json:"test_expectations,omitempty"`
	TestCommands         []string `json:"test_commands,omitempty"`
	ProductRoot          string   `json:"product_root"`
	DefinitionOfDone     []string `json:"dod"`
	ForbiddenPaths       []string `json:"forbidden_paths,omitempty"`
}

// Item is a referenced requirement, criterion, or similar snippet.
type Item struct {
	ID   string `json:"id,omitempty"`
	Text string `json:"text,omitempty"`
}

// Validate checks required packet fields. It does not invoke a worker.
func (p Packet) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported packet schema_version %d", p.SchemaVersion)
	}
	if strings.TrimSpace(p.TaskID) == "" {
		return fmt.Errorf("packet task_id is required")
	}
	if strings.TrimSpace(p.ProjectID) == "" {
		return fmt.Errorf("packet project_id is required")
	}
	if strings.TrimSpace(p.PhaseID) == "" {
		return fmt.Errorf("packet phase_id is required")
	}
	if strings.TrimSpace(p.Objective) == "" {
		return fmt.Errorf("packet objective is required")
	}
	if strings.TrimSpace(p.ProductRoot) == "" {
		return fmt.Errorf("packet product_root is required")
	}
	return nil
}

// Marshal writes canonical indented JSON.
func Marshal(p Packet) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func Unmarshal(data []byte) (Packet, error) {
	var p Packet
	if err := json.Unmarshal(data, &p); err != nil {
		return Packet{}, fmt.Errorf("decode task packet: %w", err)
	}
	if err := p.Validate(); err != nil {
		return Packet{}, err
	}
	return p, nil
}

// Prompt is the worker instruction. It points at the packet file instead of dumping the PRD.
func Prompt(packetRel string) string {
	packetRel = strings.TrimSpace(packetRel)
	if packetRel == "" {
		packetRel = ".project/packets/current.json"
	}
	return strings.TrimSpace(`You are a coding worker for the PRD→PR orchestrator.

Read the task packet at ` + packetRel + ` and implement only that packet.

Rules:
- Stay inside product_root from the packet.
- Do not modify forbidden_paths.
- Do not dump or request the entire PRD.
- Do not treat your own claim of success as verification.
- When finished, summarize what you changed in a few sentences.
`)
}
