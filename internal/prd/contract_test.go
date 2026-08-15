package prd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contractFixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", "prd", "contract", name)
}

func loadContract(t *testing.T, name string) *ContractResult {
	t.Helper()
	res, err := ValidateContractFile(contractFixture(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func hasFinding(r *ContractResult, id string, sev FindingSeverity) bool {
	for _, f := range r.Findings {
		if f.ID == id && (sev == "" || f.Severity == sev) {
			return true
		}
	}
	return false
}

func TestContractPassFixtures(t *testing.T) {
	for _, name := range []string{
		"pass_complete.md",
		"pass_safe_defaults.md",
		"pass_deterministic_ac.md",
		"pass_independent_phases.md",
	} {
		t.Run(name, func(t *testing.T) {
			res := loadContract(t, name)
			if res.Status != ContractValid {
				t.Fatalf("status=%s findings=%+v", res.Status, res.Findings)
			}
			if res.BlockingCount != 0 {
				t.Fatalf("blocking=%d %+v", res.BlockingCount, res.Findings)
			}
		})
	}
}

func TestContractExistingMinimalValidPasses(t *testing.T) {
	res, err := ValidateContractFile(filepath.Join("testdata", "prd", "minimal_valid.md"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != ContractValid {
		t.Fatalf("minimal_valid rejected: %+v", res.Findings)
	}
}

func TestContractSafeDefaultsAreWarnings(t *testing.T) {
	res := loadContract(t, "pass_safe_defaults.md")
	if res.Status != ContractValid {
		t.Fatalf("%+v", res.Findings)
	}
	if !hasFinding(res, ValRuntime, FindWarning) {
		t.Fatalf("expected runtime warning, findings=%+v", res.Findings)
	}
}

func TestContractRejectFixtures(t *testing.T) {
	cases := []struct {
		file string
		id   string
	}{
		{"reject_missing_objective.md", ValObjective},
		{"reject_ambiguous_platform.md", ValRuntime},
		{"reject_untestable.md", ValClarity},
		{"reject_missing_acceptance.md", ValAcceptance},
		{"reject_conflict.md", ValConflict},
		{"reject_missing_credential.md", ValCredential},
		{"reject_phase_cycle.md", ValPhaseGraph},
		{"reject_undefined_integration.md", ValIntegration},
		{"reject_security.md", ValSecurity},
		{"reject_missing_dod.md", ValDoD},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			res := loadContract(t, tc.file)
			if res.Status != ContractRejected {
				t.Fatalf("want REJECTED, got %s findings=%+v", res.Status, res.Findings)
			}
			if !hasFinding(res, tc.id, FindBlocking) {
				t.Fatalf("missing blocking %s in %+v", tc.id, res.Findings)
			}
		})
	}
}

func TestContractDeterministicJSONAndStableIDs(t *testing.T) {
	a := loadContract(t, "reject_conflict.md")
	b := loadContract(t, "reject_conflict.md")
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ja, jb) {
		t.Fatalf("non-deterministic JSON\n%s\n%s", ja, jb)
	}
	if !hasFinding(a, ValConflict, FindBlocking) {
		t.Fatal("stable ID PRD-VAL-006 missing")
	}
}

func TestContractJSONContainsFindings(t *testing.T) {
	res := loadContract(t, "reject_missing_acceptance.md")
	var buf bytes.Buffer
	if err := FormatContractJSON(&buf, res); err != nil {
		t.Fatal(err)
	}
	var decoded ContractResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Status != ContractRejected || decoded.BlockingCount == 0 || len(decoded.Findings) == 0 {
		t.Fatalf("%+v", decoded)
	}
}

func TestContractDoesNotMutatePRD(t *testing.T) {
	src := contractFixture(t, "pass_complete.md")
	dir := t.TempDir()
	dst := filepath.Join(dir, "PRD.md")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateContractFile(dst); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, after) {
		t.Fatal("PRD was mutated")
	}
}

func TestContractHumanRejectedAndValid(t *testing.T) {
	rej := loadContract(t, "reject_missing_objective.md")
	var buf bytes.Buffer
	if err := FormatContract(&buf, rej); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, s := range []string{"PRD CONTRACT VALIDATION", "Status: REJECTED", "Blocking issues:", "Required action:", "Update the PRD"} {
		if !strings.Contains(out, s) {
			t.Fatalf("missing %q in\n%s", s, out)
		}
	}
	ok := loadContract(t, "pass_complete.md")
	buf.Reset()
	if err := FormatContract(&buf, ok); err != nil {
		t.Fatal(err)
	}
	out = buf.String()
	if !strings.Contains(out, "Status: VALID") || !strings.Contains(out, "The PRD may proceed to project bootstrap.") {
		t.Fatalf("%s", out)
	}
	if strings.Contains(out, "Update the PRD to resolve the blocking issues.") {
		t.Fatal("valid report must not demand PRD updates")
	}
}

func TestContractRedactsSecrets(t *testing.T) {
	md := `# PRD: Secret

**Product:** Secret

# 1. Product Overview

Uses OpenAI with token=sk-abcdefghijklmnopqrstuvwxyz123456 when the user submits text. The product must send the prompt to OpenAI and return the assistant text. On failure it must exit non-zero.

# 2. Goals

- Summarize

# 3. Non-Goals

- UI

# 4. Dependencies

- openai

# 5. Requirements

- REQ-001: When the user submits text, the product must send the prompt to OpenAI and return the assistant text
- REQ-002: On failure the CLI must exit non-zero

# 6. Acceptance Criteria

- AC-001: success prints assistant text
- AC-002: failure exits non-zero

# 7. Testing

- TEST-001: fake transport

# 8. Phases

## P1: Core

Objective: Summarize
Requirements: REQ-001, REQ-002
Acceptance Criteria: AC-001, AC-002
Tests: TEST-001
Definition of Done:
- tests pass
`
	doc := Parse("secret.md", md)
	res := ValidateContract(doc, md)
	raw, _ := json.Marshal(res)
	if strings.Contains(string(raw), "sk-abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatal("secret leaked into contract JSON")
	}
}
