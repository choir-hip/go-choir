package computerevent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostlink"
)

var ErrCASConflict = errors.New("computer event CAS conflict")

type CapabilitySource func(context.Context) (string, error)

type HTTPClient struct {
	baseURL    *url.URL
	http       *http.Client
	capability CapabilitySource

	// calls counts requests sent to corpusd by endpoint (CallCounts).
	callsMu sync.Mutex
	calls   map[string]int64
}

func NewHTTPClient(baseURL string, client *http.Client, capability CapabilitySource, allowInsecureLoopback bool) (*HTTPClient, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("computer event client: invalid corpusd URL")
	}
	if parsed.Scheme != "https" {
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		isLoopback := host == "localhost" || (ip != nil && ip.IsLoopback())
		if !allowInsecureLoopback || !isLoopback {
			return nil, fmt.Errorf("computer event client: TLS is required")
		}
	}
	if client == nil || capability == nil {
		return nil, fmt.Errorf("computer event client: HTTP client and capability source are required")
	}
	return &HTTPClient{baseURL: parsed, http: client, capability: capability}, nil
}

// CallCounts is the number of requests this client has sent to corpusd, by the
// last segment of the endpoint path ("head", "pin", "append", ...). It counts
// every attempt, successful or not. Observability only: nothing branches on it
// (storage solutions 2, S5: the calls a tape event costs).
func (c *HTTPClient) CallCounts() map[string]int64 {
	if c == nil {
		return nil
	}
	c.callsMu.Lock()
	defer c.callsMu.Unlock()
	out := make(map[string]int64, len(c.calls))
	for k, v := range c.calls {
		out[k] = v
	}
	return out
}

func (c *HTTPClient) countCall(path string) {
	path, _, _ = strings.Cut(path, "?")
	label := path[strings.LastIndexByte(path, '/')+1:]
	if label == "" {
		label = "root"
	}
	c.callsMu.Lock()
	if c.calls == nil {
		c.calls = map[string]int64{}
	}
	c.calls[label]++
	c.callsMu.Unlock()
}

func (c *HTTPClient) BaseURL() string {
	if c == nil || c.baseURL == nil {
		return ""
	}
	return c.baseURL.String()
}

func (c *HTTPClient) Capability() CapabilitySource {
	if c == nil {
		return nil
	}
	return c.capability
}

func (c *HTTPClient) Head(ctx context.Context, computerID string) (*Head, error) {
	query := url.Values{"computer_id": []string{computerID}}
	var head Head
	status, err := c.do(ctx, http.MethodGet, "/internal/computers/events/head?"+query.Encode(), nil, &head)
	if status == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &head, nil
}

func (c *HTTPClient) PinEvent(ctx context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (PinResult, error) {
	request := map[string]any{
		"computer_id":        computerID,
		"payload_base64":     base64.RawStdEncoding.EncodeToString(canonicalEvent),
		"media_type":         "application/vnd.choir.computer-event+json",
		"privacy_class":      "private",
		"pin_namespace":      "computer-event",
		"request_commitment": requestCommitment,
	}
	var response struct {
		ArtifactDigest string  `json:"artifact_digest"`
		Receipt        Receipt `json:"receipt"`
	}
	_, err := c.do(ctx, http.MethodPost, "/internal/computers/events/pin", request, &response)
	return PinResult{ArtifactDigest: response.ArtifactDigest, Receipt: response.Receipt}, err
}

func (c *HTTPClient) pinPayload(ctx context.Context, computerID string, payload []byte, mediaType, privacyClass, requestCommitment string) (PinResult, error) {
	request := map[string]any{
		"computer_id":           computerID,
		"payload_base64":        base64.RawStdEncoding.EncodeToString(payload),
		"media_type":            mediaType,
		"privacy_class":         privacyClass,
		"pin_namespace":         "computer-event-payload",
		"pin_intent_commitment": requestCommitment,
	}
	var response struct {
		ArtifactDigest string  `json:"artifact_digest"`
		Receipt        Receipt `json:"receipt"`
	}
	_, err := c.do(ctx, http.MethodPost, "/internal/computers/events/pin", request, &response)
	return PinResult{ArtifactDigest: response.ArtifactDigest, Receipt: response.Receipt}, err
}

// PreparePrivatePayload encrypts and freezes the exact content-addressed
// envelope before the caller computes the event's pin-intent commitment.
func (c *HTTPClient) PreparePrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID, mediaType string, plaintext []byte) ([]byte, PrivateArtifactMetadata, error) {
	if c == nil || cipher == nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event client: private artifact cipher is required")
	}
	return cipher.Encrypt(ctx, computerID, eventID, mediaType, "private", plaintext)
}

