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
)

type restartCredentialHandoff struct {
	Version    int    `json:"version"`
	BaseURL    string `json:"base_url"`
	ComputerID string `json:"computer_id"`
	Capability string `json:"capability"`
	ExpiresAt  string `json:"expires_at"`
	KeyID      string `json:"key_id"`
	PublicKey  string `json:"public_key"`
	PrivacyKey string `json:"privacy_key"`
}

func (g *GuestCredentials) WriteRestartHandoff(ctx context.Context, path string) error {
	if g == nil || !filepath.IsAbs(filepath.Clean(path)) {
		return fmt.Errorf("guest credential: invalid restart handoff path")
	}
	if _, err := g.Capability(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	handoff := restartCredentialHandoff{
		Version: 1, BaseURL: g.baseURL, ComputerID: g.computerID, Capability: g.token,
		ExpiresAt: g.expiresAt.UTC().Format(time.RFC3339Nano), KeyID: g.keyID,
		PublicKey: base64.RawStdEncoding.EncodeToString(g.publicKey), PrivacyKey: g.privacyKey,
	}
	g.mu.Unlock()
	canonical, err := computerevent.CanonicalJSON(handoff)
	if err != nil {
		return err
	}
	path = filepath.Clean(path)
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

func RestoreGuestCredentials(path, baseURL, computerID string) (*GuestCredentials, error) {
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
	if err != nil || !bytes.Equal(canonical, raw) || handoff.Version != 1 || handoff.BaseURL != strings.TrimRight(baseURL, "/") || handoff.ComputerID != computerID {
		return nil, fmt.Errorf("guest credential: restart handoff binding mismatch")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, handoff.ExpiresAt)
	publicKey, keyErr := base64.RawStdEncoding.DecodeString(handoff.PublicKey)
	capabilityExpires, capabilityErr := capabilityExpiry(handoff.Capability)
	privacyKey, privacyErr := base64.RawStdEncoding.DecodeString(handoff.PrivacyKey)
	if err != nil || keyErr != nil || privacyErr != nil || capabilityErr != nil || len(publicKey) != ed25519.PublicKeySize || len(privacyKey) != 32 || !expiresAt.Equal(capabilityExpires) || !time.Now().UTC().Before(expiresAt) || handoff.KeyID == "" {
		return nil, fmt.Errorf("guest credential: restart handoff expired or invalid")
	}
	return &GuestCredentials{
		baseURL: handoff.BaseURL, computerID: handoff.ComputerID, http: &http.Client{Timeout: 15 * time.Second},
		token: handoff.Capability, expiresAt: expiresAt, keyID: handoff.KeyID, publicKey: ed25519.PublicKey(publicKey), privacyKey: handoff.PrivacyKey,
	}, nil
}
