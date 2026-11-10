package gateway

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Devin hourly pacing and the bounded quota probe
// (docs/evidence/devin-quota-capacity-2026-11-10.md). Ways this could fail,
// written down before the code, and the test that answers each:
//
//   - Clock-hour boundary. The quota window is the fixed clock hour, so the
//     count, the shed share and the trip tally must reset exactly at hh:00:00
//     UTC, not 60 minutes after the first call. A call at 09:59:59 and one at
//     10:00:00 are in different hours (TestPacerHourBoundary). A gap of
//     several idle hours must not raise the ceiling for hours nobody used.
//   - Concurrent counting. Calls finish on many goroutines; the count must be
//     the exact sum (TestPacerConcurrentCounting, run under -race in CI).
//     In-flight calls still overshoot the line by what is in flight; the
//     burst allowance and the 10% reserve absorb that, nothing else does.
//   - A 429 with no reset time. The breaker handles recovery (its own
//     doubling cooldown); the pacer only learns from the trip. It learns the
//     same way whether or not Devin named a reset (TestPacerTripLearnsCeiling).
//   - Token counts unknown for a streamed call until the end. The count is
//     added when the call returns its usage; a stream that fails halfway
//     returns no usage and counts 0. The gateway therefore undercounts by the
//     failed streams, never overcounts (TestPacerCountsOnlyAnsweredCalls).
//   - Cached tokens. The evidence doc shows `cached` is not a subset of
//     `input` in 18.6% of Devin calls, so IN+C overstates and IN-C can go
//     negative. We count the usage's InputTokens (the log's IN): the one
//     figure that fits both clean hour 10 (42.70M) and the trip in hour 11
//     (43.54M) from the same gateway. Cached tokens are never added or
//     subtracted. If Devin meters something else, the trip-learned ceiling
//     absorbs the difference because it is learned in the same unit.
//   - Restart mid-hour with an already-spent quota. State is in memory and
//     relearned. A pacer that first saw the hour partway counts only part of
//     it, so a trip in that hour must not shrink the ceiling to the partial
//     count (TestPacerPartialHourTripDoesNotLearn); the circuit breaker still
//     holds the model until the reset.
//   - A share that sticks. The share is a feedback controller on observed
//     served tokens per minute against the budget left per minute left; it
//     rises 0.10 per 30 s evaluation while over the allowed rate and falls
//     0.10 per 60 s while under 70% of it, so if every sheddable call is shed
//     the served rate falls to the unsheddable rate and the share comes back
//     down (TestPacerSharesFallAfterABurstEvenWhenEverySheddableCallWasShed).
//     Evaluation happens on admit only, so a stale share after a quiet gap
//     applies every elapsed 60 s step in one admit (TestPacerQuietGapDecays).
//     Under steady 2x demand the share must settle, not swing
//     (TestPacerSteadyDoubleDemand); under 0.5x it must stay 0
//     (TestPacerHalfDemandNeverSheds); with only unsheddable traffic it
//     saturates and nothing panics (TestPacerAllUnsheddableSaturates).
//     Shed sets are nested by hash, so a falling share only returns threads;
//     a thread flaps at most with the share, and the share moves 0.10 at a
//     time.
//   - A ceiling collapse after one bad hour (owner's local use trips the
//     account at a low gateway count). The ceiling has a floor, rises 2% per
//     hour that ended without a trip and reached 90% of it, and has a cap
//     (TestPacerCeilingRaise, TestPacerTripLearnsCeiling).
//   - The reserve. At 90% of the ceiling the gateway sheds every thread, so
//     the local CLI keeps the last tenth (TestPacerReserveStopsAll).
//   - A hung half-open probe holding the circuit (the 09:00-09:13 hold):
//     the probe has a deadline and its failure reopens the breaker on its
//     short backoff (TestQuotaProbeIsBounded).

type pacerHarness struct {
	n     int
	clock *fakeClock
	p     *hourPacer
	mu    sync.Mutex
	logs  []string
}

func newPacerHarness(t *testing.T, start time.Time) *pacerHarness {
	t.Helper()
	h := &pacerHarness{clock: &fakeClock{now: start}}
	h.p = newHourPacer(h.clock.Now, func(format string, args ...any) {
		h.mu.Lock()
		h.logs = append(h.logs, fmt.Sprintf(format, args...))
		h.mu.Unlock()
	})
	return h
}

func (h *pacerHarness) logged() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.logs, "\n")
}

var pacerT0 = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

func threadNames(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("thread-%03d", i)
	}
	return out
}

func shedCount(p *hourPacer, threads []string) int {
	n := 0
	for _, th := range threads {
		if !p.admit(th) {
			n++
		}
	}
	return n
}

