package proxy

import (
	"context"
	"fmt"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// StoreLineageReader adapts a *store.Store to the proxy's LineageReader
// interface. It queries the ComputerSourceLineageRecord and returns a
// minimal LineageRecord with only the fields the proxy needs for route
// resolution.
//
// This adapter is in the proxy package (not the store package) to keep
// the dependency direction clean: the proxy defines the interface, and
// this adapter bridges the store implementation to it.
type StoreLineageReader struct {
	Store *store.Store
}

// GetLineage queries the ComputerSourceLineageRecord for the given owner
// and computer, returning a minimal LineageRecord.
func (s *StoreLineageReader) GetLineage(ctx context.Context, ownerID, computerID string) (LineageRecord, error) {
	if s == nil || s.Store == nil {
		return LineageRecord{}, fmt.Errorf("store lineage reader: store not configured")
	}

	rec, err := s.Store.GetComputerSourceLineage(ctx, ownerID, computerID)
	if err != nil {
		return LineageRecord{}, fmt.Errorf("store lineage reader: %w", err)
	}

	return LineageRecord{
		OwnerID:         rec.OwnerID,
		ComputerID:      rec.ComputerID,
		ActiveSourceRef: rec.ActiveSourceRef,
		RouteProfile:    rec.RouteProfile,
	}, nil
}
