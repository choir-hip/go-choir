package store

// MA3.5: the run record's label set and a concurrent state transition. Ways
// this could fail, written first:
//
//  1. A label persist reads the run, a terminal state lands, and the persist
//     writes its stale copy over it: the run is running again, or its result
//     is gone.
//  2. A stale writer reads the run, a label persist lands, and the writer's
//     copy overwrites the labels.
//  3. The retry loops never end, or give up and drop one of the two writes.
//
// The interleavings are driven by hooks between each writer's read and write,
// not by timers.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func seedLabelRun(t *testing.T, s *Store, runID string) types.RunRecord {
	t.Helper()
	now := time.Now().UTC()
	rec := types.RunRecord{RunID: runID, AgentID: "research:doc", OwnerID: "owner-lbl", ComputerID: "autoputer-lbl",
		State: types.RunRunning, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateRun(context.Background(), rec); err != nil {
		t.Fatalf("create run: %v", err)
	}
	return rec
}

func TestALabelPersistCannotOverwriteATerminalTransition(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	rec := seedLabelRun(t, s, "run-race-1")
	labels := types.NewLabelSet(types.OwnerPrivateLabel("capture"), types.LabelWorldUntrusted)

	fired := false
	runLabelsBeforeUnionWrite = func() {
		if fired {
			return
		}
		fired = true
		done := rec
		now := time.Now().UTC()
		done.State, done.Result, done.FinishedAt, done.UpdatedAt = types.RunCompleted, "the result", &now, now
		if err := s.UpdateRun(ctx, done); err != nil {
			t.Errorf("terminal transition: %v", err)
		}
	}
	t.Cleanup(func() { runLabelsBeforeUnionWrite = func() {} })

	got, err := s.UnionRunLabels(ctx, rec.OwnerID, rec.ComputerID, rec.RunID, labels)
	if err != nil || strings.Join(got, ",") != strings.Join(labels, ",") {
		t.Fatalf("union = %v, %v", got, err)
	}
	if !fired {
		t.Fatal("the transition never interleaved")
	}
	stored, err := s.GetRunByOwner(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != types.RunCompleted || stored.Result != "the result" || stored.FinishedAt == nil {
		t.Errorf("the terminal transition was lost: %+v", stored)
	}
	if strings.Join(stored.Labels, ",") != strings.Join(labels, ",") {
		t.Errorf("the labels were lost: %v", stored.Labels)
	}
}

func TestAStaleRunWriterCannotDropLabelsPersistedBetweenItsReadAndWrite(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	rec := seedLabelRun(t, s, "run-race-2")
	labels := types.NewLabelSet(types.OwnerPrivateLabel("document"))

	fired := false
	runUpdateAfterRead = func() {
		if fired {
			return
		}
		fired = true
		if _, err := s.UnionRunLabels(ctx, rec.OwnerID, rec.ComputerID, rec.RunID, labels); err != nil {
			t.Errorf("label persist: %v", err)
		}
	}
	t.Cleanup(func() { runUpdateAfterRead = func() {} })

	done := rec // a copy that knows nothing of the labels
	now := time.Now().UTC()
	done.State, done.FinishedAt, done.UpdatedAt = types.RunCompleted, &now, now
	if err := s.UpdateRun(ctx, done); err != nil {
		t.Fatalf("terminal transition: %v", err)
	}
	if !fired {
		t.Fatal("the label persist never interleaved")
	}
	stored, _ := s.GetRunByOwner(ctx, rec.OwnerID, rec.RunID)
	if stored.State != types.RunCompleted {
		t.Errorf("state = %s", stored.State)
	}
	if strings.Join(stored.Labels, ",") != strings.Join(labels, ",") {
		t.Errorf("a stale writer dropped the labels: %v", stored.Labels)
	}
}

func TestRunLabelsOnlyGrowAndAbsentRunIsNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	rec := seedLabelRun(t, s, "run-grow")
	a, b := types.NewLabelSet(types.LabelPlatform), types.NewLabelSet(types.LabelWorldUntrusted)
	for _, set := range []types.LabelSet{a, b, a} {
		if _, err := s.UnionRunLabels(ctx, rec.OwnerID, rec.ComputerID, rec.RunID, set); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.RunLabels(ctx, rec.OwnerID, rec.ComputerID, rec.RunID)
	if err != nil || strings.Join(got, ",") != strings.Join(a.Union(b), ",") {
		t.Errorf("labels = %v, %v", got, err)
	}
	if _, err := s.UnionRunLabels(ctx, rec.OwnerID, rec.ComputerID, "no-such-run", a); err == nil {
		t.Error("a label persist on an absent run succeeded")
	}
}

func TestTerminalizeRunSurvivesALabelPersistInItsWindow(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := startContinueTestDocument(t, s, "label-window", nil)
	queued := queueLifecycleUpdateFixture(t, s, start, "command-queue-label-window") // projects a pending desk run
	labels := types.NewLabelSet(types.OwnerPrivateLabel("capture"), types.LabelWorldUntrusted)

	attempts := 0
	terminalizeRunAfterRead = func() {
		attempts++
		if attempts > 1 {
			return
		}
		// A reducer commit persists the run's labels after the transition read
		// the run and before it writes: the first write must conflict.
		if _, err := s.UnionRunLabels(ctx, start.OwnerID, start.ComputerID, queued.SourceRunID, labels); err != nil {
			t.Errorf("label persist: %v", err)
		}
	}
	t.Cleanup(func() { terminalizeRunAfterRead = func() {} })

	term := types.TerminalizeRunRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "terminalize-label-window", TrajectoryID: start.TrajectoryID,
		AgentID: start.Agent.AgentID, RunID: queued.SourceRunID, TerminalState: types.RunCancelled, Reason: "cancelled",
	}
	term.CommandDigest, _ = ComputeTerminalizeRunDigest(term)
	if _, err := s.TerminalizeRun(ctx, term); err != nil {
		t.Fatalf("terminalize: %v", err)
	}
	if attempts < 2 {
		t.Fatalf("the transition ran %d time(s): the label persist never conflicted with it", attempts)
	}
	run, err := s.GetLifecycleRun(ctx, start.OwnerID, start.ComputerID, queued.SourceRunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.State != types.RunCancelled || run.FinishedAt == nil {
		t.Errorf("the run is not terminal: %s", run.State)
	}
	if strings.Join(run.Labels, ",") != strings.Join(labels, ",") {
		t.Errorf("labels = %v, want %v", run.Labels, labels)
	}
}
