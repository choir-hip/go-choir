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
