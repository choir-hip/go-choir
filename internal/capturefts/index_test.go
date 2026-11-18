package capturefts

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// The capture full-text index (docs/reference/CAPTURE-FULLTEXT.md). Failure
// modes guarded here, guarded (the numbers are the brief's):
//   1. a fact only in a page's body is not found, or comes with no passage;
//   2. a long natural-language query returns nothing (an all-words gate);
//   3. a page is not found at rank 1 by its own title;
//   4. a body-only match outranks a title match of equal term weight;
//   5. since/until do not restrict, or the recency order is not repeatable;
//   6. one scope's pages appear in another scope's search;
//  10. non-Latin text is not found, or accents do not fold;
//  11. a 2 MB page or an empty page breaks indexing;
//  12. a removed page stays findable, or a replaced page keeps old passages.

var scopeA = Scope{Owner: "owner-a", Computer: "computer-a"}

func newTestIndex(t *testing.T) *Index {
	t.Helper()
	ix, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = ix.Close() })
	return ix
}

var day0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func put(t *testing.T, ix *Index, scope Scope, id, title, text string, fetched time.Time) {
	t.Helper()
	if _, err := ix.Put(context.Background(), scope, Page{CaptureID: id, Title: title, TextHash: "sha256:" + id + fmt.Sprint(len(text)), Text: text, FetchedAt: fetched}); err != nil {
		t.Fatalf("Put %s: %v", id, err)
	}
}

func search(t *testing.T, ix *Index, scope Scope, q Query) Result {
	t.Helper()
	res, err := ix.Search(context.Background(), scope, q)
	if err != nil {
		t.Fatalf("Search %q: %v", q.Text, err)
	}
	return res
}

func ids(res Result) []string {
	var out []string
	for _, h := range res.Hits {
		out = append(out, h.CaptureID)
	}
	return out
}

// filler is n invented sentences that share no query word with the tests.
func filler(n int, salt string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "Section %s%d notes the routine ledger of crates, barrels and ropes at the quay, nothing unusual recorded.\n\n", salt, i)
	}
	return b.String()
}

func TestBodyOnlyFactIsFoundWithItsPassage(t *testing.T) {
	ix := newTestIndex(t)
	text := filler(20, "a") + "The quorum of Valdoria ratified the amber treaty after a long night of argument.\n\n" + filler(20, "b")
	put(t, ix, scopeA, "cap-1", "Weekly digest", text, day0)
	put(t, ix, scopeA, "cap-2", "Another digest", filler(30, "c"), day0)

	res := search(t, ix, scopeA, Query{Text: "amber treaty ratified"})
	if len(res.Hits) != 1 || res.Hits[0].CaptureID != "cap-1" {
		t.Fatalf("hits %v", ids(res))
	}
	h := res.Hits[0]
	if h.TitleMatch {
		t.Fatalf("a body-only fact is not a title match: %+v", h)
	}
	passage := text[h.Start:h.End]
	if !strings.Contains(passage, "amber treaty") {
		t.Fatalf("the best passage [%d,%d) lacks the fact: %q", h.Start, h.End, passage)
	}
	if h.Passages < 3 {
		t.Fatalf("a page of this length should hold several passages, got %d", h.Passages)
	}
	snip := Snippet(passage, res.Terms, 240)
	if !strings.Contains(snip, "amber treaty") || utf8.RuneCountInString(snip) > 260 {
		t.Fatalf("snippet %q", snip)
	}
	if got := MatchedTerms(passage, res.Terms); len(got) != 3 {
		t.Fatalf("terms matched in the passage: %v", got)
	}
}

