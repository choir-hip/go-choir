// Package api implements the REST API for Choir Base items and blobs. It
// sits on top of the journal/tree packages (M3) and the blob store (this
// mission) and authenticates every request with an API key Bearer token
// (M1).
//
// Endpoints:
//
//	POST /api/base/blobs            — upload a blob (returns BlobRef)
//	GET  /api/base/blobs/{ref}      — download raw blob bytes by BlobRef
//	POST /api/base/items            — create/update an item (journal Event)
//	GET  /api/base/items/{id}       — get item at current state
//	GET  /api/base/delta?cursor=... — get events since cursor
//	GET  /api/base/items/{id}/status — get sync status for item
//	POST /api/base/repair/preview   — preview repair actions (planner)
//
// Auth: every endpoint requires either trusted proxy identity headers or an
// API key Bearer token. GET endpoints require the read:base scope; POST
// endpoints require write:base. Each mutation creates a journal Event with
// SubjectID set to the authenticated user's ID.
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
	"github.com/yusefmosiah/go-choir/internal/base/tree"
)

// Scope constants enforced by the API.
const (
	ScopeReadBase  = "read:base"
	ScopeWriteBase = "write:base"
)

const maxBlobUploadBytes = 64 << 20

// APIKeyValidator is the interface used to validate Bearer token (API key)
// auth. It mirrors the subset of *auth.Store the API needs, so tests can
// inject a mock without opening a SQLite database.
type APIKeyValidator interface {
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*auth.APIKey, error)
	TouchAPIKeyLastUsed(ctx context.Context, keyID string) error
	GetUserByID(id string) (*auth.User, error)
}

// Handler provides HTTP handlers for the /api/base/* routes.
type Handler struct {
	blobs     *blob.Store
	jr        journal.Journal
	validator APIKeyValidator
	now       func() time.Time // injectable clock for tests
}

// NewHandler creates a Base API handler with the given blob store, journal,
// and API key validator. The clock defaults to time.Now().UTC() when nil.
func NewHandler(blobs *blob.Store, jr journal.Journal, v APIKeyValidator) *Handler {
	return &Handler{
		blobs:     blobs,
		jr:        jr,
		validator: v,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// SetClock replaces the internal clock used for event timestamps. Intended
// for tests.
func (h *Handler) SetClock(now func() time.Time) {
	if now != nil {
		h.now = now
	}
}

// Routes returns an *http.ServeMux with all Base API endpoints registered.
// The mux uses Go 1.22+ method-prefixed patterns so method enforcement is
// handled by the router itself.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/base/blobs", h.handlePutBlob)
	mux.HandleFunc("GET /api/base/blobs/{ref}", h.handleGetBlob)
	mux.HandleFunc("POST /api/base/items", h.handlePutItem)
	mux.HandleFunc("GET /api/base/items/{id}", h.handleGetItem)
	mux.HandleFunc("GET /api/base/items/{id}/status", h.handleGetStatus)
	mux.HandleFunc("GET /api/base/delta", h.handleDelta)
	mux.HandleFunc("POST /api/base/repair/preview", h.handleRepairPreview)
	return mux
}

// --- auth ----------------------------------------------------------------

// authResult holds the authenticated identity for a request.
type authResult struct {
	UserID string
	Email  string
	KeyID  string
	Scopes []string
}

// authenticate returns the proxy-validated identity when the request has
// already crossed the proxy trust boundary, otherwise validates a direct
// Bearer token API key. The proxy path is necessary because the public edge
// strips raw Authorization before forwarding to the sandbox.
func (h *Handler) authenticate(r *http.Request) (*authResult, error) {
	if ar, ok := trustedProxyAuth(r); ok {
		return ar, nil
	}
	if h.validator == nil {
		return nil, errors.New("api key auth not configured")
	}
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("no authorization header")
	}
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return nil, errors.New("authorization header is not a bearer token")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	if token == "" {
		return nil, errors.New("empty bearer token")
	}
	if !strings.HasPrefix(token, auth.APIKeyPrefix) {
		return nil, errors.New("bearer token is not an api key")
	}

	hSum := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(hSum[:])

	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	ak, err := h.validator.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, fmt.Errorf("api key not found: %w", err)
	}
	if ak.RevokedAt != nil {
		return nil, errors.New("api key revoked")
	}
	if ak.ExpiresAt != nil && h.now().After(*ak.ExpiresAt) {
		return nil, errors.New("api key expired")
	}
	user, err := h.validator.GetUserByID(ak.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found for api key: %w", err)
	}
	_ = h.validator.TouchAPIKeyLastUsed(ctx, ak.ID) // non-fatal

	return &authResult{
		UserID: ak.UserID,
		Email:  user.Email,
		KeyID:  ak.ID,
		Scopes: ak.Scopes,
	}, nil
}

