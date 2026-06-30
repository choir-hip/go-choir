package desktop

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	"github.com/yusefmosiah/go-choir/internal/base/planner"
)

func TestStatusTrackerPhase(t *testing.T) {
	s := NewStatusTracker()
	if s.Snapshot().Phase != PhaseIdle {
		t.Errorf("initial phase: got %q, want idle", s.Snapshot().Phase)
	}
	s.SetPhase(PhaseScanning)
	if s.Snapshot().Phase != PhaseScanning {
		t.Errorf("phase: got %q, want scanning", s.Snapshot().Phase)
	}
}

func TestStatusTrackerCursorAndActions(t *testing.T) {
	s := NewStatusTracker()
	s.SetCursor(10, 20)
	p := s.Snapshot()
	if p.Cursor != 10 || p.RemoteHead != 20 {
		t.Errorf("cursor: got %d/%d, want 10/20", p.Cursor, p.RemoteHead)
	}
	s.SetActionTotals(5)
	s.ActionDone()
	s.ActionDone()
	p = s.Snapshot()
	if p.ActionsTotal != 5 || p.ActionsDone != 2 {
		t.Errorf("actions: got %d/%d, want 5/2", p.ActionsTotal, p.ActionsDone)
	}
}

func TestStatusTrackerErrorAndSynced(t *testing.T) {
	s := NewStatusTracker()
	s.SetError("boom")
	p := s.Snapshot()
	if p.Phase != PhaseError || p.LastError != "boom" {
		t.Errorf("error state: %+v", p)
	}
	s.MarkSynced()
	p = s.Snapshot()
	if p.Phase != PhaseIdle || p.LastError != "" || p.LastSyncAt == nil {
		t.Errorf("synced state: %+v", p)
	}
}

func TestStatusTrackerUpdateFromPlan(t *testing.T) {
	s := NewStatusTracker()
	id := model.ItemID("base_item_s1")
	actions := []planner.Action{
		{Type: planner.ActionUpload, ItemID: id},
	}
	conflicts := []planner.Conflict{
		{ItemID: model.ItemID("base_item_c1"), Reason: "both modified"},
	}
	local := planner.NewTree()
	local.Items[id] = model.Item{ItemID: id, Name: "f.txt", Kind: model.KindFile}
	remote := planner.NewTree()

	s.UpdateFromPlan(actions, conflicts, local, remote)
	p := s.Snapshot()
	if p.ConflictsCount != 1 {
		t.Errorf("conflicts count: got %d, want 1", p.ConflictsCount)
	}
	// The conflict item should have state conflict.
	found := false
	for _, st := range p.Items {
		if st.ItemID == model.ItemID("base_item_c1") && st.State == model.StateConflict {
			found = true
		}
	}
	if !found {
		t.Error("conflict item not present with StateConflict")
	}
}

func TestStatusTrackerUpdateFromPlanCarriesCollisionParticipants(t *testing.T) {
	s := NewStatusTracker()
	localID := model.ItemID("base_item_local")
	remoteID := model.ItemID("base_item_remote")
	conflicts := []planner.Conflict{{
		ItemID:       remoteID,
		LocalItemID:  localID,
		RemoteItemID: remoteID,
		Reason:       "add/add path collision",
		LocalVer:     model.Version{ItemID: localID, VersionID: "base_ver_local"},
		RemoteVer:    model.Version{ItemID: remoteID, VersionID: "base_ver_remote"},
	}}
	local := planner.NewTree()
	local.Items[localID] = model.Item{ItemID: localID, Name: "same.txt", Kind: model.KindFile}
	remote := planner.NewTree()
	remote.Items[remoteID] = model.Item{ItemID: remoteID, Name: "same.txt", Kind: model.KindFile}

	s.UpdateFromPlan(nil, conflicts, local, remote)
	p := s.Snapshot()
	if len(p.Items) != 1 {
		t.Fatalf("status items: got %d want 1", len(p.Items))
	}
	st := p.Items[0]
	if st.LocalItemID != localID || st.RemoteItemID != remoteID {
		t.Fatalf("participant ids: got local=%q remote=%q", st.LocalItemID, st.RemoteItemID)
	}
	if st.Path != "same.txt" {
		t.Fatalf("path: got %q want same.txt", st.Path)
	}
}

func TestStatusTrackerCancelled(t *testing.T) {
	s := NewStatusTracker()
	s.MarkCancelled()
	if s.Snapshot().Phase != PhaseCancelled {
		t.Errorf("phase: got %q, want cancelled", s.Snapshot().Phase)
	}
}
