package plugin_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func pluginRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}

func TestV0AdapterResolvesPATHOnly(t *testing.T) {
	root := pluginRoot(t)
	for _, name := range []string{
		filepath.Join("commands", "prdpr.md"),
		filepath.Join("skills", "prdpr", "SKILL.md"),
		"README.md",
	} {
		body := readFile(t, filepath.Join(root, name))
		if !strings.Contains(body, "PATH") {
			t.Errorf("%s: must require prdpr on PATH", name)
		}
		if strings.Contains(body, "first that works") || strings.Contains(body, "if that file is executable") {
			t.Errorf("%s: must not keep the dist/prdpr resolution fallback", name)
		}
	}
	skill := readFile(t, filepath.Join(root, "skills", "prdpr", "SKILL.md"))
	if !strings.Contains(skill, "USER_GUIDE.md") {
		t.Fatal("skill must point unavailable-prdpr users at USER_GUIDE.md")
	}
	if !strings.Contains(skill, "Do not search") && !strings.Contains(skill, "Do **not**") {
		t.Fatal("skill must refuse searching/building a checkout")
	}
}

func TestV0AdapterForbidsNestedWorker(t *testing.T) {
	root := pluginRoot(t)
	cmd := readFile(t, filepath.Join(root, "commands", "prdpr.md"))
	skill := readFile(t, filepath.Join(root, "skills", "prdpr", "SKILL.md"))
	for name, body := range map[string]string{"commands/prdpr.md": cmd, "skills/prdpr/SKILL.md": skill} {
		if !strings.Contains(body, "prdpr run") || !strings.Contains(body, "prdpr phase") {
			t.Errorf("%s: must explicitly forbid prdpr run and prdpr phase", name)
		}
		if !strings.Contains(body, "cursor-agent") {
			t.Errorf("%s: must explicitly forbid cursor-agent", name)
		}
	}
}

func TestV0ValidatePRDIsOptional(t *testing.T) {
	cmd := readFile(t, filepath.Join(pluginRoot(t), "commands", "prdpr.md"))
	skill := readFile(t, filepath.Join(pluginRoot(t), "skills", "prdpr", "SKILL.md"))
	for name, body := range map[string]string{"commands/prdpr.md": cmd, "skills/prdpr/SKILL.md": skill} {
		if !strings.Contains(strings.ToLower(body), "optional") {
			t.Errorf("%s: validate-prd / inspect / preflight optionality must be documented", name)
		}
		if strings.Contains(body, "mandatory contract gate") {
			t.Errorf("%s: must not call standalone validate-prd mandatory", name)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
