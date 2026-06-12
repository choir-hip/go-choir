package store

import (
	"context"
	"fmt"
	"sync"
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
		"publish_ref": "platformd_publication:pub-1/ver-1",
		"edition_ref": "vtext_edition:wire/rev-1",
		"":            "ignored",
	})
	if err != nil {
		t.Fatalf("update trajectory subject refs: %v", err)
	}
	if updated.SubjectRefs["processor_key"] != "processor:global" {
		t.Fatalf("existing subject ref lost: %+v", updated.SubjectRefs)
	}
	if updated.SubjectRefs["publish_ref"] != "platformd_publication:pub-1/ver-1" {
		t.Fatalf("publish_ref missing: %+v", updated.SubjectRefs)
	}
	if updated.SubjectRefs["edition_ref"] != "vtext_edition:wire/rev-1" {
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
