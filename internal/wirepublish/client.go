package wirepublish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

// PostPlatformPublication calls platformd's internal publish endpoint.
func PostPlatformPublication(ctx context.Context, client *http.Client, platformdURL string, req platform.PublishVTextRequest) (*platform.PublishVTextResponse, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	target := strings.TrimRight(strings.TrimSpace(platformdURL), "/") + "/internal/platform/publications/vtext"
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal platform publish request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build platform publish request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Caller", "true")
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call platformd: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read platformd response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &apiErr); err != nil || strings.TrimSpace(apiErr.Error) == "" {
			apiErr.Error = strings.TrimSpace(string(body))
		}
		if apiErr.Error == "" {
			apiErr.Error = fmt.Sprintf("platformd status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("%s", apiErr.Error)
	}
	var out platform.PublishVTextResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode platformd response: %w", err)
	}
	return &out, nil
}
