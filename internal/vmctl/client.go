package vmctl

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
)

// Client is an HTTP client for the vmctl service. The proxy uses this
// client to resolve user VM ownership before routing requests.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

const DefaultClientTimeout = 60 * time.Second

// ErrComputerLookupIdentityMismatch means vmctl returned a stable ComputerID
// other than the exact identity requested. Callers must classify it as denied
// authority, not as a retryable control-plane outage.
var ErrComputerLookupIdentityMismatch = errors.New("vmctl computer lookup identity mismatch")

// RecoveryRefusalError is the typed view of a structured 503 recovery refusal
// (durable recovery condition). It is deterministic for its inputs: callers
// must not retry it inside a transient retry window. It clears only when the
// recovery inputs change or the repair job advances the advertised base.
type RecoveryRefusalError struct {
	Reason            string
	Kind              string
	RetryAfterSeconds int
	Witness           *RecoveryInputWitness
	Repair            *CheckpointJobStatus
}

func (e *RecoveryRefusalError) Error() string {
	if e == nil {
		return "vmctl client: resolve blocked"
	}
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = "computer recovery is blocked"
	}
	return fmt.Sprintf("vmctl client: resolve blocked (%s): %s", e.Kind, reason)
}

// decodeRecoveryRefusal parses the structured 503 envelope. A non-503 or a
// body without a refusal kind returns nil so ordinary errors keep their path.
func decodeRecoveryRefusal(status int, body []byte, retryAfterHeader string) *RecoveryRefusalError {
	if status != http.StatusServiceUnavailable {
		return nil
	}
	var envelope struct {
		Error             string                `json:"error"`
		Reason            string                `json:"reason"`
		Kind              string                `json:"kind"`
		RetryAfterSeconds int                   `json:"retry_after_seconds"`
		Witness           *RecoveryInputWitness `json:"witness"`
		Repair            *CheckpointJobStatus  `json:"repair"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil
	}
	kind := strings.TrimSpace(envelope.Kind)
	if kind == "" {
		return nil
	}
	retryAfter := envelope.RetryAfterSeconds
	if retryAfter <= 0 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(retryAfterHeader)); err == nil {
			retryAfter = parsed
		}
	}
	if retryAfter <= 0 {
		retryAfter = recoveryRetryAfterSeconds
	}
	return &RecoveryRefusalError{
		Reason:            envelope.Reason,
		Kind:              kind,
		RetryAfterSeconds: retryAfter,
		Witness:           envelope.Witness,
		Repair:            envelope.Repair,
	}
}

// NewClient creates a vmctl client pointing at the given base URL.
func NewClient(baseURL string) *Client {
	return NewClientWithTimeout(baseURL, DefaultClientTimeout)
}

// NewClientWithTimeout creates a vmctl client with an explicit request timeout.
func NewClientWithTimeout(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultClientTimeout
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Resolve resolves or assigns a VM for the given user ID. Returns the
// ownership information including the autoputer URL where the user's VM
// is reachable (VAL-VM-001).
func (c *Client) Resolve(userID string) (*resolveResponse, error) {
	return c.ResolveDesktop(userID, PrimaryDesktopID)
}

// ResolveDesktop resolves or assigns a VM for the given user/desktop pair.
func (c *Client) ResolveDesktop(userID, desktopID string) (*resolveResponse, error) {
	return c.ResolveDesktopContext(context.Background(), userID, desktopID)
}

// ResolveDesktopContext resolves or assigns a VM for the given user/desktop
// pair and cancels the vmctl request when ctx is done.
func (c *Client) ResolveDesktopContext(ctx context.Context, userID, desktopID string) (*resolveResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reqBody := resolveRequest{UserID: userID, DesktopID: normalizeDesktopID(desktopID)}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: marshal resolve request: %w", err)
	}

	endpoint := ResolveEndpoint(c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create resolve request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: resolve call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: read resolve response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if refusal := decodeRecoveryRefusal(resp.StatusCode, body, resp.Header.Get("Retry-After")); refusal != nil {
			return nil, refusal
		}
		var errResp vmctlErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("vmctl client: resolve failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("vmctl client: resolve failed with status %s", resp.Status)
	}

	var result resolveResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode resolve response: %w", err)
	}

	return &result, nil
}

// ColdRecover requests a recover_current transition for an owner-authorized,
// inactive computer. The caller supplies both optimistic concurrency values;
// vmctl independently re-derives owner, VM, and route authority.
func (c *Client) ColdRecover(ctx context.Context, computerID, expectedCanonicalHead string, expectedRouteGeneration uint64, idempotencyKey string) (*ColdRecoverResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	requestBody := ColdRecoverRequest{
		ComputerID:              strings.TrimSpace(computerID),
		ExpectedCanonicalHead:   strings.TrimSpace(expectedCanonicalHead),
		ExpectedRouteGeneration: expectedRouteGeneration,
		IdempotencyKey:          strings.TrimSpace(idempotencyKey),
	}
	data, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: marshal cold recovery request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, ColdRecoverEndpoint(c.baseURL, requestBody.ComputerID), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create cold recovery request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Internal-Caller", "true")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: cold recovery call failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: read cold recovery response: %w", err)
	}
	if response.StatusCode != http.StatusAccepted {
		var errorResponse vmctlErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.Error != "" {
			return nil, fmt.Errorf("vmctl client: cold recovery failed: %s", errorResponse.Error)
		}
		return nil, fmt.Errorf("vmctl client: cold recovery failed with status %s", response.Status)
	}
	var result ColdRecoverResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode cold recovery response: %w", err)
	}
	return &result, nil
}

// RefreshDesktopContext force-refreshes an existing user/desktop VM while
// preserving its persistent data image.
func (c *Client) RefreshDesktopContext(ctx context.Context, userID, desktopID string) (*resolveResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reqBody := resolveRequest{UserID: userID, DesktopID: normalizeDesktopID(desktopID)}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: marshal refresh request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, RefreshEndpoint(c.baseURL), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: refresh call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if refusal := decodeRecoveryRefusal(resp.StatusCode, body, resp.Header.Get("Retry-After")); refusal != nil {
			return nil, refusal
		}
		var errResp vmctlErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("vmctl client: refresh failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("vmctl client: refresh failed with status %s", resp.Status)
	}

	var result resolveResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode refresh response: %w", err)
	}
	return &result, nil
}

// Unhold clears the host-authoritative maintenance hold for a computer. The
// proxy is the trusted internal caller; public callers must authorise the exact
// ComputerID before reaching this. ClearHold is idempotent: it succeeds when
// the computer is not held.
func (c *Client) Unhold(ctx context.Context, computerID string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	reqBody := holdRequest{ComputerID: computerID}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("vmctl client: marshal unhold request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, UnholdEndpoint(c.baseURL), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("vmctl client: create unhold request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vmctl client: unhold call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("vmctl client: read unhold response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp vmctlErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return fmt.Errorf("vmctl client: unhold failed: %s", errResp.Error)
		}
		return fmt.Errorf("vmctl client: unhold failed with status %s", resp.Status)
	}
	return nil
}

// Lookup returns the current ownership for a user without creating a VM.
// Returns nil if no ownership exists.
func (c *Client) Lookup(userID string) (*ownershipResponse, error) {
	return c.LookupDesktop(userID, PrimaryDesktopID)
}

// LookupDesktop returns the current ownership for a user/desktop pair without
// creating a VM. Returns nil if no ownership exists.
func (c *Client) LookupDesktop(userID, desktopID string) (*ownershipResponse, error) {
	return c.LookupDesktopContext(context.Background(), userID, desktopID)
}

// LookupDesktopContext returns the current ownership for a user/desktop pair
// and cancels the vmctl request when ctx is done.
func (c *Client) LookupDesktopContext(ctx context.Context, userID, desktopID string) (*ownershipResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := LookupEndpoint(c.baseURL) + "?user_id=" + url.QueryEscape(userID) + "&desktop_id=" + url.QueryEscape(normalizeDesktopID(desktopID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create lookup request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: lookup call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: read lookup response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		var errResp vmctlErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("vmctl client: lookup failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("vmctl client: lookup failed with status %s", resp.Status)
	}

	var result ownershipResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode lookup response: %w", err)
	}

	return &result, nil
}

func (c *Client) LookupComputerContext(ctx context.Context, userID, computerID string) (*ownershipResponse, error) {
	return c.lookupComputerContext(ctx, strings.TrimSpace(userID), computerID)
}

// LookupComputerByIDContext resolves an ownership by stable ComputerID without
// imposing a user-ID join. It is an internal control-plane primitive; public
// callers must authorize the exact ComputerID before using it.
func (c *Client) LookupComputerByIDContext(ctx context.Context, computerID string) (*ownershipResponse, error) {
	return c.lookupComputerContext(ctx, "", computerID)
}

func (c *Client) lookupComputerContext(ctx context.Context, userID, computerID string) (*ownershipResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	computerID = strings.TrimSpace(computerID)
	query := url.Values{"computer_id": []string{computerID}}
	if userID != "" {
		query.Set("user_id", userID)
	}
	endpoint := LookupEndpoint(c.baseURL) + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create computer lookup request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: computer lookup call failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vmctl client: computer lookup status %s", resp.Status)
	}
	var result ownershipResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode computer lookup response: %w", err)
	}
	if result.ComputerID != computerID {
		return nil, fmt.Errorf("%w: requested %q", ErrComputerLookupIdentityMismatch, computerID)
	}
	return &result, nil
}

// GuestOwnership is the live ownership bound to a guest tap source address.
// Found is false when the source does not belong to any current computer.
type GuestOwnership struct {
	Found       bool   `json:"found"`
	UserID      string `json:"user_id"`
	DesktopID   string `json:"desktop_id"`
	ComputerID  string `json:"computer_id"`
	ComputerURL string `json:"computer_url"`
	VMID        string `json:"vm_id"`
}

// LookupGuestContext resolves the ownership for a guest source RemoteAddr
// without creating or modifying a VM.
func (c *Client) LookupGuestContext(ctx context.Context, remoteAddr string) (*GuestOwnership, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	body, err := json.Marshal(map[string]string{"remote_addr": strings.TrimSpace(remoteAddr)})
	if err != nil {
		return nil, fmt.Errorf("vmctl client: marshal guest lookup request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, LookupGuestEndpoint(c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create guest lookup request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: guest lookup call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vmctl client: guest lookup failed with status %s", resp.Status)
	}
	var result GuestOwnership
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode guest lookup response: %w", err)
	}
	if !result.Found {
		return nil, nil
	}
	return &result, nil
}

// ListOwnershipsContext returns current ownership records for internal proxy
// inspection. Callers must filter the result before exposing it to users.
func (c *Client) ListOwnershipsContext(ctx context.Context) ([]ownershipResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ListEndpoint(c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create list request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: list call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: read list response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp vmctlErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("vmctl client: list failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("vmctl client: list failed with status %s", resp.Status)
	}

	var result struct {
		Ownerships []ownershipResponse `json:"ownerships"`
		Count      int                 `json:"count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode list response: %w", err)
	}
	return result.Ownerships, nil
}

