package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/engine"
	"github.com/lanternfold/prd-pr/internal/prd"
)

func looksLikePRDPath(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	if strings.HasSuffix(strings.ToLower(s), ".md") || strings.Contains(s, string(os.PathSeparator)) || strings.Contains(s, "/") {
		return true
	}
	if st, err := os.Stat(s); err == nil && !st.IsDir() {
		return true
	}
	return false
}

func runBootstrap(args []string, stdout, stderr io.Writer, rt Runtime) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: prdpr bootstrap <PRD.md>\n       prdpr <PRD.md>")
			fmt.Fprintln(stdout, docsHint("bootstrap"))
			return exitOK
		}
	}
	path := ""
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			fmt.Fprintf(stderr, "unknown flag %q\nUsage: prdpr bootstrap <PRD.md>\n", a)
			return exitUsage
		}
		if path != "" {
			fmt.Fprintln(stderr, "Usage: prdpr bootstrap <PRD.md>")
			return exitUsage
		}
		path = a
	}
	if path == "" {
		fmt.Fprintln(stderr, "Usage: prdpr bootstrap <PRD.md>")
		return exitUsage
	}
	res, err := engine.New(engineOpts(rt)).OpenFromPRD(context.Background(), path)
	if err != nil {
		printStateErr(stderr, err)
		return exitError
	}
	if res.Contract != nil && res.Contract.Rejected() {
		_ = prd.FormatContract(stdout, res.Contract)
		fmt.Fprintf(stderr, "refused: %s\n", res.Execution.RefusalReason)
		return exitError
	}
	if res.WaitingForHuman && res.Human != nil {
		fmt.Fprintf(stdout, "waiting_for_human: true\n")
		fmt.Fprintf(stdout, "human_kind: %s\n", res.Human.Kind)
		fmt.Fprintf(stdout, "human_needed: %s\n", res.Human.Needed)
		if res.ProjectLocation != "" {
			fmt.Fprintf(stdout, "product_root: %s\n", res.ProjectLocation)
		}
		return exitError
	}
	if res.Execution.RefusalReason != "" {
		fmt.Fprintf(stderr, "refused: %s\n", res.Execution.RefusalReason)
		fmt.Fprintf(stdout, "prepared: false\n")
		return exitError
	}
	root := firstNonEmptyCLI(res.ProjectLocation, res.Execution.ProductRoot)
	fmt.Fprintf(stdout, "prepared: true\n")
	fmt.Fprintf(stdout, "product_root: %s\n", root)
	fmt.Fprintf(stdout, "project_type: %s\n", firstNonEmptyCLI(res.ProjectType, "-"))
	fmt.Fprintf(stdout, "prd: %s\n", filepath.Join(root, "PRD.md"))
	fmt.Fprintf(stdout, "run_id: %s\n", res.Execution.RunID)
	fmt.Fprintf(stdout, "task_id: %s\n", res.Execution.TaskID)
	fmt.Fprintf(stdout, "phase_id: %s\n", res.Execution.PhaseID)
	fmt.Fprintf(stdout, "baseline: %s\n", emptyDash(res.Execution.Baseline.SHA))
	fmt.Fprintf(stdout, "packet: %s\n", res.Execution.PacketRef)
	fmt.Fprintf(stdout, "invoked: false\n")
	fmt.Fprintf(stdout, "verified_success: false\n")
	fmt.Fprintf(stdout, "current_state: PREPARED\n")
	return exitOK
}

func runRuntime(args []string, stdout, stderr io.Writer, rt Runtime) int {
	root, err := resolveProductRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	eng := engine.New(engineOpts(rt))
	rep, err := eng.StartRuntime(context.Background(), root)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	fmt.Fprintf(stdout, "ready: %t\n", rep.Ready)
	fmt.Fprintf(stdout, "skipped: %t\n", rep.Skipped)
	if rep.URL != "" {
		fmt.Fprintf(stdout, "url: %s\n", rep.URL)
	}
	if rep.Reason != "" {
		fmt.Fprintf(stdout, "reason: %s\n", rep.Reason)
	}
	if rep.Error != "" {
		fmt.Fprintf(stderr, "runtime error: %s\n", rep.Error)
	}
	if rep.Ready || rep.Skipped {
		return exitOK
	}
	pkt, _, rerr := eng.RepairRuntime(context.Background(), root)
	if rerr != nil {
		fmt.Fprintln(stderr, rerr.Error())
		return exitError
	}
	if pkt.IncidentID != "" {
		fmt.Fprintf(stdout, "repair_incident: %s\n", pkt.IncidentID)
		fmt.Fprintf(stdout, "repair_attempt: %d\n", pkt.Attempt)
		fmt.Fprintln(stdout, "Implement this runtime repair packet in the current Cursor session. Do not launch another Cursor.")
	}
	return exitError
}

func firstNonEmptyCLI(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
