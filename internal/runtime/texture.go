// Package runtime provides texture document API handlers for the go-choir
// sandbox runtime. These handlers expose the document CRUD, revision,
// history, snapshot, diff, blame, and agent revision APIs through the
// authenticated same-origin proxy path.
//
// API endpoints:
//
//	POST   /api/texture/documents          — create a new document
//	GET    /api/texture/documents          — list documents for the authenticated user
//	GET    /api/texture/documents/{id}     — get a document by ID
//	PUT    /api/texture/documents/{id}     — update a document (e.g., title)
//	DELETE /api/texture/documents/{id}     — delete a document and its revisions
//	POST   /api/texture/documents/{id}/revisions — create a user-authored revision
//	GET    /api/texture/documents/{id}/revisions — list revisions for a document
//	GET    /api/texture/documents/{id}/stream — stream document lifecycle changes
//	GET    /api/texture/revisions/{id}    — get a specific revision (snapshot)
//	GET    /api/texture/documents/{id}/history — get revision history with attribution
//	GET    /api/texture/diff?from={id}&to={id} — diff two revisions
//	GET    /api/texture/revisions/{id}/blame — blame a revision
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/markdownstructure"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	textureMarkerLineRE          = regexp.MustCompile(`(?im)^.*USER_[A-Z0-9_]*MARKER[A-Z0-9_]*.*$`)
	textureNumberedHeadingRE     = regexp.MustCompile(`(?m)^\s*(?:#{1,6}\s*)?(\d{1,2}\.\s+[^\n:]{2,100})\s*$`)
	textureSectionUpdatePrefixRE = regexp.MustCompile(`\bSECTION\s+\d+\s+UPDATE:`)
	textureSHA256RequirementRE   = regexp.MustCompile(`\b[a-fA-F0-9]{64}\b`)
	textureInlineSourceRefRE     = regexp.MustCompile(`\[[^\]\n]{1,160}\]\(source:[^) \t\r\n]{1,160}\)`)
)

// ----- Request/Response types -----

// textureCreateDocRequest is the JSON payload for POST /api/texture/documents.
type textureCreateDocRequest struct {
	Title string `json:"title"`
}

// textureCreateDocResponse is the JSON response for POST /api/texture/documents.
type textureCreateDocResponse struct {
	DocID     string `json:"doc_id"`
	OwnerID   string `json:"owner_id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

type textureOpenFileRequest struct {
	SourcePath     string `json:"source_path"`
	Title          string `json:"title"`
	InitialContent string `json:"initial_content"`
}

type textureOpenFileResponse struct {
	DocID             string `json:"doc_id"`
	CurrentRevisionID string `json:"current_revision_id,omitempty"`
	Created           bool   `json:"created"`
	OriginalContentID string `json:"original_content_id,omitempty"`
}

type textureMarkdownLineageImportRequest struct {
	SourcePath          string                            `json:"source_path"`
	Title               string                            `json:"title"`
	SourceEntities      []textureSourceEntity             `json:"source_entities,omitempty"`
	CitationResolutions []textureCitationMarkerResolution `json:"citation_resolutions,omitempty"`
	Versions            []textureMarkdownLineageVersion   `json:"versions"`
}

type textureMarkdownLineageVersion struct {
	Label               string                            `json:"label,omitempty"`
	SourceRevisionID    string                            `json:"source_revision_id,omitempty"`
	ContentItemID       string                            `json:"content_item_id,omitempty"`
	Content             string                            `json:"content"`
	CreatedAt           string                            `json:"created_at,omitempty"`
	Metadata            json.RawMessage                   `json:"metadata,omitempty"`
	SourceEntities      []textureSourceEntity             `json:"source_entities,omitempty"`
	CitationResolutions []textureCitationMarkerResolution `json:"citation_resolutions,omitempty"`
}

type textureCitationMarkerResolution struct {
	Marker        string `json:"marker"`
	EntityID      string `json:"entity_id"`
	Action        string `json:"action,omitempty"`
	Reason        string `json:"reason,omitempty"`
	EvidenceState string `json:"evidence_state,omitempty"`
}

type resolvedMarkdownLineageVersion struct {
	Version       textureMarkdownLineageVersion
	Content       string
	ContentItem   *types.ContentItem
	ContentID     string
	ContentHash   string
	ContentPath   string
	ContentSource string
}

type textureMarkdownLineageImportResponse struct {
	DocID              string                    `json:"doc_id"`
	CurrentRevisionID  string                    `json:"current_revision_id"`
	SourcePath         string                    `json:"source_path"`
	Created            bool                      `json:"created"`
	RevisionCount      int                       `json:"revision_count"`
	Revisions          []textureRevisionResponse `json:"revisions"`
	OriginalContentIDs []string                  `json:"original_content_ids"`
	ExistingDocID      string                    `json:"existing_doc_id,omitempty"`
}

type textureSourceGapRepairRequest struct {
	BaseRevisionID      string                            `json:"base_revision_id,omitempty"`
	SourceEntities      []textureSourceEntity             `json:"source_entities,omitempty"`
	CitationResolutions []textureCitationMarkerResolution `json:"citation_resolutions,omitempty"`
	AuthorLabel         string                            `json:"author_label,omitempty"`
}

type textureSourceArtifactAttachmentRequest struct {
	BaseRevisionID string                            `json:"base_revision_id,omitempty"`
	Attachments    []textureSourceArtifactAttachment `json:"attachments,omitempty"`
	AuthorLabel    string                            `json:"author_label,omitempty"`
}

type textureSourceArtifactAttachment struct {
	EntityID  string `json:"entity_id"`
	ContentID string `json:"content_id"`
	TextQuote string `json:"text_quote,omitempty"`
}

type textureEnsureManifestResponse struct {
	DocID      string `json:"doc_id"`
	SourcePath string `json:"source_path"`
}

type textureDocumentExportResponse struct {
	DocID       string `json:"doc_id"`
	RevisionID  string `json:"revision_id"`
	Format      string `json:"format"`
	MediaType   string `json:"media_type"`
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentHash string `json:"content_hash"`
}

// textureDocumentResponse is the JSON response for GET /api/texture/documents/{id}.
type textureDocumentResponse struct {
	DocID                string `json:"doc_id"`
	OwnerID              string `json:"owner_id"`
	Title                string `json:"title"`
	CurrentRevisionID    string `json:"current_revision_id,omitempty"`
	CurrentVersionNumber int    `json:"current_version_number"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
	RevisionCount        int    `json:"revision_count"`
	LastEditor           string `json:"last_editor,omitempty"`
	LastAuthorKind       string `json:"last_author_kind,omitempty"`
	AgentRevisionPending bool   `json:"agent_revision_pending,omitempty"`
	AgentRevisionRunID   string `json:"agent_revision_run_id,omitempty"`
}

// textureDocumentStreamEvent is the hidden transport envelope sent over the
// document-scoped SSE stream. The editor consumes document lifecycle changes
// from this stream but does not render raw agent chatter.
type textureDocumentStreamEvent struct {
	Kind              string `json:"kind"`
	DocID             string `json:"doc_id"`
	LoopID            string `json:"loop_id,omitempty"`
	RevisionID        string `json:"revision_id,omitempty"`
	CurrentRevisionID string `json:"current_revision_id,omitempty"`
	Pending           bool   `json:"pending,omitempty"`
	Error             string `json:"error,omitempty"`
}

// textureUpdateDocRequest is the JSON payload for PUT /api/texture/documents/{id}.
type textureUpdateDocRequest struct {
	Title string `json:"title"`
}

// textureListDocsResponse is the JSON response for GET /api/texture/documents.
type textureListDocsResponse struct {
	Documents []textureDocumentResponse `json:"documents"`
}

