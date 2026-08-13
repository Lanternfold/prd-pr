package plan

import (
	"fmt"
	"strings"

	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/prd"
)

const maxExcerpt = 1200

// DeterministicPlanner maps one PRD phase to a task packet without network or LLM calls.
type DeterministicPlanner struct{}

type Input struct {
	Document    *prd.Document
	PhaseID     prd.PhaseID
	ProjectID   string
	TaskID      string
	ProductRoot string
}

func (DeterministicPlanner) Packet(in Input) (packet.Packet, error) {
	if in.Document == nil {
		return packet.Packet{}, fmt.Errorf("planner requires a parsed PRD")
	}
	if in.Document.HasErrors() {
		return packet.Packet{}, fmt.Errorf("planner refuses an invalid PRD")
	}
	phase, ok := findPhase(in.Document, in.PhaseID)
	if !ok {
		return packet.Packet{}, fmt.Errorf("phase %s is not in the PRD", in.PhaseID)
	}
	if strings.TrimSpace(in.ProjectID) == "" {
		return packet.Packet{}, fmt.Errorf("planner requires project_id")
	}
	if strings.TrimSpace(in.TaskID) == "" {
		return packet.Packet{}, fmt.Errorf("planner requires task_id")
	}
	if strings.TrimSpace(in.ProductRoot) == "" {
		return packet.Packet{}, fmt.Errorf("planner requires product_root")
	}

	obj := strings.TrimSpace(phase.Objective)
	if obj == "" {
		obj = strings.TrimSpace(phase.Name)
	}
	if obj == "" {
		obj = "Complete phase " + string(phase.ID)
	}

	p := packet.Packet{
		SchemaVersion:        packet.SchemaVersion,
		TaskID:               in.TaskID,
		ProjectID:            in.ProjectID,
		PhaseID:              string(phase.ID),
		Objective:            obj,
		Requirements:         lookupReqs(in.Document, phase.Requirements),
		AcceptanceCriteria:   lookupACs(in.Document, phase.AcceptanceCriteria),
		Constraints:          constraints(phase),
		RelevantArchitecture: excerpt(in.Document.Technical.Summary),
		RelevantDesign:       excerpt(in.Document.Design.Summary),
		RelevantDecisions:    decisions(phase),
		ExpectedOutputs:      append([]string{}, phase.Outputs...),
		TestExpectations:     lookupTests(in.Document, phase.Tests),
		ProductRoot:          in.ProductRoot,
		DefinitionOfDone:     append([]string{}, phase.DefinitionOfDone...),
		ForbiddenPaths: []string{
			"paths outside product_root",
			"the running prdpr binary",
			"unrelated repositories",
		},
	}
	if err := p.Validate(); err != nil {
		return packet.Packet{}, err
	}
	return p, nil
}

func findPhase(doc *prd.Document, id prd.PhaseID) (prd.Phase, bool) {
	for _, p := range doc.Phases {
		if p.ID == id {
			return p, true
		}
	}
	return prd.Phase{}, false
}

func lookupReqs(doc *prd.Document, ids []prd.RequirementID) []packet.Item {
	idx := map[prd.RequirementID]prd.Requirement{}
	for _, r := range doc.Requirements {
		idx[r.ID] = r
	}
	out := make([]packet.Item, 0, len(ids))
	for _, id := range ids {
		item := packet.Item{ID: string(id)}
		if r, ok := idx[id]; ok {
			item.Text = strings.TrimSpace(r.Text)
			if item.Text == "" {
				item.Text = strings.TrimSpace(r.Title)
			}
		}
		out = append(out, item)
	}
	return out
}

func lookupACs(doc *prd.Document, ids []prd.AcceptanceID) []packet.Item {
	idx := map[prd.AcceptanceID]prd.Criterion{}
	for _, c := range doc.Acceptance {
		idx[c.ID] = c
	}
	out := make([]packet.Item, 0, len(ids))
	for _, id := range ids {
		item := packet.Item{ID: string(id)}
		if c, ok := idx[id]; ok {
			item.Text = strings.TrimSpace(c.Text)
			if item.Text == "" {
				item.Text = strings.TrimSpace(c.Title)
			}
		}
		out = append(out, item)
	}
	return out
}

func lookupTests(doc *prd.Document, ids []prd.TestID) []string {
	idx := map[prd.TestID]prd.TestItem{}
	for _, t := range doc.Tests {
		idx[t.ID] = t
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if t, ok := idx[id]; ok {
			text := strings.TrimSpace(t.Text)
			if text == "" {
				text = strings.TrimSpace(t.Title)
			}
			if text != "" {
				out = append(out, string(id)+": "+text)
				continue
			}
		}
		out = append(out, string(id))
	}
	return out
}

func constraints(phase prd.Phase) []string {
	out := []string{
		"Modify only files inside product_root.",
		"Do not treat worker success text as verification.",
	}
	for _, r := range phase.Risks {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	for _, t := range phase.Tasks {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, "Task: "+t)
		}
	}
	return out
}

func decisions(phase prd.Phase) []string {
	if strings.TrimSpace(phase.HumanValidation) != "" {
		return []string{"Human validation: " + strings.TrimSpace(phase.HumanValidation)}
	}
	return nil
}

func excerpt(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= maxExcerpt {
		return s
	}
	return strings.TrimSpace(s[:maxExcerpt]) + "…"
}
