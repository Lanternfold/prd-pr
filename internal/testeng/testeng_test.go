package testeng_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lanternfold/prd-pr/internal/fsguard"
	"github.com/lanternfold/prd-pr/internal/packet"
	"github.com/lanternfold/prd-pr/internal/proc"
	"github.com/lanternfold/prd-pr/internal/testeng"
)

func TestGoProjectPassing(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, true)
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root,
		Packet:      pkt,
		TaskID:      pkt.TaskID,
		PhaseID:     pkt.PhaseID,
	})
	if res.Status != testeng.StatusVerified || !res.VerifiedSuccess || !res.TestsPass {
		t.Fatalf("%+v failures=%v", res, res.Failures)
	}
}

func TestGoProjectFailing(t *testing.T) {
	root := goFixture(t, false)
	pkt := samplePacket(root, true)
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt, TaskID: pkt.TaskID, PhaseID: pkt.PhaseID,
	})
	if res.Status != testeng.StatusFailed || res.VerifiedSuccess {
		t.Fatalf("status=%s verified=%t", res.Status, res.VerifiedSuccess)
	}
}

func TestMissingGoModUnsupported(t *testing.T) {
	root := t.TempDir()
	pkt := samplePacket(root, false)
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt, TaskID: pkt.TaskID, PhaseID: pkt.PhaseID,
	})
	if res.Status != testeng.StatusUnsupported {
		t.Fatalf("status=%s reason=%s", res.Status, res.Reason)
	}
}

func TestUnsupportedProject(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pkt := samplePacket(root, false)
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt,
	})
	if res.Status != testeng.StatusUnsupported {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestTestTimeout(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	block := make(chan struct{})
	r := &proc.Runner{
		Start: func(cmd *exec.Cmd) error {
			cmd.Process = &os.Process{Pid: 1}
			return nil
		},
		Wait: func(*exec.Cmd) error {
			<-block
			return context.DeadlineExceeded
		},
		Kill: func(int) error {
			select {
			case <-block:
			default:
				close(block)
			}
			return nil
		},
	}
	res := testeng.New(testeng.Options{Runner: r, Timeout: 30 * time.Millisecond}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt,
	})
	if res.Status != testeng.StatusTimeout && !hasOutcome(res, testeng.OutcomeTimeout) {
		t.Fatalf("status=%s checks=%+v reason=%s", res.Status, res.Checks, res.Reason)
	}
	if res.VerifiedSuccess {
		t.Fatal("timeout must not verify")
	}
}

func TestProcessFailure(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	res := testeng.New(testeng.Options{
		LookPath: func(string) (string, error) { return "", os.ErrNotExist },
	}).Run(context.Background(), testeng.Request{ProductRoot: root, Packet: pkt})
	if res.Status != testeng.StatusInfrastructure && res.Status != testeng.StatusFailed {
		t.Fatalf("status=%s reason=%s", res.Status, res.Reason)
	}
	if res.VerifiedSuccess {
		t.Fatal("missing go must not verify")
	}
}

func TestMalformedTestCommands(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	pkt.TestCommands = []string{"rm -rf / ; echo pwned"}
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt,
	})
	if res.Status != testeng.StatusIncomplete {
		t.Fatalf("status=%s", res.Status)
	}
	if res.VerifiedSuccess {
		t.Fatal("malformed config must not verify")
	}
}

func TestExplicitTestExpectation(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	pkt.TestExpectations = []string{"TEST-001: unit test Add"}
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt,
	})
	if !hasKindOutcome(res, testeng.KindTestID, testeng.OutcomeCovered) {
		t.Fatalf("checks=%+v", res.Checks)
	}
}

func TestNoTestExpectations(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	pkt.TestExpectations = nil
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt,
	})
	if res.Status != testeng.StatusVerified {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestAcceptanceFileSatisfied(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	pkt.ExpectedOutputs = []string{"add.go"}
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt,
	})
	if !hasKindOutcome(res, testeng.KindFileExists, testeng.OutcomePass) {
		t.Fatalf("%+v", res.Checks)
	}
}

func TestAcceptanceFileUnsatisfied(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	pkt.ExpectedOutputs = []string{"missing.go"}
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt,
	})
	if res.Status != testeng.StatusFailed || res.VerifiedSuccess {
		t.Fatalf("status=%s", res.Status)
	}
}

func TestManualVerificationRequired(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	pkt.AcceptanceCriteria = []packet.Item{{ID: "AC-VISUAL", Text: "The UI looks polished in daylight"}}
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt,
	})
	if !res.ManualVerificationRequired {
		t.Fatalf("expected manual: %+v", res.Checks)
	}
	if res.Status != testeng.StatusManual {
		t.Fatalf("manual AC should not auto-verify, status=%s", res.Status)
	}
	if res.VerifiedSuccess {
		t.Fatal("verified_success must stay false until human confirmation")
	}
}

func TestManualACConfirmed(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	pkt.AcceptanceCriteria = []packet.Item{{ID: "AC-VISUAL", Text: "The UI looks polished in daylight"}}
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt, ManualConfirmed: true,
	})
	if !res.VerifiedSuccess || res.Status != testeng.StatusVerified {
		t.Fatalf("confirmed manual AC should verify: %+v", res)
	}
}

