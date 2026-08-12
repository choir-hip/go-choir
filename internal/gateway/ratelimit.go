package gateway

import (
	"fmt"
	"sync"
	"time"
)

const (
	// DefaultRateLimitMaxRequests is the default maximum inference requests
	// per autoputer per window. Configurable via GATEWAY_RATE_LIMIT_MAX_REQUESTS.
	DefaultRateLimitMaxRequests = 60

	// DefaultRateLimitWindowSize is the default sliding window duration for
	// per-autoputer rate limiting. Configurable via GATEWAY_RATE_LIMIT_WINDOW.
	DefaultRateLimitWindowSize = 1 * time.Minute
)

// RateLimiterConfig holds rate-limiting configuration resolved from
// environment variables.
type RateLimiterConfig struct {
	// MaxRequests is the maximum number of inference requests allowed per
	// autoputer per window. 0 means use the default.
	MaxRequests int

	// WindowSize is the sliding window duration. 0 means use the default.
	WindowSize time.Duration
}

// Resolve returns a config with zero values replaced by defaults.
func (c RateLimiterConfig) Resolve() RateLimiterConfig {
	r := c
	if r.MaxRequests <= 0 {
		r.MaxRequests = DefaultRateLimitMaxRequests
	}
	if r.WindowSize <= 0 {
		r.WindowSize = DefaultRateLimitWindowSize
	}
	return r
}

// tokenBucket is a simple sliding-window counter for a single autoputer.
// It tracks request counts within a time window and resets when the
// window expires. It is safe for concurrent use via the enclosing
// PerAutoputerRateLimiter mutex.
type tokenBucket struct {
	windowStart time.Time
	count       int
}

// PerAutoputerRateLimiter provides per-autoputer rate limiting using a
// fixed-window counter algorithm. Each autoputer identity gets its own
// independent quota. When one autoputer exhausts its quota, other
// autoputeres are unaffected (VAL-GATEWAY-005, VAL-CROSS-115).
type PerAutoputerRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket // computer_id → bucket
	maxReqs int
	window  time.Duration
}

// NewPerAutoputerRateLimiter creates a rate limiter that allows maxReqs
// requests per autoputer per window duration.
func NewPerAutoputerRateLimiter(maxReqs int, window time.Duration) *PerAutoputerRateLimiter {
	return &PerAutoputerRateLimiter{
		buckets: make(map[string]*tokenBucket),
		maxReqs: maxReqs,
		window:  window,
	}
}

// Allow checks whether the autoputer is within its rate limit and
// atomically consumes one slot if so. Returns true if the request
// is allowed, false if the rate limit has been exceeded.
func (rl *PerAutoputerRateLimiter) Allow(computerID string) bool {
	return rl.record(computerID, false)
}

// Record checks and records a request, returning whether it was allowed.
// This is the same as Allow but is named for clarity in the handler
// where we want to explicitly record the rate limit check.
func (rl *PerAutoputerRateLimiter) Record(computerID string) bool {
	return rl.record(computerID, false)
}

// record is the internal implementation. If dryRun is true, it checks
// without consuming. Otherwise it atomically checks and consumes.
func (rl *PerAutoputerRateLimiter) record(computerID string, dryRun bool) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[computerID]
	if !ok || now.Sub(b.windowStart) >= rl.window {
		// No bucket yet or window expired: start a new window.
		if dryRun {
			return true
		}
		rl.buckets[computerID] = &tokenBucket{
			windowStart: now,
			count:       1,
		}
		return true
	}

	if b.count >= rl.maxReqs {
		return false
	}

	if dryRun {
		return true
	}

	b.count++
	return true
}

// Status returns the current usage for a autoputer. Returns (used, limit, resetIn).
// Used is 0 if the autoputer has no bucket or the window has expired.
func (rl *PerAutoputerRateLimiter) Status(computerID string) (used, limit int, resetIn time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limit = rl.maxReqs

	b, ok := rl.buckets[computerID]
	if !ok {
		return 0, limit, rl.window
	}

	now := time.Now()
	elapsed := now.Sub(b.windowStart)
	if elapsed >= rl.window {
		// Window expired; effectively 0 used.
		return 0, limit, rl.window
	}

	remaining := rl.window - elapsed
	return b.count, limit, remaining
}

// RemoveBucket removes the rate-limit bucket for a autoputer, typically
// when the autoputer is revoked or stopped. This prevents unbounded memory
// growth from stale autoputer entries.
func (rl *PerAutoputerRateLimiter) RemoveBucket(computerID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.buckets, computerID)
}

// String returns a human-readable description of the rate limiter config.
func (rl *PerAutoputerRateLimiter) String() string {
	return fmt.Sprintf("PerAutoputerRateLimiter(max=%d/window=%s)", rl.maxReqs, rl.window)
}
