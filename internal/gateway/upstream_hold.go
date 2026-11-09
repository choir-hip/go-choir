package gateway

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// upstreamHold is the memory of a single upstream that has no per-model
// breaker in front of it (the Jev judgment endpoint): after a failure the
// gateway does not call it again until the hold ends, and each consecutive
// failure doubles the hold. A transient failure (a 5xx, a 429, a timeout) is
// held from upstreamTransientBase up to upstreamTransientCap; a hard one (a
// refused key, a model that is gone) from upstreamHardBase up to
// upstreamHardCap. A success clears it. Owner, 2026-11-09: the system
// "always be intelligent about not retrying"
// (docs/evidence/provider-retry-audit-2026-11-09.md).
type upstreamHold struct {
	mu      sync.Mutex
	until   time.Time
	strikes int
}

const (
	upstreamTransientBase = time.Minute
	upstreamTransientCap  = 15 * time.Minute
	upstreamHardBase      = 15 * time.Minute
	upstreamHardCap       = 6 * time.Hour
)

// heldUntil is when the hold ends; zero when the upstream may be called.
func (h *upstreamHold) heldUntil(now time.Time) time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.until.After(now) {
		return h.until
	}
	return time.Time{}
}

// fail records a failure at now and returns when the hold ends.
func (h *upstreamHold) fail(now time.Time, hard bool) time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	base, limit := upstreamTransientBase, upstreamTransientCap
	if hard {
		base, limit = upstreamHardBase, upstreamHardCap
	}
	d := base
	for n := h.strikes; n > 0 && d < limit; n-- {
		d *= 2
	}
	d = min(d, limit)
	if h.strikes < 16 {
		h.strikes++
	}
	if until := now.Add(d); until.After(h.until) {
		h.until = until
	}
	return h.until
}

// succeed ends the hold and its strikes.
func (h *upstreamHold) succeed() {
	h.mu.Lock()
	h.until, h.strikes = time.Time{}, 0
	h.mu.Unlock()
}

// upstreamHeldError is the answer to a call the hold refused.
type upstreamHeldError struct {
	name  string
	until time.Time
}

func (e *upstreamHeldError) Error() string {
	return fmt.Sprintf("%s is unavailable until %s (held after repeated failures)", e.name, e.until.UTC().Format(time.RFC3339))
}

// retryAfterSeconds is the seconds a caller should wait, at least one.
func (e *upstreamHeldError) retryAfterSeconds(now time.Time) int {
	return max(1, int(e.until.Sub(now).Seconds()+0.999))
}

// isHardUpstreamStatus reports whether an upstream status or error text is
// one a wait does not cure: a key the upstream refuses or a model it does
// not serve.
func isHardUpstreamStatus(code int) bool {
	// Only 401, our key refused, is hard by status: no guest request can
	// cause it. A bare 403 or 404 can be request-shaped (a content policy,
	// a proxy path), and holding the shared model on it would let one
	// computer's requests close it for every computer (Devin review r1).
	return code == 401
}

// isHardUpstreamError reports whether err's text says the same.
func isHardUpstreamError(err error) bool {
	if err == nil {
		return false
	}
	if m := providerStatusPattern.FindStringSubmatch(err.Error()); m != nil {
		if code, convErr := strconv.Atoi(m[1]); convErr == nil && isHardUpstreamStatus(code) {
			return true
		}
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "model_not_found") || strings.Contains(text, "model not found")
}
