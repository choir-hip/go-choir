package yaegikernel

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
				if st.isRead != nil && st.isRead(key) {
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
