package main

import (
	"context"
	"log"
	"time"
)

// retryPolicy bounds each attempt and spaces attempts with capped exponential
// backoff. Sleep is replaceable so tests do not wait.
type retryPolicy struct {
	AttemptTimeout time.Duration
	Initial        time.Duration
	Max            time.Duration
	Sleep          func(ctx context.Context, d time.Duration) bool
}

// sleepContext sleeps for d and reports false if ctx ended first.
func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// retryWithBackoff runs attempt until it succeeds or ctx ends. Every attempt
// gets its own AttemptTimeout, so a database that accepts connections but
// never answers cannot hold one attempt forever. It returns nil on success and
// ctx.Err() on cancellation; it never gives up on its own.
//
// vmctl uses this for its ComputerVersion route database at startup. The old
// behaviour was log.Fatalf on the first failed ping: on 2026-10-15 a stalled
// platform Dolt made vmctl exit every 30 s, hit start-limit-hit after 10
// restarts and left the host with no VM lifecycle until a manual reset-failed
// (docs/archive/problems/dolt-clustering-assessment-2026-10-15.md section 7).
func retryWithBackoff(ctx context.Context, p retryPolicy, what string, attempt func(context.Context) error) error {
	sleep := p.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	delay := p.Initial
	for n := 1; ; n++ {
		attemptCtx, cancel := context.WithTimeout(ctx, p.AttemptTimeout)
		err := attempt(attemptCtx)
		cancel()
		if err == nil {
			if n > 1 {
				log.Printf("vmctl: %s available after %d attempts", what, n)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("vmctl: %s not ready (attempt %d): %v; retrying in %s", what, n, err, delay)
		if !sleep(ctx, delay) {
			return ctx.Err()
		}
		delay *= 2
		if delay > p.Max {
			delay = p.Max
		}
	}
}
