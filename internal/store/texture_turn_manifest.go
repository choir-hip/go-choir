package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// validateTextureTurnSourceManifest proves that the conditional source-graph
// write set is exactly the graph projection of the canonical structured
// revision. It rejects missing, duplicate, or extra entities/refs and any
// independently supplied identity, selector, display, open-surface, or hash.
func validateTextureTurnSourceManifest(rev types.Revision, graph TextureSourceGraphWriteSet, callerRunID string) error {
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(rev.BodyDoc, &doc); err != nil {
		return fmt.Errorf("decode body_doc: %w", err)
	}
	var entities []texturedoc.SourceEntity
	if len(strings.TrimSpace(string(rev.SourceEntities))) > 0 {
		if err := json.Unmarshal(rev.SourceEntities, &entities); err != nil {
			return fmt.Errorf("decode source_entities: %w", err)
		}
	}
	if len(graph.SourceEntities) != len(entities) {
		return fmt.Errorf("source entity graph count %d does not match canonical manifest %d", len(graph.SourceEntities), len(entities))
	}
	byLegacyID := make(map[string]TextureSourceEntityGraphRecord, len(entities))
	seenCanonical := make(map[string]struct{}, len(entities))
	for i, entity := range entities {
		expected, err := textureTurnExpectedSourceEntity(rev, entity, callerRunID)
		if err != nil {
			return fmt.Errorf("source_entities[%d]: %w", i, err)
		}
		actual, err := normalizeTextureSourceEntityGraphRecord(graph.SourceEntities[i], rev.OwnerID, rev.ComputerID, rev.CreatedAt)
		if err != nil {
			return fmt.Errorf("source graph entities[%d]: %w", i, err)
		}
		if actual.CanonicalID != expected.CanonicalID || actual.VersionID != expected.VersionID || actual.ContentHash != expected.ContentHash ||
			actual.LegacySourceEntityID != expected.LegacySourceEntityID || !bytes.Equal(actual.Body, expected.Body) ||
			!bytes.Equal(actual.Metadata, expected.Metadata) {
			return fmt.Errorf("source graph entities[%d] does not exactly project source_entity_id %q", i, entity.SourceEntityID)
		}
		if _, duplicate := seenCanonical[actual.CanonicalID]; duplicate {
			return fmt.Errorf("source graph aliases more than one canonical source entity")
		}
		seenCanonical[actual.CanonicalID] = struct{}{}
		byLegacyID[entity.SourceEntityID] = actual
	}
	expectedRefs, err := textureTurnExpectedSourceRefs(rev, doc, byLegacyID, callerRunID)
	if err != nil {
		return err
	}
	if len(graph.SourceRefs) != len(expectedRefs) {
		return fmt.Errorf("source ref graph count %d does not match canonical manifest %d", len(graph.SourceRefs), len(expectedRefs))
	}
	seenRefs := make(map[string]struct{}, len(expectedRefs))
	for i, expected := range expectedRefs {
		actual, err := normalizeTextureSourceRefGraphRecord(graph.SourceRefs[i], rev, rev.CreatedAt)
		if err != nil {
			return fmt.Errorf("source graph refs[%d]: %w", i, err)
		}
		if actual.CanonicalID != expected.CanonicalID || actual.VersionID != expected.VersionID || actual.ContentHash != expected.ContentHash ||
			actual.DocID != expected.DocID || actual.TextureRevisionID != expected.TextureRevisionID || actual.BodyNodeID != expected.BodyNodeID ||
			actual.BodyNodePathHash != expected.BodyNodePathHash || actual.LegacySourceEntityID != expected.LegacySourceEntityID ||
			actual.SourceEntityCanonicalID != expected.SourceEntityCanonicalID || actual.SourceEntityVersionID != expected.SourceEntityVersionID ||
			actual.DisplayMode != expected.DisplayMode || actual.CitationState != expected.CitationState || !bytes.Equal(actual.Metadata, expected.Metadata) {
			return fmt.Errorf("source graph refs[%d] does not exactly project body source_ref", i)
		}
		identity := actual.CanonicalID + "\x00" + actual.VersionID
		if _, duplicate := seenRefs[identity]; duplicate {
			return fmt.Errorf("source graph contains duplicate source_ref identity")
		}
		seenRefs[identity] = struct{}{}
	}
	return nil
}

