package llm

import (
	"context"
	"strings"
	"testing"
)

func TestNoneRefusesCompletion(t *testing.T) {
	_, err := None{}.Complete(context.Background(), Request{Prompt: "hi"})
	if err == nil || !strings.Contains(err.Error(), "no model") {
		t.Fatalf("err=%v", err)
	}
}

func TestStaticReturnsText(t *testing.T) {
	res, err := Static{Text: "ok", Model: "cheap", In: 1, Out: 2}.Complete(context.Background(), Request{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "ok" || res.InputTokens != 1 || res.OutputTokens != 2 {
		t.Fatalf("%+v", res)
	}
}
