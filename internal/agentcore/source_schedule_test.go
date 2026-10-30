package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The Autopaper source schedule (W1c,
// docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md).
// Failure modes pinned, written before the code:
//  1. Restarts: see source_schedule_restart_test.go (a standing schedule
//     survives a crash; only the poll in flight closes).
//  2. Two chains for one source both poll: a stale generation still runs.
//  3. A stopped schedule keeps polling.
//  4. A failing feed retries at the plain interval instead of backing off.
//  5. A poll records no usage, or usage grows without bound.
//  6. A poll's captures do not reach the shared capture sink.

type recordingSink struct {
	mu    sync.Mutex
	items int
}

func (s *recordingSink) WriteCaptures(_ context.Context, _, _ string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items += len(items)
	return sourcegraph.CaptureWriteResult{New: len(items)}, nil
}

func (s *recordingSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}

func (s *recordingSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}

type armedWake struct {
	mailbox, kind, content string
	notBefore              time.Time
}

func sourceScheduleFixture(t *testing.T, sourceID string, failing *atomic.Bool) (*Runtime, *recordingSink, *[]armedWake, *atomic.Int32) {
	t.Helper()
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(prev) })
	var hits atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if failing != nil && failing.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Feed</title><item><title>Story %d</title><link>https://news.example/%d</link><description>Body</description><guid>g-%d</guid></item></channel></rss>`, hits.Load(), hits.Load(), hits.Load())
	}))
	t.Cleanup(feed.Close)

	rt, _ := testRuntime(t)
	rt.cfg.ComputerID = "computer-paper"
	sink := &recordingSink{}
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: sourceID, Type: sources.SourceTypeRSS, Name: "Feed", URL: feed.URL}}}
	rt.researchDeps = &researchtools.Dependencies{Sources: reg, Captures: sink}
	var wakes []armedWake
	rt.SetScheduleActor(func(_ context.Context, _, _, toAgentID, kind, content, _, _ string, notBefore time.Time) error {
		wakes = append(wakes, armedWake{mailbox: toAgentID, kind: kind, content: content, notBefore: notBefore})
		return nil
	})
	return rt, sink, &wakes, &hits
}

func TestSourceSchedulePollsRecordsUsageAndRearms(t *testing.T) {
	ctx := context.Background()
	rt, sink, wakes, hits := sourceScheduleFixture(t, "rss:sched-a", nil)

	sched, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:sched-a", 10*time.Minute, 5)
	if err != nil {
		t.Fatal(err)
	}
	if sched.State != store.SourceScheduleActive || len(*wakes) != 1 || (*wakes)[0].kind != sourcePollDueKind || (*wakes)[0].mailbox != "source:rss:sched-a" {
		t.Fatalf("start = %+v, wakes %+v", sched, *wakes)
	}
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:unknown", 0, 0); err == nil {
		t.Fatal("an unknown source was scheduled")
	}

	first := (*wakes)[0]
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", first.content); err != nil {
		t.Fatal(err)
	}
	// The actor commit that carried the successor fails, so the successor is
	// lost and the first wake is retried. The retry does not poll again; it
	// re-arms the saved next poll, the same wake the failed commit dropped
	// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
	successor := (*wakes)[1]
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", first.content); err != nil {
		t.Fatal(err)
	}
	if len(*wakes) != 3 || (*wakes)[2].content != successor.content || !(*wakes)[2].notBefore.Equal(successor.notBefore) {
		t.Fatalf("a retried wake did not re-arm the saved next poll: %+v", *wakes)
	}
	*wakes = (*wakes)[:2]
	got, err := rt.store.GetSourceSchedule(ctx, "owner-1", "rss:sched-a")
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 || sink.items != 1 {
		t.Fatalf("poll reached the feed %d times and the sink with %d items, want 1 and 1", hits.Load(), sink.items)
	}
	if got.Totals.Polls != 1 || len(got.Recent) != 1 || got.Recent[0].Fetched != 1 || got.Recent[0].New != 1 || got.Recent[0].Error != "" {
		t.Fatalf("usage after one poll = %+v", got)
	}
	if len(*wakes) != 2 {
		t.Fatalf("next wake not armed: %+v", *wakes)
	}
	if next := (*wakes)[1].notBefore.Sub(got.Recent[0].At); next < 9*time.Minute || next > 11*time.Minute {
		t.Fatalf("next poll in %s, want the 10 minute interval", next)
	}

	// A restart of the schedule makes the earlier chain stale (failure 2).
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:sched-a", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", (*wakes)[1].content); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("a stale wake polled the feed")
	}

	// A stopped schedule does nothing (failure 3).
	pending := (*wakes)[len(*wakes)-1]
	if _, err := rt.StopSourceSchedule(ctx, "owner-1", "rss:sched-a", "test"); err != nil {
		t.Fatal(err)
	}
	armed := len(*wakes)
	if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", pending.content); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 || len(*wakes) != armed {
		t.Fatalf("a stopped schedule polled (%d hits) or armed a wake", hits.Load())
	}
}

