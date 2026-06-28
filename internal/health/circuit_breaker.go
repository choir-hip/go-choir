package health

import (
	"errors"
	"sync"
	"time"
)

// State is the operational state of a CircuitBreaker.
type State int

const (
	// StateClosed allows calls through. Failures are counted; once they reach
	// the failure threshold within the rolling window the breaker opens.
	StateClosed State = iota
	// StateOpen rejects calls immediately without contacting the dependency.
	// After the open timeout elapses the breaker transitions to half-open.
	StateOpen
	// StateHalfOpen allows a limited number of probe calls through to test
	// whether the dependency has recovered. A probe success closes the breaker;
	// a probe failure reopens it.
	StateHalfOpen
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned by Execute when the breaker is open and the call
// is short-circuited without contacting the dependency. Callers should treat
// this as a signal to degrade gracefully rather than retry immediately.
var ErrCircuitOpen = errors.New("circuit breaker open")

// BreakerConfig governs CircuitBreaker transitions. Zero values are replaced
// with safe defaults by NewCircuitBreaker.
type BreakerConfig struct {
	// FailureThreshold is the number of consecutive failures that opens the
	// breaker from closed. Default 5.
	FailureThreshold int
	// OpenTimeout is how long the breaker stays open before transitioning to
	// half-open. Default 30s.
	OpenTimeout time.Duration
	// HalfOpenMaxProbes is the number of probe calls permitted in half-open
	// state. A probe success closes the breaker immediately; a probe failure
	// reopens it. Default 1.
	HalfOpenMaxProbes int
	// SuccessThreshold is the number of consecutive successes required in
	// half-open state to close the breaker when HalfOpenMaxProbes > 1.
	// Default 1 (a single probe success closes).
	SuccessThreshold int
	// Now returns the current time. It is overridable in tests; production
	// code leaves it nil to use time.Now.
	Now func() time.Time
}

func (c BreakerConfig) withDefaults() BreakerConfig {
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = 5
	}
	if c.OpenTimeout <= 0 {
		c.OpenTimeout = 30 * time.Second
	}
	if c.HalfOpenMaxProbes <= 0 {
		c.HalfOpenMaxProbes = 1
	}
	if c.SuccessThreshold <= 0 {
		c.SuccessThreshold = 1
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// CircuitBreaker implements a closed/open/half-open circuit breaker. It is
// safe for concurrent use. The breaker is designed to degrade gracefully:
// callers receive ErrCircuitOpen when the breaker is open and should fall back
// to a reduced-functionality path rather than hard-failing the request.
type CircuitBreaker struct {
	cfg BreakerConfig

	mu               sync.Mutex
	state            State
	consecutiveFails int
	consecutiveWins  int
	openedAt         time.Time
	probesInflight   int
}

// NewCircuitBreaker returns a breaker configured with cfg (zero values
// replaced by defaults).
func NewCircuitBreaker(cfg BreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{cfg: cfg.withDefaults(), state: StateClosed}
}

// State returns the current breaker state. It may transition open -> half-open
// as a side effect if the open timeout has elapsed.
func (b *CircuitBreaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maybeHalfOpenLocked()
	return b.state
}

// Allow reports whether a call should proceed and, when true, returns a
// function the caller MUST invoke to record the outcome. The done callback
// records a success on nil error and a failure otherwise. When Allow returns
// false the call must be short-circuited (the caller should degrade).
//
// Allow/done is the low-level API; prefer Execute for simple cases.
func (b *CircuitBreaker) Allow() (bool, func(error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maybeHalfOpenLocked()

	switch b.state {
	case StateOpen:
		return false, nil
	case StateHalfOpen:
		if b.probesInflight >= b.cfg.HalfOpenMaxProbes {
			return false, nil
		}
		b.probesInflight++
		return true, b.doneLocked
	case StateClosed:
		return true, b.doneLocked
	default:
		return true, b.doneLocked
	}
}

// doneLocked is the outcome recorder returned by Allow. It must be called
// without holding b.mu; it re-locks to mutate state.
func (b *CircuitBreaker) doneLocked(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateHalfOpen {
		b.probesInflight--
		if b.probesInflight < 0 {
			b.probesInflight = 0
		}
	}
	if err != nil {
		b.recordFailureLocked()
		return
	}
	b.recordSuccessLocked()
}

func (b *CircuitBreaker) recordSuccessLocked() {
	b.consecutiveFails = 0
	switch b.state {
	case StateHalfOpen:
		b.consecutiveWins++
		if b.consecutiveWins >= b.cfg.SuccessThreshold || b.cfg.HalfOpenMaxProbes == 1 {
			b.closeLocked()
		}
	case StateClosed:
		b.consecutiveWins++
	}
}

func (b *CircuitBreaker) recordFailureLocked() {
	b.consecutiveWins = 0
	b.consecutiveFails++
	switch b.state {
	case StateHalfOpen:
		b.openLocked()
	case StateClosed:
		if b.consecutiveFails >= b.cfg.FailureThreshold {
			b.openLocked()
		}
	case StateOpen:
		// Already open; reset the open timer to extend the cooldown.
		b.openedAt = b.cfg.Now()
	}
}

func (b *CircuitBreaker) openLocked() {
	b.state = StateOpen
	b.openedAt = b.cfg.Now()
	b.probesInflight = 0
}

func (b *CircuitBreaker) closeLocked() {
	b.state = StateClosed
	b.consecutiveFails = 0
	b.consecutiveWins = 0
	b.probesInflight = 0
}

// maybeHalfOpenLocked transitions an open breaker to half-open once the open
// timeout has elapsed. Caller must hold b.mu.
func (b *CircuitBreaker) maybeHalfOpenLocked() {
	if b.state != StateOpen {
		return
	}
	if b.cfg.Now().Sub(b.openedAt) >= b.cfg.OpenTimeout {
		b.state = StateHalfOpen
		b.consecutiveWins = 0
		b.probesInflight = 0
	}
}

// RecordSuccess records a successful call. Use this when the caller manages
// the call lifecycle directly instead of using Execute or Allow.
func (b *CircuitBreaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateHalfOpen {
		b.probesInflight--
		if b.probesInflight < 0 {
			b.probesInflight = 0
		}
	}
	b.recordSuccessLocked()
}

// RecordFailure records a failed call. Use this when the caller manages the
// call lifecycle directly instead of using Execute or Allow.
func (b *CircuitBreaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state == StateHalfOpen {
		b.probesInflight--
		if b.probesInflight < 0 {
			b.probesInflight = 0
		}
	}
	b.recordFailureLocked()
}

// Execute runs fn through the breaker. When the breaker is open (and no probe
// slot is available in half-open), it returns ErrCircuitOpen without invoking
// fn. On success fn's nil error is recorded as a success; otherwise the error
// is recorded as a failure and returned to the caller.
//
// Execute is the recommended API for wrapping a single dependency call.
func (b *CircuitBreaker) Execute(fn func() error) error {
	allowed, done := b.Allow()
	if !allowed {
		return ErrCircuitOpen
	}
	err := fn()
	done(err)
	return err
}

// Snapshot is an immutable view of the breaker state for observability.
type Snapshot struct {
	State            State      `json:"state"`
	ConsecutiveFails int        `json:"consecutive_failures"`
	ConsecutiveWins  int        `json:"consecutive_successes"`
	OpenedAt         *time.Time `json:"opened_at,omitempty"`
}

// Snapshot returns a point-in-time view of the breaker state.
func (b *CircuitBreaker) Snapshot() Snapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maybeHalfOpenLocked()
	s := Snapshot{
		State:            b.state,
		ConsecutiveFails: b.consecutiveFails,
		ConsecutiveWins:  b.consecutiveWins,
	}
	if b.state == StateOpen && !b.openedAt.IsZero() {
		t := b.openedAt
		s.OpenedAt = &t
	}
	return s
}

// Reset forces the breaker back to closed state, clearing all counters. This
// is intended for admin/ops endpoints and tests, not for normal request flow.
func (b *CircuitBreaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closeLocked()
}
