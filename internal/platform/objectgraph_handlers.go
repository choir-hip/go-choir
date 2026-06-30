package platform

import (
	"encoding/json"
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
}

func NewObjectGraphHandler(svc *objectgraph.Service) *ObjectGraphHandler {
	return &ObjectGraphHandler{service: svc}
}

// HandleObjects handles POST /internal/platform/objects (create) and
// GET /internal/platform/objects (list with optional kind/owner/limit filters).
func (h *ObjectGraphHandler) HandleObjects(w http.ResponseWriter, r *http.Request) {
	if err := requireInternalCaller(r); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	switch r.Method {
	case http.MethodPost:
		h.createObject(w, r)
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
		log.Printf("platformd: get object %s: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to get object"})
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

// HandleEdges handles POST /internal/platform/edges (create) and
// GET /internal/platform/edges (list with optional from/to/kind/limit filters).
func (h *ObjectGraphHandler) HandleEdges(w http.ResponseWriter, r *http.Request) {
	if err := requireInternalCaller(r); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	switch r.Method {
	case http.MethodPost:
		h.createEdge(w, r)
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
		log.Printf("platformd: create object: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, obj)
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
		log.Printf("platformd: list objects: %v", err)
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
		log.Printf("platformd: create edge: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, edge)
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
		log.Printf("platformd: list edges: %v", err)
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
