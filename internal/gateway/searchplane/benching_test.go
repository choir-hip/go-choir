package searchplane

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Benching model (docs/archive/problems/search-providers-cooling-most-of-the-time-2026-10-14.md).
// Ways it can fail, written before the code:
//  1. A bench stops being set at all (a real 402 must still bench).
//  2. The episode test is too wide, so a provider in a true outage is
//     called on every search.
//  3. A success no longer clears strikes and the bench.
//  4. Pacing delays the first call of a quiet period, or waits past the
//     caller's context.
//  5. Pacing counts as a failure or strike, or a paced provider is counted
//     in the N of "M of N".
//  6. Caps change other classes' first-strike durations.
//  7. The CHOIR_SEARCH_BACKOFF_BASE_SECONDS override stops applying.
//  8. A skip reason carries an error summary (possible secrets).

func stores(t *testing.T) map[string]HealthStore {
	t.Helper()
	file, err := OpenFileHealthStore(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]HealthStore{"memory": NewMemoryHealthStore(), "file": file}
}

// Parallel's record on 10-13: three calls in flight when the account ran
// out each added a strike, so one fact benched it 96 h instead of 24 h.
func TestOneEpisodeOneStrike(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			started := time.Now()
			for i := 0; i < 3; i++ {
				if _, err := store.RecordOutcome(Outcome{Provider: "parallel", Class: OutcomeQuotaLimited, Error: "status 402", At: started}); err != nil {
					t.Fatal(err)
				}
			}
			rec, err := store.Get("parallel")
			if err != nil {
				t.Fatal(err)
			}
			if rec.StrikeCount != 1 {
				t.Fatalf("strikes = %d, want 1 for one episode (three calls in flight)", rec.StrikeCount)
			}
			if rec.State != StateCoolingDown || rec.CooldownUntil == nil {
				t.Fatalf("rec = %+v, want benched (failure mode 1)", rec)
			}
			want := started.Add(DefaultBackoffPolicy().CooldownDuration(OutcomeQuotaLimited, 1))
			if !rec.CooldownUntil.Equal(want) {
				t.Fatalf("until = %s, want %s (not extended by the same episode)", rec.CooldownUntil, want)
			}
		})
	}
}

// A call started after the bench expired is a new observation: it strikes
// again and the bench doubles (failure mode 2).
func TestNewEpisodeAfterBenchStrikesAgain(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			first := time.Now().Add(-3 * time.Hour)
			if _, err := store.RecordOutcome(Outcome{Provider: "p", Class: OutcomeQuotaLimited, At: first}); err != nil {
				t.Fatal(err)
			}
			later := time.Now()
			rec, err := store.RecordOutcome(Outcome{Provider: "p", Class: OutcomeQuotaLimited, At: later})
			if err != nil {
				t.Fatal(err)
			}
			if rec.StrikeCount != 2 {
				t.Fatalf("strikes = %d, want 2 after a later episode", rec.StrikeCount)
			}
			if IsEligible(rec, later) {
				t.Fatalf("provider eligible right after a new strike: %+v", rec)
			}
		})
	}
}

// A success, even from a call already in flight, clears the bench
// (failure mode 3).
func TestSuccessClearsBench(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			at := time.Now()
			_, _ = store.RecordOutcome(Outcome{Provider: "p", Class: OutcomeRateLimited, At: at})
			rec, _ := store.RecordOutcome(Outcome{Provider: "p", Class: OutcomeSuccess, Results: 3, At: at})
			if rec.StrikeCount != 0 || rec.State != StateActive || rec.CooldownUntil != nil {
				t.Fatalf("rec = %+v, want active with no strikes", rec)
			}
		})
	}
}

// Concurrent searches failing on the same provider at once count once.
func TestRouter_ConcurrentFailuresStrikeOnce(t *testing.T) {
	store := NewMemoryHealthStore()
	gate := make(chan struct{})
	p := &stubProvider{name: "tavily", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		<-gate
		return nil, errors.New(`status 432 : {"detail":{"error":"This request exceeds your plan's set usage limit"}}`)
	}}
	router := NewRouter([]Provider{p}, store, Config{RequestTimeout: time.Second})
	var streams []*Stream
	for i := 0; i < 4; i++ {
		s, err := router.Start(context.Background(), "q", 5)
		if err != nil {
			t.Fatal(err)
		}
		streams = append(streams, s)
	}
	close(gate)
	for _, s := range streams {
		drain(t, s)
	}
	rec, _ := store.Get("tavily")
	if rec.StrikeCount != 1 || rec.LastFailureClass != string(OutcomeQuotaLimited) {
		t.Fatalf("rec = %+v, want one quota strike for four concurrent calls", rec)
	}
}

