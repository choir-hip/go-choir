package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The Synthetic concurrency gate
// (docs/problems/synthetic-provider-onboarding-2026-10-22.md, "Staging:
// concurrency 429s and the circuit breaker"). Synthetic limits an account to
// about 10 calls in flight and answers 429 above that, with no Retry-After. The
// gateway admits more than that, so a burst turned into 429 run errors and the
// 429s opened the model breaker. The gate holds a call until a slot is free,
// bounded by the call's context and a maximum wait, instead of sending it to be
// rejected. Waiting is not failover: the call still goes to Synthetic.
//
// The slot is held for the whole call, a stream to its end. The gate sits
// before media preparation so a waiting call holds no decoded images, and
// beneath the gateway's circuit breaker, which does not count a wait that never
// reached the upstream (ErrConcurrencyGateWait).

const (
	// SyntheticMaxConcurrentEnv sets the gate size. The probe measured about
	// 10 account-wide; the default leaves headroom for a second consumer.
	SyntheticMaxConcurrentEnv = "SYNTHETIC_MAX_CONCURRENT"
	// SyntheticMaxWaitEnv sets the longest a call waits for a slot (a Go
	// duration).
	SyntheticMaxWaitEnv = "SYNTHETIC_MAX_WAIT"

	defaultSyntheticMaxConcurrent = 8
	defaultSyntheticMaxWait       = 120 * time.Second
)

// ErrConcurrencyGateWait: a call ended while waiting for a Synthetic
// concurrency slot (the wait bound, or the caller's context). Nothing was sent
// upstream, so it is no evidence about the model.
var ErrConcurrencyGateWait = errors.New("call ended while waiting for a synthetic concurrency slot")

// ErrUpstreamConcurrencyLimit: Synthetic answered 429, its account concurrency
// limit. It is no evidence that the model is unhealthy.
var ErrUpstreamConcurrencyLimit = errors.New("upstream concurrency limit")

// upstreamConcurrencyError keeps the sanitized status text the run reads and
// matches ErrUpstreamConcurrencyLimit.
type upstreamConcurrencyError struct{ status string }

func (e *upstreamConcurrencyError) Error() string {
	return fmt.Sprintf("synthetic: status %s (sanitized)", e.status)
}

func (e *upstreamConcurrencyError) Is(target error) bool {
	return target == ErrUpstreamConcurrencyLimit
}

// SyntheticGateStats is the gate's state and counters since start.
type SyntheticGateStats struct {
	Limit        int   `json:"limit"`
	MaxWaitMs    int64 `json:"max_wait_ms"`
	InFlight     int   `json:"in_flight"`
	Peak         int   `json:"peak"`
	Granted      int64 `json:"granted"`
	Waited       int64 `json:"waited"`
	WaitTimeouts int64 `json:"wait_timeouts"`
	Cancelled    int64 `json:"cancelled"`
}

type concurrencyGate struct {
	slots   chan struct{}
	maxWait time.Duration

	mu                                   sync.Mutex
	inflight, peak                       int
	granted, waited, timeouts, cancelled int64
}

func newConcurrencyGate(limit int, maxWait time.Duration) *concurrencyGate {
	if limit <= 0 {
		limit = defaultSyntheticMaxConcurrent
	}
	if maxWait <= 0 {
		maxWait = defaultSyntheticMaxWait
	}
	return &concurrencyGate{slots: make(chan struct{}, limit), maxWait: maxWait}
}

// newSyntheticGateFromEnv reads the bounds; an unreadable value is the default.
func newSyntheticGateFromEnv() *concurrencyGate {
	limit, _ := strconv.Atoi(strings.TrimSpace(os.Getenv(SyntheticMaxConcurrentEnv)))
	wait, _ := time.ParseDuration(strings.TrimSpace(os.Getenv(SyntheticMaxWaitEnv)))
	return newConcurrencyGate(limit, wait)
}

// acquire takes a slot, waiting up to maxWait or until ctx ends. It returns
// the release func (call it on every path, once) or an error wrapping
// ErrConcurrencyGateWait; in that case no slot is held.
func (g *concurrencyGate) acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		g.count(&g.cancelled)
		return nil, fmt.Errorf("synthetic: %w: %w", ErrConcurrencyGateWait, err)
	}
	select {
	case g.slots <- struct{}{}:
	default:
		g.count(&g.waited)
		timer := time.NewTimer(g.maxWait)
		defer timer.Stop()
		select {
		case g.slots <- struct{}{}:
		case <-ctx.Done():
			g.count(&g.cancelled)
			return nil, fmt.Errorf("synthetic: %w: %w", ErrConcurrencyGateWait, ctx.Err())
		case <-timer.C:
			g.count(&g.timeouts)
			return nil, fmt.Errorf("synthetic: %w (waited %s)", ErrConcurrencyGateWait, g.maxWait)
		}
	}
	g.mu.Lock()
	g.granted++
	g.inflight++
	g.peak = max(g.peak, g.inflight)
	g.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			g.inflight--
			g.mu.Unlock()
			<-g.slots
		})
	}, nil
}

func (g *concurrencyGate) count(c *int64) {
	g.mu.Lock()
	*c++
	g.mu.Unlock()
}

func (g *concurrencyGate) stats() SyntheticGateStats {
	g.mu.Lock()
	defer g.mu.Unlock()
	return SyntheticGateStats{
		Limit: cap(g.slots), MaxWaitMs: g.maxWait.Milliseconds(), InFlight: g.inflight, Peak: g.peak,
		Granted: g.granted, Waited: g.waited, WaitTimeouts: g.timeouts, Cancelled: g.cancelled,
	}
}

// syntheticGate is the gate of the most recently built SyntheticProvider, for
// /health. The gateway builds one.
var syntheticGate atomic.Pointer[concurrencyGate]

// SyntheticGateStatus returns the gate's snapshot; false when no Synthetic
// provider has been built.
func SyntheticGateStatus() (SyntheticGateStats, bool) {
	g := syntheticGate.Load()
	if g == nil {
		return SyntheticGateStats{}, false
	}
	return g.stats(), true
}
