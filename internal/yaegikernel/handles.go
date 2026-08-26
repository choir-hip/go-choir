package yaegikernel

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// HandleData represents the inner subject-bound capability token metadata.
type HandleData struct {
	ComputerID   string         `json:"computer_id"`
	ActorProfile string         `json:"actor_profile"`
	Epoch        uint64         `json:"epoch"`
	Scopes       []BrokerAction `json:"scopes"`
	ExpiresAt    time.Time      `json:"expires_at"`
	Nonce        string         `json:"nonce"`
}

// HandleIssuer signs and verifies opaque, subject-bound capability handles.
type HandleIssuer struct {
	secretKey []byte
}

// NewHandleIssuer creates a new issuer with the given 32-byte secret key.
func NewHandleIssuer(secretKey []byte) (*HandleIssuer, error) {
	if len(secretKey) < 32 {
		return nil, fmt.Errorf("handles: secret key must be at least 32 bytes")
	}
	keyCopy := append([]byte(nil), secretKey...)
	return &HandleIssuer{secretKey: keyCopy}, nil
}

// Issue creates a signed opaque handle string for an activation.
func (i *HandleIssuer) Issue(computerID, actorProfile string, epoch uint64, scopes []BrokerAction, ttl time.Duration) (string, error) {
	if i == nil || len(i.secretKey) == 0 {
		return "", fmt.Errorf("handles: uninitialized issuer")
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return "", fmt.Errorf("handles: computer_id is required")
	}
	if epoch == 0 {
		return "", fmt.Errorf("handles: epoch must be positive")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	nonceBytes := make([]byte, 8)
	_, _ = rand.Read(nonceBytes)

	data := HandleData{
		ComputerID:   computerID,
		ActorProfile: actorProfile,
		Epoch:        epoch,
		Scopes:       scopes,
		ExpiresAt:    time.Now().UTC().Add(ttl),
		Nonce:        hex.EncodeToString(nonceBytes),
	}

	rawJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("handles: marshal handle: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(rawJSON)
	sig := i.computeHMAC(rawJSON)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	return fmt.Sprintf("hnd_%s.%s", payloadB64, sigB64), nil
}

// Verify validates the handle signature, expiration, computer binding, epoch, and requested scope.
func (i *HandleIssuer) Verify(handleRef string, requiredComputer string, currentEpoch uint64, requestedAction BrokerAction) (*HandleData, error) {
	if i == nil || len(i.secretKey) == 0 {
		return nil, fmt.Errorf("handles: uninitialized issuer")
	}
	if !strings.HasPrefix(handleRef, "hnd_") {
		return nil, fmt.Errorf("handles: invalid handle prefix")
	}

	parts := strings.Split(strings.TrimPrefix(handleRef, "hnd_"), ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("handles: malformed handle format")
	}

	rawJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("handles: invalid payload base64: %w", err)
	}

	providedSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("handles: invalid signature base64: %w", err)
	}

	expectedSig := i.computeHMAC(rawJSON)
	if !hmac.Equal(providedSig, expectedSig) {
		return nil, fmt.Errorf("handles: signature mismatch: invalid handle token")
	}

	var data HandleData
	if err := json.NewDecoder(strings.NewReader(string(rawJSON))).Decode(&data); err != nil {
		return nil, fmt.Errorf("handles: decode handle payload: %w", err)
	}

	if time.Now().UTC().After(data.ExpiresAt) {
		return nil, fmt.Errorf("handles: handle has expired (expired at %s)", data.ExpiresAt.Format(time.RFC3339))
	}

	if requiredComputer != "" && data.ComputerID != requiredComputer {
		return nil, fmt.Errorf("handles: computer mismatch: handle issued for %q, caller requested for %q", data.ComputerID, requiredComputer)
	}

	if currentEpoch > 0 && data.Epoch != currentEpoch {
		return nil, fmt.Errorf("handles: stale activation epoch: handle epoch=%d, current epoch=%d", data.Epoch, currentEpoch)
	}

	if requestedAction != "" {
		allowed := false
		for _, s := range data.Scopes {
			if s == requestedAction {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("handles: action %q is not authorized under handle scopes %v", requestedAction, data.Scopes)
		}
	}

	return &data, nil
}

func (i *HandleIssuer) computeHMAC(data []byte) []byte {
	mac := hmac.New(sha256.New, i.secretKey)
	mac.Write(data)
	return mac.Sum(nil)
}