func trustedProxyAuth(r *http.Request) (*authResult, bool) {
	userID := strings.TrimSpace(r.Header.Get("X-Authenticated-User"))
	if userID == "" {
		return nil, false
	}
	scopes := parseTrustedScopes(r.Header.Get("X-Authenticated-Scopes"))
	return &authResult{
		UserID: userID,
		Email:  strings.TrimSpace(r.Header.Get("X-Authenticated-Email")),
		Scopes: scopes,
	}, true
}

func parseTrustedScopes(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		scope := strings.TrimSpace(part)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

// requireScope authenticates the request and verifies the key carries the
// required scope. On failure it writes the appropriate error response and
// returns nil.
func (h *Handler) requireScope(w http.ResponseWriter, r *http.Request, scope string) *authResult {
	ar, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "authentication required"})
		return nil
	}
	if !hasScope(ar.Scopes, scope) {
		writeJSON(w, http.StatusForbidden, errorBody{Error: "missing required scope: " + scope})
		return nil
	}
	return ar
}

// hasScope reports whether scopes contains the given scope.
func hasScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// --- JSON helpers --------------------------------------------------------

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("base api: json encode error: %v", err)
	}
}

// --- POST /api/base/blobs ------------------------------------------------

type putBlobResponse struct {
	BlobRef   model.BlobRef `json:"blob_ref"`
	SizeBytes int64         `json:"size_bytes"`
	SHA256    string        `json:"sha256"`
}

// handlePutBlob uploads a blob. The request body is the raw blob bytes. The
// Content-Type header is preserved as the blob's media type in the response
// (the blob store itself is content-addressed and media-type-agnostic).
func (h *Handler) handlePutBlob(w http.ResponseWriter, r *http.Request) {
	ar := h.requireScope(w, r, ScopeWriteBase)
	if ar == nil {
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxBlobUploadBytes+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "read body: " + err.Error()})
		return
	}
	if len(data) > maxBlobUploadBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, errorBody{Error: "blob exceeds 64 MiB limit"})
		return
	}
	if len(data) == 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "empty blob"})
		return
	}
	ref, err := h.blobs.Put(data)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "store blob: " + err.Error()})
		return
	}
	stat, _ := h.blobs.Stat(ref)
	resp := putBlobResponse{
		BlobRef:   ref,
		SizeBytes: stat.SizeBytes,
		SHA256:    stat.SHA256,
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- GET /api/base/blobs/{ref} ------------------------------------------

func (h *Handler) handleGetBlob(w http.ResponseWriter, r *http.Request) {
	ar := h.requireScope(w, r, ScopeReadBase)
	if ar == nil {
		return
	}
	ref := model.BlobRef(r.PathValue("ref"))
	if !ref.Valid() {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid blob_ref"})
		return
	}
	if !h.ownerReferencesBlob(ar.UserID, ref) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "blob not found"})
		return
	}
	data, err := h.blobs.Get(ref)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorBody{Error: "blob not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "get blob: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		log.Printf("base api: blob write error: %v", err)
	}
}

func (h *Handler) ownerReferencesBlob(ownerID string, ref model.BlobRef) bool {
	for _, entry := range entriesForOwner(h.jr.Entries(), ownerID) {
		if entry.Event.BlobRef == ref {
			return true
		}
		var payload tree.Payload
		if err := json.Unmarshal([]byte(entry.Event.PayloadJSON), &payload); err == nil && payload.BlobRef == ref {
			return true
		}
	}
	return false
}

// --- POST /api/base/items ------------------------------------------------

