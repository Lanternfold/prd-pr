package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/lanternfold/prd-pr/internal/fsguard"
)

// LockInfo identifies the process holding the project lock.
type LockInfo struct {
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
	Hostname  string `json:"hostname,omitempty"`
}

// LockError is returned when an active process already holds the lock.
type LockError struct {
	Info LockInfo
}

func (e *LockError) Error() string {
	msg := fmt.Sprintf("project is locked by an active process (pid %d", e.Info.PID)
	if e.Info.StartedAt != "" {
		msg += ", started " + e.Info.StartedAt
	}
	if e.Info.Hostname != "" {
		msg += ", host " + e.Info.Hostname
	}
	msg += "). Wait for that process to finish, then retry."
	return msg
}

// Guard is a held project lock. Release it with Unlock.
type Guard struct {
	s    *Store
	info LockInfo
	path string
}

// Lock acquires an exclusive project lock, recovering a stale PID if needed.
func (s *Store) Lock() (*Guard, error) {
	if err := s.ensureProjectDir(); err != nil {
		return nil, err
	}
	path, err := s.lockPath()
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		g, err := s.tryCreateLock(path)
		if err == nil {
			return g, nil
		}
		if !errors.Is(err, os.ErrExist) && !os.IsExist(err) {
			return nil, err
		}
		stale, info, rerr := s.inspectLock(path)
		if rerr != nil {
			return nil, rerr
		}
		if !stale {
			return nil, &LockError{Info: info}
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove stale project lock: %w", err)
		}
	}
	return nil, fmt.Errorf("could not acquire project lock at %s", path)
}

func (s *Store) tryCreateLock(path string) (*Guard, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	info := LockInfo{
		PID:       s.pid(),
		StartedAt: s.timestamp(),
		Hostname:  s.hostname(),
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	data = append(data, '\n')
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if werr != nil || serr != nil || cerr != nil {
		_ = os.Remove(path)
		if werr != nil {
			return nil, fmt.Errorf("write project lock: %w", werr)
		}
		if serr != nil {
			return nil, fmt.Errorf("flush project lock: %w", serr)
		}
		return nil, fmt.Errorf("close project lock: %w", cerr)
	}
	return &Guard{s: s, info: info, path: path}, nil
}

func (s *Store) inspectLock(path string) (stale bool, info LockInfo, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, LockInfo{}, nil
		}
		return false, LockInfo{}, fmt.Errorf("read project lock: %w", err)
	}
	if len(data) == 0 {
		return true, LockInfo{}, nil
	}
	if err := json.Unmarshal(data, &info); err != nil || info.PID <= 0 {
		return true, LockInfo{}, nil
	}
	if !s.alive(info.PID) {
		return true, info, nil
	}
	return false, info, nil
}

// Jail returns the store jail. The caller must hold this guard for writes.
func (g *Guard) Jail() *fsguard.Jail {
	if g == nil || g.s == nil {
		return nil
	}
	return g.s.jail
}

// Load returns the current snapshot while the lock is held.
func (g *Guard) Load() (State, error) {
	if g == nil || g.s == nil {
		return State{}, fmt.Errorf("project lock is not held")
	}
	return g.s.loadLocked()
}

// Save replaces the snapshot while the lock is held.
func (g *Guard) Save(st State) error {
	if g == nil || g.s == nil {
		return fmt.Errorf("project lock is not held")
	}
	return g.s.saveLocked(st)
}

// AppendEvent appends a journal line while the lock is held.
func (g *Guard) AppendEvent(ev Event) error {
	if g == nil || g.s == nil {
		return fmt.Errorf("project lock is not held")
	}
	return g.s.appendEventLocked(ev)
}

// WriteFile atomically writes a path relative to the product root.
func (g *Guard) WriteFile(rel string, data []byte) error {
	if g == nil || g.s == nil {
		return fmt.Errorf("project lock is not held")
	}
	path, err := g.s.resolve(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(rel), err)
	}
	return g.s.writeAtomic(path, data)
}

// Unlock releases the lock if this guard still owns it.
func (g *Guard) Unlock() error {
	if g == nil || g.s == nil {
		return nil
	}
	data, err := os.ReadFile(g.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read project lock: %w", err)
	}
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("project lock is corrupt at %s; inspect it before deleting", g.path)
	}
	if info.PID != g.info.PID {
		return fmt.Errorf("project lock is held by pid %d, not this process (%d); not releasing", info.PID, g.info.PID)
	}
	if err := os.Remove(g.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("release project lock: %w", err)
	}
	return nil
}

func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrProcessDone) {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if errno == syscall.ESRCH {
			return false
		}
		if errno == syscall.EPERM {
			return true
		}
	}
	return true
}