// PulseSummaryContext returns the public-safe aggregate Pulse summary from
// vmctl. The response contains no raw user IDs or email addresses.
func (c *Client) PulseSummaryContext(ctx context.Context) (*PulseSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, PulseEndpoint(c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create pulse request: %w", err)
	}
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: pulse call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: read pulse response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var errResp vmctlErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("vmctl client: pulse failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("vmctl client: pulse failed with status %s", resp.Status)
	}

	var result PulseSummary
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode pulse response: %w", err)
	}
	return &result, nil
}

// Stop requests vmctl to stop the VM for the given user.
func (c *Client) Stop(userID string) error {
	return c.StopDesktop(userID, PrimaryDesktopID)
}

// Remove requests vmctl to remove the ownership for the given user.
func (c *Client) Remove(userID string) error {
	return c.RemoveDesktop(userID, PrimaryDesktopID)
}

// StopDesktop requests vmctl to stop the VM for the given user/desktop pair.
func (c *Client) StopDesktop(userID, desktopID string) error {
	return c.postAction(StopEndpoint(c.baseURL), userID, desktopID)
}

// RemoveDesktop requests vmctl to remove the ownership for the given
// user/desktop pair.
func (c *Client) RemoveDesktop(userID, desktopID string) error {
	return c.postAction(RemoveEndpoint(c.baseURL), userID, desktopID)
}

// postAction sends a POST request with a user_id/desktop_id body to the given
// endpoint.
func (c *Client) postAction(endpoint, userID, desktopID string) error {
	reqBody := resolveRequest{UserID: userID, DesktopID: normalizeDesktopID(desktopID)}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("vmctl client: marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("vmctl client: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vmctl client: call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.ReadAll(resp.Body) // drain body for connection reuse

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vmctl client: action failed with status %s", resp.Status)
	}

	return nil
}
