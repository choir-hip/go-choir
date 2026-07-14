package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

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

func TestTrajectorySubjectRefsMergePatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-refs",
		OwnerID:      "user-alice",
		Kind:         types.TrajectoryKindPublication,
		SubjectRefs:  map[string]string{"processor_key": "processor:global"},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	updated, err := s.UpdateTrajectorySubjectRefs(ctx, "user-alice", "traj-refs", map[string]string{
		"publish_ref": "corpusd_publication:pub-1/ver-1",
		"edition_ref": "texture_edition:wire/rev-1",
		"":            "ignored",
	})
	if err != nil {
		t.Fatalf("update trajectory subject refs: %v", err)
	}
	if updated.SubjectRefs["processor_key"] != "processor:global" {
		t.Fatalf("existing subject ref lost: %+v", updated.SubjectRefs)
	}
	if updated.SubjectRefs["publish_ref"] != "corpusd_publication:pub-1/ver-1" {
		t.Fatalf("publish_ref missing: %+v", updated.SubjectRefs)
	}
	if updated.SubjectRefs["edition_ref"] != "texture_edition:wire/rev-1" {
		t.Fatalf("edition_ref missing: %+v", updated.SubjectRefs)
	}
}

func TestTrajectorySubjectRefsConcurrentMergePatchesPreserveKeys(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-concurrent-refs",
		OwnerID:      "user-alice",
		Kind:         types.TrajectoryKindPublication,
		SubjectRefs:  map[string]string{"processor_key": "processor:global"},
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}

	const patches = 8
	var wg sync.WaitGroup
	errs := make(chan error, patches)
	for i := 0; i < patches; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.UpdateTrajectorySubjectRefs(ctx, "user-alice", "traj-concurrent-refs", map[string]string{
				fmt.Sprintf("ref_%02d", i): fmt.Sprintf("value-%02d", i),
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent subject ref patch: %v", err)
		}
	}

	got, err := s.GetTrajectory(ctx, "user-alice", "traj-concurrent-refs")
	if err != nil {
		t.Fatalf("get trajectory: %v", err)
	}
	if got.SubjectRefs["processor_key"] != "processor:global" {
		t.Fatalf("existing subject ref lost: %+v", got.SubjectRefs)
	}
	for i := 0; i < patches; i++ {
		key := fmt.Sprintf("ref_%02d", i)
		if got.SubjectRefs[key] != fmt.Sprintf("value-%02d", i) {
			t.Fatalf("subject ref %s = %q, want value-%02d; refs=%+v", key, got.SubjectRefs[key], i, got.SubjectRefs)
		}
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

func TestListOpenAssignedWorkItemsOnlyReturnsLiveAssignedOpenItems(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, rec := range []types.TrajectoryRecord{
		{OwnerID: "user-alice", TrajectoryID: "traj-live", Kind: types.TrajectoryKindTask},
		{OwnerID: "user-alice", TrajectoryID: "traj-settled", Kind: types.TrajectoryKindTask},
	} {
		if _, err := s.CreateTrajectoryIfAbsent(ctx, rec); err != nil {
			t.Fatalf("create trajectory %s: %v", rec.TrajectoryID, err)
		}
	}
	if _, err := s.UpdateTrajectoryStatus(ctx, "user-alice", "traj-settled", types.TrajectorySettled); err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}

	keeper, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:         "user-alice",
		TrajectoryID:    "traj-live",
		Objective:       "resume assigned open work",
		AssignedAgentID: "cosuper:assigned",
	})
	if err != nil {
		t.Fatalf("create keeper: %v", err)
	}
	if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "traj-live",
		Objective:    "unassigned work remains observable only",
	}); err != nil {
		t.Fatalf("create unassigned: %v", err)
	}
	completed, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:         "user-alice",
		TrajectoryID:    "traj-live",
		Objective:       "completed assigned work",
		AssignedAgentID: "cosuper:assigned",
	})
	if err != nil {
		t.Fatalf("create completed: %v", err)
	}
	if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", completed.WorkItemID, types.WorkItemCompleted); err != nil {
		t.Fatalf("complete work item: %v", err)
	}
	if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:         "user-alice",
		TrajectoryID:    "traj-settled",
		Objective:       "settled trajectory work",
		AssignedAgentID: "cosuper:settled",
	}); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("create settled work item error = %v, want ErrConcurrentStateChange", err)
	}

	got, err := s.ListOpenAssignedWorkItems(ctx, 20)
	if err != nil {
		t.Fatalf("list open assigned work items: %v", err)
	}
	if len(got) != 1 || got[0].WorkItemID != keeper.WorkItemID {
		t.Fatalf("open assigned work items = %+v, want only %s", got, keeper.WorkItemID)
	}
}

