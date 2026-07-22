package objectgraph

import (
	"context"
	"errors"
)

var (
	ErrNotFound = errors.New("objectgraph: not found")
	ErrConflict = errors.New("objectgraph: conditional write conflict")
)

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

// ObjectCondition is one compare predicate evaluated in the same transaction
// as a conditional batch. Exists=false requires the object to be absent.
// Exists=true requires it to be present and, when supplied, to match both
// expected immutable identifiers.
type ObjectCondition struct {
	CanonicalID         string
	Exists              bool
	ExpectedVersionID   string
	ExpectedContentHash string
}

// BatchStore is an optional interface for stores that support atomic
// batch writes of multiple objects and edges in a single transaction.
type BatchStore interface {
	Store
	PutBatch(ctx context.Context, batch Batch) error
}

// ConditionalBatchStore extends BatchStore with compare-and-write semantics.
// Every condition and mutation is evaluated in one database transaction.
type ConditionalBatchStore interface {
	BatchStore
	PutBatchConditional(ctx context.Context, conditions []ObjectCondition, batch Batch) error
}

// SnapshotStore reads all objects in one owner/computer scope from one
// serializable read transaction. Reducers use this for coherent snapshots and
// their event watermarks.
type SnapshotStore interface {
	Store
	ReadObjectSnapshot(ctx context.Context, ownerID, computerID string) ([]Object, error)
}
