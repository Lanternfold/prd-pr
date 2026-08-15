package state

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lanternfold/prd-pr/internal/fsguard"
)

const (
	// SchemaVersion is the P1 snapshot format. No migrations yet.
	SchemaVersion = 1

	DirName        = ".project"
	StateFileName  = "state.json"
	EventsFileName = "events.jsonl"
	LockFileName   = "LOCK"

	StatusCreated          = "PROJECT_CREATED"
	StatusProjectCompleted = "PROJECT_COMPLETED"

	KindIntent = "intent"
	KindResult = "result"

	EventProjectInitialized        = "project_initialized"
	EventRunStarted                = "run_started"
	EventStateChanged              = "state_changed"
	EventRunEnded                  = "run_ended"
	EventWorkerInvoked             = "worker_invoked"
	EventWorkerFinished            = "worker_finished"
	EventWorkerRefused             = "worker_refused"
	EventPrepared                  = "prepared"
	EventPrepareRefused            = "prepare_refused"
	EventVerificationStarted       = "verification_started"
	EventVerificationTestCompleted = "verification_test_completed"
	EventVerificationPassed        = "verification_passed"
	EventVerificationFailed        = "verification_failed"
	EventReviewStarted             = "review_started"
	EventReviewCompleted           = "review_completed"
	EventReviewFailed              = "review_failed"
	EventRepairStarted             = "repair_started"
	EventRepairAttempt             = "repair_attempt"
	EventRepairExhausted           = "repair_exhausted"
	EventHumanRequested            = "human_requested"
	EventHumanResolved             = "human_resolved"
	EventHumanTimeout              = "human_timeout"
	EventPhaseCompleted            = "phase_completed"
	EventCommitCreated             = "commit_created"
	EventPushSkipped               = "push_skipped"
	EventPushSucceeded             = "push_succeeded"
	EventPushFailed                = "push_failed"
	EventRepoDiscovered            = "repo_discovered"
	EventRepoCreated               = "repo_created"
	EventRepoBootstrapped          = "repo_bootstrapped"
	EventRepoSkipped               = "repo_skipped"
	EventPROpened                  = "pr_opened"
	EventPRReused                  = "pr_reused"
	EventBranchCreated             = "branch_created"
	EventBranchReused              = "branch_reused"
	EventMergeBlocked              = "merge_blocked"
	EventMergeCompleted            = "merge_completed"
	EventBranchDeleted             = "branch_deleted"
	EventGitHubBlocked             = "github_blocked"
	EventKnowledgeUpdated          = "knowledge_updated"
	EventCostRecorded              = "cost_recorded"
	EventSubagentDecision          = "subagent_decision"
	EventRecovered                 = "recovered"

	StatePrepared               = "PREPARED"
	StateWorkerRunning          = "WORKER_RUNNING"
	StateWorkerClaimedDone      = "WORKER_CLAIMED_DONE"
	StateVerifying              = "VERIFYING"
	StateVerified               = "VERIFIED"
	StateWorkerRefused          = "WORKER_REFUSED"
	StateVerificationFailed     = "VERIFICATION_FAILED"
	StateVerificationIncomplete = "VERIFICATION_INCOMPLETE"
	StateReviewing              = "REVIEWING"
	StateRepairing              = "REPAIRING"
	StateWaitingForHuman        = "WAITING_FOR_HUMAN"
	StateFailed                 = "FAILED"
	StateCompleted              = "COMPLETED"

	PushSkipped = "skipped"
	PushPending = "pending"
	PushPushed  = "pushed"
	PushFailed  = "PUSH_FAILED"

	GitHubDisabled    = "disabled"
	GitHubUnavailable = "unavailable"
	GitHubExists      = "exists"
	GitHubCreated     = "created"
	GitHubMissing     = "missing"
	GitHubSkipped     = "skipped"

	RepoTypeLocal  = "local"
	RepoTypeGitHub = "github"

	PRWaitingForMerge = "PR_WAITING_FOR_MERGE"
	MergeMerged       = "merged"
	MergeBlocked      = "blocked"

	GitHubActionBlocked = "GITHUB ACTION BLOCKED"
)

// State is the current-project snapshot. It is the source of truth for resume.
type State struct {
	SchemaVersion       int        `json:"schema_version"`
	ProjectID           string     `json:"project_id"`
	ProductRoot         string     `json:"product_root"`
	ProjectStatus       string     `json:"project_status"`
	CurrentRunID        string     `json:"current_run_id"`
	CurrentPhaseID      string     `json:"current_phase_id"`
	CurrentState        string     `json:"current_state"`
	CurrentCommit       string     `json:"current_commit"`
	LastKnownGoodCommit string     `json:"last_known_good_commit"`
	Repository          Repository `json:"repository,omitempty"`
	CreatedAt           string     `json:"created_at"`
	UpdatedAt           string     `json:"updated_at"`
}

