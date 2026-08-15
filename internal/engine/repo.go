package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/state"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func (e *Engine) ensureWorkspace(ctx context.Context, productRoot string) (string, *fsguard.Jail, error) {
	if strings.TrimSpace(productRoot) == "" {
		return "", nil, fmt.Errorf("product root is empty")
	}
	abs, err := filepath.Abs(productRoot)
	if err != nil {
		return "", nil, err
	}
	abs = filepath.Clean(abs)
	if !e.opts.AllowSelf && isOrchestratorRepo(abs) {
		return "", nil, fmt.Errorf("refusing to use the PRD→PR orchestrator repository as a product workspace")
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		if cloned, cerr := e.tryCloneMissingWorkspace(ctx, abs); cerr != nil {
			return "", nil, cerr
		} else if !cloned {
			if err := os.MkdirAll(abs, 0o755); err != nil {
				return "", nil, err
			}
		}
	}
	return identifyRoot(abs)
}

func (e *Engine) tryCloneMissingWorkspace(ctx context.Context, dest string) (bool, error) {
	cfg := e.cfg()
	if !cfg.GitHubEnabled {
		return false, nil
	}
	owner, name := e.repoIdentity(nil)
	if owner == "" || name == "" || e.opts.GH == nil || !e.opts.GH.Available() || !e.opts.GH.Authenticated(ctx) {
		return false, nil
	}
	ok, url, err := e.opts.GH.RepoExists(ctx, owner, name)
	if err != nil || !ok {
		return false, err
	}
	if url == "" {
		url = "https://github.com/" + owner + "/" + name + ".git"
	}
	if err := e.opts.Git.Clone(ctx, url, dest); err != nil {
		return false, err
	}
	return true, nil
}

func (e *Engine) repoIdentity(doc *prd.Document) (owner, name string) {
	cfg := e.cfg()
	owner = strings.TrimSpace(cfg.GitHubOwner)
	name = strings.TrimSpace(cfg.GitHubRepo)
	if owner != "" && name != "" {
		return owner, name
	}
	if doc != nil {
		o, n := vcs.ParseRepoRef(doc.Metadata.Repository)
		if owner == "" {
			owner = o
		}
		if name == "" {
			name = n
		}
	}
	return owner, name
}

func (e *Engine) bootstrapRepo(ctx context.Context, g *state.Guard, st state.State, root string, doc *prd.Document, prdOnly bool) (state.State, error) {
	if !e.opts.AllowSelf && isOrchestratorRepo(root) {
		return st, fmt.Errorf("refusing to bootstrap the PRD→PR orchestrator repository")
	}
	cfg := e.cfg()
	git := e.opts.Git
	jail := g.Jail()
	repo := st.Repository
	repo.LocalRoot = root

	obs := git.Observe(ctx, root)
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventRepoDiscovered,
		RunID:   st.CurrentRunID,
		Payload: state.Payload(map[string]any{"state": obs.State, "is_repo": obs.IsRepo, "has_head": obs.HasHEAD, "branch": obs.Branch}),
	})

	if obs.IsRepo && obs.Dirty && obs.HasHEAD {
		// Reuse existing history; do not discard uncommitted work. Baseline remains P4's gate.
		st.Repository = e.inspectExisting(ctx, root, repo, obs, cfg)
		return st, g.Save(st)
	}

	if !obs.IsRepo {
		if err := git.Init(ctx, root, cfg.Branch()); err != nil {
			return st, err
		}
		obs = git.Observe(ctx, root)
	}

	if !obs.HasHEAD {
		sha, err := git.InitialCommit(ctx, root, cfg.InitialMessage(), jail)
		if err != nil {
			return st, err
		}
		repo.InitialCommitSHA = sha
		repo.LatestCheckpointSHA = sha
		repo.CommitStatus = "initial"
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventCommitCreated,
			Payload: state.Payload(map[string]string{"sha": sha, "kind": "initial", "message": cfg.InitialMessage()}),
		})
		obs = git.Observe(ctx, root)
	} else if repo.InitialCommitSHA == "" {
		repo.InitialCommitSHA = obs.HeadSHA
		if repo.CommitStatus == "" {
			repo.CommitStatus = "initial"
		}
	}

	if err := git.EnsureBranch(ctx, root, cfg.Branch()); err != nil {
		return st, err
	}
	obs = git.Observe(ctx, root)
	repo.Branch = obs.Branch
	if repo.Branch == "" {
		repo.Branch = cfg.Branch()
	}

	st, err := e.configureRemote(ctx, g, st, root, doc, repo, cfg, prdOnly)
	if err != nil {
		return st, err
	}
	if st.CurrentState == state.StateWaitingForHuman {
		return st, g.Save(st)
	}
	repo = st.Repository
	_ = g.AppendEvent(state.Event{
		Kind: state.KindResult,
		Name: state.EventRepoBootstrapped,
		Payload: state.Payload(map[string]any{
			"type":        repo.Type,
			"branch":      repo.Branch,
			"initial_sha": repo.InitialCommitSHA,
			"github":      repo.GitHubStatus,
			"push_status": repo.PushStatus,
			"remote":      repo.RemoteURL,
			"skip_reason": repo.SkipReason,
		}),
	})
	st.CurrentCommit = obs.HeadSHA
	if st.LastKnownGoodCommit == "" {
		st.LastKnownGoodCommit = obs.HeadSHA
	}
	st.Repository = repo
	if repo.PushStatus != state.PushSkipped && e.opts.Git.RemoteURL(ctx, root, firstNonEmpty(repo.RemoteName, cfg.Remote())) != "" {
		var err error
		st, err = e.pushLocked(ctx, g, st, root, true)
		if err != nil {
			return st, err
		}
	}
	st, err = e.ensureRulesetLocked(ctx, g, st, root, doc, prdOnly)
	if err != nil {
		return st, err
	}
	return st, g.Save(st)
}

