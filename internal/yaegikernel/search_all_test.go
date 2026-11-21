package yaegikernel

import (
	"fmt"
	"strings"
	"testing"
	"time"
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
// From the review of f7df119fb:
//  8. A wording where no provider answered, or where the host refused, is
//     called an empty result ("stopped yielding") instead of a failure; or
//     two failures in a row do not end the call with the failure's reason and
//     the remaining wordings named as not sent because of it.
//  9. A broader wording is skipped because an earlier one has more terms.
//  10. A wording answered from a held search adds nothing to the merged list
//     and the line does not say how many pages it holds.
//  11. The call has no total deadline, and past it the remaining wordings
//     are not all named.
//  12. After the stop, a wording left out cannot be sent (fresh).

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
		"harbor strike":                  pagesN("b", 1, 10), // broader: sent, not skipped
		"port workers walkout mauritius": append(append([]string{}, pagesN("b", 1, 9)...), "https://h.example/p11"),
		"dockers mauritius":              pagesN("never", 1, 10),
	}
	cell, _ := searchCell(t, f)
	// The run has read the pages these wordings return, so the question lacks
	// none (search_need.go) and only the novelty stop is under test.
	out := cell(`for i := 1; i <= 11; i++ { choir.Read(fmt.Sprintf("https://h.example/p%d", i)) }
for i := 1; i <= 10; i++ { choir.Read(fmt.Sprintf("https://b.example/p%d", i)) }
r, err := choir.SearchAll("the harbor strike", []string{"harbor strike mauritius union", "harbor strike", "port workers walkout mauritius", "dockers mauritius"})
fmt.Println(err)
fmt.Println(r)
n, held := 0, 0
for _, h := range r.Results { n++; if h.Held { held++ } }
fmt.Println("merged", n, "held", held)`)
	if f.searchCalls.Load() != 3 {
		t.Fatalf("host asked to start %d searches, want 3 (the broader wording is sent; the last is after the stop):\n%s", f.searchCalls.Load(), out.Stdout)
	}
	for _, want := range []string{
		`1 "harbor strike mauritius union": sent s1 (10 new in first 10, 10 of 10 pages new)`,
		`2 "harbor strike": sent s2 (10 new in first 10, 10 of 10 pages new)`,
		`3 "port workers walkout mauritius": sent s3 (1 new in first 10, 1 of 10 pages new)`,
		`4 "dockers mauritius": not sent (the question had stopped yielding after wording 3)`,
		"merged 21 held 0",
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if st := out.SearchStats; st == nil || st.Started != 3 || st.NotSent["stopped_yielding"] != 1 || st.NotSent["covered"] != 0 {
		t.Fatalf("stats = %+v, want 3 started and 1 stopped_yielding", st)
	}
}

func TestSearchAllStopsAfterTwoEmptyWordingsInARow(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"alpha one": pagesN("a", 1, 10), "beta two": {}, "gamma three": {}, "delta four": pagesN("d", 1, 10)}
	cell, _ := searchCell(t, f)
	out := cell(readTop(10, "a") + `
r, _ := choir.SearchAll("q", []string{"alpha one", "beta two", "gamma three", "delta four"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 3 {
		t.Fatalf("host asked to start %d searches, want 3:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, `4 "delta four": not sent (the question had stopped yielding after wording 3)`) {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

// An error is not an empty result: a failed wording between an empty one and a
// good one does not make "two empties in a row".
func TestSearchAllNamesAFailureAndDoesNotCountItAsEmpty(t *testing.T) {
	f := newResearchFixture(t)
	f.urlsFor = map[string][]string{"alpha one": pagesN("a", 1, 10), "gamma three": {}, "delta four": pagesN("d", 1, 10)}
	f.failFor = map[string]string{"beta two": "provider timed out"}
	cell, _ := searchCell(t, f)
	out := cell(readTop(10, "a") + readTop(10, "d") + `
r, _ := choir.SearchAll("q", []string{"alpha one", "beta two", "gamma three", "delta four"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 4 {
		t.Fatalf("host asked to start %d searches, want all 4:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	for _, want := range []string{
		`2 "beta two": FAILED (no provider answered: provider timed out)`,
		`3 "gamma three": sent s3 (0 new in first 0, 0 of 0 pages new)`,
		`4 "delta four": sent s4`,
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if strings.Contains(out.Stdout, "stopped yielding") {
		t.Fatalf("a failure was called a yield stop:\n%s", out.Stdout)
	}
}

func TestSearchAllStopsAfterTwoFailuresWithTheReason(t *testing.T) {
	f := newResearchFixture(t)
	f.failFor = map[string]string{"alpha one": "provider timed out"}
	f.hostErr = map[string]string{"beta two": "gateway search: status 502"}
	cell, _ := searchCell(t, f)
	out := cell(`r, _ := choir.SearchAll("q", []string{"alpha one", "beta two", "gamma three", "delta four"})
fmt.Println(r)`)
	for _, want := range []string{
		`1 "alpha one": FAILED (no provider answered: provider timed out)`,
		`2 "beta two": FAILED (`,
		`3 "gamma three": not sent (search is failing: `,
		`4 "delta four": not sent (search is failing: `,
		"gateway search: status 502",
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if f.searchCalls.Load() != 2 {
		t.Fatalf("host asked to start %d searches, want 2", f.searchCalls.Load())
	}
	if st := out.SearchStats; st == nil || st.NotSent["failing"] != 2 {
		t.Fatalf("stats = %+v, want 2 not sent because of failure", st)
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
	out := cell(readTop(10, "h") + readTop(4, "b") + readTop(3, "c") + `
r, _ := choir.SearchAll("q", []string{"alpha one", "beta two", "gamma three"})
fmt.Println(r)`)
	if f.searchCalls.Load() != 3 {
		t.Fatalf("host asked to start %d searches, want all 3:\n%s", f.searchCalls.Load(), out.Stdout)
	}
	if !strings.Contains(out.Stdout, `2 "beta two": sent s2 (4 new in first 10, 4 of 10 pages new)`) {
		t.Fatalf("stdout:\n%s", out.Stdout)
	}
}

func TestSearchAllNamesWhyAWordingWasNotSentAndMergesHeldPages(t *testing.T) {
	f := newResearchFixture(t)
	cell, _ := searchCell(t, f)
	out := cell(`choir.SearchStart("harbor strike")
for i, h := range []string{"a", "b", "a", "c", "d"} { choir.Read(fmt.Sprintf("https://%s.example/harbor-strike/%d", h, i)) }
r, _ := choir.SearchAll("q", []string{"x", "strike harbor", "harbor strike mauritius", ""})
fmt.Println(r)
held := 0
for _, h := range r.Results { if h.Held { held++ } }
fmt.Println("merged", len(r.Results), "held", held)`)
	for _, want := range []string{
		`1 "x": not sent (no word of two or more letters or digits)`,
		`2 "strike harbor": not sent (answered from your search s1, under a minute ago; its 5 pages are in the list, marked held; send it alone with choir.SearchFresh)`,
		`3 "harbor strike mauritius": sent s2`,
		`4 "": not sent (empty)`,
		"merged 10 held 5",
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
` + readTop(3, "h") + `
r, _ := choir.SearchAll("q", []string{"alpha one"})
for _, h := range r.Results { fmt.Println(h.URL, h.Held) }`)
	for _, want := range []string{"https://h.example/p2 true", "https://h.example/p3 true", "https://n.example/p1 false", "https://n.example/p2 false"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
}

// A first wording whose first ten pages are already 80% held ends the call;
// the wording left out can still be sent alone with fresh.
func TestSearchAllFirstWordingHeldThenAWordingSentWithFresh(t *testing.T) {
	f := newResearchFixture(t)
	seed := pagesN("h", 1, 10)
	f.urlsFor = map[string][]string{"seed words": seed, "alpha one": append(append([]string{}, seed[:9]...), "https://h.example/p11"), "dockers mauritius": pagesN("d", 1, 10)}
	cell, _ := searchCell(t, f)
	out := cell(`choir.SearchStart("seed words")
` + readTop(10, "h") + `
r, _ := choir.SearchAll("q", []string{"alpha one", "dockers mauritius"})
fmt.Println(r)
d, _ := choir.SearchFresh("dockers mauritius", "the first wording found only pages already held")
fmt.Println("alone:", d.ID, len(d.Results), d.NewPages)`)
	for _, want := range []string{
		`1 "alpha one": sent s2 (1 new in first 10, 1 of 10 pages new)`,
		`2 "dockers mauritius": not sent (the question had stopped yielding after wording 1)`,
		"alone: s3 10 10",
	} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
}

func TestSearchAllNamesEveryWordingPastItsDeadline(t *testing.T) {
	f := newResearchFixture(t)
	saved := searchAllDeadline
	searchAllDeadline = -time.Second
	t.Cleanup(func() { searchAllDeadline = saved })
	cell, _ := searchCell(t, f)
	out := cell(`r, _ := choir.SearchAll("q", []string{"alpha one", "beta two", "gamma three"})
fmt.Println(r)`)
	for _, want := range []string{`1 "alpha one": not sent (the call's -1-second deadline passed`, `3 "gamma three": not sent (the call's -1-second deadline passed`} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout lacks %q:\n%s", want, out.Stdout)
		}
	}
	if f.searchCalls.Load() != 0 {
		t.Fatalf("a call past its deadline sent %d searches", f.searchCalls.Load())
	}
}
