package ci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

const (
	StatusUnknown = "UNKNOWN"
	StatusPending = "PENDING"
	StatusPassing = "PASSING"
	StatusFailing = "FAILING"
	StatusSkipped = "SKIPPED"

	VerdictPass    = "PASS"
	VerdictFail    = "FAIL"
	VerdictPending = "PENDING"
	VerdictUnknown = "UNKNOWN"
	VerdictSkipped = "SKIPPED"
)

// Check is one GitHub Actions required check on a pull request.
type Check struct {
	Name   string `json:"name"`
	State  string `json:"state,omitempty"`
	Bucket string `json:"bucket,omitempty"`
}

// Report is remote Actions status. Local tests remain the phase gate.
type Report struct {
	Available bool    `json:"available"`
	Status    string  `json:"status"`
	Detail    string  `json:"detail,omitempty"`
	HeadSHA   string  `json:"head_sha,omitempty"`
	Checks    []Check `json:"checks,omitempty"`
}

type GHFunc func(ctx context.Context, dir string, args ...string) (string, error)

type Watcher struct {
	LookPath func(string) (string, error)
	GH       GHFunc
}

func Default() *Watcher {
	return &Watcher{
		LookPath: exec.LookPath,
		GH: func(ctx context.Context, dir string, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, "gh", args...)
			cmd.Dir = dir
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			out := strings.TrimSpace(stdout.String())
			if err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg == "" {
					msg = err.Error()
				}
				return out, fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
			}
			return out, nil
		},
	}
}

func (w *Watcher) Status(ctx context.Context, root string) Report {
	if w == nil {
		w = Default()
	}
	look := w.LookPath
	if look == nil {
		look = exec.LookPath
	}
	if _, err := look("gh"); err != nil {
		return Report{Available: false, Status: StatusSkipped, Detail: "gh is not available; local workflow continues"}
	}
	gh := w.GH
	if gh == nil {
		gh = Default().GH
	}
	out, err := gh(ctx, root, "run", "list", "--limit", "1", "--json", "status,conclusion,headSha")
	if err != nil {
		return Report{Available: true, Status: StatusUnknown, Detail: err.Error()}
	}
	var runs []struct {
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HeadSHA    string `json:"headSha"`
	}
	if err := json.Unmarshal([]byte(out), &runs); err != nil || len(runs) == 0 {
		return Report{Available: true, Status: StatusUnknown, Detail: "no workflow runs"}
	}
	r := runs[0]
	rep := Report{Available: true, HeadSHA: r.HeadSHA, Detail: r.Status + " " + r.Conclusion}
	switch strings.ToLower(r.Conclusion) {
	case "success":
		rep.Status = StatusPassing
	case "failure", "cancelled", "timed_out":
		rep.Status = StatusFailing
	default:
		if strings.EqualFold(r.Status, "completed") {
			rep.Status = StatusUnknown
		} else {
			rep.Status = StatusPending
		}
	}
	return rep
}

// Verdict maps a P6 report to PASS/FAIL/PENDING/UNKNOWN/SKIPPED.
// UNKNOWN is never treated as PASS.
func (r Report) Verdict() string {
	switch r.Status {
	case StatusPassing, VerdictPass:
		return VerdictPass
	case StatusFailing, VerdictFail:
		return VerdictFail
	case StatusPending:
		return VerdictPending
	case StatusSkipped:
		return VerdictSkipped
	default:
		return VerdictUnknown
	}
}

// PRChecks inspects required checks on a pull request using gh. It does not replace Status().
func (w *Watcher) PRChecks(ctx context.Context, root, pr string) Report {
	if w == nil {
		w = Default()
	}
	look := w.LookPath
	if look == nil {
		look = exec.LookPath
	}
	if _, err := look("gh"); err != nil {
		return Report{Available: false, Status: StatusSkipped, Detail: "gh is not available; local workflow continues"}
	}
	gh := w.GH
	if gh == nil {
		gh = Default().GH
	}
	args := []string{"pr", "checks"}
	if strings.TrimSpace(pr) != "" {
		args = append(args, pr)
	}
	args = append(args, "--json", "name,state,bucket")
	out, err := gh(ctx, root, args...)
	if err != nil {
		return Report{Available: true, Status: StatusUnknown, Detail: err.Error()}
	}
	var raw []Check
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return Report{Available: true, Status: StatusUnknown, Detail: "cannot parse pr checks"}
	}
	if len(raw) == 0 {
		return Report{Available: true, Status: StatusUnknown, Detail: "no required checks"}
	}
	pending, failing, passing := 0, 0, 0
	rep := Report{Available: true, Checks: raw}
	for _, c := range raw {
		switch classifyCheck(c) {
		case VerdictPass:
			passing++
		case VerdictFail:
			failing++
		case VerdictPending:
			pending++
		default:
			rep.Status = StatusUnknown
			rep.Detail = c.Name + " " + checkToken(c)
			return rep
		}
	}
	rep.Detail = fmt.Sprintf("pass=%d fail=%d pending=%d", passing, failing, pending)
	if failing > 0 {
		rep.Status = StatusFailing
		return rep
	}
	if pending > 0 {
		rep.Status = StatusPending
		return rep
	}
	if passing == 0 {
		rep.Status = StatusUnknown
		return rep
	}
	rep.Status = StatusPassing
	return rep
}

func checkToken(c Check) string {
	v := strings.ToUpper(strings.TrimSpace(c.Bucket))
	if v == "" {
		v = strings.ToUpper(strings.TrimSpace(c.State))
	}
	return v
}

func classifyCheck(c Check) string {
	switch checkToken(c) {
	case "PASS", "SUCCESS", "SKIP":
		return VerdictPass
	case "FAIL", "FAILURE", "CANCELLED", "TIMED_OUT":
		return VerdictFail
	case "PENDING", "QUEUED", "IN_PROGRESS":
		return VerdictPending
	default:
		return VerdictUnknown
	}
}

// RequiredVerdict evaluates named required checks. A missing or UNKNOWN check never PASSes.
func (r Report) RequiredVerdict(required []string) (verdict, reason string) {
	if len(required) == 0 {
		v := r.Verdict()
		return v, "required CI checks are " + v
	}
	byName := map[string]Check{}
	for _, c := range r.Checks {
		byName[strings.ToLower(strings.TrimSpace(c.Name))] = c
	}
	pending := false
	for _, name := range required {
		name = strings.TrimSpace(name)
		c, ok := byName[strings.ToLower(name)]
		if !ok {
			return VerdictUnknown, "required check missing: " + name
		}
		switch classifyCheck(c) {
		case VerdictPass:
		case VerdictFail:
			return VerdictFail, "required check failed: " + name
		case VerdictPending:
			pending = true
		default:
			return VerdictUnknown, "required check UNKNOWN: " + name
		}
	}
	if pending {
		return VerdictPending, "required CI checks are " + VerdictPending
	}
	return VerdictPass, "required CI checks are " + VerdictPass
}
