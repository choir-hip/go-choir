package trace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Queries provides higher-level read helpers over a trace Store, including
// causal parent-chain reconstruction and an HTTP handler that exposes the two
// product-facing query endpoints:
//
//   - GET /api/trace/events?run_id=...   list events for a run
//   - GET /api/trace/events/{id}         single event with parent chain
//
// The handler is self-contained and mountable by the runtime router. Owner
// scoping is delegated to an OwnerResolver so the runtime can wire its existing
// authenticateUser path; the default resolver reads the owner_id query
// parameter, which tests use directly.
type Queries struct {
	store Store
}

// NewQueries wraps a Store with query helpers.
func NewQueries(s Store) *Queries {
	return &Queries{store: s}
}

// ParentChain returns the causal chain from the root ancestor down to the event
// identified by id, inclusive. It walks ParentID links up to maxDepth hops to
// guard against cycles. The returned slice is ordered root-first. If the event
// has no parent, the slice contains only the event itself.
func (q *Queries) ParentChain(ctx context.Context, id string, maxDepth int) ([]Event, error) {
	return q.parentChain(ctx, "", id, maxDepth)
}

func (q *Queries) parentChainForOwner(ctx context.Context, ownerID, id string, maxDepth int) ([]Event, error) {
	return q.parentChain(ctx, strings.TrimSpace(ownerID), id, maxDepth)
}

func (q *Queries) parentChain(ctx context.Context, ownerID, id string, maxDepth int) ([]Event, error) {
	if maxDepth <= 0 {
		maxDepth = 64
	}
	visited := make(map[string]struct{}, maxDepth+1)
	var chain []Event
	current := strings.TrimSpace(id)
	for i := 0; i <= maxDepth && current != ""; i++ {
		if _, seen := visited[current]; seen {
			return nil, fmt.Errorf("trace query: parent chain cycle at %s", current)
		}
		visited[current] = struct{}{}
		ev, err := q.getEvent(ctx, ownerID, current)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				if i == 0 {
					return nil, ErrNotFound
				}
				// A referenced parent is missing; stop walking and return what we have.
				break
			}
			return nil, fmt.Errorf("trace query: get %s: %w", current, err)
		}
		chain = append(chain, *ev)
		current = strings.TrimSpace(ev.ParentID)
	}
	// Reverse to root-first order.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

func (q *Queries) getEvent(ctx context.Context, ownerID, id string) (*Event, error) {
	if ownerID != "" {
		return q.store.GetForOwner(ctx, ownerID, id)
	}
	return q.store.Get(ctx, id)
}

// OwnerResolver extracts the authenticated owner id from a request. The runtime
// supplies its own resolver (wrapping authenticateUser). The default resolver
// reads the owner_id query parameter, which is suitable for tests and internal
// callers that already pass X-Internal-Caller-style scoping.
type OwnerResolver func(*http.Request) string

// DefaultOwnerResolver reads owner_id from the query string. Returns empty
// when absent, which causes the handler to respond 401.
func DefaultOwnerResolver(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("owner_id"))
}

// HTTPHandler is a mountable http.Handler exposing the trace event query API.
// It is safe for concurrent use. Routes:
//
//   - GET /api/trace/events?run_id=...[&limit=N]   list events for a run
//   - GET /api/trace/events/{id}                   single event with parent chain
type HTTPHandler struct {
	queries      *Queries
	owner        OwnerResolver
	maxLimit     int
	defaultLimit int
}

// NewHTTPHandler builds a query API handler over the given Store. If owner is
// nil, DefaultOwnerResolver is used.
func NewHTTPHandler(s Store, owner OwnerResolver) *HTTPHandler {
	if owner == nil {
		owner = DefaultOwnerResolver
	}
	return &HTTPHandler{
		queries:      NewQueries(s),
		owner:        owner,
		maxLimit:     1000,
		defaultLimit: 200,
	}
}

// ServeHTTP routes GET /api/trace/events and GET /api/trace/events/{id}.
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeTraceJSON(w, http.StatusMethodNotAllowed, traceAPIError{Error: "method not allowed"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/trace/events")
	path = strings.Trim(path, "/")
	if path == "" {
		h.handleList(w, r)
		return
	}
	h.handleSingle(w, r, path)
}

func (h *HTTPHandler) handleList(w http.ResponseWriter, r *http.Request) {
	ownerID := h.owner(r)
	if ownerID == "" {
		writeTraceJSON(w, http.StatusUnauthorized, traceAPIError{Error: "authentication required"})
		return
	}
	runID := strings.TrimSpace(r.URL.Query().Get("run_id"))
	if runID == "" {
		writeTraceJSON(w, http.StatusBadRequest, traceAPIError{Error: "run_id query parameter is required"})
		return
	}
	limit := h.defaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeTraceJSON(w, http.StatusBadRequest, traceAPIError{Error: "limit must be a positive integer"})
			return
		}
		limit = n
	}
	if h.maxLimit > 0 && limit > h.maxLimit {
		limit = h.maxLimit
	}

	events, err := h.queries.store.ListByRunForOwner(r.Context(), ownerID, runID, limit)
	if err != nil {
		writeTraceJSON(w, http.StatusInternalServerError, traceAPIError{Error: "failed to list trace events"})
		return
	}
	if len(events) == 0 {
		anyEvents, err := h.queries.store.ListByRun(r.Context(), runID, 1)
		if err != nil {
			writeTraceJSON(w, http.StatusInternalServerError, traceAPIError{Error: "failed to list trace events"})
			return
		}
		if len(anyEvents) > 0 {
			writeTraceJSON(w, http.StatusNotFound, traceAPIError{Error: "trace events not found"})
			return
		}
	}
	writeTraceJSON(w, http.StatusOK, traceEventListResponse{Events: events})
}

func (h *HTTPHandler) handleSingle(w http.ResponseWriter, r *http.Request, id string) {
	ownerID := h.owner(r)
	if ownerID == "" {
		writeTraceJSON(w, http.StatusUnauthorized, traceAPIError{Error: "authentication required"})
		return
	}
	id = strings.TrimSpace(id)
	if id == "" {
		writeTraceJSON(w, http.StatusBadRequest, traceAPIError{Error: "event id is required"})
		return
	}
	ev, err := h.queries.store.GetForOwner(r.Context(), ownerID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeTraceJSON(w, http.StatusNotFound, traceAPIError{Error: "trace event not found"})
			return
		}
		writeTraceJSON(w, http.StatusInternalServerError, traceAPIError{Error: "failed to load trace event"})
		return
	}
	chain, err := h.queries.parentChainForOwner(r.Context(), ownerID, id, 64)
	if err != nil && !errors.Is(err, ErrNotFound) {
		// A chain walk failure should not mask the event itself; return the
		// event with an empty chain and surface the error in the response.
		chain = nil
	}
	writeTraceJSON(w, http.StatusOK, traceEventDetailResponse{Event: ev, ParentChain: chain})
}

type traceAPIError struct {
	Error string `json:"error"`
}

type traceEventListResponse struct {
	Events []Event `json:"events"`
}

type traceEventDetailResponse struct {
	Event       *Event  `json:"event"`
	ParentChain []Event `json:"parent_chain,omitempty"`
}

func writeTraceJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// FormatTimestamp exposes the canonical RFC3339Nano formatting used by the
// store for callers that need to render trace event timestamps as strings.
func FormatTimestamp(ts time.Time) string {
	return ts.UTC().Format(time.RFC3339Nano)
}
