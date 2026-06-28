package planner

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// fixedTime is a deterministic timestamp used across all planner tests so the
// planner never depends on a wall clock.
var fixedTime = time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

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

func treeOf(items []model.Item, versions []model.Version) Tree {
	t := NewTree()
	for _, it := range items {
		t.Items[it.ItemID] = it
	}
	for _, v := range versions {
		t.Versions[v.ItemID] = v
	}
	return t
}

// containsAction reports whether the action slice contains an action with the
// given type and item id.
func containsAction(actions []Action, t ActionType, id model.ItemID) bool {
	for _, a := range actions {
		if a.Type == t && a.ItemID == id {
			return true
		}
	}
	return false
}

func containsConflict(conflicts []Conflict, id model.ItemID, reasonContains string) bool {
	for _, c := range conflicts {
		if c.ItemID == id && reasonContains == "" {
			return true
		}
		if c.ItemID == id && reasonContains != "" && strContains(c.Reason, reasonContains) {
			return true
		}
	}
	return false
}

func strContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestNoChangesConverged(t *testing.T) {
	item := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	ver := mkFileVer("base_ver_1", "base_item_1", "aaa")
	tree := treeOf([]model.Item{item}, []model.Version{ver})

	actions, conflicts := Plan(tree, tree, tree)
	if len(actions) != 0 || len(conflicts) != 0 {
		t.Errorf("converged trees should produce no actions/conflicts, got %d actions %d conflicts", len(actions), len(conflicts))
	}
}

func TestLocalChangeUpload(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	localItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_2")
	localVer := mkFileVer("base_ver_2", "base_item_1", "bbb")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	actions, conflicts := Plan(synced, local, synced)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(conflicts))
	}
	if !containsAction(actions, ActionUpdateRemote, "base_item_1") {
		t.Errorf("expected ActionUpdateRemote, got %v", actions)
	}
}

func TestRemoteChangeDownload(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	remoteItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_2")
	remoteVer := mkFileVer("base_ver_2", "base_item_1", "bbb")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	actions, conflicts := Plan(remote, synced, synced)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d", len(conflicts))
	}
	if !containsAction(actions, ActionUpdateLocal, "base_item_1") {
		t.Errorf("expected ActionUpdateLocal, got %v", actions)
	}
}

func TestBothChangeConflict(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	localItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_2")
	localVer := mkFileVer("base_ver_2", "base_item_1", "bbb")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	remoteItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_3")
	remoteVer := mkFileVer("base_ver_3", "base_item_1", "ccc")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	actions, conflicts := Plan(remote, local, synced)
	if len(actions) != 0 {
		t.Errorf("expected no actions for conflict, got %d", len(actions))
	}
	if !containsConflict(conflicts, "base_item_1", "") {
		t.Errorf("expected a conflict, got %v", conflicts)
	}
	// Verify both sides preserved.
	var c Conflict
	for _, cf := range conflicts {
		if cf.ItemID == "base_item_1" {
			c = cf
		}
	}
	if c.LocalVer.VersionID != "base_ver_2" || c.RemoteVer.VersionID != "base_ver_3" {
		t.Errorf("conflict must preserve both sides, got local=%v remote=%v", c.LocalVer.VersionID, c.RemoteVer.VersionID)
	}
}

func TestLocalDeleteRemoteUnchangedDeletesRemote(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	remote := synced
	// local deleted: tombstone with DeletedAt set and no version.
	localItem := syncedItem
	localItem.DeletedAt = &fixedTime
	localItem.CurrentVersion = ""
	local := treeOf([]model.Item{localItem}, nil)

	actions, conflicts := Plan(remote, local, synced)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflict for unchanged remote delete, got %v", conflicts)
	}
	if !containsAction(actions, ActionDeleteRemote, "base_item_1") {
		t.Errorf("expected ActionDeleteRemote, got %v", actions)
	}
}

func TestLocalDeleteRemoteChangedConflict(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	remoteItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_2")
	remoteVer := mkFileVer("base_ver_2", "base_item_1", "bbb")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	localItem := syncedItem
	localItem.DeletedAt = &fixedTime
	localItem.CurrentVersion = ""
	local := treeOf([]model.Item{localItem}, nil)

	actions, conflicts := Plan(remote, local, synced)
	if len(actions) != 0 {
		t.Errorf("expected no actions for modify/delete conflict, got %v", actions)
	}
	if !containsConflict(conflicts, "base_item_1", "modify/delete") {
		t.Errorf("expected modify/delete conflict, got %v", conflicts)
	}
}

func TestNewLocalOnlyUpload(t *testing.T) {
	localItem := mkItem("base_item_1", "base_item_0", "new.txt", model.KindFile, "base_ver_1")
	localVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	empty := NewTree()

	actions, conflicts := Plan(empty, local, empty)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts for new local item, got %v", conflicts)
	}
	if !containsAction(actions, ActionUpload, "base_item_1") {
		t.Errorf("expected ActionUpload, got %v", actions)
	}
}

func TestNewRemoteOnlyDownload(t *testing.T) {
	remoteItem := mkItem("base_item_1", "base_item_0", "new.txt", model.KindFile, "base_ver_1")
	remoteVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	empty := NewTree()

	actions, conflicts := Plan(remote, empty, empty)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts for new remote item, got %v", conflicts)
	}
	if !containsAction(actions, ActionDownload, "base_item_1") {
		t.Errorf("expected ActionDownload, got %v", actions)
	}
}