// textureCreateRevisionRequest is the public JSON payload for
// POST /api/texture/documents/{id}/revisions. The public route always creates
// user-authored revisions; author_kind/author_label are accepted only for
// older clients and are not authority-bearing.
type textureCreateRevisionRequest struct {
	Content          string           `json:"content"`
	BodyDoc          json.RawMessage  `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage  `json:"source_entities,omitempty"`
	AuthorKind       types.AuthorKind `json:"author_kind"`
	AuthorLabel      string           `json:"author_label"`
	Citations        json.RawMessage  `json:"citations,omitempty"`
	Metadata         json.RawMessage  `json:"metadata,omitempty"`
	ParentRevisionID string           `json:"parent_revision_id,omitempty"`
	AllowRebase      bool             `json:"allow_rebase,omitempty"`
}

// textureRevisionResponse is the JSON response for revision-related endpoints.
type textureRevisionResponse struct {
	RevisionID       string           `json:"revision_id"`
	DocID            string           `json:"doc_id"`
	OwnerID          string           `json:"owner_id"`
	AuthorKind       types.AuthorKind `json:"author_kind"`
	AuthorLabel      string           `json:"author_label"`
	VersionNumber    int              `json:"version_number"`
	Content          string           `json:"content"`
	BodyDoc          json.RawMessage  `json:"body_doc,omitempty"`
	SourceEntities   json.RawMessage  `json:"source_entities,omitempty"`
	Citations        json.RawMessage  `json:"citations,omitempty"`
	Metadata         json.RawMessage  `json:"metadata,omitempty"`
	Provenance       json.RawMessage  `json:"provenance,omitempty"`
	RevisionHash     string           `json:"revision_hash,omitempty"`
	ParentRevisionID string           `json:"parent_revision_id,omitempty"`
	CreatedAt        string           `json:"created_at"`
}

// textureListRevisionsResponse is the JSON response for
// GET /api/texture/documents/{id}/revisions.
type textureListRevisionsResponse struct {
	Revisions []textureRevisionResponse `json:"revisions"`
}

// textureHistoryResponse is the JSON response for
// GET /api/texture/documents/{id}/history.
type textureHistoryResponse struct {
	DocID   string               `json:"doc_id"`
	Entries []types.HistoryEntry `json:"entries"`
}

// textureDiffResponse is the JSON response for GET /api/texture/diff.
type textureDiffResponse struct {
	types.DiffResult
}

type textureSemanticCompareResponse struct {
	CompareID        string                   `json:"compare_id"`
	SourceRevisionID string                   `json:"source_revision_id"`
	TargetRevisionID string                   `json:"target_revision_id"`
	DraftLine        textureDraftLineSummary  `json:"draft_line"`
	Summary          []string                 `json:"summary"`
	Suggestions      []textureMergeSuggestion `json:"suggestions"`
	Diff             types.DiffResult         `json:"diff"`
	ModelEvidence    map[string]any           `json:"model_evidence,omitempty"`
	EvidenceID       string                   `json:"evidence_id,omitempty"`
}

type textureDraftLineSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type textureMergeSuggestion struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Source      string `json:"source"`
	Preview     string `json:"preview,omitempty"`
}

type textureMergePreviewRequest struct {
	SourceRevisionID   string   `json:"source_revision_id"`
	TargetRevisionID   string   `json:"target_revision_id"`
	SuggestionIDs      []string `json:"suggestion_ids"`
	SourceVersionLabel string   `json:"source_version_label,omitempty"`
	TargetVersionLabel string   `json:"target_version_label,omitempty"`
}

type textureMergePreviewResponse struct {
	PreviewID        string                   `json:"preview_id"`
	DocID            string                   `json:"doc_id"`
	SourceRevisionID string                   `json:"source_revision_id"`
	TargetRevisionID string                   `json:"target_revision_id"`
	DraftLine        textureDraftLineSummary  `json:"draft_line"`
	Content          string                   `json:"content"`
	Provenance       map[string]any           `json:"provenance"`
	Suggestions      []textureMergeSuggestion `json:"suggestions"`
	ModelEvidence    map[string]any           `json:"model_evidence,omitempty"`
	EvidenceID       string                   `json:"evidence_id,omitempty"`
}

type textureModelMergeEdit struct {
	SuggestionID string `json:"suggestion_id,omitempty"`
	Operation    string `json:"operation"`
	OldText      string `json:"old_text,omitempty"`
	NewText      string `json:"new_text,omitempty"`
	Rationale    string `json:"rationale,omitempty"`
}

type textureModelSemanticMergeResult struct {
	Summary     []string                 `json:"summary"`
	Suggestions []textureMergeSuggestion `json:"suggestions"`
	Edits       []textureModelMergeEdit  `json:"edits,omitempty"`
}

type textureAcceptMergeRequest struct {
	PreviewID        string         `json:"preview_id"`
	Content          string         `json:"content"`
	SourceRevisionID string         `json:"source_revision_id"`
	TargetRevisionID string         `json:"target_revision_id"`
	SuggestionIDs    []string       `json:"suggestion_ids"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type textureRestoreRevisionRequest struct {
	RevisionID string `json:"revision_id"`
	Mode       string `json:"mode,omitempty"`
}

// ----- Helper functions -----

// extractDocID extracts the document ID from the URL path.
// Expected pattern: /api/texture/documents/{docID}/...
func extractDocID(path string) string {
	if !strings.HasPrefix(path, textureDocumentsPathPrefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, textureDocumentsPathPrefix)
	// The docID is the first path segment.
	parts := strings.SplitN(rest, "/", 2)
	return parts[0]
}

// extractRevisionID extracts the revision ID from the URL path.
// Expected pattern: /api/texture/revisions/{revisionID}/...
func extractRevisionID(path string) string {
	if !strings.HasPrefix(path, textureRevisionsPathPrefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, textureRevisionsPathPrefix)
	parts := strings.SplitN(rest, "/", 2)
	return parts[0]
}

func (h *APIHandler) canonicalizeAliasedTextureDocumentTitle(ctx context.Context, ownerID string, doc *types.Document, updatedAt time.Time) error {
	return canonicalizeAliasedTextureDocumentTitle(ctx, h.rt.Store(), ownerID, doc, updatedAt)
}

func (rt *Runtime) canonicalizeAliasedTextureDocumentTitle(ctx context.Context, ownerID string, doc *types.Document, updatedAt time.Time) error {
	if rt == nil || rt.store == nil {
		return nil
	}
	return canonicalizeAliasedTextureDocumentTitle(ctx, rt.store, ownerID, doc, updatedAt)
}

func canonicalizeAliasedTextureDocumentTitle(ctx context.Context, st *store.Store, ownerID string, doc *types.Document, updatedAt time.Time) error {
	if doc == nil || isTextureShortcutPath(doc.Title) {
		return nil
	}
	if st == nil {
		return nil
	}
	sourcePath, err := st.GetDocumentAliasSourcePath(ctx, ownerID, doc.DocID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return err
	}
	nextTitle := canonicalTextureImportTitle(sourcePath, doc.Title)
	if strings.TrimSpace(nextTitle) == "" || nextTitle == doc.Title {
		return nil
	}
	doc.Title = nextTitle
	doc.UpdatedAt = updatedAt
	return st.UpdateDocument(ctx, *doc)
}

func writeSSEData(w http.ResponseWriter, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("texture api: marshal sse payload: %v", err)
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// ----- Handler methods -----

// HandleTextureCreateDocument handles POST /api/texture/documents.
// It creates a new document with a durable document identity (VAL-ETEXT-001).
func (h *APIHandler) HandleTextureCreateDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req textureCreateDocRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "title is required"})
		return
	}

	now := time.Now().UTC()
	doc := types.Document{
		DocID:     uuid.New().String(),
		OwnerID:   ownerID,
		Title:     req.Title,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.rt.Store().CreateDocument(r.Context(), doc); err != nil {
		log.Printf("texture api: create document: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create document"})
		return
	}

	writeAPIJSON(w, http.StatusCreated, textureCreateDocResponse{
		DocID:     doc.DocID,
		OwnerID:   doc.OwnerID,
		Title:     doc.Title,
		CreatedAt: doc.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleTextureImportMarkdownLineage migrates ordered Markdown snapshots into one
// canonical Texture document with durable version-numbered revisions.
func (h *APIHandler) HandleTextureImportMarkdownLineage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req textureMarkdownLineageImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	sourcePath := normalizeTextureSourcePath(req.SourcePath)
	if sourcePath == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source_path is required"})
		return
	}
	if len(req.Versions) == 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "versions are required"})
		return
	}
	if len(req.Versions) > 10000 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "too many versions"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		parts := strings.Split(sourcePath, "/")
		title = parts[len(parts)-1]
	}
	canonicalTitle := canonicalTextureImportTitle(sourcePath, title)
	if existingDocID, err := h.rt.Store().GetDocumentAlias(r.Context(), ownerID, sourcePath); err == nil {
		writeAPIJSON(w, http.StatusConflict, textureMarkdownLineageImportResponse{
			SourcePath:    sourcePath,
			Created:       false,
			ExistingDocID: existingDocID,
		})
		return
	} else if err != store.ErrNotFound {
		log.Printf("texture api: lookup markdown lineage alias %s: %v", sourcePath, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve document alias"})
		return
	}
	resolvedVersions := make([]resolvedMarkdownLineageVersion, 0, len(req.Versions))
	for i, version := range req.Versions {
		resolved, err := h.resolveMarkdownLineageVersion(r.Context(), ownerID, version)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("versions[%d]: %v", i, err)})
			return
		}
		if strings.TrimSpace(resolved.Content) == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("versions[%d].content or content_item_id is required", i)})
			return
		}
		if len(bytes.TrimSpace(version.Metadata)) > 0 {
			var obj map[string]any
			if err := json.Unmarshal(version.Metadata, &obj); err != nil || obj == nil {
				writeAPIJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("versions[%d].metadata must be a JSON object", i)})
				return
			}
		}
		sourceEntities := markdownLineageSourceEntities(req.SourceEntities, version.SourceEntities)
		resolutions := markdownLineageCitationResolutions(req.CitationResolutions, version.CitationResolutions)
		if err := validateMarkdownLineageCitationResolutions(sourceEntities, resolutions); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("versions[%d]: %v", i, err)})
			return
		}
		resolvedVersions = append(resolvedVersions, resolved)
	}

	now := time.Now().UTC()
	doc := types.Document{
		DocID:     uuid.New().String(),
		OwnerID:   ownerID,
		Title:     canonicalTitle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.rt.Store().CreateDocument(r.Context(), doc); err != nil {
		log.Printf("texture api: create markdown lineage document: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create document"})
		return
	}

	for i := range resolvedVersions {
		if resolvedVersions[i].ContentItem != nil {
			continue
		}
		version := resolvedVersions[i].Version
		versionNow := now.Add(time.Duration(i) * time.Millisecond)
		if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(version.CreatedAt)); err == nil {
			versionNow = parsed.UTC()
		}
		item := buildMarkdownLineageContentItem(ownerID, sourcePath, title, version, resolvedVersions[i].Content, versionNow)
		if err := h.rt.Store().CreateContentItem(r.Context(), item); err != nil {
			log.Printf("texture api: preserve markdown lineage snapshot %s: %v", sourcePath, err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to preserve markdown source snapshot"})
			return
		}
		resolvedVersions[i].ContentItem = &item
		resolvedVersions[i].ContentID = item.ContentID
		resolvedVersions[i].ContentHash = item.ContentHash
		resolvedVersions[i].ContentPath = item.FilePath
		resolvedVersions[i].ContentSource = "created_snapshot"
	}

	lineage := buildMarkdownLineageSummary(resolvedVersions)
	revisionResponses := make([]textureRevisionResponse, 0, len(req.Versions))
	originalIDs := make([]string, 0, len(req.Versions))
	parentID := ""
	for i, resolved := range resolvedVersions {
		version := resolved.Version
		versionNow := now.Add(time.Duration(i) * time.Millisecond)
		if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(version.CreatedAt)); err == nil {
			versionNow = parsed.UTC()
		}
		sourceEntities := markdownLineageSourceEntities(req.SourceEntities, version.SourceEntities)
		resolutions := markdownLineageCitationResolutions(req.CitationResolutions, version.CitationResolutions)
		metadata, err := buildMarkdownLineageRevisionMetadata(sourcePath, version, resolved.Content, resolved.ContentID, resolved.ContentHash, resolved.ContentPath, resolved.ContentSource, i, len(req.Versions), lineage, sourceEntities, resolutions)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("versions[%d].metadata must be a JSON object", i)})
			return
		}
		revisionID := uuid.New().String()
		bodyDoc, structuredSourceEntities, projectionContent, err := markdownLineageStructuredRevision(doc.DocID, revisionID, resolved.Content, sourceEntities, resolutions)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("versions[%d]: %v", i, err)})
			return
		}
		rev := types.Revision{
			RevisionID:       revisionID,
			DocID:            doc.DocID,
			OwnerID:          ownerID,
			AuthorKind:       types.AuthorUser,
			AuthorLabel:      ownerID,
			Content:          projectionContent,
			BodyDoc:          bodyDoc,
			SourceEntities:   structuredSourceEntities,
			Metadata:         metadata,
			ParentRevisionID: parentID,
			CreatedAt:        versionNow,
		}
		if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
			log.Printf("texture api: create markdown lineage revision %s[%d]: %v", sourcePath, i, err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create markdown lineage revision"})
			return
		}
		storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
		if err != nil {
			log.Printf("texture api: reload markdown lineage revision %s: %v", rev.RevisionID, err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load created revision"})
			return
		}
		revisionResponses = append(revisionResponses, revisionResponseFromRecord(storedRev))
		originalIDs = append(originalIDs, resolved.ContentID)
		parentID = rev.RevisionID
		doc.CurrentRevisionID = rev.RevisionID
	}
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), ownerID, sourcePath, doc.DocID, now); err != nil {
		log.Printf("texture api: upsert markdown lineage alias %s -> %s: %v", sourcePath, doc.DocID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to persist document alias"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, textureMarkdownLineageImportResponse{
		DocID:              doc.DocID,
		CurrentRevisionID:  doc.CurrentRevisionID,
		SourcePath:         sourcePath,
		Created:            true,
		RevisionCount:      len(revisionResponses),
		Revisions:          revisionResponses,
		OriginalContentIDs: originalIDs,
	})
}

