package yaegikernel

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Helpers and the tests that survive the removal of the read-first refusals (owner,
// 2026-11-22: "we shouldnt need rules"; docs/design/semantic-layer-2026-11-20.md section
// 4). No search is refused for pages the run holds untried: see search_gate_test.go
// TestNoSearchIsRefusedForUntriedPages. What stays tested here: a page is one page
// whatever its scheme or www, a hung read is an attempt, choir.Outlet is the function the
// counts use and an aggregator's page is no outlet.

const readEverything = `for _, s := range choir.Searches.List() { for _, h := range s.Results { choir.Read(h.URL) } }
`

func readTop(n int, host string) string {
	return fmt.Sprintf(`for i := 1; i <= %d; i++ { choir.Read(fmt.Sprintf("https://%s.example/p%%d", i)) }
`, n, host)
}

// readTopF is readTop for a source that goes through fmt.Sprintf.
func readTopF(n int, host string) string { return strings.ReplaceAll(readTop(n, host), "%", "%%") }

func TestAReadThatHangsIsAlreadyAnAttempt(t *testing.T) {
	f := newResearchFixture(t)
	f.scope.BindCell().Begin(SessionFrame{RunID: "run-1"})
	const u = "https://hang.example/p1"
	f.hang[u] = true
	go func() { _, _ = f.scope.Read(u) }()
	deadline := time.Now().Add(1500 * time.Millisecond)
	for !f.scope.triedKeys()[searchHitKey(u)] {
		if time.Now().After(deadline) {
			t.Fatal("a Read in flight is not yet an attempt: a read that hangs until the cell dies leaves the page untried")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestOnePageWhateverTheSchemeOrWWW(t *testing.T) {
	if a, b := searchHitKey("http://www.A.example/x/#f"), searchHitKey("https://a.example/x"); a != b {
		t.Fatalf("keys differ: %q %q", a, b)
	}
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"one": {"https://www.a.example/1", "http://a.example/2"}, "two": pagesN("b", 1, 2)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("one")`)
	cell(`choir.Read("https://a.example/1"); choir.Read("https://www.a.example/2")`)
	cell(`choir.Search("two")`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("a page read under another scheme or www form must count as tried")
	}
}

// choir.Outlet: two registrable domains under one public suffix (bbc.co.uk,
// theguardian.co.uk, and the .com.au / .co.jp forms) are two outlets, and
// www.bbc.co.uk and news.bbc.co.uk are one; the function is the one the counts use
// (PageOutlet).

func TestChoirOutletIsTheFunctionTheCountsUse(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`fmt.Println(choir.Outlet("https://www.bbc.co.uk/news/x"), choir.Outlet("https://www.theguardian.co.uk/a"), choir.Outlet("https://x.example.com.au/p"))`)
	if !strings.Contains(out.Stdout, "bbc.co.uk theguardian.co.uk example.com.au") {
		t.Fatalf("choir.Outlet: %q", out.Stdout)
	}
}

// An aggregator's page is no outlet in any count: the run summary's, the decision's
// and choir.Outlet's are one function.
func TestAggregatorPagesAreNoOutletInTheRunsCounts(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"qq": {"https://news.google.com/a", "https://www.msn.com/b", "https://www.bbc.co.uk/c", "https://news.bbc.co.uk/d"}, "two": pagesN("z", 1, 2)}
	cell, _ := searchCell(t, f)
	cell(`choir.Search("qq")`)
	cell(`for _, h := range choir.Searches.List()[0].Results { choir.Read(h.URL) }`)
	out := cell(`choir.SearchStart("two")`)
	if st := out.SearchStats; st == nil || st.OutletsLast != 1 {
		t.Fatalf("outlets held at the send: %+v, want 1 (bbc.co.uk; the two aggregators are none)", st)
	}
	if ns := f.scope.searches.needState(); ns.outlets != 1 {
		t.Fatalf("the decision counts %d outlets read, want 1", ns.outlets)
	}
}
