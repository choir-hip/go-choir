package main

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The adapter must carry the post-maintenance re-mint to vmmanager; dropping
// it leaves a long hook booting a stale envelope with every vmctl test green
// (docs/problems/offline-maintenance-outlives-the-boot-credential-2026-10-24.md).
func TestManagerConfigCarriesPostMaintenanceCredential(t *testing.T) {
	called := false
	mcfg := toManagerVMConfig(vmctl.VMManagerConfig{
		OfflineMaintenance:        func(string) error { return nil },
		PostMaintenanceCredential: func() (string, error) { called = true; return "fresh", nil },
	})
	if mcfg.PostMaintenanceCredential == nil {
		t.Fatal("the adapter dropped the post-maintenance re-mint")
	}
	if envelope, err := mcfg.PostMaintenanceCredential(); err != nil || envelope != "fresh" || !called {
		t.Fatalf("re-mint through the adapter = %q, %v", envelope, err)
	}
}