// Every class's backoff is bounded so a transient fault cannot bench a
// provider for days; first strikes keep their old lengths (failure mode 6).
func TestCooldownCapsByClass(t *testing.T) {
	p := DefaultBackoffPolicy()
	first := map[OutcomeClass]time.Duration{
		OutcomeRateLimited:  time.Minute,
		OutcomeTimeout:      2 * time.Minute,
		OutcomeServerError:  2 * time.Minute,
		OutcomeError:        5 * time.Minute,
		OutcomeQuotaLimited: time.Hour,
		OutcomeAuthError:    7 * 24 * time.Hour,
	}
	for class, want := range first {
		if got := p.CooldownDuration(class, 1); got != want {
			t.Errorf("%s first strike = %s, want %s", class, got, want)
		}
	}
	if got := p.CooldownDuration(OutcomeSuccessEmpty, EmptyStreakBeforeCooldown-1); got != 0 {
		t.Errorf("empties below the streak bench for %s", got)
	}
	caps := map[OutcomeClass]time.Duration{
		OutcomeRateLimited:  15 * time.Minute,
		OutcomeTimeout:      30 * time.Minute,
		OutcomeServerError:  30 * time.Minute,
		OutcomeError:        time.Hour,
		OutcomeSuccessEmpty: 6 * time.Hour,
		OutcomeQuotaLimited: 6 * time.Hour,
	}
	for class, cap := range caps {
		if got := p.CooldownDuration(class, 40); got != cap {
			t.Errorf("%s after 40 strikes = %s, want the cap %s", class, got, cap)
		}
	}
}

// Failure mode 7.
func TestBackoffEnvOverrideStillApplies(t *testing.T) {
	t.Setenv("CHOIR_SEARCH_BACKOFF_BASE_SECONDS", "30")
	p := BackoffPolicyFromEnv()
	if got := p.CooldownDuration(OutcomeRateLimited, 1); got != 30*time.Second {
		t.Fatalf("rate limit with override = %s, want 30s", got)
	}
	if got := p.CooldownDuration(OutcomeQuotaLimited, 1); got != 30*time.Second {
		t.Fatalf("quota with override = %s, want 30s", got)
	}
}

func pacedRouter(store HealthStore, wait time.Duration, interval time.Duration, providers ...Provider) *Router {
	return NewRouter(providers, store, Config{
		RequestTimeout: 2 * time.Second,
		MinInterval:    map[string]time.Duration{"brave": interval},
		PaceWait:       wait,
	})
}

// Brave's Free AI plan allows one request a second; two to four searches
// in one second used to 429 and bench it. Paced, the second waits for its
// slot and the first goes straight through (failure mode 4).
func TestPacing_SecondCallWaitsForItsSlot(t *testing.T) {
	var mu sync.Mutex
	var calls []time.Time
	brave := &stubProvider{name: "brave", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		mu.Lock()
		calls = append(calls, time.Now())
		mu.Unlock()
		return manyResults("brave", 1), nil
	}}
	router := pacedRouter(NewMemoryHealthStore(), time.Second, 300*time.Millisecond, brave)
	begin := time.Now()
	s1, _ := router.Start(context.Background(), "a", 5)
	s2, _ := router.Start(context.Background(), "b", 5)
	drain(t, s1)
	drain(t, s2)
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if first := calls[0].Sub(begin); first > 100*time.Millisecond {
		t.Fatalf("first call waited %s; an idle provider must go straight through", first)
	}
	if gap := calls[1].Sub(calls[0]); gap < 250*time.Millisecond {
		t.Fatalf("second call %s after the first, want about the 300ms interval", gap)
	}
}

