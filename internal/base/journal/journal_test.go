package journal

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// fixedTime is a deterministic timestamp for all journal tests. The
// journal never reads a wall clock — it only stores and compares the
// timestamps carried by events.
var fixedTime = time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

// mkEvent builds a minimal valid event for testing.
func mkEvent(eid, iid string, et model.EventType, seq int64) model.Event {
	return model.Event{
		EventID:   model.EventID(eid),
		OwnerID:   "owner",
		ItemID:    model.ItemID(iid),
		DeviceID:  "dev1",
		SubjectID: "user1",
		EventType: et,
		Kind:      model.KindFile,
		CursorSeq: seq,
		CreatedAt: fixedTime,
	}
}

// --- append tests -------------------------------------------------------

func TestMemAppendAssignsCursorSeq(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	e1 := mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0)
	entry, err := j.Append(e1)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if entry.Event.CursorSeq != 1 {
		t.Errorf("first event should get cursor seq 1, got %d", entry.Event.CursorSeq)
	}

	e2 := mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0)
	entry2, err := j.Append(e2)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if entry2.Event.CursorSeq != 2 {
		t.Errorf("second event should get cursor seq 2, got %d", entry2.Event.CursorSeq)
	}
}

func TestMemAppendSetsParentEventID(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	e1 := mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0)
	entry1, err := j.Append(e1)
	if err != nil {
		t.Fatalf("append e1: %v", err)
	}
	if entry1.Event.ParentEventID != "" {
		t.Errorf("first event should have empty parent, got %q", entry1.Event.ParentEventID)
	}

	e2 := mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0)
	entry2, err := j.Append(e2)
	if err != nil {
		t.Fatalf("append e2: %v", err)
	}
	if entry2.Event.ParentEventID != "base_evt_1" {
		t.Errorf("second event parent should be base_evt_1, got %q", entry2.Event.ParentEventID)
	}
}

func TestMemAppendRejectsNonMonotonicSeq(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	if _, err := j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0)); err != nil {
		t.Fatalf("append e1: %v", err)
	}
	// Explicitly set a seq that is not greater than the head (1).
	_, err := j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 1))
	if err == nil {
		t.Error("expected error for non-monotonic cursor seq")
	}
}

func TestMemAppendRejectsInvalidEvent(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	bad := mkEvent("nope", "base_item_1", model.EventCreate, 0)
	if _, err := j.Append(bad); err == nil {
		t.Error("expected error for invalid event id")
	}

	badType := mkEvent("base_evt_1", "base_item_1", "frobnicate", 0)
	if _, err := j.Append(badType); err == nil {
		t.Error("expected error for invalid event type")
	}
}

func TestMemAppendIsAppendOnly(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_3", "base_item_1", model.EventDelete, 0))

	entries := j.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// The journal must not allow mutation of past entries. There is no
	// mutation API; we verify the entries are immutable by confirming the
	// stored slice matches what was appended.
	if entries[0].Event.EventID != "base_evt_1" {
		t.Errorf("first entry should be base_evt_1, got %s", entries[0].Event.EventID)
	}
	if entries[2].Event.EventID != "base_evt_3" {
		t.Errorf("third entry should be base_evt_3, got %s", entries[2].Event.EventID)
	}
}

// --- chain verification tests -------------------------------------------

func TestMemVerifyChainIntact(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_3", "base_item_2", model.EventCreate, 0))

	if err := j.VerifyChain(); err != nil {
		t.Errorf("chain verification should pass on intact tape: %v", err)
	}
}

func TestMemVerifyChainDetectsTamperedHash(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))

	// Tamper: mutate a past entry's hash directly.
	j.mu.Lock()
	j.entries[0].Hash = "tampered"
	j.mu.Unlock()

	err := j.VerifyChain()
	if err == nil {
		t.Fatal("expected chain verification to fail after tampering")
	}
	if !strings.Contains(err.Error(), "hash mismatch") {
		t.Errorf("expected hash mismatch error, got %v", err)
	}
}

func TestMemVerifyChainDetectsBrokenParentLink(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))

	// Tamper: break the parent link.
	j.mu.Lock()
	j.entries[1].Event.ParentEventID = "base_evt_wrong"
	j.mu.Unlock()

	err := j.VerifyChain()
	if err == nil {
		t.Fatal("expected chain verification to fail after breaking parent link")
	}
}

func TestMemVerifyChainDetectsReorderedSeq(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))

	// Tamper: swap cursor seqs.
	j.mu.Lock()
	j.entries[0].Event.CursorSeq = 5
	j.mu.Unlock()

	err := j.VerifyChain()
	if err == nil {
		t.Fatal("expected chain verification to fail after reordering")
	}
}

func TestMemVerifyChainMultipleItems(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	// Interleave events for two items; each item has its own chain.
	j.Append(mkEvent("base_evt_1", "base_item_a", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_b", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_3", "base_item_a", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_4", "base_item_b", model.EventMove, 0))

	if err := j.VerifyChain(); err != nil {
		t.Errorf("multi-item chain should verify: %v", err)
	}

	entries := j.Entries()
	// Verify per-item parent chains.
	if entries[0].Event.ParentEventID != "" {
		t.Errorf("item a first event should have empty parent")
	}
	if entries[1].Event.ParentEventID != "" {
		t.Errorf("item b first event should have empty parent")
	}
	if entries[2].Event.ParentEventID != "base_evt_1" {
		t.Errorf("item a second event parent should be base_evt_1, got %q", entries[2].Event.ParentEventID)
	}
	if entries[3].Event.ParentEventID != "base_evt_2" {
		t.Errorf("item b second event parent should be base_evt_2, got %q", entries[3].Event.ParentEventID)
	}
}

// --- cursor tracking tests ----------------------------------------------

func TestMemCursorTracking(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))
	j.Append(mkEvent("base_evt_3", "base_item_1", model.EventDelete, 0))

	// Device hasn't acked anything.
	if got := j.Cursor("dev1"); got != 0 {
		t.Errorf("unset cursor should be 0, got %d", got)
	}

	if err := j.SetCursor("dev1", 2); err != nil {
		t.Fatalf("set cursor: %v", err)
	}
	if got := j.Cursor("dev1"); got != 2 {
		t.Errorf("cursor should be 2, got %d", got)
	}

	// EntriesUpTo should return only events up to the cursor.
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

// --- SQLite journal tests -----------------------------------------------

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

// --- Events helper test -------------------------------------------------

func TestEventsHelper(t *testing.T) {
	j := NewMemJournal()
	defer j.Close()

	j.Append(mkEvent("base_evt_1", "base_item_1", model.EventCreate, 0))
	j.Append(mkEvent("base_evt_2", "base_item_1", model.EventUpdate, 0))

	events := Events(j.Entries())
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].EventID != "base_evt_1" {
		t.Errorf("first event should be base_evt_1, got %s", events[0].EventID)
	}
}
