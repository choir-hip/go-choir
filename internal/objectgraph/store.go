package objectgraph

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("objectgraph: not found")

type Store interface {
	PutObject(ctx context.Context, obj Object) error
	GetObject(ctx context.Context, id string) (Object, error)
	ListObjects(ctx context.Context, filter ListFilter) ([]Object, error)
	PutEdge(ctx context.Context, edge Edge) error
	ListEdges(ctx context.Context, filter EdgeFilter) ([]Edge, error)
	DeleteObject(ctx context.Context, id string) error
	Close() error
}

// Batch is a list of object and edge mutations to apply atomically.
type Batch struct {
	Objects []Object
	Edges   []Edge
}

// BatchStore is an optional interface for stores that support atomic
// batch writes of multiple objects and edges in a single transaction.
type BatchStore interface {
	Store
	PutBatch(ctx context.Context, batch Batch) error
}
