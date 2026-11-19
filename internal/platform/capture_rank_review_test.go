package platform

// Findings of the review of the ranked answer (review-v4 r1), as tests first:
//   1. RankedStory.Score and .Day are never written;
//   3. one window rule for every path: published time, else fetched time, else
//      out (the exact check fell back to the object's update time, and the
//      full-text index let a page with neither time through an until-only window);
//   4. the term ordering's id tie-break and NULLS LAST are never exercised;
//   8. the citation cap is applied before eligibility, so a story whose first
//      cited ids are ineligible contributes nothing;
//   9. the note says "did not answer within 4s" when the caller cancelled.

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestRankedStoryCarriesTheScoreAndDayOfItsHit(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "cited", "plain", 99, now)
	f.story(t, "doc-a", 1, "Harbor story", "The harbor story vec04 is told here.", time.Time{})
	f.pass(t)
	_, version := f.versionOf(t, "Harbor story", 1)
	f.cite(t, version, f.id(t, "cited"))
	resp := f.rank(t, "harbor")
	if len(resp.Ranked.Stories) != 1 {
		t.Fatalf("stories %+v", resp.Ranked.Stories)
	}
	st := resp.Ranked.Stories[0]
	if st.Score < 0.95 || st.Score > 0.97 || st.Day != time.Now().UTC().Format("2006-01-02") {
		t.Fatalf("score %v day %q: want the hit's cosine 0.96 and today", st.Score, st.Day)
	}
}

// A valid capture always has a fetched time (objectgraph requires it), so a
// capture with neither time is a projection row whose times are NULL, or a
// metadata the exact check cannot read. The same rule must hold on every path.
func TestWindowRuleIsOneRuleForACaptureWithNeitherTime(t *testing.T) {
	// The exact check: published, else fetched, else none.
	mk := func(published, fetched string) objectgraph.Object {
		meta, _ := json.Marshal(objectgraph.WebCaptureMetadata{SchemaVersion: objectgraph.WebCaptureSchemaVersion, URL: "https://example.com/x", CanonicalURL: "https://example.com/x",
			Title: "x", PublishedAt: published, FetchedAt: fetched, Extractor: "x1", DocHash: "h",
			ContentBlobID: "sha256:" + strings.Repeat("a", 64), ExtractedTextBlobID: "sha256:" + strings.Repeat("a", 64), BlocksBlobID: "sha256:" + strings.Repeat("a", 64)})
		return objectgraph.Object{ObjectKind: objectgraph.WebCaptureObjectKind, Metadata: meta, UpdatedAt: time.Now()}
	}
	if at, ok := captureWindowTime(mk("2026-11-01T00:00:00Z", "2026-11-09T00:00:00Z")); !ok || at.Day() != 1 {
		t.Fatalf("published first: %v %v", at, ok)
	}
	if at, ok := captureWindowTime(mk("not a time", "2026-11-09T00:00:00Z")); !ok || at.Day() != 9 {
		t.Fatalf("fetched when published is unreadable: %v %v", at, ok)
	}
	if _, ok := captureWindowTime(objectgraph.Object{ObjectKind: objectgraph.WebCaptureObjectKind, Metadata: json.RawMessage(`{"schema_version":"x"}`), UpdatedAt: time.Now()}); ok {
		t.Fatal("an object with no readable time got one (its update time must not stand in)")
	}
	// The term half: a row whose times are NULL is outside every window.
	f, objs, base := broadFixture(t, 40)
	if _, err := f.store.corpus().Exec(`UPDATE proj_capture_index SET published_at = NULL, fetched_at = NULL WHERE capture_id = ?`, objs[0].CanonicalID); err != nil {
		t.Fatal(err)
	}
	for name, mod := range map[string]func(*CaptureSearchRequest){
		"until only": func(r *CaptureSearchRequest) { r.Until = base.Add(400 * time.Hour).Format(time.RFC3339) },
		"since only": func(r *CaptureSearchRequest) { r.Since = base.Add(-400 * time.Hour).Format(time.RFC3339) },
		"since + until": func(r *CaptureSearchRequest) {
			r.Since, r.Until = base.Add(-400*time.Hour).Format(time.RFC3339), base.Add(400*time.Hour).Format(time.RFC3339)
		},
	} {
		req := CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"harbor"}, Need: 1, Candidates: 100}
		mod(&req)
		resp, err := f.store.SearchCaptures(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Matched != 39 {
			t.Fatalf("%s: matched %d, want 39 (the capture with no time is out)", name, resp.Matched)
		}
		for _, o := range resp.Objects {
			if o.CanonicalID == objs[0].CanonicalID {
				t.Fatalf("%s: the term half holds a capture with neither time", name)
			}
		}
	}
}

