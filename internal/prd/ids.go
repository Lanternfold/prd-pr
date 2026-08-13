package prd

import (
	"fmt"
	"regexp"
	"strings"
)

// Typed identifiers. Values preserve the source spelling exactly.
type (
	RequirementID string
	AcceptanceID  string
	TestID        string
	PhaseID       string
)

var (
	reReqID   = regexp.MustCompile(`^REQ-[0-9]+$`)
	reACID    = regexp.MustCompile(`^AC-[0-9]+$`)
	reTestID  = regexp.MustCompile(`^TEST-[0-9]+$`)
	rePhaseID = regexp.MustCompile(`^P[0-9]+[A-Z]?$`)
	reIDToken = regexp.MustCompile(`^(REQ|AC|TEST)-[A-Za-z0-9_-]+$`)
)

func ParseRequirementID(s string) (RequirementID, error) {
	s = strings.TrimSpace(s)
	if reReqID.MatchString(s) {
		return RequirementID(s), nil
	}
	return "", fmt.Errorf("invalid requirement id %q", s)
}

func ParseAcceptanceID(s string) (AcceptanceID, error) {
	s = strings.TrimSpace(s)
	if reACID.MatchString(s) {
		return AcceptanceID(s), nil
	}
	return "", fmt.Errorf("invalid acceptance criteria id %q", s)
}

func ParseTestID(s string) (TestID, error) {
	s = strings.TrimSpace(s)
	if reTestID.MatchString(s) {
		return TestID(s), nil
	}
	return "", fmt.Errorf("invalid test id %q", s)
}

func ParsePhaseID(s string) (PhaseID, error) {
	s = strings.TrimSpace(s)
	if rePhaseID.MatchString(s) {
		return PhaseID(s), nil
	}
	return "", fmt.Errorf("invalid phase id %q", s)
}

func looksLikeIDToken(s string) bool {
	return reIDToken.MatchString(strings.TrimSpace(s))
}
