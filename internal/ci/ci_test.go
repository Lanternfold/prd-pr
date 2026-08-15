package ci

import (
	"context"
	"os"
	"testing"
)

func TestMissingGHSkips(t *testing.T) {
	w := &Watcher{LookPath: func(string) (string, error) { return "", os.ErrNotExist }}
	rep := w.Status(context.Background(), t.TempDir())
	if rep.Available || rep.Status != StatusSkipped {
		t.Fatalf("%+v", rep)
	}
}

func TestParsePassingRun(t *testing.T) {
	w := &Watcher{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(context.Context, string, ...string) (string, error) {
			return `[{"status":"completed","conclusion":"success","headSha":"abc"}]`, nil
		},
	}
	rep := w.Status(context.Background(), ".")
	if !rep.Available || rep.Status != StatusPassing || rep.HeadSHA != "abc" {
		t.Fatalf("%+v", rep)
	}
}

func TestPRChecksPassFailPendingUnknown(t *testing.T) {
	cases := []struct {
		name, json, want, verdict string
	}{
		{"pass", `[{"name":"test","state":"SUCCESS","bucket":"pass"}]`, StatusPassing, VerdictPass},
		{"fail", `[{"name":"test","state":"FAILURE","bucket":"fail"}]`, StatusFailing, VerdictFail},
		{"pending", `[{"name":"test","state":"PENDING","bucket":"pending"}]`, StatusPending, VerdictPending},
		{"unknown", `[{"name":"test","state":"WEIRD","bucket":"weird"}]`, StatusUnknown, VerdictUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := &Watcher{
				LookPath: func(string) (string, error) { return "/bin/gh", nil },
				GH: func(context.Context, string, ...string) (string, error) {
					return tc.json, nil
				},
			}
			rep := w.PRChecks(context.Background(), ".", "1")
			if rep.Status != tc.want || rep.Verdict() != tc.verdict {
				t.Fatalf("%+v want %s/%s", rep, tc.want, tc.verdict)
			}
			if tc.verdict == VerdictUnknown && rep.Verdict() == VerdictPass {
				t.Fatal("UNKNOWN must not be PASS")
			}
		})
	}
}

func TestVerdictUnknownIsNotPass(t *testing.T) {
	r := Report{Status: StatusUnknown}
	if r.Verdict() == VerdictPass {
		t.Fatal("unknown as pass")
	}
}
