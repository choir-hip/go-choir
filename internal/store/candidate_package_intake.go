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

// UpsertCandidatePackageIntake persists an evidence-only candidate-computer
// package intake record for owner review. It deliberately does not publish an
// AppChangePackage, create an adoption, promote a computer, or mutate routes.
func (s *Store) UpsertCandidatePackageIntake(ctx context.Context, rec types.CandidatePackageIntakeRecord) (types.CandidatePackageIntakeRecord, error) {
	s.candidatePackageIntakeMu.Lock()
	defer s.candidatePackageIntakeMu.Unlock()
	return s.upsertCandidatePackageIntakeLocked(ctx, rec)
}

func (s *Store) upsertCandidatePackageIntakeLocked(ctx context.Context, rec types.CandidatePackageIntakeRecord) (types.CandidatePackageIntakeRecord, error) {
	if rec.OwnerID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: owner_id is required")
	}
	if rec.CandidatePackageID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: candidate_package_id is required")
	}
	if rec.CandidatePackageManifestSHA256 == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: candidate_package_manifest_sha256 is required")
	}
	if rec.IntakeBoundary == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: intake_boundary is required")
	}
	if rec.IntakeID == "" {
		rec.IntakeID = uuid.NewString()
	}
	if rec.Status == "" {
		rec.Status = types.CandidatePackageIntakeOwnerReviewPending
	}
	if rec.OwnerReviewState == "" {
		rec.OwnerReviewState = types.CandidatePackageOwnerReviewRequired
	}
	if !rec.OwnerReviewRequired && rec.OwnerReviewState == types.CandidatePackageOwnerReviewRequired {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: owner_review_required cannot be false while owner_review_state is required")
	}
	adoptionBlockersJSON, err := candidatePackageIntakeJSONOrDefault("adoption_blockers_json", rec.AdoptionBlockersJSON, "[]")
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: %w", err)
	}
	contractsJSON, err := candidatePackageIntakeJSONOrDefault("verifier_contracts_json", rec.VerifierContractsJSON, "[]")
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: %w", err)
	}
	evidenceRefsJSON, err := candidatePackageIntakeJSONOrDefault("evidence_refs_json", rec.EvidenceRefsJSON, "[]")
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: %w", err)
	}
	requiredObservationsJSON, err := candidatePackageIntakeJSONOrDefault("required_observations_json", rec.RequiredObservationsJSON, "[]")
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: %w", err)
	}
	acceptanceJSON, err := candidatePackageIntakeJSONOrDefault("acceptance_json", rec.AcceptanceJSON, "{}")
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: %w", err)
	}
	if rec.AdoptionReady {
		if rec.Status != types.CandidatePackageIntakeOwnerApproved || rec.OwnerReviewState != types.CandidatePackageOwnerReviewApproved || rec.OwnerReviewRequired {
			return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: adoption_ready requires owner-approved review state")
		}
		var blockers []json.RawMessage
		if err := json.Unmarshal([]byte(adoptionBlockersJSON), &blockers); err != nil {
			return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: adoption_blockers_json is not an array")
		}
		if len(blockers) != 0 {
			return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: adoption_ready requires no adoption blockers")
		}
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	var existingOwnerID string
	err = s.db.QueryRowContext(ctx, `SELECT owner_id FROM candidate_package_intakes WHERE intake_id = ?`, rec.IntakeID).Scan(&existingOwnerID)
	if err == nil {
		if existingOwnerID != rec.OwnerID {
			return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: intake_id %q belongs to a different owner", rec.IntakeID)
		}
	} else if err != sql.ErrNoRows {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: check existing owner: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO candidate_package_intakes (
			intake_id, owner_id, candidate_package_id, candidate_package_manifest_sha256,
			source_computer_id, source_candidate_id, candidate_source_ref,
			intake_boundary, status, owner_review_state, owner_review_required,
			adoption_ready, adoption_blockers_json, verifier_contracts_json,
			evidence_refs_json, required_observations_json, acceptance_json, trace_id,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			owner_id = owner_id,
			candidate_package_id = VALUES(candidate_package_id),
			candidate_package_manifest_sha256 = VALUES(candidate_package_manifest_sha256),
			source_computer_id = VALUES(source_computer_id),
			source_candidate_id = VALUES(source_candidate_id),
			candidate_source_ref = VALUES(candidate_source_ref),
			intake_boundary = VALUES(intake_boundary),
			status = VALUES(status),
			owner_review_state = VALUES(owner_review_state),
			owner_review_required = VALUES(owner_review_required),
			adoption_ready = VALUES(adoption_ready),
			adoption_blockers_json = VALUES(adoption_blockers_json),
			verifier_contracts_json = VALUES(verifier_contracts_json),
			evidence_refs_json = VALUES(evidence_refs_json),
			required_observations_json = VALUES(required_observations_json),
			acceptance_json = VALUES(acceptance_json),
			trace_id = VALUES(trace_id),
			updated_at = VALUES(updated_at)`,
		rec.IntakeID,
		rec.OwnerID,
		rec.CandidatePackageID,
		rec.CandidatePackageManifestSHA256,
		rec.SourceComputerID,
		rec.SourceCandidateID,
		rec.CandidateSourceRef,
		rec.IntakeBoundary,
		rec.Status,
		rec.OwnerReviewState,
		rec.OwnerReviewRequired,
		rec.AdoptionReady,
		adoptionBlockersJSON,
		contractsJSON,
		evidenceRefsJSON,
		requiredObservationsJSON,
		acceptanceJSON,
		rec.TraceID,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("upsert candidate package intake: %w", err)
	}
	rec.AdoptionBlockersJSON = json.RawMessage(adoptionBlockersJSON)
	rec.VerifierContractsJSON = json.RawMessage(contractsJSON)
	rec.EvidenceRefsJSON = json.RawMessage(evidenceRefsJSON)
	rec.RequiredObservationsJSON = json.RawMessage(requiredObservationsJSON)
	rec.AcceptanceJSON = json.RawMessage(acceptanceJSON)
	return rec, nil
}

// UpdateCandidatePackageIntakeIfCurrent persists an already-loaded intake only
// if its owner and updated_at value still match the stored row. It protects
// owner-review/adoption-boundary transitions from stale load/validate/mutate
// cycles; callers must pass the UpdatedAt value observed before mutation.
func (s *Store) UpdateCandidatePackageIntakeIfCurrent(ctx context.Context, rec types.CandidatePackageIntakeRecord, previousUpdatedAt time.Time) (types.CandidatePackageIntakeRecord, error) {
	if rec.OwnerID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("update candidate package intake: owner_id is required")
	}
	if rec.IntakeID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("update candidate package intake: intake_id is required")
	}
	if previousUpdatedAt.IsZero() {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("update candidate package intake: previous updated_at is required")
	}
	s.candidatePackageIntakeMu.Lock()
	defer s.candidatePackageIntakeMu.Unlock()

	current, err := s.getCandidatePackageIntakeLocked(ctx, rec.OwnerID, rec.IntakeID)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, err
	}
	if !current.UpdatedAt.Equal(previousUpdatedAt) {
		return current, fmt.Errorf("update candidate package intake: %w", ErrConcurrentStateChange)
	}
	rec.CreatedAt = current.CreatedAt
	rec.UpdatedAt = time.Now().UTC()
	return s.upsertCandidatePackageIntakeLocked(ctx, rec)
}

func candidatePackageIntakeJSONOrDefault(name string, raw json.RawMessage, fallback string) (string, error) {
	if len(raw) == 0 {
		return fallback, nil
	}
	if !json.Valid(raw) {
		return "", fmt.Errorf("%s is not valid JSON", name)
	}
	return string(raw), nil
}

func (s *Store) getCandidatePackageIntakeLocked(ctx context.Context, ownerID, intakeID string) (types.CandidatePackageIntakeRecord, error) {
	row := s.db.QueryRowContext(ctx, candidatePackageIntakeSelectSQL()+` WHERE owner_id = ? AND intake_id = ?`, ownerID, intakeID)
	return scanCandidatePackageIntake(row)
}

func (s *Store) GetCandidatePackageIntake(ctx context.Context, ownerID, intakeID string) (types.CandidatePackageIntakeRecord, error) {
	if ownerID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("get candidate package intake: owner_id is required")
	}
	if intakeID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("get candidate package intake: intake_id is required")
	}
	row := s.db.QueryRowContext(ctx, candidatePackageIntakeSelectSQL()+` WHERE owner_id = ? AND intake_id = ?`, ownerID, intakeID)
	return scanCandidatePackageIntake(row)
}

func (s *Store) ListCandidatePackageIntakes(ctx context.Context, ownerID string, limit int) ([]types.CandidatePackageIntakeRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list candidate package intakes: owner_id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, candidatePackageIntakeSelectSQL()+` WHERE owner_id = ? ORDER BY updated_at DESC, created_at DESC LIMIT ?`, ownerID, limit)
	if err != nil {
		return nil, fmt.Errorf("query candidate package intakes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []types.CandidatePackageIntakeRecord
	for rows.Next() {
		rec, err := scanCandidatePackageIntake(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidate package intakes: %w", err)
	}
	return out, nil
}

func scanCandidatePackageIntake(row interface{ Scan(...any) error }) (types.CandidatePackageIntakeRecord, error) {
	var rec types.CandidatePackageIntakeRecord
	var adoptionBlockersJSON, contractsJSON, evidenceRefsJSON, requiredObservationsJSON, acceptanceJSON, createdAt, updatedAt string
	err := row.Scan(
		&rec.IntakeID,
		&rec.OwnerID,
		&rec.CandidatePackageID,
		&rec.CandidatePackageManifestSHA256,
		&rec.SourceComputerID,
		&rec.SourceCandidateID,
		&rec.CandidateSourceRef,
		&rec.IntakeBoundary,
		&rec.Status,
		&rec.OwnerReviewState,
		&rec.OwnerReviewRequired,
		&rec.AdoptionReady,
		&adoptionBlockersJSON,
		&contractsJSON,
		&evidenceRefsJSON,
		&requiredObservationsJSON,
		&acceptanceJSON,
		&rec.TraceID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.CandidatePackageIntakeRecord{}, ErrNotFound
		}
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("scan candidate package intake: %w", err)
	}
	rec.AdoptionBlockersJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(adoptionBlockersJSON), "[]"))
	rec.VerifierContractsJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(contractsJSON), "[]"))
	rec.EvidenceRefsJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(evidenceRefsJSON), "[]"))
	rec.RequiredObservationsJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(requiredObservationsJSON), "[]"))
	rec.AcceptanceJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(acceptanceJSON), "{}"))
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("parse candidate package intake created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("parse candidate package intake updated_at: %w", err)
	}
	return rec, nil
}

func candidatePackageIntakeSelectSQL() string {
	return `SELECT intake_id, owner_id, candidate_package_id,
		       candidate_package_manifest_sha256, source_computer_id,
		       source_candidate_id, candidate_source_ref, intake_boundary, status,
		       owner_review_state, owner_review_required, adoption_ready,
		       adoption_blockers_json, verifier_contracts_json, evidence_refs_json,
		       required_observations_json, acceptance_json, trace_id, created_at,
		       updated_at
		  FROM candidate_package_intakes`
}
