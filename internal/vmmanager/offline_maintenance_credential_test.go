package vmmanager

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

// The boot credential an offline maintenance hook outlives
// (docs/problems/offline-maintenance-outlives-the-boot-credential-2026-10-24.md).
// Failure modes, vmmanager side: the refresh boots the envelope minted before
// the hook; an unbootable hook mints anyway; a hook that failed but still
// boots keeps the stale envelope; a failed re-mint stops a boot that could
// have used the pre-hook envelope; a refresh without a hook calls the re-mint.

type maintenanceSteps struct {
	steps    []string
	envelope string
	err      error
}

func (s *maintenanceSteps) hook(err error) func(string) error {
	return func(string) error {
		s.steps = append(s.steps, "hook")
		return err
	}
}

func (s *maintenanceSteps) remint() (string, error) {
	s.steps = append(s.steps, "remint")
	return s.envelope, s.err
}

func runMaintenanceStep(t *testing.T, hookErr error, s *maintenanceSteps, withHook bool) (VMConfig, error) {
	t.Helper()
	m := NewManager(ManagerConfig{StateDir: t.TempDir(), HostBasePort: 9000})
	cfg := VMConfig{VMID: "vm-paper", RealizationID: "vm-paper-epoch-9", Epoch: 9, ComputerCredentialEnvelope: "pre-hook-envelope"}
	overrides := VMConfig{PostMaintenanceCredential: s.remint}
	if withHook {
		overrides.OfflineMaintenance = s.hook(hookErr)
	}
	err := m.runOfflineMaintenance("vm-paper", &cfg, overrides)
	return cfg, err
}

func TestOfflineMaintenanceReMintsCredentialAfterHook(t *testing.T) {
	s := &maintenanceSteps{envelope: "post-hook-envelope"}
	cfg, err := runMaintenanceStep(t, nil, s, true)
	if err != nil {
		t.Fatalf("maintenance step: %v", err)
	}
	if fmt.Sprint(s.steps) != "[hook remint]" {
		t.Fatalf("steps = %v, want the re-mint after the hook", s.steps)
	}
	if cfg.ComputerCredentialEnvelope != "post-hook-envelope" {
		t.Fatalf("boot envelope = %q, want the post-hook one", cfg.ComputerCredentialEnvelope)
	}
	if cfg.RealizationID != "vm-paper-epoch-9" || cfg.Epoch != 9 {
		t.Fatalf("the re-mint moved the realization: %s/%d", cfg.RealizationID, cfg.Epoch)
	}
}

func TestUnbootableOfflineMaintenanceDoesNotMint(t *testing.T) {
	s := &maintenanceSteps{envelope: "post-hook-envelope"}
	_, err := runMaintenanceStep(t, fmt.Errorf("restore backup: %w", ErrOfflineMaintenanceUnbootable), s, true)
	if !errors.Is(err, ErrOfflineMaintenanceUnbootable) {
		t.Fatalf("maintenance step = %v, want the unbootable error", err)
	}
	if fmt.Sprint(s.steps) != "[hook]" {
		t.Fatalf("steps = %v, an unbootable hook must not mint", s.steps)
	}
}

func TestFailedOfflineMaintenanceThatBootsReMints(t *testing.T) {
	s := &maintenanceSteps{envelope: "post-hook-envelope"}
	cfg, err := runMaintenanceStep(t, errors.New("projection-compact: refused"), s, true)
	if err != nil {
		t.Fatalf("an ordinary hook failure must still boot: %v", err)
	}
	if fmt.Sprint(s.steps) != "[hook remint]" || cfg.ComputerCredentialEnvelope != "post-hook-envelope" {
		t.Fatalf("steps=%v envelope=%q: the boot after a failed hook needs a fresh credential too", s.steps, cfg.ComputerCredentialEnvelope)
	}
}

func TestFailedReMintKeepsPreHookEnvelope(t *testing.T) {
	s := &maintenanceSteps{err: errors.New("corpusd unavailable")}
	cfg, err := runMaintenanceStep(t, nil, s, true)
	if err != nil {
		t.Fatalf("a failed re-mint must not stop the boot: %v", err)
	}
	if cfg.ComputerCredentialEnvelope != "pre-hook-envelope" {
		t.Fatalf("boot envelope = %q, want the pre-hook one kept", cfg.ComputerCredentialEnvelope)
	}
}

func TestRefreshWithoutMaintenanceDoesNotReMint(t *testing.T) {
	s := &maintenanceSteps{envelope: "post-hook-envelope"}
	cfg, err := runMaintenanceStep(t, nil, s, false)
	if err != nil || len(s.steps) != 0 || cfg.ComputerCredentialEnvelope != "pre-hook-envelope" {
		t.Fatalf("no hook: err=%v steps=%v envelope=%q, want nothing run", err, s.steps, cfg.ComputerCredentialEnvelope)
	}
}

func TestRefreshConfigNeverStoresTheReMint(t *testing.T) {
	stored := VMConfig{VMID: "vm-paper", PersistentDir: filepath.Join(t.TempDir(), "persist")}
	overrides := VMConfig{PostMaintenanceCredential: func() (string, error) { return "x", nil }, OfflineMaintenance: func(string) error { return nil }}
	got := refreshBootConfig(stored, overrides, DefaultManagerConfig())
	if got.PostMaintenanceCredential != nil || got.OfflineMaintenance != nil {
		t.Fatal("a refresh's one-shot hooks leaked into the boot config the instance keeps")
	}
}
