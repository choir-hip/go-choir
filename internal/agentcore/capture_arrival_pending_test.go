package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// L2a A5, defect 2 (docs/problems/autopaper-intake-defects-2026-10-14.md):
// a failed arrival append lost the announcement for good. Ways the pending
// list can fail, written before the code:
//   - the captures are written, the append fails, and the next poll counts
//     them as repeats and never names them (the defect);
//   - the pending list lives only in memory, so a planned restart loses it;
//   - the next arrival names pending captures after the new ones, or in a
//     different order each time, so it is not oldest first;
//   - the pending list grows without end while appends keep failing, or one
//     arrival names more than the poll's own limit;
//   - a pending capture is cleared before an arrival naming it is appended,
//     or never cleared after, so it is announced twice or not at all;
//   - a retried arrival naming the same captures is minted with different
//     content (a language field, an order), so its digest changes and the
//     replay is refused as a conflicting command, wedging the list;
//   - a crash boot silently resumes the pending list instead of closing it
//     with a visible fate (CLAUDE.md "Restarts End Work");
//   - the arrival item lacks the language and script, or management's view
//     of the record hides them.

// languageSink captures each URL once and reports what sourcegraph would:
// the item's language and a script guess.
type languageSink struct {
	mu   sync.Mutex
	seen map[string]string
}

func (s *languageSink) WriteCaptures(_ context.Context, _, _ string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out sourcegraph.CaptureWriteResult
	for _, item := range items {
		if _, ok := s.seen[item.URL]; ok {
			out.Repeated++
			continue
		}
		id := fmt.Sprintf("choir.web_capture:pending-%d", len(s.seen)+1)
		s.seen[item.URL] = id
		out.New++
		out.Captured = append(out.Captured, sourcegraph.CapturedItem{CaptureID: id, Title: item.Title, URL: item.URL,
			Language: item.Language, Script: sources.GuessScript(item.Title)})
	}
	return out, nil
}

func (s *languageSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}

func (s *languageSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}

