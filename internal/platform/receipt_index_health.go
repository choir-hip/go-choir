package platform

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"
)

// An observation of the host-side append, nothing more. Nothing in corpusd
// measured it before: the retrieval metrics time Store B reads, the guest
// appender times its own request, and PostgreSQL's pg_stat_statements is not
// loaded. The wall time of the serializable transaction (compareAndSwapTx) that
// inserts the head and the receipt is kept per computer in a fixed ring in
// memory, and corpusd logs one line a minute from it. It is the reading an
// operator watches while the receipt table's indexes are swapped by hand
// (scripts/receipts-index-swap); no code gates on it, waits for it or cancels
// anything because of it. It is lost at a restart.

// appendTimingRing is how many recent appends the ring keeps, across computers.
// Above about 13 appends a second the ring holds less than the ten minutes the
// line reports; the count in the line says how many it read.
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

// record keeps one successful append's transaction time.
func (t *appendTimings) record(computerID string, took time.Duration) {
	t.mu.Lock()
	t.samples[t.next] = appendSample{at: time.Now(), computer: computerID, took: took}
	t.next = (t.next + 1) % appendTimingRing
	if t.filled < appendTimingRing {
		t.filled++
	}
	t.mu.Unlock()
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
	P50      time.Duration
	P95      time.Duration
}

func percentile(sorted []time.Duration, p int) time.Duration {
	return sorted[(len(sorted)*p+99)/100-1]
}

// load returns the named computer's appends since `since`.
func (t *appendTimings) load(computer string, since time.Time) appendLoad {
	var took []time.Duration
	for _, s := range t.since(since) {
		if s.computer == computer {
			took = append(took, s.took)
		}
	}
	if len(took) == 0 {
		return appendLoad{Computer: computer}
	}
	slices.Sort(took)
	return appendLoad{Computer: computer, Count: len(took), P50: percentile(took, 50), P95: percentile(took, 95)}
}

// busiest returns the computer with the most appends since `since` and its
// p50 and p95; the zero value when there were none.
func (t *appendTimings) busiest(since time.Time) appendLoad {
	by := map[string][]time.Duration{}
	for _, s := range t.since(since) {
		by[s.computer] = append(by[s.computer], s.took)
	}
	var best appendLoad
	var bestTook []time.Duration
	for id, took := range by {
		if len(took) > best.Count || (len(took) == best.Count && id < best.Computer) {
			best, bestTook = appendLoad{Computer: id, Count: len(took)}, took
		}
	}
	if best.Count == 0 {
		return best
	}
	slices.Sort(bestTook)
	best.P50, best.P95 = percentile(bestTook, 50), percentile(bestTook, 95)
	return best
}

func (l appendLoad) String() string {
	if l.Count == 0 {
		if l.Computer != "" {
			return "computer=" + l.Computer + " n=0"
		}
		return "n=0"
	}
	return fmt.Sprintf("busiest=%s n=%d p50=%s p95=%s", l.Computer, l.Count, l.P50, l.P95)
}

// appendTimingLine is the line corpusd logs a minute: the busiest computer's
// append count and p50 and p95 over the last minute and over the last ten.
func (s *Store) appendTimingLine(now time.Time) string {
	ten := s.appendTimes.busiest(now.Add(-10 * time.Minute))
	one := s.appendTimes.busiest(now.Add(-time.Minute))
	line := fmt.Sprintf("corpusd append timing: 1m %s; 10m %s", one, ten)
	if ten.Count > 0 && one.Computer != ten.Computer {
		// The busiest computer of the last minute is not the busiest of the last
		// ten: the ten-minute computer's own last minute, so that a reading can
		// always be taken for the same computer.
		line += "; 1m of " + ten.Computer + " " + s.appendTimes.load(ten.Computer, now.Add(-time.Minute)).String()
	}
	return line
}

// StartAppendTimingLog logs the timing line every minute until ctx ends. It
// returns at once.
func (s *Store) StartAppendTimingLog(ctx context.Context) {
	if s == nil {
		return
	}
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-tick.C:
				log.Print(s.appendTimingLine(now))
			}
		}
	}()
}
