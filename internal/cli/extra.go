package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/lanternfold/prd-pr/internal/config"
	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/preflight"
)

func engineOpts(rt Runtime) engine.Options {
	return engine.Options{
		Now:      rt.Now,
		LookPath: rt.LookPath,
		Config:   config.Defaults(),
	}
}

func runReview(args []string, stdout, stderr io.Writer, rt Runtime) int {
	root, jsonOut, err := parseDirJSON(args, "usage: prdpr review [--json] [directory]")
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitUsage
	}
	res, err := engine.New(engineOpts(rt)).Review(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		printStateErr(stderr, err)
		return exitError
	}
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		fmt.Fprintf(stdout, "classification: %s\n", res.Diagnosis.Classification)
		fmt.Fprintf(stdout, "actionable: %t\n", res.Diagnosis.Actionable)
		fmt.Fprintf(stdout, "recommend_repair: %t\n", res.Repair)
		fmt.Fprintf(stdout, "recommend_human: %t\n", res.Human)
		fmt.Fprintf(stdout, "selected_model: %s\n", res.Model.SelectedModel)
		fmt.Fprintf(stdout, "reason: %s\n", res.Model.Reason)
		fmt.Fprintf(stdout, "summary: %s\n", res.Diagnosis.Summary)
		if res.ReviewFailure != "" {
			fmt.Fprintf(stdout, "review_failure: %s\n", res.ReviewFailure)
		}
	}
	if res.ReviewFailure != "" {
		return exitError
	}
	return exitOK
}

func runRepair(args []string, stdout, stderr io.Writer, rt Runtime) int {
	root, jsonOut, err := parseDirJSON(args, "usage: prdpr repair [--json] [directory]")
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitUsage
	}
	pkt, err := engine.New(engineOpts(rt)).PrepareRepair(context.Background(), engine.Request{ProductRoot: root})
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(pkt)
		return exitOK
	}
	fmt.Fprintf(stdout, "incident: %s\n", pkt.IncidentID)
	fmt.Fprintf(stdout, "attempt: %d\n", pkt.Attempt)
	fmt.Fprintf(stdout, "max_attempts: %d\n", pkt.MaxAttempts)
	fmt.Fprintf(stdout, "previous_attempts: %d\n", len(pkt.PreviousAttempts))
	fmt.Fprintf(stdout, "summary: %s\n", pkt.Diagnosis.Summary)
	fmt.Fprintln(stdout, "allowed_paths:")
	for _, p := range pkt.AllowedPaths {
		fmt.Fprintf(stdout, "  %s\n", p)
	}
	fmt.Fprintf(stdout, "packet: %s\n", fmt.Sprintf(".project/packets/repair_%s_%d.json", pkt.IncidentID, pkt.Attempt))
	fmt.Fprintln(stdout, "Implement this repair packet in the current Cursor session. Do not launch another Cursor.")
	return exitOK
}

func runPhase(args []string, stdout, stderr io.Writer, rt Runtime) int {
	opts, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitUsage
	}
	engOpts := engineOpts(rt)
	engOpts.Timeout = opts.timeout
	engOpts.SkipWait = true
	switch opts.worker {
	case "fake":
		engOpts.Worker = cursor.Fake{ClaimSuccess: true, WriteRel: "WORKER_FAKE.txt", WriteBody: "fake worker output\n", Now: rt.Now}
	case "cursor", "":
		engOpts.Worker = &cursor.CLI{LookPath: rt.LookPath, Now: rt.Now}
	default:
		fmt.Fprintf(stderr, "unknown --worker %q\n", opts.worker)
		return exitUsage
	}
	res, err := engine.New(engOpts).RunGraph(context.Background(), engine.Request{
		ProductRoot:   opts.root,
		PRDPath:       opts.prd,
		PhaseID:       opts.phase,
		Mode:          preflight.ModeHeadless,
		ExecutionMode: executionMode(opts.selfDevelopment),
	})
	if err != nil {
		printStateErr(stderr, err)
		return exitError
	}
	fmt.Fprintf(stdout, "completed: %t\n", res.Completed)
	fmt.Fprintf(stdout, "waiting_for_human: %t\n", res.Waiting)
	fmt.Fprintf(stdout, "project_completed: %t\n", res.ProjectCompleted)
	if len(res.Phases) > 0 {
		fmt.Fprintf(stdout, "phases: %s\n", strings.Join(res.Phases, ","))
	}
	fmt.Fprintf(stdout, "verified_success: %t\n", res.Verification.VerifiedSuccess)
	fmt.Fprintf(stdout, "worker_claimed_success: %t\n", res.Execution.WorkerClaimedSuccess)
	if res.Human != nil {
		fmt.Fprintf(stdout, "human_request: %s\n", res.Human.ID)
		fmt.Fprintf(stdout, "human_needed: %s\n", res.Human.Needed)
	}
	if res.Completed || res.ProjectCompleted {
		return exitOK
	}
	return exitError
}