func TestLongNaturalLanguageQueryStillRanks(t *testing.T) {
	ix := newTestIndex(t)
	put(t, ix, scopeA, "cap-target", "Council session", filler(5, "t")+"The quorum of Valdoria decided the amber treaty stands for another week.\n\n", day0)
	put(t, ix, scopeA, "cap-weak", "Market note", filler(5, "w")+"Prices will hold for another week, traders said.\n\n", day0)
	put(t, ix, scopeA, "cap-none", "Unrelated", filler(8, "n"), day0)
	q := "what did the quorum of valdoria decide about the amber treaty for the next week"
	res := search(t, ix, scopeA, Query{Text: q})
	got := ids(res)
	if len(got) < 2 || got[0] != "cap-target" || got[1] != "cap-weak" {
		t.Fatalf("a 15-word query must rank the target first and still return the weak match: %v", got)
	}
}

func TestPageIsFoundByItsOwnTitleAtRankOne(t *testing.T) {
	ix := newTestIndex(t)
	for i := 0; i < 40; i++ {
		put(t, ix, scopeA, fmt.Sprintf("cap-%02d", i), fmt.Sprintf("Quay report number %d on ropes", i), filler(6, fmt.Sprint(i)), day0.Add(time.Duration(i)*time.Hour))
	}
	put(t, ix, scopeA, "cap-known", "Lighthouse keepers strike over lamp oil", filler(6, "k")+"A lamp oil dispute.\n\n", day0)
	res := search(t, ix, scopeA, Query{Text: "Lighthouse keepers strike over lamp oil"})
	if len(res.Hits) == 0 || res.Hits[0].CaptureID != "cap-known" || !res.Hits[0].TitleMatch {
		t.Fatalf("known item not at rank 1: %v", ids(res))
	}
}

func TestTitleMatchOutranksBodyOnlyMatch(t *testing.T) {
	ix := newTestIndex(t)
	body := strings.Repeat("alpha beta gamma delta epsilon ", 40)
	// FTS5 gives a term found in more than half the documents no weight, so
	// the corpus holds other documents, as a real one does.
	for i := 0; i < 20; i++ {
		put(t, ix, scopeA, fmt.Sprintf("cap-other-%d", i), "Other page", body, day0)
	}
	put(t, ix, scopeA, "cap-title", "zephyr alpha", body, day0)
	put(t, ix, scopeA, "cap-body", "gamma alpha", body+"zephyr", day0)
	res := search(t, ix, scopeA, Query{Text: "zephyr"})
	if got := ids(res); len(got) != 2 || got[0] != "cap-title" {
		t.Fatalf("title match must lead: %v", got)
	}
	if !(res.Hits[0].Score > res.Hits[1].Score) {
		t.Fatalf("title score %v must exceed body score %v (title weight %v)", res.Hits[0].Score, res.Hits[1].Score, TitleWeight)
	}
	if TitleWeight != 3 {
		t.Fatalf("the title weight is 3 body words, as the published-pieces index")
	}
}

