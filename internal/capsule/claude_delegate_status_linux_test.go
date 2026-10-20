//go:build linux

package capsule

import (
	"testing"
	"time"
)

// docs/problems/engineering-completion-path-cluster-2026-10-20.md failure 9:
// the observability route reads the delegate's status, and a capsule
// teardown holds e.mu for its whole run. The status must answer while e.mu
// is held, and still report the source SetClaudeDelegateSource gave it.
func TestClaudeDelegateStatusDoesNotWaitForTheExecutorLock(t *testing.T) {
	e := &Executor{}
	e.SetClaudeDelegateSource(t.TempDir(), "/nonexistent/claude")
	want := e.ClaudeDelegateStatus()

	e.mu.Lock()
	defer e.mu.Unlock()
	got := make(chan ClaudeDelegateStatus, 1)
	go func() { got <- e.ClaudeDelegateStatus() }()
	select {
	case status := <-got:
		if status.Available != want.Available || status.Reason != want.Reason {
			t.Fatalf("status under a held lock = %+v, want %+v", status, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ClaudeDelegateStatus waited for the executor lock")
	}
}
