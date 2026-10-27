package platform

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// An epoch root (owner, 2026-10-27: "maybe we lose some event log data and
// restart it from here"): a captured base advertised as the watermark becomes
// the computer's root, and no job the platform admits for the computer asks
// for the tape below it. Ways it could fail, written before the code:
//  1. the root is not recorded durably, so nothing below knows the floor;
//  2. quarantine of the advertised seed falls back to an older pre-root base,
//     or to no seed (genesis);
//  3. a genesis repair replays from genesis instead of from the root;
//  4. a checkpoint verification below the root asks for the prefix, or one
//     above it seeds from a pre-root base;
//  5. a later, ordinary advertisement drops the root;
//  6. the advertisement path cannot tell a root from a replayed base.
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
	if ref, seq, err := s.EpochRoot(ctx, id); err != nil || ref != root || seq != 6000 {
		t.Fatalf("epoch root = %s %d %v, want %s 6000", ref, seq, err, root)
	}
	// 3
	repair, err := s.EnqueueProjectionJob(ctx, id, "repair", true)
	if err != nil || repair.SeedBaseRef != root || repair.WatermarkSequence != 6000 {
		t.Fatalf("genesis repair = %+v %v, want it seeded from the root", repair, err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM computer_projection_jobs WHERE computer_id=?`, id); err != nil {
		t.Fatal(err)
	}
	// 2
	for i := 0; i < baseQuarantineRefusals; i++ {
		if _, err := s.RecordBaseRefusal(ctx, id, root, "projection base refused"); err != nil {
			t.Fatal(err)
		}
	}
	j, err := s.EnqueueProjectionJob(ctx, id, "cadence", false)
	if err != nil || j.SeedBaseRef != root || j.WatermarkSequence != 6000 {
		t.Fatalf("cadence after quarantining the root = %+v %v, want the root (never %s or genesis)", j, err, preRoot)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM computer_projection_jobs WHERE computer_id=?`, id); err != nil {
		t.Fatal(err)
	}
	// 4
	if _, err := s.enqueueProjectionJob(ctx, id, checkpointVerifyReason+"below", false, &projectionTarget{sequence: 5000, head: strings.Repeat("e", 64)}); !errors.Is(err, ErrNoVerificationBase) || !strings.Contains(err.Error(), "epoch root") {
		t.Fatalf("verification below the root: err = %v, want no base (below the epoch root)", err)
	}
	v, err := s.enqueueProjectionJob(ctx, id, checkpointVerifyReason+"above", false, &projectionTarget{sequence: 7000, head: strings.Repeat("e", 64)})
	if err != nil || v.SeedBaseRef != root || v.WatermarkSequence != 6000 {
		t.Fatalf("verification above the root = %+v %v, want it seeded from the root", v, err)
	}
	// 5
	if err := s.RecordReplayWatermark(ctx, id, 8000, strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	if ref, seq, err := s.EpochRoot(ctx, id); err != nil || ref != root || seq != 6000 {
		t.Fatalf("epoch root after a derived base = %s %d %v, want it kept", ref, seq, err)
	}
}

// Review 365 (DeepSeek 3): the advertised-watermark fast path of seed
// selection applies the floor too. A watermark below the root (only a hand
// rollback leaves one) is not used as a seed.
func TestVerificationSeedFastPathHonorsTheRoot(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	id := "computer-epoch-fast"
	insertTestHead(t, s, id, 9000)
	preRoot, root := strings.Repeat("c", 64), strings.Repeat("d", 64)
	if err := s.RecordReplayWatermark(ctx, id, 100, preRoot); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordReplayWatermarkWithRoot(ctx, id, 6000, root, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE computer_replay_watermarks SET watermark_sequence=100,base_ref=? WHERE computer_id=?`, preRoot, id); err != nil {
		t.Fatal(err)
	}
	v, err := s.enqueueProjectionJob(ctx, id, checkpointVerifyReason+"fast", false, &projectionTarget{sequence: 7000, head: strings.Repeat("e", 64)})
	if err != nil || v.SeedBaseRef != root || v.WatermarkSequence != 6000 {
		t.Fatalf("verification seed = %+v %v, want the root, never the pre-root watermark", v, err)
	}
}

// Without a root nothing changes: a computer with none has no floor.
func TestNoEpochRootIsNoFloor(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	ctx := context.Background()
	if ref, seq, err := s.EpochRoot(ctx, "computer-rootless"); err != nil || ref != "" || seq != 0 {
		t.Fatalf("epoch root of a rootless computer = %q %d %v", ref, seq, err)
	}
}

// 6: the advertisement reads the root's mark from the host-written sidecar,
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
