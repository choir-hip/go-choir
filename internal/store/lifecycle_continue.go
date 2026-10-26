package store

import (
	"context"
	"errors"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// ComputeContinueLifecycleDocumentDigest is the canonical digest of a
// continue command: scope and timestamps are not part of it, so a retry
// from any caller with the same intent replays.
func ComputeContinueLifecycleDocumentDigest(req types.ContinueLifecycleDocumentRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.InitialWork.OwnerID, req.InitialWork.ComputerID, req.InitialWork.CreatedByRunID = "", "", ""
	req.InitialWork.CreatedAt, req.InitialWork.UpdatedAt = time.Time{}, time.Time{}
	return lifecycleDigest(req)
}

// ContinueLifecycleDocument is not implemented yet (tests first).
func (s *Store) ContinueLifecycleDocument(ctx context.Context, req types.ContinueLifecycleDocumentRequest) (types.LifecycleResult, error) {
	return types.LifecycleResult{}, errors.New("lifecycle continue: not implemented")
}

// LifecycleTrajectoryLineage is not implemented yet (tests first).
func (s *Store) LifecycleTrajectoryLineage(ctx context.Context, ownerID, computerID, trajectoryID string, max int) ([]string, error) {
	return []string{trajectoryID}, nil
}
