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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/sandbox"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	vtextMarkerLineRE          = regexp.MustCompile(`(?im)^.*USER_[A-Z0-9_]*MARKER[A-Z0-9_]*.*$`)
	vtextNumberedHeadingRE     = regexp.MustCompile(`(?m)^\s*(?:#{1,6}\s*)?(\d{1,2}\.\s+[^\n:]{2,100})\s*$`)
	vtextSectionUpdatePrefixRE = regexp.MustCompile(`\bSECTION\s+\d+\s+UPDATE:`)
	vtextSHA256RequirementRE   = regexp.MustCompile(`\b[a-fA-F0-9]{64}\b`)
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
}

type vtextEnsureManifestResponse struct {
	DocID      string `json:"doc_id"`
	SourcePath string `json:"source_path"`
}

type vtextShortcutFile struct {
	Kind       string `json:"kind"`
	DocID      string `json:"doc_id"`
	Title      string `json:"title"`
	SourcePath string `json:"source_path"`
}

// vtextDocumentResponse is the JSON response for GET /api/vtext/documents/{id}.
type vtextDocumentResponse struct {
	DocID             string `json:"doc_id"`
	OwnerID           string `json:"owner_id"`
	Title             string `json:"title"`
	CurrentRevisionID string `json:"current_revision_id,omitempty"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	RevisionCount     int    `json:"revision_count"`
	LastEditor        string `json:"last_editor,omitempty"`
	LastAuthorKind    string `json:"last_author_kind,omitempty"`
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

// vtextBlameResponse is the JSON response for
// GET /api/vtext/revisions/{id}/blame.
type vtextBlameResponse struct {
	types.BlameResult
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

func normalizeVTextSourcePath(raw string) string {
	cleaned := pathpkg.Clean("/" + strings.TrimSpace(raw))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func slugifyVTextManifestStem(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "vtext"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	stem := strings.Trim(b.String(), "-")
	if stem == "" {
		return "vtext"
	}
	return stem
}

func shortDocIDSuffix(docID string) string {
	base := strings.TrimSpace(docID)
	if idx := strings.IndexByte(base, '-'); idx > 0 {
		base = base[:idx]
	}
	if len(base) > 8 {
		base = base[:8]
	}
	if base == "" {
		return "doc"
	}
	return base
}

func isVTextShortcutPath(sourcePath string) bool {
	return strings.EqualFold(pathpkg.Ext(strings.TrimSpace(sourcePath)), ".vtext")
}

func marshalVTextShortcutFile(doc types.Document, sourcePath string) ([]byte, error) {
	return json.MarshalIndent(vtextShortcutFile{
		Kind:       "vtext",
		DocID:      doc.DocID,
		Title:      doc.Title,
		SourcePath: sourcePath,
	}, "", "  ")
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
	doc := types.Document{
		DocID:     uuid.New().String(),
		OwnerID:   ownerID,
		Title:     title,
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
		Content:     req.InitialContent,
		Metadata: json.RawMessage(fmt.Sprintf(`{"source_path":%q,"created_from":"file_open"}`,
			sourcePath,
		)),
		CreatedAt: now,
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
	})
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

func (h *APIHandler) ensureVTextManifest(ctx context.Context, ownerID string, doc types.Document) (string, error) {
	sourcePath, err := h.rt.Store().GetDocumentAliasSourcePath(ctx, ownerID, doc.DocID)
	if err != nil && err != store.ErrNotFound {
		return "", err
	}
	if err == store.ErrNotFound {
		sourcePath, err = h.allocateVTextManifestPath(ctx, ownerID, doc)
		if err != nil {
			return "", err
		}
	}

	content, err := marshalVTextShortcutFile(doc, sourcePath)
	if err != nil {
		return "", fmt.Errorf("marshal vtext shortcut: %w", err)
	}

	filesRoot := sandbox.ResolveFilesRoot("")
	absPath := filepath.Join(filesRoot, filepath.FromSlash(sourcePath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", fmt.Errorf("create manifest directory: %w", err)
	}
	if err := os.WriteFile(absPath, content, 0o644); err != nil {
		return "", fmt.Errorf("write manifest file: %w", err)
	}
	if err := h.rt.Store().UpsertDocumentAlias(ctx, ownerID, sourcePath, doc.DocID, time.Now().UTC()); err != nil {
		return "", err
	}
	return sourcePath, nil
}

func (h *APIHandler) allocateVTextManifestPath(ctx context.Context, ownerID string, doc types.Document) (string, error) {
	stem := slugifyVTextManifestStem(doc.Title)
	suffix := shortDocIDSuffix(doc.DocID)
	candidates := []string{
		fmt.Sprintf("%s.vtext", stem),
		fmt.Sprintf("%s-%s.vtext", stem, suffix),
	}
	filesRoot := sandbox.ResolveFilesRoot("")
	for _, candidate := range candidates {
		docID, err := h.rt.Store().GetDocumentAlias(ctx, ownerID, candidate)
		if err == nil {
			if docID == doc.DocID {
				return candidate, nil
			}
			continue
		}
		if err != store.ErrNotFound {
			return "", err
		}
		absPath := filepath.Join(filesRoot, filepath.FromSlash(candidate))
		if _, statErr := os.Stat(absPath); statErr == nil {
			continue
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		return candidate, nil
	}
	return fmt.Sprintf("%s-%s.vtext", stem, uuid.New().String()[:8]), nil
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
		docResp := vtextDocumentResponse{
			DocID:             doc.DocID,
			OwnerID:           doc.OwnerID,
			Title:             doc.Title,
			CurrentRevisionID: doc.CurrentRevisionID,
			CreatedAt:         doc.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
			UpdatedAt:         doc.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		}
		revs, err := h.rt.Store().ListRevisionsByDoc(r.Context(), doc.DocID, ownerID, 200)
		if err != nil {
			log.Printf("vtext api: list document revisions for recent metadata: %v", err)
		} else {
			docResp.RevisionCount = len(revs)
			if len(revs) > 0 {
				latest := revs[0]
				if doc.CurrentRevisionID != "" {
					for _, rev := range revs {
						if rev.RevisionID == doc.CurrentRevisionID {
							latest = rev
							break
						}
					}
				}
				docResp.LastEditor = latest.AuthorLabel
				docResp.LastAuthorKind = string(latest.AuthorKind)
			}
		}
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

	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
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

	rev := types.Revision{
		RevisionID:       uuid.New().String(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      ownerID,
		Content:          req.Content,
		Citations:        citations,
		Metadata:         metadata,
		ParentRevisionID: parentID,
		CreatedAt:        now,
	}

	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("vtext api: create revision: %v", err)
		if errors.Is(err, store.ErrStaleDocumentHead) {
			if req.AllowRebase && parentID != "" {
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
	h.rt.emitVTextDocumentRevisionEvent(r.Context(), ownerID, rev)

	writeAPIJSON(w, http.StatusCreated, revisionResponseFromRecord(rev))
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
	return rev, nil
}

func rebaseUserDraftContent(baseContent, headContent, userContent, staleParentID string) (string, string, bool) {
	if userContent == baseContent {
		return headContent, "no_user_change", true
	}
	if headContent == baseContent {
		return userContent, "head_unchanged", true
	}
	dmp := diffmatchpatch.New()
	patches := dmp.PatchMake(baseContent, userContent)
	merged, applied := dmp.PatchApply(patches, headContent)
	clean := len(applied) > 0
	for _, ok := range applied {
		if !ok {
			clean = false
			break
		}
	}
	if clean {
		return merged, "diff_match_patch", true
	}
	recovered := strings.TrimRight(headContent, "\n") +
		"\n\n---\n\nRecovered user draft based on revision " + staleParentID + ":\n\n" +
		strings.TrimSpace(userContent) + "\n"
	return recovered, "append_recovered_draft", false
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

func revisionResponseFromRecord(rev types.Revision) vtextRevisionResponse {
	return vtextRevisionResponse{
		RevisionID:       rev.RevisionID,
		DocID:            rev.DocID,
		OwnerID:          rev.OwnerID,
		AuthorKind:       rev.AuthorKind,
		AuthorLabel:      rev.AuthorLabel,
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

	revs, err := h.rt.Store().ListRevisionsByDoc(r.Context(), docID, ownerID, 50)
	if err != nil {
		log.Printf("vtext api: list revisions: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list revisions"})
		return
	}

	resp := vtextListRevisionsResponse{Revisions: make([]vtextRevisionResponse, 0, len(revs))}
	for _, rev := range revs {
		resp.Revisions = append(resp.Revisions, vtextRevisionResponse{
			RevisionID:       rev.RevisionID,
			DocID:            rev.DocID,
			OwnerID:          rev.OwnerID,
			AuthorKind:       rev.AuthorKind,
			AuthorLabel:      rev.AuthorLabel,
			Content:          rev.Content,
			Citations:        rev.Citations,
			Metadata:         rev.Metadata,
			ParentRevisionID: rev.ParentRevisionID,
			CreatedAt:        rev.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		})
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
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "revision not found"})
		return
	}

	writeAPIJSON(w, http.StatusOK, vtextRevisionResponse{
		RevisionID:       rev.RevisionID,
		DocID:            rev.DocID,
		OwnerID:          rev.OwnerID,
		AuthorKind:       rev.AuthorKind,
		AuthorLabel:      rev.AuthorLabel,
		Content:          rev.Content,
		Citations:        rev.Citations,
		Metadata:         rev.Metadata,
		ParentRevisionID: rev.ParentRevisionID,
		CreatedAt:        rev.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
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

	entries, err := h.rt.Store().GetHistory(r.Context(), docID, ownerID, 50)
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

	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
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

	pendingMutation, err := h.pendingAgentMutationByDoc(r.Context(), docID, ownerID)
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

// vtextAgentRevisionRequest is the JSON payload for
// POST /api/vtext/documents/{id}/revise.
// Submitting a natural-language revision request from within an open document
// creates a new canonical revision attributable to the appagent
// (VAL-ETEXT-003).
type vtextAgentRevisionRequest struct {
	Intent string `json:"intent,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

// vtextAgentRevisionResponse is the JSON response for agent revision
// submission. It returns the stable task handle so runtime/trace surfaces can
// correlate the mutation even though the editor now follows the document stream
// instead of polling the run directly (VAL-ETEXT-004).
type vtextAgentRevisionResponse struct {
	RunID     string         `json:"loop_id"`
	DocID     string         `json:"doc_id"`
	State     types.RunState `json:"state"`
	CreatedAt string         `json:"created_at"`
}

type vtextCancelRevisionResponse struct {
	DocID           string   `json:"doc_id"`
	RunID           string   `json:"loop_id,omitempty"`
	Status          string   `json:"status"`
	CancelledRunIDs []string `json:"cancelled_loop_ids,omitempty"`
	Resumable       bool     `json:"resumable"`
}

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

// HandleVTextAgentRevision handles POST
// /api/vtext/documents/{id}/revise.
//
// It creates a runtime task that, when completed, will create a canonical
// appagent-authored revision. The task ID is returned so the client can
// track progress and completion through the existing event stream
// (VAL-ETEXT-003, VAL-ETEXT-004).
//
// If a pending agent mutation already exists for this document (e.g., from
// a previous request that is still in-flight), the existing task ID is
// returned instead of creating a new mutation, preventing duplicate
// canonical revisions when renewal/retry occurs mid-mutation
// (VAL-CROSS-122).
func (h *APIHandler) HandleVTextAgentRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	var req vtextAgentRevisionRequest
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

	// Check for an existing pending agent mutation on this document.
	// If one exists, return the existing run ID instead of creating a new
	// mutation. This prevents duplicate canonical revisions when
	// renewal/retry occurs mid-mutation (VAL-CROSS-122).
	existing, err := h.pendingAgentMutationByDoc(r.Context(), docID, ownerID)
	if err != nil {
		log.Printf("vtext api: check pending mutation: %v", err)
	} else if existing != nil {
		// Return the existing run — idempotent response.
		writeAPIJSON(w, http.StatusAccepted, vtextAgentRevisionResponse{
			RunID:     existing.RunID,
			DocID:     docID,
			State:     types.RunPending,
			CreatedAt: existing.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		})
		return
	}

	rec, err := h.rt.submitVTextAgentRevisionRun(r.Context(), doc, ownerID, req, "", 0)
	if err != nil {
		log.Printf("vtext api: submit agent revision run: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to submit agent revision"})
		return
	}

	writeAPIJSON(w, http.StatusAccepted, vtextAgentRevisionResponse{
		RunID:     rec.RunID,
		DocID:     docID,
		State:     rec.State,
		CreatedAt: rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleVTextCancelAgentRevision handles POST
// /api/vtext/documents/{id}/cancel. It cancels the pending VText appagent
// revision graph without changing the canonical document head.
func (h *APIHandler) HandleVTextCancelAgentRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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
	if _, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	mutation, err := h.pendingAgentMutationByDoc(r.Context(), docID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusOK, vtextCancelRevisionResponse{DocID: docID, Status: "no_pending_revision", Resumable: true})
			return
		}
		log.Printf("vtext api: get pending mutation for cancel: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load pending revision"})
		return
	}
	if mutation == nil {
		writeAPIJSON(w, http.StatusOK, vtextCancelRevisionResponse{DocID: docID, Status: "no_pending_revision", Resumable: true})
		return
	}
	cancelled, err := h.rt.CancelRunGraph(r.Context(), mutation.RunID, ownerID)
	if err != nil {
		log.Printf("vtext api: cancel revision graph: %v", err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	if err := h.rt.Store().CancelAgentMutation(r.Context(), mutation.RunID); err != nil {
		log.Printf("vtext api: mark mutation cancelled: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record cancellation"})
		return
	}
	writeAPIJSON(w, http.StatusOK, vtextCancelRevisionResponse{
		DocID:           docID,
		RunID:           mutation.RunID,
		Status:          "cancelled",
		CancelledRunIDs: cancelled,
		Resumable:       true,
	})
}

