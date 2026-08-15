package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func runVerify(args []string, stdout, stderr io.Writer, rt Runtime) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: prdpr verify [--json] [directory]")
			fmt.Fprintln(stdout, docsHint("verify"))
			return exitOK
		}
	}
	opts, err := parseVerifyArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		if strings.Contains(err.Error(), "usage:") || strings.HasPrefix(err.Error(), "unknown flag") {
			return exitUsage
		}
		return exitError
	}

	eng := engine.New(engine.Options{
		Now:      rt.Now,
		LookPath: rt.LookPath,
	})
	res, err := eng.Verify(nil, engine.Request{ProductRoot: opts.root})
	if err != nil {
		printStateErr(stderr, err)
		return exitError
	}
	if opts.json {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(stderr, "encode verify JSON: %v\n", err)
			return exitError
		}
	} else {
		formatVerify(stdout, res)
	}
	if res.Status == testeng.StatusVerified {
		return exitOK
	}
	return exitError
}

func formatVerify(w io.Writer, res testeng.Result) {
	fmt.Fprintln(w, "PRD→PR VERIFY")
	fmt.Fprintln(w, "────────────────────────")
	fmt.Fprintf(w, "status: %s\n", res.Status)
	fmt.Fprintf(w, "verified_success: %t\n", res.VerifiedSuccess)
	fmt.Fprintf(w, "tests_pass: %t\n", res.TestsPass)
	fmt.Fprintf(w, "all_verifiable_acceptance_criteria_pass: %t\n", res.AllVerifiableAcceptanceCriteriaPass)
	fmt.Fprintf(w, "manual_verification_required: %t\n", res.ManualVerificationRequired)
	if res.RunID != "" {
		fmt.Fprintf(w, "run_id: %s\n", res.RunID)
	}
	if res.TaskID != "" {
		fmt.Fprintf(w, "task_id: %s\n", res.TaskID)
	}
	if res.PhaseID != "" {
		fmt.Fprintf(w, "phase_id: %s\n", res.PhaseID)
	}
	fmt.Fprintf(w, "baseline: %s\n", emptyDash(res.BaselineSHA))
	fmt.Fprintf(w, "head: %s\n", emptyDash(res.HeadSHA))
	if res.Reason != "" {
		fmt.Fprintf(w, "reason: %s\n", res.Reason)
	}
	fmt.Fprintln(w, "checks:")
	if len(res.Checks) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, c := range res.Checks {
			fmt.Fprintf(w, "  %s %s %s\n", c.ID, c.Outcome, c.Name)
			if c.Detail != "" && c.Outcome != testeng.OutcomePass {
				fmt.Fprintf(w, "    %s\n", c.Detail)
			}
		}
	}
	if len(res.Failures) > 0 {
		fmt.Fprintln(w, "failures:")
		for _, f := range res.Failures {
			fmt.Fprintf(w, "  %s\n", f)
		}
	}
	fmt.Fprintf(w, "verification: %s\n", ".project/verification.json")
}

type verifyOpts struct {
	root string
	json bool
}

func parseVerifyArgs(args []string) (verifyOpts, error) {
	var opts verifyOpts
	var positional []string
	usage := "usage: prdpr verify [--json] [directory]"
	for _, a := range args {
		switch {
		case a == "--json":
			opts.json = true
		case strings.HasPrefix(a, "-"):
			return verifyOpts{}, fmt.Errorf("unknown flag %q\n%s", a, usage)
		default:
			positional = append(positional, a)
		}
	}
	root, err := resolveProductRoot(positional)
	if err != nil {
		return verifyOpts{}, err
	}
	opts.root = root
	return opts, nil
}
