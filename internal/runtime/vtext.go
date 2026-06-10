// Package runtime provides vtext document API handlers for the go-choir
// sandbox runtime. These handlers expose the document CRUD, revision,
// history, snapshot, diff, blame, and agent revision APIs through the
// authenticated same-origin proxy path.
//
// API endpoints:
//
//	POST   /api/vtext/documents          — create a new document
//	GET    /api/vtext/documents          — list documents for the authenticated user
//	GET    /api/vtext/documents/{id}     — get a document by ID
//	PUT    /api/vtext/documents/{id}     — update a document (e.g., title)
//	DELETE /api/vtext/documents/{id}     — delete a document and its revisions
//	POST   /api/vtext/documents/{id}/revisions — create a user-authored revision
//	GET    /api/vtext/documents/{id}/revisions — list revisions for a document
//	GET    /api/vtext/documents/{id}/stream — stream document lifecycle changes
//	GET    /api/vtext/revisions/{id}    — get a specific revision (snapshot)
//	GET    /api/vtext/documents/{id}/history — get revision history with attribution
//	GET    /api/vtext/diff?from={id}&to={id} — diff two revisions
//	GET    /api/vtext/revisions/{id}/blame — blame a revision
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	pathpkg "path"
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
	vtextMarkerLineRE          = regexp.MustCompile(`(?im)^.*USER_[A-Z0-9_]*MARKER[A-Z0-9_]*.*$`)
	vtextNumberedHeadingRE     = regexp.MustCompile(`(?m)^\s*(?:#{1,6}\s*)?(\d{1,2}\.\s+[^\n:]{2,100})\s*$`)
	vtextSectionUpdatePrefixRE = regexp.MustCompile(`\bSECTION\s+\d+\s+UPDATE:`)
	vtextSHA256RequirementRE   = regexp.MustCompile(`\b[a-fA-F0-9]{64}\b`)
	vtextInlineSourceRefRE     = regexp.MustCompile(`\[[^\]\n]{1,160}\]\(source:[^) \t\r\n]{1,160}\)`)
)

// ----- Request/Response types -----

// vtextCreateDocRequest is the JSON payload for POST /api/vtext/documents.
type vtextCreateDocRequest struct {
	Title string `json:"title"`
}

