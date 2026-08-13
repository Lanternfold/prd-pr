package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestDoctorReportsEnvironment(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "doctor"}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	out := stdout.String()
	want := []string{
		"PRD→PR version:",
		"dev",
		"operating system:",
		"darwin",
		"architecture:",
		"arm64",
		"Go version:",
		"go1.26.4",
		"Git:",
		"available",
		"Git version:",
		"git version 2.50.0",
	}
	for _, s := range want {
		if !strings.Contains(out, s) {
			t.Fatalf("doctor output missing %q:\n%s", s, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestDoctorMissingGit(t *testing.T) {
	rt := testRuntime()
	rt.LookPath = func(string) (string, error) {
		return "", errors.New("executable file not found in $PATH")
	}
	rt.GitVersion = func() (string, error) {
		t.Fatal("GitVersion should not be called when git is missing")
		return "", nil
	}

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "doctor"}, stdout, stderr, rt)
	if code != exitError {
		t.Fatalf("exit code = %d, want %d", code, exitError)
	}
	out := stdout.String()
	if !strings.Contains(out, "missing") {
		t.Fatalf("stdout should report Git missing:\n%s", out)
	}
	if !strings.Contains(out, "not available") {
		t.Fatalf("stdout should report Git version not available:\n%s", out)
	}
	errOut := stderr.String()
	if !strings.Contains(strings.ToLower(errOut), "git") {
		t.Fatalf("stderr should mention Git, got %q", errOut)
	}
	if !strings.Contains(errOut, "PATH") {
		t.Fatalf("stderr should mention PATH, got %q", errOut)
	}
}

func TestDoctorGitPresentVersionUnknown(t *testing.T) {
	rt := testRuntime()
	rt.GitVersion = func() (string, error) {
		return "", errors.New("failed")
	}
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "doctor"}, stdout, stderr, rt)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, exitOK, stderr.String())
	}
	if !strings.Contains(stdout.String(), "unknown") {
		t.Fatalf("expected unknown git version:\n%s", stdout.String())
	}
}
