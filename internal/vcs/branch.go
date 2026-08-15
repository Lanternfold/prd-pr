package vcs

import (
	"context"
	"fmt"
	"strings"
)

// Branch describes one local branch.
type Branch struct {
	Name     string `json:"name,omitempty"`
	SHA      string `json:"sha,omitempty"`
	Current  bool   `json:"current,omitempty"`
	Exists   bool   `json:"exists,omitempty"`
	Base     string `json:"base,omitempty"`
	Conflict bool   `json:"conflict,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// FeatureBranchName is the deterministic per-project PR head.
func FeatureBranchName(projectID string) string {
	id := strings.TrimSpace(projectID)
	if id == "" {
		id = "project"
	}
	var b strings.Builder
	b.WriteString("prdpr/")
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// InspectBranch reports whether name exists and its SHA. It does not mutate.
func (c *Client) InspectBranch(ctx context.Context, root, name string) (Branch, error) {
	name = strings.TrimSpace(name)
	out := Branch{Name: name}
	if name == "" {
		return out, fmt.Errorf("branch name is required")
	}
	current, _ := c.git()(ctx, root, "branch", "--show-current")
	out.Current = strings.TrimSpace(current) == name
	sha, err := c.git()(ctx, root, "rev-parse", "--verify", name)
	if err != nil {
		return out, nil
	}
	out.Exists = true
	out.SHA = strings.TrimSpace(sha)
	return out, nil
}

// EnsureOwnedBranch creates or reuses name from HEAD when it belongs to this execution.
// An unrelated existing branch is a conflict, not silent reuse.
func (c *Client) EnsureOwnedBranch(ctx context.Context, root, name, ownedName, headSHA string) (Branch, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Branch{}, fmt.Errorf("branch name is required")
	}
	br, err := c.InspectBranch(ctx, root, name)
	if err != nil {
		return br, err
	}
	head, _ := c.git()(ctx, root, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	if !br.Exists {
		if err := c.EnsureBranch(ctx, root, name); err != nil {
			return br, err
		}
		br, err = c.InspectBranch(ctx, root, name)
		br.Reason = "created"
		return br, err
	}
	owned := strings.TrimSpace(ownedName) == name
	sameTip := br.SHA != "" && (br.SHA == head || br.SHA == strings.TrimSpace(headSHA))
	if owned || sameTip || br.Current {
		if err := c.EnsureBranch(ctx, root, name); err != nil {
			return br, err
		}
		br, err = c.InspectBranch(ctx, root, name)
		br.Reason = "reused"
		return br, err
	}
	br.Conflict = true
	br.Reason = "unrelated existing branch; refusing silent reuse"
	return br, fmt.Errorf("branch conflict: %s already exists and is not owned by this execution", name)
}

// DeleteBranch deletes a local branch. It uses -d, not -D.
func (c *Client) DeleteBranch(ctx context.Context, root, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name is required")
	}
	current, _ := c.git()(ctx, root, "branch", "--show-current")
	if strings.TrimSpace(current) == name {
		return fmt.Errorf("refusing to delete the current branch %s", name)
	}
	_, err := c.git()(ctx, root, "branch", "-d", name)
	return err
}

// DeleteRemoteBranch deletes ref on remote without force.
func (c *Client) DeleteRemoteBranch(ctx context.Context, root, remote, name string) error {
	if remote == "" {
		remote = "origin"
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name is required")
	}
	_, err := c.git()(ctx, root, "push", remote, "--delete", name)
	return err
}

// FastForwardBase updates local base from remote with --ff-only. Dirty trees are left untouched.
func (c *Client) FastForwardBase(ctx context.Context, root, base, remote string) error {
	if base == "" {
		base = "main"
	}
	if remote == "" {
		remote = "origin"
	}
	sn, err := c.Inspect(ctx, root)
	if err == nil && sn.Dirty {
		return fmt.Errorf("working tree is dirty; refusing to move local %s", base)
	}
	current := sn.Branch
	if _, err := c.git()(ctx, root, "fetch", remote, base); err != nil {
		return err
	}
	if err := c.EnsureBranch(ctx, root, base); err != nil {
		return err
	}
	if _, err := c.git()(ctx, root, "merge", "--ff-only", remote+"/"+base); err != nil {
		return err
	}
	if current != "" && current != base {
		_, _ = c.git()(ctx, root, "checkout", current)
	}
	return nil
}
