package searchplane

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Ceramic starts every search; Keenable joins on shortfall under a daily
// cap (docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md,
// docs/reference/CERAMIC.md; mutation red: gateway provider calls).
// Failure list, written before the code:
//  1. A search starts Keenable, or does not start Ceramic.
//  2. Keenable does not join when Ceramic's answer is short, or after the
//     escalation timer when Ceramic is slow.
//  3. Keenable is called at its cap of 800 a day, or the cap fails the
//     search instead of returning Ceramic's results.
//  4. When Ceramic declines the query, is benched, or is quota_limited,
//     Keenable waits for the 8 s escalation timer (a search waiting on a
//     timer for a provider that was never called).
//  5. A declined query is counted, called, or strikes Ceramic.
//  6. One Ceramic error benches it (it starts every search: it is policed
//     like the free tier), or a 402 benches it for good and never recovers.
//  7. A rate limit that says how long to wait is benched for the default
//     60 s instead.
//  8. SearXNG and Brave stop starting with every search; the defaults in
//     config drift (tiers, cap, pacing).

type decliningProvider struct {
	*stubProvider
	declines func(query string) bool
}

func (d *decliningProvider) DeclinesQuery(q string) bool { return d.declines(q) }

// ceramicConfig is the production defaults with a long escalation timer, so
// a test that passes by waiting for the timer cannot.
func ceramicConfig() Config {
	cfg := budgetConfig()
	cfg.Tiers = ParseProviderTiers(DefaultProviderTiers)
	cfg.DailyCaps = ParseDailyCaps(DefaultDailyCaps, "")
	cfg.EscalateAfter = time.Hour
	cfg.EscalateWindow = time.Hour
	cfg.MaxPaid = 2
	return cfg
}

func startsAndDrains(t *testing.T, router *Router) (*Stream, []Arrival) {
	t.Helper()
	began := time.Now()
	stream, err := router.Start(context.Background(), "q", 40)
	if err != nil {
		t.Fatal(err)
	}
	arrivals := drain(t, stream)
	if took := time.Since(began); took > 5*time.Second {
		t.Fatalf("search took %v: it waited on a timer", took)
	}
	return stream, arrivals
}

func TestCeramic_ASearchStartsCeramicAndNotKeenable(t *testing.T) { // 1
	var l callLog
	cer, keen := logged(&l, "ceramic", 0, 12), logged(&l, "keenable", 0, 12)
	router := NewRouter([]Provider{cer, keen}, NewMemoryHealthStore(), ceramicConfig())
	stream, arrivals := startsAndDrains(t, router)
	if fmt.Sprint(stream.Providers) != "[ceramic]" || names(arrivals) != "ceramic" || keen.searchCnt.Load() != 0 {
		t.Fatalf("providers=%v arrivals=%s keenable calls=%d", stream.Providers, names(arrivals), keen.searchCnt.Load())
	}
	if got := fmt.Sprint(stream.Route.Summary().Reserve); got != "[keenable]" {
		t.Fatalf("reserve = %s, want keenable held", got)
	}
	if stream.Route.Summary().PaidCalls != 0 {
		t.Fatalf("a search Ceramic answered counted a paid call")
	}
}

func TestCeramic_KeenableJoinsWhenCeramicIsShort(t *testing.T) { // 2
	var l callLog
	router := NewRouter([]Provider{logged(&l, "ceramic", 0, 3), logged(&l, "keenable", 0, 12), logged(&l, "serper", 0, 12)}, NewMemoryHealthStore(), ceramicConfig())
	stream, arrivals := startsAndDrains(t, router)
	if names(arrivals) != "ceramic,keenable" || reasons(stream.Route.Summary()) != "short:keenable" {
		t.Fatalf("arrivals=%s escalations=%s, want keenable (cheaper than serper) after a short answer", names(arrivals), reasons(stream.Route.Summary()))
	}
}

func TestCeramic_KeenableJoinsAfterTheEscalateTimer(t *testing.T) { // 2
	var l callLog
	cfg := ceramicConfig()
	cfg.EscalateAfter = 50 * time.Millisecond
	router := NewRouter([]Provider{logged(&l, "ceramic", 400*time.Millisecond, 12), logged(&l, "keenable", 0, 12)}, NewMemoryHealthStore(), cfg)
	stream, arrivals := startsAndDrains(t, router)
	if names(arrivals) != "keenable,ceramic" || reasons(stream.Route.Summary()) != "slow:keenable" {
		t.Fatalf("arrivals=%s escalations=%s", names(arrivals), reasons(stream.Route.Summary()))
	}
}