func (h *APIHandler) pendingAgentMutationByDoc(ctx context.Context, docID, ownerID string) (*store.AgentMutation, error) {
	mutation, err := h.rt.Store().GetPendingAgentMutationByDoc(ctx, docID, ownerID)
	if err != nil || mutation == nil {
		return mutation, err
	}
	run, err := h.rt.GetRun(ctx, mutation.RunID, ownerID)
	if err != nil {
		return mutation, nil
	}
	if !run.State.Terminal() {
		return mutation, nil
	}
	if err := h.rt.Store().MarkAgentMutationStale(ctx, mutation.RunID); err != nil {
		log.Printf("vtext api: mark stale pending mutation %s: %v", mutation.RunID, err)
		return mutation, nil
	}
	return nil, nil
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

func (rt *Runtime) submitVTextAgentRevisionRun(ctx context.Context, doc types.Document, ownerID string, req vtextAgentRevisionRequest, parentRunID string, scheduledMessageSeq int64) (*types.RunRecord, error) {
	// Build the backend-owned vtext revision request from current document state.
	var currentRevision types.Revision
	var currentRevisionLoaded bool
	if doc.CurrentRevisionID != "" {
		rev, err := rt.Store().GetRevision(ctx, doc.CurrentRevisionID, ownerID)
		if err == nil {
			currentRevision = rev
			currentRevisionLoaded = true
		}
	}
	metadata := decodeRevisionMetadata(currentRevision.Metadata)
	var previousRevision *types.Revision
	if currentRevisionLoaded && currentRevision.ParentRevisionID != "" {
		prev, err := rt.Store().GetRevision(ctx, currentRevision.ParentRevisionID, ownerID)
		if err == nil {
			previousRevision = &prev
		}
	}

	diffSummary := ""
	if currentRevisionLoaded && previousRevision != nil {
		if diff, err := rt.Store().GetDiff(ctx, previousRevision.RevisionID, currentRevision.RevisionID, ownerID); err == nil {
			diffSummary = summarizeDiffResult(diff)
		}
	}

	hasGroundedHistory, historyErr := rt.channelHasGroundedHistory(ctx, ownerID, doc.DocID, time.Time{})
	if historyErr != nil {
		log.Printf("vtext api: check grounded history: %v", historyErr)
		hasGroundedHistory = false
	}

	recentWorkerMessages, workerErr := rt.recentWorkerMessages(ctx, ownerID, doc.DocID, 12)
	if workerErr != nil {
		log.Printf("vtext api: recent worker messages: %v", workerErr)
	}
	userRevisionDiffs, userDiffErr := rt.userRevisionDiffSummaries(ctx, ownerID, doc.DocID, 200)
	if userDiffErr != nil {
		log.Printf("vtext api: user revision diffs: %v", userDiffErr)
	}
	allowsUngroundedCreativeDraft := vtextPromptAllowsUngroundedCreativeDraft(req.Prompt)
	requiresWorkerGrounding := vtextRevisionRequiresWorkerGrounding(hasGroundedHistory, currentRevision.AuthorKind, allowsUngroundedCreativeDraft)

	agentPrompt := buildAgentRevisionRequest(currentRevision, previousRevision, metadata, req, diffSummary, hasGroundedHistory, allowsUngroundedCreativeDraft, recentWorkerMessages, userRevisionDiffs)

	// Create the runtime run with vtext agent revision metadata.
	// Carry forward durable context keys from the current head revision
	// so they survive into appagent revision metadata.
	runMetadata := map[string]any{
		"type":                      "vtext_agent_revision",
		"agent_profile":             AgentProfileVText,
		"agent_role":                AgentProfileVText,
		"agent_id":                  "vtext:" + doc.DocID,
		"channel_id":                doc.DocID,
		"doc_id":                    doc.DocID,
		"current_revision_id":       doc.CurrentRevisionID,
		"request_intent":            strings.TrimSpace(req.Intent),
		"original_prompt":           strings.TrimSpace(req.Prompt),
		"requires_worker_grounding": requiresWorkerGrounding,
	}
	if allowsUngroundedCreativeDraft {
		runMetadata["ungrounded_generation_scope"] = "creative_draft"
	}
	if scheduledMessageSeq > 0 {
		runMetadata["scheduled_message_seq"] = scheduledMessageSeq
	}
	for _, key := range durableMetadataKeys {
		if val := metadataString(metadata, key); val != "" {
			runMetadata[key] = val
		}
	}
	if strings.TrimSpace(parentRunID) == "" {
		if conductorLoopID := metadataString(metadata, "conductor_loop_id"); conductorLoopID != "" {
			if conductorRun, err := rt.store.GetRun(ctx, conductorLoopID); err == nil && conductorRun.OwnerID == ownerID {
				parentRunID = conductorRun.RunID
			}
		}
	}

	var (
		rec *types.RunRecord
		err error
	)
	if strings.TrimSpace(parentRunID) != "" {
		rec, err = rt.StartChildRun(ctx, parentRunID, agentPrompt, ownerID, runMetadata)
	} else {
		rec, err = rt.StartRunWithMetadata(ctx, agentPrompt, ownerID, runMetadata)
	}
	if err != nil {
		return nil, err
	}

	// Record the agent mutation for idempotency tracking (VAL-CROSS-122).
	if err := rt.Store().CreateAgentMutation(ctx, store.AgentMutation{
		DocID:               doc.DocID,
		RunID:               rec.RunID,
		OwnerID:             ownerID,
		State:               "pending",
		ScheduledMessageSeq: scheduledMessageSeq,
		CreatedAt:           time.Now().UTC(),
	}); err != nil {
		log.Printf("vtext api: create agent mutation: %v", err)
	}

	// Emit the vtext-specific agent revision started event.
	startedPayload, _ := json.Marshal(map[string]string{
		"doc_id":  doc.DocID,
		"loop_id": rec.RunID,
	})
	rt.emitVTextAgentEvent(ctx, rec, types.EventVTextAgentRevisionStarted,
		events.CauseTaskLifecycle, startedPayload)

	return rec, nil
}

func vtextPromptAllowsUngroundedCreativeDraft(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	creativeMarkers := []string{
		"tell me a story",
		"tell me a short story",
		"write a story",
		"write a short story",
		"make up a story",
		"draft a story",
		"draft a short story",
		"short story",
		"fictional story",
		"fiction",
		"write one short sentence",
		"write a short sentence",
		"write one sentence",
		"write a poem",
		"compose a poem",
		"write fiction",
		"make up",
		"bedtime story",
	}
	hasCreativeMarker := false
	for _, marker := range creativeMarkers {
		if strings.Contains(text, marker) {
			hasCreativeMarker = true
			break
		}
	}
	if !hasCreativeMarker {
		return false
	}
	factualMarkers := []string{
		"latest",
		"current",
		"now",
		"today",
		"yesterday",
		"news",
		"research",
		"cite",
		"citation",
		"source",
		"sources",
		"look up",
		"what's",
		"what is",
		"who ",
		"when ",
		"where ",
		"why ",
		"how ",
	}
	for _, marker := range factualMarkers {
		if strings.Contains(text, marker) {
			return false
		}
	}
	return true
}

func vtextRevisionRequiresWorkerGrounding(hasGroundedHistory bool, authorKind types.AuthorKind, allowsUngroundedCreativeDraft bool) bool {
	return !hasGroundedHistory && authorKind != types.AuthorUser && !allowsUngroundedCreativeDraft
}

func vtextHardRequirementHints(parts ...string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, part := range parts {
		text := strings.TrimSpace(part)
		if text == "" {
			continue
		}
		for _, match := range vtextMarkerLineRE.FindAllString(text, -1) {
			add("Preserve exact marker line: " + truncatePromptSnippet(match, 180))
		}
		for _, match := range vtextSectionUpdatePrefixRE.FindAllString(text, -1) {
			add("Required sentence prefix: " + strings.Join(strings.Fields(match), " "))
		}
		for _, match := range vtextNumberedHeadingRE.FindAllStringSubmatch(text, -1) {
			if len(match) > 1 {
				add("Required numbered heading: " + strings.TrimSpace(match[1]))
			}
		}
		for _, match := range vtextSHA256RequirementRE.FindAllString(text, -1) {
			add("Required hash/value: " + match)
		}
		for _, label := range []string{"[S1]", "[S2]", "[S3]"} {
			if strings.Contains(text, label) {
				add("Required evidence label: " + label)
			}
		}
		if strings.Contains(text, "[CMD]") {
			add("Final command evidence label: [CMD] (final-only: include it only after a super delivery reports command evidence or a precise execution blocker; do not use it for initial scaffolds, pending source ledger rows, requested state, target hashes, or placeholders)")
		}
	}
	if len(out) > 32 {
		return out[:32]
	}
	return out
}

// buildAgentRevisionRequest constructs the backend-owned vtext revision
// request sent as the user turn for the vtext appagent.
func buildAgentRevisionRequest(current types.Revision, previous *types.Revision, metadata map[string]any, req vtextAgentRevisionRequest, diffSummary string, hasGroundedHistory bool, allowsUngroundedCreativeDraft bool, recentWorkerMessages []ChannelMessage, userRevisionDiffs []string) string {
	var b strings.Builder
	b.WriteString("A revise event was triggered for the current vtext document.")

	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "revise"
	}
	b.WriteString("\nIntent: ")
	b.WriteString(intent)
	b.WriteString(".")

	if seedPrompt := metadataString(metadata, "seed_prompt"); seedPrompt != "" {
		b.WriteString("\n\nOriginal user request:\n")
		b.WriteString(seedPrompt)
	}
	if legacyPrompt := strings.TrimSpace(req.Prompt); legacyPrompt != "" {
		b.WriteString("\n\nAdditional user instruction:\n")
		b.WriteString(legacyPrompt)
	}
	if sourcePath := metadataString(metadata, "source_path"); sourcePath != "" {
		b.WriteString("\n\nSource path: ")
		b.WriteString(sourcePath)
		b.WriteString(". Preserve the file-backed structure while producing the next version.")
	}
	if conductorLoopID := metadataString(metadata, "conductor_loop_id"); conductorLoopID != "" {
		b.WriteString("\nConductor loop: ")
		b.WriteString(conductorLoopID)
		b.WriteString(".")
	}
	if current.RevisionID != "" {
		b.WriteString("\n\nCurrent head revision: ")
		b.WriteString(current.RevisionID)
		b.WriteString(" (")
		b.WriteString(string(current.AuthorKind))
		b.WriteString(" by ")
		b.WriteString(current.AuthorLabel)
		b.WriteString(").")
	}
	if previous != nil {
		b.WriteString("\nPrevious revision: ")
		b.WriteString(previous.RevisionID)
		b.WriteString(".")
	}
	if diffSummary != "" {
		b.WriteString("\n\nLatest revision diff/context:\n")
		b.WriteString(diffSummary)
	}
	if len(recentWorkerMessages) > 0 {
		b.WriteString("\n\nRecent addressed worker messages:\n")
		for _, message := range recentWorkerMessages {
			b.WriteString("- [")
			if !message.Timestamp.IsZero() {
				b.WriteString(message.Timestamp.UTC().Format(time.RFC3339))
			} else {
				b.WriteString("unknown-time")
			}
			b.WriteString("] ")
			if role := strings.TrimSpace(message.Role); role != "" {
				b.WriteString(role)
			} else {
				b.WriteString("worker")
			}
			if from := strings.TrimSpace(message.From); from != "" {
				b.WriteString(" ")
				b.WriteString(from)
			}
			b.WriteString(": ")
			b.WriteString(truncatePromptSnippet(message.Content, 800))
			b.WriteString("\n")
		}
		if strings.EqualFold(intent, "integrate_worker_findings") && !vtextPromptNeedsSuperExecution(metadataString(metadata, "seed_prompt")+" "+req.Prompt) {
			b.WriteString("\nThis VText run was woken by worker findings. Make those findings visible with edit_vtext as this turn's next document revision before spawning additional workers.")
			b.WriteString("\nIf the worker evidence is partial, blocked, or inconclusive, still write an honest partial/blocker checkpoint instead of leaving the visible document at the pre-findings state.")
			b.WriteString("\nOnly spawn another researcher before editing if the worker message is unusable for any visible checkpoint; if so, name the precise blocker in the run output.")
		}
		if vtextPromptNeedsSuperExecution(metadataString(metadata, "seed_prompt")+" "+req.Prompt) && !vtextWorkerMessagesContainRole(recentWorkerMessages, AgentProfileSuper) {
			b.WriteString("\nThe original request still has an execution/code/browser/verification obligation, but these recent worker messages do not include a super delivery.")
			b.WriteString("\nThis VText turn's next side-effectful action should be request_super_execution before another source-only edit, unless you can name a precise blocker.")
			b.WriteString("\nDo not attempt a full-document rewrite in this worker-wake turn before the super request exists. Keep the request_super_execution objective concise and concrete so the next visible revision can integrate both research and command/artifact evidence.")
			b.WriteString("\nA source-grounded revision may still say command evidence is pending, but it must not use the final [CMD] evidence label before the super delivery arrives.")
		}
		if workerMessagesContainActiveDelegation(recentWorkerMessages) {
			b.WriteString("\nAt least one recent worker message says a delegated worker is still active or lacks terminal evidence.")
			b.WriteString("\nFor this case, write the next dashboard revision from the evidence and call request_super_execution with a concrete continuation objective for persistent super.")
			b.WriteString("\nThe objective must tell super to continue the existing worker_run_id, not start a duplicate worker, and to observe, redirect, cancel, or finish only through super authority until there is an AppChangePackage, reviewable blocker, cancellation certificate, or bounded timeout certificate.")
			b.WriteString("\nVText may ask for clarification or continuation; VText must not directly control worker/vsuper/co-super runs.")
		}
	}
	if len(userRevisionDiffs) > 0 {
		b.WriteString("\nUser-authored revision diffs (oldest to newest):\n")
		for _, summary := range userRevisionDiffs {
			b.WriteString("- ")
			b.WriteString(summary)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\nCurrent canonical document content:\n---\n")
	if current.Content != "" {
		b.WriteString(current.Content)
	} else {
		b.WriteString("(empty document)")
	}
	b.WriteString("\n---\n")
	hardRequirements := vtextHardRequirementHints(metadataString(metadata, "seed_prompt"), req.Prompt, current.Content)
	hasSuperDelivery := vtextWorkerMessagesContainRole(recentWorkerMessages, AgentProfileSuper)
	if !hasSuperDelivery {
		hardRequirements = vtextFilterFinalCommandEvidenceRequirements(hardRequirements)
		if strings.Contains(metadataString(metadata, "seed_prompt")+req.Prompt+current.Content, "[CMD]") {
			hardRequirements = append(hardRequirements, "Pending command evidence rule: before a super delivery exists, do not include a Source Ledger row, status row, or placeholder whose label is [CMD]; describe command evidence as pending without that label.")
		}
	}
	if len(hardRequirements) > 0 {
		b.WriteString("\nHard requirements checklist for the next canonical revision:\n")
		for _, requirement := range hardRequirements {
			b.WriteString("- ")
			b.WriteString(requirement)
			b.WriteString("\n")
		}
		b.WriteString("Treat this checklist as acceptance criteria for any replace_all edit; preserve these prefixes, labels, values, and headings verbatim unless the user explicitly changed them.\n")
	}
	if current.AuthorKind == types.AuthorUser {
		b.WriteString("\nTreat this latest user-authored revision as the canonical input for the next version.")
		b.WriteString("\nBecause VText owns the document, write the first useful owner-readable revision with edit_vtext before opening longer worker work.")
		b.WriteString("\nFor greetings or simple non-factual prompts, answer directly and do not open workers.")
		b.WriteString("\nFor factual/current/search requests, the first revision should be a short working brief with explicit uncertainty and no ungrounded claims, followed by a researcher spawn in the same run.")
		b.WriteString("\nFor coding/execution requests, the first revision should state the objective and evidence plan, followed by request_super_execution in the same run.")
		b.WriteString("\nIf execution evidence is still pending in an initial or interim revision, do not include the final [CMD] evidence label yet; describe pending command evidence without that label.")
	}
	if hasGroundedHistory {
		b.WriteString("\nThis document already has grounded workflow history on the coordination channel.")
		b.WriteString("\nReuse the informed context already present in the current document and prior worker messages.")
		b.WriteString("\nOpen new researcher work when this follow-up needs facts or evidence beyond what the workflow has already grounded.")
		b.WriteString("\nUse request_super_execution when the follow-up needs generated artifacts, execution, or verification.")
		b.WriteString("\nIf recent worker findings are only partial and the document needs more evidence, write an honest partial revision first unless there is no usable checkpoint at all. A later turn can open the next focused research branch. Do not write that a follow-up researcher was dispatched, requested, or will return unless a spawn_agent call actually succeeds in this turn or the recent worker messages already show that worker.")
	} else {
		b.WriteString("\nThis document does not yet have grounded workflow history.")
		if current.AuthorKind == types.AuthorUser {
			b.WriteString("\nYou may edit user-provided text for structure, clarity, or formatting.")
			b.WriteString("\nDo not add factual claims, citations, or coding results from model priors.")
			b.WriteString("\nIf the request needs facts, current events, citations, generated artifacts, execution, or verification, write a brief working revision first, then start the needed worker request before ending the run.")
		} else if allowsUngroundedCreativeDraft {
			b.WriteString("\nThe current conductor seed is for a creative/non-factual draft.")
			b.WriteString("\nYou may call edit_vtext to produce the requested creative document without worker grounding.")
			b.WriteString("\nDo not spawn researcher or request super for this creative draft unless the user asks for factual grounding, current events, citations, code execution, product mutation, or verification.")
			b.WriteString("\nDo not add factual, current-events, citation, coding, or product claims unless worker evidence exists.")
		} else {
			b.WriteString("\nDo not call edit_vtext with factual claims from model priors.")
			b.WriteString("\nFor factual/current claims, write a brief working revision with explicit uncertainty, then call spawn_agent with role=\"researcher\" on this document channel. Use parallel researchers when you can give each one a distinct branch; otherwise start with one broad researcher.")
			b.WriteString("\nOrdinary factual, current-events, web, or \"what is going on now\" questions are research work, not super work. Do not route them to request_super_execution unless the user also asks for code execution, product mutation, candidate-world work, or verifier contracts.")
			b.WriteString("\nFor coding, generated artifacts, execution, or verification, call request_super_execution.")
			b.WriteString("\nAfter starting the necessary worker request(s), keep the interim revision short: name the objective, worker type, evidence being gathered, and next expected revision. Worker deliveries will wake later VText runs to create evidence-backed revisions.")
		}
	}
	b.WriteString("\nTreat this run as one step in an ongoing document loop.")
	b.WriteString("\nWorker messages can wake later vtext runs and trigger the next revision.")
	b.WriteString("\nPrefer prompt-to-v1 speed and small subsequent revisions over waiting for exhaustive coverage.")
	b.WriteString("\nWhen worker findings arrive, update the document as soon as the first packet can improve it; do not wait for every researcher or super thread to finish.")
	b.WriteString("\nException: if the original request also asked for command output, code execution, generated artifacts, browser proof, or verification and no super delivery has returned that evidence, first call request_super_execution. Keep that request small and concrete; do not attempt a full-document rewrite before the super request exists. Do not spend a worker-wake turn only improving source text while that execution obligation has no super request. Do not make a source-grounded edit look final for `[CMD]`, command output, artifacts, or verification before super evidence arrives.")
	b.WriteString("\nNever use `[CMD]` as a pending/requested/target-only label, including in the initial v1 scaffold, source ledger, status table, or placeholder. If command evidence is still pending, write \"command evidence pending\" without the `[CMD]` marker. Use `[CMD]` only when a super delivery reports the actual command result or precise execution blocker.")
	b.WriteString("\nNever describe coordination as already done unless the tool action really happened. Phrases such as \"researcher dispatched\", \"follow-up researcher requested\", \"will include once targeted research returns\", or \"super has been asked\" are only allowed after the corresponding spawn_agent or request_super_execution tool call succeeded, or when a recent worker message proves that worker is active. If you only edit_vtext, phrase remaining work as \"next needed\" or \"still unresolved\" instead of as a completed delegation.")
	b.WriteString("\nBuild from the current canonical document, recent worker messages, recent change context, and user-authored diffs.")
	b.WriteString("\nIntermediate appagent revisions are compactable context, not the source of truth.")
	b.WriteString("\nPreserve explicit hard requirements from the original user request and current document across every revision. These include exact marker strings, required headings or section counts, required labels or sentence prefixes, requested source labels, command strings, target hashes, and text the user said to preserve.")
	b.WriteString("\nBefore a replace_all edit, audit the complete replacement against those hard requirements. Do not replace a requested numbered/sectioned document with a different report outline unless the user explicitly changed the structure.")
	b.WriteString("\nDo not answer knowledge or coding requests from model weights. Depend on researcher messages for knowledge and super messages for coding/execution/verification.")
	b.WriteString("\nDo not claim to be researching unless you actually open worker runs and incorporate their messages.")
	b.WriteString("\nTo create the next canonical document version, call edit_vtext. Provider final text is not a document write path.")
	b.WriteString("\nFor a precise edit against the current head, call edit_vtext with:")
	b.WriteString("\n{\"doc_id\":\"")
	b.WriteString(current.DocID)
	b.WriteString("\",\"base_revision_id\":\"")
	b.WriteString(current.RevisionID)
	b.WriteString("\",\"operation\":\"apply_edits\",\"edits\":[{\"op\":\"replace\",\"find\":\"exact previous text\",\"replace\":\"new text\"}]}")
	b.WriteString("\nA replace edit must match exactly once. If the same find text appears multiple times and every occurrence should change, set \"replace_all\":true on that edit.")
	b.WriteString("\nUse {\"op\":\"append\",\"text\":\"section text\"} to append new material when appropriate.")
	b.WriteString("\nIf a full replacement is clearer, call edit_vtext with {\"doc_id\":\"")
	b.WriteString(current.DocID)
	b.WriteString("\",\"base_revision_id\":\"")
	b.WriteString(current.RevisionID)
	b.WriteString("\",\"operation\":\"replace_all\",\"content\":\"complete current-state document\"}.")
	b.WriteString("\nIf you end the run without edit_vtext, no canonical document revision will be created.")
	return b.String()
}

func vtextFilterFinalCommandEvidenceRequirements(requirements []string) []string {
	if len(requirements) == 0 {
		return requirements
	}
	filtered := requirements[:0]
	for _, requirement := range requirements {
		if strings.HasPrefix(requirement, "Final command evidence label: [CMD]") {
			continue
		}
		filtered = append(filtered, requirement)
	}
	return filtered
}

func vtextWorkerMessagesContainRole(messages []ChannelMessage, role string) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return false
	}
	for _, message := range messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), role) {
			return true
		}
	}
	return false
}

