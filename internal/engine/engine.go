package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/plan"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/redact"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

const (
	orchestratorModule = "module github.com/lanternfold/prd-pr"
	packetsDir         = ".project/packets"
	transcriptsDir     = ".project/transcripts"
	executionFile      = ".project/execution.json"
)

// Options injects P4 dependencies. This is not the full orchestration engine.
type Options struct {
	Worker    cursor.Worker
	Git       *vcs.Client
	Now       func() time.Time
	NewID     func() string
	Timeout   time.Duration
	AllowSelf bool
}

// Request is one coding-worker invocation against a product workspace.
type Request struct {
	ProductRoot string
	PRDPath     string
	PhaseID     prd.PhaseID
}

// Execution is the persisted P4 record.
type Execution struct {
	SchemaVersion        int          `json:"schema_version"`
	RunID                string       `json:"run_id"`
	TaskID               string       `json:"task_id"`
	ProjectID            string       `json:"project_id"`
	PhaseID              string       `json:"phase_id"`
	ProductRoot          string       `json:"product_root"`
	Baseline             vcs.Baseline `json:"baseline"`
	PacketRef            string       `json:"packet_ref"`
	TranscriptRef        string       `json:"transcript_ref,omitempty"`
	Invoked              bool         `json:"invoked"`
	RefusalReason        string       `json:"refusal_reason,omitempty"`
	ExitCode             int          `json:"exit_code"`
	DurationMS           int64        `json:"duration_ms"`
	TimedOut             bool         `json:"timed_out"`
	ChangedPaths         []string     `json:"changed_paths"`
	WorkerClaimedSuccess bool         `json:"worker_claimed_success"`
	ClaimedDone          bool         `json:"claimed_done"`
	VerifiedSuccess      bool         `json:"verified_success"`
	CLIMechanism         string       `json:"cli_mechanism,omitempty"`
	RecordedAt           string       `json:"recorded_at"`
}

type Result struct {
	Execution Execution
	Packet    packet.Packet
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
	return &Engine{opts: opts}
}

func (e *Engine) Run(ctx context.Context, req Request) (Result, error) {
	root, jail, err := identifyRoot(req.ProductRoot)
	if err != nil {
		return refused("", err.Error()), nil
	}
	if !e.opts.AllowSelf && isOrchestratorRepo(root) {
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

	prdPath := req.PRDPath
	if prdPath == "" {
		prdPath = filepath.Join(root, "PRD.md")
	}
	doc, err := prd.ParseFile(prdPath)
	if err != nil {
		return e.persistRefusal(g, st, root, "", "cannot read PRD: "+err.Error())
	}
	if doc.HasErrors() {
		return e.persistRefusal(g, st, root, "", "PRD is invalid; refusing to invoke Cursor")
	}
	phaseID := req.PhaseID
	if phaseID == "" {
		if len(doc.Phases) == 0 {
			return e.persistRefusal(g, st, root, "", "PRD has no phases")
		}
		phaseID = doc.Phases[0].ID
	}

	baseline, _, err := e.opts.Git.EstablishBaseline(ctx, root)
	if err != nil {
		return e.persistRefusal(g, st, root, string(phaseID), "git baseline: "+err.Error())
	}

	runID := "run_" + e.opts.NewID()
	taskID := "task_" + e.opts.NewID()
	pkt, err := plan.DeterministicPlanner{}.Packet(plan.Input{
		Document:    doc,
		PhaseID:     phaseID,
		ProjectID:   st.ProjectID,
		TaskID:      taskID,
		ProductRoot: root,
	})
	if err != nil {
		return e.persistRefusal(g, st, root, string(phaseID), err.Error())
	}
	packetRel := filepath.Join(packetsDir, taskID+".json")
	packetBytes, err := packet.Marshal(pkt)
	if err != nil {
		return e.persistRefusal(g, st, root, string(phaseID), err.Error())
	}
	if err := g.WriteFile(packetRel, packetBytes); err != nil {
		return e.persistRefusal(g, st, root, string(phaseID), err.Error())
	}

	if r, ok := e.opts.Worker.(interface{ Ready() error }); ok {
		if err := r.Ready(); err != nil {
			return e.persistRefusal(g, st, root, string(phaseID), err.Error())
		}
	}

	if err := g.AppendEvent(state.Event{
		Kind:    state.KindIntent,
		Name:    state.EventWorkerInvoked,
		RunID:   runID,
		PhaseID: string(phaseID),
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
	st.CurrentPhaseID = string(phaseID)
	st.CurrentState = "IMPLEMENTING"
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
		PhaseID:              string(phaseID),
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
		PhaseID: string(phaseID),
		Payload: state.Payload(map[string]any{
			"invoked":                wres.Invoked,
			"worker_claimed_success": ex.WorkerClaimedSuccess,
			"verified_success":       false,
			"changed_paths":          changed,
		}),
	}); err != nil {
		return Result{}, err
	}
	st.CurrentState = "WORKER_COMPLETED"
	if !wres.Invoked {
		st.CurrentState = "WORKER_REFUSED"
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
