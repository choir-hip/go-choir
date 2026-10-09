package autoputer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

// errPrivacyKeyUndeliverable: the platform deterministically refused key
// delivery (no escrow, superseded issuance, no chain, or proof refused).
// Other delivery failures are transient.
var errPrivacyKeyUndeliverable = errors.New("privacy key delivery refused")

// errPrivacyKeyNotEscrowed: the platform holds no custodian escrow for this
// computer — no key source exists (wraps errPrivacyKeyUndeliverable).
var errPrivacyKeyNotEscrowed = fmt.Errorf("%w: no custodian escrow", errPrivacyKeyUndeliverable)

// deliverPrivacyKey installs the computer's privacy key from custodian escrow
// on a realization whose key file is missing over an existing chain (station
// SH slice 3, O21). The ephemeral recipient key exists only in memory; the
// proof is this realization's issuance capability and envelope commitment.
// A guest that crashes before installing simply asks again with the same
// proof.
func deliverPrivacyKey(ctx context.Context, platformURL, computerID, commitment, issuanceToken, keyPath string) error {
	recipientPrivate, recipientPublic, err := keyescrow.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("privacy key delivery: recipient key: %w", err)
	}
	defer clear(recipientPrivate[:])
	body, err := json.Marshal(map[string]string{
		"computer_id": computerID, "request_commitment": commitment,
		"recipient_public_key": base64.RawStdEncoding.EncodeToString(recipientPublic[:]),
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(platformURL, "/")+"/internal/computers/keys/realization-delivery", bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+issuanceToken)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return fmt.Errorf("privacy key delivery: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return fmt.Errorf("privacy key delivery: read response: %w", err)
	}
	switch {
	case response.StatusCode == http.StatusOK:
	case response.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s", errPrivacyKeyNotEscrowed, strings.TrimSpace(string(payload)))
	case response.StatusCode >= 400 && response.StatusCode < 500:
		return fmt.Errorf("%w: status %d: %s", errPrivacyKeyUndeliverable, response.StatusCode, strings.TrimSpace(string(payload)))
	default:
		return fmt.Errorf("privacy key delivery: status %d", response.StatusCode)
	}
	var delivered struct {
		WrappedKey string `json:"wrapped_key"`
		KeyDigest  string `json:"key_digest"`
	}
	if err := json.Unmarshal(payload, &delivered); err != nil {
		return fmt.Errorf("%w: decode response", errPrivacyKeyUndeliverable)
	}
	record, err := keyescrow.ParseWrappedKey([]byte(delivered.WrappedKey))
	if err != nil {
		return fmt.Errorf("%w: invalid sealed key", errPrivacyKeyUndeliverable)
	}
	dek, err := keyescrow.OpenDEK(recipientPrivate, record, computerID)
	if err != nil {
		return fmt.Errorf("%w: open sealed key: %v", errPrivacyKeyUndeliverable, err)
	}
	defer clear(dek)
	if fmt.Sprintf("%x", sha256.Sum256(dek)) != delivered.KeyDigest {
		return fmt.Errorf("%w: delivered key digest mismatch", errPrivacyKeyUndeliverable)
	}
	if err := computerevent.InstallGuestPrivacyKey(keyPath, computerID, dek); err != nil {
		return fmt.Errorf("privacy key delivery: install: %w", err)
	}
	return nil
}
