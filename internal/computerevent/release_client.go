package computerevent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Release reads (P8, docs/archive/reports/p6-p8-release-adopt-2026-10-15.md): a
// guest reads a published release record and its patch from corpusd with
// its own computer capability (event:read). The caller verifies every byte;
// these methods only move them.

// FetchReleaseRecord returns the raw JSON corpusd serves for one published
// release, and the HTTP status.
func (c *HTTPClient) FetchReleaseRecord(ctx context.Context, computerID, releaseID string) ([]byte, int, error) {
	if !IsSHA256(strings.TrimSpace(releaseID)) {
		return nil, 0, fmt.Errorf("computer event client: release id must be a sha256")
	}
	return c.getRelease(ctx, "/internal/platform/releases/"+strings.TrimSpace(releaseID), computerID, 1<<20)
}

// FetchReleaseBlob returns a release patch from the host content store, at
// most maxBytes, and the HTTP status.
func (c *HTTPClient) FetchReleaseBlob(ctx context.Context, computerID, digest string, maxBytes int64) ([]byte, int, error) {
	if !IsSHA256(strings.TrimSpace(digest)) {
		return nil, 0, fmt.Errorf("computer event client: blob digest must be a sha256")
	}
	return c.getRelease(ctx, "/internal/platform/releases/blobs/"+strings.TrimSpace(digest), computerID, maxBytes)
}

func (c *HTTPClient) getRelease(ctx context.Context, path, computerID string, maxBytes int64) ([]byte, int, error) {
	if c == nil {
		return nil, 0, ErrPayloadResolverRequired
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, 0, fmt.Errorf("computer event client: computer id is required")
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + path
	endpoint.RawQuery = url.Values{"computer_id": {computerID}}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, err
	}
	token, err := c.capability(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("computer event client: capability: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := c.http.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, response.StatusCode, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, response.StatusCode, fmt.Errorf("computer event client: release response exceeds %d bytes", maxBytes)
	}
	return raw, response.StatusCode, nil
}
