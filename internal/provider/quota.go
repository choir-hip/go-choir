package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrUpstreamQuotaExhausted: the provider answered 429 because the account's
// quota or usage limit is spent (Synthetic "You've exceeded your subscription
// rate limits", a zero weekly plan percentage; ChatGPT "usage_limit_reached").
// Unlike a concurrency 429 it does not clear in seconds, so the gateway puts
// the model in cooldown on the first one and the pool routes to its next
// member (docs/design/model-pool-autoroute-2026-11-02.md).
var ErrUpstreamQuotaExhausted = errors.New("upstream quota exhausted")

// upstreamQuotaError keeps the sanitized status text and matches
// ErrUpstreamQuotaExhausted. The text names the quota so the guest reads it
// as a failover class.
type upstreamQuotaError struct {
	provider string
	status   string
}

func (e *upstreamQuotaError) Error() string {
	return fmt.Sprintf("%s: quota exhausted: status %s (sanitized)", e.provider, e.status)
}

func (e *upstreamQuotaError) Is(target error) bool {
	return target == ErrUpstreamQuotaExhausted
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
	return reasonSaysQuota(reason)
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
	if resp.StatusCode == http.StatusTooManyRequests && reasonSaysQuota(reason) {
		return &upstreamQuotaError{provider: "chatgpt", status: resp.Status}
	}
	if suffix != "" {
		return fmt.Errorf("chatgpt: status %s %s (sanitized)", resp.Status, suffix)
	}
	return fmt.Errorf("chatgpt: status %s (sanitized)", resp.Status)
}
