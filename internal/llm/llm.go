package llm

import (
	"context"
	"fmt"
	"time"
)

const (
	RoleReview    = "review"
	RoleDiagnosis = "diagnosis"
	RolePlanning  = "planning"
	RoleLearning  = "learning"
	RoleRepair    = "repair"
)

// Request is a single completion. It must not contain secrets.
type Request struct {
	Role      string
	System    string
	Prompt    string
	MaxTokens int
	Model     string
}

// Response is adapter output. Cost may be zero when the provider does not report it.
type Response struct {
	Text         string
	Provider     string
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Duration     time.Duration
}

// Adapter talks to one configured provider. Core code must not import vendor SDKs.
type Adapter interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
}

// None never calls a network. Using it is a programming error if a model was selected.
type None struct{}

func (None) Name() string { return "none" }

func (None) Complete(context.Context, Request) (Response, error) {
	return Response{}, fmt.Errorf("llm adapter none: no model was selected")
}

// Fail is a test adapter that always fails.
type Fail struct {
	Provider string
	Err      error
}

func (f Fail) Name() string {
	if f.Provider == "" {
		return "fail"
	}
	return f.Provider
}

func (f Fail) Complete(context.Context, Request) (Response, error) {
	err := f.Err
	if err == nil {
		err = fmt.Errorf("llm adapter failed")
	}
	return Response{Provider: f.Name()}, err
}

// Static returns a fixed response. Tests use it; default go test must not call paid APIs.
type Static struct {
	Provider string
	Model    string
	Text     string
	In, Out  int
	Cost     float64
}

func (s Static) Name() string {
	if s.Provider == "" {
		return "static"
	}
	return s.Provider
}

func (s Static) Complete(_ context.Context, req Request) (Response, error) {
	model := s.Model
	if model == "" {
		model = req.Model
	}
	return Response{
		Text:         s.Text,
		Provider:     s.Name(),
		Model:        model,
		InputTokens:  s.In,
		OutputTokens: s.Out,
		CostUSD:      s.Cost,
	}, nil
}
