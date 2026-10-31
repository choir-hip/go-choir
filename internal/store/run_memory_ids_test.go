package store

import "testing"

// Failure modes of the time-ordered entry id (step S1 of
// docs/design/append-clustered-store-2026-10-31.md), written before the code:
//
//	S1a ids do not sort in creation order, so the entry_id autoindex still
//	    takes a random leaf per insert;
//	S1b an id is not a UUID string (GetRunMemoryEntry callers pass it back).
func TestRunMemoryEntryIDsSortInCreationOrder(t *testing.T) {
	prev := ""
	for i := 0; i < 20000; i++ {
		id := newRunMemoryEntryID()
		if len(id) != 36 || id[14] != '7' {
			t.Fatalf("S1b: %q is not a UUIDv7 string", id)
		}
		if id <= prev {
			t.Fatalf("S1a: id %d %q sorts before its predecessor %q", i, id, prev)
		}
		prev = id
	}
}
