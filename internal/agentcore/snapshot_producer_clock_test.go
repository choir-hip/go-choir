package agentcore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Ways the backoff could still loop (seat 1 review of lane/snapshot-loop,
// 2026-11-12), written before the code:
//
//  p. the wait is counted from the time the attempt began, so after a 90
//     minute abort it has already passed and the buffered ticks start four
//     attempts back to back;
//  q. the same at the ceiling: the 3 hour wait is counted from the start.

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

// harnessWithClock makes every Take spend 90 minutes and fail.
func harnessWithClock(t *testing.T) (*producerHarness, *fakeClock) {
	h := newProducerHarness(t)
	clock := &fakeClock{now: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)}
	h.deps.Now = clock.Now
	h.deps.Take = func(ctx context.Context, maxDuration time.Duration) (snapshot.Ref, error) {
		h.takes++
		clock.now = clock.now.Add(90 * time.Minute)
		return snapshot.Ref{}, errors.New("snapshot: witness of the snapshot: snapshot: exceeded the time budget")
	}
	return h, clock
}

func TestProducerBackoffCountsFromTheEndOfTheAttempt(t *testing.T) {
	h, clock := harnessWithClock(t)
	var p snapshotProducer
	for failure := 1; failure <= 6; failure++ {
		before := h.takes
		if outcome, _ := h.run(&p, clock.now); outcome != snapshotFailed || h.takes != before+1 {
			t.Fatalf("failure %d: attempt did not run and fail (takes %d)", failure, h.takes)
		}
		end := clock.now
		wait := snapshotBackoffWait(failure)
		// Every buffered tick inside the wait, counted from the END.
		for at := end; at.Before(end.Add(wait)); at = at.Add(snapshotTickEvery) {
			clock.now = at
			if h.run(&p, clock.now); h.takes != before+1 {
				t.Fatalf("failure %d: an attempt started %s after the previous one ended, wait is %s", failure, at.Sub(end), wait)
			}
		}
		clock.now = end.Add(wait)
	}
}

func TestProducerBackoffAtTheCeilingIsFullAfterTheEnd(t *testing.T) {
	h, clock := harnessWithClock(t)
	var p snapshotProducer
	for i := 0; i < 8; i++ {
		h.run(&p, clock.now)
		clock.now = clock.now.Add(snapshotBackoffCeiling)
	}
	before := h.takes
	h.run(&p, clock.now) // fails, the clock moves 90 minutes
	end := clock.now
	clock.now = end.Add(snapshotBackoffCeiling - time.Minute)
	if h.run(&p, clock.now); h.takes != before+1 {
		t.Fatal("at the ceiling an attempt started before the full wait after the end")
	}
	clock.now = end.Add(snapshotBackoffCeiling)
	if h.run(&p, clock.now); h.takes != before+2 {
		t.Fatal("at the ceiling the next attempt did not start once the wait passed")
	}
}
