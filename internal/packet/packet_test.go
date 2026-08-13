package packet_test

import (
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/packet"
)

func TestMarshalRoundTrip(t *testing.T) {
	p := packet.Packet{
		SchemaVersion: packet.SchemaVersion,
		TaskID:        "task_1",
		ProjectID:     "proj_1",
		PhaseID:       "P1",
		Objective:     "Implement Add",
		ProductRoot:   "/tmp/fixture",
		DefinitionOfDone: []string{"tests pass"},
	}
	raw, err := packet.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := packet.Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.TaskID != p.TaskID || got.Objective != p.Objective {
		t.Fatalf("got %+v", got)
	}
}

func TestValidateMissingFields(t *testing.T) {
	if err := (packet.Packet{SchemaVersion: 1}).Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestPromptDoesNotEmbedPRD(t *testing.T) {
	s := packet.Prompt(".project/packets/task.json")
	if !strings.Contains(s, ".project/packets/task.json") {
		t.Fatalf("prompt = %q", s)
	}
	if strings.Contains(strings.ToLower(s), "entire prd") && !strings.Contains(s, "Do not dump") {
		t.Fatal("prompt should tell the worker not to dump the PRD")
	}
}
