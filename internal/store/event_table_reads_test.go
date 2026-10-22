package store

// Review findings B, F, G and E on deploy 1, for deploy 2. Ways they fail,
// written before the code:
//   - B, F: a choir_events half decodes its JSON bodies while it holds the
//     engine mutex, so a 10,000-row read holds every object-graph query for
//     the decode (Sonnet measured 228 of 281 ms on 4,000 rows);
//   - G: the two halves of a cursor read are two statements, so an
//     og_objects event committed between them is missed while a later
//     choir_events event is returned: the caller's cursor moves past the
//     missed event and it is never read;
//   - E: with the writer on, the dry run's tape position (observed inside
//     its transaction) differs from the finalized tape_seq of a stamped row.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// B, F: the decode of every choir_events half runs with the engine mutex
// free.
func TestEventTableReadsDecodeOutsideTheEngineLock(t *testing.T) {
	ctx := context.Background()
	script := eventTableScript(60)
	s := eventTableStoreFromScript(t, "decode-unlocked", script, func(int) bool { return true })
	var mu sync.Mutex
	calls := map[string]int{}
	s.eventTableDecodeHook = func(op string) {
		mu.Lock()
		defer mu.Unlock()
		calls[op]++
		if live := s.EngineMutexLive(); live != nil && live.HolderOp != "" {
			t.Errorf("%s decodes while the engine mutex is held by %s", op, live.HolderOp)
		}
	}
	mustEvents(t)(s.ListEventsAfter(ctx, "run-1", 0, 1000))
	mustEvents(t)(s.ListEventsByOwner(ctx, "owner-1", 1000))
	mustEvents(t)(s.ListEventsByOwnerAfter(ctx, "owner-1", 0, 1000))
	mustEvents(t)(s.ListEventsByTrajectory(ctx, "owner-1", "traj-1", 3000))
	for _, op := range []string{"store.event_table.list_run_after", "store.event_table.list_owner_newest", "store.event_table.list_owner_after", "store.event_table.list_trajectory"} {
		if calls[op] == 0 {
			t.Errorf("%s never reached its decode", op)
		}
	}
}

// G: an og_objects event committed between the two halves of a cursor read,
// followed by a choir_events event, must not be skipped: either both come
// back, or neither (the next read after the cursor returns both).
func TestEventTableCursorReadsSkipNothingBetweenHalves(t *testing.T) {
	type read struct {
		name   string
		cursor func(e types.EventRecord) int64
		call   func(s *Store, after int64) ([]types.EventRecord, error)
	}
	reads := []read{
		{"ListEventsAfter", func(e types.EventRecord) int64 { return e.Seq }, func(s *Store, after int64) ([]types.EventRecord, error) {
			return s.ListEventsAfter(context.Background(), "run-g", after, 1000)
		}},
		{"ListEventsByOwnerAfter", func(e types.EventRecord) int64 { return e.StreamSeq }, func(s *Store, after int64) ([]types.EventRecord, error) {
			return s.ListEventsByOwnerAfter(context.Background(), "owner-1", after, 1000)
		}},
	}
	for _, r := range reads {
		t.Run(r.name, func(t *testing.T) {
			ctx := context.Background()
			h := newNamedTapeHarness(t, strings.ToLower(r.name))
			if err := h.store.SetEventTableWrites(true); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 4; i++ {
				kind := []types.EventKind{types.EventRunStarted, types.EventToolResult}[i%2]
				if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("g-%d", i), "run-g", "owner-1", kind, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
					t.Fatal(err)
				}
			}
			first := mustEvents(t)(r.call(h.store, 0))
			cursor := r.cursor(first[len(first)-1])

			// Between the halves of the next read: an og_objects event, then
			// a later choir_events event, both committed.
			fired := false
			h.store.eventUnionHook = func(stage string) {
				if fired || stage != "after_object_graph" {
					return
				}
				fired = true
				if err := h.store.AppendEvent(ctx, h.event("g-og", "run-g", "owner-1", types.EventChannelMessage, telemetryBase.Add(time.Minute))); err != nil {
					t.Error(err)
				}
				if err := h.store.AppendEvent(ctx, h.event("g-table", "run-g", "owner-1", types.EventToolResult, telemetryBase.Add(time.Minute+time.Second))); err != nil {
					t.Error(err)
				}
			}
			second := mustEvents(t)(r.call(h.store, cursor))
			if !fired {
				t.Fatal("the hook between the halves never ran")
			}
			h.store.eventUnionHook = nil
			got := map[string]bool{}
			for _, e := range second {
				got[e.EventID] = true
				cursor = max(cursor, r.cursor(e))
			}
			if got["g-table"] && !got["g-og"] {
				t.Fatalf("%s returned the later choir_events event without the og_objects event committed before it: the cursor skips g-og", r.name)
			}
			// Whatever the second read returned, the next read after its
			// cursor finds everything else.
			for _, e := range mustEvents(t)(r.call(h.store, cursor)) {
				got[e.EventID] = true
			}
			if !got["g-og"] || !got["g-table"] {
				t.Fatalf("%s: after two reads g-og=%v g-table=%v; an event was skipped", r.name, got["g-og"], got["g-table"])
			}
		})
	}
}

// E through the writer: every stamped row's tape_seq is a position the dry
// run passed to projectBatch inside its own transaction.
func TestEventTableWriterDryRunPositionIsTheTapePosition(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	var mu sync.Mutex
	positions := map[uint64]bool{}
	h.store.dryRunPositionHook = func(pos uint64) {
		mu.Lock()
		positions[pos] = true
		mu.Unlock()
	}
	if err := h.store.SetEventTableWrites(true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		kind := []types.EventKind{types.EventToolResult, types.EventRunStarted, types.EventRunDelta}[i%3]
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("e-%d", i), "run-e", "owner-1", kind, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := h.store.db.Query(`SELECT tape_seq FROM choir_events`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var pos uint64
		if err := rows.Scan(&pos); err != nil {
			t.Fatal(err)
		}
		n++
		if !positions[pos] {
			t.Fatalf("stamped row at tape_seq %d; the dry runs saw %v", pos, positions)
		}
	}
	if n != 4 {
		t.Fatalf("%d stamped rows, want 4", n)
	}
}