// PinPrivatePayload pins an already-frozen envelope against the pin-intent
// commitment. Encryption and pinning are intentionally separate: the envelope
// digest must be present in event intent before that commitment is computed.
func (c *HTTPClient) PinPrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (PinResult, error) {
	if c == nil || cipher == nil {
		return PinResult{}, fmt.Errorf("computer event client: private artifact cipher is required")
	}
	if _, _, err := cipher.Decrypt(ctx, envelope, computerID, eventID); err != nil {
		return PinResult{}, fmt.Errorf("computer event client: private envelope authentication failed: %w", err)
	}
	return c.pinPayload(ctx, computerID, envelope, PrivateArtifactMediaType, "private", pinIntentCommitment)
}

func (c *HTTPClient) PinNonPrivatePayload(ctx context.Context, computerID string, payload []byte, mediaType, privacyClass, requestCommitment string) (PinResult, error) {
	if privacyClass == "" || privacyClass == "private" {
		return PinResult{}, fmt.Errorf("computer event client: non-private privacy class is required")
	}
	return c.pinPayload(ctx, computerID, payload, mediaType, privacyClass, requestCommitment)
}

// CASError is the typed failure of a head CAS call. Definite is true when the
// host answered 400, 403, 409 or 422. For the FIRST send of a request that is
// a refusal: the append is not on the tape, and the appender may drop its
// prepared row. It does not prove the same for a RE-SEND: the host runs
// authorization and pin validation before it looks up the idempotency key, and
// maps unclassified internal errors to 400, so a refusal of a re-send can
// follow a first send that committed. The appender therefore keeps its row on
// any failure of a re-send. Every other failure (a transport error, a
// cancelled caller, a 5xx, a reply that did not decode) leaves the outcome
// unknown even for a first send, because the host may have committed before
// the reply was lost. Status is 0 when no HTTP status was received.
type CASError struct {
	Status   int
	Definite bool
	Err      error
}

func (e *CASError) Error() string { return e.Err.Error() }
func (e *CASError) Unwrap() error { return e.Err }

// CASRefusedDefinitely reports whether err proves the host did not commit the
// request. An error of unknown type is not a refusal: the caller must treat
// its outcome as unknown.
func CASRefusedDefinitely(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrCASConflict) {
		return true
	}
	var casErr *CASError
	return errors.As(err, &casErr) && casErr.Definite
}

func (c *HTTPClient) CompareAndSwap(ctx context.Context, request CASRequest) (Receipt, error) {
	var receipt Receipt
	status, err := c.do(ctx, http.MethodPost, "/internal/computers/events/append", request, &receipt)
	if err == nil {
		return receipt, nil
	}
	switch status {
	case http.StatusConflict:
		return Receipt{}, &CASError{Status: status, Definite: true, Err: fmt.Errorf("%w: %v", ErrCASConflict, err)}
	case http.StatusBadRequest, http.StatusForbidden, http.StatusUnprocessableEntity:
		return Receipt{}, &CASError{Status: status, Definite: true, Err: err}
	}
	return Receipt{}, &CASError{Status: status, Err: err}
}

