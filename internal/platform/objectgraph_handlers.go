package platform

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// RegisterObjectGraphRoutes adds object graph API endpoints to the platform
// server. These endpoints allow sourcecycled and VMs to project and query
// object graph data stored in corpusd (the platform Dolt SQL server).
func RegisterObjectGraphRoutes(s *server.Server, h *ObjectGraphHandler) {
	s.HandleFunc("/internal/platform/objects", h.HandleObjects)
	s.HandleFunc("/internal/platform/objects/", h.HandleObjectByID)
	s.HandleFunc("/internal/platform/edges", h.HandleEdges)
}

type ObjectGraphHandler struct {
	service *objectgraph.Service
	store   objectgraph.Store
}

// NewObjectGraphHandler builds a handler that exposes both Service-level
// methods (CreateObject/GET, used by sourcecycled and VMs that want the
// platform to derive object identity) and Store-level PUT methods (used by
// runtimes that derive identity locally and only need durable persistence).
func NewObjectGraphHandler(svc *objectgraph.Service, store objectgraph.Store) *ObjectGraphHandler {
	return &ObjectGraphHandler{service: svc, store: store}
}

// HandleObjects handles POST /internal/platform/objects (Service create),
// PUT /internal/platform/objects (Store put with a pre-built Object), and
// GET /internal/platform/objects (list with optional kind/owner/limit filters).
func (h *ObjectGraphHandler) HandleObjects(w http.ResponseWriter, r *http.Request) {
	if err := requireInternalCaller(r); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	switch r.Method {
	case http.MethodPost:
		h.createObject(w, r)
	case http.MethodPut:
		h.putObject(w, r)
	case http.MethodGet:
		h.listObjects(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

// HandleObjectByID handles GET /internal/platform/objects/{id} (get one).
func (h *ObjectGraphHandler) HandleObjectByID(w http.ResponseWriter, r *http.Request) {
	if err := requireInternalCaller(r); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/internal/platform/objects/")
	id = strings.Trim(id, "/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	obj, err := h.service.GetObject(r.Context(), id)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			http.NotFound(w, r)
			return
		}
		log.Printf("corpusd: get object %s: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get object"})
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

// HandleEdges handles POST /internal/platform/edges (Service create),
// PUT /internal/platform/edges (Store put with a pre-built Edge), and
// GET /internal/platform/edges (list with optional from/to/kind/limit filters).
func (h *ObjectGraphHandler) HandleEdges(w http.ResponseWriter, r *http.Request) {
	if err := requireInternalCaller(r); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	switch r.Method {
	case http.MethodPost:
		h.createEdge(w, r)
	case http.MethodPut:
		h.putEdge(w, r)
	case http.MethodGet:
		h.listEdges(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

type createObjectRequest struct {
	Kind        string          `json:"kind"`
	OwnerID     string          `json:"owner_id"`
	ComputerID  string          `json:"computer_id,omitempty"`
	VersionID   string          `json:"version_id,omitempty"`
	IdentityKey string          `json:"identity_key,omitempty"`
	Body        []byte          `json:"body,omitempty"`
	Metadata    json.RawMessage `json:"metadata"`
	Now         string          `json:"now,omitempty"`
}

func (h *ObjectGraphHandler) createObject(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "objectgraph unavailable"})
		return
	}
	var req createObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	now := time.Now().UTC()
	if rawNow := strings.TrimSpace(req.Now); rawNow != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, rawNow); err == nil {
			now = parsed.UTC()
		}
	}
	obj, err := h.service.CreateObject(r.Context(), objectgraph.CreateObjectRequest{
		Kind:        objectgraph.ObjectKind(req.Kind),
		OwnerID:     req.OwnerID,
		ComputerID:  req.ComputerID,
		VersionID:   req.VersionID,
		IdentityKey: req.IdentityKey,
		Body:        req.Body,
		Metadata:    req.Metadata,
		Now:         now,
	})
	if err != nil {
		log.Printf("corpusd: create object: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, obj)
}

// putObject handles PUT /internal/platform/objects. It accepts a full
// objectgraph.Object (with a pre-built canonical_id and content_hash) and
// persists it directly via the Store, bypassing Service.CreateObject's ID
// derivation. This is the path used by runtimes that derive identity locally.
// The handler validates that the canonical_id is well-formed and consistent
// with the object_kind and owner_id before storing.
func (h *ObjectGraphHandler) putObject(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "objectgraph store unavailable"})
		return
	}
	var obj objectgraph.Object
	if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if err := validateObject(obj); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	if err := h.store.PutObject(r.Context(), obj); err != nil {
		log.Printf("corpusd: put object %s: %v", obj.CanonicalID, err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

func (h *ObjectGraphHandler) listObjects(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "objectgraph unavailable"})
		return
	}
	q := r.URL.Query()
	filter := objectgraph.ListFilter{
		Kind:    objectgraph.ObjectKind(q.Get("kind")),
		OwnerID: q.Get("owner"),
		Limit:   parseLimit(q.Get("limit")),
	}
	if raw := q.Get("tombstone"); raw != "" {
		t := strings.ToLower(raw) == "true" || raw == "1"
		filter.Tombstone = &t
	}
	objs, err := h.service.ListObjects(r.Context(), filter)
	if err != nil {
		log.Printf("corpusd: list objects: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list objects"})
		return
	}
	writeJSON(w, http.StatusOK, objs)
}

