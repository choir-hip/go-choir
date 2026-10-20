package maild

// Failure modes this file guards (design section 4.3 and section 8).
// Written before digest_select.go.
//
//	M5  Stale scores. When fewer than half the rows carry an importance, the
//	    page falls back to recency; the digest must do the same and say so
//	    (ranked_by: recency) so a poor digest is explainable.
//	M6  Repeats. A story sent yesterday is sent again today. A story is
//	    skipped unless it has a newer version than the one last sent; a
//	    revised story is marked Updated. Two consecutive days are simulated.
//	S1  A promotional story, a leaked "Open a story..." instruction, or a
//	    one-source story reaches the mail. None may.
//	S2  A day-old or older story reaches the mail. Candidates are rows
//	    updated within the last 24 hours of the run clock.
//	S3  One section floods the mail. At most 3 per section, top 8 overall;
//	    stories with no section are not capped as a group.
//	S4  A thin day sends "no news". Fewer than 3 qualifying stories select
//	    nothing and say why (too_few).
//	S5  A story with no usable slug is linked to a wrong page. It is dropped.
//	S6  The mail's order differs from the page. The selection preserves the
//	    page's rank order (rankFrontRows) over the whole front list, then
//	    filters; the lead is the first story that may lead.

import (
	"fmt"
	"testing"
	"time"
)

var digestTestNow = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func intp(n int) *int { return &n }

func f64p(n float64) *float64 { return &n }

type rowOpt func(*FrontRow)

func digestRow(id string, minsAgo int, opts ...rowOpt) FrontRow {
	row := FrontRow{
		PublicationID:  id,
		Title:          "Story " + id,
		Dek:            "Dek " + id,
		UpdatedAt:      digestTestNow.Add(-time.Duration(minsAgo) * time.Minute).Format(time.RFC3339),
		VersionCount:   1,
		LatestRevision: 1,
		Importance:     f64p(6),
		SourceCount:    f64p(4),
		Words:          f64p(400),
	}
	row.Route.Bare = "/pub/texture/slug-" + id
	row.Route.Permalink = "/pub/texture/slug-" + id + "/v1"
	for _, opt := range opts {
		opt(&row)
	}
	return row
}

func withSection(s string) rowOpt { return func(r *FrontRow) { r.Section = s } }
func withImportance(n float64) rowOpt {
	return func(r *FrontRow) { r.Importance = f64p(n) }
}
func withSources(n float64) rowOpt { return func(r *FrontRow) { r.SourceCount = f64p(n) } }
func withRevision(n int) rowOpt {
	return func(r *FrontRow) { r.LatestRevision = n; r.VersionCount = float64(n) }
}
func withTitle(s string) rowOpt { return func(r *FrontRow) { r.Title = s } }
func withBare(s string) rowOpt  { return func(r *FrontRow) { r.Route.Bare = s } }
func asPromo() rowOpt           { return func(r *FrontRow) { r.Promo = true } }
func unscored() rowOpt          { return func(r *FrontRow) { r.Importance = nil } }

func selIDs(sel digestSelection) []string {
	ids := make([]string, 0, len(sel.Stories))
	for _, s := range sel.Stories {
		ids = append(ids, s.PublicationID)
	}
	return ids
}

func TestDigestSelectionFiltersPromoInstructionAndOneSource(t *testing.T) {
	rows := []FrontRow{
		digestRow("ok1", 30, withImportance(9)),
		digestRow("ok2", 40, withImportance(8)),
		digestRow("ok3", 50, withImportance(7)),
		digestRow("promo", 20, withImportance(10), asPromo()),
		digestRow("instr", 25, withImportance(10), withTitle("  Open a story on the budget")),
		digestRow("one", 15, withImportance(10), withSources(1)),
		digestRow("zero", 16, withImportance(10), withSources(0)),
	}
	sel := selectDigest(rows, digestTestNow, nil)
	if sel.Skipped != "" {
		t.Fatalf("skipped = %q", sel.Skipped)
	}
	got := selIDs(sel)
	want := []string{"ok1", "ok2", "ok3"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("selected %v, want %v", got, want)
	}
}

func TestDigestSelectionUnknownSourceCountIsAllowed(t *testing.T) {
	rows := []FrontRow{
		digestRow("a", 30, func(r *FrontRow) { r.SourceCount = nil }),
		digestRow("b", 40),
		digestRow("c", 50),
	}
	sel := selectDigest(rows, digestTestNow, nil)
	if len(sel.Stories) != 3 {
		t.Fatalf("selected %v, want 3 (unknown source count is allowed, as on the front page)", selIDs(sel))
	}
}

