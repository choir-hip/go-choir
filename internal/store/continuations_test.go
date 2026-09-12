package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRunContinuationsRecordSelectedAndStartedNextGoal(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	selected, err := s.CreateRunContinuation(ctx, types.RunContinuationRecord{
		OwnerID:          "owner-1",
		SourceRunID:      "run-source",
		Objective:        "continue with uploads after launcher proof",
		Reason:           "app adoption produced a verified candidate",
		AuthorityProfile: "super",
		LeaseSeconds:     3600,
		Details:          map[string]any{"mission": "choir-in-choir"},
	})
	if err != nil {
		t.Fatalf("create continuation: %v", err)
	}
	if selected.ContinuationID == "" {
		t.Fatalf("continuation id was not assigned")
	}

	loaded, err := s.GetRunContinuation(ctx, "owner-1", selected.ContinuationID)
	if err != nil {
		t.Fatalf("get continuation: %v", err)
	}
	if loaded.Status != types.RunContinuationSelected || loaded.AuthorityProfile != "super" {
		t.Fatalf("loaded continuation mismatch: %+v", loaded)
	}
	if _, err := s.GetRunContinuation(ctx, "owner-2", selected.ContinuationID); err != ErrNotFound {
		t.Fatalf("other owner get error = %v, want ErrNotFound", err)
	}

	loaded.Status = types.RunContinuationStarted
	loaded.NextRunID = "run-next"
	updated, err := s.UpdateRunContinuation(ctx, loaded)
	if err != nil {
		t.Fatalf("update continuation: %v", err)
	}
	if updated.NextRunID != "run-next" {
		t.Fatalf("next run not updated: %+v", updated)
	}

	continuations, err := s.ListRunContinuationsBySource(ctx, "owner-1", "run-source")
	if err != nil {
		t.Fatalf("list continuations: %v", err)
	}
	if len(continuations) != 1 || continuations[0].Status != types.RunContinuationStarted {
		t.Fatalf("list continuations mismatch: %+v", continuations)
	}
}
