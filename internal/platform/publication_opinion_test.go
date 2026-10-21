package platform

// Ways Opinion publishing can fail (docs/problems/oped-not-connected-2026-10-20.md):
//
//  F1. An op-ed is counted as an Autopaper story: listed on the news front,
//      or taken by the related-stories walk.
//  F3. One user floods Opinion.
//  F4. Opinion is listed or publishable with the kill switch off.
//  F5. Research's search returns an opinion piece with the switch off.
//  F9. An opinion piece has no author label, or one that is an email.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func newOpinionService(t *testing.T) (*Service, *Store) {
	t.Helper()
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	svc.SetOpinionListing(true)
	return svc, store
}

func publishOpinion(svc *Service, owner, label, doc string, n int, title, content string) (*PublishTextureResponse, error) {
	return svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          owner,
		SourceDocID:      doc,
		SourceRevisionID: fmt.Sprintf("%s-rev-%d", doc, n),
		RevisionNumber:   n,
		Title:            title,
		Content:          content,
		RequestedBy:      owner,
		PublishMode:      PublishModeOpinion,
		AuthorLabel:      label,
		AccessPolicy:     []byte(`{"visibility":"public","route":"public"}`),
	})
}

func opinionRows(t *testing.T, svc *Service) []FrontStory {
	t.Helper()
	page, err := svc.ListFrontStoriesInSection(context.Background(), 0, "", FrontSectionOpinion)
	if err != nil {
		t.Fatalf("opinion bucket: %v", err)
	}
	return page.Stories
}

func TestOpinionKillSwitchOff(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "signing.key"))
	// Default off in code: a publish is refused and nothing is written.
	if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", "doc-op", 1, "My view", "A view on harbors."); !errors.Is(err, ErrOpinionListingOff) {
		t.Fatalf("opinion publish with the switch off = %v, want ErrOpinionListingOff", err)
	}
	svc.SetOpinionListing(true)
	if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", "doc-op", 1, "My view", "A view on harbors."); err != nil {
		t.Fatalf("opinion publish with the switch on: %v", err)
	}
	if rows := opinionRows(t, svc); len(rows) != 1 {
		t.Fatalf("opinion bucket = %d rows, want 1", len(rows))
	}
	res, err := svc.SearchPublishedPieces(context.Background(), "harbors", 10)
	if err != nil || len(res.Pieces) != 1 || res.Pieces[0].Kind != "opinion" {
		t.Fatalf("research search with the switch on: %v %+v", err, res)
	}
	// Switched off: the bucket is empty (not an error) and research finds none.
	svc.SetOpinionListing(false)
	if rows := opinionRows(t, svc); len(rows) != 0 {
		t.Fatalf("opinion bucket with the switch off = %d rows, want 0", len(rows))
	}
	res, err = svc.SearchPublishedPieces(context.Background(), "harbors", 10)
	if err != nil || len(res.Pieces) != 0 {
		t.Fatalf("research search with the switch off: %v %+v", err, res)
	}
	// A new version is refused too.
	if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", "doc-op", 2, "My view", "Revised."); !errors.Is(err, ErrOpinionListingOff) {
		t.Fatalf("opinion v2 with the switch off = %v", err)
	}
}

func TestOpinionPublicationIsNotAnAutopaperStory(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.SetOpinionListing(true)
	// An allowlist that does not name the writer: the Opinion bucket does not
	// depend on CORPUSD_AUTOPAPER_OWNERS.
	f.svc.SetAutopaperOwners([]string{"paper-owner"})
	if _, err := publishOpinion(f.svc, "writer-1", "writer-aa11bb", "doc-op", 1, "My view on the harbor", "I think the harbor matters."); err != nil {
		t.Fatal(err)
	}
	if f.svc.isAutopaperStory(frontPublicationMeta{PublishMode: PublishModeOpinion, OwnerID: "paper-owner"}) {
		t.Fatalf("an opinion publication of the paper's own owner is an Autopaper story")
	}
	// The news front (default and every section) never lists it.
	if titles := frontTitles(t, f.svc); hasTitle(titles, "My view on the harbor") {
		t.Fatalf("opinion piece on the news front: %v", titles)
	}
	rows := opinionRows(t, f.svc)
	if len(rows) != 1 || rows[0].Title != "My view on the harbor" || rows[0].Kind != "opinion" || rows[0].Author != "writer-aa11bb" {
		t.Fatalf("opinion bucket = %+v", rows)
	}
	if rows[0].SourceCount != nil && *rows[0].SourceCount < 0 {
		t.Fatalf("bad source count")
	}
	// Stories in the news front are marked as stories.
	page, err := f.svc.ListFrontStories(context.Background(), 0, "")
	if err != nil || len(page.Stories) == 0 || page.Stories[0].Kind != "" {
		t.Fatalf("a news story must carry no opinion kind: %v %+v", err, page)
	}
	// Research tells the two kinds apart.
	res, err := f.svc.SearchPublishedPieces(context.Background(), "harbor", 10)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, p := range res.Pieces {
		kinds[p.Title] = p.Kind
	}
	if kinds["My view on the harbor"] != "opinion" || kinds["Harbor reopens"] != "story" {
		t.Fatalf("kinds = %v", kinds)
	}
	// A later version in another mode does not turn the piece into a story.
	publishAs(t, f.svc, "writer-1", PublishModeAuto, "doc-op", 2, "My view on the harbor", "Revised.", nil)
	if titles := frontTitles(t, f.svc); hasTitle(titles, "My view on the harbor") {
		t.Fatalf("opinion piece became a story through an auto-mode version: %v", titles)
	}
}

