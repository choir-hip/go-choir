package capturefts

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Review repairs (round 1): the query the engine runs is the query the model
// wrote. Failure modes, tests, run
// against copies with the repair reverted, then against the repair:
//   2. recent order took the newest 300 passages holding any one word;
//   3. -word was searched for; a quoted phrase was split into words;
//   4. dropped words were silent; the 32-term cap came before rarity;
//   5. one long page took most of the candidate slots;
//   6. a query with no usable word answered as a search with no hits;
//   7. open files stayed above the cap after release; the df cache was
//      cleared as the index grew.

func TestPlanQueryMatchArgument(t *testing.T) {
	var sixty []string
	var sixtyQuoted []string
	for i := 0; i < 60; i++ {
		sixty = append(sixty, fmt.Sprintf("term%d", i))
		sixtyQuoted = append(sixtyQuoted, fmt.Sprintf(`"term%d"`, i))
	}
	for _, c := range []struct{ query, match string }{
		{`executive order on tariffs`, `"executive" OR "order" OR "tariff"`},
		{`prices since 2020`, `"price" OR "since" OR "2020"`},
		{`"harbor strike" -union`, `("harbor strike") NOT ("union")`},
		{`AT&T (merger) NEAR/3 deal`, `"t" OR "merger" OR "near" OR "3" OR "deal"`},
		{`title: budget`, `"title" OR "budget"`},
		{strings.Join(sixty, " "), strings.Join(sixtyQuoted, " OR ")},
		{`C++`, `"c"`},
		{`covid-19 -"state of the union"`, `("covid" OR "19") NOT ("state of the union")`},
		{`harbor -union -dues strike`, `("harbor" OR "strike") NOT ("union" OR "due")`},
		{`!!! ---`, ``},
		{``, ``},
		{`-union`, ``},
	} {
		p := PlanQuery(c.query)
		if got := p.Match(p.Positive, p.Phrases); got != c.match {
			t.Errorf("query %q:\n got  %s\n want %s", c.query, got, c.match)
		}
	}
	// Exclusions and phrases are named.
	p := PlanQuery(`"harbor strike" -union`)
	if len(p.Phrases) != 1 || strings.Join(p.Phrases[0], " ") != "harbor strike" || len(p.Excluded) != 1 || p.Excluded[0] != "union" {
		t.Fatalf("plan %+v", p)
	}
}

func harborIndex(t *testing.T) *Index {
	ix := newTestIndex(t)
	put(t, ix, scopeA, "cap-a", "Dock news", "Dockworkers began a harbor strike at dawn over pay.\n\n", day0)
	put(t, ix, scopeA, "cap-b", "Union talks", "The harbor strike continues as union talks stall.\n\n", day0)
	put(t, ix, scopeA, "cap-c", "Union dues", "Union dues rise next year for all members.\n\n", day0)
	put(t, ix, scopeA, "cap-d", "Reversed", "A strike near the harbor ended on Tuesday.\n\n", day0)
	return ix
}

func TestAnExcludedWordIsNeverSearchedFor(t *testing.T) {
	ix := harborIndex(t)
	res := search(t, ix, scopeA, Query{Text: `"harbor strike" -union`})
	if got := ids(res); len(got) != 1 || got[0] != "cap-a" {
		t.Fatalf("hits %v, want only the page about the strike without union", got)
	}
	if len(res.Excluded) != 1 || res.Excluded[0] != "union" {
		t.Fatalf("the result must name the exclusion: %+v", res.Excluded)
	}
	// An excluded word the query also needs is not a search term either.
	res = search(t, ix, scopeA, Query{Text: `harbor strike -union -dues`})
	for _, id := range ids(res) {
		if id == "cap-b" || id == "cap-c" {
			t.Fatalf("an excluded page was returned: %v", ids(res))
		}
	}
	// Only exclusions: nothing to search, said so.
	res = search(t, ix, scopeA, Query{Text: `-union`})
	if len(res.Hits) != 0 || res.Note == "" {
		t.Fatalf("exclusion-only query: %+v", res)
	}
}

