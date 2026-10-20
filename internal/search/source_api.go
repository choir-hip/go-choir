package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostlink"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// SourceAPIClient calls the gateway's POST /provider/v1/source-api: one
// named endpoint of one keyed source API, with the key added on the host
// (docs/problems/keyed-source-apis-2026-10-20.md). It returns the
// gateway's answer as JSON.
type SourceAPIClient interface {
	SourceAPI(ctx context.Context, provider, endpoint string, params map[string]any) (json.RawMessage, error)
}

// sourceAPIMaxAnswer bounds the gateway answer read here; the gateway cuts
// an upstream body at 2 MiB and wraps it.
const sourceAPIMaxAnswer = 3 << 20

type gatewaySourceAPIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewGatewaySourceAPIClientFromEnv returns the gateway source-api client,
// or nil when the gateway URL or token is unavailable.
func NewGatewaySourceAPIClientFromEnv() SourceAPIClient {
	baseURL := strings.TrimSpace(os.Getenv("RUNTIME_GATEWAY_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("PROXY_VMCTL_URL"))
	}
	token := provideriface.GatewayToken()
	if baseURL == "" || token == "" {
		return nil
	}
	// The gateway bounds the upstream at 30 s; the margin covers the hop.
	return &gatewaySourceAPIClient{baseURL: baseURL, token: token, httpClient: hostlink.NewClient(45 * time.Second)}
}

func (c *gatewaySourceAPIClient) SourceAPI(ctx context.Context, provider, endpoint string, params map[string]any) (json.RawMessage, error) {
	if params == nil {
		params = map[string]any{}
	}
	data, err := json.Marshal(map[string]any{"provider": provider, "endpoint": endpoint, "params": params})
	if err != nil {
		return nil, fmt.Errorf("source api: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/provider/v1/source-api", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("source api: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("source api: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, sourceAPIMaxAnswer))
	if err != nil {
		return nil, fmt.Errorf("source api: read answer: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &errResp) == nil && strings.TrimSpace(errResp.Error) != "" {
			return nil, fmt.Errorf("source api: %s", errResp.Error)
		}
		return nil, fmt.Errorf("source api: status %s", resp.Status)
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("source api: answer is not JSON")
	}
	return json.RawMessage(body), nil
}
