package agentcore

import (
	"context"
	"sync"
	"time"
)

// Density D3a: /health reports live_capsules, the count planned_restart.go
// computes (liveCapsuleCount), so vmctl's idle decision sees a capsule held
// by a parked run, a capsule before its first Freeze, or a frozen candidate
// awaiting approval
// (docs/problems/idle-hibernate-blind-to-live-capsules-2026-10-26.md).
//
// The count walks every engineering assignment object under the engine
// mutex, which a waiter cannot abandon, and /health is probed often. So
// /health reads it through one flight at a time that every poll joins, and
// reuses an answer for a few seconds. A stale answer cannot hide a capsule
// from the idle decision: a capsule opened in the last seconds has a running
// run, which running_runs reports, and the idle timer is minutes.

const defaultLiveCapsuleHealthTTL = 3 * time.Second

// liveCapsuleHealthTTL is how long /health reuses a count; tests shorten it.
var liveCapsuleHealthTTL = defaultLiveCapsuleHealthTTL

// liveCapsuleFlightLimit bounds a flight that never returns.
const liveCapsuleFlightLimit = 2 * time.Minute

type liveCapsuleCache struct {
	mu     sync.Mutex
	value  int
	at     time.Time
	flight *liveCapsuleFlightState
}

type liveCapsuleFlightState struct {
	done  chan struct{}
	value int
	err   error
}

// liveCapsuleCountForHealth answers /health's live_capsules read. A caller
// whose context ends first gets its error (unanswered, so /health fails
// closed); the flight carries on for the next poll.
func (rt *Runtime) liveCapsuleCountForHealth(ctx context.Context) (int, error) {
	c := &rt.liveCapsules
	c.mu.Lock()
	if ttl := liveCapsuleHealthTTL; ttl > 0 && !c.at.IsZero() && time.Since(c.at) < ttl {
		value := c.value
		c.mu.Unlock()
		return value, nil
	}
	flight := c.flight
	if flight == nil {
		flight = &liveCapsuleFlightState{done: make(chan struct{})}
		c.flight = flight
		go rt.liveCapsuleFlight(flight)
	}
	c.mu.Unlock()
	select {
	case <-flight.done:
		return flight.value, flight.err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (rt *Runtime) liveCapsuleFlight(flight *liveCapsuleFlightState) {
	ctx, cancel := context.WithTimeout(context.Background(), liveCapsuleFlightLimit)
	value, err := rt.liveCapsuleCount(ctx)
	cancel()
	c := &rt.liveCapsules
	c.mu.Lock()
	if err == nil {
		c.value, c.at = value, time.Now()
	}
	c.flight = nil
	c.mu.Unlock()
	flight.value, flight.err = value, err
	close(flight.done)
}
