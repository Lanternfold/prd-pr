package preflight

import (
	"context"

	"github.com/lanternfold/prd-pr/internal/redact"
)

// Checker produces readiness reports. Machine checks are cached per instance.
type Checker struct {
	env     Env
	machine *Machine
}

func New(env Env) *Checker {
	return &Checker{env: env}
}

func (c *Checker) Machine() Machine {
	if c.machine == nil {
		m := InspectMachine(c.env)
		c.machine = &m
	}
	return *c.machine
}

// Run inspects machine and project. It does not modify files, Git, or invoke Cursor.
func (c *Checker) Run(ctx context.Context, req Request) *Report {
	if ctx == nil {
		ctx = context.Background()
	}
	r := &Report{
		SchemaVersion: SchemaVersion,
		Timestamp:     c.env.now().UTC().Format("2006-01-02T15:04:05Z"),
		ProjectRoot:   req.ProductRoot,
	}
	mode, err := NormalizeMode(req.Mode)
	if err != nil {
		r.add(Check{
			ID: "execution.mode", Name: "Execution mode", Scope: ScopeMachine,
			Status: StatusError, Blocking: true, Detail: err.Error(),
		})
		r.finalize()
		sanitizeReport(r)
		return r
	}
	worker, err := NormalizeWorker(req.Worker, mode)
	if err != nil {
		r.add(Check{
			ID: "execution.worker", Name: "Worker mechanism", Scope: ScopeMachine,
			Status: StatusError, Blocking: true, Detail: err.Error(),
		})
		r.finalize()
		sanitizeReport(r)
		return r
	}
	r.ExecutionMode = mode
	r.WorkerMechanism = workerMechanism(mode, worker)
	m := c.Machine()
	addMachineChecks(r, m, mode, worker)
	addProjectChecks(ctx, c.env, r, req, m)
	r.finalize()
	sanitizeReport(r)
	return r
}

func workerMechanism(mode, worker string) string {
	switch {
	case worker == WorkerFake:
		return "fake worker"
	case mode == ModeInteractive || worker == WorkerSession:
		return "current Cursor session"
	default:
		return "cursor-agent --print (P4 headless worker)"
	}
}

func sanitizeReport(r *Report) {
	r.NextAction = redact.String(r.NextAction)
	r.ProjectName = redact.String(r.ProjectName)
	for i := range r.Findings {
		r.Findings[i] = redact.String(r.Findings[i])
	}
	for i := range r.Blocking {
		r.Blocking[i] = redact.String(r.Blocking[i])
	}
	for i := range r.Warnings {
		r.Warnings[i] = redact.String(r.Warnings[i])
	}
	for i := range r.Checks {
		r.Checks[i].Detail = redact.String(r.Checks[i].Detail)
		r.Checks[i].Name = redact.String(r.Checks[i].Name)
	}
}
