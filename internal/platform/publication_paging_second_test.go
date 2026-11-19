package platform

// Ways paging over published stories can skip or repeat rows that share one
// second (docs/problems/story-list-paging-can-skip-rows-in-one-second-
// 2026-11-19.md). The list is ordered by (updated_at, canonical_id) descending
// in the SQL at full timestamp precision; before the fix the cursor of each
// caller below kept whole seconds only.
//
//  1. Front list (ListFrontStories, publication_front.go): a page ends on a
//     row of a second that holds more, older rows; the next page's cursor
//     says "updated_at < second.000", so those older same-second rows are
//     skipped. Never listed, no error.
//  2. Section list (listFrontSection, publication_front.go): the same, at
//     the end of a batch or of a page.
//  3. scanPublicationRows (publication_pieces.go), used by the published
//     piece search read path: the same at a batch boundary.
//  4. The piece index walk (refreshPieceIndex, publication_pieces_index.go):
//     the same at a batch boundary; the skipped story is never indexed, so a
//     search never finds it.
//  5. Rows with IDENTICAL timestamps: only canonical_id orders them; a cursor
//     that loses the time's fraction also mis-compares the tie row.
//  6. A cursor minted by an OLD release (whole seconds, no "n" member)
//     presented to the new code: it must page without an error and without
//     skipping. Rows of that cursor's second that were already listed may be
//     listed again (a repeat is acceptable, a skip is not); the new code
//     reads it as "everything older than the end of that second".
//  7. Out of scope: a cursor minted by the new code presented to an OLD
//     process during a deploy; the old process reads whole seconds and may
//     repeat or skip as it always could.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type skipRow struct {
	doc   string
	title string
	id    string
	at    time.Time
}

// publishSameSecond publishes n stories whose update times share one second:
// distinct microseconds, with some equal (only canonical_id orders those).
// The rows come back in the order the list must show them.
func publishSameSecond(t *testing.T, f frontFixture, n int) []skipRow {
	t.Helper()
	second := time.Now().UTC().Truncate(time.Second).Add(-3 * time.Hour)
	micros := []int{100, 100, 250, 250, 250, 400, 555000, 555000, 999999, 120, 130, 140}
	var rows []skipRow
	for i := 0; i < n; i++ {
		doc := fmt.Sprintf("doc-skip-%02d", i)
		title := fmt.Sprintf("Skip %02d", i)
		publishStoryRevision(t, f.svc, doc, 1, title, "skiptag story number "+title+".", nil)
		ident, err := publicationIdentityFor("paper-owner", doc)
		if err != nil {
			t.Fatal(err)
		}
		at := second.Add(time.Duration(micros[i%len(micros)]) * time.Microsecond)
		setPublicationUpdatedAt(t, f.store, "paper-owner", doc, at)
		rows = append(rows, skipRow{doc: doc, title: title, id: ident.CanonicalID, at: at})
	}
	sort.Slice(rows, func(a, b int) bool {
		if !rows[a].at.Equal(rows[b].at) {
			return rows[a].at.After(rows[b].at)
		}
		return rows[a].id > rows[b].id
	})
	return rows
}

func wantTitles(rows []skipRow) string {
	var out []string
	for _, r := range rows {
		out = append(out, r.title)
	}
	return strings.Join(out, ",")
}

// sameOnce reports whether got holds exactly the rows, each once. A page's
// stories are re-sorted by the story's own time inside the page (not by the
// publication row's), so across pages the test compares what each row came
// back as, not the order within a page; the raw order is asserted on
// scanPublicationRows.
func sameOnce(got []string, rows []skipRow) bool {
	var want []string
	for _, r := range rows {
		want = append(want, r.title)
	}
	got = append([]string(nil), got...)
	sort.Strings(got)
	sort.Strings(want)
	return strings.Join(got, ",") == strings.Join(want, ",")
}

func skipTitles(got []FrontStory) []string {
	var out []string
	for _, s := range got {
		if strings.HasPrefix(s.Title, "Skip ") {
			out = append(out, s.Title)
		}
	}
	return out
}

func TestFrontListPagesEveryRowOfOneSecondOnce(t *testing.T) {
	f := newFrontFixture(t)
	rows := publishSameSecond(t, f, 12)
	for _, limit := range []int{1, 2, 3, 5} {
		var got []string
		cursor := ""
		for i := 0; i < 40; i++ {
			page, err := f.svc.ListFrontStories(context.Background(), limit, cursor)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, skipTitles(page.Stories)...)
			if cursor = page.NextCursor; cursor == "" {
				break
			}
		}
		if !sameOnce(got, rows) {
			t.Fatalf("limit %d: front pages gave\n %v\nwant each of\n %s", limit, got, wantTitles(rows))
		}
	}
}

