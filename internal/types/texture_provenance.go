package types

import (
	"encoding/json"
	"sort"
	"time"
)

// ProvenanceSchemaVersion is the current schema version of the typed,
// system-attributed provenance record stored per Texture revision. Bump it when
// the Provenance shape changes in a way readers must branch on.
const ProvenanceSchemaVersion = 1

// Provenance is the system-attributed grounding record for a single Texture
// revision. It is filled by the runtime from ground truth and is NEVER authored
// by the model: the model writes only the document body and inline citation
// markers that reference SourceEntity ids in Sources. The runtime records which
// model authored the revision, when, which queries were actually executed, and
// the collated sources behind the body.
//
// Provenance is a typed struct with NO map fields on purpose: that makes
// CanonicalJSON produce deterministic, signable bytes. Those bytes are the
// substrate for the per-revision hash chain (mission D2) and for future digital
// signatures (out of scope now). Do not add map[string]any fields here.
type Provenance struct {
	// SchemaVersion is the Provenance schema version (ProvenanceSchemaVersion).
	SchemaVersion int `json:"schema_version"`

	// AuthoringModel is the model the runtime observed authoring this revision.
	AuthoringModel ProvenanceModel `json:"authoring_model"`

	// AuthoredAt is when the runtime recorded this revision's authorship.
	AuthoredAt time.Time `json:"authored_at"`

	// QueriesExecuted is the ordered list of research queries the runtime
	// actually observed being executed for this revision. Order is meaningful.
	QueriesExecuted []ProvenanceQuery `json:"queries_executed,omitempty"`

	// Sources is the collated source list behind this revision's body. Inline
	// citation markers in the body reference SourceEntity.EntityID values here.
	Sources []SourceEntity `json:"sources,omitempty"`

	// ParentLinks are the revision's links beyond its chain parent: the
	// revision it responds to (base) and the published versions it takes from
	// (source). They are inside the revision hash, so a link cannot be altered
	// after the fact. Written by the host at the revise call, never by a model;
	// absent on every revision that has none, so old hashes are unchanged.
	ParentLinks []ParentLink `json:"parent_links,omitempty"`
}

// Parent link roles.
const (
	ParentLinkBase   = "base"
	ParentLinkSource = "source"
)

// ParentLink is one link from a revision to a thing it was made from. A base
// link names a revision of the same document (RevisionID); a source link names
// a published version (PublicationVersionID) pinned by its manifest hash.
type ParentLink struct {
	Role                 string `json:"role"`
	RevisionID           string `json:"revision_id,omitempty"`
	PublicationVersionID string `json:"publication_version_id,omitempty"`
	ManifestHash         string `json:"manifest_hash,omitempty"`
	// RoutePath is the version's public path, for display only.
	RoutePath string `json:"route_path,omitempty"`
	// AuthorLabel is the source author's public handle when the link was made.
	AuthorLabel string `json:"author_label,omitempty"`
}

// ref is the link's sort key within its role.
func (l ParentLink) ref() string {
	if l.RevisionID != "" {
		return l.RevisionID
	}
	return l.PublicationVersionID
}

// ProvenanceModel identifies the model the runtime observed authoring a revision.
type ProvenanceModel struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

// ProvenanceQuery is a single research query the runtime observed being executed
// while producing a revision. It is system-attributed, not model-claimed.
type ProvenanceQuery struct {
	Tool        string `json:"tool,omitempty"`
	Query       string `json:"query"`
	ResultCount int    `json:"result_count,omitempty"`
}

// CanonicalJSON returns deterministic, signable bytes for this Provenance
// record. Determinism comes from two things: the struct has no map fields (Go
// marshals struct fields in declaration order), and CanonicalJSON sorts Sources
// by EntityID so the bytes do not depend on the order sources were collated.
// QueriesExecuted order is preserved because execution order is meaningful.
func (p Provenance) CanonicalJSON() ([]byte, error) {
	canonical := p
	if len(canonical.Sources) > 0 {
		sources := make([]SourceEntity, len(canonical.Sources))
		copy(sources, canonical.Sources)
		sort.SliceStable(sources, func(i, j int) bool {
			return sources[i].EntityID < sources[j].EntityID
		})
		canonical.Sources = sources
	}
	if len(canonical.ParentLinks) > 0 {
		links := make([]ParentLink, len(canonical.ParentLinks))
		copy(links, canonical.ParentLinks)
		sort.SliceStable(links, func(i, j int) bool {
			if links[i].Role != links[j].Role {
				return links[i].Role < links[j].Role
			}
			return links[i].ref() < links[j].ref()
		})
		canonical.ParentLinks = links
	}
	return json.Marshal(canonical)
}

