package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLockConflictAndStaleRecovery(t *testing.T) {
	dir := t.TempDir()
	alive := map[int]bool{111: true, 222: true}

	opts := func(pid int) Options {
		return Options{
			PID:   func() int { return pid },
			Alive: func(p int) bool { return alive[p] },
		}
	}

	a := mustOpen(t, dir, opts(111))
	b := mustOpen(t, dir, opts(222))

	ga, err := a.Lock()
	if err != nil {
		t.Fatal(err)
	}

	_, err = b.Lock()
	if err == nil {
		t.Fatal("expected lock conflict")
	}
	var le *LockError
	if !errors.As(err, &le) {
		t.Fatalf("err = %v, want LockError", err)
	}
	if le.Info.PID != 111 {
		t.Fatalf("lock pid = %d, want 111", le.Info.PID)
	}

	alive[111] = false
	gb, err := b.Lock()
	if err != nil {
		t.Fatalf("stale lock should be recoverable: %v", err)
	}
	if err := ga.Unlock(); err != nil {
		// A no longer owns the file; unlock should not delete B's lock blindly.
		// PID mismatch is expected after B replaced the stale lock.
		if errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	if err := gb.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestLockReleaseAllowsNextAcquire(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir, Options{PID: func() int { return 7 }, Alive: func(int) bool { return true }})
	g, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Unlock(); err != nil {
		t.Fatal(err)
	}
	g2, err := s.Lock()
	if err != nil {
		t.Fatal(err)
	}
	if err := g2.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyLockTreatedAsStale(t *testing.T) {
	dir := t.TempDir()
	s := mustOpen(t, dir, Options{PID: func() int { return 9 }, Alive: func(int) bool { return true }})
	proj := filepath.Join(dir, DirName)
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, LockFileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := s.Lock()
	if err != nil {
		t.Fatalf("empty lock should be stale: %v", err)
	}
	if err := g.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveBlockedWhenLockHeld(t *testing.T) {
	dir := t.TempDir()
	alive := map[int]bool{1: true, 2: true}
	a := mustOpen(t, dir, Options{PID: func() int { return 1 }, Alive: func(p int) bool { return alive[p] }})
	b := mustOpen(t, dir, Options{PID: func() int { return 2 }, Alive: func(p int) bool { return alive[p] }})
	if _, err := a.Init(); err != nil {
		t.Fatal(err)
	}
	g, err := a.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = g.Unlock() }()
	st, err := b.Load()
	if err != nil {
		t.Fatal(err)
	}
	st.CurrentRunID = "nope"
	if err := b.Save(st); err == nil {
		t.Fatal("save must fail while another process holds the lock")
	}
}