// HandleTextureOpenFile resolves a file-browser path to one canonical texture
// document. The first open creates the document + alias; later opens reuse it.
func (h *APIHandler) HandleTextureOpenFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req textureOpenFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	sourcePath := normalizeTextureSourcePath(req.SourcePath)
	if sourcePath == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source_path is required"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		parts := strings.Split(sourcePath, "/")
		title = parts[len(parts)-1]
	}
	canonicalTitle := canonicalTextureImportTitle(sourcePath, title)

	docID, err := h.rt.Store().GetDocumentAlias(r.Context(), ownerID, sourcePath)
	if err == nil {
		doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
		if err != nil {
			log.Printf("texture api: resolve aliased document %s: %v", docID, err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to open aliased document"})
			return
		}
		writeAPIJSON(w, http.StatusOK, textureOpenFileResponse{
			DocID:             doc.DocID,
			CurrentRevisionID: doc.CurrentRevisionID,
			Created:           false,
		})
		return
	}
	if err != store.ErrNotFound {
		log.Printf("texture api: lookup file alias %s: %v", sourcePath, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve file alias"})
		return
	}

	now := time.Now().UTC()
	projection := buildTextureFileImportProjection(sourcePath, req.InitialContent)
	var original *types.ContentItem
	if !isTextureShortcutPath(sourcePath) {
		item, err := h.ensureTextureOriginalContentItem(r.Context(), ownerID, title, projection, now)
		if err != nil {
			log.Printf("texture api: preserve original content item for %s: %v", sourcePath, err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to preserve original file artifact"})
			return
		}
		original = &item
	}
	doc := types.Document{
		DocID:     uuid.New().String(),
		OwnerID:   ownerID,
		Title:     canonicalTitle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := h.rt.Store().CreateDocument(r.Context(), doc); err != nil {
		log.Printf("texture api: create aliased document: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create aliased document"})
		return
	}
	rev := types.Revision{
		RevisionID:  uuid.New().String(),
		DocID:       doc.DocID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: ownerID,
		Content:     projection.ProjectionContent,
		Metadata:    buildFileOpenTextureMetadata(projection, original),
		CreatedAt:   now,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("texture api: create aliased initial revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create aliased initial revision"})
		return
	}
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), ownerID, sourcePath, doc.DocID, now); err != nil {
		log.Printf("texture api: upsert file alias %s -> %s: %v", sourcePath, doc.DocID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to persist file alias"})
		return
	}
	doc.CurrentRevisionID = rev.RevisionID
	writeAPIJSON(w, http.StatusCreated, textureOpenFileResponse{
		DocID:             doc.DocID,
		CurrentRevisionID: rev.RevisionID,
		Created:           true,
		OriginalContentID: contentIDOrEmpty(original),
	})
}