func textureTurnExpectedSourceEntity(rev types.Revision, entity texturedoc.SourceEntity, callerRunID string) (TextureSourceEntityGraphRecord, error) {
	sourceKind := strings.TrimSpace(entity.Target.Kind)
	targetIdentity := strings.TrimSpace(entity.Target.URI)
	if targetIdentity == "" {
		targetIdentity = strings.TrimSpace(entity.Target.ID)
	}
	if targetIdentity == "" {
		targetIdentity = strings.TrimSpace(entity.SourceEntityID)
	}
	ownerScope := rev.OwnerID
	if computerID := strings.TrimSpace(rev.ComputerID); computerID != "" {
		ownerScope += "\x00" + computerID
	}
	canonicalID, err := BuildTextureSourceEntityCanonicalID(rev.OwnerID, ownerScope, sourceKind, targetIdentity)
	if err != nil {
		return TextureSourceEntityGraphRecord{}, err
	}
	body := textureTurnSourceEntityBody(entity)
	metadata, err := textureTurnSourceEntityMetadata(rev, entity, sourceKind, targetIdentity, callerRunID)
	if err != nil {
		return TextureSourceEntityGraphRecord{}, err
	}
	versionID, contentHash, metadata, err := TextureSourceGraphVersionID(TextureSourceEntityObjectKind, body, metadata)
	if err != nil {
		return TextureSourceEntityGraphRecord{}, err
	}
	return TextureSourceEntityGraphRecord{
		CanonicalID: canonicalID, OwnerID: rev.OwnerID, ComputerID: rev.ComputerID, VersionID: versionID,
		ContentHash: contentHash, Body: body, Metadata: metadata, LegacySourceEntityID: strings.TrimSpace(entity.SourceEntityID), CreatedAt: rev.CreatedAt,
	}, nil
}

func textureTurnSourceEntityBody(entity texturedoc.SourceEntity) []byte {
	for _, key := range []string{"text", "summary", "content"} {
		if value, ok := entity.ReaderSnapshot[key].(string); ok && strings.TrimSpace(value) != "" {
			return []byte(strings.TrimSpace(value))
		}
	}
	return nil
}

func textureTurnSourceEntityMetadata(rev types.Revision, entity texturedoc.SourceEntity, sourceKind, targetIdentity, callerRunID string) (json.RawMessage, error) {
	target := map[string]any{"kind": sourceKind, "identity": targetIdentity}
	if value := strings.TrimSpace(entity.Target.URI); value != "" {
		target["uri"] = value
	}
	if value := strings.TrimSpace(entity.Target.ID); value != "" {
		target["id"] = value
	}
	if len(entity.Target.Metadata) > 0 {
		target["metadata"] = entity.Target.Metadata
	}
	display := map[string]any{}
	if value := strings.TrimSpace(entity.Display.Title); value != "" {
		display["title"] = value
	}
	if value := strings.TrimSpace(entity.Display.Label); value != "" {
		display["label"] = value
	}
	if value := strings.TrimSpace(entity.Display.Description); value != "" {
		display["description"] = value
	}
	if value := strings.TrimSpace(entity.Display.Mode); value != "" {
		display["display_mode"] = value
	}
	evidence := map[string]any{}
	if value := strings.TrimSpace(entity.Evidence.State); value != "" {
		evidence["state"] = value
	}
	if value := strings.TrimSpace(entity.Evidence.OpenSurface); value != "" {
		evidence["open_surface"] = value
	}
	if value := strings.TrimSpace(entity.Evidence.Relation); value != "" {
		evidence["relation"] = value
	}
	if value := strings.TrimSpace(entity.Evidence.ResearchState); value != "" {
		evidence["research_state"] = value
	}
	if value := strings.TrimSpace(entity.Evidence.Uncertainty); value != "" {
		evidence["uncertainty"] = value
	}
	if value := strings.TrimSpace(entity.Evidence.ReaderArtifactState); value != "" {
		evidence["reader_artifact_state"] = value
	}
	if len(entity.Evidence.EvidenceRefs) > 0 {
		evidence["evidence_refs"] = entity.Evidence.EvidenceRefs
	}
	provenance := map[string]any{}
	if value := strings.TrimSpace(entity.Provenance.CreatedBy); value != "" {
		provenance["created_by"] = value
	}
	if value := strings.TrimSpace(entity.Provenance.CreatedAt); value != "" {
		provenance["created_at"] = value
	}
	if value := strings.TrimSpace(entity.Provenance.SourceSystem); value != "" {
		provenance["source_system"] = value
	}
	if value := strings.TrimSpace(entity.Provenance.ImportArtifact); value != "" {
		provenance["import_artifact"] = value
	}
	if value := strings.TrimSpace(entity.Provenance.RightsScope); value != "" {
		provenance["rights_scope"] = value
	}
	if entity.Provenance.UntrustedSourceText {
		provenance["untrusted_source_text"] = true
	}
	metadata := map[string]any{
		"schema_version": "choir.source_entity.v1", "legacy_entity_id": strings.TrimSpace(entity.SourceEntityID),
		"source_kind": sourceKind, "target": target, "display": display, "evidence": evidence, "provenance": provenance,
		"texture_doc_id": rev.DocID, "texture_revision_id": rev.RevisionID, "texture_parent_revision": rev.ParentRevisionID,
	}
	if rev.ComputerID != "" {
		metadata["computer_id"] = rev.ComputerID
	}
	if callerRunID != "" {
		metadata["created_run_id"] = callerRunID
	}
	if len(entity.Selectors) > 0 {
		metadata["selectors"] = entity.Selectors
	}
	if len(entity.ReaderSnapshotStatus) > 0 {
		metadata["reader_snapshot_status"] = entity.ReaderSnapshotStatus
	}
	return objectgraph.NormalizeMetadata(metadata)
}

