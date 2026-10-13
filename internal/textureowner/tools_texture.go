package textureowner

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// tools_texture.go holds what a committed Texture revision writes besides
// its body: provenance and the source graph. The write itself is
// choir.Write (texture_cell_author.go, internal/texturewrite).

// textureUpdateDisposition settles one leftover update packet in a turn;
// choir.Reject on an update id is the only explicit disposition left.
type textureUpdateDisposition struct {
	UpdateID    string `json:"update_id"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
}

// materializedTextureEdit is a revision body and its source entities, the
// input of the source graph write.
type materializedTextureEdit struct {
	BodyDoc        json.RawMessage
	SourceEntities json.RawMessage
}

func isTextureWriteToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case "texture_cell":
		return true
	default:
		return false
	}
}

func buildStructuredAppagentRevisionProvenance(rec *types.RunRecord, sourceEntitiesRaw json.RawMessage, now time.Time) json.RawMessage {
	var structured []texturedoc.SourceEntity
	if len(strings.TrimSpace(string(sourceEntitiesRaw))) > 0 {
		_ = json.Unmarshal(sourceEntitiesRaw, &structured)
	}
	sources := make([]types.SourceEntity, 0, len(structured))
	for _, entity := range structured {
		sources = append(sources, provenanceSourceEntityFromStructured(entity))
	}
	prov := types.Provenance{
		SchemaVersion: types.ProvenanceSchemaVersion,
		AuthoredAt:    now.UTC(),
		Sources:       sources,
	}
	if rec != nil {
		prov.AuthoringModel = types.ProvenanceModel{
			Provider: strings.TrimSpace(metadataStringValue(rec.Metadata, "provider")),
			Model:    strings.TrimSpace(metadataStringValue(rec.Metadata, "model")),
		}
	}
	canonical, err := prov.CanonicalJSON()
	if err != nil {
		return nil
	}
	return json.RawMessage(canonical)
}

func provenanceSourceEntityFromStructured(entity texturedoc.SourceEntity) types.SourceEntity {
	targetKind := strings.TrimSpace(entity.Target.Kind)
	targetMetadata := entity.Target.Metadata
	readerSnapshotStatus := copyStringAnyMap(entity.ReaderSnapshotStatus)
	readerArtifactState := strings.TrimSpace(entity.Evidence.ReaderArtifactState)
	if len(readerSnapshotStatus) == 0 && readerArtifactState != "" {
		readerSnapshotStatus = map[string]any{"state": readerArtifactState}
	}
	return types.SourceEntity{
		EntityID: strings.TrimSpace(entity.SourceEntityID),
		Kind:     targetKind,
		Label:    strings.TrimSpace(firstNonEmpty(entity.Display.Title, entity.Display.Label, entity.Target.ID, entity.Target.URI)),
		Target: types.SourceEntityTarget{
			TargetKind:   targetKind,
			ItemID:       firstNonEmpty(metadataString(targetMetadata, "item_id"), strings.TrimSpace(entity.Target.ID)),
			SourceID:     metadataString(targetMetadata, "source_id"),
			FetchID:      metadataString(targetMetadata, "fetch_id"),
			ContentID:    firstNonEmpty(metadataString(targetMetadata, "content_id"), strings.TrimSpace(entity.Target.ID)),
			URL:          firstNonEmpty(metadataString(targetMetadata, "url"), strings.TrimSpace(entity.Target.URI)),
			CanonicalURL: firstNonEmpty(metadataString(targetMetadata, "canonical_url"), metadataString(targetMetadata, "url"), strings.TrimSpace(entity.Target.URI)),
		},
		Selectors:            provenanceSourceSelectorsFromStructured(entity.Selectors),
		Display:              provenanceSourceDisplayFromStructured(entity.Display, entity.Evidence),
		Evidence:             types.SourceEntityEvidence{State: strings.TrimSpace(entity.Evidence.State), ReaderSnapshot: len(entity.ReaderSnapshot) > 0, SourceRepresentationID: readerArtifactState},
		Provenance:           types.SourceEntityProvenance{CreatedBy: strings.TrimSpace(entity.Provenance.CreatedBy)},
		ReaderSnapshot:       copyStringAnyMap(entity.ReaderSnapshot),
		ReaderSnapshotStatus: readerSnapshotStatus,
	}
}

func provenanceSourceSelectorsFromStructured(selectors []texturedoc.SourceSelector) []types.SourceEntitySelector {
	out := make([]types.SourceEntitySelector, 0, len(selectors))
	for _, selector := range selectors {
		out = append(out, types.SourceEntitySelector{
			SelectorKind: strings.TrimSpace(selector.Kind),
			TextQuote:    metadataString(selector.Data, "exact"),
		})
	}
	return out
}

func provenanceSourceDisplayFromStructured(display texturedoc.SourceDisplay, evidence texturedoc.SourceEvidence) types.SourceEntityDisplay {
	return types.SourceEntityDisplay{
		InlineMode:   strings.TrimSpace(display.Mode),
		ExpandedMode: strings.TrimSpace(display.Mode),
		OpenSurface:  strings.TrimSpace(evidence.OpenSurface),
	}
}

func textureToolSourceGraphWriteSet(rev types.Revision, materialized materializedTextureEdit, rec *types.RunRecord) (store.TextureSourceGraphWriteSet, error) {
	var entities []texturedoc.SourceEntity
	rawEntities := strings.TrimSpace(string(materialized.SourceEntities))
	if rawEntities != "" && rawEntities != "null" {
		if err := json.Unmarshal(materialized.SourceEntities, &entities); err != nil {
			return store.TextureSourceGraphWriteSet{}, fmt.Errorf("decode structured source entities: %w", err)
		}
	}
	records := make([]store.TextureSourceEntityGraphRecord, 0, len(entities))
	recordsByLegacyID := make(map[string]store.TextureSourceEntityGraphRecord, len(entities))
	recordsByCanonicalID := map[string]store.TextureSourceEntityGraphRecord{}
	for i, entity := range entities {
		record, err := textureToolSourceEntityGraphRecord(rev, entity, rec)
		if err != nil {
			return store.TextureSourceGraphWriteSet{}, fmt.Errorf("source_entities[%d]: %w", i, err)
		}
		if existing, ok := recordsByCanonicalID[record.CanonicalID]; ok {
			if legacyID := strings.TrimSpace(record.LegacySourceEntityID); legacyID != "" {
				recordsByLegacyID[legacyID] = existing
			}
			continue
		}
		recordsByCanonicalID[record.CanonicalID] = record
		records = append(records, record)
		if legacyID := strings.TrimSpace(record.LegacySourceEntityID); legacyID != "" {
			recordsByLegacyID[legacyID] = record
		}
	}
	graph := store.TextureSourceGraphWriteSet{SourceEntities: records}
	rawBodyDoc := strings.TrimSpace(string(materialized.BodyDoc))
	if rawBodyDoc == "" || rawBodyDoc == "null" {
		return graph, nil
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(materialized.BodyDoc, &doc); err != nil {
		return store.TextureSourceGraphWriteSet{}, fmt.Errorf("decode structured body_doc: %w", err)
	}
	refs, err := textureToolSourceRefGraphRecords(rev, doc, recordsByLegacyID, rec)
	if err != nil {
		return store.TextureSourceGraphWriteSet{}, err
	}
	graph.SourceRefs = refs
	return graph, nil
}

func textureToolSourceRefGraphRecords(rev types.Revision, doc texturedoc.StructuredTextureDoc, entitiesByLegacyID map[string]store.TextureSourceEntityGraphRecord, rec *types.RunRecord) ([]store.TextureSourceRefGraphRecord, error) {
	var refs []store.TextureSourceRefGraphRecord
	var walk func(node texturedoc.Node, path string) error
	walk = func(node texturedoc.Node, path string) error {
		if node.Type == "source_ref" {
			ref, err := textureToolSourceRefGraphRecord(rev, node, path, entitiesByLegacyID, rec)
			if err != nil {
				return err
			}
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

func textureToolSourceRefGraphRecord(rev types.Revision, node texturedoc.Node, path string, entitiesByLegacyID map[string]store.TextureSourceEntityGraphRecord, rec *types.RunRecord) (store.TextureSourceRefGraphRecord, error) {
	nodeID := textureNodeStringAttr(node, "id")
	sourceEntityID := textureNodeStringAttr(node, "source_entity_id")
	if sourceEntityID == "" {
		return store.TextureSourceRefGraphRecord{}, fmt.Errorf("source_ref at %s has no source_entity_id", path)
	}
	entity, ok := entitiesByLegacyID[sourceEntityID]
	if !ok {
		return store.TextureSourceRefGraphRecord{}, fmt.Errorf("source_ref at %s source_entity_id %q does not resolve to a graph source entity version", path, sourceEntityID)
	}
	displayMode := textureNodeStringAttr(node, "display_mode")
	if displayMode == "" {
		displayMode = store.TextureSourceRefDisplayNumbered
	}
	pathHash := objectgraph.SHA256([]byte(path))
	occurrenceKey := pathHash + "\x00" + nodeID + "\x00" + sourceEntityID
	canonicalID, err := store.BuildTextureSourceRefCanonicalIDByScope(rev.OwnerID, rev.ComputerID, rev.RevisionID, occurrenceKey)
	if err != nil {
		return store.TextureSourceRefGraphRecord{}, err
	}
	metadata := map[string]any{
		"identity_key":               occurrenceKey,
		"schema_version":             "choir.source_ref.v1",
		"doc_id":                     rev.DocID,
		"texture_revision_id":        rev.RevisionID,
		"body_node_id":               nodeID,
		"body_node_path_hash":        pathHash,
		"legacy_source_entity_id":    sourceEntityID,
		"source_entity_canonical_id": entity.CanonicalID,
		"source_entity_version_id":   entity.VersionID,
		"display_mode":               displayMode,
		"citation_state":             "cited",
		"texture_parent_revision_id": rev.ParentRevisionID,
	}
	if computerID := strings.TrimSpace(rev.ComputerID); computerID != "" {
		metadata["computer_id"] = computerID
	}
	if rec != nil {
		if runID := strings.TrimSpace(rec.RunID); runID != "" {
			metadata["created_run_id"] = runID
		}
	}
	normalized, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		return store.TextureSourceRefGraphRecord{}, err
	}
	return store.TextureSourceRefGraphRecord{
		CanonicalID:             canonicalID,
		OwnerID:                 rev.OwnerID,
		ComputerID:              strings.TrimSpace(rev.ComputerID),
		DocID:                   rev.DocID,
		TextureRevisionID:       rev.RevisionID,
		BodyNodeID:              nodeID,
		BodyNodePathHash:        pathHash,
		LegacySourceEntityID:    sourceEntityID,
		SourceEntityCanonicalID: entity.CanonicalID,
		SourceEntityVersionID:   entity.VersionID,
		DisplayMode:             displayMode,
		CitationState:           "cited",
		Metadata:                normalized,
		CreatedAt:               rev.CreatedAt,
	}, nil
}

func textureToolSourceEntityGraphRecord(rev types.Revision, entity texturedoc.SourceEntity, rec *types.RunRecord) (store.TextureSourceEntityGraphRecord, error) {
	sourceKind := strings.TrimSpace(entity.Target.Kind)
	if sourceKind == "" {
		sourceKind = "source_entity"
	}
	targetIdentity := textureToolSourceEntityTargetIdentity(entity)
	if targetIdentity == "" {
		return store.TextureSourceEntityGraphRecord{}, fmt.Errorf("target identity is required")
	}
	ownerScope := rev.OwnerID
	if computerID := strings.TrimSpace(rev.ComputerID); computerID != "" {
		ownerScope += "\x00" + computerID
	}
	canonicalID, err := store.BuildTextureSourceEntityCanonicalID(rev.OwnerID, ownerScope, sourceKind, targetIdentity)
	if err != nil {
		return store.TextureSourceEntityGraphRecord{}, err
	}
	body := textureToolSourceEntityBody(entity)
	metadata, err := textureToolSourceEntityGraphMetadata(rev, entity, sourceKind, targetIdentity, rec)
	if err != nil {
		return store.TextureSourceEntityGraphRecord{}, err
	}
	versionID, contentHash, normalized, err := store.TextureSourceGraphVersionID(store.TextureSourceEntityObjectKind, body, metadata)
	if err != nil {
		return store.TextureSourceEntityGraphRecord{}, err
	}
	return store.TextureSourceEntityGraphRecord{
		CanonicalID:          canonicalID,
		OwnerID:              rev.OwnerID,
		ComputerID:           strings.TrimSpace(rev.ComputerID),
		VersionID:            versionID,
		ContentHash:          contentHash,
		Body:                 body,
		Metadata:             normalized,
		LegacySourceEntityID: strings.TrimSpace(entity.SourceEntityID),
		CreatedAt:            rev.CreatedAt,
	}, nil
}

func textureToolSourceEntityTargetIdentity(entity texturedoc.SourceEntity) string {
	if uri := strings.TrimSpace(entity.Target.URI); uri != "" {
		return uri
	}
	if id := strings.TrimSpace(entity.Target.ID); id != "" {
		return id
	}
	return strings.TrimSpace(entity.SourceEntityID)
}

func textureToolSourceEntityBody(entity texturedoc.SourceEntity) []byte {
	for _, key := range []string{"text", "summary", "content"} {
		if value, ok := entity.ReaderSnapshot[key].(string); ok && strings.TrimSpace(value) != "" {
			return []byte(strings.TrimSpace(value))
		}
	}
	return nil
}

func textureToolSourceEntityGraphMetadata(rev types.Revision, entity texturedoc.SourceEntity, sourceKind, targetIdentity string, rec *types.RunRecord) (json.RawMessage, error) {
	target := map[string]any{
		"kind":     sourceKind,
		"identity": targetIdentity,
	}
	if uri := strings.TrimSpace(entity.Target.URI); uri != "" {
		target["uri"] = uri
	}
	if id := strings.TrimSpace(entity.Target.ID); id != "" {
		target["id"] = id
	}
	if len(entity.Target.Metadata) > 0 {
		target["metadata"] = entity.Target.Metadata
	}
	display := map[string]any{}
	if title := strings.TrimSpace(entity.Display.Title); title != "" {
		display["title"] = title
	}
	if label := strings.TrimSpace(entity.Display.Label); label != "" {
		display["label"] = label
	}
	if description := strings.TrimSpace(entity.Display.Description); description != "" {
		display["description"] = description
	}
	if mode := strings.TrimSpace(entity.Display.Mode); mode != "" {
		display["display_mode"] = mode
	}
	evidence := map[string]any{}
	if state := strings.TrimSpace(entity.Evidence.State); state != "" {
		evidence["state"] = state
	}
	if openSurface := strings.TrimSpace(entity.Evidence.OpenSurface); openSurface != "" {
		evidence["open_surface"] = openSurface
	}
	if relation := strings.TrimSpace(entity.Evidence.Relation); relation != "" {
		evidence["relation"] = relation
	}
	if researchState := strings.TrimSpace(entity.Evidence.ResearchState); researchState != "" {
		evidence["research_state"] = researchState
	}
	if uncertainty := strings.TrimSpace(entity.Evidence.Uncertainty); uncertainty != "" {
		evidence["uncertainty"] = uncertainty
	}
	if readerArtifactState := strings.TrimSpace(entity.Evidence.ReaderArtifactState); readerArtifactState != "" {
		evidence["reader_artifact_state"] = readerArtifactState
	}
	if len(entity.Evidence.EvidenceRefs) > 0 {
		evidence["evidence_refs"] = entity.Evidence.EvidenceRefs
	}
	provenance := map[string]any{}
	if createdBy := strings.TrimSpace(entity.Provenance.CreatedBy); createdBy != "" {
		provenance["created_by"] = createdBy
	}
	if createdAt := strings.TrimSpace(entity.Provenance.CreatedAt); createdAt != "" {
		provenance["created_at"] = createdAt
	}
	if sourceSystem := strings.TrimSpace(entity.Provenance.SourceSystem); sourceSystem != "" {
		provenance["source_system"] = sourceSystem
	}
	if importArtifact := strings.TrimSpace(entity.Provenance.ImportArtifact); importArtifact != "" {
		provenance["import_artifact"] = importArtifact
	}
	if rightsScope := strings.TrimSpace(entity.Provenance.RightsScope); rightsScope != "" {
		provenance["rights_scope"] = rightsScope
	}
	if entity.Provenance.UntrustedSourceText {
		provenance["untrusted_source_text"] = true
	}
	metadata := map[string]any{
		"schema_version":          "choir.source_entity.v1",
		"legacy_entity_id":        strings.TrimSpace(entity.SourceEntityID),
		"source_kind":             sourceKind,
		"target":                  target,
		"display":                 display,
		"evidence":                evidence,
		"provenance":              provenance,
		"texture_doc_id":          rev.DocID,
		"texture_revision_id":     rev.RevisionID,
		"texture_parent_revision": rev.ParentRevisionID,
	}
	if computerID := strings.TrimSpace(rev.ComputerID); computerID != "" {
		metadata["computer_id"] = computerID
	}
	if rec != nil {
		if runID := strings.TrimSpace(rec.RunID); runID != "" {
			metadata["created_run_id"] = runID
		}
	}
	if len(entity.Selectors) > 0 {
		metadata["selectors"] = entity.Selectors
	}
	if len(entity.ReaderSnapshotStatus) > 0 {
		metadata["reader_snapshot_status"] = entity.ReaderSnapshotStatus
	}
	return objectgraph.NormalizeMetadata(metadata)
}

func sanitizeTextureToolRevisionMetadata(raw json.RawMessage) json.RawMessage {
	meta := decodeRevisionMetadata(raw)
	if meta == nil {
		return raw
	}
	for _, key := range []string{
		"source_entities",
		"media_source_refs",
		"source_gaps",
		"source_repair_resolutions",
		"source_attachment_manifest",
		"source_ref_normalization",
		"citations_json",
	} {
		delete(meta, key)
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return raw
	}
	return data
}

func structuredSourceEntitiesFromRuntimeSources(value any) []texturedoc.SourceEntity {
	runtimeEntities := decodeTextureSourceEntities(value)
	out := make([]texturedoc.SourceEntity, 0, len(runtimeEntities))
	for _, entity := range runtimeEntities {
		structured := structuredSourceEntityFromRuntimeSource(entity)
		if strings.TrimSpace(structured.SourceEntityID) != "" {
			out = append(out, structured)
		}
	}
	return out
}

func structuredSourceEntityFromRuntimeSource(entity textureSourceEntity) texturedoc.SourceEntity {
	targetKind := structuredSourceTargetKind(entity)
	displayMode := structuredSourceDisplayMode(entity)
	return texturedoc.SourceEntity{
		SourceEntityID: strings.TrimSpace(entity.EntityID),
		Target: texturedoc.SourceTarget{
			Kind: targetKind,
			URI:  strings.TrimSpace(firstNonEmpty(entity.Target.CanonicalURL, entity.Target.URL)),
			ID: strings.TrimSpace(firstNonEmpty(
				entity.Target.ItemID,
				entity.Target.ContentID,
				entity.Target.FilePath,
				entity.Target.PublicRecordID,
				entity.Target.DocID,
				entity.Target.RevisionID,
				entity.EntityID,
			)),
			Metadata: pruneEmptyMap(map[string]any{
				"target_kind":   strings.TrimSpace(entity.Target.TargetKind),
				"item_id":       strings.TrimSpace(entity.Target.ItemID),
				"source_id":     strings.TrimSpace(entity.Target.SourceID),
				"fetch_id":      strings.TrimSpace(entity.Target.FetchID),
				"content_id":    strings.TrimSpace(entity.Target.ContentID),
				"canonical_url": strings.TrimSpace(entity.Target.CanonicalURL),
				"url":           strings.TrimSpace(entity.Target.URL),
			}),
		},
		Selectors:            structuredSourceSelectorsFromRuntime(entity.Selectors),
		Display:              texturedoc.SourceDisplay{Mode: displayMode, Title: strings.TrimSpace(entity.Label)},
		Evidence:             structuredSourceEvidenceFromRuntime(entity),
		Provenance:           texturedoc.SourceEntityProvenance{CreatedBy: strings.TrimSpace(firstNonEmpty(entity.Provenance.CreatedBy, "runtime"))},
		ReaderSnapshot:       copyStringAnyMap(entity.ReaderSnapshot),
		ReaderSnapshotStatus: copyStringAnyMap(entity.ReaderSnapshotStatus),
	}
}

func structuredSourceTargetKind(entity textureSourceEntity) string {
	switch strings.TrimSpace(entity.Target.TargetKind) {
	case "source_service_item":
		return "source_service_item"
	case "content_item":
		return "content_item"
	case "image", "video", "audio", "pdf", "transcript", "texture_span", "publication_span", "source_viewer_artifact", "reader_artifact", "file_artifact",
		"command_output", "shell_session", "diff_hunk", "patch", "test_run", "capsule_bundle", "screenshot", "video_artifact", "benchmark_log":
		return strings.TrimSpace(entity.Target.TargetKind)
	case "url", "web_url":
		return "web_url"
	default:
		switch strings.TrimSpace(entity.Kind) {
		case "image":
			return "image"
		case "youtube_video", "video":
			return "video"
		default:
			return "content_item"
		}
	}
}

func structuredSourceDisplayMode(entity textureSourceEntity) string {
	// source_embed display modes (block_embed, excerpt, player, image_preview,
	// pdf_pages, transcript, source_window, inline_chip) collapsed into the
	// source_ref display_mode enum after the hard cutover. Legacy expanded
	// block modes map to expanded_ref; everything else is the default
	// numbered_ref inline citation.
	expandedModes := map[string]bool{
		"block_embed":   true,
		"excerpt":       true,
		"player":        true,
		"image_preview": true,
		"pdf_pages":     true,
		"transcript":    true,
		"source_window": true,
		"expanded_ref":  true,
	}
	mode := strings.TrimSpace(firstNonEmpty(entity.Display.ExpandedMode, entity.Display.InlineMode))
	if expandedModes[mode] {
		return "expanded_ref"
	}
	return "numbered_ref"
}

func structuredSourceSelectorsFromRuntime(selectors []textureSourceEntitySelector) []texturedoc.SourceSelector {
	if len(selectors) == 0 {
		return []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindWholeResource}}
	}
	out := make([]texturedoc.SourceSelector, 0, len(selectors))
	for _, selector := range selectors {
		kind := sourcecontract.NormalizeSelectorKind(selector.SelectorKind)
		if kind == "" {
			kind = sourcecontract.SelectorKindWholeResource
		}
		data := map[string]any{}
		if selector.TextQuote != "" {
			data["exact"] = selector.TextQuote
		}
		if selector.ContentHash != "" {
			data["content_hash"] = selector.ContentHash
		}
		out = append(out, texturedoc.SourceSelector{Kind: kind, Data: data})
	}
	return out
}

func structuredSourceEvidenceFromRuntime(entity textureSourceEntity) texturedoc.SourceEvidence {
	state := sourcecontract.NormalizeEvidenceState(entity.Evidence.State)
	if state == "" {
		state = sourcecontract.EvidenceStateAvailable
	}
	openSurface := sourcecontract.NormalizeOpenSurface(entity.Display.OpenSurface)
	if openSurface == "" {
		openSurface = sourcecontract.OpenSurfaceSource
	}
	return texturedoc.SourceEvidence{
		State:               state,
		OpenSurface:         openSurface,
		ReaderArtifactState: sourcecontract.NormalizeReaderArtifactState(entity.Evidence.SourceRepresentationID),
	}
}

func textureNodeStringAttr(node texturedoc.Node, key string) string {
	if node.Attrs == nil {
		return ""
	}
	if value, ok := node.Attrs[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func metadataBoolValue(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	switch value := metadata[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func trajectoryIDForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if trajectoryID := strings.TrimSpace(rec.TrajectoryID); trajectoryID != "" {
		return trajectoryID
	}
	return metadataStringValue(rec.Metadata, "trajectory_id")
}
