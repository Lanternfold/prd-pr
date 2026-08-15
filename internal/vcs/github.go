package vcs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// PR is a GitHub pull request. The adapter never auto-merges unless MergePR is called.
type PR struct {
	URL            string `json:"url,omitempty"`
	Number         string `json:"number,omitempty"`
	Head           string `json:"head,omitempty"`
	Base           string `json:"base,omitempty"`
	SHA            string `json:"sha,omitempty"`
	State          string `json:"state,omitempty"`
	Mergeable      string `json:"mergeable,omitempty"`
	ReviewDecision string `json:"review_decision,omitempty"`
	Skipped        bool   `json:"skipped,omitempty"`
	Reused         bool   `json:"reused,omitempty"`
	Reason         string `json:"reason,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
}

type MergeResult struct {
	SHA    string `json:"sha,omitempty"`
	Merged bool   `json:"merged"`
	Method string `json:"method,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type RepoSpec struct {
	Owner       string
	Name        string
	Visibility  string
	Description string
}

type GHClient struct {
	LookPath func(string) (string, error)
	GH       GitFunc
}

func DefaultGH() *GHClient {
	return &GHClient{
		LookPath: exec.LookPath,
		GH: func(ctx context.Context, dir string, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, "gh", args...)
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
				return out, fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
			}
			return out, nil
		},
	}
}

func (g *GHClient) Available() bool {
	if g == nil {
		g = DefaultGH()
	}
	look := g.LookPath
	if look == nil {
		look = exec.LookPath
	}
	_, err := look("gh")
	return err == nil
}

func (g *GHClient) gh() GitFunc {
	if g != nil && g.GH != nil {
		return g.GH
	}
	return DefaultGH().GH
}

// Authenticated reports whether gh can talk to GitHub. It does not print tokens.
func (g *GHClient) Authenticated(ctx context.Context) bool {
	if !g.Available() {
		return false
	}
	_, err := g.gh()(ctx, "", "auth", "status")
	return err == nil
}

// RepoExists reports whether owner/name exists. False, nil means not found.
func (g *GHClient) RepoExists(ctx context.Context, owner, name string) (bool, string, error) {
	owner, name = strings.TrimSpace(owner), strings.TrimSpace(name)
	if owner == "" || name == "" {
		return false, "", fmt.Errorf("repository owner and name are required")
	}
	if !g.Available() {
		return false, "", fmt.Errorf("gh is not available")
	}
	out, err := g.gh()(ctx, "", "repo", "view", owner+"/"+name, "--json", "url", "-q", ".url")
	if err != nil {
		return false, "", nil
	}
	return strings.TrimSpace(out) != "", strings.TrimSpace(out), nil
}

// CreateRepo creates owner/name. It does not clone, push, or make the repo public unless requested.
func (g *GHClient) CreateRepo(ctx context.Context, spec RepoSpec) (string, error) {
	spec.Owner = strings.TrimSpace(spec.Owner)
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Owner == "" || spec.Name == "" {
		return "", fmt.Errorf("repository owner and name are required; refusing to invent them")
	}
	if !g.Available() {
		return "", fmt.Errorf("gh is not available")
	}
	args := []string{"repo", "create", spec.Owner + "/" + spec.Name}
	if strings.EqualFold(spec.Visibility, "public") {
		args = append(args, "--public")
	} else {
		args = append(args, "--private")
	}
	if strings.TrimSpace(spec.Description) != "" {
		args = append(args, "--description", spec.Description)
	}
	out, err := g.gh()(ctx, "", args...)
	if err != nil {
		return "", err
	}
	if u := firstURL(out); u != "" {
		return u, nil
	}
	return "https://github.com/" + spec.Owner + "/" + spec.Name, nil
}

// OpenPR creates one PR from the current branch. It does not merge.
func (g *GHClient) OpenPR(ctx context.Context, root, title, body, base string) (PR, error) {
	return g.EnsurePR(ctx, root, title, body, base, "")
}

// FindOpenPR returns an open PR for head/base if one exists.
func (g *GHClient) FindOpenPR(ctx context.Context, root, head, base string) (PR, error) {
	if !g.Available() {
		return PR{Skipped: true, Reason: "gh is not available; local workflow continues"}, nil
	}
	args := []string{"pr", "list", "--state", "open", "--json", "number,url,headRefName,baseRefName,headRefOid,state,createdAt"}
	if strings.TrimSpace(head) != "" {
		args = append(args, "--head", head)
	}
	if strings.TrimSpace(base) != "" {
		args = append(args, "--base", base)
	}
	out, err := g.gh()(ctx, root, args...)
	if err != nil {
		return PR{}, err
	}
	var list []struct {
		Number      int    `json:"number"`
		URL         string `json:"url"`
		HeadRefName string `json:"headRefName"`
		BaseRefName string `json:"baseRefName"`
		HeadRefOid  string `json:"headRefOid"`
		State       string `json:"state"`
		CreatedAt   string `json:"createdAt"`
	}
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		return PR{}, fmt.Errorf("decode pr list: %w", err)
	}
	if len(list) == 0 {
		return PR{}, nil
	}
	p := list[0]
	return PR{
		URL:       p.URL,
		Number:    fmt.Sprintf("%d", p.Number),
		Head:      p.HeadRefName,
		Base:      p.BaseRefName,
		SHA:       p.HeadRefOid,
		State:     strings.ToLower(p.State),
		Reused:    true,
		CreatedAt: p.CreatedAt,
	}, nil
}

