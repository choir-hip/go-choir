package store

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The stream_seq seed reads a bounded number of rows, whatever the size of
// the event history (docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md).
//
// Ways it can fail, written before the code:
//  1. The seed lists the whole event kind: the engine takes the kind index,
//     sorts every event by updated_at and keeps 500. The engine-mutex counters
//     show a ListObjects over choir.event on the first append
//     (TestStreamSeqSeedDoesNotListTheEventKind). Its plan is pinned in the
//     objectgraph package.
//  2. The bounded read misses the newest event because other kinds were
//     written after it: the seed starts low and hands a number out twice
//     (TestStreamSeqSeedFindsAnEventBehindOtherKinds).
//  3. The seed forgets one of its two tables: events in choir_events above the
//     og_objects events, or the reverse (TestStreamSeqSeedAcrossBothTables).
//  4. The sealer's reseed gap is dropped or applied twice
//     (TestStreamSeqSeedKeepsTheTelemetryGap).
//  5. A store with no events, or with only other kinds, starts anywhere but 1.

const eventListOp = "(*DoltStore).ListObjects choir.event"

func appendSeedEvent(t *testing.T, s *Store, id, run string, at time.Time) *types.EventRecord {
	t.Helper()
	rec := &types.EventRecord{EventID: id, RunID: run, OwnerID: "alice", Timestamp: at,
		Kind: types.EventRunProgress, Payload: json.RawMessage(`{"id":"` + id + `"}`)}
	if err := s.AppendEvent(context.Background(), rec); err != nil {
		t.Fatalf("append %s: %v", id, err)
	}
	return rec
}

// reopenAt closes s and opens the same files again: a restart.
func reopenAt(t *testing.T, s *Store, path string) *Store {
	t.Helper()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	return s2
}

func openSeedStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

// Failure 1. The first append of a process must not list the event kind.
func TestStreamSeqSeedDoesNotListTheEventKind(t *testing.T) {
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 40; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), fmt.Sprintf("run-%d", i%4), old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 20, 10)
	s = reopenAt(t, s, path)

	rec := appendSeedEvent(t, s, "after", "run-after", time.Now().UTC())
	if rec.StreamSeq != 41 {
		t.Fatalf("stream_seq after the restart = %d, want 41", rec.StreamSeq)
	}
	if calls := s.EngineMutexStats()[eventListOp].Calls; calls != 0 {
		t.Fatalf("the first append listed the event kind %d time(s) (%s): the seed reads every event of the kind", calls, eventListOp)
	}
}

// Failure 2. Other kinds written after the last event push it out of the
// first window; the seed must still find it.
func TestStreamSeqSeedFindsAnEventBehindOtherKinds(t *testing.T) {
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second))
	}
	seedWakeOutbox(t, s, 2600, 0)
	s = reopenAt(t, s, path)

	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq != 4 {
		t.Fatalf("stream_seq after 2,600 later objects of another kind = %d, want 4", rec.StreamSeq)
	}
}

// Failure 3. The two tables, in both orders.
func TestStreamSeqSeedAcrossBothTables(t *testing.T) {
	ctx := context.Background()
	for _, tableNewer := range []bool{true, false} {
		t.Run(fmt.Sprintf("table_newer=%v", tableNewer), func(t *testing.T) {
			h := newTapeHarness(t)
			ogStream, tableStream := int64(5), int64(9)
			if !tableNewer {
				ogStream, tableStream = 12, 7
			}
			put := func(stamped bool, id string, seq, stream int64, at time.Time) {
				t.Helper()
				op := eventTableOp(t, eventTableRecord(id, "run-a", "owner-1", "traj-a", types.EventRunProgress, seq, stream, at), stamped)
				if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{op}); err != nil {
					t.Fatalf("project %s: %v", id, err)
				}
			}
			put(false, "og-1", 1, 1, eventTableBase.Add(time.Second))
			put(true, "tb-1", 2, tableStream, eventTableBase.Add(2*time.Second))
			put(false, "og-2", 3, ogStream, eventTableBase.Add(3*time.Second))
			want := max(ogStream, tableStream) + 1

			h.reopen()
			rec := h.event("after", "run-z", "owner-1", types.EventRunStarted, eventTableBase.Add(time.Hour))
			if err := h.store.AppendEvent(ctx, rec); err != nil {
				t.Fatal(err)
			}
			if rec.StreamSeq != want {
				t.Fatalf("stream_seq after the restart = %d, want %d (og newest %d, table newest %d)", rec.StreamSeq, want, ogStream, tableStream)
			}
		})
	}
}

// Failure 4. With the sealer the seed skips telemetrySeqGap once, over the
// stored maximum.
func TestStreamSeqSeedKeepsTheTelemetryGap(t *testing.T) {
	h := newTapeHarness(t)
	for i := 1; i <= 5; i++ {
		rec := h.event(fmt.Sprintf("e-%d", i), "run-a", "owner-1", types.EventRunSubmitted, telemetryBase.Add(time.Duration(i)*time.Second))
		if err := h.store.AppendEvent(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
	h.reopen()
	h.enableBatching(time.Hour)
	rec := h.event("after", "run-a", "owner-1", types.EventRunCancelled, telemetryBase.Add(time.Minute))
	if err := h.store.AppendEvent(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	// The boot watermark takes the first number above the gap; the append is
	// the next acknowledged number or later. Never inside the gap.
	if rec.StreamSeq <= 5+telemetrySeqGap {
		t.Fatalf("stream_seq with the sealer = %d, want above %d (stored max 5 + gap %d)", rec.StreamSeq, 5+telemetrySeqGap, telemetrySeqGap)
	}
	if rec.StreamSeq > 5+telemetrySeqGap+2 {
		t.Fatalf("stream_seq with the sealer = %d: the gap was applied more than once (stored max 5, gap %d)", rec.StreamSeq, telemetrySeqGap)
	}
}

// Failure 5. Nothing stored.
func TestStreamSeqSeedStartsAtOne(t *testing.T) {
	s, _ := openSeedStore(t)
	t.Cleanup(func() { _ = s.Close() })
	if rec := appendSeedEvent(t, s, "first", "run-a", time.Now().UTC()); rec.StreamSeq != 1 {
		t.Fatalf("first stream_seq of an empty store = %d, want 1", rec.StreamSeq)
	}
}

// Failure 5. Nothing but other kinds stored.
func TestStreamSeqSeedStartsAtOneBesideOtherKinds(t *testing.T) {
	s, path := openSeedStore(t)
	seedWakeOutbox(t, s, 30, 0)
	s = reopenAt(t, s, path)
	if rec := appendSeedEvent(t, s, "first", "run-a", time.Now().UTC()); rec.StreamSeq != 1 {
		t.Fatalf("first stream_seq of a store with only other kinds = %d, want 1", rec.StreamSeq)
	}
}
