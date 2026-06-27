package objectgraph

import (
	"fmt"
	"sync"
)

type StoreType string

const (
	StoreTypeMemory StoreType = "memory"
	StoreTypeSQLite StoreType = "sqlite"
)

type IdentityMode string

const (
	IdentityContentHash IdentityMode = "content_hash"
	IdentityExternalKey IdentityMode = "external_key"
)

type KindRegistration struct {
	Kind         ObjectKind   `json:"kind"`
	Store        StoreType    `json:"store"`
	IdentityMode IdentityMode `json:"identity_mode"`
	Versioned    bool         `json:"versioned"`
}

type EdgeRegistration struct {
	Kind EdgeKind `json:"kind"`
}

type Registry struct {
	mu    sync.RWMutex
	kinds map[ObjectKind]KindRegistration
	edges map[EdgeKind]EdgeRegistration
}

func NewRegistry() *Registry {
	return &Registry{
		kinds: make(map[ObjectKind]KindRegistration),
		edges: make(map[EdgeKind]EdgeRegistration),
	}
}

func DefaultRegistry() *Registry {
	r := NewRegistry()
	for _, k := range []KindRegistration{
		{Kind: "choir.source_entity", Store: StoreTypeSQLite, IdentityMode: IdentityContentHash},
		{Kind: "choir.source_ref", Store: StoreTypeSQLite, IdentityMode: IdentityContentHash},
		{Kind: "choir.web_capture", Store: StoreTypeSQLite, IdentityMode: IdentityContentHash},
		{Kind: "choir.universal_wire_story_cluster", Store: StoreTypeSQLite, IdentityMode: IdentityExternalKey, Versioned: true},
		{Kind: "choir.universal_wire_live_arrival_status", Store: StoreTypeSQLite, IdentityMode: IdentityExternalKey, Versioned: true},
		{Kind: "choir.media_item", Store: StoreTypeSQLite, IdentityMode: IdentityContentHash},
		{Kind: "choir.audio_recording", Store: StoreTypeSQLite, IdentityMode: IdentityContentHash},
		{Kind: "choir.transcript", Store: StoreTypeSQLite, IdentityMode: IdentityContentHash},
		{Kind: "choir.autoradio_run_sheet", Store: StoreTypeSQLite, IdentityMode: IdentityContentHash, Versioned: true},
	} {
		r.RegisterKind(k)
	}
	for _, e := range []EdgeRegistration{
		{Kind: "cites"},
		{Kind: "captured_from"},
		{Kind: "derived_from"},
		{Kind: "has_media"},
		{Kind: "has_transcript"},
		{Kind: "contains"},
		{Kind: "references"},
	} {
		r.RegisterEdge(e)
	}
	return r
}

func (r *Registry) RegisterKind(k KindRegistration) {
	if k.IdentityMode == "" {
		k.IdentityMode = IdentityContentHash
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.kinds[k.Kind] = k
}

func (r *Registry) RegisterEdge(e EdgeRegistration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.edges[e.Kind] = e
}

func (r *Registry) LookupKind(kind ObjectKind) (KindRegistration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.kinds[kind]
	if !ok {
		return KindRegistration{}, fmt.Errorf("objectgraph: unregistered object kind %s", kind)
	}
	return reg, nil
}

func (r *Registry) LookupEdge(kind EdgeKind) (EdgeRegistration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.edges[kind]
	if !ok {
		return EdgeRegistration{}, fmt.Errorf("objectgraph: unregistered edge kind %s", kind)
	}
	return reg, nil
}

func (r *Registry) AllKinds() []KindRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]KindRegistration, 0, len(r.kinds))
	for _, reg := range r.kinds {
		out = append(out, reg)
	}
	return out
}

func (r *Registry) AllEdges() []EdgeRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EdgeRegistration, 0, len(r.edges))
	for _, reg := range r.edges {
		out = append(out, reg)
	}
	return out
}
