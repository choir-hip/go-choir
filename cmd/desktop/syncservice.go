package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/yusefmosiah/go-choir/internal/desktop"
	"github.com/yusefmosiah/go-choir/internal/desktop/fileprovider"
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
	bridge   *fileprovider.Bridge
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
// On macOS it also starts the File Provider IPC bridge so the
// NSFileProviderReplicatedExtension can enumerate, read, and write
// Base-synced files in Finder.
func (s *SyncService) StartSync() error {
	secret, err := s.keyStore.Load()
	if err != nil {
		return err
	}
	s.engine = desktop.NewSyncEngine(s.cfg, secret)
	// Persist the synced state under the local root's Choir metadata dir.
	statePath := filepath.Join(s.cfg.LocalRoot, ".choir", "synced-state.json")
	s.engine.SetSyncedStateStore(desktop.NewFileSyncedStateStore(statePath))
	if err := s.engine.Start(context.Background()); err != nil {
		return err
	}

	// Start the File Provider IPC bridge (macOS only). The bridge listens
	// on a Unix domain socket in the app support directory; the
	// .appex extension connects to it to serve Finder requests.
	if runtime.GOOS == "darwin" {
		socketPath := fileProviderSocketPath()
		b, berr := fileprovider.NewBridge(fileprovider.BridgeConfig{
			Engine:     s.engine,
			LocalRoot:  s.cfg.LocalRoot,
			SocketPath: socketPath,
			DeviceID:   s.cfg.DeviceID,
		})
		if berr != nil {
			log.Printf("[sync] fileprovider bridge: %v (File Provider disabled)", berr)
		} else if err := b.Start(context.Background()); err != nil {
			log.Printf("[sync] fileprovider bridge start: %v (File Provider disabled)", err)
		} else {
			s.bridge = b
			log.Printf("[sync] fileprovider bridge listening on %s", socketPath)
		}
	}
	return nil
}

// StopSync cancels the background sync loop and stops the File Provider
// bridge if it is running.
func (s *SyncService) StopSync() {
	if s.bridge != nil {
		s.bridge.Stop()
		s.bridge = nil
	}
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

// fileProviderSocketPath returns the Unix domain socket path for the File
// Provider IPC bridge. The socket lives in the Choir app support directory
// so the .appex extension can access it via the app group container.
func fileProviderSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	// On macOS, the app group container is preferred, but for development
	// we use ~/Library/Application Support/Choir/ which is accessible to
	// both the host app and the extension when running with dev
	// entitlements.
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "Choir", "fileprovider.sock")
	}
	return filepath.Join(home, ".choir", "fileprovider.sock")
}
