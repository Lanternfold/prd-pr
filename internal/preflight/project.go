package preflight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/graph"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

const (
	ModeInteractive = "interactive"
	ModeHeadless    = "headless"
	WorkerCursor    = "cursor"
	WorkerFake      = "fake"
	WorkerSession   = "session"
)

type Request struct {
	ProductRoot string
	PRDPath     string
	Mode        string
	Worker      string
}

func NormalizeMode(mode string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "", ModeInteractive:
		return ModeInteractive, nil
	case ModeHeadless:
		return ModeHeadless, nil
	default:
		return "", fmt.Errorf("unknown execution mode %q (want interactive or headless)", mode)
	}
}

func NormalizeWorker(worker, mode string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(worker)) {
	case "":
		if mode == ModeInteractive {
			return WorkerSession, nil
		}
		return WorkerCursor, nil
	case WorkerCursor, WorkerFake, WorkerSession:
		return strings.ToLower(worker), nil
	default:
		return "", fmt.Errorf("unknown worker %q (want cursor, fake, or session)", worker)
	}
}

func RequireCursorAgent(mode, worker string) bool {
	return mode == ModeHeadless && worker != WorkerFake && worker != WorkerSession
}

func addProjectChecks(ctx context.Context, env Env, r *Report, req Request, machine Machine) {
	root := strings.TrimSpace(req.ProductRoot)
	jail, err := fsguard.New(root)
	if err != nil {
		r.add(Check{
			ID: "project.root", Name: "Product root", Scope: ScopeProject,
			Status: StatusError, Blocking: true, Detail: err.Error(),
		})
		return
	}
	root = jail.Root()
	r.ProjectRoot = root
	if err := readableDir(root); err != nil {
		r.add(Check{
			ID: "project.filesystem", Name: "Filesystem", Scope: ScopeProject,
			Status: StatusError, Blocking: true, Detail: err.Error(),
		})
		return
	}
	r.add(Check{
		ID: "project.root", Name: "Product root", Scope: ScopeProject,
		Status: StatusAvailable, Blocking: false, Detail: "valid",
	})
	r.add(Check{
		ID: "project.filesystem", Name: "Filesystem", Scope: ScopeProject,
		Status: StatusAvailable, Blocking: false, Detail: "accessible",
	})

	prdPath := strings.TrimSpace(req.PRDPath)
	if prdPath == "" {
		prdPath = filepath.Join(root, "PRD.md")
	}
	resolved, err := jail.Resolve(prdPath)
	if err != nil {
		r.add(Check{
			ID: "project.prd", Name: "PRD", Scope: ScopeProject,
			Status: StatusError, Blocking: true, Detail: "PRD path is outside the product root",
		})
		addGit(ctx, env, r, root)
		return
	}
	r.PRDPath = resolved

	doc := addPRD(r, resolved)
	if doc != nil && doc.Metadata.Product != "" {
		r.ProjectName = doc.Metadata.Product
	} else if doc != nil && doc.Metadata.Title != "" {
		r.ProjectName = doc.Metadata.Title
	} else {
		r.ProjectName = filepath.Base(root)
	}

	addGraph(r, doc)
	addGit(ctx, env, r, root)
	addDependencies(env, r, doc, machine)
	addCredentials(env, r, doc)
}

func readableDir(root string) error {
	f, err := os.Open(root)
	if err != nil {
		return err
	}
	return f.Close()
}

func addPRD(r *Report, path string) *prd.Document {
	fi, err := os.Stat(path)
	if err != nil {
		r.add(Check{
			ID: "project.prd", Name: "PRD", Scope: ScopeProject,
			Status: StatusMissing, Blocking: true, Detail: "PRD.md not found",
		})
		return nil
	}
	if fi.IsDir() {
		r.add(Check{
			ID: "project.prd", Name: "PRD", Scope: ScopeProject,
			Status: StatusError, Blocking: true, Detail: "PRD path is a directory",
		})
		return nil
	}
	doc, err := prd.ParseFile(path)
	if err != nil {
		r.add(Check{
			ID: "project.prd", Name: "PRD", Scope: ScopeProject,
			Status: StatusError, Blocking: true, Detail: "cannot read PRD",
		})
		return nil
	}
	if doc.HasErrors() {
		_, errs := doc.Counts()
		r.add(Check{
			ID: "project.prd", Name: "PRD", Scope: ScopeProject,
			Status: StatusError, Blocking: true,
			Detail: "parses with validation errors (" + strconv.Itoa(errs) + ")",
		})
		return doc
	}
	r.add(Check{
		ID: "project.prd", Name: "PRD", Scope: ScopeProject,
		Status: StatusAvailable, Blocking: false,
		Detail: "available; " + strconv.Itoa(len(doc.Phases)) + " phases",
	})
	return doc
}

func addGraph(r *Report, doc *prd.Document) {
	if doc == nil {
		return
	}
	g := graph.FromDocument(doc)
	if g.HasErrors() {
		detail := "invalid"
		for _, d := range g.Diagnostics {
			if d.Severity == graph.SevError {
				detail = d.Message
				break
			}
		}
		r.add(Check{
			ID: "project.graph", Name: "Graph", Scope: ScopeProject,
			Status: StatusError, Blocking: true, Detail: detail,
		})
		return
	}
	r.add(Check{
		ID: "project.graph", Name: "Graph", Scope: ScopeProject,
		Status: StatusAvailable, Blocking: false,
		Detail: "valid; " + strconv.Itoa(len(g.Nodes)) + " nodes",
	})
}

