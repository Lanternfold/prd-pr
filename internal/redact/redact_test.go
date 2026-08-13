package redact_test

import (
	"strings"
	"testing"

	"github.com/lanternfold/prd-pr/internal/redact"
)

func TestStringMasksSecrets(t *testing.T) {
	in := "CURSOR_API_KEY=cursor_secretvalue Authorization: Bearer abcdefghijklmnop token=supersecret ghp_abcdefghijklmnopqrstuvwxyz1234"
	out := redact.String(in)
	for _, leak := range []string{"cursor_secretvalue", "abcdefghijklmnop", "supersecret", "abcdefghijklmnopqrstuvwxyz1234"} {
		if strings.Contains(out, leak) {
			t.Fatalf("leaked %q in %q", leak, out)
		}
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("expected mask, got %q", out)
	}
}
