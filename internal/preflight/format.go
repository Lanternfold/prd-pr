package preflight

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func Format(w io.Writer, r *Report) error {
	if r == nil {
		return fmt.Errorf("nil preflight report")
	}
	fmt.Fprintln(w, "PRD→PR PREFLIGHT")
	fmt.Fprintln(w, "────────────────────────")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Execution mode: %s\n", valueOrDash(r.ExecutionMode))
	fmt.Fprintf(w, "Worker mechanism: %s\n", valueOrDash(r.WorkerMechanism))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Project:")
	fmt.Fprintln(w, valueOrDash(r.ProjectName))
	fmt.Fprintln(w)

	writeNamed(w, "PRD", checkByID(r, "project.prd"))
	fmt.Fprintln(w)
	writeNamed(w, "Product root", checkByID(r, "project.root"))
	fmt.Fprintln(w)
	writeGit(w, r)
	fmt.Fprintln(w)
	writeNamed(w, "Cursor Editor", checkByID(r, "machine.cursor_editor"))
	fmt.Fprintln(w)
	writeNamed(w, "Cursor Agent", checkByID(r, "machine.cursor_agent"))
	fmt.Fprintln(w)
	writeNamed(w, "Execution mode", checkByID(r, "execution.mode"))
	fmt.Fprintln(w)
	writeNamed(w, "GitHub CLI", checkByID(r, "machine.github_cli"))
	fmt.Fprintln(w)
	writeNamed(w, "Go", checkByID(r, "machine.go"))
	fmt.Fprintln(w)
	writeCredentials(w, r)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Overall:")
	fmt.Fprintln(w, r.Status)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Reason:")
	if len(r.Blocking) > 0 {
		fmt.Fprintln(w, r.Blocking[0])
	} else {
		fmt.Fprintln(w, r.NextAction)
	}
	return nil
}

func FormatJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func writeNamed(w io.Writer, title string, c *Check) {
	fmt.Fprintf(w, "%s:\n", title)
	if c == nil {
		fmt.Fprintln(w, "⚠ not evaluated")
		return
	}
	fmt.Fprintf(w, "%s %s\n", mark(*c), summary(*c))
	if c.Blocking || c.Status == StatusBlocking || c.Status == StatusError {
		fmt.Fprintln(w, "BLOCKING")
	}
}

func writeGit(w io.Writer, r *Report) {
	fmt.Fprintln(w, "Git:")
	mg := checkByID(r, "machine.git")
	pg := checkByID(r, "project.git")
	if mg != nil {
		if mg.Status == StatusAvailable {
			fmt.Fprintln(w, "✓ available")
		} else {
			fmt.Fprintf(w, "%s %s\n", mark(*mg), summary(*mg))
		}
	}
	if r.Repository != nil {
		switch r.Repository.State {
		case "clean":
			fmt.Fprintln(w, "✓ repository")
			fmt.Fprintln(w, "✓ clean")
		case "dirty":
			fmt.Fprintln(w, "✓ repository")
			fmt.Fprintln(w, "⚠ dirty")
		case "no_commits":
			fmt.Fprintln(w, "✓ repository")
			fmt.Fprintln(w, "✗ no commits")
		case "not_repository":
			fmt.Fprintln(w, "✗ not a repository")
		case "not_installed":
			fmt.Fprintln(w, "✗ missing")
		default:
			if pg != nil {
				fmt.Fprintf(w, "%s %s\n", mark(*pg), summary(*pg))
			}
		}
	} else if pg != nil {
		fmt.Fprintf(w, "%s %s\n", mark(*pg), summary(*pg))
	}
	if pg != nil && (pg.Blocking || pg.Status == StatusBlocking || pg.Status == StatusError) {
		fmt.Fprintln(w, "BLOCKING")
	}
}

func writeCredentials(w io.Writer, r *Report) {
	fmt.Fprintln(w, "Credentials:")
	var creds []Check
	for _, c := range r.Checks {
		if strings.HasPrefix(c.ID, "project.credential") {
			creds = append(creds, c)
		}
	}
	if len(creds) == 0 {
		c := checkByID(r, "project.credentials")
		if c == nil {
			fmt.Fprintln(w, "⚠ not evaluated")
			return
		}
		fmt.Fprintf(w, "%s %s\n", mark(*c), summary(*c))
		return
	}
	for _, c := range creds {
		fmt.Fprintf(w, "%s %s\n", mark(c), summary(c))
		if c.Blocking {
			fmt.Fprintln(w, "BLOCKING")
		}
	}
}

func checkByID(r *Report, id string) *Check {
	for i := range r.Checks {
		if r.Checks[i].ID == id {
			return &r.Checks[i]
		}
	}
	return nil
}

func mark(c Check) string {
	switch {
	case c.Blocking || c.Status == StatusBlocking || c.Status == StatusError || c.Status == StatusMissing:
		return "✗"
	case c.Status == StatusWarning || c.Status == StatusOptional:
		return "⚠"
	default:
		return "✓"
	}
}

func summary(c Check) string {
	if c.Detail != "" {
		switch c.Status {
		case StatusAvailable:
			if strings.HasPrefix(c.Detail, "available") || c.Detail == "valid" || c.Detail == "accessible" || strings.Contains(c.Detail, "clean") {
				return firstToken(c.Detail)
			}
		}
		return c.Detail
	}
	return strings.ToLower(string(c.Status))
}

func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " ;("); i > 0 {
		return s[:i]
	}
	return s
}

func valueOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
