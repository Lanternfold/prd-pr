package vcs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/fsguard"
)

const defaultIgnore = `# PRD→PR product gitignore
.project/
.env
.env.*
*.pem
credentials.json
`

var secretNamed = []string{
	".env",
	"credentials.json",
	"id_rsa",
	"id_ecdsa",
	"id_ed25519",
}

// Init creates a Git repository at root if one does not already exist.
func (c *Client) Init(ctx context.Context, root, branch string) error {
	root = canonicalPath(root)
	obs := c.Observe(ctx, root)
	if obs.IsRepo {
		return nil
	}
	if branch == "" {
		branch = "main"
	}
	if _, err := c.git()(ctx, root, "init", "--template=", "-b", branch); err != nil {
		if _, err2 := c.git()(ctx, root, "init", "--template="); err2 != nil {
			return err
		}
		_, _ = c.git()(ctx, root, "checkout", "-b", branch)
	}
	return nil
}

// EnsureIgnore writes a safe .gitignore when missing, or appends .project/ when absent.
// It does not rewrite an existing ignore file.
func (c *Client) EnsureIgnore(ctx context.Context, root string) error {
	root = canonicalPath(root)
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(path, []byte(defaultIgnore), 0o644)
		}
		return err
	}
	text := string(data)
	if gitignoreHasProject(text) {
		return nil
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return os.WriteFile(path, []byte(text+".project/\n"), 0o644)
}

func gitignoreHasProject(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == ".project/" || line == ".project" || line == "/.project/" || line == "/.project" {
			return true
		}
	}
	return false
}

// InitialCommit creates the first product commit when HEAD is missing.
// It does not amend, reset history, or include .project/.
func (c *Client) InitialCommit(ctx context.Context, root, message string, jail *fsguard.Jail) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "chore: initialize product"
	}
	root = canonicalPath(root)
	obs := c.Observe(ctx, root)
	if obs.HasHEAD {
		return obs.HeadSHA, nil
	}
	if !obs.IsRepo {
		return "", fmt.Errorf("product root is not a Git repository")
	}
	if err := c.EnsureIgnore(ctx, root); err != nil {
		return "", err
	}
	paths, err := c.commitCandidates(ctx, root, jail)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		readme := filepath.Join(root, "README.md")
		if _, err := os.Stat(readme); os.IsNotExist(err) {
			_ = os.WriteFile(readme, []byte("# Product\n"), 0o644)
			paths = []string{"README.md"}
		}
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("nothing to commit for initial baseline")
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := c.git()(ctx, root, args...); err != nil {
		return "", err
	}
	if _, err := c.git()(ctx, root, "commit", "-m", message); err != nil {
		return "", err
	}
	return c.git()(ctx, root, "rev-parse", "HEAD")
}

func (c *Client) commitCandidates(ctx context.Context, root string, jail *fsguard.Jail) ([]string, error) {
	tracked, err := c.git()(ctx, root, "ls-files")
	if err != nil {
		return nil, err
	}
	untracked, err := c.git()(ctx, root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, block := range []string{tracked, untracked} {
		for _, line := range splitLines(block) {
			rel := strings.TrimSpace(line)
			if rel == "" || isProjectMeta(rel) || looksSecretPath(rel) {
				continue
			}
			abs := canonicalPath(filepath.Join(root, rel))
			if jail != nil && !jail.Contains(abs) {
				continue
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

func looksSecretPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)
	for _, n := range secretNamed {
		if strings.EqualFold(base, n) {
			return true
		}
	}
	if strings.HasPrefix(base, ".env.") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(base))
	return ext == ".pem" || ext == ".key"
}

// AddRemoteIfMissing adds name→url when the remote does not exist. It never overwrites.
func (c *Client) AddRemoteIfMissing(ctx context.Context, root, name, url string) error {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" || url == "" {
		return fmt.Errorf("remote name and url are required")
	}
	remotes, err := c.RemoteState(ctx, root)
	if err != nil {
		return err
	}
	for _, r := range remotes {
		if r.Name == name {
			return nil
		}
	}
	_, err = c.git()(ctx, root, "remote", "add", name, url)
	return err
}

// RemoteURL returns the fetch URL for name, or empty if missing.
func (c *Client) RemoteURL(ctx context.Context, root, name string) string {
	if name == "" {
		name = "origin"
	}
	out, err := c.git()(ctx, root, "remote", "get-url", name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// Clone clones url into dest. dest must not already be a Git repository.
func (c *Client) Clone(ctx context.Context, url, dest string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("clone url is required")
	}
	if _, err := os.Stat(dest); err == nil {
		obs := c.Observe(ctx, dest)
		if obs.IsRepo {
			return nil
		}
		return fmt.Errorf("refusing to clone into existing path %s", dest)
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	_, err := c.git()(ctx, parent, "clone", url, dest)
	return err
}

// EnsureBranch switches to name, creating it from HEAD when missing. It does not rewrite existing branches.
func (c *Client) EnsureBranch(ctx context.Context, root, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("branch name is required")
	}
	current, _ := c.git()(ctx, root, "branch", "--show-current")
	if strings.TrimSpace(current) == name {
		return nil
	}
	if _, err := c.git()(ctx, root, "rev-parse", "--verify", name); err == nil {
		_, err := c.git()(ctx, root, "checkout", name)
		return err
	}
	_, err := c.git()(ctx, root, "checkout", "-b", name)
	return err
}

// Pushed reports whether local SHA is already on remote/ref.
func (c *Client) Pushed(ctx context.Context, root, remote, ref, sha string) bool {
	if remote == "" || ref == "" || sha == "" {
		return false
	}
	out, err := c.git()(ctx, root, "rev-parse", remote+"/"+ref)
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) == sha
}
