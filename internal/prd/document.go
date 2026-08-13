package prd

const SchemaVersion = 1

type Severity string

const (
	SevError   Severity = "ERROR"
	SevWarning Severity = "WARNING"
	SevInfo    Severity = "INFO"
)

type SectionKind string

const (
	KindRequired SectionKind = "required"
	KindOptional SectionKind = "optional"
	KindUnknown  SectionKind = "unknown"
	KindPreamble SectionKind = "preamble"
)

// SourceRef points at a span in the original Markdown. It does not embed the text.
type SourceRef struct {
	File      string `json:"file,omitempty"`
	Section   string `json:"section,omitempty"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type Diagnostic struct {
	Severity  Severity `json:"severity"`
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	File      string   `json:"file,omitempty"`
	Section   string   `json:"section,omitempty"`
	StartLine int      `json:"start_line,omitempty"`
	EndLine   int      `json:"end_line,omitempty"`
}

type Metadata struct {
	Title      string `json:"title,omitempty"`
	Status     string `json:"status,omitempty"`
	Product    string `json:"product,omitempty"`
	Owner      string `json:"owner,omitempty"`
	Repository string `json:"repository,omitempty"`
}

type Section struct {
	Key    string      `json:"key"`
	Title  string      `json:"title"`
	Kind   SectionKind `json:"kind"`
	Source SourceRef   `json:"source"`
	Body   string      `json:"body"`
}

type Requirement struct {
	ID     RequirementID `json:"id"`
	Title  string        `json:"title,omitempty"`
	Text   string        `json:"text,omitempty"`
	Source SourceRef     `json:"source"`
}

type Criterion struct {
	ID     AcceptanceID `json:"id"`
	Title  string       `json:"title,omitempty"`
	Text   string       `json:"text,omitempty"`
	Source SourceRef    `json:"source"`
}

type TestItem struct {
	ID     TestID    `json:"id"`
	Title  string    `json:"title,omitempty"`
	Text   string    `json:"text,omitempty"`
	Source SourceRef `json:"source"`
}

type Phase struct {
	ID                 PhaseID         `json:"id"`
	Name               string          `json:"name,omitempty"`
	Objective          string          `json:"objective,omitempty"`
	Dependencies       []PhaseID       `json:"dependencies,omitempty"`
	Inputs             []string        `json:"inputs,omitempty"`
	Outputs            []string        `json:"outputs,omitempty"`
	Requirements       []RequirementID `json:"requirements,omitempty"`
	AcceptanceCriteria []AcceptanceID  `json:"acceptance_criteria,omitempty"`
	Tasks              []string        `json:"tasks,omitempty"`
	Tests              []TestID        `json:"tests,omitempty"`
	DesignWork         []string        `json:"design_work,omitempty"`
	Risks              []string        `json:"risks,omitempty"`
	HumanValidation    string          `json:"human_validation,omitempty"`
	DefinitionOfDone   []string        `json:"definition_of_done,omitempty"`
	Source             SourceRef       `json:"source"`
}

type Dependency struct {
	Name   string    `json:"name"`
	Class  string    `json:"class,omitempty"`
	Source SourceRef `json:"source"`
}

type CredentialInfo struct {
	Name   string    `json:"name"`
	Source SourceRef `json:"source"`
}

type DesignInfo struct {
	Present bool   `json:"present"`
	Summary string `json:"summary,omitempty"`
}

type TechnicalInfo struct {
	Present bool   `json:"present"`
	Summary string `json:"summary,omitempty"`
}

type TestingInfo struct {
	Present bool   `json:"present"`
	Summary string `json:"summary,omitempty"`
}

type HumanInfo struct {
	Present bool   `json:"present"`
	Summary string `json:"summary,omitempty"`
}

// Document is the structured PRD. Diagnostics are part of the result, not a panic.
type Document struct {
	SchemaVersion int              `json:"schema_version"`
	SourceFile    string           `json:"source_file,omitempty"`
	Metadata      Metadata         `json:"metadata"`
	Sections      []Section        `json:"sections"`
	Goals         []string         `json:"goals,omitempty"`
	NonGoals      []string         `json:"non_goals,omitempty"`
	Requirements  []Requirement    `json:"requirements,omitempty"`
	Acceptance    []Criterion      `json:"acceptance_criteria,omitempty"`
	Tests         []TestItem       `json:"tests,omitempty"`
	Phases        []Phase          `json:"phases,omitempty"`
	Dependencies  []Dependency     `json:"dependencies,omitempty"`
	Design        DesignInfo       `json:"design"`
	Technical     TechnicalInfo    `json:"technical"`
	Testing       TestingInfo      `json:"testing"`
	Human         HumanInfo        `json:"human"`
	Credentials   []CredentialInfo `json:"credentials,omitempty"`
	Diagnostics   []Diagnostic     `json:"diagnostics"`
}

func (d Document) HasErrors() bool {
	for _, diag := range d.Diagnostics {
		if diag.Severity == SevError {
			return true
		}
	}
	return false
}

func (d Document) Status() string {
	if d.HasErrors() {
		return "INVALID"
	}
	return "VALID"
}

func (d Document) Counts() (warnings, errors int) {
	for _, diag := range d.Diagnostics {
		switch diag.Severity {
		case SevWarning:
			warnings++
		case SevError:
			errors++
		}
	}
	return warnings, errors
}