// SourceEntity is the canonical collated-source schema referenced by inline
// citation markers in a Texture body. It is the single home for what the runtime
// previously defined privately; runtime aliases this type. A source id
// (EntityID) is minted by the runtime at the retrieval boundary, never chosen by
// a model.
type SourceEntity struct {
	EntityID             string                 `json:"entity_id"`
	Kind                 string                 `json:"kind"`
	Label                string                 `json:"label,omitempty"`
	Target               SourceEntityTarget     `json:"target"`
	Selectors            []SourceEntitySelector `json:"selectors,omitempty"`
	Display              SourceEntityDisplay    `json:"display"`
	Evidence             SourceEntityEvidence   `json:"evidence"`
	Provenance           SourceEntityProvenance `json:"provenance"`
	ReaderSnapshot       map[string]any         `json:"reader_snapshot,omitempty"`
	ReaderSnapshotStatus map[string]any         `json:"reader_snapshot_status,omitempty"`
}

// SourceEntityTarget identifies what a source entity points at.
type SourceEntityTarget struct {
	TargetKind           string `json:"target_kind"`
	ItemID               string `json:"item_id,omitempty"`
	SourceID             string `json:"source_id,omitempty"`
	FetchID              string `json:"fetch_id,omitempty"`
	ContentID            string `json:"content_id,omitempty"`
	FilePath             string `json:"file_path,omitempty"`
	DocID                string `json:"doc_id,omitempty"`
	RevisionID           string `json:"revision_id,omitempty"`
	PublicationID        string `json:"publication_id,omitempty"`
	PublicationVersionID string `json:"publication_version_id,omitempty"`
	PublicRecordID       string `json:"public_record_id,omitempty"`
	URL                  string `json:"url,omitempty"`
	CanonicalURL         string `json:"canonical_url,omitempty"`
}

// SourceEntitySelector binds a citation to a specific span of a source.
type SourceEntitySelector struct {
	SelectorKind string  `json:"selector_kind"`
	StartSeconds float64 `json:"start_seconds,omitempty"`
	EndSeconds   float64 `json:"end_seconds,omitempty"`
	TextQuote    string  `json:"text_quote,omitempty"`
	ContentHash  string  `json:"content_hash,omitempty"`
}

// SourceEntityDisplay controls how a source renders inline and expanded.
type SourceEntityDisplay struct {
	InlineMode       string `json:"inline_mode"`
	ExpandedMode     string `json:"expanded_mode"`
	OpenSurface      string `json:"open_surface,omitempty"`
	DefaultCollapsed bool   `json:"default_collapsed"`
}

// SourceEntityEvidence records the evidentiary state of a source.
type SourceEntityEvidence struct {
	State                  string `json:"state"`
	ResearchState          string `json:"research_state,omitempty"`
	Relation               string `json:"relation,omitempty"`
	BodyKind               string `json:"body_kind,omitempty"`
	BodyLength             int    `json:"body_length,omitempty"`
	ReaderSnapshot         bool   `json:"reader_snapshot,omitempty"`
	TranscriptContentID    string `json:"transcript_content_id,omitempty"`
	TranscriptAvailability string `json:"transcript_availability,omitempty"`
	SourceRepresentationID string `json:"source_representation_id,omitempty"`
	Uncertainty            string `json:"uncertainty,omitempty"`
}

// SourceEntityProvenance records who created a source entity and its rights.
type SourceEntityProvenance struct {
	CreatedBy           string `json:"created_by"`
	RightsScope         string `json:"rights_scope,omitempty"`
	UntrustedSourceText bool   `json:"untrusted_source_text,omitempty"`
}
