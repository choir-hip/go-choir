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

// What a restart's first stream_seq must be, after the stream_seq lease
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md; the seed
// record is docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md).
// These tests began as the bounded seed's; the seed is now the lease's, and
// the bounded windows are its cross-check. Ways it can fail:
//  1. A trusted restart lists the event kind: the engine takes the kind index,
//     sorts every event by updated_at and keeps 500. The engine-mutex counters
//     show a ListObjects over choir.event on the first append
//     (TestStreamSeqSeedDoesNotListTheEventKind).
//  2. A restart starts below the previous lease because other kinds were
//     written after the last event: a number is handed out twice
//     (TestStreamSeqSeedFindsAnEventBehindOtherKinds). With a trusted lease the
//     first number is exactly R + 1, whatever the windows hold.
//  3. An exact seed forgets one of its two tables: events in choir_events
//     above the og_objects events, or the reverse
//     (TestStreamSeqSeedAcrossBothTables): maximum + 2^20 + 1.
//  4. The sealer's old reseed gap is applied on top of the lease
//     (TestStreamSeqSeedWithTheSealerStaysAboveThePreviousLease).
//  5. A store with no events, or with only other kinds, starts anywhere but
//     2^20 + 1: the exact seed of nothing is 0, plus the gap.

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
	// A seeder or refill still running when the test returns would read the
	// lease constants (variables only tests write) while the next test's
	// shrinkLease writes them: stop the lease work first. Cleanups run last
	// in, first out, so this runs before shrinkLease restores the constants.
	t.Cleanup(func() {
		s.cancelLeaseWork()
		s.drainLeaseWork(time.Now().Add(10 * time.Second))
	})
	return s, path
}

// Failure 1. The first append of a process must not list the event kind.
func TestStreamSeqSeedDoesNotListTheEventKind(t *testing.T) {
	s, path := openSeedStore(t)
	old := time.Now().UTC().Add(-time.Hour)
	var lastStored int64
	for i := 0; i < 40; i++ {
		lastStored = appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), fmt.Sprintf("run-%d", i%4), old.Add(time.Duration(i)*time.Second)).StreamSeq
	}
	row, _ := readLeaseQuiet(s)
	seedWakeOutbox(t, s, 20, 10)
	s = reopenAt(t, s, path)

	rec := appendSeedEvent(t, s, "after", "run-after", time.Now().UTC())
	if rec.StreamSeq <= lastStored || rec.StreamSeq != row.Reserved+1 {
		t.Fatalf("stream_seq after the restart = %d, want the lease's R + 1 = %d (above the stored %d)", rec.StreamSeq, row.Reserved+1, lastStored)
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
	var lastStored int64
	for i := 0; i < 3; i++ {
		lastStored = appendSeedEvent(t, s, fmt.Sprintf("e-%d", i), "run-a", old.Add(time.Duration(i)*time.Second)).StreamSeq
	}
	row, _ := readLeaseQuiet(s)
	seedWakeOutbox(t, s, 2600, 0)
	s = reopenAt(t, s, path)

	rec := appendSeedEvent(t, s, "after", "run-b", time.Now().UTC())
	if rec.StreamSeq <= lastStored || rec.StreamSeq != row.Reserved+1 {
		t.Fatalf("stream_seq after 2,600 later objects of another kind = %d, want the lease's R + 1 = %d (above %d)", rec.StreamSeq, row.Reserved+1, lastStored)
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
			want := max(ogStream, tableStream) + streamSeqExactGap + 1 // no lease: the exact seed plus the gap

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

// Failure 4. With the sealer a number can be published and never stored; the
// lease covers it (it replaced telemetrySeqGap for stream_seq). After a restart
// the first number is above the previous process's lease, once: the boot
// watermark takes the first, the append the next or later.
func TestStreamSeqSeedWithTheSealerStaysAboveThePreviousLease(t *testing.T) {
	h := newTapeHarness(t)
	for i := 1; i <= 5; i++ {
		rec := h.event(fmt.Sprintf("e-%d", i), "run-a", "owner-1", types.EventRunSubmitted, telemetryBase.Add(time.Duration(i)*time.Second))
		if err := h.store.AppendEvent(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
	row, found := readLeaseQuiet(h.store)
	if !found {
		t.Fatal("no lease after five appends")
	}
	h.reopen()
	h.enableBatching(time.Hour)
	rec := h.event("after", "run-a", "owner-1", types.EventRunCancelled, telemetryBase.Add(time.Minute))
	if err := h.store.AppendEvent(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if rec.StreamSeq <= row.Reserved {
		t.Fatalf("stream_seq with the sealer = %d, want above the previous lease %d", rec.StreamSeq, row.Reserved)
	}
	if rec.StreamSeq > row.Reserved+2+streamSeqLeaseBlock {
		t.Fatalf("stream_seq with the sealer = %d: more than one block above the previous lease %d", rec.StreamSeq, row.Reserved)
	}
}

// Failure 5. Nothing stored: the exact seed of nothing is 0, plus the gap.
func TestStreamSeqSeedStartsAtOne(t *testing.T) {
	s, _ := openSeedStore(t)
	t.Cleanup(func() { _ = s.Close() })
	if rec := appendSeedEvent(t, s, "first", "run-a", time.Now().UTC()); rec.StreamSeq != streamSeqExactGap+1 {
		t.Fatalf("first stream_seq of an empty store = %d, want %d", rec.StreamSeq, streamSeqExactGap+1)
	}
}

// Failure 5. Nothing but other kinds stored.
func TestStreamSeqSeedStartsAtOneBesideOtherKinds(t *testing.T) {
	s, path := openSeedStore(t)
	seedWakeOutbox(t, s, 30, 0)
	s = reopenAt(t, s, path)
	if rec := appendSeedEvent(t, s, "first", "run-a", time.Now().UTC()); rec.StreamSeq != streamSeqExactGap+1 {
		t.Fatalf("first stream_seq of a store with only other kinds = %d, want %d", rec.StreamSeq, streamSeqExactGap+1)
	}
}
