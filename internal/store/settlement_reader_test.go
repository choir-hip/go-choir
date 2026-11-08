package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A run reaching a terminal state settles its trajectory inside the store.
// That bookkeeping reads Texture's document and revision as the store, never
// as an undeclared reader (d@e on 8dda5426f counted one pair per terminal
// run).
func TestTerminalRunSettlementDeclaresStoreReader(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	run := lifecycleRunFixture(start, "run-settlement-reader", types.RunPending)
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	settleInitialLifecycleWork(t, s, start, "command-work-before-settlement-reader")
	restore := objectgraph.SetModeForTest(objectgraph.WriterGateObserve)
	defer restore()
	objectgraph.ResetReaderGateCounts()
	defer objectgraph.ResetReaderGateCounts()
	run.State = types.RunCompleted
	run.UpdatedAt = time.Now().UTC()
	run.FinishedAt = &run.UpdatedAt
	if err := s.UpdateRun(context.Background(), run); err != nil {
		t.Fatalf("complete run: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(objectgraph.WithReader(ctx, objectgraph.WriterStore), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.Trajectory.Status != types.TrajectorySettled {
		t.Fatalf("terminal run did not settle the trajectory: %+v, %v", snapshot.Trajectory, err)
	}
	if counts := objectgraph.ReaderGateCounts(); len(counts) != 0 {
		t.Fatalf("settlement counted reader crossings: %v", counts)
	}
}
