package testeng

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/lanternfold/prd-pr/internal/packet"
)

var (
	reUnsafeShell = regexp.MustCompile(`[;&|$\x60<>]|&&|\|\|`)
	reFilePath    = regexp.MustCompile(`^[A-Za-z0-9_./-]+\.[A-Za-z0-9]+$`)
	reGoSymbol    = regexp.MustCompile(`(?i)(?:expose|implement)\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	reTestsPass   = regexp.MustCompile(`(?i)tests?\s+pass`)
)

const kindGo = "go"

// BuildPlan is deterministic. It does not invent tests and does not execute PRD shell strings.
func BuildPlan(root string, pkt packet.Packet) Plan {
	p := Plan{
		ProjectRoot: root,
		TaskID:      pkt.TaskID,
		PhaseID:     pkt.PhaseID,
	}
	if hasGoMod(root) {
		p.Kind = kindGo
		p.Checks = append(p.Checks, Check{
			ID:       "go.test",
			Kind:     KindGoTest,
			Name:     "go test ./...",
			Required: true,
			Command:  Command{Program: "go", Args: []string{"test", "./..."}},
		})
	} else {
		p.Kind = "unsupported"
	}

	for i, t := range pkt.TestExpectations {
		name := strings.TrimSpace(t)
		if name == "" {
			continue
		}
		p.Checks = append(p.Checks, Check{
			ID:       "test.expect." + strconv.Itoa(i+1),
			Kind:     KindTestID,
			Name:     name,
			Required: p.Kind == kindGo,
		})
	}

	for _, cmd := range pkt.TestCommands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		detail := "PRD/packet test_commands are not executed"
		if reUnsafeShell.MatchString(cmd) {
			detail = "malformed test_commands (shell metacharacters); not executed"
		}
		p.Checks = append(p.Checks, Check{
			ID:       "packet.test_command",
			Kind:     KindMalformed,
			Name:     "packet test_commands",
			Required: true,
			Detail:   detail,
		})
		break
	}

	for i, out := range pkt.ExpectedOutputs {
		out = strings.TrimSpace(out)
		if !reFilePath.MatchString(out) {
			continue
		}
		p.Checks = append(p.Checks, Check{
			ID:       "file." + strconv.Itoa(i+1),
			Kind:     KindFileExists,
			Name:     "file exists: " + out,
			Required: true,
			Detail:   out,
		})
	}

	for i, ac := range pkt.AcceptanceCriteria {
		text := strings.TrimSpace(ac.Text)
		if text == "" {
			text = strings.TrimSpace(ac.ID)
		}
		if text == "" {
			continue
		}
		id := "ac." + strconv.Itoa(i+1)
		if ac.ID != "" {
			id = "ac." + ac.ID
		}
		p.Checks = append(p.Checks, acceptanceCheck(id, "AC", text, p.Kind))
	}

	for i, d := range pkt.DefinitionOfDone {
		text := strings.TrimSpace(d)
		if text == "" {
			continue
		}
		p.Checks = append(p.Checks, acceptanceCheck("dod."+strconv.Itoa(i+1), "DoD", text, p.Kind))
	}
	return p
}

func acceptanceCheck(id, prefix, text, projectKind string) Check {
	if reTestsPass.MatchString(text) {
		return Check{ID: id, Kind: KindDoD, Name: prefix + ": tests pass", Required: true, Detail: text}
	}
	if m := reGoSymbol.FindStringSubmatch(text); m != nil && projectKind == kindGo {
		return Check{ID: id, Kind: KindGoSymbol, Name: prefix + ": Go symbol " + m[1], Required: true, Detail: m[1]}
	}
	return Check{ID: id, Kind: KindAcceptance, Name: prefix + ": " + clip(text, 80), Required: true, Detail: text}
}

func hasGoMod(root string) bool {
	_, err := os.Stat(filepath.Join(root, "go.mod"))
	return err == nil
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return strings.TrimRightFunc(s[:n], unicode.IsSpace) + "…"
}