func contentIDOrEmpty(item *types.ContentItem) string {
	if item == nil {
		return ""
	}
	return item.ContentID
}

// HandleTextureEnsureManifest ensures a canonical texture document has a
// filesystem shortcut so it appears in Files while the real document state
// stays canonical in Dolt.
func (h *APIHandler) HandleTextureEnsureManifest(w http.ResponseWriter, r *http.Request) {
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
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document id is required"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("texture api: get document for manifest %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load document"})
		return
	}
	sourcePath, err := h.ensureTextureManifest(r.Context(), ownerID, doc)
	if err != nil {
		log.Printf("texture api: ensure manifest for %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to persist file manifest"})
		return
	}
	writeAPIJSON(w, http.StatusOK, textureEnsureManifestResponse{
		DocID:      doc.DocID,
		SourcePath: sourcePath,
	})
}

func (h *APIHandler) HandleTextureExportDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "md"
	}
	if format == "markdown" {
		format = "md"
	}
	if format != "md" && format != "txt" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "unsupported export format"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	revisionID := strings.TrimSpace(r.URL.Query().Get("revision_id"))
	if revisionID == "" {
		revisionID = doc.CurrentRevisionID
	}
	if revisionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document has no current revision"})
		return
	}
	rev, err := h.rt.Store().GetRevision(r.Context(), revisionID, ownerID)
	if err != nil || rev.DocID != doc.DocID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "revision not found"})
		return
	}
	mediaType := "text/markdown; charset=utf-8"
	if format == "txt" {
		mediaType = "text/plain; charset=utf-8"
	}
	content := rev.Content
	writeAPIJSON(w, http.StatusOK, textureDocumentExportResponse{
		DocID:       doc.DocID,
		RevisionID:  rev.RevisionID,
		Format:      format,
		MediaType:   mediaType,
		Filename:    textureDocumentExportFilename(doc.Title, format),
		Content:     content,
		ContentHash: contentHash(content),
	})
}

// HandleTextureListDocuments handles GET /api/texture/documents.
// It returns documents owned by the authenticated user.
func (h *APIHandler) HandleTextureListDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	docs, err := h.rt.Store().ListDocumentsByOwner(r.Context(), ownerID, 50)
	if err != nil {
		log.Printf("texture api: list documents: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list documents"})
		return
	}

	resp := textureListDocsResponse{Documents: make([]textureDocumentResponse, 0, len(docs))}
	for _, doc := range docs {
		docResp := h.textureDocumentResponse(r.Context(), doc)
		resp.Documents = append(resp.Documents, docResp)
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

// HandleTextureDocument handles GET/PUT/DELETE /api/texture/documents/{id}.

func internalTextureDocumentIDFromPath(path string) string {
	const prefix = "/internal/texture/documents/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if id == "" || strings.Contains(id, "/") {
		return ""
	}
	return id
}

func internalTextureRevisionIDFromPath(path string) string {
	const prefix = "/internal/texture/revisions/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	id := strings.TrimSpace(strings.TrimPrefix(path, prefix))
	if id == "" || strings.Contains(id, "/") {
		return ""
	}
	return id
}

func (h *APIHandler) HandleInternalTextureDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if ownerID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "authenticated user is required"})
		return
	}
	docID := internalTextureDocumentIDFromPath(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	writeAPIJSON(w, http.StatusOK, sandboxTextureDocumentResponseFromRecord(doc))
}

func (h *APIHandler) HandleInternalTextureRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	ownerID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if ownerID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "authenticated user is required"})
		return
	}
	revisionID := internalTextureRevisionIDFromPath(r.URL.Path)
	if revisionID == "" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "revision not found"})
		return
	}
	rev, err := h.rt.Store().GetRevision(r.Context(), revisionID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "revision not found"})
		return
	}
	writeAPIJSON(w, http.StatusOK, sandboxTextureRevisionResponseFromRecord(rev))
}

func sandboxTextureDocumentResponseFromRecord(doc types.Document) map[string]any {
	return map[string]any{
		"doc_id":              doc.DocID,
		"owner_id":            doc.OwnerID,
		"title":               doc.Title,
		"current_revision_id": doc.CurrentRevisionID,
	}
}

func sandboxTextureRevisionResponseFromRecord(rev types.Revision) map[string]any {
	return map[string]any{
		"revision_id":     rev.RevisionID,
		"doc_id":          rev.DocID,
		"owner_id":        rev.OwnerID,
		"content":         rev.Content,
		"body_doc":        rev.BodyDoc,
		"source_entities": rev.SourceEntities,
		"citations":       rev.Citations,
		"metadata":        rev.Metadata,
		"provenance":      rev.Provenance,
		"revision_hash":   rev.RevisionHash,
	}
}