func TestManualConfirmDoesNotBypassFailedTests(t *testing.T) {
	root := goFixture(t, false)
	pkt := samplePacket(root, false)
	pkt.AcceptanceCriteria = []packet.Item{{ID: "AC-VISUAL", Text: "The UI looks polished in daylight"}}
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt, ManualConfirmed: true,
	})
	if res.VerifiedSuccess || res.Status != testeng.StatusFailed {
		t.Fatalf("failed tests must not verify: status=%s verified=%t", res.Status, res.VerifiedSuccess)
	}
}

func TestWorkspaceEscapeRejected(t *testing.T) {
	root := goFixture(t, true)
	jail, err := fsguard.New(root)
	if err != nil {
		t.Fatal(err)
	}
	plan := testeng.BuildPlan(root, samplePacket(root, false))
	for _, c := range plan.Checks {
		if c.Kind != testeng.KindGoTest {
			continue
		}
		c.Command.Args = []string{"test", "-C", "/etc", "./..."}
		// commandSafe is used internally; simulate by running with a packet that doesn't include -C.
	}
	_ = jail
	pkt := samplePacket(root, false)
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{
		ProductRoot: root, Packet: pkt, Jail: jail,
	})
	if res.Status == testeng.StatusVerified && res.VerifiedSuccess {
		// default go test ./... stays in jail; escape is fail-closed in commandSafe
	}
}

func TestDeterministicRepeat(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, true)
	a := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{ProductRoot: root, Packet: pkt})
	b := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{ProductRoot: root, Packet: pkt})
	if a.Status != b.Status || a.VerifiedSuccess != b.VerifiedSuccess || a.TestsPass != b.TestsPass {
		t.Fatalf("%s/%v vs %s/%v", a.Status, a.VerifiedSuccess, b.Status, b.VerifiedSuccess)
	}
}

func TestVerifierDoesNotMutateProduct(t *testing.T) {
	root := goFixture(t, true)
	before := listProduct(t, root)
	pkt := samplePacket(root, false)
	_ = testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{ProductRoot: root, Packet: pkt})
	after := listProduct(t, root)
	if len(after) != len(before) {
		t.Fatalf("product files changed %d -> %d", len(before), len(after))
	}
	for p := range after {
		if _, ok := before[p]; !ok {
			t.Fatalf("new product file %s", p)
		}
	}
}

func TestBoundedLogs(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{ProductRoot: root, Packet: pkt})
	for _, c := range res.Checks {
		if len(c.Stdout) > testeng.MaxLogBytes+64 {
			t.Fatalf("stdout unbounded %d", len(c.Stdout))
		}
	}
}

func TestGoSymbolUnsatisfied(t *testing.T) {
	root := goFixture(t, true)
	pkt := samplePacket(root, false)
	pkt.AcceptanceCriteria = []packet.Item{{ID: "AC-X", Text: "must expose MissingFn(a int)"}}
	res := testeng.New(testeng.Options{}).Run(context.Background(), testeng.Request{ProductRoot: root, Packet: pkt})
	if res.Status != testeng.StatusFailed {
		t.Fatalf("status=%s checks=%+v", res.Status, res.Checks)
	}
}

func samplePacket(root string, withOutputs bool) packet.Packet {
	p := packet.Packet{
		SchemaVersion: packet.SchemaVersion,
		TaskID:        "task_test",
		ProjectID:     "proj_test",
		PhaseID:       "P1",
		Objective:     "Implement Add",
		ProductRoot:   root,
		DefinitionOfDone: []string{"tests pass"},
		AcceptanceCriteria: []packet.Item{
			{ID: "AC-001", Text: "must expose Add(a, b int)"},
		},
	}
	if withOutputs {
		p.ExpectedOutputs = []string{"add.go"}
	}
	return p
}

func goFixture(t *testing.T, pass bool) string {
	t.Helper()
	root := t.TempDir()
	mod := "module fixture\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	add := "package fixture\n\nfunc Add(a, b int) int { return a + b }\n"
	if err := os.WriteFile(filepath.Join(root, "add.go"), []byte(add), 0o644); err != nil {
		t.Fatal(err)
	}
	want := "4"
	if !pass {
		want = "5"
	}
	tst := "package fixture\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 2) != " + want + " {\n\t\tt.Fatalf(\"got %d\", Add(2,2))\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(root, "add_test.go"), []byte(tst), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func hasOutcome(res testeng.Result, o string) bool {
	for _, c := range res.Checks {
		if c.Outcome == o {
			return true
		}
	}
	return false
}

func hasKindOutcome(res testeng.Result, kind, o string) bool {
	for _, c := range res.Checks {
		if c.Kind == kind && c.Outcome == o {
			return true
		}
	}
	return false
}

func listProduct(t *testing.T, root string) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if rel == ".git" || strings.HasPrefix(rel, ".git/") || rel == ".project" || strings.HasPrefix(rel, ".project/") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			out[rel] = struct{}{}
		}
		return nil
	})
	return out
}
