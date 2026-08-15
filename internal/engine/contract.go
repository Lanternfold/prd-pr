package engine

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lanternfold/prd-pr/internal/prd"
)

func (e *Engine) refuseSelfRepo(productRoot string) (bool, Result) {
	if e.opts.AllowSelf || strings.TrimSpace(productRoot) == "" {
		return false, Result{}
	}
	abs, err := filepath.Abs(productRoot)
	if err != nil {
		return false, Result{}
	}
	if _, err := os.Stat(abs); err != nil {
		return false, Result{}
	}
	if isOrchestratorRepo(abs) {
		return true, refused(abs, "refusing to invoke a coding worker against the PRD→PR orchestrator repository")
	}
	return false, Result{}
}

// contractGate is the mandatory Step 0 check. It reads only the PRD path and
// must not create a product directory, Git repo, GitHub resource, or worker run.
func (e *Engine) contractGate(req Request) (blocked bool, res Result) {
	path := strings.TrimSpace(req.PRDPath)
	if path == "" && strings.TrimSpace(req.ProductRoot) != "" {
		path = filepath.Join(req.ProductRoot, "PRD.md")
	}
	if path == "" {
		return true, Result{Execution: Execution{RefusalReason: "PRD path is empty"}}
	}
	report, err := prd.ValidateContractFile(path)
	if err != nil {
		return true, Result{Execution: Execution{RefusalReason: err.Error()}}
	}
	if report.Rejected() {
		return true, Result{
			Execution: Execution{RefusalReason: "PRD contract validation rejected"},
			Contract:  report,
		}
	}
	return false, Result{}
}