func TestTimeWindowAndRecencyOrder(t *testing.T) {
	ix := newTestIndex(t)
	// Ten pages about the same subject, fetched a day apart. Page 3 was
	// published long before it was fetched; page 4 was published after day 8.
	for i := 0; i < 10; i++ {
		p := Page{CaptureID: fmt.Sprintf("cap-%d", i), Title: "Quay inspection", TextHash: fmt.Sprintf("h%d", i),
			Text: "The inspector walked the quay and counted the barrels again.\n\n", FetchedAt: day0.AddDate(0, 0, i)}
		switch i {
		case 3:
			p.PublishedAt = day0.AddDate(0, 0, -60)
		case 4:
			p.PublishedAt = day0.AddDate(0, 0, 8)
		}
		if _, err := ix.Put(context.Background(), scopeA, p); err != nil {
			t.Fatal(err)
		}
	}
	window := search(t, ix, scopeA, Query{Text: "inspector quay", Since: day0.AddDate(0, 0, 2), Until: day0.AddDate(0, 0, 6)})
	got := map[string]bool{}
	for _, id := range ids(window) {
		got[id] = true
	}
	// Fetched days 2..5 are inside [day 2, day 6); page 3 is published
	// before the window and page 4 after it: the published time governs.
	for _, want := range []string{"cap-2", "cap-5"} {
		if !got[want] {
			t.Fatalf("%s missing from the window: %v", want, ids(window))
		}
	}
	for _, not := range []string{"cap-0", "cap-1", "cap-3", "cap-4", "cap-6", "cap-9"} {
		if got[not] {
			t.Fatalf("%s must be outside the window: %v", not, ids(window))
		}
	}
	recent := search(t, ix, scopeA, Query{Text: "inspector quay", Order: ByRecency})
	again := search(t, ix, scopeA, Query{Text: "inspector quay", Order: ByRecency})
	if strings.Join(ids(recent), ",") != strings.Join(ids(again), ",") {
		t.Fatalf("recency order is not repeatable: %v vs %v", ids(recent), ids(again))
	}
	// Effective times, newest first: cap-4 (published day 8), cap-9 ... cap-5,
	// cap-2, cap-1, cap-0, then cap-3 (published 60 days before).
	want := "cap-9,cap-4,cap-8,cap-7,cap-6,cap-5,cap-2,cap-1,cap-0,cap-3"
	if got := strings.Join(ids(recent), ","); got != want {
		t.Fatalf("recency order\n got %s\nwant %s", got, want)
	}
	if limited := search(t, ix, scopeA, Query{Text: "inspector quay", Order: ByRecency, Limit: 3}); len(limited.Hits) != 3 || limited.Hits[0].CaptureID != "cap-9" {
		t.Fatalf("limit: %v", ids(limited))
	}
}

func TestScopesNeverMix(t *testing.T) {
	ix := newTestIndex(t)
	other := Scope{Owner: "owner-a", Computer: "computer-b"}
	stranger := Scope{Owner: "owner-b", Computer: "computer-a"}
	put(t, ix, scopeA, "cap-a", "Kestrel census", "kestrel eggs counted on the north cliff.\n\n", day0)
	put(t, ix, other, "cap-b", "Heron census", "heron nests counted on the south cliff.\n\n", day0)
	if got := ids(search(t, ix, scopeA, Query{Text: "heron nests"})); len(got) != 0 {
		t.Fatalf("scope A saw another computer's page: %v", got)
	}
	if got := ids(search(t, ix, other, Query{Text: "kestrel"})); len(got) != 0 {
		t.Fatalf("another computer saw scope A's page: %v", got)
	}
	if got := ids(search(t, ix, stranger, Query{Text: "kestrel heron"})); len(got) != 0 {
		t.Fatalf("another owner's search on the same computer id saw pages: %v", got)
	}
	if got := ids(search(t, ix, scopeA, Query{Text: "kestrel"})); len(got) != 1 || got[0] != "cap-a" {
		t.Fatalf("scope A lost its own page: %v", got)
	}
	// A scope with no captures answers empty and creates nothing to find.
	if res, err := ix.Search(context.Background(), Scope{Owner: "none", Computer: "none"}, Query{Text: "kestrel"}); err != nil || len(res.Hits) != 0 {
		t.Fatalf("empty scope: %v %v", res, err)
	}
	if _, err := ix.Search(context.Background(), Scope{}, Query{Text: "kestrel"}); err == nil {
		t.Fatal("a search without owner and computer must be refused")
	}
}

func TestNonLatinTextAndAccentsAreFound(t *testing.T) {
	ix := newTestIndex(t)
	put(t, ix, scopeA, "cap-han", "港口消息", "港口今天重新开放，货轮陆续进入港区。\n\n", day0)
	put(t, ix, scopeA, "cap-ru", "Новости порта", "Порт сегодня снова открыт, суда заходят в гавань.\n\n", day0)
	put(t, ix, scopeA, "cap-ar", "أخبار الميناء", "أعيد فتح الميناء اليوم وتدخل السفن تباعا إلى الرصيف.\n\n", day0)
	put(t, ix, scopeA, "cap-fr", "Café Münster", "La réouverture du café de Münster a eu lieu hier.\n\n", day0)
	put(t, ix, scopeA, "cap-en", "Harbor notes", "Two harbors reopened and the stories about them multiplied.\n\n", day0)
	for _, c := range []struct{ query, want string }{
		{"重新开放", "cap-han"},
		{"货轮", "cap-han"},
		{"гавань суда", "cap-ru"},
		{"ПОРТ", "cap-ru"},
		{"الميناء", "cap-ar"},
		{"ميناء", "cap-ar"},
		{"cafe munster reouverture", "cap-fr"},
		{"Café", "cap-fr"},
		{"harbor", "cap-en"},
		{"story", "cap-en"},
	} {
		got := ids(search(t, ix, scopeA, Query{Text: c.query}))
		if len(got) == 0 || got[0] != c.want {
			t.Errorf("query %q: %v, want %s first", c.query, got, c.want)
		}
	}
}

