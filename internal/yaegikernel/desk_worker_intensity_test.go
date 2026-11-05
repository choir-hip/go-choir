package yaegikernel

import (
	"strings"
	"testing"
	"time"
)

// TestCrashLedgerRefusesACrashLoopAndAgesOut: five self-ended workers of one
// role in the window refuse the next spawn naming the cause; another role is
// unaffected; the window moving on lifts the refusal.
func TestCrashLedgerRefusesACrashLoopAndAgesOut(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	l := &crashLedger{now: func() time.Time { return now }, byRole: map[string][]time.Time{}, last: map[string]string{}}
	for i := 0; i < crashLoopMax-1; i++ {
		l.record("research", "exited on its own (signal: killed)")
		now = now.Add(time.Second)
	}
	if err := l.admit("research"); err != nil {
		t.Fatalf("below the bound: %v", err)
	}
	l.record("research", "exited on its own (signal: killed)")
	err := l.admit("research")
	if err == nil || !strings.Contains(err.Error(), "crash loop") || !strings.Contains(err.Error(), "signal: killed") {
		t.Fatalf("at the bound: %v", err)
	}
	if err := l.admit("texture"); err != nil {
		t.Fatalf("another role refused: %v", err)
	}
	now = now.Add(crashLoopWindow)
	if err := l.admit("research"); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}
