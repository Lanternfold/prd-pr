package notify

import (
	"testing"
	"time"
)

func TestWaitUntilTimeout(t *testing.T) {
	ok := WaitUntil(30*time.Millisecond, func() bool { return false })
	if ok {
		t.Fatal("expected timeout")
	}
}

func TestWaitUntilReady(t *testing.T) {
	ok := WaitUntil(time.Second, func() bool { return true })
	if !ok {
		t.Fatal("expected ready")
	}
}

func TestBellRecords(t *testing.T) {
	var n int
	b := &Bell{Notify: func(title, body string) error {
		n++
		if title == "" || body == "" {
			t.Fatalf("%q %q", title, body)
		}
		return nil
	}}
	if err := b.Ring("PRD→PR", "waiting"); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("n=%d", n)
	}
}