func TestHugeAndEmptyPagesDoNotBreakIndexing(t *testing.T) {
	ix := newTestIndex(t)
	var b strings.Builder
	b.WriteString("headmarker opens the long page.\n\n")
	para := strings.Repeat("ledger entry about ropes and barrels recorded at the quay today. ", 20) + "\n\n"
	for b.Len() < 2<<20 {
		b.WriteString(para)
	}
	b.WriteString("tailmarker closes the long page.\n\n")
	big := b.String()
	if len(big) < 2<<20 {
		t.Fatal("test page is not 2 MB")
	}
	res, err := ix.Put(context.Background(), scopeA, Page{CaptureID: "cap-big", Title: "Long ledger", TextHash: "h-big", Text: big, FetchedAt: day0})
	if err != nil {
		t.Fatalf("2 MB page: %v", err)
	}
	if !res.Truncated || res.Passages == 0 || res.Passages > MaxIndexedBytes/900 {
		t.Fatalf("a 2 MB page is indexed up to the cap: %+v", res)
	}
	if _, err := ix.Put(context.Background(), scopeA, Page{CaptureID: "cap-empty", Title: "Quiet page about gulls", TextHash: "h-empty", Text: "", FetchedAt: day0}); err != nil {
		t.Fatalf("empty page: %v", err)
	}
	if _, err := ix.Put(context.Background(), scopeA, Page{CaptureID: "cap-blank", Title: "", TextHash: "h-blank", Text: " \n\n\t ", FetchedAt: day0}); err != nil {
		t.Fatalf("blank page: %v", err)
	}
	if got := ids(search(t, ix, scopeA, Query{Text: "headmarker"})); len(got) != 1 || got[0] != "cap-big" {
		t.Fatalf("head of the long page: %v", got)
	}
	if got := ids(search(t, ix, scopeA, Query{Text: "tailmarker"})); len(got) != 0 {
		t.Fatalf("text past the cap is not indexed: %v", got)
	}
	if got := ids(search(t, ix, scopeA, Query{Text: "gulls"})); len(got) != 1 || got[0] != "cap-empty" {
		t.Fatalf("an empty page is still found by its title: %v", got)
	}
	st, err := ix.Stats(context.Background(), scopeA)
	if err != nil || st.Pages != 3 {
		t.Fatalf("stats %+v %v", st, err)
	}
}

