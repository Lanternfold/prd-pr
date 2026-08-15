package prd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lanternfold/prd-pr/internal/redact"
)

const ContractSchemaVersion = 1

type ContractStatus string

const (
	ContractValid    ContractStatus = "VALID"
	ContractRejected ContractStatus = "REJECTED"
)

type FindingSeverity string

const (
	FindBlocking FindingSeverity = "BLOCKING"
	FindWarning  FindingSeverity = "WARNING"
	FindInfo     FindingSeverity = "INFO"
)

// Stable finding IDs. Same condition always uses the same ID.
const (
	ValStructure   = "PRD-VAL-001"
	ValObjective   = "PRD-VAL-002"
	ValScope       = "PRD-VAL-003"
	ValClarity     = "PRD-VAL-004"
	ValAcceptance  = "PRD-VAL-005"
	ValConflict    = "PRD-VAL-006"
	ValRuntime     = "PRD-VAL-007"
	ValDependency  = "PRD-VAL-008"
	ValCredential  = "PRD-VAL-009"
	ValPhaseGraph  = "PRD-VAL-010"
	ValDoD         = "PRD-VAL-011"
	ValTesting     = "PRD-VAL-012"
	ValSecurity    = "PRD-VAL-013"
	ValIntegration = "PRD-VAL-014"
	ValDecision    = "PRD-VAL-015"
)

// ContractFinding is one contract-validation issue. It is not a Markdown style note.
type ContractFinding struct {
	ID             string          `json:"id"`
	Severity       FindingSeverity `json:"severity"`
	Category       string          `json:"category"`
	Location       string          `json:"location"`
	Problem        string          `json:"problem"`
	Why            string          `json:"why"`
	RequiredAction string          `json:"required_action"`
	RequirementID  string          `json:"requirement_id,omitempty"`
	PhaseID        string          `json:"phase_id,omitempty"`
}

// ContractResult is the deterministic gate result. Only BLOCKING findings reject.
type ContractResult struct {
	SchemaVersion int               `json:"schema_version"`
	Status        ContractStatus    `json:"status"`
	BlockingCount int               `json:"blocking_count"`
	WarningCount  int               `json:"warning_count"`
	InfoCount     int               `json:"info_count"`
	Findings      []ContractFinding `json:"findings"`
	SourceFile    string            `json:"source_file,omitempty"`
}

func (r ContractResult) Rejected() bool {
	return r.Status == ContractRejected
}

// ValidateContractFile reads a PRD path and returns a contract result.
// It does not create directories, Git repositories, or GitHub resources.
func ValidateContractFile(path string) (*ContractResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve PRD %s: %w", path, err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot read PRD %s: %w", path, err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("%s is a directory; pass a Markdown file", path)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot read PRD %s: %w", path, err)
	}
	doc := Parse(abs, string(data))
	res := ValidateContract(doc, string(data))
	res.SourceFile = path
	return res, nil
}

// ValidateContract evaluates whether a parsed PRD is precise enough for autonomous implementation.
func ValidateContract(doc *Document, markdown string) *ContractResult {
	res := &ContractResult{
		SchemaVersion: ContractSchemaVersion,
		Status:        ContractValid,
		Findings:      []ContractFinding{},
	}
	if doc == nil {
		res.add(finding(ValStructure, FindBlocking, "STRUCTURE", "PRD",
			"PRD could not be parsed.",
			"The orchestrator cannot implement an unreadable PRD.",
			"Provide a UTF-8 Markdown PRD with the required product sections.",
			"", ""))
		res.finalize()
		return res
	}
	res.SourceFile = doc.SourceFile
	ctx := newContractCtx(doc, markdown)
	checkStructure(ctx)
	checkObjective(ctx)
	checkScope(ctx)
	checkRequirements(ctx)
	checkAcceptance(ctx)
	checkConflicts(ctx)
	checkRuntime(ctx)
	checkDependencies(ctx)
	checkCredentials(ctx)
	checkPhases(ctx)
	checkDoD(ctx)
	checkTesting(ctx)
	checkSecurity(ctx)
	checkIntegrations(ctx)
	checkUnauthorizedDecisions(ctx)
	res.Findings = ctx.findings
	res.finalize()
	return res
}

func (r *ContractResult) add(f ContractFinding) {
	r.Findings = append(r.Findings, f)
}

func (r *ContractResult) finalize() {
	if r.Findings == nil {
		r.Findings = []ContractFinding{}
	}
	sort.SliceStable(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Location != b.Location {
			return a.Location < b.Location
		}
		if a.RequirementID != b.RequirementID {
			return a.RequirementID < b.RequirementID
		}
		if a.PhaseID != b.PhaseID {
			return a.PhaseID < b.PhaseID
		}
		return a.Problem < b.Problem
	})
	r.BlockingCount, r.WarningCount, r.InfoCount = 0, 0, 0
	for i := range r.Findings {
		r.Findings[i].Problem = redact.String(r.Findings[i].Problem)
		r.Findings[i].Why = redact.String(r.Findings[i].Why)
		r.Findings[i].RequiredAction = redact.String(r.Findings[i].RequiredAction)
		r.Findings[i].Location = redact.String(r.Findings[i].Location)
		switch r.Findings[i].Severity {
		case FindBlocking:
			r.BlockingCount++
		case FindWarning:
			r.WarningCount++
		case FindInfo:
			r.InfoCount++
		}
	}
	if r.BlockingCount > 0 {
		r.Status = ContractRejected
	} else {
		r.Status = ContractValid
	}
}

func finding(id string, sev FindingSeverity, cat, loc, problem, why, action, reqID, phaseID string) ContractFinding {
	return ContractFinding{
		ID:             id,
		Severity:       sev,
		Category:       cat,
		Location:       loc,
		Problem:        strings.TrimSpace(problem),
		Why:            strings.TrimSpace(why),
		RequiredAction: strings.TrimSpace(action),
		RequirementID:  reqID,
		PhaseID:        phaseID,
	}
}

type contractCtx struct {
	doc      *Document
	markdown string
	lower    string
	findings []ContractFinding
}

func newContractCtx(doc *Document, markdown string) *contractCtx {
	if markdown == "" && doc != nil {
		var b strings.Builder
		for _, s := range doc.Sections {
			b.WriteString(s.Title)
			b.WriteByte('\n')
			b.WriteString(s.Body)
			b.WriteByte('\n')
		}
		markdown = b.String()
	}
	return &contractCtx{doc: doc, markdown: markdown, lower: strings.ToLower(markdown)}
}

func (c *contractCtx) add(f ContractFinding) {
	c.findings = append(c.findings, f)
}

func (c *contractCtx) section(key string) *Section {
	for i := range c.doc.Sections {
		if c.doc.Sections[i].Key == key {
			return &c.doc.Sections[i]
		}
	}
	return nil
}

func (c *contractCtx) hasSection(key string) bool {
	return c.section(key) != nil
}

func (c *contractCtx) sectionBody(key string) string {
	if s := c.section(key); s != nil {
		return s.Body
	}
	return ""
}

func locOf(src SourceRef, fallback string) string {
	if src.Section != "" && src.StartLine > 0 {
		return fmt.Sprintf("%s:%d", src.Section, src.StartLine)
	}
	if src.StartLine > 0 {
		return fmt.Sprintf("line %d", src.StartLine)
	}
	if src.Section != "" {
		return src.Section
	}
	return fallback
}

func nonempty(s string) bool {
	return strings.TrimSpace(s) != ""
}
