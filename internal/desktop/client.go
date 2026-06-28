package desktop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// BaseClient is the HTTP client for the Choir Base API (M4). It authenticates
// every request with a Bearer API key (M1) and exposes the endpoints the sync
// engine needs: delta fetch, blob upload, item create/update, and item get.
//
// The client is safe for concurrent use: it wraps a *http.Client with no
// shared mutable state beyond the base URL and API key, both of which are
// immutable after construction.
type BaseClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
	now     func() time.Time
}

// NewBaseClient creates a Base API client targeting baseURL (e.g.
// "https://choir.news") with the given API key secret (choir_sk_...).
func NewBaseClient(baseURL, apiKey string) *BaseClient {
	return &BaseClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		http: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		now: func() time.Time { return time.Now().UTC() },
	}
}

// SetHTTPClient replaces the underlying *http.Client. Intended for tests that
// want to inject a stub transport (e.g. httptest.Server's default client).
func (c *BaseClient) SetHTTPClient(h *http.Client) {
	if h != nil {
		c.http = h
	}
}

// --- delta response types (mirror internal/base/api/handlers.go) --------

// DeltaResponse is the JSON body returned by GET /api/base/delta.
type DeltaResponse struct {
	Events []model.Event `json:"events"`
	Cursor int64         `json:"cursor"`
	Head   int64         `json:"head"`
}

// PutBlobResponse is the JSON body returned by POST /api/base/blobs.
type PutBlobResponse struct {
	BlobRef   model.BlobRef `json:"blob_ref"`
	SizeBytes int64         `json:"size_bytes"`
	SHA256    string        `json:"sha256"`
}

// PutItemRequest is the JSON body for POST /api/base/items.
type PutItemRequest struct {
	ItemID       model.ItemID    `json:"item_id"`
	OwnerID      string          `json:"owner_id,omitempty"`
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

// PutItemResponse is the JSON body returned by POST /api/base/items.
type PutItemResponse struct {
	EventID   model.EventID `json:"event_id"`
	CursorSeq int64         `json:"cursor_seq"`
	ItemID    model.ItemID  `json:"item_id"`
}

// ItemResponse is the JSON body returned by GET /api/base/items/{id}.
type ItemResponse struct {
	Item    model.Item    `json:"item"`
	Version model.Version `json:"version,omitempty"`
}

// FetchDelta returns journal events with CursorSeq > cursor. The returned
// cursor is the highest seq in the response; Head is the journal head so the
// client knows whether it has caught up.
func (c *BaseClient) FetchDelta(cursor int64) (DeltaResponse, error) {
	u := c.baseURL + "/api/base/delta?cursor=" + strconv.FormatInt(cursor, 10)
	var out DeltaResponse
	if err := c.doJSON("GET", u, nil, &out); err != nil {
		return DeltaResponse{}, fmt.Errorf("base client: fetch delta: %w", err)
	}
	return out, nil
}

// PutBlob uploads raw blob bytes and returns the content-addressed BlobRef.
func (c *BaseClient) PutBlob(data []byte, mediaType string) (PutBlobResponse, error) {
	u := c.baseURL + "/api/base/blobs"
	req, err := http.NewRequest("POST", u, bytes.NewReader(data))
	if err != nil {
		return PutBlobResponse{}, fmt.Errorf("base client: put blob request: %w", err)
	}
	c.setAuth(req)
	if mediaType != "" {
		req.Header.Set("Content-Type", mediaType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return PutBlobResponse{}, fmt.Errorf("base client: put blob: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return PutBlobResponse{}, c.parseError(resp)
	}
	var out PutBlobResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return PutBlobResponse{}, fmt.Errorf("base client: put blob decode: %w", err)
	}
	return out, nil
}

// PutItem creates or updates an item by appending a journal event.
func (c *BaseClient) PutItem(req PutItemRequest) (PutItemResponse, error) {
	u := c.baseURL + "/api/base/items"
	var out PutItemResponse
	body, err := json.Marshal(req)
	if err != nil {
		return PutItemResponse{}, fmt.Errorf("base client: put item marshal: %w", err)
	}
	if err := c.doJSON("POST", u, body, &out); err != nil {
		return PutItemResponse{}, fmt.Errorf("base client: put item: %w", err)
	}
	return out, nil
}

// GetItem fetches the current state of an item by ID.
func (c *BaseClient) GetItem(id model.ItemID) (ItemResponse, error) {
	u := c.baseURL + "/api/base/items/" + string(id)
	var out ItemResponse
	if err := c.doJSON("GET", u, nil, &out); err != nil {
		return ItemResponse{}, fmt.Errorf("base client: get item: %w", err)
	}
	return out, nil
}

// --- helpers ------------------------------------------------------------

func (c *BaseClient) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
}

// doJSON performs a request with an optional JSON body and decodes the
// response into out. For GET requests with a nil body, no Content-Type is set.
func (c *BaseClient) doJSON(method, u string, body []byte, out interface{}) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, reader)
	if err != nil {
		return err
	}
	c.setAuth(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// parseError reads a non-200 response body and returns an error carrying the
// status code and the API's error message when present.
func (c *BaseClient) parseError(resp *http.Response) error {
	var errBody struct {
		Error string `json:"error"`
	}
	msg := ""
	if err := json.NewDecoder(resp.Body).Decode(&errBody); err == nil && errBody.Error != "" {
		msg = errBody.Error
	} else {
		msg = resp.Status
	}
	return fmt.Errorf("base api: HTTP %d: %s", resp.StatusCode, msg)
}
