package objectgraph

import (
	"fmt"
	"sync"
)

type StoreType string

const (
	StoreTypeMemory StoreType = "memory"
	StoreTypeSQLite StoreType = "sqlite"
	StoreTypeDolt   StoreType = "dolt"
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
		// --- Existing kinds (source graph / media) ---
		{Kind: "choir.source_entity", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.source_ref", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.web_capture", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.universal_wire_story_cluster", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Versioned: true},
		{Kind: "choir.media_item", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.audio_recording", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.transcript", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.autoradio_run_sheet", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Versioned: true},

		// --- VM store: audited computer ---
		{Kind: "choir.agent", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.run", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.event", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.channel_message", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.inbox_delivery", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.run_memory_entry", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.trajectory", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.work_item", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.run_acceptance", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.run_continuation", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.texture_document", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.texture_revision", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Versioned: true},
		{Kind: "choir.texture_decision", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.agent_evidence", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.content_item", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.podcast_subscription", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.browser_session", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.app_change_package", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.app_adoption", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.desktop_session", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.desktop_app_instance", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},

		// --- corpusd: publication layer ---
		{Kind: "choir.subject", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.publication", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.publication_version", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Versioned: true},
		{Kind: "choir.publication_proposal", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.public_route", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.artifact_manifest", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.artifact_blob", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.provenance_entity", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.provenance_activity", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.provenance_agent", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.consent_record", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.review_record", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.retrieval_source", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.retrieval_span", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.retrieval_manifest", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.publication_source_entity", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
		{Kind: "choir.publication_transclusion", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.publication_policy", Store: StoreTypeDolt, IdentityMode: IdentityContentHash},
		{Kind: "choir.verifier_attestation", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey},
	} {
		r.RegisterKind(k)
	}
	for _, e := range []EdgeRegistration{
		// --- Existing edges ---
		{Kind: "cites"},
		{Kind: "captured_from"},
		{Kind: "derived_from"},
		{Kind: "has_media"},
		{Kind: "has_transcript"},
		{Kind: "contains"},
		{Kind: "references"},

		// --- Structural edges: publication layer ---
		{Kind: "has_version"},
		{Kind: "supersedes"},
		{Kind: "derived_from_proposal"},
		{Kind: "routes_to"},
		{Kind: "has_manifest"},
		{Kind: "contains_blob"},
		{Kind: "owns"},
		{Kind: "has_agent"},

		// --- Structural edges: VM audited computer ---
		{Kind: "document_revision"},
		{Kind: "revision_parent"},
		{Kind: "run_agent"},
		{Kind: "run_trajectory"},
		{Kind: "run_parent"},
		{Kind: "event_run"},
		{Kind: "message_from_run"},
		{Kind: "message_to_run"},
		{Kind: "work_item_trajectory"},
		{Kind: "work_item_assigned_agent"},
		{Kind: "acceptance_run"},
		{Kind: "acceptance_trajectory"},
		{Kind: "continuation_from_run"},
		{Kind: "continuation_to_run"},
		{Kind: "decision_document"},
		{Kind: "decision_run"},
		{Kind: "evidence_agent"},
		{Kind: "subscription_content"},
		{Kind: "browser_session_run"},
		{Kind: "package_source_computer"},
		{Kind: "adoption_package"},
		{Kind: "adoption_target_computer"},
		{Kind: "session_desktop"},
		{Kind: "app_instance_desktop"},

		// --- Provenance edges ---
		{Kind: "was_derived_from"},
		{Kind: "was_generated_by"},
		{Kind: "was_associated_with"},
		{Kind: "generated"},
		{Kind: "attested"},
		{Kind: "attests_to"},
		{Kind: "granted_consent"},
		{Kind: "consent_for"},
		{Kind: "authored_review"},
		{Kind: "reviews"},
		{Kind: "contains_span"},
		{Kind: "has_retrieval_manifest"},
		{Kind: "references_entity"},
		{Kind: "transcludes"},
		{Kind: "transcludes_from"},
		{Kind: "has_policy"},

		// --- Edge-only tables (no object) ---
		{Kind: "document_alias"},
		{Kind: "document_mutation"},
		{Kind: "document_checkpoint"},
		{Kind: "coagent_mailbox"},
		{Kind: "super_slot"},
		{Kind: "computer_lineage"},
		{Kind: "media_progress"},
		{Kind: "media_recent"},
		{Kind: "user_preference"},
		{Kind: "desktop_state"},
		{Kind: "desktop_workspace"},
		{Kind: "window_placement"},
		{Kind: "worker_update"},
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
