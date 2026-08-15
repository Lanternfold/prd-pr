package studio

import (
	"os"
	"path/filepath"
	"strings"
)

// KnownCategories is the Studio layout observed on disk. Tests may inject a subset.
var KnownCategories = []string{
	"Archive", "Data", "Design", "Experiments", "Inbox", "Learning",
	"Products", "Resources", "Tools", "Writing",
}

const envStudio = "PRDPR_STUDIO"

// Layout is a discovered Studio root plus the category directories that exist.
type Layout struct {
	Root       string
	Categories map[string]string // name → absolute path
}

// Discover finds the Studio root without hardcoding a personal product path.
// Order: explicit root, PRDPR_STUDIO, walk from cwd, $HOME/Studio when it looks like Studio.
func Discover(explicit, cwd, home string) (Layout, error) {
	candidates := []string{}
	if strings.TrimSpace(explicit) != "" {
		candidates = append(candidates, explicit)
	}
	if v := strings.TrimSpace(os.Getenv(envStudio)); v != "" {
		candidates = append(candidates, v)
	}
	if cwd != "" {
		candidates = append(candidates, walkParents(cwd)...)
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, "Studio"))
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if seen[c] {
			continue
		}
		seen[c] = true
		if lay, ok := inspect(c); ok {
			return lay, nil
		}
	}
	return Layout{}, nil
}

func inspect(root string) (Layout, bool) {
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return Layout{}, false
	}
	cats := map[string]string{}
	for _, name := range KnownCategories {
		p := filepath.Join(root, name)
		st, err := os.Stat(p)
		if err == nil && st.IsDir() {
			cats[name] = p
		}
	}
	if len(cats) < 2 {
		return Layout{}, false
	}
	if _, tools := cats["Tools"]; !tools {
		if _, products := cats["Products"]; !products {
			return Layout{}, false
		}
	}
	return Layout{Root: root, Categories: cats}, true
}

func walkParents(start string) []string {
	var out []string
	p := filepath.Clean(start)
	for i := 0; i < 8; i++ {
		out = append(out, p)
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return out
}

func (l Layout) CategoryPath(name string) string {
	if l.Categories == nil {
		return ""
	}
	return l.Categories[name]
}

func (l Layout) Has(name string) bool {
	return l.CategoryPath(name) != ""
}

func (l Layout) Empty() bool {
	return strings.TrimSpace(l.Root) == "" || len(l.Categories) == 0
}