// putItemRequest is the JSON body for creating or updating an item. The
// EventType determines whether this is a create, update, delete, or move.
type putItemRequest struct {
	ItemID       model.ItemID    `json:"item_id"`
	OwnerID      string          `json:"owner_id"`
	EventType    model.EventType `json:"event_type"`
	Kind         model.ItemKind  `json:"kind"`
	ParentItemID model.ItemID    `json:"parent_item_id,omitempty"`
	Name         string          `json:"name,omitempty"`
	BlobRef      model.BlobRef   `json:"blob_ref,omitempty"`
	VersionID    model.VersionID `json:"version_id,omitempty"`
	MediaType    string          `json:"media_type,omitempty"`
	ContentHash  string          `json:"content_hash,omitempty"`
	DeviceID     string          `json:"device_id,omitempty"`
}

type putItemResponse struct {
	EventID   model.EventID `json:"event_id"`
	CursorSeq int64         `json:"cursor_seq"`
	ItemID    model.ItemID  `json:"item_id"`
}

// handlePutItem creates or updates an item by appending a journal Event. The
// SubjectID is set to the authenticated user ID so every mutation is
// attributable.
func (h *Handler) handlePutItem(w http.ResponseWriter, r *http.Request) {
	ar := h.requireScope(w, r, ScopeWriteBase)
	if ar == nil {
		return
	}
	var req putItemRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
		return
	}
	if req.OwnerID != "" && req.OwnerID != ar.UserID {
		writeJSON(w, http.StatusForbidden, errorBody{Error: "owner_id does not match authenticated user"})
		return
	}
	if status, msg := h.validatePutItemRequest(req); status != 0 {
		writeJSON(w, status, errorBody{Error: msg})
		return
	}
	deviceID := req.DeviceID
	if deviceID == "" {
		deviceID = "api:" + ar.KeyID
	}

	// Build the tree payload so tree.Derive can reconstruct the item/version.
	payload := tree.Payload{
		Name:         req.Name,
		ParentItemID: req.ParentItemID,
		Kind:         req.Kind,
		VersionID:    req.VersionID,
		BlobRef:      req.BlobRef,
		MediaType:    req.MediaType,
		ContentHash:  req.ContentHash,
	}

	evt := model.Event{
		EventID:     model.EventID("base_evt_" + uuid.NewString()),
		OwnerID:     ar.UserID,
		ItemID:      req.ItemID,
		DeviceID:    deviceID,
		SubjectID:   ar.UserID,
		EventType:   req.EventType,
		Kind:        req.Kind,
		BlobRef:     req.BlobRef,
		PayloadJSON: payload.JSON(),
		CreatedAt:   h.now(),
	}

	entry, err := h.jr.Append(evt)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "append event: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, putItemResponse{
		EventID:   entry.Event.EventID,
		CursorSeq: entry.Event.CursorSeq,
		ItemID:    entry.Event.ItemID,
	})
}

// --- GET /api/base/items/{id} -------------------------------------------

type itemResponse struct {
	Item    model.Item    `json:"item"`
	Version model.Version `json:"version,omitempty"`
}

