package testeng

import (
	"context"
	"strings"
	"time"

	"github.com/lanternfold/prd-pr/internal/fsguard"
)

// Run executes the verification plan. It does not diagnose or repair.
func (e *Engine) Run(ctx context.Context, req Request) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	now := e.opts.now().UTC().Format(time.RFC3339Nano)
	res := Result{
		SchemaVersion: SchemaVersion,
		RunID:         req.RunID,
		TaskID:        req.TaskID,
		PhaseID:       req.PhaseID,
		Timestamp:     now,
		BaselineSHA:   req.BaselineSHA,
	}
	if req.TaskID == "" {
		req.TaskID = req.Packet.TaskID
		res.TaskID = req.TaskID
	}
	if req.PhaseID == "" {
		req.PhaseID = req.Packet.PhaseID
		res.PhaseID = req.PhaseID
	}

	root := strings.TrimSpace(req.ProductRoot)
	jail := req.Jail
	if jail == nil {
		j, err := fsguard.New(root)
		if err != nil {
			res.Status = StatusInfrastructure
			res.Reason = err.Error()
			res.Failures = []string{err.Error()}
			return res
		}
		jail = j
		root = j.Root()
	}

	if err := req.Packet.Validate(); err != nil {
		res.Status = StatusIncomplete
		res.Reason = "packet is incomplete: " + err.Error()
		res.Failures = []string{res.Reason}
		return res
	}

	plan := BuildPlan(root, req.Packet)
	res.Plan = plan

	if req.BaselineSHA != "" {
		cs, err := e.opts.git().ChangesFrom(ctx, root, req.BaselineSHA, jail)
		if err != nil {
			res.Status = StatusInfrastructure
			res.Reason = "git inspection: " + err.Error()
			res.Failures = []string{res.Reason}
			res.Checks = plan.Checks
			return res
		}
		res.ChangedFiles = cs.All
		res.UntrackedFiles = cs.Untracked
		res.DeletedFiles = cs.Deleted
		res.HeadSHA = cs.HeadSHA
	} else if sn, err := e.opts.git().Inspect(ctx, root); err == nil {
		res.HeadSHA = sn.HeadSHA
	}

	if plan.Kind != kindGo {
		res.Status = StatusUnsupported
		res.Reason = "unsupported project: no go.mod (V1 verifies Go projects only)"
		res.Failures = []string{res.Reason}
		res.Checks = annotateUnsupported(plan.Checks)
		return res
	}

	var goOutcome Check
	checks := make([]Check, 0, len(plan.Checks))
	for _, ch := range plan.Checks {
		switch ch.Kind {
		case KindGoTest:
			if err := commandSafe(jail, ch.Command); err != nil {
				ch.Outcome = OutcomeFail
				ch.Detail = err.Error()
				res.Failures = append(res.Failures, ch.ID+": "+ch.Detail)
				checks = append(checks, ch)
				continue
			}
			goOutcome = e.runGoTest(ctx, jail)
			goOutcome.ID = ch.ID
			goOutcome.Required = ch.Required
			checks = append(checks, goOutcome)
		case KindTestID:
			if goOutcome.ID == "" {
				ch.Outcome = OutcomeSkip
				ch.Detail = "project tests have not run"
			} else if goOutcome.Outcome == OutcomePass {
				ch.Outcome = OutcomeCovered
				ch.Detail = "mapped to go test ./...; not executed as a PRD shell command"
			} else if goOutcome.Outcome == OutcomeTimeout {
				ch.Outcome = OutcomeTimeout
				ch.Detail = "project tests timed out"
			} else {
				ch.Outcome = OutcomeFail
				ch.Detail = "project tests did not pass"
			}
			checks = append(checks, ch)
		case KindMalformed:
			ch.Outcome = OutcomeFail
			if ch.Detail == "" {
				ch.Detail = "packet test_commands are not executed"
			}
			res.Failures = append(res.Failures, ch.ID+": "+ch.Detail)
			checks = append(checks, ch)
		case KindFileExists:
			ok, err := fileExistsInJail(jail, ch.Detail)
			if err != nil {
				ch.Outcome = OutcomeFail
				ch.Detail = err.Error()
				res.Failures = append(res.Failures, ch.ID+": "+ch.Detail)
			} else if ok {
				ch.Outcome = OutcomePass
			} else {
				ch.Outcome = OutcomeFail
				ch.Detail = "missing file: " + ch.Detail
				res.Failures = append(res.Failures, ch.ID+": "+ch.Detail)
			}
			checks = append(checks, ch)
		case KindGoSymbol:
			ok, err := goSymbolPresent(root, ch.Detail, jail)
			if err != nil {
				ch.Outcome = OutcomeError
				ch.Detail = err.Error()
				res.Failures = append(res.Failures, ch.ID+": "+ch.Detail)
			} else if ok {
				ch.Outcome = OutcomePass
			} else {
				ch.Outcome = OutcomeFail
				ch.Detail = "func " + ch.Detail + " not found"
				res.Failures = append(res.Failures, ch.ID+": "+ch.Detail)
			}
			checks = append(checks, ch)
		case KindDoD:
			if goOutcome.Outcome == OutcomePass {
				ch.Outcome = OutcomePass
			} else if goOutcome.Outcome == OutcomeTimeout {
				ch.Outcome = OutcomeTimeout
				ch.Detail = "tests timed out"
			} else {
				ch.Outcome = OutcomeFail
				ch.Detail = "tests did not pass"
				res.Failures = append(res.Failures, ch.ID+": "+ch.Detail)
			}
			checks = append(checks, ch)
		case KindAcceptance:
			ch.Outcome = OutcomeManual
			res.ManualVerificationRequired = true
			checks = append(checks, ch)
		default:
			ch.Outcome = OutcomeSkip
			checks = append(checks, ch)
		}
	}
	res.Checks = checks
	return finalize(res, goOutcome, req.ManualConfirmed)
}

