package selfdev

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

type restartCredentialHandoff struct {
	Version                  int                                    `json:"version"`
	BaseURL                  string                                 `json:"base_url"`
	ComputerID               string                                 `json:"computer_id"`
	RealizationID            string                                 `json:"realization_id"`
	Capability               string                                 `json:"capability"`
	PostRevocationCapability string                                 `json:"post_revocation_capability,omitempty"`
	ExpiresAt                string                                 `json:"expires_at"`
	KeyID                    string                                 `json:"key_id"`
	PublicKey                string                                 `json:"public_key"`
	PendingLifecycleReceipts []computerevent.Receipt                `json:"pending_lifecycle_receipts,omitempty"`
	PendingConsumption       *platform.CredentialConsumptionRequest `json:"pending_consumption,omitempty"`
}

func (g *GuestCredentials) ConfigureRecoveryHandoff(ctx context.Context, path string) error {
	path = filepath.Clean(path)
	if g == nil || !filepath.IsAbs(path) {
		return fmt.Errorf("guest credential: invalid recovery handoff path")
	}
	if _, err := g.Capability(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.recoveryHandoffPath = path
	return g.writeRestartHandoffLocked(path)
}

func (g *GuestCredentials) WriteRestartHandoff(ctx context.Context, path string) error {
	if g == nil || !filepath.IsAbs(filepath.Clean(path)) {
		return fmt.Errorf("guest credential: invalid restart handoff path")
	}
	if _, err := g.Capability(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.writeRestartHandoffLocked(filepath.Clean(path))
}

func (g *GuestCredentials) writeRestartHandoffLocked(path string) error {
	handoff := restartCredentialHandoff{
		Version: 1, BaseURL: g.baseURL, ComputerID: g.computerID, RealizationID: g.realizationID,
		Capability: g.token, PostRevocationCapability: g.postRevocationToken,
		ExpiresAt: g.expiresAt.UTC().Format(time.RFC3339Nano), KeyID: g.keyID,
		PublicKey:                base64.RawStdEncoding.EncodeToString(g.publicKey),
		PendingLifecycleReceipts: append([]computerevent.Receipt(nil), g.pendingLifecycle...),
		PendingConsumption: func() *platform.CredentialConsumptionRequest {
			if g.pendingConsumption == nil {
				return nil
			}
			copy := *g.pendingConsumption
			return &copy
		}(),
	}
	canonical, err := computerevent.CanonicalJSON(handoff)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".restart-credential-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o400); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(canonical); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func RestoreGuestCredentials(path, baseURL, computerID, realizationID string) (*GuestCredentials, error) {
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o400 {
		return nil, fmt.Errorf("guest credential: restart handoff unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var handoff restartCredentialHandoff
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&handoff); err != nil {
		return nil, fmt.Errorf("guest credential: invalid restart handoff")
	}
	canonical, err := computerevent.CanonicalJSON(handoff)
	if err != nil || !bytes.Equal(canonical, raw) || handoff.Version != 1 ||
		handoff.BaseURL != strings.TrimRight(baseURL, "/") || handoff.ComputerID != computerID || handoff.RealizationID != realizationID {
		return nil, fmt.Errorf("guest credential: restart handoff binding mismatch")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, handoff.ExpiresAt)
	publicKey, keyErr := base64.RawStdEncoding.DecodeString(handoff.PublicKey)
	capabilityExpires, capabilityErr := capabilityExpiry(handoff.Capability)
	var postRevocationExpires time.Time
	var postRevocationErr error
	if handoff.PostRevocationCapability != "" {
		postRevocationExpires, postRevocationErr = capabilityExpiry(handoff.PostRevocationCapability)
	}
	pendingConsumptionInvalid := handoff.PendingConsumption != nil &&
		(handoff.PendingConsumption.ComputerID != computerID || strings.TrimSpace(handoff.PendingConsumption.Nonce) == "" ||
			!computerevent.IsSHA256(handoff.PendingConsumption.RequestCommitment))
	if err != nil || keyErr != nil || capabilityErr != nil || postRevocationErr != nil || len(publicKey) != ed25519.PublicKeySize ||
		!expiresAt.Equal(capabilityExpires) || (!postRevocationExpires.IsZero() && !expiresAt.Equal(postRevocationExpires)) ||
		!time.Now().UTC().Before(expiresAt) || handoff.KeyID == "" || pendingConsumptionInvalid {
		return nil, fmt.Errorf("guest credential: restart handoff expired or invalid")
	}
	credentials := &GuestCredentials{
		baseURL: handoff.BaseURL, computerID: handoff.ComputerID, realizationID: handoff.RealizationID,
		http: &http.Client{Timeout: 15 * time.Second}, token: handoff.Capability, postRevocationToken: handoff.PostRevocationCapability, expiresAt: expiresAt,
		keyID: handoff.KeyID, publicKey: ed25519.PublicKey(publicKey),
		pendingLifecycle: append([]computerevent.Receipt(nil), handoff.PendingLifecycleReceipts...),
		pendingConsumption: func() *platform.CredentialConsumptionRequest {
			if handoff.PendingConsumption == nil {
				return nil
			}
			copy := *handoff.PendingConsumption
			return &copy
		}(),
	}
	return credentials, nil
}
