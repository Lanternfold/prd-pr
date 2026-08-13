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

	StatusCreated = "PROJECT_CREATED"

	KindIntent = "intent"
	KindResult = "result"

	EventProjectInitialized = "project_initialized"
	EventRunStarted         = "run_started"
	EventStateChanged       = "state_changed"
	EventRunEnded           = "run_ended"
	EventWorkerInvoked      = "worker_invoked"
	EventWorkerFinished     = "worker_finished"
	EventWorkerRefused      = "worker_refused"
)

// State is the current-project snapshot. It is the source of truth for resume.
type State struct {
	SchemaVersion       int    `json:"schema_version"`
	ProjectID           string `json:"project_id"`
	ProductRoot         string `json:"product_root"`
	ProjectStatus       string `json:"project_status"`
	CurrentRunID        string `json:"current_run_id"`
	CurrentPhaseID      string `json:"current_phase_id"`
	CurrentState        string `json:"current_state"`
	CurrentCommit       string `json:"current_commit"`
	LastKnownGoodCommit string `json:"last_known_good_commit"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
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
