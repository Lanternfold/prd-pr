package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
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

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "PRD→PR version:\t%s\n", versionLine(rt))
	fmt.Fprintf(w, "operating system:\t%s\n", valueOrUnknown(rt.GOOS))
	fmt.Fprintf(w, "architecture:\t%s\n", valueOrUnknown(rt.GOARCH))
	fmt.Fprintf(w, "Go version:\t%s\n", valueOrUnknown(rt.GoVersion))
	fmt.Fprintf(w, "Git:\t%s\n", gitStatus)
	fmt.Fprintf(w, "Git version:\t%s\n", gitVersion)
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

func valueOrUnknown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}