func TestTermSearchOrderTieBreaksOnIdDescendingAndPutsNoFetchedTimeLast(t *testing.T) {
	f := newFTFixture(t, false)
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)
	var objs []objectgraph.Object
	for i := 0; i < 3; i++ { // three captures share one fetched time
		objs = append(objs, f.capture(t, "owner-a", "computer-a", fmt.Sprintf("tie%d", i), "Tidal gauge", "https://example.com/t", "x", base))
	}
	newer := f.capture(t, "owner-a", "computer-a", "newer", "Tidal gauge", "https://example.com/n", "x", base.Add(time.Hour))
	none := f.capture(t, "owner-a", "computer-a", "none", "Tidal gauge", "https://example.com/z", "x", base.Add(2*time.Hour))
	f.put(t, append(objs, newer, none)...)
	if err := f.store.ReconcileCaptureIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The capture with no fetched time: its term rows hold NULL.
	if _, err := f.store.corpus().Exec(`UPDATE proj_capture_terms SET fetched_at = NULL WHERE capture_id = ?`, none.CanonicalID); err != nil {
		t.Fatal(err)
	}
	resp, err := f.store.SearchCaptures(context.Background(), CaptureSearchRequest{OwnerID: "owner-a", ComputerID: "computer-a", Words: []string{"tidal"}, Need: 1, Candidates: 10})
	if err != nil {
		t.Fatal(err)
	}
	var tiedIDs []string
	for _, o := range objs {
		tiedIDs = append(tiedIDs, o.CanonicalID)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(tiedIDs)))
	want := append(append([]string{newer.CanonicalID}, tiedIDs...), none.CanonicalID)
	var got []string
	for _, o := range resp.Objects {
		got = append(got, o.CanonicalID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order\n got  %v\n want %v (newest, then the tie by id descending, then no time last)", got, want)
	}
}

func TestRankedAnswerCapsAStorysCitationsAfterEligibility(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	var keys []string
	for i := 0; i < 12; i++ {
		keys = append(keys, fmt.Sprintf("c%02d", i))
		f.page(t, keys[i], "a cited page", 99, now)
	}
	f.story(t, "doc-a", 1, "Harbor story", "The harbor story vec04 is told here.", time.Time{})
	f.pass(t)
	_, version := f.versionOf(t, "Harbor story", 1)
	ids := map[string]string{}
	var order []string
	for _, k := range keys {
		ids[f.id(t, k)] = k
		order = append(order, f.id(t, k))
	}
	sort.Strings(order)
	f.cite(t, version, order...)
	// The first eight cited ids in id order are withdrawn; the last four are public.
	for _, id := range order[:8] {
		if _, err := f.store.corpus().Exec(`UPDATE og_objects SET tombstone = TRUE WHERE canonical_id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
	resp := f.rank(t, "harbor")
	if len(resp.Ranked.Stories) != 1 || strings.Join(resp.Ranked.Stories[0].Captures, ",") != strings.Join(order[8:], ",") {
		t.Fatalf("stories %+v; want the four eligible citations %v", resp.Ranked.Stories, order[8:])
	}
	n := 0
	for _, e := range resp.Ranked.Entries {
		if containsStr(e.Reasons, "story") {
			n++
		}
	}
	if n != 4 {
		t.Fatalf("%d pages reached through the story, want 4", n)
	}
}

func TestVectorHalfNoteSaysWhyItEnded(t *testing.T) {
	f := newRankFixture(t)
	f.disagree(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, why := f.store.searchVectorsBounded(ctx, VectorSearchRequest{Query: "zorblax", Owner: f.owner, Computer: f.computer})
	if why == "" || strings.Contains(why, "within") {
		t.Fatalf("a cancelled caller was told %q", why)
	}
	if !strings.Contains(why, "cancel") {
		t.Fatalf("the reason %q does not say the caller cancelled", why)
	}
	parent, stop := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer stop()
	f.emb.Override = func(in string) []float32 {
		if strings.Contains(in, "query: ") {
			time.Sleep(300 * time.Millisecond)
		}
		return markerVector(in)
	}
	_, why = f.store.searchVectorsBounded(parent, VectorSearchRequest{Query: "zorblax", Owner: f.owner, Computer: f.computer})
	if strings.Contains(why, "within 4s") {
		t.Fatalf("the caller's own deadline was reported as the vector half's 4 s: %q", why)
	}
}
