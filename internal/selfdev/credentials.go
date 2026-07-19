package selfdev

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// GuestCredentials owns the short-lived corpusd capability in guest-core
// memory. It renews before expiry and never exposes the token to agents,
// capsules, logs, durable state, or command arguments.
type GuestCredentials struct {
	mu                  sync.Mutex
	baseURL             string
	computerID          string
	realizationID       string
	http                *http.Client
	token               string
	postRevocationToken string
	expiresAt           time.Time
	keyID               string
	publicKey           ed25519.PublicKey
	pendingLifecycle    []computerevent.Receipt
}

func ExchangeGuestCredential(ctx context.Context, baseURL, encodedEnvelope, computerID, realizationID string) (*GuestCredentials, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encodedEnvelope))
	if err != nil {
		return nil, fmt.Errorf("guest credential: malformed bootstrap envelope")
	}
	var envelope platform.ComputerCredentialEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("guest credential: decode bootstrap envelope: %w", err)
	}
	canonical, err := computerevent.CanonicalJSON(envelope)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, fmt.Errorf("guest credential: non-canonical bootstrap envelope")
	}
	publicKey, err := envelope.VerifyBootstrap(computerID, realizationID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	manager := &GuestCredentials{
		baseURL: strings.TrimRight(baseURL, "/"), computerID: computerID, realizationID: realizationID,
		http: &http.Client{Timeout: 15 * time.Second}, publicKey: publicKey,
		keyID: envelope.SigningKeyID,
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, manager.baseURL+"/internal/computers/credentials/exchange", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/vnd.choir.computer-credential-envelope+json")
	response, err := manager.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("guest credential: exchange: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("guest credential: exchange refused")
	}
	var result platform.CredentialExchangeResult
	if err := json.NewDecoder(io.LimitReader(response.Body, 256<<10)).Decode(&result); err != nil {
		return nil, fmt.Errorf("guest credential: decode exchange: %w", err)
	}
	expiresAt, err := capabilityExpiry(result.Capability)
	postRevocationExpiresAt, postRevocationErr := capabilityExpiry(result.PostRevocationCapability)
	if err != nil || postRevocationErr != nil || !postRevocationExpiresAt.Equal(expiresAt) {
		return nil, fmt.Errorf("guest credential: invalid revocation handoff capability")
	}
	manager.token, manager.postRevocationToken, manager.expiresAt = result.Capability, result.PostRevocationCapability, expiresAt
	manager.pendingLifecycle = append([]computerevent.Receipt(nil), result.PendingLifecycleReceipts...)
	return manager, nil
}

func (g *GuestCredentials) Capability(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if time.Until(g.expiresAt) > 90*time.Second {
		return g.token, nil
	}
	body, err := computerevent.CanonicalJSON(map[string]string{"computer_id": g.computerID})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/internal/computers/credentials/renew", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+g.token)
	response, err := g.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("guest credential: renew: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("guest credential: renewal refused")
	}
	var result platform.CredentialExchangeResult
	if err := json.NewDecoder(io.LimitReader(response.Body, 256<<10)).Decode(&result); err != nil {
		return "", err
	}
	expiresAt, err := capabilityExpiry(result.Capability)
	if err != nil {
		return "", err
	}
	g.token, g.expiresAt = result.Capability, expiresAt
	return g.token, nil
}

func (g *GuestCredentials) PublishCheckpoint(ctx context.Context, checkpoint selfdevprotocol.CheckpointRequest) (selfdevprotocol.CheckpointResponse, error) {
	if g == nil || checkpoint.ComputerID != g.computerID {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("guest credential: checkpoint computer binding mismatch")
	}
	body, err := computerevent.CanonicalJSON(checkpoint)
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	token, err := g.Capability(ctx)
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/internal/computers/checkpoints", bytes.NewReader(body))
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := g.http.Do(request)
	if err != nil {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("guest credential: publish checkpoint: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("guest credential: checkpoint refused with status %d", response.StatusCode)
	}
	var result selfdevprotocol.CheckpointResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("guest credential: decode checkpoint: %w", err)
	}
	expectedCheckpoint, expectedErr := computerevent.CanonicalJSON(checkpoint)
	actualCheckpoint, actualErr := computerevent.CanonicalJSON(result.Checkpoint.Request)
	if expectedErr != nil || actualErr != nil || !bytes.Equal(expectedCheckpoint, actualCheckpoint) || result.Receipt.Kind != selfdevprotocol.ReceiptKindCheckpoint || result.Receipt.ComputerID != checkpoint.ComputerID || result.Receipt.ArtifactDigest != result.Checkpoint.Digest || result.Receipt.Verify(g.PublicKey()) != nil {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("guest credential: checkpoint receipt binding failed")
	}
	return result, nil
}

func (g *GuestCredentials) PublishRouteProjection(ctx context.Context, projection selfdevprotocol.RouteProjectionRequest) (selfdevprotocol.RouteProjectionResponse, error) {
	if g == nil || projection.ComputerID != g.computerID {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("guest credential: route projection computer binding mismatch")
	}
	body, err := computerevent.CanonicalJSON(projection)
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	token, err := g.Capability(ctx)
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/internal/computers/route-projection-certificates", bytes.NewReader(body))
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := g.http.Do(request)
	if err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("guest credential: publish route projection: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("guest credential: route projection refused with status %d", response.StatusCode)
	}
	var result selfdevprotocol.RouteProjectionResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 512<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("guest credential: decode route projection: %w", err)
	}
	expected, artifact, err := selfdevprotocol.RouteProjectionFromRequest(projection, result.Receipt.IssuedAt)
	if err != nil || result.Certificate != expected || result.Receipt.Kind != selfdevprotocol.ReceiptKindRouteProjection ||
		result.Receipt.ComputerID != projection.ComputerID || result.Receipt.ArtifactDigest != computerevent.DigestBytes(artifact) ||
		result.Receipt.Verify(g.PublicKey()) != nil {
		return selfdevprotocol.RouteProjectionResponse{}, fmt.Errorf("guest credential: route projection receipt binding failed")
	}
	return result, nil
}

