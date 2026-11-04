package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrUpstreamQuotaExhausted: the provider answered 429 because the account's
// quota or usage limit is spent (Synthetic "You've exceeded your subscription
// rate limits", a zero weekly plan percentage; ChatGPT "usage_limit_reached").
// Unlike a concurrency 429 it does not clear in seconds, so the gateway puts
// the model in cooldown on the first one and the pool routes to its next
// member (docs/design/model-pool-autoroute-2026-11-02.md).
var ErrUpstreamQuotaExhausted = errors.New("upstream quota exhausted")

// QuotaExhaustedError is an exhausted upstream quota, typed: the provider,
// the model when known, the sanitized status, and when the quota is
// expected to reset. It matches ErrUpstreamQuotaExhausted. The gateway hands
// it to the guest as a typed answer (HTTP 429, kind provider_quota) so a
// run on a pinned model switches to another model, skipping this one until
// RetryAt, instead of dying
// (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md).
type QuotaExhaustedError struct {
	Provider string
	Model    string
	// Status is the upstream's sanitized status; empty when the call was
	// refused by the gateway's quota circuit without reaching the upstream.
	Status string
	// RetryAt is when the quota is expected to reset: the upstream's
	// Retry-After, else its own reset time (Synthetic's nextRegenAt), else
	// (RetryAtEstimated) the gateway's cooldown. Zero when unknown.
	RetryAt          time.Time
	RetryAtEstimated bool
	// CircuitOpen says the gateway refused the call: the model's quota
	// circuit is open until RetryAt.
	CircuitOpen bool
}

func (e *QuotaExhaustedError) Error() string {
	var b strings.Builder
	if e.CircuitOpen {
		fmt.Fprintf(&b, "provider %s model %s: provider quota exhausted (%s)", e.Provider, firstNonEmptyQuota(e.Model, "(default)"), e.Provider)
	} else {
		// The status text stays: the breaker and older guests read it.
		fmt.Fprintf(&b, "%s: quota exhausted: status %s (sanitized)", e.Provider, e.Status)
	}
	if !e.RetryAt.IsZero() {
		fmt.Fprintf(&b, "; resets at %s", e.RetryAt.UTC().Format(time.RFC3339))
		if e.RetryAtEstimated {
			b.WriteString(" (estimated)")
		}
	}
	if e.CircuitOpen {
		// No status code in this text: a guest that reads "429" waits it
		// out as a rate limit; "circuit open" moves a pool to its next
		// member at once.
		b.WriteString(" (circuit open)")
	}
	return b.String()
}

func (e *QuotaExhaustedError) Is(target error) bool {
	return target == ErrUpstreamQuotaExhausted
}

// AsQuotaExhausted returns err's typed quota, if it is one.
func AsQuotaExhausted(err error) (*QuotaExhaustedError, bool) {
	var q *QuotaExhaustedError
	if errors.As(err, &q) && q != nil {
		return q, true
	}
	return nil, false
}

func firstNonEmptyQuota(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

// quotaRetryAtHorizon bounds a reset time the gateway believes: a header
// further out than this is not a reset time it can act on.
const quotaRetryAtHorizon = 8 * 24 * time.Hour

// retryAfterAt reads a Retry-After header (seconds or an HTTP date) as an
// absolute time; zero when absent, unreadable, or past the horizon.
func retryAfterAt(h http.Header, now time.Time) time.Time {
	raw := strings.TrimSpace(h.Get("Retry-After"))
	if raw == "" {
		return time.Time{}
	}
	var at time.Time
	if secs, err := strconv.Atoi(raw); err == nil {
		if secs <= 0 {
			// "Retry-After: 0" names no wait: not a reset time.
			return time.Time{}
		}
		at = now.Add(time.Duration(secs) * time.Second)
	} else if t, err := http.ParseTime(raw); err == nil {
		at = t
	} else {
		return time.Time{}
	}
	if at.Sub(now) > quotaRetryAtHorizon {
		return time.Time{}
	}
	return at.UTC()
}

// syntheticResetAt reads x-synthetic-quotas for the reset of the limit
// that is spent: the weekly plan's nextRegenAt when the plan is at zero,
// else the subscription window's renewsAt, else nextRegenAt (review r1
// DeepSeek B2). Only a future time within the horizon counts.
func syntheticResetAt(h http.Header, now time.Time) time.Time {
	raw := strings.TrimSpace(h.Get("x-synthetic-quotas"))
	if raw == "" {
		return time.Time{}
	}
	var q struct {
		Subscription struct {
			RenewsAt string `json:"renewsAt"`
		} `json:"subscription"`
		Plan struct {
			NextRegenAt string `json:"nextRegenAt"`
		} `json:"planWeeklyLimit"`
	}
	if json.Unmarshal([]byte(raw), &q) != nil {
		return time.Time{}
	}
	future := func(v string) time.Time {
		t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(v))
		if err != nil || !t.After(now) || t.Sub(now) > quotaRetryAtHorizon {
			return time.Time{}
		}
		return t.UTC()
	}
	regen, renews := future(q.Plan.NextRegenAt), future(q.Subscription.RenewsAt)
	switch {
	case syntheticWeeklyExhausted(h) && !regen.IsZero():
		return regen
	case !renews.IsZero():
		return renews
	}
	return regen
}

