// Package model defines the value types for the Choir Base reconciliation
// kernel. These types are pure values: they carry no I/O, no wall clock, no
// network, and no random source. They describe the immutable vocabulary used
// by the planner, journal, blob store, and sync status surfaces.
//
// Identity is path-independent: an Item is identified by a stable ItemID
// (base_item_<uuid>), not by its location in a tree. Content is
// content-addressed via BlobRef (sha256:<hex>). Versions are immutable
// snapshots of an item at one point in time.
package model

import (
	"strings"
	"time"
)

// ItemID is a stable, path-independent identifier for a file or folder.
// Format: base_item_<uuid>
type ItemID string

// Valid reports whether the ItemID has the required base_item_ prefix and a
// non-empty body.
func (id ItemID) Valid() bool {
	return strings.HasPrefix(string(id), "base_item_") && len(id) > len("base_item_")
}

// BlobRef is a content-addressed reference to immutable bytes.
// Format: sha256:<hex>
type BlobRef string

// Valid reports whether the BlobRef is empty (valid for folders) or has the
// sha256: prefix followed by 64 lowercase hex characters.
func (b BlobRef) Valid() bool {
	if b == "" {
		return true // folders carry no blob
	}
	s := string(b)
	if !strings.HasPrefix(s, "sha256:") {
		return false
	}
	hex := s[len("sha256:"):]
	if len(hex) != 64 {
		return false
	}
	for _, c := range hex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// VersionID is a unique identifier for one version of an item.
// Format: base_ver_<uuid>
type VersionID string

// Valid reports whether the VersionID is empty (valid for a deleted item) or
// has the required base_ver_ prefix and a non-empty body.
func (v VersionID) Valid() bool {
	if v == "" {
		return true // deleted items carry no current version
	}
	return strings.HasPrefix(string(v), "base_ver_") && len(v) > len("base_ver_")
}

// EventID is a unique identifier for a journal event.
// Format: base_evt_<uuid>
type EventID string

// Valid reports whether the EventID has the required base_evt_ prefix and a
// non-empty body.
func (e EventID) Valid() bool {
	return strings.HasPrefix(string(e), "base_evt_") && len(e) > len("base_evt_")
}

// ItemKind classifies an Item as a file or a folder.
type ItemKind string

const (
	KindFile   ItemKind = "file"
	KindFolder ItemKind = "folder"
)

// Valid reports whether the ItemKind is one of the defined constants.
func (k ItemKind) Valid() bool {
	switch k {
	case KindFile, KindFolder:
		return true
	}
	return false
}

// Item represents a file or folder in the Base namespace. The Item record is
// the mutable, location-bearing side of identity; the immutable content lives
// in the Version it points at.
type Item struct {
	ItemID         ItemID
	OwnerID        string
	ParentItemID   ItemID    // empty for root
	Name           string    // basename within parent
	Kind           ItemKind
	CurrentVersion VersionID // empty if deleted
	DeletedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Valid reports whether the Item has a valid identity, kind, and version
// reference. It does NOT consult the clock (CreatedAt/UpdatedAt may be zero
// in tests).
func (i Item) Valid() bool {
	if !i.ItemID.Valid() {
		return false
	}
	if !i.Kind.Valid() {
		return false
	}
	if !i.CurrentVersion.Valid() {
		return false
	}
	// A folder carries no blob, so its version's BlobRef must be empty; that
	// constraint is enforced on the Version, not here.
	if i.Kind == KindFolder && i.CurrentVersion == "" && i.DeletedAt == nil {
		// a folder with no version and no deletion marker is ambiguous
		return false
	}
	return true
}

// Location returns the (parent, name) pair that identifies the item's position
// in a tree. Two items with the same Location collide.
func (i Item) Location() (ItemID, string) {
	return i.ParentItemID, i.Name
}

// Version represents one immutable snapshot of an item's content. A Version
// is content-addressed in spirit: its VersionID is paired with a ContentHash
// and (for files) a BlobRef pointing at the immutable bytes.
type Version struct {
	VersionID        VersionID
	ItemID           ItemID
	BlobRef          BlobRef // empty for folder
	MediaType        string
	ContentHash      string // hex SHA-256 of content
	ManifestJSON     string // filesystem metadata (mode, mtime, size)
	ProvenanceJSON   string // author, device, subject
	CreatedByDevice  string
	CreatedBySubject string // user ID or API key ID
	CreatedAt        time.Time
}

// Valid reports whether the Version has a valid ID, item reference, and a
// blob ref consistent with whether it is a file or folder version. A folder
// version must have an empty BlobRef; a file version must have a non-empty,
// valid BlobRef.
func (v Version) Valid() bool {
	if !v.VersionID.Valid() {
		return false
	}
	if v.VersionID == "" {
		return false
	}
	if !v.ItemID.Valid() {
		return false
	}
	if !v.BlobRef.Valid() {
		return false
	}
	// A file version must reference a blob; a folder version must not.
	// We infer file-ness from a non-empty BlobRef.
	if v.BlobRef == "" {
		// folder version: content hash should be empty
		if v.ContentHash != "" {
			return false
		}
	}
	return true
}

// Blob represents immutable content-addressed bytes.
type Blob struct {
	BlobRef   BlobRef
	SizeBytes int64
	SHA256    string
	CreatedAt time.Time
}

// Valid reports whether the Blob has a valid, non-empty BlobRef whose hex
// matches the recorded SHA256, and a non-negative size.
func (b Blob) Valid() bool {
	if b.BlobRef == "" || !b.BlobRef.Valid() {
		return false
	}
	if b.SizeBytes < 0 {
		return false
	}
	hex := string(b.BlobRef)[len("sha256:"):]
	if b.SHA256 != "" && b.SHA256 != hex {
		return false
	}
	return true
}

// EventType classifies a journal event.
type EventType string

const (
	EventCreate     EventType = "create"
	EventUpdate     EventType = "update"
	EventDelete     EventType = "delete"
	EventMove       EventType = "move"
	EventBlobUpload EventType = "blob_upload"
)

// Valid reports whether the EventType is one of the defined constants.
func (e EventType) Valid() bool {
	switch e {
	case EventCreate, EventUpdate, EventDelete, EventMove, EventBlobUpload:
		return true
	}
	return false
}

// Event is an append-only journal entry recording a mutation. This IS a tape
// entry: it has an author (SubjectID), a code version, inputs
// (ParentEventID), outputs (PayloadJSON), and a monotonic CursorSeq. The
// ParentEventID chain makes the journal tamper-evident per item.
type Event struct {
	EventID       EventID
	OwnerID       string
	ItemID        ItemID
	DeviceID      string
	SubjectID     string // user ID or API key ID (the author)
	EventType     EventType
	Kind          ItemKind  // file or folder (ItemType in the spec)
	BlobRef       BlobRef   // content-addressed blob for blob_upload events
	ParentEventID EventID   // previous event for this item (hash chain)
	CursorSeq     int64     // monotonic sequence number
	PayloadJSON   string    // version ref, new name, new parent, etc.
	CreatedAt     time.Time
}

// Valid reports whether the Event has valid identity, item reference, type,
// and a non-negative cursor sequence.
func (e Event) Valid() bool {
	if !e.EventID.Valid() {
		return false
	}
	if !e.ItemID.Valid() {
		return false
	}
	if !e.EventType.Valid() {
		return false
	}
	if e.CursorSeq < 0 {
		return false
	}
	if e.OwnerID == "" {
		return false
	}
	return true
}

// SyncState represents the sync status of an item on a device.
type SyncState string

const (
	StateSynced      SyncState = "synced"
	StateLocalOnly   SyncState = "local_only"
	StateRemoteOnly  SyncState = "remote_only"
	StateConflict    SyncState = "conflict"
	StateStuck       SyncState = "stuck"
)

// Valid reports whether the SyncState is one of the defined constants.
func (s SyncState) Valid() bool {
	switch s {
	case StateSynced, StateLocalOnly, StateRemoteOnly, StateConflict, StateStuck:
		return true
	}
	return false
}

// SyncStatus tracks per-item, per-device sync state. It is derived downstream
// from planner output (Actions + Conflicts); the planner itself never
// produces a SyncStatus.
type SyncStatus struct {
	OwnerID         string
	DeviceID        string
	ItemID          ItemID
	LocalVersionID  VersionID
	RemoteVersionID VersionID
	SyncedVersionID VersionID
	State           SyncState
	LastError       string
	RepairHandle    string
	UpdatedAt       time.Time
}

// Valid reports whether the SyncStatus has a valid item, state, and version
// references.
func (s SyncStatus) Valid() bool {
	if !s.ItemID.Valid() {
		return false
	}
	if !s.State.Valid() {
		return false
	}
	if !s.LocalVersionID.Valid() || !s.RemoteVersionID.Valid() || !s.SyncedVersionID.Valid() {
		return false
	}
	return true
}
