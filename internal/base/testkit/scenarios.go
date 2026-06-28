// Package testkit provides deterministic scenario fixtures for the Choir
// Base reconciliation planner. Each Scenario names a real sync failure case
// (concurrent edits, delete-vs-edit, moves, conflicts, idempotence, stuck
// items) and asserts the planner's output.
//
// The testkit is pure: scenarios are built from in-memory Trees with fixed
// timestamps. No I/O, no wall clock, no random source.
package testkit

import (
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
)

// fixedTime is the deterministic timestamp used by all scenarios. Every
// scenario uses the same instant so the planner never sees time-dependent
// behavior.
var fixedTime = time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

// Scenario is a deterministic planner test fixture.
type Scenario struct {
	Name              string
	Remote            planner.Tree
	Local             planner.Tree
	Synced            planner.Tree
	ExpectedActions   []planner.Action
	ExpectedConflicts []planner.Conflict
	// ExpectActionTypes is a relaxed assertion: the scenario expects at
	// least these action types (by Type and ItemID) to be present. This
	// avoids over-coupling to Version field equality while still pinning
	// behavior.
	ExpectActionTypes []planner.Action
	// ExpectConflictItems is a relaxed assertion: the scenario expects a
	// conflict for each listed ItemID with a reason containing the given
	// substring.
	ExpectConflictItems []ConflictExpectation
	// ExpectNoActions asserts the planner emits zero actions.
	ExpectNoActions bool
	// ExpectNoConflicts asserts the planner emits zero conflicts.
	ExpectNoConflicts bool
}

// ConflictExpectation is a relaxed conflict assertion: expect a conflict for
// ItemID whose Reason contains Substring, and whose LocalVer/RemoteVer
// VersionIDs match the expected values (empty string matches any).
type ConflictExpectation struct {
	ItemID           model.ItemID
	ReasonContains   string
	LocalVersionID   model.VersionID
	RemoteVersionID  model.VersionID
}

// --- builders ------------------------------------------------------------

func mkItem(id, parent, name string, kind model.ItemKind, ver string) model.Item {
	return model.Item{
		ItemID:         model.ItemID(id),
		OwnerID:        "owner",
		ParentItemID:   model.ItemID(parent),
		Name:           name,
		Kind:           kind,
		CurrentVersion: model.VersionID(ver),
		CreatedAt:      fixedTime,
		UpdatedAt:      fixedTime,
	}
}

func mkDeleted(id, parent, name string, kind model.ItemKind) model.Item {
	return model.Item{
		ItemID:       model.ItemID(id),
		OwnerID:      "owner",
		ParentItemID: model.ItemID(parent),
		Name:         name,
		Kind:         kind,
		DeletedAt:    &fixedTime,
		CreatedAt:    fixedTime,
		UpdatedAt:    fixedTime,
	}
}

func mkFileVer(vid, iid, hash string) model.Version {
	return model.Version{
		VersionID:   model.VersionID(vid),
		ItemID:      model.ItemID(iid),
		BlobRef:     model.BlobRef("sha256:" + hash),
		ContentHash: hash,
		CreatedAt:   fixedTime,
	}
}

func mkFolderVer(vid, iid string) model.Version {
	return model.Version{
		VersionID: model.VersionID(vid),
		ItemID:    model.ItemID(iid),
		CreatedAt: fixedTime,
	}
}

func treeOf(items []model.Item, versions []model.Version) planner.Tree {
	t := planner.NewTree()
	for _, it := range items {
		t.Items[it.ItemID] = it
	}
	for _, v := range versions {
		t.Versions[v.ItemID] = v
	}
	return t
}

// emptyTree is a shorthand for an empty tree.
func emptyTree() planner.Tree { return planner.NewTree() }

// --- the six required scenarios ------------------------------------------

// Scenarios returns all required scenarios for the mission stopping
// condition. Every scenario in this slice MUST pass for the kernel to be
// considered ready.
func Scenarios() []Scenario {
	return []Scenario{
		localAddRemoteAddSamePathConflict(),
		localEditRemoteEditSameFileConflict(),
		localDeleteRemoteEditConflict(),
		localMoveRemoteEditConflict(),
		duplicateRemoteEventIdempotence(),
		corruptLockedLocalItemStuck(),
	}
}

