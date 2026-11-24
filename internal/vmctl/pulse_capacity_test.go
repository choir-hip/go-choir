package vmctl

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostroom"
)

// The owner reads the host's capacity without a terminal, on the Pulse
// summary that already exists (docs/system-map.md; /api/pulse/summary), not
// on a second route. Sign-up is open when the host can hold one more computer
// (internal/hostroom); Pulse shows the same reading the auth service decides
// on. Failure modes, written before the code:
//  1. the reading missing what the owner decides on: memory, disk free and the
//     reserve, computers, accounts, the room held for computers, the places
//     left, sign-ups in the last day;
//  2. an account that began a sign-up and holds no passkey counted;
//  3. an account with no computer holding room (owner, 2026-11-24); a
//     computer that has not yet grown to a footprint releasing the whole of
//     its room (opening must not make room);
//  4. the place shown open with less than one footprint of room;
//  5. any number shown when a reading is missing (the owners file, the disk
//     sample), or the plan is not configured; the room depending on the auth
//     database, which gives only the account counts;
//  6. any address or id in the output;
//  7. the public summary carrying live host figures: it says only whether
//     sign-up is open, closed or unreadable (the figures are the owner's).

func TestPulseCapacityReadsWhatTheOwnerDecidesOn(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	users := []pulseAccountRecord{
		{UserID: "u-old", Email: "old.person@mosiah.org", CreatedAt: now.Add(-40 * 24 * time.Hour), HasPasskey: true, Proved: true},
		{UserID: "u-new", Email: "new.person@mosiah.org", CreatedAt: now.Add(-3 * time.Hour), HasPasskey: true, Proved: true},
		{UserID: "u-abandoned", Email: "gave.up@mosiah.org", CreatedAt: now.Add(-1 * time.Hour), HasPasskey: false},
		// A passkey holder from before the proof rule: it cannot sign in or
		// open a computer, so it counts for nothing.
		{UserID: "u-unproved", Email: "unproved@mosiah.org", CreatedAt: now.Add(-60 * 24 * time.Hour), HasPasskey: true},
		// Began 30 hours ago, finished its passkey 2 hours ago: a sign-up of today.
		{UserID: "u-late", Email: "late@example.com", CreatedAt: now.Add(-30 * time.Hour), FirstPasskeyAt: now.Add(-2 * time.Hour), HasPasskey: true, Proved: true},
	}
	// u-old's computer holds a whole footprint, u-new's has just opened.
	owners := map[string]uint64{"u-old": 9 << 30, "u-new": 644245094, "paper": 50 << 30}
	ownerships := []*VMOwnership{
		{VMID: "vm-1", UserID: "u-old", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, MemSizeMib: 4096},
		{VMID: "vm-2", UserID: "u-new", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated, MemSizeMib: 4096},
		{VMID: "vm-3", UserID: "paper", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive},
	}
	t.Setenv(interactiveVMMemSizeMibEnv, "8192")
	sample := HostPressureSample{MemoryTotalBytes: 32 << 30, MemoryAvailableBytes: 20 << 30, MemoryFloorBytes: 4800 << 20, StateDirAvailableBytes: 154 << 30}
	cfg := PressureReclaimConfig{MinStateDirAvailableBytes: 32 << 30}
	plan := hostroom.Plan{ReserveBytes: 100 << 30, FootprintBytes: 9 << 30}

	got := pulseCapacity(now, users, true, ownerships, owners, true, sample, cfg, plan)

	if !got.Memory.Available || got.Memory.TotalBytes != 32<<30 || got.Memory.AvailableBytes != 20<<30 || got.Memory.FloorBytes != 4800<<20 {
		t.Fatalf("memory = %+v", got.Memory)
	}
	// The disk floor stays the pressure floor (one meaning); the reserve the
	// room keeps back is its own field.
	if !got.Disk.Available || got.Disk.AvailableBytes != 154<<30 || got.Disk.FloorBytes != 32<<30 {
		t.Fatalf("disk = %+v, want 154 GiB free over the 32 GiB pressure floor", got.Disk)
	}
	if got.Computers.Running != 2 || got.Computers.Hibernated != 1 {
		t.Fatalf("computers = %+v, want 2 running 1 hibernated", got.Computers)
	}
	if got.Computers.RunningGuestMemoryMiB != 4096+8192 {
		t.Fatalf("running guest memory = %d MiB", got.Computers.RunningGuestMemoryMiB)
	}
	r := got.Room
	// u-old and u-new have computers; u-late holds a passkey and none yet,
	// and holds nothing (owner, 2026-11-24).
	if !r.Available || r.Accounts != 3 {
		t.Fatalf("room = %+v, want available, 3 accounts", r)
	}
	// 154 - 100 = 54 GiB; u-new's computer is 8.4 GiB short of a footprint,
	// u-old's holds none back: 54 - 8.4 = 45.6; 45.6 / 9 = 5.
	if r.Places != 5 || r.HeldBytes != 9<<30-644245094 || !r.Open || r.FootprintBytes != 9<<30 || r.ReserveBytes != 100<<30 {
		t.Fatalf("room = %+v, want 5 places, open, 9 GiB footprint, 100 GiB reserve", r)
	}
	// Hundreds more accounts with a passkey and no computer change nothing in
	// what the room holds or the places it offers.
	many := append([]pulseAccountRecord{}, users...)
	for i := 0; i < 300; i++ {
		many = append(many, pulseAccountRecord{UserID: fmt.Sprintf("u-rec-%d", i), Email: fmt.Sprintf("rec%d@mosiah.org", i), CreatedAt: now.Add(-90 * 24 * time.Hour), HasPasskey: true, Proved: true})
	}
	if m := pulseCapacity(now, many, true, ownerships, owners, true, sample, cfg, plan).Room; m.Places != r.Places || m.HeldBytes != r.HeldBytes || !m.Open {
		t.Fatalf("300 accounts with no computer moved the room: %+v, was %+v", m, r)
	}
	if r.SignUpsLast24h != 2 {
		t.Fatalf("sign-ups in the last day = %d, want 2 (u-new, and u-late by its passkey time; no abandoned begin)", r.SignUpsLast24h)
	}

	// Less than one footprint over the reserve reads closed, with the numbers.
	tight := sample
	tight.StateDirAvailableBytes = 109<<30 + 8<<30
	if c := pulseCapacity(now, users, true, ownerships, owners, true, tight, cfg, plan).Room; c.Open || c.Places != 0 || !c.Available {
		t.Fatalf("room under a footprint: %+v, want available and closed", c)
	}

	// No reading: no numbers and not open, never a guess.
	bad := sample
	bad.ObservationError = "statfs failed"
	for name, room := range map[string]PulseCapacityRoom{
		"the disk sample failed": pulseCapacity(now, users, true, ownerships, owners, true, bad, cfg, plan).Room,
		"no plan configured":     pulseCapacity(now, users, true, ownerships, owners, true, sample, cfg, hostroom.Plan{}).Room,
		// The owners file the auth service reads is missing, torn or stale:
		// auth closes sign-up, so Pulse says no reading, never open.
		"the owners file is unreadable": pulseCapacity(now, users, true, ownerships, nil, false, sample, cfg, plan).Room,
	} {
		if room != (PulseCapacityRoom{}) {
			t.Fatalf("%s: %+v, want no numbers and not open", name, room)
		}
	}
	// The auth database is read only for the account counts: without it the
	// room still reads, with no account figures.
	if noUsers := pulseCapacity(now, users, false, ownerships, owners, true, sample, cfg, plan).Room; !noUsers.Available || noUsers.Places != r.Places || noUsers.HeldBytes != r.HeldBytes || noUsers.Accounts != 0 || noUsers.SignUpsLast24h != 0 {
		t.Fatalf("room without the auth database = %+v, want the same places and held as %+v and no account figures", noUsers, r)
	}
	// A failed memory or disk sample shows no figure (not "0 B").
	if m := pulseCapacity(now, users, true, ownerships, owners, true, bad, cfg, plan); m.Disk.Available || m.Memory.Available || m.Disk.AvailableBytes != 0 {
		t.Fatalf("a failed sample shows numbers: %+v %+v", m.Memory, m.Disk)
	}

	// The public view keeps only whether sign-up is open.
	pub := got.Public()
	if !pub.Room.Available || !pub.Room.Open {
		t.Fatalf("public room = %+v, want available and open", pub.Room)
	}
	rawPub, _ := json.Marshal(pub)
	for _, figure := range []string{"bytes", "accounts", "promised", "places", "footprint", "reserve", "running", "hibernated", "mib", "sign_ups"} {
		if strings.Contains(string(rawPub), `"`+figure) || strings.Contains(string(rawPub), figure+`"`) {
			t.Fatalf("the public capacity carries %q: %s", figure, rawPub)
		}
	}
	if closedPub := pulseCapacity(now, users, true, ownerships, nil, false, sample, cfg, plan).Public(); closedPub.Room.Available || closedPub.Room.Open {
		t.Fatalf("an unreadable room is public as %+v, want no reading and not open", closedPub.Room)
	}

	raw, _ := json.Marshal(got)
	for _, leak := range []string{"@", "mosiah", "example", "u-old", "u-new", "paper", "vm-1"} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("capacity output carries %q: %s", leak, raw)
		}
	}
}

