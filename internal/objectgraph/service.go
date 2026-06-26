package objectgraph

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	registry *Registry
	memory   Store
	sqlite   Store
}

type Config struct {
	Registry *Registry
	Memory   Store
	SQLite   Store
}

type CreateObjectRequest struct {
	Kind        ObjectKind
	OwnerID     string
	ComputerID  string
	VersionID   string
	IdentityKey string
	Body        []byte
	Metadata    any
	Now         time.Time
}

func NewService(cfg Config) *Service {
	s := &Service{
		registry: cfg.Registry,
		memory:   cfg.Memory,
		sqlite:   cfg.SQLite,
	}
	if s.registry == nil {
		s.registry = DefaultRegistry()
	}
	if s.memory == nil {
		s.memory = NewMemoryStore()
	}
	if s.sqlite == nil {
		s.sqlite = s.memory
	}
	return s
}

func (s *Service) CreateObject(ctx context.Context, req CreateObjectRequest) (Object, error) {
	reg, err := s.registry.LookupKind(req.Kind)
	if err != nil {
		return Object{}, err
	}
	if req.OwnerID == "" {
		return Object{}, fmt.Errorf("owner_id is required")
	}
	meta, err := NormalizeMetadata(req.Metadata)
	if err != nil {
		return Object{}, err
	}
	contentHash := ContentHash(req.Kind, req.Body, meta)
	suffix := StableSuffixFromContent(contentHash)
	if reg.IdentityMode == IdentityExternalKey || req.IdentityKey != "" {
		if req.IdentityKey == "" {
			return Object{}, fmt.Errorf("identity_key is required for %s", req.Kind)
		}
		suffix = StableSuffixFromKey(req.IdentityKey)
	}
	id, err := BuildCanonicalID(req.Kind, req.OwnerID, suffix)
	if err != nil {
		return Object{}, err
	}
	now := req.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	obj := Object{
		CanonicalID: id,
		ObjectKind:  req.Kind,
		OwnerID:     req.OwnerID,
		ComputerID:  req.ComputerID,
		VersionID:   req.VersionID,
		ContentHash: contentHash,
		Body:        append([]byte(nil), req.Body...),
		Metadata:    meta,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.storeFor(reg.Store).PutObject(ctx, obj); err != nil {
		return Object{}, err
	}
	return obj, nil
}

func (s *Service) GetObject(ctx context.Context, id string) (Object, error) {
	kind, _, _, err := ParseCanonicalID(id)
	if err != nil {
		return Object{}, err
	}
	reg, err := s.registry.LookupKind(kind)
	if err != nil {
		return Object{}, err
	}
	return s.storeFor(reg.Store).GetObject(ctx, id)
}

func (s *Service) ListObjects(ctx context.Context, filter ListFilter) ([]Object, error) {
	if filter.Kind != "" {
		reg, err := s.registry.LookupKind(filter.Kind)
		if err != nil {
			return nil, err
		}
		return s.storeFor(reg.Store).ListObjects(ctx, filter)
	}
	seen := map[string]bool{}
	var out []Object
	for _, store := range []Store{s.memory, s.sqlite} {
		objs, err := store.ListObjects(ctx, filter)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			if seen[obj.CanonicalID] {
				continue
			}
			seen[obj.CanonicalID] = true
			out = append(out, obj)
		}
	}
	limit := normalizedLimit(filter.Limit)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Service) PutEdge(ctx context.Context, fromID, toID string, kind EdgeKind, metadata any) (Edge, error) {
	if _, err := s.registry.LookupEdge(kind); err != nil {
		return Edge{}, err
	}
	from, err := s.GetObject(ctx, fromID)
	if err != nil {
		return Edge{}, fmt.Errorf("edge source not found: %w", err)
	}
	if _, err := s.GetObject(ctx, toID); err != nil {
		return Edge{}, fmt.Errorf("edge target not found: %w", err)
	}
	meta, err := NormalizeMetadata(metadata)
	if err != nil {
		return Edge{}, err
	}
	id, err := BuildEdgeID(fromID, toID, kind, meta)
	if err != nil {
		return Edge{}, err
	}
	edge := Edge{
		EdgeID:    id,
		FromID:    fromID,
		ToID:      toID,
		Kind:      kind,
		Metadata:  meta,
		CreatedAt: time.Now().UTC(),
	}
	reg, err := s.registry.LookupKind(from.ObjectKind)
	if err != nil {
		return Edge{}, err
	}
	if err := s.storeFor(reg.Store).PutEdge(ctx, edge); err != nil {
		return Edge{}, err
	}
	return edge, nil
}

func (s *Service) ListEdges(ctx context.Context, filter EdgeFilter) ([]Edge, error) {
	var out []Edge
	seen := map[string]bool{}
	for _, store := range []Store{s.memory, s.sqlite} {
		edges, err := store.ListEdges(ctx, filter)
		if err != nil {
			return nil, err
		}
		for _, edge := range edges {
			if seen[edge.EdgeID] {
				continue
			}
			seen[edge.EdgeID] = true
			out = append(out, edge)
		}
	}
	limit := normalizedLimit(filter.Limit)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Service) Close() error {
	var err error
	if s.memory != nil {
		err = s.memory.Close()
	}
	if s.sqlite != nil && s.sqlite != s.memory {
		if closeErr := s.sqlite.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

func (s *Service) storeFor(storeType StoreType) Store {
	if storeType == StoreTypeMemory {
		return s.memory
	}
	return s.sqlite
}
