package store

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Scoped store handles (modular architecture plan MA3.3).
//
// A read of the object graph declares its reader, and a handle is how code
// that holds one authority declares it once: store.As(reader) names the
// reader, Ctx puts it on a context for the typed Store methods, and Graph
// returns the object graph's read surface with the reader already declared.
// The reader is chosen by trusted in-process code (the run record's desk, a
// kind's custodian from the registry, the owner), never by a cell.
//
// The typed Store methods keep their shapes: a handle does not wrap each of
// them, because the reader gate sits under them, in the SQL store, and sees
// whatever context they were called with.

// ScopedStore is the store as one reader.
type ScopedStore struct {
	s      *Store
	reader objectgraph.Writer
}

// As returns a handle whose reads declare reader.
func (s *Store) As(reader objectgraph.Writer) *ScopedStore {
	return &ScopedStore{s: s, reader: reader}
}

// AsCustodianOf returns a handle for the custodian of kind, and false when no
// manifest claims the kind. A host loop reading a custodian's kind on its
// behalf uses it, so the loop names no desk.
func (s *Store) AsCustodianOf(kind objectgraph.ObjectKind) (*ScopedStore, bool) {
	w, ok := objectgraph.CustodianOf(kind)
	if !ok {
		return nil, false
	}
	return s.As(w), true
}

// Reader is the reader the handle declares.
func (h *ScopedStore) Reader() objectgraph.Writer { return h.reader }

// Ctx returns ctx with the handle's reader declared, for the typed Store
// methods.
func (h *ScopedStore) Ctx(ctx context.Context) context.Context {
	return objectgraph.WithReader(ctx, h.reader)
}

// Graph is the object graph's read surface with the handle's reader declared.
// Writes pass through unchanged: the writer gate has its own declaration.
func (h *ScopedStore) Graph() objectgraph.Store {
	return scopedGraph{Store: h.s.lifecycleGraph(), reader: h.reader}
}

type scopedGraph struct {
	objectgraph.Store
	reader objectgraph.Writer
}

func (g scopedGraph) GetObject(ctx context.Context, id string) (objectgraph.Object, error) {
	return g.Store.GetObject(objectgraph.WithReader(ctx, g.reader), id)
}

func (g scopedGraph) ListObjects(ctx context.Context, filter objectgraph.ListFilter) ([]objectgraph.Object, error) {
	return g.Store.ListObjects(objectgraph.WithReader(ctx, g.reader), filter)
}
