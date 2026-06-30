package objectgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// HTTPStore implements the Store interface by querying corpusd (the platform
// Dolt SQL server) through the platformd HTTP API. It is the durable store
// used by runtimes that derive object identity locally and only need remote
// persistence/querying.
//
// All requests carry the X-Internal-Caller: true header so platformd treats
// them as trusted internal callers.
type HTTPStore struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPStore returns an HTTPStore that talks to platformd at baseURL.
// baseURL is the platformd root (e.g. "http://127.0.0.1:7421"); trailing
// slashes are trimmed.
func NewHTTPStore(baseURL string) *HTTPStore {
	return &HTTPStore{
		baseURL:    trimTrailingSlash(baseURL),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Compile-time assertion that HTTPStore satisfies Store.
var _ Store = (*HTTPStore)(nil)

// PutObject persists a pre-built Object via PUT /internal/platform/objects.
// The caller is responsible for canonical_id and content_hash derivation; the
// platform Store upserts the object as-is.
func (h *HTTPStore) PutObject(ctx context.Context, obj Object) error {
	body, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("objectgraph http: marshal object: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, h.baseURL+"/internal/platform/objects", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("objectgraph http: build put object request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("objectgraph http: put object: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("objectgraph http: put object: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// GetObject fetches a single Object by canonical id via
// GET /internal/platform/objects/{id}. A 404 response maps to ErrNotFound.
func (h *HTTPStore) GetObject(ctx context.Context, id string) (Object, error) {
	endpoint := h.baseURL + "/internal/platform/objects/" + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Object{}, fmt.Errorf("objectgraph http: build get object request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return Object{}, fmt.Errorf("objectgraph http: get object: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Object{}, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return Object{}, fmt.Errorf("objectgraph http: get object: unexpected status %d", resp.StatusCode)
	}
	var obj Object
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		return Object{}, fmt.Errorf("objectgraph http: decode object: %w", err)
	}
	return obj, nil
}

// ListObjects lists objects via GET /internal/platform/objects with optional
// kind/owner/limit query filters.
func (h *HTTPStore) ListObjects(ctx context.Context, filter ListFilter) ([]Object, error) {
	q := url.Values{}
	if filter.Kind != "" {
		q.Set("kind", string(filter.Kind))
	}
	if filter.OwnerID != "" {
		q.Set("owner", filter.OwnerID)
	}
	if filter.Limit > 0 {
		q.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Tombstone != nil {
		q.Set("tombstone", strconv.FormatBool(*filter.Tombstone))
	}
	endpoint := h.baseURL + "/internal/platform/objects"
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("objectgraph http: build list objects request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("objectgraph http: list objects: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("objectgraph http: list objects: unexpected status %d", resp.StatusCode)
	}
	var objs []Object
	if err := json.NewDecoder(resp.Body).Decode(&objs); err != nil {
		return nil, fmt.Errorf("objectgraph http: decode objects: %w", err)
	}
	return objs, nil
}

// PutEdge persists a pre-built Edge via PUT /internal/platform/edges. The
// caller is responsible for edge_id derivation; the platform Store upserts the
// edge as-is.
func (h *HTTPStore) PutEdge(ctx context.Context, edge Edge) error {
	body, err := json.Marshal(edge)
	if err != nil {
		return fmt.Errorf("objectgraph http: marshal edge: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, h.baseURL+"/internal/platform/edges", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("objectgraph http: build put edge request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("objectgraph http: put edge: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("objectgraph http: put edge: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// ListEdges lists edges via GET /internal/platform/edges with optional
// from/to/kind/limit query filters.
func (h *HTTPStore) ListEdges(ctx context.Context, filter EdgeFilter) ([]Edge, error) {
	q := url.Values{}
	if filter.FromID != "" {
		q.Set("from", filter.FromID)
	}
	if filter.ToID != "" {
		q.Set("to", filter.ToID)
	}
	if filter.Kind != "" {
		q.Set("kind", string(filter.Kind))
	}
	if filter.Limit > 0 {
		q.Set("limit", strconv.Itoa(filter.Limit))
	}
	if filter.Tombstone != nil {
		q.Set("tombstone", strconv.FormatBool(*filter.Tombstone))
	}
	endpoint := h.baseURL + "/internal/platform/edges"
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("objectgraph http: build list edges request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("objectgraph http: list edges: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("objectgraph http: list edges: unexpected status %d", resp.StatusCode)
	}
	var edges []Edge
	if err := json.NewDecoder(resp.Body).Decode(&edges); err != nil {
		return nil, fmt.Errorf("objectgraph http: decode edges: %w", err)
	}
	return edges, nil
}

// Close is a no-op: HTTP connections are pooled by the http.Client and do not
// require explicit teardown.
func (h *HTTPStore) Close() error { return nil }

func trimTrailingSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
