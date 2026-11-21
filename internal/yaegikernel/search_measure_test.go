package yaegikernel

import (
	"strings"
	"testing"
	"time"
)

// The next measurement is made from journals (Landing 0 of
// docs/definitions/corpus-first-search-one-verb-2026-11-19.md). Failure list,
// written before the code:
//  1. Nothing says why in-run reuse declined a prior identical search (256 of
//     265 exact repeats are unexplained): no fixed token, or a token for a
//     search that was never identical, or a decline of a real reuse.
//  2. Nothing says how many of a run's searches came back mostly already held.
//  3. Nothing says how many distinct outlets the run held at its sends.
//  4. A count carries a query or a URL.
func TestReuseDeclineReasonsAreFixedTokens(t *testing.T) {
	f := newResearchFixture(t)
	clk := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	f.scope.searches.now = func() time.Time { return clk }
	f.urlsFor = map[string][]string{"empty query": {}, "harbor strike": pagesN("h", 1, 3)}
	cell, hooks := searchCell(t, f)
	// Nothing prior: no token (a first ask is not a decline).
	out := cell(`choir.SearchStart("harbor strike")`)
	if st := out.SearchStats; st != nil && len(st.ReuseDeclined) != 0 {
		t.Fatalf("a first ask is no decline: %+v", st.ReuseDeclined)
	}
	cell(readEverything)
	// A search that returned nothing.
	cell(`choir.SearchStart("empty query")`)
	out = cell(`choir.SearchStart("empty query")`)
	if st := out.SearchStats; st == nil || st.ReuseDeclined["no_results"] != 1 {
		t.Fatalf("stats = %+v, want no_results=1", st)
	}
	// Out of the window.
	clk = clk.Add(31 * time.Minute)
	out = cell(`choir.SearchStart("harbor strike")`)
	if st := out.SearchStats; st == nil || st.ReuseDeclined["out_of_window"] != 1 {
		t.Fatalf("stats = %+v, want out_of_window=1", st)
	}
	// An earlier run's search.
	hooks.Begin(SessionFrame{RunID: "run-2"})
	out = cell(`choir.SearchStart("harbor strike")`)
	if st := out.SearchStats; st == nil || st.ReuseDeclined["earlier_run"] != 1 {
		t.Fatalf("stats = %+v, want earlier_run=1", st)
	}
	// fresh over a usable match is counted as fresh, and is no decline of reuse.
	out = cell(`choir.SearchFresh("harbor strike", "the earlier answer is out of date")`)
	if st := out.SearchStats; st == nil || st.ReuseDeclined["fresh"] != 1 {
		t.Fatalf("stats = %+v, want fresh=1", st)
	}
	for tok := range out.SearchStats.ReuseDeclined {
		if strings.ContainsAny(tok, " /:.") {
			t.Fatalf("token %q is not a fixed token", tok)
		}
	}
}

func TestSeenShareIsCountedPerSearch(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{RunID: "run-1"})
	st := scope.searches
	finish := func(id string, urls []string) {
		st.apply(SearchFrame{ID: id, Kind: SearchFrameStart, Query: id, Providers: []string{"p"}})
		st.apply(providerFrame(id, "p", true, urls...))
		st.apply(SearchFrame{ID: id, Kind: SearchFrameDone})
	}
	finish("s1", pagesN("a", 1, 10))                                                               // 0% held
	finish("s2", pagesN("a", 1, 9))                                                                // 90% of its 9 pages held: mostly seen
	finish("s3", append(pagesN("a", 1, 3), pagesN("b", 1, 7)...))                                  // 30% held
	st.apply(SearchFrame{ID: "s4", Kind: SearchFrameStart, Query: "s4", Providers: []string{"p"}}) // running: not judged
	got := hooks.SearchStats()
	if got == nil || got.SeenJudged != 3 || got.SeenMostly != 1 {
		t.Fatalf("stats = %+v, want 3 judged, 1 mostly seen", got)
	}
	if again := hooks.SearchStats(); again != nil && (again.SeenJudged != 0 || again.SeenMostly != 0) {
		t.Fatalf("a search is judged once: %+v", again)
	}
}

func TestOutletsHeldAreCountedAtTheSend(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{
		"one": {"https://www.reuters.com/a", "https://news.reuters.com/b", "https://bbc.co.uk/c", "https://www.bbc.co.uk/d", "https://example.org/e"},
		"two": {"https://apnews.com/f"},
	}
	cell, _ := searchCell(t, f)
	out := cell(`choir.SearchStart("one")`)
	// At the first send the run holds nothing.
	if st := out.SearchStats; st == nil || st.OutletsLast != 0 {
		t.Fatalf("first send: %+v", st)
	}
	cell(readEverything)
	out = cell(`choir.SearchStart("two")`)
	// reuters.com, bbc.co.uk, example.org: three outlets.
	if st := out.SearchStats; st == nil || st.OutletsLast != 3 || st.OutletsMax != 3 {
		t.Fatalf("second send: %+v, want 3 outlets (reuters.com, bbc.co.uk, example.org)", st)
	}
}

func TestOutletOfIsTheRegistrableDomain(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.reuters.com/a":     "reuters.com",
		"https://news.bbc.co.uk/x":      "bbc.co.uk",
		"https://a.b.example.org/x?y=1": "example.org",
		"https://localhost/x":           "localhost",
		"https://1.2.3.4/x":             "1.2.3.4",
		"https://data.gov.uk/x":         "data.gov.uk",
		"https://www.mod.gov.ua/news":   "mod.gov.ua",
		"https://www.theguardian.co.uk": "theguardian.co.uk",
		"https://www.abc.net.au/x":      "abc.net.au",
		"https://x.asahi.co.jp/x":       "asahi.co.jp",
		"https://news.example.com.au/x": "example.com.au",
		"https://www.reuters.com.":      "reuters.com",
	} {
		if got := outletOf(in); got != want {
			t.Errorf("outletOf(%q) = %q, want %q", in, got, want)
		}
	}
}
