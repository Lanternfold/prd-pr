package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/preflight"
)

func runRun(args []string, stdout, stderr io.Writer, rt Runtime) int {
	opts, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitUsage
	}

	engOpts := engine.Options{Timeout: opts.timeout, Now: rt.Now}
	switch opts.worker {
	case "fake":
		engOpts.Worker = cursor.Fake{
			ClaimSuccess: true,
			WriteRel:     "WORKER_FAKE.txt",
			WriteBody:    "fake worker output\n",
			Now:          rt.Now,
		}
	case "cursor", "":
		engOpts.Worker = &cursor.CLI{LookPath: rt.LookPath, Now: rt.Now}
	default:
		fmt.Fprintf(stderr, "unknown --worker %q (want cursor or fake)\n", opts.worker)
		return exitUsage
	}

	res, err := engine.New(engOpts).Run(context.Background(), engine.Request{
		ProductRoot: opts.root,
		PRDPath:     opts.prd,
		PhaseID:     opts.phase,
		Mode:        preflight.ModeHeadless,
	})
	if err != nil {
		printStateErr(stderr, err)
		return exitError
	}
	ex := res.Execution
	if ex.RefusalReason != "" && !ex.Invoked {
		fmt.Fprintf(stderr, "refused: %s\n", ex.RefusalReason)
		fmt.Fprintf(stdout, "invoked: false\n")
		fmt.Fprintf(stdout, "worker_claimed_success: false\n")
		fmt.Fprintf(stdout, "verified_success: false\n")
		if ex.PacketRef != "" || fileExists(opts.root, ".project/execution.json") {
			fmt.Fprintf(stdout, "execution: %s\n", ".project/execution.json")
		}
		return exitError
	}

	fmt.Fprintf(stdout, "run_id: %s\n", ex.RunID)
	fmt.Fprintf(stdout, "task_id: %s\n", ex.TaskID)
	fmt.Fprintf(stdout, "phase_id: %s\n", ex.PhaseID)
	fmt.Fprintf(stdout, "baseline: %s\n", emptyDash(ex.Baseline.SHA))
	fmt.Fprintf(stdout, "invoked: %t\n", ex.Invoked)
	fmt.Fprintf(stdout, "worker_claimed_success: %t\n", ex.WorkerClaimedSuccess)
	fmt.Fprintf(stdout, "verified_success: %t\n", ex.VerifiedSuccess)
	fmt.Fprintf(stdout, "exit_code: %d\n", ex.ExitCode)
	fmt.Fprintf(stdout, "changed_paths:\n")
	if len(ex.ChangedPaths) == 0 {
		fmt.Fprintln(stdout, "  (none)")
	} else {
		for _, p := range ex.ChangedPaths {
			fmt.Fprintf(stdout, "  %s\n", p)
		}
	}
	fmt.Fprintf(stdout, "execution: %s\n", ".project/execution.json")
	fmt.Fprintf(stdout, "packet: %s\n", ex.PacketRef)
	if ex.TimedOut {
		fmt.Fprintln(stderr, "worker timed out")
		return exitError
	}
	return exitOK
}

type runOpts struct {
	root    string
	prd     string
	phase   prd.PhaseID
	worker  string
	timeout time.Duration
}

func parseRunArgs(args []string) (runOpts, error) {
	opts := runOpts{worker: "cursor", timeout: cursor.DefaultTimeout}
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			return runOpts{}, fmt.Errorf("usage: prdpr run [--prd FILE] [--phase ID] [--worker cursor|fake] [--timeout DURATION] [directory]\n%s", docsHint("run"))
		case a == "--prd":
			if i+1 >= len(args) {
				return runOpts{}, fmt.Errorf("usage: prdpr run [--prd FILE] [--phase ID] [--worker cursor|fake] [--timeout DURATION] [directory]")
			}
			i++
			opts.prd = args[i]
		case strings.HasPrefix(a, "--prd="):
			opts.prd = strings.TrimPrefix(a, "--prd=")
		case a == "--phase":
			if i+1 >= len(args) {
				return runOpts{}, fmt.Errorf("missing --phase value")
			}
			i++
			id, err := prd.ParsePhaseID(args[i])
			if err != nil {
				return runOpts{}, err
			}
			opts.phase = id
		case strings.HasPrefix(a, "--phase="):
			id, err := prd.ParsePhaseID(strings.TrimPrefix(a, "--phase="))
			if err != nil {
				return runOpts{}, err
			}
			opts.phase = id
		case a == "--worker":
			if i+1 >= len(args) {
				return runOpts{}, fmt.Errorf("missing --worker value")
			}
			i++
			opts.worker = args[i]
		case strings.HasPrefix(a, "--worker="):
			opts.worker = strings.TrimPrefix(a, "--worker=")
		case a == "--timeout":
			if i+1 >= len(args) {
				return runOpts{}, fmt.Errorf("missing --timeout value")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return runOpts{}, fmt.Errorf("invalid --timeout: %w", err)
			}
			opts.timeout = d
		case strings.HasPrefix(a, "--timeout="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "--timeout="))
			if err != nil {
				return runOpts{}, fmt.Errorf("invalid --timeout: %w", err)
			}
			opts.timeout = d
		case strings.HasPrefix(a, "-"):
			return runOpts{}, fmt.Errorf("unknown flag %q\nusage: prdpr run [--prd FILE] [--phase ID] [--worker cursor|fake] [--timeout DURATION] [directory]", a)
		default:
			positional = append(positional, a)
		}
	}
	root, err := resolveProductRoot(positional)
	if err != nil {
		return runOpts{}, err
	}
	opts.root = root
	return opts, nil
}

func fileExists(root, rel string) bool {
	_, err := os.Stat(root + "/" + rel)
	return err == nil
}