func TestPacerAdmitsEverythingUnderTheLine(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	// 9M (a quarter of the budget) spent steadily over half the hour is
	// under any even-spend line.
	if shed, total := driveMinutes(h, threadNames(200), 30, 10, 30_000); shed != 0 {
		t.Fatalf("shed %d of %d calls under the line, want 0", shed, total)
	}
}

// driveMinutes runs minutes of offered demand: perMinute calls of
// tokensPerCall each over a rotating set of threads. Shed calls are not
// served and spend nothing. It returns the shed calls and total calls.
func driveMinutes(h *pacerHarness, threads []string, minutes, perMinute, tokensPerCall int) (shed, total int) {
	for m := 0; m < minutes; m++ {
		for k := 0; k < perMinute; k++ {
			h.clock.Advance(time.Minute / time.Duration(perMinute))
			th := threads[(total*7919)%len(threads)]
			total++
			if h.p.admit(th) {
				h.p.record(tokensPerCall, true)
			} else {
				shed++
			}
		}
	}
	return
}

func TestPacerCarriesNothingAfterAnHourThatEndedFarUnderBudget(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(10 * time.Minute)
	h.p.record(int(devinCeilingStart*0.6), true)
	shedCount(h.p, threadNames(100)) // share > 0
	if h.p.snapshot().share == 0 {
		t.Fatal("test needs a share")
	}
	h.clock.Advance(50 * time.Minute)
	h.p.admit("x")
	if got := h.p.snapshot().share; got != 0 {
		t.Fatalf("carried share = %v after an hour that ended at 60%% of the ceiling, want 0", got)
	}
}

func TestPacerShedSetsAreNestedByHash(t *testing.T) {
	// A rising share only sheds more threads and a falling one only returns
	// threads: a thread's side of the line moves only with the share.
	h := newPacerHarness(t, pacerT0)
	threads := threadNames(500)
	prev := map[string]bool{}
	for _, share := range []float64{0.1, 0.3, 0.6, 0.95} {
		h.p.mu.Lock()
		h.p.share = share
		h.p.mu.Unlock()
		cur := map[string]bool{}
		for _, th := range threads {
			if !h.p.admit(th) {
				cur[th] = true
			}
		}
		for th := range prev {
			if !cur[th] {
				t.Fatalf("share %.2f: thread %s shed at a lower share but served at a higher one", share, th)
			}
		}
		if len(cur) <= len(prev) {
			t.Fatalf("share %.2f shed %d, not more than the %d at the lower share", share, len(cur), len(prev))
		}
		prev = cur
	}
}

func TestPacerReserveStopsAll(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(59 * time.Minute)
	h.p.record(int(devinCeilingStart*(1-devinReserveFraction))+1, true)
	if n := shedCount(h.p, threadNames(100)); n != 100 {
		t.Fatalf("shed %d of 100 at the reserve, want all", n)
	}
}

func TestPacerHourBoundary(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(59*time.Minute + 59*time.Second)
	h.p.record(int(devinCeilingStart*0.95), true)
	shedCount(h.p, threadNames(50)) // all shed; the share is 1
	h.clock.Advance(time.Second)    // 10:00:00
	if n := shedCount(h.p, threadNames(200)); n == 200 {
		t.Fatal("share did not fall at the hour boundary")
	}
	if got := h.p.snapshot().tokens; got != 0 {
		t.Fatalf("tokens after the boundary = %d, want 0", got)
	}
	if !strings.Contains(h.logged(), "hour=2026-11-10T09:00Z") || !strings.Contains(h.logged(), "tokens=") {
		t.Fatalf("hour-boundary line missing:\n%s", h.logged())
	}
	if n := strings.Count(h.logged(), "devin pacing hour="); n != 1 {
		t.Fatalf("hour lines = %d, want one per boundary\n%s", n, h.logged())
	}
}

func TestPacerNextHourCarriesHalfTheShare(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(59*time.Minute + 59*time.Second)
	h.p.record(int(devinCeilingStart), true)
	h.p.admit("x") // share 1
	h.clock.Advance(time.Second)
	h.p.admit("x")
	if s := h.p.snapshot().share; s != 0.5 {
		t.Fatalf("carried share = %v, want 0.5", s)
	}
	// An idle gap of hours carries nothing.
	h.clock.Advance(3 * time.Hour)
	h.p.admit("x")
	if s := h.p.snapshot().share; s != 0 {
		t.Fatalf("share after an idle gap = %v, want 0", s)
	}
}

