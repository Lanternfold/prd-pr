package testeng

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/redact"
)

func boundLog(b []byte) string {
	s := redact.String(string(b))
	if len(s) <= MaxLogBytes {
		return s
	}
	return s[:MaxLogBytes] + "\n…[truncated]\n"
}

func goSymbolPresent(root, name string, jail *fsguard.Jail) (bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, nil
	}
	found := false
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == ".project" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			// Allow _test.go too for symbol presence? Implementation should be in non-test files.
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
		}
		if jail != nil && !jail.Contains(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte("func "+name+"(")) || bytes.Contains(data, []byte("func "+name+" (")) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}

func fileExistsInJail(jail *fsguard.Jail, rel string) (bool, error) {
	path, err := jail.Resolve(rel)
	if err != nil {
		return false, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !fi.IsDir(), nil
}
