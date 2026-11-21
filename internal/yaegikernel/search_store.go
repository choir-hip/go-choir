package yaegikernel

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/searchquery"
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
	// Held and Read are set on a copy the desk reads, never on the wire.
	// Held: an earlier search of this run already returned this page.
	// Read: this run captured it with choir.Read.
	Held bool `json:"held,omitempty"`
	Read bool `json:"read,omitempty"`
	// CaptureID is set on a page the paper already held for the run (the corpus
	// brief): read it with choir.ReadCapture(CaptureID, 0, 0), not a web fetch.
	CaptureID string `json:"capture_id,omitempty"`
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
	// NewPages, HeldPages and ReadPages count Results: pages no earlier
	// search of this run returned, pages one did, and pages this run read.
	NewPages  int      `json:"new_pages,omitempty"`
	HeldPages int      `json:"held_pages,omitempty"`
	ReadPages int      `json:"read_pages,omitempty"`
	Done      bool     `json:"done"`
	Pending   []string `json:"pending,omitempty"`
	Error     string   `json:"error,omitempty"`
	// Refused is why no search was sent; Results are then the unread pages.
	Refused string `json:"refused,omitempty"`
	// Running is set with Refused when another search of this run is still
	// running: ID is then that search's, not an answer to this query.
	Running bool `json:"running,omitempty"`
	// Reused says this query was not sent: it asks the same question as an
	// earlier search of the run, and this value IS that search (ID, Query and
	// Results are its, not an answer to a new query). ReusedFrom is its id.
	Reused     string `json:"reused,omitempty"`
	ReusedFrom string `json:"reused_from,omitempty"`

	// startedAt and key let SearchStart answer a later query with the same
	// terms from this search (search_marks.go reuseSearch).
	startedAt time.Time
	key       string
	// question is the key of the SearchAll question that sent this search,
	// empty for a plain search (search_need.go).
	question string
	// run is the run the session was serving when the search's FIRST frame
	// arrived (usually its start frame): a frame that lands after another run
	// bound is attributed to that run.
	run string
	// near is searchquery.NearKey of the query (the question once date words,
	// operators, quotes and order are set aside), nearTerms its terms; both
	// are empty when nothing is left (search_hold.go).
	near      string
	nearTerms []string
}

// SearchCount is how far one search has come.
type SearchCount struct {
	ID       string `json:"id"`
	Answered int    `json:"answered"`
	Arrived  int    `json:"arrived"`
	Total    int    `json:"total"`
	Done     bool   `json:"done"`
	// Pages are the search's distinct result pages; NewPages the ones no
	// earlier search of this run returned.
	Pages    int `json:"pages,omitempty"`
	NewPages int `json:"new_pages,omitempty"`
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
	// Reused says why no search was sent: the query has the terms of an
	// earlier search of this run, and ID is that search.
	Reused string `json:"reused,omitempty"`
	// Refused says why no search was sent.
	Refused string `json:"refused,omitempty"`
	// Running is set when the refusal is that another search of this run is
	// still running; ID is then that search's id, which SearchWait takes.
	Running bool        `json:"running,omitempty"`
	Unread  []SearchHit `json:"unread,omitempty"`
	// Reason is the fixed token the refusal is counted by (read_first,
	// held_first, site_held, mostly_seen, search_running, ...); empty when sent.
	Reason string `json:"reason,omitempty"`
}

func (s SearchStub) String() string {
	if s.Refused != "" {
		var b strings.Builder
		fmt.Fprintf(&b, "not sent: %s\n", s.Refused)
		if s.Reason == reasonReadFirst || s.Reason == reasonHeldFirst || s.Reason == reasonSiteHeld {
			// The refusal names the pages itself, before its last line.
			return b.String()
		}
		writeHostGroups(&b, groupByHost(s.Unread), func(n int) string {
			return fmt.Sprintf("... %d more hosts; they are in the Unread of this value\n", n)
		})
		return b.String()
	}
	if s.Reused != "" {
		return fmt.Sprintf("%s not sent: %s", s.ID, s.Reused)
	}
	line := fmt.Sprintf("%s started: %d providers", s.ID, s.Providers)
	if len(s.Cooling) > 0 {
		line += fmt.Sprintf(" (%d cooling: %s)", len(s.Cooling), strings.Join(s.Cooling, ", "))
	}
	return line
}

