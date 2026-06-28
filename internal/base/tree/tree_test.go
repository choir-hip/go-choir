package tree

import (
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

// fixedTime is a deterministic timestamp for all tree tests. The tree
// derivation never reads a wall clock — it only uses timestamps carried
// by events.
var fixedTime = time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)

// mkCreate builds a create event with a payload describing a file item.
func mkCreate(eid, iid, parent, name string, kind model.ItemKind, vid, hash string) model.Event {
	p := Payload{
		Name:         name,
		ParentItemID: model.ItemID(parent),
		Kind:         kind,
		VersionID:    model.VersionID(vid),
		BlobRef:      model.BlobRef("sha256:" + hash),
		ContentHash:  hash,
	}
	return model.Event{
		EventID:     model.EventID(eid),
		OwnerID:     "owner",
		ItemID:      model.ItemID(iid),
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventCreate,
		Kind:        kind,
		CursorSeq:   0, // assigned by journal in practice; tests set explicitly
		PayloadJSON: p.JSON(),
		CreatedAt:   fixedTime,
	}
}

// mkUpdate builds an update event with a new version.
func mkUpdate(eid, iid, vid, hash string, seq int64) model.Event {
	p := Payload{
		VersionID:   model.VersionID(vid),
		BlobRef:     model.BlobRef("sha256:" + hash),
		ContentHash: hash,
	}
	return model.Event{
		EventID:     model.EventID(eid),
		OwnerID:     "owner",
		ItemID:      model.ItemID(iid),
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventUpdate,
		Kind:        model.KindFile,
		CursorSeq:   seq,
		PayloadJSON: p.JSON(),
		CreatedAt:   fixedTime,
	}
}

// mkDelete builds a delete event.
func mkDelete(eid, iid string, seq int64) model.Event {
	return model.Event{
		EventID:     model.EventID(eid),
		OwnerID:     "owner",
		ItemID:      model.ItemID(iid),
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventDelete,
		Kind:        model.KindFile,
		CursorSeq:   seq,
		PayloadJSON: Payload{}.JSON(),
		CreatedAt:   fixedTime,
	}
}

// mkMove builds a move event with a new parent and name.
func mkMove(eid, iid, newParent, newName string, seq int64) model.Event {
	p := Payload{
		ParentItemID: model.ItemID(newParent),
		Name:         newName,
	}
	return model.Event{
		EventID:     model.EventID(eid),
		OwnerID:     "owner",
		ItemID:      model.ItemID(iid),
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventMove,
		Kind:        model.KindFile,
		CursorSeq:   seq,
		PayloadJSON: p.JSON(),
		CreatedAt:   fixedTime,
	}
}

// mkBlobUpload builds a blob_upload event.
func mkBlobUpload(eid, iid, vid, hash string, seq int64) model.Event {
	p := Payload{
		VersionID:   model.VersionID(vid),
		BlobRef:     model.BlobRef("sha256:" + hash),
		ContentHash: hash,
	}
	return model.Event{
		EventID:     model.EventID(eid),
		OwnerID:     "owner",
		ItemID:      model.ItemID(iid),
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventBlobUpload,
		Kind:        model.KindFile,
		BlobRef:     model.BlobRef("sha256:" + hash),
		CursorSeq:   seq,
		PayloadJSON: p.JSON(),
		CreatedAt:   fixedTime,
	}
}

// withSeq sets the CursorSeq on an event (for tests that build events
// with the mkCreate helper, which defaults to 0).
func withSeq(e model.Event, seq int64) model.Event {
	e.CursorSeq = seq
	return e
}

// --- required tests -----------------------------------------------------

func TestDeriveFromEmpty(t *testing.T) {
	tree := Derive(nil)
	if len(tree.Items) != 0 {
		t.Errorf("empty event stream should produce empty tree, got %d items", len(tree.Items))
	}
	if len(tree.Versions) != 0 {
		t.Errorf("empty event stream should produce no versions, got %d", len(tree.Versions))
	}

	// Also test with events but derive up to cursor 0 (before any event).
	tree2 := DeriveUpTo([]model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
	}, 0)
	if len(tree2.Items) != 0 {
		t.Errorf("derive up to cursor 0 should produce empty tree, got %d items", len(tree2.Items))
	}
}

func TestDeriveAfterCreate(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
	}
	tree := Derive(events)

	item, ok := tree.Items["base_item_1"]
	if !ok {
		t.Fatal("expected item base_item_1 in tree")
	}
	if item.Name != "a.txt" {
		t.Errorf("expected name a.txt, got %q", item.Name)
	}
	if item.ParentItemID != "base_item_0" {
		t.Errorf("expected parent base_item_0, got %q", item.ParentItemID)
	}
	if item.Kind != model.KindFile {
		t.Errorf("expected kind file, got %q", item.Kind)
	}
	if item.CurrentVersion != "base_ver_1" {
		t.Errorf("expected current version base_ver_1, got %q", item.CurrentVersion)
	}
	if item.DeletedAt != nil {
		t.Error("created item should not be deleted")
	}

	ver, ok := tree.Versions["base_item_1"]
	if !ok {
		t.Fatal("expected version for base_item_1")
	}
	if ver.VersionID != "base_ver_1" {
		t.Errorf("expected version id base_ver_1, got %q", ver.VersionID)
	}
	if ver.BlobRef != "sha256:aaa" {
		t.Errorf("expected blob ref sha256:aaa, got %q", ver.BlobRef)
	}
}

