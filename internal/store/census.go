package store

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ListOwnerObjectsOfKind lists up to limit of an owner's objects of one kind,
// newest first, for the runtime census.
func (s *Store) ListOwnerObjectsOfKind(ctx context.Context, kind, ownerID string, limit int) ([]objectgraph.Object, error) {
	return s.ogListByOwnerAndBody(ctx, objectgraph.ObjectKind(kind), ownerID, nil, limit)
}
