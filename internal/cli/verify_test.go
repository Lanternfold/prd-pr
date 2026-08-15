package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/testeng"
)

func TestVerifyCLIPassAndFail(t *testing.T) {
	pass := verifyFixture(t, true)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "prepare", "--phase", "P1", pass}, stdout, stderr, verifyRuntime())
	if code != exitOK {
		t.Fatalf("prepare %d %s %s", code, stderr, stdout)
	}
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	code = Main([]string{"prdpr", "verify", "--json", pass}, stdout, stderr, verifyRuntime())
	if code != exitOK {
		t.Fatalf("verify pass %d stderr=%s stdout=%s", code, stderr, stdout)
	}
	var res testeng.Result
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.VerifiedSuccess {
		t.Fatalf("%+v", res)
	}

	fail := verifyFixture(t, false)
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	if code := Main([]string{"prdpr", "prepare", "--phase", "P1", fail}, stdout, stderr, verifyRuntime()); code != exitOK {
		t.Fatalf("prepare fail %d %s", code, stderr)
	}
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	code = Main([]string{"prdpr", "verify", fail}, stdout, stderr, verifyRuntime())
	if code != exitError {
		t.Fatalf("verify fail exit %d stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout.String(), "verified_success: false") {
		t.Fatalf("stdout=%s", stdout)
	}
}

func TestVerifyUnknownFlag(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	code := Main([]string{"prdpr", "verify", "--bogus"}, stdout, stderr, verifyRuntime())
	if code != exitUsage {
		t.Fatalf("exit %d stderr=%s", code, stderr)
	}
}

func verifyRuntime() Runtime {
	rt := testRuntime()
	look := rt.LookPath
	rt.LookPath = func(file string) (string, error) {
		if file == "go" {
			if p, err := exec.LookPath("go"); err == nil {
				return p, nil
			}
		}
		return look(file)
	}
	return rt
}

func verifyFixture(t *testing.T, pass bool) string {
	t.Helper()
	root := t.TempDir()
	want := "4"
	if !pass {
		want = "5"
	}
	files := map[string]string{
		"go.mod":      "module fixture\n\ngo 1.22\n",
		"add.go":      "package fixture\n\nfunc Add(a, b int) int { return a + b }\n",
		"add_test.go": "package fixture\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 2) != " + want + " { t.Fatal(Add(2,2)) }\n}\n",
	}
	prd, err := os.ReadFile(filepath.Join("..", "prd", "testdata", "prd", "auto_verify.md"))
	if err != nil {
		t.Fatal(err)
	}
	files["PRD.md"] = string(prd)
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "--template=")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	git("add", ".")
	git("commit", "-m", "init")
	return root
}
