package yaegikernel

import (
	"strings"
	"testing"
	"time"
)

// The desk is told what it holds and does not ask the same thing twice
// (search-reuse commit 2, docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md).
// Failure list, written before the code:
//  1. A page an earlier search of this run already returned is not marked
//     held, or a page the run read is not marked read, so the desk cannot
//     tell a search that found something from one that found nothing new.
//  2. The cell result's count line says "+N new" (arrivals) and nothing
//     about pages, or says "new" for both and the desk reads arrivals as pages.
//  3. A query with the same terms as a search this run made in the last 30
//     minutes goes to the host again (reordered, re-quoted, a year added).
//  4. The reuse reaches back past 30 minutes, or into an earlier run, or
//     answers from a search that failed and returned nothing.
//  5. A query that differs by a real word is answered from another's results.
//  6. The answer to a reused query does not say which search it came from and
//     how old it is, so the desk thinks a new search ran.
//  7. A query with no word of two or more letters ("x") is sent to the web.
//  8. The summary of a thread of searches does not show what the latest
//     few added.

func searchCell(t *testing.T, f *researchFixture) (func(string) SessionResult, CellHooks) {
	t.Helper()
	cell, hooks := f.session(t)
	hooks.Begin(SessionFrame{RunID: "run-1"})
	return cell, hooks
}

func TestPagesAreMarkedNewHeldAndRead(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{
		"first":  {"https://a.example/1", "https://a.example/2", "https://a.example/3"},
		"second": {"https://a.example/2/", "https://A.example/3#top", "https://a.example/4"},
		"third":  {"https://a.example/1", "https://a.example/4"},
	}
	cell, _ := searchCell(t, f)
	out := cell(`a, _ := choir.SearchStart("first")
b, _ := choir.SearchStart("second")
c, _ := choir.SearchStart("third")
choir.Read("https://a.example/2")
s, _ := choir.Searches.Get(b.ID)
fmt.Println(s.NewPages, s.HeldPages, s.ReadPages)
for _, h := range s.Results { fmt.Println(h.URL, h.Held, h.Read) }
fmt.Println(choir.Searches.Summary(c.ID))`)
	for _, want := range []string{
		"1 2 1\n",
		"https://a.example/2/ true true\n",
		"https://A.example/3#top true false\n",
		"https://a.example/4 false false\n",
		`s3 "third": 2 results on 1 hosts, 1/1 providers answered; 0 of 2 pages new`,
		"https://a.example/1 [held]",
		"https://a.example/4 [held]",
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	// The count lines: arrivals and pages are different words, and the
	// latest searches' yield closes the lines.
	lines := strings.Join(out.Searches, "\n")
	for _, want := range []string{
		"s1: 1/1 answered, +1 in, 3 of 3 pages new, done",
		"s2: 1/1 answered, +1 in, 1 of 3 pages new, done",
		"s3: 1/1 answered, +1 in, 0 of 2 pages new, done",
		"new pages in the latest searches: s1 3, s2 1, s3 0",
	} {
		if !strings.Contains(lines, want) {
			t.Fatalf("count lines lack %q:\n%s", want, lines)
		}
	}
	if st := out.SearchStats; st == nil || st.Started != 3 || st.PagesReturned != 8 || st.PagesNew != 4 || st.PagesRead != 1 {
		t.Fatalf("stats = %+v, want 3 started, 8 returned, 4 new, 1 read", st)
	}
}

func TestSearchStartAnswersTheSameTermsFromHeldResults(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`a, _ := choir.SearchStart("Harbor strike Mauritius")
b, _ := choir.SearchStart("\"mauritius\" strike, harbor 2026")
c, _ := choir.SearchStart("harbor strike mauritius union")
fmt.Println(a.ID, b.ID, c.ID)
fmt.Println(b)`)
	if !strings.Contains(out.Stdout, "s1 s1 s2\n") {
		t.Fatalf("ids: the reworded query must come back as s1, the one with another word as s2:\n%s", out.Stdout)
	}
	if !strings.Contains(out.Stdout, "s1 not sent: answered from your search s1, under a minute ago (same terms)") {
		t.Fatalf("the stub does not say it was answered from s1:\n%s", out.Stdout)
	}
	if got := f.searchCalls.Load(); got != 2 {
		t.Fatalf("host asked to start %d searches, want 2", got)
	}
	if !strings.Contains(strings.Join(out.Searches, "\n"), `not sent: "mauritius" strike, harbor 2026: answered from your search s1`) {
		t.Fatalf("the cell result does not say a search was not sent:\n%v", out.Searches)
	}
	if st := out.SearchStats; st == nil || st.FromHeld != 1 || st.Started != 2 {
		t.Fatalf("stats = %+v, want 1 from held, 2 started", st)
	}
}

func TestSearchReuseHoldsThirtyMinutesAndOneRun(t *testing.T) {
	f := newResearchFixture(t)
	clk := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	f.scope.searches.now = func() time.Time { return clk }
	cell, hooks := searchCell(t, f)
	cell(`choir.SearchStart("harbor strike")`)

	clk = clk.Add(29 * time.Minute)
	out := cell(`b, _ := choir.SearchStart("strike harbor"); fmt.Println(b)`)
	if !strings.Contains(out.Stdout, "answered from your search s1, 29 minutes ago") || f.searchCalls.Load() != 1 {
		t.Fatalf("29 minutes on must reuse (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	clk = clk.Add(2 * time.Minute)
	out = cell(`b, _ := choir.SearchStart("strike harbor"); fmt.Println(b.ID)`)
	if strings.TrimSpace(out.Stdout) != "s2" || f.searchCalls.Load() != 2 {
		t.Fatalf("31 minutes on must go to the web (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	// A new run starts clean: the earlier run's s2 is not this run's.
	hooks.Begin(SessionFrame{RunID: "run-2"})
	out = cell(`b, _ := choir.SearchStart("strike harbor"); fmt.Println(b.ID)`)
	if strings.TrimSpace(out.Stdout) != "s3" || f.searchCalls.Load() != 3 {
		t.Fatalf("a new run must search again (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestSearchReuseSkipsASearchThatFoundNothing(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"empty query": {}}
	cell, _ := searchCell(t, f)
	f.urlsFor["empty query"] = []string{} // non-nil: the fixture returns no URLs
	cell(`choir.SearchStart("empty query")`)
	out := cell(`b, _ := choir.SearchStart("query empty"); fmt.Println(b.ID)`)
	if strings.TrimSpace(out.Stdout) != "s2" || f.searchCalls.Load() != 2 {
		t.Fatalf("a search that returned nothing must be retried (calls %d):\n%s", f.searchCalls.Load(), out.Stdout)
	}
}

func TestSearchStartRefusesAQueryWithNoWord(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`_, err := choir.SearchStart("x")
fmt.Println(err)
_, err = choir.SearchStart(" - ? ")
fmt.Println(err)
s, err := choir.SearchStart("x ai")
fmt.Println(s.ID, err)`)
	if strings.Count(out.Stdout, "has no word of two or more letters or digits") != 2 || !strings.Contains(out.Stdout, "s1 <nil>") {
		t.Fatalf("refusals:\n%s", out.Stdout)
	}
	if f.searchCalls.Load() != 1 {
		t.Fatalf("host asked to start %d searches, want only the real one", f.searchCalls.Load())
	}
	if st := out.SearchStats; st == nil || st.NotSent["refused"] != 2 {
		t.Fatalf("stats = %+v, want 2 refused", st)
	}
}
