package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// Bell requests human attention once. It must not loop.
type Bell struct {
	Notify func(title, body string) error
}

func Default() *Bell {
	return &Bell{Notify: platformNotify}
}

func (b *Bell) Ring(title, body string) error {
	if b != nil && b.Notify != nil {
		return b.Notify(title, body)
	}
	return platformNotify(title, body)
}

func platformNotify(title, body string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	script := fmt.Sprintf(`display notification %q with title %q`, body, title)
	cmd := exec.Command("osascript", "-e", script)
	return cmd.Run()
}

// WaitUntil waits for cond or timeout. It does not retry the request.
func WaitUntil(timeout time.Duration, cond func() bool) bool {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}
