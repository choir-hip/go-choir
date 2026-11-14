package searchplane

import (
	"context"
	"errors"
	"testing"
	"time"
)

// docs/problems/keenable-auth-errors-leave-searches-with-no-provider-2026-11-14.md
// (Repair): for an uncapped provider auth_error is one more transient class.

const keenable403 = "status 403 Forbidden: access denied"

func failing403ThenOK(failures int32) *stubProvider {
	p := failingThenOK(failures)
	n := int32(0)
	p.searchFn = func(ctx context.Context, q string, max int) ([]Result, error) {
		n++
		if n <= failures {
			return nil, errors.New(keenable403)
		}
		return manyResults("keenable", 3), nil
	}
	return p
}

func TestOneKeenable403IsAnsweredByKeenableNextSearch(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			keen := failing403ThenOK(1)
			serper := &stubProvider{name: "serper"}
			router := NewRouter([]Provider{keen, serper}, store, Config{Routing: RoutingBudget, RequestTimeout: time.Second})
			if _, err := store.RecordOutcome(Outcome{Provider: "serper", Class: OutcomeQuotaLimited, Error: "status 402", At: time.Now()}); err != nil {
				t.Fatal(err)
			}
			_, first := startAndDrain(t, router)
			if len(first) != 1 || first[0].Provider != "keenable" || first[0].Answered() {
				t.Fatalf("first search arrivals = %+v, want one failed keenable call", first)
			}
			stream, second := startAndDrain(t, router)
			if cooling(stream, "keenable") {
				t.Fatalf("keenable skipped after one 403: %+v", stream.Cooling)
			}
			if len(second) != 1 || !second[0].Answered() {
				t.Fatalf("second search arrivals = %+v, want keenable's answer", second)
			}
		})
	}
}

func TestFive403sBenchBriefAndOneProbeGoesOut(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			clock := newFakeClock()
			keen := slowFailing("keenable", clock, time.Second, errors.New(keenable403))
			router := NewRouter([]Provider{keen}, store, Config{Routing: RoutingBudget, RequestTimeout: 30 * time.Second})
			router.now = clock.Now
			for i := 1; i <= TransientStrikesBeforeCooldown; i++ {
				if stream, _ := startAndDrain(t, router); cooling(stream, "keenable") {
					t.Fatalf("search %d skipped keenable before the run of %d", i, TransientStrikesBeforeCooldown)
				}
			}
			rec, _ := store.Get("keenable")
			if rec.CooldownUntil == nil || rec.CooldownUntil.Sub(clock.Now()) != TransientBackoff {
				t.Fatalf("after five 403s: %+v, want a bench of %s", rec, TransientBackoff)
			}
			calls := keen.searchCnt.Load()
			if stream, _ := startAndDrain(t, router); !cooling(stream, "keenable") || keen.searchCnt.Load() != calls {
				t.Fatalf("keenable called during the bench")
			}
			clock.Advance(TransientBackoff + time.Second)
			if !router.needsProbe(mustGet(t, store, "keenable")) {
				t.Fatalf("needsProbe false for a five-403 streak")
			}
			// Exactly one probe: it is out, a concurrent search skips.
			release := make(chan struct{})
			started := make(chan struct{}, 4)
			keen.searchFn = func(ctx context.Context, q string, max int) ([]Result, error) {
				started <- struct{}{}
				<-release
				return nil, errors.New(keenable403)
			}
			first, err := router.Start(context.Background(), "q", 5)
			if err != nil {
				t.Fatal(err)
			}
			<-started
			second, _ := router.Start(context.Background(), "q2", 5)
			if !cooling(second, "keenable") {
				t.Fatalf("second search called keenable while the probe was out")
			}
			drain(t, second)
			close(release)
			drain(t, first)
			waitProbesReleased(t, router)
			if got := keen.searchCnt.Load(); got != calls+1 {
				t.Fatalf("calls = %d, want %d (one probe)", got, calls+1)
			}
			rec, _ = store.Get("keenable")
			if rec.CooldownUntil == nil || !rec.CooldownUntil.After(clock.Now()) {
				t.Fatalf("failed probe did not bench again: %+v", rec)
			}
		})
	}
}