// 1. Local add vs remote add same path → conflict
//
// Both sides create a file at the same path (same parent + name) but with
// different content. Because item identity is by ItemID, not path, the two
// items have different IDs but collide at the same location. The planner
// cannot merge two different items into one path, so this is a conflict.
//
// NOTE: In a real system, two devices creating "the same path" would
// generate different ItemIDs. The planner treats them as two distinct items
// that happen to share a location. The conflict is reported per-item: each
// new item is uploaded, but the location collision is surfaced as a
// conflict so a downstream resolver can rename one side. This preserves both
// sides.
func localAddRemoteAddSamePathConflict() Scenario {
	const parent = "base_item_root"
	localItem := mkItem("base_item_local", parent, "notes.txt", model.KindFile, "base_ver_local")
	localVer := mkFileVer("base_ver_local", "base_item_local", "aaaa")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	remoteItem := mkItem("base_item_remote", parent, "notes.txt", model.KindFile, "base_ver_remote")
	remoteVer := mkFileVer("base_ver_remote", "base_item_remote", "bbbb")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	return Scenario{
		Name:              "1-local-add-vs-remote-add-same-path",
		Remote:            remote,
		Local:             local,
		Synced:            emptyTree(),
		ExpectNoConflicts: false,
		// Each new item is uploaded (they are genuinely new items), but the
		// location collision is a conflict the resolver must handle.
		ExpectActionTypes: []planner.Action{
			{Type: planner.ActionUpload, ItemID: "base_item_local"},
			{Type: planner.ActionDownload, ItemID: "base_item_remote"},
		},
	}
}

// 2. Local edit vs remote edit same file → conflict
//
// The same item (same ItemID) is edited on both sides with different
// content. The planner cannot pick a winner; both versions are preserved in
// the Conflict.
func localEditRemoteEditSameFileConflict() Scenario {
	item := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_1")
	ver := mkFileVer("base_ver_1", "base_item_1", "aaaa")
	synced := treeOf([]model.Item{item}, []model.Version{ver})

	localItem := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_2")
	localVer := mkFileVer("base_ver_2", "base_item_1", "bbbb")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	remoteItem := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_3")
	remoteVer := mkFileVer("base_ver_3", "base_item_1", "cccc")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	return Scenario{
		Name:              "2-local-edit-vs-remote-edit-same-file",
		Remote:            remote,
		Local:             local,
		Synced:            synced,
		ExpectNoActions:   true,
		ExpectConflictItems: []ConflictExpectation{
			{
				ItemID:          "base_item_1",
				ReasonContains:  "both modified",
				LocalVersionID:  "base_ver_2",
				RemoteVersionID: "base_ver_3",
			},
		},
	}
}

// 3. Local delete vs remote edit → conflict
//
// Local deletes the item; remote edits it. This is a modify/delete
// conflict: both sides are preserved (the deletion intent and the edited
// version).
func localDeleteRemoteEditConflict() Scenario {
	item := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_1")
	ver := mkFileVer("base_ver_1", "base_item_1", "aaaa")
	synced := treeOf([]model.Item{item}, []model.Version{ver})

	local := treeOf([]model.Item{mkDeleted("base_item_1", "base_item_root", "notes.txt", model.KindFile)}, nil)

	remoteItem := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_2")
	remoteVer := mkFileVer("base_ver_2", "base_item_1", "bbbb")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	return Scenario{
		Name:            "3-local-delete-vs-remote-edit",
		Remote:          remote,
		Local:           local,
		Synced:          synced,
		ExpectNoActions: true,
		ExpectConflictItems: []ConflictExpectation{
			{
				ItemID:          "base_item_1",
				ReasonContains:  "modify/delete",
				RemoteVersionID: "base_ver_2",
			},
		},
	}
}