func TestAQuotedPhraseIsAPhraseAndFallsBackToItsWords(t *testing.T) {
	ix := harborIndex(t)
	res := search(t, ix, scopeA, Query{Text: `"harbor strike"`})
	got := ids(res)
	if len(got) != 2 || (got[0] != "cap-a" && got[0] != "cap-b") || len(res.PhraseFallbacks) != 0 {
		t.Fatalf("phrase hits %v (fallbacks %v): only the pages with the words together", got, res.PhraseFallbacks)
	}
	for _, id := range got {
		if id == "cap-d" {
			t.Fatalf("a page with the words apart matched the phrase")
		}
	}
	// "strike harbor" is nowhere in that order: its words are searched, and
	// the result says so.
	res = search(t, ix, scopeA, Query{Text: `"strike harbor"`})
	if len(res.PhraseFallbacks) != 1 || res.PhraseFallbacks[0] != "strike harbor" || len(res.Hits) == 0 {
		t.Fatalf("fallback %v, hits %v", res.PhraseFallbacks, ids(res))
	}
}

func TestRecentOrderKeepsRelevance(t *testing.T) {
	ix := newTestIndex(t)
	put(t, ix, scopeA, "cap-harbor", "Harbor strike", "Dockworkers called a harbor strike this week.\n\n", day0)
	// Many newer pages that say only "strike".
	for i := 0; i < 400; i++ {
		put(t, ix, scopeA, fmt.Sprintf("cap-s%03d", i), "Bakers", fmt.Sprintf("A strike of bakers number %d ended.\n\n", i), day0.Add(time.Duration(i+1)*time.Hour))
	}
	res := search(t, ix, scopeA, Query{Text: "harbor strike", Order: ByRecency, Limit: 5})
	if len(res.Hits) == 0 || res.Hits[0].CaptureID != "cap-harbor" {
		t.Fatalf("recent order must lead with the page about the harbor strike, got %v", ids(res))
	}
	for _, id := range ids(res) {
		if strings.HasPrefix(id, "cap-s") {
			t.Fatalf("a page that says only 'strike' is not relevant enough for the recent list: %v", ids(res))
		}
	}
}

func TestOneLongPageDoesNotTakeTheCandidateSlots(t *testing.T) {
	saved := candidatePassages
	candidatePassages = 50
	t.Cleanup(func() { candidatePassages = saved })
	ix := newTestIndex(t)
	var long strings.Builder
	for i := 0; i < 140; i++ {
		long.WriteString(strings.Repeat("strike strike strike strike strike ", 2) + strings.Repeat("ledger entry about ropes and crates. ", 45) + "\n\n")
	}
	put(t, ix, scopeA, "cap-long", "Long ledger", long.String(), day0)
	for i := 0; i < 10; i++ {
		put(t, ix, scopeA, fmt.Sprintf("cap-short%d", i), "Short note", "A strike was called. "+strings.Repeat("ledger entry about ropes and crates. ", 45)+"\n\n", day0)
	}
	res := search(t, ix, scopeA, Query{Text: "strike", Limit: 25})
	seen := map[string]bool{}
	for _, id := range ids(res) {
		seen[id] = true
	}
	if !seen["cap-long"] {
		t.Fatal("the long page should be found")
	}
	for i := 0; i < 10; i++ {
		if !seen[fmt.Sprintf("cap-short%d", i)] {
			t.Fatalf("short page %d lost to the long page's passages: %v", i, ids(res))
		}
	}
	if MaxPassagesPerPage != 3 {
		t.Fatalf("the per-page cap is documented as 3, got %d", MaxPassagesPerPage)
	}
}

