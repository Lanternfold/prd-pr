package config

import (
	"strings"
	"time"
)

const (
	BudgetDowngrade = "downgrade"
	BudgetAskHuman  = "ask_human"
	BudgetContinue  = "continue"

	PRBoundaryRun   = "run"
	PRBoundaryNever = "never"
	PRBoundaryPhase = "phase"

	VisibilityPrivate = "private"
	VisibilityPublic  = "public"

	BranchMain = "main"

	MergeSquash = "squash"
	MergeMerge  = "merge"
	MergeRebase = "rebase"
)

// Config holds user and project settings. Secrets are never stored here.
type Config struct {
	HumanTimeout           time.Duration `json:"human_timeout"`
	MaxRepairAttempts      int           `json:"max_repair_attempts"`
	CostBudgetUSD          float64       `json:"cost_budget_usd"`
	BudgetBreachPolicy     string        `json:"budget_breach_policy"`
	GitHubEnabled          bool          `json:"github_enabled"`
	GitHubOwner            string        `json:"github_owner,omitempty"`
	GitHubRepo             string        `json:"github_repo,omitempty"`
	GitHubVisibility       string        `json:"github_visibility,omitempty"`
	GitHubDescription      string        `json:"github_description,omitempty"`
	DefaultBranch          string        `json:"default_branch,omitempty"`
	UseFeatureBranch       bool          `json:"use_feature_branch,omitempty"`
	RemoteName             string        `json:"remote_name,omitempty"`
	InitialCommitMessage   string        `json:"initial_commit_message,omitempty"`
	AutoCommit             bool          `json:"auto_commit"`
	AutoPush               bool          `json:"auto_push"`
	PRBoundary             string        `json:"pr_boundary"`
	AutoMergeEnabled       bool          `json:"auto_merge_enabled"`
	MergeMethod            string        `json:"merge_method,omitempty"`
	RequireApproval        bool          `json:"require_approval"`
	RequiredChecks         []string      `json:"required_checks,omitempty"`
	DeleteBranchAfterMerge bool          `json:"delete_branch_after_merge"`
	CheapModel             string        `json:"cheap_model"`
	StrongModel            string        `json:"strong_model"`
	StudioRoot             string        `json:"studio_root,omitempty"`
	RulesetsEnabled        bool          `json:"rulesets_enabled"`
	RequiredApprovals      int           `json:"required_approvals,omitempty"`
	AllowForcePush         bool          `json:"allow_force_push"`
	AllowBranchDeletion    bool          `json:"allow_branch_deletion"`
}

// Defaults returns V1 local-first configuration.
func Defaults() Config {
	return Config{
		HumanTimeout:           30 * time.Second,
		MaxRepairAttempts:      3,
		CostBudgetUSD:          0,
		BudgetBreachPolicy:     BudgetAskHuman,
		GitHubEnabled:          false,
		GitHubVisibility:       VisibilityPrivate,
		DefaultBranch:          BranchMain,
		RemoteName:             "origin",
		InitialCommitMessage:   "chore: initialize product",
		AutoCommit:             true,
		AutoPush:               true,
		PRBoundary:             PRBoundaryRun,
		AutoMergeEnabled:       false,
		MergeMethod:            MergeSquash,
		RequireApproval:        false,
		DeleteBranchAfterMerge: false,
		CheapModel:             "cheap",
		StrongModel:            "strong",
		RulesetsEnabled:        true,
	}
}

// RemoteVisibility is the GitHub visibility used for new repositories.
// Unset or unknown values are private so a first-time product is never public by accident.
func (c Config) RemoteVisibility() string {
	if strings.EqualFold(strings.TrimSpace(c.GitHubVisibility), VisibilityPublic) {
		return VisibilityPublic
	}
	return VisibilityPrivate
}

func (c Config) Branch() string {
	b := strings.TrimSpace(c.DefaultBranch)
	if b == "" {
		return BranchMain
	}
	return b
}

func (c Config) Remote() string {
	n := strings.TrimSpace(c.RemoteName)
	if n == "" {
		return "origin"
	}
	return n
}

func (c Config) InitialMessage() string {
	m := strings.TrimSpace(c.InitialCommitMessage)
	if m == "" {
		return "chore: initialize product"
	}
	return m
}

func (c Config) MergeMethodName() string {
	switch strings.ToLower(strings.TrimSpace(c.MergeMethod)) {
	case MergeMerge:
		return MergeMerge
	case MergeRebase:
		return MergeRebase
	default:
		return MergeSquash
	}
}

// FeatureBranches reports whether the GitHub PR workflow should use a feature branch.
func (c Config) FeatureBranches() bool {
	if c.UseFeatureBranch {
		return true
	}
	return c.GitHubEnabled && c.PRBoundary != PRBoundaryNever
}
