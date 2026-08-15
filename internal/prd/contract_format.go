package prd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// FormatContract writes the human-facing validation report.
func FormatContract(w io.Writer, r *ContractResult) error {
	if r == nil {
		return fmt.Errorf("nil contract result")
	}
	fmt.Fprintln(w, "PRD CONTRACT VALIDATION")
	fmt.Fprintf(w, "Status: %s\n", r.Status)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Blocking issues: %d\n", r.BlockingCount)
	fmt.Fprintf(w, "Warnings: %d\n", r.WarningCount)
	if r.InfoCount > 0 {
		fmt.Fprintf(w, "Info: %d\n", r.InfoCount)
	}
	for _, f := range r.Findings {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "[%s] %s\n", f.ID, f.Severity)
		fmt.Fprintf(w, "category: %s\n", f.Category)
		fmt.Fprintf(w, "location: %s\n", dash(f.Location))
		if f.RequirementID != "" {
			fmt.Fprintf(w, "requirement: %s\n", f.RequirementID)
		}
		if f.PhaseID != "" {
			fmt.Fprintf(w, "phase: %s\n", f.PhaseID)
		}
		fmt.Fprintln(w, "problem:")
		fmt.Fprintln(w, f.Problem)
		fmt.Fprintln(w, "why:")
		fmt.Fprintln(w, f.Why)
		fmt.Fprintln(w, "required_action:")
		fmt.Fprintln(w, f.RequiredAction)
	}
	fmt.Fprintln(w)
	if r.Status == ContractRejected {
		fmt.Fprintln(w, "Required action:")
		fmt.Fprintln(w, "Update the PRD to resolve the blocking issues.")
	} else {
		fmt.Fprintln(w, "The PRD may proceed to project bootstrap.")
	}
	return nil
}

// FormatContractJSON writes the complete deterministic finding set.
func FormatContractJSON(w io.Writer, r *ContractResult) error {
	if r == nil {
		return fmt.Errorf("nil contract result")
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
