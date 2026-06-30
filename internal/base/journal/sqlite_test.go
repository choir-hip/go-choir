package journal

import (
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func TestSQLiteAppendAndReadBack(t *testing.T) {
	j, err := NewSQLiteJournal(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer j.Close()

	e1 := mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0)
	entry, err := j.Append(e1)
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if entry.Event.CursorSeq != 1 {
		t.Errorf("expected cursor seq 1, got %d", entry.Event.CursorSeq)
	}
	if entry.Event.ParentEventID != "" {
		t.Errorf("first event parent should be empty, got %q", entry.Event.ParentEventID)
	}

	e2 := mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0)
	entry2, _ := j.Append(e2)
	if entry2.Event.ParentEventID != "base_evt_1" {
		t.Errorf("second event parent should be base_evt_1, got %q", entry2.Event.ParentEventID)
	}

	entries := j.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1].Event.ParentEventID != "base_evt_1" {
		t.Errorf("read-back parent should be base_evt_1, got %q", entries[1].Event.ParentEventID)
	}
}

func TestSQLiteAppendRejectsParentOnFirstItemEvent(t *testing.T) {
	j, err := NewSQLiteJournal(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer j.Close()

	first := mkEvent("base_evt_sqlite_1", "base_item_sqlite_1", model.EventCreate, 0)
	first.ParentEventID = "base_evt_missing"
	if _, err := j.Append(first); err == nil {
		t.Fatal("expected first event with explicit parent to fail")
	}
}

func TestSQLiteAppendWorksAcrossIndependentHandles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "base-journal.db")
	j1, err := NewSQLiteJournal(path)
	if err != nil {
		t.Fatalf("open sqlite j1: %v", err)
	}
	defer j1.Close()
	j2, err := NewSQLiteJournal(path)
	if err != nil {
		t.Fatalf("open sqlite j2: %v", err)
	}
	defer j2.Close()

	entryOne, err := j1.Append(mkEvent("base_evt_sqlite_handle_1", "base_item_sqlite_handle", model.EventCreate, 0))
	if err != nil {
		t.Fatalf("append via j1: %v", err)
	}
	entryTwo, err := j2.Append(mkEvent("base_evt_sqlite_handle_2", "base_item_sqlite_handle", model.EventUpdate, 0))
	if err != nil {
		t.Fatalf("append via j2: %v", err)
	}

	if entryTwo.Event.CursorSeq != entryOne.Event.CursorSeq+1 {
		t.Fatalf("cursor seq: got %d want %d", entryTwo.Event.CursorSeq, entryOne.Event.CursorSeq+1)
	}
	if entryTwo.Event.ParentEventID != entryOne.Event.EventID {
		t.Fatalf("parent id: got %q want %q", entryTwo.Event.ParentEventID, entryOne.Event.EventID)
	}
	if err := j1.VerifyChain(); err != nil {
		t.Fatalf("verify through j1: %v", err)
	}
	if err := j2.VerifyChain(); err != nil {
		t.Fatalf("verify through j2: %v", err)
	}
}

func TestSQLiteAppendKeepsParentEventIDOwnerScoped_whenOwnersShareItemID(t *testing.T) {
	j, err := NewSQLiteJournal(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer j.Close()
	itemID := "base_item_sqlite_shared"
	ownerOne := mkEvent("base_evt_sqlite_owner_1", itemID, model.EventCreate, 0)
	ownerOne.OwnerID = "owner_1"
	ownerTwo := mkEvent("base_evt_sqlite_owner_2", itemID, model.EventCreate, 0)
	ownerTwo.OwnerID = "owner_2"
	ownerOneUpdate := mkEvent("base_evt_sqlite_owner_1_update", itemID, model.EventUpdate, 0)
	ownerOneUpdate.OwnerID = "owner_1"

	entryOne, err := j.Append(ownerOne)
	if err != nil {
		t.Fatalf("append owner one: %v", err)
	}
	entryTwo, err := j.Append(ownerTwo)
	if err != nil {
		t.Fatalf("append owner two: %v", err)
	}
	entryOneUpdate, err := j.Append(ownerOneUpdate)
	if err != nil {
		t.Fatalf("append owner one update: %v", err)
	}

	if entryOne.Event.ParentEventID != "" {
		t.Fatalf("owner one parent: got %q want empty", entryOne.Event.ParentEventID)
	}
	if entryTwo.Event.ParentEventID != "" {
		t.Fatalf("owner two parent: got %q want empty", entryTwo.Event.ParentEventID)
	}
	if entryOneUpdate.Event.ParentEventID != entryOne.Event.EventID {
		t.Fatalf("owner one update parent: got %q want %q", entryOneUpdate.Event.ParentEventID, entryOne.Event.EventID)
	}
	if err := j.VerifyChain(); err != nil {
		t.Fatalf("verify chain: %v", err)
	}
}

func TestSQLiteVerifyChain(t *testing.T) {
	j, err := NewSQLiteJournal(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_3", "base_item_2", model.EventCreate, 0))

	if err := j.VerifyChain(); err != nil {
		t.Errorf("sqlite chain should verify: %v", err)
	}
}

func TestSQLiteAppendRejectsDuplicateEventID(t *testing.T) {
	j, err := NewSQLiteJournal(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	_, err = j.Append(mkEvent("base_evt_1", "base_item_1", model.EventUpdate, 0))
	if err == nil {
		t.Error("expected error for duplicate event id")
	}
}
