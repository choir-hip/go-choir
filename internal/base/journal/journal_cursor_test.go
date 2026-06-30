package journal

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func TestMemCursorTracking(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_3", "base_item_1", model.EventDelete, 0))

	if got := j.Cursor("dev1"); got != 0 {
		t.Errorf("unset cursor should be 0, got %d", got)
	}

	if err := j.SetCursor("dev1", 2); err != nil {
		t.Fatalf("set cursor: %v", err)
	}
	if got := j.Cursor("dev1"); got != 2 {
		t.Errorf("cursor should be 2, got %d", got)
	}

	entries := j.EntriesUpTo(2)
	if len(entries) != 2 {
		t.Errorf("EntriesUpTo(2) should return 2 entries, got %d", len(entries))
	}
}

func TestMemCursorRejectsExceedsHead(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))

	if err := j.SetCursor("dev1", 99); err == nil {
		t.Error("expected error for cursor exceeding head")
	}
}

func TestMemCursorMultipleDevices(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_3", "base_item_1", model.EventDelete, 0))

	j.SetCursor("dev1", 1)
	j.SetCursor("dev2", 3)

	if j.Cursor("dev1") != 1 {
		t.Errorf("dev1 cursor should be 1, got %d", j.Cursor("dev1"))
	}
	if j.Cursor("dev2") != 3 {
		t.Errorf("dev2 cursor should be 3, got %d", j.Cursor("dev2"))
	}
}

func TestMemCursorTrackingIsOwnerScoped_whenOwnersShareDeviceID(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_3", "base_item_1", model.EventDelete, 0))

	if err := j.SetCursorForOwner("owner_1", "shared-device", 1); err != nil {
		t.Fatalf("set owner 1 cursor: %v", err)
	}
	if err := j.SetCursorForOwner("owner_2", "shared-device", 3); err != nil {
		t.Fatalf("set owner 2 cursor: %v", err)
	}

	if got := j.CursorForOwner("owner_1", "shared-device"); got != 1 {
		t.Fatalf("owner 1 cursor: got %d want 1", got)
	}
	if got := j.CursorForOwner("owner_2", "shared-device"); got != 3 {
		t.Fatalf("owner 2 cursor: got %d want 3", got)
	}
}

func TestSQLiteCursorTracking(t *testing.T) {
	j, err := NewSQLiteJournal(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))

	if err := j.SetCursor("dev1", 2); err != nil {
		t.Fatalf("set cursor: %v", err)
	}
	if j.Cursor("dev1") != 2 {
		t.Errorf("cursor should be 2, got %d", j.Cursor("dev1"))
	}

	entries := j.EntriesUpTo(1)
	if len(entries) != 1 {
		t.Errorf("EntriesUpTo(1) should return 1 entry, got %d", len(entries))
	}
}

func TestSQLiteCursorTrackingIsOwnerScoped_whenOwnersShareDeviceID(t *testing.T) {
	j, err := NewSQLiteJournal(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer j.Close()

	j.Append(mkEvent("base_evt_sqlite_cursor_1", "base_item_sqlite_cursor", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_sqlite_cursor_2", "base_item_sqlite_cursor", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_sqlite_cursor_3", "base_item_sqlite_cursor", model.EventDelete, 0))

	if err := j.SetCursorForOwner("owner_1", "shared-device", 1); err != nil {
		t.Fatalf("set owner 1 cursor: %v", err)
	}
	if err := j.SetCursorForOwner("owner_2", "shared-device", 3); err != nil {
		t.Fatalf("set owner 2 cursor: %v", err)
	}

	if got := j.CursorForOwner("owner_1", "shared-device"); got != 1 {
		t.Fatalf("owner 1 cursor: got %d want 1", got)
	}
	if got := j.CursorForOwner("owner_2", "shared-device"); got != 3 {
		t.Fatalf("owner 2 cursor: got %d want 3", got)
	}
}

func TestSQLiteCursorMigrationPreservesLegacyDeviceCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-cursor.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy sqlite: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE device_cursors (
		device_id TEXT PRIMARY KEY,
		cursor_seq INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy cursor table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO device_cursors (device_id, cursor_seq) VALUES (?, ?)`, "legacy-device", 2); err != nil {
		t.Fatalf("insert legacy cursor: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy sqlite: %v", err)
	}

	j, err := NewSQLiteJournal(path)
	if err != nil {
		t.Fatalf("open migrated journal: %v", err)
	}
	defer j.Close()

	if got := j.Cursor("legacy-device"); got != 2 {
		t.Fatalf("legacy compatibility cursor: got %d want 2", got)
	}
	if got := j.CursorForOwner("owner_1", "legacy-device"); got != 0 {
		t.Fatalf("owner-scoped cursor should not inherit legacy value: got %d want 0", got)
	}
}
