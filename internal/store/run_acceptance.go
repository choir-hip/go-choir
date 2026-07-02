package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// UpsertRunAcceptance stores a synthesized acceptance record. Callers should
// derive checkpoints from existing product/control evidence before upserting.
func (s *Store) UpsertRunAcceptance(ctx context.Context, rec types.RunAcceptanceRecord) (types.RunAcceptanceRecord, error) {
	if rec.OwnerID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("upsert run acceptance: user_id is required")
	}
	if rec.TargetMissionID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("upsert run acceptance: target_mission_id is required")
	}
	if rec.TrajectoryID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("upsert run acceptance: trajectory_id is required")
	}
	if rec.AcceptanceID == "" {
		rec.AcceptanceID = uuid.NewString()
	}
	if rec.AcceptanceLevel == "" {
		rec.AcceptanceLevel = types.RunAcceptanceDocsLevel
	}
	if rec.State == "" {
		rec.State = types.RunAcceptanceSynthesized
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if now.Before(rec.CreatedAt) {
		now = rec.CreatedAt
	}
	rec.UpdatedAt = now

	if s.og != nil {
		if err := s.CreateRunAcceptanceOG(ctx, rec); err != nil {
			return types.RunAcceptanceRecord{}, fmt.Errorf("upsert run acceptance: %w", err)
		}
		return rec, nil
	}

	checkpoints, err := marshalAcceptanceJSON(rec.Checkpoints)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("marshal acceptance checkpoints: %w", err)
	}
	invariants, err := marshalAcceptanceJSON(rec.InvariantChecks)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("marshal acceptance invariant checks: %w", err)
	}
	contracts, err := marshalAcceptanceJSON(rec.VerifierContracts)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("marshal acceptance verifier contracts: %w", err)
	}
	evidence, err := marshalAcceptanceJSON(rec.EvidenceRefs)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("marshal acceptance evidence refs: %w", err)
	}
	rollback, err := marshalAcceptanceJSON(rec.RollbackRefs)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("marshal acceptance rollback refs: %w", err)
	}
	risks, err := marshalAcceptanceJSON(rec.FailureResidualRisks)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("marshal acceptance residual risks: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO run_acceptances (
			acceptance_id, target_mission_id, source_prompt_or_objective,
			owner_id, desktop_id, trajectory_id, loop_id, authority_profile,
			base_sha, deployment_commit, ci_run_id, deploy_run_id, staging_url,
			health_commit, acceptance_level, vm_mode, gateway_provider_evidence,
			state, checkpoints_json, invariant_checks_json, verifier_contracts_json,
			evidence_refs_json, rollback_refs_json, failure_residual_risks_json,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			target_mission_id = VALUES(target_mission_id),
			source_prompt_or_objective = VALUES(source_prompt_or_objective),
			owner_id = VALUES(owner_id),
			desktop_id = VALUES(desktop_id),
			trajectory_id = VALUES(trajectory_id),
			loop_id = VALUES(loop_id),
			authority_profile = VALUES(authority_profile),
			base_sha = VALUES(base_sha),
			deployment_commit = VALUES(deployment_commit),
			ci_run_id = VALUES(ci_run_id),
			deploy_run_id = VALUES(deploy_run_id),
			staging_url = VALUES(staging_url),
			health_commit = VALUES(health_commit),
			acceptance_level = VALUES(acceptance_level),
			vm_mode = VALUES(vm_mode),
			gateway_provider_evidence = VALUES(gateway_provider_evidence),
			state = VALUES(state),
			checkpoints_json = VALUES(checkpoints_json),
			invariant_checks_json = VALUES(invariant_checks_json),
			verifier_contracts_json = VALUES(verifier_contracts_json),
			evidence_refs_json = VALUES(evidence_refs_json),
			rollback_refs_json = VALUES(rollback_refs_json),
			failure_residual_risks_json = VALUES(failure_residual_risks_json),
			updated_at = VALUES(updated_at)`,
		rec.AcceptanceID,
		rec.TargetMissionID,
		rec.SourcePromptObjective,
		rec.OwnerID,
		rec.DesktopID,
		rec.TrajectoryID,
		rec.RunID,
		rec.AuthorityProfile,
		rec.BaseSHA,
		rec.DeploymentCommit,
		rec.CIRunID,
		rec.DeployRunID,
		rec.StagingURL,
		rec.HealthCommit,
		rec.AcceptanceLevel,
		rec.VMMode,
		rec.GatewayProviderEvidence,
		rec.State,
		string(checkpoints),
		string(invariants),
		string(contracts),
		string(evidence),
		string(rollback),
		string(risks),
		rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("upsert run acceptance: %w", err)
	}
	return rec, nil
}

func (s *Store) GetRunAcceptance(ctx context.Context, ownerID, acceptanceID string) (types.RunAcceptanceRecord, error) {
	if ownerID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("get run acceptance: user_id is required")
	}
	if acceptanceID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("get run acceptance: acceptance_id is required")
	}
	if s.og != nil {
		rec, err := s.GetRunAcceptanceOG(ctx, ownerID, acceptanceID)
		if err == nil || err != ErrNotFound {
			return rec, err
		}
		// Fall through to SQL for legacy records.
	}
	row := s.db.QueryRowContext(ctx, runAcceptanceSelectSQL()+` WHERE owner_id = ? AND acceptance_id = ?`, ownerID, acceptanceID)
	return scanRunAcceptance(row)
}

func (s *Store) GetRunAcceptanceByID(ctx context.Context, acceptanceID string) (types.RunAcceptanceRecord, error) {
	if acceptanceID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("get run acceptance by id: acceptance_id is required")
	}
	row := s.db.QueryRowContext(ctx, runAcceptanceSelectSQL()+` WHERE acceptance_id = ?`, acceptanceID)
	return scanRunAcceptance(row)
}

func (s *Store) ListRunAcceptances(ctx context.Context, ownerID string, limit int) ([]types.RunAcceptanceRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list run acceptances: user_id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if s.og != nil {
		objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
			Kind:    ogKindRunAccept,
			OwnerID: ownerID,
			Limit:   limit,
		})
		if err != nil {
			return nil, fmt.Errorf("query run acceptances: %w", err)
		}
		acceptances := make([]types.RunAcceptanceRecord, 0, len(objs))
		for _, obj := range objs {
			var rec types.RunAcceptanceRecord
			if err := ogDecode(obj, &rec); err != nil {
				return nil, err
			}
			acceptances = append(acceptances, rec)
		}
		// Sort by updated_at DESC, created_at DESC.
		sort.Slice(acceptances, func(i, j int) bool {
			if !acceptances[i].UpdatedAt.Equal(acceptances[j].UpdatedAt) {
				return acceptances[i].UpdatedAt.After(acceptances[j].UpdatedAt)
			}
			return acceptances[i].CreatedAt.After(acceptances[j].CreatedAt)
		})
		if len(acceptances) > limit {
			acceptances = acceptances[:limit]
		}
		if len(acceptances) > 0 {
			return acceptances, nil
		}
		// Fall through to SQL if OG returned nothing.
	}
	rows, err := s.db.QueryContext(ctx,
		runAcceptanceSelectSQL()+` WHERE owner_id = ? ORDER BY updated_at DESC, created_at DESC LIMIT ?`,
		ownerID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query run acceptances: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRunAcceptances(rows)
}

func (s *Store) ListRunAcceptancesByTrajectory(ctx context.Context, ownerID, trajectoryID string, limit int) ([]types.RunAcceptanceRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list run acceptances by trajectory: user_id is required")
	}
	if trajectoryID == "" {
		return nil, fmt.Errorf("list run acceptances by trajectory: trajectory_id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if s.og != nil {
		acceptances, err := s.ListRunAcceptancesByTrajectoryOG(ctx, ownerID, trajectoryID, limit)
		if err == nil && len(acceptances) > 0 {
			return acceptances, nil
		}
		// Fall through to SQL if OG returned nothing.
	}
	rows, err := s.db.QueryContext(ctx,
		runAcceptanceSelectSQL()+` WHERE owner_id = ? AND trajectory_id = ? ORDER BY updated_at DESC, created_at DESC LIMIT ?`,
		ownerID,
		trajectoryID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query run acceptances by trajectory: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanRunAcceptances(rows)
}

func runAcceptanceSelectSQL() string {
	return `SELECT acceptance_id, target_mission_id, source_prompt_or_objective,
	       owner_id, desktop_id, trajectory_id, loop_id, authority_profile,
	       base_sha, deployment_commit, ci_run_id, deploy_run_id, staging_url,
	       health_commit, acceptance_level, vm_mode, gateway_provider_evidence,
	       state, checkpoints_json, invariant_checks_json, verifier_contracts_json,
	       evidence_refs_json, rollback_refs_json, failure_residual_risks_json,
	       created_at, updated_at
	  FROM run_acceptances`
}

func scanRunAcceptances(rows *sql.Rows) ([]types.RunAcceptanceRecord, error) {
	var records []types.RunAcceptanceRecord
	for rows.Next() {
		rec, err := scanRunAcceptance(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run acceptances: %w", err)
	}
	return records, nil
}

func scanRunAcceptance(row interface{ Scan(...any) error }) (types.RunAcceptanceRecord, error) {
	var rec types.RunAcceptanceRecord
	var checkpoints, invariants, contracts, evidence, rollback, risks string
	var createdAt, updatedAt string
	err := row.Scan(
		&rec.AcceptanceID,
		&rec.TargetMissionID,
		&rec.SourcePromptObjective,
		&rec.OwnerID,
		&rec.DesktopID,
		&rec.TrajectoryID,
		&rec.RunID,
		&rec.AuthorityProfile,
		&rec.BaseSHA,
		&rec.DeploymentCommit,
		&rec.CIRunID,
		&rec.DeployRunID,
		&rec.StagingURL,
		&rec.HealthCommit,
		&rec.AcceptanceLevel,
		&rec.VMMode,
		&rec.GatewayProviderEvidence,
		&rec.State,
		&checkpoints,
		&invariants,
		&contracts,
		&evidence,
		&rollback,
		&risks,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.RunAcceptanceRecord{}, ErrNotFound
		}
		return types.RunAcceptanceRecord{}, fmt.Errorf("scan run acceptance: %w", err)
	}
	if err := unmarshalAcceptanceJSON(checkpoints, &rec.Checkpoints); err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("decode acceptance checkpoints: %w", err)
	}
	if err := unmarshalAcceptanceJSON(invariants, &rec.InvariantChecks); err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("decode acceptance invariant checks: %w", err)
	}
	if err := unmarshalAcceptanceJSON(contracts, &rec.VerifierContracts); err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("decode acceptance verifier contracts: %w", err)
	}
	if err := unmarshalAcceptanceJSON(evidence, &rec.EvidenceRefs); err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("decode acceptance evidence refs: %w", err)
	}
	if err := unmarshalAcceptanceJSON(rollback, &rec.RollbackRefs); err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("decode acceptance rollback refs: %w", err)
	}
	if err := unmarshalAcceptanceJSON(risks, &rec.FailureResidualRisks); err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("decode acceptance residual risks: %w", err)
	}
	var errParse error
	rec.CreatedAt, errParse = time.Parse(time.RFC3339Nano, createdAt)
	if errParse != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("parse acceptance created_at: %w", errParse)
	}
	rec.UpdatedAt, errParse = time.Parse(time.RFC3339Nano, updatedAt)
	if errParse != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("parse acceptance updated_at: %w", errParse)
	}
	return rec, nil
}

func marshalAcceptanceJSON(v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if !json.Valid(data) {
		return json.RawMessage(`[]`), nil
	}
	return data, nil
}

func unmarshalAcceptanceJSON(raw string, out any) error {
	if raw == "" {
		raw = "[]"
	}
	if !json.Valid([]byte(raw)) {
		raw = "[]"
	}
	return json.Unmarshal([]byte(raw), out)
}