func TestWorkItemDetailsMergePatch(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	item, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-alice",
		TrajectoryID:         "traj-details",
		Objective:            "record a processor decision",
		ObjectiveFingerprint: "fp-details",
		Details: map[string]any{
			"kind":       "wire_processor_request_resolution",
			"request_id": "request-1",
		},
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	updated, err := s.UpdateWorkItemDetails(ctx, "user-alice", item.WorkItemID, map[string]any{
		"decision":         "already_covered",
		"decision_summary": "Existing coverage already satisfies this batch.",
	})
	if err != nil {
		t.Fatalf("update work item details: %v", err)
	}
	if updated.Details["kind"] != "wire_processor_request_resolution" || updated.Details["request_id"] != "request-1" {
		t.Fatalf("existing details lost after patch: %+v", updated.Details)
	}
	if updated.Details["decision"] != "already_covered" || updated.Details["decision_summary"] != "Existing coverage already satisfies this batch." {
		t.Fatalf("patched details missing: %+v", updated.Details)
	}
}

func TestWorkItemDetailsConcurrentMergePatchesPreserveKeys(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	item, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-alice",
		TrajectoryID:         "traj-concurrent-details",
		Objective:            "record concurrent processor decisions",
		ObjectiveFingerprint: "fp-concurrent-details",
		Details: map[string]any{
			"kind":       "wire_processor_request_resolution",
			"request_id": "request-concurrent",
		},
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}

	const patches = 8
	var wg sync.WaitGroup
	errs := make(chan error, patches)
	for i := 0; i < patches; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.UpdateWorkItemDetails(ctx, "user-alice", item.WorkItemID, map[string]any{
				fmt.Sprintf("decision_%02d", i): fmt.Sprintf("value-%02d", i),
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent work item detail patch: %v", err)
		}
	}

	got, err := s.GetWorkItem(ctx, "user-alice", item.WorkItemID)
	if err != nil {
		t.Fatalf("get work item: %v", err)
	}
	if got.Details["kind"] != "wire_processor_request_resolution" || got.Details["request_id"] != "request-concurrent" {
		t.Fatalf("existing details lost: %+v", got.Details)
	}
	for i := 0; i < patches; i++ {
		key := fmt.Sprintf("decision_%02d", i)
		if got.Details[key] != fmt.Sprintf("value-%02d", i) {
			t.Fatalf("detail %s = %q, want value-%02d; details=%+v", key, got.Details[key], i, got.Details)
		}
	}
}

