package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/lanternfold/prd-pr/internal/preflight"
)

func runDoctor(stdout, stderr io.Writer, rt Runtime) int {
	gitStatus := "available"
	gitVersion := ""
	gitOK := true

	path, err := lookGit(rt)
	if err != nil || path == "" {
		gitStatus = "missing"
		gitVersion = "not available"
		gitOK = false
	} else {
		ver, verr := gitVer(rt)
		if verr != nil || strings.TrimSpace(ver) == "" {
			gitVersion = "unknown"
		} else {
			gitVersion = strings.TrimSpace(ver)
		}
	}

	m := preflight.InspectMachine(preflightEnv(rt))

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "PRD→PR version:\t%s\n", versionLine(rt))
	fmt.Fprintf(w, "operating system:\t%s\n", valueOrUnknown(rt.GOOS))
	fmt.Fprintf(w, "architecture:\t%s\n", valueOrUnknown(rt.GOARCH))
	fmt.Fprintf(w, "Go version:\t%s\n", valueOrUnknown(rt.GoVersion))
	fmt.Fprintf(w, "Git:\t%s\n", gitStatus)
	fmt.Fprintf(w, "Git version:\t%s\n", gitVersion)
	fmt.Fprintf(w, "Cursor editor:\t%s\n", presentMissing(m.CursorEditor))
	fmt.Fprintf(w, "Cursor Agent:\t%s\n", presentMissing(m.CursorAgent))
	fmt.Fprintf(w, "GitHub CLI:\t%s\n", presentMissing(m.GitHubCLI))
	_ = w.Flush()

	if !gitOK {
		fmt.Fprintln(stderr, "Git was not found on PATH. Install Git and ensure the git executable is available.")
		return exitError
	}
	return exitOK
}

func lookGit(rt Runtime) (string, error) {
	if rt.LookPath == nil {
		return "", fmt.Errorf("git lookup not configured")
	}
	return rt.LookPath("git")
}

func gitVer(rt Runtime) (string, error) {
	if rt.GitVersion == nil {
		return "", fmt.Errorf("git version lookup not configured")
	}
	return rt.GitVersion()
}

func presentMissing(ok bool) string {
	if ok {
		return "available"
	}
	return "missing"
}

func valueOrUnknown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}
