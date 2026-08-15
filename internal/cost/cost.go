package cost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const LedgerRel = ".project/cost.jsonl"

// Line is one model-usage record. Missing cost must not block execution.
type Line struct {
	Timestamp    string  `json:"timestamp"`
	Provider     string  `json:"provider,omitempty"`
	Model        string  `json:"model,omitempty"`
	Purpose      string  `json:"purpose,omitempty"`
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	EstimatedUSD float64 `json:"estimated_usd,omitempty"`
	DurationMS   int64   `json:"duration_ms,omitempty"`
	RunID        string  `json:"run_id,omitempty"`
	PhaseID      string  `json:"phase_id,omitempty"`
}

func Append(productRoot string, line Line) error {
	if line.Timestamp == "" {
		line.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	path := filepath.Join(productRoot, filepath.FromSlash(LedgerRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(line)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(b, '\n'))
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

func SpentUSD(productRoot string) float64 {
	path := filepath.Join(productRoot, filepath.FromSlash(LedgerRel))
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var total float64
	for _, line := range split(data) {
		var l Line
		if json.Unmarshal(line, &l) == nil {
			total += l.EstimatedUSD
		}
	}
	return total
}

func split(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			if i > start {
				out = append(out, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}