func TestCancelTrajectoryAuthorityAtomicallyClosesOpenAuthority(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-cancel",
		OwnerID:      "user-alice",
		Kind:         types.TrajectoryKindTask,
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	openItem, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "traj-cancel",
		Objective:    "cancel me",
	})
	if err != nil {
		t.Fatalf("create open work item: %v", err)
	}
	completedItem, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "traj-cancel",
		Objective:    "leave me completed",
	})
	if err != nil {
		t.Fatalf("create completed work item: %v", err)
	}
	completedItem, err = s.UpdateWorkItemStatus(ctx, "user-alice", completedItem.WorkItemID, types.WorkItemCompleted)
	if err != nil {
		t.Fatalf("complete work item: %v", err)
	}
	preCancelledItem, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "traj-cancel",
		Objective:    "leave me cancelled",
	})
	if err != nil {
		t.Fatalf("create pre-cancelled work item: %v", err)
	}
	preCancelledItem, err = s.UpdateWorkItemStatus(ctx, "user-alice", preCancelledItem.WorkItemID, types.WorkItemCancelled)
	if err != nil {
		t.Fatalf("pre-cancel work item: %v", err)
	}

	cancelled, err := s.CancelTrajectoryAuthority(ctx, "user-alice", "traj-cancel")
	if err != nil {
		t.Fatalf("cancel trajectory authority: %v", err)
	}
	if cancelled.Status != types.TrajectoryCancelled {
		t.Fatalf("trajectory status = %q, want cancelled", cancelled.Status)
	}
	open, err := s.ListWorkItemsByTrajectory(ctx, "user-alice", "traj-cancel", true)
	if err != nil {
		t.Fatalf("list open work items: %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("open work items after cancellation = %+v, want none", open)
	}
	cancelledItem, err := s.GetWorkItem(ctx, "user-alice", openItem.WorkItemID)
	if err != nil {
		t.Fatalf("get cancelled work item: %v", err)
	}
	if cancelledItem.Status != types.WorkItemCancelled {
		t.Fatalf("work item status = %q, want cancelled", cancelledItem.Status)
	}
	if !cancelledItem.UpdatedAt.Equal(cancelled.UpdatedAt) {
		t.Fatalf("atomic transition timestamps differ: trajectory=%s item=%s", cancelled.UpdatedAt, cancelledItem.UpdatedAt)
	}
	stillCompleted, err := s.GetWorkItem(ctx, "user-alice", completedItem.WorkItemID)
	if err != nil {
		t.Fatalf("get completed work item: %v", err)
	}
	if stillCompleted.Status != types.WorkItemCompleted || !stillCompleted.UpdatedAt.Equal(completedItem.UpdatedAt) {
		t.Fatalf("completed work item was mutated: before=%+v after=%+v", completedItem, stillCompleted)
	}
	stillPreCancelled, err := s.GetWorkItem(ctx, "user-alice", preCancelledItem.WorkItemID)
	if err != nil {
		t.Fatalf("get pre-cancelled work item: %v", err)
	}
	if stillPreCancelled.Status != types.WorkItemCancelled || !stillPreCancelled.UpdatedAt.Equal(preCancelledItem.UpdatedAt) {
		t.Fatalf("cancelled work item was mutated: before=%+v after=%+v", preCancelledItem, stillPreCancelled)
	}

	again, err := s.CancelTrajectoryAuthority(ctx, "user-alice", "traj-cancel")
	if err != nil {
		t.Fatalf("repeat cancellation: %v", err)
	}
	if again.Status != types.TrajectoryCancelled || !again.UpdatedAt.Equal(cancelled.UpdatedAt) {
		t.Fatalf("repeat cancellation was not idempotent: first=%+v second=%+v", cancelled, again)
	}
	sameCancelledItem, err := s.UpdateWorkItemStatus(ctx, "user-alice", openItem.WorkItemID, types.WorkItemCancelled)
	if err != nil {
		t.Fatalf("repeat work item cancellation: %v", err)
	}
	if !sameCancelledItem.UpdatedAt.Equal(cancelledItem.UpdatedAt) {
		t.Fatalf("repeat work item cancellation changed timestamp: first=%s second=%s", cancelledItem.UpdatedAt, sameCancelledItem.UpdatedAt)
	}
	sameCancelledTrajectory, err := s.UpdateTrajectoryStatus(ctx, "user-alice", "traj-cancel", types.TrajectoryCancelled)
	if err != nil {
		t.Fatalf("repeat trajectory cancellation status: %v", err)
	}
	if !sameCancelledTrajectory.UpdatedAt.Equal(cancelled.UpdatedAt) {
		t.Fatalf("repeat trajectory cancellation status changed timestamp: first=%s second=%s", cancelled.UpdatedAt, sameCancelledTrajectory.UpdatedAt)
	}
	if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "traj-cancel",
		Objective:    "late authority",
	}); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("create work item after cancellation error = %v, want ErrConcurrentStateChange", err)
	}
	if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", openItem.WorkItemID, types.WorkItemCompleted); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("late completion error = %v, want ErrConcurrentStateChange", err)
	}
	if _, err := s.UpdateTrajectoryStatus(ctx, "user-alice", "traj-cancel", types.TrajectorySettled); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("settle cancelled trajectory error = %v, want ErrConcurrentStateChange", err)
	}
	if _, err := s.CancelTrajectoryAuthority(ctx, "user-bob", "traj-cancel"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner cancellation error = %v, want ErrNotFound", err)
	}
}

