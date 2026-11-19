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

// capturePlain puts a public capture with neither a published nor a fetched time.
func (f *rankFixture) captureWithNoTime(t *testing.T, key, text string) objectgraph.Object {
	t.Helper()
	obj := f.capture(t, f.owner, f.computer, key, "Page "+key, text, time.Now().UTC(), false)
	var meta map[string]any
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	meta["fetched_at"], meta["published_at"] = "", ""
	obj.Metadata, _ = json.Marshal(meta)
	obj.ContentHash += "-notime"
	if err := f.og.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestWindowRuleIsOneRuleForACaptureWithNeitherTime(t *testing.T) {
	f := newRankFixture(t)
	now := time.Now().UTC().Add(-time.Hour)
	f.page(t, "dated", "zorblax harbor", 2, now)
	notime := f.captureWithNoTime(t, "notime", "zorblax harbor vec01")
	f.pass(t)
	until := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	since := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	// Without a window the capture is found: the control.
	if got := f.keys(t, entryIDs(f.rank(t, "zorblax harbor")), "dated", "notime"); !strings.Contains(got, "notime") {
		t.Fatalf("control: %s", got)
	}
	for name, mod := range map[string]func(*CaptureSearchRequest){
		"until only":    func(r *CaptureSearchRequest) { r.Until = until },
		"since only":    func(r *CaptureSearchRequest) { r.Since = since },
		"since + until": func(r *CaptureSearchRequest) { r.Since, r.Until = since, until },
	} {
		resp := f.rank(t, "zorblax harbor", mod)
		for _, id := range entryIDs(resp) {
			if id == notime.CanonicalID {
				t.Fatalf("%s: the ranked answer holds a capture with neither time", name)
			}
		}
		for _, h := range resp.Hits {
			if h.CaptureID == notime.CanonicalID {
				t.Fatalf("%s: the page-text half holds a capture with neither time", name)
			}
		}
		for _, o := range resp.Objects {
			if o.CanonicalID == notime.CanonicalID {
				t.Fatalf("%s: the term half holds a capture with neither time", name)
			}
		}
		if !strings.Contains(f.keys(t, entryIDs(resp), "dated"), "dated") {
			t.Fatalf("%s: the dated capture is missing", name)
		}
	}
}

func TestTermSearchOrderTieBreaksOnIdDescendingAndPutsNoFetchedTimeLast(t *testing.T) {
	f := newFTFixture(t, false)
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)
	var objs []objectgraph.Object
	tied := base // three captures share one fetched time
	for i := 0; i < 3; i++ {
		objs = append(objs, f.capture(t, "owner-a", "computer-a", fmt.Sprintf("tie%d", i), "Tidal gauge", "https://example.com/t", "x", tied))
	}
	newer := f.capture(t, "owner-a", "computer-a", "newer", "Tidal gauge", "https://example.com/n", "x", base.Add(time.Hour))
	none := f.captureWithoutTimes(t, "none")
	f.put(t, append(objs, newer, none)...)
	if err := f.store.ReconcileCaptureIndex(context.Background()); err != nil {
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

func (f *ftFixture) captureWithoutTimes(t *testing.T, key string) objectgraph.Object {
	t.Helper()
	obj := f.capture(t, "owner-a", "computer-a", key, "Tidal gauge", "https://example.com/"+key, "x", time.Now().UTC())
	var meta map[string]any
	_ = json.Unmarshal(obj.Metadata, &meta)
	meta["fetched_at"], meta["published_at"] = "", ""
	obj.Metadata, _ = json.Marshal(meta)
	return obj
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
