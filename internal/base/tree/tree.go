// Package tree implements pure tree derivation from a journal event
// stream.
//
// The derivation is a PURE function: given a slice of model.Event values
// (in CursorSeq order), it produces a Tree snapshot of all items at that
// point in time. It performs no filesystem, network, database, or wall-
// clock operations. It imports only the model package, encoding/json (for
// parsing event payloads), and sort (for ordering events by CursorSeq).
//
// Semantics:
//   - create    → insert a new item with its first version.
//   - update    → replace the item's current version (latest wins).
//   - delete    → tombstone the item (set DeletedAt, clear CurrentVersion).
//   - move      → change the item's ParentItemID and Name.
//   - blob_upload → replace the item's version with a new blob-backed version.
//
// A deleted item's record is retained (tombstone) so deletes are
// observable downstream. A create after a delete reactivates the item.
package tree

import (
	"encoding/json"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// Tree is a snapshot of items at a point in time, keyed by ItemID. The
// Versions map holds the current version for each live item. An item in
// Items with no entry in Versions represents a tombstone (deleted item).
type Tree struct {
	Items    map[model.ItemID]model.Item
	Versions map[model.ItemID]model.Version
}

// NewTree returns an empty Tree ready for use.
func NewTree() Tree {
	return Tree{
		Items:    make(map[model.ItemID]model.Item),
		Versions: make(map[model.ItemID]model.Version),
	}
}

// Payload is the structured content of an event's PayloadJSON. It carries
// the item details needed to reconstruct the item and its version during
// tree derivation.
type Payload struct {
	Name         string          `json:"name,omitempty"`
	ParentItemID model.ItemID    `json:"parent_item_id,omitempty"`
	Kind         model.ItemKind  `json:"kind,omitempty"`
	VersionID    model.VersionID `json:"version_id,omitempty"`
	BlobRef      model.BlobRef   `json:"blob_ref,omitempty"`
	MediaType    string          `json:"media_type,omitempty"`
	ContentHash  string          `json:"content_hash,omitempty"`
}

// MarshalJSON encodes the payload to a JSON string suitable for storing
// in model.Event.PayloadJSON.
func (p Payload) JSON() string {
	b, err := json.Marshal(p)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ParsePayload decodes a JSON payload string. An empty or invalid payload
// yields a zero Payload — derivation treats missing fields as "no change".
func ParsePayload(s string) Payload {
	if s == "" {
		return Payload{}
	}
	var p Payload
	_ = json.Unmarshal([]byte(s), &p)
	return p
}

// Derive rebuilds a complete tree from a slice of journal events. Events
// are sorted by CursorSeq before processing, so the caller may pass them
// in any order. The function is pure: it does not mutate the input slice
// and performs no I/O.
func Derive(events []model.Event) Tree {
	return DeriveUpTo(events, maxCursor(events))
}

// DeriveUpTo rebuilds a tree from events with CursorSeq <= cursor. This
// produces a snapshot at a point in time (an intermediate cursor
// position). Events are sorted by CursorSeq before processing.
func DeriveUpTo(events []model.Event, cursor int64) Tree {
	tree := NewTree()

	// Copy and sort to avoid mutating the caller's slice.
	ordered := make([]model.Event, 0, len(events))
	for _, e := range events {
		if e.CursorSeq <= cursor {
			ordered = append(ordered, e)
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CursorSeq < ordered[j].CursorSeq })

	for _, evt := range ordered {
		applyEvent(tree, evt)
	}
	return tree
}

// applyEvent folds a single event into the tree. It mutates the tree
// in place; the caller (Derive/DeriveUpTo) has already created a fresh
// tree so purity is preserved at the API boundary.
func applyEvent(tree Tree, evt model.Event) {
	payload := ParsePayload(evt.PayloadJSON)

	switch evt.EventType {
	case model.EventCreate:
		item := model.Item{
			ItemID:         evt.ItemID,
			OwnerID:        evt.OwnerID,
			ParentItemID:   payload.ParentItemID,
			Name:           payload.Name,
			Kind:           payload.Kind,
			CurrentVersion: payload.VersionID,
			CreatedAt:      evt.CreatedAt,
			UpdatedAt:      evt.CreatedAt,
		}
		// If the item already exists (re-create after delete), preserve
		// the original CreatedAt.
		if existing, ok := tree.Items[evt.ItemID]; ok && !existing.CreatedAt.IsZero() {
			item.CreatedAt = existing.CreatedAt
		}
		tree.Items[evt.ItemID] = item
		if payload.VersionID != "" {
			tree.Versions[evt.ItemID] = versionFromPayload(evt, payload)
		}

	case model.EventUpdate:
		item, ok := tree.Items[evt.ItemID]
		if !ok {
			// An update for an unknown item is treated as a create
			// (defensive: the journal should have a prior create, but
			// derivation must not panic on partial logs).
			item = model.Item{
				ItemID:    evt.ItemID,
				OwnerID:   evt.OwnerID,
				Kind:      payload.Kind,
				CreatedAt: evt.CreatedAt,
			}
		}
		if payload.Name != "" {
			item.Name = payload.Name
		}
		if payload.ParentItemID != "" {
			item.ParentItemID = payload.ParentItemID
		}
		if payload.Kind != "" {
			item.Kind = payload.Kind
		}
		item.CurrentVersion = payload.VersionID
		item.DeletedAt = nil // an update revives a tombstoned item
		item.UpdatedAt = evt.CreatedAt
		tree.Items[evt.ItemID] = item
		if payload.VersionID != "" {
			tree.Versions[evt.ItemID] = versionFromPayload(evt, payload)
		}

	case model.EventBlobUpload:
		item, ok := tree.Items[evt.ItemID]
		if !ok {
			item = model.Item{
				ItemID:    evt.ItemID,
				OwnerID:   evt.OwnerID,
				Kind:      model.KindFile,
				CreatedAt: evt.CreatedAt,
			}
		}
		// A blob upload creates a new version backed by the blob.
		item.CurrentVersion = payload.VersionID
		item.DeletedAt = nil
		item.UpdatedAt = evt.CreatedAt
		tree.Items[evt.ItemID] = item
		if payload.VersionID != "" {
			tree.Versions[evt.ItemID] = versionFromPayload(evt, payload)
		}

	case model.EventDelete:
		item, ok := tree.Items[evt.ItemID]
		if !ok {
			// A delete for an unknown item creates a tombstone so the
			// deletion is observable downstream.
			item = model.Item{
				ItemID:    evt.ItemID,
				OwnerID:   evt.OwnerID,
				Kind:      payload.Kind,
				CreatedAt: evt.CreatedAt,
			}
		}
		t := evt.CreatedAt
		item.DeletedAt = &t
		item.CurrentVersion = ""
		item.UpdatedAt = evt.CreatedAt
		tree.Items[evt.ItemID] = item
		delete(tree.Versions, evt.ItemID)

	case model.EventMove:
		item, ok := tree.Items[evt.ItemID]
		if !ok {
			// A move for an unknown item is a no-op (cannot move
			// something that does not exist in this snapshot).
			return
		}
		if payload.ParentItemID != "" {
			item.ParentItemID = payload.ParentItemID
		}
		if payload.Name != "" {
			item.Name = payload.Name
		}
		item.UpdatedAt = evt.CreatedAt
		tree.Items[evt.ItemID] = item
	}
}

// versionFromPayload builds a model.Version from an event and its decoded
// payload. The version carries the blob ref, content hash, and provenance
// from the event.
func versionFromPayload(evt model.Event, p Payload) model.Version {
	return model.Version{
		VersionID:        p.VersionID,
		ItemID:           evt.ItemID,
		BlobRef:          p.BlobRef,
		MediaType:        p.MediaType,
		ContentHash:      p.ContentHash,
		CreatedByDevice:  evt.DeviceID,
		CreatedBySubject: evt.SubjectID,
		CreatedAt:        evt.CreatedAt,
	}
}

// maxCursor returns the highest CursorSeq in the event slice, or 0 if
// empty. This is used by Derive to include all events.
func maxCursor(events []model.Event) int64 {
	var max int64
	for _, e := range events {
		if e.CursorSeq > max {
			max = e.CursorSeq
		}
	}
	return max
}

// LiveItems returns the items in the tree that are not deleted (no
// tombstone). This is a convenience for downstream consumers.
func (t Tree) LiveItems() []model.Item {
	var out []model.Item
	for _, it := range t.Items {
		if it.DeletedAt == nil && it.CurrentVersion != "" {
			out = append(out, it)
		}
	}
	return out
}

// IsDeleted reports whether an item is tombstoned in the tree.
func (t Tree) IsDeleted(id model.ItemID) bool {
	it, ok := t.Items[id]
	if !ok {
		return false
	}
	return it.DeletedAt != nil || it.CurrentVersion == ""
}

// Has reports whether an item exists in the tree (live or tombstoned).
func (t Tree) Has(id model.ItemID) bool {
	_, ok := t.Items[id]
	return ok
}
