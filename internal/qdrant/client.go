package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Health(ctx context.Context) error {
	_, _, err := c.do(ctx, http.MethodGet, "/healthz", nil)
	return err
}

func (c *Client) CreateCollection(ctx context.Context, name string, cfg CollectionConfig) error {
	body := map[string]any{
		"vectors": map[string]any{
			"size":     cfg.VectorSize,
			"distance": cfg.Distance,
		},
		"on_disk": cfg.OnDisk,
	}
	_, _, err := c.do(ctx, http.MethodPut, "/collections/"+name, body)
	return err
}

func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	_, _, err := c.do(ctx, http.MethodDelete, "/collections/"+name, nil)
	return err
}

func (c *Client) GetCollectionInfo(ctx context.Context, name string) (CollectionInfo, error) {
	data, _, err := c.do(ctx, http.MethodGet, "/collections/"+name, nil)
	if err != nil {
		return CollectionInfo{}, err
	}
	var resp struct {
		Result struct {
			PointsCount int    `json:"points_count"`
			Status      string `json:"status"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return CollectionInfo{}, fmt.Errorf("parse collection info: %w", err)
	}
	return CollectionInfo{
		PointsCount: resp.Result.PointsCount,
		Status:      resp.Result.Status,
	}, nil
}

func (c *Client) UpsertPoints(ctx context.Context, collectionName string, points []Point) error {
	body := struct {
		Points []Point `json:"points"`
	}{Points: points}
	_, _, err := c.do(ctx, http.MethodPut, "/collections/"+collectionName+"/points", body)
	return err
}

func (c *Client) Search(ctx context.Context, collectionOrAlias string, vector []float32, limit int) ([]ScoredPoint, error) {
	if limit <= 0 {
		limit = 10
	}
	body := map[string]any{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
		"with_vector":  false,
	}
	data, _, err := c.do(ctx, http.MethodPost, "/collections/"+collectionOrAlias+"/points/search", body)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result []struct {
			ID      string       `json:"id"`
			Score   float32      `json:"score"`
			Payload PointPayload `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse search response: %w", err)
	}
	out := make([]ScoredPoint, len(resp.Result))
	for i, r := range resp.Result {
		out[i] = ScoredPoint{ID: r.ID, Score: r.Score, Payload: r.Payload}
	}
	return out, nil
}

func (c *Client) ListAliases(ctx context.Context) ([]AliasInfo, error) {
	data, _, err := c.do(ctx, http.MethodGet, "/aliases", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Aliases []AliasInfo `json:"aliases"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse aliases: %w", err)
	}
	return resp.Result.Aliases, nil
}

func (c *Client) UpdateAliases(ctx context.Context, actions []AliasAction) error {
	body := struct {
		Actions []AliasAction `json:"actions"`
	}{Actions: actions}
	_, _, err := c.do(ctx, http.MethodPost, "/collections/aliases", body)
	return err
}

// CreatePayloadIndex creates a payload field index on the given collection.
// Qdrant supports indexing payload fields for filtered search.
func (c *Client) CreatePayloadIndex(ctx context.Context, collectionName, fieldName, fieldType string) error {
	body := map[string]any{
		"field_name": fieldName,
		"field_schema": map[string]any{
			"type": fieldType,
		},
	}
	_, _, err := c.do(ctx, http.MethodPut, "/collections/"+collectionName+"/index", body)
	return err
}

// EnsureProductionCollection creates the production Qdrant collection if it does
// not already exist, configured for 1024-dim Cosine distance with payload
// indexes on vm_owner and content_hash. It is idempotent.
func EnsureProductionCollection(ctx context.Context, client API, collectionName string) error {
	// Check if collection already exists.
	if _, err := client.GetCollectionInfo(ctx, collectionName); err == nil {
		return nil
	}
	cfg := CollectionConfig{
		VectorSize: 1024,
		Distance:   DefaultDistance,
		OnDisk:     false,
	}
	if err := client.CreateCollection(ctx, collectionName, cfg); err != nil {
		return fmt.Errorf("ensure production collection: create: %w", err)
	}
	for _, field := range []string{"vm_owner", "content_hash"} {
		if err := client.CreatePayloadIndex(ctx, collectionName, field, "keyword"); err != nil {
			return fmt.Errorf("ensure production collection: index %s: %w", field, err)
		}
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respBody, resp.StatusCode, fmt.Errorf("qdrant %s %s: status %s: %s", method, path, resp.Status, truncateResp(respBody))
	}
	return respBody, resp.StatusCode, nil
}

func truncateResp(data []byte) string {
	s := string(data)
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}