func (e *Engine) inspectExisting(ctx context.Context, root string, repo state.Repository, obs vcs.Observation, cfg config.Config) state.Repository {
	repo.Type = state.RepoTypeLocal
	repo.Branch = obs.Branch
	repo.InitialCommitSHA = firstNonEmpty(repo.InitialCommitSHA, obs.HeadSHA)
	if url := e.opts.Git.RemoteURL(ctx, root, cfg.Remote()); url != "" {
		repo.RemoteName = cfg.Remote()
		repo.RemoteURL = url
		if cfg.GitHubEnabled {
			repo.Type = state.RepoTypeGitHub
			repo.GitHubStatus = state.GitHubExists
		}
	}
	return repo
}

func (e *Engine) configureRemote(ctx context.Context, g *state.Guard, st state.State, root string, doc *prd.Document, repo state.Repository, cfg config.Config, prdOnly bool) (state.State, error) {
	existingURL := e.opts.Git.RemoteURL(ctx, root, cfg.Remote())
	if existingURL != "" {
		repo.RemoteName = cfg.Remote()
		repo.RemoteURL = existingURL
		repo.Type = state.RepoTypeLocal
		if cfg.GitHubEnabled {
			repo.Type = state.RepoTypeGitHub
			repo.GitHubStatus = state.GitHubExists
			if repo.PushStatus == "" || repo.PushStatus == state.PushSkipped {
				repo.PushStatus = state.PushPending
			}
		} else {
			repo.GitHubStatus = state.GitHubDisabled
			if repo.PushStatus == "" {
				repo.PushStatus = state.PushPending
			}
		}
		st.Repository = repo
		return st, nil
	}

	if !cfg.GitHubEnabled {
		repo.Type = state.RepoTypeLocal
		repo.GitHubStatus = state.GitHubDisabled
		repo.PushStatus = state.PushSkipped
		repo.SkipReason = "GitHub integration is disabled; continuing locally"
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventRepoSkipped,
			Payload: state.Payload(map[string]string{"reason": repo.SkipReason}),
		})
		st.Repository = repo
		return st, nil
	}

	if e.opts.GH == nil || !e.opts.GH.Available() {
		if prdOnly {
			return e.githubHuman(g, st, repo, "gh is not available")
		}
		return e.skipRemote(g, st, repo, state.GitHubUnavailable, "gh is not available; continuing locally")
	}
	if !e.opts.GH.Authenticated(ctx) {
		if prdOnly {
			return e.githubHuman(g, st, repo, "gh is not authenticated")
		}
		return e.skipRemote(g, st, repo, state.GitHubUnavailable, "gh is not authenticated; continuing locally")
	}

	owner, name := e.repoIdentity(doc)
	if owner == "" || name == "" {
		return e.skipRemote(g, st, repo, state.GitHubSkipped, "repository owner/name are not configured; refusing to invent them")
	}

	exists, url, err := e.opts.GH.RepoExists(ctx, owner, name)
	if err != nil {
		if prdOnly {
			return e.githubHuman(g, st, repo, "GitHub repository lookup failed: "+err.Error())
		}
		return e.skipRemote(g, st, repo, state.GitHubUnavailable, "GitHub repository lookup failed: "+err.Error())
	}
	if !exists {
		url, err = e.opts.GH.CreateRepo(ctx, vcs.RepoSpec{
			Owner:       owner,
			Name:        name,
			Visibility:  cfg.RemoteVisibility(),
			Description: firstNonEmpty(cfg.GitHubDescription, descriptionFromDoc(doc)),
		})
		if err != nil {
			if prdOnly {
				return e.githubHuman(g, st, repo, "GitHub repository creation failed: "+err.Error())
			}
			return e.skipRemote(g, st, repo, state.GitHubUnavailable, "GitHub repository creation skipped: "+err.Error())
		}
		repo.GitHubStatus = state.GitHubCreated
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventRepoCreated,
			Payload: state.Payload(map[string]string{"owner": owner, "name": name, "url": url, "visibility": cfg.RemoteVisibility()}),
		})
	} else {
		repo.GitHubStatus = state.GitHubExists
	}
	if url == "" {
		url = "https://github.com/" + owner + "/" + name + ".git"
	}
	if err := e.opts.Git.AddRemoteIfMissing(ctx, root, cfg.Remote(), url); err != nil {
		return st, err
	}
	repo.Type = state.RepoTypeGitHub
	repo.RemoteName = cfg.Remote()
	repo.RemoteURL = e.opts.Git.RemoteURL(ctx, root, cfg.Remote())
	if repo.RemoteURL == "" {
		repo.RemoteURL = url
	}
	if repo.PushStatus == "" {
		repo.PushStatus = state.PushPending
	}
	st.Repository = repo
	return st, nil
}

