package searchplane

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// One Keenable error must not take the only uncapped provider out of rotation
// (docs/problems/one-keenable-error-cools-the-only-uncapped-search-provider-2026-11-13.md;
// owner, 2026-11-13: "cooldown is doing multiple duties and so failing. a
// single [Keenable] error shouldn't put it into cooldown, whereas an error
// from most other providers means that we're actually out of quota").
// Ways it can fail, written before the code:
//  1. One transient error (404 after 6 s) benches a free-tier provider, so the
//     next search is answered by no one while every paid provider is out of
//     quota.
//  2. The fix is by provider name, not by tier, so another free provider or a
//     renamed one still benches on one error.
//  3. A metered provider's error stops backing it off ("an error means the
//     quota is gone" for paid providers).
//  4. A real refusal from the uncapped provider (429, 402, 401) stops
//     backing it off.
//  5. A sustained outage is never protected, or its backoff grows without
//     bound, or a success does not clear the count.
//  6. A Keenable failure stops being logged with its class and status.

const keenable404 = "status 404 Not Found: no such route"

func failingThenOK(failures int32) *stubProvider {
	var n atomic.Int32
	return &stubProvider{name: "keenable", searchFn: func(ctx context.Context, q string, max int) ([]Result, error) {
		if n.Add(1) <= failures {
			return nil, errors.New(keenable404)
		}
		return manyResults("keenable", 3), nil
	}}
}

func startAndDrain(t *testing.T, r *Router) (*Stream, []Arrival) {
	t.Helper()
	stream, err := r.Start(context.Background(), "q", 5)
	if err != nil {
		t.Fatal(err)
	}
	return stream, drain(t, stream)
}

// Failure mode 1, end to end: every paid provider is out of quota, Keenable
// errs once, and the next search is answered by Keenable.
func TestOneKeenableErrorIsAnsweredByKeenableNextSearch(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			keen := failingThenOK(1)
			serper := &stubProvider{name: "serper"}
			router := NewRouter([]Provider{keen, serper}, store, Config{Routing: RoutingBudget, RequestTimeout: time.Second})
			// Serper's quota is gone.
			if _, err := store.RecordOutcome(Outcome{Provider: "serper", Class: OutcomeQuotaLimited, Error: "status 402", At: time.Now()}); err != nil {
				t.Fatal(err)
			}
			_, first := startAndDrain(t, router)
			if len(first) != 1 || first[0].Provider != "keenable" || first[0].Answered() {
				t.Fatalf("first search arrivals = %+v, want one failed keenable call", first)
			}
			stream, second := startAndDrain(t, router)
			for _, c := range stream.Cooling {
				if c.Provider == "keenable" {
					t.Fatalf("keenable skipped after one error: %+v", c)
				}
			}
			if len(second) != 1 || second[0].Provider != "keenable" || !second[0].Answered() {
				t.Fatalf("second search arrivals = %+v, want keenable's answer", second)
			}
			if got := keen.searchCnt.Load(); got != 2 {
				t.Fatalf("keenable calls = %d, want 2", got)
			}
		})
	}
}

// Failure mode 2: the rule follows the tier, so every free-tier provider is
// covered and no name is special.
func TestTransientErrorOfAFreeTierProviderIsNotACooldown(t *testing.T) {
	for _, name := range []string{"keenable", "searxng", "brave"} {
		store := NewMemoryHealthStore()
		p := &stubProvider{name: name}
		router := NewRouter([]Provider{p}, store, Config{Routing: RoutingBudget, RequestTimeout: time.Second})
		startAndDrain(t, router)
		rec, _ := store.Get(name)
		if !IsEligible(rec, time.Now()) || rec.State != StateActive {
			t.Fatalf("%s after one error: %+v, want active", name, rec)
		}
	}
}

