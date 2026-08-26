package autoputer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

// keyEscrowClient performs the lazy per-boot custodian escrow upgrade
// (Track K): if the platform has no custodian wrap for this computer, the
// guest seals its DEK under the host escrow public key and uploads it.
// Failure is logged and retried on the next boot; escrow must never block a
// boot (design: docs/designs/choir-durable-substrate-2026-08-23.md §3.2).
type keyEscrowClient struct {
	platformURL string
	httpClient  *http.Client
}

func newKeyEscrowClient(platformURL string) *keyEscrowClient {
	return &keyEscrowClient{
		platformURL: strings.TrimRight(platformURL, "/"),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

type keyEscrowStatusResponse struct {
	Escrows []struct {
		Protector string `json:"protector"`
		KeyDigest string `json:"key_digest"`
	} `json:"escrows"`
}

type keyEscrowPublicKeyResponse struct {
	PublicKey string `json:"public_key"`
}

type keyEscrowPutRequest struct {
	ComputerID string `json:"computer_id"`
	Protector  string `json:"protector"`
	WrappedKey string `json:"wrapped_key"`
	KeyDigest  string `json:"key_digest"`
}

// EnsureCustodianEscrow seals rawDEK under the host escrow public key and
// uploads the wrap when the platform lacks a custodian record. Returns true
// when the wrap is present after the call (pre-existing or newly uploaded).
func (c *keyEscrowClient) EnsureCustodianEscrow(ctx context.Context, computerID string, rawDEK []byte) (bool, error) {
	escrowed, err := c.custodianEscrowed(ctx, computerID)
	if err != nil {
		return false, err
	}
	if escrowed {
		return true, nil
	}
	pubKeyB64, err := c.fetchPublicKey(ctx)
	if err != nil {
		return false, err
	}
	var pub keyescrow.PublicKey
	decoded, err := base64.RawStdEncoding.DecodeString(pubKeyB64)
	if err != nil || len(decoded) != len(pub) {
		return false, fmt.Errorf("key escrow: invalid host public key")
	}
	copy(pub[:], decoded)
	record, err := keyescrow.SealDEK(pub, computerID, rawDEK)
	if err != nil {
		return false, fmt.Errorf("key escrow: seal dek: %w", err)
	}
	wrapped, err := json.Marshal(record)
	if err != nil {
		return false, fmt.Errorf("key escrow: encode wrap: %w", err)
	}
	body, err := json.Marshal(keyEscrowPutRequest{
		ComputerID: computerID,
		Protector:  keyescrow.ProtectorCustodian,
		WrappedKey: string(wrapped),
		KeyDigest:  record.KeyDigest,
	})
	if err != nil {
		return false, fmt.Errorf("key escrow: encode request: %w", err)
	}
	if err := c.do(ctx, http.MethodPut, "/internal/computers/keys/escrow", body); err != nil {
		return false, err
	}
	log.Printf("autoputer: custodian key escrow uploaded for %s", computerID)
	return true, nil
}

func (c *keyEscrowClient) custodianEscrowed(ctx context.Context, computerID string) (bool, error) {
	body, err := c.get(ctx, "/internal/computers/keys/escrow/status?computer_id="+computerID)
	if err != nil {
		return false, err
	}
	var status keyEscrowStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return false, fmt.Errorf("key escrow: decode status: %w", err)
	}
	for _, escrow := range status.Escrows {
		if escrow.Protector == keyescrow.ProtectorCustodian {
			return true, nil
		}
	}
	return false, nil
}

func (c *keyEscrowClient) fetchPublicKey(ctx context.Context) (string, error) {
	body, err := c.get(ctx, "/internal/computers/keys/escrow-public-key")
	if err != nil {
		return "", err
	}
	var pub keyEscrowPublicKeyResponse
	if err := json.Unmarshal(body, &pub); err != nil {
		return "", fmt.Errorf("key escrow: decode public key: %w", err)
	}
	if strings.TrimSpace(pub.PublicKey) == "" {
		return "", fmt.Errorf("key escrow: empty host public key")
	}
	return pub.PublicKey, nil
}

func (c *keyEscrowClient) get(ctx context.Context, path string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.platformURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("key escrow: build request: %w", err)
	}
	return c.roundTrip(request)
}

func (c *keyEscrowClient) do(ctx context.Context, method, path string, body []byte) error {
	request, err := http.NewRequestWithContext(ctx, method, c.platformURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("key escrow: build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	_, err = c.roundTrip(request)
	return err
}

func (c *keyEscrowClient) roundTrip(request *http.Request) ([]byte, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("key escrow: %s %s: %w", request.Method, request.URL.Path, err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("key escrow: read response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("key escrow: %s %s: status %d", request.Method, request.URL.Path, response.StatusCode)
	}
	return payload, nil
}
