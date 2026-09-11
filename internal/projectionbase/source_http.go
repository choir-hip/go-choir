package projectionbase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// CapabilityFunc mints the bearer capability for platform reads.
type CapabilityFunc func(ctx context.Context) (string, error)

// HTTPSource reads watermarks, descriptors, blobs, and tail pages from the
// platform over one base URL. Blobs stream to the caller's writer; descriptors
// are small and decode inline. Every consumption verifies content against the
// descriptor; transport errors that are not typed refusals stay plain so
// callers can distinguish outage from refusal.
type HTTPSource struct {
	baseURL    string
	capability CapabilityFunc
	client     *http.Client
	// blobClient streams base blobs. It carries NO whole-request timeout: a
	// retained store's base is tens of GiB and cannot finish inside a fixed
	// deadline. Cancellation comes from the caller's context only.
	blobClient *http.Client
}

// NewHTTPSource returns a platform base source. A nil capability fails closed
// at request time, never as silent deferral.
func NewHTTPSource(baseURL string, capability CapabilityFunc) *HTTPSource {
	return &HTTPSource{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		capability: capability,
		client:     &http.Client{Timeout: 30 * time.Second},
		blobClient: &http.Client{},
	}
}

func (s *HTTPSource) bearer(ctx context.Context) (string, error) {
	if s == nil || strings.TrimSpace(s.baseURL) == "" || s.capability == nil {
		return "", fmt.Errorf("%w: platform source is not configured", ErrBaseRefused)
	}
	token, err := s.capability(ctx)
	if err != nil || strings.TrimSpace(token) == "" {
		return "", fmt.Errorf("%w: platform capability unavailable", ErrBaseRefused)
	}
	return strings.TrimSpace(token), nil
}

func (s *HTTPSource) get(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	return s.getWith(ctx, s.client, path, query)
}

func (s *HTTPSource) getWith(ctx context.Context, client *http.Client, path string, query url.Values) (*http.Response, error) {
	token, err := s.bearer(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return client.Do(req)
}

// Watermark returns the advertised base. A missing watermark is a missing
// required base, refused loudly.
func (s *HTTPSource) Watermark(ctx context.Context, computerID string) (uint64, string, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return 0, "", fmt.Errorf("%w: computer is required", ErrBaseRefused)
	}
	resp, err := s.get(ctx, "/internal/computers/files/watermark", url.Values{"computer_id": {computerID}})
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return 0, "", fmt.Errorf("%w: no advertised base for %s", ErrBaseRefused, computerID)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, "", fmt.Errorf("projection base: watermark status %d", resp.StatusCode)
	}
	var wm struct {
		WatermarkSequence uint64 `json:"watermark_sequence"`
		BaseRef           string `json:"base_ref"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wm); err != nil {
		return 0, "", fmt.Errorf("projection base: decode watermark: %w", err)
	}
	if wm.WatermarkSequence == 0 || strings.TrimSpace(wm.BaseRef) == "" {
		return 0, "", fmt.Errorf("%w: platform advertised an empty base", ErrBaseRefused)
	}
	return wm.WatermarkSequence, strings.TrimSpace(wm.BaseRef), nil
}

// Descriptor fetches the sidecar and binds it to the requested digest and
// computer before Validate.
func (s *HTTPSource) Descriptor(ctx context.Context, computerID, baseRef string) (Descriptor, error) {
	computerID = strings.TrimSpace(computerID)
	baseRef = strings.TrimSpace(baseRef)
	if computerID == "" || baseRef == "" {
		return Descriptor{}, fmt.Errorf("%w: computer and base digest are required", ErrBaseRefused)
	}
	resp, err := s.get(ctx, "/internal/computers/files/projection-base/descriptor", url.Values{"computer_id": {computerID}, "base_ref": {baseRef}})
	if err != nil {
		return Descriptor{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Descriptor{}, fmt.Errorf("%w: descriptor unavailable for base %s", ErrBaseRefused, baseRef)
	}
	if resp.StatusCode != http.StatusOK {
		return Descriptor{}, fmt.Errorf("projection base: descriptor status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Descriptor{}, fmt.Errorf("projection base: read descriptor: %w", err)
	}
	descriptor, err := ParseDescriptor(raw)
	if err != nil {
		return Descriptor{}, fmt.Errorf("%w: %v", ErrBaseRefused, err)
	}
	if descriptor.BlobSHA256 != baseRef {
		return Descriptor{}, fmt.Errorf("%w: descriptor blob %s is not advertised base %s", ErrBaseRefused, descriptor.BlobSHA256, baseRef)
	}
	if descriptor.ComputerID != computerID {
		return Descriptor{}, fmt.Errorf("%w: descriptor computer %q is not %q", ErrBaseRefused, descriptor.ComputerID, computerID)
	}
	return descriptor, nil
}

// DownloadBlob streams the base blob. Digest verification belongs to the
// installer after the stream completes, never to the wire. The blob client
// carries no whole-request timeout: a retained store's base is tens of GiB
// and cannot finish inside a fixed deadline; the caller's ctx cancels.
func (s *HTTPSource) DownloadBlob(ctx context.Context, computerID, baseRef string, dst io.Writer) error {
	computerID = strings.TrimSpace(computerID)
	baseRef = strings.TrimSpace(baseRef)
	if computerID == "" || baseRef == "" || dst == nil {
		return fmt.Errorf("%w: computer, base digest, and destination are required", ErrBaseRefused)
	}
	resp, err := s.getWith(ctx, s.blobClient, "/internal/computers/files/projection-base/blob", url.Values{"computer_id": {computerID}, "base_ref": {baseRef}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: blob unavailable for base %s", ErrBaseRefused, baseRef)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("projection base: blob status %d", resp.StatusCode)
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("projection base: download blob: %w", err)
	}
	return nil
}

// TailPage reads immutable tail events. Page validation (sequence progress,
// ancestry) belongs to the installer and replay, never to the fetch.
func (s *HTTPSource) TailPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, fmt.Errorf("%w: computer is required", ErrBaseRefused)
	}
	if pageSize <= 0 {
		pageSize = 1
	}
	resp, err := s.get(ctx, "/internal/computers/events/replay", url.Values{
		"computer_id":    {computerID},
		"after_sequence": {fmt.Sprintf("%d", afterSequence)},
		"limit":          {fmt.Sprintf("%d", pageSize)},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("projection base: tail status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("projection base: decode tail: %w", err)
	}
	page, err := computerevent.DecodeHistoricDurableEvents(raw)
	if err != nil {
		return nil, fmt.Errorf("projection base: decode tail: %w", err)
	}
	return page, nil
}
