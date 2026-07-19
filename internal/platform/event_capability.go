package platform

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const defaultComputerCapabilityTTL = 5 * time.Minute

var capabilityDomain = []byte("choir-computer-capability-v1\x00")

type ComputerCapability struct {
	Version         int      `json:"version"`
	ComputerID      string   `json:"computer_id"`
	Scopes          []string `json:"scopes"`
	ExpiresAt       string   `json:"expires_at"`
	RevocationEpoch uint64   `json:"revocation_epoch"`
	Nonce           string   `json:"nonce"`
}

type SignedCapabilityVerifier struct {
	Store     *Store
	PublicKey ed25519.PublicKey
	Now       func() time.Time
	MaxTTL    time.Duration
}

func MintComputerCapability(capability ComputerCapability, privateKey ed25519.PrivateKey) (string, error) {
	if capability.Version != 1 || capability.ComputerID == "" || len(capability.Scopes) == 0 || capability.ExpiresAt == "" || capability.Nonce == "" || len(privateKey) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("computer capability: incomplete capability")
	}
	if !validCapabilityScopes(capability.Scopes) {
		return "", fmt.Errorf("computer capability: invalid scope set")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, capability.ExpiresAt)
	if err != nil || expiresAt.Location() != time.UTC || expiresAt.Format(time.RFC3339Nano) != capability.ExpiresAt {
		return "", fmt.Errorf("computer capability: expiry must be canonical UTC RFC3339")
	}
	payload, err := computerevent.CanonicalJSON(capability)
	if err != nil {
		return "", err
	}
	signature := ed25519.Sign(privateKey, computerCapabilityPreimage(payload))
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (v SignedCapabilityVerifier) Authorize(r *http.Request, computerID, requiredScope string) error {
	if v.Store == nil || len(v.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("computer capability: verifier unavailable")
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return fmt.Errorf("computer capability: bearer required")
	}
	parts := strings.Split(strings.TrimPrefix(header, "Bearer "), ".")
	if len(parts) != 2 {
		return fmt.Errorf("computer capability: malformed token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fmt.Errorf("computer capability: malformed payload")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("computer capability: malformed signature")
	}
	if !ed25519.Verify(v.PublicKey, computerCapabilityPreimage(payload), signature) {
		return fmt.Errorf("computer capability: invalid signature")
	}
	var capability ComputerCapability
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&capability); err != nil {
		return fmt.Errorf("computer capability: invalid payload")
	}
	canonical, err := computerevent.CanonicalJSON(capability)
	if err != nil || !bytes.Equal(canonical, payload) {
		return fmt.Errorf("computer capability: non-canonical payload")
	}
	if capability.Version != 1 || capability.ComputerID != computerID {
		return fmt.Errorf("computer capability: computer scope mismatch")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, capability.ExpiresAt)
	if err != nil {
		return fmt.Errorf("computer capability: invalid expiry")
	}
	if expiresAt.Location() != time.UTC || expiresAt.Format(time.RFC3339Nano) != capability.ExpiresAt {
		return fmt.Errorf("computer capability: expiry must be canonical UTC RFC3339")
	}
	now := time.Now().UTC()
	if v.Now != nil {
		now = v.Now().UTC()
	}
	if !now.Before(expiresAt) {
		return fmt.Errorf("computer capability: expired")
	}
	maxTTL := v.MaxTTL
	if maxTTL <= 0 {
		maxTTL = defaultComputerCapabilityTTL
	}
	if expiresAt.Sub(now) > maxTTL {
		return fmt.Errorf("computer capability: expiry exceeds maximum lifetime")
	}
	if !validCapabilityScopes(capability.Scopes) {
		return fmt.Errorf("computer capability: invalid scope set")
	}
	if !containsCapabilityScope(capability.Scopes, requiredScope) {
		return fmt.Errorf("computer capability: scope refused")
	}
	head, err := readComputerEventHead(r.Context(), v.Store.db, computerID, false)
	if err != nil {
		return err
	}
	if head != nil && head.CredentialRevocationEpoch != capability.RevocationEpoch {
		return fmt.Errorf("computer capability: revoked")
	}
	if head == nil && capability.RevocationEpoch != 0 {
		return fmt.Errorf("computer capability: invalid pre-genesis epoch")
	}
	return nil
}

func computerCapabilityPreimage(payload []byte) []byte {
	preimage := make([]byte, 0, len(capabilityDomain)+len(payload))
	preimage = append(preimage, capabilityDomain...)
	return append(preimage, payload...)
}

func validCapabilityScopes(scopes []string) bool {
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		switch scope {
		case "event:read", "event:pin", "event:append":
		default:
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return len(scopes) > 0
}

func containsCapabilityScope(scopes []string, required string) bool {
	for _, scope := range scopes {
		if scope == required {
			return true
		}
	}
	return false
}
