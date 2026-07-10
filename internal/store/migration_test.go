package store

import (
	"context"
	"testing"
	"time"
)

func TestRunOGBackfillStepRunsOnlyForEmptyKind(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	emptyKind := ogKindRun
	emptyCalls := 0
	if err := s.runOGBackfillStep(ctx, "empty-test", emptyKind, func(context.Context) error {
		emptyCalls++
		return nil
	}); err != nil {
		t.Fatalf("empty kind backfill: %v", err)
	}
	if emptyCalls != 1 {
		t.Fatalf("empty kind backfill calls = %d, want 1", emptyCalls)
	}

	populatedKind := ogKindRun
	if _, err := s.ogPut(ctx, populatedKind, "owner-test", "existing", map[string]string{"state": "newer-og"}, map[string]any{"key": "existing"}, time.Now().UTC()); err != nil {
		t.Fatalf("seed populated kind: %v", err)
	}
	populatedCalls := 0
	if err := s.runOGBackfillStep(ctx, "populated-test", populatedKind, func(context.Context) error {
		populatedCalls++
		return nil
	}); err != nil {
		t.Fatalf("populated kind backfill: %v", err)
	}
	if populatedCalls != 0 {
		t.Fatalf("populated kind backfill calls = %d, want 0", populatedCalls)
	}
}
