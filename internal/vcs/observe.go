package vcs

import (
	"context"
	"path/filepath"
)

const (
	StateNotInstalled  = "not_installed"
	StateNotRepository = "not_repository"
	StateNoCommits     = "no_commits"
	StateClean         = "clean"
	StateDirty         = "dirty"
	StateMismatchRoot  = "toplevel_mismatch"
)

// Observation is a read-only Git view for preflight. It does not fail closed
// and never mutates the repository. P4 Inspect/EstablishBaseline remain the
// write-safety gate.
type Observation struct {
	Root          string
	GitAvailable  bool
	IsRepo        bool
	Toplevel      string
	ToplevelMatch bool
	Branch        string
	HeadSHA       string
	HasHEAD       bool
	Dirty         bool
	DirtyPaths    []string
	State         string
}

// Observe inspects Git without creating commits, resetting, or stashing.
func (c *Client) Observe(ctx context.Context, root string) Observation {
	root = filepath.Clean(root)
	obs := Observation{Root: root}
	if _, err := c.lookPath()("git"); err != nil {
		obs.State = StateNotInstalled
		return obs
	}
	obs.GitAvailable = true

	inside, err := c.git()(ctx, root, "rev-parse", "--is-inside-work-tree")
	if err != nil || inside != "true" {
		obs.State = StateNotRepository
		return obs
	}
	obs.IsRepo = true

	top, err := c.git()(ctx, root, "rev-parse", "--show-toplevel")
	if err == nil {
		obs.Toplevel = canonicalPath(top)
		obs.ToplevelMatch = obs.Toplevel == canonicalPath(root)
	}
	if obs.Toplevel != "" && !obs.ToplevelMatch {
		obs.State = StateMismatchRoot
	}

	head, err := c.git()(ctx, root, "rev-parse", "HEAD")
	if err == nil && looksLikeSHA(head) {
		obs.HasHEAD = true
		obs.HeadSHA = head
	}
	if branch, berr := c.git()(ctx, root, "branch", "--show-current"); berr == nil {
		obs.Branch = branch
	}

	porcelain, err := c.git()(ctx, root, "status", "--porcelain")
	if err == nil {
		obs.DirtyPaths = productDirtyPaths(porcelain)
		obs.Dirty = len(obs.DirtyPaths) > 0
	}

	if obs.State == StateMismatchRoot {
		return obs
	}
	if !obs.HasHEAD {
		obs.State = StateNoCommits
		return obs
	}
	if obs.Dirty {
		obs.State = StateDirty
		return obs
	}
	obs.State = StateClean
	return obs
}
