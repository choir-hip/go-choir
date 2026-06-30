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

func (s *Store) UpsertComputerSourceLineage(ctx context.Context, rec types.ComputerSourceLineageRecord) (types.ComputerSourceLineageRecord, error) {
	if rec.OwnerID == "" {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("upsert source lineage: owner_id is required")
	}
	if rec.ComputerID == "" {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("upsert source lineage: computer_id is required")
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO computer_source_lineages (
			owner_id, computer_id, computer_kind, active_source_ref, runtime_digest,
			ui_digest, route_profile, default_base_profile, last_adoption_id,
			last_package_id, last_candidate_ref, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			computer_kind = VALUES(computer_kind),
			active_source_ref = VALUES(active_source_ref),
			runtime_digest = VALUES(runtime_digest),
			ui_digest = VALUES(ui_digest),
			route_profile = VALUES(route_profile),
			default_base_profile = VALUES(default_base_profile),
			last_adoption_id = VALUES(last_adoption_id),
			last_package_id = VALUES(last_package_id),
			last_candidate_ref = VALUES(last_candidate_ref),
			updated_at = VALUES(updated_at)`,
		rec.OwnerID,
		rec.ComputerID,
		rec.ComputerKind,
		rec.ActiveSourceRef,
		rec.RuntimeDigest,
		rec.UIDigest,
		rec.RouteProfile,
		rec.DefaultBaseProfile,
		rec.LastAdoptionID,
		rec.LastPackageID,
		rec.LastCandidateRef,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("upsert source lineage: %w", err)
	}
	return rec, nil
}

func (s *Store) GetComputerSourceLineage(ctx context.Context, ownerID, computerID string) (types.ComputerSourceLineageRecord, error) {
	if ownerID == "" {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("get source lineage: owner_id is required")
	}
	if computerID == "" {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("get source lineage: computer_id is required")
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT owner_id, computer_id, computer_kind, active_source_ref, runtime_digest,
		        ui_digest, route_profile, default_base_profile, last_adoption_id,
		        last_package_id, last_candidate_ref, created_at, updated_at
		   FROM computer_source_lineages
		  WHERE owner_id = ? AND computer_id = ?`,
		ownerID,
		computerID,
	)
	return scanComputerSourceLineage(row)
}

func (s *Store) UpsertAppChangePackage(ctx context.Context, rec types.AppChangePackageRecord) (types.AppChangePackageRecord, error) {
	if rec.OwnerID == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("upsert app change package: owner_id is required")
	}
	if rec.PackageID == "" {
		rec.PackageID = uuid.NewString()
	}
	if rec.Status == "" {
		rec.Status = types.AppChangePackagePublishedPrivate
	}
	if rec.Visibility == "" {
		rec.Visibility = "private"
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	manifestJSON := rawJSONOrDefault(rec.ManifestJSON, "{}")
	contractsJSON := rawJSONOrDefault(rec.VerifierContractsJSON, "[]")
	provenanceJSON := rawJSONOrDefault(rec.ProvenanceRefsJSON, "[]")
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_change_packages (
			package_id, owner_id, app_id, status, visibility, source_computer_id,
			source_candidate_id, source_active_ref, candidate_source_ref,
			runtime_source_delta, ui_source_delta, runtime_source_delta_sha256,
			ui_source_delta_sha256, package_manifest_sha256,
			app_protocol_contract, app_protocol_contract_sha256,
			source_runtime_artifact_digest, source_ui_artifact_digest,
			manifest_json, verifier_contracts_json, provenance_refs_json,
			trace_id, subject_id, subject_auth_method, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			owner_id = VALUES(owner_id),
			app_id = VALUES(app_id),
			status = VALUES(status),
			visibility = VALUES(visibility),
			source_computer_id = VALUES(source_computer_id),
			source_candidate_id = VALUES(source_candidate_id),
			source_active_ref = VALUES(source_active_ref),
			candidate_source_ref = VALUES(candidate_source_ref),
			runtime_source_delta = VALUES(runtime_source_delta),
			ui_source_delta = VALUES(ui_source_delta),
			runtime_source_delta_sha256 = VALUES(runtime_source_delta_sha256),
			ui_source_delta_sha256 = VALUES(ui_source_delta_sha256),
			package_manifest_sha256 = VALUES(package_manifest_sha256),
			app_protocol_contract = VALUES(app_protocol_contract),
			app_protocol_contract_sha256 = VALUES(app_protocol_contract_sha256),
			source_runtime_artifact_digest = VALUES(source_runtime_artifact_digest),
			source_ui_artifact_digest = VALUES(source_ui_artifact_digest),
			manifest_json = VALUES(manifest_json),
			verifier_contracts_json = VALUES(verifier_contracts_json),
			provenance_refs_json = VALUES(provenance_refs_json),
			trace_id = VALUES(trace_id),
			subject_id = VALUES(subject_id),
			subject_auth_method = VALUES(subject_auth_method),
			updated_at = VALUES(updated_at)`,
		rec.PackageID,
		rec.OwnerID,
		rec.AppID,
		rec.Status,
		rec.Visibility,
		rec.SourceComputerID,
		rec.SourceCandidateID,
		rec.SourceActiveRef,
		rec.CandidateSourceRef,
		rec.RuntimeSourceDelta,
		rec.UISourceDelta,
		rec.RuntimeSourceDeltaSHA256,
		rec.UISourceDeltaSHA256,
		rec.PackageManifestSHA256,
		rec.AppProtocolContract,
		rec.AppProtocolContractSHA256,
		rec.SourceRuntimeArtifactDigest,
		rec.SourceUIArtifactDigest,
		manifestJSON,
		contractsJSON,
		provenanceJSON,
		rec.TraceID,
		rec.SubjectID,
		rec.SubjectAuthMethod,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("upsert app change package: %w", err)
	}
	rec.ManifestJSON = json.RawMessage(manifestJSON)
	rec.VerifierContractsJSON = json.RawMessage(contractsJSON)
	rec.ProvenanceRefsJSON = json.RawMessage(provenanceJSON)
	return rec, nil
}

func (s *Store) GetAppChangePackage(ctx context.Context, packageID string) (types.AppChangePackageRecord, error) {
	if packageID == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("get app change package: package_id is required")
	}
	row := s.db.QueryRowContext(ctx, appChangePackageSelectSQL()+` WHERE package_id = ?`, packageID)
	return scanAppChangePackage(row)
}

func (s *Store) GetAppChangePackageForViewer(ctx context.Context, viewerID, packageID string) (types.AppChangePackageRecord, error) {
	rec, err := s.GetAppChangePackage(ctx, packageID)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	if rec.OwnerID == viewerID || rec.Visibility == "public" || rec.Visibility == "unlisted" || rec.Status == types.AppChangePackagePublishedPublic || rec.Status == types.AppChangePackagePublishedUnlisted {
		return rec, nil
	}
	return types.AppChangePackageRecord{}, ErrNotFound
}

func (s *Store) ListAppChangePackages(ctx context.Context, viewerID string, limit int) ([]types.AppChangePackageRecord, error) {
	if viewerID == "" {
		return nil, fmt.Errorf("list app change packages: owner_id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		appChangePackageSelectSQL()+` WHERE owner_id = ? OR visibility IN ('public', 'unlisted')
		  ORDER BY updated_at DESC, created_at DESC LIMIT ?`,
		viewerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query app change packages: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []types.AppChangePackageRecord
	for rows.Next() {
		rec, err := scanAppChangePackage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate app change packages: %w", err)
	}
	return out, nil
}

func (s *Store) UpsertAppAdoption(ctx context.Context, rec types.AppAdoptionRecord) (types.AppAdoptionRecord, error) {
	if rec.OwnerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("upsert app adoption: owner_id is required")
	}
	if rec.PackageID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("upsert app adoption: package_id is required")
	}
	if rec.TargetComputerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("upsert app adoption: target_computer_id is required")
	}
	if rec.AdoptionID == "" {
		rec.AdoptionID = uuid.NewString()
	}
	if rec.Status == "" {
		rec.Status = types.AppAdoptionProposed
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}
	conflictsJSON := rawJSONOrDefault(rec.MergeConflictsJSON, "[]")
	resultsJSON := rawJSONOrDefault(rec.VerifierResultsJSON, "[]")
	rollbackJSON := rawJSONOrDefault(rec.RollbackProfileJSON, "{}")
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO app_adoptions (
			adoption_id, owner_id, package_id, app_id, target_computer_id,
			target_computer_kind, target_candidate_id, status,
			target_active_source_ref_at_candidate_start,
			target_active_source_ref_at_cutover, candidate_source_ref,
			foreground_tail_merge_result, merge_strategy, merge_conflicts_json,
			runtime_artifact_digest, ui_artifact_digest, verifier_results_json,
			rollback_profile_json, route_profile, default_base_profile, trace_id,
			subject_id, subject_auth_method, error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			owner_id = VALUES(owner_id),
			package_id = VALUES(package_id),
			app_id = VALUES(app_id),
			target_computer_id = VALUES(target_computer_id),
			target_computer_kind = VALUES(target_computer_kind),
			target_candidate_id = VALUES(target_candidate_id),
			status = VALUES(status),
			target_active_source_ref_at_candidate_start = VALUES(target_active_source_ref_at_candidate_start),
			target_active_source_ref_at_cutover = VALUES(target_active_source_ref_at_cutover),
			candidate_source_ref = VALUES(candidate_source_ref),
			foreground_tail_merge_result = VALUES(foreground_tail_merge_result),
			merge_strategy = VALUES(merge_strategy),
			merge_conflicts_json = VALUES(merge_conflicts_json),
			runtime_artifact_digest = VALUES(runtime_artifact_digest),
			ui_artifact_digest = VALUES(ui_artifact_digest),
			verifier_results_json = VALUES(verifier_results_json),
			rollback_profile_json = VALUES(rollback_profile_json),
			route_profile = VALUES(route_profile),
			default_base_profile = VALUES(default_base_profile),
			trace_id = VALUES(trace_id),
			subject_id = VALUES(subject_id),
			subject_auth_method = VALUES(subject_auth_method),
			error = VALUES(error),
			updated_at = VALUES(updated_at)`,
		rec.AdoptionID,
		rec.OwnerID,
		rec.PackageID,
		rec.AppID,
		rec.TargetComputerID,
		rec.TargetComputerKind,
		rec.TargetCandidateID,
		rec.Status,
		rec.TargetActiveSourceRefAtCandidateStart,
		rec.TargetActiveSourceRefAtCutover,
		rec.CandidateSourceRef,
		rec.ForegroundTailMergeResult,
		rec.MergeStrategy,
		conflictsJSON,
		rec.RuntimeArtifactDigest,
		rec.UIArtifactDigest,
		resultsJSON,
		rollbackJSON,
		rec.RouteProfile,
		rec.DefaultBaseProfile,
		rec.TraceID,
		rec.SubjectID,
		rec.SubjectAuthMethod,
		rec.Error,
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("upsert app adoption: %w", err)
	}
	rec.MergeConflictsJSON = json.RawMessage(conflictsJSON)
	rec.VerifierResultsJSON = json.RawMessage(resultsJSON)
	rec.RollbackProfileJSON = json.RawMessage(rollbackJSON)
	return rec, nil
}

// PromotionTransactionInput carries the two records that a promotion must
// apply atomically: the adoption record (advanced to adopted/rolled-back) and
// the computer source lineage (advanced to the new active ref). The store
// commits both in a single database transaction so a failure between the two
// writes cannot leave a half-promoted state.
type PromotionTransactionInput struct {
	Adoption types.AppAdoptionRecord
	Lineage  types.ComputerSourceLineageRecord
}

// PromoteAppAdoptionTransaction applies the adoption status update and the
// lineage advancement in a single database transaction. Either both commits
// or neither commits — there is no half-promoted state. This is the
// transaction-semantics hardening for the promotion protected surface.
func (s *Store) PromoteAppAdoptionTransaction(ctx context.Context, in PromotionTransactionInput) (types.AppAdoptionRecord, types.ComputerSourceLineageRecord, error) {
	if s == nil || s.db == nil {
		return types.AppAdoptionRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("promote transaction: store is unavailable")
	}
	if in.Adoption.OwnerID == "" {
		return types.AppAdoptionRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("promote transaction: owner_id is required")
	}
	if in.Adoption.AdoptionID == "" {
		return types.AppAdoptionRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("promote transaction: adoption_id is required")
	}
	if in.Lineage.OwnerID == "" || in.Lineage.ComputerID == "" {
		return types.AppAdoptionRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("promote transaction: lineage owner/computer is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return types.AppAdoptionRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("promote transaction: begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	adoption := in.Adoption
	if adoption.UpdatedAt.IsZero() {
		adoption.UpdatedAt = now
	}
	lineage := in.Lineage
	lineage.UpdatedAt = now
	if lineage.CreatedAt.IsZero() {
		lineage.CreatedAt = now
	}

	conflictsJSON := rawJSONOrDefault(adoption.MergeConflictsJSON, "[]")
	resultsJSON := rawJSONOrDefault(adoption.VerifierResultsJSON, "[]")
	rollbackJSON := rawJSONOrDefault(adoption.RollbackProfileJSON, "{}")

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO app_adoptions (
			adoption_id, owner_id, package_id, app_id, target_computer_id,
			target_computer_kind, target_candidate_id, status,
			target_active_source_ref_at_candidate_start,
			target_active_source_ref_at_cutover, candidate_source_ref,
			foreground_tail_merge_result, merge_strategy, merge_conflicts_json,
			runtime_artifact_digest, ui_artifact_digest, verifier_results_json,
			rollback_profile_json, route_profile, default_base_profile, trace_id,
			subject_id, subject_auth_method, error, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			owner_id = VALUES(owner_id),
			package_id = VALUES(package_id),
			app_id = VALUES(app_id),
			target_computer_id = VALUES(target_computer_id),
			target_computer_kind = VALUES(target_computer_kind),
			target_candidate_id = VALUES(target_candidate_id),
			status = VALUES(status),
			target_active_source_ref_at_candidate_start = VALUES(target_active_source_ref_at_candidate_start),
			target_active_source_ref_at_cutover = VALUES(target_active_source_ref_at_cutover),
			candidate_source_ref = VALUES(candidate_source_ref),
			foreground_tail_merge_result = VALUES(foreground_tail_merge_result),
			merge_strategy = VALUES(merge_strategy),
			merge_conflicts_json = VALUES(merge_conflicts_json),
			runtime_artifact_digest = VALUES(runtime_artifact_digest),
			ui_artifact_digest = VALUES(ui_artifact_digest),
			verifier_results_json = VALUES(verifier_results_json),
			rollback_profile_json = VALUES(rollback_profile_json),
			route_profile = VALUES(route_profile),
			default_base_profile = VALUES(default_base_profile),
			trace_id = VALUES(trace_id),
			subject_id = VALUES(subject_id),
			subject_auth_method = VALUES(subject_auth_method),
			error = VALUES(error),
			updated_at = VALUES(updated_at)`,
		adoption.AdoptionID,
		adoption.OwnerID,
		adoption.PackageID,
		adoption.AppID,
		adoption.TargetComputerID,
		adoption.TargetComputerKind,
		adoption.TargetCandidateID,
		adoption.Status,
		adoption.TargetActiveSourceRefAtCandidateStart,
		adoption.TargetActiveSourceRefAtCutover,
		adoption.CandidateSourceRef,
		adoption.ForegroundTailMergeResult,
		adoption.MergeStrategy,
		conflictsJSON,
		adoption.RuntimeArtifactDigest,
		adoption.UIArtifactDigest,
		resultsJSON,
		rollbackJSON,
		adoption.RouteProfile,
		adoption.DefaultBaseProfile,
		adoption.TraceID,
		adoption.SubjectID,
		adoption.SubjectAuthMethod,
		adoption.Error,
		adoption.CreatedAt.UTC().Format(time.RFC3339Nano),
		adoption.UpdatedAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return types.AppAdoptionRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("promote transaction: upsert adoption: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO computer_source_lineages (
			owner_id, computer_id, computer_kind, active_source_ref, runtime_digest,
			ui_digest, route_profile, default_base_profile, last_adoption_id,
			last_package_id, last_candidate_ref, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			computer_kind = VALUES(computer_kind),
			active_source_ref = VALUES(active_source_ref),
			runtime_digest = VALUES(runtime_digest),
			ui_digest = VALUES(ui_digest),
			route_profile = VALUES(route_profile),
			default_base_profile = VALUES(default_base_profile),
			last_adoption_id = VALUES(last_adoption_id),
			last_package_id = VALUES(last_package_id),
			last_candidate_ref = VALUES(last_candidate_ref),
			updated_at = VALUES(updated_at)`,
		lineage.OwnerID,
		lineage.ComputerID,
		lineage.ComputerKind,
		lineage.ActiveSourceRef,
		lineage.RuntimeDigest,
		lineage.UIDigest,
		lineage.RouteProfile,
		lineage.DefaultBaseProfile,
		lineage.LastAdoptionID,
		lineage.LastPackageID,
		lineage.LastCandidateRef,
		lineage.CreatedAt.UTC().Format(time.RFC3339Nano),
		lineage.UpdatedAt.UTC().Format(time.RFC3339Nano),
	); err != nil {
		return types.AppAdoptionRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("promote transaction: upsert lineage: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return types.AppAdoptionRecord{}, types.ComputerSourceLineageRecord{}, fmt.Errorf("promote transaction: commit: %w", err)
	}
	committed = true
	adoption.MergeConflictsJSON = json.RawMessage(conflictsJSON)
	adoption.VerifierResultsJSON = json.RawMessage(resultsJSON)
	adoption.RollbackProfileJSON = json.RawMessage(rollbackJSON)
	return adoption, lineage, nil
}

func (s *Store) GetAppAdoption(ctx context.Context, ownerID, adoptionID string) (types.AppAdoptionRecord, error) {
	if ownerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("get app adoption: owner_id is required")
	}
	if adoptionID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("get app adoption: adoption_id is required")
	}
	row := s.db.QueryRowContext(ctx, appAdoptionSelectSQL()+` WHERE owner_id = ? AND adoption_id = ?`, ownerID, adoptionID)
	return scanAppAdoption(row)
}

func (s *Store) ListAppAdoptions(ctx context.Context, ownerID string, limit int) ([]types.AppAdoptionRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list app adoptions: owner_id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		appAdoptionSelectSQL()+` WHERE owner_id = ? ORDER BY updated_at DESC, created_at DESC LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query app adoptions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []types.AppAdoptionRecord
	for rows.Next() {
		rec, err := scanAppAdoption(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate app adoptions: %w", err)
	}
	return out, nil
}

func scanComputerSourceLineage(row interface{ Scan(...any) error }) (types.ComputerSourceLineageRecord, error) {
	var rec types.ComputerSourceLineageRecord
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.OwnerID,
		&rec.ComputerID,
		&rec.ComputerKind,
		&rec.ActiveSourceRef,
		&rec.RuntimeDigest,
		&rec.UIDigest,
		&rec.RouteProfile,
		&rec.DefaultBaseProfile,
		&rec.LastAdoptionID,
		&rec.LastPackageID,
		&rec.LastCandidateRef,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.ComputerSourceLineageRecord{}, ErrNotFound
		}
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("scan source lineage: %w", err)
	}
	parsedCreated, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("parse source lineage created_at: %w", err)
	}
	rec.CreatedAt = parsedCreated
	parsedUpdated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.ComputerSourceLineageRecord{}, fmt.Errorf("parse source lineage updated_at: %w", err)
	}
	rec.UpdatedAt = parsedUpdated
	return rec, nil
}

func scanAppChangePackage(row interface{ Scan(...any) error }) (types.AppChangePackageRecord, error) {
	var rec types.AppChangePackageRecord
	var manifestJSON, contractsJSON, provenanceJSON, createdAt, updatedAt string
	err := row.Scan(
		&rec.PackageID,
		&rec.OwnerID,
		&rec.AppID,
		&rec.Status,
		&rec.Visibility,
		&rec.SourceComputerID,
		&rec.SourceCandidateID,
		&rec.SourceActiveRef,
		&rec.CandidateSourceRef,
		&rec.RuntimeSourceDelta,
		&rec.UISourceDelta,
		&rec.RuntimeSourceDeltaSHA256,
		&rec.UISourceDeltaSHA256,
		&rec.PackageManifestSHA256,
		&rec.AppProtocolContract,
		&rec.AppProtocolContractSHA256,
		&rec.SourceRuntimeArtifactDigest,
		&rec.SourceUIArtifactDigest,
		&manifestJSON,
		&contractsJSON,
		&provenanceJSON,
		&rec.TraceID,
		&rec.SubjectID,
		&rec.SubjectAuthMethod,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.AppChangePackageRecord{}, ErrNotFound
		}
		return types.AppChangePackageRecord{}, fmt.Errorf("scan app change package: %w", err)
	}
	rec.ManifestJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(manifestJSON), "{}"))
	rec.VerifierContractsJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(contractsJSON), "[]"))
	rec.ProvenanceRefsJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(provenanceJSON), "[]"))
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("parse app change package created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("parse app change package updated_at: %w", err)
	}
	return rec, nil
}

func scanAppAdoption(row interface{ Scan(...any) error }) (types.AppAdoptionRecord, error) {
	var rec types.AppAdoptionRecord
	var conflictsJSON, resultsJSON, rollbackJSON, createdAt, updatedAt string
	err := row.Scan(
		&rec.AdoptionID,
		&rec.OwnerID,
		&rec.PackageID,
		&rec.AppID,
		&rec.TargetComputerID,
		&rec.TargetComputerKind,
		&rec.TargetCandidateID,
		&rec.Status,
		&rec.TargetActiveSourceRefAtCandidateStart,
		&rec.TargetActiveSourceRefAtCutover,
		&rec.CandidateSourceRef,
		&rec.ForegroundTailMergeResult,
		&rec.MergeStrategy,
		&conflictsJSON,
		&rec.RuntimeArtifactDigest,
		&rec.UIArtifactDigest,
		&resultsJSON,
		&rollbackJSON,
		&rec.RouteProfile,
		&rec.DefaultBaseProfile,
		&rec.TraceID,
		&rec.SubjectID,
		&rec.SubjectAuthMethod,
		&rec.Error,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.AppAdoptionRecord{}, ErrNotFound
		}
		return types.AppAdoptionRecord{}, fmt.Errorf("scan app adoption: %w", err)
	}
	rec.MergeConflictsJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(conflictsJSON), "[]"))
	rec.VerifierResultsJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(resultsJSON), "[]"))
	rec.RollbackProfileJSON = json.RawMessage(rawJSONOrDefault(json.RawMessage(rollbackJSON), "{}"))
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("parse app adoption created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("parse app adoption updated_at: %w", err)
	}
	return rec, nil
}

func appChangePackageSelectSQL() string {
	return `SELECT package_id, owner_id, app_id, status, visibility,
		       source_computer_id, source_candidate_id, source_active_ref,
		       candidate_source_ref, runtime_source_delta, ui_source_delta,
		       runtime_source_delta_sha256, ui_source_delta_sha256,
		       package_manifest_sha256, app_protocol_contract,
		       app_protocol_contract_sha256, source_runtime_artifact_digest,
		       source_ui_artifact_digest, manifest_json, verifier_contracts_json,
		       provenance_refs_json, trace_id, subject_id, subject_auth_method,
		       created_at, updated_at
		  FROM app_change_packages`
}

func appAdoptionSelectSQL() string {
	return `SELECT adoption_id, owner_id, package_id, app_id, target_computer_id,
		       target_computer_kind, target_candidate_id, status,
		       target_active_source_ref_at_candidate_start,
		       target_active_source_ref_at_cutover, candidate_source_ref,
		       foreground_tail_merge_result, merge_strategy, merge_conflicts_json,
		       runtime_artifact_digest, ui_artifact_digest, verifier_results_json,
		       rollback_profile_json, route_profile, default_base_profile, trace_id,
		       subject_id, subject_auth_method, error, created_at, updated_at
		  FROM app_adoptions`
}
