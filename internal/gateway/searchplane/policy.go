package searchplane

import (
	"context"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// EmptyStreakBeforeCooldown is how many empty results in a row, with no
// success between them, bench a provider. One empty result is usually the
// query's doing (a narrow site: filter, a date the index lacks), not the
// provider's; benching on it took healthy providers out for every caller
// (docs/problems/search-plane-empty-result-benches-healthy-providers-2026-10-11.md).
const EmptyStreakBeforeCooldown = 10

// BackoffPolicy computes cooldown durations from outcome class and strike count.
type BackoffPolicy struct {
	BaseSeconds map[OutcomeClass]int64
	// MaxByClass bounds each class's backoff below MaxCooldown, so a
	// transient fault (a rate limit, a malformed body) can never bench a
	// provider for days, and an exhausted quota is probed every few hours
	// (docs/problems/search-providers-cooling-most-of-the-time-2026-10-14.md).
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
	d := time.Duration(float64(base) * exp)
	if limit, ok := p.MaxByClass[class]; ok && limit > 0 && d > limit {
		d = limit
	}
	if p.MaxCooldown > 0 && d > p.MaxCooldown {
		return p.MaxCooldown
	}
	return d
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
