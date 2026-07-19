package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
)

// Tape is a tamper-evident append-only log of capsule transaction records.
// Each entry is linked to the previous entry via a SHA-256 hash chain,
// making any modification or deletion detectable.
//
// The tape models the candidate branch's transaction history in the TLA+
// promotion protocol spec (capsuleTxns variable). Each CapsuleTxn action
// appends one entry to the tape. The tape is:
//   - Append-only: no edits, no deletes before merge (CapsuleTapeIntegrity)
//   - Tamper-evident: hash chain links each entry to the previous
//   - Content-addressed: each entry's hash is derived from its content
//     and the previous entry's hash
type Tape struct {
	mu      sync.Mutex
	entries []TapeEntry
}

// TapeEntry is one record in the tamper-evident tape.
type TapeEntry struct {
	Index    int                `json:"index"`
	PrevHash string             `json:"prev_hash"` // hash of the previous entry (empty for genesis)
	Hash     string             `json:"hash"`      // hash of this entry (content + prevHash)
	Record   *TransactionRecord `json:"record"`
}

// NewTape creates a new empty tape.
func NewTape() *Tape {
	return &Tape{
		entries: make([]TapeEntry, 0),
	}
}

// Append adds a transaction record to the tape. The record is hashed
// with the previous entry's hash to form a tamper-evident chain.
// Returns the hash of the new entry.
func (t *Tape) Append(record *TransactionRecord) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if record == nil {
		return "", fmt.Errorf("tape: cannot append nil record")
	}

	// Rejected records are NOT appended to the tape.
	if record.Rejected {
		return "", fmt.Errorf("tape: rejected record cannot be appended: %s", record.RejectReason)
	}

	recordBytes, err := record.MarshalForTape()
	if err != nil {
		return "", fmt.Errorf("tape: marshal record: %w", err)
	}

	var prevHash string
	index := len(t.entries)
	if index > 0 {
		prevHash = t.entries[index-1].Hash
	}

	// Compute the entry hash: SHA-256(prevHash || recordJSON).
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(recordBytes)
	entryHash := hex.EncodeToString(h.Sum(nil))

	entry := TapeEntry{
		Index:    index,
		PrevHash: prevHash,
		Hash:     entryHash,
		Record:   record,
	}
	t.entries = append(t.entries, entry)

	return entryHash, nil
}

// Entries returns a copy of all tape entries.
func (t *Tape) Entries() []TapeEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TapeEntry, len(t.entries))
	copy(out, t.entries)
	return out
}

// Head returns the hash of the last entry, or empty string if the tape is empty.
func (t *Tape) Head() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.entries) == 0 {
		return ""
	}
	return t.entries[len(t.entries)-1].Hash
}

// Len returns the number of entries in the tape.
func (t *Tape) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.entries)
}

// Verify checks the tamper-evident chain. Returns nil if the chain is
// intact, or an error describing the first broken link.
func (t *Tape) Verify() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i, entry := range t.entries {
		if entry.Index != i {
			return fmt.Errorf("tape: index mismatch at %d: expected %d, got %d", i, i, entry.Index)
		}

		var expectedPrev string
		if i > 0 {
			expectedPrev = t.entries[i-1].Hash
		}
		if entry.PrevHash != expectedPrev {
			return fmt.Errorf("tape: broken chain at index %d: prev_hash mismatch", i)
		}

		// Recompute the entry hash.
		recordBytes, err := entry.Record.MarshalForTape()
		if err != nil {
			return fmt.Errorf("tape: marshal record at index %d: %w", i, err)
		}
		h := sha256.New()
		h.Write([]byte(entry.PrevHash))
		h.Write(recordBytes)
		expectedHash := hex.EncodeToString(h.Sum(nil))
		if entry.Hash != expectedHash {
			return fmt.Errorf("tape: hash mismatch at index %d: expected %s, got %s", i, expectedHash, entry.Hash)
		}
	}
	return nil
}

// MarshalJSON serializes the tape for persistence.
func (t *Tape) MarshalJSON() ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return json.Marshal(t.entries)
}

// UnmarshalJSON deserializes the tape from persisted state.
func (t *Tape) UnmarshalJSON(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return json.Unmarshal(data, &t.entries)
}

// Reset clears the tape to empty. Used when a candidate is aborted or
// reverted (the branch is dropped).
func (t *Tape) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = make([]TapeEntry, 0)
}