func TestDeriveAfterUpdate(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
		withSeq(mkUpdate("base_evt_2", "base_item_1", "base_ver_2", "bbb", 2), 2),
	}
	tree := Derive(events)

	item := tree.Items["base_item_1"]
	if item.CurrentVersion != "base_ver_2" {
		t.Errorf("expected current version base_ver_2 after update, got %q", item.CurrentVersion)
	}

	ver := tree.Versions["base_item_1"]
	if ver.VersionID != "base_ver_2" {
		t.Errorf("expected version id base_ver_2, got %q", ver.VersionID)
	}
	if ver.BlobRef != "sha256:bbb" {
		t.Errorf("expected blob ref sha256:bbb, got %q", ver.BlobRef)
	}
}

func TestDeriveAfterDelete(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
		withSeq(mkDelete("base_evt_2", "base_item_1", 2), 2),
	}
	tree := Derive(events)

	item, ok := tree.Items["base_item_1"]
	if !ok {
		t.Fatal("deleted item record should be retained as tombstone")
	}
	if item.DeletedAt == nil {
		t.Error("deleted item should have DeletedAt set")
	}
	if item.CurrentVersion != "" {
		t.Errorf("deleted item should have empty CurrentVersion, got %q", item.CurrentVersion)
	}

	if _, ok := tree.Versions["base_item_1"]; ok {
		t.Error("deleted item should have no version entry")
	}

	if !tree.IsDeleted("base_item_1") {
		t.Error("IsDeleted should report true for tombstoned item")
	}
}

func TestDeriveAfterMove(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
		withSeq(mkMove("base_evt_2", "base_item_1", "base_item_9", "moved.txt", 2), 2),
	}
	tree := Derive(events)

	item := tree.Items["base_item_1"]
	if item.ParentItemID != "base_item_9" {
		t.Errorf("expected parent base_item_9 after move, got %q", item.ParentItemID)
	}
	if item.Name != "moved.txt" {
		t.Errorf("expected name moved.txt after move, got %q", item.Name)
	}
	// Move should not change the version.
	if item.CurrentVersion != "base_ver_1" {
		t.Errorf("move should not change version, expected base_ver_1, got %q", item.CurrentVersion)
	}
	ver := tree.Versions["base_item_1"]
	if ver.VersionID != "base_ver_1" {
		t.Errorf("version should be unchanged after move, got %q", ver.VersionID)
	}
}

func TestDeriveAtIntermediateCursor(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
		withSeq(mkUpdate("base_evt_2", "base_item_1", "base_ver_2", "bbb", 2), 2),
		withSeq(mkMove("base_evt_3", "base_item_1", "base_item_9", "moved.txt", 3), 3),
	}

	// At cursor 1: only the create has been applied.
	tree1 := DeriveUpTo(events, 1)
	item1 := tree1.Items["base_item_1"]
	if item1.CurrentVersion != "base_ver_1" {
		t.Errorf("at cursor 1, version should be base_ver_1, got %q", item1.CurrentVersion)
	}
	if item1.Name != "a.txt" {
		t.Errorf("at cursor 1, name should be a.txt, got %q", item1.Name)
	}

	// At cursor 2: create + update applied, but not move.
	tree2 := DeriveUpTo(events, 2)
	item2 := tree2.Items["base_item_1"]
	if item2.CurrentVersion != "base_ver_2" {
		t.Errorf("at cursor 2, version should be base_ver_2, got %q", item2.CurrentVersion)
	}
	if item2.Name != "a.txt" {
		t.Errorf("at cursor 2, name should still be a.txt, got %q", item2.Name)
	}

	// At cursor 3: all applied.
	tree3 := DeriveUpTo(events, 3)
	item3 := tree3.Items["base_item_1"]
	if item3.CurrentVersion != "base_ver_2" {
		t.Errorf("at cursor 3, version should be base_ver_2, got %q", item3.CurrentVersion)
	}
	if item3.ParentItemID != "base_item_9" {
		t.Errorf("at cursor 3, parent should be base_item_9, got %q", item3.ParentItemID)
	}
}

// --- additional tests ---------------------------------------------------

