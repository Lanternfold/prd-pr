package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/prd"
)

func runPrepare(args []string, stdout, stderr io.Writer, rt Runtime) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: prdpr prepare [--prd FILE] [--phase ID] [directory]")
			fmt.Fprintln(stdout, docsHint("prepare"))
			return exitOK
		}
	}
	opts, err := parsePrepareArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		if strings.Contains(err.Error(), "usage:") || strings.HasPrefix(err.Error(), "unknown flag") || strings.HasPrefix(err.Error(), "missing --phase") {
			return exitUsage
		}
		return exitError
	}

	engOpts := engine.Options{Now: rt.Now, PreflightEnv: preflightEnv(rt)}
	res, err := engine.New(engOpts).Prepare(context.Background(), engine.Request{
		ProductRoot: opts.root,
		PRDPath:     opts.prd,
		PhaseID:     opts.phase,
	})
	if err != nil {
		printStateErr(stderr, err)
		return exitError
	}
	ex := res.Execution
	if res.ProjectCompleted {
		fmt.Fprintf(stdout, "prepared: false\n")
		fmt.Fprintf(stdout, "project_completed: true\n")
		fmt.Fprintf(stdout, "project_status: %s\n", "PROJECT_COMPLETED")
		fmt.Fprintf(stdout, "current_state: COMPLETED\n")
		fmt.Fprintf(stdout, "invoked: false\n")
		fmt.Fprintf(stdout, "verified_success: false\n")
		return exitOK
	}
	if ex.RefusalReason != "" {
		if res.Contract != nil && res.Contract.Rejected() {
			_ = prd.FormatContract(stdout, res.Contract)
		}
		fmt.Fprintf(stderr, "refused: %s\n", ex.RefusalReason)
		fmt.Fprintf(stdout, "prepared: false\n")
		fmt.Fprintf(stdout, "invoked: false\n")
		fmt.Fprintf(stdout, "verified_success: false\n")
		if fileExists(opts.root, ".project/execution.json") {
			fmt.Fprintf(stdout, "execution: %s\n", ".project/execution.json")
		}
		return exitError
	}

	fmt.Fprintf(stdout, "prepared: true\n")
	fmt.Fprintf(stdout, "run_id: %s\n", ex.RunID)
	fmt.Fprintf(stdout, "task_id: %s\n", ex.TaskID)
	fmt.Fprintf(stdout, "phase_id: %s\n", ex.PhaseID)
	fmt.Fprintf(stdout, "baseline: %s\n", emptyDash(ex.Baseline.SHA))
	fmt.Fprintf(stdout, "packet: %s\n", ex.PacketRef)
	fmt.Fprintf(stdout, "execution: %s\n", ".project/execution.json")
	fmt.Fprintf(stdout, "invoked: false\n")
	fmt.Fprintf(stdout, "worker_claimed_success: false\n")
	fmt.Fprintf(stdout, "verified_success: false\n")
	fmt.Fprintf(stdout, "current_state: PREPARED\n")
	return exitOK
}

type prepareOpts struct {
	root  string
	prd   string
	phase prd.PhaseID
}

func parsePrepareArgs(args []string) (prepareOpts, error) {
	var opts prepareOpts
	var positional []string
	usage := "usage: prdpr prepare [--prd FILE] [--phase ID] [directory]"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			return prepareOpts{}, fmt.Errorf("%s", usage)
		case a == "--prd":
			if i+1 >= len(args) {
				return prepareOpts{}, fmt.Errorf("%s", usage)
			}
			i++
			opts.prd = args[i]
		case strings.HasPrefix(a, "--prd="):
			opts.prd = strings.TrimPrefix(a, "--prd=")
		case a == "--phase":
			if i+1 >= len(args) {
				return prepareOpts{}, fmt.Errorf("missing --phase value")
			}
			i++
			id, err := prd.ParsePhaseID(args[i])
			if err != nil {
				return prepareOpts{}, err
			}
			opts.phase = id
		case strings.HasPrefix(a, "--phase="):
			id, err := prd.ParsePhaseID(strings.TrimPrefix(a, "--phase="))
			if err != nil {
				return prepareOpts{}, err
			}
			opts.phase = id
		case strings.HasPrefix(a, "-"):
			return prepareOpts{}, fmt.Errorf("unknown flag %q\n%s", a, usage)
		default:
			positional = append(positional, a)
		}
	}
	root, err := resolveProductRoot(positional)
	if err != nil {
		return prepareOpts{}, err
	}
	opts.root = root
	return opts, nil
}