func (c *HTTPClient) FetchPayload(ctx context.Context, computerID, artifactDigest string) ([]byte, error) {
	if c == nil {
		return nil, ErrPayloadResolverRequired
	}
	computerID = strings.TrimSpace(computerID)
	artifactDigest = strings.TrimSpace(artifactDigest)
	if computerID == "" || !IsSHA256(artifactDigest) {
		return nil, fmt.Errorf("computer event client: computer and payload digest are required")
	}
	query := url.Values{
		"computer_id":     []string{computerID},
		"artifact_digest": []string{artifactDigest},
	}
	var response struct {
		PayloadBase64 string `json:"payload_base64"`
	}
	_, err := c.doWithResponseLimit(ctx, http.MethodGet, "/internal/computers/events/payload?"+query.Encode(), nil, &response, EventPayloadMaxResponseBytes)
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawStdEncoding.DecodeString(response.PayloadBase64)
	if err != nil {
		raw, err = base64.StdEncoding.DecodeString(response.PayloadBase64)
	}
	if err != nil {
		return nil, fmt.Errorf("computer event client: payload encoding: %w", err)
	}
	if DigestBytes(raw) != artifactDigest {
		return nil, fmt.Errorf("%w: %s", ErrPayloadDigestMismatch, artifactDigest)
	}
	return raw, nil
}

// FetchBlobRaw streams a content-addressed blob from the corpusd
// platform-update artifact namespace over the guest's authenticated
// connection. The body is not buffered: the caller streams it to disk and
// verifies the digest itself. Gated by the same Bearer capability as
// FetchPayload; the path namespace is platform-update, not event payloads.
func (c *HTTPClient) FetchBlobRaw(ctx context.Context, computerID, artifactDigest string) (io.ReadCloser, error) {
	if c == nil {
		return nil, ErrPayloadResolverRequired
	}
	computerID = strings.TrimSpace(computerID)
	artifactDigest = strings.TrimSpace(artifactDigest)
	if computerID == "" || !IsSHA256(artifactDigest) {
		return nil, fmt.Errorf("computer event client: computer and blob digest are required")
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + "/internal/computers/platform-updates/blob/" + artifactDigest
	q := endpoint.Query()
	q.Set("computer_id", computerID)
	endpoint.RawQuery = q.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	token, err := c.capability(ctx)
	if err != nil {
		return nil, fmt.Errorf("computer event client: capability: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	result, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		defer result.Body.Close()
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(result.Body, 1<<20)).Decode(&failure)
		if failure.Error == "" {
			failure.Error = http.StatusText(result.StatusCode)
		}
		return nil, fmt.Errorf("computer event client: corpusd blob fetch returned %d: %s", result.StatusCode, failure.Error)
	}
	return result.Body, nil
}

func (c *HTTPClient) EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]DurableEvent, error) {
	if pageSize <= 0 || pageSize > EventReplayMaxPageSize {
		return nil, fmt.Errorf("computer event client: replay page size %d is invalid", pageSize)
	}
	query := url.Values{
		"computer_id":    []string{computerID},
		"after_sequence": []string{fmt.Sprintf("%d", afterSequence)},
		"limit":          []string{fmt.Sprintf("%d", pageSize)},
	}
	var rawPage json.RawMessage
	_, err := c.doWithResponseLimit(ctx, http.MethodGet, "/internal/computers/events/replay?"+query.Encode(), nil, &rawPage, EventReplayMaxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("computer event client: replay page after sequence %d: %w", afterSequence, err)
	}
	page, err := DecodeHistoricDurableEvents(rawPage)
	if err != nil {
		return nil, fmt.Errorf("computer event client: replay page after sequence %d: %w", afterSequence, err)
	}
	if len(page) > pageSize {
		return nil, fmt.Errorf("computer event client: replay page returned %d records, exceeds requested %d", len(page), pageSize)
	}
	return page, nil
}

func (c *HTTPClient) Events(ctx context.Context, computerID string, afterSequence uint64) ([]DurableEvent, error) {
	pageSize := EventReplayPageSize
	after := afterSequence
	var records []DurableEvent
	for {
		page, err := c.EventsPage(ctx, computerID, after, pageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		previous := after
		for _, record := range page {
			sequence := record.Request.Event.Sequence
			if previous == ^uint64(0) || sequence != previous+1 {
				return nil, fmt.Errorf("computer event client: replay sequence did not progress after %d (got %d)", previous, sequence)
			}
			records = append(records, record)
			previous = sequence
		}
		after = previous
		if len(page) < pageSize || after == ^uint64(0) {
			break
		}
	}
	if records == nil {
		records = []DurableEvent{}
	}
	return records, nil
}

// DoJSON sends one authenticated JSON request to corpusd and decodes the
// answer; a non-2xx status returns the status and corpusd's error. It is the
// transport other corpusd routes on the guest's connection reuse (the host
// build route, P5).
func (c *HTTPClient) DoJSON(ctx context.Context, method, path string, body, response any) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("computer event client: not configured")
	}
	return c.do(ctx, method, path, body, response)
}