// vtextCreateDocResponse is the JSON response for POST /api/vtext/documents.
type vtextCreateDocResponse struct {
	DocID     string `json:"doc_id"`
	OwnerID   string `json:"owner_id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

type vtextOpenFileRequest struct {
	SourcePath     string `json:"source_path"`
	Title          string `json:"title"`
	InitialContent string `json:"initial_content"`
}

type vtextOpenFileResponse struct {
	DocID             string `json:"doc_id"`
	CurrentRevisionID string `json:"current_revision_id,omitempty"`
	Created           bool   `json:"created"`
	OriginalContentID string `json:"original_content_id,omitempty"`
}

type vtextMarkdownLineageImportRequest struct {
	SourcePath          string                          `json:"source_path"`
	Title               string                          `json:"title"`
	SourceEntities      []vtextSourceEntity             `json:"source_entities,omitempty"`
	CitationResolutions []vtextCitationMarkerResolution `json:"citation_resolutions,omitempty"`
	Versions            []vtextMarkdownLineageVersion   `json:"versions"`
}

type vtextMarkdownLineageVersion struct {
	Label               string                          `json:"label,omitempty"`
	SourceRevisionID    string                          `json:"source_revision_id,omitempty"`
	ContentItemID       string                          `json:"content_item_id,omitempty"`
	Content             string                          `json:"content"`
	CreatedAt           string                          `json:"created_at,omitempty"`
	Metadata            json.RawMessage                 `json:"metadata,omitempty"`
	SourceEntities      []vtextSourceEntity             `json:"source_entities,omitempty"`
	CitationResolutions []vtextCitationMarkerResolution `json:"citation_resolutions,omitempty"`
}

type vtextCitationMarkerResolution struct {
	Marker        string `json:"marker"`
	EntityID      string `json:"entity_id"`
	Action        string `json:"action,omitempty"`
	Reason        string `json:"reason,omitempty"`
	EvidenceState string `json:"evidence_state,omitempty"`
}

type resolvedMarkdownLineageVersion struct {
	Version       vtextMarkdownLineageVersion
	Content       string
	ContentItem   *types.ContentItem
	ContentID     string
	ContentHash   string
	ContentPath   string
	ContentSource string
}

type vtextMarkdownLineageImportResponse struct {
	DocID              string                  `json:"doc_id"`
	CurrentRevisionID  string                  `json:"current_revision_id"`
	SourcePath         string                  `json:"source_path"`
	Created            bool                    `json:"created"`
	RevisionCount      int                     `json:"revision_count"`
	Revisions          []vtextRevisionResponse `json:"revisions"`
	OriginalContentIDs []string                `json:"original_content_ids"`
	ExistingDocID      string                  `json:"existing_doc_id,omitempty"`
}

type vtextSourceGapRepairRequest struct {
	BaseRevisionID      string                          `json:"base_revision_id,omitempty"`
	SourceEntities      []vtextSourceEntity             `json:"source_entities,omitempty"`
	CitationResolutions []vtextCitationMarkerResolution `json:"citation_resolutions,omitempty"`
	AuthorLabel         string                          `json:"author_label,omitempty"`
}

type vtextSourceArtifactAttachmentRequest struct {
	BaseRevisionID string                          `json:"base_revision_id,omitempty"`
	Attachments    []vtextSourceArtifactAttachment `json:"attachments,omitempty"`
	AuthorLabel    string                          `json:"author_label,omitempty"`
}

type vtextSourceArtifactAttachment struct {
	EntityID  string `json:"entity_id"`
	ContentID string `json:"content_id"`
	TextQuote string `json:"text_quote,omitempty"`
}

type vtextEnsureManifestResponse struct {
	DocID      string `json:"doc_id"`
	SourcePath string `json:"source_path"`
}

type vtextDocumentExportResponse struct {
	DocID       string `json:"doc_id"`
	RevisionID  string `json:"revision_id"`
	Format      string `json:"format"`
	MediaType   string `json:"media_type"`
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentHash string `json:"content_hash"`
}

// vtextDocumentResponse is the JSON response for GET /api/vtext/documents/{id}.
type vtextDocumentResponse struct {
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

// vtextDocumentStreamEvent is the hidden transport envelope sent over the
// document-scoped SSE stream. The editor consumes document lifecycle changes
// from this stream but does not render raw agent chatter.
type vtextDocumentStreamEvent struct {
	Kind              string `json:"kind"`
	DocID             string `json:"doc_id"`
	LoopID            string `json:"loop_id,omitempty"`
	RevisionID        string `json:"revision_id,omitempty"`
	CurrentRevisionID string `json:"current_revision_id,omitempty"`
	Pending           bool   `json:"pending,omitempty"`
	Error             string `json:"error,omitempty"`
}

// vtextUpdateDocRequest is the JSON payload for PUT /api/vtext/documents/{id}.
type vtextUpdateDocRequest struct {
	Title string `json:"title"`
}

// vtextListDocsResponse is the JSON response for GET /api/vtext/documents.
type vtextListDocsResponse struct {
	Documents []vtextDocumentResponse `json:"documents"`
}

// vtextCreateRevisionRequest is the public JSON payload for
// POST /api/vtext/documents/{id}/revisions. The public route always creates
// user-authored revisions; author_kind/author_label are accepted only for
// older clients and are not authority-bearing.
type vtextCreateRevisionRequest struct {
	Content          string           `json:"content"`
	AuthorKind       types.AuthorKind `json:"author_kind"`
	AuthorLabel      string           `json:"author_label"`
	Citations        json.RawMessage  `json:"citations,omitempty"`
	Metadata         json.RawMessage  `json:"metadata,omitempty"`
	ParentRevisionID string           `json:"parent_revision_id,omitempty"`
	AllowRebase      bool             `json:"allow_rebase,omitempty"`
}

// vtextRevisionResponse is the JSON response for revision-related endpoints.
type vtextRevisionResponse struct {
	RevisionID       string           `json:"revision_id"`
	DocID            string           `json:"doc_id"`
	OwnerID          string           `json:"owner_id"`
	AuthorKind       types.AuthorKind `json:"author_kind"`
	AuthorLabel      string           `json:"author_label"`
	VersionNumber    int              `json:"version_number"`
	Content          string           `json:"content"`
	Citations        json.RawMessage  `json:"citations,omitempty"`
	Metadata         json.RawMessage  `json:"metadata,omitempty"`
	ParentRevisionID string           `json:"parent_revision_id,omitempty"`
	CreatedAt        string           `json:"created_at"`
}

// vtextListRevisionsResponse is the JSON response for
// GET /api/vtext/documents/{id}/revisions.
type vtextListRevisionsResponse struct {
	Revisions []vtextRevisionResponse `json:"revisions"`
}

// vtextHistoryResponse is the JSON response for
// GET /api/vtext/documents/{id}/history.
type vtextHistoryResponse struct {
	DocID   string               `json:"doc_id"`
	Entries []types.HistoryEntry `json:"entries"`
}

// vtextDiffResponse is the JSON response for GET /api/vtext/diff.
type vtextDiffResponse struct {
	types.DiffResult
}

type vtextSemanticCompareResponse struct {
	CompareID        string                 `json:"compare_id"`
	SourceRevisionID string                 `json:"source_revision_id"`
	TargetRevisionID string                 `json:"target_revision_id"`
	DraftLine        vtextDraftLineSummary  `json:"draft_line"`
	Summary          []string               `json:"summary"`
	Suggestions      []vtextMergeSuggestion `json:"suggestions"`
	Diff             types.DiffResult       `json:"diff"`
	ModelEvidence    map[string]any         `json:"model_evidence,omitempty"`
	EvidenceID       string                 `json:"evidence_id,omitempty"`
}

type vtextDraftLineSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type vtextMergeSuggestion struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Source      string `json:"source"`
	Preview     string `json:"preview,omitempty"`
}

type vtextMergePreviewRequest struct {
	SourceRevisionID   string   `json:"source_revision_id"`
	TargetRevisionID   string   `json:"target_revision_id"`
	SuggestionIDs      []string `json:"suggestion_ids"`
	SourceVersionLabel string   `json:"source_version_label,omitempty"`
	TargetVersionLabel string   `json:"target_version_label,omitempty"`
}

type vtextMergePreviewResponse struct {
	PreviewID        string                 `json:"preview_id"`
	DocID            string                 `json:"doc_id"`
	SourceRevisionID string                 `json:"source_revision_id"`
	TargetRevisionID string                 `json:"target_revision_id"`
	DraftLine        vtextDraftLineSummary  `json:"draft_line"`
	Content          string                 `json:"content"`
	Provenance       map[string]any         `json:"provenance"`
	Suggestions      []vtextMergeSuggestion `json:"suggestions"`
	ModelEvidence    map[string]any         `json:"model_evidence,omitempty"`
	EvidenceID       string                 `json:"evidence_id,omitempty"`
}

type vtextModelMergeEdit struct {
	SuggestionID string `json:"suggestion_id,omitempty"`
	Operation    string `json:"operation"`
	OldText      string `json:"old_text,omitempty"`
	NewText      string `json:"new_text,omitempty"`
	Rationale    string `json:"rationale,omitempty"`
}

type vtextModelSemanticMergeResult struct {
	Summary     []string               `json:"summary"`
	Suggestions []vtextMergeSuggestion `json:"suggestions"`
	Edits       []vtextModelMergeEdit  `json:"edits,omitempty"`
}

type vtextAcceptMergeRequest struct {
	PreviewID        string         `json:"preview_id"`
	Content          string         `json:"content"`
	SourceRevisionID string         `json:"source_revision_id"`
	TargetRevisionID string         `json:"target_revision_id"`
	SuggestionIDs    []string       `json:"suggestion_ids"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type vtextRestoreRevisionRequest struct {
	RevisionID string `json:"revision_id"`
	Mode       string `json:"mode,omitempty"`
}

// ----- Helper functions -----

// extractDocID extracts the document ID from the URL path.
// Expected pattern: /api/vtext/documents/{docID}/...
func extractDocID(path string) string {
	const prefix = "/api/vtext/documents/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	// The docID is the first path segment.
	parts := strings.SplitN(rest, "/", 2)
	return parts[0]
}

// extractRevisionID extracts the revision ID from the URL path.
// Expected pattern: /api/vtext/revisions/{revisionID}/...
func extractRevisionID(path string) string {
	const prefix = "/api/vtext/revisions/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	return parts[0]
}

func (h *APIHandler) canonicalizeAliasedVTextDocumentTitle(ctx context.Context, ownerID string, doc *types.Document, updatedAt time.Time) error {
	return canonicalizeAliasedVTextDocumentTitle(ctx, h.rt.Store(), ownerID, doc, updatedAt)
}

func (rt *Runtime) canonicalizeAliasedVTextDocumentTitle(ctx context.Context, ownerID string, doc *types.Document, updatedAt time.Time) error {
	if rt == nil || rt.store == nil {
		return nil
	}
	return canonicalizeAliasedVTextDocumentTitle(ctx, rt.store, ownerID, doc, updatedAt)
}

func canonicalizeAliasedVTextDocumentTitle(ctx context.Context, st *store.Store, ownerID string, doc *types.Document, updatedAt time.Time) error {
	if doc == nil || strings.EqualFold(pathpkg.Ext(strings.TrimSpace(doc.Title)), ".vtext") {
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
	nextTitle := canonicalVTextImportTitle(sourcePath, doc.Title)
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
		log.Printf("vtext api: marshal sse payload: %v", err)
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

// ----- Handler methods -----

// HandleVTextCreateDocument handles POST /api/vtext/documents.
// It creates a new document with a durable document identity (VAL-ETEXT-001).
func (h *APIHandler) HandleVTextCreateDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req vtextCreateDocRequest
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
		log.Printf("vtext api: create document: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create document"})
		return
	}

	writeAPIJSON(w, http.StatusCreated, vtextCreateDocResponse{
		DocID:     doc.DocID,
		OwnerID:   doc.OwnerID,
		Title:     doc.Title,
		CreatedAt: doc.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleVTextImportMarkdownLineage migrates ordered Markdown snapshots into one
// canonical VText document with durable version-numbered revisions.
func (h *APIHandler) HandleVTextImportMarkdownLineage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req vtextMarkdownLineageImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	sourcePath := normalizeVTextSourcePath(req.SourcePath)
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
	canonicalTitle := canonicalVTextImportTitle(sourcePath, title)
	if existingDocID, err := h.rt.Store().GetDocumentAlias(r.Context(), ownerID, sourcePath); err == nil {
		writeAPIJSON(w, http.StatusConflict, vtextMarkdownLineageImportResponse{
			SourcePath:    sourcePath,
			Created:       false,
			ExistingDocID: existingDocID,
		})
		return
	} else if err != store.ErrNotFound {
		log.Printf("vtext api: lookup markdown lineage alias %s: %v", sourcePath, err)
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
		log.Printf("vtext api: create markdown lineage document: %v", err)
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
			log.Printf("vtext api: preserve markdown lineage snapshot %s: %v", sourcePath, err)
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
	revisionResponses := make([]vtextRevisionResponse, 0, len(req.Versions))
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
		projectionContent := markdownLineageProjectionContent(resolved.Content, resolutions)
		rev := types.Revision{
			RevisionID:       uuid.New().String(),
			DocID:            doc.DocID,
			OwnerID:          ownerID,
			AuthorKind:       types.AuthorUser,
			AuthorLabel:      ownerID,
			Content:          projectionContent,
			Metadata:         metadata,
			ParentRevisionID: parentID,
			CreatedAt:        versionNow,
		}
		if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
			log.Printf("vtext api: create markdown lineage revision %s[%d]: %v", sourcePath, i, err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create markdown lineage revision"})
			return
		}
		storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
		if err != nil {
			log.Printf("vtext api: reload markdown lineage revision %s: %v", rev.RevisionID, err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load created revision"})
			return
		}
		revisionResponses = append(revisionResponses, revisionResponseFromRecord(storedRev))
		originalIDs = append(originalIDs, resolved.ContentID)
		parentID = rev.RevisionID
		doc.CurrentRevisionID = rev.RevisionID
	}
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), ownerID, sourcePath, doc.DocID, now); err != nil {
		log.Printf("vtext api: upsert markdown lineage alias %s -> %s: %v", sourcePath, doc.DocID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to persist document alias"})
		return
	}
	writeAPIJSON(w, http.StatusCreated, vtextMarkdownLineageImportResponse{
		DocID:              doc.DocID,
		CurrentRevisionID:  doc.CurrentRevisionID,
		SourcePath:         sourcePath,
		Created:            true,
		RevisionCount:      len(revisionResponses),
		Revisions:          revisionResponses,
		OriginalContentIDs: originalIDs,
	})
}

// HandleVTextOpenFile resolves a file-browser path to one canonical vtext
// document. The first open creates the document + alias; later opens reuse it.
func (h *APIHandler) HandleVTextOpenFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req vtextOpenFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	sourcePath := normalizeVTextSourcePath(req.SourcePath)
	if sourcePath == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source_path is required"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		parts := strings.Split(sourcePath, "/")
		title = parts[len(parts)-1]
	}
	canonicalTitle := canonicalVTextImportTitle(sourcePath, title)

	docID, err := h.rt.Store().GetDocumentAlias(r.Context(), ownerID, sourcePath)
	if err == nil {
		doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
		if err != nil {
			log.Printf("vtext api: resolve aliased document %s: %v", docID, err)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to open aliased document"})
			return
		}
		writeAPIJSON(w, http.StatusOK, vtextOpenFileResponse{
			DocID:             doc.DocID,
			CurrentRevisionID: doc.CurrentRevisionID,
			Created:           false,
		})
		return
	}
	if err != store.ErrNotFound {
		log.Printf("vtext api: lookup file alias %s: %v", sourcePath, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve file alias"})
		return
	}

	now := time.Now().UTC()
	projection := buildVTextFileImportProjection(sourcePath, req.InitialContent)
	var original *types.ContentItem
	if !isVTextShortcutPath(sourcePath) {
		item, err := h.ensureVTextOriginalContentItem(r.Context(), ownerID, title, projection, now)
		if err != nil {
			log.Printf("vtext api: preserve original content item for %s: %v", sourcePath, err)
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
		log.Printf("vtext api: create aliased document: %v", err)
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
		Metadata:    buildFileOpenVTextMetadata(projection, original),
		CreatedAt:   now,
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("vtext api: create aliased initial revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create aliased initial revision"})
		return
	}
	if err := h.rt.Store().UpsertDocumentAlias(r.Context(), ownerID, sourcePath, doc.DocID, now); err != nil {
		log.Printf("vtext api: upsert file alias %s -> %s: %v", sourcePath, doc.DocID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to persist file alias"})
		return
	}
	doc.CurrentRevisionID = rev.RevisionID
	writeAPIJSON(w, http.StatusCreated, vtextOpenFileResponse{
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

// HandleVTextEnsureManifest ensures a canonical vtext document has a
// filesystem shortcut so it appears in Files while the real document state
// stays canonical in Dolt.
func (h *APIHandler) HandleVTextEnsureManifest(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("vtext api: get document for manifest %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load document"})
		return
	}
	sourcePath, err := h.ensureVTextManifest(r.Context(), ownerID, doc)
	if err != nil {
		log.Printf("vtext api: ensure manifest for %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to persist file manifest"})
		return
	}
	writeAPIJSON(w, http.StatusOK, vtextEnsureManifestResponse{
		DocID:      doc.DocID,
		SourcePath: sourcePath,
	})
}

func (h *APIHandler) HandleVTextExportDocument(w http.ResponseWriter, r *http.Request) {
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
	writeAPIJSON(w, http.StatusOK, vtextDocumentExportResponse{
		DocID:       doc.DocID,
		RevisionID:  rev.RevisionID,
		Format:      format,
		MediaType:   mediaType,
		Filename:    vtextDocumentExportFilename(doc.Title, format),
		Content:     content,
		ContentHash: contentHash(content),
	})
}

// HandleVTextListDocuments handles GET /api/vtext/documents.
// It returns documents owned by the authenticated user.
func (h *APIHandler) HandleVTextListDocuments(w http.ResponseWriter, r *http.Request) {
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
		log.Printf("vtext api: list documents: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list documents"})
		return
	}

	resp := vtextListDocsResponse{Documents: make([]vtextDocumentResponse, 0, len(docs))}
	for _, doc := range docs {
		docResp := h.vtextDocumentResponse(r.Context(), doc)
		resp.Documents = append(resp.Documents, docResp)
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

// HandleVTextDocument handles GET/PUT/DELETE /api/vtext/documents/{id}.
func (h *APIHandler) HandleVTextDocument(w http.ResponseWriter, r *http.Request) {
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleVTextGetDocument(w, r, docID)
	case http.MethodPut:
		h.handleVTextUpdateDocument(w, r, docID)
	case http.MethodDelete:
		h.handleVTextDeleteDocument(w, r, docID)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) handleVTextGetDocument(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	readOwnerID, err := h.resolveUniversalWireVTextReadOwner(r.Context(), ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("vtext api: resolve document owner %s: %v", docID, err)
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
		log.Printf("vtext api: get pending mutation for document: %v", err)
	}

	resp := h.vtextDocumentResponse(r.Context(), doc)
	resp.AgentRevisionPending = pendingMutation != nil
	if pendingMutation != nil {
		resp.AgentRevisionRunID = pendingMutation.RunID
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) handleVTextUpdateDocument(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req vtextUpdateDocRequest
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
		log.Printf("vtext api: update document: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to update document"})
		return
	}

	writeAPIJSON(w, http.StatusOK, vtextDocumentResponse{
		DocID:             doc.DocID,
		OwnerID:           doc.OwnerID,
		Title:             doc.Title,
		CurrentRevisionID: doc.CurrentRevisionID,
		CreatedAt:         doc.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:         doc.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

func (h *APIHandler) handleVTextDeleteDocument(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	if err := h.rt.Store().DeleteDocument(r.Context(), docID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}

	writeAPIJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// HandleVTextRevisions handles POST and GET
// /api/vtext/documents/{id}/revisions.
func (h *APIHandler) HandleVTextRevisions(w http.ResponseWriter, r *http.Request) {
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handleVTextCreateRevision(w, r, docID)
	case http.MethodGet:
		h.handleVTextListRevisions(w, r, docID)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *APIHandler) handleVTextCreateRevision(w http.ResponseWriter, r *http.Request, docID string) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req vtextCreateRevisionRequest
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
	if err := h.canonicalizeAliasedVTextDocumentTitle(r.Context(), ownerID, &doc, now); err != nil {
		log.Printf("vtext api: canonicalize aliased document title: %v", err)
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
			content, stabilized = stabilizeVTextUserMarkdownStructures(parentRev.Content, content)
			if stabilized {
				metadata = mergeVTextRevisionMetadata(metadata, map[string]any{
					"vtext_structure_stabilized":        true,
					"vtext_structure_stabilized_reason": "preserved_parent_markdown_table_after_collapsed_draft",
				})
			}
		} else {
			log.Printf("vtext api: load parent revision for structure stabilization %s: %v", parentID, err)
		}
	}
	if hasParentRev {
		metadata = carryForwardDurableVTextMetadata(metadata, parentRev.Metadata)
	}
	if canonicalPath, err := h.ensureCanonicalVTextProjectionPath(r.Context(), ownerID, doc); err == nil && canonicalPath != "" {
		metadata = mergeVTextRevisionMetadata(metadata, map[string]any{
			"canonical_vtext_source_path": canonicalPath,
		})
	} else if err != nil {
		log.Printf("vtext api: ensure canonical vtext projection path: %v", err)
	}

	rev := types.Revision{
		RevisionID:       uuid.New().String(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      ownerID,
		Content:          content,
		Citations:        citations,
		Metadata:         metadata,
		ParentRevisionID: parentID,
		CreatedAt:        now,
	}

	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("vtext api: create revision: %v", err)
		if errors.Is(err, store.ErrStaleDocumentHead) {
			if req.AllowRebase && parentID != "" {
				req.Content = content
				rebased, rebaseErr := h.createRebasedUserRevision(r.Context(), docID, ownerID, req, parentID, citations, metadata, now)
				if rebaseErr == nil {
					h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, rebased)
					writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(rebased))
					return
				}
				log.Printf("vtext api: rebase stale user revision: %v", rebaseErr)
			}
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "document head changed; reload the latest version before saving"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to create revision"})
		return
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		log.Printf("vtext api: load created revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load created revision"})
		return
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)

	writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(storedRev))
}

