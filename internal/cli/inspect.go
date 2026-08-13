package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/prd"
)

func runInspect(args []string, stdout, stderr io.Writer) int {
	jsonOut := false
	showGraph := false
	var path string
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "--graph":
			showGraph = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: prdpr inspect [--json] [--graph] <PRD.md>")
			return exitOK
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(stderr, "unknown flag %q\nUsage: prdpr inspect [--json] [--graph] <PRD.md>\n", a)
				return exitUsage
			}
			if path != "" {
				fmt.Fprintln(stderr, "Usage: prdpr inspect [--json] [--graph] <PRD.md>")
				return exitUsage
			}
			path = a
		}
	}
	if path == "" {
		fmt.Fprintln(stderr, "Usage: prdpr inspect [--json] [--graph] <PRD.md>")
		return exitUsage
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(stderr, "cannot resolve %s: %v\n", path, err)
		return exitError
	}
	fi, err := os.Stat(abs)
	if err != nil {
		fmt.Fprintf(stderr, "cannot read PRD %s: %v\n", path, err)
		return exitError
	}
	if fi.IsDir() {
		fmt.Fprintf(stderr, "%s is a directory; pass a Markdown file\n", path)
		return exitError
	}

	doc, err := prd.ParseFile(abs)
	if err != nil {
		fmt.Fprintf(stderr, "cannot read PRD %s: %v\n", path, err)
		return exitError
	}

	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(doc); err != nil {
			fmt.Fprintf(stderr, "encode inspect JSON: %v\n", err)
			return exitError
		}
		if doc.HasErrors() {
			return exitError
		}
		return exitOK
	}

	display := path
	w, e := doc.Counts()
	fmt.Fprintln(stdout, "PRD→PR")
	fmt.Fprintln(stdout, "──────────────")
	fmt.Fprintf(stdout, "PRD: %s\n\n", display)
	fmt.Fprintf(stdout, "Sections: %d\n", len(doc.Sections))
	fmt.Fprintf(stdout, "Requirements: %d\n", len(doc.Requirements))
	fmt.Fprintf(stdout, "Acceptance criteria: %d\n", len(doc.Acceptance))
	fmt.Fprintf(stdout, "Phases: %d\n\n", len(doc.Phases))
	fmt.Fprintf(stdout, "Warnings: %d\n", w)
	fmt.Fprintf(stdout, "Errors: %d\n\n", e)
	fmt.Fprintf(stdout, "Status: %s\n", doc.Status())

	printDiags(stdout, doc, prd.SevError, "Errors")
	printDiags(stdout, doc, prd.SevWarning, "Warnings")

	code := exitOK
	if doc.HasErrors() {
		code = exitError
	}
	if showGraph {
		g := graph.FromDocument(doc)
		printGraph(stdout, g)
		if g.HasErrors() {
			code = exitError
		}
	}
	return code
}

func printGraph(w io.Writer, g *graph.Graph) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Graph:")
	fmt.Fprintf(w, "Nodes: %d\n", len(g.Nodes))
	fmt.Fprintf(w, "Edges: %d\n", len(g.Edges))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Topological order:")
	order := g.SequentialOrder()
	if len(order) == 0 {
		fmt.Fprintln(w, "(none — graph is cyclic or empty)")
	} else {
		for _, id := range order {
			fmt.Fprintln(w, id)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Ready:")
	ready := g.Ready()
	if len(ready) == 0 {
		fmt.Fprintln(w, "(none)")
	} else {
		for _, id := range ready {
			fmt.Fprintln(w, id)
		}
	}
	cycles := 0
	for _, d := range g.Diagnostics {
		if d.Code == graph.CodeCycle {
			cycles++
			fmt.Fprintf(w, "\nCycles:\n%d\n  %s\n", cycles, d.Message)
		}
	}
	if cycles == 0 {
		fmt.Fprintf(w, "\nCycles:\n0\n")
	}
	for _, d := range g.Diagnostics {
		if d.Severity == graph.SevError {
			fmt.Fprintf(w, "\n  ERROR %s\n  %s\n", d.Code, d.Message)
		}
		if d.Code == graph.CodeNoExplicitDeps {
			fmt.Fprintf(w, "\n%s\n", d.Message)
		}
	}
}

func printDiags(w io.Writer, doc *prd.Document, sev prd.Severity, heading string) {
	var list []prd.Diagnostic
	for _, d := range doc.Diagnostics {
		if d.Severity == sev {
			list = append(list, d)
		}
	}
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s:\n", heading)
	for _, d := range list {
		fmt.Fprintf(w, "  %s %s\n", d.Severity, d.Code)
		loc := d.File
		if loc == "" {
			loc = doc.SourceFile
		}
		if d.StartLine > 0 {
			fmt.Fprintf(w, "  %s:%d\n", filepath.Base(loc), d.StartLine)
		} else {
			fmt.Fprintf(w, "  %s\n", filepath.Base(loc))
		}
		if d.Message != "" {
			fmt.Fprintf(w, "  %s\n", d.Message)
		}
	}
}