// quotaResetAt is when a quota 429 is expected to clear: Retry-After first,
// then the provider's own reset header.
func quotaResetAt(providerName string, h http.Header, now time.Time) time.Time {
	if at := retryAfterAt(h, now); !at.IsZero() {
		return at
	}
	if providerName == "synthetic" {
		return syntheticResetAt(h, now)
	}
	return time.Time{}
}

func newQuotaError(providerName string, resp *http.Response) *QuotaExhaustedError {
	return &QuotaExhaustedError{Provider: providerName, Status: resp.Status, RetryAt: quotaResetAt(providerName, resp.Header, time.Now())}
}

// IsQuotaExhausted reports whether err is an exhausted upstream quota.
func IsQuotaExhausted(err error) bool {
	return errors.Is(err, ErrUpstreamQuotaExhausted)
}

// quotaReasonMarkers are the 429 reasons that mean a spent quota or plan
// limit rather than a momentary rate or concurrency limit. Matched on the
// lowercased, redacted reason logRejectedRequest returns.
var quotaReasonMarkers = []string{
	"usage_limit", "usage limit", "usage_not_included", "insufficient_quota",
	"quota", "subscription", "upgrade", "limit has been reached", "plan limit",
}

func reasonSaysQuota(reason string) bool {
	r := strings.ToLower(reason)
	for _, m := range quotaReasonMarkers {
		if strings.Contains(r, m) {
			return true
		}
	}
	return false
}

// syntheticRateLimitIsQuota reports whether a Synthetic 429 is quota
// exhaustion. Synthetic answers both its account concurrency limit ("Too
// many concurrent requests") and its subscription limits with 429. A zero
// weekly plan percentage in x-synthetic-quotas is quota whatever the body;
// otherwise the body decides, and an unrecognized body stays a concurrency
// limit (the onboarding rule: a concurrency burst must not demote the model,
// docs/problems/synthetic-provider-onboarding-2026-10-22.md).
func syntheticRateLimitIsQuota(reason string, h http.Header) bool {
	if syntheticWeeklyExhausted(h) {
		return true
	}
	if strings.Contains(strings.ToLower(reason), "concurrent") {
		return false
	}
	// Synthetic sends no Retry-After on its concurrency 429; one that names
	// a reset time is a limit to wait out, not a burst.
	return reasonSaysQuota(reason) || !retryAfterAt(h, time.Now()).IsZero()
}

// syntheticWeeklyExhausted reports whether x-synthetic-quotas says the weekly
// plan has no percentage left.
func syntheticWeeklyExhausted(h http.Header) bool {
	raw := strings.TrimSpace(h.Get("x-synthetic-quotas"))
	if raw == "" {
		return false
	}
	var q struct {
		Plan struct {
			PercentRemaining *float64 `json:"percentRemaining"`
		} `json:"planWeeklyLimit"`
	}
	if json.Unmarshal([]byte(raw), &q) != nil || q.Plan.PercentRemaining == nil {
		return false
	}
	return *q.Plan.PercentRemaining <= 0
}

// chatGPTStatusError is the ChatGPT error for a non-2xx answer. It logs the
// redacted reason like Synthetic's (a ChatGPT 429 used to leave no cause in
// the journal), types an exhausted usage limit as quota, and otherwise keeps
// the sanitized status text. suffix follows the status ("after auth
// refresh"). It drains and closes the body.
func chatGPTStatusError(resp *http.Response, suffix string) error {
	reason := logRejectedRequest("chatgpt", resp)
	_ = resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests && (reasonSaysQuota(reason) || !retryAfterAt(resp.Header, time.Now()).IsZero()) {
		return newQuotaError("chatgpt", resp)
	}
	if suffix != "" {
		return fmt.Errorf("chatgpt: status %s %s (sanitized)", resp.Status, suffix)
	}
	return fmt.Errorf("chatgpt: status %s (sanitized)", resp.Status)
}
