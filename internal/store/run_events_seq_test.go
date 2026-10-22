package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestRunEventFoldedIntoLifecycleBatchGetsSeq is the regression test for
// docs/problems/run-events-without-seq-2026-10-22.md.
//
// UpdateRunWithEvent folds the run-state event into the lifecycle projection
// batch (persistLifecycleRunWithEvent -> projectLifecycleRun -> buildEventObject).
// Before the fix that path never called AppendEvent, so the stored event had
// Seq == 0 and StreamSeq == 0 and every reader that filters on seq > after
// dropped it (failed on origin/main 85a8115b1 as TestKnownFailing...).
func TestRunEventFoldedIntoLifecycleBatchGetsSeq(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	run := lifecycleRunFixture(start, "run-seq-fold", types.RunRunning)
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID:    "project-seq-fold:" + start.TrajectoryID,
		TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: run,
	}
	project.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project activation: %v", err)
	}

	newEvent := func(id string, kind types.EventKind, at time.Time) *types.EventRecord {
		return &types.EventRecord{
			EventID: id, RunID: run.RunID, AgentID: run.AgentID,
			OwnerID: run.OwnerID, TrajectoryID: run.TrajectoryID,
			Kind: kind, Timestamp: at,
		}
	}
	base := run.UpdatedAt
	// An ordinary event before the fold (what a tool call writes).
	before := newEvent("ev-before", types.EventKind("test.before"), base.Add(time.Second))
	if err := s.AppendEvent(ctx, before); err != nil {
		t.Fatalf("append before: %v", err)
	}
	// The run-state event, folded into the lifecycle batch with the state change.
	run.State = types.RunCompleted
	run.UpdatedAt = base.Add(2 * time.Second)
	folded := newEvent("ev-folded", types.EventRunCompleted, run.UpdatedAt)
	if err := s.UpdateRunWithEvent(ctx, run, folded); err != nil {
		t.Fatalf("update run with event: %v", err)
	}
	// An ordinary event after the fold.
	after := newEvent("ev-after", types.EventKind("test.after"), base.Add(3*time.Second))
	if err := s.AppendEvent(ctx, after); err != nil {
		t.Fatalf("append after: %v", err)
	}
	t.Logf("[M] AppendEvent assigned: before seq=%d stream_seq=%d; after seq=%d stream_seq=%d",
		before.Seq, before.StreamSeq, after.Seq, after.StreamSeq)

	// Control: the unfiltered object-graph read does see the event, with the numbers it was stored with.
	og, err := s.ListEventsOG(ctx, run.RunID, 100)
	if err != nil {
		t.Fatalf("list events OG: %v", err)
	}
	var stored *types.EventRecord
	for i := range og {
		if og[i].EventID == "ev-folded" {
			stored = &og[i]
		}
	}
	if stored == nil {
		t.Fatalf("control failed: folded event is not stored at all: %+v", og)
	}
	t.Logf("[M] stored folded event: seq=%d stream_seq=%d", stored.Seq, stored.StreamSeq)
	if stored.Seq <= 0 || stored.StreamSeq <= 0 {
		t.Errorf("folded event stored with seq=%d stream_seq=%d, want both > 0", stored.Seq, stored.StreamSeq)
	}

	ids := func(evs []types.EventRecord) []string {
		out := make([]string, 0, len(evs))
		for _, e := range evs {
			out = append(out, e.EventID)
		}
		return out
	}
	has := func(evs []types.EventRecord, id string) bool {
		for _, e := range evs {
			if e.EventID == id {
				return true
			}
		}
		return false
	}

	t.Run("run events API and trace export, after=0", func(t *testing.T) {
		// handleRunEvents and `choir trace` page through ListEventsAfter(after=0).
		got, err := s.ListEventsAfter(ctx, run.RunID, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("[M] ListEventsAfter(run, 0) = %v", ids(got))
		if !has(got, "ev-folded") {
			t.Errorf("run.completed missing from ListEventsAfter(after=0): %v", ids(got))
		}
	})

	t.Run("websocket catch-up", func(t *testing.T) {
		// live_ws.go catch-up reads ListEventsByOwnerAfter(owner, after_seq).
		got, err := s.ListEventsByOwnerAfter(ctx, run.OwnerID, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("[M] ListEventsByOwnerAfter(owner, 0) = %v", ids(got))
		if !has(got, "ev-folded") {
			t.Errorf("run.completed missing from ListEventsByOwnerAfter(after=0): %v", ids(got))
		}
		// A client whose cursor is the stream_seq of the event before the fold.
		got, err = s.ListEventsByOwnerAfter(ctx, run.OwnerID, before.StreamSeq, 100)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("[M] ListEventsByOwnerAfter(owner, %d) = %v", before.StreamSeq, ids(got))
		if !has(got, "ev-folded") {
			t.Errorf("run.completed missing from catch-up after the previous event: %v", ids(got))
		}
	})

	t.Run("trajectory read order", func(t *testing.T) {
		// run_acceptance and the Texture workflow verifier read ListEventsByTrajectory,
		// sorted by stream_seq. The run-state event must sit between its neighbours.
		got, err := s.ListEventsByTrajectory(ctx, run.OwnerID, run.TrajectoryID, 100)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("[M] ListEventsByTrajectory = %v", ids(got))
		want := []string{"ev-before", "ev-folded", "ev-after"}
		var order []string
		for _, id := range ids(got) {
			for _, w := range want {
				if id == w {
					order = append(order, id)
				}
			}
		}
		if len(order) != len(want) || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
			t.Errorf("trajectory order = %v, want %v", order, want)
		}
	})

	t.Run("acceptance evidence url", func(t *testing.T) {
		// run_acceptance.go runEventEvidenceURL cites an event as
		// /api/runs/{id}/events?after=<seq-1>&limit=1 (floored at 0). Following
		// the URL for the folded event must return that event.
		cite := stored.Seq - 1
		if cite < 0 {
			cite = 0
		}
		got, err := s.ListEventsAfter(ctx, run.RunID, cite, 1)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("[M] evidence URL after=%d&limit=1 returns %v (cited ev-folded)", cite, ids(got))
		if len(got) != 1 || got[0].EventID != "ev-folded" {
			t.Errorf("evidence URL for run.completed returns %v, want [ev-folded]", ids(got))
		}
	})
}

