package searchplane

import (
	"testing"
	"time"
)

// Keenable is not paced against a monthly allowance (owner, 2026-11-08:
// "Keenable has quota!! Don't forget Keenable."; "Keenable is abundant. It
// shouldn't be showing an outage."). Failure written first: the intra-day
// accrual held it "capped" at 1858 calls of a 3060 day, so searches got no
// results. Keenable is limited only by its per-second rate and its own
// refusals (429 / quota answers cool it through the health store).
func TestKeenableIsNeverCappedByCallCount(t *testing.T) {
	for _, name := range []string{"CHOIR_SEARCH_PROVIDER_RPS", "CHOIR_SEARCH_PROVIDER_DAILY_CAPS"} {
		t.Setenv(name, "")
	}
	cfg := ConfigFromEnv()
	cfg.MinInterval = nil // the per-second pace is tested separately
	router := NewRouter([]Provider{countingKeenable()}, NewMemoryHealthStore(), cfg)
	day := mustDay("2026-11-08")
	for _, at := range []time.Duration{0, 30 * time.Minute, 13 * time.Hour, 23*time.Hour + 59*time.Minute} {
		now := day.Add(at)
		for i := 0; i < 5000; i++ { // far above the old ~3000 day cap
			router.counter.ReserveCall("keenable", DayOf(now), NoCap)
		}
		l, cooling := router.admit(countingKeenable(), now, &Route{})
		if cooling != nil || l.provider == nil {
			t.Fatalf("at %v keenable not admitted: %+v", at, cooling)
		}
	}
}
