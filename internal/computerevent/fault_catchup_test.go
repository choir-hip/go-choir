package computerevent

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// The per-attempt page bound is a deadline, not a limit: progress is kept and
// the next append resumes.
func TestFaultCatchUpIsBoundedPerAttemptAndResumes(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(EventReplayPageSize + 1)
	rig.app.catchUpPages = 1
	if _, err := rig.appendOne(rig.app); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("append after a bounded attempt = %v, want projection repair", err)
	}
	if got, want := rig.proj.head.Sequence, uint64(1+EventReplayPageSize); got != want {
		t.Fatalf("local seq=%d, want %d (one page applied)", got, want)
	}
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatalf("the next attempt did not resume: %v", err)
	}
	rig.requireAligned("after resuming", uint64(1+EventReplayPageSize+1+1))
}

// Caught-up events were never delivered to the post-commit observer by a live
// append; each kind is delivered once.
func TestFaultCatchUpDeliversEachKindToTheObserverOnce(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(3)
	var mu sync.Mutex
	fired := map[EventKind]int{}
	rig.app.SetPostCommitObserver(func(kind EventKind) {
		mu.Lock()
		fired[kind]++
		mu.Unlock()
	})
	if _, err := rig.appendOne(rig.app); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := fired[EventArtifactProduced]
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	// One delivery for the three caught-up events (one kind), one for the live append.
	if fired[EventArtifactProduced] != 2 || len(fired) != 1 {
		t.Fatalf("observer deliveries = %v, want {%s: 2}", fired, EventArtifactProduced)
	}
}

// A catch-up never runs inside a reconstruct.
func TestFaultCatchUpRefusedDuringReconstruct(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(2)
	rig.app.reconstructing.Store(true)
	pagesBefore := rig.cas.pageCalls
	if _, err := rig.appendOne(rig.app); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("append during reconstruct = %v, want projection repair", err)
	}
	if rig.cas.pageCalls != pagesBefore || rig.proj.head.Sequence != 1 {
		t.Fatalf("catch-up ran during a reconstruct: pages=%d local=%d", rig.cas.pageCalls-pagesBefore, rig.proj.head.Sequence)
	}
}

// F14c: HoldAppends takes a.mu, so a catch-up cannot overlap it.
func TestFaultCatchUpWaitsForHoldAppends(t *testing.T) {
	rig := newFaultRig(t)
	rig.genesis(rig.app)
	rig.hostAhead(2)
	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_ = rig.app.HoldAppends(func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	go func() { _, err := rig.appendOne(rig.app); done <- err }()
	select {
	case <-done:
		t.Fatal("a catch-up ran inside HoldAppends")
	case <-time.After(100 * time.Millisecond):
	}
	if rig.proj.head.Sequence != 1 {
		t.Fatalf("local head moved inside HoldAppends: %d", rig.proj.head.Sequence)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	rig.requireAligned("after the hold", 4)
}
