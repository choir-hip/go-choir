package vmctl

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
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

	// No reading: no numbers and not open, never a guess from a partial list.
	for name, bad := range map[string]PulseCapacityRegistration{
		"auth data unavailable (partial list)": pulseCapacity(now, users, false, ownerships, sample, cfg, RegistrationCapConfig{Cap: 4}).Registration,
		"malformed cap":                        pulseCapacity(now, users, true, ownerships, sample, cfg, RegistrationCapConfig{Unreadable: true}).Registration,
		"malformed cap, no cap value":          pulseCapacity(now, users, true, ownerships, sample, cfg, RegistrationCapConfig{Cap: 0, Unreadable: true}).Registration,
	} {
		if bad != (PulseCapacityRegistration{}) {
			t.Fatalf("%s: %+v, want no numbers and not open", name, bad)
		}
	}
	// The disk floor shown is the one the cap uses (deploy floor plus the
	// paper's reserve), not the pressure floor.
	reserved := pulseCapacity(now, users, true, ownerships, sample, cfg, RegistrationCapConfig{Cap: 4, DiskReserveBytes: 100 << 30})
	if reserved.Disk.FloorBytes != 100<<30 {
		t.Fatalf("disk floor = %d, want the 100 GiB the cap keeps back", reserved.Disk.FloorBytes)
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

// vmctl reads the auth database the auth service writes (real file, read-only):
// the operator flag and the passkey time come through; a database an older
// build wrote (no operator column) is "no reading", never every account
// counted as a person.
func TestPulseAccountsFromARealAuthDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	finished := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	for _, q := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, created_at DATETIME NOT NULL, operator INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE credentials (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, created_at DATETIME NOT NULL)`,
		`INSERT INTO users VALUES ('p', 'p@mosiah.org', '2026-11-23 01:00:00+00:00', 0), ('o', 'o@example.com', '2026-11-23 01:00:00+00:00', 1), ('x', 'x@mosiah.org', '2026-11-23 01:00:00+00:00', 0)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []string{"p", "o"} {
		if _, err := db.Exec(`INSERT INTO credentials VALUES (?, ?, ?)`, "c-"+u, u, finished); err != nil {
			t.Fatal(err)
		}
	}
	users, warnings := loadPulseAccountsFromAuthDB(path)
	if len(warnings) != 0 || len(users) != 3 {
		t.Fatalf("users = %+v warnings = %v", users, warnings)
	}
	for _, u := range users {
		switch u.UserID {
		case "p":
			if !u.HasPasskey || u.Operator || !u.FirstPasskeyAt.Equal(finished) {
				t.Fatalf("p = %+v", u)
			}
		case "o":
			if !u.Operator || !u.HasPasskey {
				t.Fatalf("o = %+v", u)
			}
		case "x":
			if u.HasPasskey {
				t.Fatalf("x = %+v", u)
			}
		}
	}

	// The same database before the operator column exists.
	old := filepath.Join(t.TempDir(), "old.db")
	odb, err := sql.Open("sqlite", "file:"+old)
	if err != nil {
		t.Fatal(err)
	}
	defer odb.Close()
	for _, q := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, created_at DATETIME NOT NULL)`,
		`CREATE TABLE credentials (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, created_at DATETIME NOT NULL)`,
		`INSERT INTO users VALUES ('p', 'p@mosiah.org', '2026-11-23 01:00:00+00:00')`,
	} {
		if _, err := odb.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	oldUsers, oldWarnings := loadPulseAccountsFromAuthDB(old)
	if len(oldWarnings) == 0 {
		t.Fatalf("a database without the operator column read as available: %+v", oldUsers)
	}
	got := pulseCapacity(time.Now(), oldUsers, len(oldWarnings) == 0, nil, HostPressureSample{}, PressureReclaimConfig{}, RegistrationCapConfig{Cap: 22}).Registration
	if got != (PulseCapacityRegistration{}) {
		t.Fatalf("registration = %+v, want no reading", got)
	}
}