func (c *HTTPClient) do(ctx context.Context, method, path string, body any, response any) (int, error) {
	return c.doWithResponseLimit(ctx, method, path, body, response, 0)
}

// doWithResponseLimit keeps ordinary protocol responses on the historical
// streaming limit while allowing explicitly bounded large replay pages and
// payload envelopes to be read in full. A response that crosses that bound is
// rejected before JSON decoding, so truncation cannot masquerade as valid data.
func (c *HTTPClient) doWithResponseLimit(ctx context.Context, method, path string, body any, response any, maxResponseBytes int) (int, error) {
	c.countCall(path)
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = CanonicalJSON(body)
		if err != nil {
			return 0, err
		}
	}
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(c.baseURL.Path, "/") + strings.SplitN(path, "?", 2)[0]
	if queryIndex := strings.IndexByte(path, '?'); queryIndex >= 0 {
		endpoint.RawQuery = path[queryIndex+1:]
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return 0, err
	}
	token, err := c.capability(ctx)
	if err != nil {
		return 0, fmt.Errorf("computer event client: capability: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	// The guest client's transport (hostlink) resends a refused dial across
	// a host restart (host-deploy-restart-fails-in-flight-guest-turns-2026-10-10).
	result, err := c.http.Do(request)
	if err != nil {
		return 0, err
	}
	defer result.Body.Close()
	var responseReader io.Reader = io.LimitReader(result.Body, 1<<20)
	if maxResponseBytes > 0 {
		if result.ContentLength > int64(maxResponseBytes) {
			return result.StatusCode, fmt.Errorf("computer event client: response exceeds %d bytes", maxResponseBytes)
		}
		encoded, readErr := io.ReadAll(io.LimitReader(result.Body, int64(maxResponseBytes)+1))
		if readErr != nil {
			return result.StatusCode, fmt.Errorf("computer event client: read response: %w", readErr)
		}
		if len(encoded) > maxResponseBytes {
			return result.StatusCode, fmt.Errorf("computer event client: response exceeds %d bytes", maxResponseBytes)
		}
		responseReader = bytes.NewReader(encoded)
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(responseReader).Decode(&failure)
		if failure.Error == "" {
			failure.Error = http.StatusText(result.StatusCode)
		}
		return result.StatusCode, fmt.Errorf("computer event client: corpusd returned %d: %s", result.StatusCode, failure.Error)
	}
	if response != nil {
		decoder := json.NewDecoder(responseReader)
		decoder.UseNumber()
		if err := decoder.Decode(response); err != nil {
			return result.StatusCode, fmt.Errorf("computer event client: decode response: %w", err)
		}
	}
	return result.StatusCode, nil
}

// NewGuestHTTPClient permits authenticated, signed-receipt traffic over the
// Firecracker host-only RFC1918 tap. Public/non-private HTTP remains refused.

func NewGuestHTTPClient(baseURL string, capability CapabilitySource) (*HTTPClient, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme != "http" {
		return nil, fmt.Errorf("computer event client: invalid guest corpusd URL")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || (!ip.IsPrivate() && !ip.IsLoopback()) {
		return nil, fmt.Errorf("computer event client: guest HTTP requires a private host address")
	}
	return &HTTPClient{
		baseURL: parsed,
		// Host-tap traffic is fast, but a ~150MB blob fetch can exceed a tight
		// deadline under load; the request context remains the real bound.
		// hostlink resends a refused dial across a host restart.
		http:       hostlink.NewClient(10 * time.Minute),
		capability: capability,
	}, nil
}
