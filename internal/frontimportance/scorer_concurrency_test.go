package frontimportance

import (
	"context"
	"os"
	"strings"
	"fmt"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway"
)

// Failure mode: scoring one story at a time (80 calls took about 9 s), or a
// pool that drops a story's row. The scorer asks stories concurrently within
// scorerConcurrency, and every story still gets its own row.

type slowJev struct {
	inner     *fakeJev
	cur, peak atomic.Int64
	delay     time.Duration
}

func (s *slowJev) Decide(ctx context.Context, id string, req gateway.JudgmentRequest) ([]byte, error) {
	n := s.cur.Add(1)
	for p := s.peak.Load(); n > p && !s.peak.CompareAndSwap(p, n); p = s.peak.Load() {
	}
	time.Sleep(s.delay)
	s.cur.Add(-1)
	return s.inner.Decide(ctx, id, req)
}

func TestScorerAsksStoriesConcurrently(t *testing.T) {
	var stories []frontStory
	for i := 0; i < 30; i++ {
		stories = append(stories, frontStory{PublicationID: fmt.Sprintf("pub-%d", i), Title: fmt.Sprintf("T%d", i), LatestRevision: 1})
	}
	srv := frontServer(t, http.StatusOK, stories)
	path := filepath.Join(t.TempDir(), "scores.json")
	jev := &slowJev{inner: &fakeJev{}, delay: 10 * time.Millisecond}
	stats, err := run(context.Background(), testConfig(srv.URL, path), http.DefaultClient, jev, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Scored+stats.Failed != 30 {
		t.Fatalf("stats = %+v", stats)
	}
	if peak := jev.peak.Load(); peak < 4 || peak > scorerConcurrency {
		t.Fatalf("peak concurrency = %d, want 4..%d", peak, scorerConcurrency)
	}
}

// Failure mode: a failed call at the scorer left no record, so a calibration
// read saw only the calls that worked. Every call is recorded, with the
// score it fed, beside the scores file.
func TestScorerRecordsEveryCallFailuresIncluded(t *testing.T) {
	stories := []frontStory{
		{PublicationID: "pub-ok", Title: "Fine", LatestRevision: 3},
		{PublicationID: "pub-bad", Title: "Bad", LatestRevision: 1},
	}
	srv := frontServer(t, http.StatusOK, stories)
	dir := t.TempDir()
	path := filepath.Join(dir, "scores.json")
	jev := &fakeJev{answers: map[string]string{"Fine": "7", "Bad": ""}}
	if _, err := run(context.Background(), testConfig(srv.URL, path), http.DefaultClient, jev, time.Now); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "jev-calls.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if got := strings.Count(text, "\n"); got != 2 {
		t.Fatalf("records = %d, want 2:\n%s", got, text)
	}
	for _, want := range []string{`"decision_id":"pub-ok@3"`, `"decision_id":"pub-bad@1"`, `"ok":false`, `"ok":true`, `"site":"front_importance"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("records lack %s:\n%s", want, text)
		}
	}
}