// Beyond the wait bound the provider is skipped for this search as paced:
// not called, not in N, no strike, and listed with reason "paced"
// (failure mode 5).
func TestPacing_SkipsBeyondWaitWithoutStrike(t *testing.T) {
	store := NewMemoryHealthStore()
	brave := delayedProvider("brave", 0, 1)
	keen := delayedProvider("keenable", 0, 1)
	router := pacedRouter(store, 100*time.Millisecond, time.Hour, brave, keen)
	s1, _ := router.Start(context.Background(), "a", 5)
	drain(t, s1)
	s2, err := router.Start(context.Background(), "b", 5)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(s2.Providers, ",") != "keenable" {
		t.Fatalf("providers = %v, want brave left out of N while paced", s2.Providers)
	}
	if len(s2.Cooling) != 1 || s2.Cooling[0].Provider != "brave" || s2.Cooling[0].State != StatePaced || s2.Cooling[0].Reason != "paced" {
		t.Fatalf("cooling = %+v, want brave paced", s2.Cooling)
	}
	drain(t, s2)
	if brave.searchCnt.Load() != 1 {
		t.Fatalf("brave called %d times, want once", brave.searchCnt.Load())
	}
	rec, _ := store.Get("brave")
	if rec.StrikeCount != 0 || rec.State != StateActive {
		t.Fatalf("brave rec = %+v; pacing must never strike", rec)
	}
}

// A caller that goes away while its call waits for a slot is caller_gone,
// never a strike, and the wait ends with the caller (failure mode 4).
func TestPacing_CallerGoneWhileWaiting(t *testing.T) {
	store := NewMemoryHealthStore()
	brave := delayedProvider("brave", 0, 1)
	router := pacedRouter(store, 5*time.Second, 2*time.Second, brave)
	s1, _ := router.Start(context.Background(), "a", 5)
	drain(t, s1)
	ctx, cancel := context.WithCancel(context.Background())
	s2, _ := router.Start(ctx, "b", 5)
	time.AfterFunc(50*time.Millisecond, cancel)
	began := time.Now()
	arrivals := drain(t, s2)
	if waited := time.Since(began); waited > time.Second {
		t.Fatalf("waited %s after the caller left", waited)
	}
	if len(arrivals) != 1 || arrivals[0].Status != "caller_gone" {
		t.Fatalf("arrivals = %+v, want one caller_gone", arrivals)
	}
	if brave.searchCnt.Load() != 1 {
		t.Fatalf("brave called %d times; the abandoned wait must not call it", brave.searchCnt.Load())
	}
	if rec, _ := store.Get("brave"); rec.StrikeCount != 0 {
		t.Fatalf("strikes = %d after a caller left", rec.StrikeCount)
	}
}

// The skip reason is the failure class, never the error summary
// (failure mode 8).
func TestCoolingReasonIsTheClassNotTheSummary(t *testing.T) {
	store := NewMemoryHealthStore()
	_, _ = store.RecordOutcome(Outcome{Provider: "serpapi", Class: OutcomeQuotaLimited, Error: "status 429 api_key=sk-secret run out of searches", At: time.Now()})
	router := NewRouter([]Provider{delayedProvider("serpapi", 0, 1)}, store, Config{RequestTimeout: time.Second})
	s, _ := router.Start(context.Background(), "q", 5)
	drain(t, s)
	if len(s.Cooling) != 1 || s.Cooling[0].Reason != string(OutcomeQuotaLimited) {
		t.Fatalf("cooling = %+v, want reason quota_limited", s.Cooling)
	}
}

func TestParseProviderRPS(t *testing.T) {
	got := parseProviderRPS("brave=1, tavily=4,bad,exa=0,parallel=x")
	if got["brave"] != time.Second || got["tavily"] != 250*time.Millisecond {
		t.Fatalf("intervals = %v", got)
	}
	if _, ok := got["exa"]; ok {
		t.Fatalf("rps 0 must disable pacing: %v", got)
	}
	if len(got) != 2 {
		t.Fatalf("intervals = %v, want brave and tavily only", got)
	}
	t.Setenv("CHOIR_SEARCH_PROVIDER_RPS", "")
	if cfg := ConfigFromEnv(); cfg.MinInterval["brave"] != time.Second {
		t.Fatalf("default pacing = %v, want brave at 1/s", cfg.MinInterval)
	}
}
