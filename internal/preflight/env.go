package preflight

import (
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/lanternfold/prd-pr/internal/vcs"
)

// Env injects lookups so tests never depend on the host machine.
type Env struct {
	Now        func() time.Time
	LookPath   func(file string) (string, error)
	LookupEnv  func(key string) (string, bool)
	Git        *vcs.Client
	GOOS       string
	GOARCH     string
	GoVersion  string
}

func DefaultEnv() Env {
	return Env{
		Now:       func() time.Time { return time.Now().UTC() },
		LookPath:  exec.LookPath,
		LookupEnv: os.LookupEnv,
		Git:       vcs.Default(),
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		GoVersion: runtime.Version(),
	}
}

func (e Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now().UTC()
}

func (e Env) lookPath() func(string) (string, error) {
	if e.LookPath != nil {
		return e.LookPath
	}
	return exec.LookPath
}

func (e Env) lookupEnv() func(string) (string, bool) {
	if e.LookupEnv != nil {
		return e.LookupEnv
	}
	return os.LookupEnv
}

func (e Env) git() *vcs.Client {
	if e.Git != nil {
		if e.Git.LookPath == nil {
			e.Git.LookPath = e.lookPath()
		}
		return e.Git
	}
	return &vcs.Client{LookPath: e.lookPath()}
}

func (e Env) hasBinary(name string) (string, bool) {
	p, err := e.lookPath()(name)
	if err != nil || p == "" {
		return "", false
	}
	return p, true
}