func TestDigestSelectionOnlyLast24Hours(t *testing.T) {
	rows := []FrontRow{
		digestRow("fresh1", 60),
		digestRow("fresh2", 600),
		digestRow("fresh3", 24*60-1),
		digestRow("old", 24*60+5, withImportance(10)),
		digestRow("ancient", 72*60, withImportance(10)),
		digestRow("baddate", 10, func(r *FrontRow) { r.UpdatedAt = "yesterday-ish" }),
	}
	sel := selectDigest(rows, digestTestNow, nil)
	got := map[string]bool{}
	for _, id := range selIDs(sel) {
		got[id] = true
	}
	if got["old"] || got["ancient"] || got["baddate"] {
		t.Fatalf("selected stories outside the 24 hour window: %v", selIDs(sel))
	}
	if !got["fresh1"] || !got["fresh2"] || !got["fresh3"] {
		t.Fatalf("missing fresh stories: %v", selIDs(sel))
	}
}

func TestDigestSelectionCapsSectionsAndTotal(t *testing.T) {
	var rows []FrontRow
	for i := 0; i < 6; i++ {
		rows = append(rows, digestRow(fmt.Sprintf("w%d", i), 10+i, withSection("world"), withImportance(9)))
	}
	for i := 0; i < 6; i++ {
		rows = append(rows, digestRow(fmt.Sprintf("b%d", i), 70+i, withSection("business"), withImportance(7)))
	}
	for i := 0; i < 3; i++ {
		rows = append(rows, digestRow(fmt.Sprintf("t%d", i), 130+i, withSection("tech_ai"), withImportance(5)))
	}
	sel := selectDigest(rows, digestTestNow, nil)
	if len(sel.Stories) != 8 {
		t.Fatalf("selected %d stories %v, want 8", len(sel.Stories), selIDs(sel))
	}
	count := map[string]int{}
	for _, s := range sel.Stories {
		count[s.Section]++
	}
	for section, n := range count {
		if n > 3 {
			t.Fatalf("section %s has %d stories, want at most 3 (%v)", section, n, selIDs(sel))
		}
	}
	if count["world"] != 3 || count["business"] != 3 || count["tech_ai"] != 2 {
		t.Fatalf("section counts = %v, want world 3, business 3, tech_ai 2", count)
	}
}

func TestDigestSelectionUnsectionedStoriesAreNotCappedAsAGroup(t *testing.T) {
	var rows []FrontRow
	for i := 0; i < 6; i++ {
		rows = append(rows, digestRow(fmt.Sprintf("u%d", i), 10+i, unscored()))
	}
	sel := selectDigest(rows, digestTestNow, nil)
	if len(sel.Stories) != 6 {
		t.Fatalf("selected %v, want all 6 unsectioned stories", selIDs(sel))
	}
	if sel.RankedBy != "recency" {
		t.Fatalf("ranked_by = %q, want recency (no row scored)", sel.RankedBy)
	}
}

func TestDigestSelectionTooFewSendsNothing(t *testing.T) {
	rows := []FrontRow{digestRow("a", 30), digestRow("b", 40), digestRow("promo", 20, asPromo())}
	sel := selectDigest(rows, digestTestNow, nil)
	if sel.Skipped != "too_few" || len(sel.Stories) != 0 {
		t.Fatalf("selection = skipped %q stories %v, want too_few and none", sel.Skipped, selIDs(sel))
	}
	if sel := selectDigest(nil, digestTestNow, nil); sel.Skipped != "too_few" {
		t.Fatalf("empty front list: skipped = %q, want too_few", sel.Skipped)
	}
}

func TestDigestSelectionStaleScoresFallBackToRecency(t *testing.T) {
	rows := []FrontRow{
		digestRow("new", 10, unscored()),
		digestRow("mid", 100, unscored()),
		digestRow("old", 200, unscored()),
		digestRow("scored-old", 300, withImportance(10)),
		digestRow("late", 400, unscored()),
	}
	sel := selectDigest(rows, digestTestNow, nil)
	if sel.RankedBy != "recency" {
		t.Fatalf("ranked_by = %q, want recency with 1 of 5 rows scored", sel.RankedBy)
	}
	if got := selIDs(sel); got[0] != "new" {
		t.Fatalf("recency lead = %v, want new first", got)
	}
	rows = []FrontRow{digestRow("a", 10), digestRow("b", 20), digestRow("c", 30)}
	if sel := selectDigest(rows, digestTestNow, nil); sel.RankedBy != "importance" {
		t.Fatalf("ranked_by = %q, want importance", sel.RankedBy)
	}
}

