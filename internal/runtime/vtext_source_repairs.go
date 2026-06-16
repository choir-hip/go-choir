package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func (h *APIHandler) HandleVTextSourceGapRepair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}
	var req vtextSourceGapRepairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if len(req.CitationResolutions) == 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "citation_resolutions are required"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	if err := h.canonicalizeAliasedVTextDocumentTitle(r.Context(), ownerID, &doc, time.Now().UTC()); err != nil {
		log.Printf("vtext api: canonicalize source repair document title: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to canonicalize document title"})
		return
	}
	baseRevisionID := strings.TrimSpace(req.BaseRevisionID)
	if baseRevisionID == "" {
		baseRevisionID = doc.CurrentRevisionID
	}
	if baseRevisionID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "base revision is required"})
		return
	}
	baseRev, err := h.rt.Store().GetRevision(r.Context(), baseRevisionID, ownerID)
	if err != nil || baseRev.DocID != docID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "base revision not found"})
		return
	}

	metadata := decodeRevisionMetadata(baseRev.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	existingEntities := decodeVTextSourceEntities(metadata["source_entities"])
	resolutions := markdownLineageCitationResolutions(nil, req.CitationResolutions)
	if len(resolutions) == 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "citation_resolutions are required"})
		return
	}
	requestEntities := normalizeVTextSourceRepairEvidence(req.SourceEntities, resolutions)
	sourceEntities, _ := mergeVTextSourceEntities(existingEntities, requestEntities)
	if err := validateMarkdownLineageCitationResolutions(sourceEntities, resolutions); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	repaired := markdownLineageResolutionMap(resolutions)
	repairedContent := applyVTextCitationResolutions(baseRev.Content, resolutions)
	if repairedContent == baseRev.Content {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "citation_resolutions did not match unresolved markers in the base revision"})
		return
	}
	remainingGaps := filterVTextSourceGaps(metadata["source_gaps"], repaired)

	nextMetadata := map[string]any{}
	for key, value := range metadata {
		nextMetadata[key] = value
	}
	nextMetadata["source"] = "texture_source_gap_repair"
	nextMetadata["base_revision_id"] = baseRev.RevisionID
	nextMetadata["draft_line"] = defaultDraftLine()
	nextMetadata["source_repair_resolution_count"] = len(resolutions)
	nextMetadata["source_repair_resolutions"] = markdownLineageSourceRepairResolutionManifest(resolutions)
	if len(sourceEntities) > 0 {
		nextMetadata["source_entities"] = sourceEntities
	}
	if len(remainingGaps) > 0 {
		nextMetadata["source_gaps"] = remainingGaps
	} else {
		delete(nextMetadata, "source_gaps")
	}
	encoded, _ := json.Marshal(nextMetadata)
	authorLabel := strings.TrimSpace(req.AuthorLabel)
	if authorLabel == "" {
		authorLabel = ownerID
	}
	rev := types.Revision{
		RevisionID:       uuid.New().String(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      authorLabel,
		Content:          repairedContent,
		Citations:        baseRev.Citations,
		Metadata:         encoded,
		ParentRevisionID: baseRev.RevisionID,
		CreatedAt:        time.Now().UTC(),
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("vtext api: repair source gaps: %v", err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "failed to repair source gaps; document head may have changed"})
		return
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		log.Printf("vtext api: load source gap repair revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load source gap repair revision"})
		return
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(storedRev))
}

func (h *APIHandler) HandleVTextSourceArtifactAttachment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}
	var req vtextSourceArtifactAttachmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if len(req.Attachments) == 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "attachments are required"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	if err := h.canonicalizeAliasedVTextDocumentTitle(r.Context(), ownerID, &doc, time.Now().UTC()); err != nil {
		log.Printf("vtext api: canonicalize source attachment document title: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to canonicalize document title"})
		return
	}
	baseRevisionID := strings.TrimSpace(req.BaseRevisionID)
	if baseRevisionID == "" {
		baseRevisionID = doc.CurrentRevisionID
	}
	if baseRevisionID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "base revision is required"})
		return
	}
	baseRev, err := h.rt.Store().GetRevision(r.Context(), baseRevisionID, ownerID)
	if err != nil || baseRev.DocID != docID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "base revision not found"})
		return
	}

	metadata := decodeRevisionMetadata(baseRev.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	sourceEntities := decodeVTextSourceEntities(metadata["source_entities"])
	if len(sourceEntities) == 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "base revision has no source_entities"})
		return
	}
	updatedEntities, manifest, changed, err := h.applyVTextSourceArtifactAttachments(r.Context(), ownerID, sourceEntities, req.Attachments)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	if !changed {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source attachments did not change source metadata"})
		return
	}

	nextMetadata := map[string]any{}
	for key, value := range metadata {
		nextMetadata[key] = value
	}
	nextMetadata["source"] = "texture_source_artifact_attachment"
	nextMetadata["base_revision_id"] = baseRev.RevisionID
	nextMetadata["draft_line"] = defaultDraftLine()
	nextMetadata["source_attachment_count"] = len(manifest)
	nextMetadata["source_attachment_manifest"] = manifest
	nextMetadata["source_entities"] = updatedEntities
	encoded, _ := json.Marshal(nextMetadata)
	authorLabel := strings.TrimSpace(req.AuthorLabel)
	if authorLabel == "" {
		authorLabel = ownerID
	}
	rev := types.Revision{
		RevisionID:       uuid.New().String(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      authorLabel,
		Content:          baseRev.Content,
		Citations:        baseRev.Citations,
		Metadata:         encoded,
		ParentRevisionID: baseRev.RevisionID,
		CreatedAt:        time.Now().UTC(),
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("vtext api: attach source artifacts: %v", err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "failed to attach source artifacts; document head may have changed"})
		return
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		log.Printf("vtext api: load source attachment revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load source attachment revision"})
		return
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(storedRev))
}

