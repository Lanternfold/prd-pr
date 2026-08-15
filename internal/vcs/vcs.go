package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/fsguard"
)

const projectMetaDir = ".project"

// Snapshot is the local Git view required before a coding worker may write.
type Snapshot struct {
	Root       string
	Branch     string
	HeadSHA    string
	Dirty      bool
	DirtyPaths []string
}

// Baseline is a recoverable commit SHA recorded before the worker runs.
type Baseline struct {
	SHA    string `json:"sha"`
	Branch string `json:"branch"`
}

// GitFunc runs git in dir and returns combined stdout (trimmed) or an error.
type GitFunc func(ctx context.Context, dir string, args ...string) (string, error)

// Client is the local Git adapter: inspect, baseline, commit, branch, remote, push.
type Client struct {
	Git      GitFunc
	LookPath func(file string) (string, error)
}

func Default() *Client {
	return &Client{
		LookPath: exec.LookPath,
		Git: func(ctx context.Context, dir string, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, "git", args...)
			cmd.Dir = dir
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			out := strings.TrimSpace(stdout.String())
			if err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg == "" {
					msg = err.Error()
				}
				return out, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
			}
			return out, nil
		},
	}
}

func (c *Client) git() GitFunc {
	inner := Default().Git
	if c != nil && c.Git != nil {
		inner = c.Git
	}
	return func(ctx context.Context, dir string, args ...string) (string, error) {
		if err := refuseDestructiveGit(args); err != nil {
			return "", err
		}
		return inner(ctx, dir, args...)
	}
}

func refuseDestructiveGit(args []string) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "push":
		for _, a := range args[1:] {
			if a == "--force" || a == "-f" || a == "--force-with-lease" || strings.HasPrefix(a, "--force=") {
				return fmt.Errorf("refusing force push")
			}
		}
	case "reset":
		for _, a := range args[1:] {
			if a == "--hard" || a == "--merge" {
				return fmt.Errorf("refusing destructive git reset")
			}
		}
	case "rebase":
		return fmt.Errorf("refusing git rebase")
	}
	return nil
}

func (c *Client) lookPath() func(string) (string, error) {
	if c != nil && c.LookPath != nil {
		return c.LookPath
	}
	return exec.LookPath
}

// Inspect reads branch, HEAD, and dirty state. .project/ metadata does not count as dirty.
func (c *Client) Inspect(ctx context.Context, root string) (Snapshot, error) {
	if _, err := c.lookPath()("git"); err != nil {
		return Snapshot{}, fmt.Errorf("git is not available on PATH")
	}
	root = filepath.Clean(root)
	sn := Snapshot{Root: root}

	inside, err := c.git()(ctx, root, "rev-parse", "--is-inside-work-tree")
	if err != nil || inside != "true" {
		return sn, fmt.Errorf("product root is not a Git repository")
	}

	top, err := c.git()(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return sn, fmt.Errorf("cannot resolve Git toplevel: %w", err)
	}
	top = canonicalPath(top)
	root = canonicalPath(root)
	sn.Root = root
	if top != root {
		return sn, fmt.Errorf("Git toplevel %s is not the product root %s", top, root)
	}

	head, err := c.git()(ctx, root, "rev-parse", "HEAD")
	if err != nil || !looksLikeSHA(head) {
		return sn, fmt.Errorf("repository has no recoverable HEAD commit")
	}
	sn.HeadSHA = head

	branch, berr := c.git()(ctx, root, "branch", "--show-current")
	if berr == nil {
		sn.Branch = branch
	}

	porcelain, err := c.git()(ctx, root, "status", "--porcelain")
	if err != nil {
		return sn, err
	}
	sn.DirtyPaths = productDirtyPaths(porcelain)
	sn.Dirty = len(sn.DirtyPaths) > 0
	return sn, nil
}