func (h *APIHandler) HandleTextureDocument(w http.ResponseWriter, r *http.Request) {
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleTextureGetDocument(w, r, docID)
	case http.MethodPut:
		h.handleTextureUpdateDocument(w, r, docID)
	case http.MethodDelete:
		h.handleTextureDeleteDocument(w, r, docID)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) handleTextureGetDocument(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	readOwnerID, err := h.resolveUniversalWireTextureReadOwner(r.Context(), ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("texture api: resolve document owner %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load document"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, readOwnerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	pendingMutation, err := h.pendingAgentMutationByDoc(r.Context(), docID, readOwnerID)
	if err != nil {
		log.Printf("texture api: get pending mutation for document: %v", err)
	}

	resp := h.textureDocumentResponse(r.Context(), doc)
	resp.AgentRevisionPending = pendingMutation != nil
	if pendingMutation != nil {
		resp.AgentRevisionRunID = pendingMutation.RunID
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) handleTextureUpdateDocument(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req textureUpdateDocRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}

	doc.Title = req.Title
	doc.UpdatedAt = time.Now().UTC()

	if err := h.rt.Store().UpdateDocument(r.Context(), doc); err != nil {
		log.Printf("texture api: update document: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update document"})
		return
	}

	writeAPIJSON(w, http.StatusOK, textureDocumentResponse{
		DocID:             doc.DocID,
		OwnerID:           doc.OwnerID,
		Title:             doc.Title,
		CurrentRevisionID: doc.CurrentRevisionID,
		CreatedAt:         doc.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:         doc.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

func (h *APIHandler) handleTextureDeleteDocument(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	if err := h.cancelTextureActorForDeletedDocument(r.Context(), docID, ownerID); err != nil {
		log.Printf("texture api: cancel actor before deleting document %s: %v", docID, err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}

	if err := h.rt.Store().DeleteDocument(r.Context(), docID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}

	writeAPIJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *APIHandler) cancelTextureActorForDeletedDocument(ctx context.Context, docID, ownerID string) error {
	mutation, err := h.pendingAgentMutationByDoc(ctx, docID, ownerID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("load pending Texture actor: %w", err)
	}
	if mutation != nil {
		if _, err := h.rt.CancelRunTrajectory(ctx, mutation.RunID, ownerID); err != nil {
			return fmt.Errorf("cancel Texture actor trajectory: %w", err)
		}
		if err := h.rt.Store().CancelAgentMutation(ctx, mutation.RunID); err != nil {
			return fmt.Errorf("mark Texture actor mutation cancelled: %w", err)
		}
		return nil
	}
	if err := h.rt.CancelAgent(ctx, currentTextureAgentID(docID), ownerID); err != nil && !strings.Contains(err.Error(), "agent not found:") {
		return fmt.Errorf("cancel Texture actor: %w", err)
	}
	return nil
}

// HandleTextureRevisions handles POST and GET
// /api/texture/documents/{id}/revisions.
func (h *APIHandler) HandleTextureRevisions(w http.ResponseWriter, r *http.Request) {
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handleTextureCreateRevision(w, r, docID)
	case http.MethodGet:
		h.handleTextureListRevisions(w, r, docID)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) handleTextureCreateRevision(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req textureCreateRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	// Verify the document exists and belongs to this owner.
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}

	now := time.Now().UTC()
	if err := h.canonicalizeAliasedTextureDocumentTitle(r.Context(), ownerID, &doc, now); err != nil {
		log.Printf("texture api: canonicalize aliased document title: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to canonicalize document title"})
		return
	}

	// If parent_revision_id is not specified, use the document's current head.
	parentID := req.ParentRevisionID
	if parentID == "" {
		parentID = doc.CurrentRevisionID
	}

	citations := req.Citations
	if citations == nil {
		citations = json.RawMessage("[]")
	}
	metadata := req.Metadata
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}
	content := req.Content
	var parentRev types.Revision
	var hasParentRev bool
	if strings.TrimSpace(parentID) != "" {
		if rev, err := h.rt.Store().GetRevision(r.Context(), parentID, ownerID); err == nil {
			parentRev = rev
			hasParentRev = true
			var stabilized bool
			content, stabilized = stabilizeTextureUserMarkdownStructures(parentRev.Content, content)
			if stabilized {
				metadata = mergeTextureRevisionMetadata(metadata, map[string]any{
					"texture_structure_stabilized":        true,
					"texture_structure_stabilized_reason": "preserved_parent_markdown_table_after_collapsed_draft",
				})
			}
		} else {
			log.Printf("texture api: load parent revision for structure stabilization %s: %v", parentID, err)
		}
	}
	if hasParentRev {
		metadata = carryForwardDurableTextureMetadata(metadata, parentRev.Metadata)
	}
	if canonicalPath, err := h.ensureCanonicalTextureProjectionPath(r.Context(), ownerID, doc); err == nil && canonicalPath != "" {
		metadata = mergeTextureRevisionMetadata(metadata, map[string]any{
			canonicalTextureSourcePathMetadataKey: canonicalPath,
		})
	} else if err != nil {
		log.Printf("texture api: ensure canonical texture projection path: %v", err)
	}

	rev := types.Revision{
		RevisionID:       uuid.New().String(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      ownerID,
		Content:          content,
		BodyDoc:          req.BodyDoc,
		SourceEntities:   req.SourceEntities,
		Citations:        citations,
		Metadata:         metadata,
		ParentRevisionID: parentID,
		CreatedAt:        now,
	}

	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("texture api: create revision: %v", err)
		if errors.Is(err, store.ErrStaleDocumentHead) {
			if req.AllowRebase && parentID != "" && len(strings.TrimSpace(string(req.BodyDoc))) == 0 {
				req.Content = content
				rebased, rebaseErr := h.createRebasedUserRevision(r.Context(), docID, ownerID, req, parentID, citations, metadata, now)
				if rebaseErr == nil {
					h.rt.emitTextureDocumentRevisionEvent(r.Context(), ownerID, rebased)
					writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(rebased))
					return
				}
				log.Printf("texture api: rebase stale user revision: %v", rebaseErr)
			}
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "document head changed; reload the latest version before saving"})
			return
		}
		if errors.Is(err, store.ErrInvalidTextureRevision) {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create revision"})
		return
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		log.Printf("texture api: load created revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load created revision"})
		return
	}
	h.rt.emitTextureDocumentRevisionEvent(r.Context(), ownerID, storedRev)

	writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(storedRev))
}