func workerMessagesContainActiveDelegation(messages []ChannelMessage) bool {
	for _, message := range messages {
		content := strings.ToLower(message.Content)
		for _, marker := range []string{
			"worker_run_active",
			"finish_ready=false",
			"finish_ready: false",
			"active_worker_obligation=true",
			"runtime supervision continuation required",
			"missing_terminal_evidence",
		} {
			if strings.Contains(content, marker) {
				return true
			}
		}
	}
	return false
}

func (rt *Runtime) recentWorkerMessages(ctx context.Context, ownerID, channelID string, limit int) ([]ChannelMessage, error) {
	if limit <= 0 {
		limit = 12
	}
	messages, err := rt.Store().ListChannelMessages(ctx, ownerID, channelID, 0, 200)
	if err != nil {
		return nil, err
	}
	runs, err := rt.Store().ListRunsByChannel(ctx, ownerID, channelID, 200)
	if err != nil {
		return nil, err
	}
	runProfiles := make(map[string]string, len(runs))
	for _, run := range runs {
		runProfiles[run.RunID] = agentProfileForRun(&run)
	}
	filtered := make([]ChannelMessage, 0, len(messages))
	targetAgentID := "vtext:" + strings.TrimSpace(channelID)
	for _, message := range messages {
		if strings.TrimSpace(message.ToAgentID) != targetAgentID {
			continue
		}
		switch runProfiles[strings.TrimSpace(message.FromRunID)] {
		case AgentProfileResearcher, AgentProfileSuper, AgentProfileCoSuper:
			filtered = append(filtered, message)
		}
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered, nil
}

func (rt *Runtime) userRevisionDiffSummaries(ctx context.Context, ownerID, docID string, limit int) ([]string, error) {
	revs, err := rt.Store().ListRevisionsByDoc(ctx, docID, ownerID, limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]string, 0, len(revs))
	for i := len(revs) - 1; i >= 0; i-- {
		rev := revs[i]
		if rev.AuthorKind != types.AuthorUser {
			continue
		}
		label := rev.CreatedAt.UTC().Format(time.RFC3339)
		if rev.ParentRevisionID == "" {
			summaries = append(summaries, fmt.Sprintf("%s %s: initial user-authored draft", rev.RevisionID, label))
			continue
		}
		diff, err := rt.Store().GetDiff(ctx, rev.ParentRevisionID, rev.RevisionID, ownerID)
		if err != nil {
			continue
		}
		summaries = append(summaries, fmt.Sprintf("%s %s: %s", rev.RevisionID, label, summarizeDiffResult(diff)))
	}
	return summaries, nil
}