func TestPacerTripLearnsCeiling(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	// The pacer was created at the top of the hour: it saw all of it.
	h.clock.Advance(20 * time.Minute)
	h.p.record(30_000_000, true)
	h.p.trip(h.clock.Now())
	if got, want := h.p.snapshot().ceiling, int64(28_500_000); got != want {
		t.Fatalf("ceiling after a trip at 30M = %d, want %d (95%% of the count)", got, want)
	}
	if !strings.Contains(h.logged(), "ceiling 40000000 -> 28500000") {
		t.Fatalf("ceiling change not logged:\n%s", h.logged())
	}
	// A trip above the ceiling never raises it.
	h.p.record(10_000_000, true)
	h.p.trip(h.clock.Now())
	if got := h.p.snapshot().ceiling; got != 28_500_000 {
		t.Fatalf("ceiling after a higher trip = %d, want unchanged", got)
	}
}

func TestPacerTripBelowHalfTheCeilingIsNotLearned(t *testing.T) {
	// The owner's local CLI spent the account; the gateway counted little.
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(30 * time.Minute)
	h.p.record(1_000, true)
	h.p.trip(h.clock.Now())
	if got := h.p.snapshot().ceiling; got != devinCeilingStart {
		t.Fatalf("ceiling after a trip at 1k = %d, want unchanged", got)
	}
	if !strings.Contains(h.logged(), "not learned: count too low to be the gateway's doing") {
		t.Fatalf("expected the not-learned log line:\n%s", h.logged())
	}
	// Just under half is not learned; half is.
	h.p.record(int(devinCeilingStart/2)-1_000-1, true)
	h.p.trip(h.clock.Now())
	if got := h.p.snapshot().ceiling; got != devinCeilingStart {
		t.Fatalf("ceiling at just under half = %d, want unchanged", got)
	}
	h.p.record(2, true)
	h.p.trip(h.clock.Now())
	if got := h.p.snapshot().ceiling; got >= devinCeilingStart {
		t.Fatalf("ceiling at half = %d, want learned", got)
	}
}

func TestPacerTripJustAfterRolloverIsNobodysDoing(t *testing.T) {
	// A call dispatched at 10:00:10 and refused at 10:00:30: a lagging reset,
	// not the new hour's spend. A 429 in the first two minutes of the hour,
	// for a call dispatched in it, is attributed to nothing.
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(59 * time.Minute)
	h.p.record(38_000_000, true)
	h.clock.Advance(time.Minute + 10*time.Second) // 10:00:10
	dispatched := h.clock.Now()
	h.clock.Advance(20 * time.Second) // 10:00:30
	h.p.trip(dispatched)
	s := h.p.snapshot()
	if s.ceiling < devinCeilingStart || s.trips != 0 {
		t.Fatalf("after an early-hour trip: ceiling=%d trips=%d, want the ceiling not lowered and no trip counted", s.ceiling, s.trips)
	}
	if !strings.Contains(h.logged(), "first 2 minutes") {
		t.Fatalf("expected the early-trip log line:\n%s", h.logged())
	}
}

func TestPacerTripForACallFromThePreviousHourLearnsFromThatHoursCount(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(59*time.Minute + 50*time.Second)
	h.p.record(38_000_000, true) // hour 9 count
	dispatched := h.clock.Now()
	h.clock.Advance(40 * time.Second) // 10:00:30, new hour
	h.p.trip(dispatched)
	if got, want := h.p.snapshot().ceiling, int64(36_100_000); got != want {
		t.Fatalf("ceiling = %d, want %d learned from hour 9's 38M (not from hour 10's 0)", got, want)
	}
	// A previous-hour call when that hour's count was small teaches nothing.
	h2 := newPacerHarness(t, pacerT0)
	h2.clock.Advance(59*time.Minute + 50*time.Second)
	h2.p.record(1_000_000, true)
	d2 := h2.clock.Now()
	h2.clock.Advance(40 * time.Second)
	h2.p.trip(d2)
	if got := h2.p.snapshot().ceiling; got != devinCeilingStart {
		t.Fatalf("ceiling = %d after a previous-hour trip at a small count, want unchanged", got)
	}
	if !strings.Contains(h2.logged(), "not learned") {
		t.Fatalf("expected a not-learned line:\n%s", h2.logged())
	}
}

func TestPacerPartialHourTripDoesNotLearn(t *testing.T) {
	// The gateway restarted at 09:40; the account was already spent.
	h := newPacerHarness(t, pacerT0.Add(40*time.Minute))
	h.p.record(30_000_000, true)
	h.p.trip(h.clock.Now())
	if got := h.p.snapshot().ceiling; got != devinCeilingStart {
		t.Fatalf("ceiling = %d after a trip in a partly seen hour, want unchanged %d", got, int64(devinCeilingStart))
	}
	if !strings.Contains(h.logged(), "partly observed") {
		t.Fatalf("expected a log line about the partly observed hour:\n%s", h.logged())
	}
}

