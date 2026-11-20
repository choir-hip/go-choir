package agentcore

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// One line per research run on how its searches went (search-reuse,
// docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md): the
// desk's worker counts per cell (yaegikernel.SearchStats), the host sums them
// per run, and the line is logged when the run ends. Counts and ids only.

type runSearchTotals struct {
	desk  string
	stats yaegikernel.SearchStats
}

var runSearchMu sync.Mutex

// noteRunSearchStats adds one cell's counts to its run.
func (rt *Runtime) noteRunSearchStats(runID, desk string, stats yaegikernel.SearchStats) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" || stats.IsZero() {
		return
	}
	runSearchMu.Lock()
	defer runSearchMu.Unlock()
	v, _ := rt.runSearch.Load(runID)
	t, _ := v.(*runSearchTotals)
	if t == nil {
		t = &runSearchTotals{desk: desk}
		rt.runSearch.Store(runID, t)
	}
	t.stats.Add(stats)
}

// takeRunSearchSummary returns the run's summary line and forgets the run's
// counts; ok is false when the run searched nothing.
func (rt *Runtime) takeRunSearchSummary(runID string) (string, bool) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" {
		return "", false
	}
	runSearchMu.Lock()
	v, loaded := rt.runSearch.LoadAndDelete(runID)
	runSearchMu.Unlock()
	t, _ := v.(*runSearchTotals)
	if !loaded || t == nil {
		return "", false
	}
	s := t.stats
	notSent, reasons := 0, make([]string, 0, len(s.NotSent))
	for reason, n := range s.NotSent {
		notSent += n
		reasons = append(reasons, fmt.Sprintf("%s=%d", reason, n))
	}
	sort.Strings(reasons)
	line := fmt.Sprintf("search run=%s desk=%s started=%d from_held=%d not_sent=%d", runID, t.desk, s.Started, s.FromHeld, notSent)
	if len(reasons) > 0 {
		line += " (" + strings.Join(reasons, " ") + ")"
	}
	line += fmt.Sprintf(" search_all_calls=%d wordings_offered=%d wordings_sent=%d fresh_over_unread=%d untagged=%d", s.AllCalls, s.AllOffered, s.AllSent, s.FreshOverUnread, s.Untagged)
	line += fmt.Sprintf(" pages_returned=%d pages_new=%d pages_read=%d", s.PagesReturned, s.PagesNew, s.PagesRead)
	return line, true
}

// logRunSearchSummary logs the run's summary line when the run ends.
func (rt *Runtime) logRunSearchSummary(runID string) {
	if line, ok := rt.takeRunSearchSummary(runID); ok {
		log.Printf("runtime: %s", line)
	}
}