// vmctl reads the auth database the auth service writes (real file, read-only):
// who holds a passkey and when it was stored come through; a database it
// cannot read whole is "no reading", never every account counted as a person
// or none counted.
func TestPulseAccountsFromARealAuthDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	finished := time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC)
	for _, q := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, created_at DATETIME NOT NULL, email_verified_at DATETIME)`,
		`CREATE TABLE credentials (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, created_at DATETIME NOT NULL)`,
		`INSERT INTO users VALUES ('p', 'p@mosiah.org', '2026-11-23 01:00:00+00:00', '2026-11-23 01:00:00+00:00'), ('x', 'x@mosiah.org', '2026-11-23 01:00:00+00:00', NULL), ('q', 'q@mosiah.org', '2026-11-23 01:00:00+00:00', NULL)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []string{"p", "q"} {
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
			if !u.HasPasskey || !u.Proved || !u.FirstPasskeyAt.Equal(finished) {
				t.Fatalf("p = %+v", u)
			}
		case "q":
			if !u.HasPasskey || u.Proved {
				t.Fatalf("q (a passkey, no proof of its address) = %+v", u)
			}
		case "x":
			if u.HasPasskey {
				t.Fatalf("x = %+v", u)
			}
		}
	}

	// A database whose credentials table is missing is no reading.
	broken := filepath.Join(t.TempDir(), "broken.db")
	bdb, err := sql.Open("sqlite", "file:"+broken)
	if err != nil {
		t.Fatal(err)
	}
	defer bdb.Close()
	if _, err := bdb.Exec(`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT UNIQUE NOT NULL, created_at DATETIME NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	brokenUsers, brokenWarnings := loadPulseAccountsFromAuthDB(broken)
	if len(brokenWarnings) == 0 || brokenUsers != nil {
		t.Fatalf("a database with no credentials table read as available: %+v", brokenUsers)
	}
	plan := hostroom.Plan{ReserveBytes: 100 << 30, FootprintBytes: 9 << 30}
	sample := HostPressureSample{StateDirAvailableBytes: 154 << 30}
	// The account counts are no reading; the room does not depend on them.
	got := pulseCapacity(time.Now(), brokenUsers, len(brokenWarnings) == 0, nil, nil, true, sample, PressureReclaimConfig{}, plan).Room
	if got.Accounts != 0 || got.SignUpsLast24h != 0 {
		t.Fatalf("room = %+v, want no account figures", got)
	}
}

// Pulse reads back the file auth reads, with the age auth accepts: a missing
// or stale file is no reading, a fresh one is.
func TestPulseReadsTheOwnersFileAuthReads(t *testing.T) {
	reg := NewOwnershipRegistry("")
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	if _, ok := reg.roomOwnersReading(now); ok {
		t.Fatal("no path set read as available")
	}
	path := filepath.Join(t.TempDir(), "computer-owners.json")
	reg.SetRoomOwnersPath(path)
	if _, ok := reg.roomOwnersReading(now); ok {
		t.Fatal("a missing file read as available")
	}
	if err := hostroom.WriteOwners(path, now.Add(-time.Minute), []hostroom.Owner{{UserID: "a", DiskBytes: 7}}); err != nil {
		t.Fatal(err)
	}
	if owners, ok := reg.roomOwnersReading(now); !ok || owners["a"] != 7 {
		t.Fatalf("a fresh file = %v, %v", owners, ok)
	}
	if _, ok := reg.roomOwnersReading(now.Add(hostroom.MaxOwnersAge + time.Minute)); ok {
		t.Fatal("a stale file read as available")
	}
}