func (h *APIHandler) createRebasedUserRevision(ctx context.Context, docID, ownerID string, req vtextCreateRevisionRequest, staleParentID string, citations, metadata json.RawMessage, now time.Time) (types.Revision, error) {
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
	mergedMetadata := mergeVTextRevisionMetadata(metadata, map[string]any{
		"rebased_from_revision_id": staleParentID,
		"rebase_onto_revision_id":  headRev.RevisionID,
		"rebase_strategy":          strategy,
		"rebase_clean":             clean,
	})
	mergedMetadata = carryForwardDurableVTextMetadata(mergedMetadata, headRev.Metadata)

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

func mergeVTextRevisionMetadata(raw json.RawMessage, additions map[string]any) json.RawMessage {
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

func defaultDraftLine() vtextDraftLineSummary {
	return vtextDraftLineSummary{ID: "primary", Name: "Primary draft"}
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

func countVTextCitationMarkers(content string) int {
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

func (h *APIHandler) vtextDocumentResponse(ctx context.Context, doc types.Document) vtextDocumentResponse {
	resp := vtextDocumentResponse{
		DocID:             doc.DocID,
		OwnerID:           doc.OwnerID,
		Title:             doc.Title,
		CurrentRevisionID: doc.CurrentRevisionID,
		CreatedAt:         doc.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:         doc.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
	count, err := h.rt.Store().CountRevisionsByDoc(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		log.Printf("vtext api: count document revisions for recent metadata: %v", err)
	} else {
		resp.RevisionCount = count
	}
	versionNumber, err := h.rt.Store().CurrentVersionNumberByDoc(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		log.Printf("vtext api: get current document version number: %v", err)
	} else if versionNumber >= 0 {
		resp.CurrentVersionNumber = versionNumber
	}
	if strings.TrimSpace(doc.CurrentRevisionID) != "" {
		if rev, err := h.rt.Store().GetRevision(ctx, doc.CurrentRevisionID, doc.OwnerID); err == nil {
			resp.LastEditor = rev.AuthorLabel
			resp.LastAuthorKind = string(rev.AuthorKind)
			resp.CurrentVersionNumber = rev.VersionNumber
		} else {
			log.Printf("vtext api: get current revision for recent metadata: %v", err)
		}
	}
	return resp
}

func revisionResponseFromRecord(rev types.Revision) vtextRevisionResponse {
	return vtextRevisionResponse{
		RevisionID:       rev.RevisionID,
		DocID:            rev.DocID,
		OwnerID:          rev.OwnerID,
		AuthorKind:       rev.AuthorKind,
		AuthorLabel:      rev.AuthorLabel,
		VersionNumber:    rev.VersionNumber,
		Content:          rev.Content,
		Citations:        rev.Citations,
		Metadata:         rev.Metadata,
		ParentRevisionID: rev.ParentRevisionID,
		CreatedAt:        rev.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
}

func (h *APIHandler) handleVTextListRevisions(w http.ResponseWriter, r *http.Request, docID string) {
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
	readOwnerID, err := h.resolveUniversalWireVTextReadOwner(r.Context(), ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("vtext api: resolve revision list owner %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list revisions"})
		return
	}
	revs, err := h.rt.Store().ListRevisionsByDoc(r.Context(), docID, readOwnerID, limit)
	if err != nil {
		log.Printf("vtext api: list revisions: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list revisions"})
		return
	}

	resp := vtextListRevisionsResponse{Revisions: make([]vtextRevisionResponse, 0, len(revs))}
	for _, rev := range revs {
		if readOwnerID != ownerID {
			rev = normalizeWireArticleRevisionForRead(rev)
		}
		resp.Revisions = append(resp.Revisions, revisionResponseFromRecord(rev))
	}

	writeAPIJSON(w, http.StatusOK, resp)
}

// HandleVTextRevision handles GET /api/vtext/revisions/{id}.
// Opening a historical revision does not mutate the document head
// (VAL-ETEXT-007: historical snapshots can be opened without mutating head).
func (h *APIHandler) HandleVTextRevision(w http.ResponseWriter, r *http.Request) {
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
				readOwnerID, resolveErr := h.resolveUniversalWireVTextReadOwner(r.Context(), ownerID, unscoped.DocID)
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

	if readOwnerID, resolveErr := h.resolveUniversalWireVTextReadOwner(r.Context(), ownerID, rev.DocID); resolveErr == nil && readOwnerID != ownerID {
		rev = normalizeWireArticleRevisionForRead(rev)
	}
	writeAPIJSON(w, http.StatusOK, revisionResponseFromRecord(rev))
}

// HandleVTextHistory handles GET /api/vtext/documents/{id}/history.
// It returns the revision history with explicit attribution metadata
// (VAL-ETEXT-006: version history lists revisions with explicit
// attribution metadata).
func (h *APIHandler) HandleVTextHistory(w http.ResponseWriter, r *http.Request) {
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

	readOwnerID, err := h.resolveUniversalWireVTextReadOwner(r.Context(), ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("vtext api: resolve history owner %s: %v", docID, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get history"})
		return
	}
	entries, err := h.rt.Store().GetHistory(r.Context(), docID, readOwnerID, 50)
	if err != nil {
		log.Printf("vtext api: get history: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get history"})
		return
	}

	writeAPIJSON(w, http.StatusOK, vtextHistoryResponse{
		DocID:   docID,
		Entries: entries,
	})
}

// HandleVTextDocumentStream handles GET /api/vtext/documents/{id}/stream.
// It provides a document-scoped SSE transport so the editor can follow the
// canonical document head instead of polling a specific loop ID.
func (h *APIHandler) HandleVTextDocumentStream(w http.ResponseWriter, r *http.Request) {
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

	readOwnerID, err := h.resolveUniversalWireVTextReadOwner(r.Context(), ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
			return
		}
		log.Printf("vtext api: resolve stream owner %s: %v", docID, err)
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
		log.Printf("vtext api: get pending mutation for stream: %v", err)
	}
	writeSSEData(w, vtextDocumentStreamEvent{
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
			streamEvent, ok := vtextStreamEventFromRecord(ev.Record)
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
						log.Printf("vtext api: get document after revision create: %v", err)
						continue
					}
					currentRevisionID = updatedDoc.CurrentRevisionID
				}
				writeSSEData(w, vtextDocumentStreamEvent{
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
				log.Printf("vtext api: get document after synth completion: %v", err)
				continue
			}
			if streamEvent.RevisionID != "" {
				writeSSEData(w, vtextDocumentStreamEvent{
					Kind:              "revision_created",
					DocID:             docID,
					LoopID:            streamEvent.LoopID,
					RevisionID:        streamEvent.RevisionID,
					CurrentRevisionID: updatedDoc.CurrentRevisionID,
				})
			}
			writeSSEData(w, vtextDocumentStreamEvent{
				Kind:              "head_changed",
				DocID:             docID,
				LoopID:            streamEvent.LoopID,
				RevisionID:        streamEvent.RevisionID,
				CurrentRevisionID: updatedDoc.CurrentRevisionID,
			})
		}
	}
}

func vtextStreamEventFromRecord(rec types.EventRecord) (vtextDocumentStreamEvent, bool) {
	var payload map[string]string
	if len(rec.Payload) > 0 {
		if err := json.Unmarshal(rec.Payload, &payload); err != nil {
			return vtextDocumentStreamEvent{}, false
		}
	}

	docID := strings.TrimSpace(payload["doc_id"])
	if docID == "" {
		return vtextDocumentStreamEvent{}, false
	}

	event := vtextDocumentStreamEvent{
		DocID:             docID,
		LoopID:            strings.TrimSpace(payload["loop_id"]),
		RevisionID:        strings.TrimSpace(payload["revision_id"]),
		CurrentRevisionID: strings.TrimSpace(payload["current_revision_id"]),
		Error:             strings.TrimSpace(payload["error"]),
	}
	switch rec.Kind {
	case types.EventVTextAgentRevisionStarted, types.EventVTextAgentRevisionProgress:
		event.Kind = "synth_started"
	case types.EventVTextAgentRevisionCompleted:
		event.Kind = "synth_completed"
	case types.EventVTextAgentRevisionFailed:
		event.Kind = "synth_failed"
	case types.EventVTextDocumentRevisionCreated:
		event.Kind = "revision_created"
	default:
		return vtextDocumentStreamEvent{}, false
	}
	return event, true
}

// HandleVTextDiff handles GET /api/vtext/diff?from={id}&to={id}.
// It compares selected from and to revisions and shows the changed
// sections (VAL-ETEXT-008: diff view compares selected revisions and
// changed sections).
func (h *APIHandler) HandleVTextDiff(w http.ResponseWriter, r *http.Request) {
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

	writeAPIJSON(w, http.StatusOK, vtextDiffResponse{DiffResult: diff})
}

func (h *APIHandler) HandleVTextRestoreRevision(w http.ResponseWriter, r *http.Request) {
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
	var req vtextRestoreRevisionRequest
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
	if err := h.canonicalizeAliasedVTextDocumentTitle(r.Context(), ownerID, &doc, time.Now().UTC()); err != nil {
		log.Printf("vtext api: canonicalize restore document title: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to canonicalize document title"})
		return
	}
	metadata := mergeVTextRevisionMetadata(sourceRev.Metadata, map[string]any{
		"source":                     "restore_historical_revision",
		"restored_from_revision_id":  sourceRev.RevisionID,
		"restore_target_revision_id": doc.CurrentRevisionID,
		"restore_mode":               strings.TrimSpace(req.Mode),
		"draft_line":                 defaultDraftLine(),
	})
	content := sourceRev.Content
	if normalized, changed := markdownstructure.NormalizeTableShapedRows(content); changed {
		content = normalized
		metadata = mergeVTextRevisionMetadata(metadata, map[string]any{
			"vtext_structure_stabilized":        true,
			"vtext_structure_stabilized_reason": "normalized_restored_markdown_table_rows",
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
		log.Printf("vtext api: restore revision: %v", err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "failed to restore revision; document head may have changed"})
		return
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		log.Printf("vtext api: load restored revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load restored revision"})
		return
	}
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(storedRev))
}

