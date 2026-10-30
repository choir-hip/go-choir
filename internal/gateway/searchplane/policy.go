package searchplane

import (
	"context"
	"errors"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EmptyStreakBeforeCooldown is how many empty results in a row, with no
// success between them, bench a provider. One empty result is usually the
// query's doing (a narrow site: filter, a date the index lacks), not the
// provider's; benching on it took healthy providers out for every caller
// (docs/archive/problems/search-plane-empty-result-benches-healthy-providers-2026-10-11.md).
const EmptyStreakBeforeCooldown = 10

// BackoffPolicy computes cooldown durations from outcome class and strike count.
type BackoffPolicy struct {
	BaseSeconds map[OutcomeClass]int64
	// MaxByClass bounds each class's backoff below MaxCooldown, so a
	// transient fault (a rate limit, a malformed body) can never bench a
	// provider for days, and an exhausted quota is probed every few hours
	// (docs/archive/problems/search-providers-cooling-most-of-the-time-2026-10-14.md).
	MaxByClass  map[OutcomeClass]time.Duration
	MaxCooldown time.Duration
	Multiplier  float64
}

// DefaultBackoffPolicy returns production defaults from the design doc.
func DefaultBackoffPolicy() BackoffPolicy {
	return BackoffPolicy{
		BaseSeconds: map[OutcomeClass]int64{
			OutcomeSuccessEmpty: 5 * 60,
			// A rate limit clears in seconds to minutes; a burst must not bench
			// a provider for a day. Quota exhaustion keeps the long bench.
			OutcomeRateLimited: 60,
			// Quota exhaustion does not clear in minutes, but a probe is one
			// call: an hour, doubling to six, sees an owner top-up the same day.
			OutcomeQuotaLimited: 60 * 60,
			OutcomeAuthError:    7 * 24 * 60 * 60,
			OutcomeServerError:  2 * 60,
			OutcomeTimeout:      2 * 60,
			OutcomeError:        5 * 60,
		},
		MaxByClass: map[OutcomeClass]time.Duration{
			OutcomeRateLimited:  15 * time.Minute,
			OutcomeServerError:  30 * time.Minute,
			OutcomeTimeout:      30 * time.Minute,
			OutcomeError:        time.Hour,
			OutcomeSuccessEmpty: 6 * time.Hour,
			OutcomeQuotaLimited: 6 * time.Hour,
		},
		MaxCooldown: 7 * 24 * time.Hour,
		Multiplier:  2,
	}
}

// BackoffPolicyFromEnv builds a policy with env overrides.
func BackoffPolicyFromEnv() BackoffPolicy {
	p := DefaultBackoffPolicy()
	if v := envInt64("CHOIR_SEARCH_BACKOFF_BASE_SECONDS", 0); v > 0 {
		p.BaseSeconds[OutcomeRateLimited] = v
		p.BaseSeconds[OutcomeQuotaLimited] = v
	}
	if v := envInt64("CHOIR_SEARCH_BACKOFF_MAX_SECONDS", 0); v > 0 {
		p.MaxCooldown = time.Duration(v) * time.Second
	}
	if v := envFloat("CHOIR_SEARCH_BACKOFF_MULTIPLIER", 0); v > 1 {
		p.Multiplier = v
	}
	return p
}

// CooldownDuration returns how long a provider should stay in cooling_down.
func (p BackoffPolicy) CooldownDuration(class OutcomeClass, strikeCount int) time.Duration {
	if class == OutcomeSuccess || class == OutcomeSkippedCoolingDown {
		return 0
	}
	if strikeCount < 1 {
		strikeCount = 1
	}
	if class == OutcomeSuccessEmpty {
		if strikeCount < EmptyStreakBeforeCooldown {
			return 0
		}
		strikeCount -= EmptyStreakBeforeCooldown - 1
	}
	baseSec, ok := p.BaseSeconds[class]
	if !ok || baseSec <= 0 {
		baseSec = 300
	}
	base := time.Duration(baseSec) * time.Second
	exp := math.Pow(p.Multiplier, float64(strikeCount-1))
	if exp < 1 {
		exp = 1
	}
	// Clamp in float before converting: an out-of-range float-to-int
	// conversion is implementation-defined (amd64 yields MinInt64, a
	// negative bench), and enough strikes overflow int64 nanoseconds.
	d := time.Duration(math.MaxInt64)
	if f := float64(base) * exp; f < float64(math.MaxInt64) {
		d = time.Duration(f)
	}
	if limit, ok := p.MaxByClass[class]; ok && limit > 0 && d > limit {
		d = limit
	}
	if p.MaxCooldown > 0 && d > p.MaxCooldown {
		return p.MaxCooldown
	}
	return d
}

// ClampCooldown bounds a stored deadline by the policy in force: at most
// UpdatedAt plus MaxByClass of the row's last failure class, and overall
// at most UpdatedAt plus MaxCooldown. A deadline persisted under an older,
// longer policy otherwise outlives the policy change that shortened it
// (docs/archive/problems/search-stale-cooldowns-outlive-policy-2026-10-15.md).
// It never lengthens a deadline, and a class with no MaxByClass entry
// (auth_error) is bounded by MaxCooldown only. UpdatedAt can be later than
// the moment the bench was set, which only loosens the clamp.
func (p BackoffPolicy) ClampCooldown(rec ProviderHealth) (ProviderHealth, bool) {
	if rec.CooldownUntil == nil || rec.UpdatedAt.IsZero() {
		return rec, false
	}
	limit := p.MaxCooldown
	if c, ok := p.MaxByClass[OutcomeClass(rec.LastFailureClass)]; ok && c > 0 && (limit <= 0 || c < limit) {
		limit = c
	}
	if limit <= 0 {
		return rec, false
	}
	ceiling := rec.UpdatedAt.Add(limit)
	if !rec.CooldownUntil.After(ceiling) {
		return rec, false
	}
	rec.CooldownUntil = &ceiling
	return rec, true
}

// ClassifyCall maps an error and result count to an outcome class.
func ClassifyCall(err error, resultCount int) OutcomeClass {
	if err == nil {
		if resultCount > 0 {
			return OutcomeSuccess
		}
		return OutcomeSuccessEmpty
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return OutcomeTimeout
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return OutcomeTimeout
	}
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		// "run out of searches" arrives as a 429 but does not clear in
		// minutes. Match phrases, not the bare word: a per-second limit's
		// body can carry quota_limit/quota_current fields.
		for _, phrase := range []string{"run out", "out of credits", "no more credits", "insufficient", "quota exceeded", "quota limit exceeded", "exceeded your quota", "monthly"} {
			if strings.Contains(msg, phrase) {
				return OutcomeQuotaLimited
			}
		}
		return OutcomeRateLimited
	}
	if strings.Contains(msg, "402") || strings.Contains(msg, "432") ||
		strings.Contains(msg, "payment required") ||
		strings.Contains(msg, "quota") ||
		strings.Contains(msg, "credits") ||
		strings.Contains(msg, "usage limit") {
		return OutcomeQuotaLimited
	}
	if strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
		strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "invalid api key") || strings.Contains(msg, "invalid key") {
		return OutcomeAuthError
	}
	if strings.Contains(msg, "500") || strings.Contains(msg, "502") || strings.Contains(msg, "503") ||
		strings.Contains(msg, "504") || strings.Contains(msg, "server error") {
		return OutcomeServerError
	}
	return OutcomeError
}