// handleGetItem derives the current tree from the journal and returns the
// item (and its current version) at the latest cursor.
func (h *Handler) handleGetItem(w http.ResponseWriter, r *http.Request) {
	ar := h.requireScope(w, r, ScopeReadBase)
	if ar == nil {
		return
	}
	_ = ar
	id := model.ItemID(r.PathValue("id"))
	if !id.Valid() {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid item id"})
		return
	}
	tr := tree.Derive(journal.Events(entriesForOwner(h.jr.Entries(), ar.UserID)))
	item, ok := tr.Items[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "item not found"})
		return
	}
	resp := itemResponse{Item: item}
	if ver, ok := tr.Versions[id]; ok {
		resp.Version = ver
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- GET /api/base/items/{id}/status ------------------------------------

type statusResponse struct {
	ItemID    model.ItemID    `json:"item_id"`
	State     model.SyncState `json:"state"`
	VersionID model.VersionID `json:"version_id,omitempty"`
	UpdatedAt time.Time       `json:"updated_at,omitempty"`
}

// handleGetStatus returns a derived sync status for an item. Without a local
// device tree, the remote-only item is reported as "synced" if it exists and
// "remote_only" if it has no local counterpart. This is a minimal status
// surface; the full per-device status is computed downstream by the sync
// engine.
func (h *Handler) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	ar := h.requireScope(w, r, ScopeReadBase)
	if ar == nil {
		return
	}
	_ = ar
	id := model.ItemID(r.PathValue("id"))
	if !id.Valid() {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid item id"})
		return
	}
	tr := tree.Derive(journal.Events(entriesForOwner(h.jr.Entries(), ar.UserID)))
	item, ok := tr.Items[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "item not found"})
		return
	}
	state := model.StateSynced
	if item.DeletedAt != nil || item.CurrentVersion == "" {
		state = model.StateSynced // tombstone is converged
	}
	resp := statusResponse{
		ItemID:    id,
		State:     state,
		UpdatedAt: item.UpdatedAt,
	}
	if item.CurrentVersion != "" {
		resp.VersionID = item.CurrentVersion
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- GET /api/base/delta -------------------------------------------------

type deltaResponse struct {
	Events []model.Event `json:"events"`
	Cursor int64         `json:"cursor"`
	Head   int64         `json:"head"`
}

// handleDelta returns journal events with CursorSeq > cursor (i.e. events
// since the given cursor). The response includes the new cursor (the highest
// returned seq) and the journal head so clients know how far they've caught
// up.
func (h *Handler) handleDelta(w http.ResponseWriter, r *http.Request) {
	ar := h.requireScope(w, r, ScopeReadBase)
	if ar == nil {
		return
	}
	_ = ar
	cursorStr := r.URL.Query().Get("cursor")
	var cursor int64
	if cursorStr != "" {
		var err error
		cursor, err = strconv.ParseInt(cursorStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid cursor"})
			return
		}
		if cursor < 0 {
			cursor = 0
		}
	}

	entries := entriesForOwner(h.jr.Entries(), ar.UserID)
	var out []model.Event
	var newCursor int64 = cursor
	var head int64
	for _, e := range entries {
		if e.Event.CursorSeq > head {
			head = e.Event.CursorSeq
		}
		if e.Event.CursorSeq > cursor {
			out = append(out, e.Event)
			if e.Event.CursorSeq > newCursor {
				newCursor = e.Event.CursorSeq
			}
		}
	}
	if out == nil {
		out = []model.Event{}
	}
	writeJSON(w, http.StatusOK, deltaResponse{
		Events: out,
		Cursor: newCursor,
		Head:   head,
	})
}

// --- POST /api/base/repair/preview --------------------------------------

// repairPreviewRequest carries the three trees (remote, local, synced) as
// JSON-encoded item/version lists. The planner runs on the decoded trees and
// returns the actions and conflicts.
type repairPreviewRequest struct {
	Remote treeSnapshot `json:"remote"`
	Local  treeSnapshot `json:"local"`
	Synced treeSnapshot `json:"synced"`
}

type treeSnapshot struct {
	Items    []model.Item    `json:"items"`
	Versions []model.Version `json:"versions"`
}

func (ts treeSnapshot) toPlannerTree() planner.Tree {
	t := planner.NewTree()
	for _, it := range ts.Items {
		t.Items[it.ItemID] = it
	}
	for _, v := range ts.Versions {
		t.Versions[v.ItemID] = v
	}
	return t
}

type repairPreviewResponse struct {
	Actions   []planner.Action   `json:"actions"`
	Conflicts []planner.Conflict `json:"conflicts"`
}

// handleRepairPreview runs the pure planner over the provided three trees and
// returns the reconciliation actions and conflicts. This is a read-only
// preview: no journal events are appended.
func (h *Handler) handleRepairPreview(w http.ResponseWriter, r *http.Request) {
	ar := h.requireScope(w, r, ScopeWriteBase)
	if ar == nil {
		return
	}
	_ = ar
	var req repairPreviewRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
		return
	}
	actions, conflicts := planner.Plan(
		req.Remote.toPlannerTree(),
		req.Local.toPlannerTree(),
		req.Synced.toPlannerTree(),
	)
	if actions == nil {
		actions = []planner.Action{}
	}
	if conflicts == nil {
		conflicts = []planner.Conflict{}
	}
	writeJSON(w, http.StatusOK, repairPreviewResponse{
		Actions:   actions,
		Conflicts: conflicts,
	})
}
