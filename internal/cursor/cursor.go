package cursor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/proc"
	"github.com/lanternfold/prd-pr/internal/redact"
)

// PinnedMechanism is the non-interactive Cursor Agent CLI contract for P4 (OAQ-8).
// The editor `cursor` binary (observed 3.11.13) is not this worker.
const PinnedMechanism = "cursor-agent --print --force --trust --workspace <product-root> --output-format text"

const (
	DefaultTimeout = 15 * time.Minute
	binCursorAgent = "cursor-agent"
	binAgent       = "agent"
)

// Result is the worker outcome. verified_success is always false in P4.
type Result struct {
	Invoked              bool          `json:"invoked"`
	RefusalReason        string        `json:"refusal_reason,omitempty"`
	ExitCode             int           `json:"exit_code"`
	Duration             time.Duration `json:"duration"`
	Stdout               string        `json:"-"`
	Stderr               string        `json:"-"`
	Transcript           string        `json:"-"`
	TimedOut             bool          `json:"timed_out"`
	WorkerClaimedSuccess bool          `json:"worker_claimed_success"`
	ClaimedDone          bool          `json:"claimed_done"`
	VerifiedSuccess      bool          `json:"verified_success"`
	Binary               string        `json:"binary,omitempty"`
	Args                 []string      `json:"args,omitempty"`
	CLIMechanism         string        `json:"cli_mechanism"`
}

// Worker runs a coding task. Implementations must not set VerifiedSuccess.
type Worker interface {
	Run(ctx context.Context, req Request) Result
}

type Request struct {
	ProductRoot string
	Packet      packet.Packet
	PacketRel   string
	Timeout     time.Duration
}

type LookPathFunc func(file string) (string, error)

// CLI invokes the Cursor Agent CLI. Tests inject LookPath, Runner, and Clock.
type CLI struct {
	LookPath LookPathFunc
	Runner   *proc.Runner
	Now      func() time.Time
	Bin      string
}

func (c *CLI) now() time.Time {
	if c != nil && c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *CLI) lookPath() LookPathFunc {
	if c != nil && c.LookPath != nil {
		return c.LookPath
	}
	return exec.LookPath
}

func (c *CLI) runner() *proc.Runner {
	if c != nil && c.Runner != nil {
		return c.Runner
	}
	return &proc.Runner{}
}

// ResolveBinary finds cursor-agent or agent. The editor `cursor` binary is rejected.
func (c *CLI) Ready() error {
	_, err := c.ResolveBinary()
	return err
}

func (c *CLI) ResolveBinary() (string, error) {
	if c != nil && strings.TrimSpace(c.Bin) != "" {
		return c.Bin, nil
	}
	if env := strings.TrimSpace(os.Getenv("CURSOR_AGENT_BIN")); env != "" {
		return env, nil
	}
	look := c.lookPath()
	for _, name := range []string{binCursorAgent, binAgent} {
		path, err := look(name)
		if err == nil && path != "" {
			return path, nil
		}
	}
	if _, err := look("cursor"); err == nil {
		return "", fmt.Errorf("found editor cursor binary, not Cursor Agent CLI (%s)", PinnedMechanism)
	}
	return "", fmt.Errorf("Cursor Agent CLI not found; expected %s", PinnedMechanism)
}

func (c *CLI) Run(ctx context.Context, req Request) Result {
	start := c.now()
	res := Result{CLIMechanism: PinnedMechanism, VerifiedSuccess: false}
	bin, err := c.ResolveBinary()
	if err != nil {
		res.RefusalReason = err.Error()
		res.Duration = c.now().Sub(start)
		return res
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"--print",
		"--force",
		"--trust",
		"--workspace", req.ProductRoot,
		"--output-format", "text",
		packet.Prompt(req.PacketRel),
	}
	res.Invoked = true
	res.Binary = bin
	res.Args = append([]string{}, args...)
	procRes := c.runner().Run(runCtx, proc.Spec{
		Name: bin,
		Args: args,
		Dir:  req.ProductRoot,
	})
	res.Duration = c.now().Sub(start)
	res.Stdout = redact.String(string(procRes.Stdout))
	res.Stderr = redact.String(string(procRes.Stderr))
	res.Transcript = strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	res.ExitCode = procRes.ExitCode
	res.TimedOut = procRes.TimedOut
	if procRes.TimedOut {
		res.WorkerClaimedSuccess = false
		res.ClaimedDone = false
		return res
	}
	claimed := procRes.Err == nil && procRes.ExitCode == 0
	res.WorkerClaimedSuccess = claimed
	res.ClaimedDone = claimed
	res.VerifiedSuccess = false
	return res
}

// Fake is a test worker. It never talks to Cursor. VerifiedSuccess stays false.
type Fake struct {
	ClaimSuccess bool
	WriteRel     string
	WriteBody    string
	Hang         time.Duration
	Now          func() time.Time
}

func (f Fake) Run(ctx context.Context, req Request) Result {
	start := time.Now()
	if f.Now != nil {
		start = f.Now()
	}
	res := Result{Invoked: true, CLIMechanism: "fake", VerifiedSuccess: false}
	if f.Hang > 0 {
		timer := time.NewTimer(f.Hang)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			res.TimedOut = true
			res.ExitCode = -1
			res.Duration = time.Since(start)
			return res
		case <-timer.C:
		}
	}
	if f.WriteRel != "" {
		path := filepath.Join(req.ProductRoot, filepath.FromSlash(f.WriteRel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			res.ExitCode = 1
			res.Stderr = err.Error()
			res.Duration = time.Since(start)
			return res
		}
		if err := os.WriteFile(path, []byte(f.WriteBody), 0o644); err != nil {
			res.ExitCode = 1
			res.Stderr = err.Error()
			res.Duration = time.Since(start)
			return res
		}
	}
	if f.ClaimSuccess {
		res.ExitCode = 0
		res.WorkerClaimedSuccess = true
		res.ClaimedDone = true
		res.Stdout = "Done."
		res.Transcript = "Done."
	} else {
		res.ExitCode = 1
		res.Stdout = "failed"
		res.Transcript = "failed"
	}
	res.VerifiedSuccess = false
	res.Duration = time.Since(start)
	return res
}

// Sequence runs Fake steps in order. Tests use it for implement-then-repair.
type Sequence struct {
	Steps []Fake
	i     int
}

func (s *Sequence) Run(ctx context.Context, req Request) Result {
	if s == nil || s.i >= len(s.Steps) {
		return Result{Invoked: false, RefusalReason: "no remaining fake worker steps", VerifiedSuccess: false}
	}
	step := s.Steps[s.i]
	s.i++
	return step.Run(ctx, req)
}
