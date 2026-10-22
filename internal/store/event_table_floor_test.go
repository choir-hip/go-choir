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

func TestEventTableUnboundStoreWritesNoFloor(t *testing.T) {
	s := openTestStore(t)
	if err := s.SetEventTableWrites(true); err != nil {
		t.Fatal(err)
	}
	if _, found := storeFloor(t, s.texturePath); found {
		t.Fatal("an unbound store, which never stamps, wrote a floor")
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
