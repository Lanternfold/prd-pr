package subagent

const (
	UseSubagent = "USE_SUBAGENT"
	NoSubagent  = "NO_SUBAGENT"
)

// Input is the decision evidence. V1 does not spawn workers from this.
type Input struct {
	Parallelizable     bool
	NeedsIsolation     bool
	Complexity         string
	ExpectedSpeedup    float64
	EstimatedCostUSD   float64
	Risk               string
}

// Decision records whether a subagent would help. V1 still executes sequentially.
type Decision struct {
	Choice          string  `json:"choice"`
	Reason          string  `json:"reason"`
	ExpectedBenefit string  `json:"expected_benefit,omitempty"`
	EstimatedCost   float64 `json:"estimated_cost,omitempty"`
	Risk            string  `json:"risk,omitempty"`
}

// Decide returns a structured recommendation. V1 defaults to NO_SUBAGENT.
func Decide(in Input) Decision {
	if in.Parallelizable && in.NeedsIsolation && in.ExpectedSpeedup >= 2 && in.EstimatedCostUSD < 1 && in.Risk == "low" && in.Complexity == "high" {
		return Decision{
			Choice:          UseSubagent,
			Reason:          "isolated parallel work could reduce wall time",
			ExpectedBenefit: "speed",
			EstimatedCost:   in.EstimatedCostUSD,
			Risk:            in.Risk,
		}
	}
	return Decision{
		Choice: NoSubagent,
		Reason: "V1 sequential scheduler; additional agents are not justified",
		Risk:   in.Risk,
	}
}
