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
//  2. the operator's accounts counted against the cap or as sign-ups, so CI
//     drains the strangers' room, and an address on example.com counted as
//     exempt (a stranger can type one);
//  3. an account that began a sign-up and holds no passkey counted;
//  4. the cap shown as open at or over it;
//  5. any address or id in the output.

func TestPulseCapacityReadsWhatTheOwnerDecidesOn(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	users := []pulseAccountRecord{
		{UserID: "u-old", Email: "old.person@mosiah.org", CreatedAt: now.Add(-40 * 24 * time.Hour), HasPasskey: true},
		{UserID: "u-new", Email: "new.person@mosiah.org", CreatedAt: now.Add(-3 * time.Hour), HasPasskey: true},
		{UserID: "u-abandoned", Email: "gave.up@mosiah.org", CreatedAt: now.Add(-1 * time.Hour), HasPasskey: false},
		{UserID: "u-op-1", Email: "ci1@mosiah.org", CreatedAt: now.Add(-1 * time.Hour), HasPasskey: true, Operator: true},
		{UserID: "u-op-2", Email: "CI2@mosiah.org", CreatedAt: now.Add(-50 * time.Hour), HasPasskey: true, Operator: true},
		// Began 30 hours ago, finished its passkey 2 hours ago: a sign-up of today.
		{UserID: "u-late", Email: "late@example.com", CreatedAt: now.Add(-30 * time.Hour), FirstPasskeyAt: now.Add(-2 * time.Hour), HasPasskey: true},
	}
	ownerships := []*VMOwnership{
		{VMID: "vm-1", UserID: "u-old", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, MemSizeMib: 2048},
		{VMID: "vm-2", UserID: "u-new", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated, MemSizeMib: 2048},
		{VMID: "vm-3", UserID: "u-op-1", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive},
	}
	t.Setenv(interactiveVMMemSizeMibEnv, "8192")
	sample := HostPressureSample{MemoryTotalBytes: 32 << 30, MemoryAvailableBytes: 20 << 30, MemoryFloorBytes: 4800 << 20, StateDirAvailableBytes: 150 << 30}
	cfg := PressureReclaimConfig{MinStateDirAvailableBytes: 32 << 30}
	reg := RegistrationCapConfig{Cap: 4}

	got := pulseCapacity(now, users, true, ownerships, sample, cfg, reg)

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
	if !r.Available || r.Cap != 4 || r.Counted != 3 {
		t.Fatalf("registration = %+v, want available, cap 4, 3 counted (example.com is a person; operator accounts are not)", r)
	}
	if r.SignUpsLast24h != 2 {
		t.Fatalf("sign-ups in the last day = %d, want 2 (u-new, and u-late by its passkey time; no operator, no abandoned begin)", r.SignUpsLast24h)
	}
	if !r.Open || r.Remaining != 1 {
		t.Fatalf("open = %v remaining = %d, want open with 1 left", r.Open, r.Remaining)
	}

	// At the cap it reads closed.
	reg.Cap = 3
	if closed := pulseCapacity(now, users, true, ownerships, sample, cfg, reg).Registration; closed.Open || closed.Remaining != 0 {
		t.Fatalf("at the cap: %+v, want closed", closed)
	}
	// No cap set reads open with no number to run out of.
	reg.Cap = 0
	if none := pulseCapacity(now, users, true, ownerships, sample, cfg, reg).Registration; !none.Open {
		t.Fatalf("no cap: %+v, want open", none)
	}

	// An unreadable auth database or cap is "no reading", not open.
	if bad := pulseCapacity(now, nil, false, ownerships, sample, cfg, RegistrationCapConfig{Cap: 4}).Registration; bad.Available {
		t.Fatalf("auth data unavailable: %+v, want no reading", bad)
	}
	if bad := pulseCapacity(now, users, true, ownerships, sample, cfg, RegistrationCapConfig{Unreadable: true}).Registration; bad.Available {
		t.Fatalf("malformed cap: %+v, want no reading", bad)
	}

	raw, _ := json.Marshal(got)
	for _, leak := range []string{"test_cap", "test_counted", "example"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("capacity output names the test reserve %q: %s", leak, raw)
		}
	}
	for _, leak := range []string{"@", "mosiah", "u-old", "u-new", "vm-1"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("capacity output carries %q: %s", leak, raw)
		}
	}
}
