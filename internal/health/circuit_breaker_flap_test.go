package health

import (
	"testing"
	"time"
)

type flapClock struct{ now time.Time }

func (c *flapClock) Now() time.Time { return c.now }

func flapBreaker(c *flapClock) *CircuitBreaker {
	return NewCircuitBreaker(BreakerConfig{
		FailureThreshold: 2,
		Now:              c.Now,
		Backoff: func(trips int) time.Duration {
			d := 10 * time.Second
			for i := 1; i < trips; i++ {
				d *= 2
			}
			return d
		},
	})
}

func fail(b *CircuitBreaker, n int) {
	for i := 0; i < n; i++ {
		if ok, done := b.Allow(); ok {
			done(errTest)
		}
	}
}

type testErr struct{}

func (testErr) Error() string { return "refused" }

var errTest = testErr{}

func openFor(b *CircuitBreaker) time.Duration {
	s := b.Snapshot()
	if s.OpenedAt == nil || s.OpenUntil == nil {
		return 0
	}
	return s.OpenUntil.Sub(*s.OpenedAt)
}

// A breaker that one passing probe resets to its first, shortest cooldown
// flaps under a burst: open, probe, close, the whole burst fails again, open
// for the same short time. One probe's success must not reset the doubling;
// the cooldown is forgotten only by time spent closed.
func TestOneProbeSuccessDoesNotResetTheDoubling(t *testing.T) {
	c := &flapClock{now: time.Unix(1000, 0)}
	b := flapBreaker(c)
	fail(b, 2)
	if got := openFor(b); got != 10*time.Second {
		t.Fatalf("first open lasts %s, want 10s", got)
	}
	c.now = c.now.Add(10 * time.Second)
	if ok, done := b.Allow(); !ok {
		t.Fatal("no probe admitted after the open window")
	} else {
		done(nil)
	}
	if b.State() != StateClosed {
		t.Fatalf("state %s after a passing probe, want closed", b.State())
	}
	fail(b, 2)
	if got := openFor(b); got != 20*time.Second {
		t.Fatalf("the reopen right after one passing probe lasts %s, want the doubled 20s", got)
	}

	// Closed for as long as it was last open (20s), the doubling is forgotten
	// one step; closed much longer, all of it.
	c.now = c.now.Add(20 * time.Second)
	if ok, done := b.Allow(); ok {
		done(nil)
	}
	c.now = c.now.Add(time.Hour)
	fail(b, 2)
	if got := openFor(b); got != 10*time.Second {
		t.Fatalf("an open after an hour closed lasts %s, want the base 10s", got)
	}
}

// Failures of calls that were already in flight when the breaker opened
// belong to the same open: they must not slide the window forward.
func TestLateFailuresOfTheSameOpenDoNotSlideTheWindow(t *testing.T) {
	c := &flapClock{now: time.Unix(1000, 0)}
	b := flapBreaker(c)
	fail(b, 2)
	before := b.Snapshot()
	c.now = c.now.Add(4 * time.Second)
	b.RecordFailure()
	b.RecordFailure()
	after := b.Snapshot()
	if !after.OpenUntil.Equal(*before.OpenUntil) {
		t.Fatalf("open until moved from %s to %s on late failures", before.OpenUntil, after.OpenUntil)
	}
	if after.Opens != before.Opens || after.Opens != 1 {
		t.Fatalf("opens = %d then %d, want one open", before.Opens, after.Opens)
	}
}