func (h *APIHandler) HandleVTextDiagnosis(w http.ResponseWriter, r *http.Request) {
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
	resp := vtextDiagnosisResponse{
		OwnerID:   ownerID,
		DocID:     docID,
		StorePath: h.rt.Store().Path(),
		VTextPath: h.rt.Store().VTextPath(),
	}
	if docID != "" {
		if doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID); err == nil {
			docResp := h.vtextDocumentResponse(r.Context(), doc)
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
		}
	}
	if docID != "" {
		if channelRuns, err := h.rt.Store().ListRunsByChannel(r.Context(), ownerID, docID, limit); err == nil {
			resp.Runs = append(resp.Runs, channelRuns...)
		} else {
			log.Printf("vtext api: list channel diagnosis runs for %s: %v", docID, err)
		}
		if ownerRuns, err := h.rt.Store().ListRunsByOwner(r.Context(), ownerID, diagnosisOwnerRunScanLimit(limit)); err == nil {
			var docRuns []types.RunRecord
			for _, run := range ownerRuns {
				if runRecordBelongsToVTextDoc(run, docID) {
					docRuns = append(docRuns, run)
				}
			}
			resp.Runs = appendUniqueRunRecords(resp.Runs, docRuns...)
		} else {
			log.Printf("vtext api: list owner runs for document diagnosis %s: %v", docID, err)
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

// HandleVTextBlame handles GET /api/vtext/revisions/{id}/blame.
// It provides section-level attribution that distinguishes whether the
// last editor was the user or the agent (VAL-ETEXT-009: blame identifies
// the last editor per section).
func (h *APIHandler) HandleVTextBlame(w http.ResponseWriter, r *http.Request) {
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

	writeAPIJSON(w, http.StatusOK, vtextBlameResponse{BlameResult: blame})
}

// ----- VText revise -----

type testVTextResearchFindingsRequest struct {
	DocID     string                         `json:"doc_id"`
	FindingID string                         `json:"finding_id"`
	Findings  []string                       `json:"findings,omitempty"`
	Evidence  []researchFindingEvidenceInput `json:"evidence,omitempty"`
	Notes     []string                       `json:"notes,omitempty"`
	Questions []string                       `json:"questions,omitempty"`
}

type testVTextWorkerUpdateRequest struct {
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

// HandleTestVTextResearchFindings is a local-only dry-run browser test seam that
// routes through the real researcher tool path instead of inventing a fake
// direct revision shortcut. It is not product proof and must stay disabled
// outside local/test environments.
func (h *APIHandler) HandleTestVTextResearchFindings(w http.ResponseWriter, r *http.Request) {
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

	var req testVTextResearchFindingsRequest
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
		log.Printf("vtext test api: list channel runs: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve vtext agent"})
		return
	}

	var parent *types.RunRecord
	for i := len(runs) - 1; i >= 0; i-- {
		if agentProfileForRun(&runs[i]) == AgentProfileVText {
			parent = &runs[i]
			break
		}
	}
	if parent == nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "vtext agent is not initialized for this document"})
		return
	}

	targetAgentID := strings.TrimSpace(agentIDForRun(parent))
	if targetAgentID == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "vtext agent is missing an agent_id"})
		return
	}

	researcherRun, err := h.rt.StartChildRun(r.Context(), parent.RunID, "Browser test: submit research findings", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    req.DocID,
		"doc_id":                req.DocID,
	})
	if err != nil {
		log.Printf("vtext test api: start researcher run: %v", err)
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

	raw, err := registry.Execute(WithToolExecutionContext(r.Context(), researcherRun), "submit_coagent_update", rawArgs)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Printf("vtext test api: decode tool response: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to encode findings response"})
		return
	}
	resp["loop_id"] = researcherRun.RunID
	writeAPIJSON(w, http.StatusAccepted, resp)
}

