package store

import "github.com/google/uuid"

// newRunMemoryEntryID is the id of a new run-memory entry: a UUIDv7, whose
// first 48 bits are the Unix time in milliseconds, so the entry_id primary
// key (an autoindex) and idx_run_memory_entries_parent (a parent is the
// loop's previous entry) take each insert at their right edge instead of a
// random leaf of a 1.1-million-row B-tree
// (docs/design/append-clustered-store-2026-10-31.md, step S1). The id stays
// an opaque string: it is recorded on the tape in the projection op and
// replay writes the recorded one, and GetRunMemoryEntry reads it as given.
// Entries written before keep their UUIDv4 ids.
func newRunMemoryEntryID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}