func TestCancelTrajectoryAuthorityKeepsSettledTrajectorySettled(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-already-settled",
		OwnerID:      "user-alice",
		Kind:         types.TrajectoryKindTask,
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	item, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "traj-already-settled",
		Objective:    "existing obligation",
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	settled, err := s.UpdateTrajectoryStatus(ctx, "user-alice", "traj-already-settled", types.TrajectorySettled)
	if err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
	settledAgain, err := s.UpdateTrajectoryStatus(ctx, "user-alice", "traj-already-settled", types.TrajectorySettled)
	if err != nil {
		t.Fatalf("repeat settlement: %v", err)
	}
	if !settledAgain.UpdatedAt.Equal(settled.UpdatedAt) {
		t.Fatalf("repeat settlement changed timestamp: first=%s second=%s", settled.UpdatedAt, settledAgain.UpdatedAt)
	}

	got, err := s.CancelTrajectoryAuthority(ctx, "user-alice", "traj-already-settled")
	if err != nil {
		t.Fatalf("cancel settled trajectory: %v", err)
	}
	if got.Status != types.TrajectorySettled || got.SettledAt == nil || !got.UpdatedAt.Equal(settled.UpdatedAt) {
		t.Fatalf("settled trajectory changed during cancellation: before=%+v after=%+v", settled, got)
	}
	stillOpen, err := s.GetWorkItem(ctx, "user-alice", item.WorkItemID)
	if err != nil {
		t.Fatalf("get settled trajectory work item: %v", err)
	}
	if stillOpen.Status != types.WorkItemOpen {
		t.Fatalf("settled terminal fast path mutated work item: %+v", stillOpen)
	}
}

func TestListOpenWorkItemsByKindReturnsRecoveryMarkers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	orphan, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "missing-trajectory",
		Objective:    "retry publication",
		Details:      map[string]any{"kind": "wire_publication"},
	})
	if err != nil {
		t.Fatalf("create orphan marker: %v", err)
	}
	completed, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "missing-trajectory",
		Objective:    "already published",
		Details:      map[string]any{"kind": "wire_publication"},
	})
	if err != nil {
		t.Fatalf("create completed marker: %v", err)
	}
	if _, err := s.UpdateWorkItemStatus(ctx, "user-alice", completed.WorkItemID, types.WorkItemCompleted); err != nil {
		t.Fatalf("complete marker: %v", err)
	}
	if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:      "user-alice",
		TrajectoryID: "missing-trajectory",
		Objective:    "different recovery kind",
		Details:      map[string]any{"kind": "other"},
	}); err != nil {
		t.Fatalf("create other marker: %v", err)
	}

	got, err := s.ListOpenWorkItemsByKind(ctx, "wire_publication", 0)
	if err != nil {
		t.Fatalf("list open work items by kind: %v", err)
	}
	if len(got) != 1 || got[0].WorkItemID != orphan.WorkItemID {
		t.Fatalf("recovery markers = %+v, want only orphan %s", got, orphan.WorkItemID)
	}
}

func TestCancelTrajectoryAuthorityPagesAllOldOpenWorkItems(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	const itemCount = ogMetadataPageSize + 17
	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-many-old-open",
		OwnerID:      "user-alice",
		Kind:         types.TrajectoryKindTask,
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	itemIDs := make([]string, 0, itemCount)
	for i := range itemCount {
		itemID := fmt.Sprintf("old-open-%04d", i)
		if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
			WorkItemID:   itemID,
			OwnerID:      "user-alice",
			TrajectoryID: "traj-many-old-open",
			Objective:    fmt.Sprintf("old obligation %d", i),
			CreatedAt:    old.Add(time.Duration(i) * time.Second),
			UpdatedAt:    old.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("create old open work item %d: %v", i, err)
		}
		itemIDs = append(itemIDs, itemID)
	}

	if _, err := s.CancelTrajectoryAuthority(ctx, "user-alice", "traj-many-old-open"); err != nil {
		t.Fatalf("cancel trajectory authority: %v", err)
	}
	for _, itemID := range itemIDs {
		item, err := s.GetWorkItem(ctx, "user-alice", itemID)
		if err != nil {
			t.Fatalf("get work item %s: %v", itemID, err)
		}
		if item.Status != types.WorkItemCancelled {
			t.Fatalf("work item %s status = %q, want cancelled", itemID, item.Status)
		}
	}
}