// SearchStartPayload is the ActionSearchStart request.
type SearchStartPayload struct {
	Query string `json:"query"`
	// The need state, reported with each search the guest sends so the host can
	// write what the run held and had read when it asked (search.decision):
	// the verb; searches earlier in the run; pages returned and tried (new to
	// the run); pages read and outlets read; the brief's held pages and how
	// many of them were tried; and, for SearchFresh, the reason given.
	Verb      string `json:"verb,omitempty"`
	Earlier   int    `json:"earlier,omitempty"`
	Returned  int    `json:"returned,omitempty"`
	Tried     int    `json:"tried,omitempty"`
	Read      int    `json:"read,omitempty"`
	Outlets   int    `json:"outlets,omitempty"`
	HeldBrief int    `json:"held_brief,omitempty"`
	HeldTried int    `json:"held_tried,omitempty"`
	FreshWhy  string `json:"fresh_why,omitempty"`
	// Probe asks for the research tree as it stands now and starts nothing: the
	// guest sends one before it refuses a search, so a page a sibling session
	// tried since the cell began is not demanded again.
	Probe bool `json:"probe,omitempty"`
}

// SearchStartResult is the host's answer to ActionSearchStart.
type SearchStartResult struct {
	ID        string   `json:"id"`
	Query     string   `json:"query"`
	Providers []string `json:"providers"`
	Cooling   []string `json:"cooling,omitempty"`
	// Tree answers a Probe: the pages any session of the research tree has tried, now.
	Tree *TreeState `json:"tree,omitempty"`
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
	// run is the id of the run the session is serving now: held is judged
	// against that run's searches only (markRunStart). A desk session serves
	// several runs in turn, so a search carries the run it was started under
	// and a run that returns finds its own searches again.
	run string
	// now is the clock (nil: time.Now); a test sets it.
	now func() time.Time
	// readKeys returns the searchHitKey of every page this run read; nil
	// means the store knows no read ledger.
	readKeys func() map[string]bool
	// triedKeys returns the searchHitKey of every page this run tried to read,
	// whether or not the read worked (search_need.go).
	triedKeys func() map[string]bool
	// pos is a search's index in order; first is, for each page key a search
	// of this run returned, the lowest index that returned it: a page is held
	// in search i when first[key] < i. Both make one search's marks cost its
	// own results, not the store's (search_marks.go).
	pos   map[string]int
	first map[string]int
	// counted is what the host has been told (takeStats).
	counted map[string][2]int
	// judged is the searches whose share of already-held pages was counted.
	judged map[string]bool
	// heldRun is, per run, the pages the paper held for it when it began: search
	// zero (seedHeld).
	heldRun map[string][]HeldPage
	// tree and treeTried are the host's view of the research tree this cell
	// belongs to (setTree).
	tree      TreeState
	treeTried map[string]bool
}

func newSearchStore() *SearchStore {
	return &SearchStore{searches: map[string]*Search{}, changed: make(chan struct{}), pos: map[string]int{}, first: map[string]int{}}
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
		s = &Search{ID: f.ID, startedAt: st.nowTime(), run: st.run}
		st.searches[f.ID] = s
		st.pos[f.ID] = len(st.order)
		st.order = append(st.order, f.ID)
	}
	switch f.Kind {
	case SearchFrameStart:
		s.Query = f.Query
		s.key = searchquery.Key(f.Query)
		s.near, s.nearTerms = nearOf(f.Query, st.nowTime())
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
			st.noteKeysLocked(st.pos[f.ID], f.Arrival.Results)
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
	id = strings.TrimSpace(id)
	if st.searches[id] == nil {
		return Search{}, st.unknownLocked(id)
	}
	return st.markOneLocked(st.pos[id], st.readSet()), nil
}