func TestOpinionDailyLimit(t *testing.T) {
	svc, store := newOpinionService(t)
	svc.SetOpinionDailyLimit(3)
	for i := 1; i <= 3; i++ {
		if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", fmt.Sprintf("doc-%d", i), 1, fmt.Sprintf("Piece %d", i), "Words."); err != nil {
			t.Fatalf("piece %d: %v", i, err)
		}
	}
	if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", "doc-4", 1, "Piece 4", "Words."); !errors.Is(err, ErrOpinionDailyLimit) {
		t.Fatalf("fourth piece in a day = %v, want ErrOpinionDailyLimit", err)
	}
	// A new version of an existing piece is not a new listing row.
	if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", "doc-1", 2, "Piece 1", "Words revised."); err != nil {
		t.Fatalf("new version of an existing piece: %v", err)
	}
	// Another writer has their own count.
	if _, err := publishOpinion(svc, "writer-2", "writer-cc22dd", "doc-1", 1, "Other piece", "Words."); err != nil {
		t.Fatalf("another writer: %v", err)
	}
	// Auto and owner publications do not count against the limit.
	publishAs(t, svc, "writer-1", PublishModeOwner, "doc-hand", 1, "By hand", "Words.", nil)
	// A day later the count has reset: age the pieces past 24 hours.
	old := time.Now().UTC().Add(-25 * time.Hour)
	if _, err := store.corpus().Exec(`UPDATE og_objects SET created_at = ? WHERE object_kind = 'choir.publication' AND owner_id = ?`, old, "writer-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", "doc-5", 1, "Piece 5", "Words."); err != nil {
		t.Fatalf("a day later: %v", err)
	}
}

func TestOpinionAuthorLabel(t *testing.T) {
	svc, _ := newOpinionService(t)
	for _, label := range []string{"", "  ", "me@example.com", "writer@", "<script>", "a-label-that-is-far-too-long-to-be-a-short-handle-at-all"} {
		if _, err := publishOpinion(svc, "writer-1", label, "doc-x", 1, "Piece", "Words."); !errors.Is(err, ErrInvalidAuthorLabel) {
			t.Fatalf("label %q = %v, want ErrInvalidAuthorLabel", label, err)
		}
	}
	if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", "doc-x", 1, "Piece", "Words."); err != nil {
		t.Fatal(err)
	}
	// The label is fixed by the first version.
	if _, err := publishOpinion(svc, "writer-1", "writer-zz99zz", "doc-x", 2, "Piece", "Words, revised."); err != nil {
		t.Fatal(err)
	}
	rows := opinionRows(t, svc)
	if len(rows) != 1 || rows[0].Author != "writer-aa11bb" || rows[0].VersionCount != 2 {
		t.Fatalf("opinion rows = %+v, want one piece by writer-aa11bb with 2 versions", rows)
	}
	// A story or hand-published Texture carries no author label.
	publishAs(t, svc, "paper-owner", PublishModeAuto, "doc-story", 1, "A story", "Words.", nil)
	for _, row := range func() []FrontStory { p, _ := svc.ListFrontStories(context.Background(), 0, ""); return p.Stories }() {
		if row.Author != "" {
			t.Fatalf("a news story carries an author: %+v", row)
		}
	}
}

func TestOpinionModeIsOneOfTheKnownModes(t *testing.T) {
	svc, _ := newOpinionService(t)
	if _, err := svc.PublishTexture(context.Background(), PublishTextureRequest{OwnerID: "w", SourceDocID: "d", SourceRevisionID: "d-1",
		RevisionNumber: 1, Title: "T", Content: "x", PublishMode: "editorial"}); err == nil {
		t.Fatalf("an unknown publish mode was accepted")
	}
}

