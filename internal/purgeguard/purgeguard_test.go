package purgeguard

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestCheckClaim(t *testing.T) {
	now := time.Now().UTC()
	if err := CheckClaim(time.Time{}, now); !errors.Is(err, ErrNoRecord) {
		t.Fatal(err)
	}
	if err := CheckClaim(now.Add(-GracePeriod+time.Second), now); !errors.Is(err, ErrTooEarly) {
		t.Fatal(err)
	}
	if err := CheckClaim(now.Add(time.Hour), now); !errors.Is(err, ErrTooEarly) {
		t.Fatal("a future request time was accepted")
	}
	if err := CheckClaim(now.Add(-GracePeriod), now); err != nil {
		t.Fatal(err)
	}
}

func TestClassRefused(t *testing.T) {
	for class, want := range map[string]bool{"critical_protected": true, "public_platform": true, " public_platform ": true, "primary": false, "premium_always_on": false, "test": false, "": false} {
		if ClassRefused(class) != want {
			t.Fatalf("%q", class)
		}
	}
}

func TestProtectedComputerList(t *testing.T) {
	for id := range ProtectedComputers {
		if what, ok := ProtectedComputer(" " + id + " "); !ok || what == "" {
			t.Fatalf("%s is not refused", id)
		}
	}
	if len(ProtectedComputers) != 2 {
		t.Fatalf("the hard list is the paper and the owner's computer; it holds %d", len(ProtectedComputers))
	}
	if _, ok := ProtectedComputer("computer-0000"); ok {
		t.Fatal("an ordinary computer is on the hard list")
	}
}

func openTempAuth(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE account_deletions (user_id TEXT PRIMARY KEY, state TEXT NOT NULL, requested_at DATETIME NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	return db
}

// A cancelled record (the tombstone a cancel leaves) is no record: it never
// makes a purge eligible, however old, and is not a pending account.
func TestACancelledRecordIsNoRecord(t *testing.T) {
	db := openTempAuth(t)
	old := time.Now().UTC().Add(-100 * time.Hour)
	for id, state := range map[string]string{"pend": "pending", "purg": "purging", "canc": "cancelled"} {
		if _, err := db.Exec(`INSERT INTO account_deletions VALUES (?, ?, ?)`, id, state, old); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	if err := Eligible(db, "pend", now); err != nil {
		t.Fatalf("pending and old: %v", err)
	}
	if err := Eligible(db, "canc", now); !errors.Is(err, ErrNoRecord) {
		t.Fatalf("a cancelled record made a purge eligible: %v", err)
	}
	set, err := PendingUsers(db)
	if err != nil || !set["pend"] || !set["purg"] || set["canc"] || len(set) != 2 {
		t.Fatalf("pending users: %v %v", set, err)
	}
}

func TestPendingUsersWithNoTableIsEmptyAndAnyOtherFailureIsAnError(t *testing.T) {
	empty, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Close()
	if set, err := PendingUsers(empty); err != nil || len(set) != 0 {
		t.Fatalf("no table: %v %v", set, err)
	}
	if _, err := OpenReadOnly(""); err == nil {
		t.Fatal("no path opened")
	}
	ro, err := OpenReadOnly(filepath.Join(t.TempDir(), "missing.db"))
	if err == nil {
		defer ro.Close()
		if _, err := PendingUsers(ro); err == nil {
			t.Fatal("an unreadable file read as an empty set (it must fail closed)")
		}
	}
}
