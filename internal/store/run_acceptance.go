package store

import (
	"context"
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

	if err := s.CreateRunAcceptanceOG(ctx, rec); err != nil {
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
	return s.GetRunAcceptanceOG(ctx, ownerID, acceptanceID)
}

func (s *Store) GetRunAcceptanceByID(ctx context.Context, acceptanceID string) (types.RunAcceptanceRecord, error) {
	if acceptanceID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("get run acceptance by id: acceptance_id is required")
	}
	return s.GetRunAcceptanceByIDOG(ctx, acceptanceID)
}

func (s *Store) ListRunAcceptances(ctx context.Context, ownerID string, limit int) ([]types.RunAcceptanceRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("list run acceptances: user_id is required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	// Fetch a large window since ListObjects orders by og_objects.updated_at
	// which may not match RunAcceptanceRecord.UpdatedAt. We need all
	// acceptances to sort by record updated_at and then apply the limit.
	objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    ogKindRunAccept,
		OwnerID: ownerID,
		Limit:   100000,
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
	return acceptances, nil
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
	return s.ListRunAcceptancesByTrajectoryOG(ctx, ownerID, trajectoryID, limit)
}
