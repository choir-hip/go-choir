package platform

// Ways research's published-piece search can fail (docs/problems/oped-not-connected-2026-10-20.md):
//
//  F5. It returns a piece that was never published to the paper: a
//      hand-published owner-mode Texture, or a publication whose state is not
//      "published".
//  F6. It reads only the first window of publications, so a piece past the
//      first batch is never found, or it returns an unbounded list.
//  F7. A hit carries no URL that choir.Read can capture, so research could
//      only cite a snippet.

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

func pieceTitles(res *PublishedPieceSearch) []string {
	out := []string{}
	for _, p := range res.Pieces {
		out = append(out, p.Title)
	}
	return out
}

func TestSearchPublishedPiecesOnlyPublishedStoriesAndOpinion(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	publishAs(t, f.svc, "paper-owner", PublishModeOwner, "doc-essay", 1, "Harbor essay", "A hand published harbor essay.", nil)
	publishAs(t, f.svc, "paper-owner", PublishModeAuto, "doc-gone", 1, "Harbor withdrawn", "A harbor story no longer published.", nil)
	ident, err := publicationIdentityFor("paper-owner", "doc-gone")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET metadata = jsonb_set(metadata::jsonb, '{state}', '"withdrawn"')::text WHERE canonical_id = ?`, ident.CanonicalID); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	res, err := f.svc.SearchPublishedPieces(ctx, "harbor", 10)
	if err != nil {
		t.Fatal(err)
	}
	got := pieceTitles(res)
	if len(got) != 1 || got[0] != "Harbor reopens" {
		t.Fatalf("pieces = %v, want only the harbor story", got)
	}
	if res.Pieces[0].Kind != "story" {
		t.Fatalf("kind = %q, want story", res.Pieces[0].Kind)
	}
	// The unfiltered retrieval search still sees the owner-mode essay; the
	// research route does not.
	all, err := f.svc.SearchPublished(ctx, "harbor")
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, r := range all.Results {
		if r.Title == "Harbor essay" {
			seen = true
		}
		if r.Title == "Harbor withdrawn" {
			t.Fatalf("unfiltered search returned a publication whose state is not published")
		}
	}
	if !seen {
		t.Fatalf("unfiltered search lost the owner-mode essay: %+v", all.Results)
	}
}

func TestSearchPublishedPiecesPastTheFirstWindow(t *testing.T) {
	old := publishedPieceBatch
	publishedPieceBatch = 2
	t.Cleanup(func() { publishedPieceBatch = old })
	f := newFrontFixture(t)
	// A needle story older than the three fixture stories: past several
	// windows of two.
	publishStoryRevision(t, f.svc, "doc-needle", 1, "Lighthouse relit", "The lighthouse keeper relit the lamp.", nil)
	setPublicationUpdatedAt(t, f.store, "paper-owner", "doc-needle", timeNowMinusDay())
	res, err := f.svc.SearchPublishedPieces(context.Background(), "lighthouse", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := pieceTitles(res); fmt.Sprint(got) != "[Lighthouse relit]" {
		t.Fatalf("pieces = %v, want the lighthouse story found past the first window", got)
	}
	// The unfiltered route reads past the first window as well.
	all, err := f.svc.SearchPublished(context.Background(), "lighthouse")
	if err != nil || len(all.Results) != 1 {
		t.Fatalf("SearchPublished past the first window: %v %+v", err, all)
	}
}

func TestSearchPublishedPiecesBoundedResults(t *testing.T) {
	f := newFrontFixture(t)
	for i := 0; i < 6; i++ {
		publishStoryRevision(t, f.svc, fmt.Sprintf("doc-tide-%d", i), 1, fmt.Sprintf("Tide report %d", i), "The tide table for the week.", nil)
	}
	res, err := f.svc.SearchPublishedPieces(context.Background(), "tide", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Pieces) != 3 {
		t.Fatalf("got %d pieces, want exactly the limit 3", len(res.Pieces))
	}
	big, err := f.svc.SearchPublishedPieces(context.Background(), "tide", 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(big.Pieces) > 25 {
		t.Fatalf("got %d pieces, the limit is clamped to 25", len(big.Pieces))
	}
	// A scan that stops at its cap says so.
	old := publishedPieceScanMax
	publishedPieceScanMax = 2
	t.Cleanup(func() { publishedPieceScanMax = old })
	capped, err := f.svc.SearchPublishedPieces(context.Background(), "zzz-no-such-word", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !capped.Capped {
		t.Fatalf("a scan cut by its cap must report capped")
	}
}

func TestSearchPublishedPiecesReadURLIsPlainText(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.SetPublicBaseURL("https://paper.example/")
	res, err := f.svc.SearchPublishedPieces(context.Background(), "bridge", 10)
	if err != nil || len(res.Pieces) != 1 {
		t.Fatalf("search: %v %+v", err, res)
	}
	p := res.Pieces[0]
	u, err := url.Parse(p.ReadURL)
	if err != nil {
		t.Fatalf("read_url %q: %v", p.ReadURL, err)
	}
	if u.Host != "paper.example" || u.Path != "/api/platform/publications/export" ||
		u.Query().Get("route") != p.RoutePath || u.Query().Get("format") != "txt" || u.Query().Get("raw") != "1" {
		t.Fatalf("read_url = %q, want the raw plain-text export of %s", p.ReadURL, p.RoutePath)
	}
	if !strings.Contains(p.Snippet, "bridge") {
		t.Fatalf("snippet = %q", p.Snippet)
	}
}

func timeNowMinusDay() time.Time { return time.Now().UTC().Truncate(time.Second).Add(-24 * time.Hour) }
