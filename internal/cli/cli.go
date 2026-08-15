package cli

import (
	"fmt"
	"io"
	"strings"
)

// Version is the CLI version. Override at link time, for example:
//
//	go build -ldflags "-X github.com/lanternfold/prd-pr/internal/cli.Version=1.0.0"
var Version = "dev"

const (
	exitOK             = 0
	exitError          = 1
	exitUsage          = 2
	exitNotImplemented = 1
)

const usage = `PRD→PR is a local engineering orchestrator.

Usage:
  prdpr <command> [arguments]

Commands:
  version    Print the PRD→PR version
  help       Show this help
  doctor     Inspect the local environment
  init       Initialize .project/ in a product directory
  inspect    Parse and validate a PRD.md
  preflight  Report project and environment readiness
  prepare    Prepare a task packet without invoking a worker
  run        Run one coding-worker task against a product workspace
  verify     Independently verify a prepared implementation
  review     Diagnose failed verification
  repair     Prepare a bounded repair packet
  phase      Headless phase loop (worker, verify, review, repair)
  commit     Commit verified product files
  pr         Open a milestone GitHub PR when enabled
  checks     Inspect GitHub PR/CI checks
  merge      Evaluate auto-merge policy and merge if allowed
  feedback   Record one human response
  resume     Resume after human input
  status     Show persisted project state

Use "prdpr help" for this message. Use "prdpr version" to print the version.
`

// Main dispatches a CLI invocation. args[0] is the program name.
func Main(args []string, stdout, stderr io.Writer, rt Runtime) int {
	if len(args) < 2 {
		fmt.Fprint(stdout, usage)
		return exitOK
	}

	cmd := args[1]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "version", "--version":
		fmt.Fprintln(stdout, versionLine(rt))
		return exitOK
	case "doctor":
		return runDoctor(stdout, stderr, rt)
	case "init":
		return runInit(args[2:], stdout, stderr)
	case "inspect":
		return runInspect(args[2:], stdout, stderr)
	case "preflight":
		return runPreflight(args[2:], stdout, stderr, rt)
	case "prepare":
		return runPrepare(args[2:], stdout, stderr, rt)
	case "run":
		return runRun(args[2:], stdout, stderr, rt)
	case "verify":
		return runVerify(args[2:], stdout, stderr, rt)
	case "review":
		return runReview(args[2:], stdout, stderr, rt)
	case "repair":
		return runRepair(args[2:], stdout, stderr, rt)
	case "phase":
		return runPhase(args[2:], stdout, stderr, rt)
	case "commit":
		return runCommit(args[2:], stdout, stderr, rt)
	case "pr":
		return runPR(args[2:], stdout, stderr, rt)
	case "checks":
		return runChecks(args[2:], stdout, stderr, rt)
	case "merge":
		return runMerge(args[2:], stdout, stderr, rt)
	case "feedback":
		return runFeedback(args[2:], stdout, stderr, rt)
	case "resume":
		return runResume(args[2:], stdout, stderr, rt)
	case "status":
		return runStatus(args[2:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", cmd)
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
}

func versionLine(rt Runtime) string {
	v := strings.TrimSpace(rt.AppVersion)
	if v == "" {
		v = Version
	}
	return v
}

func notImplemented(stderr io.Writer, command string) int {
	fmt.Fprintf(stderr, "prdpr %s is not implemented in this version.\n", command)
	return exitNotImplemented
}
