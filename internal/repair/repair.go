package repair

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/diagnose"
	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/review"
	"github.com/lanternfold/prd-pr/internal/testeng"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

const (
	SchemaVersion = 1
	MaxAttempts   = 3
	IncidentRel   = ".project/incidents"
)

// Attempt is one bounded repair try.
type Attempt struct {
	Number       int      `json:"number"`
	Checkpoint   string   `json:"checkpoint_sha,omitempty"`
	ChangedPaths []string `json:"changed_paths,omitempty"`
	Verified     bool     `json:"verified"`
	Summary      string   `json:"summary,omitempty"`
	At           string   `json:"at"`
}

// Incident is one failure chain. Unrelated failures get a new incident.
type Incident struct {
	SchemaVersion int             `json:"schema_version"`
	ID            string          `json:"id"`
	PhaseID       string          `json:"phase_id"`
	TaskID        string          `json:"task_id"`
	RunID         string          `json:"run_id"`
	MaxAttempts   int             `json:"max_attempts"`
	Attempts      []Attempt       `json:"attempts"`
	Diagnoses     []diagnose.Report `json:"diagnoses,omitempty"`
	HumanAction   string          `json:"recommended_human_action,omitempty"`
	Exhausted     bool            `json:"exhausted"`
}

func Max(cfg config.Config) int {
	if cfg.MaxRepairAttempts > 0 {
		return cfg.MaxRepairAttempts
	}
	return MaxAttempts
}

func Remaining(inc Incident, cfg config.Config) int {
	n := Max(cfg) - len(inc.Attempts)
	if inc.MaxAttempts > 0 {
		n = inc.MaxAttempts - len(inc.Attempts)
	}
	if n < 0 {
		return 0
	}
	return n
}

func CanAttempt(inc Incident, cfg config.Config) bool {
	return Remaining(inc, cfg) > 0 && !inc.Exhausted
}

// Packet is the bounded repair instruction. It is not "fix everything".
type Packet struct {
	SchemaVersion    int              `json:"schema_version"`
	IncidentID       string           `json:"incident_id"`
	Attempt          int              `json:"attempt"`
	Original         packet.Packet    `json:"original_task"`
	Diagnosis        diagnose.Report  `json:"diagnosis"`
	Verification     testeng.Result   `json:"verification_evidence"`
	AllowedPaths     []string         `json:"allowed_paths"`
	Constraints      []string         `json:"constraints"`
	PreviousAttempts []Attempt        `json:"previous_attempts,omitempty"`
	DiffSummary      string           `json:"diff_summary,omitempty"`
	ProductRoot      string           `json:"product_root"`
	MaxAttempts      int              `json:"max_attempts"`
}

func NewPacket(inc Incident, orig packet.Packet, rev review.Result, v testeng.Result, diff string) Packet {
	scope := rev.Diagnosis.RecommendedScope
	if len(scope) == 0 {
		scope = orig.ExpectedOutputs
	}
	attempt := len(inc.Attempts) + 1
	max := MaxAttempts
	if inc.MaxAttempts > 0 {
		max = inc.MaxAttempts
	}
	constraints := append([]string{}, orig.Constraints...)
	constraints = append(constraints,
		"Stay inside product_root.",
		"Modify only allowed_paths.",
		"Do not treat a successful exit as verification.",
		fmt.Sprintf("This is repair attempt %d of %d.", attempt, max),
	)
	return Packet{
		SchemaVersion:    SchemaVersion,
		IncidentID:       inc.ID,
		Attempt:          attempt,
		Original:         orig,
		Diagnosis:        rev.Diagnosis,
		Verification:     v,
		AllowedPaths:     scope,
		Constraints:      constraints,
		PreviousAttempts: inc.Attempts,
		DiffSummary:      diff,
		ProductRoot:      orig.ProductRoot,
		MaxAttempts:      max,
	}
}