func (h *APIHandler) createRebasedUserRevision(ctx context.Context, docID, ownerID string, req textureCreateRevisionRequest, staleParentID string, citations, metadata json.RawMessage, now time.Time) (types.Revision, error) {
	currentDoc, err := h.rt.Store().GetDocument(ctx, docID, ownerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("load current document for rebase: %w", err)
	}
	if strings.TrimSpace(currentDoc.CurrentRevisionID) == "" || strings.TrimSpace(currentDoc.CurrentRevisionID) == staleParentID {
		return types.Revision{}, fmt.Errorf("document head is not rebaseable")
	}
	baseRev, err := h.rt.Store().GetRevision(ctx, staleParentID, ownerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("load stale base revision: %w", err)
	}
	headRev, err := h.rt.Store().GetRevision(ctx, currentDoc.CurrentRevisionID, ownerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("load current head revision: %w", err)
	}

	mergedContent, strategy, clean := rebaseUserDraftContent(baseRev.Content, headRev.Content, req.Content, staleParentID)
	mergedMetadata := mergeTextureRevisionMetadata(metadata, map[string]any{
		"rebased_from_revision_id": staleParentID,
		"rebase_onto_revision_id":  headRev.RevisionID,
		"rebase_strategy":          strategy,
		"rebase_clean":             clean,
	})
	mergedMetadata = carryForwardDurableTextureMetadata(mergedMetadata, headRev.Metadata)

	rev := types.Revision{
		RevisionID:       uuid.New().String(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      ownerID,
		Content:          mergedContent,
		Citations:        citations,
		Metadata:         mergedMetadata,
		ParentRevisionID: headRev.RevisionID,
		CreatedAt:        now,
	}
	if err := h.rt.Store().CreateRevision(ctx, rev); err != nil {
		return types.Revision{}, fmt.Errorf("create rebased user revision: %w", err)
	}
	storedRev, err := h.rt.Store().GetRevision(ctx, rev.RevisionID, ownerID)
	if err != nil {
		return types.Revision{}, fmt.Errorf("load rebased user revision: %w", err)
	}
	return storedRev, nil
}

func mergeTextureRevisionMetadata(raw json.RawMessage, additions map[string]any) json.RawMessage {
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	for key, value := range additions {
		out[key] = value
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return encoded
}

func defaultDraftLine() textureDraftLineSummary {
	return textureDraftLineSummary{ID: "primary", Name: "Primary draft"}
}

func shortHash(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 10 {
		return value
	}
	return value[:10]
}

func mustMarshalString(value any) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"marshal_error":%q}`, err.Error())
	}
	return string(data)
}

func extractJSONObject(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty model response")
	}
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
			text = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return "", fmt.Errorf("model response did not contain a JSON object")
	}
	return text[start : end+1], nil
}

func countTextureCitationMarkers(content string) int {
	return strings.Count(content, "](source:") + len(regexp.MustCompile(`\[[0-9]{1,3}\]`).FindAllString(content, -1))
}

func countRawJSONItems(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return len(arr)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil && len(obj) > 0 {
		return 1
	}
	return 0
}

func snippet(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func (h *APIHandler) textureDocumentResponse(ctx context.Context, doc types.Document) textureDocumentResponse {
	resp := textureDocumentResponse{
		DocID:             doc.DocID,
		OwnerID:           doc.OwnerID,
		Title:             doc.Title,
		CurrentRevisionID: doc.CurrentRevisionID,
		CreatedAt:         doc.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:         doc.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	count, err := h.rt.Store().CountRevisionsByDoc(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		log.Printf("texture api: count document revisions for recent metadata: %v", err)
	} else {
		resp.RevisionCount = count
	}
	versionNumber, err := h.rt.Store().CurrentVersionNumberByDoc(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		log.Printf("texture api: get current document version number: %v", err)
	} else if versionNumber >= 0 {
		resp.CurrentVersionNumber = versionNumber
	}
	if strings.TrimSpace(doc.CurrentRevisionID) != "" {
		if rev, err := h.rt.Store().GetRevision(ctx, doc.CurrentRevisionID, doc.OwnerID); err == nil {
			resp.LastEditor = rev.AuthorLabel
			resp.LastAuthorKind = string(rev.AuthorKind)
			resp.CurrentVersionNumber = rev.VersionNumber
		} else {
			log.Printf("texture api: get current revision for recent metadata: %v", err)
		}
	}
	return resp
}

func revisionResponseFromRecord(rev types.Revision) textureRevisionResponse {
	return textureRevisionResponse{
		RevisionID:       rev.RevisionID,
		DocID:            rev.DocID,
		OwnerID:          rev.OwnerID,
		AuthorKind:       rev.AuthorKind,
		AuthorLabel:      rev.AuthorLabel,
		VersionNumber:    rev.VersionNumber,
		Content:          rev.Content,
		BodyDoc:          rev.BodyDoc,
		SourceEntities:   rev.SourceEntities,
		Citations:        rev.Citations,
		Metadata:         rev.Metadata,
		Provenance:       rev.Provenance,
		RevisionHash:     rev.RevisionHash,
		ParentRevisionID: rev.ParentRevisionID,
		CreatedAt:        rev.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
}

func (h *APIHandler) handleTextureListRevisions(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	limit := 10000
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 10000 {
		limit = 10000
	}
	readOwnerID, err := h.resolveUniversalWireTextureReadOwner(r.Context(), ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("texture api: resolve revision list owner %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list revisions"})
		return
	}
	revs, err := h.rt.Store().ListRevisionsByDoc(r.Context(), docID, readOwnerID, limit)
	if err != nil {
		log.Printf("texture api: list revisions: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list revisions"})
		return
	}

	resp := textureListRevisionsResponse{Revisions: make([]textureRevisionResponse, 0, len(revs))}
	for _, rev := range revs {
		if readOwnerID != ownerID {
			rev = normalizeWireArticleRevisionForRead(rev)
		}
		resp.Revisions = append(resp.Revisions, revisionResponseFromRecord(rev))
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

// HandleTextureRevision handles GET /api/texture/revisions/{id}.
// Opening a historical revision does not mutate the document head
// (VAL-ETEXT-007: historical snapshots can be opened without mutating head).
func (h *APIHandler) HandleTextureRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	revisionID := extractRevisionID(r.URL.Path)
	if revisionID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "revision ID is required"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	rev, err := h.rt.Store().GetRevision(r.Context(), revisionID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if unscoped, unscopedErr := h.rt.Store().GetRevisionUnscoped(r.Context(), revisionID); unscopedErr == nil {
				readOwnerID, resolveErr := h.resolveUniversalWireTextureReadOwner(r.Context(), ownerID, unscoped.DocID)
				if resolveErr == nil && readOwnerID == unscoped.OwnerID {
					rev = unscoped
					err = nil
				}
			}
		}
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "revision not found"})
			return
		}
	}

	if readOwnerID, resolveErr := h.resolveUniversalWireTextureReadOwner(r.Context(), ownerID, rev.DocID); resolveErr == nil && readOwnerID != ownerID {
		rev = normalizeWireArticleRevisionForRead(rev)
	}
	writeAPIJSON(w, http.StatusOK, revisionResponseFromRecord(rev))
}

// HandleTextureHistory handles GET /api/texture/documents/{id}/history.
// It returns the revision history with explicit attribution metadata
// (VAL-ETEXT-006: version history lists revisions with explicit
// attribution metadata).
func (h *APIHandler) HandleTextureHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	readOwnerID, err := h.resolveUniversalWireTextureReadOwner(r.Context(), ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("texture api: resolve history owner %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get history"})
		return
	}
	entries, err := h.rt.Store().GetHistory(r.Context(), docID, readOwnerID, 50)
	if err != nil {
		log.Printf("texture api: get history: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get history"})
		return
	}

	writeAPIJSON(w, http.StatusOK, textureHistoryResponse{
		DocID:   docID,
		Entries: entries,
	})
}

// HandleTextureDocumentStream handles GET /api/texture/documents/{id}/stream.
// It provides a document-scoped SSE transport so the editor can follow the
// canonical document head instead of polling a specific loop ID.
func (h *APIHandler) HandleTextureDocumentStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	readOwnerID, err := h.resolveUniversalWireTextureReadOwner(r.Context(), ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("texture api: resolve stream owner %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to open document stream"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, readOwnerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	pendingMutation, err := h.pendingAgentMutationByDoc(r.Context(), docID, readOwnerID)
	if err != nil {
		log.Printf("texture api: get pending mutation for stream: %v", err)
	}
	writeSSEData(w, textureDocumentStreamEvent{
		Kind:              "snapshot",
		DocID:             doc.DocID,
		CurrentRevisionID: doc.CurrentRevisionID,
		Pending:           pendingMutation != nil,
		LoopID: func() string {
			if pendingMutation == nil {
				return ""
			}
			return pendingMutation.RunID
		}(),
	})

	ch := h.rt.EventBus().SubscribeWithBuffer(128)
	defer h.rt.EventBus().Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.Record.OwnerID != ownerID && ev.Record.OwnerID != "" {
				continue
			}
			streamEvent, ok := textureStreamEventFromRecord(ev.Record)
			if !ok || streamEvent.DocID != docID {
				continue
			}
			writeSSEData(w, streamEvent)

			if streamEvent.Kind != "synth_completed" {
				if streamEvent.Kind != "revision_created" {
					continue
				}
				currentRevisionID := streamEvent.CurrentRevisionID
				if currentRevisionID == "" {
					updatedDoc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
					if err != nil {
						log.Printf("texture api: get document after revision create: %v", err)
						continue
					}
					currentRevisionID = updatedDoc.CurrentRevisionID
				}
				writeSSEData(w, textureDocumentStreamEvent{
					Kind:              "head_changed",
					DocID:             docID,
					LoopID:            streamEvent.LoopID,
					RevisionID:        streamEvent.RevisionID,
					CurrentRevisionID: currentRevisionID,
				})
				continue
			}

			updatedDoc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
			if err != nil {
				log.Printf("texture api: get document after synth completion: %v", err)
				continue
			}
			if streamEvent.RevisionID != "" {
				writeSSEData(w, textureDocumentStreamEvent{
					Kind:              "revision_created",
					DocID:             docID,
					LoopID:            streamEvent.LoopID,
					RevisionID:        streamEvent.RevisionID,
					CurrentRevisionID: updatedDoc.CurrentRevisionID,
				})
			}
			writeSSEData(w, textureDocumentStreamEvent{
				Kind:              "head_changed",
				DocID:             docID,
				LoopID:            streamEvent.LoopID,
				RevisionID:        streamEvent.RevisionID,
				CurrentRevisionID: updatedDoc.CurrentRevisionID,
			})
		}
	}
}

func textureStreamEventFromRecord(rec types.EventRecord) (textureDocumentStreamEvent, bool) {
	var payload map[string]any
	if len(rec.Payload) > 0 {
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return textureDocumentStreamEvent{}, false
		}
	}

	docID := metadataStringValue(payload, "doc_id")
	if docID == "" {
		return textureDocumentStreamEvent{}, false
	}

	event := textureDocumentStreamEvent{
		DocID:             docID,
		LoopID:            metadataStringValue(payload, "loop_id"),
		RevisionID:        metadataStringValue(payload, "revision_id"),
		CurrentRevisionID: metadataStringValue(payload, "current_revision_id"),
		Error:             metadataStringValue(payload, "error"),
	}
	switch rec.Kind {
	case types.EventTextureAgentRevisionStarted:
		event.Kind = "synth_started"
	case types.EventTextureAgentRevisionProgress:
		event.Kind = "synth_progress"
	case types.EventTextureAgentRevisionCompleted:
		event.Kind = "synth_completed"
	case types.EventRunPassivated:
		if !isTextureAgentID(rec.AgentID) {
			return textureDocumentStreamEvent{}, false
		}
		event.Kind = "synth_completed"
	case types.EventTextureAgentRevisionFailed:
		event.Kind = "synth_failed"
	case types.EventTextureDocumentRevisionCreated:
		event.Kind = "revision_created"
	default:
		return textureDocumentStreamEvent{}, false
	}
	return event, true
}

// HandleTextureDiff handles GET /api/texture/diff?from={id}&to={id}.
// It compares selected from and to revisions and shows the changed
// sections (VAL-ETEXT-008: diff view compares selected revisions and
// changed sections).
func (h *APIHandler) HandleTextureDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	fromRevID := r.URL.Query().Get("from")
	toRevID := r.URL.Query().Get("to")
	if fromRevID == "" || toRevID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "from and to revision IDs are required"})
		return
	}

	diff, err := h.rt.Store().GetDiff(r.Context(), fromRevID, toRevID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: fmt.Sprintf("failed to compute diff: %v", err)})
		return
	}

	writeAPIJSON(w, http.StatusOK, textureDiffResponse{DiffResult: diff})
}

func (h *APIHandler) HandleTextureRestoreRevision(w http.ResponseWriter, r *http.Request) {
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
	var req textureRestoreRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	sourceRev, err := h.rt.Store().GetRevision(r.Context(), strings.TrimSpace(req.RevisionID), ownerID)
	if err != nil || sourceRev.DocID != docID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "revision not found"})
		return
	}
	if err := h.canonicalizeAliasedTextureDocumentTitle(r.Context(), ownerID, &doc, time.Now().UTC()); err != nil {
		log.Printf("texture api: canonicalize restore document title: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to canonicalize document title"})
		return
	}
	metadata := mergeTextureRevisionMetadata(sourceRev.Metadata, map[string]any{
		"source":                     "restore_historical_revision",
		"restored_from_revision_id":  sourceRev.RevisionID,
		"restore_target_revision_id": doc.CurrentRevisionID,
		"restore_mode":               strings.TrimSpace(req.Mode),
		"draft_line":                 defaultDraftLine(),
	})
	content := sourceRev.Content
	if normalized, changed := markdownstructure.NormalizeTableShapedRows(content); changed {
		content = normalized
		metadata = mergeTextureRevisionMetadata(metadata, map[string]any{
			"texture_structure_stabilized":        true,
			"texture_structure_stabilized_reason": "normalized_restored_markdown_table_rows",
		})
	}
	rev := types.Revision{
		RevisionID:       uuid.New().String(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      ownerID,
		Content:          content,
		Citations:        sourceRev.Citations,
		Metadata:         metadata,
		ParentRevisionID: doc.CurrentRevisionID,
		CreatedAt:        time.Now().UTC(),
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("texture api: restore revision: %v", err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "failed to restore revision; document head may have changed"})
		return
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		log.Printf("texture api: load restored revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load restored revision"})
		return
	}
	h.rt.emitTextureDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(storedRev))
}

func (h *APIHandler) HandleTextureDiagnosis(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}
	includeContent := diagnosisIncludeContent(r)
	resp := textureDiagnosisResponse{
		OwnerID:     ownerID,
		DocID:       docID,
		StorePath:   h.rt.Store().Path(),
		TexturePath: h.rt.Store().TexturePath(),
	}
	if docID != "" {
		if doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID); err == nil {
			docResp := h.textureDocumentResponse(r.Context(), doc)
			resp.Document = &docResp
			if revs, err := h.rt.Store().ListRevisionsByDoc(r.Context(), docID, ownerID, limit); err == nil {
				for _, rev := range revs {
					resp.RevisionStructures = append(resp.RevisionStructures, revisionStructureSummaryFromRecord(rev))
					if includeContent {
						resp.Revisions = append(resp.Revisions, revisionResponseFromRecord(rev))
					}
				}
			}
			if messages, err := h.rt.Store().ListChannelMessages(r.Context(), ownerID, docID, 0, limit); err == nil {
				resp.Messages = messages
			}
			if decisions, err := h.rt.Store().ListTextureDecisionsByDocument(r.Context(), ownerID, docID, limit); err == nil {
				resp.Decisions = decisions
			} else {
				log.Printf("texture api: list decisions for %s: %v", docID, err)
			}
		}
	}
	if docID != "" {
		if channelRuns, err := h.rt.Store().ListRunsByChannel(r.Context(), ownerID, docID, limit); err == nil {
			resp.Runs = append(resp.Runs, channelRuns...)
		} else {
			log.Printf("texture api: list channel diagnosis runs for %s: %v", docID, err)
		}
		if ownerRuns, err := h.rt.Store().ListRunsByOwner(r.Context(), ownerID, diagnosisOwnerRunScanLimit(limit)); err == nil {
			var docRuns []types.RunRecord
			for _, run := range ownerRuns {
				if runRecordBelongsToTextureDoc(run, docID) {
					docRuns = append(docRuns, run)
				}
			}
			resp.Runs = appendUniqueRunRecords(resp.Runs, docRuns...)
		} else {
			log.Printf("texture api: list owner runs for document diagnosis %s: %v", docID, err)
		}
	}
	if runs, err := h.rt.Store().ListRunsByOwner(r.Context(), ownerID, limit); err == nil {
		resp.Runs = appendUniqueRunRecords(resp.Runs, runs...)
	}
	for _, run := range resp.Runs {
		if strings.Contains(run.Error, "Incorrect string value") || strings.Contains(run.Result, "Incorrect string value") {
			resp.ErrorMatches = append(resp.ErrorMatches, run.RunID+": Incorrect string value")
		}
	}
	if events, err := h.rt.Store().ListEventsByOwner(r.Context(), ownerID, limit); err == nil {
		resp.Events = events
		for _, ev := range events {
			if strings.Contains(string(ev.Payload), "Incorrect string value") {
				resp.ErrorMatches = append(resp.ErrorMatches, ev.EventID+": Incorrect string value")
			}
		}
	}
	if evidence, err := h.rt.Store().ListEvidenceByAgent(r.Context(), ownerID, "", limit); err == nil {
		resp.Evidence = evidence
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

// HandleTextureBlame handles GET /api/texture/revisions/{id}/blame.
// It provides section-level attribution that distinguishes whether the
// last editor was the user or the agent (VAL-ETEXT-009: blame identifies
// the last editor per section).
func (h *APIHandler) HandleTextureBlame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	revisionID := extractRevisionID(r.URL.Path)
	if revisionID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "revision ID is required"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	blame, err := h.rt.Store().GetBlame(r.Context(), revisionID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "revision not found"})
		return
	}

	writeAPIJSON(w, http.StatusOK, textureBlameResponse{BlameResult: blame})
}

// ----- Texture revise -----

type testTextureResearchFindingsRequest struct {
	DocID     string                         `json:"doc_id"`
	FindingID string                         `json:"finding_id"`
	Findings  []string                       `json:"findings,omitempty"`
	Evidence  []researchFindingEvidenceInput `json:"evidence,omitempty"`
	Notes     []string                       `json:"notes,omitempty"`
	Questions []string                       `json:"questions,omitempty"`
}

type testTextureWorkerUpdateRequest struct {
	DocID       string   `json:"doc_id"`
	UpdateID    string   `json:"update_id"`
	Role        string   `json:"role,omitempty"`
	Findings    []string `json:"findings,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
	Refs        []string `json:"refs,omitempty"`
	Tests       []string `json:"tests,omitempty"`
	Questions   []string `json:"questions,omitempty"`
	Proposals   []string `json:"proposals,omitempty"`
	Notes       []string `json:"notes,omitempty"`
}

