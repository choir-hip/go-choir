package platform

// Ways research's search over the paper's own archive can fail
// (docs/problems/published-search-recall-2026-11-10.md), written before the
// BM25 index:
//
//  S1. One query word the story lacks zeroes the story (all-words rule).
//  S2. A story older than the newest scan window is invisible.
//  S3. Ranking by raw counts lets one repeated word beat a story that holds
//      the query's rare words.
//  S4. A republished story is found by its old text, or not by its new text.
//  S5. A story withdrawn after it was indexed is still returned.
//  S6. A non-English title or an accented word is not found.
//  S7. A restarted corpusd (a new Service on the same store) finds nothing
//      until something is published.

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSearchPublishedPiecesPartialMatchRanksByRareWords(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	// S1: three words asked, the story holds two of them.
	publishStoryRevision(t, f.svc, "doc-two", 1, "Lighthouse relit", "The lighthouse keeper relit the lamp after the storm.", nil)
	// All three words.
	publishStoryRevision(t, f.svc, "doc-three", 1, "Lighthouse storm damage", "The lighthouse keeper counted damage after the storm at Pelican Point.", nil)
	// S3: one common word repeated many times, none of the rare ones.
	publishStoryRevision(t, f.svc, "doc-spam", 1, "Weather", "storm storm storm storm storm storm storm storm storm storm storm storm", nil)
	res, err := f.svc.SearchPublishedPieces(ctx, "lighthouse storm pelican", 10)
	if err != nil {
		t.Fatal(err)
	}
	got := pieceTitles(res)
	if len(got) < 3 || got[0] != "Lighthouse storm damage" || got[1] != "Lighthouse relit" {
		t.Fatalf("pieces = %v, want the three-word story, then the two-word story, then the rest", got)
	}
	for _, title := range got[:2] {
		if title == "Weather" {
			t.Fatalf("pieces = %v, want the repeated-word story below both", got)
		}
	}
}

func TestSearchPublishedPiecesWholeArchiveNotANewestWindow(t *testing.T) {
	old := publishedPieceBatch
	publishedPieceBatch = 2
	t.Cleanup(func() { publishedPieceBatch = old })
	f := newFrontFixture(t)
	for i := 0; i < 7; i++ {
		publishStoryRevision(t, f.svc, fmt.Sprintf("doc-filler-%d", i), 1, fmt.Sprintf("Filler %d", i), "Filler text about nothing in particular.", nil)
	}
	publishStoryRevision(t, f.svc, "doc-needle", 1, "Lighthouse relit", "The lighthouse keeper relit the lamp.", nil)
	setPublicationUpdatedAt(t, f.store, "paper-owner", "doc-needle", timeNowMinusDay().Add(-240*3600*1e9))
	res, err := f.svc.SearchPublishedPieces(context.Background(), "lighthouse", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := pieceTitles(res); fmt.Sprint(got) != "[Lighthouse relit]" {
		t.Fatalf("pieces = %v, want the oldest story found", got)
	}
	if res.Capped {
		t.Fatalf("a whole-archive search must not report capped")
	}
}

func TestSearchPublishedPiecesFollowsRepublishAndWithdrawal(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	publishStoryRevision(t, f.svc, "doc-ferry", 1, "Ferry timetable", "The ferry runs from the quay on weekdays.", nil)
	if res, err := f.svc.SearchPublishedPieces(ctx, "quay", 10); err != nil || len(res.Pieces) != 1 {
		t.Fatalf("first search: %v %+v", err, res)
	}
	publishStoryRevision(t, f.svc, "doc-ferry", 2, "Ferry timetable", "The ferry now sails from the pier on weekdays.", nil)
	if res, _ := f.svc.SearchPublishedPieces(ctx, "pier", 10); len(res.Pieces) != 1 {
		t.Fatalf("S4: a republished story is not found by its new text: %+v", res)
	}
	if res, _ := f.svc.SearchPublishedPieces(ctx, "quay", 10); len(res.Pieces) != 0 {
		t.Fatalf("S4: a republished story is still found by its old text: %+v", res)
	}
	ident, err := publicationIdentityFor("paper-owner", "doc-ferry")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{state}', '"withdrawn"')::text WHERE canonical_id = ?`, ident.CanonicalID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if res, _ := f.svc.SearchPublishedPieces(ctx, "pier", 10); len(res.Pieces) != 0 {
		t.Fatalf("S5: a withdrawn story is still returned: %+v", res)
	}
}

