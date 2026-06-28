package main

import (
	"context"
	"log"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/yusefmosiah/go-choir/internal/desktop"
)

// SyncService is the Wails-exposed service that owns the Base sync engine.
// It bridges the internal/desktop sync engine (main module) into the desktop
// app's frontend: the frontend calls these methods to start/stop sync, read
// status, and resolve conflicts.
//
// The API key is loaded from the OS keychain (macOS Keychain / Linux Secret
// Service) via desktop.APIKeyStore. On first run the user must provision a
// key via the WebAuthn session (POST /auth/api-keys) and store it with
// StoreAPIKey.
type SyncService struct {
	engine   *desktop.SyncEngine
	keyStore desktop.APIKeyStore
	cfg      desktop.SyncConfig
}

// NewSyncService constructs a SyncService. The localRoot is the folder to
// sync; baseURL is the Choir backend; deviceID identifies this device.
func NewSyncService(localRoot, baseURL, deviceID string) *SyncService {
	cfg := desktop.SyncConfig{
		BaseURL:   baseURL,
		LocalRoot: localRoot,
		DeviceID:  deviceID,
		Interval:  30 * time.Second,
	}
	return &SyncService{
		cfg:      cfg,
		keyStore: desktop.NewKeychainKeyStore(),
	}
}

// StoreAPIKey saves the Choir API key secret (choir_sk_...) to the OS
// keychain. Called once after the user creates a key via the WebAuthn
// session. Returns an error if the keychain is unavailable.
func (s *SyncService) StoreAPIKey(secret string) error {
	return s.keyStore.Save(secret)
}

// HasAPIKey reports whether an API key has been stored.
func (s *SyncService) HasAPIKey() bool {
	_, err := s.keyStore.Load()
	return err == nil
}

// ClearAPIKey removes the stored API key.
func (s *SyncService) ClearAPIKey() error {
	return s.keyStore.Delete()
}

// StartSync launches the background sync loop using the stored API key. It
// returns an error if no key is stored or the loop is already running.
func (s *SyncService) StartSync() error {
	secret, err := s.keyStore.Load()
	if err != nil {
		return err
	}
	s.engine = desktop.NewSyncEngine(s.cfg, secret)
	// Persist the synced state under the local root's Choir metadata dir.
	statePath := filepath.Join(s.cfg.LocalRoot, ".choir", "synced-state.json")
	s.engine.SetSyncedStateStore(desktop.NewFileSyncedStateStore(statePath))
	return s.engine.Start(context.Background())
}

// StopSync cancels the background sync loop.
func (s *SyncService) StopSync() {
	if s.engine != nil {
		s.engine.Stop()
	}
}

// SyncNow triggers an immediate sync cycle.
func (s *SyncService) SyncNow() {
	if s.engine != nil {
		s.engine.SyncNow()
	}
}

// GetSyncStatus returns the current sync progress for the frontend.
func (s *SyncService) GetSyncStatus() desktop.SyncProgress {
	if s.engine == nil {
		return desktop.SyncProgress{Phase: desktop.PhaseIdle}
	}
	return s.engine.Status().Snapshot()
}

// GetConflicts returns the current conflict set (resolved and pending).
func (s *SyncService) GetConflicts() []desktop.ConflictRecord {
	if s.engine == nil {
		return nil
	}
	return s.engine.Conflicts().All()
}

// ResolveConflict records the user's resolution for a conflict. The next
// sync cycle applies it. resolution must be one of: keep_local, keep_remote,
// keep_both.
func (s *SyncService) ResolveConflict(itemID, resolution string) error {
	if s.engine == nil {
		return errSentinelMsg("sync engine not started")
	}
	return s.engine.Conflicts().Resolve(desktop.ModelItemID(itemID), desktop.ConflictResolution(resolution))
}

// ServiceStartup is the Wails lifecycle hook. We do not auto-start sync here;
// the frontend calls StartSync after confirming the API key is present.
func (s *SyncService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	log.Printf("[sync] SyncService starting (localRoot=%s backend=%s)", s.cfg.LocalRoot, s.cfg.BaseURL)
	return nil
}

// ServiceShutdown stops the sync loop when the app quits.
func (s *SyncService) ServiceShutdown() error {
	s.StopSync()
	return nil
}

// errSentinelMsg is a local error helper (the desktop package's errSentinel is
// unexported). We avoid importing fmt here to keep the service file lean.
type sentinelMsg string

func (e sentinelMsg) Error() string { return string(e) }

func errSentinelMsg(s string) error { return sentinelMsg(s) }
