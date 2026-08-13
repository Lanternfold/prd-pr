package proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// Spec is one subprocess invocation.
type Spec struct {
	Name string
	Args []string
	Dir  string
	Env  []string
}

// Result is the captured subprocess outcome.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	TimedOut bool
	Err      error
}

// Runner starts processes in their own group so a timeout can kill children.
type Runner struct {
	Start func(cmd *exec.Cmd) error
	Wait  func(cmd *exec.Cmd) error
	Kill  func(pid int) error
}

func (r *Runner) startFn() func(*exec.Cmd) error {
	if r != nil && r.Start != nil {
		return r.Start
	}
	return func(cmd *exec.Cmd) error { return cmd.Start() }
}

func (r *Runner) waitFn() func(*exec.Cmd) error {
	if r != nil && r.Wait != nil {
		return r.Wait
	}
	return func(cmd *exec.Cmd) error { return cmd.Wait() }
}

func (r *Runner) killFn() func(int) error {
	if r != nil && r.Kill != nil {
		return r.Kill
	}
	return killProcessGroup
}

// Run executes spec until completion, context cancel, or timeout.
func (r *Runner) Run(ctx context.Context, spec Spec) Result {
	if spec.Name == "" {
		return Result{ExitCode: -1, Err: fmt.Errorf("process name is empty")}
	}
	cmd := exec.Command(spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	if len(spec.Env) > 0 {
		cmd.Env = spec.Env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	setProcessGroup(cmd)

	if err := r.startFn()(cmd); err != nil {
		return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: -1, Err: err}
	}

	done := make(chan error, 1)
	go func() {
		done <- r.waitFn()(cmd)
	}()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = r.killFn()(cmd.Process.Pid)
		}
		select {
		case waitErr = <-done:
		case <-time.After(2 * time.Second):
			waitErr = ctx.Err()
		}
		res := Result{
			Stdout:   stdout.Bytes(),
			Stderr:   stderr.Bytes(),
			ExitCode: -1,
			TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled),
			Err:      ctx.Err(),
		}
		if waitErr != nil && res.Err == nil {
			res.Err = waitErr
		}
		return res
	}

	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: 0, Err: waitErr}
	if waitErr == nil {
		return res
	}
	var ee *exec.ExitError
	if errors.As(waitErr, &ee) {
		res.ExitCode = ee.ExitCode()
		res.Err = nil
		return res
	}
	res.ExitCode = -1
	return res
}

func killProcessGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(-pid, syscall.SIGKILL)
}
