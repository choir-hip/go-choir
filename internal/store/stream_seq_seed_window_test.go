package store

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The windows of the stream_seq seed: how far back through the newest
// objects of any kind it looks for an event, and what it does when it finds
// none. Ways it can fail, written before the code:
//  1. The first window holds an event but the seed keeps widening, or reads
//     more than one window: the cost is no longer the window's.
//  2. A newest event behind more other-kind objects than the first window
//     is missed: the widening does not happen.
//  3. A store with no events at all pays for the widest window and then
//     warns, or guesses a number other than 0.
//  4. Events exist but sit behind more objects than every window: the seed
//     seeds from the event table alone and hands a stream_seq out twice. It
//     must fall back to the exact read of the kind (slow, rare), seed above
//     every stored number, and say so with the duration.

func shrinkSeedWindows(t *testing.T, windows ...int) {
	t.Helper()
	old := streamSeedWindows
	streamSeedWindows = windows
	t.Cleanup(func() { streamSeedWindows = old })
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestStreamSeqSeedStopsAtTheFirstWindowWithAnEvent(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	var last int64
	for i := 0; i < 5; i++ {
		last = appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second)).StreamSeq
	}
	seedWakeOutbox(t, s, 8, 0)
	s = reopenAt(t, s, path) // a trusted lease: the windows are the cross-check
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq <= last {
		t.Fatalf("stream_seq = %d, want above %d", rec.StreamSeq, last)
	}
	stats := s.EngineMutexStats()
	if calls := stats["(*DoltStore).ListNewestObjectHeaders"].Calls; calls != 1 {
		t.Fatalf("the seed read %d windows, want the first only", calls)
	}
	if calls := stats["(*DoltStore).HasObjectOfKind"].Calls; calls != 0 {
		t.Fatalf("the seed probed for the kind %d times after finding an event", calls)
	}
}

func TestStreamSeqSeedWidensPastOtherKinds(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	var last int64
	for i := 0; i < 5; i++ {
		last = appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second)).StreamSeq
	}
	seedWakeOutbox(t, s, 60, 0) // behind the first window, inside the second
	s = reopenAt(t, s, path)
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq <= last {
		t.Fatalf("stream_seq = %d, want above %d", rec.StreamSeq, last)
	}
	if calls := s.EngineMutexStats()["(*DoltStore).ListNewestObjectHeaders"].Calls; calls != 2 {
		t.Fatalf("the seed read %d windows, want 2", calls)
	}
}

func TestStreamSeqSeedWithNoEventsIsQuiet(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	seedWakeOutbox(t, s, 300, 0)
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	rec := appendSeedEvent(t, s, "first", "run-a", time.Now().UTC())
	if rec.StreamSeq != streamSeqExactGap+1 {
		t.Fatalf("stream_seq = %d, want %d", rec.StreamSeq, streamSeqExactGap+1)
	}
	if strings.Contains(buf.String(), "stream_seq seed") {
		t.Fatalf("a store without events warned: %s", buf.String())
	}
}

func TestStreamSeqSeedSaysWhenEventsAreOutOfReach(t *testing.T) {
	// No lease (events a lease-unaware binary stored), every event behind
	// every window: the exact seed finds them, and says so with a duration.
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		putStoredEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", int64(i+1), old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 400, 0) // behind every window
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if want := 5 + streamSeqExactGap + 1; rec.StreamSeq != want {
		t.Fatalf("stream_seq with every event beyond every window = %d, want %d (above the stored 5): a number was handed out twice", rec.StreamSeq, want)
	}
	if !strings.Contains(buf.String(), "stream_seq exact seed") || !strings.Contains(buf.String(), "took ") {
		t.Fatalf("events exist beyond every window and the seed did not say so with a duration:\n%s", buf.String())
	}
}