func TestListOpenWorkItemsByKindPagesAllOldRecoveryMarkers(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	const markerCount = ogMetadataPageSize + 17
	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	for i := range markerCount {
		if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
			WorkItemID:   fmt.Sprintf("old-recovery-%04d", i),
			OwnerID:      "user-alice",
			TrajectoryID: "missing-recovery-trajectory",
			Objective:    fmt.Sprintf("recover old marker %d", i),
			Details:      map[string]any{"kind": "wire_publication"},
			CreatedAt:    old.Add(time.Duration(i) * time.Second),
			UpdatedAt:    old.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("create old recovery marker %d: %v", i, err)
		}
	}
	for i := range 17 {
		if _, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
			WorkItemID:   fmt.Sprintf("other-recovery-%04d", i),
			OwnerID:      "user-alice",
			TrajectoryID: "missing-recovery-trajectory",
			Objective:    fmt.Sprintf("other recovery marker %d", i),
			Details:      map[string]any{"kind": "other"},
		}); err != nil {
			t.Fatalf("create other recovery marker %d: %v", i, err)
		}
	}

	got, err := s.ListOpenWorkItemsByKind(ctx, "wire_publication", 0)
	if err != nil {
		t.Fatalf("list old recovery markers: %v", err)
	}
	if len(got) != markerCount {
		t.Fatalf("recovery markers = %d, want %d", len(got), markerCount)
	}
	for _, item := range got {
		if kind, _ := item.Details["kind"].(string); kind != "wire_publication" {
			t.Fatalf("recovery marker %s kind = %q, want wire_publication", item.WorkItemID, kind)
		}
	}
}

func TestCreateRunRejectsActiveAdmissionToTerminalTrajectory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, trajectoryStatus := range []types.TrajectoryStatus{types.TrajectorySettled, types.TrajectoryCancelled} {
		trajectoryID := "traj-" + string(trajectoryStatus)
		if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
			TrajectoryID: trajectoryID,
			OwnerID:      "user-alice",
			Kind:         types.TrajectoryKindTask,
		}); err != nil {
			t.Fatalf("create %s trajectory: %v", trajectoryStatus, err)
		}
		if _, err := s.UpdateTrajectoryStatus(ctx, "user-alice", trajectoryID, trajectoryStatus); err != nil {
			t.Fatalf("make trajectory %s: %v", trajectoryStatus, err)
		}
		for _, runState := range []types.RunState{types.RunPending, types.RunRunning, types.RunBlocked} {
			runID := fmt.Sprintf("rejected-%s-%s", trajectoryStatus, runState)
			err := s.CreateRun(ctx, types.RunRecord{
				RunID:        runID,
				OwnerID:      "user-alice",
				TrajectoryID: trajectoryID,
				State:        runState,
			})
			if !errors.Is(err, ErrConcurrentStateChange) {
				t.Fatalf("create %s run on %s trajectory error = %v, want ErrConcurrentStateChange", runState, trajectoryStatus, err)
			}
			if _, err := s.GetRun(ctx, runID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("rejected run %s persisted: %v", runID, err)
			}
		}
	}
}

func TestUpdateRunRejectsReactivationOnTerminalTrajectory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for _, trajectoryStatus := range []types.TrajectoryStatus{types.TrajectorySettled, types.TrajectoryCancelled} {
		trajectoryID := "traj-reactivate-" + string(trajectoryStatus)
		runID := "passivated-" + string(trajectoryStatus)
		if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
			TrajectoryID: trajectoryID,
			OwnerID:      "user-alice",
			Kind:         types.TrajectoryKindTask,
		}); err != nil {
			t.Fatalf("create %s trajectory: %v", trajectoryStatus, err)
		}
		if err := s.CreateRun(ctx, types.RunRecord{
			RunID:        runID,
			OwnerID:      "user-alice",
			TrajectoryID: trajectoryID,
			State:        types.RunPassivated,
		}); err != nil {
			t.Fatalf("create passivated run: %v", err)
		}
		if _, err := s.UpdateTrajectoryStatus(ctx, "user-alice", trajectoryID, trajectoryStatus); err != nil {
			t.Fatalf("make trajectory %s: %v", trajectoryStatus, err)
		}
		rec, err := s.GetRun(ctx, runID)
		if err != nil {
			t.Fatalf("get passivated run: %v", err)
		}
		rec.State = types.RunPending
		if err := s.UpdateRun(ctx, rec); !errors.Is(err, ErrConcurrentStateChange) {
			t.Fatalf("reactivate run on %s trajectory error = %v, want ErrConcurrentStateChange", trajectoryStatus, err)
		}
		stored, err := s.GetRun(ctx, runID)
		if err != nil {
			t.Fatalf("get rejected reactivation: %v", err)
		}
		if stored.State != types.RunPassivated {
			t.Fatalf("rejected reactivation state = %s, want passivated", stored.State)
		}
	}
}