// List returns a copy of every search in this session, oldest first.
func (st *SearchStore) List() []Search {
	if st == nil {
		return []Search{}
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.markedLocked()
}

// summaryMaxBytes keeps a Summary under the cell output cut (8000 runes).
const summaryMaxBytes = 5000

// Summary prints one search's results deduped by host: a count per host and
// its first two hits (title and URL), hosts in first-seen order. It stays
// under summaryMaxBytes, so a 40-result search is read whole rather than cut
// in the middle (problems/ordinary-prompt-path-2026-11-09.md section 1.2 item 3).
func (st *SearchStore) Summary(id string) (string, error) {
	s, err := st.Get(id)
	if err != nil {
		return "", err
	}
	groups := groupByHost(s.Results)
	var b strings.Builder
	fmt.Fprintf(&b, "%s %q: %d results on %d hosts, %d/%d providers answered; %d of %d pages new\n", s.ID, s.Query, len(s.Results), len(groups), s.Answered, s.Total, s.NewPages, len(s.Results))
	writeHostGroups(&b, groups, func(n int) string {
		return fmt.Sprintf("... %d more hosts; read choir.Searches.Get(%q).Results for them\n", n, s.ID)
	})
	return b.String(), nil
}

type hostGroup struct {
	host string
	hits []SearchHit
}

// groupByHost groups hits by host, hosts in first-seen order.
func groupByHost(hits []SearchHit) []*hostGroup {
	var groups []*hostGroup
	byHost := map[string]*hostGroup{}
	for _, hit := range hits {
		host := hostOf(hit.URL)
		g := byHost[host]
		if g == nil {
			g = &hostGroup{host: host}
			byHost[host] = g
			groups = append(groups, g)
		}
		g.hits = append(g.hits, hit)
	}
	return groups
}

// writeHostGroups prints each host with its count and first two hits, until
// b would pass summaryMaxBytes; more says what the cut left out.
func writeHostGroups(b *strings.Builder, groups []*hostGroup, more func(n int) string) {
	for i, g := range groups {
		var line strings.Builder
		fmt.Fprintf(&line, "%s (%d)", g.host, len(g.hits))
		// The first two hits shown are pages the desk does not hold yet.
		sort.SliceStable(g.hits, func(a, b int) bool { return !g.hits[a].Held && g.hits[b].Held })
		for j, hit := range g.hits {
			if j == 2 {
				break
			}
			title := strings.Join(strings.Fields(hit.Title), " ")
			if r := []rune(title); len(r) > 80 {
				title = string(r[:80]) + "..."
			}
			fmt.Fprintf(&line, "\n  %s | %s%s", title, hit.URL, hitMarks(hit))
		}
		line.WriteString("\n")
		if b.Len()+line.Len() > summaryMaxBytes {
			b.WriteString(more(len(groups) - i))
			break
		}
		b.WriteString(line.String())
	}
}

func hostOf(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return strings.TrimSpace(raw)
	}
	return strings.TrimPrefix(strings.ToLower(u.Host), "www.")
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
		count := countOfMarked(st.markOneLocked(st.pos[s.ID], st.readSet()))
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
	for _, c := range st.markedLocked() {
		out = append(out, countOfMarked(c))
	}
	return out
}

func countOfMarked(c Search) SearchCount {
	return SearchCount{ID: c.ID, Answered: c.Answered, Arrived: len(c.Arrivals), Total: len(c.Providers), Done: c.Done, Pages: len(c.Results), NewPages: c.NewPages}
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
// same page: no fragment, a lower-case host without www., https for http, no
// trailing slash.
func searchHitKey(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimSuffix(raw, "/")
	}
	u.Fragment = ""
	// One page whatever the scheme or a leading www.: http and https, www. and
	// the bare host are the same page for the read-first rule.
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	u.Host = strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	return strings.TrimSuffix(u.String(), "/")
}

