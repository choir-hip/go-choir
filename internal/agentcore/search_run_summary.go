package agentcore

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// One line per research run on how its searches went (search-reuse,
// docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md): the
// desk's worker counts per cell (yaegikernel.SearchStats), the host sums them
// per run, and the line is logged when the run ends. Counts and ids only.

type runSearchTotals struct {
	desk  string
	stats yaegikernel.SearchStats
	// children counts the choir.Research children the run's tree started, and
	// started the searches each desk of the tree sent (research, research_web),
	// so the deployed reading shows where a run's searches come from.
	children int
	started  map[string]int
	// resumed is set when the run began before this process did: its earlier
	// segment's counts were in the old process's memory and are not in stats.
	resumed bool
}

var runSearchMu sync.Mutex

// noteRunSearchStats adds one cell's counts to its run.
func (rt *Runtime) noteRunSearchStats(runID, desk string, stats yaegikernel.SearchStats) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" || stats.IsZero() {
		return
	}
	// The entry is made under the lock before the store is asked whether a
	// restart cut the run, so a run that ends meanwhile still logs its line (with
	// the flag unset); the flag is set afterwards if the entry is still there.
	runSearchMu.Lock()
	v, _ := rt.runSearch.Load(runID)
	t, _ := v.(*runSearchTotals)
	created := false
	if t == nil {
		t = &runSearchTotals{desk: desk}
		rt.runSearch.Store(runID, t)
		created = true
	}
	t.stats.Add(stats)
	if t.started == nil {
		t.started = map[string]int{}
	}
	t.started[desk] += stats.Started
	runSearchMu.Unlock()
	if !created {
		return
	}
	resumed := rt.runStartedBeforeBoot(runID)
	runSearchMu.Lock()
	if cur, _ := rt.runSearch.Load(runID); cur == any(t) {
		t.resumed = resumed
	}
	runSearchMu.Unlock()
}

// noteRunChild counts one choir.Research child of the run's tree.
func (rt *Runtime) noteRunChild(runID string) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" {
		return
	}
	runSearchMu.Lock()
	defer runSearchMu.Unlock()
	v, _ := rt.runSearch.Load(runID)
	t, _ := v.(*runSearchTotals)
	if t == nil {
		t = &runSearchTotals{desk: deskmanifest.ResearcherName()}
		rt.runSearch.Store(runID, t)
	}
	t.children++
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
	// Where the tree's searches come from: the children it started and the searches
	// each desk sent (fixed tokens).
	byDesk := make([]string, 0, len(t.started))
	for desk, n := range t.started {
		byDesk = append(byDesk, fmt.Sprintf("started_%s=%d", desk, n))
	}
	sort.Strings(byDesk)
	line += fmt.Sprintf(" children=%d", t.children)
	if len(byDesk) > 0 {
		line += " " + strings.Join(byDesk, " ")
	}
	line += fmt.Sprintf(" search_all_calls=%d wordings_offered=%d wordings_sent=%d fresh_sent=%d untagged=%d wordings_running=%d", s.AllCalls, s.AllOffered, s.AllSent, s.FreshSent, s.Untagged, s.AllRunning)
	line += fmt.Sprintf(" pages_returned=%d pages_new=%d pages_read=%d", s.PagesReturned, s.PagesNew, s.PagesRead)
	// Landing 0 of docs/definitions/corpus-first-search-one-verb-2026-11-19.md:
	// searches judged and how many came back 75% or more already held; the
	// outlets held at the run's sends; why reuse declined a prior identical
	// search, one reuse_declined_<token> count each. Counts and fixed tokens.
	line += fmt.Sprintf(" searches_judged=%d searches_mostly_seen=%d outlets_held_max=%d outlets_held_last=%d", s.SeenJudged, s.SeenMostly, s.OutletsMax, s.OutletsLast)
	declined := make([]string, 0, len(s.ReuseDeclined))
	for reason, n := range s.ReuseDeclined {
		declined = append(declined, fmt.Sprintf("reuse_declined_%s=%d", reason, n))
	}
	sort.Strings(declined)
	if len(declined) > 0 {
		line += " " + strings.Join(declined, " ")
	}
	if t.resumed {
		// The run began before this process did: a planned restart parked it and it
		// resumed. The counts above are this segment's only; the earlier segment's
		// were in the old process's memory.
		line += " resumed_segment=1"
	}
	return line, true
}

// logRunSearchSummary logs the run's summary line when the run ends.
func (rt *Runtime) logRunSearchSummary(runID string) {
	if line, ok := rt.takeRunSearchSummary(runID); ok {
		log.Printf("runtime: %s", line)
	}
}

// runStartedBeforeBoot reports whether the run began executing before this
// process booted: a loop.started event older than the boot. For a run still
// searching that means a planned restart resumed it and the counts are one
// segment's. A run created before the boot that only started after it is not
// flagged. False when the record or events cannot be read within two seconds.
func (rt *Runtime) runStartedBeforeBoot(runID string) bool {
	if rt == nil || rt.store == nil || rt.bootedAt.IsZero() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rec, err := rt.store.GetRun(ctx, runID)
	if err != nil || rec.CreatedAt.IsZero() || !rec.CreatedAt.Before(rt.bootedAt) {
		return false
	}
	events, err := rt.store.ListRunEventsOfKind(ctx, rec.OwnerID, runID, types.EventRunStarted)
	if err != nil {
		return false
	}
	for _, e := range events {
		if e.Timestamp.Before(rt.bootedAt) {
			return true
		}
	}
	return false
}