func TestPacerCeilingRaise(t *testing.T) {
	// A full hour, no trip, spend at the budget: +5%.
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(30 * time.Minute)
	h.p.record(int(devinCeilingStart*0.95), true)
	h.clock.Advance(31 * time.Minute)
	h.p.admit("x")
	if got, want := h.p.snapshot().ceiling, int64(42_000_000); got != want {
		t.Fatalf("ceiling after a clean full hour = %d, want %d", got, want)
	}
	if !strings.Contains(h.logged(), "ceiling 40000000 -> 42000000") {
		t.Fatalf("raise not logged:\n%s", h.logged())
	}
	// Light use, nothing shed: no raise.
	c := h.p.snapshot().ceiling
	driveMinutes(h, threadNames(100), 58, 10, int(c/4/600))
	h.clock.Advance(3 * time.Minute)
	h.p.admit("x")
	if got := h.p.snapshot().ceiling; got != c {
		t.Fatalf("ceiling after a half-used hour with no shedding = %d, want unchanged %d", got, c)
	}
	// A tripped hour never raises, even at full use.
	h.clock.Advance(10 * time.Minute)
	h.p.record(int(float64(c)*0.99), true)
	h.p.trip(h.clock.Now())
	c = h.p.snapshot().ceiling
	h.clock.Advance(time.Hour)
	h.p.admit("x")
	if got := h.p.snapshot().ceiling; got != c {
		t.Fatalf("ceiling after a tripped hour = %d, want unchanged %d", got, c)
	}
	// The cap holds.
	h3 := newPacerHarness(t, pacerT0)
	for i := 0; i < 60; i++ {
		h3.p.record(int(h3.p.snapshot().ceiling), true)
		h3.clock.Advance(time.Hour)
		h3.p.admit("x")
	}
	if got := h3.p.snapshot().ceiling; got != devinCeilingCap {
		t.Fatalf("ceiling after many clean hours = %d, want the cap %d", got, int64(devinCeilingCap))
	}
}

// pacedHour runs one hour in which demand exceeds the paced rate: at minute
// 10 the count is 60% of the ceiling, far ahead of the line, so the pacer
// sheds and the hour then ends under the budget.
func pacedHour(h *pacerHarness) {
	h.clock.Advance(10 * time.Minute)
	h.p.record(int(float64(h.p.snapshot().ceiling)*0.6), true)
	shedCount(h.p, threadNames(100))
	h.clock.Advance(50 * time.Minute)
}

func TestPacerRaisesWhenPacedUnderBudget(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	pacedHour(h)
	h.p.admit("x") // closes the hour
	s := h.p.snapshot()
	if s.ceiling != 42_000_000 {
		t.Fatalf("ceiling = %d after an hour that shed and ended under budget, want 42M", s.ceiling)
	}
}

func TestPacerRecoversFromALearnedLowCeilingWithinADay(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	h.clock.Advance(20 * time.Minute)
	h.p.record(20_000_000, true)
	h.p.trip(h.clock.Now()) // learned 19M
	if got := h.p.snapshot().ceiling; got != 19_000_000 {
		t.Fatalf("learned ceiling = %d, want 19M", got)
	}
	h.clock.Advance(40 * time.Minute) // to 10:00
	hours := 0
	for h.p.snapshot().ceiling < devinCeilingStart && hours < 24 {
		pacedHour(h)
		hours++
	}
	if got := h.p.snapshot().ceiling; got < devinCeilingStart {
		t.Fatalf("ceiling %d after %d hours, want back at %d within a day", got, hours, int64(devinCeilingStart))
	}
}

func TestPacerLogsUnkeyedTokens(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	h.p.record(1_000, false)
	h.p.record(500, true)
	h.clock.Advance(time.Hour)
	h.p.admit("x")
	if !strings.Contains(h.logged(), "tokens=1500") || !strings.Contains(h.logged(), "unsheddable_tokens=1000") {
		t.Fatalf("hour line lacks unsheddable_tokens:\n%s", h.logged())
	}
}