// SearchStart asks the host to start a search on every eligible provider
// and returns as soon as the host has named them; it never waits for an
// answer. Each provider's answer lands in choir.Searches as it arrives.
// Every web search passes the need test (search_need.go): it is not sent while
// the run has untried pages from the top of its earlier searches or from the
// pages the paper held for it; the stub says which and carries them. A second
// argument true is refused: the one way past is SearchFresh with a reason.
func (s *ChoirScope) SearchStart(query string, fresh ...bool) (SearchStub, error) {
	if len(fresh) > 0 && fresh[0] {
		return s.freshArgRefused(query), nil
	}
	return s.startSearch(query, searchOpts{verb: "search_start"})
}

// freshArgRefused answers the old fresh argument: it is not a way past, and
// the stub says what is.
func (s *ChoirScope) freshArgRefused(query string) SearchStub {
	s.noteNotSent("fresh_arg")
	why := fmt.Sprintf("the second argument true is no longer a way past the need test: choir.SearchFresh(query, why) sends a search with a reason of at least %d characters, which is recorded", freshWhyMin)
	return SearchStub{Query: strings.TrimSpace(query), Refused: why, Reason: "fresh_arg"}
}

// searchOpts says how a search reaches startSearch: the verb (for the host's
// record), SearchAll's question key, and SearchFresh's reason.
type searchOpts struct {
	verb     string
	qk       string
	freshWhy string
	fresh    bool
}

// startSearch is the one path a web search takes to the host. One search at a
// time passes the need test and starts (sendMu): the next is judged only when
// none of the run's searches is still running (it is refused at once
// otherwise, never waited for: a cell cannot time out on it).
func (s *ChoirScope) startSearch(query string, o searchOpts) (SearchStub, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchStub{}, fmt.Errorf("choir: SearchStart needs a query")
	}
	if !searchquery.HasRealWord(query) {
		s.noteNotSent("refused")
		return SearchStub{}, fmt.Errorf("choir: SearchStart refused: %q has no word of two or more letters or digits, so it is not a question; send the words of the question", query)
	}
	override := o.fresh
	if override && len([]rune(strings.TrimSpace(o.freshWhy))) < freshWhyMin {
		s.noteNotSent("fresh_why")
		return SearchStub{Query: query, Refused: fmt.Sprintf("SearchFresh needs the reason as its second argument, at least %d characters: why the pages you hold cannot answer what you lack. It is recorded", freshWhyMin), Reason: "fresh_why"}, nil
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	refuse := func(reason, why string, pages []SearchHit) (SearchStub, error) {
		s.noteNotSent(reason)
		s.statsMu.Lock()
		if !s.reuseQuiet {
			s.notes = append(s.notes, fmt.Sprintf("not sent: %s: %s", cutRunes(query, 70), why))
		}
		s.statsMu.Unlock()
		return SearchStub{Query: query, Refused: why, Unread: pages, Reason: reason}, nil
	}
	// A finished search with the same terms answers first, whatever else
	// runs: it costs nothing and never sends. Then the running rule: a search
	// of this run still running is refused at once, not waited on for up to
	// 15 s, and a same-terms match still running is such a search. SearchAll
	// applies both to each wording too.
	if !override {
		if stub, ok := s.reuseSearch(query); ok {
			return stub, nil
		}
		if id := s.searches.runningSearch(); id != "" {
			s.noteNotSent("search_running")
			why := runningRefusal(id)
			s.statsMu.Lock()
			if !s.reuseQuiet {
				s.notes = append(s.notes, fmt.Sprintf("not sent: %s: %s", cutRunes(query, 70), why))
			}
			s.statsMu.Unlock()
			// ID is the running search's, so SearchWait(stub.ID, ...) works.
			return SearchStub{ID: id, Query: query, Refused: why, Running: true, Reason: "search_running"}, nil
		}
		// The host is asked for the live research tree once, when the guest
		// would refuse for pages: a sibling may have tried them since this cell
		// began (search_need.go refreshTree).
		refreshed := false
		refresh := func() bool {
			if refreshed {
				return false
			}
			refreshed = true
			return s.refreshTree()
		}
		// A hunt for an outlet the run already holds untried pages of.
		for _, host := range searchquery.SiteRestrictions(query) {
			pages := s.searches.untriedOfOutlet(siteMatcher(host))
			if len(pages) > 0 && refresh() {
				pages = s.searches.untriedOfOutlet(siteMatcher(host))
			}
			if len(pages) > 0 {
				return refuse(reasonSiteHeld, siteHeldRefusal(host, pages), pages)
			}
		}
		// The web has shown what it has for this question (search_hold.go).
		pages, held := s.searches.mostlySeen(query, o.qk, s.searches.nowTime())
		if held && refresh() {
			pages, held = s.searches.mostlySeen(query, o.qk, s.searches.nowTime())
		}
		if held {
			return refuse("mostly_seen", mostlySeenRefusal(len(pages)), pages)
		}
		// Read what the run holds first: the paper's held pages, then the top
		// pages of every earlier search.
		need := s.searches.readNeed()
		if len(need.pages) > 0 && refresh() {
			need = s.searches.readNeed()
		}
		if len(need.pages) > 0 {
			reason := reasonReadFirst
			if need.held {
				reason = reasonHeldFirst
			}
			return refuse(reason, readFirstRefusal(need), need.pages)
		}
	} else {
		s.noteFreshOverMatch(query)
	}
	outlets := s.searches.outletsHeld() // before the send: what the run held when it asked
	ns := s.searches.needState()
	var out SearchStartResult
	payload := SearchStartPayload{Query: query, Verb: o.verb, Earlier: ns.earlier, Returned: ns.returned, Tried: ns.tried, Read: ns.read, Outlets: ns.outlets, HeldBrief: ns.heldBrief, HeldTried: ns.heldTried}
	if override {
		payload.FreshWhy = strings.TrimSpace(o.freshWhy)
	}
	if err := s.call(ActionSearchStart, payload, &out); err != nil {
		return SearchStub{}, err
	}
	s.noteOutlets(outlets)
	if override {
		s.statsMu.Lock()
		s.allStat.freshSent++
		s.statsMu.Unlock()
	}
	// The next search is judged against this one: wait for its start frame.
	s.searches.knows(out.ID, 2*time.Second)
	return SearchStub{ID: out.ID, Query: out.Query, Providers: len(out.Providers), Cooling: out.Cooling}, nil
}