// Failure mode 3: a metered provider's error still backs it off as today.
func TestMeteredProviderErrorStillBacksItOff(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			p := &stubProvider{name: "serper"}
			router := NewRouter([]Provider{p}, store, Config{Routing: RoutingBudget, RequestTimeout: time.Second})
			startAndDrain(t, router)
			rec, _ := store.Get("serper")
			if rec.State != StateCoolingDown || rec.CooldownUntil == nil {
				t.Fatalf("serper after one error: %+v, want cooling", rec)
			}
			want := DefaultBackoffPolicy().CooldownDuration(OutcomeError, 1)
			if d := time.Until(*rec.CooldownUntil); d < want-5*time.Second || d > want+5*time.Second {
				t.Fatalf("serper cools for %s, want about %s", d, want)
			}
			stream, _ := startAndDrain(t, router)
			if len(stream.Cooling) != 1 || stream.Cooling[0].Reason != string(OutcomeError) {
				t.Fatalf("second search cooling = %+v, want serper=error", stream.Cooling)
			}
		})
	}
}

// Failure mode 4: the uncapped provider's own refusals still back it off.
func TestUncappedProviderRealRefusalStillBacksOff(t *testing.T) {
	for _, class := range []OutcomeClass{OutcomeRateLimited, OutcomeQuotaLimited, OutcomeAuthError} {
		for name, store := range stores(t) {
			t.Run(string(class)+"/"+name, func(t *testing.T) {
				if _, err := store.RecordOutcome(Outcome{Provider: "keenable", Class: class, Uncapped: true, Error: "status 4xx", At: time.Now()}); err != nil {
					t.Fatal(err)
				}
				rec, _ := store.Get("keenable")
				if rec.State != StateCoolingDown || rec.CooldownUntil == nil {
					t.Fatalf("keenable after %s: %+v, want cooling", class, rec)
				}
			})
		}
	}
}

// Failure mode 5: repeated transient failures of the uncapped provider.
func TestUncappedProviderRepeatedFailuresBackOffShortAndFixed(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			at := time.Now()
			fail := func(class OutcomeClass) ProviderHealth {
				rec, err := store.RecordOutcome(Outcome{Provider: "keenable", Class: class, Uncapped: true, Error: keenable404, At: at})
				if err != nil {
					t.Fatal(err)
				}
				at = at.Add(time.Second)
				return rec
			}
			for i := 1; i < TransientStrikesBeforeCooldown; i++ {
				if rec := fail(OutcomeError); rec.State == StateCoolingDown {
					t.Fatalf("failure %d benched keenable; want only after %d in a row", i, TransientStrikesBeforeCooldown)
				}
			}
			rec := fail(OutcomeServerError)
			if rec.State != StateCoolingDown || rec.CooldownUntil == nil {
				t.Fatalf("failure %d: %+v, want cooling", TransientStrikesBeforeCooldown, rec)
			}
			if d := rec.CooldownUntil.Sub(at.Add(-time.Second)); d != TransientBackoff {
				t.Fatalf("backoff = %s, want fixed %s", d, TransientBackoff)
			}
			// After the backoff the next failing probe is again TransientBackoff,
			// never longer.
			at = rec.CooldownUntil.Add(time.Second)
			rec = fail(OutcomeTimeout)
			if rec.CooldownUntil == nil || rec.CooldownUntil.Sub(at.Add(-time.Second)) != TransientBackoff {
				t.Fatalf("second backoff = %+v, want fixed %s", rec, TransientBackoff)
			}
			// A success clears the run.
			rec, _ = store.RecordOutcome(Outcome{Provider: "keenable", Class: OutcomeSuccess, Uncapped: true, Results: 3, At: at})
			if rec.StrikeCount != 0 || rec.State != StateActive {
				t.Fatalf("after success: %+v", rec)
			}
			if rec = fail(OutcomeError); rec.State == StateCoolingDown {
				t.Fatalf("one failure after a success benched keenable: %+v", rec)
			}
		})
	}
}

// Failure mode 6: the Keenable failure is logged with class and status.
func TestKeenableFailureIsStillLogged(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	router := NewRouter([]Provider{failingThenOK(1)}, NewMemoryHealthStore(), Config{RequestTimeout: time.Second})
	startAndDrain(t, router)
	if want := "searchplane: keenable call failed class=error http_status=404 stage=http_status"; !strings.Contains(buf.String(), want) {
		t.Fatalf("log = %q, want it to contain %q", buf.String(), want)
	}
}