func textureTurnExpectedSourceRefs(rev types.Revision, doc texturedoc.StructuredTextureDoc, entities map[string]TextureSourceEntityGraphRecord, callerRunID string) ([]TextureSourceRefGraphRecord, error) {
	var refs []TextureSourceRefGraphRecord
	var walk func(texturedoc.Node, string) error
	walk = func(node texturedoc.Node, path string) error {
		if node.Type == "source_ref" {
			nodeID := textureTurnNodeAttr(node, "id")
			legacyID := textureTurnNodeAttr(node, "source_entity_id")
			entity, ok := entities[legacyID]
			if !ok {
				return fmt.Errorf("source_ref %s does not resolve", path)
			}
			displayMode := textureTurnNodeAttr(node, "display_mode")
			if displayMode == "" {
				displayMode = TextureSourceRefDisplayNumbered
			}
			pathHash := objectgraph.SHA256([]byte(path))
			occurrenceKey := pathHash + "\x00" + nodeID + "\x00" + legacyID
			canonicalID, err := BuildTextureSourceRefCanonicalIDByScope(rev.OwnerID, rev.ComputerID, rev.RevisionID, occurrenceKey)
			if err != nil {
				return err
			}
			metadata := map[string]any{
				"identity_key": occurrenceKey, "schema_version": "choir.source_ref.v1", "doc_id": rev.DocID,
				"texture_revision_id": rev.RevisionID, "body_node_id": nodeID, "body_node_path_hash": pathHash,
				"legacy_source_entity_id": legacyID, "source_entity_canonical_id": entity.CanonicalID,
				"source_entity_version_id": entity.VersionID, "display_mode": displayMode, "citation_state": "cited",
				"texture_parent_revision_id": rev.ParentRevisionID,
			}
			if rev.ComputerID != "" {
				metadata["computer_id"] = rev.ComputerID
			}
			if callerRunID != "" {
				metadata["created_run_id"] = callerRunID
			}
			normalized, err := objectgraph.NormalizeMetadata(metadata)
			if err != nil {
				return err
			}
			ref := TextureSourceRefGraphRecord{
				CanonicalID: canonicalID, OwnerID: rev.OwnerID, ComputerID: rev.ComputerID, DocID: rev.DocID,
				TextureRevisionID: rev.RevisionID, BodyNodeID: nodeID, BodyNodePathHash: pathHash, LegacySourceEntityID: legacyID,
				SourceEntityCanonicalID: entity.CanonicalID, SourceEntityVersionID: entity.VersionID,
				DisplayMode: displayMode, CitationState: "cited", Metadata: normalized, CreatedAt: rev.CreatedAt,
			}
			versionID, contentHash, normalizedMetadata, err := TextureSourceGraphVersionID(TextureSourceRefObjectKind, sourceRefVersionBody(ref), ref.Metadata)
			if err != nil {
				return err
			}
			ref.VersionID, ref.ContentHash, ref.Metadata = versionID, contentHash, normalizedMetadata
			refs = append(refs, ref)
		}
		for i, child := range node.Content {
			if err := walk(child, fmt.Sprintf("%s.content[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc.Doc, "doc"); err != nil {
		return nil, err
	}
	return refs, nil
}

func textureTurnNodeAttr(node texturedoc.Node, key string) string {
	value, _ := node.Attrs[key].(string)
	return strings.TrimSpace(value)
}