// TestFoldedRunEventRetryTakesOneNumber covers the retry case: a caller that
// re-enters UpdateRunWithEvent with the same event (a replay, or a CAS retry
// above the store) must not take a second number, and must not write the event
// twice. The next ordinary append takes the very next number.
func TestFoldedRunEventRetryTakesOneNumber(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	run := lifecycleRunFixture(start, "run-seq-retry", types.RunRunning)
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID:    "project-seq-retry:" + start.TrajectoryID,
		TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: run,
	}
	project.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project activation: %v", err)
	}

	run.State = types.RunCompleted
	run.UpdatedAt = run.UpdatedAt.Add(time.Second)
	ev := &types.EventRecord{
		EventID: "ev-retry", RunID: run.RunID, AgentID: run.AgentID,
		OwnerID: run.OwnerID, TrajectoryID: run.TrajectoryID,
		Kind: types.EventRunCompleted, Timestamp: run.UpdatedAt,
	}
	if err := s.UpdateRunWithEvent(ctx, run, ev); err != nil {
		t.Fatalf("first update: %v", err)
	}
	first, firstStream := ev.Seq, ev.StreamSeq
	t.Logf("[M] first fold: seq=%d stream_seq=%d", first, firstStream)
	if first <= 0 || firstStream <= 0 {
		t.Fatalf("first fold left seq=%d stream_seq=%d, want both > 0", first, firstStream)
	}
	// The same event again: the caller keeps its pointer across a retry.
	if err := s.UpdateRunWithEvent(ctx, run, ev); err != nil {
		t.Fatalf("retry update: %v", err)
	}
	if ev.Seq != first || ev.StreamSeq != firstStream {
		t.Errorf("retry renumbered the event: seq %d -> %d, stream_seq %d -> %d", first, ev.Seq, firstStream, ev.StreamSeq)
	}
	next := &types.EventRecord{
		EventID: "ev-next", RunID: run.RunID, AgentID: run.AgentID,
		OwnerID: run.OwnerID, TrajectoryID: run.TrajectoryID,
		Kind: types.EventKind("test.next"), Timestamp: run.UpdatedAt.Add(time.Second),
	}
	if err := s.AppendEvent(ctx, next); err != nil {
		t.Fatalf("append next: %v", err)
	}
	t.Logf("[M] next append: seq=%d stream_seq=%d", next.Seq, next.StreamSeq)
	if next.Seq != first+1 || next.StreamSeq != firstStream+1 {
		t.Errorf("next append took seq=%d stream_seq=%d, want %d and %d (one number per event)", next.Seq, next.StreamSeq, first+1, firstStream+1)
	}
	got, err := s.ListEventsAfter(ctx, run.RunID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, e := range got {
		if e.EventID == "ev-retry" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("ev-retry stored %d times in ListEventsAfter(0), want 1: %+v", count, got)
	}
}
