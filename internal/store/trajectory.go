package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// CreateTrajectoryIfAbsent durably records a trajectory, keeping the first
// record when one already exists for the ID. Minting is idempotent because
// every run on a trajectory passes through a spawn point that attempts the
// mint; only the root spawn wins.
func (s *Store) CreateTrajectoryIfAbsent(ctx context.Context, rec types.TrajectoryRecord) (types.TrajectoryRecord, error) {
	rec.TrajectoryID = strings.TrimSpace(rec.TrajectoryID)
	if rec.TrajectoryID == "" {
		return types.TrajectoryRecord{}, fmt.Errorf("create trajectory: trajectory_id is required")
	}
	if rec.OwnerID == "" {
		return types.TrajectoryRecord{}, fmt.Errorf("create trajectory: owner_id is required")
	}
	if rec.Kind == "" {
		rec.Kind = types.TrajectoryKindTask
	}
	if rec.Status == "" {
		rec.Status = types.TrajectoryLive
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	if rec.UpdatedAt.IsZero() {
		rec.UpdatedAt = now
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()

	return s.CreateTrajectoryIfAbsentOG(ctx, rec)
}

const selectTrajectoryByID = `SELECT trajectory_id, owner_id, kind, subject_refs_json, status,
        settlement_rule_json, created_at, updated_at, settled_at
   FROM trajectories
  WHERE trajectory_id = ? AND owner_id = ?`

// GetTrajectory returns the trajectory with the given ID, owner-scoped.
func (s *Store) GetTrajectory(ctx context.Context, ownerID, trajectoryID string) (types.TrajectoryRecord, error) {
	return s.GetTrajectoryOG(ctx, ownerID, trajectoryID)
}

// ListTrajectoriesByOwner returns trajectories for the owner ordered by most
// recently updated.
func (s *Store) ListTrajectoriesByOwner(ctx context.Context, ownerID string, limit int) ([]types.TrajectoryRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.ListTrajectoriesByOwnerOG(ctx, ownerID, limit)
}

// UpdateTrajectoryStatus transitions a live trajectory to a terminal status.
// Repeating the stored status is idempotent; a terminal trajectory cannot be
// rewritten to a different terminal status.
func (s *Store) UpdateTrajectoryStatus(ctx context.Context, ownerID, trajectoryID string, status types.TrajectoryStatus) (types.TrajectoryRecord, error) {
	if exists, err := s.lifecycleTrajectoryExists(ctx, ownerID, trajectoryID); err != nil {
		return types.TrajectoryRecord{}, err
	} else if exists {
		return types.TrajectoryRecord{}, ErrLifecycleAuthorityRequired
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	return s.UpdateTrajectoryStatusOG(ctx, ownerID, trajectoryID, status)
}

// CancelTrajectoryAuthority atomically cancels a live trajectory and every
// open work item on it. Terminal trajectories are returned unchanged.
func (s *Store) CancelTrajectoryAuthority(ctx context.Context, ownerID, trajectoryID string) (types.TrajectoryRecord, error) {
	if exists, err := s.lifecycleTrajectoryExists(ctx, ownerID, trajectoryID); err != nil {
		return types.TrajectoryRecord{}, err
	} else if exists {
		return types.TrajectoryRecord{}, ErrLifecycleAuthorityRequired
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	return s.cancelTrajectoryAuthorityOG(ctx, ownerID, trajectoryID)
}

// UpdateTrajectorySubjectRefs merges the provided subject refs into the
// trajectory record and stamps updated_at. Empty keys or values are ignored.
// Merge patches are serialized within one Store instance so concurrent callers
// cannot drop each other's keys by overwriting the whole JSON object.
func (s *Store) UpdateTrajectorySubjectRefs(ctx context.Context, ownerID, trajectoryID string, patch map[string]string) (types.TrajectoryRecord, error) {
	if exists, err := s.lifecycleTrajectoryExists(ctx, ownerID, trajectoryID); err != nil {
		return types.TrajectoryRecord{}, err
	} else if exists {
		return types.TrajectoryRecord{}, ErrLifecycleAuthorityRequired
	}
	if len(patch) == 0 {
		return s.GetTrajectory(ctx, ownerID, trajectoryID)
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	s.jsonPatchMu.Lock()
	defer s.jsonPatchMu.Unlock()

	// Fetch the existing OG object to preserve object ID and created_at.
	obj, err := s.getTrajectoryObjectOG(ctx, ownerID, trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, err
	}
	var rec types.TrajectoryRecord
	if err := ogDecode(obj, &rec); err != nil {
		return types.TrajectoryRecord{}, err
	}
	if rec.OwnerID != ownerID {
		return types.TrajectoryRecord{}, ErrNotFound
	}
	if rec.SubjectRefs == nil {
		rec.SubjectRefs = map[string]string{}
	}
	changed := false
	for key, value := range patch {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if rec.SubjectRefs[key] == value {
			continue
		}
		rec.SubjectRefs[key] = value
		changed = true
	}
	if !changed {
		return rec, nil
	}
	rec.UpdatedAt = time.Now().UTC()
	return s.upsertTrajectoryOG(ctx, rec, obj)
}

// CreateWorkItem records a durable assignment on a trajectory. When a
// fingerprint is provided and an open or completed work item with the same
// (owner, trajectory, fingerprint) exists, the existing record is returned
// instead of inserting a duplicate (the ported continuation dedup).
func (s *Store) CreateWorkItem(ctx context.Context, rec types.WorkItemRecord) (types.WorkItemRecord, error) {
	if rec.OwnerID == "" {
		return types.WorkItemRecord{}, fmt.Errorf("create work item: owner_id is required")
	}
	rec.TrajectoryID = strings.TrimSpace(rec.TrajectoryID)
	if rec.TrajectoryID == "" {
		return types.WorkItemRecord{}, fmt.Errorf("create work item: trajectory_id is required")
	}
	if strings.TrimSpace(rec.Objective) == "" {
		return types.WorkItemRecord{}, fmt.Errorf("create work item: objective is required")
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()

	trajectory, err := s.GetTrajectoryOG(ctx, rec.OwnerID, rec.TrajectoryID)
	if err == nil {
		if trajectory.Status == types.TrajectorySettled || trajectory.Status == types.TrajectoryCancelled {
			return types.WorkItemRecord{}, ErrConcurrentStateChange
		}
	} else if !errors.Is(err, ErrNotFound) {
		return types.WorkItemRecord{}, err
	}

	if fingerprint := strings.TrimSpace(rec.ObjectiveFingerprint); fingerprint != "" {
		existing, ok, err := s.findWorkItemByFingerprint(ctx, rec.OwnerID, rec.TrajectoryID, fingerprint)
		if err != nil {
			return types.WorkItemRecord{}, err
		}
		if ok {
			return existing, nil
		}
	}
	if rec.WorkItemID == "" {
		rec.WorkItemID = uuid.NewString()
	}
	if rec.Status == "" {
		rec.Status = types.WorkItemOpen
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

	return s.CreateWorkItemOG(ctx, rec)
}

func (s *Store) findWorkItemByFingerprint(ctx context.Context, ownerID, trajectoryID, fingerprint string) (types.WorkItemRecord, bool, error) {
	objs, err := s.ogListByMetadata(ctx, ogKindWorkItem, "objective_fingerprint", fingerprint, 100)
	if err != nil {
		return types.WorkItemRecord{}, false, err
	}
	var earliest *types.WorkItemRecord
	for i := range objs {
		var rec types.WorkItemRecord
		if err := ogDecode(objs[i], &rec); err != nil {
			return types.WorkItemRecord{}, false, err
		}
		if rec.OwnerID != ownerID || rec.TrajectoryID != trajectoryID {
			continue
		}
		if rec.Status != types.WorkItemOpen && rec.Status != types.WorkItemCompleted {
			continue
		}
		if earliest == nil || rec.CreatedAt.Before(earliest.CreatedAt) {
			recCopy := rec
			earliest = &recCopy
		}
	}
	if earliest == nil {
		return types.WorkItemRecord{}, false, nil
	}
	return *earliest, true, nil
}

// FindWorkItemByFingerprint returns the first open or completed work item
// matching the owner/trajectory fingerprint tuple.
func (s *Store) FindWorkItemByFingerprint(ctx context.Context, ownerID, trajectoryID, fingerprint string) (types.WorkItemRecord, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	fingerprint = strings.TrimSpace(fingerprint)
	if ownerID == "" || trajectoryID == "" || fingerprint == "" {
		return types.WorkItemRecord{}, false, nil
	}
	return s.findWorkItemByFingerprint(ctx, ownerID, trajectoryID, fingerprint)
}

// GetWorkItem returns the work item with the given ID, owner-scoped.
func (s *Store) GetWorkItem(ctx context.Context, ownerID, workItemID string) (types.WorkItemRecord, error) {
	return s.GetWorkItemOG(ctx, ownerID, workItemID)
}

// ListWorkItemsByTrajectory returns the trajectory's work items, optionally
// filtered to open ones (the open-obligations query: "what is this
// trajectory waiting on?").
func (s *Store) ListWorkItemsByTrajectory(ctx context.Context, ownerID, trajectoryID string, openOnly bool) ([]types.WorkItemRecord, error) {
	return s.ListWorkItemsByTrajectoryOG(ctx, ownerID, trajectoryID, openOnly)
}

// ListOpenWorkItemsByKind returns open work items whose details.kind matches
// kind, oldest first. A non-positive limit returns every matching marker.
// Missing trajectory rows do not hide orphaned durable recovery markers.
func (s *Store) ListOpenWorkItemsByKind(ctx context.Context, kind string, limit int) ([]types.WorkItemRecord, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return nil, nil
	}
	return s.ListOpenWorkItemsByKindOG(ctx, kind, limit)
}

// ListOpenAssignedWorkItems returns open work items on live trajectories that
// already name the durable agent responsible for processing them. This is the
// boot-recovery query for cold actors whose update_coagent backlog is empty but
// whose trajectory still has assigned work.
func (s *Store) ListOpenAssignedWorkItems(ctx context.Context, limit int) ([]types.WorkItemRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	// Fetch a large window of work items from OG, filter for assigned +
	// live trajectory. Use a large limit to avoid missing eligible items
	// that are older than many ineligible ones.
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:  ogKindWorkItem,
		Limit: 100000,
	})
	if err != nil {
		return nil, fmt.Errorf("list open assigned work items: %w", err)
	}
	var candidates []types.WorkItemRecord
	for _, obj := range objs {
		var rec types.WorkItemRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.Status != types.WorkItemOpen {
			continue
		}
		if strings.TrimSpace(rec.AssignedAgentID) == "" {
			continue
		}
		// Check trajectory is live.
		traj, err := s.GetTrajectoryOG(ctx, rec.OwnerID, rec.TrajectoryID)
		if err != nil {
			continue
		}
		if traj.Status != types.TrajectoryLive {
			continue
		}
		candidates = append(candidates, rec)
	}
	// Sort by updated_at ASC, created_at ASC, work_item_id ASC.
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].UpdatedAt.Equal(candidates[j].UpdatedAt) {
			return candidates[i].UpdatedAt.Before(candidates[j].UpdatedAt)
		}
		if !candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		return candidates[i].WorkItemID < candidates[j].WorkItemID
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

// UpdateWorkItemStatus transitions an open work item to a terminal status.
// Repeating the stored status is idempotent; a terminal work item cannot be
// rewritten to the other terminal status.
func (s *Store) UpdateWorkItemStatus(ctx context.Context, ownerID, workItemID string, status types.WorkItemStatus) (types.WorkItemRecord, error) {
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	return s.UpdateWorkItemStatusOG(ctx, ownerID, workItemID, status)
}

// UpdateWorkItemDetails merges the provided details into the work item and
// stamps updated_at. Empty string keys or nil values are ignored.
// Merge patches are serialized within one Store instance so concurrent callers
// cannot drop each other's keys by overwriting the whole JSON object.
func (s *Store) UpdateWorkItemDetails(ctx context.Context, ownerID, workItemID string, patch map[string]any) (types.WorkItemRecord, error) {
	if len(patch) == 0 {
		return s.GetWorkItem(ctx, ownerID, workItemID)
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	s.jsonPatchMu.Lock()
	defer s.jsonPatchMu.Unlock()

	rec, err := s.GetWorkItemOG(ctx, ownerID, workItemID)
	if err != nil {
		return types.WorkItemRecord{}, err
	}
	if rec.Details == nil {
		rec.Details = map[string]any{}
	}
	changed := false
	for key, value := range patch {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		if existing, ok := rec.Details[key]; ok {
			existingJSON, existingErr := marshalJSON(existing)
			valueJSON, valueErr := marshalJSON(value)
			if existingErr == nil && valueErr == nil && string(existingJSON) == string(valueJSON) {
				continue
			}
		}
		rec.Details[key] = value
		changed = true
	}
	if !changed {
		return rec, nil
	}
	rec.UpdatedAt = time.Now().UTC()
	// Upsert back to OG.
	if _, err := s.CreateWorkItemOG(ctx, rec); err != nil {
		return types.WorkItemRecord{}, fmt.Errorf("update work item details: %w", err)
	}
	return s.GetWorkItemOG(ctx, ownerID, workItemID)
}

const workItemColumns = `work_item_id, trajectory_id, owner_id, objective, reason,
	authority_profile, step_budget, token_budget, objective_fingerprint,
	status, assigned_agent_id, created_by_loop_id, details_json,
	created_at, updated_at`

func scanTrajectory(row interface{ Scan(...any) error }) (types.TrajectoryRecord, error) {
	var rec types.TrajectoryRecord
	var kind, status, subjectRefsJSON, ruleJSON, createdAt, updatedAt string
	var settledAt sql.NullString

	err := row.Scan(
		&rec.TrajectoryID,
		&rec.OwnerID,
		&kind,
		&subjectRefsJSON,
		&status,
		&ruleJSON,
		&createdAt,
		&updatedAt,
		&settledAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.TrajectoryRecord{}, ErrNotFound
		}
		return types.TrajectoryRecord{}, fmt.Errorf("scan trajectory: %w", err)
	}
	rec.Kind = types.TrajectoryKind(kind)
	rec.Status = types.TrajectoryStatus(status)
	if err := json.Unmarshal([]byte(subjectRefsJSON), &rec.SubjectRefs); err != nil {
		return types.TrajectoryRecord{}, fmt.Errorf("decode trajectory subject refs: %w", err)
	}
	if err := json.Unmarshal([]byte(ruleJSON), &rec.SettlementRule); err != nil {
		return types.TrajectoryRecord{}, fmt.Errorf("decode trajectory settlement rule: %w", err)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.TrajectoryRecord{}, fmt.Errorf("parse trajectory created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.TrajectoryRecord{}, fmt.Errorf("parse trajectory updated_at: %w", err)
	}
	if settledAt.Valid && settledAt.String != "" {
		t, err := time.Parse(time.RFC3339Nano, settledAt.String)
		if err != nil {
			return types.TrajectoryRecord{}, fmt.Errorf("parse trajectory settled_at: %w", err)
		}
		rec.SettledAt = &t
	}
	return rec, nil
}

func scanWorkItem(row interface{ Scan(...any) error }) (types.WorkItemRecord, error) {
	var rec types.WorkItemRecord
	var status, detailsJSON, createdAt, updatedAt string

	err := row.Scan(
		&rec.WorkItemID,
		&rec.TrajectoryID,
		&rec.OwnerID,
		&rec.Objective,
		&rec.Reason,
		&rec.AuthorityProfile,
		&rec.StepBudget,
		&rec.TokenBudget,
		&rec.ObjectiveFingerprint,
		&status,
		&rec.AssignedAgentID,
		&rec.CreatedByRunID,
		&detailsJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.WorkItemRecord{}, ErrNotFound
		}
		return types.WorkItemRecord{}, fmt.Errorf("scan work item: %w", err)
	}
	rec.Status = types.WorkItemStatus(status)
	if err := json.Unmarshal([]byte(detailsJSON), &rec.Details); err != nil {
		return types.WorkItemRecord{}, fmt.Errorf("decode work item details: %w", err)
	}
	rec.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return types.WorkItemRecord{}, fmt.Errorf("parse work item created_at: %w", err)
	}
	rec.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return types.WorkItemRecord{}, fmt.Errorf("parse work item updated_at: %w", err)
	}
	return rec, nil
}
