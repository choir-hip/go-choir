package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// PublicationGraphStore writes publication data as objects + edges in the
// object graph, replacing the relational publication tables. It is the
// graph-native implementation of the publication layer — the single write
// and read path for all publication data.
type PublicationGraphStore struct {
	store *ObjectGraphStore
}

// NewPublicationGraphStore creates a graph-native publication store backed
// by the given ObjectGraphStore.
func NewPublicationGraphStore(s *ObjectGraphStore) *PublicationGraphStore {
	return &PublicationGraphStore{store: s}
}

// PublishTextureToGraph writes a texture publication as objects + edges.
// It creates the same logical entities as Service.PublishTexture but in
// the object graph instead of relational tables.
func (p *PublicationGraphStore) PublishTextureToGraph(ctx context.Context, params PublishGraphParams) error {
	if p == nil || p.store == nil {
		return fmt.Errorf("publication graph: nil store")
	}

	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	batch := objectgraph.Batch{}

	// --- Subject (owner) ---
	subjectID, err := objectgraph.BuildCanonicalID("choir.subject", params.OwnerID, "self")
	if err != nil {
		return fmt.Errorf("publication graph: subject id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: subjectID,
		ObjectKind:  "choir.subject",
		OwnerID:     params.OwnerID,
		ContentHash: objectgraph.ContentHash("choir.subject", nil, mustJSONRaw(map[string]any{
			"subject_kind":  "user",
			"display_name":  params.OwnerID,
			"canonical_uri": "",
		})),
		Metadata:  mustJSONRaw(map[string]any{"subject_kind": "user", "display_name": params.OwnerID}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Publication proposal ---
	proposalSuffix := objectgraph.StableSuffixFromKey(params.ProposalID)
	proposalID, err := objectgraph.BuildCanonicalID("choir.publication_proposal", params.OwnerID, proposalSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: proposal id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: proposalID,
		ObjectKind:  "choir.publication_proposal",
		OwnerID:     params.OwnerID,
		ContentHash: objectgraph.ContentHash("choir.publication_proposal", nil, mustJSONRaw(map[string]any{
			"source_doc_id":         params.SourceDocID,
			"source_revision_id":    params.SourceRevisionID,
			"source_revision_hash":  params.SourceRevisionHash,
			"projection_hash":       params.ProjectionHash,
			"title":                 params.Title,
			"state":                 "published",
			"created_by":            params.RequestedBy,
			"created_trace_id":      params.SourceTraceID,
		})),
		Metadata: mustJSONRaw(map[string]any{
			"source_doc_id":        params.SourceDocID,
			"source_revision_id":   params.SourceRevisionID,
			"source_revision_hash": params.SourceRevisionHash,
			"projection_hash":      params.ProjectionHash,
			"title":                params.Title,
			"state":                "published",
			"created_by":           params.RequestedBy,
			"created_trace_id":     params.SourceTraceID,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Publication ---
	pubSuffix := objectgraph.StableSuffixFromKey(params.PublicationID)
	publicationID, err := objectgraph.BuildCanonicalID("choir.publication", params.OwnerID, pubSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: publication id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: publicationID,
		ObjectKind:  "choir.publication",
		OwnerID:     params.OwnerID,
		VersionID:   params.PublicationVersionID,
		ContentHash: objectgraph.ContentHash("choir.publication", nil, mustJSONRaw(map[string]any{
			"slug":    params.Slug,
			"title":   params.Title,
			"state":   "published",
		})),
		Metadata:  mustJSONRaw(map[string]any{"handle": "", "publication_id": params.PublicationID, "slug": params.Slug, "title": params.Title, "state": "published", "latest_version_id": params.PublicationVersionID}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Publication version ---
	versionSuffix := objectgraph.StableSuffixFromKey(params.PublicationVersionID)
	versionID, err := objectgraph.BuildCanonicalID("choir.publication_version", params.OwnerID, versionSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: version id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: versionID,
		ObjectKind:  "choir.publication_version",
		OwnerID:     params.OwnerID,
		VersionID:   params.PublicationVersionID,
		ContentHash: params.ContentHash,
		Metadata: mustJSONRaw(map[string]any{
			"edition_label":          "v1",
			"publication_id":         params.PublicationID,
			"publication_version_id": params.PublicationVersionID,
			"source_doc_id":          params.SourceDocID,
			"source_revision_id":     params.SourceRevisionID,
			"source_revision_hash":   params.SourceRevisionHash,
			"projection_hash":        params.ProjectionHash,
			"artifact_manifest_id":   params.ArtifactManifestID,
			"published_at":           now.Format(time.RFC3339),
			"supersedes_version_id":  "",
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Public route ---
	routeSuffix := objectgraph.StableSuffixFromKey(params.RoutePath)
	routeID, err := objectgraph.BuildCanonicalID("choir.public_route", params.OwnerID, routeSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: route id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: routeID,
		ObjectKind:  "choir.public_route",
		OwnerID:     params.OwnerID,
		VersionID:   params.PublicationVersionID,
		ContentHash: objectgraph.ContentHash("choir.public_route", nil, mustJSONRaw(map[string]any{
			"route_path":        params.RoutePath,
			"target_kind":       "publication",
			"target_id":         params.PublicationID,
			"target_version_id": params.PublicationVersionID,
			"state":             "active",
		})),
		Metadata:  mustJSONRaw(map[string]any{"handle": "", "route_path": params.RoutePath, "target_kind": "publication", "target_id": params.PublicationID, "target_version_id": params.PublicationVersionID, "state": "active"}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Artifact manifest ---
	manifestSuffix := objectgraph.StableSuffixFromKey(params.ArtifactManifestID)
	manifestID, err := objectgraph.BuildCanonicalID("choir.artifact_manifest", params.OwnerID, manifestSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: manifest id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: manifestID,
		ObjectKind:  "choir.artifact_manifest",
		OwnerID:     params.OwnerID,
		ContentHash: params.ManifestHash,
		Body:        params.ManifestJSON,
		Metadata:    mustJSONRaw(map[string]any{"subject_kind": "publication_version", "subject_id": params.PublicationVersionID, "media_type": "text/plain", "manifest_hash": params.ManifestHash}),
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	// --- Artifact blob ---
	blobSuffix := objectgraph.StableSuffixFromContent(params.ContentHash)
	blobID, err := objectgraph.BuildCanonicalID("choir.artifact_blob", params.OwnerID, blobSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: blob id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: blobID,
		ObjectKind:  "choir.artifact_blob",
		OwnerID:     params.OwnerID,
		ContentHash: params.ContentHash,
		Metadata: mustJSONRaw(map[string]any{
			"hash_algorithm":       "sha256",
			"media_type":           "text/plain",
			"byte_size":            fmt.Sprintf("%d", params.ContentSize),
			"storage_ref":          params.StorageRef,
			"artifact_manifest_id": params.ArtifactManifestID,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Consent record ---
	consentSuffix := objectgraph.StableSuffixFromKey(params.ConsentID)
	consentID, err := objectgraph.BuildCanonicalID("choir.consent_record", params.OwnerID, consentSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: consent id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: consentID,
		ObjectKind:  "choir.consent_record",
		OwnerID:     params.OwnerID,
		ContentHash: objectgraph.ContentHash("choir.consent_record", nil, mustJSONRaw(map[string]any{
			"target_kind":  "publication_version",
			"target_id":    params.PublicationVersionID,
			"action":       "publish",
			"state":        "granted",
			"evidence_ref": "requested_by:" + params.RequestedBy,
		})),
		Metadata:  mustJSONRaw(map[string]any{"target_kind": "publication_version", "target_id": params.PublicationVersionID, "action": "publish", "state": "granted", "evidence_ref": "requested_by:" + params.RequestedBy}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Review record ---
	reviewSuffix := objectgraph.StableSuffixFromKey(params.ReviewID)
	reviewID, err := objectgraph.BuildCanonicalID("choir.review_record", params.OwnerID, reviewSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: review id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: reviewID,
		ObjectKind:  "choir.review_record",
		OwnerID:     params.RequestedBy,
		ContentHash: objectgraph.ContentHash("choir.review_record", []byte("v0 owner consent publication path"), mustJSONRaw(map[string]any{
			"target_kind": "publication_version",
			"target_id":   params.PublicationVersionID,
			"decision":    "approve",
		})),
		Body:      []byte("v0 owner consent publication path"),
		Metadata:  mustJSONRaw(map[string]any{"target_kind": "publication_version", "target_id": params.PublicationVersionID, "decision": "approve"}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Retrieval source ---
	sourceSuffix := objectgraph.StableSuffixFromKey(params.RetrievalSourceID)
	retrievalSourceID, err := objectgraph.BuildCanonicalID("choir.retrieval_source", params.OwnerID, sourceSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: retrieval source id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: retrievalSourceID,
		ObjectKind:  "choir.retrieval_source",
		OwnerID:     params.OwnerID,
		ContentHash: params.ContentHash,
		Metadata: mustJSONRaw(map[string]any{
			"source_kind":    "publication_version",
			"canonical_uri":  params.PublicURI,
			"visibility":     "public",
			"state":          "active",
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Retrieval span ---
	spanSuffix := objectgraph.StableSuffixFromKey(params.RetrievalSpanID)
	retrievalSpanID, err := objectgraph.BuildCanonicalID("choir.retrieval_span", params.OwnerID, spanSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: retrieval span id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: retrievalSpanID,
		ObjectKind:  "choir.retrieval_span",
		OwnerID:     params.OwnerID,
		VersionID:   params.PublicationVersionID,
		ContentHash: params.ContentHash,
		Metadata: mustJSONRaw(map[string]any{
			"source_version_id": params.PublicationVersionID,
			"selector_kind":     "text_position",
			"selector_json":     params.WholeSelector,
			"text_hash":         params.ContentHash,
			"chunk_hash":        params.ContentHash,
			"token_count":       fmt.Sprintf("%d", params.TokenCount),
			"scope":             "whole_document",
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Retrieval manifest ---
	retManSuffix := objectgraph.StableSuffixFromKey(params.RetrievalManifestID)
	retrievalManifestID, err := objectgraph.BuildCanonicalID("choir.retrieval_manifest", params.OwnerID, retManSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: retrieval manifest id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: retrievalManifestID,
		ObjectKind:  "choir.retrieval_manifest",
		OwnerID:     params.OwnerID,
		ContentHash: objectgraph.SHA256([]byte("publish:" + params.PublicationVersionID)),
		Metadata: mustJSONRaw(map[string]any{
			"output_kind":             "publication_version",
			"output_id":               params.PublicationVersionID,
			"query_or_objective_hash": objectgraph.SHA256([]byte("publish:" + params.PublicationVersionID)),
			"index_manifest_id":       params.ArtifactManifestID,
			"selected_refs":           string(params.SelectedRefsJSON),
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Provenance: private entity (source revision) ---
	privateEntitySuffix := objectgraph.StableSuffixFromContent(params.SourceRevisionHash)
	privateEntityID, err := objectgraph.BuildCanonicalID("choir.provenance_entity", params.OwnerID, privateEntitySuffix)
	if err != nil {
		return fmt.Errorf("publication graph: private entity id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: privateEntityID,
		ObjectKind:  "choir.provenance_entity",
		OwnerID:     params.OwnerID,
		ContentHash: params.SourceRevisionHash,
		Metadata: mustJSONRaw(map[string]any{
			"entity_kind":    "private_texture_revision",
			"canonical_uri":  "choir-private:texture/" + params.SourceDocID + "/revisions/" + params.SourceRevisionID,
			"visibility":     "private",
			"projection":     "hash_only",
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Provenance: public entity (publication version) ---
	publicEntitySuffix := objectgraph.StableSuffixFromContent(params.ContentHash)
	publicEntityID, err := objectgraph.BuildCanonicalID("choir.provenance_entity", params.OwnerID, publicEntitySuffix)
	if err != nil {
		return fmt.Errorf("publication graph: public entity id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: publicEntityID,
		ObjectKind:  "choir.provenance_entity",
		OwnerID:     params.OwnerID,
		ContentHash: params.ContentHash,
		Body:        params.ManifestJSON,
		Metadata: mustJSONRaw(map[string]any{
			"entity_kind":   "publication_version",
			"canonical_uri": params.PublicURI,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Provenance: agent ---
	agentSuffix := objectgraph.StableSuffixFromKey(params.RequestedBy)
	provenanceAgentID, err := objectgraph.BuildCanonicalID("choir.provenance_agent", params.OwnerID, agentSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: provenance agent id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: provenanceAgentID,
		ObjectKind:  "choir.provenance_agent",
		OwnerID:     params.OwnerID,
		ContentHash: objectgraph.ContentHash("choir.provenance_agent", nil, mustJSONRaw(map[string]any{
			"agent_kind": "user",
			"subject_id": params.RequestedBy,
			"authority":  "owner_publish_v0",
		})),
		Metadata:  mustJSONRaw(map[string]any{"agent_kind": "user", "subject_id": params.RequestedBy, "authority": "owner_publish_v0"}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Provenance: activity ---
	activitySuffix := objectgraph.StableSuffixFromKey(params.ActivityID)
	provenanceActivityID, err := objectgraph.BuildCanonicalID("choir.provenance_activity", params.OwnerID, activitySuffix)
	if err != nil {
		return fmt.Errorf("publication graph: activity id: %w", err)
	}
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: provenanceActivityID,
		ObjectKind:  "choir.provenance_activity",
		OwnerID:     params.OwnerID,
		ContentHash: objectgraph.ContentHash("choir.provenance_activity", nil, mustJSONRaw(map[string]any{
			"activity_kind": "publish_texture_revision",
			"trace_id":      params.SourceTraceID,
			"started_at":    now.Format(time.RFC3339),
			"ended_at":      now.Format(time.RFC3339),
			"proposal_id":   params.ProposalID,
			"route_path":    params.RoutePath,
		})),
		Metadata:  mustJSONRaw(map[string]any{"activity_kind": "publish_texture_revision", "trace_id": params.SourceTraceID, "started_at": now.Format(time.RFC3339), "ended_at": now.Format(time.RFC3339), "proposal_id": params.ProposalID, "route_path": params.RoutePath}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Verifier attestation ---
	attSuffix := objectgraph.StableSuffixFromKey(params.AttestationID)
	attestationID, err := objectgraph.BuildCanonicalID("choir.verifier_attestation", params.OwnerID, attSuffix)
	if err != nil {
		return fmt.Errorf("publication graph: attestation id: %w", err)
	}
	attestationMeta := mustJSONRaw(map[string]any{
		"target_kind":      "publication_version",
		"target_id":        params.PublicationVersionID,
		"verifier_id":      "corpusd",
		"verifier_kind":    "service",
		"result":           "passed",
		"predicate_type":   "choir.platform.publish_texture.v0",
		"subject_digest":   params.ContentHash,
		"route_path":       params.RoutePath,
		"source_revision_hash": params.SourceRevisionHash,
	})
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: attestationID,
		ObjectKind:  "choir.verifier_attestation",
		OwnerID:     "corpusd",
		ContentHash: params.ContentHash,
		Body:        params.AttestationEvidenceJSON,
		Metadata:    attestationMeta,
		CreatedAt:   now,
		UpdatedAt:   now,
	})

	// --- Publication policy ---
	if len(params.AccessPolicy) > 0 || len(params.ExportPolicy) > 0 {
		policySuffix := objectgraph.StableSuffixFromContent(objectgraph.SHA256(append(params.AccessPolicy, params.ExportPolicy...)))
		policyID, err := objectgraph.BuildCanonicalID("choir.publication_policy", params.OwnerID, policySuffix)
		if err != nil {
			return fmt.Errorf("publication graph: policy id: %w", err)
		}
		batch.Objects = append(batch.Objects, objectgraph.Object{
			CanonicalID: policyID,
			ObjectKind:  "choir.publication_policy",
			OwnerID:     params.OwnerID,
			VersionID:   params.PublicationVersionID,
			ContentHash: objectgraph.SHA256(append(params.AccessPolicy, params.ExportPolicy...)),
			Metadata: mustJSONRaw(map[string]any{
				"access_policy_json": string(params.AccessPolicy),
				"export_policy_json": string(params.ExportPolicy),
			}),
			CreatedAt: now,
			UpdatedAt: now,
		})
		// Edge: version -> policy
		batch.Edges = append(batch.Edges, makeEdge(versionID, policyID, "has_policy", now, mustJSONRaw(map[string]any{})))
	}

	// --- Source entities ---
	for _, se := range params.SourceEntities {
		seSuffix := objectgraph.StableSuffixFromKey(se.SourceEntityID)
		seID, err := objectgraph.BuildCanonicalID("choir.publication_source_entity", params.OwnerID, seSuffix)
		if err != nil {
			return fmt.Errorf("publication graph: source entity id: %w", err)
		}
		batch.Objects = append(batch.Objects, objectgraph.Object{
			CanonicalID: seID,
			ObjectKind:  "choir.publication_source_entity",
			OwnerID:     params.OwnerID,
			VersionID:   params.PublicationVersionID,
			ContentHash: objectgraph.SHA256(se.EntityJSON),
			Body:        se.EntityJSON,
			Metadata: mustJSONRaw(map[string]any{
				"kind":           se.Kind,
				"target_kind":    se.TargetKind,
				"target_id":      se.TargetID,
				"display_policy": se.DisplayPolicy,
				"open_surface":   se.OpenSurface,
			}),
			CreatedAt: now,
			UpdatedAt: now,
		})
		batch.Edges = append(batch.Edges, makeEdge(versionID, seID, "references_entity", now, mustJSONRaw(map[string]any{
			"display_policy": se.DisplayPolicy,
			"open_surface":   se.OpenSurface,
		})))
	}

	// --- Transclusions ---
	for _, tr := range params.Transclusions {
		trSuffix := objectgraph.StableSuffixFromContent(tr.ContentHash)
		trID, err := objectgraph.BuildCanonicalID("choir.publication_transclusion", params.OwnerID, trSuffix)
		if err != nil {
			return fmt.Errorf("publication graph: transclusion id: %w", err)
		}
		batch.Objects = append(batch.Objects, objectgraph.Object{
			CanonicalID: trID,
			ObjectKind:  "choir.publication_transclusion",
			OwnerID:     params.OwnerID,
			VersionID:   params.PublicationVersionID,
			ContentHash: tr.ContentHash,
			Body:        []byte(tr.SnapshotText),
			Metadata: mustJSONRaw(map[string]any{
				"source_entity_id":        tr.SourceEntityID,
				"host_selector_json":      string(tr.HostSelector),
				"source_selector_json":    string(tr.SourceSelector),
				"relation_type":           tr.RelationType,
				"default_display_mode":    tr.DefaultDisplayMode,
				"access_policy_json":      string(params.AccessPolicy),
				"export_policy_json":      string(params.ExportPolicy),
			}),
			CreatedAt: now,
			UpdatedAt: now,
		})
		batch.Edges = append(batch.Edges, makeEdge(versionID, trID, "transcludes", now, mustJSONRaw(map[string]any{
			"host_selector_json":   string(tr.HostSelector),
			"default_display_mode": tr.DefaultDisplayMode,
		})))
	}

	// --- Edges: structural ---
	batch.Edges = append(batch.Edges,
		// subject -> publication
		makeEdge(subjectID, publicationID, "owns", now, mustJSONRaw(map[string]any{})),
		// publication -> version
		makeEdge(publicationID, versionID, "has_version", now, mustJSONRaw(map[string]any{"edition_label": "v1", "published_at": now.Format(time.RFC3339)})),
		// version -> proposal
		makeEdge(versionID, proposalID, "derived_from_proposal", now, mustJSONRaw(map[string]any{})),
		// route -> publication
		makeEdge(routeID, publicationID, "routes_to", now, mustJSONRaw(map[string]any{"state": "active"})),
		// version -> manifest
		makeEdge(versionID, manifestID, "has_manifest", now, mustJSONRaw(map[string]any{"media_type": "text/plain"})),
		// manifest -> blob
		makeEdge(manifestID, blobID, "contains_blob", now, mustJSONRaw(map[string]any{"media_type": "text/plain", "byte_size": fmt.Sprintf("%d", params.ContentSize)})),
		// subject -> consent
		makeEdge(subjectID, consentID, "granted_consent", now, mustJSONRaw(map[string]any{"action": "publish", "state": "granted"})),
		// consent -> version
		makeEdge(consentID, versionID, "consent_for", now, mustJSONRaw(map[string]any{"action": "publish", "state": "granted"})),
		// subject (reviewer) -> review
		makeEdge(subjectID, reviewID, "authored_review", now, mustJSONRaw(map[string]any{"decision": "approve"})),
		// review -> version
		makeEdge(reviewID, versionID, "reviews", now, mustJSONRaw(map[string]any{"decision": "approve"})),
		// source -> span
		makeEdge(retrievalSourceID, retrievalSpanID, "contains_span", now, mustJSONRaw(map[string]any{"selector_kind": "text_position", "token_count": fmt.Sprintf("%d", params.TokenCount)})),
		// version -> retrieval manifest
		makeEdge(versionID, retrievalManifestID, "has_retrieval_manifest", now, mustJSONRaw(map[string]any{})),
		// subject -> provenance agent
		makeEdge(subjectID, provenanceAgentID, "has_agent", now, mustJSONRaw(map[string]any{"agent_kind": "user"})),
		// public entity -> private entity (wasDerivedFrom)
		makeEdge(publicEntityID, privateEntityID, "was_derived_from", now, mustJSONRaw(map[string]any{"activity_id": params.ActivityID, "source_private_content": "not_copied_as_private_ref"})),
		// agent -> activity (wasAssociatedWith)
		makeEdge(provenanceActivityID, provenanceAgentID, "was_associated_with", now, mustJSONRaw(map[string]any{})),
		// activity -> public entity (generated)
		makeEdge(provenanceActivityID, publicEntityID, "generated", now, mustJSONRaw(map[string]any{})),
		// agent -> attestation (attested)
		makeEdge(provenanceAgentID, attestationID, "attested", now, mustJSONRaw(map[string]any{"verifier_kind": "service", "result": "passed"})),
		// attestation -> version (attests_to)
		makeEdge(attestationID, versionID, "attests_to", now, mustJSONRaw(map[string]any{"predicate_type": "choir.platform.publish_texture.v0", "result": "passed"})),
	)

	// --- Citation edges ---
	for _, cite := range params.Citations {
		citeEdge := makeEdge(versionID, cite.ToID, objectgraph.EdgeKind(cite.RelationType), now, mustJSONRaw(map[string]any{
			"from_selector_json": cite.FromSelector,
			"to_selector_json":   cite.ToSelector,
			"state":              cite.State,
			"proposed_by":        params.RequestedBy,
			"evidence_ref":       cite.EvidenceRef,
			"confidence":         fmt.Sprintf("%.1f", cite.Confidence),
		}))
		batch.Edges = append(batch.Edges, citeEdge)
	}

	return p.store.PutBatch(ctx, batch)
}

// PublishGraphParams holds the parameters for publishing a texture to the
// object graph. It mirrors the logical data that Service.PublishTexture
// writes to relational tables.
type PublishGraphParams struct {
	OwnerID              string
	RequestedBy          string
	SourceDocID          string
	SourceRevisionID     string
	SourceRevisionHash   string
	SourceTraceID        string
	Title                string
	Slug                 string
	Content              string
	ContentHash          string
	ContentSize          int
	ProjectionHash       string
	BodyDoc              json.RawMessage
	WholeSelector        string
	PublicURI            string
	RoutePath            string
	StorageRef           string
	ManifestJSON         []byte
	ManifestHash         string
	TokenCount           int
	SelectedRefsJSON     json.RawMessage

	// IDs (generated by the caller, same as the relational path)
	PublicationID           string
	ProposalID              string
	PublicationVersionID    string
	ArtifactManifestID      string
	ConsentID               string
	ReviewID                string
	RetrievalSourceID       string
	RetrievalSpanID         string
	RetrievalManifestID     string
	ActivityID              string
	AttestationID           string

	// Attestation evidence
	AttestationEvidenceJSON json.RawMessage

	// Policies
	AccessPolicy  json.RawMessage
	ExportPolicy  json.RawMessage

	// Source entities
	SourceEntities []GraphSourceEntity

	// Transclusions
	Transclusions []GraphTransclusion

	// Citations
	Citations []GraphCitation

	Now time.Time
}

// GraphSourceEntity is a source entity referenced in a publication.
type GraphSourceEntity struct {
	SourceEntityID string
	Kind           string
	TargetKind     string
	TargetID       string
	DisplayPolicy  string
	OpenSurface    string
	EntityJSON     []byte
}

// GraphTransclusion is a transclusion embedded in a publication.
type GraphTransclusion struct {
	SourceEntityID      string
	HostSelector        json.RawMessage
	SourceSelector      json.RawMessage
	RelationType        string
	DefaultDisplayMode  string
	SnapshotText        string
	ContentHash         string
	EntityJSON          []byte
}

// GraphCitation is a citation edge from a publication version to a target.
type GraphCitation struct {
	ToID          string
	RelationType  string
	FromSelector  string
	ToSelector    string
	State         string
	EvidenceRef   string
	Confidence    float64
}

func makeEdge(fromID, toID string, kind objectgraph.EdgeKind, now time.Time, metadata json.RawMessage) objectgraph.Edge {
	edgeID, _ := objectgraph.BuildEdgeID(fromID, toID, kind, metadata)
	return objectgraph.Edge{
		EdgeID:    edgeID,
		FromID:    fromID,
		ToID:      toID,
		Kind:      kind,
		Metadata:  metadata,
		CreatedAt: now,
	}
}

// SubmitProposalToGraph writes a reader-submitted publication proposal
// as objects + edges in the object graph.
func (p *PublicationGraphStore) SubmitProposalToGraph(ctx context.Context, params ProposalGraphParams) error {
	if p == nil || p.store == nil {
		return fmt.Errorf("publication graph: nil store")
	}

	now := params.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()

	batch := objectgraph.Batch{}

	// --- Subject (submitter) ---
	subjectID, _ := objectgraph.BuildCanonicalID("choir.subject", params.SubmitterID, "self")
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: subjectID,
		ObjectKind:  "choir.subject",
		OwnerID:     params.SubmitterID,
		ContentHash: objectgraph.ContentHash("choir.subject", nil, mustJSONRaw(map[string]any{
			"subject_kind":  "user",
			"display_name":  params.SubmitterID,
		})),
		Metadata:  mustJSONRaw(map[string]any{"subject_kind": "user", "display_name": params.SubmitterID}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Proposal ---
	proposalSuffix := objectgraph.StableSuffixFromKey(params.ProposalID)
	proposalID, _ := objectgraph.BuildCanonicalID("choir.publication_proposal", params.SubmitterID, proposalSuffix)
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: proposalID,
		ObjectKind:  "choir.publication_proposal",
		OwnerID:     params.SubmitterID,
		ContentHash: params.ContentHash,
		Body:        []byte(params.Content),
		Metadata: mustJSONRaw(map[string]any{
			"proposal_id":                  params.ProposalID,
			"publication_id":               params.PublicationID,
			"publication_version_id":       params.PublicationVersionID,
			"source_owner_id":              params.SourceOwnerID,
			"submitter_id":                 params.SubmitterID,
			"submitter_doc_id":             params.SubmitterDocID,
			"submitter_revision_id":        params.SubmitterRevisionID,
			"submitter_revision_hash":      params.ProposalRevisionHash,
			"content_hash":                 params.ContentHash,
			"projection_hash":              params.ProjectionHash,
			"artifact_manifest_id":         params.ArtifactManifestID,
			"title":                        params.Title,
			"state":                        "proposed",
			"delivery_id":                  params.DeliveryID,
			"delivery_state":               "recorded_for_author",
			"delivery_ref":                 params.DeliveryRef,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Artifact manifest ---
	manifestSuffix := objectgraph.StableSuffixFromKey(params.ArtifactManifestID)
	manifestID, _ := objectgraph.BuildCanonicalID("choir.artifact_manifest", params.SubmitterID, manifestSuffix)
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: manifestID,
		ObjectKind:  "choir.artifact_manifest",
		OwnerID:     params.SubmitterID,
		ContentHash: params.ManifestHash,
		Body:        params.ManifestJSON,
		Metadata: mustJSONRaw(map[string]any{
			"subject_kind": "publication_version_proposal",
			"subject_id":   params.ProposalID,
			"media_type":   "text/plain",
			"manifest_hash": params.ManifestHash,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Artifact blob ---
	blobSuffix := objectgraph.StableSuffixFromContent(params.ContentHash)
	blobID, _ := objectgraph.BuildCanonicalID("choir.artifact_blob", params.SubmitterID, blobSuffix)
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: blobID,
		ObjectKind:  "choir.artifact_blob",
		OwnerID:     params.SubmitterID,
		ContentHash: params.ContentHash,
		Metadata: mustJSONRaw(map[string]any{
			"hash_algorithm":       "sha256",
			"media_type":           "text/plain",
			"byte_size":            fmt.Sprintf("%d", params.ContentSize),
			"storage_ref":          params.StorageRef,
			"artifact_manifest_id": params.ArtifactManifestID,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Provenance: source entity (the publication version being proposed against) ---
	sourceEntitySuffix := objectgraph.StableSuffixFromContent(params.SourceVersionContentHash)
	sourceEntityID, _ := objectgraph.BuildCanonicalID("choir.provenance_entity", params.SubmitterID, sourceEntitySuffix)
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: sourceEntityID,
		ObjectKind:  "choir.provenance_entity",
		OwnerID:     params.SubmitterID,
		ContentHash: params.SourceVersionContentHash,
		Metadata: mustJSONRaw(map[string]any{
			"entity_kind":   "publication_version",
			"canonical_uri": "choir:publication/" + params.PublicationID + "/versions/" + params.PublicationVersionID,
			"publication_id": params.PublicationID,
			"publication_version_id": params.PublicationVersionID,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Provenance: proposal entity ---
	proposalEntitySuffix := objectgraph.StableSuffixFromContent(params.ContentHash)
	proposalEntityID, _ := objectgraph.BuildCanonicalID("choir.provenance_entity", params.SubmitterID, proposalEntitySuffix)
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: proposalEntityID,
		ObjectKind:  "choir.provenance_entity",
		OwnerID:     params.SubmitterID,
		ContentHash: params.ContentHash,
		Body:        params.ManifestJSON,
		Metadata: mustJSONRaw(map[string]any{
			"entity_kind":   "publication_version_proposal",
			"canonical_uri": "choir:publication/" + params.PublicationID + "/proposals/" + params.ProposalID,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Provenance: agent ---
	agentSuffix := objectgraph.StableSuffixFromKey(params.SubmitterID)
	provenanceAgentID, _ := objectgraph.BuildCanonicalID("choir.provenance_agent", params.SubmitterID, agentSuffix)
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: provenanceAgentID,
		ObjectKind:  "choir.provenance_agent",
		OwnerID:     params.SubmitterID,
		ContentHash: objectgraph.ContentHash("choir.provenance_agent", nil, mustJSONRaw(map[string]any{
			"agent_kind": "user",
			"subject_id": params.SubmitterID,
			"authority":  "reader_derivative_proposal_v0",
		})),
		Metadata:  mustJSONRaw(map[string]any{"agent_kind": "user", "subject_id": params.SubmitterID, "authority": "reader_derivative_proposal_v0"}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Provenance: activity ---
	activitySuffix := objectgraph.StableSuffixFromKey(params.ActivityID)
	provenanceActivityID, _ := objectgraph.BuildCanonicalID("choir.provenance_activity", params.SubmitterID, activitySuffix)
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: provenanceActivityID,
		ObjectKind:  "choir.provenance_activity",
		OwnerID:     params.SubmitterID,
		ContentHash: objectgraph.ContentHash("choir.provenance_activity", nil, mustJSONRaw(map[string]any{
			"activity_kind":         "submit_publication_derivative_proposal",
			"started_at":            now.Format(time.RFC3339),
			"ended_at":              now.Format(time.RFC3339),
			"proposal_id":           params.ProposalID,
			"source_publication_id": params.PublicationID,
		})),
		Metadata: mustJSONRaw(map[string]any{
			"activity_kind":         "submit_publication_derivative_proposal",
			"started_at":            now.Format(time.RFC3339),
			"ended_at":              now.Format(time.RFC3339),
			"proposal_id":           params.ProposalID,
			"source_publication_id": params.PublicationID,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Verifier attestation ---
	attSuffix := objectgraph.StableSuffixFromKey(params.AttestationID)
	attestationID, _ := objectgraph.BuildCanonicalID("choir.verifier_attestation", params.SubmitterID, attSuffix)
	batch.Objects = append(batch.Objects, objectgraph.Object{
		CanonicalID: attestationID,
		ObjectKind:  "choir.verifier_attestation",
		OwnerID:     "corpusd",
		ContentHash: params.ContentHash,
		Metadata: mustJSONRaw(map[string]any{
			"target_kind":      "publication_version_proposal",
			"target_id":        params.ProposalID,
			"verifier_id":      "corpusd",
			"verifier_kind":    "service",
			"result":           "passed",
			"predicate_type":   "choir.platform.reader_proposal.v0",
			"subject_digest":   params.ContentHash,
			"source_publication_id":         params.PublicationID,
			"source_publication_version_id": params.PublicationVersionID,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})

	// --- Edges ---
	batch.Edges = append(batch.Edges,
		// proposal -> manifest
		makeEdge(proposalID, manifestID, "has_manifest", now, mustJSONRaw(map[string]any{"media_type": "text/plain"})),
		// manifest -> blob
		makeEdge(manifestID, blobID, "contains_blob", now, mustJSONRaw(map[string]any{"media_type": "text/plain", "byte_size": fmt.Sprintf("%d", params.ContentSize)})),
		// subject -> proposal (owns)
		makeEdge(subjectID, proposalID, "owns", now, mustJSONRaw(map[string]any{})),
		// proposal entity -> source entity (was_derived_from)
		makeEdge(proposalEntityID, sourceEntityID, "was_derived_from", now, mustJSONRaw(map[string]any{"activity_id": params.ActivityID, "delivery_id": params.DeliveryID})),
		// activity -> agent (was_associated_with)
		makeEdge(provenanceActivityID, provenanceAgentID, "was_associated_with", now, mustJSONRaw(map[string]any{})),
		// activity -> proposal entity (generated)
		makeEdge(provenanceActivityID, proposalEntityID, "generated", now, mustJSONRaw(map[string]any{})),
		// agent -> attestation (attested)
		makeEdge(provenanceAgentID, attestationID, "attested", now, mustJSONRaw(map[string]any{"verifier_kind": "service", "result": "passed"})),
		// attestation -> proposal (attests_to)
		makeEdge(attestationID, proposalID, "attests_to", now, mustJSONRaw(map[string]any{"predicate_type": "choir.platform.reader_proposal.v0", "result": "passed"})),
	)

	// --- Transclusion edges ---
	for _, tr := range params.TransclusionEdges {
		batch.Edges = append(batch.Edges, makeEdge(proposalID, tr.ToID, "transcludes", now, mustJSONRaw(map[string]any{
			"to_kind":        tr.ToKind,
			"to_selector":    tr.Selector,
			"state":          "proposed",
			"proposed_by":    params.SubmitterID,
			"evidence_ref":   tr.EvidenceRef,
			"confidence":     "0.9",
		})))
	}

	// --- Citation edges ---
	for _, cite := range params.CitationEdges {
		batch.Edges = append(batch.Edges, makeEdge(proposalID, cite.ToID, "references", now, mustJSONRaw(map[string]any{
			"state":        "candidate",
			"proposed_by":  params.SubmitterID,
			"evidence_ref": cite.EvidenceRef,
			"confidence":   "0.5",
		})))
	}

	return p.store.PutBatch(ctx, batch)
}

// UpdateProposalDeliveryState updates the delivery_state metadata on a
// proposal object in the graph.
func (p *PublicationGraphStore) UpdateProposalDeliveryState(ctx context.Context, proposalID, deliveryID, deliveryState, deliveryRef string, now time.Time) error {
	if p == nil || p.store == nil {
		return fmt.Errorf("publication graph: nil store")
	}
	// Find the proposal object by metadata proposal_id.
	// We don't know the owner, so we search by metadata.
	objs, err := p.store.ListObjectsByMetadata(ctx, "choir.publication_proposal", "$.proposal_id", proposalID, 1)
	if err != nil || len(objs) == 0 {
		return fmt.Errorf("publication graph: proposal not found: %s", proposalID)
	}
	obj := objs[0]
	// Update metadata with new delivery state.
	var meta map[string]any
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		return fmt.Errorf("publication graph: parse proposal metadata: %w", err)
	}
	meta["delivery_state"] = deliveryState
	meta["delivery_ref"] = deliveryRef
	updatedMeta, _ := json.Marshal(meta)
	obj.Metadata = json.RawMessage(updatedMeta)
	obj.UpdatedAt = now
	return p.store.PutObject(ctx, obj)
}

// ProposalGraphParams holds parameters for writing a publication proposal
// to the object graph.
type ProposalGraphParams struct {
	ProposalID              string
	PublicationID           string
	PublicationVersionID    string
	SourceOwnerID           string
	SubmitterID             string
	SubmitterDocID          string
	SubmitterRevisionID     string
	ProposalRevisionHash    string
	Content                 string
	ContentHash             string
	ContentSize             int
	ProjectionHash          string
	Title                   string
	ArtifactManifestID      string
	ManifestJSON            []byte
	ManifestHash            string
	StorageRef              string
	DeliveryID              string
	DeliveryRef             string
	ActivityID              string
	AttestationID           string
	SourceVersionContentHash string
	TransclusionEdges       []ProposalTransclusionEdge
	CitationEdges           []ProposalCitationEdge
	Now                     time.Time
}

type ProposalTransclusionEdge struct {
	ToID       string
	ToKind     string
	Selector   string
	EvidenceRef string
}

type ProposalCitationEdge struct {
	ToID       string
	EvidenceRef string
}

// Compile-time assertion that PublicationGraphStore is usable.
var _ *PublicationGraphStore = (*PublicationGraphStore)(nil)