// Round 3. A window miss says nothing about the table's newest row: stamped
// table events and unstamped og events coexist (a stamp rollback, a mixed
// fleet, a non-event backfill). When the windows hold no event the seed takes
// the exact read of the kind whatever the table holds, and the table's maximum
// joins through max(). This pins the counterexample of review bunch 156
// (DeepSeek): the table's newest is 9, an og event holds 12 and is newer, 400
// newer objects of another kind push it out of both windows. Round 2 seeded 10.
// Flipped from round 2's TestStreamSeqSeedUsesTheTableWhenTheWindowsHoldNoEvent,
// which pinned the table shortcut (no exact read) and so pinned the defect.
func TestStreamSeqSeedTakesTheExactReadWhenTheWindowsHoldNoEventWhateverTheTableHolds(t *testing.T) {
	ctx := context.Background()
	shrinkSeedWindows(t, 20, 200)
	h := newTapeHarness(t)
	put := func(stamped bool, id string, stream int64, at time.Time) {
		t.Helper()
		op := eventTableOp(t, eventTableRecord(id, "run-a", "owner-1", "traj-a", types.EventRunProgress, stream, stream, at), stamped)
		if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{op}); err != nil {
			t.Fatalf("project %s: %v", id, err)
		}
	}
	for i := int64(1); i <= 3; i++ {
		put(false, fmt.Sprintf("og-%d", i), i, eventTableBase.Add(time.Duration(i)*time.Second))
	}
	for i := int64(4); i <= 9; i++ {
		put(true, fmt.Sprintf("tb-%d", i), i, eventTableBase.Add(time.Duration(i)*time.Second))
	}
	put(false, "og-12", 12, eventTableBase.Add(20*time.Second)) // newer than the table's events
	seedWakeOutbox(t, h.store, 400, 0)                          // newer still, behind both windows
	h.reopen()
	buf := captureLog(t)
	rec := h.event("after", "run-z", "owner-1", types.EventRunStarted, eventTableBase.Add(time.Hour))
	if err := h.store.AppendEvent(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if want := 12 + streamSeqExactGap + 1; rec.StreamSeq != want {
		t.Fatalf("stream_seq = %d, want %d (above the og event's 12 and the table's 9): a number was handed out twice", rec.StreamSeq, want)
	}
	if calls := pageOpCalls(h.store); calls == 0 {
		t.Fatal("the exact seed of the kind did not run")
	}
	if !strings.Contains(buf.String(), "stream_seq exact seed") || !strings.Contains(buf.String(), "took ") {
		t.Fatalf("the exact seed did not log its case and duration:\n%s", buf.String())
	}
}

// Round 3, Devin F2. An empty window list must not panic when it names the
// widest window in its log line; with no window the seed takes the exact read.
func TestStreamSeqSeedWithNoWindowsTakesTheExactRead(t *testing.T) {
	shrinkSeedWindows(t)
	s, path := openSeedStore(t)
	var last int64
	for i := 0; i < 3; i++ {
		last = appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", time.Now().UTC().Add(-time.Hour+time.Duration(i)*time.Second)).StreamSeq
	}
	s = reopenAt(t, s, path)
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq <= last {
		t.Fatalf("stream_seq = %d, want above %d (the lease serves; the cross-check has no windows)", rec.StreamSeq, last)
	}
}

// Round 2, F1 (cheap part). A window event whose metadata carries no
// parseable stream_seq must not count as a found event: it would seed 0
// beneath a stored number. The seed takes the exact read, which decodes the
// body.
func TestStreamSeqSeedDoesNotTrustAnEventWithoutMetadataSeq(t *testing.T) {
	s, path := openSeedStore(t)
	at := time.Now().UTC().Add(-time.Minute)
	rec := eventTableRecord("legacy", "run-a", "alice", "traj-a", types.EventRunProgress, 1, 7, at)
	obj, err := buildEventObject(rec, at)
	if err != nil {
		t.Fatal(err)
	}
	obj.Metadata = []byte(`{"event_id":"legacy"}`) // no stream_seq; the body still says 7
	obj.ContentHash = objectgraph.ContentHash(obj.ObjectKind, obj.Body, obj.Metadata)
	if err := s.ogStore.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatal(err)
	}
	s = reopenAt(t, s, path)
	next := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if want := 7 + streamSeqExactGap + 1; next.StreamSeq != want {
		t.Fatalf("stream_seq = %d, want %d (the event without metadata stream_seq carries 7 in its body)", next.StreamSeq, want)
	}
}
