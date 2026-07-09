// Package fileprovider implements the IPC bridge between the macOS File
// Provider extension (Swift/Objective-C) and the Go Base sync engine.
//
// The macOS File Provider extension runs in a separate process (an.appex)
// and cannot directly call Go code without cgo. Rather than embedding a Go
// shared library into the extension binary (which complicates signing and
// app-store submission), we expose the sync engine over a local HTTP/Unix
// socket API. The Swift extension issues JSON requests to this bridge; the
// bridge translates them into sync-engine operations and filesystem reads/
// writes against the local sync root.
//
// The bridge is intentionally thin: it does not re-implement sync logic. It
// delegates to internal/desktop.SyncEngine for planning and conflict
// management, and to internal/desktop.LocalTreeBuilder for filesystem
// enumeration. Writes received from Finder are written to the local sync
// root and then a SyncNow is triggered so the next cycle uploads them.
package fileprovider

import (
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/desktop"
)

// EntryKind classifies a filesystem entry for the File Provider enumeration
// response. It mirrors NSFileProviderItemContentType at the Go level.
type EntryKind string

const (
	KindFile     EntryKind = "file"
	KindFolder   EntryKind = "folder"
	KindConflict EntryKind = "conflict" // a projected .conflict file
)

// Entry is the JSON representation of a single item in the File Provider
// domain. The Swift extension maps this to an NSFileProviderItem.
type Entry struct {
	// Path is the POSIX-style relative path from the sync root (e.g.
	// "notes/idea.md"). The root itself is represented by the empty string.
	Path string `json:"path"`

	// Name is the basename (e.g. "idea.md").
	Name string `json:"name"`

	// Kind is "file", "folder", or "conflict".
	Kind EntryKind `json:"kind"`

	// Size is the file size in bytes (0 for folders).
	Size int64 `json:"size"`

	// ModifiedAt is the filesystem mtime (UTC).
	ModifiedAt time.Time `json:"modified_at"`

	// SyncState is the sync engine's per-item state, if known.
	SyncState model.SyncState `json:"sync_state,omitempty"`

	// ConflictPath is set only when Kind == "conflict": it is the path of
	// the original file that this .conflict file shadows.
	ConflictPath string `json:"conflict_path,omitempty"`

	// ItemID is the Base model ItemID, when available from the sync engine.
	ItemID model.ItemID `json:"item_id,omitempty"`
}

// EnumerateResponse is returned by GET /enumerate. It lists the children of
// a directory within the sync root.
type EnumerateResponse struct {
	Path    string  `json:"path"`
	Entries []Entry `json:"entries"`
}

// ReadResponse is returned by GET /read. It carries the raw file bytes
// base64-encoded (so the JSON body is safe for binary content).
type ReadResponse struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ContentB64 string    `json:"content_b64"`
	MediaType  string    `json:"media_type,omitempty"`
	ModifiedAt time.Time `json:"modified_at"`
}

// WriteRequest is the body for PUT /write. The extension sends the new file
// content base64-encoded.
type WriteRequest struct {
	Path       string `json:"path"`
	ContentB64 string `json:"content_b64"`
	MediaType  string `json:"media_type,omitempty"`
}

// WriteResponse confirms a write and reports whether a sync cycle was
// triggered.
type WriteResponse struct {
	Path          string `json:"path"`
	SyncTriggered bool   `json:"sync_triggered"`
}

// CreateDirRequest creates a directory in the sync root.
type CreateDirRequest struct {
	Path string `json:"path"`
}

// MoveRequest renames or moves an item within the sync root.
type MoveRequest struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
}

// DeleteRequest deletes an item from the sync root.
type DeleteRequest struct {
	Path string `json:"path"`
}

// ConflictEntry is a projected conflict file. The bridge surfaces Base
// conflicts as virtual .conflict files alongside the original so the user
// can see both versions in Finder.
type ConflictEntry struct {
	ItemID       model.ItemID               `json:"item_id"`
	LocalItemID  model.ItemID               `json:"local_item_id,omitempty"`
	RemoteItemID model.ItemID               `json:"remote_item_id,omitempty"`
	Path         string                     `json:"path"`
	ConflictPath string                     `json:"conflict_path"` // the .conflict file path
	Reason       string                     `json:"reason"`
	LocalVer     model.Version              `json:"local_version"`
	RemoteVer    model.Version              `json:"remote_version"`
	Resolution   desktop.ConflictResolution `json:"resolution,omitempty"`
}

// ConflictsResponse lists all current conflicts.
type ConflictsResponse struct {
	Conflicts []ConflictEntry `json:"conflicts"`
}

// StatusResponse mirrors desktop.SyncProgress for the extension.
type StatusResponse struct {
	Phase          string     `json:"phase"`
	LastSyncAt     *time.Time `json:"last_sync_at,omitempty"`
	Cursor         int64      `json:"cursor"`
	RemoteHead     int64      `json:"remote_head"`
	ConflictsCount int        `json:"conflicts_count"`
	LastError      string     `json:"last_error,omitempty"`
}

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Error string `json:"error"`
}
