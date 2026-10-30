package yaegikernel

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// Async search, worker side (Cut 2 unit 2a,
// docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md).
// choir.SearchStart asks the host to start a search and returns at once. The
// host streams each provider's answer to the worker as a StreamSearchArrival
// frame, and the worker's single reader loop (workerMux) writes it into the
// SearchStore: a host call cannot outlive its cell (errCallAfterCell), but a
// frame the reader loop applies can land at any time, during a cell or
// between cells. A running cell reads later providers through
// choir.Searches.Get without waiting for a new cell. The store lives as long
// as the worker's session; a cell's end does not clear it, and the host's
// search table is the authority for what is still streaming (this store is a
// copy). Nothing here wakes a desk: arrivals after the desk's turn ended
// wait in the store, and the next cell's result counts them.

// Search frame kinds.
const (
	SearchFrameStart    = "start"
	SearchFrameProvider = "provider"
	SearchFrameDone     = "done"
)

// SearchFrame is one StreamSearchArrival frame, host to worker.
type SearchFrame struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Start: the query, the providers the search started (N) and the
	// providers it skipped because they were cooling down.
	Query     string   `json:"query,omitempty"`
	Providers []string `json:"providers,omitempty"`
	Cooling   []string `json:"cooling,omitempty"`
	// Provider: one provider's completed call.
	Arrival *SearchArrival `json:"arrival,omitempty"`
	// Done: providers still out when the stream ended, and why it ended
	// early if it did.
	Pending []string `json:"pending,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// SearchArrival is one provider's answer to a search.
type SearchArrival struct {
	Provider  string      `json:"provider"`
	Status    string      `json:"status"`
	LatencyMs int64       `json:"latency_ms"`
	Results   []SearchHit `json:"results,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// Answered reports whether the provider answered, possibly with nothing.
func (a SearchArrival) Answered() bool {
	return a.Status == "success" || a.Status == "success_empty"
}

// SearchHit is one search result.
type SearchHit struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Snippet     string  `json:"snippet,omitempty"`
	PublishedAt string  `json:"published_at,omitempty"`
	Score       float64 `json:"score,omitempty"`
	Provider    string  `json:"provider,omitempty"`
}

