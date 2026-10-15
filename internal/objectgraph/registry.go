package objectgraph

import (
	"errors"
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

// Owners of registered kinds: the package that writes them, on which side.
const (
	OwnerVMStore   = "internal/store (computer)"
	OwnerCorpusd   = "internal/platform (corpusd)"
	OwnerAutoradio = "autoradio (reserved, no writer yet)"
)

// KindRegistration is one object kind the graph accepts. Owner names the
// package that writes it; Schema, when recorded, names the Go type of its
// body. Reserved marks a kind kept for planned work that no code writes
// yet (TestEveryRegisteredKindHasAWriter holds every other kind to a
// writer).
type KindRegistration struct {
	Kind         ObjectKind   `json:"kind"`
	Store        StoreType    `json:"store"`
	IdentityMode IdentityMode `json:"identity_mode"`
	Versioned    bool         `json:"versioned"`
	Owner        string       `json:"owner,omitempty"`
	Schema       string       `json:"schema,omitempty"`
	Reserved     bool         `json:"reserved,omitempty"`
}

type EdgeRegistration struct {
	Kind  EdgeKind `json:"kind"`
	Owner string   `json:"owner,omitempty"`
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
		// --- Source graph, captures and media ---
		{Kind: "choir.source_entity", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.source_ref", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.web_capture", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd, Schema: "objectgraph.WebCaptureMetadata"},
		// Reserved for Autoradio and Autopaper media
		// (docs/research/autopaper-media-and-rights-2026-10-14/repo-media-survey.md).
		{Kind: "choir.media_item", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerAutoradio, Reserved: true},
		{Kind: "choir.audio_recording", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerAutoradio, Reserved: true},
		{Kind: "choir.transcript", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerAutoradio, Reserved: true},
		{Kind: "choir.autoradio_run_sheet", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Versioned: true, Owner: OwnerAutoradio, Reserved: true},

		// --- VM store: audited computer ---
		{Kind: "choir.agent", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.run", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.event", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.channel_message", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.inbox_delivery", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.run_memory_entry", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.trajectory", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.work_item", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.run_acceptance", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.run_continuation", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.texture_document", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.texture_revision", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Versioned: true, Owner: OwnerVMStore},
		{Kind: "choir.texture_decision", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.agent_evidence", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.content_item", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.podcast_subscription", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.browser_session", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.desktop_session", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.desktop_app_instance", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.worker_update", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.coagent_mailbox", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.source_schedule", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.computer_setting", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.capture_label", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.capture_label_override", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},

		// --- VM store: written through PutBatch before the write gate
		// existed (H-OG1, docs/problems/object-graph-registration-and-anchor-gaps-2026-10-14.md).
		// Identity is lifecycleCanonicalID: a key scoped by computer.
		{Kind: "choir.actor_wake_outbox", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.ActorWakeOutbox"},
		{Kind: "choir.capture_disposition", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.CaptureDisposition"},
		{Kind: "choir.citation_check", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.CitationCheck"},
		{Kind: "choir.commitment_record", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.CommitmentRecord"},
		{Kind: "choir.conversation_head", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.ConversationHead"},
		{Kind: "choir.inbox_command", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.InboxCommitReceipt"},
		{Kind: "choir.inbox_cursor", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.InboxCursor"},
		{Kind: "choir.inbox_head", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.InboxHead"},
		{Kind: "choir.lifecycle_cancel_intent", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.LifecycleCancellationIntent"},
		{Kind: "choir.lifecycle_command", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.LifecycleCommandReceipt"},
		{Kind: "choir.lifecycle_event", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.LifecycleEvent"},
		{Kind: "choir.lifecycle_sequence", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.lifecycleSequence"},
		{Kind: "choir.management_task", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.ManagementTask"},
		{Kind: "choir.migration_marker", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.actorWakeOutboxMigrationMarker"},
		{Kind: "choir.scheduler_sequence", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.arrivalSequence"},
		{Kind: "choir.story_library_head", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.StoryLibraryHead"},
		{Kind: "choir.support_verdict", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.SupportVerdict"},
		// Engineering records keep their frozen V1 kind spelling
		// (vocabmigrate frozenOGObjectKinds).
		{Kind: "choir.co_super_assignment", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.EngineeringAssignment"},
		{Kind: "choir.co_super_assignment_report", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.EngineeringAssignmentReport"},
		{Kind: "choir.co_super_subject_candidate", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.EngineeringSubjectCandidate"},
		{Kind: "choir.co_super_capability_claim", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.engineeringCapabilityClaim"},
		{Kind: "choir.co_super_capsule_claim", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.engineeringCapsuleClaim"},
		{Kind: "choir.co_super_run_claim", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.engineeringRunClaim"},

		// --- corpusd: publication layer ---
		{Kind: "choir.subject", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.publication", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.publication_version", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Versioned: true, Owner: OwnerCorpusd},
		{Kind: "choir.publication_proposal", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.public_route", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.artifact_manifest", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd},
		{Kind: "choir.artifact_blob", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd},
		{Kind: "choir.provenance_entity", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd},
		{Kind: "choir.provenance_activity", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.provenance_agent", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.consent_record", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.review_record", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.retrieval_source", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd},
		{Kind: "choir.retrieval_span", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd},
		{Kind: "choir.retrieval_manifest", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd},
		{Kind: "choir.publication_source_entity", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		{Kind: "choir.publication_transclusion", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd},
		{Kind: "choir.publication_policy", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerCorpusd},
		{Kind: "choir.verifier_attestation", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd},
		// Written before the write gate existed (H-OG1).
		{Kind: "choir.publication_resolved_blocks", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "platform.resolvedBlocksDoc"},
		{Kind: "choir.publication_check", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "platform.publicationCheck"},
		{Kind: "choir.support_judgement", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "platform.supportJudgement"},
		// A published software release record (P7,
		// docs/reports/p6-p8-release-adopt-2026-10-15.md): source only, never
		// executable bytes; the patch lives in the host content store.
		{Kind: "choir.software_release", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "release.Record"},
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
		{Kind: "has_retrieval_source"},
		{Kind: "references_entity"},
		{Kind: "transcludes"},
		{Kind: "transcludes_from"},
		{Kind: "has_policy"},

		// --- Written before the write gate existed (H-OG1) ---
		// corpusd: version -> its pinned blocks, and the citation relation
		// a version has to the source revision it was published from
		// (platform/service.go; a citation edge takes its relation type as
		// its edge kind).
		{Kind: "has_resolved_blocks", Owner: OwnerCorpusd},
		{Kind: "is_version_of", Owner: OwnerCorpusd},
		// computer: engineering records (frozen V1 spelling) and the
		// commitment record's delivery packet.
		{Kind: "co_super_assignment_trajectory", Owner: OwnerVMStore},
		{Kind: "co_super_assignment_parent", Owner: OwnerVMStore},
		{Kind: "co_super_assignment_parent_run", Owner: OwnerVMStore},
		{Kind: "co_super_assignment_parent_work", Owner: OwnerVMStore},
		{Kind: "co_super_assignment_agent", Owner: OwnerVMStore},
		{Kind: "co_super_assignment_work", Owner: OwnerVMStore},
		{Kind: "co_super_assignment_run", Owner: OwnerVMStore},
		{Kind: "co_super_report_assignment", Owner: OwnerVMStore},
		{Kind: "co_super_report_candidate", Owner: OwnerVMStore},
		{Kind: "commitment_record_packet", Owner: OwnerVMStore},

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

// ErrUnregisteredKind refuses a write of an object or edge kind the
// registry does not know.
var ErrUnregisteredKind = errors.New("objectgraph: unregistered kind")

// writeRegistry is the one list every durable write is checked against.
var writeRegistry = DefaultRegistry()

// CheckWrite is the object graph's write gate (H-OG1): it refuses a batch
// that carries a live object or edge of a kind DefaultRegistry does not
// register. Both durable stores call it first in every mutation method
// (DoltStore before its validator and its projection interceptor, so a
// refused write never reaches the event tape; platform.ObjectGraphStore
// before SQL), so every write path passes it: Service, direct PutBatch,
// and the corpusd HTTP routes.
//
// Tombstones pass: retiring a historic object of a kind no longer
// registered must stay possible. Replay (the event projector), vocabulary
// migrations and deletes do not call it: they move existing rows, they do
// not mint new kinds.
func CheckWrite(objects []Object, edges []Edge) error {
	for _, obj := range objects {
		if obj.Tombstone {
			continue
		}
		if _, err := writeRegistry.LookupKind(obj.ObjectKind); err != nil {
			return fmt.Errorf("%w: object kind %q (object %s); register it in objectgraph.DefaultRegistry with its owner", ErrUnregisteredKind, obj.ObjectKind, obj.CanonicalID)
		}
	}
	for _, edge := range edges {
		if edge.Tombstone {
			continue
		}
		if _, err := writeRegistry.LookupEdge(edge.Kind); err != nil {
			return fmt.Errorf("%w: edge kind %q (edge %s); register it in objectgraph.DefaultRegistry with its owner", ErrUnregisteredKind, edge.Kind, edge.EdgeID)
		}
	}
	return nil
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
