package searchplane

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Repair of c2de94dd4 (review findings). Ways it can fail, written first:
//  1. A bench is anchored at the call's start, so after a 30 s timeout a 30 s
//     bench is already over when it is set and the dead provider is called by
//     every search. Metered providers share the anchor.
//  2. success_empty and the transient classes share one strike count, so nine
//     empties plus one 404 benches on a single error, and five errors plus
//     five empties trip the empty bench.
//  3. When the bench ends during an outage every concurrent search calls the
//     provider until the first probe returns.

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Now()} }
func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// slowFailing takes the whole request timeout on the router's clock, then
// fails with fail (nil: answers).
func slowFailing(name string, clock *fakeClock, took time.Duration, fail error) *stubProvider {
	return &stubProvider{name: name, searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		clock.Advance(took)
		if fail != nil {
			return nil, fail
		}
		return manyResults(name, 3), nil
	}}
}

func cooling(s *Stream, name string) bool {
	for _, c := range s.Cooling {
		if c.Provider == name {
			return true
		}
	}
	return false
}

func TestKeenableTimeoutOutageIsBenchedFromCompletion(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			clock := newFakeClock()
			fail := context.DeadlineExceeded
			keen := slowFailing("keenable", clock, 30*time.Second, fail)
			router := NewRouter([]Provider{keen}, store, Config{Routing: RoutingBudget, RequestTimeout: 30 * time.Second})
			router.now = clock.Now
			for i := 1; i <= TransientStrikesBeforeCooldown; i++ {
				stream, _ := startAndDrain(t, router)
				if cooling(stream, "keenable") {
					t.Fatalf("search %d skipped keenable before the run of %d", i, TransientStrikesBeforeCooldown)
				}
			}
			calls := keen.searchCnt.Load()
			stream, got := startAndDrain(t, router)
			if !cooling(stream, "keenable") || len(got) != 0 || keen.searchCnt.Load() != calls {
				t.Fatalf("sixth search called keenable (cooling=%v arrivals=%d); the bench was over before it began", stream.Cooling, len(got))
			}
			clock.Advance(TransientBackoff + time.Second)
			// The probe fails again: benched again from its completion.
			if stream, _ = startAndDrain(t, router); cooling(stream, "keenable") || keen.searchCnt.Load() != calls+1 {
				t.Fatalf("no probe after the bench: cooling=%v calls=%d", stream.Cooling, keen.searchCnt.Load())
			}
			if stream, _ = startAndDrain(t, router); !cooling(stream, "keenable") {
				t.Fatalf("a failed probe did not bench again")
			}
			// A success resets the run.
			clock.Advance(TransientBackoff + time.Second)
			keen.searchFn = func(ctx context.Context, q string, max int) ([]Result, error) { return manyResults("keenable", 3), nil }
			if _, got = startAndDrain(t, router); len(got) != 1 || !got[0].Answered() {
				t.Fatalf("probe arrivals = %+v, want an answer", got)
			}
			keen.searchFn = func(ctx context.Context, q string, max int) ([]Result, error) { return nil, errors.New(keenable404) }
			startAndDrain(t, router)
			if stream, _ = startAndDrain(t, router); cooling(stream, "keenable") {
				t.Fatalf("keenable benched on the second failure after a success")
			}
		})
	}
}

// The same anchor for a metered provider: a 503 that took 30 s benches it for
// the server_error base from the moment it failed.
func TestMeteredBenchFollowsCompletionNotStart(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryHealthStore()
	serper := slowFailing("serper", clock, 30*time.Second, errors.New("status 503 Service Unavailable"))
	router := NewRouter([]Provider{serper}, store, Config{Routing: RoutingFanout, RequestTimeout: 30 * time.Second})
	router.now = clock.Now
	startAndDrain(t, router)
	rec, _ := store.Get("serper")
	if rec.CooldownUntil == nil {
		t.Fatalf("serper not benched: %+v", rec)
	}
	want := clock.Now().Add(DefaultBackoffPolicy().CooldownDuration(OutcomeServerError, 1))
	if !rec.CooldownUntil.Equal(want) {
		t.Fatalf("bench until %s, want %s (completion plus the base)", rec.CooldownUntil, want)
	}
}

func TestEmptyAndTransientStreaksAreSeparate(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			at := time.Now()
			rec := func(provider string, class OutcomeClass, uncapped bool) ProviderHealth {
				r, err := store.RecordOutcome(Outcome{Provider: provider, Class: class, Uncapped: uncapped, Error: keenable404, At: at})
				if err != nil {
					t.Fatal(err)
				}
				at = at.Add(time.Minute) // past any 30 s bench
				return r
			}
			// Nine empties, then one 404: one error, no bench.
			for i := 0; i < EmptyStreakBeforeCooldown-1; i++ {
				rec("keenable", OutcomeSuccessEmpty, true)
			}
			if r := rec("keenable", OutcomeError, true); !IsEligible(r, at) || r.StrikeCount != 1 {
				t.Fatalf("nine empties then one error: %+v, want one strike and no bench", r)
			}
			// Five errors (benched), then five empties: not the empty bench.
			for i := 0; i < TransientStrikesBeforeCooldown; i++ {
				rec("searxng", OutcomeError, true)
			}
			var r ProviderHealth
			for i := 0; i < 5; i++ {
				r = rec("searxng", OutcomeSuccessEmpty, true)
			}
			if !IsEligible(r, at) || r.StrikeCount != 5 {
				t.Fatalf("five errors then five empties: %+v, want 5 empties and no bench", r)
			}
			// A metered provider: nine empties then an error is a first strike.
			for i := 0; i < EmptyStreakBeforeCooldown-1; i++ {
				rec("serper", OutcomeSuccessEmpty, false)
			}
			r = rec("serper", OutcomeError, false)
			if r.StrikeCount != 1 || r.CooldownUntil == nil || r.CooldownUntil.Sub(at.Add(-time.Minute)) != DefaultBackoffPolicy().CooldownDuration(OutcomeError, 1) {
				t.Fatalf("metered after nine empties: %+v, want a first-strike bench", r)
			}
		})
	}
}

// After the bench ends, one search probes; the others wait for its answer.
func TestOneProbeAfterBench(t *testing.T) {
	clock := newFakeClock()
	store := NewMemoryHealthStore()
	release := make(chan struct{})
	started := make(chan struct{}, 8)
	keen := &stubProvider{name: "keenable"}
	router := NewRouter([]Provider{keen}, store, Config{Routing: RoutingBudget, RequestTimeout: 30 * time.Second})
	router.now = clock.Now
	for i := 0; i < TransientStrikesBeforeCooldown; i++ {
		startAndDrain(t, router)
	}
	clock.Advance(TransientBackoff + time.Second)
	keen.searchFn = func(ctx context.Context, q string, max int) ([]Result, error) {
		started <- struct{}{}
		<-release
		return manyResults("keenable", 3), nil
	}
	first, err := router.Start(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	second, err := router.Start(context.Background(), "q2", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !cooling(second, "keenable") || len(second.Providers) != 0 {
		t.Fatalf("second search while the probe is out: providers=%v cooling=%v, want it to skip keenable", second.Providers, second.Cooling)
	}
	drain(t, second)
	close(release)
	if got := drain(t, first); len(got) != 1 || !got[0].Answered() {
		t.Fatalf("probe arrivals = %+v", got)
	}
	third, _ := startAndDrain(t, router)
	if cooling(third, "keenable") {
		t.Fatalf("keenable still skipped after the probe answered: %v", third.Cooling)
	}
}
