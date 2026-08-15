package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/prd"
)

func TestSelectTypeGoLibrary(t *testing.T) {
	doc, err := prd.ParseFile(filepath.Join("..", "prd", "testdata", "prd", "contract", "pass_complete.md"))
	if err != nil {
		t.Fatal(err)
	}
	sel := SelectType(doc)
	if sel.Ambiguous || sel.Type != TypeGoLibrary || sel.Category != "Tools" {
		t.Fatalf("%+v", sel)
	}
	if sel.Slug != "notes-library" {
		t.Fatalf("slug=%s", sel.Slug)
	}
}

func TestSelectTypeAmbiguousPlatform(t *testing.T) {
	doc, err := prd.ParseFile(filepath.Join("..", "prd", "testdata", "prd", "contract", "reject_ambiguous_platform.md"))
	if err != nil {
		t.Fatal(err)
	}
	sel := SelectType(doc)
	if !sel.Ambiguous {
		t.Fatalf("%+v", sel)
	}
}

func TestPlaceCreatesAndIsIdempotent(t *testing.T) {
	srcDir := t.TempDir()
	prdPath := filepath.Join(srcDir, "PRD.md")
	raw, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "contract", "pass_complete.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prdPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "Tools", "notes-library")
	sel := Selection{Type: TypeGoLibrary, Category: "Tools", Slug: "notes-library"}
	r1, err := Place(prdPath, dest, sel, nil)
	if err != nil || r1.Conflict || !r1.Created || !r1.Copied {
		t.Fatalf("%+v %v", r1, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "PRD.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "go.mod")); err != nil {
		t.Fatal(err)
	}
	r2, err := Place(prdPath, dest, sel, nil)
	if err != nil || r2.Conflict || r2.Created || r2.Copied {
		t.Fatalf("second place %+v %v", r2, err)
	}
}

func TestPlaceRejectsUnrelatedDirectory(t *testing.T) {
	srcDir := t.TempDir()
	prdPath := filepath.Join(srcDir, "PRD.md")
	if err := os.WriteFile(prdPath, []byte("# PRD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "secrets.txt"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Place(prdPath, dest, Selection{Type: TypeGoLibrary, Slug: "x"}, nil)
	if err != nil || !r.Conflict || !strings.Contains(r.Reason, "unrelated") {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestCursorRulesPreservesUserFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".cursor", "rules", "prdpr-engineering.mdc")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(root, ".cursor", "rules", "team.mdc")
	if err := os.WriteFile(sibling, []byte("# team\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrote, _, err := EnsureCursorRules(root, Selection{Type: TypeGoLibrary})
	if err != nil || wrote {
		t.Fatalf("wrote=%t err=%v", wrote, err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "# mine\n" {
		t.Fatalf("%q", b)
	}
	sb, _ := os.ReadFile(sibling)
	if string(sb) != "# team\n" {
		t.Fatalf("sibling overwritten: %q", sb)
	}
}

func TestCursorRulesIdempotentGenerate(t *testing.T) {
	root := t.TempDir()
	w1, _, err := EnsureCursorRules(root, Selection{Type: TypeGoLibrary})
	if err != nil || !w1 {
		t.Fatal(err)
	}
	w2, path, err := EnsureCursorRules(root, Selection{Type: TypeGoLibrary})
	if err != nil || w2 {
		t.Fatalf("second write=%t err=%v", w2, err)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), generatedMarker) {
		t.Fatalf("%s %v", b, err)
	}
}

func TestCursorRulesWritesExecutionPolicy(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	wrote, path, err := EnsureCursorRules(root, Selection{Type: TypeGoLibrary})
	if err != nil || !wrote {
		t.Fatalf("wrote=%t err=%v", wrote, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, want := range []string{
		generatedMarker,
		"PRD→PR execution policy",
		"implementation actor, not the workflow scheduler",
		"Implement the current PRD→PR packet faithfully",
		"do not grant terminal permissions",
		"Cursor's configured Run Mode",
		"Do not repeatedly ask the user to manually execute routine build, test, or install commands",
		"Do not independently skip phases or dependencies",
		"Do not invoke another Cursor session",
		"Do not invoke `prdpr run`",
		"Git commit, push, pull-request creation, merge, and repository lifecycle operations are engine-owned",
		"Do not independently commit, push, create pull requests, merge",
		"Do not access credentials or secrets",
		"Do not modify files outside this assigned project workspace",
		"PRD→PR human-interaction mechanism",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	for _, forbid := range []string{
		"permissions.json",
		"grants terminal permission",
		"allowlist",
		"this rule enables unsandboxed",
	} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(forbid)) {
			t.Fatalf("policy must not claim permissions via %q", forbid)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".cursor", "permissions.json")); err == nil {
		t.Fatal("must not write project permissions.json")
	}
	if entries, err := os.ReadDir(home); err != nil {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("must not modify global Cursor config under HOME: %v", names(entries))
	}
}

func TestCursorRulesRefreshesGeneratedPolicy(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".cursor", "rules", "prdpr-engineering.mdc")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := "---\ndescription: old\nalwaysApply: true\n---\n\n<!-- " + generatedMarker + " project-type=go_library -->\n\n# Engineering constraints\n\nstale\n"
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	wrote, _, err := EnsureCursorRules(root, Selection{Type: TypeGoLibrary})
	if err != nil || !wrote {
		t.Fatalf("wrote=%t err=%v", wrote, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "PRD→PR execution policy") {
		t.Fatalf("generated file was not refreshed:\n%s", b)
	}
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}