// HandleTestVTextWorkerUpdate is a local-only dry-run browser test seam that
// routes through the real structured worker-update tool. It is not product
// proof and stays disabled unless RUNTIME_ENABLE_TEST_APIS is set.
func (h *APIHandler) HandleTestVTextWorkerUpdate(w http.ResponseWriter, r *http.Request) {
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

	var req testVTextWorkerUpdateRequest
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
	targetAgentID := "vtext:" + req.DocID
	if _, err := h.rt.Store().GetAgent(r.Context(), targetAgentID); err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "vtext agent is not initialized for this document"})
		return
	}

	runs, err := h.rt.Store().ListRunsByChannel(r.Context(), ownerID, req.DocID, 50)
	if err != nil {
		log.Printf("vtext test api: list channel runs for worker update: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve vtext agent"})
		return
	}

	var parent *types.RunRecord
	for i := len(runs) - 1; i >= 0; i-- {
		if agentProfileForRun(&runs[i]) == AgentProfileVText {
			parent = &runs[i]
			break
		}
	}
	if parent == nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "vtext agent has no run context for this document"})
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

	workerRun, err := h.rt.StartChildRun(r.Context(), parent.RunID, "Browser test: submit structured worker update", ownerID, map[string]any{
		runMetadataAgentProfile: role,
		runMetadataAgentRole:    role,
		runMetadataAgentID:      role + ":test:" + req.DocID,
		runMetadataChannelID:    req.DocID,
		"doc_id":                req.DocID,
	})
	if err != nil {
		log.Printf("vtext test api: start worker update run: %v", err)
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

	raw, err := registry.Execute(WithToolExecutionContext(r.Context(), workerRun), "submit_coagent_update", rawArgs)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		log.Printf("vtext test api: decode worker update response: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to encode worker update response"})
		return
	}
	resp["loop_id"] = workerRun.RunID
	writeAPIJSON(w, http.StatusAccepted, resp)
}