// offer runs minutes of traffic: keyedPerMin calls of tokens each, rotating
// over threads, and unshedPerMin tokens a minute of calls that cannot be shed
// (spread over the minute). Shed calls spend nothing. It returns the calls
// shed and offered.
func offer(h *pacerHarness, threads []string, minutes, keyedPerMin, tokens, unshedPerMin int) (shed, total int) {
	slots := max(keyedPerMin, 1)
	for m := 0; m < minutes; m++ {
		for k := 0; k < slots; k++ {
			h.clock.Advance(time.Minute / time.Duration(slots))
			if unshedPerMin > 0 {
				h.p.record(unshedPerMin/slots, false)
			}
			if keyedPerMin == 0 {
				continue
			}
			th := threads[(h.n*7919)%len(threads)]
			h.n++
			total++
			if h.p.admit(th) {
				h.p.record(tokens, true)
			} else {
				shed++
			}
		}
	}
	return
}

// lowestHashThreads returns the n thread names that are shed first (smallest
// pacing hash): the threads a rising share turns away before any other.
func lowestHashThreads(n int) []string {
	all := threadNames(2000)
	sort.Slice(all, func(i, j int) bool { return threadHash(all[i]) < threadHash(all[j]) })
	return all[:n]
}

func budgetOf(h *pacerHarness) int64 { return int64(h.p.snapshot().ceiling) * 9 / 10 }

func TestPacerHalfDemandNeverSheds(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	threads := threadNames(400)
	// 0.3M a minute against 0.6M a minute allowed.
	shed := 0
	for m := 0; m < 59; m++ {
		sh, _ := offer(h, threads, 1, 10, 30_000, 0)
		shed += sh
		if m == 4 || m == 14 || m == 29 || m == 57 {
			t.Logf("minute %d: share %.2f tokens %d", m+1, h.p.snapshot().share, h.p.snapshot().tokens)
		}
	}
	if s := h.p.snapshot(); shed != 0 || s.share != 0 {
		t.Fatalf("shed %d calls, share %v at 0.5x demand, want none", shed, s.share)
	}
}

func TestPacerSteadyDoubleDemand(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	threads := threadNames(400)
	budget := budgetOf(h)
	lo, hi := 2.0, -1.0
	var atMin57 int64
	for m := 0; m < 58; m++ {
		offer(h, threads, 1, 20, 60_000, 0) // 1.2M a minute offered: 2x the budget rate
		sn := h.p.snapshot()
		if m == 56 {
			atMin57 = sn.tokens
		}
		if m >= 20 {
			lo, hi = min(lo, sn.share), max(hi, sn.share)
		}
		if m == 4 || m == 14 || m == 29 || m == 57 {
			t.Logf("minute %d: share %.2f tokens %d", m+1, sn.share, sn.tokens)
		}
	}
	end := h.p.snapshot()
	t.Logf("final tokens %d of budget %d; share over minutes 20-58: %.2f..%.2f", end.tokens, budget, lo, hi)
	if end.tokens > budget+200_000 {
		t.Fatalf("tokens %d, want the hour to end within the budget %d", end.tokens, budget)
	}
	if end.tokens <= atMin57 {
		t.Fatalf("Devin served nothing in the last minutes (%d then %d): a cliff", atMin57, end.tokens)
	}
	if lo < 0.3 || hi > 0.7 {
		t.Fatalf("share ranged %.2f..%.2f over minutes 20-58, want inside [0.3, 0.7]", lo, hi)
	}
}

func TestPacerSharesFallAfterABurstEvenWhenEverySheddableCallWasShed(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	threads := threadNames(400)
	quiet := lowestHashThreads(3) // the first threads any share turns away
	// A burst to 15M in 5 minutes against a 36M budget.
	offer(h, threads, 5, 50, 60_000, 0)
	t.Logf("burst end (minute 5): share %.2f tokens %d", h.p.snapshot().share, h.p.snapshot().tokens)
	if h.p.snapshot().share < 0.3 {
		t.Fatal("the burst did not raise the share; the test needs it high")
	}
	// Then only the three quiet threads, 60k tokens a minute in all. They are
	// all shed at first, so nothing is served and nothing is measured.
	zeroAt, shedQuiet := -1, 0
	for m := 0; m < 40; m++ {
		sh, _ := offer(h, quiet, 1, 3, 20_000, 0)
		shedQuiet += sh
		if zeroAt < 0 && h.p.snapshot().share == 0 {
			zeroAt = m + 1
		}
		if m == 9 || m == 24 {
			t.Logf("minute %d: share %.2f tokens %d", 5+m+1, h.p.snapshot().share, h.p.snapshot().tokens)
		}
	}
	if zeroAt < 0 || zeroAt > 15 {
		t.Fatalf("share reached 0 %d minutes after the burst (-1 = never), want within 15", zeroAt)
	}
	before := h.p.snapshot().tokens
	if sh, _ := offer(h, quiet, 5, 3, 20_000, 0); sh != 0 {
		t.Fatalf("the quiet threads are still shed after the share fell (%d shed)", sh)
	}
	if h.p.snapshot().tokens <= before {
		t.Fatal("Devin served nothing for the quiet threads")
	}
	t.Logf("share 0 %d minutes after the burst; %d quiet calls were shed on the way", zeroAt, shedQuiet)
}