// S6: the mail follows the page's order, with the lead the first story that
// may lead. The filter runs after the rank over the whole list.
func TestDigestSelectionFollowsThePageRankOrder(t *testing.T) {
	rows := []FrontRow{
		digestRow("low", 20, withImportance(3)),
		digestRow("high", 60, withImportance(9)),
		digestRow("mid", 40, withImportance(6)),
		digestRow("onesource-top", 5, withImportance(10), withSources(1)),
	}
	page := goRankedIDs(rows)
	sel := selectDigest(rows, digestTestNow, nil)
	var want []string
	for _, id := range page {
		if id != "onesource-top" {
			want = append(want, id)
		}
	}
	if fmt.Sprint(selIDs(sel)) != fmt.Sprint(want) {
		t.Fatalf("selection %v, want page order without the one-source story %v", selIDs(sel), want)
	}
}

// M6: two consecutive days. Day one sends three stories. Day two sees the
// same three plus one new and one revised; the repeats are skipped and the
// revised story returns marked Updated.
func TestDigestNoRepeatsAcrossTwoDays(t *testing.T) {
	day1 := []FrontRow{
		digestRow("a", 30, withImportance(9)),
		digestRow("b", 60, withImportance(8)),
		digestRow("c", 90, withImportance(7)),
	}
	sel1 := selectDigest(day1, digestTestNow, map[string]int{})
	if len(sel1.Stories) != 3 {
		t.Fatalf("day 1 selected %v", selIDs(sel1))
	}
	sent := map[string]int{}
	for _, s := range sel1.Stories {
		sent[s.PublicationID] = s.Revision
	}
	day2Now := digestTestNow.Add(24 * time.Hour)
	mk := func(id string, minsAgo int, opts ...rowOpt) FrontRow {
		row := digestRow(id, 0, opts...)
		row.UpdatedAt = day2Now.Add(-time.Duration(minsAgo) * time.Minute).Format(time.RFC3339)
		return row
	}
	day2 := []FrontRow{
		mk("a", 30, withImportance(9)),                  // sent, unchanged: skipped
		mk("b", 60, withImportance(8), withRevision(2)), // sent as v1, now v2: back, Updated
		mk("c", 90, withImportance(7)),                  // sent, unchanged: skipped
		mk("d", 40, withImportance(6)),
		mk("e", 50, withImportance(5)),
		mk("f", 70, withImportance(4)),
	}
	sel2 := selectDigest(day2, day2Now, sent)
	got := map[string]digestStory{}
	for _, s := range sel2.Stories {
		got[s.PublicationID] = s
	}
	if _, ok := got["a"]; ok {
		t.Fatalf("day 2 repeated a: %v", selIDs(sel2))
	}
	if _, ok := got["c"]; ok {
		t.Fatalf("day 2 repeated c: %v", selIDs(sel2))
	}
	if b, ok := got["b"]; !ok || !b.Updated || b.Revision != 2 {
		t.Fatalf("day 2: b = %+v ok=%v, want present, Updated, revision 2", b, ok)
	}
	if len(sel2.Stories) != 4 {
		t.Fatalf("day 2 selected %v, want b d e f", selIDs(sel2))
	}
	// A third day with nothing new sends nothing.
	for _, s := range sel2.Stories {
		sent[s.PublicationID] = s.Revision
	}
	sel3 := selectDigest(day2, day2Now.Add(time.Hour), sent)
	if sel3.Skipped != "too_few" {
		t.Fatalf("day 3 with nothing new: skipped = %q stories %v", sel3.Skipped, selIDs(sel3))
	}
}

// S5: a story whose bare route has no safe slug is dropped, not mislinked.
func TestDigestSelectionDropsStoriesWithoutASafeSlug(t *testing.T) {
	rows := []FrontRow{
		digestRow("ok1", 10),
		digestRow("ok2", 20),
		digestRow("ok3", 30),
		digestRow("noslug", 5, withBare("/pub/texture/")),
		digestRow("notpub", 6, withBare("/elsewhere/x")),
		digestRow("evil", 7, withBare("/pub/texture/x\"><script>alert(1)</script>")),
		digestRow("slash", 8, withBare("/pub/texture/a/b")),
	}
	sel := selectDigest(rows, digestTestNow, nil)
	if fmt.Sprint(selIDs(sel)) != "[ok1 ok2 ok3]" {
		t.Fatalf("selected %v, want only the stories with safe slugs", selIDs(sel))
	}
	for _, s := range sel.Stories {
		if s.Slug != "slug-"+s.PublicationID {
			t.Fatalf("slug = %q for %s", s.Slug, s.PublicationID)
		}
	}
}
