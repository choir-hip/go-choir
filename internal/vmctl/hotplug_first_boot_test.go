package vmctl

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// RES-09, P5 of docs/definitions/choir-launch-fp8-image-jailer-2026-10-21.md:
// a test account minted since the last idle sweep is class test at boot
// (bootWarmnessClass refreshes the tier), but the hotplug selection was read
// before that refresh, so its first cold boot had no virtio-mem device and
// the probe saw "not selected". Every class:test selection read at boot must
// see the refreshed tier.
func TestFreshTestAccountFirstBootIsSelectedForHotplug(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, created_at DATETIME NOT NULL);
		INSERT INTO users VALUES ('real-1','someone@mosiah.org','2026-10-13');`); err != nil {
		t.Fatal(err)
	}
	newRegistry := func() (*OwnershipRegistry, *mockVMManager) {
		reg := NewOwnershipRegistry("http://127.0.0.1:8085")
		reg.SetRetentionPruneConfig(RetentionPruneConfig{AuthDBPath: path, EphemeralEmailDomains: []string{"example.test"}})
		reg.SetWarmnessPolicyConfig(tierConfig())
		reg.SetMemoryHotplugConfig(MemoryHotplugConfig{Enabled: true, Only: []string{"class:test"}, RegionMiB: 4096})
		mgr := &mockVMManager{}
		reg.SetVMManager(mgr)
		reg.refreshTestAccounts() // the sweep, before the QA account exists
		return reg, mgr
	}

	// New computer path (ResolveOrAssignDesktopContext's mint and boot).
	reg, mgr := newRegistry()
	if _, err := db.Exec(`INSERT INTO users VALUES ('qa-new','tracer-9@example.test','2026-10-15')`); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.ResolveOrAssignDesktopContext(context.Background(), "qa-new", PrimaryDesktopID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(mgr.boots) != 1 {
		t.Fatalf("boots = %d, want 1", len(mgr.boots))
	}
	if got := mgr.boots[0]; got.WarmnessClass != string(WarmnessClassTest) || got.MemoryHotplugMiB != 4096 {
		t.Fatalf("first boot of a fresh test account: class=%q hotplug=%d, want test and 4096", got.WarmnessClass, got.MemoryHotplugMiB)
	}

	// Fresh realization path (reserveFreshVMConfig: restart, refresh, recover).
	reg2, mgr2 := newRegistry()
	if _, err := db.Exec(`INSERT INTO users VALUES ('qa-newer','tracer-10@example.test','2026-10-15')`); err != nil {
		t.Fatal(err)
	}
	own := &VMOwnership{VMID: "vm-qa-newer", ComputerID: "computer-qa-newer", UserID: "qa-newer", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive}
	cfg, err := reg2.reserveFreshVMConfig(own, "", mgr2)
	if err != nil {
		t.Fatalf("reserveFreshVMConfig: %v", err)
	}
	if cfg.WarmnessClass != string(WarmnessClassTest) || cfg.MemoryHotplugMiB != 4096 {
		t.Fatalf("fresh realization of a fresh test account: class=%q hotplug=%d, want test and 4096", cfg.WarmnessClass, cfg.MemoryHotplugMiB)
	}

	// A real account is never selected.
	reg3, mgr3 := newRegistry()
	if _, err := reg3.ResolveOrAssignDesktopContext(context.Background(), "real-1", PrimaryDesktopID); err != nil {
		t.Fatalf("resolve real: %v", err)
	}
	if got := mgr3.boots[0]; got.MemoryHotplugMiB != 0 || got.WarmnessClass == string(WarmnessClassTest) {
		t.Fatalf("real account boot: class=%q hotplug=%d", got.WarmnessClass, got.MemoryHotplugMiB)
	}
}
