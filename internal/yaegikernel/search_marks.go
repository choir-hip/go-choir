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
	st.first = map[string]int{}
	st.mu.Unlock()
}

// noteKeysLocked records that search idx returned these pages, for the held
// test. A search of an earlier run records nothing.
func (st *SearchStore) noteKeysLocked(idx int, hits []SearchHit) {
	if idx < st.runStart {
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
	if i < st.runStart {
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
	// FreshOverUnread counts searches sent with fresh although the run held
	// 15 or more unread pages: the override, used.
	FreshOverUnread int `json:"fresh_over_unread,omitempty"`
	// Untagged counts searches SearchAll sent that its gate could not tag.
	Untagged int `json:"untagged,omitempty"`
	// AllRunning counts wordings SearchAll did not send because another search
	// of the run was still running. They are not in AllOffered: the call could
	// not have sent them, so the sent share of AllOffered stays honest.
	AllRunning int `json:"all_running,omitempty"`
}

// IsZero reports whether nothing was counted.
func (a SearchStats) IsZero() bool {
	return a.Started == 0 && a.FromHeld == 0 && len(a.NotSent) == 0 && a.PagesReturned == 0 && a.PagesNew == 0 && a.PagesRead == 0 &&
		a.AllCalls == 0 && a.AllOffered == 0 && a.AllSent == 0 && a.FreshOverUnread == 0 && a.Untagged == 0 && a.AllRunning == 0
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
	a.FreshOverUnread += o.FreshOverUnread
	a.Untagged += o.Untagged
	a.AllRunning += o.AllRunning
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
	out.AllCalls, out.AllOffered, out.AllSent, out.FreshOverUnread = s.allStat.calls, s.allStat.offered, s.allStat.sent, s.allStat.freshOverUnread
	out.Untagged, out.AllRunning = s.allStat.untagged, s.allStat.running
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
	if len(marked)-st.runStart < n {
		return ""
	}
	parts := make([]string, 0, n)
	for _, c := range marked[len(marked)-n:] {
		parts = append(parts, fmt.Sprintf("%s %d", c.ID, c.NewPages))
	}
	return "new pages in the latest searches: " + strings.Join(parts, ", ")
}

// searchUsable reports whether a search can answer another query: it is
// still running, or it ended with at least one provider answering and at
// least one page. A search that failed, or found nothing, is tried again.
func searchUsable(c *Search) bool {
	if !c.Done {
		return true
	}
	return c.Answered > 0 && searchHasResults(c)
}

// reuseSearch answers query from a finished search of this run with the same
// terms (searchquery.Key), started within the window (10 minutes for news,
// else 30), with no host call and no wait. A match still running is not
// answered from here: startSearch's running rule refuses it at once (or, once
// it has run two minutes and is no longer waited for, the search is sent). One
// that failed or came back empty is not reused. The answer names the search,
// its age and "fresh", the argument that sends the search anyway.
func (s *ChoirScope) reuseSearch(query string) (SearchStub, bool) {
	if s == nil || s.searches == nil {
		return SearchStub{}, false
	}
	st := s.searches
	key := searchquery.Key(query)
	window := searchReuseWindow
	if searchquery.IsTimeSensitive(query) {
		window = searchReuseWindowNews
	}
	st.mu.Lock()
	var c Search
	var age time.Duration
	found := false
	for i := len(st.order) - 1; i >= st.runStart; i-- {
		cand := st.searches[st.order[i]]
		d := st.nowTime().Sub(cand.startedAt)
		if cand.key != key || d > window || !cand.Done || !searchUsable(cand) {
			continue
		}
		c, age, found = copySearch(cand), d, true
		break
	}
	st.mu.Unlock()
	if !found {
		return SearchStub{}, false
	}
	why := fmt.Sprintf("answered from your search %s, %s (same terms); send it anyway with fresh", c.ID, agoText(age))
	s.statsMu.Lock()
	s.heldStat++
	if !s.reuseQuiet {
		s.notes = append(s.notes, fmt.Sprintf("not sent: %s: answered from your search %s, %s; read it with choir.Searches.Summary(%q), or send it anyway with fresh: choir.SearchStart(query, true)", cutRunes(query, 70), c.ID, agoText(age), c.ID))
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