type createEdgeRequest struct {
	FromID   string          `json:"from_id"`
	ToID     string          `json:"to_id"`
	Kind     string          `json:"kind"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

func (h *ObjectGraphHandler) createEdge(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "objectgraph unavailable"})
		return
	}
	var req createEdgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	edge, err := h.service.PutEdge(r.Context(), req.FromID, req.ToID, objectgraph.EdgeKind(req.Kind), req.Metadata)
	if err != nil {
		log.Printf("corpusd: create edge: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, edge)
}

// putEdge handles PUT /internal/platform/edges. It accepts a full
// objectgraph.Edge (with a pre-built edge_id) and persists it directly via the
// Store, bypassing Service.PutEdge's validation/derivation. This is the path
// used by runtimes that derive edge identity locally. The handler validates
// that required fields are non-empty before storing.
func (h *ObjectGraphHandler) putEdge(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "objectgraph store unavailable"})
		return
	}
	var edge objectgraph.Edge
	if err := json.NewDecoder(r.Body).Decode(&edge); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if err := validateEdge(edge); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	if err := h.store.PutEdge(r.Context(), edge); err != nil {
		log.Printf("corpusd: put edge %s: %v", edge.EdgeID, err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, edge)
}

func (h *ObjectGraphHandler) listEdges(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.service == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "objectgraph unavailable"})
		return
	}
	q := r.URL.Query()
	filter := objectgraph.EdgeFilter{
		FromID: q.Get("from"),
		ToID:   q.Get("to"),
		Kind:   objectgraph.EdgeKind(q.Get("kind")),
		Limit:  parseLimit(q.Get("limit")),
	}
	if raw := q.Get("tombstone"); raw != "" {
		t := strings.ToLower(raw) == "true" || raw == "1"
		filter.Tombstone = &t
	}
	edges, err := h.service.ListEdges(r.Context(), filter)
	if err != nil {
		log.Printf("corpusd: list edges: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to list edges"})
		return
	}
	writeJSON(w, http.StatusOK, edges)
}

func requireInternalCaller(r *http.Request) error {
	if r.Header.Get("X-Internal-Caller") != "true" {
		return errInternalCallerRequired
	}
	return nil
}

var errInternalCallerRequired = &internalCallerError{}

type internalCallerError struct{}

func (e *internalCallerError) Error() string { return "internal caller required" }

func parseLimit(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// validateObject checks that a pre-built Object has a well-formed canonical_id
// consistent with its object_kind and owner_id, and that the content_hash
// matches the body/metadata. This prevents a buggy or malicious internal
// caller from persisting inconsistent graph rows via the PUT endpoint.
func validateObject(obj objectgraph.Object) error {
	if strings.TrimSpace(obj.CanonicalID) == "" {
		return fmt.Errorf("canonical_id is required")
	}
	kind, ownerID, _, err := objectgraph.ParseCanonicalID(obj.CanonicalID)
	if err != nil {
		return fmt.Errorf("invalid canonical_id: %w", err)
	}
	if kind != obj.ObjectKind {
		return fmt.Errorf("canonical_id kind %q does not match object_kind %q", kind, obj.ObjectKind)
	}
	if ownerID != obj.OwnerID {
		return fmt.Errorf("canonical_id owner %q does not match owner_id %q", ownerID, obj.OwnerID)
	}
	if strings.TrimSpace(obj.ContentHash) == "" {
		return fmt.Errorf("content_hash is required")
	}
	expectedHash := objectgraph.ContentHash(obj.ObjectKind, obj.Body, obj.Metadata)
	if obj.ContentHash != expectedHash {
		return fmt.Errorf("content_hash %q does not match computed hash %q", obj.ContentHash, expectedHash)
	}
	return nil
}

// validateEdge checks that a pre-built Edge has all required non-empty fields.
func validateEdge(edge objectgraph.Edge) error {
	if strings.TrimSpace(edge.EdgeID) == "" {
		return fmt.Errorf("edge_id is required")
	}
	if strings.TrimSpace(edge.FromID) == "" {
		return fmt.Errorf("from_id is required")
	}
	if strings.TrimSpace(edge.ToID) == "" {
		return fmt.Errorf("to_id is required")
	}
	if strings.TrimSpace(string(edge.Kind)) == "" {
		return fmt.Errorf("kind is required")
	}
	return nil
}
