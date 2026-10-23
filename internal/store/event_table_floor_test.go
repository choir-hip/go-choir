package store

// The store floor through the store (review finding A on deploy 1, for
// deploy 2). Ways it can fail, written before the code:
//   - the writer stamps before the floor is durable, so a crash between the
//     first stamp and the floor write leaves stamped rows with no floor;
//   - a store with stamped rows but no floor file (a rebuild or a restore
//     that replayed stamped ops into a fresh workspace) stays unfloored;
//   - deploy 1 alone (no writer switched on) writes a floor, so the
//     updater's recovery to a pre-deploy-1 release is refused for no reason;
//   - an unbound store (no tape, never stamps) writes a floor;
//   - Open runs under a floor above its own epoch, or lowers a newer
//     epoch receipt.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func storeFloor(t *testing.T, workspace string) (storeschema.Floor, bool) {
	t.Helper()
	floor, found, err := storeschema.ReadFloor(filepath.Join(workspace, storeschema.FloorFile))
	if err != nil {
		t.Fatal(err)
	}
	return floor, found
}

func TestEventTableFloorIsDurableBeforeTheFirstStamp(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	for i := 0; i < 3; i++ {
		if err := h.store.AppendEvent(ctx, h.event("pre-"+string(rune('a'+i)), "run-f", "owner-1", types.EventToolResult, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	if _, found := storeFloor(t, h.store.texturePath); found {
		t.Fatal("deploy 1 behaviour (the writer off) wrote a floor")
	}
	if err := h.store.SetEventTableWrites(true); err != nil {
		t.Fatal(err)
	}
	floor, found := storeFloor(t, h.store.texturePath)
	if !found || floor.Version != eventTableFloorEpoch {
		t.Fatalf("after the writer switched on, floor = %+v found=%v; want %d before any stamp", floor, found, eventTableFloorEpoch)
	}
	if eventTableFloorEpoch != 2 {
		t.Fatalf("the floor epoch is %d; deploy 1 (epoch 2) is the floor", eventTableFloorEpoch)
	}
	// Off keeps the floor: stamped rows may already exist.
	if err := h.store.SetEventTableWrites(false); err != nil {
		t.Fatal(err)
	}
	if _, found := storeFloor(t, h.store.texturePath); !found {
		t.Fatal("turning the writer off removed the floor")
	}
}

// A floor that cannot be written keeps the writer off: it fails closed.
func TestEventTableWriterStaysOffWhenTheFloorCannotBeWritten(t *testing.T) {
	h := newTapeHarness(t)
	if err := os.WriteFile(filepath.Join(h.store.texturePath, storeschema.FloorFile), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := h.store.SetEventTableWrites(true); err == nil {
		t.Fatal("the writer switched on without a durable floor")
	}
	if h.store.EventTableWritesOn() {
		t.Fatal("the writer is on after the floor write failed")
	}
}

// A workspace that holds stamped rows but no floor file (a replay into a
// fresh workspace, an installed base) is floored at its next Open.
func TestEventTableFloorAtOpenWhenTheTableHasRows(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	projectAt(t, s, 2, eventTableOp(t, eventTableRecord("open-1", "run-o", "owner-1", "traj-o", types.EventToolResult, 1, 1, eventTableBase), true))
	workspace := s.texturePath
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(workspace, storeschema.FloorFile))
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if floor, found := storeFloor(t, workspace); !found || floor.Version != eventTableFloorEpoch {
		t.Fatalf("Open over a non-empty choir_events: floor = %+v found=%v", floor, found)
	}
}

func TestOpenRefusesAFloorAboveItsEpoch(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	workspace := s.texturePath
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := storeschema.WriteFloor(workspace, storeschema.Version+1, "a later binary stamped"); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(path); err == nil {
		_ = s.Close()
		t.Fatal("Open ran under a floor above its own epoch")
	} else if !strings.Contains(err.Error(), "floor") {
		t.Fatalf("Open's refusal does not name the floor: %v", err)
	}
}

// Round 2 of deploy 2, bunch 148 (DeepSeek P3 and P4, Devin 2a), written
// before the fixes.

// SetEventTableWrites(true) on a store with no tape used to set the switch
// and write no floor. The switch is refused instead, so the store itself
// keeps the invariant "writer on implies floored" rather than leaving it to
// the runtime's ProjectionTapeBound check.
func TestEventTableUnboundStoreRefusesTheWriter(t *testing.T) {
	s := openTestStore(t)
	if err := s.SetEventTableWrites(true); err == nil {
		t.Fatal("SetEventTableWrites(true) on a store with no tape returned nil")
	}
	if s.EventTableWritesOn() {
		t.Fatal("the switch is on for a store with no tape")
	}
	if _, found := storeFloor(t, s.texturePath); found {
		t.Fatal("a refused switch wrote a floor")
	}
	// Off is always allowed.
	if err := s.SetEventTableWrites(false); err != nil {
		t.Fatalf("off on an unbound store: %v", err)
	}
}

// The floor the writer writes must be one this build may run on: a
// storeschema.Version below eventTableFloorEpoch would floor the store above
// its own binary, and the next boot would refuse its own store.
func TestEventTableFloorEpochIsAtMostTheBuildEpoch(t *testing.T) {
	if storeschema.Version < eventTableFloorEpoch {
		t.Fatalf("storeschema.Version %d is below eventTableFloorEpoch %d: the writer would floor the store above this build", storeschema.Version, eventTableFloorEpoch)
	}
}

// Reopen adopts a workspace; the floor goes into that workspace before it is
// adopted, so no stamped append can land in an adopted workspace that has no
// floor yet. The ordering is observed at the moment just before the handles
// are swapped (reopenBeforeAdoptHook): the adopted workspace already holds
// the floor and the store still points at its old workspace. A floor written
// after the swap, as it was before round 2c, fails here.
func TestEventTableReopenFloorsTheAdoptedWorkspaceBeforeAdopting(t *testing.T) {
	other := testStorePath(t) + "-adopt"
	cleanupTestStorePath(other)
	t.Cleanup(func() { cleanupTestStorePath(other) }) // runs after the harness closes the store
	h := newTapeHarness(t)
	s := h.store
	if err := s.SetEventTableWrites(true); err != nil {
		t.Fatal(err)
	}
	oldWorkspace := s.texturePath
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	observed := false
	reopenBeforeAdoptHook = func(adopted string) {
		observed = true
		if floor, found := storeFloor(t, adopted); !found || floor.Version != eventTableFloorEpoch {
			t.Errorf("before the swap the adopted workspace has floor %+v found=%v, want %d", floor, found, eventTableFloorEpoch)
		}
		if s.texturePath != oldWorkspace {
			t.Errorf("the hook ran after the swap: store points at %s, not %s", s.texturePath, oldWorkspace)
		}
	}
	t.Cleanup(func() { reopenBeforeAdoptHook = nil })
	if err := s.Reopen(other); err != nil {
		t.Fatal(err)
	}
	if !observed {
		t.Fatal("Reopen never reached the before-adopt hook")
	}
	if !s.EventTableWritesOn() {
		t.Fatal("the switch went off across a clean reopen")
	}
	if floor, found := storeFloor(t, s.texturePath); !found || floor.Version != eventTableFloorEpoch {
		t.Fatalf("adopted workspace floor = %+v found=%v, want %d", floor, found, eventTableFloorEpoch)
	}
}