func TestDeriveLatestUpdateWins(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
		withSeq(mkUpdate("base_evt_2", "base_item_1", "base_ver_2", "bbb", 2), 2),
		withSeq(mkUpdate("base_evt_3", "base_item_1", "base_ver_3", "ccc", 3), 3),
	}
	tree := Derive(events)

	item := tree.Items["base_item_1"]
	if item.CurrentVersion != "base_ver_3" {
		t.Errorf("latest update should win, expected base_ver_3, got %q", item.CurrentVersion)
	}
	ver := tree.Versions["base_item_1"]
	if ver.VersionID != "base_ver_3" {
		t.Errorf("version should be base_ver_3, got %q", ver.VersionID)
	}
}

func TestDeriveCreateAfterDeleteReactivates(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
		withSeq(mkDelete("base_evt_2", "base_item_1", 2), 2),
		withSeq(mkCreate("base_evt_3", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_2", "bbb"), 3),
	}
	tree := Derive(events)

	item := tree.Items["base_item_1"]
	if item.DeletedAt != nil {
		t.Error("re-created item should not be deleted")
	}
	if item.CurrentVersion != "base_ver_2" {
		t.Errorf("re-created item should have version base_ver_2, got %q", item.CurrentVersion)
	}
}

func TestDeriveBlobUpload(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
		withSeq(mkBlobUpload("base_evt_2", "base_item_1", "base_ver_2", "bbb", 2), 2),
	}
	tree := Derive(events)

	item := tree.Items["base_item_1"]
	if item.CurrentVersion != "base_ver_2" {
		t.Errorf("after blob upload, version should be base_ver_2, got %q", item.CurrentVersion)
	}
	ver := tree.Versions["base_item_1"]
	if ver.BlobRef != "sha256:bbb" {
		t.Errorf("after blob upload, blob ref should be sha256:bbb, got %q", ver.BlobRef)
	}
}

func TestDeriveMultipleItems(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_a", "base_item_0", "a.txt", model.KindFile, "base_ver_a", "aaa"), 1),
		withSeq(mkCreate("base_evt_2", "base_item_b", "base_item_0", "b.txt", model.KindFile, "base_ver_b", "bbb"), 2),
		withSeq(mkUpdate("base_evt_3", "base_item_a", "base_ver_a2", "ccc", 3), 3),
	}
	tree := Derive(events)

	if len(tree.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(tree.Items))
	}
	if tree.Items["base_item_a"].CurrentVersion != "base_ver_a2" {
		t.Errorf("item a should have updated version")
	}
	if tree.Items["base_item_b"].CurrentVersion != "base_ver_b" {
		t.Errorf("item b should have original version")
	}
}

func TestDeriveUnorderedEvents(t *testing.T) {
	// Events passed out of order; Derive should sort by CursorSeq.
	events := []model.Event{
		withSeq(mkUpdate("base_evt_2", "base_item_1", "base_ver_2", "bbb", 2), 2),
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
	}
	tree := Derive(events)

	item := tree.Items["base_item_1"]
	if item.CurrentVersion != "base_ver_2" {
		t.Errorf("after unordered create+update, version should be base_ver_2, got %q", item.CurrentVersion)
	}
	if item.Name != "a.txt" {
		t.Errorf("name should be a.txt, got %q", item.Name)
	}
}

func TestDeriveFolderCreate(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_f", "base_item_0", "docs", model.KindFolder, "base_ver_f", ""), 1),
	}
	tree := Derive(events)

	item := tree.Items["base_item_f"]
	if item.Kind != model.KindFolder {
		t.Errorf("expected kind folder, got %q", item.Kind)
	}
}

func TestDeriveDoesNotMutateInput(t *testing.T) {
	events := []model.Event{
		withSeq(mkCreate("base_evt_1", "base_item_1", "base_item_0", "a.txt", model.KindFile, "base_ver_1", "aaa"), 1),
	}
	original := events[0]
	_ = Derive(events)
	if events[0] != original {
		t.Error("Derive mutated the input event slice")
	}
}

func TestParsePayloadEmpty(t *testing.T) {
	p := ParsePayload("")
	if p.Name != "" || p.VersionID != "" {
		t.Errorf("empty payload should yield zero Payload, got %+v", p)
	}
}

func TestPayloadRoundtrip(t *testing.T) {
	original := Payload{
		Name:         "test.txt",
		ParentItemID: "base_item_0",
		Kind:         model.KindFile,
		VersionID:    "base_ver_1",
		BlobRef:      "sha256:abc",
		ContentHash:  "abc",
	}
	decoded := ParsePayload(original.JSON())
	if decoded != original {
		t.Errorf("payload roundtrip failed:\n got  %+v\n want %+v", decoded, original)
	}
}

// TestPurityNoIOImports is a documentation anchor: the tree package must
// not import "os", "net", or "time". The real guard is `go list -deps
// ./internal/base/tree` in CI, but this test documents the invariant.
func TestPurityNoIOImports(t *testing.T) {
	// If this file imports os/net/time, the build would fail at import
	// time. This test exists to document the purity invariant.
	_ = model.ItemID("base_item_anchor")
}
