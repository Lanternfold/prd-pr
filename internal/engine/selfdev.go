package engine

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

const (
	selfDevDeclarationPrefix = "execution mode:"
	orchestratorIdentity     = "github.com/lanternfold/prd-pr"
)

// DeclaredSelfDevelopment reports an explicit SELF_DEVELOPMENT execution request.
// It does not inspect the working directory, .project, or PRD title.
func DeclaredSelfDevelopment(mode string) bool {
	return strings.EqualFold(strings.TrimSpace(mode), state.ExecutionModeSelfDevelopment)
}

// PRDDeclaresSelfDevelopment reports an explicit PRD declaration of the form
// "Execution mode: SELF_DEVELOPMENT". Title or prose mentions are not enough.
func PRDDeclaresSelfDevelopment(prdPath string) bool {
	path := strings.TrimSpace(prdPath)
	if path == "" {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := normalizeSelfDevLine(sc.Text())
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, selfDevDeclarationPrefix) {
			continue
		}
		rest := strings.TrimSpace(line[len(selfDevDeclarationPrefix):])
		rest = strings.Trim(rest, "*_` \t")
		if strings.EqualFold(rest, state.ExecutionModeSelfDevelopment) {
			return true
		}
	}
	return false
}

func normalizeSelfDevLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "*", "")
	line = strings.ReplaceAll(line, "`", "")
	return strings.TrimSpace(line)
}

func repoModulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

type selfDevAuth struct {
	ok     bool
	record state.SelfDevelopment
	reason string
}

func (e *Engine) authorizeSelfDevelopment(ctx context.Context, req Request) selfDevAuth {
	rec := state.SelfDevelopment{
		Mode:   state.ExecutionModeSelfDevelopment,
		Status: state.SelfDevStatusRefused,
	}
	if !DeclaredSelfDevelopment(req.ExecutionMode) {
		rec.AuthorizationResult = "self-development refused: SELF_DEVELOPMENT was not explicitly declared on the execution request"
		return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
	}

	root := strings.TrimSpace(req.ProductRoot)
	if root == "" {
		rec.AuthorizationResult = "self-development refused: product root is empty"
		return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		rec.AuthorizationResult = "self-development refused: " + err.Error()
		return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
	}
	abs = filepath.Clean(abs)
	rec.TargetIdentity = repoModulePath(abs)
	if rec.TargetIdentity == "" {
		rec.TargetIdentity = abs
	}

	if !isOrchestratorRepo(abs) {
		rec.AuthorizationResult = "self-development refused: target is not the PRD→PR orchestrator repository"
		return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
	}
	rec.TargetIdentity = orchestratorIdentity

	prdPath := strings.TrimSpace(req.PRDPath)
	if prdPath == "" {
		prdPath = filepath.Join(abs, "PRD.md")
	}
	if !PRDDeclaresSelfDevelopment(prdPath) {
		rec.AuthorizationResult = "self-development refused: PRD does not explicitly declare Execution mode: SELF_DEVELOPMENT"
		return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
	}

	report, err := prd.ValidateContractFile(prdPath)
	if err != nil {
		rec.AuthorizationResult = "self-development refused: PRD is invalid: " + err.Error()
		return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
	}
	if report != nil && report.Rejected() {
		rec.AuthorizationResult = "self-development refused: PRD contract validation rejected"
		return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
	}

	if e.opts.Git != nil {
		obs := e.opts.Git.Observe(ctx, abs)
		if !obs.IsRepo {
			rec.AuthorizationResult = "self-development refused: repository is not a git checkout"
			return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
		}
		if obs.Dirty {
			rec.AuthorizationResult = "self-development refused: working tree is dirty"
			return selfDevAuth{record: rec, reason: rec.AuthorizationResult}
		}
	}

	rec.Authorized = true
	rec.AuthorizationResult = "authorized"
	rec.Status = state.SelfDevStatusRunning
	return selfDevAuth{ok: true, record: rec}
}

func (e *Engine) refuseSelfDevelopment(req Request, rec state.SelfDevelopment) Result {
	root := strings.TrimSpace(req.ProductRoot)
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	res := refused(root, rec.AuthorizationResult)
	res.Execution.ExecutionMode = state.ExecutionModeSelfDevelopment
	res.Execution.SelfDevelopment = rec
	e.persistSelfDevRefusal(root, rec, res.Execution)
	return res
}

