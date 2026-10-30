package frontimportance

// Ways carrying score rows past the window can fail (see
// docs/archive/problems/autopaper-section-pages-empty-2026-10-15.md):
//
//  1. A story that left the newest 100 loses its row (and its section).
//  2. Carried rows never expire (the file grows without bound), or expire
//     at the wrong age / ignore the configured retention.
//  3. A row with no readable scored_at is carried forever.
//  4. A carried row overwrites, or causes a rescore of, a window story.

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestScorerCarriesForwardRowsOutsideTheWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	base := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	ts := func(age time.Duration) string { return base.Add(-age).Format(time.RFC3339) }
	if err := writeScores(path, scoresFile{Version: 1, Scores: map[string]scoreRow{
		"pub-fresh":  {Revision: 1, Importance: 6, Section: "us_politics", Criteria: scoringCriteria, ScoredAt: ts(2 * time.Hour)},
		"pub-aging":  {Revision: 1, Importance: 5, Section: "world", Criteria: scoringCriteria, ScoredAt: ts(13 * 24 * time.Hour)},
		"pub-stale":  {Revision: 1, Importance: 9, Section: "world", Criteria: scoringCriteria, ScoredAt: ts(15 * 24 * time.Hour)},
		"pub-nodate": {Revision: 1, Importance: 9, Section: "world", Criteria: scoringCriteria},
		"pub-in":     {Revision: 1, Importance: 7, Section: "business", Criteria: scoringCriteria, ScoredAt: ts(time.Hour)},
	}}); err != nil {
		t.Fatal(err)
	}
	// Run 2's window holds only pub-in (kept) and pub-new (scored).
	stories := []frontStory{
		{PublicationID: "pub-in", Title: "In", LatestRevision: 1},
		{PublicationID: "pub-new", Title: "New", LatestRevision: 1},
	}
	jev := &fakeJev{answers: map[string]string{"New": "4"}}
	srv := frontServer(t, http.StatusOK, stories)
	cfg := testConfig(srv.URL, path)
	clock := func() time.Time { return base }
	if _, err := run(context.Background(), cfg, srv.Client(), jev, clock); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := readScores(path).Scores
	if r, ok := got["pub-fresh"]; !ok || r.Section != "us_politics" || r.Importance != 6 {
		t.Fatalf("pub-fresh (outside the window, within retention) = %+v ok=%v, want carried", r, ok)
	}
	if _, ok := got["pub-aging"]; !ok {
		t.Fatalf("pub-aging (13 days, within the 14-day default) was dropped")
	}
	for _, id := range []string{"pub-stale", "pub-nodate"} {
		if _, ok := got[id]; ok {
			t.Fatalf("%s past retention or unbounded was carried", id)
		}
	}
	if r := got["pub-in"]; r.Section != "business" || r.ScoredAt != ts(time.Hour) {
		t.Fatalf("pub-in (in the window, kept) changed: %+v", r)
	}
	if _, ok := got["pub-new"]; !ok || len(got) != 4 {
		t.Fatalf("scores = %+v, want pub-fresh, pub-aging, pub-in, pub-new", got)
	}
	for _, title := range jev.calls {
		if title != "New" {
			t.Fatalf("carried or kept row was rescored: %v", jev.calls)
		}
	}

	// A shorter configured retention drops the 13-day row too.
	cfg.Retention = 24 * time.Hour
	if _, err := run(context.Background(), cfg, srv.Client(), jev, clock); err != nil {
		t.Fatalf("run 3: %v", err)
	}
	got = readScores(path).Scores
	if _, ok := got["pub-aging"]; ok {
		t.Fatalf("pub-aging survived a 24h retention")
	}
	if _, ok := got["pub-fresh"]; !ok {
		t.Fatalf("pub-fresh (2h) dropped under a 24h retention")
	}
}
