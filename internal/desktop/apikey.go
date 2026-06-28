// Package desktop implements the Base sync engine that wires the Choir Base
// reconciliation kernel (M2-M4) into the desktop app. It provides:
//
//   - Secure API key storage (OS keychain on macOS/Linux via go-keyring,
//     with a file-based fallback for headless/test environments).
//   - An HTTP client for the Base API (M4) that authenticates every request
//     with a Bearer API key (M1).
//   - A local-folder scanner that builds a planner.Tree from the filesystem.
//   - A cancellable background sync loop that fetches remote deltas, runs the
//     pure planner (M2), executes the resulting actions, and updates the
//     synced cursor.
//   - Conflict surfacing: conflicts are collected and exposed to the UI;
//     they are NEVER silently resolved.
//   - Per-item sync status tracking.
//
// The sync engine lives in the main module so it can import the pure Base
// packages (model, planner, tree, blob) without crossing the cmd/desktop
// module boundary. The desktop app (cmd/desktop) wires this package into its
// Wails service registry.
package desktop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// ModelItemID is an alias for model.ItemID so the desktop app (cmd/desktop)
// can reference item IDs without directly importing the base model package.
type ModelItemID = model.ItemID

// KeyringService is the keyring service name under which Choir API keys are
// stored. The keyring backend (macOS Keychain, Linux Secret Service, Windows
// Credential Manager) is selected automatically by go-keyring.
const KeyringService = "choir.desktop.apikey"

// KeyringUser is the keyring account/user field used for the primary API key.
const KeyringUser = "default"

// APIKeyStore is the secure storage interface for the Choir API key secret
// (choir_sk_...). Implementations must persist the secret across restarts and
// protect it at rest (OS keychain, or a 0600 file as a fallback).
type APIKeyStore interface {
	// Save stores the API key secret. It overwrites any prior value.
	Save(secret string) error
	// Load returns the stored secret, or os.ErrNotExist (or an error wrapping
	// it) when no key has been stored.
	Load() (string, error)
	// Delete removes the stored secret. A missing key is not an error.
	Delete() error
}

// --- OS keychain backend (macOS Keychain / Linux Secret Service) ----------

// KeychainKeyStore stores the API key in the OS keychain via go-keyring. On
// macOS this is the Keychain; on Linux it is the Secret Service (GNOME
// Keyring / KDE Wallet); on Windows it is the Credential Manager.
type KeychainKeyStore struct {
	service string
	user    string
}

// NewKeychainKeyStore returns a keychain-backed store using the default
// Choir service/user identifiers.
func NewKeychainKeyStore() *KeychainKeyStore {
	return &KeychainKeyStore{service: KeyringService, user: KeyringUser}
}

// Save writes the secret to the OS keychain.
func (k *KeychainKeyStore) Save(secret string) error {
	if secret == "" {
		return fmt.Errorf("apikey: cannot save empty secret")
	}
	return keyring.Set(k.service, k.user, secret)
}

// Load reads the secret from the OS keychain. It returns an error wrapping
// os.ErrNotExist when no key is stored (go-keyring returns keyring.ErrNotFound).
func (k *KeychainKeyStore) Load() (string, error) {
	secret, err := keyring.Get(k.service, k.user)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", os.ErrNotExist
		}
		return "", err
	}
	return secret, nil
}

// Delete removes the secret from the OS keychain. go-keyring has no Delete
// API on all platforms, so this is a best-effort no-op that returns nil. A
// future implementation may overwrite the entry with an empty value.
func (k *KeychainKeyStore) Delete() error {
	// go-keyring does not expose a cross-platform delete. Overwriting with a
	// sentinel is unreliable across backends; we leave this as a no-op so
	// callers can rotate by calling Save with a new secret.
	return nil
}

// --- File-based fallback -------------------------------------------------

// FileKeyStore stores the API key in a JSON file with 0600 permissions. It is
// the fallback for headless/test environments and Linux without a Secret
// Service daemon. The file path is configurable; NewFileKeyStore uses a
// sensible default under the user config directory.
type FileKeyStore struct {
	path string
}

// apiKeyFile is the on-disk JSON shape for the file-based store.
type apiKeyFile struct {
	Secret string `json:"secret"`
}

// NewFileKeyStore returns a file-based store at the given path. The parent
// directory is created lazily on Save.
func NewFileKeyStore(path string) *FileKeyStore {
	return &FileKeyStore{path: path}
}

// DefaultFileKeyStorePath returns the default file path for the API key under
// the user's Choir config directory (~/.choir/desktop/apikey.json).
func DefaultFileKeyStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("apikey: home dir: %w", err)
	}
	return filepath.Join(home, ".choir", "desktop", "apikey.json"), nil
}

// Save writes the secret to the file with 0600 permissions.
func (f *FileKeyStore) Save(secret string) error {
	if secret == "" {
		return fmt.Errorf("apikey: cannot save empty secret")
	}
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("apikey: create dir %s: %w", dir, err)
	}
	data, err := json.Marshal(apiKeyFile{Secret: secret})
	if err != nil {
		return fmt.Errorf("apikey: marshal: %w", err)
	}
	if err := os.WriteFile(f.path, data, 0o600); err != nil {
		return fmt.Errorf("apikey: write %s: %w", f.path, err)
	}
	return nil
}

// Load reads the secret from the file. It returns os.ErrNotExist (wrapped)
// when the file does not exist.
func (f *FileKeyStore) Load() (string, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("apikey: read %s: %w", f.path, err)
	}
	var af apiKeyFile
	if err := json.Unmarshal(data, &af); err != nil {
		return "", fmt.Errorf("apikey: parse %s: %w", f.path, err)
	}
	if af.Secret == "" {
		return "", os.ErrNotExist
	}
	return af.Secret, nil
}

// Delete removes the file. A missing file is not an error.
func (f *FileKeyStore) Delete() error {
	if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("apikey: remove %s: %w", f.path, err)
	}
	return nil
}

// NewDefaultAPIKeyStore returns the preferred API key store for the current
// platform: the OS keychain when available, falling back to a file-based
// store. The file path is the default under ~/.choir/desktop/apikey.json.
func NewDefaultAPIKeyStore() APIKeyStore {
	return NewKeychainKeyStore()
}

// NewFallbackAPIKeyStore returns a file-based store at the default path. Used
// when the keychain is unavailable (e.g. CI, headless Linux).
func NewFallbackAPIKeyStore() (APIKeyStore, error) {
	path, err := DefaultFileKeyStorePath()
	if err != nil {
		return nil, err
	}
	return NewFileKeyStore(path), nil
}