// Search is a copy of one search as the worker holds it now.
type Search struct {
	ID    string `json:"id"`
	Query string `json:"query"`
	// Providers are the providers the search started: the N of "M of N".
	Providers []string `json:"providers"`
	// Cooling are the providers it skipped; they are not failures.
	Cooling  []string        `json:"cooling,omitempty"`
	Arrivals []SearchArrival `json:"arrivals"`
	// Answered counts arrivals that answered (an empty answer counts).
	Answered int `json:"answered"`
	// Total is how many providers the search started: Answered of Total.
	Total int `json:"total"`
	// Results are every arrival's hits, each URL once, in arrival order.
	Results []SearchHit `json:"results"`
	Done    bool        `json:"done"`
	Pending []string    `json:"pending,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SearchCount is how far one search has come.
type SearchCount struct {
	ID       string `json:"id"`
	Answered int    `json:"answered"`
	Arrived  int    `json:"arrived"`
	Total    int    `json:"total"`
	Done     bool   `json:"done"`
}

func (c SearchCount) String() string {
	line := fmt.Sprintf("%s: %d/%d answered", c.ID, c.Answered, c.Total)
	if c.Done {
		return line + ", done"
	}
	return line + fmt.Sprintf(", %d pending", c.Total-c.Arrived)
}

// SearchStub is what choir.SearchStart returns: the search's id and the
// providers it started, before any of them has answered.
type SearchStub struct {
	ID        string   `json:"id"`
	Query     string   `json:"query"`
	Providers int      `json:"providers"`
	Cooling   []string `json:"cooling,omitempty"`
}

func (s SearchStub) String() string {
	line := fmt.Sprintf("%s started: %d providers", s.ID, s.Providers)
	if len(s.Cooling) > 0 {
		line += fmt.Sprintf(" (%d cooling: %s)", len(s.Cooling), strings.Join(s.Cooling, ", "))
	}
	return line
}

// SearchStartPayload is the ActionSearchStart request.
type SearchStartPayload struct {
	Query string `json:"query"`
}

// SearchStartResult is the host's answer to ActionSearchStart.
type SearchStartResult struct {
	ID        string   `json:"id"`
	Query     string   `json:"query"`
	Providers []string `json:"providers"`
	Cooling   []string `json:"cooling,omitempty"`
}

// searchWaitMax bounds one choir.SearchWait; the gateway ends a search well
// before it (provider timeout plus a margin).
const searchWaitMax = 2 * time.Minute

// SearchStore holds this session's searches. Only the worker's reader loop
// writes it (apply); cells read copies.
type SearchStore struct {
	mu       sync.Mutex
	searches map[string]*Search
	order    []string
	// changed is closed and replaced on every apply, waking SearchWait.
	changed chan struct{}
}

func newSearchStore() *SearchStore {
	return &SearchStore{searches: map[string]*Search{}, changed: make(chan struct{})}
}

// apply writes one frame. It is called only by the worker's reader loop.
func (st *SearchStore) apply(f SearchFrame) {
	if st == nil || strings.TrimSpace(f.ID) == "" {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	s := st.searches[f.ID]
	if s == nil {
		s = &Search{ID: f.ID}
		st.searches[f.ID] = s
		st.order = append(st.order, f.ID)
	}
	switch f.Kind {
	case SearchFrameStart:
		s.Query = f.Query
		s.Providers = append([]string(nil), f.Providers...)
		s.Cooling = append([]string(nil), f.Cooling...)
	case SearchFrameProvider:
		if f.Arrival != nil {
			// The gateway's budget routing may add a paid provider after
			// the start frame; it joins N so "M of N" keeps M <= N.
			if !slices.Contains(s.Providers, f.Arrival.Provider) {
				s.Providers = append(s.Providers, f.Arrival.Provider)
			}
			s.Arrivals = append(s.Arrivals, *f.Arrival)
			if f.Arrival.Answered() {
				s.Answered++
			}
		}
	case SearchFrameDone:
		s.Done = true
		s.Pending = append([]string(nil), f.Pending...)
		s.Error = f.Error
	}
	close(st.changed)
	st.changed = make(chan struct{})
}

// applyFrame decodes and applies one StreamSearchArrival payload.
func (st *SearchStore) applyFrame(payload []byte) {
	var f SearchFrame
	if json.Unmarshal(payload, &f) == nil {
		st.apply(f)
	}
}

// Get returns a copy of one search.
func (st *SearchStore) Get(id string) (Search, error) {
	if st == nil {
		return Search{}, fmt.Errorf("no searches in this session")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	s := st.searches[strings.TrimSpace(id)]
	if s == nil {
		return Search{}, st.unknownLocked(id)
	}
	return copySearch(s), nil
}

// List returns a copy of every search in this session, oldest first.
func (st *SearchStore) List() []Search {
	if st == nil {
		return []Search{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]Search, 0, len(st.order))
	for _, id := range st.order {
		out = append(out, copySearch(st.searches[id]))
	}
	return out
}

func (st *SearchStore) unknownLocked(id string) error {
	if len(st.order) == 0 {
		return fmt.Errorf("no search %q: this session has started no search", id)
	}
	return fmt.Errorf("no search %q; this session's searches: %s", id, strings.Join(st.order, ", "))
}

// wait blocks until the search is done or d passes, and returns its counts.
func (st *SearchStore) wait(id string, d time.Duration) (SearchCount, error) {
	if st == nil {
		return SearchCount{}, fmt.Errorf("no searches in this session")
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		st.mu.Lock()
		s := st.searches[strings.TrimSpace(id)]
		if s == nil {
			err := st.unknownLocked(id)
			st.mu.Unlock()
			return SearchCount{}, err
		}
		count := countOf(s)
		changed := st.changed
		st.mu.Unlock()
		if count.Done {
			return count, nil
		}
		select {
		case <-changed:
		case <-timer.C:
			return count, nil
		}
	}
}

// counts returns every search's counts, oldest first.
func (st *SearchStore) counts() []SearchCount {
	if st == nil {
		return nil
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]SearchCount, 0, len(st.order))
	for _, id := range st.order {
		out = append(out, countOf(st.searches[id]))
	}
	return out
}

func countOf(s *Search) SearchCount {
	return SearchCount{ID: s.ID, Answered: s.Answered, Arrived: len(s.Arrivals), Total: len(s.Providers), Done: s.Done}
}

func copySearch(s *Search) Search {
	out := *s
	out.Providers = append([]string{}, s.Providers...)
	out.Total = len(out.Providers)
	out.Cooling = append([]string(nil), s.Cooling...)
	out.Pending = append([]string(nil), s.Pending...)
	out.Arrivals = make([]SearchArrival, len(s.Arrivals))
	seen := map[string]bool{}
	out.Results = []SearchHit{}
	for i, a := range s.Arrivals {
		a.Results = append([]SearchHit(nil), a.Results...)
		out.Arrivals[i] = a
		for _, hit := range a.Results {
			key := searchHitKey(hit.URL)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out.Results = append(out.Results, hit)
		}
	}
	return out
}

// searchHitKey is the URL two providers' hits share when they point at the
// same page: no fragment, a lower-case host, no trailing slash.
func searchHitKey(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimSuffix(raw, "/")
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	return strings.TrimSuffix(u.String(), "/")
}

// SearchStart asks the host to start a search on every eligible provider
// and returns as soon as the host has named them; it never waits for an
// answer. Each provider's answer lands in choir.Searches as it arrives.
func (s *ChoirScope) SearchStart(query string) (SearchStub, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchStub{}, fmt.Errorf("choir: SearchStart needs a query")
	}
	var out SearchStartResult
	if err := s.call(ActionSearchStart, SearchStartPayload{Query: query}, &out); err != nil {
		return SearchStub{}, err
	}
	return SearchStub{ID: out.ID, Query: out.Query, Providers: len(out.Providers), Cooling: out.Cooling}, nil
}

// SearchWait blocks until the search is done or ms milliseconds pass, and
// returns its counts. It never waits longer than two minutes.
func (s *ChoirScope) SearchWait(id string, ms int) (SearchCount, error) {
	d := time.Duration(ms) * time.Millisecond
	if d < 0 {
		d = 0
	}
	if d > searchWaitMax {
		d = searchWaitMax
	}
	var store *SearchStore
	if s != nil {
		store = s.searches
	}
	return store.wait(id, d)
}

// searchCountLines returns one line per search with arrivals the desk has
// not been told about, and marks them told: the only push of arrivals to
// the model, and it carries counts, never results.
func (s *ChoirScope) searchCountLines() []string {
	if s == nil || s.searches == nil {
		return nil
	}
	if s.searchTold == nil {
		s.searchTold = map[string]int{}
	}
	var lines []string
	for _, c := range s.searches.counts() {
		fresh := c.Arrived - s.searchTold[c.ID]
		if fresh <= 0 {
			continue
		}
		s.searchTold[c.ID] = c.Arrived
		line := fmt.Sprintf("%s: %d/%d answered, +%d new", c.ID, c.Answered, c.Total, fresh)
		if c.Done {
			line += ", done"
		}
		lines = append(lines, line)
	}
	return lines
}
