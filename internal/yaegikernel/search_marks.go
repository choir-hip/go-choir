package yaegikernel

import (
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
)

// Which pages a search added (search-reuse commit 1 counts them for the host;
// commit 2 shows them to the model). A page is held when an earlier search of
// the current run already returned it, new otherwise; it is read when the run
// captured it. Pages are keyed as search_seen.go keys them (searchHitKey:
// fragment dropped, host lower-cased, trailing slash trimmed). "Earlier" is
// the order the searches were started in, so a burst fired at one instant
// still has a first, a second and a third.

// inRun reports whether search i started under the run the session is serving.
func (st *SearchStore) inRun(i int) bool {
	return st.searches[st.order[i]].run == st.run
}

// markRunStart makes runID the run the session serves: its searches are held
// against each other, and pages another run's searches returned are new again.
// A run the session served before finds its own searches and their pages (the
// held set is rebuilt from them), so another run's cell between two of its
// cells does not wipe what it holds.
func (st *SearchStore) markRunStart(runID string) {
	if st == nil {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	st.run = runID
	// The brief pages of every other run are dropped: each cell's frame carries its
	// run's Held, so a run served again is seeded again (seedHeld).
	for k := range st.heldRun {
		if k != runID {
			delete(st.heldRun, k)
		}
	}
	st.first = map[string]int{}
	for i, id := range st.order {
		if c := st.searches[id]; c.run == runID {
			for _, a := range c.Arrivals {
				st.noteKeysLocked(i, a.Results)
			}
		}
	}
}

// noteKeysLocked records that search idx returned these pages, for the held
// test. A search of another run records nothing.
func (st *SearchStore) noteKeysLocked(idx int, hits []SearchHit) {
	if !st.inRun(idx) {
		return
	}
	if st.first == nil {
		st.first = map[string]int{}
	}
	for _, h := range hits {
		key := searchHitKey(h.URL)
		if cur, ok := st.first[key]; !ok || idx < cur {
			st.first[key] = idx
		}
	}
}

// readSet is the pages this run read (nil when the store has no ledger).
func (st *SearchStore) readSet() map[string]bool {
	if st.readKeys == nil {
		return nil
	}
	return st.readKeys()
}

// markOneLocked returns a copy of search i with Held and Read set on each
// hit and the page counts filled in; a search of an earlier run carries no
// marks. It reads only that search's results.
func (st *SearchStore) markOneLocked(i int, read map[string]bool) Search {
	c := copySearch(st.searches[st.order[i]])
	if !st.inRun(i) {
		return c
	}
	for j := range c.Results {
		key := searchHitKey(c.Results[j].URL)
		if f, ok := st.first[key]; ok && f < i {
			c.Results[j].Held = true
			c.HeldPages++
		} else {
			c.NewPages++
		}
		if read[key] {
			c.Results[j].Read = true
			c.ReadPages++
		}
	}
	return c
}

// markedLocked returns a copy of every search, oldest first, marked.
func (st *SearchStore) markedLocked() []Search {
	read := st.readSet()
	out := make([]Search, 0, len(st.order))
	for i := range st.order {
		out = append(out, st.markOneLocked(i, read))
	}
	return out
}

// SearchStats are the counts of one cell (or, summed by the host, of one
// run): searches the gateway started, searches answered from held results,
// wordings not sent by reason, pages returned, pages new, pages read. Counts
// only, never a query or a URL.
type SearchStats struct {
	Started       int            `json:"started,omitempty"`
	FromHeld      int            `json:"from_held,omitempty"`
	NotSent       map[string]int `json:"not_sent,omitempty"`
	PagesReturned int            `json:"pages_returned,omitempty"`
	PagesNew      int            `json:"pages_new,omitempty"`
	PagesRead     int            `json:"pages_read,omitempty"`
	// SearchAll: calls made, wordings offered to them, wordings they sent.
	AllCalls   int `json:"all_calls,omitempty"`
	AllOffered int `json:"all_offered,omitempty"`
	AllSent    int `json:"all_sent,omitempty"`
	// FreshSent counts searches sent with choir.SearchFresh: over the need
	// test, with a recorded reason.
	FreshSent int `json:"fresh_sent,omitempty"`
	// Untagged counts searches SearchAll sent that its gate could not tag.
	Untagged int `json:"untagged,omitempty"`
	// AllRunning counts wordings SearchAll did not send because another search
	// of the run was still running. They are not in AllOffered: the call could
	// not have sent them, so the sent share of AllOffered stays honest.
	AllRunning int `json:"all_running,omitempty"`
	// ReuseDeclined counts, by fixed token, why in-run reuse did not answer a
	// search although the run had sent an identical one (search_measure.go);
	// "fresh" counts searches sent over a match that would have answered.
	ReuseDeclined map[string]int `json:"reuse_declined,omitempty"`
	// SeenJudged counts finished searches of the run; SeenMostly those whose
	// pages were 75% or more already held.
	SeenJudged int `json:"seen_judged,omitempty"`
	SeenMostly int `json:"seen_mostly,omitempty"`
	// OutletsMax and OutletsLast are the distinct outlets (registrable
	// domains) the run held at its sends: the most, and at the latest send.
	OutletsMax  int `json:"outlets_max,omitempty"`
	OutletsLast int `json:"outlets_last,omitempty"`
	// OutletsSends counts the sends those two were taken at, so a send that held
	// no outlet is the latest value, not a stale one.
	OutletsSends int `json:"outlets_sends,omitempty"`
}

// IsZero reports whether nothing was counted.
func (a SearchStats) IsZero() bool {
	return a.Started == 0 && a.FromHeld == 0 && len(a.NotSent) == 0 && a.PagesReturned == 0 && a.PagesNew == 0 && a.PagesRead == 0 &&
		a.AllCalls == 0 && a.AllOffered == 0 && a.AllSent == 0 && a.FreshSent == 0 && a.Untagged == 0 && a.AllRunning == 0 &&
		len(a.ReuseDeclined) == 0 && a.SeenJudged == 0 && a.SeenMostly == 0 && a.OutletsMax == 0 && a.OutletsLast == 0 && a.OutletsSends == 0
}

// Add sums o into a.
func (a *SearchStats) Add(o SearchStats) {
	a.Started += o.Started
	a.FromHeld += o.FromHeld
	a.PagesReturned += o.PagesReturned
	a.PagesNew += o.PagesNew
	a.PagesRead += o.PagesRead
	a.AllCalls += o.AllCalls
	a.AllOffered += o.AllOffered
	a.AllSent += o.AllSent
	a.FreshSent += o.FreshSent
	a.Untagged += o.Untagged
	a.AllRunning += o.AllRunning
	a.SeenJudged += o.SeenJudged
	a.SeenMostly += o.SeenMostly
	a.OutletsMax = max(a.OutletsMax, o.OutletsMax)
	if o.OutletsSends > 0 {
		a.OutletsLast = o.OutletsLast
	}
	a.OutletsSends += o.OutletsSends
	for k, v := range o.ReuseDeclined {
		if a.ReuseDeclined == nil {
			a.ReuseDeclined = map[string]int{}
		}
		a.ReuseDeclined[k] += v
	}
	for k, v := range o.NotSent {
		if a.NotSent == nil {
			a.NotSent = map[string]int{}
		}
		a.NotSent[k] += v
	}
}

// takeSearchStats returns what the cell added since the last call, nil when
// nothing. A page an earlier run's search returns late is not counted.
func (s *ChoirScope) takeSearchStats() *SearchStats {
	if s == nil || s.searches == nil {
		return nil
	}
	st := s.searches
	var out SearchStats
	st.mu.Lock()
	if st.counted == nil {
		st.counted = map[string][2]int{}
	}
	marked := st.markedLocked()
	for i, c := range marked {
		if !st.inRun(i) {
			continue
		}
		prev, counted := st.counted[c.ID]
		if !counted {
			out.Started++ // only this run's searches: another run's are its own line's
		}
		returned, fresh := len(c.Results), c.NewPages
		out.PagesReturned += max(returned-prev[0], 0)
		out.PagesNew += max(fresh-prev[1], 0)
		st.counted[c.ID] = [2]int{returned, fresh}
		if c.Done && returned > 0 && !st.judged[c.ID] {
			if st.judged == nil {
				st.judged = map[string]bool{}
			}
			st.judged[c.ID] = true
			out.SeenJudged++
			if c.HeldPages*100 >= seenMostlyPercent*returned {
				out.SeenMostly++
			}
		}
	}
	st.mu.Unlock()
	out.PagesRead = int(s.readStat.Swap(0))
	s.statsMu.Lock()
	out.FromHeld, s.heldStat = s.heldStat, 0
	out.NotSent, s.notSentStat = s.notSentStat, nil
	out.AllCalls, out.AllOffered, out.AllSent, out.FreshSent = s.allStat.calls, s.allStat.offered, s.allStat.sent, s.allStat.freshSent
	out.Untagged, out.AllRunning = s.allStat.untagged, s.allStat.running
	out.ReuseDeclined, s.declineStat = s.declineStat, nil
	out.OutletsMax, out.OutletsLast, out.OutletsSends = s.outletsMax, s.outletsLast, s.outletsSends
	s.outletsMax, s.outletsLast, s.outletsSends = 0, 0, 0
	s.allStat = allCounts{}
	s.statsMu.Unlock()
	if out.IsZero() {
		return nil
	}
	return &out
}

// Reuse windows: how long an earlier search of the same terms still answers
// a query. News of the moment goes stale sooner (searchquery.IsTimeSensitive).
const (
	searchReuseWindow     = 30 * time.Minute
	searchReuseWindowNews = 10 * time.Minute
)

func (st *SearchStore) nowTime() time.Time {
	if st != nil && st.now != nil {
		return st.now()
	}
	return time.Now()
}

// hitMarks is the suffix a summary line carries for a held or read page.
func hitMarks(h SearchHit) string {
	switch {
	case h.Held && h.Read:
		return " [held, read]"
	case h.Held:
		return " [held]"
	case h.Read:
		return " [read]"
	}
	return ""
}

// latestYield is one line on what the latest n searches of this run added:
// "new pages in the latest searches: s4 12, s5 3, s6 0". It is empty before
// the run has n searches.
func (st *SearchStore) latestYield(n int) string {
	st.mu.Lock()
	defer st.mu.Unlock()
	marked := st.markedLocked()
	var mine []Search
	for i, c := range marked {
		if st.inRun(i) {
			mine = append(mine, c)
		}
	}
	if len(mine) < n {
		return ""
	}
	parts := make([]string, 0, n)
	for _, c := range mine[len(mine)-n:] {
		parts = append(parts, fmt.Sprintf("%s %d", c.ID, c.NewPages))
	}
	return "new pages in the latest searches: " + strings.Join(parts, ", ")
}

// searchUsable reports whether a FINISHED search can answer another query: at
// least one provider answered and at least one page came back. A search that
// failed, or found nothing, is tried again. Its only caller requires Done.
func searchUsable(c *Search) bool {
	return c.Answered > 0 && searchHasResults(c)
}

// reuseVerdict finds the finished search of this run that answers query: the
// same terms (searchquery.Key) or the same question (searchquery.NearKeyAt: a
// wording that differs only by a recency decoration: today's or yesterday's
// date, a weekday, "today"), started within the window (10 minutes for news,
// else 30). reason is empty when one was found or when no prior search of the
// session asked the same question; otherwise it is the fixed token for why the
// latest such search was not used: earlier_run, out_of_window, running,
// no_provider_answered, no_results. Near-key matches are reported the same way
// as exact ones.
func (st *SearchStore) reuseVerdict(query string) (c Search, age time.Duration, reason string) {
	key := searchquery.Key(query)
	near, _ := nearOf(query, st.nowTime())
	window := searchReuseWindow
	if searchquery.IsTimeSensitive(query) {
		window = searchReuseWindowNews
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	asks := func(cand *Search) bool {
		return cand.key == key || (near != "" && cand.near == near) || cand.Query == query
	}
	for i := len(st.order) - 1; i >= 0; i-- {
		if !st.inRun(i) {
			continue
		}
		cand := st.searches[st.order[i]]
		d := st.nowTime().Sub(cand.startedAt)
		if !asks(cand) || d > window || !cand.Done || !searchUsable(cand) {
			continue
		}
		return copySearch(cand), d, ""
	}
	// Nothing answers. Say why the latest search that asked the same question
	// did not.
	for i := len(st.order) - 1; i >= 0; i-- {
		cand := st.searches[st.order[i]]
		if !asks(cand) {
			continue
		}
		switch d := st.nowTime().Sub(cand.startedAt); {
		case !st.inRun(i):
			return Search{}, 0, "earlier_run"
		case d > window:
			return Search{}, 0, "out_of_window"
		case !cand.Done:
			return Search{}, 0, "running"
		case cand.Answered == 0:
			return Search{}, 0, "no_provider_answered"
		case !searchHasResults(cand):
			return Search{}, 0, "no_results"
		}
	}
	return Search{}, 0, ""
}

// reuseSearch answers query from a finished search of this run that asked the
// same terms (reuseVerdict), with no host call and no wait. A match still
// running is not answered from here: startSearch's running rule refuses it at
// once (or, once it has run two minutes and is no longer waited for, the
// search is sent). One that failed or came back empty is not reused. The
// answer names the search, its age and SearchFresh, the verb that sends the
// search anyway.
func (s *ChoirScope) reuseSearch(query string) (SearchStub, bool) {
	if s == nil || s.searches == nil {
		return SearchStub{}, false
	}
	c, age, reason := s.searches.reuseVerdict(query)
	if c.ID == "" {
		s.noteDeclined(reason)
		return SearchStub{}, false
	}
	why := fmt.Sprintf("answered from your search %s, %s (same question); send it anyway with choir.SearchFresh(query, why)", c.ID, agoText(age))
	s.statsMu.Lock()
	s.heldStat++
	if !s.reuseQuiet {
		s.notes = append(s.notes, fmt.Sprintf("not sent: %s: answered from your search %s, %s; read it with choir.Searches.Summary(%q), or send it anyway with choir.SearchFresh(query, why)", cutRunes(query, 70), c.ID, agoText(age), c.ID))
	}
	s.statsMu.Unlock()
	return SearchStub{ID: c.ID, Query: query, Providers: len(c.Providers), Cooling: c.Cooling, Reused: why}, true
}

func searchHasResults(c *Search) bool {
	for _, a := range c.Arrivals {
		if len(a.Results) > 0 {
			return true
		}
	}
	return false
}

func agoText(d time.Duration) string {
	switch m := int(d / time.Minute); {
	case m < 1:
		return "under a minute ago"
	case m == 1:
		return "1 minute ago"
	default:
		return fmt.Sprintf("%d minutes ago", m)
	}
}

func cutRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}

// noteNotSent counts a wording the desk did not send, by reason.
func (s *ChoirScope) noteNotSent(reason string) {
	if s == nil {
		return
	}
	s.statsMu.Lock()
	if s.notSentStat == nil {
		s.notSentStat = map[string]int{}
	}
	s.notSentStat[reason]++
	s.statsMu.Unlock()
}

// takeNotes returns and clears the lines for searches not sent since the
// last cell result.
func (s *ChoirScope) takeNotes() []string {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	out := s.notes
	s.notes = nil
	return out
}
