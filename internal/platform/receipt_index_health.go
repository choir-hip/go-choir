package platform

import (
	"slices"
	"sync"
	"time"
)

// The signal the receipt index migration reads between its phases (owner's
// ruling N3 for the live table: one index at a time, stop if the busiest
// computer's append times rise).
//
// Nothing in corpusd measured the host-side append before this: the retrieval
// metrics time Store B reads, the guest appender times the guest's request, and
// PostgreSQL's pg_stat_statements is not loaded. So the smallest read-only
// signal is the one the append already has in hand: the wall time of the
// serializable transaction (compareAndSwapTx) that inserts the head and the
// receipt, recorded per computer in a fixed ring in memory. It is a number
// read, never a gate on an append, and it is lost at a restart (the migration
// then reads no samples and, having no baseline, does not wait).

// appendTimingRing is how many recent appends the ring keeps, across computers.
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

// appendLoad is the busiest computer's recent append times.
type appendLoad struct {
	Computer string
	Count    int
	P95      time.Duration
}

// busiest returns the computer with the most appends since `since`, and the
// 95th percentile of its append times.
func (t *appendTimings) busiest(since time.Time) appendLoad {
	t.mu.Lock()
	counts := map[string][]time.Duration{}
	for i := 0; i < t.filled; i++ {
		s := t.samples[i]
		if s.at.Before(since) {
			continue
		}
		counts[s.computer] = append(counts[s.computer], s.took)
	}
	t.mu.Unlock()
	var best appendLoad
	for id, took := range counts {
		if len(took) > best.Count || (len(took) == best.Count && id < best.Computer) {
			slices.Sort(took)
			best = appendLoad{Computer: id, Count: len(took), P95: took[(len(took)*95+99)/100-1]}
		}
	}
	return best
}

const (
	// receiptIndexMinAppends is the fewest appends in a window that say
	// anything about append times; with fewer the computer is quiet and the
	// database is not being asked for anything the build could slow.
	receiptIndexMinAppends = 20
	// receiptIndexBaselineWindow and receiptIndexHealthWindow are the spans
	// the baseline before the migration and each check read.
	receiptIndexBaselineWindow = 10 * time.Minute
	receiptIndexHealthWindow   = 2 * time.Minute
	// receiptIndexFloor keeps a baseline of a few milliseconds from making any
	// jitter look like a rise.
	receiptIndexFloor = 50 * time.Millisecond
	// A phase's append times have risen when the busiest computer's p95 is both
	// receiptIndexRise times the baseline and more than receiptIndexSlack above it.
	receiptIndexRise  = 2
	receiptIndexSlack = 250 * time.Millisecond
)

// appendHealth is a baseline taken before the migration touches the table and
// the test of a later reading against it.
type appendHealth struct {
	timings  *appendTimings
	baseline time.Duration
	known    bool
}

func newAppendHealth(t *appendTimings, now time.Time) appendHealth {
	load := t.busiest(now.Add(-receiptIndexBaselineWindow))
	h := appendHealth{timings: t}
	if load.Count >= receiptIndexMinAppends {
		h.known = true
		h.baseline = max(load.P95, receiptIndexFloor)
	}
	return h
}

// degraded reports whether the busiest computer's append times have risen over
// the baseline, and what it read. With no baseline or a quiet window it is
// never degraded: there is nothing to compare, and nothing to slow.
func (h appendHealth) degraded(now time.Time) (bool, appendLoad) {
	load := h.timings.busiest(now.Add(-receiptIndexHealthWindow))
	if !h.known || load.Count < receiptIndexMinAppends {
		return false, load
	}
	limit := max(h.baseline*receiptIndexRise, h.baseline+receiptIndexSlack)
	return load.P95 > limit, load
}