// HandleTestTextureResearchFindings is a local-only dry-run browser test seam that
// routes through the real researcher tool path instead of inventing a fake
// direct revision shortcut. It is not product proof and must stay disabled
// outside local/test environments.
func (h *APIHandler) HandleTestTextureResearchFindings(w http.ResponseWriter, r *http.Request) {
	if !h.rt.cfg.EnableTestAPIs {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "test endpoint not found"})
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req testTextureResearchFindingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	req.DocID = strings.TrimSpace(req.DocID)
	req.FindingID = strings.TrimSpace(req.FindingID)
	if req.DocID == "" || req.FindingID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "doc_id and finding_id are required"})
		return
	}

	if _, err := h.rt.Store().GetDocument(r.Context(), req.DocID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}

	runs, err := h.rt.Store().ListRunsByChannel(r.Context(), ownerID, req.DocID, 50)
	if err != nil {
		log.Printf("texture test api: list channel runs: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve texture agent"})
		return
	}

	var parent *types.RunRecord
	for i := len(runs) - 1; i >= 0; i-- {
		if agentProfileForRun(&runs[i]) == AgentProfileTexture {
			parent = &runs[i]
			break
		}
	}
	if parent == nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "texture agent is not initialized for this document"})
		return
	}

	targetAgentID := strings.TrimSpace(agentIDForRun(parent))
	if targetAgentID == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "texture agent is missing an agent_id"})
		return
	}

	researcherRun, err := h.rt.StartCoagentRun(r.Context(), parent.RunID, "Browser test: submit research findings", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    req.DocID,
		"doc_id":                req.DocID,
	})
	if err != nil {
		log.Printf("texture test api: start researcher run: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create researcher context"})
		return
	}

	registry := h.rt.ToolRegistryForProfile(AgentProfileResearcher)
	if registry == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "researcher tools are unavailable"})
		return
	}

	rawArgs, err := json.Marshal(map[string]any{
		"update_id":  req.FindingID,
		"kind":       "findings",
		"agent_id":   targetAgentID,
		"channel_id": req.DocID,
		"findings":   req.Findings,
		"evidence":   req.Evidence,
		"notes":      req.Notes,
		"questions":  req.Questions,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	raw, err := registry.Execute(WithToolExecutionContext(r.Context(), researcherRun), "update_coagent", rawArgs)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Printf("texture test api: decode tool response: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to encode findings response"})
		return
	}
	resp["loop_id"] = researcherRun.RunID
	writeAPIJSON(w, http.StatusAccepted, resp)
}