func TestCeramic_KeenableAtItsCapIsSkippedAndCeramicStillAnswers(t *testing.T) { // 3
	var l callLog
	keen := logged(&l, "keenable", 0, 12)
	router := NewRouter([]Provider{logged(&l, "ceramic", 0, 3), keen}, NewMemoryHealthStore(), ceramicConfig())
	if DefaultDailyCaps == "" || router.capOf("keenable", time.Now()) != 800 {
		t.Fatalf("keenable cap = %d, want 800", router.capOf("keenable", time.Now()))
	}
	day := DayOf(time.Now())
	for i := 0; i < 800; i++ {
		if _, ok, _ := router.counter.ReserveCall("keenable", day, 800); !ok {
			t.Fatalf("call %d refused below the cap", i)
		}
	}
	stream, arrivals := startsAndDrains(t, router)
	if names(arrivals) != "ceramic" || !arrivals[0].Answered() || len(arrivals[0].Results) != 3 || keen.searchCnt.Load() != 0 {
		t.Fatalf("arrivals=%+v keenable calls=%d, want ceramic's 3 results and no keenable call", arrivals, keen.searchCnt.Load())
	}
	var capped bool
	for _, c := range stream.Cooling {
		if c.Provider == "keenable" && c.State == StateCapped && c.Reason == "capped" {
			capped = true
		}
	}
	if !capped {
		t.Fatalf("cooling = %+v, want keenable capped", stream.Cooling)
	}
	if rec, _ := router.store.Get("keenable"); rec.StrikeCount != 0 {
		t.Fatalf("the cap struck keenable: %+v", rec)
	}
}

// The 801st call of a day is the first refused, under concurrency.
func TestCeramic_KeenableCapIs800Calls(t *testing.T) { // 3
	router := NewRouter([]Provider{delayedProvider("keenable", 0, 1)}, NewMemoryHealthStore(), ceramicConfig())
	day := DayOf(time.Now())
	var last int
	for i := 1; i <= 801; i++ {
		calls, ok, _ := router.counter.ReserveCall("keenable", day, router.capOf("keenable", time.Now()))
		if (i <= 800) != ok {
			t.Fatalf("call %d: ok=%v calls=%d", i, ok, calls)
		}
		last = calls
	}
	if last != 800 {
		t.Fatalf("count after refusal = %d", last)
	}
}

func TestCeramic_KeenableStartsAtOnceWhenCeramicCannotRun(t *testing.T) { // 4, 5
	benched := func(class OutcomeClass, errText string) func(*MemoryHealthStore) {
		return func(s *MemoryHealthStore) {
			s.RecordOutcome(Outcome{Provider: "ceramic", Class: class, Uncapped: true, Error: errText, At: time.Now()})
			if class == OutcomeAuthError || class == OutcomeError {
				// An uncapped provider needs a run of strikes before a bench.
				for i := 1; i < TransientStrikesBeforeCooldown; i++ {
					s.RecordOutcome(Outcome{Provider: "ceramic", Class: class, Uncapped: true, Error: errText, At: time.Now()})
				}
			}
		}
	}
	cases := map[string]struct {
		setup  func(*MemoryHealthStore)
		decl   bool
		reason string
	}{
		"declines the query":  {decl: true, reason: "unsupported_query"},
		"quota_limited":       {setup: benched(OutcomeQuotaLimited, "status 402 Payment Required"), reason: "quota_limited"},
		"rate_limited":        {setup: benched(OutcomeRateLimited, "status 429"), reason: "rate_limited"},
		"benched after a run": {setup: benched(OutcomeError, "status 500"), reason: "error"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			store := NewMemoryHealthStore()
			if c.setup != nil {
				c.setup(store)
			}
			cer := &decliningProvider{stubProvider: delayedProvider("ceramic", 0, 12), declines: func(string) bool { return c.decl }}
			keen := delayedProvider("keenable", 0, 12)
			router := NewRouter([]Provider{cer, keen}, store, ceramicConfig()) // EscalateAfter is an hour
			before, _ := store.CallsOn(DayOf(time.Now()))
			stream, arrivals := startsAndDrains(t, router)
			if fmt.Sprint(stream.Providers) != "[keenable]" || names(arrivals) != "keenable" || !arrivals[0].Answered() {
				t.Fatalf("providers=%v arrivals=%s, want keenable alone, at once", stream.Providers, names(arrivals))
			}
			if got := reasons(stream.Route.Summary()); got != "no_free:keenable" {
				t.Fatalf("escalations = %s, want no_free:keenable", got)
			}
			if cer.searchCnt.Load() != 0 {
				t.Fatal("ceramic was called")
			}
			found := false
			for _, cool := range stream.Cooling {
				if cool.Provider == "ceramic" && cool.Reason == c.reason {
					found = true
				}
			}
			if !found {
				t.Fatalf("cooling = %+v, want ceramic with reason %s", stream.Cooling, c.reason)
			}
			if c.decl {
				after, _ := store.CallsOn(DayOf(time.Now()))
				if after["ceramic"] != before["ceramic"] {
					t.Fatalf("a declined query was counted: %v", after)
				}
				if rec, _ := store.Get("ceramic"); rec.StrikeCount != 0 || rec.State != StateActive {
					t.Fatalf("a declined query struck ceramic: %+v", rec)
				}
				if stream.Cooling[0].State != StateDeclined {
					t.Fatalf("state = %s", stream.Cooling[0].State)
				}
			}
		})
	}
}

