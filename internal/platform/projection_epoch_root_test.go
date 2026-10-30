package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// An epoch root (owner, 2026-10-27: "maybe we lose some event log data and
// restart it from here"): a captured base advertised as the watermark becomes
// the computer's root, and no job the platform admits for the computer asks
// for the tape below it. Ways it could fail:
//  1. the root is not recorded durably, so nothing below knows the floor;
//  2. a genesis repair replays from genesis instead of from the root;
//  3. a later, ordinary advertisement drops the root;
//  4. the advertisement path cannot tell a root from a replayed base.
func TestEpochRootFloorsEveryAdmittedJob(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	id := "computer-epoch"
	insertTestHead(t, s, id, 9000)
	preRoot, root := strings.Repeat("c", 64), strings.Repeat("d", 64)
	if err := s.RecordReplayWatermark(ctx, id, 100, preRoot); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReplayWatermarkWithRoot(ctx, id, 6000, root, true); err != nil {
		t.Fatal(err)
	}
	// 1
	if ref, seq, err := epochRoot(ctx, s.db, id); err != nil || ref != root || seq != 6000 {
		t.Fatalf("epoch root = %s %d %v, want %s 6000", ref, seq, err, root)
	}
	// 2
	repair, err := s.EnqueueProjectionJob(ctx, id, "repair", true)
	if err != nil || repair.SeedBaseRef != root || repair.WatermarkSequence != 6000 {
		t.Fatalf("genesis repair = %+v %v, want it seeded from the root", repair, err)
	}
	// 3
	if err := s.RecordReplayWatermark(ctx, id, 8000, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	if ref, seq, err := epochRoot(ctx, s.db, id); err != nil || ref != root || seq != 6000 {
		t.Fatalf("epoch root after a derived base = %s %d %v, want it kept", ref, seq, err)
	}
}

// Without a root nothing changes: a computer with none has no floor.
func TestNoEpochRootIsNoFloor(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	if ref, seq, err := epochRoot(ctx, s.db, "computer-rootless"); err != nil || ref != "" || seq != 0 {
		t.Fatalf("epoch root of a rootless computer = %q %d %v", ref, seq, err)
	}
}

// 4: the advertisement reads the root's mark from the host-written sidecar,
// not from the caller.
func TestAdvertisementBindingNamesAnEpochRoot(t *testing.T) {
	for raw, want := range map[string]bool{
		`{"computer_id":"c","sequence":1,"origin":"live_captured"}`: true,
		`{"computer_id":"c","sequence":1}`:                          false,
		`{"computer_id":"c","sequence":1,"origin":"other"}`:         false,
	} {
		var d projectionBaseDescriptorBinding
		if err := json.Unmarshal([]byte(raw), &d); err != nil {
			t.Fatal(err)
		}
		if got := d.epochRoot(); got != want {
			t.Fatalf("%s: epoch root = %v, want %v", raw, got, want)
		}
	}
}
