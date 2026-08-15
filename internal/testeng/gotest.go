package testeng

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/proc"
)

func (e *Engine) runGoTest(ctx context.Context, jail *fsguard.Jail) Check {
	c := Check{
		ID:       "go.test",
		Kind:     KindGoTest,
		Name:     "go test ./...",
		Required: true,
		Command:  Command{Program: "go", Args: []string{"test", "./..."}},
	}
	bin, err := e.opts.lookPath()("go")
	if err != nil || bin == "" {
		c.Outcome = OutcomeError
		c.Detail = "go binary not found on PATH"
		c.ExitCode = -1
		return c
	}
	start := e.opts.now()
	timeout := e.opts.timeout()
	runCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	res := e.opts.runner().Run(runCtx, proc.Spec{
		Name: bin,
		Args: []string{"test", "./..."},
		Dir:  jail.Root(),
	})
	c.DurationMS = e.opts.now().Sub(start).Milliseconds()
	if c.DurationMS == 0 {
		c.DurationMS = time.Since(start).Milliseconds()
	}
	c.Stdout = boundLog(res.Stdout)
	c.Stderr = boundLog(res.Stderr)
	c.ExitCode = res.ExitCode
	if res.TimedOut {
		c.Outcome = OutcomeTimeout
		c.Detail = "test timed out"
		c.ExitCode = -1
		return c
	}
	if res.Err != nil && res.ExitCode == -1 {
		c.Outcome = OutcomeError
		c.Detail = res.Err.Error()
		return c
	}
	if res.ExitCode == 0 {
		c.Outcome = OutcomePass
		c.Detail = summarizeGoTest(c.Stdout, true)
		return c
	}
	c.Outcome = OutcomeFail
	c.Detail = summarizeGoTest(c.Stdout+"\n"+c.Stderr, false)
	return c
}

func summarizeGoTest(out string, ok bool) string {
	var pkgs []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "FAIL") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				pkgs = append(pkgs, fields[0]+" "+fields[1])
			}
		}
	}
	if len(pkgs) == 0 {
		if ok {
			return "go test ./... passed"
		}
		return "go test ./... failed"
	}
	msg := strings.Join(pkgs, "; ")
	if len(msg) > 400 {
		return msg[:400] + "…"
	}
	return msg
}

func commandSafe(jail *fsguard.Jail, cmd Command) error {
	if cmd.Program == "" {
		return fmt.Errorf("empty program")
	}
	if filepath.IsAbs(cmd.Program) && jail != nil && !jail.Contains(cmd.Program) {
		// Interpreters like /usr/bin/go live outside the product root; that is allowed.
		// Product-relative binaries must stay in jail. Absolute system bins are OK if they are not a path inside another workspace.
	}
	for _, a := range cmd.Args {
		if strings.HasPrefix(a, "-C") && len(a) > 2 {
			return fmt.Errorf("workspace escape: %s", a)
		}
		if a == "-C" {
			return fmt.Errorf("workspace escape: -C")
		}
		if filepath.IsAbs(a) && jail != nil && !jail.Contains(a) {
			return fmt.Errorf("path %q is outside product root", a)
		}
	}
	return nil
}
