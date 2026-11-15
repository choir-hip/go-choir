package agentcore

import (
	"errors"
	"testing"
	"time"
)

// Ways the producer could loop on a store it cannot snapshot (the paper,
// 2026-11-12: a full attempt every 30 minutes, none completing, 6 GiB an hour
// of chunks the host keeps), written before the code:
//
//  k. an attempt that fails is due again at the very next tick, so every
//     tick uploads the store again;
//  l. the wait after a failure never grows, so a store that cannot be
//     snapshotted is retried at the tick forever;
//  m. the wait grows without end, and a computer whose tail passes the
//     recovery bound waits days for its next attempt;
//  n. a success does not reset the wait, so the next failure after a long
//     good run waits at the ceiling;
//  o. the default time budget is below the measured witness (22 to 35
//     minutes on the paper's 21 GiB store) so no attempt can complete.

// k: the tick after a failure does not start another attempt.
func TestProducerDoesNotRetryAFailedAttemptAtTheNextTick(t *testing.T) {
	h := newProducerHarness(t)
	h.takeErr = errors.New("snapshot: witness of the snapshot: snapshot: exceeded the time budget")
	var p snapshotProducer
	t0 := time.Now()
	if outcome, _ := h.run(&p, t0); outcome != snapshotFailed || h.takes != 1 {
		t.Fatalf("first attempt: outcome=%s takes=%d", outcome, h.takes)
	}
	if outcome, detail := h.run(&p, t0.Add(snapshotTickEvery)); h.takes != 1 {
		t.Fatalf("the tick after a failure started another attempt (takes=%d, outcome=%s, %q)", h.takes, outcome, detail)
	}
}

// l, m: the wait doubles from a floor above the tick and stops at the ceiling.
func TestProducerBackoffGrowsThenStopsAtTheCeiling(t *testing.T) {
	h := newProducerHarness(t)
	h.takeErr = errors.New("platform unreachable")
	var p snapshotProducer
	now := time.Now()
	h.deps.Now = func() time.Time { return now }
	var gaps []time.Duration
	lastTake := time.Time{}
	for end := now.Add(48 * time.Hour); now.Before(end); now = now.Add(time.Minute) {
		before := h.takes
		h.run(&p, now)
		if h.takes > before {
			if !lastTake.IsZero() {
				gaps = append(gaps, now.Sub(lastTake))
			}
			lastTake = now
		}
	}
	if len(gaps) < 6 {
		t.Fatalf("only %d retries in 48h: %v", len(gaps), gaps)
	}
	if gaps[0] <= snapshotTickEvery {
		t.Fatalf("the first wait %s is no longer than the tick %s", gaps[0], snapshotTickEvery)
	}
	for i := 1; i < 4; i++ {
		if gaps[i] < gaps[i-1]*2-time.Minute {
			t.Fatalf("waits do not double: %v", gaps)
		}
	}
	ceiling := gaps[len(gaps)-1]
	if ceiling > 4*time.Hour || ceiling < 2*time.Hour {
		t.Fatalf("the ceiling is %s (waits %v), want a few hours", ceiling, gaps)
	}
	if gaps[len(gaps)-2] != ceiling {
		t.Fatalf("the wait kept moving at the end: %v", gaps)
	}
}

// n: a success resets the wait.
func TestProducerBackoffResetsOnSuccess(t *testing.T) {
	h := newProducerHarness(t)
	h.set.TailEvents = 100
	h.head = 1000
	var p snapshotProducer
	now := time.Now()
	h.deps.Now = func() time.Time { return now }
	h.takeErr = errors.New("platform unreachable")
	// Three failures, each at its retry time, to grow the wait.
	step := func() {
		for i := 0; i < 24*60; i++ {
			before := h.takes
			h.run(&p, now)
			now = now.Add(time.Minute)
			if h.takes > before {
				return
			}
		}
		t.Fatal("no retry within a day")
	}
	step()
	step()
	step()
	h.takeErr = nil
	step() // succeeds
	h.head += 100
	h.takeErr = errors.New("platform unreachable")
	step() // fails once, at the floor
	start := now
	for i := 0; i < 24*60; i++ {
		before := h.takes
		h.run(&p, now)
		now = now.Add(time.Minute)
		if h.takes > before {
			break
		}
	}
	if wait := now.Sub(start); wait > 45*time.Minute {
		t.Fatalf("after a success the first wait is %s, want the floor", wait)
	}
}

// o: the default budget covers the measured witness.
func TestSnapshotDefaultBudgetCoversTheMeasuredWitness(t *testing.T) {
	if s := snapshotSettingsFromEnv(func(string) string { return "" }); s.MaxDuration < 90*time.Minute {
		t.Fatalf("default budget %s is below the 35 minute witness plus the stream", s.MaxDuration)
	}
}