func (p Packet) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported repair packet schema")
	}
	max := p.MaxAttempts
	if max == 0 {
		max = MaxAttempts
	}
	if p.IncidentID == "" || p.Attempt < 1 || p.Attempt > max {
		return fmt.Errorf("repair packet attempt out of bounds")
	}
	if err := p.Original.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(p.ProductRoot) == "" {
		return fmt.Errorf("repair packet product_root is required")
	}
	return nil
}

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
		return Packet{}, err
	}
	return p, p.Validate()
}

func Prompt(rel string) string {
	return strings.TrimSpace(`You are applying a bounded repair for PRD→PR.

Read the repair packet at ` + rel + `.

Rules:
- Implement only the diagnosis and allowed_paths.
- Stay inside product_root.
- Do not rewrite unrelated files.
- Do not claim verification.
- Use original task packet, verification evidence, and prior attempts.
`)
}

func AsTaskPacket(p Packet) packet.Packet {
	orig := p.Original
	orig.Objective = fmt.Sprintf("Repair attempt %d: %s", p.Attempt, p.Diagnosis.Summary)
	orig.Constraints = p.Constraints
	orig.ForbiddenPaths = append(orig.ForbiddenPaths, ".git")
	return orig
}

func BeginAttempt(inc Incident, now time.Time) Incident {
	if inc.MaxAttempts == 0 {
		inc.MaxAttempts = MaxAttempts
	}
	inc.Attempts = append(inc.Attempts, Attempt{
		Number:  len(inc.Attempts) + 1,
		Summary: "repair packet prepared",
		At:      now.UTC().Format(time.RFC3339Nano),
	})
	return inc
}

func FinishAttempt(inc Incident, number int, checkpoint string, changed []string, verified bool, summary string, now time.Time) Incident {
	if inc.MaxAttempts == 0 {
		inc.MaxAttempts = MaxAttempts
	}
	found := false
	for i := range inc.Attempts {
		if inc.Attempts[i].Number == number {
			inc.Attempts[i].Checkpoint = checkpoint
			inc.Attempts[i].ChangedPaths = changed
			inc.Attempts[i].Verified = verified
			if summary != "" {
				inc.Attempts[i].Summary = summary
			}
			inc.Attempts[i].At = now.UTC().Format(time.RFC3339Nano)
			found = true
			break
		}
	}
	if !found {
		inc = RecordAttempt(inc, checkpoint, changed, verified, summary, now)
		return inc
	}
	return markExhausted(inc, verified)
}

func RecordAttempt(inc Incident, checkpoint string, changed []string, verified bool, summary string, now time.Time) Incident {
	if inc.MaxAttempts == 0 {
		inc.MaxAttempts = MaxAttempts
	}
	inc.Attempts = append(inc.Attempts, Attempt{
		Number:       len(inc.Attempts) + 1,
		Checkpoint:   checkpoint,
		ChangedPaths: changed,
		Verified:     verified,
		Summary:      summary,
		At:           now.UTC().Format(time.RFC3339Nano),
	})
	return markExhausted(inc, verified)
}

func markExhausted(inc Incident, verified bool) Incident {
	if inc.MaxAttempts == 0 {
		inc.MaxAttempts = MaxAttempts
	}
	if verified {
		inc.Exhausted = false
		return inc
	}
	if len(inc.Attempts) >= inc.MaxAttempts {
		inc.Exhausted = true
		inc.HumanAction = "Inspect verification.json, incident attempts, and the failing tests. Autonomous repair is exhausted."
	}
	return inc
}

// RewindTarget uses the graph: local failure stays on the phase; upstream origin rewinds there.
func RewindTarget(g *graph.Graph, phase string, origin string) (graph.RewindPlan, error) {
	id := graph.NodeID(phase)
	if origin == diagnose.OriginUpstreamPhase && g != nil {
		// caller passes the upstream id in phase when origin is upstream
	}
	return g.Rewind(id)
}

func CheckpointSHA(baseline vcs.Baseline, head string) string {
	if head != "" {
		return head
	}
	return baseline.SHA
}
