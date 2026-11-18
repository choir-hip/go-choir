package yaegikernel

import "testing"

// Per-cell search counts for the host's one-line-per-run summary (search-reuse
// commit 1, docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md).
// Failure list, written before the code:
//  1. The cell reports a search twice (started counted again in the next
//     cell) or never (a late arrival after the cell's end is lost).
//  2. A page is "new" although an earlier search of this run returned it
//     (normalised as search_seen.go normalises: fragment, host case,
//     trailing slash), or "held" although only a search of an earlier run
//     returned it.
//  3. pages_read counts a failed read, or the same capture twice.
//  4. Counts are not zero-valued when nothing happened, so the host logs an
//     empty line for every cell.
func TestSearchStatsAreDeltasPerCell(t *testing.T) {
	scope := readLedgerScope(t, "research", map[string]string{"https://bad.example/x": "HTTP 403"})
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{RunID: "run-1"})
	st := scope.searches
	st.apply(SearchFrame{ID: "s1", Kind: SearchFrameStart, Query: "a", Providers: []string{"p"}})
	st.apply(providerFrame("s1", "p", true, "https://x.example/1", "https://x.example/2"))
	st.apply(SearchFrame{ID: "s2", Kind: SearchFrameStart, Query: "b", Providers: []string{"p"}})
	st.apply(providerFrame("s2", "p", true, "https://X.example/2/", "https://x.example/3#frag"))

	_, _ = scope.Read("https://a.example/1")
	_, _ = scope.Read("https://a.example/1") // the same capture again
	_, _ = scope.Read("https://bad.example/x")

	got := hooks.SearchStats()
	want := SearchStats{Started: 2, PagesReturned: 4, PagesNew: 3, PagesRead: 1}
	if got == nil || !statsEqual(*got, want) {
		t.Fatalf("first cell stats = %+v, want %+v", got, want)
	}
	if again := hooks.SearchStats(); again != nil {
		t.Fatalf("a cell with nothing new reported %+v, want nil", again)
	}

	// A late arrival on s2 is counted once, in the next cell.
	st.apply(providerFrame("s2", "q", true, "https://x.example/3", "https://x.example/4"))
	got = hooks.SearchStats()
	want = SearchStats{PagesReturned: 1, PagesNew: 1}
	if got == nil || !statsEqual(*got, want) {
		t.Fatalf("late arrival stats = %+v, want %+v", got, want)
	}

	// A new run: pages an earlier run returned are new again.
	hooks.Begin(SessionFrame{RunID: "run-2"})
	st.apply(SearchFrame{ID: "s3", Kind: SearchFrameStart, Query: "c", Providers: []string{"p"}})
	st.apply(providerFrame("s3", "p", true, "https://x.example/1", "https://x.example/9"))
	got = hooks.SearchStats()
	want = SearchStats{Started: 1, PagesReturned: 2, PagesNew: 2}
	if got == nil || !statsEqual(*got, want) {
		t.Fatalf("second run stats = %+v, want %+v", got, want)
	}
}

func statsEqual(a, b SearchStats) bool {
	if a.Started != b.Started || a.FromHeld != b.FromHeld || a.PagesReturned != b.PagesReturned || a.PagesNew != b.PagesNew || a.PagesRead != b.PagesRead || len(a.NotSent) != len(b.NotSent) {
		return false
	}
	for k, v := range a.NotSent {
		if b.NotSent[k] != v {
			return false
		}
	}
	return true
}
