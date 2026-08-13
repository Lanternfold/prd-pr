package preflight

const SchemaVersion = 1

type Scope string

const (
	ScopeMachine Scope = "MACHINE"
	ScopeProject Scope = "PROJECT"
)

// Status is a deterministic check classification.
type Status string

const (
	StatusAvailable Status = "AVAILABLE"
	StatusMissing   Status = "MISSING"
	StatusOptional  Status = "OPTIONAL"
	StatusBlocking  Status = "BLOCKING"
	StatusWarning   Status = "WARNING"
	StatusError     Status = "ERROR"
)

const (
	OverallReady   = "READY"
	OverallBlocked = "BLOCKED"
)

// Check is one environment or project observation.
type Check struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Scope    Scope  `json:"scope"`
	Status   Status `json:"status"`
	Blocking bool   `json:"blocking"`
	Detail   string `json:"detail,omitempty"`
}

type RepositoryInfo struct {
	State      string   `json:"state"`
	Toplevel   string   `json:"toplevel,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	HeadSHA    string   `json:"head_sha,omitempty"`
	Dirty      bool     `json:"dirty"`
	DirtyPaths []string `json:"dirty_paths,omitempty"`
}

// Report is the structured P3 readiness document. It contains no secrets.
type Report struct {
	SchemaVersion int              `json:"schema_version"`
	Timestamp     string           `json:"timestamp"`
	ProjectRoot   string           `json:"project_root"`
	ProjectName   string           `json:"project_name,omitempty"`
	PRDPath       string           `json:"prd_path,omitempty"`
	Repository    *RepositoryInfo  `json:"repository,omitempty"`
	Checks        []Check          `json:"checks"`
	Status        string           `json:"status"`
	Findings      []string         `json:"findings"`
	Blocking      []string         `json:"blocking_issues"`
	Warnings      []string         `json:"warnings"`
	NextAction    string           `json:"recommended_next_action"`
}

func (r *Report) add(c Check) {
	r.Checks = append(r.Checks, c)
	msg := c.Name
	if c.Detail != "" {
		msg = c.Name + ": " + c.Detail
	}
	r.Findings = append(r.Findings, msg)
	if c.Blocking || c.Status == StatusBlocking || c.Status == StatusError {
		r.Blocking = append(r.Blocking, msg)
	}
	if c.Status == StatusWarning {
		r.Warnings = append(r.Warnings, msg)
	}
}

func (r *Report) finalize() {
	if len(r.Blocking) > 0 {
		r.Status = OverallBlocked
		r.NextAction = "Resolve blocking issues, then re-run preflight. P4 still enforces Git baseline before Cursor writes."
		return
	}
	r.Status = OverallReady
	r.NextAction = "Environment is ready for PRD→PR execution. P4 still enforces Git baseline before Cursor writes."
}
