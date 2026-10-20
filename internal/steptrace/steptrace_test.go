package steptrace

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// A step that has begun and not ended is visible with its start time and a
// growing elapsed time; that is the whole point: a hang writes no log line.
func TestInFlightStepShowsStartAndElapsedUntilItEnds(t *testing.T) {
	tr := New()
	clock := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	tr.now = func() time.Time { return clock }

	end := tr.Begin("run:a", "freeze.extract_diff")
	clock = clock.Add(90 * time.Second)
	snap := tr.Snapshot()
	if len(snap.InFlight) != 1 || snap.InFlight[0].Scope != "run:a" || snap.InFlight[0].Step != "freeze.extract_diff" ||
		snap.InFlight[0].ElapsedMS != 90_000 || snap.InFlight[0].StartedAt != "2026-10-20T12:00:00Z" || len(snap.Recent) != 0 {
		t.Fatalf("in-flight snapshot = %+v", snap)
	}

	clock = clock.Add(10 * time.Second)
	end(nil)
	end(errors.New("second end is ignored"))
	snap = tr.Snapshot()
	if len(snap.InFlight) != 0 || len(snap.Recent) != 1 || snap.Recent[0].DurationMS != 100_000 || snap.Recent[0].Error != "" {
		t.Fatalf("finished snapshot = %+v", snap)
	}
}

func TestFinishedStepKeepsAnErrorBoundedInLength(t *testing.T) {
	tr := New()
	tr.Begin("run:a", "complete.commit")(errors.New(strings.Repeat("x", 1000)))
	if got := tr.Snapshot().Recent[0].Error; len(got) != errorLimit {
		t.Fatalf("error length = %d, want %d", len(got), errorLimit)
	}
}

func TestInFlightIsOldestFirstAndRecentIsRingBounded(t *testing.T) {
	tr := New()
	clock := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	tr.now = func() time.Time { return clock }
	tr.Begin("run:a", "first")
	clock = clock.Add(time.Second)
	tr.Begin("run:a", "second")
	snap := tr.Snapshot()
	if len(snap.InFlight) != 2 || snap.InFlight[0].Step != "first" || snap.InFlight[1].Step != "second" {
		t.Fatalf("order = %+v", snap.InFlight)
	}
	for i := 0; i < recentLimit+10; i++ {
		tr.Begin("run:b", "loop")(nil)
	}
	if got := len(tr.Snapshot().Recent); got != recentLimit {
		t.Fatalf("recent ring = %d, want %d", got, recentLimit)
	}
}

// A nil tracker is a no-op so callers never check whether tracing is wired.
func TestNilTrackerIsInert(t *testing.T) {
	var tr *Tracker
	tr.Begin("run:a", "anything")(errors.New("ignored"))
	if snap := tr.Snapshot(); snap.InFlight == nil || snap.Recent == nil || len(snap.InFlight)+len(snap.Recent) != 0 {
		t.Fatalf("nil snapshot = %+v", snap)
	}
}

func TestConcurrentStepsAreRaceFree(t *testing.T) {
	tr := New()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			end := tr.Begin("run:c", "step")
			_ = tr.Snapshot()
			end(nil)
		}()
	}
	wg.Wait()
	if snap := tr.Snapshot(); len(snap.InFlight) != 0 {
		t.Fatalf("leaked in-flight steps: %+v", snap.InFlight)
	}
}
