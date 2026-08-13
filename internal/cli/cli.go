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
  run        Run one coding-worker task against a product workspace
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
	case "run":
		return runRun(args[2:], stdout, stderr, rt)
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