// AttemptStatus maps an outcome class to the attempt status string exposed in API traces.
func AttemptStatus(class OutcomeClass) string {
	switch class {
	case OutcomeSuccess:
		return "success"
	case OutcomeSuccessEmpty:
		return "success_empty"
	case OutcomeRateLimited:
		return "rate_limited"
	case OutcomeQuotaLimited:
		return "quota_limited"
	case OutcomeAuthError:
		return "auth_error"
	case OutcomeServerError:
		return "server_error"
	case OutcomeTimeout:
		return "timeout"
	case OutcomeSkippedCoolingDown:
		return "cooling_down"
	case OutcomeCallerGone:
		return "caller_gone"
	default:
		return "error"
	}
}

func envInt64(name string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return v
}

func envFloat(name string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return v
}

// failureStatusPattern finds the HTTP status the adapters put in an error
// ("status 402 Payment Required: ...").
var failureStatusPattern = regexp.MustCompile(`\bstatus (\d{3})\b`)

// failureStagePattern is an adapter's failing stage: the lowercase words
// before the first colon ("parse response", "http request").
var failureStagePattern = regexp.MustCompile(`^([a-z]+(?: [a-z]+){0,2}):`)

// FailureFields names why a provider call failed for the gateway log: the
// HTTP status ("-" when none was received) and the adapter stage that
// failed ("http_status" for a non-2xx answer, "parse_response",
// "http_request", "timeout", ... or "other"). Both come from a fixed
// shape (lowercase words, three digits), so the log line never carries a
// key, a query or a response body.
func FailureFields(err error) (status, stage string) {
	if err == nil {
		return "-", "none"
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "-", "timeout"
	}
	msg := err.Error()
	status = "-"
	if m := failureStatusPattern.FindStringSubmatch(msg); m != nil {
		status = m[1]
	}
	if strings.HasPrefix(msg, "status ") && status != "-" {
		return status, "http_status"
	}
	if m := failureStagePattern.FindStringSubmatch(msg); m != nil {
		return status, strings.ReplaceAll(m[1], " ", "_")
	}
	return status, "other"
}
