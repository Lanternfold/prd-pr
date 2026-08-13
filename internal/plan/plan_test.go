package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/plan"
	"github.com/lanternfold/prd-pr/internal/prd"
)

func TestDeterministicPlannerGolden(t *testing.T) {
	path := filepath.Join("..", "prd", "testdata", "prd", "minimal_valid.md")
	doc, err := prd.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := plan.DeterministicPlanner{}.Packet(plan.Input{
		Document:    doc,
		PhaseID:     "P1",
		ProjectID:   "proj_test",
		TaskID:      "task_test",
		ProductRoot: "/tmp/fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pkt.PhaseID != "P1" || pkt.ProjectID != "proj_test" {
		t.Fatalf("packet = %+v", pkt)
	}
	if pkt.Objective == "" {
		t.Fatal("missing objective")
	}
	if len(pkt.Requirements) != 2 {
		t.Fatalf("requirements = %d", len(pkt.Requirements))
	}
	if pkt.Requirements[0].ID != "REQ-001" {
		t.Fatalf("req id = %s", pkt.Requirements[0].ID)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(pkt.Objective, string(raw)) || strings.Contains(pkt.RelevantArchitecture, "# PRD:") {
		t.Fatal("planner dumped the entire PRD into the packet")
	}
}

func TestPlannerUnknownPhase(t *testing.T) {
	path := filepath.Join("..", "prd", "testdata", "prd", "minimal_valid.md")
	doc, err := prd.ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = plan.DeterministicPlanner{}.Packet(plan.Input{
		Document:    doc,
		PhaseID:     "P99",
		ProjectID:   "p",
		TaskID:      "t",
		ProductRoot: "/tmp/x",
	})
	if err == nil {
		t.Fatal("expected unknown phase error")
	}
}
