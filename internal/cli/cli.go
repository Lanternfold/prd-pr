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

// DocsURL is the published documentation root for this repository.
const DocsURL = "https://github.com/lanternfold/prd-pr/blob/main/docs"

const usage = `PRD→PR is a local engineering orchestrator.

Usage:
  prdpr <command> [arguments]
  prdpr <path/to/PRD.md>

The intended entry point is a PRD path. PRD→PR validates the PRD, places a
Studio project when needed, then prepares the next READY phase.

Commands:
  version      Print the PRD→PR version
  help         Show this help
  doctor       Inspect the local environment
  init         Initialize .project/ in a product directory
  inspect      Parse and report a PRD.md
  validate-prd Contract-validate a PRD before any project mutation
  bootstrap    Place a PRD into a Studio project and prepare (PRD path only)
  preflight    Report project and environment readiness
  prepare      Prepare a task packet without invoking a worker
  run          Run one coding-worker task against a product workspace
  verify       Independently verify a prepared implementation
  review       Diagnose failed verification
  repair       Prepare a bounded repair packet
  phase        Walk READY phases (worker, verify, review, repair)
  commit       Commit verified product files
  pr           Open a milestone GitHub PR when enabled
  checks       Inspect GitHub PR/CI checks
  merge        Evaluate auto-merge policy and merge if allowed
  runtime      Start local application runtime validation
  feedback     Record one human response
  resume       Resume after human input
  status       Show persisted project state

Documentation:
  User guide:     ` + DocsURL + `/USER_GUIDE.md
  CLI reference:  ` + DocsURL + `/CLI.md
  Architecture:   ` + DocsURL + `/FLOW.md

Use "prdpr help" for this message. Use "prdpr <command> --help" for command usage.
Use "prdpr version" to print the version.
`

func docsHint(anchor string) string {
	if strings.TrimSpace(anchor) == "" {
		return "See " + DocsURL + "/CLI.md"
	}
	return "See " + DocsURL + "/CLI.md#" + anchor
}

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
	case "validate-prd":
		return runValidatePRD(args[2:], stdout, stderr)
	case "bootstrap":
		return runBootstrap(args[2:], stdout, stderr, rt)
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
	case "runtime":
		return runRuntime(args[2:], stdout, stderr, rt)
	case "feedback":
		return runFeedback(args[2:], stdout, stderr, rt)
	case "resume":
		return runResume(args[2:], stdout, stderr, rt)
	case "status":
		return runStatus(args[2:], stdout, stderr)
	default:
		if looksLikePRDPath(cmd) {
			return runBootstrap(args[1:], stdout, stderr, rt)
		}
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
