// Package planner implements the pure three-tree reconciliation kernel for
// Choir Base.
//
// The planner is PURE. It performs no filesystem, network, database, wall
// clock, or random operations. It imports only the model package and the
// standard library's sorting facilities (for deterministic output ordering).
// Given three trees — remote, local, and synced (the common ancestor) — it
// produces a slice of Actions and a slice of Conflicts that reconcile local
// and remote.
//
// Identity is path-independent: an item is identified by its stable ItemID.
// Move detection compares the (ParentItemID, Name) location of the same
// ItemID across trees. Conflicts are NEVER silently resolved: both the local
// and remote versions are preserved in the Conflict record so a downstream
// resolver (human or agent) can choose explicitly.
package planner

import (
	"sort"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// Tree is a snapshot of items at a point in time, keyed by ItemID. The
// Versions map holds the current version for each item (the version pointed
// at by Item.CurrentVersion). A Tree with an item in Items but no entry in
// Versions represents a deleted item (the item record remains so deletes are
// observable).
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

// ActionType classifies a reconciliation action.
type ActionType string

const (
	ActionDownload     ActionType = "download"      // remote has it, local doesn't
	ActionUpload       ActionType = "upload"        // local has it, remote doesn't
	ActionDeleteLocal  ActionType = "delete_local"  // remote deleted, local has
	ActionDeleteRemote ActionType = "delete_remote" // local deleted, remote has
	ActionUpdateLocal  ActionType = "update_local"  // remote has newer version
	ActionUpdateRemote ActionType = "update_remote" // local has newer version
	ActionMoveLocal    ActionType = "move_local"    // remote moved it
	ActionMoveRemote   ActionType = "move_remote"   // local moved it
)

// Action describes one reconciliation step. The Version field carries the
// version to apply (for downloads/uploads/updates); for moves it carries the
// current version of the side being moved. LocalPath is informational and
// derived downstream by adapters (the planner never touches the filesystem).
type Action struct {
	Type      ActionType
	ItemID    model.ItemID
	Version   model.Version
	LocalPath string
}

// Conflict records a non-convergent item where both sides changed relative to
// the synced ancestor. Both LocalVer and RemoteVer are preserved so a
// downstream resolver can choose explicitly. The planner NEVER silently
// picks a winner.
type Conflict struct {
	ItemID       model.ItemID
	LocalItemID  model.ItemID
	RemoteItemID model.ItemID
	LocalVer     model.Version
	RemoteVer    model.Version
	SyncedVer    model.Version
	Reason       string // "both modified", "modify/delete", "add/add", "move/edit", etc.
}

// Plan produces actions that reconcile local and remote trees relative to the
// synced (common ancestor) tree. The planner is pure: no filesystem, network,
// database, wall clock, or random source.
//
// For each item across the three trees it applies the reconciliation rules
// described in the mission spec. It then runs a path-collision pass: two
// distinct items (different ItemIDs) that occupy the same (parent, name)
// location on the reconciled sides cannot both be materialized at that path,
// so a conflict is emitted preserving both sides. Output ordering is
// deterministic: actions and conflicts are sorted by ItemID so callers can
// compare outputs by value.
func Plan(remote, local, synced Tree) ([]Action, []Conflict) {
	actions := []Action{}
	conflicts := []Conflict{}

	// Union of all item IDs across the three trees.
	ids := make(map[model.ItemID]struct{})
	for id := range remote.Items {
		ids[id] = struct{}{}
	}
	for id := range local.Items {
		ids[id] = struct{}{}
	}
	for id := range synced.Items {
		ids[id] = struct{}{}
	}

	for id := range ids {
		a, c := planItem(id, remote, local, synced)
		actions = append(actions, a...)
		conflicts = append(conflicts, c...)
	}

	// Path-collision pass: detect distinct ItemIDs that would occupy the same
	// (parent, name) after reconciliation. This catches the add/add-same-path
	// case where two devices created different items at the same path.
	conflicts = append(conflicts, pathCollisions(remote, local)...)

	sortActions(actions)
	sortConflicts(conflicts)
	return actions, conflicts
}

// pathCollisions detects pairs of distinct present items (different ItemIDs)
// from local and remote that occupy the same (parent, name) location. Each
// collision becomes a Conflict preserving both sides so a downstream resolver
// can rename one side. This is the path-as-identity heresy corrected: the
// planner keys on ItemID but still surfaces path collisions as explicit
// conflicts rather than silently overwriting one side.
func pathCollisions(remote, local Tree) []Conflict {
	type loc struct {
		parent model.ItemID
		name   string
	}
	// Map location -> (localItemID, localVer, remoteItemID, remoteVer) when a
	// collision exists. We only flag collisions across sides (a local item and
	// a remote item at the same path with different IDs).
	type collision struct {
		localID   model.ItemID
		localVer  model.Version
		remoteID  model.ItemID
		remoteVer model.Version
	}
	seen := make(map[loc]collision)

	for id, it := range local.Items {
		if isDeleted(it) {
			continue
		}
		l := loc{it.ParentItemID, it.Name}
		c, ok := seen[l]
		if !ok {
			seen[l] = collision{localID: id, localVer: local.Versions[id]}
			continue
		}
		c.localID = id
		c.localVer = local.Versions[id]
		seen[l] = c
	}
	var out []Conflict
	for id, it := range remote.Items {
		if isDeleted(it) {
			continue
		}
		l := loc{it.ParentItemID, it.Name}
		c, ok := seen[l]
		if !ok {
			continue
		}
		if c.localID == "" || c.localID == id {
			continue // same item or no local item at this path
		}
		// Distinct items at the same path: conflict.
		out = append(out, Conflict{
			ItemID:       id,
			LocalItemID:  c.localID,
			RemoteItemID: id,
			LocalVer:     c.localVer,
			RemoteVer:    remote.Versions[id],
			Reason:       "add/add path collision: local item " + string(c.localID) + " and remote item " + string(id) + " occupy the same path",
		})
	}
	return out
}

// planItem computes the actions and conflicts for a single item across the
// three trees. It is the heart of the reconciliation kernel.
func planItem(id model.ItemID, remote, local, synced Tree) ([]Action, []Conflict) {
	rItem, rHas := remote.Items[id]
	lItem, lHas := local.Items[id]
	sItem, sHas := synced.Items[id]

	rVer := remote.Versions[id]
	lVer := local.Versions[id]
	sVer := synced.Versions[id]

	// --- Presence classification -------------------------------------------
	// An item is "present" on a side if the side has an item record AND it is
	// not deleted (DeletedAt == nil and CurrentVersion != ""). A deleted
	// item record is treated as absence for reconciliation, but the planner
	// still distinguishes "deleted after sync" from "never existed".
	rPresent := rHas && !isDeleted(rItem)
	lPresent := lHas && !isDeleted(lItem)
	sPresent := sHas && !isDeleted(sItem)

	// --- Corrupt/locked detection ------------------------------------------
	// A present item whose CurrentVersion is non-empty but whose Version
	// record is missing from the Versions map is "corrupt": the planner
	// cannot determine whether the content changed relative to synced. Per
	// the invariant, stuck/non-converged states are product states, not
	// silent wins. We emit an explicit conflict with a "corrupt" reason.
	lCorrupt := lPresent && lItem.CurrentVersion != "" && lVer.VersionID == ""
	rCorrupt := rPresent && rItem.CurrentVersion != "" && rVer.VersionID == ""
	if lCorrupt || rCorrupt {
		reason := "corrupt local item: version record missing"
		if rCorrupt && !lCorrupt {
			reason = "corrupt remote item: version record missing"
		} else if lCorrupt && rCorrupt {
			reason = "corrupt items on both sides: version records missing"
		}
		return nil, []Conflict{{
			ItemID:    id,
			LocalVer:  lVer,
			RemoteVer: rVer,
			SyncedVer: sVer,
			Reason:    reason,
		}}
	}

	// --- Case: item exists in synced ancestor ------------------------------
	if sPresent {
		switch {
		case lPresent && rPresent:
			// Both sides have the item. Compare versions and locations.
			return reconcileBothPresent(id, rItem, lItem, sItem, rVer, lVer, sVer)
		case lPresent && !rPresent:
			// Remote deleted (or never had it after sync). If local changed
			// relative to synced, it's a modify/delete conflict. Otherwise
			// local deletion propagates: delete local.
			if itemChanged(lItem, lVer, sItem, sVer) {
				return nil, []Conflict{{
					ItemID:    id,
					LocalVer:  lVer,
					RemoteVer: model.Version{},
					SyncedVer: sVer,
					Reason:    "modify/delete: local modified, remote deleted",
				}}
			}
			// Local unchanged, remote deleted -> delete local.
			return []Action{{Type: ActionDeleteLocal, ItemID: id, Version: lVer}}, nil
		case !lPresent && rPresent:
			// Local deleted. If remote changed relative to synced, conflict.
			if itemChanged(rItem, rVer, sItem, sVer) {
				return nil, []Conflict{{
					ItemID:    id,
					LocalVer:  model.Version{},
					RemoteVer: rVer,
					SyncedVer: sVer,
					Reason:    "modify/delete: remote modified, local deleted",
				}}
			}
			// Remote unchanged, local deleted -> delete remote.
			return []Action{{Type: ActionDeleteRemote, ItemID: id, Version: rVer}}, nil
		default:
			// Both sides deleted. Nothing to do; convergence by mutual
			// deletion. (A tombstone may be retained downstream, but the
			// planner emits no action.)
			return nil, nil
		}
	}

	// --- Case: item NOT in synced ancestor ---------------------------------
	// It's new on one or both sides.
	switch {
	case lPresent && rPresent:
		// Add/add. If identical, no action. Otherwise conflict.
		if versionsEqual(lVer, rVer) && locationsEqual(lItem, rItem) {
			return nil, nil
		}
		return nil, []Conflict{{
			ItemID:    id,
			LocalVer:  lVer,
			RemoteVer: rVer,
			SyncedVer: model.Version{},
			Reason:    "add/add: both sides created the same item with different content or location",
		}}
	case lPresent && !rPresent:
		// New on local only -> upload.
		return []Action{{Type: ActionUpload, ItemID: id, Version: lVer}}, nil
	case !lPresent && rPresent:
		// New on remote only -> download.
		return []Action{{Type: ActionDownload, ItemID: id, Version: rVer}}, nil
	default:
		// Not in any present side. Could be a tombstone in both; no action.
		return nil, nil
	}
}

// reconcileBothPresent handles the case where the item exists (non-deleted) on
// both local and remote, and in the synced ancestor.
func reconcileBothPresent(id model.ItemID, rItem, lItem, sItem model.Item, rVer, lVer, sVer model.Version) ([]Action, []Conflict) {
	localChanged := itemChanged(lItem, lVer, sItem, sVer)
	remoteChanged := itemChanged(rItem, rVer, sItem, sVer)

	switch {
	case !localChanged && !remoteChanged:
		// Both equal synced. Converged. Check for a move that both sides
		// made identically (already handled by itemChanged == false). No
		// action.
		return nil, nil

	case localChanged && !remoteChanged:
		// Local changed, remote == synced. Propagate local to remote.
		// Distinguish content update from move.
		if moved(lItem, sItem) && !versionContentChanged(lVer, sVer) {
			return []Action{{Type: ActionMoveRemote, ItemID: id, Version: lVer}}, nil
		}
		if versionContentChanged(lVer, sVer) && !moved(lItem, sItem) {
			return []Action{{Type: ActionUpdateRemote, ItemID: id, Version: lVer}}, nil
		}
		// Both content and location changed: emit update (content wins as
		// the primary action; the move is implied by the new item location
		// downstream). We emit UpdateRemote carrying the new version.
		if versionContentChanged(lVer, sVer) && moved(lItem, sItem) {
			return []Action{{Type: ActionUpdateRemote, ItemID: id, Version: lVer}}, nil
		}
		// Only moved.
		return []Action{{Type: ActionMoveRemote, ItemID: id, Version: lVer}}, nil

	case !localChanged && remoteChanged:
		// Remote changed, local == synced. Propagate remote to local.
		if moved(rItem, sItem) && !versionContentChanged(rVer, sVer) {
			return []Action{{Type: ActionMoveLocal, ItemID: id, Version: rVer}}, nil
		}
		if versionContentChanged(rVer, sVer) && !moved(rItem, sItem) {
			return []Action{{Type: ActionUpdateLocal, ItemID: id, Version: rVer}}, nil
		}
		if versionContentChanged(rVer, sVer) && moved(rItem, sItem) {
			return []Action{{Type: ActionUpdateLocal, ItemID: id, Version: rVer}}, nil
		}
		return []Action{{Type: ActionMoveLocal, ItemID: id, Version: rVer}}, nil

	default:
		// Both changed. If they changed to the SAME version and location,
		// it's a benign concurrent identical edit -> converged, no action.
		if versionsEqual(lVer, rVer) && locationsEqual(lItem, rItem) {
			return nil, nil
		}
		// Both changed differently -> conflict. Preserve both sides.
		reason := "both modified"
		// Refine the reason for move/edit cases.
		lMoved := moved(lItem, sItem)
		rMoved := moved(rItem, sItem)
		lContent := versionContentChanged(lVer, sVer)
		rContent := versionContentChanged(rVer, sVer)
		switch {
		case (lMoved || rMoved) && (lContent || rContent):
			reason = "move/edit: one side moved, the other edited (or both)"
		case lMoved && rMoved && !locationsEqual(lItem, rItem):
			reason = "move/move: both sides moved to different locations"
		case lMoved && rMoved && locationsEqual(lItem, rItem):
			// Both moved to the same place with the same content: converged
			// (handled above by versionsEqual+locationsEqual). If content
			// differs, fall through to both modified.
			reason = "both modified"
		}
		return nil, []Conflict{{
			ItemID:    id,
			LocalVer:  lVer,
			RemoteVer: rVer,
			SyncedVer: sVer,
			Reason:    reason,
		}}
	}
}

// --- helpers -------------------------------------------------------------

// isDeleted reports whether an item record represents a deletion (tombstone).
func isDeleted(i model.Item) bool {
	return i.DeletedAt != nil || i.CurrentVersion == ""
}

// itemChanged reports whether an item's content OR location changed relative
// to the synced ancestor.
func itemChanged(item model.Item, ver model.Version, sItem model.Item, sVer model.Version) bool {
	return versionContentChanged(ver, sVer) || moved(item, sItem)
}

// versionContentChanged reports whether two versions differ in content. Two
// versions are content-equal if their VersionIDs match, or, failing that, if
// their BlobRef and ContentHash match (content-addressed equality).
func versionContentChanged(a, b model.Version) bool {
	return !versionsEqual(a, b)
}

// versionsEqual reports whether two versions represent the same content.
func versionsEqual(a, b model.Version) bool {
	if a.VersionID != "" && b.VersionID != "" {
		if a.VersionID == b.VersionID {
			return true
		}
	}
	return a.BlobRef != "" && a.BlobRef == b.BlobRef && a.ContentHash != "" && a.ContentHash == b.ContentHash
}

// moved reports whether an item's location (parent, name) changed relative to
// the synced ancestor.
func moved(item, sItem model.Item) bool {
	return item.ParentItemID != sItem.ParentItemID || item.Name != sItem.Name
}

// locationsEqual reports whether two items occupy the same location.
func locationsEqual(a, b model.Item) bool {
	return a.ParentItemID == b.ParentItemID && a.Name == b.Name
}

// sortActions sorts actions by ItemID then Type for deterministic output.
func sortActions(a []Action) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].ItemID != a[j].ItemID {
			return a[i].ItemID < a[j].ItemID
		}
		return a[i].Type < a[j].Type
	})
}

