package apprun

import (
	"context"
	"testing"
)

func TestProcStarterNoneSkipped(t *testing.T) {
	rep := ProcStarter{}.Start(context.Background(), t.TempDir(), Def{Kind: KindNone})
	if !rep.Skipped || rep.Ready {
		t.Fatalf("%+v", rep)
	}
}

func TestFakeStarter(t *testing.T) {
	f := &Fake{Ready: true, URL: "http://127.0.0.1:9"}
	rep := f.Start(context.Background(), t.TempDir(), Def{Kind: KindGoRun})
	if !rep.Ready || f.Calls != 1 {
		t.Fatalf("%+v", rep)
	}
}
