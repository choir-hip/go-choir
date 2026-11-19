package capturefts

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Taking the best passages before the joins (rankPassages) must change
// nothing but the time. Failure modes, written before the code:
//   T1. a page with more than three matching passages takes slots of the
//       others after the cut, or the cut drops a page's third passage;
//   T2. a query whose best passages all belong to few pages is answered from a
//       list that is too short instead of falling back to the full statement;
//   T3. passages that tie on score at the cut are chosen differently from the
//       full statement (it breaks ties by newest, the cut by passage id);
//   T4. a windowed query is answered from the best passages overall and loses
//       the matches inside its window;
//   T5. passages whose page rows are gone (the cut joins to nothing) are
//       answered as an empty search, though the full statement may find more;
//   T6. the shortcut is never taken (the test would pass with it dead), or the
//       fallback is never taken.

// topCorpus has: a long page whose passages all hold the word many times (it
// has more than three matching passages and scores best), pages with one or
// two matching passages of different density, and ten identical pages that
// tie on score and differ only in time.
func topCorpus(t *testing.T) *Index {
	ix := newTestIndex(t)
	filler := func(n int, word string, every int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			if every > 0 && i%every == 0 {
				b.WriteString(word + " ")
			} else {
				b.WriteString(fmt.Sprintf("w%d ", i%97))
			}
			if i%50 == 49 {
				b.WriteString(".\n\n")
			}
		}
		return b.String()
	}
	put(t, ix, scopeA, "long", "a long page", filler(3000, "harbor", 5), day0)
	for i := 0; i < 12; i++ {
		put(t, ix, scopeA, fmt.Sprintf("mid%02d", i), "a page", filler(700+30*i, "harbor", 40+i), day0.Add(time.Duration(i)*time.Hour))
	}
	for i := 0; i < 10; i++ {
		put(t, ix, scopeA, fmt.Sprintf("tie%02d", i), "twin", filler(100, "harbor", 10), day0.Add(time.Duration(i)*24*time.Hour))
	}
	return ix
}

func hitsKey(res Result) string {
	var b strings.Builder
	for _, h := range res.Hits {
		fmt.Fprintf(&b, "%s/%d/%d/%.12f/%d;", h.CaptureID, h.PassageID, h.Ordinal, h.Score, h.Passages)
	}
	return b.String()
}

func withTop(k, cand int, f func()) {
	ot, oc := topPassages, candidatePassages
	topPassages, candidatePassages = k, cand
	defer func() { topPassages, candidatePassages = ot, oc }()
	f()
}

func TestTopPassagesAnswerLikeTheFullStatement(t *testing.T) {
	ix := topCorpus(t)
	ctx := context.Background()
	queries := []Query{
		{Text: "harbor", Limit: 50},
		{Text: "harbor w3 w7", Limit: 50},
		{Text: "harbor", Limit: 50, Order: ByRecency},
		{Text: "harbor", Limit: 50, Since: day0.Add(48 * time.Hour)},
		{Text: "harbor", Limit: 50, Until: day0.Add(48 * time.Hour)},
	}
	took, fell := 0, 0
	for _, cand := range []int{2, 3, 4, 7, 300} {
		for _, q := range queries {
			var want string
			var wantRes Result
			withTop(0, cand, func() { wantRes = search(t, ix, scopeA, q); want = hitsKey(wantRes) })
			for _, k := range []int{1, 2, 3, 5, 8, 13, 21, 40, 80, 1500} {
				var got Result
				withTop(k, cand, func() { got = search(t, ix, scopeA, q) })
				if hitsKey(got) != want || got.Pages != wantRes.Pages || got.Capped != wantRes.Capped || !reflect.DeepEqual(got.Terms, wantRes.Terms) {
					t.Fatalf("cand=%d k=%d query %+v: shortcut answers\n %s\nfull statement answers\n %s", cand, k, q, hitsKey(got), want)
				}
				if q.Since.IsZero() && q.Until.IsZero() {
					s, rel, err := ix.acquire(scopeA, false)
					if err != nil {
						t.Fatal(err)
					}
					plan := PlanQuery(q.Text)
					var ok bool
					withTop(k, cand, func() { _, _, ok, err = s.rankPassages(ctx, plan.Match(plan.Positive, nil), q, k) })
					rel()
					if err != nil {
						t.Fatal(err)
					}
					if ok {
						took++
					} else {
						fell++
					}
				}
			}
		}
	}
	if took == 0 || fell == 0 {
		t.Fatalf("the corpus must exercise both the shortcut (%d) and the fallback (%d)", took, fell)
	}
}

// A query whose best passages belong to one page cannot be answered from them.
func TestTopPassagesFallBackWhenFewPagesFillTheCut(t *testing.T) {
	ix := topCorpus(t)
	s, rel, err := ix.acquire(scopeA, false)
	if err != nil {
		t.Fatal(err)
	}
	defer rel()
	q := Query{Text: "harbor"}
	match := PlanQuery("harbor").Match([]string{"harbor"}, nil)
	var ok bool
	withTop(3, 300, func() { _, _, ok, err = s.rankPassages(context.Background(), match, q, 3) })
	if err != nil || ok {
		t.Fatalf("three passages of the long page fill the cut: ok=%v err=%v, want the fallback", ok, err)
	}
	// With room for every matching passage the shortcut stands.
	withTop(1500, 300, func() { _, _, ok, err = s.rankPassages(context.Background(), match, q, 1500) })
	if err != nil || !ok {
		t.Fatalf("cut larger than the matches: ok=%v err=%v, want the shortcut", ok, err)
	}
}

// T5. A cut whose passages all lack a page row joins to nothing; that is not
// an answer, the full statement must run. And a query nothing matches is
// answered empty either way.
func TestTopPassagesEmptyJoinFallsBack(t *testing.T) {
	ix := topCorpus(t)
	ctx := context.Background()
	s, rel, err := ix.acquire(scopeA, false)
	if err != nil {
		t.Fatal(err)
	}
	defer rel()
	match := PlanQuery("harbor").Match([]string{"harbor"}, nil)
	if _, err := s.db.ExecContext(ctx, `DELETE FROM pages`); err != nil {
		t.Fatal(err)
	}
	var ok bool
	hits, _, ok, err := s.rankPassages(ctx, match, Query{Text: "harbor"}, 1500)
	if err != nil || ok || len(hits) != 0 {
		t.Fatalf("the best passages have no page rows: ok=%v hits=%d err=%v, want the fallback", ok, len(hits), err)
	}
	// Nothing matches at all: the fallback is also empty and cheap.
	none := PlanQuery("zzzznothing").Match([]string{"zzzznothing"}, nil)
	hits, _, ok, err = s.rankPassages(ctx, none, Query{Text: "zzzznothing"}, 1500)
	if err != nil || len(hits) != 0 {
		t.Fatalf("no match: hits=%d err=%v", len(hits), err)
	}
	full, _, fok, err := s.rankPassages(ctx, none, Query{Text: "zzzznothing"}, 0)
	if err != nil || !fok || len(full) != 0 {
		t.Fatalf("no match, full statement: %d %v %v", len(full), fok, err)
	}
	_ = ok
}
