package engine

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lanternfold/prd-pr/internal/human"
	"github.com/lanternfold/prd-pr/internal/llm"
	"github.com/lanternfold/prd-pr/internal/prd"
	"github.com/lanternfold/prd-pr/internal/state"
)

type completenessFinding struct {
	Severity string `json:"severity"`
	Topic    string `json:"topic"`
	Problem  string `json:"problem"`
	Question string `json:"question"`
}

type completenessPayload struct {
	Findings []completenessFinding `json:"findings"`
}

// reviewCompleteness is advisory LLM review after deterministic contract validation.
// BLOCKING_QUESTION stops for a human. The LLM must not mutate the PRD.
func (e *Engine) reviewCompleteness(ctx context.Context, path string, doc *prd.Document) (human.Request, error) {
	adapter := e.opts.LLM
	if adapter == nil {
		return human.Request{}, nil
	}
	if _, ok := adapter.(llm.None); ok {
		return human.Request{}, nil
	}
	prompt := completenessPrompt(path, doc)
	resp, err := adapter.Complete(ctx, llm.Request{
		Role:      llm.RoleCompleteness,
		System:    "You review product PRDs for unanswered product decisions. Return JSON only. Do not invent product answers. Do not include secrets.",
		Prompt:    prompt,
		MaxTokens: 800,
	})
	if err != nil {
		return human.Request{}, nil
	}
	payload, ok := parseCompleteness(resp.Text)
	if !ok {
		return human.Request{}, nil
	}
	for _, f := range payload.Findings {
		sev := strings.ToUpper(strings.TrimSpace(f.Severity))
		if sev == "BLOCKING_QUESTION" {
			q := strings.TrimSpace(f.Question)
			if q == "" {
				q = strings.TrimSpace(f.Problem)
			}
			if q == "" {
				q = "A material product decision is missing from the PRD."
			}
			return human.Request{
				Kind:    human.KindProductQuestion,
				Reason:  firstNonEmpty(f.Topic, "product_completeness"),
				Needed:  q + " Incorporate the answer into the PRD, then re-run validation. The engine will not edit the PRD from this review.",
				Urgency: human.UrgencyHigh,
			}, nil
		}
	}
	return human.Request{}, nil
}

func completenessPrompt(path string, doc *prd.Document) string {
	var b strings.Builder
	b.WriteString("PRD path: " + path + "\n")
	if doc != nil {
		b.WriteString("Product: " + doc.Metadata.Product + "\n")
		b.WriteString("Identify BLOCKING_QUESTION, WARNING, or INFO findings as JSON {\"findings\":[{\"severity\",\"topic\",\"problem\",\"question\"}]}.\n")
		b.WriteString("BLOCKING_QUESTION only for material unanswered product decisions. Do not answer them.\n")
	}
	return b.String()
}

func parseCompleteness(text string) (completenessPayload, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return completenessPayload{}, false
	}
	if i := strings.Index(text, "{"); i >= 0 {
		if j := strings.LastIndex(text, "}"); j > i {
			text = text[i : j+1]
		}
	}
	var p completenessPayload
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return completenessPayload{}, false
	}
	return p, true
}

func (e *Engine) noteCompleteness(g *state.Guard, st state.State, skipped bool, blocking bool) {
	if g == nil {
		return
	}
	_ = g.AppendEvent(state.Event{
		Kind:    state.KindResult,
		Name:    state.EventCompletenessReviewed,
		RunID:   st.CurrentRunID,
		Payload: state.Payload(map[string]any{"skipped": skipped, "blocking": blocking, "authoritative": false}),
	})
}
