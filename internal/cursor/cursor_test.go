package cursor_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lanternfold/prd-pr/internal/cursor"
	"github.com/lanternfold/prd-pr/internal/packet"
)

func TestCLIRefusesEditorBinary(t *testing.T) {
	c := &cursor.CLI{
		LookPath: func(file string) (string, error) {
			if file == "cursor" {
				return "/usr/local/bin/cursor", nil
			}
			return "", errors.New("not found")
		},
	}
	_, err := c.ResolveBinary()
	if err == nil || !strings.Contains(err.Error(), "editor") {
		t.Fatalf("err=%v", err)
	}
	res := c.Run(context.Background(), cursor.Request{ProductRoot: t.TempDir(), PacketRel: "p.json"})
	if res.Invoked {
		t.Fatal("must not invoke editor cursor")
	}
	if res.VerifiedSuccess || res.WorkerClaimedSuccess {
		t.Fatalf("%+v", res)
	}
}

func TestCLITrueBinaryNeverVerifies(t *testing.T) {
	c := &cursor.CLI{Bin: "true"}
	res := c.Run(context.Background(), cursor.Request{
		ProductRoot: t.TempDir(),
		Packet:      packet.Packet{TaskID: "t"},
		PacketRel:   ".project/packets/t.json",
	})
	if !res.Invoked || !res.WorkerClaimedSuccess || !res.ClaimedDone {
		t.Fatalf("%+v", res)
	}
	if res.VerifiedSuccess {
		t.Fatal("verified_success must stay false")
	}
	if !strings.Contains(res.CLIMechanism, "--print") {
		t.Fatalf("mechanism=%s", res.CLIMechanism)
	}
}

func TestFakeClaimIsNotVerification(t *testing.T) {
	dir := t.TempDir()
	f := cursor.Fake{ClaimSuccess: true, WriteRel: "out.txt", WriteBody: "x\n"}
	res := f.Run(context.Background(), cursor.Request{ProductRoot: dir})
	if !res.WorkerClaimedSuccess || res.VerifiedSuccess {
		t.Fatalf("%+v", res)
	}
	if res.Stdout != "Done." {
		t.Fatalf("stdout=%q", res.Stdout)
	}
}

func TestFakeTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	res := cursor.Fake{ClaimSuccess: true, Hang: time.Second}.Run(ctx, cursor.Request{ProductRoot: t.TempDir()})
	if !res.TimedOut || res.VerifiedSuccess {
		t.Fatalf("%+v", res)
	}
}
