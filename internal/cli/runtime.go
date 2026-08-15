package cli

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Runtime is the environment doctor inspects. Tests inject fakes.
type Runtime struct {
	AppVersion string
	GOOS       string
	GOARCH     string
	GoVersion  string
	LookPath   func(file string) (string, error)
	GitVersion func() (string, error)
	LookupEnv  func(key string) (string, bool)
	Now        func() time.Time
}

// DefaultRuntime inspects this process and the local PATH.
func DefaultRuntime() Runtime {
	return Runtime{
		AppVersion: currentVersion(),
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		GoVersion:  runtime.Version(),
		LookPath:   exec.LookPath,
		LookupEnv:  os.LookupEnv,
		GitVersion: func() (string, error) {
			out, err := exec.Command("git", "version").Output()
			return strings.TrimSpace(string(out)), err
		},
		Now: func() time.Time { return time.Now().UTC() },
	}
}
