// Package journal implements an append-only event store for the Choir Base
// reconciliation kernel.
//
// The journal is a tamper-evident tape: each event carries a ParentEventID
// linking it to the previous event for the same item, and a content hash
// chaining every entry to its predecessor. The hash chain makes any
// mutation, deletion, or reordering of a past event detectable via
// VerifyChain.
//
// CursorSeq is a monotonic sequence number assigned by the journal on
// append. Device cursors track per-device sync positions so a device can
// resume from where it left off.
//
// Two implementations are provided:
//   - MemJournal: in-memory, for unit tests and fast iteration.
//   - SQLiteJournal: persistent, backed by SQLite (modernc.org/sqlite).
package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// Entry pairs a model.Event with its computed chain hash. The hash is
// SHA-256 over the parent entry's hash concatenated with a canonical JSON
// encoding of the event, forming a tamper-evident hash chain per item.
type Entry struct {
	Event model.Event
	Hash  string
}

// Journal is an append-only event store with parent-event chaining and
// per-device cursor tracking.
type Journal interface {
	// Append adds an event to the journal. The journal assigns CursorSeq
	// (if zero) and ParentEventID (based on the previous event for the
	// same item), computes the chain hash, and persists the entry. The
	// returned Entry has all assigned fields populated. Append is
	// append-only: past entries are never mutated.
	Append(evt model.Event) (Entry, error)

	// Entries returns all journal entries in CursorSeq order.
	Entries() []Entry

	// EntriesUpTo returns entries with CursorSeq <= maxSeq, in CursorSeq
	// order. This is the slice consumed by tree.DeriveUpTo to rebuild a
	// snapshot at a point in time.
	EntriesUpTo(maxSeq int64) []Entry

	// Cursor returns the last-acked CursorSeq for a device. Returns 0 if
	// the device has no recorded cursor.
	Cursor(deviceID string) int64

	// SetCursor records the sync position for a device. The sequence must
	// not exceed the journal's current head.
	SetCursor(deviceID string, seq int64) error

	// VerifyChain walks every entry and confirms the hash chain is
	// intact, ParentEventID links are consistent, and CursorSeq is
	// monotonic. Returns nil if the tape is tamper-free.
	VerifyChain() error

	// Close releases any resources held by the journal.
	Close() error
}

// --- hash chain ---------------------------------------------------------

// computeHash returns the SHA-256 hex digest over the parent hash
// concatenated with a canonical JSON encoding of the event. This is the
// link that makes the tape tamper-evident: changing any field of a past
// event changes its hash, which breaks the chain for every descendant.
func computeHash(evt model.Event, parentHash string) string {
	b, err := json.Marshal(evt)
	if err != nil {
		// json.Marshal of a plain struct with string/int/time fields
		// cannot fail in practice; fall back to a stable encoding.
		b = []byte(fmt.Sprintf("%v", evt))
	}
	digest := sha256.New()
	digest.Write([]byte(parentHash))
	digest.Write(b)
	return hex.EncodeToString(digest.Sum(nil))
}

// --- MemJournal ---------------------------------------------------------

// MemJournal is an in-memory append-only event store. It is safe for
// concurrent use.
type MemJournal struct {
	mu         sync.Mutex
	entries    []Entry
	lastByItem map[model.ItemID]model.EventID
	lastSeq    int64
	cursors    map[string]int64
}

// NewMemJournal returns a new empty in-memory journal.
func NewMemJournal() *MemJournal {
	return &MemJournal{
		lastByItem: make(map[model.ItemID]model.EventID),
		cursors:    make(map[string]int64),
	}
}

// Append adds an event to the in-memory journal.
func (j *MemJournal) Append(evt model.Event) (Entry, error) {
	if err := validateEvent(evt); err != nil {
		return Entry{}, err
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	// Assign CursorSeq if the caller left it zero.
	if evt.CursorSeq == 0 {
		evt.CursorSeq = j.lastSeq + 1
	}
	if evt.CursorSeq <= j.lastSeq {
		return Entry{}, fmt.Errorf("journal: cursor seq %d is not greater than head %d (append-only)", evt.CursorSeq, j.lastSeq)
	}

	// Assign ParentEventID from the previous event for this item. If the
	// caller set it explicitly, it must match the known predecessor.
	expectedParent := j.lastByItem[evt.ItemID]
	if evt.ParentEventID == "" {
		evt.ParentEventID = expectedParent
	} else if expectedParent != "" && evt.ParentEventID != expectedParent {
		return Entry{}, fmt.Errorf("journal: parent event %q does not match known predecessor %q for item %s",
			evt.ParentEventID, expectedParent, evt.ItemID)
	}

	// Compute the chain hash from the parent entry's hash.
	parentHash := ""
	if expectedParent != "" {
		for i := len(j.entries) - 1; i >= 0; i-- {
			if j.entries[i].Event.ItemID == evt.ItemID {
				parentHash = j.entries[i].Hash
				break
			}
		}
	}

	hash := computeHash(evt, parentHash)
	entry := Entry{Event: evt, Hash: hash}

	j.entries = append(j.entries, entry)
	j.lastByItem[evt.ItemID] = evt.EventID
	j.lastSeq = evt.CursorSeq

	return entry, nil
}

// Entries returns all journal entries in CursorSeq order.
func (j *MemJournal) Entries() []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Entry, len(j.entries))
	copy(out, j.entries)
	sort.Slice(out, func(i, k int) bool { return out[i].Event.CursorSeq < out[k].Event.CursorSeq })
	return out
}

