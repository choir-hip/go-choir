package platform

import (
	"fmt"
	"slices"
	"sync"
	"time"
)

// The signal the receipt index migration reads (owner's ruling N3 for the live
// table: one index at a time, stop if the busiest computer's append times
// rise).
//
// Nothing in corpusd measured the host-side append before this: the retrieval
// metrics time Store B reads, the guest appender times its own request, and
// PostgreSQL's pg_stat_statements is not loaded. So the smallest read-only
// signal is the one the append already has in hand: the wall time of the
// serializable transaction (compareAndSwapTx) that inserts the head and the
// receipt, recorded per computer in a fixed ring in memory. It is a number
// read, never a gate on an append. It is lost at a restart, which is why the
// migration takes its baseline only after the listener is serving and the ring
// holds appends (awaitAppendBaseline).

// appendTimingRing is how many recent appends the ring keeps, across computers.
// Above about 13 appends a second the ring holds less than the ten minutes of
// the baseline window; the baseline then covers what fits, and the log says
// over how many appends and how long.
const appendTimingRing = 8192

type appendSample struct {
	at       time.Time
	computer string
	took     time.Duration
}

type appendTimings struct {
	mu      sync.Mutex
	samples [appendTimingRing]appendSample
	next    int
	filled  int
	born    time.Time // when something first began to watch the ring
}

func (t *appendTimings) record(computerID string, took time.Duration) {
	t.mu.Lock()
	t.samples[t.next] = appendSample{at: time.Now(), computer: computerID, took: took}
	t.next = (t.next + 1) % appendTimingRing
	if t.filled < appendTimingRing {
		t.filled++
	}
	t.mu.Unlock()
}

// watch marks the first moment something began to read the ring.
func (t *appendTimings) watch(now time.Time) time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.born.IsZero() {
		t.born = now
	}
	return t.born
}

// since copies the samples at or after `since`.
func (t *appendTimings) since(since time.Time) []appendSample {
	t.mu.Lock()
	out := make([]appendSample, 0, t.filled)
	for i := 0; i < t.filled; i++ {
		if s := t.samples[i]; !s.at.Before(since) {
			out = append(out, s)
		}
	}
	t.mu.Unlock()
	return out
}

// appendLoad is one computer's append times over a window.
type appendLoad struct {
	Computer string
	Count    int
	P95      time.Duration
	First    time.Time // the oldest sample counted
}

func loadOf(computer string, took []time.Duration, first time.Time) appendLoad {
	slices.Sort(took)
	return appendLoad{Computer: computer, Count: len(took), P95: took[(len(took)*95+99)/100-1], First: first}
}

// load returns the named computer's appends since `since`.
func (t *appendTimings) load(computer string, since time.Time) appendLoad {
	var took []time.Duration
	var first time.Time
	for _, s := range t.since(since) {
		if s.computer != computer {
			continue
		}
		if first.IsZero() || s.at.Before(first) {
			first = s.at
		}
		took = append(took, s.took)
	}
	if len(took) == 0 {
		return appendLoad{Computer: computer}
	}
	return loadOf(computer, took, first)
}

// busiest returns the computer with the most appends since `since`.
func (t *appendTimings) busiest(since time.Time) appendLoad {
	by := map[string][]time.Duration{}
	firsts := map[string]time.Time{}
	for _, s := range t.since(since) {
		by[s.computer] = append(by[s.computer], s.took)
		if f, ok := firsts[s.computer]; !ok || s.at.Before(f) {
			firsts[s.computer] = s.at
		}
	}
	var best appendLoad
	for id, took := range by {
		if len(took) > best.Count || (len(took) == best.Count && id < best.Computer) {
			best = loadOf(id, took, firsts[id])
		}
	}
	return best
}

const (
	// receiptIndexMinAppends is the fewest appends in a window that say
	// anything about append times.
	receiptIndexMinAppends = 20
	// receiptIndexHealthWindow is the span each check reads.
	receiptIndexHealthWindow = 2 * time.Minute
	// receiptIndexFloor keeps a baseline of a few milliseconds from making any
	// jitter look like a rise.
	receiptIndexFloor = 50 * time.Millisecond
	// A computer's append times have risen when its p95 is both
	// receiptIndexRise times its baseline and more than receiptIndexSlack above it.
	receiptIndexRise  = 2
	receiptIndexSlack = 250 * time.Millisecond
	// The appends have collapsed when, against a baseline that had at least
	// receiptIndexCollapseMin appends in a span, a tenth or fewer of the
	// expected number completed.
	receiptIndexCollapseMin = 4 * receiptIndexMinAppends
)

// appendBaseline is the busiest computer's append p95 and rate before the
// migration touches the table.
type appendBaseline struct {
	Computer string
	P95      time.Duration
	Count    int
	Span     time.Duration
}

func (b appendBaseline) rate() float64 { // appends per second
	return float64(b.Count) / max(b.Span, time.Second).Seconds()
}

// appendHealth is a baseline and the test of a later reading against it.
type appendHealth struct {
	timings  *appendTimings
	baseline appendBaseline
	known    bool
	at       time.Time // when the baseline was taken
}

func newAppendHealth(t *appendTimings, now time.Time) appendHealth {
	load := t.busiest(now.Add(-receiptIndexBaselineWindow))
	h := appendHealth{timings: t, at: now}
	if load.Count >= receiptIndexMinAppends {
		h.known = true
		h.baseline = appendBaseline{Computer: load.Computer, P95: max(load.P95, receiptIndexFloor), Count: load.Count, Span: now.Sub(load.First)}
	}
	return h
}

// degraded reports whether the baseline's own computer has slowed or stopped
// since the baseline: its p95 over the last two minutes both receiptIndexRise
// times the baseline and receiptIndexSlack above it, or a collapse of its
// append rate. With no baseline there is nothing to compare, and the second
// result says why it is or is not degraded.
func (h appendHealth) degraded(now time.Time) (bool, string) {
	if !h.known {
		return false, "no baseline"
	}
	b := h.baseline
	from := now.Add(-receiptIndexHealthWindow)
	if from.Before(h.at) {
		from = h.at
	}
	cur := h.timings.load(b.Computer, from)
	if cur.Count >= receiptIndexMinAppends {
		limit := max(b.P95*receiptIndexRise, b.P95+receiptIndexSlack)
		if cur.P95 > limit {
			return true, fmt.Sprintf("%s: append p95 %s over %d appends, baseline %s", b.Computer, cur.P95, cur.Count, b.P95)
		}
	}
	if span := now.Sub(from); span >= 10*time.Second {
		if expected := b.rate() * span.Seconds(); expected >= receiptIndexCollapseMin && float64(cur.Count) <= expected/10 {
			return true, fmt.Sprintf("%s: %d appends in %s where its baseline of %d in %s expects about %.0f", b.Computer, cur.Count, span.Round(time.Second), b.Count, b.Span.Round(time.Second), expected)
		}
	}
	return false, fmt.Sprintf("%s: append p95 %s over %d appends, baseline %s", b.Computer, cur.P95, cur.Count, b.P95)
}
