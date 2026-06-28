package desktop

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
)

func TestConflictManagerSetAndPending(t *testing.T) {
	m := NewConflictManager()
	id := model.ItemID("base_item_conflict1")
	cs := []planner.Conflict{{
		ItemID:   id,
		Reason:   "both modified",
		LocalVer: model.Version{VersionID: "base_ver_l1"},
		RemoteVer: model.Version{VersionID: "base_ver_r1"},
	}}
	local := planner.NewTree()
	remote := planner.NewTree()
	m.SetConflicts(cs, local, remote)

	if m.Count() != 1 {
		t.Fatalf("Count: got %d, want 1", m.Count())
	}
	pending := m.Pending()
	if len(pending) != 1 {
		t.Fatalf("Pending: got %d, want 1", len(pending))
	}
	if !m.HasUnresolved() {
		t.Fatal("HasUnresolved: got false, want true")
	}
}

func TestConflictManagerResolve(t *testing.T) {
	m := NewConflictManager()
	id := model.ItemID("base_item_c1")
	cs := []planner.Conflict{{
		ItemID: id,
		Reason: "modify/delete",
	}}
	m.SetConflicts(cs, planner.NewTree(), planner.NewTree())

	// Resolve.
	if err := m.Resolve(id, ResolveKeepLocal); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if m.HasUnresolved() {
		t.Fatal("HasUnresolved after resolve: got true, want false")
	}
	res, ok := m.Resolution(id)
	if !ok || res != ResolveKeepLocal {
		t.Errorf("Resolution: got %q ok=%v, want keep_local", res, ok)
	}

	// Invalid resolution.
	if err := m.Resolve(id, ConflictResolution("bogus")); err == nil {
		t.Fatal("Resolve with invalid value should error")
	}

	// Unknown item.
	if err := m.Resolve(model.ItemID("base_item_unknown"), ResolveKeepRemote); err == nil {
		t.Fatal("Resolve unknown item should error")
	}
}

func TestConflictManagerClear(t *testing.T) {
	m := NewConflictManager()
	m.SetConflicts([]planner.Conflict{{ItemID: "base_item_x"}}, planner.NewTree(), planner.NewTree())
	m.Clear()
	if m.Count() != 0 {
		t.Fatalf("Count after clear: got %d, want 0", m.Count())
	}
}

func TestConflictResolutionValid(t *testing.T) {
	for _, r := range []ConflictResolution{ResolveKeepLocal, ResolveKeepRemote, ResolveKeepBoth} {
		if !r.Valid() {
			t.Errorf("%q should be valid", r)
		}
	}
	if ConflictResolution("nope").Valid() {
		t.Error("invalid resolution reported valid")
	}
}