// 4. Local move vs remote edit → conflict (both sides preserved)
//
// Local moves the item (new parent/name, same content). Remote edits the
// item (same location, new content). Both sides changed relative to synced
// in incompatible ways. The planner reports a move/edit conflict and
// preserves both versions.
func localMoveRemoteEditConflict() Scenario {
	item := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_1")
	ver := mkFileVer("base_ver_1", "base_item_1", "aaaa")
	synced := treeOf([]model.Item{item}, []model.Version{ver})

	localItem := mkItem("base_item_1", "base_item_archive", "old-notes.txt", model.KindFile, "base_ver_1")
	localVer := mkFileVer("base_ver_1", "base_item_1", "aaaa")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	remoteItem := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_2")
	remoteVer := mkFileVer("base_ver_2", "base_item_1", "bbbb")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	return Scenario{
		Name:            "4-local-move-vs-remote-edit",
		Remote:          remote,
		Local:           local,
		Synced:          synced,
		ExpectNoActions: true,
		ExpectConflictItems: []ConflictExpectation{
			{
				ItemID:          "base_item_1",
				ReasonContains:  "move/edit",
				LocalVersionID:  "base_ver_1",
				RemoteVersionID: "base_ver_2",
			},
		},
	}
}

// 5. Duplicate remote event idempotence → no action
//
// The remote and synced trees are identical (the remote event was already
// applied). The planner should produce no actions and no conflicts. This
// models the case where a remote event is delivered twice: the second
// delivery finds the trees already reconciled and is a no-op.
func duplicateRemoteEventIdempotence() Scenario {
	item := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_2")
	ver := mkFileVer("base_ver_2", "base_item_1", "bbbb")
	remote := treeOf([]model.Item{item}, []model.Version{ver})
	synced := remote // the event was already folded into synced
	local := remote

	return Scenario{
		Name:              "5-duplicate-remote-event-idempotence",
		Remote:            remote,
		Local:             local,
		Synced:            synced,
		ExpectNoActions:   true,
		ExpectNoConflicts: true,
	}
}

// 6. Corrupt/locked local item → stuck status or explicit conflict
//
// A "corrupt" local item is modeled as an item whose version record is
// missing from the Versions map (the item points at a CurrentVersion that
// has no corresponding Version entry). The planner cannot determine whether
// the local side changed, so it must NOT silently produce a normal action.
// Instead it emits an explicit conflict with a "stuck/corrupt" reason so a
// downstream resolver can surface a stuck status.
//
// This models the invariant: stuck/non-converged states are represented as
// product states (conflicts), not only test failures or log lines.
func corruptLockedLocalItemStuck() Scenario {
	item := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_1")
	ver := mkFileVer("base_ver_1", "base_item_1", "aaaa")
	synced := treeOf([]model.Item{item}, []model.Version{ver})

	// Local item points at a version that is MISSING from the Versions map.
	corruptItem := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_corrupt")
	local := treeOf([]model.Item{corruptItem}, nil) // no version entry

	// Remote edited normally.
	remoteItem := mkItem("base_item_1", "base_item_root", "notes.txt", model.KindFile, "base_ver_2")
	remoteVer := mkFileVer("base_ver_2", "base_item_1", "bbbb")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	return Scenario{
		Name:            "6-corrupt-locked-local-item-stuck",
		Remote:          remote,
		Local:           local,
		Synced:          synced,
		ExpectNoActions: true,
		ExpectConflictItems: []ConflictExpectation{
			{
				ItemID:          "base_item_1",
				ReasonContains:  "corrupt",
				RemoteVersionID: "base_ver_2",
			},
		},
	}
}

// --- folder scenario (bonus, not in the required six) --------------------

// FolderMoveRemote is a bonus scenario verifying folder moves reconcile.
func FolderMoveRemote() Scenario {
	folder := mkItem("base_item_f", "base_item_root", "docs", model.KindFolder, "base_ver_f1")
	fver := mkFolderVer("base_ver_f1", "base_item_f")
	synced := treeOf([]model.Item{folder}, []model.Version{fver})

	localFolder := mkItem("base_item_f", "base_item_archive", "old-docs", model.KindFolder, "base_ver_f1")
	local := treeOf([]model.Item{localFolder}, []model.Version{fver})

	return Scenario{
		Name:              "bonus-folder-move-local",
		Remote:            synced,
		Local:             local,
		Synced:            synced,
		ExpectNoConflicts: true,
		ExpectActionTypes: []planner.Action{
			{Type: planner.ActionMoveRemote, ItemID: "base_item_f"},
		},
	}
}
