package frontimportance

// Ways the per-run depth summary (the deepen measure, written before the
// code; docs/archive/reports/autopaper-deepen-important-stories-2026-10-14.md) can
// fail:
//
//  1. A story whose depth corpusd did not report counts as thin (or as
//     deep), skewing the measure while corpusd rolls out.
//  2. A promo story counts among the important ones.
//  3. Importance is read from the previous file instead of the scores this
//     run writes, so a rescored story lands in the wrong set.
//  4. The median is wrong for an even count, or the summary divides by
//     zero with no important stories.
//  5. The run fails or writes a different scores file because of the
//     summary (it must only log).

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func intp(n int) *int { return &n }

func TestDepthSummaryCountsImportantStoriesWithKnownDepth(t *testing.T) {
	stories := []frontStory{
		{PublicationID: "pub-stub", SourceCount: intp(1)},
		{PublicationID: "pub-two", SourceCount: intp(2)},
		{PublicationID: "pub-deep", SourceCount: intp(5)},
		{PublicationID: "pub-deeper", SourceCount: intp(19)},
		{PublicationID: "pub-unknown"},                        // mode 1: skipped
		{PublicationID: "pub-promo", SourceCount: intp(1)},    // mode 2: skipped
		{PublicationID: "pub-minor", SourceCount: intp(1)},    // below 7: skipped
		{PublicationID: "pub-unscored", SourceCount: intp(1)}, // no score: skipped
	}
	scores := map[string]scoreRow{
		"pub-stub":    {Importance: 8},
		"pub-two":     {Importance: 7},
		"pub-deep":    {Importance: 9},
		"pub-deeper":  {Importance: 7},
		"pub-unknown": {Importance: 10},
		"pub-promo":   {Importance: 8, Promo: true},
		"pub-minor":   {Importance: 6},
	}
	got := summarizeDepth(stories, scores)
	if got.Important != 4 || got.Deep != 2 {
		t.Fatalf("important/deep = %d/%d, want 4/2", got.Important, got.Deep)
	}
	if got.MedianSources != 3.5 {
		t.Fatalf("median = %v, want 3.5 (1, 2, 5, 19)", got.MedianSources)
	}
	if len(got.Thin) != 2 || got.Thin[0] != "pub-stub" || got.Thin[1] != "pub-two" {
		t.Fatalf("thin = %v, want [pub-stub pub-two]", got.Thin)
	}
	empty := summarizeDepth(nil, nil)
	if empty.Important != 0 || empty.MedianSources != 0 || len(empty.Thin) != 0 {
		t.Fatalf("empty summary = %+v", empty)
	}
}

func TestRunReportsDepthFromThisRunsScores(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scores.json")
	// Previously scored 3; this run rescores the new revision to 8 (mode 3).
	if err := writeScores(path, scoresFile{Version: 1, Scores: map[string]scoreRow{
		"pub-a": {Revision: 1, Importance: 3, Section: "world"},
	}}); err != nil {
		t.Fatal(err)
	}
	stories := []frontStory{{PublicationID: "pub-a", Title: "A", LatestRevision: 2, SourceCount: intp(1)}}
	srv := frontServer(t, http.StatusOK, stories)
	stats, err := run(context.Background(), testConfig(srv.URL, path), srv.Client(), &fakeJev{answers: map[string]string{"A": "8"}}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Depth.Important != 1 || len(stats.Depth.Thin) != 1 {
		t.Fatalf("depth = %+v, want the rescored story counted as important and thin", stats.Depth)
	}
	if got := readScores(path).Scores["pub-a"]; got.Importance != 8 || got.Revision != 2 {
		t.Fatalf("scores file = %+v, want the rescore written (mode 5)", got)
	}
}