func TestCeramic_A402FallsThroughAtOnceAndRecoversAfterTheBench(t *testing.T) { // 4, 6
	store := NewMemoryHealthStore()
	paid := false
	cer := &stubProvider{name: "ceramic", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		if !paid {
			return nil, errors.New("status 402 Payment Required: Payment Required")
		}
		return manyResults("ceramic", 12), nil
	}}
	keen := delayedProvider("keenable", 0, 12)
	router := NewRouter([]Provider{cer, keen}, store, ceramicConfig())

	stream, arrivals := startsAndDrains(t, router)
	if names(arrivals) != "ceramic,keenable" || arrivals[0].Status != "quota_limited" || reasons(stream.Route.Summary()) != "failed:keenable" {
		t.Fatalf("arrivals=%+v escalations=%s, want the 402 then keenable, at once", arrivals, reasons(stream.Route.Summary()))
	}
	rec, _ := store.Get("ceramic")
	if rec.State != StateCoolingDown || rec.LastFailureClass != "quota_limited" || rec.CooldownUntil == nil {
		t.Fatalf("after a 402: %+v", rec)
	}
	if bench := time.Until(*rec.CooldownUntil); bench < 59*time.Minute || bench > time.Hour {
		t.Fatalf("a first 402 benches %v, want one hour", bench)
	}
	// While benched: keenable answers alone at once.
	stream, _ = startsAndDrains(t, router)
	if fmt.Sprint(stream.Providers) != "[keenable]" {
		t.Fatalf("benched: providers = %v", stream.Providers)
	}
	// The payment clears and the hour passes, with no restart: the bench is
	// a deadline in the store, and the next search calls ceramic again.
	paid = true
	store.ResetProvider("ceramic")
	store.RecordOutcome(Outcome{Provider: "ceramic", Class: OutcomeQuotaLimited, Uncapped: true, Error: "status 402", At: time.Now().Add(-61 * time.Minute)})
	stream, arrivals = startsAndDrains(t, router)
	if fmt.Sprint(stream.Providers) != "[ceramic]" || !arrivals[0].Answered() {
		t.Fatalf("after the bench: providers=%v arrivals=%+v, want ceramic back", stream.Providers, arrivals)
	}
	if rec, _ := store.Get("ceramic"); rec.StrikeCount != 0 || rec.State != StateActive {
		t.Fatalf("after a success: %+v", rec)
	}
}

func TestCeramic_OneErrorDoesNotBenchIt(t *testing.T) { // 6
	store := NewMemoryHealthStore()
	n := 0
	cer := &stubProvider{name: "ceramic", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		n++
		if n == 1 {
			return nil, errors.New("status 503 Service Unavailable")
		}
		return manyResults("ceramic", 12), nil
	}}
	router := NewRouter([]Provider{cer, delayedProvider("keenable", 0, 12)}, store, ceramicConfig())
	startsAndDrains(t, router)
	stream, arrivals := startsAndDrains(t, router)
	if fmt.Sprint(stream.Providers) != "[ceramic]" || !arrivals[0].Answered() {
		t.Fatalf("after one 503: providers=%v arrivals=%+v", stream.Providers, arrivals)
	}
}

