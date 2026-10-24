package store

// The event script and the read matrix the trace-log differential tests share
// (docs/design/trace-segment-log-2026-10-22.md, F2). The golden file beside it
// was produced by running traceLogReadMatrix on origin/main, before any code
// of slice S2 existed, so an empty log is compared with the code it must not
// change. This file uses only the store API that existed on that checkout.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

var traceLogScriptBase = time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)

type traceLogScriptEvent struct {
	id, run, owner, traj string
	kind                 types.EventKind
}

// traceLogScript interleaves trace kinds with acknowledged kinds inside every
// run, shares one run id between two owners, shares one trajectory between two
// runs, and has runless events of both classes. About two thirds of it is
// trace kinds.
func traceLogScript() []traceLogScriptEvent {
	kinds := []types.EventKind{
		types.EventRunStarted, types.EventToolInvoked, types.EventToolResult, types.EventRunProgress,
		types.EventDeskCell, types.EventRunDelta, types.EventChannelMessage, types.EventToolResult,
		types.EventRunProgress, types.EventRunCompleted,
	}
	var out []traceLogScriptEvent
	n := 0
	add := func(run, owner, traj string, kind types.EventKind) {
		n++
		out = append(out, traceLogScriptEvent{id: fmt.Sprintf("ev-%03d", n), run: run, owner: owner, traj: traj, kind: kind})
	}
	for round := 0; round < 3; round++ {
		for i, kind := range kinds {
			add("run-a", "owner-1", "traj-a", kind)
			if i%2 == 0 {
				add("run-b", "owner-1", "traj-a", kind)
			}
			if i%3 == 0 {
				add("run-a", "owner-2", "traj-c", kind) // the same run id under another owner
			}
			if i%4 == 1 {
				add("run-c", "owner-2", "traj-c", kind)
			}
		}
		add("", "owner-1", "", types.EventRunProgress) // a runless trace kind
		add("", "", "", types.EventRuntimeHealth)      // a runless acknowledged kind, ownerless
		add("", "owner-2", "", types.EventTextureAgentRevisionProgress)
	}
	return out
}

func (e traceLogScriptEvent) record(i int) *types.EventRecord {
	return &types.EventRecord{
		EventID: e.id, RunID: e.run, OwnerID: e.owner, TrajectoryID: e.traj,
		Timestamp: traceLogScriptBase.Add(time.Duration(i) * time.Second),
		Kind:      e.kind, Payload: json.RawMessage(fmt.Sprintf(`{"id":%q,"n":%d}`, e.id, i)),
	}
}

// appendTraceLogScript appends the script through AppendEvent, in order.
func appendTraceLogScript(t *testing.T, s *Store) []types.EventRecord {
	t.Helper()
	var out []types.EventRecord
	for i, e := range traceLogScript() {
		rec := e.record(i)
		if err := s.AppendEvent(context.Background(), rec); err != nil {
			t.Fatalf("append %s: %v", e.id, err)
		}
		out = append(out, *rec)
	}
	return out
}

// traceLogReadMatrix calls every event-list method of the store over a grid of
// cursors and limits and returns each answer as the JSON a page would carry.
// The cursors are taken from the records themselves, so the grid covers every
// boundary between two events.
func traceLogReadMatrix(t *testing.T, s *Store, recs []types.EventRecord) map[string]string {
	t.Helper()
	ctx := context.Background()
	out := map[string]string{}
	put := func(name string, list []types.EventRecord, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		raw, jerr := json.Marshal(list)
		if jerr != nil {
			t.Fatal(jerr)
		}
		out[name] = string(raw)
	}
	runs := map[string]bool{}
	owners := map[string]bool{}
	trajs := map[[2]string]bool{}
	seqCursors := map[int64]bool{0: true}
	streamCursors := map[int64]bool{0: true}
	for i, r := range recs {
		runs[r.RunID] = true
		owners[r.OwnerID] = true
		trajs[[2]string{r.OwnerID, r.TrajectoryID}] = true
		// A sample of the cursors: every boundary near either end and every
		// fourth in between.
		if i < 3 || i%4 == 0 || i >= len(recs)-3 {
			seqCursors[r.Seq] = true
			streamCursors[r.StreamSeq] = true
		}
	}
	streamCursors[recs[len(recs)-1].StreamSeq+1] = true
	limits := []int{1, 2, 3, 7, 25, 200}
	for run := range runs {
		for _, limit := range limits {
			list, err := s.ListEvents(ctx, run, limit)
			put(fmt.Sprintf("run=%q limit=%d", run, limit), list, err)
		}
		for after := range seqCursors {
			for _, limit := range []int{1, 3, 200} {
				list, err := s.ListEventsAfter(ctx, run, after, limit)
				put(fmt.Sprintf("run=%q after=%d limit=%d", run, after, limit), list, err)
			}
		}
	}
	for owner := range owners {
		for _, limit := range limits {
			list, err := s.ListEventsByOwner(ctx, owner, limit)
			put(fmt.Sprintf("owner=%q newest limit=%d", owner, limit), list, err)
		}
		for after := range streamCursors {
			for _, limit := range []int{1, 3, 200} {
				list, err := s.ListEventsByOwnerAfter(ctx, owner, after, limit)
				put(fmt.Sprintf("owner=%q stream-after=%d limit=%d", owner, after, limit), list, err)
			}
		}
	}
	for key := range trajs {
		for _, limit := range []int{1, 4, 500} {
			list, err := s.ListEventsByTrajectory(ctx, key[0], key[1], limit)
			put(fmt.Sprintf("owner=%q trajectory=%q limit=%d", key[0], key[1], limit), list, err)
		}
	}
	return out
}

// traceLogSeedProbe closes s, opens its path again and appends one more event
// to three runs, one known to the store and one new, and returns what the
// restarted allocators handed out: the per-run and the stream seeds.
func traceLogSeedProbe(t *testing.T, s *Store) map[string]string {
	t.Helper()
	path := s.Path()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	out := map[string]string{}
	for i, key := range [][2]string{{"run-a", "owner-1"}, {"run-b", "owner-1"}, {"run-a", "owner-2"}, {"run-new", "owner-1"}} {
		rec := &types.EventRecord{EventID: fmt.Sprintf("probe-%d", i), RunID: key[0], OwnerID: key[1], TrajectoryID: "traj-probe",
			Timestamp: traceLogScriptBase.Add(time.Hour + time.Duration(i)*time.Second), Kind: types.EventToolResult, Payload: json.RawMessage(`{}`)}
		if err := s2.AppendEvent(context.Background(), rec); err != nil {
			t.Fatalf("probe append: %v", err)
		}
		out[fmt.Sprintf("seed run=%s owner=%s", key[0], key[1])] = fmt.Sprintf("seq=%d stream_seq=%d", rec.Seq, rec.StreamSeq)
	}
	return out
}
