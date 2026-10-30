package searchplane

import (
	"errors"
	"testing"
	"time"
)

func TestBackoffPolicy_StrikeEscalation(t *testing.T) {
	p := BackoffPolicy{
		BaseSeconds: map[OutcomeClass]int64{OutcomeQuotaLimited: 60},
		MaxCooldown: 7 * 24 * time.Hour,
		Multiplier:  2,
	}
	d1 := p.CooldownDuration(OutcomeQuotaLimited, 1)
	d2 := p.CooldownDuration(OutcomeQuotaLimited, 2)
	if d2 <= d1 {
		t.Fatalf("strike escalation: d1=%s d2=%s", d1, d2)
	}
	if d1 != time.Minute {
		t.Fatalf("d1 = %s, want 1m", d1)
	}
	if d2 != 2*time.Minute {
		t.Fatalf("d2 = %s, want 2m", d2)
	}
}

func TestClassifyCall_QuotaLimited(t *testing.T) {
	class := ClassifyCall(errors.New("status 402 Payment Required: NO_MORE_CREDITS"), 0)
	if class != OutcomeQuotaLimited {
		t.Fatalf("class = %q, want %q", class, OutcomeQuotaLimited)
	}
}

// A per-second plan limit clears in seconds; benching the provider for a day
// discarded most of the plane after one burst of research children (S1c,
// docs/archive/problems/search-rate-limit-benches-provider-for-a-day-2026-10-11.md).
// Quota exhaustion keeps the long bench even when it arrives as a 429.
func TestRateLimitBenchesBrieflyQuotaExhaustionForADay(t *testing.T) {
	p := DefaultBackoffPolicy()
	brave := errors.New(`status 429 Too Many Requests: {"type":"ErrorResponse","error":{"status":429,"detail":"Request rate limit exceeded for plan","meta":{"plan":"Free AI","rate_limit":1,"rate_current":1,"quota_limit":2000,"quota_current":312}}}`)
	if class := ClassifyCall(brave, 0); class != OutcomeRateLimited {
		t.Fatalf("a plan rate limit is %q, want %q", class, OutcomeRateLimited)
	}
	if d := p.CooldownDuration(OutcomeRateLimited, 1); d > 2*time.Minute {
		t.Fatalf("a first rate-limit strike benches for %s; it should clear within minutes", d)
	}
	if d := p.CooldownDuration(OutcomeRateLimited, 3); d <= p.CooldownDuration(OutcomeRateLimited, 1) {
		t.Fatalf("repeated rate limits must still back off")
	}
	for _, msg := range []string{
		`status 429 Too Many Requests: {"error": "Your account has run out of searches"}`,
		`status 429: monthly quota exceeded`,
		`429 Too Many Requests: insufficient credits`,
	} {
		if class := ClassifyCall(errors.New(msg), 0); class != OutcomeQuotaLimited {
			t.Fatalf("%q is %q, want %q", msg, class, OutcomeQuotaLimited)
		}
	}
	// Quota exhaustion benches far longer than a rate limit, but is probed
	// hourly so an owner top-up is seen within hours, not days
	// (docs/archive/problems/search-providers-cooling-most-of-the-time-2026-10-14.md).
	if d := p.CooldownDuration(OutcomeQuotaLimited, 1); d < time.Hour {
		t.Fatalf("quota exhaustion must bench at least an hour, got %s", d)
	}
	if d := p.CooldownDuration(OutcomeQuotaLimited, 10); d > 6*time.Hour {
		t.Fatalf("quota exhaustion must be probed at least every 6h, got %s", d)
	}
	// Brave's monthly allowance arrives as a 429 whose body also carries the
	// per-second fields; its phrase marks it as quota.
	braveQuota := errors.New(`status 429 Too Many Requests: {"detail":"Request quota limit exceeded for plan","meta":{"plan":"Free AI","rate_limit":1,"quota_limit":2000,"quota_current":2000}}`)
	if class := ClassifyCall(braveQuota, 0); class != OutcomeQuotaLimited {
		t.Fatalf("brave monthly quota is %q, want %q", class, OutcomeQuotaLimited)
	}
}
