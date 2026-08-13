package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/lanternfold/prd-pr/internal/state"
)

func runInit(args []string, stdout, stderr io.Writer) int {
	root, err := resolveProductRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		if len(args) > 1 {
			return exitUsage
		}
		return exitError
	}
	store, err := state.Open(root)
	if err != nil {
		fmt.Fprintf(stderr, "Product root is invalid: %v\n", err)
		return exitError
	}
	result, err := store.Init()
	if err != nil {
		printStateErr(stderr, err)
		return exitError
	}
	if result.Created {
		fmt.Fprintf(stdout, "Initialized PRD→PR project %s\n", result.State.ProjectID)
		fmt.Fprintf(stdout, "Product root: %s\n", result.State.ProductRoot)
		fmt.Fprintf(stdout, "Created %s and %s\n", filepath.Join(state.DirName, state.StateFileName), filepath.Join(state.DirName, state.EventsFileName))
		return exitOK
	}
	fmt.Fprintf(stdout, "Project already initialized (%s).\n", result.State.ProjectID)
	fmt.Fprintf(stdout, "Existing state was left unchanged.\n")
	fmt.Fprintf(stdout, "Use \"prdpr status\" to inspect it.\n")
	return exitOK
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	root, err := resolveProductRoot(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		if len(args) > 1 {
			return exitUsage
		}
		return exitError
	}
	store, err := state.Open(root)
	if err != nil {
		fmt.Fprintf(stderr, "Product root is invalid: %v\n", err)
		return exitError
	}
	st, err := store.Load()
	if err != nil {
		if errors.Is(err, state.ErrNotInitialized) {
			fmt.Fprintf(stderr, "No PRD→PR project found in %s.\nRun \"prdpr init\" to create %s/.\n", root, state.DirName)
			return exitError
		}
		printStateErr(stderr, err)
		return exitError
	}
	fmt.Fprintf(stdout, "schema_version: %d\n", st.SchemaVersion)
	fmt.Fprintf(stdout, "project_id: %s\n", st.ProjectID)
	fmt.Fprintf(stdout, "product_root: %s\n", st.ProductRoot)
	fmt.Fprintf(stdout, "project_status: %s\n", st.ProjectStatus)
	fmt.Fprintf(stdout, "current_run_id: %s\n", emptyDash(st.CurrentRunID))
	fmt.Fprintf(stdout, "current_phase_id: %s\n", emptyDash(st.CurrentPhaseID))
	fmt.Fprintf(stdout, "current_state: %s\n", emptyDash(st.CurrentState))
	fmt.Fprintf(stdout, "current_commit: %s\n", emptyDash(st.CurrentCommit))
	fmt.Fprintf(stdout, "last_known_good_commit: %s\n", emptyDash(st.LastKnownGoodCommit))
	fmt.Fprintf(stdout, "created_at: %s\n", st.CreatedAt)
	fmt.Fprintf(stdout, "updated_at: %s\n", st.UpdatedAt)
	return exitOK
}

func resolveProductRoot(args []string) (string, error) {
	var p string
	switch len(args) {
	case 0:
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot determine working directory: %w", err)
		}
		p = wd
	case 1:
		p = args[0]
	default:
		return "", fmt.Errorf("usage: prdpr init|status [directory]")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("product root %q is invalid: %w", p, err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("product root %q is invalid: %w", abs, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("product root %q is not a directory", abs)
	}
	return abs, nil
}

func printStateErr(stderr io.Writer, err error) {
	var le *state.LockError
	if errors.As(err, &le) {
		fmt.Fprintln(stderr, le.Error())
		return
	}
	fmt.Fprintln(stderr, err.Error())
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
