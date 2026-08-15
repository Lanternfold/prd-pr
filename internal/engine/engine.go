package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/apprun"
	"github.com/lanternfold/prd-pr/internal/ci"
	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/llm"
	"github.com/lanternfold/prd-pr/internal/notify"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/preflight"
	"github.com/lanternfold/prd-pr/internal/proc"
	"github.com/lanternfold/prd-pr/internal/redact"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

const (
	orchestratorModule = "module github.com/lanternfold/prd-pr"
	packetsDir         = ".project/packets"
	transcriptsDir     = ".project/transcripts"
	executionFile      = ".project/execution.json"
	reviewFile         = ".project/review.json"
	graphFile          = ".project/graph.json"
	forecastFile       = ".project/forecast.json"
	incidentFile       = ".project/incident.json"
)

// Options injects P4 dependencies. This is not the full orchestration engine.
type Options struct {
	Worker       cursor.Worker
	Git          *vcs.Client
	Now          func() time.Time
	NewID        func() string
	Timeout      time.Duration
	TestTimeout  time.Duration
	AllowSelf    bool
	PreflightEnv preflight.Env
	LookPath     func(file string) (string, error)
	ProcRunner   *proc.Runner
	Config       config.Config
	LLM          llm.Adapter
	RepairWorker cursor.Worker
	GH           *vcs.GHClient
	CI           *ci.Watcher
	Bell         *notify.Bell
	SkipWait     bool
	Cwd          string
	Home         string
	Runtime      apprun.Starter
}

// Request is one coding-worker invocation against a product workspace.
type Request struct {
	ProductRoot   string
	PRDPath       string
	PhaseID       prd.PhaseID
	Mode          string
	ExecutionMode string
	PRDOnly       bool
}

type Result struct {
	Execution        Execution
	Packet           packet.Packet
	ProjectCompleted bool
	Contract         *prd.ContractResult
	WaitingForHuman  bool
	Human            *human.Request
	ProjectType      string
	ProjectLocation  string
	Runtime          *apprun.Report
}

// Execution is the persisted P4 record.
type Execution struct {
	SchemaVersion        int                   `json:"schema_version"`
	RunID                string                `json:"run_id"`
	TaskID               string                `json:"task_id"`
	ProjectID            string                `json:"project_id"`
	PhaseID              string                `json:"phase_id"`
	ProductRoot          string                `json:"product_root"`
	Baseline             vcs.Baseline          `json:"baseline"`
	PacketRef            string                `json:"packet_ref"`
	TranscriptRef        string                `json:"transcript_ref,omitempty"`
	Invoked              bool                  `json:"invoked"`
	RefusalReason        string                `json:"refusal_reason,omitempty"`
	ExitCode             int                   `json:"exit_code"`
	DurationMS           int64                 `json:"duration_ms"`
	TimedOut             bool                  `json:"timed_out"`
	ChangedPaths         []string              `json:"changed_paths"`
	WorkerClaimedSuccess bool                  `json:"worker_claimed_success"`
	ClaimedDone          bool                  `json:"claimed_done"`
	VerifiedSuccess      bool                  `json:"verified_success"`
	CLIMechanism         string                `json:"cli_mechanism,omitempty"`
	RecordedAt           string                `json:"recorded_at"`
	IncidentID           string                `json:"incident_id,omitempty"`
	RepairAttempt        int                   `json:"repair_attempt,omitempty"`
	Subagent             string                `json:"subagent,omitempty"`
	ExecutionMode        string                `json:"execution_mode,omitempty"`
	SelfDevelopment      state.SelfDevelopment `json:"self_development,omitempty"`
}

type Engine struct {
	opts Options
}

