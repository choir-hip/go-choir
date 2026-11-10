package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// One limiter stands in front of the one Jev provider key. Every door
// (a computer's /provider/v1/judgments and the host route) converges in
// JevTransport.Decide, in the single gateway process that holds the key, so
// the limit is organization-wide by construction. Latency is about 0.1 to 0.2
// s, so by Little's law a sustained 15 a second needs about 3 calls in flight;
// the binding control is the rate, and the in-flight cap only hides latency
// spikes (docs/evidence/jev-measurement-2026-11-10.md, section 5). These are
// tuning constants in code, not switches.
const (
	jevRatePerSecond = 15.0
	jevBurst         = 30.0
	jevMaxInFlight   = 32
	// jevInteractiveReserve is the in-flight share bulk callers cannot take,
	// and jevInteractiveTokenReserve the burst tokens they cannot spend, so a
	// scorer burst never delays the prompt router or the disclosure gate.
	jevInteractiveReserve      = 8
	jevInteractiveTokenReserve = 5.0
	// One computer cannot take the whole organization share. This replaces the
	// 60-a-minute per-computer bucket, which capped the paper at 1 a second.
	jevComputerInFlight = 24
	jevComputerRate     = 10.0
	jevComputerBurst    = 30.0
	jevQueueMaxWait     = 20 * time.Second
	jevBackoffDefault   = 2 * time.Second
	jevBackoffCap       = time.Minute
	jevStatsInterval    = time.Hour
	// jevPollCeiling bounds one sleep in the queue; a release wakes sleepers
	// sooner.
	jevPollCeiling = 50 * time.Millisecond
)

// JudgmentScopeInteractive marks the latency-critical judgments (prompt
// router, disclosure gate, a desk's own System 1 question): they draw on the
// reserved share. Every other scope is bulk.
const JudgmentScopeInteractive = "interactive"

type jevClass int

const (
	jevBulk jevClass = iota
	jevInteractive
)

func (c jevClass) String() string {
	if c == jevInteractive {
		return "interactive"
	}
	return "bulk"
}

// judgmentRateScope validates a request's scope. The scope no longer picks a
// rate bucket; it names a priority class.
func judgmentRateScope(scope string) (jevClass, error) {
	switch strings.TrimSpace(scope) {
	case "", JudgmentScopeIntakeLabels:
		return jevBulk, nil
	case JudgmentScopeInteractive:
		return jevInteractive, nil
	}
	return jevBulk, fmt.Errorf("unknown judgment scope %q", scope)
}

// rateBucket is a token bucket refilled lazily from the limiter's clock.
type rateBucket struct {
	tokens float64
	last   time.Time
}

func (b *rateBucket) refill(now time.Time, rate, burst float64) {
	if b.last.IsZero() {
		b.tokens, b.last = burst, now
		return
	}
	if d := now.Sub(b.last).Seconds(); d > 0 {
		b.tokens = min(burst, b.tokens+d*rate)
		b.last = now
	}
}

type computerShare struct {
	bucket   rateBucket
	inFlight int
}

// jevLimiter is the shared rate, in-flight cap, interactive reserve,
// per-computer fairness and backoff.
type jevLimiter struct {
	mu           sync.Mutex
	now          func() time.Time
	rate, burst  float64
	maxInFlight  int
	reserve      int
	tokenReserve float64
	maxWait      time.Duration
	global       rateBucket
	computers    map[string]*computerShare
	inFlight     int
	backoffUntil time.Time
	wake         chan struct{} // closed and replaced whenever capacity may have appeared
}

func newJevLimiter() *jevLimiter {
	return &jevLimiter{
		now: time.Now, rate: jevRatePerSecond, burst: jevBurst, maxInFlight: jevMaxInFlight,
		reserve: jevInteractiveReserve, tokenReserve: jevInteractiveTokenReserve, maxWait: jevQueueMaxWait,
		computers: map[string]*computerShare{}, wake: make(chan struct{}),
	}
}

// jevLimitedError is the refusal of a caller that waited the whole queue
// bound; the door answers 429 with Retry-After.
type jevLimitedError struct{ retryAfter time.Duration }

func (e *jevLimitedError) Error() string {
	return fmt.Sprintf("Jev judgment queue full; retry in %s", e.retryAfter.Round(time.Second))
}

