package health

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestBreaker(cfg BreakerConfig) *CircuitBreaker {
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now() }
	}
	return NewCircuitBreaker(cfg)
}

func TestCircuitBreaker_StartsClosed(t *testing.T) {
	b := newTestBreaker(BreakerConfig{})
	if b.State() != StateClosed {
		t.Fatalf("initial state = %v, want closed", b.State())
	}
}

func TestCircuitBreaker_ExecutesWhenClosed(t *testing.T) {
	b := newTestBreaker(BreakerConfig{FailureThreshold: 3})
	called := false
	err := b.Execute(func() error { called = true; return nil })
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !called {
		t.Fatal("fn was not called")
	}
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed", b.State())
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	b := newTestBreaker(BreakerConfig{FailureThreshold: 3, OpenTimeout: time.Hour})
	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error { return errors.New("boom") })
	}
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open after threshold", b.State())
	}
}

func TestCircuitBreaker_OpenShortCircuits(t *testing.T) {
	b := newTestBreaker(BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour})
	_ = b.Execute(func() error { return errors.New("boom") })
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	called := false
	err := b.Execute(func() error { called = true; return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("err = %v, want ErrCircuitOpen", err)
	}
	if called {
		t.Fatal("fn should not be called when open")
	}
}

func TestCircuitBreaker_ConsecutiveFailuresResetOnSuccess(t *testing.T) {
	b := newTestBreaker(BreakerConfig{FailureThreshold: 3, OpenTimeout: time.Hour})
	_ = b.Execute(func() error { return errors.New("boom") })
	_ = b.Execute(func() error { return errors.New("boom") })
	// A success resets the counter.
	_ = b.Execute(func() error { return nil })
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed", b.State())
	}
	// Two more failures should not open (threshold 3, counter reset).
	_ = b.Execute(func() error { return errors.New("boom") })
	_ = b.Execute(func() error { return errors.New("boom") })
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed (counter reset by success)", b.State())
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	now := atomicNow()
	b := newTestBreaker(BreakerConfig{
		FailureThreshold:   1,
		OpenTimeout:        10 * time.Second,
		HalfOpenMaxProbes:  1,
		Now:                now.now,
	})
	_ = b.Execute(func() error { return errors.New("boom") })
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}

	// Advance past the open timeout.
	now.advance(11 * time.Second)
	if b.State() != StateHalfOpen {
		t.Fatalf("state = %v, want half-open", b.State())
	}
}

func TestCircuitBreaker_HalfOpenSuccessCloses(t *testing.T) {
	now := atomicNow()
	b := newTestBreaker(BreakerConfig{
		FailureThreshold:   1,
		OpenTimeout:        10 * time.Second,
		HalfOpenMaxProbes:  1,
		Now:                now.now,
	})
	_ = b.Execute(func() error { return errors.New("boom") })
	now.advance(11 * time.Second)
	if b.State() != StateHalfOpen {
		t.Fatalf("state = %v, want half-open", b.State())
	}
	err := b.Execute(func() error { return nil })
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed after probe success", b.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	now := atomicNow()
	b := newTestBreaker(BreakerConfig{
		FailureThreshold:   1,
		OpenTimeout:        10 * time.Second,
		HalfOpenMaxProbes:  1,
		Now:                now.now,
	})
	_ = b.Execute(func() error { return errors.New("boom") })
	now.advance(11 * time.Second)
	if b.State() != StateHalfOpen {
		t.Fatalf("state = %v, want half-open", b.State())
	}
	_ = b.Execute(func() error { return errors.New("still down") })
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open after probe failure", b.State())
	}
}

func TestCircuitBreaker_HalfOpenRejectsExtraProbes(t *testing.T) {
	now := atomicNow()
	b := newTestBreaker(BreakerConfig{
		FailureThreshold:   1,
		OpenTimeout:        10 * time.Second,
		HalfOpenMaxProbes:  1,
		Now:                now.now,
	})
	_ = b.Execute(func() error { return errors.New("boom") })
	now.advance(11 * time.Second)

	// Hold the single probe slot without completing it.
	allowed, done := b.Allow()
	if !allowed {
		t.Fatal("first probe should be allowed")
	}
	// A concurrent probe should be rejected while the slot is held.
	allowed2, _ := b.Allow()
	if allowed2 {
		t.Fatal("second probe should be rejected while slot held")
	}
	// Complete the probe; the slot frees.
	done(nil)
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed after probe success", b.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	b := newTestBreaker(BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour})
	_ = b.Execute(func() error { return errors.New("boom") })
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	b.Reset()
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed after reset", b.State())
	}
}

func TestCircuitBreaker_Snapshot(t *testing.T) {
	b := newTestBreaker(BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})
	_ = b.Execute(func() error { return errors.New("boom") })
	snap := b.Snapshot()
	if snap.State != StateClosed {
		t.Fatalf("snapshot state = %v, want closed", snap.State)
	}
	if snap.ConsecutiveFails != 1 {
		t.Fatalf("snapshot fails = %d, want 1", snap.ConsecutiveFails)
	}
	_ = b.Execute(func() error { return errors.New("boom") })
	snap = b.Snapshot()
	if snap.State != StateOpen {
		t.Fatalf("snapshot state = %v, want open", snap.State)
	}
	if snap.OpenedAt == nil {
		t.Fatal("snapshot OpenedAt nil for open breaker")
	}
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	b := newTestBreaker(BreakerConfig{FailureThreshold: 100, OpenTimeout: time.Hour})
	var wg sync.WaitGroup
	var successes, failures, openRejected int64
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := b.Execute(func() error {
				if i%2 == 0 {
					return nil
				}
				return errors.New("boom")
			})
			switch {
			case err == nil:
				atomic.AddInt64(&successes, 1)
			case errors.Is(err, ErrCircuitOpen):
				atomic.AddInt64(&openRejected, 1)
			default:
				atomic.AddInt64(&failures, 1)
			}
		}(i)
	}
	wg.Wait()
	if atomic.LoadInt64(&successes)+atomic.LoadInt64(&failures)+atomic.LoadInt64(&openRejected) != 50 {
		t.Fatal("not all goroutines completed")
	}
	// Should remain closed (threshold 100).
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed under concurrent load", b.State())
	}
}

func TestCircuitBreaker_RecordSuccessFailureDirect(t *testing.T) {
	b := newTestBreaker(BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})
	b.RecordFailure()
	b.RecordFailure()
	if b.State() != StateOpen {
		t.Fatalf("state = %v, want open", b.State())
	}
	b.Reset()
	b.RecordSuccess()
	if b.State() != StateClosed {
		t.Fatalf("state = %v, want closed", b.State())
	}
}

// atomicNow provides a controllable clock for breaker tests.
type atomicClock struct {
	t atomic.Int64
}

func atomicNow() *atomicClock {
	return &atomicClock{}
}

func (c *atomicClock) now() time.Time {
	return time.Unix(0, c.t.Load())
}

func (c *atomicClock) advance(d time.Duration) {
	c.t.Add(int64(d))
}
