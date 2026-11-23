package vmctl

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The owner reads the host's capacity without a terminal, on the Pulse
// summary that already exists (docs/system-map.md; /api/pulse/summary), not
// on a second route. Failure modes, written before the code:
//  1. the reading missing what the owner decides on: memory total and
//     available, disk free, computers running and hibernated, accounts,
//     sign-ups in the last day, the cap;
//  2. test accounts (example.com, example.test) counted against the cap or
//     as sign-ups, so CI drains the strangers' room;
//  3. an account that began a sign-up and holds no passkey counted;
//  4. the cap shown as open at or over it;
//  5. any address or id in the output.

func TestPulseCapacityReadsWhatTheOwnerDecidesOn(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	users := []pulseAccountRecord{
		{UserID: "u-old", Email: "old.person@mosiah.org", CreatedAt: now.Add(-40 * 24 * time.Hour), HasPasskey: true},
		{UserID: "u-new", Email: "new.person@mosiah.org", CreatedAt: now.Add(-3 * time.Hour), HasPasskey: true},
		{UserID: "u-abandoned", Email: "gave.up@mosiah.org", CreatedAt: now.Add(-1 * time.Hour), HasPasskey: false},
		{UserID: "u-test-1", Email: "ci1@example.com", CreatedAt: now.Add(-1 * time.Hour), HasPasskey: true},
		{UserID: "u-test-2", Email: "CI2@Example.Test", CreatedAt: now.Add(-50 * time.Hour), HasPasskey: true},
	}
	ownerships := []*VMOwnership{
		{VMID: "vm-1", UserID: "u-old", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, MemSizeMib: 2048},
		{VMID: "vm-2", UserID: "u-new", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated, MemSizeMib: 2048},
		{VMID: "vm-3", UserID: "u-test-1", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive},
	}
	t.Setenv(interactiveVMMemSizeMibEnv, "8192")
	sample := HostPressureSample{MemoryTotalBytes: 32 << 30, MemoryAvailableBytes: 20 << 30, MemoryFloorBytes: 4800 << 20, StateDirAvailableBytes: 150 << 30}
	cfg := PressureReclaimConfig{MinStateDirAvailableBytes: 32 << 30}
	reg := RegistrationCapConfig{Cap: 3, TestCap: 100, TestDomains: []string{"example.com", "example.test"}}

	got := pulseCapacity(now, users, ownerships, sample, cfg, reg)

	if got.Memory.TotalBytes != 32<<30 || got.Memory.AvailableBytes != 20<<30 || got.Memory.FloorBytes != 4800<<20 {
		t.Fatalf("memory = %+v", got.Memory)
	}
	if got.Disk.AvailableBytes != 150<<30 || got.Disk.FloorBytes != 32<<30 {
		t.Fatalf("disk = %+v", got.Disk)
	}
	if got.Computers.Running != 2 || got.Computers.Hibernated != 1 {
		t.Fatalf("computers = %+v, want 2 running 1 hibernated", got.Computers)
	}
	// Guest memory promised to running computers: one 2 GiB, one at the host's 8 GiB.
	if got.Computers.RunningGuestMemoryMiB != 2048+8192 {
		t.Fatalf("running guest memory = %d MiB", got.Computers.RunningGuestMemoryMiB)
	}
	r := got.Registration
	if r.Cap != 3 || r.Counted != 2 || r.TestCounted != 2 || r.TestCap != 100 {
		t.Fatalf("registration = %+v, want cap 3, 2 counted, 2 test", r)
	}
	if r.SignUpsLast24h != 1 {
		t.Fatalf("sign-ups in the last day = %d, want 1 (a passkey holder, not a test address, not an abandoned begin)", r.SignUpsLast24h)
	}
	if !r.Open || r.Remaining != 1 {
		t.Fatalf("open = %v remaining = %d, want open with 1 left", r.Open, r.Remaining)
	}

	// At the cap it reads closed.
	reg.Cap = 2
	if closed := pulseCapacity(now, users, ownerships, sample, cfg, reg).Registration; closed.Open || closed.Remaining != 0 {
		t.Fatalf("at the cap: %+v, want closed", closed)
	}
	// No cap set reads open with no number to run out of.
	reg.Cap = 0
	if none := pulseCapacity(now, users, ownerships, sample, cfg, reg).Registration; !none.Open {
		t.Fatalf("no cap: %+v, want open", none)
	}

	raw, _ := json.Marshal(got)
	for _, leak := range []string{"@", "mosiah", "u-old", "u-new", "vm-1"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("capacity output carries %q: %s", leak, raw)
		}
	}
}