func TestPacerAllUnsheddableSaturates(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	// 1.2M a minute of calls that cannot be shed (2x the budget rate) and a
	// trickle of keyed calls so the controller is evaluated.
	var peak float64
	for m := 0; m < 58; m++ {
		offer(h, threadNames(10), 1, 2, 60_000, 1_200_000)
		peak = max(peak, h.p.snapshot().share)
		if m == 4 || m == 14 || m == 29 || m == 57 {
			t.Logf("minute %d: share %.2f tokens %d", m+1, h.p.snapshot().share, h.p.snapshot().tokens)
		}
	}
	if peak < devinShedMax {
		t.Fatalf("share peaked at %.2f, want saturation at %.2f or the reserve", peak, devinShedMax)
	}
	end := h.p.snapshot()
	if end.tokens < budgetOf(h) {
		t.Fatalf("tokens %d, want the budget %d reached (nothing could be shed)", end.tokens, budgetOf(h))
	}
	h.clock.Advance(3 * time.Minute)
	h.p.admit("x")
	if !strings.Contains(h.logged(), "shed_share=") {
		t.Fatalf("hour line missing:\n%s", h.logged())
	}
	t.Logf("final tokens %d, peak share %.2f", end.tokens, peak)
}

func TestPacerQuietGapDecays(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	threads := threadNames(400)
	offer(h, threads, 12, 20, 60_000, 0)
	high := h.p.snapshot().share
	if high < 0.3 {
		t.Fatalf("share %v after 12 minutes of 2x demand, the test needs it up", high)
	}
	h.clock.Advance(20 * time.Minute) // no traffic, no admits
	h.p.admit("resume")
	if got := h.p.snapshot().share; got != 0 {
		t.Fatalf("share = %v on the first admit after a 20-minute gap, want 0 (all elapsed 60 s steps applied)", got)
	}
	t.Logf("share %.2f before the gap, %.2f on resume", high, h.p.snapshot().share)
}

func TestPacerCountsOnlyAnsweredCalls(t *testing.T) {
	clock := &fakeClock{now: pacerT0}
	stub := &quotaStub{name: "devin"}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	stub.set(errors.New("devin: read stream frame: INTERNAL_ERROR"))
	_, _ = cbp.Stream(context.Background(), provider.LLMRequest{Model: "swe-2-flash", CacheKey: "k"}, nil)
	if got := cbp.pacer.snapshot().tokens; got != 0 {
		t.Fatalf("tokens after a failed stream = %d, want 0", got)
	}
}

func TestPacerConcurrentCounting(t *testing.T) {
	h := newPacerHarness(t, pacerT0)
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				h.p.record(7, true)
				h.p.admit("t")
			}
		}()
	}
	wg.Wait()
	if got := h.p.snapshot().tokens; got != 16*1000*7 {
		t.Fatalf("tokens = %d, want %d", got, 16*1000*7)
	}
}

// devinStub answers like Devin: usage on success, a typed quota otherwise,
// or blocks until its context ends (a hung probe).
type devinStub struct {
	nm     string
	mu     sync.Mutex
	err    error
	hang   bool
	input  int
	calls  int
	deadln []bool
}

func (s *devinStub) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	s.mu.Lock()
	s.calls++
	err, hang, in := s.err, s.hang, s.input
	_, has := ctx.Deadline()
	s.deadln = append(s.deadln, has)
	s.mu.Unlock()
	if hang {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	return &provider.LLMResponse{Text: "ok", Usage: provider.Usage{InputTokens: in, CachedInputTokens: in * 3}}, nil
}
func (s *devinStub) Stream(ctx context.Context, req provider.LLMRequest, f func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	return s.Call(ctx, req)
}
func (s *devinStub) Name() string {
	if s.nm != "" {
		return s.nm
	}
	return "devin"
}
func (s *devinStub) IsReal() bool { return true }
func (s *devinStub) set(f func(*devinStub)) {
	s.mu.Lock()
	f(s)
	s.mu.Unlock()
}

