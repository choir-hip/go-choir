package frontimportance

// Ways reading more than one front-list page can fail (the -pages backfill
// for docs/archive/problems/autopaper-section-pages-empty-2026-10-15.md):
//
//  1. The default (pages 1) follows next_cursor, changing today's behaviour.
//  2. -pages N does not follow next_cursor, or sends it unescaped / drops the
//     existing query (limit=100).
//  3. It reads past N pages, or past the last page (empty next_cursor).
//  4. A story on page 2 is not scored, or a later page failing discards the
//     stories already read.
//  5. Rows for stories on later pages are dropped as if outside the window.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// pagedFrontServer serves pages[i] for cursor "c<i>" (the first page for no
// cursor); every page but the last carries next_cursor "c<i+1>". A page in
// failPages answers 500. It records each request's raw query.
func pagedFrontServer(t *testing.T, pages [][]frontStory, failPages ...int) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.RawQuery)
		mu.Unlock()
		idx := 0
		if c := r.URL.Query().Get("cursor"); c != "" {
			idx = int(c[1] - '0')
		}
		for _, f := range failPages {
			if f == idx {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		body := map[string]any{"stories": pages[idx]}
		if idx+1 < len(pages) {
			body["next_cursor"] = "c" + string(rune('0'+idx+1))
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &queries
}

func TestScorerFollowsPagesOnlyWhenAsked(t *testing.T) {
	pages := [][]frontStory{
		{{PublicationID: "pub-1", Title: "One", LatestRevision: 1}},
		{{PublicationID: "pub-2", Title: "Two", LatestRevision: 1}},
		{{PublicationID: "pub-3", Title: "Three", LatestRevision: 1}},
	}
	answers := map[string]string{"One": "5", "Two": "6", "Three": "7"}
	run1 := func(pagesFlag int) (map[string]scoreRow, []string) {
		srv, queries := pagedFrontServer(t, pages)
		path := filepath.Join(t.TempDir(), "scores.json")
		cfg := testConfig(srv.URL+"?limit=100", path)
		cfg.Pages = pagesFlag
		if _, err := run(context.Background(), cfg, srv.Client(), &fakeJev{answers: answers}, time.Now); err != nil {
			t.Fatalf("run pages=%d: %v", pagesFlag, err)
		}
		return readScores(path).Scores, *queries
	}

	// Default: one page, today's behaviour.
	got, queries := run1(0)
	if len(queries) != 1 || queries[0] != "limit=100" || len(got) != 1 || got["pub-1"].Importance != 5 {
		t.Fatalf("default read queries=%v scores=%+v, want one page", queries, got)
	}
	// -pages 2: the second page is read through next_cursor, the third not.
	got, queries = run1(2)
	if len(queries) != 2 || queries[1] != "limit=100&cursor=c1" {
		t.Fatalf("pages=2 queries = %v", queries)
	}
	if len(got) != 2 || got["pub-1"].Importance != 5 || got["pub-2"].Importance != 6 {
		t.Fatalf("pages=2 scores = %+v, want pub-1 and pub-2", got)
	}
	// -pages 9 stops at the last page (no next_cursor).
	got, queries = run1(9)
	if len(queries) != 3 || len(got) != 3 {
		t.Fatalf("pages=9 queries = %v scores = %+v, want 3 pages", queries, got)
	}
}

func TestScorerKeepsStoriesReadWhenALaterPageFails(t *testing.T) {
	pages := [][]frontStory{
		{{PublicationID: "pub-1", Title: "One", LatestRevision: 1}},
		{{PublicationID: "pub-2", Title: "Two", LatestRevision: 1}},
	}
	srv, _ := pagedFrontServer(t, pages, 1)
	path := filepath.Join(t.TempDir(), "scores.json")
	cfg := testConfig(srv.URL, path)
	cfg.Pages = 3
	if _, err := run(context.Background(), cfg, srv.Client(), &fakeJev{answers: map[string]string{"One": "5"}}, time.Now); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := readScores(path).Scores; len(got) != 1 || got["pub-1"].Importance != 5 {
		t.Fatalf("scores = %+v, want pub-1 kept despite page 2 failing", got)
	}
}