func New(opts Options) *Engine {
	if opts.Git == nil {
		opts.Git = vcs.Default()
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	if opts.NewID == nil {
		opts.NewID = func() string {
			return fmt.Sprintf("%d", opts.Now().UnixNano())
		}
	}
	if opts.Worker == nil {
		opts.Worker = &cursor.CLI{Now: opts.Now}
	}
	if opts.Timeout <= 0 {
		opts.Timeout = cursor.DefaultTimeout
	}
	if opts.Config.MaxRepairAttempts == 0 && opts.Config.PRBoundary == "" && opts.Config.CheapModel == "" {
		opts.Config = config.Defaults()
	}
	if opts.LLM == nil {
		opts.LLM = llm.None{}
	}
	if opts.GH == nil {
		opts.GH = vcs.DefaultGH()
	}
	if opts.CI == nil {
		opts.CI = ci.Default()
	}
	if opts.Bell == nil {
		opts.Bell = &notify.Bell{Notify: func(string, string) error { return nil }}
	}
	if opts.Runtime == nil {
		opts.Runtime = apprun.ProcStarter{Runner: opts.ProcRunner}
	}
	return &Engine{opts: opts}
}

func (e *Engine) cfg() config.Config {
	c := e.opts.Config
	if c.MaxRepairAttempts == 0 {
		d := config.Defaults()
		if c.HumanTimeout == 0 {
			c.HumanTimeout = d.HumanTimeout
		}
		c.MaxRepairAttempts = d.MaxRepairAttempts
		if c.BudgetBreachPolicy == "" {
			c.BudgetBreachPolicy = d.BudgetBreachPolicy
		}
		if c.PRBoundary == "" {
			c.PRBoundary = d.PRBoundary
		}
		if c.CheapModel == "" {
			c.CheapModel = d.CheapModel
		}
		if c.StrongModel == "" {
			c.StrongModel = d.StrongModel
		}
		if c.GitHubVisibility == "" {
			c.GitHubVisibility = d.GitHubVisibility
		}
		if c.DefaultBranch == "" {
			c.DefaultBranch = d.DefaultBranch
		}
		if c.RemoteName == "" {
			c.RemoteName = d.RemoteName
		}
		if c.InitialCommitMessage == "" {
			c.InitialCommitMessage = d.InitialCommitMessage
		}
	}
	return c
}

func (e *Engine) repairWorker() cursor.Worker {
	if e.opts.RepairWorker != nil {
		return e.opts.RepairWorker
	}
	return e.opts.Worker
}

func (e *Engine) fakeWorker() bool {
	switch e.opts.Worker.(type) {
	case cursor.Fake, *cursor.Fake, *cursor.Sequence:
		return true
	default:
		return false
	}
}

func (e *Engine) Run(ctx context.Context, req Request) (Result, error) {
	if req.Mode == "" {
		req.Mode = preflight.ModeHeadless
	}
	if DeclaredSelfDevelopment(req.ExecutionMode) {
		return e.runSelfDevelopment(ctx, req)
	}
	if blocked, res := e.refuseSelfRepo(req.ProductRoot); blocked {
		return res, nil
	}
	return e.runProduct(ctx, req, false)
}

func (e *Engine) runProduct(ctx context.Context, req Request, selfDev bool) (Result, error) {
	if blocked, res := e.contractGate(req); blocked {
		return res, nil
	}
	root, jail, err := e.ensureWorkspace(ctx, req.ProductRoot, selfDev)
	if err != nil {
		return refused("", err.Error()), nil
	}
	if !e.opts.AllowSelf && !selfDev && isOrchestratorRepo(root) {
		return refused(root, "refusing to invoke a coding worker against the PRD→PR orchestrator repository"), nil
	}

	store, err := state.Open(root)
	if err != nil {
		return refused(root, err.Error()), nil
	}
	if _, err := store.Init(); err != nil {
		return refused(root, err.Error()), nil
	}
	g, err := store.Lock()
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = g.Unlock() }()

	st, err := g.Load()
	if err != nil {
		return refused(root, err.Error()), nil
	}
	st, err = e.recoverLocked(ctx, g, st)
	if err != nil {
		return Result{}, err
	}
	switch st.CurrentState {
	case state.StateWorkerClaimedDone, state.StateVerificationFailed, state.StateVerificationIncomplete,
		state.StateReviewing, state.StateRepairing, state.StateVerifying, state.StateVerified:
		if ex, err := loadExecutionFromGuard(g); err == nil && ex.Invoked {
			pkt, _ := loadPacket(root, ex.PacketRef)
			return Result{Execution: ex, Packet: pkt}, nil
		}
	}

	prep, err := e.prepareLocked(ctx, g, st, req, root, selfDev)
	if err != nil {
		return Result{}, err
	}
	if prep.Execution.RefusalReason != "" {
		return prep, nil
	}
	st, err = g.Load()
	if err != nil {
		return Result{}, err
	}

	if r, ok := e.opts.Worker.(interface{ Ready() error }); ok {
		if err := r.Ready(); err != nil {
			return e.persistRefusal(g, st, root, prep.Execution.PhaseID, err.Error())
		}
	}

	runID := prep.Execution.RunID
	phaseID := prep.Execution.PhaseID
	taskID := prep.Execution.TaskID
	packetRel := prep.Execution.PacketRef
	baseline := prep.Execution.Baseline
	pkt := prep.Packet

	if err := g.AppendEvent(state.Event{
		Kind:    state.KindIntent,
		Name:    state.EventWorkerInvoked,
		RunID:   runID,
		PhaseID: phaseID,
		Payload: state.Payload(map[string]string{
			"task_id":   taskID,
			"packet":    packetRel,
			"baseline":  baseline.SHA,
			"mechanism": cursor.PinnedMechanism,
		}),
	}); err != nil {
		return Result{}, err
	}

	st.CurrentRunID = runID
	st.CurrentPhaseID = phaseID
	st.CurrentState = state.StateWorkerRunning
	st.CurrentCommit = baseline.SHA
	if err := g.Save(st); err != nil {
		return Result{}, err
	}

	wreq := cursor.Request{
		ProductRoot: root,
		Packet:      pkt,
		PacketRel:   packetRel,
		Timeout:     e.opts.Timeout,
	}
	wres := e.opts.Worker.Run(ctx, wreq)
	wres.VerifiedSuccess = false

	changed, diffErr := e.opts.Git.ChangedSince(ctx, root, baseline.SHA, jail)
	refusal := wres.RefusalReason
	if diffErr != nil {
		if refusal == "" {
			refusal = diffErr.Error()
		}
		changed = nil
	}

	transcriptRel := filepath.Join(transcriptsDir, taskID+".txt")
	transcript := redact.String(wres.Transcript)
	if wres.Invoked {
		_ = g.WriteFile(transcriptRel, []byte(transcript+"\n"))
	} else if transcriptRel != "" {
		transcriptRel = ""
	}

	ex := Execution{
		SchemaVersion:        1,
		RunID:                runID,
		TaskID:               taskID,
		ProjectID:            st.ProjectID,
		PhaseID:              phaseID,
		ProductRoot:          root,
		Baseline:             baseline,
		PacketRef:            packetRel,
		TranscriptRef:        transcriptRel,
		Invoked:              wres.Invoked,
		RefusalReason:        refusal,
		ExitCode:             wres.ExitCode,
		DurationMS:           wres.Duration.Milliseconds(),
		TimedOut:             wres.TimedOut,
		ChangedPaths:         changed,
		WorkerClaimedSuccess: wres.WorkerClaimedSuccess,
		ClaimedDone:          wres.ClaimedDone,
		VerifiedSuccess:      false,
		CLIMechanism:         wres.CLIMechanism,
		RecordedAt:           e.opts.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeExecution(g, ex); err != nil {
		return Result{}, err
	}
	name := state.EventWorkerFinished
	if !wres.Invoked {
		name = state.EventWorkerRefused
	}
	if err := g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    name,
		RunID:   runID,
		PhaseID: phaseID,
		Payload: state.Payload(map[string]any{
			"invoked":                wres.Invoked,
			"worker_claimed_success": ex.WorkerClaimedSuccess,
			"verified_success":       false,
			"changed_paths":          changed,
		}),
	}); err != nil {
		return Result{}, err
	}
	st.CurrentState = state.StateWorkerClaimedDone
	if !wres.Invoked {
		st.CurrentState = state.StateWorkerRefused
	}
	if err := g.Save(st); err != nil {
		return Result{}, err
	}
	return Result{Execution: ex, Packet: pkt}, nil
}

