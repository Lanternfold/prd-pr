package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/lanternfold/prd-pr/internal/preflight"
	"github.com/lanternfold/prd-pr/internal/vcs"
)

func runPreflight(args []string, stdout, stderr io.Writer, rt Runtime) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "Usage: prdpr preflight [--json] [--prd FILE] [--mode interactive|headless] [--worker cursor|fake] [directory]")
			fmt.Fprintln(stdout, docsHint("preflight"))
			return exitOK
		}
	}
	opts, err := parsePreflightArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		if strings.Contains(err.Error(), "usage:") || strings.HasPrefix(err.Error(), "unknown flag") {
			return exitUsage
		}
		return exitError
	}
	env := preflightEnv(rt)
	rep := preflight.New(env).Run(nil, preflight.Request{
		ProductRoot: opts.root,
		PRDPath:     opts.prd,
		Mode:        opts.mode,
		Worker:      opts.worker,
	})
	if opts.json {
		if err := preflight.FormatJSON(stdout, rep); err != nil {
			fmt.Fprintf(stderr, "encode preflight JSON: %v\n", err)
			return exitError
		}
	} else if err := preflight.Format(stdout, rep); err != nil {
		fmt.Fprintf(stderr, "format preflight: %v\n", err)
		return exitError
	}
	if rep.Status == preflight.OverallBlocked {
		return exitError
	}
	return exitOK
}

type preflightOpts struct {
	root   string
	prd    string
	json   bool
	mode   string
	worker string
}

func parsePreflightArgs(args []string) (preflightOpts, error) {
	var opts preflightOpts
	var positional []string
	usage := "usage: prdpr preflight [--json] [--prd FILE] [--mode interactive|headless] [--worker cursor|fake] [directory]"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			return preflightOpts{}, fmt.Errorf("%s", usage) // handled in runPreflight
		case a == "--json":
			opts.json = true
		case a == "--prd":
			if i+1 >= len(args) {
				return preflightOpts{}, fmt.Errorf("%s", usage)
			}
			i++
			opts.prd = args[i]
		case strings.HasPrefix(a, "--prd="):
			opts.prd = strings.TrimPrefix(a, "--prd=")
		case a == "--mode":
			if i+1 >= len(args) {
				return preflightOpts{}, fmt.Errorf("%s", usage)
			}
			i++
			opts.mode = args[i]
		case strings.HasPrefix(a, "--mode="):
			opts.mode = strings.TrimPrefix(a, "--mode=")
		case a == "--worker":
			if i+1 >= len(args) {
				return preflightOpts{}, fmt.Errorf("%s", usage)
			}
			i++
			opts.worker = args[i]
		case strings.HasPrefix(a, "--worker="):
			opts.worker = strings.TrimPrefix(a, "--worker=")
		case strings.HasPrefix(a, "-"):
			return preflightOpts{}, fmt.Errorf("unknown flag %q\n%s", a, usage)
		default:
			positional = append(positional, a)
		}
	}
	root, err := resolveProductRoot(positional)
	if err != nil {
		return preflightOpts{}, err
	}
	opts.root = root
	if opts.mode == "" {
		opts.mode = preflight.ModeInteractive
	}
	return opts, nil
}

func preflightEnv(rt Runtime) preflight.Env {
	env := preflight.DefaultEnv()
	if rt.LookPath != nil {
		env.LookPath = rt.LookPath
	}
	if rt.Now != nil {
		env.Now = rt.Now
	}
	if rt.GOOS != "" {
		env.GOOS = rt.GOOS
	}
	if rt.GOARCH != "" {
		env.GOARCH = rt.GOARCH
	}
	if rt.GoVersion != "" {
		env.GoVersion = rt.GoVersion
	}
	if rt.LookupEnv != nil {
		env.LookupEnv = rt.LookupEnv
	}
	env.Git = &vcs.Client{LookPath: env.LookPath}
	return env
}