func TestRemoveAndReplacePassages(t *testing.T) {
	ix := newTestIndex(t)
	ctx := context.Background()
	put(t, ix, scopeA, "cap-1", "Old title", "the walrus colony moved north.\n\n", day0)
	before := search(t, ix, scopeA, Query{Text: "walrus"})
	if len(before.Hits) != 1 {
		t.Fatalf("setup: %v", ids(before))
	}
	oldPassage := before.Hits[0].PassageID

	// The same page again changes nothing (backfill idempotence).
	again, err := ix.Put(ctx, scopeA, Page{CaptureID: "cap-1", Title: "Old title", TextHash: "sha256:cap-1" + fmt.Sprint(len("the walrus colony moved north.\n\n")), Text: "the walrus colony moved north.\n\n", FetchedAt: day0})
	if err != nil || !again.Unchanged {
		t.Fatalf("an identical Put must be a no-op: %+v %v", again, err)
	}
	if ok, err := ix.Indexed(ctx, scopeA, "cap-1", "sha256:cap-1"+fmt.Sprint(len("the walrus colony moved north.\n\n"))); err != nil || !ok {
		t.Fatalf("Indexed: %v %v", ok, err)
	}
	if ok, _ := ix.Indexed(ctx, scopeA, "cap-1", "some-other-hash"); ok {
		t.Fatal("a different text hash is not indexed")
	}

	// A changed text replaces the passages: the old word is gone, the new
	// is found, and the passage id is new (ids are never reused, so a vector
	// keyed on a passage id cannot point at the wrong text).
	if _, err := ix.Put(ctx, scopeA, Page{CaptureID: "cap-1", Title: "New title", TextHash: "h2", Text: "the narwhal pod moved south.\n\n", FetchedAt: day0}); err != nil {
		t.Fatal(err)
	}
	if got := ids(search(t, ix, scopeA, Query{Text: "walrus"})); len(got) != 0 {
		t.Fatalf("replaced passages still found: %v", got)
	}
	after := search(t, ix, scopeA, Query{Text: "narwhal"})
	if len(after.Hits) != 1 || after.Hits[0].PassageID <= oldPassage || after.Hits[0].Title != "New title" {
		t.Fatalf("replacement: %+v (old passage %d)", after.Hits, oldPassage)
	}
	if st, _ := ix.Stats(ctx, scopeA); st.Pages != 1 || st.Passages != 1 {
		t.Fatalf("stats after replace %+v", st)
	}

	if err := ix.Remove(ctx, scopeA, "cap-1"); err != nil {
		t.Fatal(err)
	}
	if got := ids(search(t, ix, scopeA, Query{Text: "narwhal new title"})); len(got) != 0 {
		t.Fatalf("removed page still found: %v", got)
	}
	if st, _ := ix.Stats(ctx, scopeA); st.Pages != 0 || st.Passages != 0 {
		t.Fatalf("stats after remove %+v", st)
	}
	// Removing what is not there is not an error.
	if err := ix.Remove(ctx, scopeA, "cap-missing"); err != nil {
		t.Fatal(err)
	}
	if err := ix.Remove(ctx, Scope{Owner: "never", Computer: "seen"}, "cap-1"); err != nil {
		t.Fatal(err)
	}
}