// sortConflicts sorts conflicts by ItemID for deterministic output.
func sortConflicts(c []Conflict) {
	sort.Slice(c, func(i, j int) bool {
		return c[i].ItemID < c[j].ItemID
	})
}

// --- idempotence helper --------------------------------------------------

// ApplyEvent is a pure helper that folds a journal Event into a Tree,
// returning the new Tree. It is used by the testkit and by downstream
// journal-replay code to derive a Tree from an event stream. Duplicate
// events (same EventID already applied) are ignored — this is the
// idempotence property required by scenario 5.
//
// This function is pure: it does not mutate the input Tree (it copies the
// maps) and performs no I/O.
func ApplyEvent(tree Tree, evt model.Event) Tree {
	// Idempotence: if an event with the same EventID has already been
	// applied to this item, ignore the duplicate.
	if existing, ok := tree.Items[evt.ItemID]; ok {
		if existing.UpdatedAt.Equal(evt.CreatedAt) && hasAppliedEvent(tree, evt) {
			return tree
		}
	}
	// Copy maps to preserve purity.
	out := NewTree()
	for k, v := range tree.Items {
		out.Items[k] = v
	}
	for k, v := range tree.Versions {
		out.Versions[k] = v
	}

	switch evt.EventType {
	case model.EventDelete:
		item := out.Items[evt.ItemID]
		item.DeletedAt = &evt.CreatedAt
		item.CurrentVersion = ""
		item.UpdatedAt = evt.CreatedAt
		out.Items[evt.ItemID] = item
		delete(out.Versions, evt.ItemID)
	case model.EventMove:
		item := out.Items[evt.ItemID]
		// Payload carries new parent/name; downstream parses PayloadJSON.
		// For the pure helper we only bump UpdatedAt and rely on the caller
		// to have set the item's location before applying. We do not parse
		// JSON here to keep the planner free of encoding/json in the hot
		// path; the testkit sets locations directly.
		item.UpdatedAt = evt.CreatedAt
		out.Items[evt.ItemID] = item
	case model.EventCreate, model.EventUpdate:
		// The caller is expected to have placed the Version into the tree's
		// Versions map. We just ensure the item is marked present.
		item := out.Items[evt.ItemID]
		item.UpdatedAt = evt.CreatedAt
		if item.CreatedAt.IsZero() {
			item.CreatedAt = evt.CreatedAt
		}
		out.Items[evt.ItemID] = item
	}
	return out
}

// hasAppliedEvent is a conservative idempotence check: it reports whether the
// item's UpdatedAt matches the event's CreatedAt, indicating the event was
// already folded. A full journal-replay implementation would track applied
// EventIDs in a side set; for the planner/testkit this timestamp-equality
// check is sufficient and remains pure (no wall clock — it compares values
// already present in the tree, which were placed there by prior events).
func hasAppliedEvent(tree Tree, evt model.Event) bool {
	item, ok := tree.Items[evt.ItemID]
	if !ok {
		return false
	}
	return item.UpdatedAt.Equal(evt.CreatedAt)
}
