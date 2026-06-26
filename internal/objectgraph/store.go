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
	Close() error
}
