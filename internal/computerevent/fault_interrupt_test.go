package computerevent

import (
	"context"
	"testing"
	"time"
)

// Shutdown does not wait for detached work: a catch-up parked in a page fetch
// is cancelled by InterruptDetachedWork, changes nothing, and the next append
// resumes.
func TestRepairInterruptStopsACatchUpParkedInAPageFetch(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(3)
	parked := make(chan struct{})
	park := true
	rig.cas.onEventsPage = func(ctx context.Context, _ uint64) error {
		if !park {
			return nil
		}
		close(parked)
		<-ctx.Done()
		return ctx.Err()
	}
	done := make(chan error, 1)
	go func() { _, err := rig.appendOne(rig.app); done <- err }()
	select {
	case <-parked:
	case <-time.After(5 * time.Second):
		t.Fatal("the catch-up never fetched events")
	}
	rig.app.InterruptDetachedWork()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("append reported success although its catch-up was interrupted")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the interrupt did not stop the catch-up")
	}
	if rig.proj.head.Sequence != 1 || len(rig.proj.prepared) != 0 {
		t.Fatalf("state after the interrupt: local=%d prepared=%d, want 1/0", rig.proj.head.Sequence, len(rig.proj.prepared))
	}
	park = false
	// No cooldown follows an interrupt: the very next append recovers.
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("the append after the interrupt did not recover: %v", err)
	}
	rig.requireAligned("after the interrupt", 5)
}