func truncatePromptSnippet(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}

func decodeRevisionMetadata(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func summarizeDiffResult(diff types.DiffResult) string {
	if len(diff.Sections) == 0 {
		return "No line-level changes were detected."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Added lines: %d. Removed lines: %d.", diff.AddedLines, diff.RemovedLines))
	changesShown := 0
	for _, section := range diff.Sections {
		if section.Type == "unchanged" {
			continue
		}
		if changesShown >= 4 {
			b.WriteString("\n- Additional changed sections omitted for brevity.")
			break
		}
		var snippet string
		switch section.Type {
		case "added":
			snippet = strings.TrimSpace(section.ToContent)
		case "removed":
			snippet = strings.TrimSpace(section.FromContent)
		default:
			snippet = strings.TrimSpace(section.ToContent)
			if snippet == "" {
				snippet = strings.TrimSpace(section.FromContent)
			}
		}
		if snippet == "" {
			snippet = "(empty change block)"
		}
		if len(snippet) > 240 {
			snippet = snippet[:240] + "..."
		}
		b.WriteString("\n- ")
		b.WriteString(section.Type)
		b.WriteString(": ")
		b.WriteString(snippet)
		changesShown++
	}
	return b.String()
}

// emitVTextAgentEvent is a helper that emits an vtext-specific agent revision
// event, carrying the doc_id in the payload so the frontend can correlate
// progress to the open document (VAL-ETEXT-004).
func (rt *Runtime) emitVTextAgentEvent(ctx context.Context, rec *types.RunRecord, kind types.EventKind, cause events.EventCause, payload json.RawMessage) {
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Payload:      payload,
	}
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist vtext agent event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  cause,
	})
}

