package prd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) *Document {
	t.Helper()
	p := filepath.Join("testdata", "prd", name)
	doc, err := ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func hasCode(doc *Document, code string) bool {
	for _, d := range doc.Diagnostics {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestMinimalValid(t *testing.T) {
	doc := loadFixture(t, "minimal_valid.md")
	if doc.HasErrors() {
		t.Fatalf("expected valid, diagnostics=%v", doc.Diagnostics)
	}
	if doc.Metadata.Product != "Fixture" {
		t.Fatalf("product = %q", doc.Metadata.Product)
	}
	if len(doc.Requirements) != 2 || doc.Requirements[0].ID != "REQ-001" {
		t.Fatalf("requirements = %+v", doc.Requirements)
	}
	if len(doc.Acceptance) != 2 {
		t.Fatalf("ac = %+v", doc.Acceptance)
	}
	if len(doc.Tests) != 1 || doc.Tests[0].ID != "TEST-001" {
		t.Fatalf("tests = %+v", doc.Tests)
	}
	if len(doc.Phases) != 1 || doc.Phases[0].ID != "P1" || doc.Phases[0].Name != "Core" {
		t.Fatalf("phases = %+v", doc.Phases)
	}
	if len(doc.Goals) == 0 {
		t.Fatal("expected goals")
	}
	if doc.SchemaVersion != SchemaVersion {
		t.Fatalf("schema = %d", doc.SchemaVersion)
	}
}

func TestCurrentFormatNumberedOutline(t *testing.T) {
	doc := loadFixture(t, "current_format.md")
	foundGoals := false
	for _, s := range doc.Sections {
		if s.Key == "goals" {
			foundGoals = true
		}
	}
	if !foundGoals {
		t.Fatalf("numbered Goals section not recognized: %+v", sectionKeys(doc))
	}
	if len(doc.Requirements) != 1 {
		t.Fatalf("req = %+v", doc.Requirements)
	}
}

func TestMissingOptionalIsInfo(t *testing.T) {
	doc := loadFixture(t, "missing_optional.md")
	if doc.HasErrors() {
		t.Fatalf("warnings should not be fatal: %v", doc.Diagnostics)
	}
	if !hasCode(doc, "MISSING_OPTIONAL_SECTION") {
		t.Fatalf("expected INFO missing optional: %v", doc.Diagnostics)
	}
}

func TestDuplicateRequirement(t *testing.T) {
	doc := loadFixture(t, "duplicate_req.md")
	if !hasCode(doc, "REQ_DUPLICATE") {
		t.Fatalf("expected REQ_DUPLICATE: %v", doc.Diagnostics)
	}
	if doc.Status() != "INVALID" {
		t.Fatal(doc.Status())
	}
}

func TestDuplicatePhase(t *testing.T) {
	doc := loadFixture(t, "duplicate_phase.md")
	if !hasCode(doc, "PHASE_DUPLICATE") {
		t.Fatalf("expected PHASE_DUPLICATE: %v", doc.Diagnostics)
	}
}

func TestMalformedRequirement(t *testing.T) {
	doc := loadFixture(t, "malformed_req.md")
	if !hasCode(doc, "REQ_MALFORMED") {
		t.Fatalf("expected REQ_MALFORMED: %v", doc.Diagnostics)
	}
}

func TestMalformedPhase(t *testing.T) {
	doc := loadFixture(t, "malformed_phase.md")
	if !hasCode(doc, "PHASE_MALFORMED") {
		t.Fatalf("expected PHASE_MALFORMED: %v", doc.Diagnostics)
	}
}

func TestUnknownSectionPreserved(t *testing.T) {
	doc := loadFixture(t, "unknown_section.md")
	if !hasCode(doc, "UNKNOWN_SECTION") {
		t.Fatalf("expected UNKNOWN_SECTION: %v", doc.Diagnostics)
	}
	found := false
	for _, s := range doc.Sections {
		if strings.Contains(strings.ToLower(s.Title), "alien") {
			found = true
			if !strings.Contains(s.Body, "Do not treat") {
				t.Fatalf("unknown section body lost: %q", s.Body)
			}
		}
	}
	if !found {
		t.Fatalf("unknown section not preserved: %+v", sectionKeys(doc))
	}
}

func TestUnknownRequirementRef(t *testing.T) {
	doc := loadFixture(t, "unknown_req_ref.md")
	if !hasCode(doc, "UNKNOWN_REQ_REF") {
		t.Fatalf("expected UNKNOWN_REQ_REF: %v", doc.Diagnostics)
	}
	if doc.HasErrors() {
		t.Fatalf("unknown ref is a warning, got errors %v", doc.Diagnostics)
	}
}

func TestMissingRequiredSection(t *testing.T) {
	doc := loadFixture(t, "missing_required.md")
	if !hasCode(doc, "MISSING_REQUIRED_SECTION") {
		t.Fatalf("expected MISSING_REQUIRED_SECTION: %v", doc.Diagnostics)
	}
}

func TestInvalidIdentifier(t *testing.T) {
	doc := loadFixture(t, "invalid_id.md")
	if !hasCode(doc, "INVALID_IDENTIFIER") {
		t.Fatalf("expected INVALID_IDENTIFIER: %v", doc.Diagnostics)
	}
}

func TestMixedValidInvalid(t *testing.T) {
	doc := loadFixture(t, "mixed.md")
	if !hasCode(doc, "REQ_DUPLICATE") || !hasCode(doc, "UNKNOWN_SECTION") || !hasCode(doc, "UNKNOWN_REQ_REF") {
		t.Fatalf("expected mixed diagnostics: %v", doc.Diagnostics)
	}
	if len(doc.Requirements) < 2 {
		t.Fatalf("valid requirements should still be extracted: %+v", doc.Requirements)
	}
}

func TestSourceLineMapping(t *testing.T) {
	doc := loadFixture(t, "source_lines.md")
	if len(doc.Requirements) != 1 {
		t.Fatalf("req = %+v", doc.Requirements)
	}
	if doc.Requirements[0].ID != "REQ-010" {
		t.Fatalf("id = %s", doc.Requirements[0].ID)
	}
	if doc.Requirements[0].Source.StartLine != 15 {
		t.Fatalf("start_line = %d, want 15", doc.Requirements[0].Source.StartLine)
	}
}

func TestDeterministicRepeatedParse(t *testing.T) {
	p := filepath.Join("testdata", "prd", "minimal_valid.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	a := Parse(p, string(b))
	c := Parse(p, string(b))
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	jc, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if string(ja) != string(jc) {
		t.Fatal("repeated parse produced different JSON")
	}
}

func TestDoesNotInventIDs(t *testing.T) {
	doc := Parse("x.md", "# PRD: X\n\n# 1. Product Overview\n\nNeed a login page.\n\n# 2. Goals\n\n- Ship\n")
	if len(doc.Requirements) != 0 {
		t.Fatalf("invented requirements: %+v", doc.Requirements)
	}
}

func TestDoesNotExtractProseMentions(t *testing.T) {
	md := "# PRD: X\n\n# 1. Product Overview\n\nWhich tests prove that REQ-004 works?\n\n# 2. Goals\n\n- Ship\n"
	doc := Parse("x.md", md)
	if len(doc.Requirements) != 0 {
		t.Fatalf("prose mention became a requirement: %+v", doc.Requirements)
	}
}

func TestFencedCodeNotParsedAsSection(t *testing.T) {
	md := "# PRD: X\n\n# 1. Product Overview\n\n```text\n# Alien Heading\nREQ-001: fake\n```\n\n# 2. Goals\n\n- Ship\n"
	doc := Parse("x.md", md)
	for _, s := range doc.Sections {
		if strings.Contains(strings.ToLower(s.Title), "alien") {
			t.Fatalf("fenced heading became a section: %+v", s)
		}
	}
	if len(doc.Requirements) != 0 {
		t.Fatalf("fenced REQ became a requirement: %+v", doc.Requirements)
	}
}

func TestLargePRD(t *testing.T) {
	var b strings.Builder
	b.WriteString("# PRD: Large\n\n**Product:** Large\n\n# 1. Product Overview\n\nBig.\n\n# 2. Goals\n\n- Scale\n\n# 3. Requirements\n\n")
	for i := 1; i <= 40; i++ {
		b.WriteString(fmtReq(i))
	}
	b.WriteString("\n# 4. Phases\n\n")
	for i := 1; i <= 12; i++ {
		b.WriteString(fmtPhase(i))
	}
	doc := Parse("large.md", b.String())
	if len(doc.Requirements) != 40 {
		t.Fatalf("got %d requirements", len(doc.Requirements))
	}
	if len(doc.Phases) != 12 {
		t.Fatalf("got %d phases", len(doc.Phases))
	}
	if doc.HasErrors() {
		t.Fatalf("large valid PRD had errors: %v", doc.Diagnostics)
	}
}

func TestRepoPRD(t *testing.T) {
	p := filepath.Join("..", "..", "PRD.md")
	doc, err := ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Sections) < 20 {
		t.Fatalf("expected many sections from repo PRD, got %d (%v)", len(doc.Sections), sectionKeys(doc))
	}
	if len(doc.Phases) < 10 {
		t.Fatalf("expected development phases, got %d", len(doc.Phases))
	}
	// Example IDs in the contract section must not be invented as full requirements.
	for _, r := range doc.Requirements {
		if r.Text == "" && r.Title == "" {
			t.Fatalf("empty requirement extracted from repo PRD: %+v", r)
		}
	}
}

func sectionKeys(doc *Document) []string {
	var k []string
	for _, s := range doc.Sections {
		k = append(k, s.Key+":"+s.Title)
	}
	return k
}

func fmtReq(i int) string {
	return "- REQ-" + pad3(i) + ": Requirement number " + pad3(i) + "\n"
}

func fmtPhase(i int) string {
	return "P" + itoa(i) + ": Phase " + itoa(i) + "\nObjective: do work " + itoa(i) + "\n\n"
}

func pad3(i int) string {
	s := itoa(i)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var d [12]byte
	n := len(d)
	for i > 0 {
		n--
		d[n] = byte('0' + i%10)
		i /= 10
	}
	return string(d[n:])
}
