package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/lanternfold/prd-pr/internal/prd"
)

func runValidatePRD(args []string, stdout, stderr io.Writer) int {
	jsonOut := false
	var path string
	for _, a := range args {
		switch a {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "Usage: prdpr validate-prd [--json] <PRD.md>")
			fmt.Fprintln(stdout, docsHint("validate-prd"))
			return exitOK
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(stderr, "unknown flag %q\nUsage: prdpr validate-prd [--json] <PRD.md>\n", a)
				return exitUsage
			}
			if path != "" {
				fmt.Fprintln(stderr, "Usage: prdpr validate-prd [--json] <PRD.md>")
				return exitUsage
			}
			path = a
		}
	}
	if path == "" {
		fmt.Fprintln(stderr, "Usage: prdpr validate-prd [--json] <PRD.md>")
		return exitUsage
	}

	res, err := prd.ValidateContractFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return exitError
	}
	if jsonOut {
		if err := prd.FormatContractJSON(stdout, res); err != nil {
			fmt.Fprintf(stderr, "encode validate-prd JSON: %v\n", err)
			return exitError
		}
	} else if err := prd.FormatContract(stdout, res); err != nil {
		fmt.Fprintf(stderr, "format validate-prd: %v\n", err)
		return exitError
	}
	if res.Rejected() {
		return exitError
	}
	return exitOK
}
