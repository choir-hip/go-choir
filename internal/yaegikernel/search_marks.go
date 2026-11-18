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

// markRunStart makes searches started from now on the first of a new run:
// pages an earlier run returned are new again.
func (st *SearchStore) markRunStart() {
	if st == nil {
		return
	}
	st.mu.Lock()
	st.runStart = len(st.order)
	st.mu.Unlock()
}

// markedLocked returns a copy of every search, oldest first, with Held and
// Read set on each hit and the page counts filled in. Searches of earlier
// runs carry no marks.
func (st *SearchStore) markedLocked() []Search {
	out := make([]Search, 0, len(st.order))
	held := map[string]bool{}
	var read map[string]bool
	if st.readKeys != nil {
		read = st.readKeys()
	}
	for i, id := range st.order {
		c := copySearch(st.searches[id])
		if i >= st.runStart {
			for j := range c.Results {
				key := searchHitKey(c.Results[j].URL)
				if held[key] {
					c.Results[j].Held = true
					c.HeldPages++
				} else {
					c.NewPages++
					held[key] = true
				}
				if read[key] {
					c.Results[j].Read = true
					c.ReadPages++
				}
			}
		}
		out = append(out, c)
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
}

// IsZero reports whether nothing was counted.
func (a SearchStats) IsZero() bool {
	return a.Started == 0 && a.FromHeld == 0 && len(a.NotSent) == 0 && a.PagesReturned == 0 && a.PagesNew == 0 && a.PagesRead == 0
}

// Add sums o into a.
func (a *SearchStats) Add(o SearchStats) {
	a.Started += o.Started
	a.FromHeld += o.FromHeld
	a.PagesReturned += o.PagesReturned
	a.PagesNew += o.PagesNew
	a.PagesRead += o.PagesRead
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
	out.Started = len(marked) - st.startedCounted
	st.startedCounted = len(marked)
	for i, c := range marked {
		if i < st.runStart {
			continue
		}
		prev := st.counted[c.ID]
		returned, fresh := len(c.Results), c.NewPages
		out.PagesReturned += max(returned-prev[0], 0)
		out.PagesNew += max(fresh-prev[1], 0)
		st.counted[c.ID] = [2]int{returned, fresh}
	}
	st.mu.Unlock()
	out.PagesRead = int(s.readStat.Swap(0))
	s.statsMu.Lock()
	out.FromHeld, s.heldStat = s.heldStat, 0
	out.NotSent, s.notSentStat = s.notSentStat, nil
	s.statsMu.Unlock()
	if out.IsZero() {
		return nil
	}
	return &out
}

// searchReuseWindow is how long an earlier search of the same terms still
// answers a query. News older than that is searched for again.
const searchReuseWindow = 30 * time.Minute

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
	if len(marked)-st.runStart < n {
		return ""
	}
	parts := make([]string, 0, n)
	for _, c := range marked[len(marked)-n:] {
		parts = append(parts, fmt.Sprintf("%s %d", c.ID, c.NewPages))
	}
	return "new pages in the latest searches: " + strings.Join(parts, ", ")
}

// reuseSearch answers query from an earlier search of this run with the same
// terms (searchquery.Key), started in the last 30 minutes, that has results
// or is still on its way: no host call is made. A search that finished with
// nothing is not reused, so a failed search is tried again.
func (s *ChoirScope) reuseSearch(query string) (SearchStub, bool) {
	if s == nil || s.searches == nil {
		return SearchStub{}, false
	}
	st := s.searches
	key := searchquery.Key(query)
	st.mu.Lock()
	var found *Search
	var age time.Duration
	for i := len(st.order) - 1; i >= st.runStart; i-- {
		c := st.searches[st.order[i]]
		d := st.nowTime().Sub(c.startedAt)
		if c.key != key || d > searchReuseWindow {
			continue
		}
		if c.Done && !searchHasResults(c) {
			continue
		}
		cp := copySearch(c)
		found, age = &cp, d
		break
	}
	st.mu.Unlock()
	if found == nil {
		return SearchStub{}, false
	}
	why := fmt.Sprintf("answered from your search %s, %s (same terms)", found.ID, agoText(age))
	s.statsMu.Lock()
	s.heldStat++
	{
		s.notes = append(s.notes, fmt.Sprintf("not sent: %s: answered from your search %s, %s; read it with choir.Searches.Summary(%q)", cutRunes(query, 70), found.ID, agoText(age), found.ID))
	}
	s.statsMu.Unlock()
	return SearchStub{ID: found.ID, Query: query, Providers: len(found.Providers), Cooling: found.Cooling, Reused: why}, true
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