func (e *Engine) persistSelfDevRefusal(root string, rec state.SelfDevelopment, ex Execution) {
	if strings.TrimSpace(root) == "" {
		return
	}
	if _, err := os.Stat(root); err != nil {
		return
	}
	store, err := state.Open(root)
	if err != nil {
		return
	}
	if _, err := store.Init(); err != nil {
		return
	}
	g, err := store.Lock()
	if err != nil {
		return
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return
	}
	st.ExecutionMode = state.ExecutionModeSelfDevelopment
	st.SelfDevelopment = rec
	if ex.RecordedAt == "" {
		ex.RecordedAt = e.opts.Now().UTC().Format(time.RFC3339Nano)
	}
	_ = writeExecution(g, ex)
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventSelfDevelopmentRefused,
		Payload: state.Payload(map[string]string{"reason": rec.AuthorizationResult, "identity": rec.TargetIdentity}),
	})
	_ = g.Save(st)
}

func stampSelfDev(ex *Execution, st *state.State, rec state.SelfDevelopment) {
	if ex != nil {
		ex.ExecutionMode = rec.Mode
		ex.SelfDevelopment = rec
	}
	if st != nil {
		st.ExecutionMode = rec.Mode
		st.SelfDevelopment = rec
	}
}

func (e *Engine) persistSelfDevStamp(root string, rec state.SelfDevelopment) {
	store, err := state.Open(root)
	if err != nil {
		return
	}
	g, err := store.Lock()
	if err != nil {
		return
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return
	}
	st.ExecutionMode = rec.Mode
	st.SelfDevelopment = rec
	ex, err := loadExecutionFromGuard(g)
	if err == nil {
		stampSelfDev(&ex, &st, rec)
		_ = writeExecution(g, ex)
	}
	name := state.EventSelfDevelopmentAuthorized
	if !rec.Authorized || rec.Status == state.SelfDevStatusRefused {
		name = state.EventSelfDevelopmentRefused
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    name,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]any{
			"mode":                  rec.Mode,
			"identity":              rec.TargetIdentity,
			"status":                rec.Status,
			"authorized":            rec.Authorized,
			"authorization_result":  rec.AuthorizationResult,
			"implementation_result": rec.ImplementationResult,
			"verification_result":   rec.VerificationResult,
		}),
	})
	_ = g.Save(st)
}

func (e *Engine) prepareSelfDevelopment(ctx context.Context, req Request) (Result, error) {
	auth := e.authorizeSelfDevelopment(ctx, req)
	if !auth.ok {
		return e.refuseSelfDevelopment(req, auth.record), nil
	}
	res, err := e.prepareProduct(ctx, req, true)
	if err != nil {
		return res, err
	}
	rec := auth.record
	if res.Execution.RefusalReason != "" {
		rec.Status = state.SelfDevStatusFailed
		rec.ImplementationResult = res.Execution.RefusalReason
	} else {
		rec.Status = state.SelfDevStatusRunning
		rec.ImplementationResult = "prepared; verification required before success"
	}
	stampSelfDev(&res.Execution, nil, rec)
	if res.Execution.ProductRoot != "" {
		e.persistSelfDevStamp(res.Execution.ProductRoot, rec)
	}
	return res, nil
}

func (e *Engine) runSelfDevelopment(ctx context.Context, req Request) (Result, error) {
	auth := e.authorizeSelfDevelopment(ctx, req)
	if !auth.ok {
		return e.refuseSelfDevelopment(req, auth.record), nil
	}
	res, err := e.runProduct(ctx, req, true)
	if err != nil {
		return res, err
	}
	rec := auth.record
	switch {
	case res.Execution.RefusalReason != "" && !res.Execution.Invoked:
		rec.Status = state.SelfDevStatusFailed
		rec.ImplementationResult = res.Execution.RefusalReason
	case res.Execution.Invoked:
		rec.Status = state.SelfDevStatusRunning
		rec.ImplementationResult = "worker invoked; verification required before success"
	default:
		rec.Status = state.SelfDevStatusFailed
		rec.ImplementationResult = "worker did not invoke"
	}
	rec.VerificationResult = "pending"
	stampSelfDev(&res.Execution, nil, rec)
	root := res.Execution.ProductRoot
	if root == "" {
		root = req.ProductRoot
	}
	e.persistSelfDevStamp(root, rec)
	return res, nil
}

func applySelfDevVerification(ex *Execution, st *state.State, vres testeng.Result) {
	if ex == nil || ex.ExecutionMode != state.ExecutionModeSelfDevelopment {
		return
	}
	rec := ex.SelfDevelopment
	rec.Mode = state.ExecutionModeSelfDevelopment
	if vres.VerifiedSuccess && vres.Status == testeng.StatusVerified {
		rec.Status = state.SelfDevStatusCompleted
		rec.VerificationResult = "verified"
	} else {
		rec.Status = state.SelfDevStatusFailed
		rec.VerificationResult = vres.Reason
		if rec.VerificationResult == "" {
			rec.VerificationResult = string(vres.Status)
		}
		ex.VerifiedSuccess = false
	}
	stampSelfDev(ex, st, rec)
}
