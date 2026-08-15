package vcs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/fsguard"
)

// CreateBranch creates name from HEAD when missing, or checks it out.
// It does not move an existing branch with -B and does not push.
func (c *Client) CreateBranch(ctx context.Context, root, name string) error {
	return c.EnsureBranch(ctx, root, name)
}

// Commit stages listed product paths (or all product changes) and creates a commit.
// .project/ metadata is excluded unless explicitly listed.
func (c *Client) Commit(ctx context.Context, root, message string, paths []string, jail *fsguard.Jail) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", fmt.Errorf("commit message is required")
	}
	if isPlaceholderMessage(message) {
		return "", fmt.Errorf("refusing placeholder commit message %q", message)
	}
	root = canonicalPath(root)
	if len(paths) == 0 {
		changed, err := c.ChangedSince(ctx, root, "HEAD", jail)
		if err != nil {
			return "", err
		}
		paths = changed
	}
	var add []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || isProjectMeta(p) || looksSecretPath(p) {
			continue
		}
		if strings.Contains(p, "..") {
			return "", fmt.Errorf("refusing to commit path %q", p)
		}
		abs := canonicalPath(filepath.Join(root, p))
		if jail != nil && !jail.Contains(abs) {
			return "", fmt.Errorf("commit path %q is outside product root", p)
		}
		add = append(add, p)
	}
	if len(add) == 0 {
		return "", fmt.Errorf("nothing to commit")
	}
	args := append([]string{"add", "--"}, add...)
	if _, err := c.git()(ctx, root, args...); err != nil {
		return "", err
	}
	if _, err := c.git()(ctx, root, "commit", "-m", message); err != nil {
		return "", err
	}
	sha, err := c.git()(ctx, root, "rev-parse", "HEAD")
	return sha, err
}

// Push sends ref to remote without force or history rewrite.
// Missing remotes are a structured error, not a local-run failure.
func (c *Client) Push(ctx context.Context, root, remote, ref string) error {
	if remote == "" {
		remote = "origin"
	}
	if ref == "" {
		return fmt.Errorf("push ref is required")
	}
	if strings.HasPrefix(ref, "+") || strings.Contains(ref, ":") && strings.Contains(ref, "+") {
		return fmt.Errorf("refusing force push ref %q", ref)
	}
	_, err := c.git()(ctx, root, "push", "-u", remote, ref)
	return err
}

type Remote struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

func (c *Client) RemoteState(ctx context.Context, root string) ([]Remote, error) {
	out, err := c.git()(ctx, root, "remote", "-v")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var remotes []Remote
	for _, line := range splitLines(out) {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		key := f[0] + " " + f[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		remotes = append(remotes, Remote{Name: f[0], URL: f[1]})
	}
	return remotes, nil
}

func isPlaceholderMessage(message string) bool {
	m := strings.ToLower(strings.TrimSpace(message))
	switch m {
	case "update code", "changes", "ai stuff", "wip", "update", "misc":
		return true
	}
	return false
}

// CheckpointMessage is the deterministic verified-phase commit message.
func CheckpointMessage(phaseID, objective string) string {
	phaseID = strings.TrimSpace(phaseID)
	objective = compactMessage(objective)
	if phaseID == "" && objective == "" {
		return "feat: implement verified phase"
	}
	prefix := "feat"
	blob := strings.ToLower(phaseID + " " + objective)
	if strings.Contains(blob, "fix") || strings.Contains(blob, "bug") {
		prefix = "fix"
	}
	if objective == "" {
		return prefix + ": implement " + phaseID
	}
	if phaseID == "" {
		return prefix + ": " + objective
	}
	return prefix + ": implement " + phaseID + " " + objective
}

func compactMessage(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	fields := strings.Fields(s)
	s = strings.Join(fields, " ")
	if len(s) > 72 {
		s = strings.TrimSpace(s[:72])
	}
	return s
}