// EstablishBaseline records HEAD when the tree is an acceptable recoverable checkpoint.
func (c *Client) EstablishBaseline(ctx context.Context, root string) (Baseline, Snapshot, error) {
	sn, err := c.Inspect(ctx, root)
	if err != nil {
		return Baseline{}, sn, err
	}
	if sn.Dirty {
		return Baseline{}, sn, fmt.Errorf("working tree is dirty; refusing to establish a Git baseline (%s)", strings.Join(sn.DirtyPaths, ", "))
	}
	if sn.HeadSHA == "" {
		return Baseline{}, sn, fmt.Errorf("repository has no recoverable HEAD commit")
	}
	return Baseline{SHA: sn.HeadSHA, Branch: sn.Branch}, sn, nil
}

// ChangedSince lists product paths that differ from baseline, excluding .project/.
func (c *Client) ChangedSince(ctx context.Context, root, sha string, jail *fsguard.Jail) ([]string, error) {
	if strings.TrimSpace(sha) == "" {
		return nil, fmt.Errorf("baseline SHA is missing")
	}
	root = canonicalPath(root)
	committed, err := c.git()(ctx, root, "diff", "--name-only", sha, "HEAD")
	if err != nil {
		return nil, err
	}
	unstaged, err := c.git()(ctx, root, "diff", "--name-only", sha)
	if err != nil {
		return nil, err
	}
	untracked, err := c.git()(ctx, root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, block := range []string{committed, unstaged, untracked} {
		for _, line := range splitLines(block) {
			rel := strings.TrimSpace(line)
			if rel == "" || isProjectMeta(rel) {
				continue
			}
			abs := canonicalPath(filepath.Join(root, rel))
			if jail != nil && !jail.Contains(abs) {
				return nil, fmt.Errorf("worker change %q is outside product root %q", rel, root)
			}
			if seen[rel] {
				continue
			}
			seen[rel] = true
			out = append(out, rel)
		}
	}
	return out, nil
}

// ChangeSet is a read-only classification of workspace changes vs a baseline SHA.
type ChangeSet struct {
	All       []string `json:"all,omitempty"`
	Untracked []string `json:"untracked,omitempty"`
	Deleted   []string `json:"deleted,omitempty"`
	HeadSHA   string   `json:"head_sha,omitempty"`
	Branch    string   `json:"branch,omitempty"`
}

// ChangesFrom lists product changes since baseline without mutating Git.
func (c *Client) ChangesFrom(ctx context.Context, root, sha string, jail *fsguard.Jail) (ChangeSet, error) {
	sn, err := c.Inspect(ctx, root)
	if err != nil {
		return ChangeSet{}, err
	}
	all, err := c.ChangedSince(ctx, root, sha, jail)
	if err != nil {
		return ChangeSet{}, err
	}
	untrackedRaw, err := c.git()(ctx, root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return ChangeSet{}, err
	}
	deletedRaw, err := c.git()(ctx, root, "diff", "--name-only", "--diff-filter=D", sha)
	if err != nil {
		return ChangeSet{}, err
	}
	filter := func(block string) []string {
		var out []string
		for _, line := range splitLines(block) {
			rel := strings.TrimSpace(line)
			if rel == "" || isProjectMeta(rel) {
				continue
			}
			abs := canonicalPath(filepath.Join(root, rel))
			if jail != nil && !jail.Contains(abs) {
				continue
			}
			out = append(out, rel)
		}
		return out
	}
	return ChangeSet{
		All:       all,
		Untracked: filter(untrackedRaw),
		Deleted:   filter(deletedRaw),
		HeadSHA:   sn.HeadSHA,
		Branch:    sn.Branch,
	}, nil
}

func productDirtyPaths(porcelain string) []string {
	var out []string
	for _, line := range splitLines(porcelain) {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		path = strings.Trim(path, `"`)
		if path == "" || isProjectMeta(path) {
			continue
		}
		out = append(out, path)
	}
	return out
}

func isProjectMeta(rel string) bool {
	rel = filepath.ToSlash(rel)
	rel = strings.TrimPrefix(rel, "./")
	return rel == projectMetaDir || strings.HasPrefix(rel, projectMetaDir+"/")
}

func looksLikeSHA(s string) bool {
	if len(s) < 7 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func splitLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func canonicalPath(p string) string {
	p = filepath.Clean(p)
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}