// HandleTestTextureWorkerUpdate is a local-only dry-run browser test seam that
// routes through the real structured worker-update tool. It is not product
// proof and stays disabled unless RUNTIME_ENABLE_TEST_APIS is set.
func (h *APIHandler) HandleTestTextureWorkerUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.rt.cfg.EnableTestAPIs {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "test endpoint not found"})
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req testTextureWorkerUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	req.DocID = strings.TrimSpace(req.DocID)
	req.UpdateID = strings.TrimSpace(req.UpdateID)
	if req.DocID == "" || req.UpdateID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "doc_id and update_id are required"})
		return
	}

	if _, err := h.rt.Store().GetDocument(r.Context(), req.DocID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	targetAgentID := currentTextureAgentID(req.DocID)
	if _, err := h.rt.Store().GetAgent(r.Context(), targetAgentID); err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "Texture agent is not initialized for this document"})
		return
	}

	runs, err := h.rt.Store().ListRunsByChannel(r.Context(), ownerID, req.DocID, 50)
	if err != nil {
		log.Printf("texture test api: list channel runs for worker update: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve texture agent"})
		return
	}

	var parent *types.RunRecord
	for i := len(runs) - 1; i >= 0; i-- {
		if agentProfileForRun(&runs[i]) == AgentProfileTexture {
			parent = &runs[i]
			break
		}
	}
	if parent == nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "texture agent has no run context for this document"})
		return
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = AgentProfileSuper
	}
	switch role {
	case AgentProfileResearcher, AgentProfileSuper, AgentProfileVSuper, AgentProfileCoSuper:
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "role must be researcher, super, vsuper, or co-super"})
		return
	}

	workerRun, err := h.rt.StartCoagentRun(r.Context(), parent.RunID, "Browser test: submit structured worker update", ownerID, map[string]any{
		runMetadataAgentProfile: role,
		runMetadataAgentRole:    role,
		runMetadataAgentID:      role + ":test:" + req.DocID,
		runMetadataChannelID:    req.DocID,
		"doc_id":                req.DocID,
	})
	if err != nil {
		log.Printf("texture test api: start worker update run: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create worker context"})
		return
	}

	registry := h.rt.ToolRegistryForProfile(role)
	if registry == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "worker tools are unavailable"})
		return
	}

	rawArgs, err := json.Marshal(map[string]any{
		"update_id":    req.UpdateID,
		"kind":         "status",
		"agent_id":     targetAgentID,
		"channel_id":   req.DocID,
		"findings":     req.Findings,
		"evidence_ids": req.EvidenceIDs,
		"artifacts":    req.Artifacts,
		"refs":         req.Refs,
		"tests":        req.Tests,
		"questions":    req.Questions,
		"proposals":    req.Proposals,
		"notes":        req.Notes,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	raw, err := registry.Execute(WithToolExecutionContext(r.Context(), workerRun), "update_coagent", rawArgs)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Printf("texture test api: decode worker update response: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to encode worker update response"})
		return
	}
	resp["loop_id"] = workerRun.RunID
	writeAPIJSON(w, http.StatusAccepted, resp)
}
