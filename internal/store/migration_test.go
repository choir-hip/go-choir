package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenWithOptionsDefersBackfillUntilExplicitResume(t *testing.T) {
	ctx := context.Background()
	s, err := OpenWithOptions(filepath.Join(t.TempDir(), "runtime.db"), OpenOptions{
		DeferObjectGraphBackfill: true,
	})
	if err != nil {
		t.Fatalf("open deferred store: %v", err)
	}
	defer func() { _ = s.Close() }()

	complete, err := s.ogBackfillMigrationComplete(ctx, ogKindRun)
	if err != nil {
		t.Fatalf("inspect deferred migration: %v", err)
	}
	if complete {
		t.Fatal("run migration marked complete before explicit resume")
	}

	if err := s.BackfillObjectGraph(ctx); err != nil {
		t.Fatalf("resume deferred migration: %v", err)
	}
	complete, err = s.ogBackfillMigrationComplete(ctx, ogKindRun)
	if err != nil {
		t.Fatalf("inspect completed migration: %v", err)
	}
	if !complete {
		t.Fatal("run migration not marked complete after explicit resume")
	}
}

func TestRunOGBackfillStepUsesDurableCompletionMarker(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.textureHandle().ExecContext(ctx, `DELETE FROM og_migrations WHERE migration_id = ?`, ogBackfillMigrationID(ogKindRun)); err != nil {
		t.Fatalf("clear run migration marker: %v", err)
	}

	firstCalls := 0
	if err := s.runOGBackfillStep(ctx, "first-test", ogKindRun, func(context.Context) error {
		firstCalls++
		return nil
	}); err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if firstCalls != 1 {
		t.Fatalf("first backfill calls = %d, want 1", firstCalls)
	}

	if _, err := s.ogPut(ctx, ogKindRun, "owner-test", "existing", map[string]string{"state": "newer-og"}, map[string]any{"key": "existing"}, time.Now().UTC()); err != nil {
		t.Fatalf("seed populated kind: %v", err)
	}
	secondCalls := 0
	if err := s.runOGBackfillStep(ctx, "second-test", ogKindRun, func(context.Context) error {
		secondCalls++
		return nil
	}); err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if secondCalls != 0 {
		t.Fatalf("completed migration callback calls = %d, want 0", secondCalls)
	}
}

func TestRunOGBackfillStepDoesNotMarkFailedPassComplete(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.textureHandle().ExecContext(ctx, `DELETE FROM og_migrations WHERE migration_id = ?`, ogBackfillMigrationID(ogKindRun)); err != nil {
		t.Fatalf("clear run migration marker: %v", err)
	}

	wantErr := errors.New("interrupted")
	if err := s.runOGBackfillStep(ctx, "interrupted-test", ogKindRun, func(context.Context) error {
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("interrupted backfill error = %v, want %v", err, wantErr)
	}

	resumeCalls := 0
	if err := s.runOGBackfillStep(ctx, "resume-test", ogKindRun, func(context.Context) error {
		resumeCalls++
		return nil
	}); err != nil {
		t.Fatalf("resume backfill: %v", err)
	}
	if resumeCalls != 1 {
		t.Fatalf("resume backfill calls = %d, want 1", resumeCalls)
	}
}