func TestDevinShedIsATypedQuotaAnswerAndOtherProvidersAreNotPaced(t *testing.T) {
	clock := &fakeClock{now: pacerT0.Add(5 * time.Minute)}
	stub := &devinStub{input: 100_000}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	// Spend 60% of the budget at minute 5: far ahead of the line.
	cbp.pacer.record(int(devinCeilingStart*0.54), true)
	clock.Advance(devinShedEvalEvery)
	shed, served := 0, 0
	for _, th := range threadNames(300) {
		resp, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "swe-2-flash", CacheKey: th})
		if err == nil {
			served++
			if resp == nil {
				t.Fatal("nil response")
			}
			continue
		}
		q, ok := provider.AsQuotaExhausted(err)
		if !ok || !q.Shed {
			t.Fatalf("shed answer = %v, want a typed quota with Shed set", err)
		}
		if !q.RetryAt.After(clock.Now().Add(5*time.Second)) || !q.RetryAtEstimated {
			t.Fatalf("shed RetryAt = %v, want an estimate comfortably past now (guest clocks skew)", q.RetryAt)
		}
		if !strings.Contains(err.Error(), "provider quota exhausted (devin)") {
			t.Fatalf("shed text %q must read as a quota so older guests switch models", err.Error())
		}
		shed++
	}
	if shed == 0 || served == 0 {
		t.Fatalf("shed=%d served=%d, want both (Devin keeps serving the remaining threads)", shed, served)
	}
	if got := cbp.ModelBreaker("swe-2-flash").Snapshot().State; got != health.StateClosed {
		t.Fatalf("a shed opened the breaker (%v); it is pacing, not a quota", got)
	}
	// A call that says it has no next pool member is never shed.
	for _, th := range threadNames(300) {
		if _, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "swe-2-flash", CacheKey: th, NoShed: true}); err != nil {
			t.Fatalf("NoShed call shed: %v", err)
		}
	}
	// A call with no thread key cannot be sticky, so it is never shed.
	if _, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "swe-2-flash"}); err != nil {
		t.Fatalf("unkeyed call shed: %v", err)
	}

	// Another provider with the same traffic is never paced.
	other := NewCircuitBreakingProvider(&quotaStub{name: "synthetic"}, health.BreakerConfig{Now: clock.Now})
	if other.pacer != nil {
		t.Fatal("a non-devin provider has a pacer")
	}
}

func TestDevinTripFeedsThePacer(t *testing.T) {
	clock := &fakeClock{now: pacerT0}
	stub := &devinStub{input: 1_000_000}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	for i := 0; i < 20; i++ {
		clock.Advance(time.Minute)
		if _, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "swe-2-flash"}); err != nil {
			t.Fatal(err)
		}
	}
	stub.set(func(s *devinStub) {
		s.err = &provider.QuotaExhaustedError{Provider: "devin", Status: "429", RetryAt: pacerT0.Add(time.Hour)}
	})
	if _, err := cbp.Call(context.Background(), provider.LLMRequest{Model: "swe-2-flash"}); err == nil {
		t.Fatal("expected the quota error")
	}
	s := cbp.pacer.snapshot()
	if s.trips != 1 || s.ceiling != 19_000_000 {
		t.Fatalf("pacer after the trip: trips=%d ceiling=%d, want 1 and 19M (95%% of 20M counted)", s.trips, s.ceiling)
	}
}

func TestQuotaProbeIsBounded(t *testing.T) {
	// 09:00 reset, the probe hangs (the 2026-11-10 hold). With a deadline the
	// probe fails, the breaker reopens on its short backoff, and the next
	// probe after that backoff is bounded too (the quota hold is gone by
	// then), until one answers and closes the breaker.
	clock := &fakeClock{now: time.Date(2026, 11, 10, 8, 30, 0, 0, time.UTC)}
	stub := &devinStub{}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	cbp.probeDeadline = 20 * time.Millisecond
	reset := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	stub.set(func(s *devinStub) {
		s.err = &provider.QuotaExhaustedError{Provider: "devin", Status: "429", RetryAt: reset}
	})
	req := provider.LLMRequest{Model: "swe-2-flash"}
	_, _ = cbp.Call(context.Background(), req)

	clock.Advance(31 * time.Minute) // 09:01, half-open
	stub.set(func(s *devinStub) { s.err, s.hang = nil, true })
	for probe := 1; probe <= 2; probe++ {
		start := time.Now()
		_, err := cbp.Call(context.Background(), req)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("probe %d: hung probe error = %v, want the deadline", probe, err)
		}
		if time.Since(start) > 5*time.Second {
			t.Fatalf("probe %d was not bounded", probe)
		}
		stub.mu.Lock()
		hadDeadline := stub.deadln[len(stub.deadln)-1]
		stub.mu.Unlock()
		if !hadDeadline {
			t.Fatalf("probe %d reached the upstream without a deadline", probe)
		}
		snap := cbp.ModelBreaker("swe-2-flash").Snapshot()
		if snap.State != health.StateOpen || snap.OpenUntil == nil {
			t.Fatalf("after failed probe %d: %+v, want open on a backoff", probe, snap)
		}
		if wait := snap.OpenUntil.Sub(clock.Now()); wait > 5*time.Minute {
			t.Fatalf("backoff after failed probe %d = %v, want short", probe, wait)
		}
		// While reopened nothing hangs: the call is refused at once.
		if _, err := cbp.Call(context.Background(), req); err == nil {
			t.Fatal("expected a refusal while reopened")
		}
		clock.Advance(snap.OpenUntil.Sub(clock.Now()) + time.Second)
	}
	stub.set(func(s *devinStub) { s.hang, s.input = false, 10 })
	if _, err := cbp.Call(context.Background(), req); err != nil {
		t.Fatalf("third probe: %v", err)
	}
	if got := cbp.ModelBreaker("swe-2-flash").Snapshot().State; got != health.StateClosed {
		t.Fatalf("state after the good probe = %v, want closed", got)
	}
}