func TestIndexSurvivesRestartAndConcurrentUse(t *testing.T) {
	dir := t.TempDir()
	ix, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	put(t, ix, scopeA, "cap-1", "Persistent page", "an osprey nest on the mast.\n\n", day0)
	if err := ix.Close(); err != nil {
		t.Fatal(err)
	}
	ix, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	if got := ids(search(t, ix, scopeA, Query{Text: "osprey"})); len(got) != 1 {
		t.Fatalf("after restart: %v", got)
	}
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				id := fmt.Sprintf("cap-w%d-%d", w, i)
				if _, err := ix.Put(context.Background(), scopeA, Page{CaptureID: id, Title: "Concurrent", TextHash: id, Text: "an osprey nest, again.\n\n", FetchedAt: day0}); err != nil {
					t.Errorf("Put: %v", err)
					return
				}
				if _, err := ix.Search(context.Background(), scopeA, Query{Text: "osprey nest"}); err != nil {
					t.Errorf("Search: %v", err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	if st, _ := ix.Stats(context.Background(), scopeA); st.Pages != 101 {
		t.Fatalf("pages after concurrent puts: %d", st.Pages)
	}
}

func TestPassagesCoverTheTextInOrder(t *testing.T) {
	text := filler(60, "p")
	spans := Passages(text)
	if len(spans) < 4 {
		t.Fatalf("passages: %d", len(spans))
	}
	prev := 0
	for i, s := range spans {
		if s.Start < prev || s.End <= s.Start || s.End > len(text) {
			t.Fatalf("passage %d out of order: %+v after %d", i, s, prev)
		}
		if n := utf8.RuneCountInString(text[s.Start:s.End]); n > MaxPassageRunes {
			t.Fatalf("passage %d holds %d runes (max %d)", i, n, MaxPassageRunes)
		}
		prev = s.End
	}
	// One huge paragraph with no newline is still split.
	long := strings.Repeat("word ", 5000)
	if got := Passages(long); len(got) < 8 {
		t.Fatalf("a long single paragraph must split, got %d passages", len(got))
	}
	// CJK has no spaces: it splits by runes.
	han := strings.Repeat("港口重新开放。", 1000)
	for _, s := range Passages(han) {
		if utf8.RuneCountInString(han[s.Start:s.End]) > MaxPassageRunes {
			t.Fatal("a CJK passage is too long")
		}
	}
}

func TestCommonWordsOfALongQueryArePrunedAndTheSubjectStillWins(t *testing.T) {
	ix := newTestIndex(t)
	ix.SetScanBudget(130)
	// Every page holds "ledger", "crates" and "quay" (filler); one holds the
	// subject. Terms in no passage are dropped as well.
	for i := 0; i < 120; i++ {
		put(t, ix, scopeA, fmt.Sprintf("cap-%03d", i), "Quay report", filler(2, fmt.Sprint(i)), day0)
	}
	put(t, ix, scopeA, "cap-subject", "Quay report", filler(2, "s")+"The zanzibar clause was read aloud.\n\n", day0)
	res := search(t, ix, scopeA, Query{Text: "zanzibar clause ledger crates quay routine nonexistentword"})
	if len(res.Hits) == 0 || res.Hits[0].CaptureID != "cap-subject" {
		t.Fatalf("hits %v", ids(res))
	}
	pruned := strings.Join(res.Pruned, ",")
	common := 0
	for _, w := range []string{"ledger", "crate", "quay", "routine"} {
		if strings.Contains(pruned, w) {
			common++
		}
	}
	// Of the four words every page holds, the third rarest is kept with the
	// subject's two (the minimum kept), the others are pruned.
	if common != 3 || !strings.Contains(pruned, "nonexistentword") {
		t.Fatalf("pruned %v", res.Pruned)
	}
	if strings.Contains(pruned, "zanzibar") || strings.Contains(pruned, "clause") {
		t.Fatalf("the subject must not be pruned: %v", res.Pruned)
	}
	// A short query keeps its words however common they are.
	short := search(t, ix, scopeA, Query{Text: "ledger crates", Limit: 100})
	if len(short.Pruned) != 0 || len(short.Hits) != 100 {
		t.Fatalf("a two-word query is never pruned: pruned %v, hits %d", short.Pruned, len(short.Hits))
	}
}

func TestMoreScopesThanOpenFilesAllWork(t *testing.T) {
	ix := newTestIndex(t)
	for i := 0; i < maxOpenScopes*2+1; i++ {
		scope := Scope{Owner: "owner", Computer: fmt.Sprintf("computer-%d", i)}
		put(t, ix, scope, "cap-1", "Page", fmt.Sprintf("the falcon number%d circled.\n\n", i), day0)
	}
	for round := 0; round < 2; round++ {
		for i := 0; i < maxOpenScopes*2+1; i++ {
			scope := Scope{Owner: "owner", Computer: fmt.Sprintf("computer-%d", i)}
			if got := ids(search(t, ix, scope, Query{Text: fmt.Sprintf("falcon number%d", i)})); len(got) != 1 {
				t.Fatalf("scope %d round %d: %v", i, round, got)
			}
		}
	}
	ix.mu.Lock()
	open := len(ix.scopes)
	ix.mu.Unlock()
	if open > maxOpenScopes {
		t.Fatalf("%d files open, at most %d", open, maxOpenScopes)
	}
	scopes, err := ix.Scopes(context.Background())
	if err != nil || len(scopes) != maxOpenScopes*2+1 {
		t.Fatalf("Scopes: %d %v", len(scopes), err)
	}
}
