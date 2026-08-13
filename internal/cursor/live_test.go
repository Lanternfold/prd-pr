package cursor_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/packet"
)

func TestRealCursorAgentIfAvailable(t *testing.T) {
	if os.Getenv("PRDPR_CURSOR_LIVE") != "1" {
		t.Skip("set PRDPR_CURSOR_LIVE=1 to run the controlled Cursor Agent CLI test")
	}
	cli := &cursor.CLI{}
	bin, err := cli.ResolveBinary()
	if err != nil {
		t.Skip(err.Error())
	}
	out, err := exec.Command(bin, "--help").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--print") {
		t.Skipf("%s does not advertise --print (not a non-interactive agent CLI)", bin)
	}

	root := t.TempDir()
	if filepath.Base(filepath.Dir(root)) == "prd-pr" || strings.Contains(root, "prd-pr") && strings.Contains(root, "Studio/Tools/prd-pr") {
		t.Fatal("refusing to use the orchestrator repo as the live worker target")
	}
	run(t, root, "git", "init", "--template=")
	run(t, root, "git", "config", "user.email", "test@example.com")
	run(t, root, "git", "config", "user.name", "Test")
	hello := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(hello, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", "hello.txt")
	run(t, root, "git", "commit", "-m", "init")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	res := cli.Run(ctx, cursor.Request{
		ProductRoot: root,
		PacketRel:   "hello.txt",
		Timeout:     2 * time.Minute,
		Packet: packet.Packet{
			TaskID:      "live",
			ProjectID:   "live",
			PhaseID:     "P1",
			Objective:   "Append the line live-ok to hello.txt and do not modify other files.",
			ProductRoot: root,
		},
	})
	if res.VerifiedSuccess {
		t.Fatal("verified_success must stay false even after a live Cursor run")
	}
	if res.TimedOut {
		t.Fatalf("live Cursor timed out: %s", res.Transcript)
	}
	if !res.Invoked {
		t.Fatalf("expected invoke, refusal=%s", res.RefusalReason)
	}
}

func run(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}
