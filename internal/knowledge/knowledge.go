package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const SchemaVersion = 1

const (
	ScopeProject = "PROJECT"
	ScopeGlobal  = "GLOBAL"
)

// Entry is versioned reusable knowledge. It is not a prompt rewrite.
type Entry struct {
	ID            string    `json:"id"`
	Category      string    `json:"category"`
	Scope         string    `json:"scope"`
	Observation   string    `json:"observation"`
	Evidence      string    `json:"evidence,omitempty"`
	Confidence    float64   `json:"confidence"`
	Source        string    `json:"source"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
	Applicability string    `json:"applicability,omitempty"`
	Successes     int       `json:"successes"`
	Failures      int       `json:"failures"`
}

type Store struct {
	Dir string
	Now func() time.Time
}

func ProjectStore(productRoot string) *Store {
	return &Store{Dir: filepath.Join(productRoot, ".project", "knowledge")}
}

func (s *Store) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Store) Put(e Entry) (Entry, error) {
	if e.Scope != ScopeProject && e.Scope != ScopeGlobal {
		return Entry{}, fmt.Errorf("knowledge scope must be PROJECT or GLOBAL")
	}
	if strings.TrimSpace(e.Observation) == "" {
		return Entry{}, fmt.Errorf("knowledge observation is required")
	}
	if strings.TrimSpace(e.Category) == "" {
		return Entry{}, fmt.Errorf("knowledge category is required")
	}
	ts := s.now().UTC().Format(time.RFC3339Nano)
	if e.ID == "" {
		e.ID = fmt.Sprintf("k_%d", s.now().UnixNano())
	}
	if e.CreatedAt == "" {
		e.CreatedAt = ts
	}
	e.UpdatedAt = ts
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return Entry{}, err
	}
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return Entry{}, err
	}
	path := filepath.Join(s.Dir, e.ID+".json")
	return e, os.WriteFile(path, append(data, '\n'), 0o644)
}

func (s *Store) List() ([]Entry, error) {
	ents, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var item Entry
		if json.Unmarshal(raw, &item) == nil {
			out = append(out, item)
		}
	}
	return out, nil
}

// Hints returns observations for a category. Project stores never include other projects.
func (s *Store) Hints(category string) []string {
	list, err := s.List()
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range list {
		if category != "" && e.Category != category {
			continue
		}
		out = append(out, e.Observation)
	}
	return out
}