// A Texture first published as a plain link keeps that mode for good, so an
// opinion publish of it is refused rather than silently left unlisted.
func TestOpinionPublishOfAPlainPublicationIsRefused(t *testing.T) {
	svc, _ := newOpinionService(t)
	publishAs(t, svc, "writer-1", PublishModeOwner, "doc-plain", 1, "Plain", "Words.", nil)
	if _, err := publishOpinion(svc, "writer-1", "writer-aa11bb", "doc-plain", 2, "Plain", "Words, again."); !errors.Is(err, ErrOpinionModeConflict) {
		t.Fatalf("opinion publish of a plain publication = %v, want ErrOpinionModeConflict", err)
	}
	if rows := opinionRows(t, svc); len(rows) != 0 {
		t.Fatalf("a plain publication was listed as opinion: %+v", rows)
	}
}

// The story page labels a piece from the server's record, never from the
// link: an opinion piece resolves as kind "opinion" with its byline, an
// Autopaper story as kind "story" (so the reader shows the AI disclosure at
// /pub/texture/<slug> as it did at the retired /autopaper/<slug>, and a
// crafted link cannot hide it), and a hand-published Texture as neither.
func TestResolvedPublicationNamesOpinionKindAndAuthor(t *testing.T) {
	f := newFrontFixture(t)
	f.svc.SetOpinionListing(true)
	op, err := publishOpinion(f.svc, "writer-1", "writer-aa11bb", "doc-op", 1, "My view", "I think so.")
	if err != nil {
		t.Fatal(err)
	}
	for _, nav := range []bool{false, true} {
		var bundle *PublicationBundle
		if nav {
			bundle, err = f.svc.GetPublicationNavByRoute(context.Background(), op.RoutePath)
		} else {
			bundle, err = f.svc.GetPublicationBundleByRoute(context.Background(), op.RoutePath)
		}
		if err != nil || bundle.Publication.Kind != "opinion" || bundle.Publication.AuthorLabel != "writer-aa11bb" {
			t.Fatalf("opinion bundle (nav=%v) = %+v, %v", nav, bundle.Publication, err)
		}
	}
	story, err := f.svc.GetPublicationBundleByRoute(context.Background(), f.routes["doc-harbor"])
	if err != nil || story.Publication.Kind != "story" || story.Publication.AuthorLabel != "" {
		t.Fatalf("story bundle publication = %+v, %v", story.Publication, err)
	}
	plain := publishAs(t, f.svc, "writer-2", PublishModeOwner, "doc-hand", 1, "By hand", "My own words.", nil)
	hand, err := f.svc.GetPublicationBundleByRoute(context.Background(), plain.RoutePath)
	if err != nil || hand.Publication.Kind != "" || hand.Publication.AuthorLabel != "" {
		t.Fatalf("hand-published bundle publication = %+v, %v (a person's own Texture is not AI-written)", hand.Publication, err)
	}
}

func TestOpinionListingSwitchParsesOnlyOn(t *testing.T) {
	for raw, want := range map[string]bool{"on": true, " ON ": true, "true": true, "1": true, "": false, "off": false, "no": false, "maybe": false} {
		if got := ParseOpinionListing(raw); got != want {
			t.Fatalf("ParseOpinionListing(%q) = %v, want %v", raw, got, want)
		}
	}
	t.Setenv("CORPUSD_OPINION_LISTING", "on")
	t.Setenv("CORPUSD_OPINION_DAILY_LIMIT", "7")
	cfg, err := LoadConfig()
	if err != nil || cfg.OpinionListingRaw != "on" || cfg.OpinionDailyLimitRaw != "7" {
		t.Fatalf("config = %+v %v", cfg, err)
	}
}

func TestOpinionHTTPStatuses(t *testing.T) {
	svc, _ := newOpinionService(t)
	svc.SetOpinionDailyLimit(1)
	h := NewHandler(svc)
	post := func(doc, label string) int {
		raw, _ := json.Marshal(PublishTextureRequest{OwnerID: "writer-1", SourceDocID: doc, SourceRevisionID: doc + "-1", RevisionNumber: 1,
			Title: "T", Content: "Words.", PublishMode: PublishModeOpinion, AuthorLabel: label})
		req := httptest.NewRequest(http.MethodPost, "/internal/platform/publications/texture", bytes.NewReader(raw))
		req.Header.Set("X-Internal-Caller", "true")
		w := httptest.NewRecorder()
		h.HandleInternalPublishTexture(w, req)
		return w.Code
	}
	if got := post("doc-a", "writer-aa11bb"); got != http.StatusCreated {
		t.Fatalf("first piece = %d", got)
	}
	if got := post("doc-b", "writer-aa11bb"); got != http.StatusTooManyRequests {
		t.Fatalf("over the limit = %d, want 429", got)
	}
	if got := post("doc-c", "a@b.co"); got != http.StatusBadRequest {
		t.Fatalf("email label = %d, want 400", got)
	}
	svc.SetOpinionListing(false)
	if got := post("doc-d", "writer-aa11bb"); got != http.StatusConflict {
		t.Fatalf("switch off = %d, want 409", got)
	}
}
