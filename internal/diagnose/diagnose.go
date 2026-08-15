package diagnose

import (
	"strings"

	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

const (
	ClassNone            = "NONE"
	ClassProduct         = "product"
	ClassInfrastructure  = "infrastructure"
	ClassMissingReq      = "missing_requirement"
	ClassTestGap         = "test_gap"
	ClassManual          = "manual_acceptance"
	ClassUnsafe          = "unsafe"
	ClassBlocked         = "blocked"
	ClassAmbiguous       = "ambiguous"
)

const (
	OriginLocalPhase     = "local_phase"
	OriginUpstreamPhase  = "upstream_phase"
)

// Report is a structured failure classification. It does not apply a fix.
type Report struct {
	Actionable       bool     `json:"actionable"`
	Classification   string   `json:"classification"`
	Origin           string   `json:"origin"`
	Summary          string   `json:"summary"`
	RecommendedScope []string `json:"recommended_scope,omitempty"`
	HumanReason      string   `json:"human_reason,omitempty"`
	ConsumesAttempt  bool     `json:"consumes_attempt"`
	Confidence       float64  `json:"confidence"`
	NeedsLLM         bool     `json:"needs_llm"`
}

// Classify maps P7 evidence into a diagnosis without calling an LLM.
func Classify(v testeng.Result, pkt packet.Packet) Report {
	r := Report{Origin: OriginLocalPhase, Confidence: 0.8}
	if v.ManualVerificationRequired && v.TestsPass && !v.VerifiedSuccess {
		r.Classification = ClassManual
		r.Summary = "manual acceptance criteria remain"
		r.HumanReason = "manual_acceptance"
		r.Actionable = false
		r.ConsumesAttempt = false
		return r
	}
	switch v.Status {
	case testeng.StatusVerified:
		r.Classification = ClassNone
		r.Summary = "verification already succeeded"
		r.Confidence = 1
		return r
	case testeng.StatusInfrastructure, testeng.StatusTimeout:
		r.Classification = ClassInfrastructure
		r.Summary = "infrastructure or timeout; do not consume a product repair attempt"
		if v.Reason != "" {
			r.Summary = v.Reason
		}
		r.ConsumesAttempt = false
		r.Actionable = false
		r.HumanReason = "retry or inspect environment"
		return r
	case testeng.StatusIncomplete, testeng.StatusUnsupported:
		r.Classification = ClassBlocked
		r.Summary = v.Reason
		r.Actionable = false
		r.HumanReason = v.Reason
		r.ConsumesAttempt = false
		return r
	}

	if v.ManualVerificationRequired && v.TestsPass {
		r.Classification = ClassManual
		r.Summary = "manual acceptance criteria remain"
		r.HumanReason = "manual_acceptance"
		r.Actionable = false
		r.ConsumesAttempt = false
		return r
	}

	joined := strings.ToLower(strings.Join(v.Failures, "\n") + " " + v.Reason)
	if strings.Contains(joined, "ambiguous") || strings.Contains(joined, "unclear requirement") {
		r.Classification = ClassAmbiguous
		r.Summary = "requirement appears ambiguous"
		r.HumanReason = "ambiguous_requirement"
		r.Actionable = false
		r.NeedsLLM = false
		return r
	}

	scope := boundScope(pkt, v.ChangedFiles)
	r.Classification = ClassProduct
	r.Actionable = true
	r.ConsumesAttempt = true
	r.RecommendedScope = scope
	r.Summary = "product verification failed"
	if len(v.Failures) > 0 {
		r.Summary = v.Failures[0]
	}
	if len(v.ChangedFiles) == 0 {
		r.Summary = "no product files changed and tests failed; likely missing implementation"
		r.Classification = ClassMissingReq
	}
	if looksLikeTestGap(joined) && len(v.ChangedFiles) > 0 {
		r.Classification = ClassTestGap
		r.Summary = "implementation present but tests failed; likely logic or test mismatch"
	}
	r.NeedsLLM = r.Classification == ClassAmbiguous
	return r
}

func looksLikeTestGap(joined string) bool {
	return strings.Contains(joined, "fail") || strings.Contains(joined, "not equal") || strings.Contains(joined, "want")
}

func boundScope(pkt packet.Packet, changed []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
			return
		}
		if seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	for _, p := range changed {
		add(p)
	}
	for _, p := range pkt.ExpectedOutputs {
		add(p)
	}
	if len(out) == 0 {
		add("*.go")
	}
	return out
}
