package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxChunkBytes bounds one sealed chunk read: the platform accepts 8 MiB.
const maxChunkBytes = 8<<20 + 1

// HTTPChunks is the ChunkStore over the platform's file CAS routes, on the
// guest's authenticated connection (the same routes file sync uses).
type HTTPChunks struct {
	BaseURL    string
	ComputerID string
	// Capability returns the bearer token for the guest's platform calls.
	Capability func(context.Context) (string, error)
	Client     *http.Client
}

func (h HTTPChunks) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 5 * time.Minute}
}

func (h HTTPChunks) request(ctx context.Context, method, digest string, body io.Reader) (*http.Request, error) {
	endpoint := strings.TrimRight(h.BaseURL, "/") + "/internal/computers/files/chunks/" + digest + "?computer_id=" + url.QueryEscape(h.ComputerID)
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if h.Capability != nil {
		token, err := h.Capability(ctx)
		if err != nil {
			return nil, fmt.Errorf("capability: %w", err)
		}
		if strings.TrimSpace(token) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
		}
	}
	return req, nil
}

// PutChunk stores one sealed chunk. The route is immutable and idempotent.
func (h HTTPChunks) PutChunk(ctx context.Context, digest string, sealed []byte) error {
	req, err := h.request(ctx, http.MethodPut, digest, bytes.NewReader(sealed))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// HasChunk asks the host whether the computer's own namespace holds the chunk
// (HEAD on the chunk route; the host also claims it against the sweep's grace
// window). 200 is held, 404 is not; anything else, including an older host
// that does not know HEAD, is an error and the caller uploads.
func (h HTTPChunks) HasChunk(ctx context.Context, digest string) (bool, error) {
	req, err := h.request(ctx, http.MethodHead, digest, nil)
	if err != nil {
		return false, err
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	}
	return false, fmt.Errorf("status %d", resp.StatusCode)
}

// RegisterSnapshotRoot declares the snapshot's chunk digests to the platform
// in plaintext (digests are not content), so the file-CAS chunk sweep keeps
// them live; the manifest itself is sealed in the snapshot_recorded event.
func (h HTTPChunks) RegisterSnapshotRoot(ctx context.Context, root string, headSequence uint64, chunks []string) error {
	body, err := json.Marshal(map[string]any{"computer_id": h.ComputerID, "root": root, "head_sequence": headSequence, "chunks": chunks})
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(h.BaseURL, "/") + "/internal/computers/files/snapshot-root"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Capability != nil {
		token, err := h.Capability(ctx)
		if err != nil {
			return fmt.Errorf("capability: %w", err)
		}
		if strings.TrimSpace(token) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
		}
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// GetChunk fetches one sealed chunk.
func (h HTTPChunks) GetChunk(ctx context.Context, digest string) ([]byte, error) {
	req, err := h.request(ctx, http.MethodGet, digest, nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxChunkBytes))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) >= maxChunkBytes {
		return nil, fmt.Errorf("chunk %s has an invalid size %d", digest, len(data))
	}
	return data, nil
}
