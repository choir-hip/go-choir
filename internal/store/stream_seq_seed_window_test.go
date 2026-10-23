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
	for i := 0; i < 5; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 8, 0)
	s = reopenAt(t, s, path)
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq != 6 {
		t.Fatalf("stream_seq = %d, want 6", rec.StreamSeq)
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
	for i := 0; i < 5; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 60, 0) // behind the first window, inside the second
	s = reopenAt(t, s, path)
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq != 6 {
		t.Fatalf("stream_seq = %d, want 6", rec.StreamSeq)
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
	if rec.StreamSeq != 1 {
		t.Fatalf("stream_seq = %d, want 1", rec.StreamSeq)
	}
	if strings.Contains(buf.String(), "stream_seq seed") {
		t.Fatalf("a store without events warned: %s", buf.String())
	}
}

func TestStreamSeqSeedSaysWhenEventsAreOutOfReach(t *testing.T) {
	shrinkSeedWindows(t, 20, 200)
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 400, 0) // behind every window
	s = reopenAt(t, s, path)
	buf := captureLog(t)
	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq != 6 {
		t.Fatalf("stream_seq with every event beyond every window = %d, want 6 (above the stored 5): a number was handed out twice", rec.StreamSeq)
	}
	if !strings.Contains(buf.String(), "stream_seq seed") || !strings.Contains(buf.String(), "out of reach") || !strings.Contains(buf.String(), "took ") {
		t.Fatalf("events exist beyond every window and the seed did not say so with a duration:\n%s", buf.String())
	}
}

// Round 2, F2. After the event table takes most kinds, "no og event in the
// newest objects" is the common case. With the table holding events, an og
// event older than every window is older than the table's newest under the
// same newest-carries-max assumption: seed from the table, and do not take
// the exact read of the kind.
func TestStreamSeqSeedUsesTheTableWhenTheWindowsHoldNoEvent(t *testing.T) {
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
	seedWakeOutbox(t, h.store, 400, 0) // newer than the og events, behind no window of theirs
	h.reopen()
	buf := captureLog(t)
	rec := h.event("after", "run-z", "owner-1", types.EventRunStarted, eventTableBase.Add(time.Hour))
	if err := h.store.AppendEvent(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if rec.StreamSeq != 10 {
		t.Fatalf("stream_seq = %d, want 10 (above the table's 9 and the og events' 3)", rec.StreamSeq)
	}
	if calls := h.store.EngineMutexStats()[eventListOp].Calls; calls != 0 {
		t.Fatalf("the exact read of the kind ran %d time(s) although the event table holds events", calls)
	}
	if strings.Contains(buf.String(), "out of reach") {
		t.Fatalf("the out-of-reach path ran although the event table holds events:\n%s", buf.String())
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
	if next.StreamSeq != 8 {
		t.Fatalf("stream_seq = %d, want 8 (the event without metadata stream_seq carries 7 in its body)", next.StreamSeq)
	}
}