func mustGet(t *testing.T, s HealthStore, name string) ProviderHealth {
	t.Helper()
	rec, err := s.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestMetered403StillBenchesFifteenMinutes(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			clock := newFakeClock()
			serper := slowFailing("serper", clock, time.Second, errors.New(keenable403))
			router := NewRouter([]Provider{serper}, store, Config{Routing: RoutingBudget, RequestTimeout: 30 * time.Second})
			router.now = clock.Now
			startAndDrain(t, router)
			rec, _ := store.Get("serper")
			if rec.CooldownUntil == nil || rec.CooldownUntil.Sub(clock.Now()) != 15*time.Minute {
				t.Fatalf("serper after one 403: %+v, want 15 minutes", rec)
			}
		})
	}
}

func TestAuthAndTransientFailuresAreOneStreak(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			at := time.Now()
			rec := func(class OutcomeClass) ProviderHealth {
				r, err := store.RecordOutcome(Outcome{Provider: "keenable", Class: class, Uncapped: true, Error: keenable403, At: at})
				if err != nil {
					t.Fatal(err)
				}
				at = at.Add(time.Second)
				return r
			}
			for _, c := range []OutcomeClass{OutcomeError, OutcomeServerError, OutcomeAuthError, OutcomeTimeout} {
				if r := rec(c); r.State == StateCoolingDown {
					t.Fatalf("%s benched before the fifth strike: %+v", c, r)
				}
			}
			r := rec(OutcomeAuthError)
			if r.StrikeCount != TransientStrikesBeforeCooldown || r.CooldownUntil == nil || r.CooldownUntil.Sub(at.Add(-time.Second)) != TransientBackoff {
				t.Fatalf("fifth strike (a 403 after four mixed): %+v, want streak 5 and a %s bench", r, TransientBackoff)
			}
			// A 403 first, then transient ones: same streak, same bench.
			at = at.Add(time.Minute)
			rec2 := func(provider string, class OutcomeClass) ProviderHealth {
				at = at.Add(time.Second)
				r, err := store.RecordOutcome(Outcome{Provider: provider, Class: class, Uncapped: true, Error: keenable403, At: at})
				if err != nil {
					t.Fatal(err)
				}
				return r
			}
			rec2("searxng", OutcomeAuthError)
			for i := 0; i < 3; i++ {
				rec2("searxng", OutcomeError)
			}
			if r = rec2("searxng", OutcomeTimeout); r.StrikeCount != 5 || r.CooldownUntil == nil {
				t.Fatalf("403 then four transient: %+v, want streak 5 benched", r)
			}
			// A success between resets the run.
			for i := 0; i < 4; i++ {
				rec2("serpapi", OutcomeAuthError)
			}
			rec2("serpapi", OutcomeSuccess)
			if r = rec2("serpapi", OutcomeAuthError); r.StrikeCount != 1 || r.State == StateCoolingDown {
				t.Fatalf("403 after a success: %+v, want streak 1, no bench", r)
			}
		})
	}
}

// A real refusal ends the uncapped streak and is its own first strike, in
// either direction: four 403s then a 429 is a 60 s bench, not the fifth
// strike's 15 minutes (Gemini review of a5100a629; the same inheritance
// existed for transient errors then a 429 since 2026-11-13).
func TestRefusalAndUncappedStreakDoNotShareStrikes(t *testing.T) {
	policy := DefaultBackoffPolicy()
	for _, from := range []OutcomeClass{OutcomeAuthError, OutcomeError, OutcomeServerError, OutcomeTimeout} {
		for _, to := range []OutcomeClass{OutcomeRateLimited, OutcomeQuotaLimited} {
			for name, store := range stores(t) {
				t.Run(string(from)+"_then_"+string(to)+"/"+name, func(t *testing.T) {
					at := time.Now()
					rec := func(class OutcomeClass) ProviderHealth {
						at = at.Add(time.Minute)
						r, err := store.RecordOutcome(Outcome{Provider: "keenable", Class: class, Uncapped: true, Error: "status 4xx", At: at})
						if err != nil {
							t.Fatal(err)
						}
						return r
					}
					for i := 0; i < TransientStrikesBeforeCooldown-1; i++ {
						rec(from)
					}
					r := rec(to)
					if r.StrikeCount != 1 || r.CooldownUntil == nil || r.CooldownUntil.Sub(at) != policy.CooldownDuration(to, 1) {
						t.Fatalf("%s after four %s: %+v, want strike 1 and a %s bench", to, from, r, policy.CooldownDuration(to, 1))
					}
					// And back: the uncapped streak starts at 1, not after the refusal.
					at = at.Add(24 * time.Hour)
					if r = rec(from); r.StrikeCount != 1 || r.State == StateCoolingDown {
						t.Fatalf("%s after %s: %+v, want a fresh streak of 1 and no bench", from, to, r)
					}
				})
			}
		}
	}
}