func annotateUnsupported(checks []Check) []Check {
	out := make([]Check, 0, len(checks))
	for _, c := range checks {
		if c.Kind == KindGoTest {
			c.Outcome = OutcomeSkip
			c.Detail = "unsupported project"
		} else if c.Kind == KindTestID || c.Kind == KindDoD {
			c.Outcome = OutcomeSkip
			c.Detail = "unsupported project"
		} else if c.Kind == KindAcceptance {
			c.Outcome = OutcomeManual
		}
		out = append(out, c)
	}
	return out
}

func finalize(res Result, goTest Check, manualConfirmed bool) Result {
	testsRan := goTest.ID != ""
	res.TestsPass = testsRan && goTest.Outcome == OutcomePass

	verifiableFail := false
	verifiablePending := false
	manual := false
	timeout := false
	infra := false
	malformed := false

	for _, c := range res.Checks {
		if c.Outcome == OutcomeManual {
			manual = true
			continue
		}
		if c.Kind == KindMalformed && c.Outcome == OutcomeFail {
			malformed = true
			continue
		}
		if !c.Required && (c.Outcome == OutcomeSkip || c.Outcome == OutcomeCovered || c.Outcome == OutcomeManual) {
			continue
		}
		switch c.Outcome {
		case OutcomePass, OutcomeCovered:
		case OutcomeTimeout:
			timeout = true
			verifiableFail = true
		case OutcomeError:
			infra = true
			verifiableFail = true
		case OutcomeFail:
			verifiableFail = true
		case OutcomeSkip:
			if c.Required {
				verifiablePending = true
			}
		}
	}
	res.ManualVerificationRequired = manual
	res.AllVerifiableAcceptanceCriteriaPass = !verifiableFail && !verifiablePending && !malformed

	switch {
	case timeout:
		res.Status = StatusTimeout
		res.Reason = "test timed out"
	case infra && !res.TestsPass:
		res.Status = StatusInfrastructure
		if res.Reason == "" {
			res.Reason = "test process failure"
		}
	case malformed:
		res.Status = StatusIncomplete
		res.Reason = "malformed test configuration"
	case verifiableFail:
		res.Status = StatusFailed
		if res.Reason == "" {
			res.Reason = "verification failed"
		}
	case verifiablePending:
		res.Status = StatusIncomplete
		res.Reason = "verification incomplete"
	case manual && res.TestsPass:
		res.Status = StatusManual
		res.Reason = "tests passed; required acceptance criteria need human confirmation"
		res.VerifiedSuccess = false
		if manualConfirmed {
			res.Status = StatusVerified
			res.Reason = "tests passed; manual acceptance criteria confirmed"
			res.VerifiedSuccess = true
		}
	case res.TestsPass && res.AllVerifiableAcceptanceCriteriaPass:
		res.Status = StatusVerified
		res.VerifiedSuccess = true
	default:
		res.Status = StatusIncomplete
		res.Reason = "verification incomplete"
	}
	return res
}
