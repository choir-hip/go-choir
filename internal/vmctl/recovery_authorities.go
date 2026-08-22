package vmctl

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

// HTTPRecoveryHeadReader reads the canonical event head through the internal
// corpusd product endpoint. It never appends events.
type HTTPRecoveryHeadReader struct {
	BaseURL string
	Client  *http.Client
}

func (h HTTPRecoveryHeadReader) CanonicalHead(ctx context.Context, computerID string, token RecoveryFencingToken) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(h.BaseURL), "/")
	if base == "" || strings.TrimSpace(computerID) == "" || token.ComputerID != computerID || token.OwnerID == "" {
		return "", fmt.Errorf("recovery head reader: invalid binding")
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/internal/computers/events/head?computer_id="+url.QueryEscape(computerID), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("X-Internal-Caller", "true")
	request.Header.Set("X-Authenticated-User", token.OwnerID)
	request.Header.Set("X-Recovery-Operation", "recover_current")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("recovery head reader: corpusd status %s", response.Status)
	}
	var head computerevent.Head
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	if err := decoder.Decode(&head); err != nil || head.ComputerID != computerID || !computerevent.IsSHA256(head.CanonicalEventHead) {
		return "", fmt.Errorf("recovery head reader: invalid head")
	}
	return head.CanonicalEventHead, nil
}

// HTTPRecoveryVerifier performs the owner-scoped post-boot replay probe and
// checks the guest's serving identity before vmctl publishes the route.
type HTTPRecoveryVerifier struct {
	Client      *http.Client
	GuestURLFor func(computerID string) (string, error)
}

func (v HTTPRecoveryVerifier) VerifyRecovery(ctx context.Context, identity ColdRecoveryIdentity) (string, error) {
	if identity.Token.ComputerID == "" || identity.Token.OwnerID == "" || v.GuestURLFor == nil {
		return "", fmt.Errorf("recovery verifier: invalid authority")
	}
	guestURL, err := v.GuestURLFor(identity.Token.ComputerID)
	if err != nil {
		return "", err
	}
	guestURL = strings.TrimRight(strings.TrimSpace(guestURL), "/")
	if guestURL == "" {
		return "", fmt.Errorf("recovery verifier: guest URL unavailable")
	}
	client := v.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, guestURL+"/api/computers/"+url.PathEscape(identity.Token.ComputerID)+"/self-development/replay-completeness", nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("X-Authenticated-User", identity.Token.OwnerID)
	request.Header.Set("X-Authenticated-Computer", identity.Token.ComputerID)
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("recovery verifier: replay status %s", response.Status)
	}
	var report struct {
		LiveHead   *computerevent.Head `json:"live_head"`
		ReplayHead *computerevent.Head `json:"replay_head"`
		Result     struct {
			Status string `json:"status"`
		} `json:"result"`
		Eligibility struct {
			Eligible bool `json:"eligible"`
		} `json:"eligibility"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&report); err != nil {
		return "", fmt.Errorf("recovery verifier: decode replay report: %w", err)
	}
	if !report.Eligibility.Eligible || report.Result.Status != "equivalent" || report.LiveHead == nil || report.ReplayHead == nil || report.LiveHead.CanonicalEventHead != report.ReplayHead.CanonicalEventHead || !computerevent.IsSHA256(report.LiveHead.CanonicalEventHead) {
		return "", fmt.Errorf("recovery verifier: replay equivalence refused")
	}
	observability, err := http.NewRequestWithContext(ctx, http.MethodGet, guestURL+"/api/runtime/observability", nil)
	if err != nil {
		return "", err
	}
	observability.Header.Set("X-Authenticated-User", identity.Token.OwnerID)
	observability.Header.Set("X-Authenticated-Computer", identity.Token.ComputerID)
	health, err := client.Do(observability)
	if err != nil {
		return "", err
	}
	defer health.Body.Close()
	if health.StatusCode != http.StatusOK {
		return "", fmt.Errorf("recovery verifier: guest observability status %s", health.Status)
	}
	var guest struct {
		ComputerID string `json:"computer_id"`
		Service    string `json:"service"`
	}
	if err := json.NewDecoder(io.LimitReader(health.Body, 64<<10)).Decode(&guest); err != nil || guest.ComputerID != identity.Token.ComputerID || guest.Service != "autoputer" {
		return "", fmt.Errorf("recovery verifier: guest serving identity refused")
	}
	return report.LiveHead.CanonicalEventHead, nil
}
