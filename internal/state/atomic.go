package state

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *Store) writeAtomic(absPath string, data []byte) error {
	if !s.jail.Contains(absPath) {
		return fmt.Errorf("refusing to write outside product root: %s", absPath)
	}
	dir := filepath.Dir(absPath)
	f, err := os.CreateTemp(dir, "."+filepath.Base(absPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary state file: %w", err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temporary state file: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("flush temporary state file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temporary state file: %w", err)
	}
	if err := s.rename(tmp, absPath); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	ok = true
	return nil
}
