package agentcore

// Witness v2, the hash-only bind (docs/problems/paper-never-checkpointed-2026-10-27.md,
// S3). Written before the code. Ways this could fail:
//
//  1. the bind still extracts every row under the append hold (the 2.2 s on
//     d@e, minutes on the paper);
//  2. the live store's Dolt table hashes differ from a tape replay's at the
//     same head, so every checkpoint is judged not derivable;
//  3. a table whose wall-clock columns differ by construction is compared;
//  4. a restore to a v2 checkpoint refuses, or accepts a non-derivable row.
//
// Finding (2026-10-28, this fixture): DOLT_HASHOF_TABLE matches between the
// live store and a fresh tape replay for 24 of 26 tables. The two that differ
// are computer_event_index and computer_event_projection_heads, exactly the
// tables whose wall-clock columns the v1 witness ignores.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

func tapeReplayV2Witness(t *testing.T, fx *checkpointHeadFixture) selfdevprotocol.VMLocalContentWitness {
	t.Helper()
	ctx := context.Background()
	replay, err := choirstore.OpenFresh(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer replay.Close()
	if err := fx.rt.eventAppender.ReconstructInto(ctx, replay); err != nil {
		t.Fatal(err)
	}
	w, err := doltHashWitness(ctx, replay.DB(), replayDoltStateExtractor(replay.TexturePath()))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// Failure modes 2 and 3: the live v2 witness equals a tape replay's.
func TestCheckpointBindV2WitnessEqualsTapeReplay(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-hash-v2-replay")
	fx.appendDeskEvent(t, "desk-turn-1")
	fx.appendDeskEvent(t, "desk-turn-2")
	ctx := context.Background()
	manifest, err := fx.rt.ensureCheckpointReleaseManifest(ctx, fx.rt.cfg.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	binding, _, err := fx.rt.bindCheckpoint(ctx, fx.rt.cfg.ComputerID, manifest.ContentDigest, manifest.Files, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !binding.Witness.IsDoltHashWitness() {
		t.Fatalf("the bind produced witness version %d, want the v2 hash witness", binding.Witness.WitnessVersion)
	}
	for _, table := range []string{"computer_event_index", "computer_event_projection_heads"} {
		if _, hashed := binding.Witness.DoltTables[table]; hashed {
			t.Fatalf("operational table %s is hashed", table)
		}
	}
	replay := tapeReplayV2Witness(t, fx)
	if err := selfdevprotocol.WitnessContentMatches(replay, binding.Witness); err != nil {
		t.Fatalf("live v2 witness differs from the tape replay's: %v", err)
	}
	if v := selfdevprotocol.JudgeCheckpointWitness(selfdevprotocol.DepositModeUpcast, binding.Witness, replay); v.Status != selfdevprotocol.CheckpointVerdictVerified {
		t.Fatalf("host verdict on a derivable v2 checkpoint: %+v", v)
	}
}

// Failure mode 1: the hold covers only the hash read; no row extract runs.
func TestCheckpointBindV2HoldsOnlyForTheHashRead(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-hash-v2-hold")
	fx.appendDeskEvent(t, "desk-turn-1")
	ctx := context.Background()
	manifest, err := fx.rt.ensureCheckpointReleaseManifest(ctx, fx.rt.cfg.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	binding, _, err := fx.rt.bindCheckpoint(ctx, fx.rt.cfg.ComputerID, manifest.ContentDigest, manifest.Files, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !binding.Witness.IsDoltHashWitness() || len(binding.Witness.Tables) != 0 || len(binding.Witness.Schema) != 0 {
		t.Fatalf("the bind hashed rows under the hold: %+v", binding.Witness)
	}
	if binding.WitnessDuration <= 0 || binding.WitnessDuration > 10*time.Second {
		t.Fatalf("witness read under the hold took %v", binding.WitnessDuration)
	}
	// The v1 row extract, for the record: what the hold used to cover.
	head, err := fx.rt.store.Head(ctx, fx.rt.cfg.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := replayDoltStateExtractor(fx.rt.store.TexturePath()).Extract(ctx, computerversion.ExtractRequest{Name: "live-dolt", Version: replayCompletenessVersion(fx.rt, head)}); err != nil {
		t.Fatal(err)
	}
	t.Logf("hold witness read: v2 %v, v1 row extract %v", binding.WitnessDuration, time.Since(start))
}

// A nil store handle on the restore path is an error, not a panic.
func TestDoltHashWitnessRefusesNilDB(t *testing.T) {
	if _, err := doltHashWitness(context.Background(), nil, replayDoltStateExtractor(t.TempDir())); err == nil {
		t.Fatal("doltHashWitness accepted a nil store handle")
	}
}