func addGit(ctx context.Context, env Env, r *Report, root string) {
	obs := env.git().Observe(ctx, root)
	info := &RepositoryInfo{
		State:      obs.State,
		Toplevel:   obs.Toplevel,
		Branch:     obs.Branch,
		HeadSHA:    obs.HeadSHA,
		Dirty:      obs.Dirty,
		DirtyPaths: obs.DirtyPaths,
	}
	r.Repository = info

	switch obs.State {
	case vcs.StateNotInstalled:
		r.add(Check{
			ID: "project.git", Name: "Git repository", Scope: ScopeProject,
			Status: StatusBlocking, Blocking: true, Detail: "Git is not installed",
		})
	case vcs.StateNotRepository:
		r.add(Check{
			ID: "project.git", Name: "Git repository", Scope: ScopeProject,
			Status: StatusBlocking, Blocking: true, Detail: "not a repository",
		})
	case vcs.StateMismatchRoot:
		r.add(Check{
			ID: "project.git", Name: "Git repository", Scope: ScopeProject,
			Status: StatusError, Blocking: true, Detail: "repository root is not the product root",
		})
	case vcs.StateNoCommits:
		r.add(Check{
			ID: "project.git", Name: "Git repository", Scope: ScopeProject,
			Status: StatusBlocking, Blocking: true, Detail: "no commits",
		})
	case vcs.StateDirty:
		r.add(Check{
			ID: "project.git", Name: "Git repository", Scope: ScopeProject,
			Status: StatusWarning, Blocking: false, Detail: "dirty (P4 will refuse Cursor writes until the tree is clean)",
		})
	default:
		r.add(Check{
			ID: "project.git", Name: "Git repository", Scope: ScopeProject,
			Status: StatusAvailable, Blocking: false, Detail: "repository; clean",
		})
	}
}

func addDependencies(env Env, r *Report, doc *prd.Document, machine Machine) {
	if doc == nil || len(doc.Dependencies) == 0 {
		r.add(Check{
			ID: "project.dependencies", Name: "Dependencies", Scope: ScopeProject,
			Status: StatusWarning, Blocking: false, Detail: "Dependency declarations not specified",
		})
		return
	}
	for i, dep := range doc.Dependencies {
		id := "project.dependency." + strconv.Itoa(i)
		class := strings.ToUpper(strings.TrimSpace(dep.Class))
		optional := class == "OPTIONAL"
		bin, known := binaryForDep(dep.Name)
		if !known {
			r.add(Check{
				ID: id, Name: "Dependency " + dep.Name, Scope: ScopeProject,
				Status: StatusWarning, Blocking: false,
				Detail: "not a known tool; not executed",
			})
			continue
		}
		present := false
		switch bin {
		case "git":
			present = machine.GitAvailable
		case "go":
			present = machine.GoAvailable
		case "gh":
			present = machine.GitHubCLI
		case "cursor-agent", "agent":
			present = machine.CursorAgent
		default:
			_, present = env.hasBinary(bin)
		}
		if present {
			r.add(Check{
				ID: id, Name: "Dependency " + dep.Name, Scope: ScopeProject,
				Status: StatusAvailable, Blocking: false, Detail: "available",
			})
			continue
		}
		if optional {
			r.add(Check{
				ID: id, Name: "Dependency " + dep.Name, Scope: ScopeProject,
				Status: StatusOptional, Blocking: false, Detail: "missing optional dependency",
			})
			continue
		}
		r.add(Check{
			ID: id, Name: "Dependency " + dep.Name, Scope: ScopeProject,
			Status: StatusBlocking, Blocking: true, Detail: "missing required dependency",
		})
	}
}

func addCredentials(env Env, r *Report, doc *prd.Document) {
	if doc == nil || len(doc.Credentials) == 0 {
		r.add(Check{
			ID: "project.credentials", Name: "Credentials", Scope: ScopeProject,
			Status: StatusOptional, Blocking: false, Detail: "Credential declarations not specified",
		})
		return
	}
	lookup := env.lookupEnv()
	anyPresent := false
	for i, cred := range doc.Credentials {
		id := "project.credential." + strconv.Itoa(i)
		key := envVarForCredential(cred.Name)
		if key == "" {
			r.add(Check{
				ID: id, Name: "Credential " + cred.Name, Scope: ScopeProject,
				Status: StatusWarning, Blocking: false,
				Detail: "declared; cannot map to a local metadata check",
			})
			continue
		}
		val, ok := lookup(key)
		if !ok || strings.TrimSpace(val) == "" {
			r.add(Check{
				ID: id, Name: "Credential " + key, Scope: ScopeProject,
				Status: StatusBlocking, Blocking: true, Detail: "MISSING",
			})
			continue
		}
		anyPresent = true
		r.add(Check{
			ID: id, Name: "Credential " + key, Scope: ScopeProject,
			Status: StatusWarning, Blocking: false,
			Detail: "configured credentials not verified",
		})
	}
	if anyPresent {
		r.add(Check{
			ID: "project.credentials", Name: "Credentials", Scope: ScopeProject,
			Status: StatusWarning, Blocking: false,
			Detail: "configured credentials not verified",
		})
	}
}