func (g *GuestCredentials) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), g.publicKey...)
}

func (g *GuestCredentials) KeyResolver() PlatformKeyResolver {
	return PlatformKeyResolver{ComputerID: g.computerID, KeyID: g.keyID, PublicKey: g.PublicKey()}
}
func (g *GuestCredentials) PendingLifecycleReceipts() []computerevent.Receipt {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]computerevent.Receipt(nil), g.pendingLifecycle...)
}

func (g *GuestCredentials) AcknowledgePendingLifecycleReceipt(receiptID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for index := range g.pendingLifecycle {
		if g.pendingLifecycle[index].ReceiptID == receiptID {
			g.pendingLifecycle = append(g.pendingLifecycle[:index], g.pendingLifecycle[index+1:]...)
			return
		}
	}
}

func (g *GuestCredentials) ActivatePostRevocationCapability() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if strings.TrimSpace(g.postRevocationToken) == "" {
		return fmt.Errorf("guest credential: revocation handoff unavailable")
	}
	g.token, g.postRevocationToken = g.postRevocationToken, ""
	return nil
}

func capabilityExpiry(token string) (time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("guest credential: malformed capability")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("guest credential: malformed capability")
	}
	var capability struct {
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(payload, &capability); err != nil {
		return time.Time{}, fmt.Errorf("guest credential: malformed capability")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, capability.ExpiresAt)
	if err != nil || !time.Now().UTC().Before(expiresAt) {
		return time.Time{}, fmt.Errorf("guest credential: expired capability")
	}
	return expiresAt, nil
}

type PlatformKeyResolver struct {
	ComputerID string
	KeyID      string
	PublicKey  ed25519.PublicKey
}

func (r PlatformKeyResolver) ResolveReceiptKey(domain, computerID, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != "platform-control" || keyID != r.KeyID || (computerID != "" && computerID != r.ComputerID) || len(r.PublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("guest credential: receipt key refused")
	}
	return append(ed25519.PublicKey(nil), r.PublicKey...), nil
}