// EntriesUpTo returns entries with CursorSeq <= maxSeq, in CursorSeq order.
func (j *MemJournal) EntriesUpTo(maxSeq int64) []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	var out []Entry
	for _, e := range j.entries {
		if e.Event.CursorSeq <= maxSeq {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Event.CursorSeq < out[k].Event.CursorSeq })
	return out
}

// Cursor returns the last-acked CursorSeq for a device.
func (j *MemJournal) Cursor(deviceID string) int64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cursors[deviceID]
}

// SetCursor records the sync position for a device.
func (j *MemJournal) SetCursor(deviceID string, seq int64) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if seq < 0 {
		return fmt.Errorf("journal: cursor seq must be non-negative, got %d", seq)
	}
	if seq > j.lastSeq {
		return fmt.Errorf("journal: cursor seq %d exceeds journal head %d", seq, j.lastSeq)
	}
	j.cursors[deviceID] = seq
	return nil
}

// VerifyChain walks every entry and confirms the hash chain is intact.
func (j *MemJournal) VerifyChain() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return verifyChain(j.entries)
}

// Close is a no-op for the in-memory journal.
func (j *MemJournal) Close() error { return nil }

// --- shared helpers -----------------------------------------------------

// validateEvent checks that an event has the required fields before
// appending. It does not check CursorSeq or ParentEventID (those are
// assigned by the journal).
func validateEvent(evt model.Event) error {
	if !evt.EventID.Valid() {
		return fmt.Errorf("journal: invalid event id %q", evt.EventID)
	}
	if !evt.ItemID.Valid() {
		return fmt.Errorf("journal: invalid item id %q", evt.ItemID)
	}
	if !evt.EventType.Valid() {
		return fmt.Errorf("journal: invalid event type %q", evt.EventType)
	}
	if evt.OwnerID == "" {
		return fmt.Errorf("journal: event has no owner id")
	}
	return nil
}

// verifyChain walks a slice of entries (in CursorSeq order) and confirms:
//   - CursorSeq is strictly increasing.
//   - Each entry's ParentEventID matches the previous event for the same
//     item (empty for the first event of an item).
//   - Each entry's hash equals computeHash(event, parentHash).
//
// Any discrepancy means the tape has been tampered with.
func verifyChain(entries []Entry) error {
	// Index entries by EventID for parent lookup.
	byID := make(map[model.EventID]Entry, len(entries))
	lastByItem := make(map[model.ItemID]model.EventID)
	var lastSeq int64 = -1

	for _, e := range entries {
		if e.Event.CursorSeq <= lastSeq {
			return fmt.Errorf("journal: cursor seq %d is not strictly greater than previous %d",
				e.Event.CursorSeq, lastSeq)
		}
		lastSeq = e.Event.CursorSeq

		expectedParent := lastByItem[e.Event.ItemID]
		if e.Event.ParentEventID != expectedParent {
			return fmt.Errorf("journal: event %s has parent %q but expected %q for item %s",
				e.Event.EventID, e.Event.ParentEventID, expectedParent, e.Event.ItemID)
		}

		parentHash := ""
		if expectedParent != "" {
			parent, ok := byID[expectedParent]
			if !ok {
				return fmt.Errorf("journal: parent event %q not found in chain", expectedParent)
			}
			parentHash = parent.Hash
		}

		want := computeHash(e.Event, parentHash)
		if e.Hash != want {
			return fmt.Errorf("journal: hash mismatch for event %s: stored %s, recomputed %s (tape tampered)",
				e.Event.EventID, e.Hash, want)
		}

		byID[e.Event.EventID] = e
		lastByItem[e.Event.ItemID] = e.Event.EventID
	}
	return nil
}

// Events extracts the raw model.Event slice from a slice of entries, in
// CursorSeq order. This is the input consumed by tree.Derive.
func Events(entries []Entry) []model.Event {
	out := make([]model.Event, len(entries))
	for i, e := range entries {
		out[i] = e.Event
	}
	return out
}