func TestSourceScheduleBacksOffAFailingFeed(t *testing.T) {
	ctx := context.Background()
	var failing atomic.Bool
	failing.Store(true)
	rt, _, wakes, _ := sourceScheduleFixture(t, "rss:sched-b", &failing)
	if _, err := rt.StartSourceSchedule(ctx, "owner-1", "computer-paper", "rss:sched-b", 10*time.Minute, 5); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := rt.HandleSourcePollDue(ctx, "owner-1", "computer-paper", (*wakes)[len(*wakes)-1].content); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := rt.store.GetSourceSchedule(ctx, "owner-1", "rss:sched-b")
	if got.ConsecutiveFailures != 2 || got.Totals.Errors != 2 || got.Recent[1].Error == "" {
		t.Fatalf("after two failures = %+v", got)
	}
	if next := (*wakes)[len(*wakes)-1].notBefore.Sub(got.Recent[1].At); next < 39*time.Minute || next > 41*time.Minute {
		t.Fatalf("after two failures the next poll is in %s, want 40 minutes", next)
	}
}

func TestSourceScheduleUsageIsBounded(t *testing.T) {
	var sched store.SourceSchedule
	for i := 0; i < store.SourcePollUsageLimit+6; i++ {
		sched.RecordPoll(store.SourcePollUsage{Fetched: i})
	}
	if len(sched.Recent) != store.SourcePollUsageLimit || sched.Totals.Polls != store.SourcePollUsageLimit+6 || sched.Recent[0].Fetched != 6 {
		t.Fatalf("usage = %d recent, %d polls, oldest fetched %d", len(sched.Recent), sched.Totals.Polls, sched.Recent[0].Fetched)
	}
}

// An active schedule whose next poll is long past has no live wake (its
// re-arm kept failing), and the list must say so; a stopped or
// interrupted schedule, or one just due, is not overdue.
func TestSourceScheduleListMarksADeadScheduleOverdue(t *testing.T) {
	now := time.Date(2026, 10, 11, 14, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		sched store.SourceSchedule
		want  bool
	}{
		{"active, long past due", store.SourceSchedule{State: store.SourceScheduleActive, NextDue: now.Add(-time.Hour)}, true},
		{"active, just due", store.SourceSchedule{State: store.SourceScheduleActive, NextDue: now.Add(-time.Minute)}, false},
		{"active, due later", store.SourceSchedule{State: store.SourceScheduleActive, NextDue: now.Add(time.Hour)}, false},
		{"stopped", store.SourceSchedule{State: store.SourceScheduleStopped, NextDue: now.Add(-time.Hour)}, false},
		{"interrupted", store.SourceSchedule{State: "interrupted_by_restart"}, false},
	}
	for _, c := range cases {
		if got := sourceScheduleOverdue(c.sched, now); got != c.want {
			t.Errorf("%s: overdue = %v, want %v", c.name, got, c.want)
		}
	}
}
