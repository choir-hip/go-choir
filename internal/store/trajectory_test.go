package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTrajectoryCreateIsIdempotentAndKeepsFirstRecord(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	first, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   "traj-1",
		OwnerID:        "user-alice",
		Kind:           types.TrajectoryKindDocument,
		SubjectRefs:    map[string]string{"channel_id": "channel-1", "root_loop_id": "run-1"},
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true},
	})
	if err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	if first.Status != types.TrajectoryLive || first.Kind != types.TrajectoryKindDocument {
		t.Fatalf("unexpected first record: %+v", first)
	}

	// A second mint (e.g. a child spawn on the same trajectory) must keep
	// the first record, including its kind.
	second, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-1",
		OwnerID:      "user-alice",
		Kind:         types.TrajectoryKindTask,
	})
	if err != nil {
		t.Fatalf("re-mint trajectory: %v", err)
	}
	if second.Kind != types.TrajectoryKindDocument {
		t.Fatalf("re-mint overwrote kind: %+v", second)
	}
	if second.SubjectRefs["channel_id"] != "channel-1" {
		t.Fatalf("re-mint lost subject refs: %+v", second)
	}

	listed, err := s.ListTrajectoriesByOwner(ctx, "user-alice", 10)
	if err != nil {
		t.Fatalf("list trajectories: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("trajectories = %d, want 1", len(listed))
	}

	// Owner scoping: another owner cannot read it.
	if _, err := s.GetTrajectory(ctx, "user-bob", "traj-1"); err != ErrNotFound {
		t.Fatalf("cross-owner get = %v, want ErrNotFound", err)
	}
}

func TestTrajectoryStatusTransitionStampsSettledAt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-settle",
		OwnerID:      "user-alice",
		Kind:         types.TrajectoryKindPublication,
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	settled, err := s.UpdateTrajectoryStatus(ctx, "user-alice", "traj-settle", types.TrajectorySettled)
	if err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
	if settled.Status != types.TrajectorySettled || settled.SettledAt == nil {
		t.Fatalf("settled record missing status/settled_at: %+v", settled)
	}
}

func TestWorkItemFingerprintDedupAndOpenObligationsQuery(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	created, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-alice",
		TrajectoryID:         "traj-wi",
		Objective:            "port the continuation mechanics to work items",
		AuthorityProfile:     "vsuper",
		StepBudget:           50,
		TokenBudget:          200000,
		ObjectiveFingerprint: "fp-1",
		CreatedByRunID:       "run-1",
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	if created.Status != types.WorkItemOpen {
		t.Fatalf("created status = %s, want open", created.Status)
	}

	// Same fingerprint on the same trajectory dedupes to the existing item.
	duplicate, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-alice",
		TrajectoryID:         "traj-wi",
		Objective:            "port the continuation mechanics to work items (rephrased)",
		ObjectiveFingerprint: "fp-1",
	})
	if err != nil {
		t.Fatalf("create duplicate work item: %v", err)
	}
	if duplicate.WorkItemID != created.WorkItemID {
		t.Fatalf("duplicate = %s, want existing %s", duplicate.WorkItemID, created.WorkItemID)
	}

	// A different fingerprint creates a second item.
	other, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-alice",
		TrajectoryID:         "traj-wi",
		Objective:            "verify the port with example tests",
		ObjectiveFingerprint: "fp-2",
	})
	if err != nil {
		t.Fatalf("create second work item: %v", err)
	}

	open, err := s.ListWorkItemsByTrajectory(ctx, "user-alice", "traj-wi", true)
	if err != nil {
		t.Fatalf("list open work items: %v", err)
	}
	if len(open) != 2 {
		t.Fatalf("open work items = %d, want 2", len(open))
	}

	// Completing one removes it from the open-obligations query but not
	// from the full list, and completed items still block fingerprint
	// reuse (an objective done once is not reassigned).
	if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", other.WorkItemID, types.WorkItemCompleted); err != nil {
		t.Fatalf("complete work item: %v", err)
	}
	open, err = s.ListWorkItemsByTrajectory(ctx, "user-alice", "traj-wi", true)
	if err != nil {
		t.Fatalf("list open work items: %v", err)
	}
	if len(open) != 1 || open[0].WorkItemID != created.WorkItemID {
		t.Fatalf("open work items after completion = %+v, want only %s", open, created.WorkItemID)
	}
	all, err := s.ListWorkItemsByTrajectory(ctx, "user-alice", "traj-wi", false)
	if err != nil {
		t.Fatalf("list all work items: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("all work items = %d, want 2", len(all))
	}
	redo, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-alice",
		TrajectoryID:         "traj-wi",
		Objective:            "verify the port with example tests",
		ObjectiveFingerprint: "fp-2",
	})
	if err != nil {
		t.Fatalf("re-create completed work item: %v", err)
	}
	if redo.WorkItemID != other.WorkItemID {
		t.Fatalf("completed fingerprint was reassigned: %+v", redo)
	}

	// Cancelled items release their fingerprint.
	if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", created.WorkItemID, types.WorkItemCancelled); err != nil {
		t.Fatalf("cancel work item: %v", err)
	}
	again, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-alice",
		TrajectoryID:         "traj-wi",
		Objective:            "port the continuation mechanics to work items",
		ObjectiveFingerprint: "fp-1",
	})
	if err != nil {
		t.Fatalf("re-create cancelled work item: %v", err)
	}
	if again.WorkItemID == created.WorkItemID {
		t.Fatalf("cancelled fingerprint was not released")
	}
}