// SearchWait blocks until the search is done or ms milliseconds pass, and
// returns its counts. It never waits longer than two minutes.
func (s *ChoirScope) SearchWait(id string, ms int) (SearchCount, error) {
	if strings.TrimSpace(id) == "" {
		return SearchCount{}, fmt.Errorf("choir: SearchWait needs the id of a search that was sent: this search was not sent. A search refused because the run holds unread pages has no id (read the pages in its stub); one refused because another search of this run is still running carries that running search's id in ID")
	}
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
// the model, and it carries counts, never results. A line says how many
// providers answered, how many arrivals are new to the desk ("+2 in"), and
// how many of the search's pages no earlier search of this run returned
// ("12 of 40 pages new"). When any line is shown, one more says what the
// latest searches added.
func (s *ChoirScope) searchCountLines() []string {
	if s == nil || s.searches == nil {
		return nil
	}
	if s.searchTold == nil {
		s.searchTold = map[string]int{}
	}
	var lines []string
	counts := s.searches.counts()
	for _, c := range counts {
		fresh := c.Arrived - s.searchTold[c.ID]
		if fresh <= 0 {
			continue
		}
		s.searchTold[c.ID] = c.Arrived
		line := fmt.Sprintf("%s: %d/%d answered, +%d in", c.ID, c.Answered, c.Total, fresh)
		if c.Pages > 0 || c.Done {
			line += fmt.Sprintf(", %d of %d pages new", c.NewPages, c.Pages)
		}
		if c.Done {
			line += ", done"
		}
		lines = append(lines, line)
	}
	if len(lines) > 0 {
		if thread := s.searches.latestYield(3); thread != "" {
			lines = append(lines, thread)
		}
	}
	lines = append(lines, s.takeNotes()...)
	return lines
}