func TestCreateRunAllowsHistoricalAndMissingTrajectoryAuthority(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, err := s.CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-terminal-history",
		OwnerID:      "user-alice",
		Kind:         types.TrajectoryKindTask,
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	if _, err := s.UpdateTrajectoryStatus(ctx, "user-alice", "traj-terminal-history", types.TrajectorySettled); err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}

	allowed := []types.RunRecord{
		{RunID: "historical-completed", OwnerID: "user-alice", TrajectoryID: "traj-terminal-history", State: types.RunCompleted},
		{RunID: "historical-failed", OwnerID: "user-alice", TrajectoryID: "traj-terminal-history", State: types.RunFailed},
		{RunID: "historical-cancelled", OwnerID: "user-alice", TrajectoryID: "traj-terminal-history", State: types.RunCancelled},
		{RunID: "active-without-trajectory", OwnerID: "user-alice", State: types.RunPending},
		{RunID: "active-with-missing-trajectory", OwnerID: "user-alice", TrajectoryID: "traj-missing", State: types.RunRunning},
		{RunID: "active-other-owner", OwnerID: "user-bob", TrajectoryID: "traj-terminal-history", State: types.RunBlocked},
	}
	for _, rec := range allowed {
		if err := s.CreateRun(ctx, rec); err != nil {
			t.Fatalf("create allowed run %s: %v", rec.RunID, err)
		}
		if _, err := s.GetRun(ctx, rec.RunID); err != nil {
			t.Fatalf("get allowed run %s: %v", rec.RunID, err)
		}
	}
}

func TestListActiveRunsByTrajectoryExhaustsPagesBeforeResultLimit(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	const activeCount = ogMetadataPageSize + 17
	states := []types.RunState{types.RunPending, types.RunRunning, types.RunBlocked}
	for i := range activeCount {
		if err := s.CreateRun(ctx, types.RunRecord{
			RunID:        fmt.Sprintf("active-trajectory-run-%04d", i),
			OwnerID:      "user-alice",
			TrajectoryID: "traj-many-active",
			State:        states[i%len(states)],
		}); err != nil {
			t.Fatalf("create active run %d: %v", i, err)
		}
	}
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID:        "terminal-trajectory-run",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-many-active",
		State:        types.RunCompleted,
	}); err != nil {
		t.Fatalf("create terminal run: %v", err)
	}
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID:        "other-owner-trajectory-run",
		OwnerID:      "user-bob",
		TrajectoryID: "traj-many-active",
		State:        types.RunPending,
	}); err != nil {
		t.Fatalf("create other-owner run: %v", err)
	}

	all, err := s.ListActiveRunsByTrajectory(ctx, "user-alice", "traj-many-active", 0)
	if err != nil {
		t.Fatalf("list every active trajectory run: %v", err)
	}
	if len(all) != activeCount {
		t.Fatalf("active trajectory runs = %d, want %d", len(all), activeCount)
	}
	limited, err := s.ListActiveRunsByTrajectory(ctx, "user-alice", "traj-many-active", 7)
	if err != nil {
		t.Fatalf("list limited active trajectory runs: %v", err)
	}
	if len(limited) != 7 {
		t.Fatalf("limited active trajectory runs = %d, want 7", len(limited))
	}
	for _, rec := range limited {
		if rec.OwnerID != "user-alice" || !rec.State.Active() {
			t.Fatalf("limited result contains ineligible run: %+v", rec)
		}
	}
}

func TestListActiveRunsByTrajectoryNormalizesLegacyBodyIdentity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const trajectoryID = "traj-legacy-body"

	rec := types.RunRecord{
		RunID:        "run-legacy-body",
		OwnerID:      "user-alice",
		TrajectoryID: trajectoryID,
		State:        types.RunPending,
		Metadata:     map[string]any{"trajectory_id": trajectoryID},
	}
	if err := s.CreateRunOG(ctx, rec); err != nil {
		t.Fatalf("create indexed legacy run: %v", err)
	}
	obj, err := s.ogGetByKey(ctx, ogKindRun, "run_id", rec.RunID)
	if err != nil {
		t.Fatalf("get indexed legacy object: %v", err)
	}
	rec.TrajectoryID = ""
	body, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal legacy run body: %v", err)
	}
	obj.Body = body
	if err := s.ogStore.PutObject(ctx, obj); err != nil {
		t.Fatalf("persist legacy run body: %v", err)
	}

	active, err := s.ListActiveRunsByTrajectory(ctx, rec.OwnerID, trajectoryID, 0)
	if err != nil {
		t.Fatalf("list legacy trajectory runs: %v", err)
	}
	if len(active) != 1 || active[0].RunID != rec.RunID || active[0].TrajectoryID != trajectoryID {
		t.Fatalf("legacy trajectory runs = %+v, want normalized %s", active, rec.RunID)
	}
}
