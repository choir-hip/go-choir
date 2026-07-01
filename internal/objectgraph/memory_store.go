package objectgraph

import (
	"context"
	"sort"
	"sync"
)

type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string]Object
	edges   map[string]Edge
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		objects: make(map[string]Object),
		edges:   make(map[string]Edge),
	}
}

func (m *MemoryStore) PutObject(_ context.Context, obj Object) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[obj.CanonicalID] = obj
	return nil
}

func (m *MemoryStore) GetObject(_ context.Context, id string) (Object, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	obj, ok := m.objects[id]
	if !ok {
		return Object{}, ErrNotFound
	}
	return obj, nil
}

func (m *MemoryStore) ListObjects(_ context.Context, filter ListFilter) ([]Object, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limit := NormalizedLimit(filter.Limit)
	out := make([]Object, 0, len(m.objects))
	for _, obj := range m.objects {
		if !objectMatches(obj, filter) {
			continue
		}
		out = append(out, obj)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryStore) PutEdge(_ context.Context, edge Edge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.edges[edge.EdgeID] = edge
	return nil
}

func (m *MemoryStore) ListEdges(_ context.Context, filter EdgeFilter) ([]Edge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	limit := NormalizedLimit(filter.Limit)
	out := make([]Edge, 0, len(m.edges))
	for _, edge := range m.edges {
		if !edgeMatches(edge, filter) {
			continue
		}
		out = append(out, edge)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *MemoryStore) Close() error { return nil }

func NormalizedLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	return limit
}

func objectMatches(obj Object, filter ListFilter) bool {
	if filter.Kind != "" && obj.ObjectKind != filter.Kind {
		return false
	}
	if filter.OwnerID != "" && obj.OwnerID != filter.OwnerID {
		return false
	}
	return filter.Tombstone == nil || obj.Tombstone == *filter.Tombstone
}

func edgeMatches(edge Edge, filter EdgeFilter) bool {
	if filter.FromID != "" && edge.FromID != filter.FromID {
		return false
	}
	if filter.ToID != "" && edge.ToID != filter.ToID {
		return false
	}
	if filter.Kind != "" && edge.Kind != filter.Kind {
		return false
	}
	return filter.Tombstone == nil || edge.Tombstone == *filter.Tombstone
}
