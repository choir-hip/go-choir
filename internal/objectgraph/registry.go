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
	// Writer is the one writer of the kind: a desk, the runtime store acting
	// for the desks, or corpusd (writer.go; SDC ARCH-04). Owner names the
	// package; Writer names who may write through it.
	Writer Writer `json:"writer,omitempty"`
	// OwnerEdits admits the owner's own edit path as a second writer class of
	// this kind (canonical documents and revisions: AuthorUser beside
	// AuthorAppAgent).
	OwnerEdits bool `json:"owner_edits,omitempty"`
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
	kinds := []KindRegistration{
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
		{Kind: "choir.texture_document", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, OwnerEdits: true},
		{Kind: "choir.texture_revision", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Versioned: true, Owner: OwnerVMStore, OwnerEdits: true},
		{Kind: "choir.texture_decision", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.agent_evidence", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.EvidenceRecord"},
		{Kind: "choir.content_item", Store: StoreTypeDolt, IdentityMode: IdentityContentHash, Owner: OwnerVMStore},
		{Kind: "choir.podcast_subscription", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.browser_session", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.desktop_session", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.desktop_app_instance", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.worker_update", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.coagent_mailbox", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.source_schedule", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, OwnerEdits: true},
		{Kind: "choir.computer_setting", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		// An app's engineering lineage: the handoff its last engineer left
		// (docs/design/engineer-reuse-2026-11-04.md, phase B).
		{Kind: "choir.engineering_lineage", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.EngineeringLineage"},
		// The Texture root marker (PD3 S1): which document is the computer's
		// master document, and its generation, claimed by compare-and-set.
		{Kind: "choir.texture_root", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.TextureRoot"},
		// The claim of a stack rebase notice by the assignment that serves it
		// (U4): one per (operation, booted base), written by the assignment
		// open's own transaction.
		{Kind: "choir.rebase_notice_claim", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.StackRebaseClaim"},
		// The intake kinds (capture_label, capture_label_override,
		// capture_disposition, world_event, event_membership) are written by
		// background loops with no run record; under MA3.2 they declare
		// desk:management from the host loop. source_schedule is a standing
		// owner intent, so the owner's route writes it too (OwnerEdits).
		{Kind: "choir.capture_label", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		{Kind: "choir.capture_label_override", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},
		// The event layer's V1 (docs/reports/event-layer-at-ingestion-design-2026-10-16.md).
		{Kind: "choir.world_event", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.WorldEvent"},
		{Kind: "choir.event_membership", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.EventMembership"},
		// An owner's install or uninstall of a Library package release
		// (docs/design/package-install-2026-11-04.md). The kind was renamed
		// from its pre-MA2.4 name; records of the old kind stay registered
		// and readable until the store converts them (store/package_install_legacy.go).
		// The desk package event log (MA4.4): install, update, candidate, supplant
		// and disable facts the package desk registry is folded from; appended
		// by the boot reconcile (the kernel), keyed by sequence.
		{Kind: "choir.desk_package_event", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.DeskPackageEvent"},
		{Kind: "choir.lease", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "lease.Record"},
		{Kind: "choir.package_install", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.PackageInstall"},
		{Kind: "choir.release_adoption", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.PackageInstall"},
		// The file object (I27; docs/design/file-object-2026-10-24.md): a file
		// with a stable id and its plaintext digest, and the path view from a
		// live path to that id. Written by the runtime for the Files API and
		// the reconcile pass, behind the files.object computer setting.
		{Kind: "choir.file", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.FileObject"},
		{Kind: "choir.file_path", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.filePathView"},

		// The stream_seq lease: the highest stream_seq the computer may hand
		// out, an external-key singleton written ahead in blocks by the
		// store's allocator (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md).
		// system:store, not system:kernel (which admits every writer); the
		// hard fence in CheckWriteAs refuses any other writer in every gate
		// mode.
		{Kind: KindStreamSeqLease, Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore},

		// --- VM store: written through PutBatch before the write gate
		// existed (H-OG1, docs/archive/problems/object-graph-registration-and-anchor-gaps-2026-10-14.md).
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
		// The story-index projection (MA3.7a): Texture publishes it; management reads it.
		{Kind: "choir.story_index", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.StoryIndexEntry"},
		{Kind: "choir.support_verdict", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.SupportVerdict"},
		// Engineering records keep their frozen V1 kind spelling
		// (vocabmigrate frozenOGObjectKinds).
		{Kind: "choir.co_super_assignment", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.EngineeringAssignment"},
		{Kind: "choir.co_super_assignment_report", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.EngineeringAssignmentReport"},
		{Kind: "choir.co_super_subject_candidate", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "types.EngineeringSubjectCandidate"},
		{Kind: "choir.co_super_capability_claim", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.engineeringCapabilityClaim"},
		{Kind: "choir.co_super_capsule_claim", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.engineeringCapsuleClaim"},
		// The slot claim of a trajectory: one object per (trajectory, slot)
		// holding the run that owns it. Written by the store's slot claim
		// (docs/problems/data-has-more-than-one-home-2026-10-24.md, step 3).
		{Kind: "choir.co_super_slot", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerVMStore, Schema: "store.EngineeringSlotRecord"},
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
		// The entity layer (docs/problems/related-stories-no-entity-layer-2026-10-26.md;
		// ontology claims-events-entities-ontology-2026-10-14.md D3/D4): a
		// person, organization, place, product, work or law the paper's
		// stories are about, keyed by its Wikidata QID or its normalised
		// name. Written once by corpusd, as the export of the research desk's
		// story.entities tape records.
		{Kind: "choir.entity", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "platform.entityRow"},
		// The entity lexicon's learning loop (slice c0 §6): one review of a
		// story's entities per tape record, and a phrase's not-an-entity
		// evidence and stoplist state. Written by corpusd from the export.
		{Kind: "choir.entity_review", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "platform.entityReviewRow"},
		{Kind: "choir.entity_stopword", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "platform.entityStopwordRow"},
		// A published software release record (P7). Retired: its routes are
		// deleted and nothing writes one (docs/design/package-install-2026-11-04.md);
		// the records it left stay readable.
		{Kind: "choir.software_release", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "release.Record"},
		// The Choir Library (S2, docs/design/library-package-propagation-2026-10-26.md
		// section 13): a package release, one package's own change as source,
		// immutable; and a record's listing, one object per version, naming its
		// predecessor. corpusd writes both, on the owner's act.
		{Kind: "choir.package_release", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "pkgrelease.Record"},
		{Kind: "choir.release_listing", Store: StoreTypeDolt, IdentityMode: IdentityExternalKey, Owner: OwnerCorpusd, Schema: "pkgrelease.Listing"},
	}
	// The custodian of each kind is not written here: it is derived from the
	// desk and kernel-service manifests, and the load fails when a kind has
	// none or two (MA3.1).
	derived, err := DeriveCustody(kinds, ManifestClaims())
	if err != nil {
		panic(err)
	}
	for _, k := range derived {
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
		// corpusd: a publication version to each capture its anchors cite;
		// the related-stories walk's reverse index
		// (docs/reports/related-stories-graph-design-2026-10-19.md).
		{Kind: "cites_capture", Owner: OwnerCorpusd},
		// corpusd: a publication to a publication the research desk judged
		// related (the relation is symmetric; stored once, from < to). The
		// export of the computer's story.relation tape records, read by the
		// related-stories walk behind CORPUSD_RELATED_STORY.
		{Kind: "related_story", Owner: OwnerCorpusd},
		// corpusd: a publication's entity, with its role (central or
		// incidental); the export of story.entities, read by the
		// related-stories walk (central only).
		{Kind: "mentions_entity", Owner: OwnerCorpusd},
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
// (SQLStore before its validator and its projection interceptor, so a
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
