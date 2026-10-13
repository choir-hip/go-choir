package researchtools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
)

// CaptureSink writes captures (a poll's items, or a page choir.Read
// fetched) to the shared store, reads any shared capture back, and lists a
// computer's captures for its trace.
type CaptureSink interface {
	WriteCaptures(ctx context.Context, ownerID, computerID string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error)
	ReadCapture(ctx context.Context, captureID string, fromBlock, maxChars int) (json.RawMessage, error)
	ListCaptures(ctx context.Context, ownerID, computerID string, limit int) ([]objectgraph.Object, error)
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
// poll never meets the request bound. One read page (up to 16 MiB of raw
// bytes, base64 in the JSON) always goes alone.
const (
	captureBodyBytes  = 1 << 20
	captureBatchItems = 100
	captureBatchBytes = 8 << 20
)

func (c *CorpusdCaptureSink) WriteCaptures(ctx context.Context, ownerID, computerID string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
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
		out.Repeats = append(out.Repeats, got.Repeats...)
		out.Errors = append(out.Errors, got.Errors...)
	}
	return out, nil
}

// captureBatches trims each item as corpusd would and groups the items into
// requests under the batch bounds. An item is never split.
func captureBatches(items []sourcegraph.CaptureItem) [][]sourcegraph.CaptureItem {
	var batches [][]sourcegraph.CaptureItem
	var batch []sourcegraph.CaptureItem
	size := 0
	for _, item := range items {
		if len(item.Body) > captureBodyBytes {
			item.Body = item.Body[:captureBodyBytes]
		}
		item.RawJSON = ""
		n := len(item.Body) + len(item.Title) + len(item.URL) + 1024 + len(item.Raw)*4/3
		for _, b := range item.Blocks {
			n += len(b.Text) + 64 + 48*len(b.Links)
		}
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

func (c *CorpusdCaptureSink) writeCaptureBatch(ctx context.Context, ownerID, computerID string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
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

// ReadCapture returns one shared capture's metadata and, from block
// fromBlock, as many blocks as fit in maxChars of text (a capture written
// before Cut 2 returns its body text) from corpusd. Any bound computer may
// read any capture.
func (c *CorpusdCaptureSink) ReadCapture(ctx context.Context, captureID string, fromBlock, maxChars int) (json.RawMessage, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("capture reader: corpusd URL is not configured")
	}
	query := url.Values{}
	if fromBlock > 0 {
		query.Set("from_block", strconv.Itoa(fromBlock))
	}
	if maxChars > 0 {
		query.Set("max_chars", strconv.Itoa(maxChars))
	}
	target := c.BaseURL + "/internal/platform/captures/" + url.PathEscape(strings.TrimSpace(captureID))
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	raw, err := c.get(ctx, target, 16<<20)
	if err != nil {
		return nil, fmt.Errorf("capture reader: %w", err)
	}
	return json.RawMessage(raw), nil
}

// ListCaptures returns the computer's captures, newest first, at most limit
// of them, from corpusd's object list (captures carry no body in the row).
func (c *CorpusdCaptureSink) ListCaptures(ctx context.Context, ownerID, computerID string, limit int) ([]objectgraph.Object, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("capture list: corpusd URL is not configured")
	}
	query := url.Values{"kind": {string(objectgraph.WebCaptureObjectKind)}, "owner": {ownerID}, "computer": {computerID}, "limit": {strconv.Itoa(limit)}}
	raw, err := c.get(ctx, c.BaseURL+"/internal/platform/objects?"+query.Encode(), 32<<20)
	if err != nil {
		return nil, fmt.Errorf("capture list: %w", err)
	}
	var objs []objectgraph.Object
	if err := json.Unmarshal(raw, &objs); err != nil {
		return nil, fmt.Errorf("capture list: decode: %w", err)
	}
	return objs, nil
}

func (c *CorpusdCaptureSink) get(ctx context.Context, target string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", ErrCaptureNotFound, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("corpusd returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

// ErrCaptureNotFound is corpusd's answer for a capture id it does not hold.
var ErrCaptureNotFound = errors.New("capture not found")
