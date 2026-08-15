package bootstrap

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/prd"
)

var adoptNames = map[string]bool{
	"prd.md": true, "readme.md": true, "go.mod": true, "go.sum": true,
	".gitignore": true, ".git": true, ".project": true, ".cursor": true,
	".ds_store": true,
}

// PlaceResult is one idempotent placement of a PRD into a Studio project directory.
type PlaceResult struct {
	Root     string
	Created  bool
	Reused   bool
	Conflict bool
	Reason   string
	Copied   bool
}

// Place copies the PRD into dest and creates the minimum type scaffolding.
// It does not overwrite an existing project, Git history, or user files.
func Place(sourcePRD, dest string, sel Selection, doc *prd.Document) (PlaceResult, error) {
	sourcePRD, err := filepath.Abs(sourcePRD)
	if err != nil {
		return PlaceResult{}, err
	}
	dest, err = filepath.Abs(dest)
	if err != nil {
		return PlaceResult{}, err
	}
	out := PlaceResult{Root: dest}

	fi, err := os.Stat(dest)
	if err != nil {
		if !os.IsNotExist(err) {
			return out, err
		}
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return out, err
		}
		out.Created = true
	} else if !fi.IsDir() {
		out.Conflict = true
		out.Reason = "destination exists and is not a directory"
		return out, nil
	} else {
		conflict, reason := conflictingContents(dest, sourcePRD)
		if conflict {
			out.Conflict = true
			out.Reason = reason
			return out, nil
		}
		out.Reused = true
	}

	destPRD := filepath.Join(dest, "PRD.md")
	srcBytes, err := os.ReadFile(sourcePRD)
	if err != nil {
		return out, err
	}
	existing, err := os.ReadFile(destPRD)
	switch {
	case err == nil:
		if !bytes.Equal(existing, srcBytes) && canonicalPath(sourcePRD) != canonicalPath(destPRD) {
			out.Conflict = true
			out.Reason = "destination already contains a different PRD.md; refusing to overwrite"
			return out, nil
		}
	case os.IsNotExist(err):
		if err := os.WriteFile(destPRD, srcBytes, 0o644); err != nil {
			return out, err
		}
		out.Copied = true
	default:
		return out, err
	}

	if err := writeMinimumStructure(dest, sel, doc); err != nil {
		return out, err
	}
	return out, nil
}

func conflictingContents(dest, sourcePRD string) (bool, string) {
	entries, err := os.ReadDir(dest)
	if err != nil {
		return true, err.Error()
	}
	if len(entries) == 0 {
		return false, ""
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if adoptNames[name] {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		return true, fmt.Sprintf("destination contains unrelated material (%s); refusing to adopt it", e.Name())
	}
	destPRD := filepath.Join(dest, "PRD.md")
	if _, err := os.Stat(destPRD); err == nil {
		src, err1 := os.ReadFile(sourcePRD)
		dst, err2 := os.ReadFile(destPRD)
		if err1 == nil && err2 == nil && !bytes.Equal(src, dst) && canonicalPath(sourcePRD) != canonicalPath(destPRD) {
			return true, "destination already contains a different PRD.md; refusing to overwrite"
		}
	}
	return false, ""
}

func writeMinimumStructure(root string, sel Selection, doc *prd.Document) error {
	readme := filepath.Join(root, "README.md")
	if _, err := os.Stat(readme); os.IsNotExist(err) {
		title := sel.Slug
		if doc != nil && doc.Metadata.Product != "" {
			title = doc.Metadata.Product
		}
		body := "# " + title + "\n\nBootstrapped by PRD→PR. Implementation follows PRD.md.\n"
		if err := os.WriteFile(readme, []byte(body), 0o644); err != nil {
			return err
		}
	}
	switch sel.Type {
	case TypeGoLibrary, TypeGoCLI:
		mod := filepath.Join(root, "go.mod")
		if _, err := os.Stat(mod); os.IsNotExist(err) {
			module := "example.com/" + sel.Slug
			if doc != nil && strings.Contains(doc.Metadata.Repository, "/") {
				module = "github.com/" + strings.TrimSuffix(strings.TrimPrefix(doc.Metadata.Repository, "github.com/"), ".git")
			}
			body := "module " + module + "\n\ngo 1.22\n"
			if err := os.WriteFile(mod, []byte(body), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func canonicalPath(p string) string {
	p = filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
