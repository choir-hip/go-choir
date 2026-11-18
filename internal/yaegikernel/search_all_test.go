package yaegikernel

import (
	"fmt"
	"strings"
	"testing"
)

// Ask once for many (search-reuse commit 3, docs/problems/search-volume-outruns-keenable-credits-2026-11-18.md).
// choir.SearchAll takes every wording of ONE question, drops a wording an
// earlier wording covers, sends the rest one at a time in order, and stops
// when the question stops yielding. Failure list, written before the code:
//  1. A wording that only repeats the terms of an earlier one (or a subset
//     of them) is sent.
//  2. The wordings are sent at once, so no result can inform the next, or
//     all of them are sent although the first ten pages of a wording are
//     already 80% held.
//  3. Two wordings in a row that return nothing do not stop the question.
//  4. A wording the call does not send is not named, with its reason, in the
//     result, so the desk does not know why it has fewer searches than
//     wordings (and cannot send the wording alone).
//  5. The merged list repeats a page, or loses the new/held mark of a page.
//  6. A wording with no word of two or more letters is sent.
//  7. The existing single-search calls change.

func pagesN(host string, from, to int) []string {
	var out []string
	for i := from; i <= to; i++ {
		out = append(out, fmt.Sprintf("https://%s.example/p%d", host, i))
	}
	return out
}

func TestSearchAllStopsWhenTheQuestionStopsYielding(t *testing.T) {
	f := newResearchFixture(t)
	first := pagesN("h", 1, 10)
	f.urlsFor = map[string][]string{
		"harbor strike mauritius union":  first,
		"harbor strike":                  pagesN("never", 1, 10),
		"port workers walkout mauritius": append(append([]string{}, first[:9]...), "https://h.example/p11"),
		"dockers mauritius":              pagesN("never", 1, 10),
	}
	cell, _ := searchCell(t, f)
	out := cell(`r, err := choir.SearchAll("the harbor strike", []string{"harbor strike mauritius union", "Strike harbor", "port workers walkout mauritius", "dockers mauritius"})
fmt.Println(err)
fmt.Println(r)
n, held := 0, 0
for _, h := range r.Results { n++; if h.Held { held++ } }
fmt.Println("merged", n, "held", held)`)
	if f.searchCalls.Load() != 2 {
		t.Fatalf("host asked to start %d searches, want 2 (one covered, one after the stop):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	for _, want := range []string{
		`1 "harbor strike mauritius union": sent s1 (10 new in first 10, 10 of 10 pages new)`,
		`2 "Strike harbor": not sent (covered by wording 1)`,
		`3 "port workers walkout mauritius": sent s2 (1 new in first 10, 1 of 10 pages new)`,
		`4 "dockers mauritius": not sent (the question had stopped yielding after wording 3)`,
		"merged 11 held 0",
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if st := out.SearchStats; st == nil || st.Started != 2 || st.NotSent["covered"] != 1 || st.NotSent["stopped_yielding"] != 1 {
		t.Fatalf("stats = %+v, want 2 started, 1 covered, 1 stopped_yielding", st)
	}
}

func TestSearchAllStopsAfterTwoEmptyWordingsInARow(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"alpha one": pagesN("a", 1, 10), "beta two": {}, "gamma three": {}, "delta four": pagesN("d", 1, 10)}
	cell, _ := searchCell(t, f)
	out := cell(`r, _ := choir.SearchAll("q", []string{"alpha one", "beta two", "gamma three", "delta four"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 3 {
		t.Fatalf("host asked to start %d searches, want 3:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, `4 "delta four": not sent (the question had stopped yielding after wording 3)`) {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

func TestSearchAllKeepsGoingWhileWordingsStillFindPages(t *testing.T) {
	f := newResearchFixture(t)
	first := pagesN("h", 1, 10)
	f.urlsFor = map[string][]string{
		"alpha one":   first,
		"beta two":    append(append([]string{}, first[:6]...), pagesN("b", 1, 4)...), // 60% held
		"gamma three": append(append([]string{}, first[:7]...), pagesN("c", 1, 3)...), // 70% held
	}
	cell, _ := searchCell(t, f)
	out := cell(`r, _ := choir.SearchAll("q", []string{"alpha one", "beta two", "gamma three"})
fmt.Println(r)
for _, h := range r.Results { if h.Held { fmt.Println("held:", h.URL) } }`)
	if f.searchCalls.Load() != 3 {
		t.Fatalf("host asked to start %d searches, want all 3:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, `2 "beta two": sent s2 (4 new in first 10, 4 of 10 pages new)`) {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

func TestSearchAllNamesWhyAWordingWasNotSent(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`choir.SearchStart("harbor strike")
r, _ := choir.SearchAll("q", []string{"x", "strike harbor", "harbor strike mauritius", ""})
fmt.Println(r)`)
	for _, want := range []string{
		`1 "x": not sent (no word of two or more letters or digits)`,
		`2 "strike harbor": not sent (answered from your search s1, under a minute ago)`,
		`3 "harbor strike mauritius": sent s2`,
		`4 "": not sent (empty)`,
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if f.searchCalls.Load() != 2 {
		t.Fatalf("host asked to start %d searches, want 2", f.searchCalls.Load())
	}
}

func TestSearchAllMarksPagesAlreadyHeldBeforeTheCall(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"seed words": pagesN("h", 1, 3), "alpha one": append(pagesN("h", 2, 3), pagesN("n", 1, 2)...)}
	cell, _ := searchCell(t, f)
	out := cell(`choir.SearchStart("seed words")
r, _ := choir.SearchAll("q", []string{"alpha one"})
for _, h := range r.Results { fmt.Println(h.URL, h.Held) }`)
	for _, want := range []string{"https://h.example/p2 true", "https://h.example/p3 true", "https://n.example/p1 false", "https://n.example/p2 false"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
}