// acquire waits for one call's worth of rate and in-flight capacity. It
// returns the release func and how long the caller queued. A cancelled
// context leaves the queue without taking anything.
func (l *jevLimiter) acquire(ctx context.Context, computerID string, class jevClass) (release func(), waited time.Duration, err error) {
	start := l.now()
	deadline := start.Add(l.maxWait)
	for {
		l.mu.Lock()
		now := l.now()
		l.global.refill(now, l.rate, l.burst)
		cs := l.computers[computerID]
		if cs == nil {
			cs = &computerShare{}
			l.computers[computerID] = cs
		}
		cs.bucket.refill(now, jevComputerRate, jevComputerBurst)
		var wait time.Duration
		switch {
		case now.Before(l.backoffUntil):
			wait = l.backoffUntil.Sub(now)
		case l.inFlightFull(class) || cs.inFlight >= jevComputerInFlight:
			wait = jevPollCeiling
		default:
			needed := 1.0
			if class == jevBulk {
				needed += l.tokenReserve
			}
			if l.global.tokens < needed {
				wait = time.Duration((needed - l.global.tokens) / l.rate * float64(time.Second))
			} else if cs.bucket.tokens < 1 {
				wait = time.Duration((1 - cs.bucket.tokens) / jevComputerRate * float64(time.Second))
			}
		}
		if wait == 0 {
			l.global.tokens--
			cs.bucket.tokens--
			l.inFlight++
			cs.inFlight++
			l.mu.Unlock()
			var once sync.Once
			return func() { once.Do(func() { l.release(computerID) }) }, now.Sub(start), nil
		}
		wakeCh := l.wake
		l.mu.Unlock()
		if !now.Before(deadline) {
			return nil, now.Sub(start), &jevLimitedError{retryAfter: max(wait, time.Second)}
		}
		timer := time.NewTimer(min(max(wait, time.Millisecond), jevPollCeiling))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, l.now().Sub(start), ctx.Err()
		case <-wakeCh:
		case <-timer.C:
		}
		timer.Stop()
	}
}

// inFlightFull reports, under mu, whether class may not start another call.
func (l *jevLimiter) inFlightFull(class jevClass) bool {
	if class == jevBulk {
		return l.inFlight >= l.maxInFlight-l.reserve
	}
	return l.inFlight >= l.maxInFlight
}

func (l *jevLimiter) release(computerID string) {
	l.mu.Lock()
	l.inFlight--
	if cs := l.computers[computerID]; cs != nil {
		cs.inFlight--
	}
	close(l.wake)
	l.wake = make(chan struct{})
	l.mu.Unlock()
}

// backoff makes every caller wait until d from now (an upstream 429's
// Retry-After). It only ever extends the wait.
func (l *jevLimiter) backoff(d time.Duration) {
	d = min(max(d, jevBackoffDefault/2), jevBackoffCap)
	l.mu.Lock()
	if until := l.now().Add(d); until.After(l.backoffUntil) {
		l.backoffUntil = until
	}
	l.mu.Unlock()
}

// parseRetryAfter reads a Retry-After header in seconds or as an HTTP date.
func parseRetryAfter(h http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return jevBackoffDefault
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil && secs >= 0 {
		return time.Duration(secs * float64(time.Second))
	}
	if at, err := http.ParseTime(v); err == nil {
		return max(at.Sub(now), 0)
	}
	return jevBackoffDefault
}

// jevStats keeps one hour of call outcomes for the summary line. There is no
// goroutine: the first call after the hour emits it.
type jevStats struct {
	mu        sync.Mutex
	since     time.Time
	calls     int
	failures  int
	rate429   int
	latencies []time.Duration
	queueWait time.Duration
	queueMax  time.Duration
}

// observe records one call and returns the summary line when the hour is up.
func (s *jevStats) observe(now time.Time, latency, queued time.Duration, status int, ok bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.since.IsZero() {
		s.since = now
	}
	s.calls++
	if !ok {
		s.failures++
	}
	if status == http.StatusTooManyRequests {
		s.rate429++
	}
	s.latencies = append(s.latencies, latency)
	s.queueWait += queued
	s.queueMax = max(s.queueMax, queued)
	if now.Sub(s.since) < jevStatsInterval {
		return ""
	}
	sort.Slice(s.latencies, func(i, j int) bool { return s.latencies[i] < s.latencies[j] })
	pct := func(p float64) time.Duration {
		return s.latencies[min(len(s.latencies)-1, int(p*float64(len(s.latencies))))]
	}
	line := fmt.Sprintf("gateway: jev hour summary: calls=%d failures=%d 429s=%d p50=%s p90=%s queue_mean=%s queue_max=%s",
		s.calls, s.failures, s.rate429, pct(0.5).Round(time.Millisecond), pct(0.9).Round(time.Millisecond),
		(s.queueWait / time.Duration(s.calls)).Round(time.Millisecond), s.queueMax.Round(time.Millisecond))
	s.since, s.calls, s.failures, s.rate429, s.latencies, s.queueWait, s.queueMax = now, 0, 0, 0, nil, 0, 0
	return line
}

func logJevCall(computerID string, class jevClass, questions int, queued, upstream time.Duration, status int, err error) {
	state := "ok"
	if err != nil {
		state = "failed"
	}
	log.Printf("gateway: jev call computer=%s class=%s questions=%d queue_ms=%d upstream_ms=%d status=%d %s",
		computerID, class, questions, queued.Milliseconds(), upstream.Milliseconds(), status, state)
}
