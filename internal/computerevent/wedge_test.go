package computerevent

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

// Wedge reporting (docs/problems/paper-wedged-on-post-cas-projection-failure-2026-11-02.md,
// next probes 1 and 2). Ways it can fail, written before the code:
//  1. A computer whose appends keep refusing reports no wedge.
//  2. The wedge survives a successful append (a stale health degradation).
//  3. A single transient refusal flips health (threshold ignored), or a long
//     one never does.
//  4. The wedge's first cause is overwritten by later refusals, or the last
//     error leaks payload content.
//  5. Every refusal logs, so the console rotates the first cause away.

type wedgeClock struct{ now time.Time }

func (c *wedgeClock) Now() time.Time { return c.now }

func wedgedAppender(t *testing.T) (*ComputerEventAppender, *memoryProjection, *wedgeClock) {
	t.Helper()
	appender, projection, _ := newRecoveryTestAppender(t, &memoryProjection{failFinalizeOnce: true})
	clock := &wedgeClock{now: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	appender.wedgeNow = clock.Now
	event := testEvent(t, nil, EventGenesisImported)
	event.ResultingEffectiveCommitment = testDigestA
	input := TransitionInput{TargetStateCommitment: testDigestA}
	bindTestRequest(t, &event, input, nil)
	if _, err := appender.Append(context.Background(), event, input, nil); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("setup append error = %v, want projection repair", err)
	}
	return appender, projection, clock
}

func refuseOnce(t *testing.T, appender *ComputerEventAppender) {
	t.Helper()
	if _, err := appender.AppendNew(context.Background(), testEvent(t, nil, EventTrajectoryStarted), TransitionInput{}, nil); !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("append error = %v, want projection repair", err)
	}
}

func TestWedgeReportedAfterRefusals(t *testing.T) {
	appender, _, clock := wedgedAppender(t)
	appender.reconstructing.Store(true) // in-place recovery refuses: the wedge persists
	clock.now = clock.now.Add(10 * time.Second)
	refuseOnce(t, appender)
	w := appender.WedgeStatus()
	if !w.Active || w.Consecutive != 2 {
		t.Fatalf("wedge = %+v, want active with 2 refusals", w)
	}
	if !w.Since.Equal(time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("since = %v, want the first refusal's time", w.Since)
	}
	if w.PlatformSeq != 1 || w.LocalSeq != 0 {
		t.Fatalf("heads local=%d platform=%d, want 0 and 1", w.LocalSeq, w.PlatformSeq)
	}
	if w.FirstError == "" || w.LastError == "" || !strings.Contains(w.FirstError, "finalize") {
		t.Fatalf("errors first=%q last=%q, want the classified first cause kept", w.FirstError, w.LastError)
	}
	if strings.Contains(w.FirstError, "injected finalize crash") {
		t.Fatalf("first error leaks the raw finalize error: %q", w.FirstError)
	}
}

func TestWedgeClearedOnSuccessfulAppend(t *testing.T) {
	appender, _, _ := wedgedAppender(t)
	if w := appender.WedgeStatus(); !w.Active {
		t.Fatalf("setup produced no wedge: %+v", w)
	}
	// In-place recovery finalizes the orphan; the next append succeeds.
	if _, err := appender.AppendNew(context.Background(), testEvent(t, nil, EventTrajectoryStarted), TransitionInput{}, nil); err != nil {
		t.Fatalf("append after recovery: %v", err)
	}
	if w := appender.WedgeStatus(); w.Active || w.Consecutive != 0 || w.Persistent {
		t.Fatalf("wedge not cleared by a successful append: %+v", w)
	}
}

func TestWedgePersistentOnlyPastThreshold(t *testing.T) {
	appender, _, clock := wedgedAppender(t)
	appender.reconstructing.Store(true)
	// Many refusals, but young: not persistent.
	for i := 0; i < wedgePersistentRefusals; i++ {
		clock.now = clock.now.Add(time.Second)
		refuseOnce(t, appender)
	}
	if w := appender.WedgeStatus(); !w.Active || w.Persistent {
		t.Fatalf("young wedge persistent: %+v", w)
	}
	// Old enough and enough refusals: persistent.
	clock.now = clock.now.Add(wedgePersistentAfter)
	if w := appender.WedgeStatus(); !w.Persistent {
		t.Fatalf("old wedge with %d refusals not persistent: %+v", w.Consecutive, w)
	}
}

func TestWedgeOldButFewRefusalsIsNotPersistent(t *testing.T) {
	appender, _, clock := wedgedAppender(t)
	clock.now = clock.now.Add(time.Hour)
	if w := appender.WedgeStatus(); w.Persistent {
		t.Fatalf("one refusal reported persistent: %+v", w)
	}
}

func TestWedgeLogsFirstCauseOnceAndRateLimitsRepeats(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	appender, _, clock := wedgedAppender(t)
	appender.reconstructing.Store(true)
	for i := 0; i < 50; i++ {
		clock.now = clock.now.Add(time.Second)
		refuseOnce(t, appender)
	}
	out := buf.String()
	if n := strings.Count(out, "computer event appender: wedge began"); n != 1 {
		t.Fatalf("wedge began logged %d times, want 1:\n%s", n, out)
	}
	// 50s of refusals with a 60s interval: no continuation line yet.
	if n := strings.Count(out, "computer event appender: wedge continues"); n != 0 {
		t.Fatalf("continuation logged %d times inside the interval:\n%s", n, out)
	}
	clock.now = clock.now.Add(wedgeLogInterval)
	refuseOnce(t, appender)
	if n := strings.Count(buf.String(), "computer event appender: wedge continues"); n != 1 {
		t.Fatalf("continuation logged %d times after the interval, want 1", n)
	}
}
