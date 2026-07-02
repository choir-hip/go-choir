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

	if s.og != nil {
		if err := s.CreateRunContinuationOG(ctx, rec); err != nil {
			return types.RunContinuationRecord{}, fmt.Errorf("insert run continuation: %w", err)
		}
		return rec, nil
	}

	detailsJSON, err := marshalJSON(rec.Details)
	if err != nil {
		return types.RunContinuationRecord{}, fmt.Errorf("marshal run continuation details: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO run_continuations (
			continuation_id, owner_id, source_loop_id, next_loop_id, objective,
			reason, authority_profile, lease_seconds, status, details_json,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ContinuationID,
		rec.OwnerID,
		rec.SourceRunID,
		rec.NextRunID,
		rec.Objective,
		rec.Reason,
		rec.AuthorityProfile,
		rec.LeaseSeconds,
		rec.Status,
		string(detailsJSON),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
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

	if s.og != nil {
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

	detailsJSON, err := marshalJSON(rec.Details)
	if err != nil {
		return types.RunContinuationRecord{}, fmt.Errorf("marshal run continuation details: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE run_continuations
		    SET next_loop_id = ?,
		        objective = ?,
		        reason = ?,
		        authority_profile = ?,
		        lease_seconds = ?,
		        status = ?,
		        details_json = ?,
		        updated_at = ?
		  WHERE owner_id = ? AND continuation_id = ?`,
		rec.NextRunID,
		rec.Objective,
		rec.Reason,
		rec.AuthorityProfile,
		rec.LeaseSeconds,
		rec.Status,
		string(detailsJSON),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		rec.OwnerID,
		rec.ContinuationID,
	)
	if err != nil {
		return types.RunContinuationRecord{}, fmt.Errorf("update run continuation: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return types.RunContinuationRecord{}, fmt.Errorf("update run continuation rows: %w", err)
	}
	if count == 0 {
		return types.RunContinuationRecord{}, ErrNotFound
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
	if s.og != nil {
		rec, err := s.GetRunContinuationOG(ctx, ownerID, continuationID)
		if err == nil || err != ErrNotFound {
			return rec, err
		}
		// Fall through to SQL for legacy records.
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT continuation_id, owner_id, source_loop_id, next_loop_id,
		        objective, reason, authority_profile, lease_seconds, status,
		        details_json, created_at, updated_at
		   FROM run_continuations
		  WHERE owner_id = ? AND continuation_id = ?`,
		ownerID,
		continuationID,
	)
	return scanRunContinuation(row)
}

// ListRunContinuationsBySource returns continuations selected from a source run.
func (s *Store) ListRunContinuationsBySource(ctx context.Context, ownerID, sourceRunID string) ([]types.RunContinuationRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list run continuations: owner_id is required")
	}
	if sourceRunID == "" {
		return nil, fmt.Errorf("list run continuations: source_loop_id is required")
	}
	if s.og != nil {
		conts, err := s.ListRunContinuationsBySourceRunOG(ctx, ownerID, sourceRunID, 500)
		if err == nil && len(conts) > 0 {
			return conts, nil
		}
		// Fall through to SQL if OG returned nothing.
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT continuation_id, owner_id, source_loop_id, next_loop_id,
		        objective, reason, authority_profile, lease_seconds, status,
		        details_json, created_at, updated_at
		   FROM run_continuations
		  WHERE owner_id = ? AND source_loop_id = ?
		  ORDER BY updated_at DESC, created_at DESC`,
		ownerID,
		sourceRunID,
	)
	if err != nil {
		return nil, fmt.Errorf("query run continuations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var continuations []types.RunContinuationRecord
	for rows.Next() {
		rec, err := scanRunContinuation(rows)
		if err != nil {
			return nil, err
		}
		continuations = append(continuations, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run continuations: %w", err)
	}
	return continuations, nil
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
