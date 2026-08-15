package subagent

import "testing"

func TestDefaultNoSubagent(t *testing.T) {
	d := Decide(Input{Parallelizable: true, Complexity: "low"})
	if d.Choice != NoSubagent {
		t.Fatalf("%+v", d)
	}
}

func TestUseSubagentOnlyWhenJustified(t *testing.T) {
	d := Decide(Input{
		Parallelizable: true, NeedsIsolation: true, Complexity: "high",
		ExpectedSpeedup: 2, EstimatedCostUSD: 0.1, Risk: "low",
	})
	if d.Choice != UseSubagent {
		t.Fatalf("%+v", d)
	}
}