func (e *Engine) persistRefusal(g *state.Guard, st state.State, root, phaseID, reason string) (Result, error) {
	ex := Execution{
		SchemaVersion:   1,
		RunID:           st.CurrentRunID,
		ProjectID:       st.ProjectID,
		PhaseID:         phaseID,
		ProductRoot:     root,
		Invoked:         false,
		RefusalReason:   reason,
		VerifiedSuccess: false,
		RecordedAt:      e.opts.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writeExecution(g, ex); err != nil {
		return Result{}, err
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventWorkerRefused,
		RunID:   st.CurrentRunID,
		PhaseID: phaseID,
		Payload: state.Payload(map[string]string{"reason": reason}),
	})
	return Result{Execution: ex}, nil
}

func writeExecution(g *state.Guard, ex Execution) error {
	data, err := json.MarshalIndent(ex, "", "  ")
	if err != nil {
		return err
	}
	return g.WriteFile(executionFile, append(data, '\n'))
}

func identifyRoot(productRoot string) (string, *fsguard.Jail, error) {
	if strings.TrimSpace(productRoot) == "" {
		return "", nil, fmt.Errorf("product root is empty")
	}
	jail, err := fsguard.New(productRoot)
	if err != nil {
		return "", nil, err
	}
	return jail.Root(), jail, nil
}

func isOrchestratorRepo(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), orchestratorModule)
}

func refused(root, reason string) Result {
	return Result{Execution: Execution{
		SchemaVersion:   1,
		ProductRoot:     root,
		Invoked:         false,
		RefusalReason:   reason,
		VerifiedSuccess: false,
	}}
}
