package testeng

import (
	"os/exec"
	"time"

	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/proc"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

const (
	SchemaVersion = 1
	MaxLogBytes   = 64 * 1024
	DefaultTimeout = 2 * time.Minute
)

// Overall status values.
const (
	StatusVerified               = "VERIFIED"
	StatusFailed                 = "FAILED"
	StatusIncomplete             = "VERIFICATION_INCOMPLETE"
	StatusTimeout                = "TEST_TIMEOUT"
	StatusUnsupported            = "UNSUPPORTED_PROJECT"
	StatusInfrastructure         = "INFRASTRUCTURE_ERROR"
	StatusManual                 = "MANUAL_VERIFICATION_REQUIRED"
)

// Check outcomes.
const (
	OutcomePass     = "PASS"
	OutcomeFail     = "FAIL"
	OutcomeTimeout  = "TIMEOUT"
	OutcomeSkip     = "SKIP"
	OutcomeManual   = "MANUAL_VERIFICATION_REQUIRED"
	OutcomeError    = "ERROR"
	OutcomeCovered  = "COVERED_BY_PROJECT_TESTS"
	OutcomeIgnored  = "IGNORED"
)

const (
	KindGoTest     = "go_test"
	KindTestID     = "test_id"
	KindAcceptance = "acceptance"
	KindFileExists = "file_exists"
	KindGoSymbol   = "go_symbol"
	KindDoD        = "dod"
	KindMalformed  = "malformed_test_config"
)

// Options injects process, PATH, and clock dependencies.
type Options struct {
	Git      *vcs.Client
	Runner   *proc.Runner
	LookPath func(file string) (string, error)
	Now      func() time.Time
	Timeout  time.Duration
}

func (o Options) lookPath() func(string) (string, error) {
	if o.LookPath != nil {
		return o.LookPath
	}
	return exec.LookPath
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now().UTC()
}

func (o Options) timeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return DefaultTimeout
}

func (o Options) runner() *proc.Runner {
	if o.Runner != nil {
		return o.Runner
	}
	return &proc.Runner{}
}

func (o Options) git() *vcs.Client {
	if o.Git != nil {
		return o.Git
	}
	return vcs.Default()
}

// Request is one independent verification of a prepared packet.
type Request struct {
	ProductRoot string
	Packet      packet.Packet
	RunID       string
	TaskID      string
	PhaseID     string
	BaselineSHA     string
	Jail            *fsguard.Jail
	ManualConfirmed bool
}

// Command is a structured subprocess (never a PRD shell string).
type Command struct {
	Program string   `json:"program"`
	Args    []string `json:"args,omitempty"`
}

// Check is one planned or executed verification step.
type Check struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Name     string  `json:"name"`
	Required bool    `json:"required"`
	Command  Command `json:"command,omitempty"`
	Outcome  string  `json:"outcome,omitempty"`
	Detail   string  `json:"detail,omitempty"`
	ExitCode int     `json:"exit_code,omitempty"`
	DurationMS int64 `json:"duration_ms,omitempty"`
	StdoutRef  string `json:"stdout_ref,omitempty"`
	StderrRef  string `json:"stderr_ref,omitempty"`
	Stdout     string `json:"-"`
	Stderr     string `json:"-"`
}

// Plan is the deterministic verification plan. It does not execute tests.
type Plan struct {
	ProjectRoot string  `json:"project_root"`
	TaskID      string  `json:"task_id"`
	PhaseID     string  `json:"phase_id"`
	Kind        string  `json:"project_kind"`
	Checks      []Check `json:"checks"`
}

// Result is independent of worker_claimed_success.
type Result struct {
	SchemaVersion                      int           `json:"schema_version"`
	Status                             string        `json:"status"`
	RunID                              string        `json:"run_id,omitempty"`
	TaskID                             string        `json:"task_id,omitempty"`
	PhaseID                            string        `json:"phase_id,omitempty"`
	Timestamp                          string        `json:"timestamp"`
	BaselineSHA                        string        `json:"baseline_sha,omitempty"`
	HeadSHA                            string        `json:"head_sha,omitempty"`
	ChangedFiles                       []string      `json:"changed_files,omitempty"`
	UntrackedFiles                     []string      `json:"untracked_files,omitempty"`
	DeletedFiles                       []string      `json:"deleted_files,omitempty"`
	Plan                               Plan          `json:"plan"`
	Checks                             []Check       `json:"checks"`
	Failures                           []string      `json:"failures,omitempty"`
	TestsPass                          bool          `json:"tests_pass"`
	AllVerifiableAcceptanceCriteriaPass bool         `json:"all_verifiable_acceptance_criteria_pass"`
	ManualVerificationRequired         bool          `json:"manual_verification_required"`
	VerifiedSuccess                    bool          `json:"verified_success"`
	Reason                             string        `json:"reason,omitempty"`
}

type Engine struct {
	opts Options
}

func New(opts Options) *Engine {
	return &Engine{opts: opts}
}
