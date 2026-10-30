package agentcore

// Witness v3, the SQLite content witness bind
// (docs/design/data-system-build-sheet-2026-10-30.md, decision 6). Written
// before the code. Ways this could fail:
//
//  1. the live store's table digests differ from a tape replay's at the same
//     head, so every checkpoint is judged not derivable (the replay writes
//     fresh wall-clock values in computer_event_index and
//     computer_event_projection_heads; those columns must be excluded, the
//     rows must not be);
//  2. the bind holds appends for longer than a streamed read of the store
//     takes (it must read in one read transaction, not under a lock the
//     writer needs);
//  3. a restore to a v3 checkpoint refuses, or accepts a non-derivable row;
//  4. a nil store handle panics instead of refusing.

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

func tapeReplayWitness(t *testing.T, fx *checkpointHeadFixture) selfdevprotocol.VMLocalContentWitness {
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
	w, err := LiveStoreWitness(ctx, replay.DB())
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// Failure mode 1: the live v3 witness equals a tape replay's.
func TestCheckpointBindV3WitnessEqualsTapeReplay(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-witness-v3-replay")
	fx.appendDeskEvent(t, "desk-turn-1")
	fx.appendDeskEvent(t, "desk-turn-2")
	ctx := context.Background()
	manifest, err := fx.rt.ensureCheckpointReleaseManifest(ctx, fx.rt.cfg.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	binding, _, err := fx.rt.bindCheckpoint(ctx, fx.rt.cfg.ComputerID, manifest.ContentDigest, manifest.Files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if binding.Witness.WitnessVersion != selfdevprotocol.WitnessVersionSQLite {
		t.Fatalf("the bind produced witness version %d, want %d", binding.Witness.WitnessVersion, selfdevprotocol.WitnessVersionSQLite)
	}
	for _, table := range []string{"computer_event_index", "computer_event_projection_heads"} {
		if _, hashed := binding.Witness.Tables[table]; !hashed {
			t.Fatalf("table %s is not in the witness; only its wall-clock columns are excluded", table)
		}
	}
	replay := tapeReplayWitness(t, fx)
	if err := selfdevprotocol.WitnessContentMatches(replay, binding.Witness); err != nil {
		t.Fatalf("live v3 witness differs from the tape replay's: %v", err)
	}
	if v := selfdevprotocol.JudgeCheckpointWitness(selfdevprotocol.DepositModeUpcast, binding.Witness, replay); v.Status != selfdevprotocol.CheckpointVerdictVerified {
		t.Fatalf("verdict on a derivable v3 checkpoint: %+v", v)
	}
}

// Failure mode 2: the witness read under the hold is bounded.
func TestCheckpointBindV3WitnessReadIsBounded(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-witness-v3-hold")
	fx.appendDeskEvent(t, "desk-turn-1")
	ctx := context.Background()
	manifest, err := fx.rt.ensureCheckpointReleaseManifest(ctx, fx.rt.cfg.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	binding, _, err := fx.rt.bindCheckpoint(ctx, fx.rt.cfg.ComputerID, manifest.ContentDigest, manifest.Files, nil)
	if err != nil {
		t.Fatal(err)
	}
	if binding.WitnessDuration <= 0 || binding.WitnessDuration > 10*time.Second {
		t.Fatalf("witness read under the hold took %v", binding.WitnessDuration)
	}
	t.Logf("hold witness read: %v", binding.WitnessDuration)
}

// Failure mode 4: a nil store handle on the restore path is an error, not a
// panic.
func TestStoreWitnessRefusesNilDB(t *testing.T) {
	if _, err := LiveStoreWitness(context.Background(), nil); err == nil {
		t.Fatal("LiveStoreWitness accepted a nil store handle")
	}
}