// EnsurePR reuses a matching open PR or creates one. It does not merge.
func (g *GHClient) EnsurePR(ctx context.Context, root, title, body, base, head string) (PR, error) {
	if g == nil {
		g = DefaultGH()
	}
	if !g.Available() {
		return PR{Skipped: true, Reason: "gh is not available; local workflow continues"}, nil
	}
	if title == "" {
		title = "PRD→PR milestone"
	}
	if base == "" {
		base = "main"
	}
	existing, err := g.FindOpenPR(ctx, root, head, base)
	if err != nil {
		return PR{Skipped: true, Reason: err.Error()}, nil
	}
	if existing.Number != "" {
		existing.Reused = true
		return existing, nil
	}
	args := []string{"pr", "create", "--title", title, "--body", body, "--base", base}
	if strings.TrimSpace(head) != "" {
		args = append(args, "--head", head)
	}
	out, err := g.gh()(ctx, root, args...)
	if err != nil {
		return PR{Skipped: true, Reason: err.Error()}, nil
	}
	pr := PR{URL: firstURL(out), Base: base, Head: head, State: "open"}
	pr.Number = numberFromPRURL(pr.URL)
	if pr.Number == "" {
		if viewed, verr := g.ViewPR(ctx, root, head); verr == nil {
			pr = viewed
		}
	}
	return pr, nil
}

// ViewPR reads PR metadata. It does not merge.
func (g *GHClient) ViewPR(ctx context.Context, root, selector string) (PR, error) {
	if !g.Available() {
		return PR{Skipped: true, Reason: "gh is not available; local workflow continues"}, nil
	}
	sel := strings.TrimSpace(selector)
	args := []string{"pr", "view", "--json", "number,url,headRefName,baseRefName,headRefOid,state,mergeable,reviewDecision,createdAt"}
	if sel != "" {
		args = []string{"pr", "view", sel, "--json", "number,url,headRefName,baseRefName,headRefOid,state,mergeable,reviewDecision,createdAt"}
	}
	out, err := g.gh()(ctx, root, args...)
	if err != nil {
		return PR{Skipped: true, Reason: err.Error()}, nil
	}
	var raw struct {
		Number         int    `json:"number"`
		URL            string `json:"url"`
		HeadRefName    string `json:"headRefName"`
		BaseRefName    string `json:"baseRefName"`
		HeadRefOid     string `json:"headRefOid"`
		State          string `json:"state"`
		Mergeable      string `json:"mergeable"`
		ReviewDecision string `json:"reviewDecision"`
		CreatedAt      string `json:"createdAt"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return PR{}, fmt.Errorf("decode pr view: %w", err)
	}
	return PR{
		URL:            raw.URL,
		Number:         fmt.Sprintf("%d", raw.Number),
		Head:           raw.HeadRefName,
		Base:           raw.BaseRefName,
		SHA:            raw.HeadRefOid,
		State:          strings.ToLower(raw.State),
		Mergeable:      strings.ToUpper(raw.Mergeable),
		ReviewDecision: strings.ToUpper(raw.ReviewDecision),
		CreatedAt:      raw.CreatedAt,
	}, nil
}

// MergePR merges an open PR. Engine policy must already have allowed it.
func (g *GHClient) MergePR(ctx context.Context, root, number, method string) (MergeResult, error) {
	if !g.Available() {
		return MergeResult{Reason: "gh is not available"}, nil
	}
	number = strings.TrimSpace(number)
	if number == "" {
		return MergeResult{Reason: "PR number is required"}, nil
	}
	flag := "--squash"
	switch strings.ToLower(method) {
	case "merge":
		flag = "--merge"
	case "rebase":
		return MergeResult{Reason: "refusing rebase merge; history rewrite is not allowed", Method: method}, nil
	}
	out, err := g.gh()(ctx, root, "pr", "merge", number, flag)
	if err != nil {
		return MergeResult{Reason: err.Error(), Method: method}, nil
	}
	sha := ""
	for _, line := range splitLines(out) {
		if looksLikeSHA(line) {
			sha = line
			break
		}
	}
	return MergeResult{Merged: true, SHA: sha, Method: method}, nil
}

func numberFromPRURL(u string) string {
	u = strings.TrimSpace(u)
	i := strings.LastIndex(u, "/pull/")
	if i < 0 {
		return ""
	}
	n := strings.Trim(u[i+6:], "/")
	for _, r := range n {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return n
}

func firstURL(s string) string {
	for _, line := range splitLines(s) {
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line
		}
	}
	return strings.TrimSpace(s)
}

// ParseRepoRef extracts owner/name from configured metadata. It does not guess.
func ParseRepoRef(ref string) (owner, name string) {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimSuffix(ref, ".git")
	ref = strings.TrimPrefix(ref, "https://github.com/")
	ref = strings.TrimPrefix(ref, "http://github.com/")
	ref = strings.TrimPrefix(ref, "git@github.com:")
	ref = strings.Trim(ref, "/")
	parts := strings.Split(ref, "/")
	if len(parts) < 2 {
		return "", ""
	}
	owner, name = parts[len(parts)-2], parts[len(parts)-1]
	if owner == "" || name == "" || strings.Contains(owner, ":") {
		return "", ""
	}
	return owner, name
}
