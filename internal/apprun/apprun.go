package apprun

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/proc"
)

const (
	KindNone   = "none"
	KindGoTest = "go_test"
	KindGoRun  = "go_run"

	RelFile = ".project/runtime.json"

	DefaultTimeout = 20 * time.Second
	LogLimit       = 32 * 1024
)

// Def is a structured local runtime. It is not a raw PRD shell snippet.
type Def struct {
	Kind      string   `json:"kind"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	URL       string   `json:"url,omitempty"`
	KeepAlive bool     `json:"keep_alive,omitempty"`
	ReadyHint string   `json:"ready_hint,omitempty"`
}

// Report is one bounded start attempt.
type Report struct {
	Kind     string `json:"kind"`
	Command  string `json:"command,omitempty"`
	Ready    bool   `json:"ready"`
	Skipped  bool   `json:"skipped,omitempty"`
	URL      string `json:"url,omitempty"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Error    string `json:"error,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Starter starts a process according to Def. Tests inject fakes.
type Starter interface {
	Start(ctx context.Context, root string, def Def) Report
}

// ProcStarter uses the bounded subprocess runner. It never takes a raw PRD command string.
type ProcStarter struct {
	Runner  *proc.Runner
	Timeout time.Duration
}

func (s ProcStarter) Start(ctx context.Context, root string, def Def) Report {
	rep := Report{Kind: def.Kind, Command: def.Command, URL: def.URL}
	if def.Kind == "" || def.Kind == KindNone || def.Command == "" {
		rep.Skipped = true
		rep.Reason = "no local application runtime for this project type"
		return rep
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	r := s.Runner
	if r == nil {
		r = &proc.Runner{}
	}
	res := r.Run(ctx, proc.Spec{Name: def.Command, Args: def.Args, Dir: root})
	rep.Stdout = clip(string(res.Stdout))
	rep.Stderr = clip(string(res.Stderr))
	rep.ExitCode = res.ExitCode
	rep.TimedOut = res.TimedOut
	if res.Err != nil {
		rep.Error = res.Err.Error()
	}
	switch def.Kind {
	case KindGoTest:
		rep.Ready = res.ExitCode == 0 && !res.TimedOut && res.Err == nil
	case KindGoRun:
		if def.KeepAlive {
			rep.Ready = res.TimedOut || (res.Err == nil && looksReady(rep.Stdout+rep.Stderr, def.ReadyHint, def.URL))
			if res.TimedOut {
				rep.Error = ""
			}
		} else {
			rep.Ready = res.ExitCode == 0 && !res.TimedOut && res.Err == nil
		}
	default:
		rep.Skipped = true
		rep.Reason = "unsupported runtime kind"
	}
	if !rep.Ready && !rep.Skipped && rep.Error == "" {
		rep.Error = "application did not become ready"
	}
	return rep
}

func looksReady(logs, hint, url string) bool {
	low := strings.ToLower(logs)
	if strings.Contains(low, "listening") || strings.Contains(low, "ready") {
		return true
	}
	if hint != "" && strings.Contains(low, strings.ToLower(hint)) {
		return true
	}
	if url != "" && strings.Contains(low, strings.ToLower(url)) {
		return true
	}
	return false
}

func clip(s string) string {
	if len(s) <= LogLimit {
		return s
	}
	return s[:LogLimit]
}

func ForType(projectType string) Def {
	switch projectType {
	case "go_cli":
		return Def{Kind: KindGoRun, Command: "go", Args: []string{"run", "."}, KeepAlive: false}
	case "go_library":
		return Def{Kind: KindNone}
	case "web":
		return Def{Kind: KindGoRun, Command: "go", Args: []string{"run", "."}, KeepAlive: true, URL: "http://127.0.0.1:8080", ReadyHint: "listening"}
	default:
		return Def{Kind: KindNone}
	}
}

func Load(root string) Def {
	raw, err := os.ReadFile(filepath.Join(root, RelFile))
	if err != nil {
		return Def{}
	}
	var d Def
	if json.Unmarshal(raw, &d) != nil {
		return Def{}
	}
	return d
}

func Save(root string, d Def) error {
	if err := os.MkdirAll(filepath.Join(root, ".project"), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, RelFile), append(raw, '\n'), 0o644)
}

type Fake struct {
	Ready   bool
	Skipped bool
	Error   string
	Stdout  string
	URL     string
	Calls   int
}

func (f *Fake) Start(context.Context, string, Def) Report {
	f.Calls++
	return Report{Ready: f.Ready, Skipped: f.Skipped, Error: f.Error, Stdout: f.Stdout, URL: f.URL, Kind: "fake"}
}
