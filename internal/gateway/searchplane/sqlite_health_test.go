package searchplane

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileHealthStore_PersistsOutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.db")
	store, err := OpenFileHealthStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := store.RecordOutcome(Outcome{Provider: "brave", Class: OutcomeQuotaLimited, Error: "quota", At: time.Now()}); err != nil {
		t.Fatalf("record: %v", err)
	}
	reopened, err := OpenFileHealthStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	rec, err := reopened.Get("brave")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rec.StrikeCount != 1 {
		t.Fatalf("strike_count = %d, want 1", rec.StrikeCount)
	}
	if rec.State != StateCoolingDown {
		t.Fatalf("state = %q, want cooling_down", rec.State)
	}
}

func TestCooldownClamp_ErrorClassStaleDeadlineIsRetried(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.db")
	store, err := OpenFileHealthStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// A row as an older, 7-day-max policy wrote it.
	updated := time.Now().Add(-48 * time.Hour)
	until := updated.Add(7 * 24 * time.Hour)
	if err := store.upsert(ProviderHealth{Provider: "exa", State: StateCoolingDown, CooldownUntil: &until, StrikeCount: 14, LastFailureClass: string(OutcomeError), UpdatedAt: updated}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = store.Close()

	reopened, err := OpenFileHealthStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	rec, err := reopened.Get("exa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rec.State != StateActive || !IsEligible(rec, time.Now()) {
		t.Fatalf("exa state = %q, want active and eligible (deadline clamped to updated_at+1h, already past)", rec.State)
	}
	// The startup pass persisted the clamp: the raw row holds updated_at+1h.
	var raw string
	if err := reopened.db.QueryRow(`SELECT cooldown_until FROM provider_health WHERE provider='exa'`).Scan(&raw); err != nil {
		t.Fatalf("raw: %v", err)
	}
	got, _ := time.Parse(time.RFC3339Nano, raw)
	if want := updated.Add(time.Hour); !got.Equal(want) {
		t.Fatalf("stored cooldown_until = %s, want %s", got, want.UTC())
	}
}

func TestCooldownClamp_QuotaLimitedKeepsSixHourCap(t *testing.T) {
	updated := time.Now()
	until := updated.Add(7 * 24 * time.Hour)
	rec := ProviderHealth{Provider: "brave", State: StateCoolingDown, CooldownUntil: &until, LastFailureClass: string(OutcomeQuotaLimited), UpdatedAt: updated}
	got, changed := DefaultBackoffPolicy().ClampCooldown(rec)
	if !changed || !got.CooldownUntil.Equal(updated.Add(6*time.Hour)) {
		t.Fatalf("clamped = %v changed=%v, want updated_at+6h", got.CooldownUntil, changed)
	}
	within := updated.Add(3 * time.Hour)
	rec.CooldownUntil = &within
	if got, changed := DefaultBackoffPolicy().ClampCooldown(rec); changed || !got.CooldownUntil.Equal(within) {
		t.Fatalf("in-policy deadline was changed: %v", got.CooldownUntil)
	}
}

// A stored auth bench (state disabled, seven days, the pre-2026-11-05 rule)
// clamps to six hours from its last update and then expires: Keenable was
// benched from 2026-11-03 until a hand reset under the old rule.
func TestCooldownClamp_StoredAuthBenchExpires(t *testing.T) {
	updated := time.Now().Add(-48 * time.Hour)
	until := updated.Add(7 * 24 * time.Hour)
	rec := ProviderHealth{Provider: "keenable", State: StateDisabled, CooldownUntil: &until, LastFailureClass: string(OutcomeAuthError), UpdatedAt: updated}
	got, changed := DefaultBackoffPolicy().ClampCooldown(rec)
	if !changed || !got.CooldownUntil.Equal(updated.Add(6*time.Hour)) {
		t.Fatalf("clamped = %v changed=%v, want updated_at+6h", got.CooldownUntil, changed)
	}
	if !IsEligible(got, time.Now()) {
		t.Fatalf("expired auth bench still blocks: %+v", got)
	}
	if IsEligible(rec, updated.Add(time.Hour)) {
		t.Fatalf("auth bench admitted traffic before its deadline")
	}
}

// An auth error benches on the doubling schedule and the provider is probed
// again once the bench passes; it is never disabled for good.
func TestAuthErrorBacksOffAndRecovers(t *testing.T) {
	now := time.Now()
	var rec ProviderHealth
	p := DefaultBackoffPolicy()
	applyOutcome(&rec, Outcome{Provider: "keenable", Class: OutcomeAuthError, Error: "401"}, p, now)
	if rec.State != StateCoolingDown || !rec.CooldownUntil.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("first auth strike = %+v", rec)
	}
	if IsEligible(rec, now.Add(14*time.Minute)) || !IsEligible(rec, now.Add(16*time.Minute)) {
		t.Fatalf("auth bench eligibility wrong: %+v", rec)
	}
	later := now.Add(16 * time.Minute)
	applyOutcome(&rec, Outcome{Provider: "keenable", Class: OutcomeAuthError, Error: "401", At: later}, p, later)
	if !rec.CooldownUntil.Equal(later.Add(30 * time.Minute)) {
		t.Fatalf("second auth strike did not double: %v", rec.CooldownUntil)
	}
	applyOutcome(&rec, Outcome{Provider: "keenable", Class: OutcomeSuccess}, p, later.Add(time.Hour))
	if rec.State != StateActive || rec.StrikeCount != 0 {
		t.Fatalf("success did not clear the bench: %+v", rec)
	}
}