func TestSearchPublishedPiecesNonEnglishAndRestart(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	publishStoryRevision(t, f.svc, "doc-zh", 1, "港口重新开放", "港口在一周风暴之后于黎明重新开放。", nil)
	publishStoryRevision(t, f.svc, "doc-fr", 1, "Élections municipales", "Les élections municipales ont lieu dimanche à Québec.", nil)
	for q, want := range map[string]string{"港口": "港口重新开放", "elections": "", "élections": "Élections municipales", "Québec": "Élections municipales"} {
		res, err := f.svc.SearchPublishedPieces(ctx, q, 10)
		if err != nil {
			t.Fatal(err)
		}
		if want == "" {
			continue // an unaccented query may or may not fold; only the accented forms are required
		}
		if got := pieceTitles(res); len(got) == 0 || got[0] != want {
			t.Fatalf("S6: query %q = %v, want %q first", q, got, want)
		}
	}
	// S7: a new Service on the same store answers from the whole store.
	again := NewService(f.store, f.svc.artifactsRoot, "")
	res, err := again.SearchPublishedPieces(ctx, "bridge", 10)
	if err != nil || len(res.Pieces) != 1 {
		t.Fatalf("S7: restarted service: %v %+v", err, res)
	}
}

// S8: a search that arrives while the boot build holds the refresher does
// not wait for it and says the answer may be partial.
func TestSearchPublishedPiecesDuringTheBuildSaysCapped(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.pieces.refreshMu.Lock()
	defer f.svc.pieces.refreshMu.Unlock()
	res, err := f.svc.SearchPublishedPieces(context.Background(), "harbor", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Capped {
		t.Fatalf("a search during the build must report capped: %+v", res)
	}
}

// B1: an empty or term-less query lists the newest pieces by the
// publication's updated time, not by the order the index saw them; a
// republished older story moves to the top.
func TestSearchPublishedPiecesEmptyQueryListsNewestFirst(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	publishStoryRevision(t, f.svc, "doc-oldest", 1, "Oldest story", "Text of the oldest story.", nil)
	setPublicationUpdatedAt(t, f.store, "paper-owner", "doc-oldest", timeNowMinusDay().Add(-72*time.Hour))
	publishStoryRevision(t, f.svc, "doc-newest", 1, "Newest story", "Text of the newest story.", nil)
	// Stored times are whole seconds: put the newest clearly before now, so a
	// later republish is clearly newer.
	setPublicationUpdatedAt(t, f.store, "paper-owner", "doc-newest", time.Now().UTC().Truncate(time.Second).Add(-30*time.Minute))
	first := func(q string) []string {
		t.Helper()
		res, err := f.svc.SearchPublishedPieces(ctx, q, 25)
		if err != nil {
			t.Fatal(err)
		}
		return pieceTitles(res)
	}
	for _, q := range []string{"", "!!! ..."} {
		got := first(q)
		if len(got) < 5 || got[0] != "Newest story" || got[len(got)-1] != "Oldest story" {
			t.Fatalf("query %q = %v, want newest first and oldest last", q, got)
		}
	}
	publishStoryRevision(t, f.svc, "doc-oldest", 2, "Oldest story", "Text of the oldest story, revised.", nil)
	if got := first(""); got[0] != "Oldest story" {
		t.Fatalf("after a republish = %v, want the republished story first", got)
	}
}

// A transient read failure while indexing does not drop a piece for good:
// it is retried on the next refresh.
func TestSearchPublishedPiecesRetriesATransientLoadFailure(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	if _, err := f.svc.SearchPublishedPieces(ctx, "harbor", 10); err != nil {
		t.Fatal(err)
	}
	ident, err := publicationIdentityFor("paper-owner", "doc-never-published")
	if err != nil {
		t.Fatal(err)
	}
	row := frontPublicationRow{CanonicalID: ident.CanonicalID, Metadata: []byte(`{"state":"published","route_path":"/pub/texture/nowhere","publish_mode":"auto","owner_id":"paper-owner"}`)}
	og := f.svc.ogStore()
	f.svc.indexPieceRow(ctx, og, row) // cannot load: the version is not there
	f.svc.pieces.mu.RLock()
	_, dirty := f.svc.pieces.dirty[ident.CanonicalID]
	f.svc.pieces.mu.RUnlock()
	if !dirty {
		t.Fatalf("a failed load of a would-be piece must be kept for retry")
	}
}
