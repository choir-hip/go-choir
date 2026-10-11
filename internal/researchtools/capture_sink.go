package researchtools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// CaptureSink writes one poll's items as shared captures.
type CaptureSink interface {
	WriteCaptures(ctx context.Context, ownerID, computerID string, items []sources.Item) (sourcegraph.CaptureWriteResult, error)
}

// CorpusdCaptureSink posts a poll's items to corpusd's capture route, which
// writes them to Store B with bodies on host disk. The guest is
// authenticated by its tap address, as for every guest object write.
type CorpusdCaptureSink struct {
	BaseURL string
	Client  *http.Client
}

func NewCorpusdCaptureSink(baseURL string) *CorpusdCaptureSink {
	return &CorpusdCaptureSink{BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), Client: &http.Client{Timeout: 60 * time.Second}}
}

func (c *CorpusdCaptureSink) WriteCaptures(ctx context.Context, ownerID, computerID string, items []sources.Item) (sourcegraph.CaptureWriteResult, error) {
	var out sourcegraph.CaptureWriteResult
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("capture sink: corpusd URL is not configured")
	}
	if len(items) == 0 {
		return out, nil
	}
	raw, err := json.Marshal(map[string]any{"owner_id": ownerID, "computer_id": computerID, "items": items})
	if err != nil {
		return out, fmt.Errorf("capture sink: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/internal/platform/captures", bytes.NewReader(raw))
	if err != nil {
		return out, fmt.Errorf("capture sink: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return out, fmt.Errorf("capture sink: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("capture sink: corpusd returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("capture sink: decode response: %w", err)
	}
	return out, nil
}