func TestTheTermCapComesAfterRarityAndIsReported(t *testing.T) {
	ix := newTestIndex(t)
	var words []string
	for i := 0; i < 34; i++ {
		words = append(words, fmt.Sprintf("filler%d", i))
	}
	common := strings.Join(words, " ")
	for i := 0; i < 3; i++ {
		put(t, ix, scopeA, fmt.Sprintf("cap-%d", i), "Page", common+" common text.\n\n", day0)
	}
	put(t, ix, scopeA, "cap-rare", "Page", common+" zzrare appears here.\n\n", day0)
	res := search(t, ix, scopeA, Query{Text: common + " zzrare"})
	if len(res.Hits) == 0 || res.Hits[0].CaptureID != "cap-rare" {
		t.Fatalf("the rare word at position 35 must decide: %v", ids(res))
	}
	if len(res.TermCapped) != 3 {
		t.Fatalf("35 terms, cap 32: three dropped and named, got %v", res.TermCapped)
	}
	for _, w := range res.TermCapped {
		if w == "zzrare" {
			t.Fatal("the rarest word was capped away")
		}
	}
	// The budget's drops are named too.
	ix.SetScanBudget(4)
	res = search(t, ix, scopeA, Query{Text: common + " zzrare"})
	if len(res.Pruned) == 0 {
		t.Fatalf("budget pruning must be reported: %+v", res)
	}
}

func TestAQueryWithNoUsableWordSaysSo(t *testing.T) {
	ix := harborIndex(t)
	for _, q := range []string{"", "!!! ---", "   "} {
		res := search(t, ix, scopeA, Query{Text: q})
		if len(res.Hits) != 0 || !strings.Contains(res.Note, "no searchable words") {
			t.Fatalf("query %q: %+v", q, res)
		}
	}
}

func TestOpenFilesAreBoundedAfterRelease(t *testing.T) {
	ix := newTestIndex(t)
	var releases []func()
	for i := 0; i < maxOpenScopes+3; i++ {
		s, release, err := ix.acquire(Scope{Owner: "o", Computer: fmt.Sprint("c", i)}, true)
		if err != nil || s == nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	for _, r := range releases {
		r()
	}
	ix.mu.Lock()
	open := len(ix.scopes)
	ix.mu.Unlock()
	if open > maxOpenScopes {
		t.Fatalf("%d files open after every release, at most %d", open, maxOpenScopes)
	}
}

func TestTheDocumentFrequencyCacheSurvivesGrowth(t *testing.T) {
	ix := newTestIndex(t)
	put(t, ix, scopeA, "cap-0", "Page", "alpha beta gamma delta.\n\n", day0)
	search(t, ix, scopeA, Query{Text: "alpha beta gamma delta epsilon"})
	s, release, _ := ix.acquire(scopeA, false)
	defer release()
	s.dfMu.Lock()
	before := len(s.df)
	s.df["sentinel"] = 7 // a cached count that no search would refill
	s.dfMu.Unlock()
	if before == 0 {
		t.Fatal("setup: the cache holds nothing")
	}
	for i := 1; i < 30; i++ { // the index grows many times over
		put(t, ix, scopeA, fmt.Sprintf("cap-%d", i), "Page", "alpha beta gamma delta.\n\n", day0)
	}
	search(t, ix, scopeA, Query{Text: "alpha beta gamma delta epsilon"})
	s.dfMu.Lock()
	_, kept := s.df["sentinel"]
	s.dfMu.Unlock()
	if !kept {
		t.Fatalf("the cache was cleared as the index grew (it held %d entries)", before)
	}
}

func TestDocumentFrequencyCountIsBounded(t *testing.T) {
	ix := newTestIndex(t)
	ix.SetScanBudget(5)
	for i := 0; i < 40; i++ {
		put(t, ix, scopeA, fmt.Sprintf("cap-%d", i), "Page", "ubiquitous word here.\n\n", day0)
	}
	s, release, _ := ix.acquire(scopeA, false)
	defer release()
	n, err := s.termDF(context.Background(), "ubiquitous")
	if err != nil || n != 6 { // budget + 1: "at least this many"
		t.Fatalf("bounded df = %d, %v; want 6", n, err)
	}
}