func TestFailedArrivalLeavesCapturesPendingAndTheNextPollAnnouncesThemFirst(t *testing.T) {
	ctx := context.Background()
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(prev) })
	// Each poll the feed has two more items, oldest first.
	var hits atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(hits.Add(1)) * 2
		var items strings.Builder
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&items, `<item><title>Item %d</title><link>https://pending.example/%d</link><description>Body %d</description><guid>p-%d</guid></item>`, i, i, i, i)
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Pending</title>%s</channel></rss>`, items.String())
	}))
	t.Cleanup(feed.Close)

	rt, s := testRuntime(t)
	owner, computer, sourceID := "owner-pending", rt.cfg.ComputerID, "rss:pending-arrivals"
	management := agentprofile.Management + ":" + owner
	sink := &languageSink{seen: map[string]string{}}
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: sourceID, Type: sources.SourceTypeRSS, Name: "Pending", URL: feed.URL, Languages: []string{"en"}}}}
	rt.researchDeps = &researchtools.Dependencies{Sources: reg, Captures: sink}
	var wakes []armedWake
	rt.SetScheduleActor(func(_ context.Context, _, _, toAgentID, kind, content, _, _ string, notBefore time.Time) error {
		wakes = append(wakes, armedWake{mailbox: toAgentID, kind: kind, content: content, notBefore: notBefore})
		return nil
	})
	var failAppend atomic.Bool
	failAppend.Store(true)
	rt.commitCaptureArrival = func(ctx context.Context, req types.CommitInboxRecordsRequest) error {
		if failAppend.Load() {
			return fmt.Errorf("store unavailable")
		}
		_, err := rt.store.CommitInboxRecords(ctx, req)
		return err
	}
	// The poll limit is 3.
	if _, err := rt.StartSourceSchedule(ctx, owner, computer, sourceID, 10*time.Minute, 3); err != nil {
		t.Fatal(err)
	}
	pollNext := func() store.SourceSchedule {
		t.Helper()
		if err := rt.HandleSourcePollDue(ctx, owner, computer, wakes[len(wakes)-1].content); err != nil {
			t.Fatal(err)
		}
		sched, err := s.GetSourceSchedule(ctx, owner, sourceID)
		if err != nil {
			t.Fatal(err)
		}
		return sched
	}

	// Poll 1 writes items 1-2; the append fails, so both stay pending.
	sched := pollNext()
	if got := arrivalIDs(sched.PendingArrivals); strings.Join(got, ",") != sink.seen["https://pending.example/1"]+","+sink.seen["https://pending.example/2"] {
		t.Fatalf("pending after a failed append = %v; want items 1 and 2, oldest first", got)
	}
	if records, _ := s.ListInboxRecords(ctx, owner, computer, management, 0, 0); len(records) != 0 {
		t.Fatalf("a failed append left records: %+v", records)
	}
	// The pending list is durable: it survives a reload of the schedule.
	if item := sched.PendingArrivals[0]; item.Title != "Item 1" || item.URL != "https://pending.example/1" || item.Language != "en" || item.Script != sources.ScriptLatin {
		t.Fatalf("pending item = %+v; want title, URL, language and script kept", item)
	}

	// Poll 2 writes items 3-4. The arrival names pending first, bounded by
	// the limit of 3: items 1, 2, 3. Item 4 stays pending.
	failAppend.Store(false)
	sched = pollNext()
	records, err := s.ListInboxRecords(ctx, owner, computer, management, 0, 0)
	if err != nil || len(records) != 1 || records[0].Captures == nil {
		t.Fatalf("management inbox after poll 2 = %+v, %v; want one arrival", records, err)
	}
	want := []string{sink.seen["https://pending.example/1"], sink.seen["https://pending.example/2"], sink.seen["https://pending.example/3"]}
	if got := arrivalIDs(records[0].Captures.Items); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("poll 2 arrival = %v; want pending items first then the new one, at most 3: %v", got, want)
	}
	if got := arrivalIDs(sched.PendingArrivals); len(got) != 1 || got[0] != sink.seen["https://pending.example/4"] {
		t.Fatalf("pending after poll 2 = %v; want item 4 only", got)
	}

	// Poll 3 writes items 5-6 and announces 4, 5, 6; nothing is left.
	sched = pollNext()
	records, _ = s.ListInboxRecords(ctx, owner, computer, management, 0, 0)
	want = []string{sink.seen["https://pending.example/4"], sink.seen["https://pending.example/5"], sink.seen["https://pending.example/6"]}
	if len(records) != 2 || strings.Join(arrivalIDs(records[1].Captures.Items), ",") != strings.Join(want, ",") {
		t.Fatalf("poll 3 arrival = %+v; want %v", records, want)
	}
	if len(sched.PendingArrivals) != 0 {
		t.Fatalf("pending after poll 3 = %+v; want none", sched.PendingArrivals)
	}

	// Management's view of an arrival shows each item's language and script.
	view := inboxRecordView(records[1]).String()
	if !strings.Contains(view, `"language":"en"`) || !strings.Contains(view, `"script":"latin"`) {
		t.Fatalf("management's record view hides the language: %s", view)
	}
}

func TestReplayedArrivalKeepsItsDigestAndAConflictClearsPending(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, computer, sourceID := "owner-replay", rt.cfg.ComputerID, "rss:replay"
	management := agentprofile.Management + ":" + owner
	items := []types.CaptureArrival{
		{CaptureID: "choir.web_capture:r-2", Title: "東京で地震", URL: "https://replay.example/2", Language: "en", Script: sources.ScriptKana},
		{CaptureID: "choir.web_capture:r-1", Title: "Markets steady", URL: "https://replay.example/1", PublishedAt: "2026-10-14T01:00:00Z", Language: "en", Script: sources.ScriptLatin},
	}
	first, err := captureArrivalRequest(owner, computer, sourceID, items)
	if err != nil {
		t.Fatal(err)
	}
	// The pending list is stored as JSON on the schedule; an arrival built
	// from the stored items is the same command.
	if err := s.PutSourceSchedule(ctx, store.SourceSchedule{OwnerID: owner, ComputerID: computer, SourceID: sourceID, PendingArrivals: items}); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetSourceSchedule(ctx, owner, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := captureArrivalRequest(owner, computer, sourceID, stored.PendingArrivals)
	if err != nil {
		t.Fatal(err)
	}
	if first.CommandID != again.CommandID || first.CommandDigest != again.CommandDigest {
		t.Fatalf("a replayed arrival changed: %s/%s then %s/%s", first.CommandID, first.CommandDigest, again.CommandID, again.CommandDigest)
	}
	if err := rt.appendCaptureArrival(ctx, owner, computer, sourceID, items); err != nil {
		t.Fatal(err)
	}
	if err := rt.appendCaptureArrival(ctx, owner, computer, sourceID, stored.PendingArrivals); err != nil {
		t.Fatalf("replaying the same arrival was refused: %v", err)
	}
	if records, _ := s.ListInboxRecords(ctx, owner, computer, management, 0, 0); len(records) != 1 {
		t.Fatalf("a replayed arrival minted %d records, want 1", len(records))
	}

	// An arrival naming the same captures with other content conflicts with
	// the committed one: those captures were announced, so they leave the
	// pending list instead of wedging it.
	changed := append([]types.CaptureArrival(nil), items...)
	changed[0].Title = "A different title"
	left, err := rt.announceCaptures(ctx, owner, computer, sourceID, changed, nil, 5)
	if err != nil || len(left) != 0 {
		t.Fatalf("a conflicting arrival left %v pending (err %v); want the announced captures cleared", left, err)
	}
}

func TestCrashBootClosesPendingArrivalsAndAPlannedBootKeepsThem(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	owner, sourceID := "owner-crash-pending", "rss:crash-pending"
	pending := []types.CaptureArrival{{CaptureID: "choir.web_capture:c-1", URL: "https://crash.example/1"}, {CaptureID: "choir.web_capture:c-2", URL: "https://crash.example/2"}}
	if err := s.PutSourceSchedule(ctx, store.SourceSchedule{OwnerID: owner, ComputerID: rt.cfg.ComputerID, SourceID: sourceID,
		State: store.SourceScheduleStopped, PendingArrivals: pending}); err != nil {
		t.Fatal(err)
	}
	WithBootRestart(PlannedRestart{Reason: "platform_update", Target: "next"}, true)(rt)
	rt.closeSourceSchedulesAfterCrash(ctx)
	if got, _ := s.GetSourceSchedule(ctx, owner, sourceID); len(got.PendingArrivals) != 2 || got.ArrivalsClosed != 0 {
		t.Fatalf("a planned boot touched the pending list: %+v", got)
	}
	WithBootRestart(PlannedRestart{}, false)(rt)
	rt.closeSourceSchedulesAfterCrash(ctx)
	got, _ := s.GetSourceSchedule(ctx, owner, sourceID)
	if len(got.PendingArrivals) != 0 || got.ArrivalsClosed != 2 || got.ArrivalsClosedReason != sourceScheduleInterruptedWhy {
		t.Fatalf("after a crash boot = %+v; want the pending list closed with its count and fate", got)
	}
}

func TestPendingArrivalsAreBounded(t *testing.T) {
	var pending []types.CaptureArrival
	for i := 0; i < sourceArrivalPendingLimit+7; i++ {
		pending = append(pending, types.CaptureArrival{CaptureID: fmt.Sprintf("choir.web_capture:b-%d", i)})
	}
	kept, dropped := boundPendingArrivals(pending)
	if len(kept) != sourceArrivalPendingLimit || dropped != 7 || kept[0].CaptureID != "choir.web_capture:b-7" {
		t.Fatalf("bounded pending = %d kept (first %s), %d dropped; want the newest %d kept", len(kept), kept[0].CaptureID, dropped, sourceArrivalPendingLimit)
	}
}