func TestAddAddSameContentConverges(t *testing.T) {
	item := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	ver := mkFileVer("base_ver_1", "base_item_1", "aaa")
	local := treeOf([]model.Item{item}, []model.Version{ver})
	remote := treeOf([]model.Item{item}, []model.Version{ver})

	actions, conflicts := Plan(remote, local, NewTree())
	if len(actions) != 0 || len(conflicts) != 0 {
		t.Errorf("identical add/add should converge, got %d actions %d conflicts", len(actions), len(conflicts))
	}
}

func TestAddAddDifferentContentConflict(t *testing.T) {
	localItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	localVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	remoteItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_2")
	remoteVer := mkFileVer("base_ver_2", "base_item_1", "bbb")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	actions, conflicts := Plan(remote, local, NewTree())
	if len(actions) != 0 {
		t.Errorf("expected no actions for add/add conflict, got %v", actions)
	}
	if !containsConflict(conflicts, "base_item_1", "add/add") {
		t.Errorf("expected add/add conflict, got %v", conflicts)
	}
}

func TestLocalMoveRemoteUnchangedMovesRemote(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	localItem := mkItem("base_item_1", "base_item_9", "moved.txt", model.KindFile, "base_ver_1")
	localVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	actions, conflicts := Plan(synced, local, synced)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflict for local move, got %v", conflicts)
	}
	if !containsAction(actions, ActionMoveRemote, "base_item_1") {
		t.Errorf("expected ActionMoveRemote, got %v", actions)
	}
}

func TestRemoteMoveLocalUnchangedMovesLocal(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	remoteItem := mkItem("base_item_1", "base_item_9", "moved.txt", model.KindFile, "base_ver_1")
	remoteVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	actions, conflicts := Plan(remote, synced, synced)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflict for remote move, got %v", conflicts)
	}
	if !containsAction(actions, ActionMoveLocal, "base_item_1") {
		t.Errorf("expected ActionMoveLocal, got %v", actions)
	}
}

func TestMoveVsEditConflict(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	// local moved (same content, new location)
	localItem := mkItem("base_item_1", "base_item_9", "moved.txt", model.KindFile, "base_ver_1")
	localVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	local := treeOf([]model.Item{localItem}, []model.Version{localVer})

	// remote edited (same location, new content)
	remoteItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_2")
	remoteVer := mkFileVer("base_ver_2", "base_item_1", "bbb")
	remote := treeOf([]model.Item{remoteItem}, []model.Version{remoteVer})

	actions, conflicts := Plan(remote, local, synced)
	if len(actions) != 0 {
		t.Errorf("expected no actions for move/edit conflict, got %v", actions)
	}
	if !containsConflict(conflicts, "base_item_1", "move/edit") {
		t.Errorf("expected move/edit conflict, got %v", conflicts)
	}
	// both sides preserved
	var c Conflict
	for _, cf := range conflicts {
		if cf.ItemID == "base_item_1" {
			c = cf
		}
	}
	if c.LocalVer.VersionID != "base_ver_1" || c.RemoteVer.VersionID != "base_ver_2" {
		t.Errorf("move/edit conflict must preserve both sides, got local=%v remote=%v", c.LocalVer.VersionID, c.RemoteVer.VersionID)
	}
}

func TestBothDeleteConverges(t *testing.T) {
	syncedItem := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	syncedVer := mkFileVer("base_ver_1", "base_item_1", "aaa")
	synced := treeOf([]model.Item{syncedItem}, []model.Version{syncedVer})

	deleted := syncedItem
	deleted.DeletedAt = &fixedTime
	deleted.CurrentVersion = ""
	local := treeOf([]model.Item{deleted}, nil)
	remote := treeOf([]model.Item{deleted}, nil)

	actions, conflicts := Plan(remote, local, synced)
	if len(actions) != 0 || len(conflicts) != 0 {
		t.Errorf("mutual delete should converge, got %d actions %d conflicts", len(actions), len(conflicts))
	}
}

func TestDeterministicOrdering(t *testing.T) {
	// Two new local items; actions should be sorted by ItemID.
	i1 := mkItem("base_item_b", "base_item_0", "b.txt", model.KindFile, "base_ver_1")
	v1 := mkFileVer("base_ver_1", "base_item_b", "bbb")
	i2 := mkItem("base_item_a", "base_item_0", "a.txt", model.KindFile, "base_ver_2")
	v2 := mkFileVer("base_ver_2", "base_item_a", "aaa")
	local := treeOf([]model.Item{i1, i2}, []model.Version{v1, v2})

	actions, _ := Plan(NewTree(), local, NewTree())
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].ItemID != "base_item_a" || actions[1].ItemID != "base_item_b" {
		t.Errorf("actions not sorted by ItemID: %v then %v", actions[0].ItemID, actions[1].ItemID)
	}
}

func TestApplyEventIdempotence(t *testing.T) {
	item := mkItem("base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1")
	ver := mkFileVer("base_ver_1", "base_item_1", "aaa")
	tree := treeOf([]model.Item{item}, []model.Version{ver})

	evt := model.Event{
		EventID:   "base_evt_1",
		OwnerID:   "owner",
		ItemID:    "base_item_1",
		EventType: model.EventUpdate,
		CursorSeq: 1,
		CreatedAt: fixedTime,
	}
	applied := ApplyEvent(tree, evt)
	again := ApplyEvent(applied, evt)
	if len(again.Items) != len(applied.Items) {
		t.Errorf("duplicate event changed tree: %d vs %d items", len(applied.Items), len(again.Items))
	}
}