func (rt *Runtime) emitVTextDocumentRevisionEvent(ctx context.Context, ownerID string, rev types.Revision) {
	payload, err := json.Marshal(map[string]string{
		"doc_id":              rev.DocID,
		"revision_id":         rev.RevisionID,
		"current_revision_id": rev.RevisionID,
	})
	if err != nil {
		log.Printf("runtime: marshal vtext document revision event: %v", err)
		return
	}
	evRec := &types.EventRecord{
		EventID:   uuid.New().String(),
		OwnerID:   ownerID,
		Timestamp: time.Now().UTC(),
		Kind:      types.EventVTextDocumentRevisionCreated,
		Payload:   payload,
	}
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist vtext document revision event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  events.CauseTaskLifecycle,
	})
}

func (rt *Runtime) emitVTextDocumentRevisionEventForRun(ctx context.Context, rec *types.RunRecord, rev types.Revision) {
	if rec == nil {
		rt.emitVTextDocumentRevisionEvent(ctx, rev.OwnerID, rev)
		return
	}
	payload, err := json.Marshal(map[string]string{
		"doc_id":              rev.DocID,
		"revision_id":         rev.RevisionID,
		"current_revision_id": rev.RevisionID,
		"loop_id":             rec.RunID,
	})
	if err != nil {
		log.Printf("runtime: marshal vtext document revision event: %v", err)
		return
	}
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rev.DocID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventVTextDocumentRevisionCreated,
		Payload:      payload,
	}
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist vtext document revision event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  events.CauseTaskLifecycle,
	})
}
