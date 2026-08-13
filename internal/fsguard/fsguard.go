package fsguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Jail confines orchestrator filesystem operations to a product root.
type Jail struct {
	root string
}

// New validates productRoot as an existing directory and returns a jail rooted there.
func New(productRoot string) (*Jail, error) {
	if strings.TrimSpace(productRoot) == "" {
		return nil, fmt.Errorf("product root is empty")
	}
	abs, err := filepath.Abs(productRoot)
	if err != nil {
		return nil, fmt.Errorf("product root %q is invalid: %w", productRoot, err)
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("product root %q is invalid: %w", abs, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("product root %q is not a directory", abs)
	}
	return &Jail{root: abs}, nil
}

// Root returns the cleaned absolute product root.
func (j *Jail) Root() string {
	return j.root
}

// Resolve maps a relative (or in-root absolute) path to an absolute path inside the jail.
func (j *Jail) Resolve(p string) (string, error) {
	if j == nil || j.root == "" {
		return "", fmt.Errorf("filesystem jail is not configured")
	}
	if strings.TrimSpace(p) == "" {
		return "", fmt.Errorf("path is empty")
	}
	var candidate string
	if filepath.IsAbs(p) {
		candidate = filepath.Clean(p)
	} else {
		candidate = filepath.Clean(filepath.Join(j.root, p))
	}
	if !j.Contains(candidate) {
		return "", fmt.Errorf("path %q is outside product root %q", p, j.root)
	}
	return candidate, nil
}

// Contains reports whether abs is the product root or a path inside it.
func (j *Jail) Contains(abs string) bool {
	if j == nil {
		return false
	}
	abs = filepath.Clean(abs)
	if abs == j.root {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(abs, j.root+sep)
}