// Repository is the product Git/GitHub lifecycle snapshot. It never stores credentials.
type Repository struct {
	Type                string            `json:"type,omitempty"`
	LocalRoot           string            `json:"local_root,omitempty"`
	RemoteName          string            `json:"remote_name,omitempty"`
	RemoteURL           string            `json:"remote_url,omitempty"`
	Branch              string            `json:"branch,omitempty"`
	InitialCommitSHA    string            `json:"initial_commit_sha,omitempty"`
	LatestCheckpointSHA string            `json:"latest_checkpoint_sha,omitempty"`
	CommitStatus        string            `json:"commit_status,omitempty"`
	PushStatus          string            `json:"push_status,omitempty"`
	GitHubStatus        string            `json:"github_status,omitempty"`
	SkipReason          string            `json:"skip_reason,omitempty"`
	PhaseCheckpoints    map[string]string `json:"phase_checkpoints,omitempty"`
	BaseBranch          string            `json:"base_branch,omitempty"`
	FeatureBranch       string            `json:"feature_branch,omitempty"`
	BranchState         string            `json:"branch_state,omitempty"`
	BranchSHA           string            `json:"branch_sha,omitempty"`
	PRNumber            string            `json:"pr_number,omitempty"`
	PRURL               string            `json:"pr_url,omitempty"`
	PRHead              string            `json:"pr_head,omitempty"`
	PRBase              string            `json:"pr_base,omitempty"`
	PRSHA               string            `json:"pr_sha,omitempty"`
	PRCreatedAt         string            `json:"pr_created_at,omitempty"`
	MergeSHA            string            `json:"merge_sha,omitempty"`
	MergeMethod         string            `json:"merge_method,omitempty"`
	MergeAt             string            `json:"merge_at,omitempty"`
	MergeStatus         string            `json:"merge_status,omitempty"`
	ChecksStatus        string            `json:"checks_status,omitempty"`
	GitHubBlock         string            `json:"github_block,omitempty"`
}

// Event is one append-only journal line. It is not the source of truth.
type Event struct {
	SchemaVersion int             `json:"schema_version"`
	Timestamp     string          `json:"timestamp"`
	Kind          string          `json:"kind"`
	Name          string          `json:"name"`
	RunID         string          `json:"run_id,omitempty"`
	PhaseID       string          `json:"phase_id,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

// Options injects filesystem and process behavior for tests.
type Options struct {
	Rename   func(oldpath, newpath string) error
	PID      func() int
	Alive    func(pid int) bool
	Now      func() time.Time
	Hostname func() string
	NewID    func() string
}

// Store persists orchestration metadata under <product>/.project/.
type Store struct {
	jail     *fsguard.Jail
	rename   func(oldpath, newpath string) error
	pid      func() int
	alive    func(pid int) bool
	now      func() time.Time
	hostname func() string
	newID    func() string
}

// Open returns a store jailed to productRoot.
func Open(productRoot string) (*Store, error) {
	return OpenWith(productRoot, Options{})
}

// OpenWith is Open with injectable behavior.
func OpenWith(productRoot string, opts Options) (*Store, error) {
	jail, err := fsguard.New(productRoot)
	if err != nil {
		return nil, err
	}
	s := &Store{jail: jail}
	s.rename = os.Rename
	if opts.Rename != nil {
		s.rename = opts.Rename
	}
	s.pid = os.Getpid
	if opts.PID != nil {
		s.pid = opts.PID
	}
	s.alive = pidAlive
	if opts.Alive != nil {
		s.alive = opts.Alive
	}
	s.now = func() time.Time { return time.Now().UTC() }
	if opts.Now != nil {
		s.now = opts.Now
	}
	s.hostname = func() string {
		h, _ := os.Hostname()
		return h
	}
	if opts.Hostname != nil {
		s.hostname = opts.Hostname
	}
	s.newID = randomID
	if opts.NewID != nil {
		s.newID = opts.NewID
	}
	return s, nil
}

// Root is the jailed product directory.
func (s *Store) Root() string {
	return s.jail.Root()
}

func (s *Store) resolve(rel string) (string, error) {
	return s.jail.Resolve(rel)
}

func (s *Store) statePath() (string, error) {
	return s.resolve(filepath.Join(DirName, StateFileName))
}

func (s *Store) eventsPath() (string, error) {
	return s.resolve(filepath.Join(DirName, EventsFileName))
}

func (s *Store) lockPath() (string, error) {
	return s.resolve(filepath.Join(DirName, LockFileName))
}

func (s *Store) projectDir() (string, error) {
	return s.resolve(DirName)
}

func (s *Store) timestamp() string {
	return s.now().UTC().Format(time.RFC3339Nano)
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("prj_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func marshalState(st State) ([]byte, error) {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
