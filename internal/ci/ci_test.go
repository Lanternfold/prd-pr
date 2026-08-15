package ci

import (
	"context"
	"os"
	"strings"
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

func TestPRChecksEmptyIsUnknown(t *testing.T) {
	w := &Watcher{
		LookPath: func(string) (string, error) { return "/bin/gh", nil },
		GH: func(context.Context, string, ...string) (string, error) {
			return `[]`, nil
		},
	}
	rep := w.PRChecks(context.Background(), ".", "1")
	if rep.Status != StatusUnknown || rep.Verdict() == VerdictPass {
		t.Fatalf("%+v", rep)
	}
}

func TestRequiredVerdictMissingFailedPending(t *testing.T) {
	pass := Report{Status: StatusPassing, Checks: []Check{{Name: "test", Bucket: "pass"}}}
	v, reason := pass.RequiredVerdict([]string{"ci"})
	if v != VerdictUnknown || !strings.Contains(reason, "missing") {
		t.Fatalf("%s %s", v, reason)
	}
	fail := Report{Checks: []Check{{Name: "ci", Bucket: "fail"}}}
	v, reason = fail.RequiredVerdict([]string{"ci"})
	if v != VerdictFail || !strings.Contains(reason, "failed") {
		t.Fatalf("%s %s", v, reason)
	}
	pend := Report{Checks: []Check{{Name: "ci", Bucket: "pending"}}}
	v, _ = pend.RequiredVerdict([]string{"ci"})
	if v != VerdictPending {
		t.Fatalf("%s", v)
	}
}
