package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/lanternfold/prd-pr/internal/ci"
	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/testeng"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

type MergeDecision struct {
	Allow  bool   `json:"allow"`
	Status string `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type GitHubBlock struct {
	Operation   string `json:"operation"`
	Reason      string `json:"reason"`
	HumanAction string `json:"human_action"`
}

func (b GitHubBlock) Error() string {
	return fmt.Sprintf("%s\n\nOperation:\n%s\n\nReason:\n%s\n\nHuman action required:\n%s",
		state.GitHubActionBlocked, b.Operation, b.Reason, b.HumanAction)
}

func FeatureBranchFor(projectID string) string {
	return vcs.FeatureBranchName(projectID)
}

func (e *Engine) ensureExecutionBranch(ctx context.Context, g *state.Guard, st state.State, root string) (state.State, error) {
	cfg := e.cfg()
	if !cfg.FeatureBranches() {
		st.Repository.BaseBranch = cfg.Branch()
		return st, nil
	}
	base := cfg.Branch()
	name := st.Repository.FeatureBranch
	if name == "" {
		name = FeatureBranchFor(st.ProjectID)
	}
	br, err := e.opts.Git.EnsureOwnedBranch(ctx, root, name, st.Repository.FeatureBranch, st.Repository.BranchSHA)
	if err != nil {
		return st, err
	}
	ev := state.EventBranchCreated
	if br.Reason == "reused" {
		ev = state.EventBranchReused
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    ev,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]string{"branch": br.Name, "sha": br.SHA, "base": base, "state": br.Reason}),
	})
	st.Repository.BaseBranch = base
	st.Repository.FeatureBranch = br.Name
	st.Repository.Branch = br.Name
	st.Repository.BranchSHA = br.SHA
	st.Repository.BranchState = br.Reason
	return st, nil
}

func (e *Engine) OpenMilestonePR(ctx context.Context, root, title, body string) (vcs.PR, error) {
	root, _, err := identifyRoot(root)
	if err != nil {
		return vcs.PR{}, err
	}
	if _, err := e.requireVerifiedExecution(root); err != nil {
		return vcs.PR{}, err
	}
	store, err := state.Open(root)
	if err != nil {
		return vcs.PR{}, err
	}
	g, err := store.Lock()
	if err != nil {
		return vcs.PR{}, err
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return vcs.PR{}, err
	}
	st, pr, err := e.ensurePRLocked(ctx, g, st, root, title, body)
	if err != nil {
		return pr, err
	}
	_ = g.Save(st)
	return pr, nil
}

func (e *Engine) ensurePRLocked(ctx context.Context, g *state.Guard, st state.State, root, title, body string) (state.State, vcs.PR, error) {
	cfg := e.cfg()
	if !cfg.GitHubEnabled || cfg.PRBoundary == config.PRBoundaryNever {
		return st, vcs.PR{Skipped: true, Reason: "GitHub disabled or pr_boundary=never"}, nil
	}
	if e.opts.GH == nil || !e.opts.GH.Available() || !e.opts.GH.Authenticated(ctx) {
		block := GitHubBlock{
			Operation:   "create or reuse pull request",
			Reason:      "GitHub CLI is unavailable or not authenticated",
			HumanAction: "Install gh and run gh auth login, then prdpr pr / prdpr resume",
		}
		st = e.recordGitHubBlock(g, st, block)
		return st, vcs.PR{Skipped: true, Reason: block.Error()}, nil
	}
	if st.Repository.RemoteURL == "" && e.opts.Git.RemoteURL(ctx, root, cfg.Remote()) == "" {
		return st, vcs.PR{Skipped: true, Reason: "no git remote configured"}, nil
	}
	if st.Repository.PushStatus != state.PushPushed && !e.opts.Git.Pushed(ctx, root, cfg.Remote(), firstNonEmpty(st.Repository.FeatureBranch, st.Repository.Branch), st.CurrentCommit) {
		if st.Repository.PushStatus == state.PushFailed {
			return st, vcs.PR{Skipped: true, Reason: "branch is not pushed"}, nil
		}
	}
	base := firstNonEmpty(st.Repository.BaseBranch, cfg.Branch())
	head := firstNonEmpty(st.Repository.FeatureBranch, st.Repository.Branch)
	if title == "" {
		title = "PRD→PR milestone"
	}
	if body == "" {
		body = "Created by prdpr. Auto-merge is off unless explicitly enabled."
	}
	pr, err := e.opts.GH.EnsurePR(ctx, root, title, body, base, head)
	if err != nil {
		block := GitHubBlock{Operation: "create pull request", Reason: err.Error(), HumanAction: "Resolve GitHub access and retry prdpr pr"}
		st = e.recordGitHubBlock(g, st, block)
		return st, vcs.PR{Skipped: true, Reason: block.Error()}, nil
	}
	if pr.Skipped {
		if strings.Contains(strings.ToLower(pr.Reason), "gh") || strings.Contains(strings.ToLower(pr.Reason), "auth") || strings.Contains(strings.ToLower(pr.Reason), "http") {
			st = e.recordGitHubBlock(g, st, GitHubBlock{Operation: "create pull request", Reason: pr.Reason, HumanAction: "Fix GitHub authentication/permissions and retry"})
		}
		return st, pr, nil
	}
	name := state.EventPROpened
	if pr.Reused {
		name = state.EventPRReused
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    name,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]any{
			"number": pr.Number, "url": pr.URL, "head": pr.Head, "base": pr.Base, "sha": pr.SHA,
			"reused": pr.Reused, "phase": st.CurrentPhaseID, "run_id": st.CurrentRunID,
		}),
	})
	st.Repository.PRNumber = pr.Number
	st.Repository.PRURL = pr.URL
	st.Repository.PRHead = firstNonEmpty(pr.Head, head)
	st.Repository.PRBase = firstNonEmpty(pr.Base, base)
	st.Repository.PRSHA = firstNonEmpty(pr.SHA, st.CurrentCommit)
	if pr.CreatedAt != "" {
		st.Repository.PRCreatedAt = pr.CreatedAt
	} else if st.Repository.PRCreatedAt == "" {
		st.Repository.PRCreatedAt = e.opts.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	st.Repository.GitHubBlock = ""
	if st.Repository.MergeStatus != state.PRMerged {
		st.Repository.MergeStatus = state.PROpen
	}
	return st, pr, nil
}

func (e *Engine) InspectChecks(ctx context.Context, root string) ci.Report {
	root, _, err := identifyRoot(root)
	if err != nil {
		return ci.Report{Status: ci.StatusUnknown, Detail: err.Error()}
	}
	cfg := e.cfg()
	if !cfg.GitHubEnabled {
		return ci.Report{Available: false, Status: ci.StatusSkipped, Detail: "GitHub disabled"}
	}
	pr := ""
	store, err := state.Open(root)
	if err == nil {
		if st, lerr := store.Load(); lerr == nil {
			pr = st.Repository.PRNumber
		}
	}
	var rep ci.Report
	if pr != "" {
		rep = e.opts.CI.PRChecks(ctx, root, pr)
	} else {
		rep = e.opts.CI.Status(ctx, root)
	}
	if store == nil {
		return rep
	}
	g, lerr := store.Lock()
	if lerr != nil {
		return rep
	}
	defer func() { _ = g.Unlock() }()
	st, lerr := g.Load()
	if lerr != nil {
		return rep
	}
	if st.Repository.MergeStatus == state.PRMerged {
		return rep
	}
	st.Repository.ChecksStatus = rep.Verdict()
	switch rep.Verdict() {
	case ci.VerdictPending:
		st.Repository.MergeStatus = state.PRChecking
	case ci.VerdictPass:
		if st.Repository.PRNumber != "" && st.Repository.MergeStatus == "" {
			st.Repository.MergeStatus = state.PROpen
		}
	default:
		if st.Repository.PRNumber != "" && st.Repository.MergeStatus == "" {
			st.Repository.MergeStatus = state.PROpen
		}
	}
	_ = g.Save(st)
	return rep
}

func EvaluateMerge(cfg config.Config, st state.State, ex Execution, pr vcs.PR, checks ci.Report, v *testeng.Result) MergeDecision {
	wait := func(reason string) MergeDecision {
		return MergeDecision{Allow: false, Status: state.PRWaitingForMerge, Reason: reason}
	}
	checking := func(reason string) MergeDecision {
		return MergeDecision{Allow: false, Status: state.PRChecking, Reason: reason}
	}
	if pr.Number == "" && st.Repository.PRNumber == "" {
		return wait("PR does not exist")
	}
	if pr.State != "" && pr.State != "open" {
		if pr.State == "merged" {
			return MergeDecision{Allow: false, Status: state.PRMerged, Reason: "PR already merged"}
		}
		return wait("PR is not open")
	}
	head := firstNonEmpty(st.Repository.FeatureBranch, st.Repository.Branch)
	if pr.Head != "" && head != "" && pr.Head != head {
		return wait("PR head branch does not match current execution")
	}
	base := firstNonEmpty(st.Repository.BaseBranch, cfg.Branch())
	if pr.Base != "" && base != "" && pr.Base != base {
		return wait("PR base branch does not match configured base")
	}
	if !ex.VerifiedSuccess {
		return wait("verified_success is not true")
	}
	if v != nil && v.ManualVerificationRequired {
		return wait("manual verification is pending")
	}
	switch st.CurrentState {
	case state.StateWaitingForHuman:
		return wait("human intervention is pending")
	case state.StateRepairing:
		return wait("repair is pending")
	case state.StateVerificationFailed, state.StateVerificationIncomplete:
		return wait("local verification has not passed")
	}
	if st.CurrentState != state.StateVerified && st.CurrentState != state.StateCompleted {
		return wait("local verification has not passed")
	}
	verdict, reason := checks.RequiredVerdict(cfg.RequiredChecks)
	if verdict == ci.VerdictPending {
		return checking(reason)
	}
	if verdict != ci.VerdictPass {
		return wait(reason)
	}
	if strings.EqualFold(pr.Mergeable, "CONFLICTING") || strings.EqualFold(pr.Mergeable, "CONFLICT") {
		return wait("merge conflict")
	}
	if cfg.RequireApproval && !strings.EqualFold(pr.ReviewDecision, "APPROVED") {
		return wait("approval policy is not satisfied")
	}
	if !cfg.AutoMergeEnabled {
		return wait("AutoMergeEnabled is false")
	}
	return MergeDecision{Allow: true, Status: state.PRReadyToMerge}
}

func (e *Engine) TryMerge(ctx context.Context, root string) (MergeDecision, vcs.MergeResult, error) {
	root, _, err := identifyRoot(root)
	if err != nil {
		return MergeDecision{}, vcs.MergeResult{}, err
	}
	ex, err := e.requireVerifiedExecution(root)
	if err != nil {
		return MergeDecision{Allow: false, Status: state.PRWaitingForMerge, Reason: err.Error()}, vcs.MergeResult{}, nil
	}
	store, err := state.Open(root)
	if err != nil {
		return MergeDecision{}, vcs.MergeResult{}, err
	}
	g, err := store.Lock()
	if err != nil {
		return MergeDecision{}, vcs.MergeResult{}, err
	}
	defer func() { _ = g.Unlock() }()
	st, err := g.Load()
	if err != nil {
		return MergeDecision{}, vcs.MergeResult{}, err
	}
	return e.mergeLocked(ctx, g, st, root, ex)
}

func (e *Engine) mergeLocked(ctx context.Context, g *state.Guard, st state.State, root string, ex Execution) (MergeDecision, vcs.MergeResult, error) {
	cfg := e.cfg()
	if st.Repository.PRNumber == "" {
		dec := EvaluateMerge(cfg, st, ex, vcs.PR{}, ci.Report{Status: ci.StatusUnknown}, nil)
		st.Repository.MergeStatus = dec.Status
		_ = g.AppendEvent(state.Event{Kind: state.KindResult, Name: state.EventMergeBlocked, Payload: state.Payload(map[string]string{"reason": dec.Reason})})
		_ = g.Save(st)
		return dec, vcs.MergeResult{}, nil
	}
	pr, err := e.opts.GH.ViewPR(ctx, root, st.Repository.PRNumber)
	if err != nil || pr.Skipped {
		reason := "cannot view PR"
		if err != nil {
			reason = err.Error()
		} else if pr.Reason != "" {
			reason = pr.Reason
		}
		block := GitHubBlock{Operation: "inspect pull request", Reason: reason, HumanAction: "Restore GitHub access and retry"}
		st = e.recordGitHubBlock(g, st, block)
		_ = g.Save(st)
		return MergeDecision{Allow: false, Status: state.PRWaitingForMerge, Reason: block.Error()}, vcs.MergeResult{}, nil
	}
	if strings.EqualFold(pr.State, "merged") {
		st = e.recordMerged(g, st, pr, vcs.MergeResult{Merged: true, SHA: firstNonEmpty(st.Repository.MergeSHA, pr.SHA), Method: firstNonEmpty(st.Repository.MergeMethod, cfg.MergeMethodName())}, cfg)
		_ = g.Save(st)
		return MergeDecision{Allow: false, Status: state.PRMerged, Reason: "PR already merged"}, vcs.MergeResult{Merged: true, SHA: st.Repository.MergeSHA}, nil
	}
	checks := e.opts.CI.PRChecks(ctx, root, pr.Number)
	st.Repository.ChecksStatus = checks.Verdict()
	v := loadVerificationPtr(root)
	dec := EvaluateMerge(cfg, st, ex, pr, checks, v)
	if !dec.Allow {
		st.Repository.MergeStatus = dec.Status
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventMergeBlocked,
			RunID:   st.CurrentRunID,
			PhaseID: st.CurrentPhaseID,
			Payload: state.Payload(map[string]string{"reason": dec.Reason, "status": dec.Status}),
		})
		_ = g.Save(st)
		return dec, vcs.MergeResult{}, nil
	}
	res := vcs.MergeResult{}
	merged, merr := e.opts.GH.MergePR(ctx, root, pr.Number, cfg.MergeMethodName())
	if merr != nil {
		block := GitHubBlock{Operation: "merge pull request", Reason: merr.Error(), HumanAction: "Inspect the PR on GitHub and retry"}
		st = e.recordGitHubBlock(g, st, block)
		_ = g.Save(st)
		return MergeDecision{Allow: false, Status: state.PRWaitingForMerge, Reason: block.Error()}, res, nil
	}
	res = merged
	if !res.Merged {
		st.Repository.MergeStatus = state.PRWaitingForMerge
		_ = g.AppendEvent(state.Event{Kind: state.KindResult, Name: state.EventMergeBlocked, Payload: state.Payload(map[string]string{"reason": res.Reason})})
		_ = g.Save(st)
		return MergeDecision{Allow: false, Status: state.PRWaitingForMerge, Reason: res.Reason}, res, nil
	}
	st = e.recordMerged(g, st, pr, res, cfg)
	st = e.cleanupMergedBranch(ctx, g, st, root, pr, cfg)
	_ = g.Save(st)
	return dec, res, nil
}

func (e *Engine) recordMerged(g *state.Guard, st state.State, pr vcs.PR, res vcs.MergeResult, cfg config.Config) state.State {
	already := st.Repository.MergeStatus == state.PRMerged && st.Repository.MergeSHA != ""
	if pr.Number != "" {
		st.Repository.PRNumber = pr.Number
	}
	st.Repository.MergeSHA = firstNonEmpty(st.Repository.MergeSHA, res.SHA, pr.SHA)
	st.Repository.MergeMethod = firstNonEmpty(st.Repository.MergeMethod, res.Method, cfg.MergeMethodName())
	if st.Repository.MergeAt == "" {
		st.Repository.MergeAt = e.opts.Now().UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	st.Repository.MergeStatus = state.PRMerged
	st.Repository.MergeBranch = firstNonEmpty(st.Repository.MergeBranch, pr.Head, st.Repository.FeatureBranch, st.Repository.Branch)
	st.Repository.MergeRepository = firstNonEmpty(st.Repository.MergeRepository, st.Repository.RemoteURL, strings.Trim(cfg.GitHubOwner+"/"+cfg.GitHubRepo, "/"))
	st.Repository.GitHubBlock = ""
	if already {
		return st
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventMergeCompleted,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]string{
			"sha":        st.Repository.MergeSHA,
			"number":     st.Repository.PRNumber,
			"method":     st.Repository.MergeMethod,
			"branch":     st.Repository.MergeBranch,
			"repository": st.Repository.MergeRepository,
			"merged_at":  st.Repository.MergeAt,
		}),
	})
	return st
}

func (e *Engine) cleanupMergedBranch(ctx context.Context, g *state.Guard, st state.State, root string, pr vcs.PR, cfg config.Config) state.State {
	if !cfg.DeleteBranchAfterMerge {
		if err := e.opts.Git.FastForwardBase(ctx, root, cfg.Branch(), cfg.Remote()); err != nil {
			_ = g.AppendEvent(state.Event{Kind: state.KindResult, Name: state.EventMergeBlocked, Payload: state.Payload(map[string]string{"reason": "local base not updated: " + err.Error()})})
		}
		return st
	}
	head := firstNonEmpty(pr.Head, st.Repository.FeatureBranch, st.Repository.MergeBranch)
	if head == "" || head == cfg.Branch() {
		return st
	}
	br, _ := e.opts.Git.InspectBranch(ctx, root, head)
	if !br.Exists {
		st.Repository.BranchState = "deleted"
		return st
	}
	_ = e.opts.Git.DeleteRemoteBranch(ctx, root, cfg.Remote(), head)
	if err := e.opts.Git.DeleteBranch(ctx, root, head); err == nil {
		st.Repository.BranchState = "deleted"
		_ = g.AppendEvent(state.Event{Kind: state.KindResult, Name: state.EventBranchDeleted, Payload: state.Payload(map[string]string{"branch": head})})
	}
	if err := e.opts.Git.FastForwardBase(ctx, root, cfg.Branch(), cfg.Remote()); err != nil {
		_ = g.AppendEvent(state.Event{Kind: state.KindResult, Name: state.EventMergeBlocked, Payload: state.Payload(map[string]string{"reason": "local base not updated: " + err.Error()})})
	}
	return st
}

func (e *Engine) recordGitHubBlock(g *state.Guard, st state.State, block GitHubBlock) state.State {
	st.Repository.GitHubBlock = block.Error()
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventGitHubBlocked,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]string{"operation": block.Operation, "reason": block.Reason, "human": block.HumanAction}),
	})
	return st
}

func loadVerificationPtr(root string) *testeng.Result {
	v := loadVerification(root)
	if v.Status == "" && v.TaskID == "" {
		return nil
	}
	return &v
}

func (e *Engine) maybeDeliverLocked(ctx context.Context, g *state.Guard, st state.State, root string) (state.State, error) {
	cfg := e.cfg()
	if !cfg.GitHubEnabled || cfg.PRBoundary == config.PRBoundaryNever {
		return st, nil
	}
	open := cfg.PRBoundary == config.PRBoundaryPhase
	if cfg.PRBoundary == config.PRBoundaryRun && st.ProjectStatus == state.StatusProjectCompleted {
		open = true
	}
	if !open {
		return st, nil
	}
	var err error
	st, _, err = e.ensurePRLocked(ctx, g, st, root, "PRD→PR milestone", "")
	if err != nil {
		return st, err
	}
	if !cfg.AutoMergeEnabled {
		return st, nil
	}
	ex, _ := loadExecutionFromGuard(g)
	_, _, _ = e.mergeLocked(ctx, g, st, root, ex)
	st, _ = g.Load()
	return st, nil
}
