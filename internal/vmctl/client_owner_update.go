package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// DeployRefreshEndpoint is vmctl's deploy-refresh endpoint.
func DeployRefreshEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/deploy-refresh"
}

// UpdateAvailableEndpoint is vmctl's host-side "updates available" read.
func UpdateAvailableEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/update-available"
}

// OwnerUpdateResult is the structured answer of an owner update. Outcome is
// refreshed, current, deferred or skipped; Refusal is the guest's busy
// refusal, unparsed.
type OwnerUpdateResult struct {
	Outcome    string          `json:"outcome"`
	Reason     string          `json:"reason,omitempty"`
	ReasonCode string          `json:"reason_code,omitempty"`
	Refusal    json.RawMessage `json:"refusal,omitempty"`
	Target     string          `json:"target,omitempty"`
}

// OwnerUpdateContext asks vmctl for the owner update of one computer (a
// deploy refresh with owner_busy). A 409 deferral or skip is a structured
// answer, not an error; only transport failures and unstructured statuses are.
func (c *Client) OwnerUpdateContext(ctx context.Context, userID, desktopID string) (*OwnerUpdateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	data, err := json.Marshal(deployRefreshRequest{UserID: userID, DesktopID: normalizeDesktopID(desktopID), OwnerBusy: true})
	if err != nil {
		return nil, fmt.Errorf("vmctl client: marshal owner update request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, DeployRefreshEndpoint(c.baseURL), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create owner update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	hostauth.MarkInternal(req)
	setLifecycleCaller(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: owner update call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return nil, fmt.Errorf("vmctl client: read owner update response: %w", err)
	}
	var result OwnerUpdateResult
	structured := json.Unmarshal(body, &result) == nil && result.Outcome != ""
	switch resp.StatusCode {
	case http.StatusOK, http.StatusConflict, http.StatusNotFound, http.StatusInternalServerError:
		if structured {
			return &result, nil
		}
		if resp.StatusCode == http.StatusConflict {
			// An unstructured 409 is a refusal before the refresh: a skip with its reason.
			var plain vmctlErrorResponse
			if json.Unmarshal(body, &plain) == nil && plain.Error != "" {
				return &OwnerUpdateResult{Outcome: "skipped", Reason: plain.Error, ReasonCode: ownerReasonRetryLater}, nil
			}
		}
	}
	if refusal := decodeRecoveryRefusal(resp.StatusCode, body, resp.Header.Get("Retry-After")); refusal != nil {
		return nil, refusal
	}
	if structured {
		return nil, fmt.Errorf("vmctl client: owner update %s: %s %s", resp.Status, result.Outcome, result.Reason)
	}
	return nil, fmt.Errorf("vmctl client: owner update failed with status %s", resp.Status)
}

// UpdateAvailable reads vmctl's pending-tracker signal for a computer.
func (c *Client) UpdateAvailable(ctx context.Context, userID, desktopID string) (*UpdateAvailableResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := UpdateAvailableEndpoint(c.baseURL) + "?user_id=" + url.QueryEscape(userID) + "&desktop_id=" + url.QueryEscape(normalizeDesktopID(desktopID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: create update-available request: %w", err)
	}
	hostauth.MarkInternal(req)
	setLifecycleCaller(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vmctl client: update-available call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("vmctl client: read update-available response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vmctl client: update-available status %s", resp.Status)
	}
	var result UpdateAvailableResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("vmctl client: decode update-available response: %w", err)
	}
	return &result, nil
}