// tickClock advances by step on every read, so a decision made from one read
// and an admission made from a later one can straddle the reset.
type tickClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *tickClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now
	c.now = c.now.Add(c.step)
	return t
}

func TestQuotaProbeAdmittedJustAtTheResetIsBounded(t *testing.T) {
	// The call that Allow() admits as the probe is decided at admission: a
	// state read taken a moment earlier (still open) must not decide the
	// context. Start the clock a few seconds before the reset, ticking.
	reset := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	admittedAny := false
	for lead := 0; lead <= 3; lead++ {
		clock := &tickClock{now: reset.Add(-time.Duration(lead) * time.Second), step: time.Second}
		stub := &devinStub{}
		cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
		cbp.probeDeadline = 20 * time.Millisecond
		// Trip with the clock frozen well before the reset.
		clock.mu.Lock()
		clock.now, clock.step = reset.Add(-30*time.Minute), 0
		clock.mu.Unlock()
		stub.set(func(s *devinStub) {
			s.err = &provider.QuotaExhaustedError{Provider: "devin", Status: "429", RetryAt: reset}
		})
		req := provider.LLMRequest{Model: "swe-2-flash"}
		_, _ = cbp.Call(context.Background(), req)
		stub.set(func(s *devinStub) { s.err, s.hang = nil, true })
		clock.mu.Lock()
		clock.now, clock.step = reset.Add(-time.Duration(lead)*time.Second), time.Second
		clock.mu.Unlock()
		before := stub.callCount()
		_, _ = cbp.Call(context.Background(), req)
		if stub.callCount() == before {
			continue // refused: admitted nothing, nothing to bound
		}
		admittedAny = true
		stub.mu.Lock()
		hadDeadline := stub.deadln[len(stub.deadln)-1]
		stub.mu.Unlock()
		if !hadDeadline {
			t.Fatalf("lead %ds: the call admitted as the probe ran without a deadline", lead)
		}
	}
	if !admittedAny {
		t.Fatal("no lead produced an admitted probe; the test proved nothing")
	}
}

func (s *devinStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestNonDevinProbesStayBoundedForTheWholeRecoveryEpisode(t *testing.T) {
	clock := &fakeClock{now: time.Date(2026, 11, 10, 8, 30, 0, 0, time.UTC)}
	stub := &devinStub{nm: "synthetic"}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second, Now: clock.Now})
	cbp.probeDeadline = 20 * time.Millisecond
	reset := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	stub.set(func(s *devinStub) {
		s.err = &provider.QuotaExhaustedError{Provider: "synthetic", Status: "429", RetryAt: reset}
	})
	req := provider.LLMRequest{Model: "m"}
	_, _ = cbp.Call(context.Background(), req)
	clock.Advance(31 * time.Minute)
	stub.set(func(s *devinStub) { s.err, s.hang = nil, true })
	for probe := 1; probe <= 3; probe++ {
		if _, err := cbp.Call(context.Background(), req); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("probe %d: %v, want the deadline", probe, err)
		}
		stub.mu.Lock()
		had := stub.deadln[len(stub.deadln)-1]
		stub.mu.Unlock()
		if !had {
			t.Fatalf("probe %d ran without a deadline", probe)
		}
		snap := cbp.ModelBreaker("m").Snapshot()
		clock.Advance(snap.OpenUntil.Sub(clock.Now()) + time.Second)
	}
	// A recovery that closes the breaker ends the episode: an ordinary slow
	// call after that is not bounded.
	stub.set(func(s *devinStub) { s.hang, s.input = false, 1 })
	if _, err := cbp.Call(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if cbp.heldForQuota("m") {
		t.Fatal("the recovery episode outlived the closed breaker")
	}
}
