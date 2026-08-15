package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	generatedMarker = "generated-by: prdpr"
	rulesRel        = ".cursor/rules/prdpr-engineering.mdc"
)

// EnsureCursorRules writes project-local Cursor rules when missing.
// It does not overwrite user-authored rules, does not duplicate the PRD,
// and does not manage Cursor global command permissions or Run Mode.
func EnsureCursorRules(root string, sel Selection) (written bool, path string, err error) {
	path = filepath.Join(root, rulesRel)
	existing, err := os.ReadFile(path)
	if err == nil {
		if !strings.Contains(string(existing), generatedMarker) {
			return false, path, nil
		}
		want := cursorRulesBody(sel)
		if string(existing) == want {
			return false, path, nil
		}
		// Refresh only engine-generated rules with the same marker.
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			return false, path, err
		}
		return true, path, nil
	}
	if !os.IsNotExist(err) {
		return false, path, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, path, err
	}
	if err := os.WriteFile(path, []byte(cursorRulesBody(sel)), 0o644); err != nil {
		return false, path, err
	}
	return true, path, nil
}

func cursorRulesBody(sel Selection) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: Project-specific engineering and PRD→PR implementation-actor guidance (not orchestration).\n")
	b.WriteString("alwaysApply: true\n")
	b.WriteString("---\n\n")
	b.WriteString("<!-- " + generatedMarker + " project-type=" + sel.Type + " -->\n\n")
	b.WriteString("# Engineering constraints\n\n")
	b.WriteString("The Go PRD→PR engine is the source of truth for orchestration, Git safety, verification, and GitHub governance.\n")
	b.WriteString("These rules cover this product's engineering and how the Cursor session should act as the implementation worker. Do not duplicate PRD.md. Do not encode orchestration, repair limits, or repository rulesets here.\n\n")
	writeExecutionPolicy(&b)
	switch sel.Type {
	case TypeGoLibrary, TypeGoCLI:
		b.WriteString("## Architecture\n\n")
		b.WriteString("- This is a local Go module. Keep the public API small and tested.\n")
		b.WriteString("- Do not add network services, databases, or extra binaries unless the PRD requires them.\n\n")
		b.WriteString("## Testing\n\n")
		b.WriteString("- Unit tests live beside the code. The independent verifier runs `go test ./...`.\n")
		b.WriteString("- Acceptance criteria must remain objectively checkable.\n\n")
		b.WriteString("## Structure\n\n")
		b.WriteString("- Do not commit `.project/` or secret-shaped files (`.env`, `*.pem`, `credentials.json`).\n")
		b.WriteString("- Do not rewrite `go.mod` module path without an explicit product reason.\n\n")
		b.WriteString("## Security\n\n")
		b.WriteString("- No secrets in source, tests, or logs.\n")
		if sel.Type == TypeGoCLI {
			b.WriteString("\n## Runtime\n\n- The CLI entrypoint is the module main package. Prefer `go run .` for local smoke.\n")
		}
	case TypeIOS:
		b.WriteString("## Architecture\n\n- Native iOS application. Follow the PRD's platform constraints.\n\n## Testing\n\n- Prefer XCTest. Do not invent a second platform.\n")
	case TypeAndroid:
		b.WriteString("## Architecture\n\n- Native Android application. Follow the PRD's platform constraints.\n")
	case TypeWeb:
		b.WriteString("## Architecture\n\n- Web application. Keep client/server boundaries as the PRD states them.\n")
	default:
		b.WriteString("## Architecture\n\n- Follow the PRD. Do not change product type.\n\n## Testing\n\n- Add tests the engine can run, or mark acceptance as explicitly manual.\n")
	}
	b.WriteString("\n")
	return b.String()
}

func writeExecutionPolicy(b *strings.Builder) {
	b.WriteString("## PRD→PR execution policy\n\n")
	b.WriteString("This project is orchestrated by PRD→PR. The Cursor session is the implementation actor, not the workflow scheduler. Implement the current PRD→PR packet faithfully. Defer workflow decisions to the PRD→PR engine. Git commit, push, pull-request creation, merge, and repository lifecycle operations are engine-owned.\n\n")
	b.WriteString("These project rules are behavioral instructions only. They do not grant terminal permissions, do not configure Cursor Run Mode or Approvals & Execution, and do not replace Cursor's global command-permission system. Routine project-local development commands may be executed autonomously only when Cursor's configured Run Mode already permits them.\n\n")
	b.WriteString("- Do not repeatedly ask the user to manually execute routine build, test, or install commands.\n")
	b.WriteString("- Do not ask the user for approval for ordinary development decisions that the packet already specifies.\n")
	b.WriteString("- Do not independently skip phases or dependencies.\n")
	b.WriteString("- Do not invoke another Cursor session.\n")
	b.WriteString("- Do not invoke `prdpr run` or another orchestration path from this interactive Cursor implementation flow.\n")
	b.WriteString("- Do not independently commit, push, create pull requests, merge, or perform other Git/GitHub repository lifecycle operations unless the engine packet explicitly instructs that step.\n")
	b.WriteString("- Do not access credentials or secrets unless the packet requires it.\n")
	b.WriteString("- Do not modify files outside this assigned project workspace.\n")
	b.WriteString("- If a genuinely unresolved product decision exists, return it to the PRD→PR human-interaction mechanism rather than inventing a requirement.\n\n")
}