func runCommit(args []string, stdout, stderr io.Writer, rt Runtime) int {
	root, err := resolveProductRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	sha, err := engine.New(engineOpts(rt)).CommitVerified(context.Background(), root, "prdpr: verified changes")
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	fmt.Fprintf(stdout, "commit: %s\n", sha)
	return exitOK
}

func runPR(args []string, stdout, stderr io.Writer, rt Runtime) int {
	root, err := resolveProductRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	eng := engine.New(engine.Options{Now: rt.Now, LookPath: rt.LookPath, Config: cfg})
	pr, err := eng.OpenMilestonePR(context.Background(), root, "PRD→PR milestone", "Created by prdpr. Do not auto-merge.")
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	fmt.Fprintf(stdout, "skipped: %t\n", pr.Skipped)
	if pr.URL != "" {
		fmt.Fprintf(stdout, "url: %s\n", pr.URL)
	}
	if pr.Reason != "" {
		fmt.Fprintf(stdout, "reason: %s\n", pr.Reason)
	}
	return exitOK
}

func runChecks(args []string, stdout, stderr io.Writer, rt Runtime) int {
	root, err := resolveProductRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	rep := engine.New(engine.Options{Now: rt.Now, LookPath: rt.LookPath, Config: cfg}).InspectChecks(context.Background(), root)
	fmt.Fprintf(stdout, "status: %s\n", rep.Status)
	fmt.Fprintf(stdout, "verdict: %s\n", rep.Verdict())
	if rep.Detail != "" {
		fmt.Fprintf(stdout, "detail: %s\n", rep.Detail)
	}
	return exitOK
}

func runMerge(args []string, stdout, stderr io.Writer, rt Runtime) int {
	root, err := resolveProductRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	cfg := config.Defaults()
	cfg.GitHubEnabled = true
	dec, res, err := engine.New(engine.Options{Now: rt.Now, LookPath: rt.LookPath, Config: cfg}).TryMerge(context.Background(), root)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	fmt.Fprintf(stdout, "allow: %t\n", dec.Allow)
	fmt.Fprintf(stdout, "status: %s\n", dec.Status)
	if dec.Reason != "" {
		fmt.Fprintf(stdout, "reason: %s\n", dec.Reason)
	}
	if res.Merged {
		fmt.Fprintf(stdout, "merged: true\n")
		fmt.Fprintf(stdout, "sha: %s\n", res.SHA)
	}
	if !dec.Allow {
		return exitError
	}
	return exitOK
}

func runFeedback(args []string, stdout, stderr io.Writer, rt Runtime) int {
	opts, err := parseFeedbackArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitUsage
	}
	if err := engine.New(engineOpts(rt)).Feedback(opts.root, opts.id, opts.text, opts.status, opts.cred); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	fmt.Fprintln(stdout, "recorded")
	return exitOK
}

func runResume(args []string, stdout, stderr io.Writer, rt Runtime) int {
	root, err := resolveProductRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	if err := engine.New(engineOpts(rt)).Resume(context.Background(), root); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	fmt.Fprintln(stdout, "resumed")
	return exitOK
}

func parseDirJSON(args []string, usage string) (root string, jsonOut bool, err error) {
	var positional []string
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			return "", false, fmt.Errorf("%s\n%s", usage, docsHint(""))
		case a == "--json":
			jsonOut = true
		case strings.HasPrefix(a, "-"):
			return "", false, fmt.Errorf("unknown flag %q\n%s", a, usage)
		default:
			positional = append(positional, a)
		}
	}
	root, err = resolveProductRoot(positional)
	return root, jsonOut, err
}

type feedbackOpts struct {
	root, id, text, status, cred string
}

func parseFeedbackArgs(args []string) (feedbackOpts, error) {
	var opts feedbackOpts
	var positional []string
	usage := "usage: prdpr feedback [--request ID] [--text TEXT] [--status STATUS] [--credential NAME] [directory]"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--request":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("%s", usage)
			}
			opts.id = args[i]
		case a == "--text":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("%s", usage)
			}
			opts.text = args[i]
		case a == "--status":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("%s", usage)
			}
			opts.status = args[i]
		case a == "--credential":
			i++
			if i >= len(args) {
				return opts, fmt.Errorf("%s", usage)
			}
			opts.cred = args[i]
		case strings.HasPrefix(a, "-"):
			return opts, fmt.Errorf("unknown flag %q\n%s", a, usage)
		default:
			positional = append(positional, a)
		}
	}
	root, err := resolveProductRoot(positional)
	if err != nil {
		return opts, err
	}
	opts.root = root
	if opts.status == "" {
		opts.status = human.CredPresentUnverified
	}
	return opts, nil
}
