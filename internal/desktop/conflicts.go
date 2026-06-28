package desktop

import (
	"sync"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
)

// ConflictResolution is the user's choice for resolving a conflict.
type ConflictResolution string

const (
	// ResolveKeepLocal applies the local version to the remote (upload/update
	// remote). The local content wins.
	ResolveKeepLocal ConflictResolution = "keep_local"
	// ResolveKeepRemote applies the remote version to the local (download/
	// update local). The remote content wins.
	ResolveKeepRemote ConflictResolution = "keep_remote"
	// ResolveKeepBoth preserves both: the local version is uploaded as a new
	// item (renamed), and the remote version is downloaded. Neither side is
	// discarded.
	ResolveKeepBoth ConflictResolution = "keep_both"
)

// Valid reports whether the resolution is one of the defined constants.
func (r ConflictResolution) Valid() bool {
	switch r {
	case ResolveKeepLocal, ResolveKeepRemote, ResolveKeepBoth:
		return true
	}
	return false
}

// ConflictRecord is a surfaced conflict awaiting user resolution. It carries
// both the local and remote versions (preserved, never silently dropped) and
// the planner's reason.
type ConflictRecord struct {
	ItemID    model.ItemID       `json:"item_id"`
	Path      string             `json:"path,omitempty"`
	Reason    string             `json:"reason"`
	LocalVer  model.Version      `json:"local_version"`
	RemoteVer model.Version      `json:"remote_version"`
	SyncedVer model.Version      `json:"synced_version,omitempty"`
	Resolved  ConflictResolution `json:"resolved,omitempty"`
}

// ConflictManager collects conflicts produced by the planner and tracks
// per-conflict user resolutions. Conflicts are NEVER silently resolved: the
// sync engine pauses execution when conflicts are present and waits for the
// user to choose via Resolve.
//
// The manager is concurrency-safe; the sync engine writes conflicts from the
// sync goroutine and the frontend reads/resolves from the UI goroutine.
type ConflictManager struct {
	mu        sync.Mutex
	conflicts map[model.ItemID]ConflictRecord
}

// NewConflictManager returns an empty conflict manager.
func NewConflictManager() *ConflictManager {
	return &ConflictManager{conflicts: make(map[model.ItemID]ConflictRecord)}
}

// SetConflicts replaces the current conflict set with the planner's output.
// Already-resolved conflicts for items that are no longer in conflict are
// cleared. This is called once per sync cycle after the planner runs.
func (m *ConflictManager) SetConflicts(cs []planner.Conflict, local, remote planner.Tree) {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := make(map[model.ItemID]ConflictRecord, len(cs))
	for _, c := range cs {
		rec := ConflictRecord{
			ItemID:    c.ItemID,
			Reason:    c.Reason,
			LocalVer:  c.LocalVer,
			RemoteVer: c.RemoteVer,
			SyncedVer: c.SyncedVer,
		}
		rec.Path = RelPathFromID(local, c.ItemID)
		if rec.Path == "" {
			rec.Path = RelPathFromID(remote, c.ItemID)
		}
		// Preserve a prior resolution if the same item is still in conflict.
		if prior, ok := m.conflicts[c.ItemID]; ok && prior.Resolved.Valid() {
			rec.Resolved = prior.Resolved
		}
		next[c.ItemID] = rec
	}
	m.conflicts = next
}

// Pending returns the conflicts that have not yet been resolved. These block
// sync execution for the affected items.
func (m *ConflictManager) Pending() []ConflictRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ConflictRecord, 0, len(m.conflicts))
	for _, c := range m.conflicts {
		if !c.Resolved.Valid() {
			out = append(out, c)
		}
	}
	return out
}

// All returns every conflict (resolved and pending).
func (m *ConflictManager) All() []ConflictRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ConflictRecord, 0, len(m.conflicts))
	for _, c := range m.conflicts {
		out = append(out, c)
	}
	return out
}

// Resolve records the user's resolution for a conflict. It returns an error
// if the item is not in conflict or the resolution is invalid.
func (m *ConflictManager) Resolve(itemID model.ItemID, resolution ConflictResolution) error {
	if !resolution.Valid() {
		return ErrInvalidResolution
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.conflicts[itemID]
	if !ok {
		return ErrConflictNotFound
	}
	rec.Resolved = resolution
	m.conflicts[itemID] = rec
	return nil
}

// Resolution returns the recorded resolution for an item, or false if the
// item has no conflict or the conflict is unresolved.
func (m *ConflictManager) Resolution(itemID model.ItemID) (ConflictResolution, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.conflicts[itemID]
	if !ok || !rec.Resolved.Valid() {
		return "", false
	}
	return rec.Resolved, true
}

// HasUnresolved reports whether any conflict lacks a user resolution. The
// sync engine uses this to decide whether to pause for user input.
func (m *ConflictManager) HasUnresolved() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.conflicts {
		if !c.Resolved.Valid() {
			return true
		}
	}
	return false
}

// Clear removes all conflicts. Called after a successful sync cycle that
// applied all resolutions.
func (m *ConflictManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.conflicts = make(map[model.ItemID]ConflictRecord)
}

// Count returns the total number of conflicts (resolved + pending).
func (m *ConflictManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.conflicts)
}

// Sentinel errors for the conflict manager.
var (
	// ErrConflictNotFound is returned by Resolve when the item ID has no
	// recorded conflict.
	ErrConflictNotFound = errSentinel("conflict not found for item")
	// ErrInvalidResolution is returned by Resolve when the resolution value
	// is not one of the defined constants.
	ErrInvalidResolution = errSentinel("invalid conflict resolution")
)

// errSentinel is a simple error type that satisfies the error interface with
// a fixed message. We avoid fmt.Errorf here so equality checks are stable.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }
