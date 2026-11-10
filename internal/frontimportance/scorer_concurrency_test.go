package frontimportance

import (
	"context"
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
