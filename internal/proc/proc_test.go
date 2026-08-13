package proc_test

import (
	"context"
	"testing"
	"time"

	"github.com/lanternfold/prd-pr/internal/proc"
)

func TestRunSuccess(t *testing.T) {
	res := (&proc.Runner{}).Run(context.Background(), proc.Spec{
		Name: "true",
	})
	if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
		t.Fatalf("%+v", res)
	}
}

func TestTimeoutKillsProcessGroup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	res := (&proc.Runner{}).Run(ctx, proc.Spec{
		Name: "sleep",
		Args: []string{"8"},
	})
	if !res.TimedOut {
		t.Fatalf("expected timeout, got %+v", res)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("timeout did not kill promptly: %s", time.Since(start))
	}
}
