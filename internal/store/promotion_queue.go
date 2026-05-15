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

// UpsertPromotionCandidate creates or replaces the durable queue record for a
// candidate-world patchset. It does not verify or promote the candidate.
func (s *Store) UpsertPromotionCandidate(ctx context.Context, rec types.PromotionCandidateRecord) (types.PromotionCandidateRecord, error) {
	if rec.OwnerID == "" {
		return types.PromotionCandidateRecord{}, fmt.Errorf("upsert promotion candidate: owner_id is required")
	}
	if rec.CandidateID == "" {
		rec.CandidateID = uuid.NewString()
	}
	if rec.Status == "" {
		rec.Status = types.PromotionCandidateQueued
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	candidateJSON := rawJSONOrDefault(rec.CandidateJSON, "{}")
	contractsJSON := rawJSONOrDefault(rec.ContractsJSON, "[]")
	reportJSON := rawJSONOrDefault(rec.ReportJSON, "{}")

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO promotion_candidates (
			candidate_id, owner_id, status, source_loop_id, trace_id, vm_id,
			snapshot_id, base_sha, worker_head_sha, manifest_path, patchset_path,
			integration_branch, destination_branch, summary, candidate_json,
			contracts_json, report_json, error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			owner_id = VALUES(owner_id),
			status = VALUES(status),
			source_loop_id = VALUES(source_loop_id),
			trace_id = VALUES(trace_id),
			vm_id = VALUES(vm_id),
			snapshot_id = VALUES(snapshot_id),
			base_sha = VALUES(base_sha),
			worker_head_sha = VALUES(worker_head_sha),
			manifest_path = VALUES(manifest_path),
			patchset_path = VALUES(patchset_path),
			integration_branch = VALUES(integration_branch),
			destination_branch = VALUES(destination_branch),
			summary = VALUES(summary),
			candidate_json = VALUES(candidate_json),
			contracts_json = VALUES(contracts_json),
			report_json = VALUES(report_json),
			error = VALUES(error),
			updated_at = VALUES(updated_at)`,
		rec.CandidateID,
		rec.OwnerID,
		rec.Status,
		rec.SourceRunID,
		rec.TraceID,
		rec.VMID,
		rec.SnapshotID,
		rec.BaseSHA,
		rec.WorkerHeadSHA,
		rec.ManifestPath,
		rec.PatchsetPath,
		rec.IntegrationBranch,
		rec.DestinationBranch,
		rec.Summary,
		candidateJSON,
		contractsJSON,
		reportJSON,
		rec.Error,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("upsert promotion candidate: %w", err)
	}
	rec.CandidateJSON = json.RawMessage(candidateJSON)
	rec.ContractsJSON = json.RawMessage(contractsJSON)
	rec.ReportJSON = json.RawMessage(reportJSON)
	return rec, nil
}

// UpdatePromotionCandidate updates an existing candidate queue record.
func (s *Store) UpdatePromotionCandidate(ctx context.Context, rec types.PromotionCandidateRecord) (types.PromotionCandidateRecord, error) {
	if rec.OwnerID == "" {
		return types.PromotionCandidateRecord{}, fmt.Errorf("update promotion candidate: owner_id is required")
	}
	if rec.CandidateID == "" {
		return types.PromotionCandidateRecord{}, fmt.Errorf("update promotion candidate: candidate_id is required")
	}
	if rec.Status == "" {
		return types.PromotionCandidateRecord{}, fmt.Errorf("update promotion candidate: status is required")
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = time.Now().UTC()
	}
	candidateJSON := rawJSONOrDefault(rec.CandidateJSON, "{}")
	contractsJSON := rawJSONOrDefault(rec.ContractsJSON, "[]")
	reportJSON := rawJSONOrDefault(rec.ReportJSON, "{}")

	res, err := s.db.ExecContext(ctx,
		`UPDATE promotion_candidates
		    SET status = ?,
		        source_loop_id = ?,
		        trace_id = ?,
		        vm_id = ?,
		        snapshot_id = ?,
		        base_sha = ?,
		        worker_head_sha = ?,
		        manifest_path = ?,
		        patchset_path = ?,
		        integration_branch = ?,
		        destination_branch = ?,
		        summary = ?,
		        candidate_json = ?,
		        contracts_json = ?,
		        report_json = ?,
		        error = ?,
		        updated_at = ?
		  WHERE owner_id = ? AND candidate_id = ?`,
		rec.Status,
		rec.SourceRunID,
		rec.TraceID,
		rec.VMID,
		rec.SnapshotID,
		rec.BaseSHA,
		rec.WorkerHeadSHA,
		rec.ManifestPath,
		rec.PatchsetPath,
		rec.IntegrationBranch,
		rec.DestinationBranch,
		rec.Summary,
		candidateJSON,
		contractsJSON,
		reportJSON,
		rec.Error,
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		rec.OwnerID,
		rec.CandidateID,
	)
	if err != nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("update promotion candidate: %w", err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("update promotion candidate rows: %w", err)
	}
	if count == 0 {
		return types.PromotionCandidateRecord{}, ErrNotFound
	}
	rec.CandidateJSON = json.RawMessage(candidateJSON)
	rec.ContractsJSON = json.RawMessage(contractsJSON)
	rec.ReportJSON = json.RawMessage(reportJSON)
	return rec, nil
}

// GetPromotionCandidate returns a queue record scoped by owner.
func (s *Store) GetPromotionCandidate(ctx context.Context, ownerID, candidateID string) (types.PromotionCandidateRecord, error) {
	if ownerID == "" {
		return types.PromotionCandidateRecord{}, fmt.Errorf("get promotion candidate: owner_id is required")
	}
	if candidateID == "" {
		return types.PromotionCandidateRecord{}, fmt.Errorf("get promotion candidate: candidate_id is required")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT candidate_id, owner_id, status, source_loop_id, trace_id, vm_id,
		        snapshot_id, base_sha, worker_head_sha, manifest_path, patchset_path,
		        integration_branch, destination_branch, summary, candidate_json,
		        contracts_json, report_json, error, created_at, updated_at
		   FROM promotion_candidates
		  WHERE owner_id = ? AND candidate_id = ?`,
		ownerID,
		candidateID,
	)
	return scanPromotionCandidate(row)
}

// ListPromotionCandidates returns recent candidate queue records for an owner.
func (s *Store) ListPromotionCandidates(ctx context.Context, ownerID string, limit int) ([]types.PromotionCandidateRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list promotion candidates: owner_id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT candidate_id, owner_id, status, source_loop_id, trace_id, vm_id,
		        snapshot_id, base_sha, worker_head_sha, manifest_path, patchset_path,
		        integration_branch, destination_branch, summary, candidate_json,
		        contracts_json, report_json, error, created_at, updated_at
		   FROM promotion_candidates
		  WHERE owner_id = ?
		  ORDER BY updated_at DESC, created_at DESC
		  LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query promotion candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var candidates []types.PromotionCandidateRecord
	for rows.Next() {
		rec, err := scanPromotionCandidate(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate promotion candidates: %w", err)
	}
	return candidates, nil
}

func scanPromotionCandidate(row interface{ Scan(...any) error }) (types.PromotionCandidateRecord, error) {
	var rec types.PromotionCandidateRecord
	var candidateJSON, contractsJSON, reportJSON, createdAt, updatedAt string
	err := row.Scan(
		&rec.CandidateID,
		&rec.OwnerID,
		&rec.Status,
		&rec.SourceRunID,
		&rec.TraceID,
		&rec.VMID,
		&rec.SnapshotID,
		&rec.BaseSHA,
		&rec.WorkerHeadSHA,
		&rec.ManifestPath,
		&rec.PatchsetPath,
		&rec.IntegrationBranch,
		&rec.DestinationBranch,
		&rec.Summary,
		&candidateJSON,
		&contractsJSON,
		&reportJSON,
		&rec.Error,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.PromotionCandidateRecord{}, ErrNotFound
		}
		return types.PromotionCandidateRecord{}, fmt.Errorf("scan promotion candidate: %w", err)
	}
	rec.CandidateJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(candidateJSON), "{}"))
	rec.ContractsJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(contractsJSON), "[]"))
	rec.ReportJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(reportJSON), "{}"))
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("parse promotion candidate created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.PromotionCandidateRecord{}, fmt.Errorf("parse promotion candidate updated_at: %w", err)
	}
	return rec, nil
}

func rawJSONOrDefault(raw json.RawMessage, fallback string) string {
	trimmed := string(raw)
	if trimmed == "" || !json.Valid([]byte(trimmed)) {
		return fallback
	}
	return trimmed
}