type waitError struct{ wait time.Duration }

func (e waitError) Error() string             { return "status 429 Too Many Requests" }
func (e waitError) RetryAfter() time.Duration { return e.wait }

func TestCeramic_ARateLimitThatSaysHowLongIsHonoured(t *testing.T) { // 7
	for name, c := range map[string]struct {
		err  error
		want time.Duration
	}{
		"asked 7 s":       {waitError{7 * time.Second}, 7 * time.Second},
		"asked an hour":   {waitError{time.Hour}, 15 * time.Minute}, // the class's bound
		"said nothing":    {errors.New("status 429 Too Many Requests"), time.Minute},
		"wrapped request": {fmt.Errorf("http request: %w", waitError{9 * time.Second}), 9 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			store := NewMemoryHealthStore()
			cer := &stubProvider{name: "ceramic", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) { return nil, c.err }}
			router := NewRouter([]Provider{cer, delayedProvider("keenable", 0, 12)}, store, ceramicConfig())
			startsAndDrains(t, router)
			rec, _ := store.Get("ceramic")
			if rec.LastFailureClass != "rate_limited" || rec.CooldownUntil == nil {
				t.Fatalf("rec = %+v", rec)
			}
			if got := time.Until(*rec.CooldownUntil); got > c.want || got < c.want-2*time.Second {
				t.Fatalf("bench = %v, want %v", got, c.want)
			}
		})
	}
}

func TestCeramic_SearXNGAndBraveStillStartWithEverySearch(t *testing.T) { // 8
	var l callLog
	providers := []Provider{logged(&l, "searxng", 0, 4), logged(&l, "brave", 0, 4), logged(&l, "ceramic", 0, 4), logged(&l, "keenable", 0, 12), logged(&l, "serper", 0, 12)}
	router := NewRouter(providers, NewMemoryHealthStore(), ceramicConfig())
	stream, arrivals := startsAndDrains(t, router)
	if fmt.Sprint(stream.Providers) != "[searxng brave ceramic]" {
		t.Fatalf("providers = %v", stream.Providers)
	}
	// 12 distinct results from the first step is enough: nobody joins.
	if len(arrivals) != 3 || len(stream.Route.Summary().Escalations) != 0 {
		// searxng, brave and ceramic return disjoint URLs: 12 >= MinResults 10.
		t.Fatalf("arrivals=%s escalations=%s", names(arrivals), reasons(stream.Route.Summary()))
	}
}

func TestCeramic_Defaults(t *testing.T) { // 8
	for _, name := range []string{"CHOIR_SEARCH_PROVIDER_RPS", "CHOIR_SEARCH_PROVIDER_DAILY_CAPS", "CHOIR_SEARCH_PROVIDER_TIERS"} {
		t.Setenv(name, "")
	}
	cfg := ConfigFromEnv()
	ceramic, keenable, serper := cfg.Tiers["ceramic"], cfg.Tiers["keenable"], cfg.Tiers["serper"]
	if ceramic.Tier != TierFirst || !ceramic.startsSearch() || ceramic.Tier == TierFree {
		t.Fatalf("ceramic tier = %+v, want the prepaid starts-every-search tier, not free", ceramic)
	}
	if keenable.Tier != TierPaid || keenable.Cost >= serper.Cost || keenable.Cost >= cfg.Tiers["parallel"].Cost {
		t.Fatalf("keenable tier = %+v, want paid and the cheapest to join (serper %+v)", keenable, serper)
	}
	if cfg.DailyCaps["keenable"] != 800 {
		t.Fatalf("keenable cap = %d", cfg.DailyCaps["keenable"])
	}
	if cfg.MinInterval["ceramic"] != time.Second/15 || cfg.MinInterval["keenable"] != 125*time.Millisecond || cfg.MinInterval["brave"] != time.Second {
		t.Fatalf("pacing = %v", cfg.MinInterval)
	}
	if cfg.Tiers["searxng"].Tier != TierFree || cfg.Tiers["brave"].Tier != TierFree {
		t.Fatalf("tiers = %v", cfg.Tiers)
	}
	if !strings.Contains(FormatDailyCaps(cfg.DailyCaps), "keenable=800") {
		t.Fatal("cap not in the logged caps")
	}
}
