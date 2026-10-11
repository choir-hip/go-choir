package researchtools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// CaptureSink writes one poll's items as shared captures and reads any
// shared capture back.
type CaptureSink interface {
	WriteCaptures(ctx context.Context, ownerID, computerID string, items []sources.Item) (sourcegraph.CaptureWriteResult, error)
	ReadCapture(ctx context.Context, captureID string, maxChars int) (json.RawMessage, error)
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

// Corpusd's capture route keeps at most 1 MiB of a body, drops raw feed
// JSON, takes at most 100 items and refuses a request over 64 MiB. The sink
// applies the same trims before sending and splits a poll into requests of
// at most captureBatchItems items and roughly captureBatchBytes, so a large
// poll never meets the request bound.
const (
	captureBodyBytes  = 1 << 20
	captureBatchItems = 100
	captureBatchBytes = 8 << 20
)

func (c *CorpusdCaptureSink) WriteCaptures(ctx context.Context, ownerID, computerID string, items []sources.Item) (sourcegraph.CaptureWriteResult, error) {
	var out sourcegraph.CaptureWriteResult
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("capture sink: corpusd URL is not configured")
	}
	for _, batch := range captureBatches(items) {
		got, err := c.writeCaptureBatch(ctx, ownerID, computerID, batch)
		if err != nil {
			return out, err
		}
		out.New += got.New
		out.Repeated += got.Repeated
		out.Skipped += got.Skipped
		out.CaptureIDs = append(out.CaptureIDs, got.CaptureIDs...)
		out.Captured = append(out.Captured, got.Captured...)
	}
	return out, nil
}

// captureBatches trims each item as corpusd would and groups the items into
// requests under the batch bounds. An item is never split.
func captureBatches(items []sources.Item) [][]sources.Item {
	var batches [][]sources.Item
	var batch []sources.Item
	size := 0
	for _, item := range items {
		if len(item.Body) > captureBodyBytes {
			item.Body = item.Body[:captureBodyBytes]
		}
		item.RawJSON = ""
		n := len(item.Body) + len(item.Title) + len(item.URL) + 1024
		if len(batch) > 0 && (len(batch) >= captureBatchItems || size+n > captureBatchBytes) {
			batches = append(batches, batch)
			batch, size = nil, 0
		}
		batch = append(batch, item)
		size += n
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}
	return batches
}

func (c *CorpusdCaptureSink) writeCaptureBatch(ctx context.Context, ownerID, computerID string, items []sources.Item) (sourcegraph.CaptureWriteResult, error) {
	var out sourcegraph.CaptureWriteResult
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

// ReadCapture returns one shared capture's metadata and body text from
// corpusd. Any bound computer may read any capture.
func (c *CorpusdCaptureSink) ReadCapture(ctx context.Context, captureID string, maxChars int) (json.RawMessage, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("capture reader: corpusd URL is not configured")
	}
	target := c.BaseURL + "/internal/platform/captures/" + url.PathEscape(strings.TrimSpace(captureID))
	if maxChars > 0 {
		target += "?max_chars=" + strconv.Itoa(maxChars)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("capture reader: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("capture reader: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("capture reader: corpusd returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return json.RawMessage(raw), nil
}
