package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func testRuntime() Runtime {
	return Runtime{
		AppVersion: "dev",
		GOOS:       "darwin",
		GOARCH:     "arm64",
		GoVersion:  "go1.26.4",
		LookPath: func(file string) (string, error) {
			if file == "git" {
				return "/usr/bin/git", nil
			}
			return "", errors.New("not found")
		},
		GitVersion: func() (string, error) {
			return "git version 2.50.0", nil
		},
	}
}

func TestDispatchVersion(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "version"}, stdout, stderr, testRuntime())
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if got := strings.TrimSpace(stdout.String()); got != "dev" {
		t.Fatalf("version output = %q, want %q", got, "dev")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestDispatchHelp(t *testing.T) {
	for _, args := range [][]string{
		{"prdpr"},
		{"prdpr", "help"},
		{"prdpr", "-h"},
		{"prdpr", "--help"},
	} {
		stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
		code := Main(args, stdout, stderr, testRuntime())
		if code != exitOK {
			t.Fatalf("%v: exit code = %d, want %d", args, code, exitOK)
		}
		out := stdout.String()
		if !strings.Contains(out, "Usage:") {
			t.Fatalf("%v: help missing Usage:\n%s", args, out)
		}
		for _, cmd := range []string{"version", "help", "doctor", "init", "inspect", "validate-prd", "bootstrap", "preflight", "prepare", "run", "verify", "review", "repair", "phase", "runtime", "feedback", "resume", "status"} {
			if !strings.Contains(out, cmd) {
				t.Fatalf("%v: help missing command %q\n%s", args, cmd, out)
			}
		}
		if !strings.Contains(out, DocsURL) {
			t.Fatalf("%v: help missing documentation URL\n%s", args, out)
		}
		if stderr.Len() != 0 {
			t.Fatalf("%v: stderr = %q, want empty", args, stderr.String())
		}
	}
}

func TestDispatchUnknownCommand(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "bogus"}, stdout, stderr, testRuntime())
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, `unknown command "bogus"`) {
		t.Fatalf("stderr = %q, want unknown command message", errOut)
	}
	if !strings.Contains(errOut, "Usage:") {
		t.Fatalf("stderr should include usage, got %q", errOut)
	}
}

func TestVersionUsesRuntimeOverride(t *testing.T) {
	rt := testRuntime()
	rt.AppVersion = "1.2.3"
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "version"}, stdout, stderr, rt)
	if code != exitOK {
		t.Fatalf("exit code = %d, want %d", code, exitOK)
	}
	if got := strings.TrimSpace(stdout.String()); got != "1.2.3" {
		t.Fatalf("version output = %q, want %q", got, "1.2.3")
	}
}