func (e *Engine) skipRemote(g *state.Guard, st state.State, repo state.Repository, ghStatus, reason string) (state.State, error) {
	repo.Type = state.RepoTypeLocal
	repo.GitHubStatus = ghStatus
	repo.PushStatus = state.PushSkipped
	repo.SkipReason = reason
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventPushSkipped,
		Payload: state.Payload(map[string]string{"reason": reason, "github_status": ghStatus}),
	})
	st.Repository = repo
	return st, nil
}

func (e *Engine) githubHuman(g *state.Guard, st state.State, repo state.Repository, reason string) (state.State, error) {
	repo.SkipReason = reason
	repo.GitHubStatus = state.GitHubUnavailable
	st.Repository = repo
	h := human.Request{
		Kind:    human.KindGitHubAuth,
		Reason:  "github_unavailable",
		Needed:  reason + ". Authenticate with gh, then prdpr resume. Local Git state was preserved.",
		Urgency: human.UrgencyHigh,
	}
	root := st.ProductRoot
	if root == "" {
		root = repo.LocalRoot
	}
	_, _ = e.requestHumanLocked(g, st, root, h)
	st.CurrentState = state.StateWaitingForHuman
	return st, nil
}

func (e *Engine) checkpointLocked(ctx context.Context, g *state.Guard, st state.State, root string, ex Execution) (state.State, error) {
	cfg := e.cfg()
	if !cfg.AutoCommit {
		return st, nil
	}
	if err := e.assertCommitGate(st, ex); err != nil {
		return st, err
	}
	phase := st.CurrentPhaseID
	if st.Repository.PhaseCheckpoints == nil {
		st.Repository.PhaseCheckpoints = map[string]string{}
	}
	if sha, ok := st.Repository.PhaseCheckpoints[phase]; ok && sha != "" {
		sn, err := e.opts.Git.Inspect(ctx, root)
		if err == nil && !sn.Dirty && (sn.HeadSHA == sha || st.Repository.LatestCheckpointSHA == sha) {
			return st, nil
		}
	}
	msg := vcs.CheckpointMessage(phase, ex.PacketRef)
	if pkt, err := loadPacket(root, ex.PacketRef); err == nil {
		msg = vcs.CheckpointMessage(pkt.PhaseID, pkt.Objective)
	}
	jail := g.Jail()
	sha, err := e.opts.Git.Commit(ctx, root, msg, nil, jail)
	if err != nil {
		if strings.Contains(err.Error(), "nothing to commit") {
			sn, _ := e.opts.Git.Inspect(ctx, root)
			if sn.HeadSHA != "" {
				st.Repository.PhaseCheckpoints[phase] = sn.HeadSHA
				st.Repository.LatestCheckpointSHA = sn.HeadSHA
				st.CurrentCommit = sn.HeadSHA
				st.LastKnownGoodCommit = sn.HeadSHA
			}
			return st, nil
		}
		return st, err
	}
	st.CurrentCommit = sha
	st.LastKnownGoodCommit = sha
	st.Repository.LatestCheckpointSHA = sha
	st.Repository.CommitStatus = "checkpointed"
	st.Repository.PhaseCheckpoints[phase] = sha
	if st.Repository.PushStatus == "" || st.Repository.PushStatus == state.PushPushed {
		st.Repository.PushStatus = state.PushPending
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventCommitCreated,
		RunID:   st.CurrentRunID,
		PhaseID: phase,
		Payload: state.Payload(map[string]string{"sha": sha, "kind": "phase", "message": msg}),
	})
	return st, nil
}