func TestSectionListPagesEveryRowOfOneSecondOnce(t *testing.T) {
	f := newFrontFixture(t)
	rows := publishSameSecond(t, f, 12)
	all, err := f.svc.ListFrontStories(context.Background(), 100, "")
	if err != nil {
		t.Fatal(err)
	}
	scores := map[string]map[string]any{}
	for _, s := range all.Stories {
		scores[s.PublicationID] = map[string]any{"revision": s.LatestRevision, "importance": 5, "section": "world"}
	}
	raw, _ := json.Marshal(map[string]any{"version": 1, "scores": scores})
	path := filepath.Join(t.TempDir(), "scores.json")
	writeScores(t, path, string(raw))
	f.svc.SetFrontImportancePath(path)
	savedBatch := frontSectionBatchSize
	frontSectionBatchSize = 3
	defer func() { frontSectionBatchSize = savedBatch }()
	for _, limit := range []int{1, 2, 4} {
		var got []string
		cursor := ""
		for i := 0; i < 60; i++ {
			page := sectionPage(t, f.svc, limit, cursor, "world")
			got = append(got, skipTitles(page.Stories)...)
			if cursor = page.NextCursor; cursor == "" {
				break
			}
		}
		if !sameOnce(got, rows) {
			t.Fatalf("limit %d: section pages gave\n %v\nwant each of\n %s", limit, got, wantTitles(rows))
		}
	}
}

func TestScanPublicationRowsReadsEveryRowOfOneSecondOnce(t *testing.T) {
	f := newFrontFixture(t)
	rows := publishSameSecond(t, f, 12)
	old := publishedPieceBatch
	publishedPieceBatch = 3
	t.Cleanup(func() { publishedPieceBatch = old })
	got, capped, err := f.svc.scanPublicationRows(context.Background(), 100)
	if err != nil || capped {
		t.Fatalf("scan: capped %v err %v", capped, err)
	}
	var ids []string
	for _, r := range got {
		for _, w := range rows {
			if r.CanonicalID == w.id {
				ids = append(ids, w.title)
			}
		}
	}
	if strings.Join(ids, ",") != wantTitles(rows) {
		t.Fatalf("scan gave\n %v\nwant\n %s", ids, wantTitles(rows))
	}
}

func TestPieceIndexWalkIndexesEveryRowOfOneSecond(t *testing.T) {
	f := newFrontFixture(t)
	rows := publishSameSecond(t, f, 12)
	old := publishedPieceBatch
	publishedPieceBatch = 3
	t.Cleanup(func() { publishedPieceBatch = old })
	res, err := f.svc.SearchPublishedPieces(context.Background(), "skiptag", 50)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, title := range pieceTitles(res) {
		have[title] = true
	}
	for _, r := range rows {
		if !have[r.title] {
			t.Fatalf("the piece index never indexed %q (found %v)", r.title, pieceTitles(res))
		}
	}
}

// Failure mode 6: a cursor an old release minted (whole seconds, no "n").
func TestFrontListTakesACursorOfAnOldReleaseWithoutSkipping(t *testing.T) {
	f := newFrontFixture(t)
	rows := publishSameSecond(t, f, 12)
	first, err := f.svc.ListFrontStories(context.Background(), 3+4, "")
	if err != nil {
		t.Fatal(err)
	}
	last := rows[3] // the fourth row of the newest-first order ends the page
	if got := skipTitles(first.Stories); !sameOnce(got, rows[:4]) {
		t.Fatalf("first page %v", got)
	}
	old, _ := json.Marshal(map[string]any{"u": last.at.Unix(), "id": last.id})
	cursor := base64.RawURLEncoding.EncodeToString(old)
	var got []string
	for i := 0; i < 40; i++ {
		page, err := f.svc.ListFrontStories(context.Background(), 5, cursor)
		if err != nil {
			t.Fatalf("an old cursor is refused: %v", err)
		}
		got = append(got, skipTitles(page.Stories)...)
		if cursor = page.NextCursor; cursor == "" {
			break
		}
	}
	// Nothing after the cursor may be missing; the same-second rows listed
	// before it may come again.
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	for _, r := range rows[4:] {
		if !seen[r.title] {
			t.Fatalf("old cursor skipped %q (got %v)", r.title, got)
		}
	}
}

func TestFrontCursorIsOpaqueAndJunkIsRefused(t *testing.T) {
	c := frontCursorAt(frontPublicationRow{CanonicalID: "obj:x", UpdatedAt: time.Unix(1700000000, 123456000).UTC()})
	back, err := decodeFrontCursor(encodeFrontCursor(*c))
	if err != nil || *back != *c {
		t.Fatalf("round trip: %+v %v", back, err)
	}
	zero := frontCursorAt(frontPublicationRow{CanonicalID: "obj:x", UpdatedAt: time.Unix(1700000000, 0).UTC()})
	back, err = decodeFrontCursor(encodeFrontCursor(*zero))
	if err != nil || *back != *zero {
		t.Fatalf("a whole-second row's own cursor must stay exact: %+v %v", back, err)
	}
	for _, bad := range []string{"!!", base64.RawURLEncoding.EncodeToString([]byte(`{"u":1}`))} {
		if _, err := decodeFrontCursor(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}
