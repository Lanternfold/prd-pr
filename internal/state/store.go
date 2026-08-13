package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

var (
	// ErrNotInitialized means .project/state.json is missing.
	ErrNotInitialized = errors.New("project is not initialized")
)

// InitResult reports whether Init created a new snapshot.
type InitResult struct {
	Created bool
	State   State
}

// Init creates .project/state.json and events.jsonl, or leaves a valid project unchanged.
func (s *Store) Init() (InitResult, error) {
	g, err := s.Lock()
	if err != nil {
		return InitResult{}, err
	}
	defer func() { _ = g.Unlock() }()

	existing, err := s.loadLocked()
	if err == nil {
		if err := validateState(existing); err != nil {
			return InitResult{}, err
		}
		if err := s.ensureEventsFile(); err != nil {
			return InitResult{}, err
		}
		return InitResult{Created: false, State: existing}, nil
	}
	if !errors.Is(err, ErrNotInitialized) {
		return InitResult{}, err
	}

	now := s.timestamp()
	st := State{
		SchemaVersion: SchemaVersion,
		ProjectID:     s.newID(),
		ProductRoot:   s.jail.Root(),
		ProjectStatus: StatusCreated,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.appendEventLocked(Event{
		Kind:    KindIntent,
		Name:    EventProjectInitialized,
		Payload: payloadJSON(map[string]string{"product_root": st.ProductRoot}),
	}); err != nil {
		return InitResult{}, err
	}
	if err := s.writeStateLocked(st); err != nil {
		return InitResult{}, err
	}
	if err := s.appendEventLocked(Event{
		Kind:    KindResult,
		Name:    EventProjectInitialized,
		Payload: payloadJSON(map[string]string{"project_id": st.ProjectID, "status": "created"}),
	}); err != nil {
		return InitResult{}, err
	}
	return InitResult{Created: true, State: st}, nil
}

// Load returns the current snapshot. It does not take the run lock.
func (s *Store) Load() (State, error) {
	return s.loadLocked()
}

// Save replaces the snapshot atomically and records intent/result events.
func (s *Store) Save(st State) error {
	g, err := s.Lock()
	if err != nil {
		return err
	}
	defer func() { _ = g.Unlock() }()
	return s.saveLocked(st)
}

// AppendEvent appends one journal line under the project lock.
func (s *Store) AppendEvent(ev Event) error {
	g, err := s.Lock()
	if err != nil {
		return err
	}
	defer func() { _ = g.Unlock() }()
	return s.appendEventLocked(ev)
}

func (s *Store) saveLocked(st State) error {
	st.UpdatedAt = s.timestamp()
	if st.SchemaVersion == 0 {
		st.SchemaVersion = SchemaVersion
	}
	if err := validateState(st); err != nil {
		return err
	}
	if err := s.appendEventLocked(Event{
		Kind:    KindIntent,
		Name:    EventStateChanged,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: payloadJSON(map[string]string{
			"project_status": st.ProjectStatus,
			"current_state":  st.CurrentState,
		}),
	}); err != nil {
		return err
	}
	if err := s.writeStateLocked(st); err != nil {
		return err
	}
	return s.appendEventLocked(Event{
		Kind:    KindResult,
		Name:    EventStateChanged,
		RunID:   st.CurrentRunID,
		PhaseID: st.CurrentPhaseID,
		Payload: payloadJSON(map[string]string{
			"project_status": st.ProjectStatus,
			"current_state":  st.CurrentState,
		}),
	})
}

func (s *Store) writeStateLocked(st State) error {
	path, err := s.statePath()
	if err != nil {
		return err
	}
	data, err := marshalState(st)
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	return s.writeAtomic(path, data)
}

func (s *Store) loadLocked() (State, error) {
	path, err := s.statePath()
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, ErrNotInitialized
		}
		return State{}, fmt.Errorf("read project state: %w", err)
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("corrupt project state in %s: %w; not overwriting. Inspect the file or restore a known-good copy", path, err)
	}
	if err := validateState(st); err != nil {
		return State{}, err
	}
	return st, nil
}

func (s *Store) ensureProjectDir() error {
	dir, err := s.projectDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", DirName, err)
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", dir)
	}
	return nil
}

func (s *Store) ensureEventsFile() error {
	path, err := s.eventsPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create event log: %w", err)
	}
	return f.Close()
}

func validateState(st State) error {
	if st.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported state schema_version %d (this binary supports %d); not overwriting", st.SchemaVersion, SchemaVersion)
	}
	if st.ProjectID == "" {
		return fmt.Errorf("corrupt project state: missing project_id; not overwriting")
	}
	if st.ProductRoot == "" {
		return fmt.Errorf("corrupt project state: missing product_root; not overwriting")
	}
	if st.ProjectStatus == "" {
		return fmt.Errorf("corrupt project state: missing project_status; not overwriting")
	}
	return nil
}