func (h *APIHandler) applyVTextSourceArtifactAttachments(ctx context.Context, ownerID string, sourceEntities []vtextSourceEntity, attachments []vtextSourceArtifactAttachment) ([]vtextSourceEntity, []map[string]any, bool, error) {
	byID := make(map[string]int, len(sourceEntities))
	for i, entity := range sourceEntities {
		if id := strings.TrimSpace(entity.EntityID); id != "" {
			byID[id] = i
		}
	}
	updated := append([]vtextSourceEntity{}, sourceEntities...)
	manifest := make([]map[string]any, 0, len(attachments))
	changed := false
	for _, attachment := range attachments {
		entityID := strings.TrimSpace(attachment.EntityID)
		contentID := strings.TrimSpace(attachment.ContentID)
		if entityID == "" || contentID == "" {
			return nil, nil, false, fmt.Errorf("source artifact attachments require entity_id and content_id")
		}
		idx, ok := byID[entityID]
		if !ok {
			return nil, nil, false, fmt.Errorf("source entity %s not found", entityID)
		}
		item, err := h.rt.Store().GetContentItem(ctx, ownerID, contentID)
		if err != nil {
			if err == store.ErrNotFound {
				return nil, nil, false, fmt.Errorf("content item %s not found", contentID)
			}
			return nil, nil, false, fmt.Errorf("load content item %s: %w", contentID, err)
		}
		if item.OwnerID != ownerID {
			return nil, nil, false, fmt.Errorf("content item %s does not belong to owner", contentID)
		}
		if strings.TrimSpace(item.TextContent) == "" {
			return nil, nil, false, fmt.Errorf("content item %s has no readable text_content", contentID)
		}
		entity := updated[idx]
		before := sourceEntityJSONKey(entity)
		if entity.Target.TargetKind == "" || entity.Target.TargetKind == "url" {
			entity.Target.TargetKind = "content_item"
		}
		entity.Target.ContentID = item.ContentID
		if entity.Target.URL == "" {
			entity.Target.URL = item.SourceURL
		}
		if entity.Target.CanonicalURL == "" {
			entity.Target.CanonicalURL = firstNonEmpty(item.CanonicalURL, item.SourceURL)
		}
		if entity.Label == "" {
			entity.Label = firstNonEmpty(item.Title, entity.Target.CanonicalURL, item.SourceURL, "Source "+item.ContentID)
		}
		if entity.Display.OpenSurface == "" || sourcecontract.IsSourceReaderOpenSurface(entity.Display.OpenSurface) {
			entity.Display.OpenSurface = sourcecontract.OpenSurfaceSource
		}
		if len(entity.Selectors) == 0 {
			entity.Selectors = []vtextSourceEntitySelector{{SelectorKind: "whole_resource"}}
		}
		if quote := strings.TrimSpace(attachment.TextQuote); quote != "" {
			entity.Selectors[0].SelectorKind = "text_quote"
			entity.Selectors[0].TextQuote = quote
		}
		if item.ContentHash != "" && entity.Selectors[0].ContentHash == "" {
			entity.Selectors[0].ContentHash = item.ContentHash
		}
		entity.Evidence.State = "available"
		if entity.Evidence.ResearchState == "" || entity.Evidence.ResearchState == "pending" || entity.Evidence.ResearchState == "gap" {
			entity.Evidence.ResearchState = "represented"
		}
		if entity.Provenance.CreatedBy == "" {
			entity.Provenance.CreatedBy = "source_artifact_attachment"
		}
		if entity.Provenance.RightsScope == "" {
			entity.Provenance.RightsScope = "private_user_source"
		}
		entity.Provenance.UntrustedSourceText = true
		if sourceEntityJSONKey(entity) != before {
			changed = true
			updated[idx] = entity
		}
		manifest = append(manifest, map[string]any{
			"entity_id":     entityID,
			"content_id":    item.ContentID,
			"content_hash":  item.ContentHash,
			"source_url":    item.SourceURL,
			"canonical_url": item.CanonicalURL,
			"media_type":    item.MediaType,
		})
	}
	return updated, manifest, changed, nil
}
