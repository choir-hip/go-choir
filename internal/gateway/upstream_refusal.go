package gateway

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// An upstream refusal is a provider's ordinary 429 (a rate limit, not a
// spent quota) or its concurrency ceiling. It used to leave the gateway as a
// 502 with no Retry-After, so the guest took it for an outage and retried it
// blind, three times at once and four more after that, for every caller of a
// burst. It now leaves as HTTP 429, kind upstream_rate_limit, with a
// Retry-After (docs/problems/sub-call-spacer-outlived-its-bucket-2026-11-24.md).
//
// The providers drop the upstream's own Retry-After on an ordinary 429, so
// the wait is computed: the time until the model's breaker lets a probe
// through when the refusals opened it, else the header's resolution of one
// second.

// refusalRetryFloor is the Retry-After header's own resolution: whole seconds.
const refusalRetryFloor = time.Second

// refusalRetryCap is the longest wait the gateway tells a guest to wait: the
// guest clamps any Retry-After to one minute (gatewayruntime maxRateLimitWait),
// so a longer promise would be one the guest cannot keep. The one number is a
// minute; a model open for longer is reached by the guest's next call, which
// the open breaker answers "circuit open" and fails over on.
const refusalRetryCap = time.Minute

// isUpstreamRefusal reports whether err is an ordinary upstream 429 or a
// concurrency refusal: not a quota (typed separately), not a breaker's own
// refusal, and not a call that ended waiting for a slot.
func isUpstreamRefusal(err error) bool {
	if err == nil || provider.IsQuotaExhausted(err) || errors.Is(err, provider.ErrConcurrencyGateWait) {
		return false
	}
	if errors.Is(err, provider.ErrUpstreamConcurrencyLimit) {
		return true
	}
	if m := providerStatusPattern.FindStringSubmatch(err.Error()); m != nil {
		return m[1] == "429"
	}
	return false
}

// upstreamRefusalError carries the computed wait with the refusal.
type upstreamRefusalError struct {
	err        error
	retryAfter time.Duration
}

func (e *upstreamRefusalError) Error() string { return e.err.Error() }
func (e *upstreamRefusalError) Unwrap() error { return e.err }

// refusalRetryAfter is how long a refused caller should wait: until model's
// breaker half-opens when it is open, never less than the header's second.
func (c *CircuitBreakingProvider) refusalRetryAfter(model string) time.Duration {
	wait := refusalRetryFloor
	if b := c.lookupModelBreaker(model); b != nil {
		if s := b.Snapshot(); s.State == health.StateOpen && s.OpenUntil != nil {
			if left := s.OpenUntil.Sub(c.now()); left > wait {
				wait = left
			}
		}
	}
	if wait > refusalRetryCap {
		wait = refusalRetryCap
	}
	return wait
}

// upstreamRefusalWait returns the wait to answer an upstream refusal with.
func upstreamRefusalWait(err error) (time.Duration, bool) {
	var r *upstreamRefusalError
	if errors.As(err, &r) {
		return r.retryAfter, true
	}
	if isUpstreamRefusal(err) {
		return refusalRetryFloor, true
	}
	return 0, false
}

// writeUpstreamRefusal answers an upstream refusal: 429, Retry-After in whole
// seconds (rounded up), the typed body the guest waits or fails over on.
func writeUpstreamRefusal(w http.ResponseWriter, providerName, model, sanitized string, wait time.Duration) {
	secs := int((wait + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{
		Error: sanitized, Kind: ErrorKindUpstreamRateLimit, Provider: providerName, Model: strings.TrimSpace(model),
	})
}
