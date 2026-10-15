package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

// recordedSleeper returns a sleep function that records requested delays and
// returns immediately (or reports cancellation when ctx is done).
func recordedSleeper(delays *[]time.Duration) func(context.Context, time.Duration) bool {
	return func(ctx context.Context, d time.Duration) bool {
		*delays = append(*delays, d)
		return ctx.Err() == nil
	}
}

func TestRetryWithBackoffReturnsAfterTransientFailures(t *testing.T) {
	var delays []time.Duration
	calls := 0
	p := retryPolicy{AttemptTimeout: time.Second, Initial: time.Second, Max: 30 * time.Second, Sleep: recordedSleeper(&delays)}
	err := retryWithBackoff(context.Background(), p, "route database", func(context.Context) error {
		calls++
		if calls < 4 {
			return errors.New("context deadline exceeded")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("want success after transient failures, got %v", err)
	}
	if calls != 4 {
		t.Fatalf("calls = %d, want 4", calls)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if len(delays) != len(want) {
		t.Fatalf("delays = %v, want %v", delays, want)
	}
	for i := range want {
		if delays[i] != want[i] {
			t.Fatalf("delays = %v, want %v", delays, want)
		}
	}
}

func TestRetryWithBackoffCapsTheDelay(t *testing.T) {
	var delays []time.Duration
	calls := 0
	p := retryPolicy{AttemptTimeout: time.Second, Initial: time.Second, Max: 30 * time.Second, Sleep: recordedSleeper(&delays)}
	_ = retryWithBackoff(context.Background(), p, "route database", func(context.Context) error {
		calls++
		if calls < 10 {
			return errors.New("down")
		}
		return nil
	})
	for _, d := range delays {
		if d > 30*time.Second {
			t.Fatalf("delay %s exceeds the cap; delays=%v", d, delays)
		}
	}
	if last := delays[len(delays)-1]; last != 30*time.Second {
		t.Fatalf("delay never reached the cap: %v", delays)
	}
}

func TestRetryWithBackoffBoundsEachAttemptAndKeepsTrying(t *testing.T) {
	var delays []time.Duration
	calls := 0
	p := retryPolicy{AttemptTimeout: 50 * time.Millisecond, Initial: time.Millisecond, Max: 2 * time.Millisecond, Sleep: recordedSleeper(&delays)}
	start := time.Now()
	err := retryWithBackoff(context.Background(), p, "route database", func(ctx context.Context) error {
		calls++
		if calls == 1 {
			<-ctx.Done() // a hung server: only the per-attempt deadline frees it
			return ctx.Err()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("want success on the second attempt, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("hung attempt was not bounded: %s", elapsed)
	}
}

func TestRetryWithBackoffStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	p := retryPolicy{AttemptTimeout: time.Second, Initial: time.Hour, Max: time.Hour, Sleep: sleepContext}
	done := make(chan error, 1)
	go func() {
		done <- retryWithBackoff(ctx, p, "route database", func(context.Context) error {
			calls++
			return errors.New("down")
		})
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("retryWithBackoff did not return promptly on cancellation (SIGTERM would hang the stop)")
	}
}
