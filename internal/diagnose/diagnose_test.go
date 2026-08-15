package diagnose

import (
	"testing"

	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func TestInfrastructureDoesNotConsumeAttempt(t *testing.T) {
	r := Classify(testeng.Result{Status: testeng.StatusInfrastructure, Reason: "network"}, packet.Packet{})
	if r.ConsumesAttempt || r.Classification != ClassInfrastructure {
		t.Fatalf("%+v", r)
	}
}

func TestProductFailureIsActionable(t *testing.T) {
	r := Classify(testeng.Result{
		Status:       testeng.StatusFailed,
		Failures:     []string{"TestAdd: want 4"},
		ChangedFiles: []string{"add.go"},
	}, packet.Packet{ExpectedOutputs: []string{"add.go"}})
	if !r.Actionable || !r.ConsumesAttempt || r.Classification == ClassInfrastructure {
		t.Fatalf("%+v", r)
	}
	if len(r.RecommendedScope) == 0 {
		t.Fatal("scope")
	}
}

func TestVerifiedIsNone(t *testing.T) {
	r := Classify(testeng.Result{Status: testeng.StatusVerified}, packet.Packet{})
	if r.Classification != ClassNone || r.Actionable {
		t.Fatalf("%+v", r)
	}
}