func (e *Engine) assertCommitGate(st state.State, ex Execution) error {
	if st.CurrentState == state.StateWaitingForHuman {
		return fmt.Errorf("commit refused: human intervention is pending")
	}
	if st.CurrentState == state.StateRepairing {
		return fmt.Errorf("commit refused: repair is pending")
	}
	if st.CurrentState == state.StateVerificationFailed {
		return fmt.Errorf("commit refused: verification failed")
	}
	if !ex.VerifiedSuccess {
		return fmt.Errorf("commit refused: verified_success is not true")
	}
	switch st.CurrentState {
	case state.StateVerified, state.StateCompleted:
		return nil
	default:
		return fmt.Errorf("commit refused: phase is not VERIFIED (state %s)", st.CurrentState)
	}
}

func (e *Engine) pushLocked(ctx context.Context, g *state.Guard, st state.State, root string, initial bool) (state.State, error) {
	cfg := e.cfg()
	repo := st.Repository
	if !cfg.AutoPush {
		return st, nil
	}
	if repo.PushStatus == state.PushSkipped && e.opts.Git.RemoteURL(ctx, root, firstNonEmpty(repo.RemoteName, cfg.Remote())) == "" {
		return st, nil
	}
	if !cfg.GitHubEnabled && repo.Type != state.RepoTypeGitHub && repo.RemoteURL == "" {
		if repo.PushStatus == "" {
			repo.PushStatus = state.PushSkipped
			repo.SkipReason = "no remote configured; continuing locally"
			_ = g.AppendEvent(state.Event{
				Kind:    state.KindResult,
				Name:    state.EventPushSkipped,
				Payload: state.Payload(map[string]string{"reason": repo.SkipReason}),
			})
			st.Repository = repo
		}
		return st, nil
	}
	remote := repo.RemoteName
	if remote == "" {
		remote = cfg.Remote()
	}
	if e.opts.Git.RemoteURL(ctx, root, remote) == "" {
		repo.PushStatus = state.PushSkipped
		repo.SkipReason = "no git remote configured; continuing locally"
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventPushSkipped,
			Payload: state.Payload(map[string]string{"reason": repo.SkipReason}),
		})
		st.Repository = repo
		return st, nil
	}
	ref := firstNonEmpty(repo.FeatureBranch, repo.Branch)
	if ref == "" {
		ref = cfg.Branch()
	}
	base := firstNonEmpty(repo.BaseBranch, cfg.Branch())
	if !initial && cfg.FeatureBranches() && ref != "" && base != "" && ref == base {
		repo.PushStatus = state.PushFailed
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventPushFailed,
			RunID:   st.CurrentRunID,
			PhaseID: st.CurrentPhaseID,
			Payload: state.Payload(map[string]string{"error": "refusing push to protected/base branch " + base}),
		})
		st.Repository = repo
		return st, nil
	}
	sha := st.CurrentCommit
	if sha == "" {
		if sn, err := e.opts.Git.Inspect(ctx, root); err == nil {
			sha = sn.HeadSHA
		}
	}
	if e.opts.Git.Pushed(ctx, root, remote, ref, sha) {
		repo.PushStatus = state.PushPushed
		st.Repository = repo
		return st, nil
	}
	if err := e.opts.Git.Push(ctx, root, remote, ref); err != nil {
		repo.PushStatus = state.PushFailed
		_ = g.AppendEvent(state.Event{
			Kind:    state.KindResult,
			Name:    state.EventPushFailed,
			RunID:   st.CurrentRunID,
			PhaseID: st.CurrentPhaseID,
			Payload: state.Payload(map[string]string{"error": err.Error(), "initial": fmt.Sprintf("%t", initial)}),
		})
		st.Repository = repo
		if cfg.GitHubEnabled && !initial {
			return e.githubHuman(g, st, repo, "git push failed: "+err.Error())
		}
		return st, nil
	}
	repo.PushStatus = state.PushPushed
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventPushSucceeded,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: state.Payload(map[string]string{"remote": remote, "ref": ref, "sha": sha}),
	})
	st.Repository = repo
	return st, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func descriptionFromDoc(doc *prd.Document) string {
	if doc == nil {
		return ""
	}
	if doc.Metadata.Product != "" {
		return doc.Metadata.Product
	}
	return doc.Metadata.Title
}
