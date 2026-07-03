package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// CreateRunContinuation records a durable next objective selected from a run's
// compacted state. It does not start the next run.
func (s *Store) CreateRunContinuation(ctx context.Context, rec types.RunContinuationRecord) (types.RunContinuationRecord, error) {
	if rec.OwnerID == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("create run continuation: owner_id is required")
	}
	if rec.SourceRunID == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("create run continuation: source_loop_id is required")
	}
	if rec.Objective == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("create run continuation: objective is required")
	}
	if rec.ContinuationID == "" {
		rec.ContinuationID = uuid.NewString()
	}
	if rec.Status == "" {
		rec.Status = types.RunContinuationSelected
	}
	if rec.Details == nil {
		rec.Details = map[string]any{}
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}

	if err := s.CreateRunContinuationOG(ctx, rec); err != nil {
		return types.RunContinuationRecord{}, fmt.Errorf("insert run continuation: %w", err)
	}
	return rec, nil
}

// UpdateRunContinuation updates a selected continuation after it starts or
// becomes blocked.
func (s *Store) UpdateRunContinuation(ctx context.Context, rec types.RunContinuationRecord) (types.RunContinuationRecord, error) {
	if rec.OwnerID == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("update run continuation: owner_id is required")
	}
	if rec.ContinuationID == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("update run continuation: continuation_id is required")
	}
	if rec.Status == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("update run continuation: status is required")
	}
	if rec.Details == nil {
		rec.Details = map[string]any{}
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now().UTC()
	}

	// Check it exists first.
	if _, err := s.GetRunContinuationOG(ctx, rec.OwnerID, rec.ContinuationID); err != nil {
		if err == ErrNotFound {
			return types.RunContinuationRecord{}, ErrNotFound
		}
		return types.RunContinuationRecord{}, fmt.Errorf("update run continuation: %w", err)
	}
	// Upsert back to OG.
	if err := s.CreateRunContinuationOG(ctx, rec); err != nil {
		return types.RunContinuationRecord{}, fmt.Errorf("update run continuation: %w", err)
	}
	return rec, nil
}

// GetRunContinuation returns a continuation scoped by owner.
func (s *Store) GetRunContinuation(ctx context.Context, ownerID, continuationID string) (types.RunContinuationRecord, error) {
	if ownerID == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("get run continuation: owner_id is required")
	}
	if continuationID == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("get run continuation: continuation_id is required")
	}
	return s.GetRunContinuationOG(ctx, ownerID, continuationID)
}

// ListRunContinuationsBySource returns continuations selected from a source run.
func (s *Store) ListRunContinuationsBySource(ctx context.Context, ownerID, sourceRunID string) ([]types.RunContinuationRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list run continuations: owner_id is required")
	}
	if sourceRunID == "" {
		return nil, fmt.Errorf("list run continuations: source_loop_id is required")
	}
	return s.ListRunContinuationsBySourceRunOG(ctx, ownerID, sourceRunID, 500)
}

func scanRunContinuation(row interface{ Scan(...any) error }) (types.RunContinuationRecord, error) {
	var rec types.RunContinuationRecord
	var detailsJSON, createdAt, updatedAt string
	err := row.Scan(
		&rec.ContinuationID,
		&rec.OwnerID,
		&rec.SourceRunID,
		&rec.NextRunID,
		&rec.Objective,
		&rec.Reason,
		&rec.AuthorityProfile,
		&rec.LeaseSeconds,
		&rec.Status,
		&detailsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.RunContinuationRecord{}, ErrNotFound
		}
		return types.RunContinuationRecord{}, fmt.Errorf("scan run continuation: %w", err)
	}
	if detailsJSON != "" && detailsJSON != "{}" {
		if err := json.Unmarshal([]byte(detailsJSON), &rec.Details); err != nil {
			return types.RunContinuationRecord{}, fmt.Errorf("decode run continuation details: %w", err)
		}
	}
	if rec.Details == nil {
		rec.Details = map[string]any{}
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.RunContinuationRecord{}, fmt.Errorf("parse run continuation created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.RunContinuationRecord{}, fmt.Errorf("parse run continuation updated_at: %w", err)
	}
	return rec, nil
}
