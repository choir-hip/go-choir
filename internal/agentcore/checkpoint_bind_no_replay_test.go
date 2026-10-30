package agentcore

// The checkpoint bind without a replay
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.1 and 4.6
// slice 1). The bind reads the head and the live witness under one append
// hold and publishes. It installs no snapshot, replays no tape and writes no
// scratch: the replay proved derivability early, and
// restore proves it again whenever the checkpoint is used. Written before the
// code. Ways this could fail:
//
//  1. the bind still opens the replay probe: it admits a scratch against the
//     volume, installs the advertised base and replays the tail (the apply
//     that filled b@c's tmpfs);
//  2. the bind's witness differs from the replay path's at the same head, so
//     a restore to it refuses;
//  3. a checkpoint the bind published cannot be restored to;
//  4. a live store with a non-derivable row is refused at the bind (the early
//     gate is withdrawn), or is restorable: restore alone must refuse it;

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// noReplayFixture is a checkpoint fixture whose volume cannot hold any replay
// scratch (any probe refuses by name) and whose probe seam records a probe,
// so a bind that still replays is caught on its first step.
type noReplayFixture struct {
	*checkpointHeadFixture
	volume *fakeScratchVolume
	tmp    string
	probed bool
}

func newNoReplayFixture(t *testing.T, computerID string) *noReplayFixture {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	fx := newCheckpointHeadFixture(t, computerID)
	fx.appendDeskEvent(t, "desk-turn-1")
	fx.appendDeskEvent(t, "desk-turn-2")
	nf := &noReplayFixture{checkpointHeadFixture: fx, tmp: tmp}
	nf.volume = &fakeScratchVolume{avail: replayScratchTestGiB} // under the 2 GiB reserve: a probe refuses
	fx.rt.replayScratchUsage = nf.volume.usage
	fx.rt.replayProbeAfterCapture = func() { nf.probed = true }
	return nf
}

// scratchTraces lists everything a probe leaves or would leave: its root in
// the old location and the new one, and its parent directory.
func (nf *noReplayFixture) scratchTraces(t *testing.T) []string {
	t.Helper()
	var traces []string
	parent := nf.rt.replayScratchParent()
	if _, err := os.Stat(parent); err == nil {
		traces = append(traces, parent)
	}
	traces = append(traces, replayScratchRoots(t, nf.tmp)...)
	if parent != "" {
		traces = append(traces, replayScratchRoots(t, parent)...)
	}
	return traces
}

func (nf *noReplayFixture) bind(t *testing.T) CheckpointBindReport {
	t.Helper()
	report, err := nf.rt.BindCheckpointRestoreSet(context.Background(), nf.rt.cfg.ComputerID)
	if err != nil {
		t.Fatalf("bind on a volume with no room for a replay: %v", err)
	}
	if report.PublishedCheckpoint == nil {
		t.Fatal("no checkpoint was published")
	}
	return report
}

// Failure mode 1: the bind creates no scratch and installs no snapshot.
func TestCheckpointBindCreatesNoScratchAndInstallsNoSnapshot(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-bind-no-scratch")
	headBefore := nf.liveHead(t)

	report := nf.bind(t)

	if nf.probed {
		t.Fatal("the bind opened a replay probe")
	}
	if calls := nf.volume.callCount(); calls != 0 {
		t.Fatalf("the bind admitted a scratch against the volume %d times; it needs none", calls)
	}
	if reads := nf.base.chunkReadCount(); reads != 0 {
		t.Fatalf("the bind read %d snapshot chunks; it installs no snapshot", reads)
	}
	if traces := nf.scratchTraces(t); len(traces) != 0 {
		t.Fatalf("the bind left scratch traces: %v", traces)
	}
	if headAfter := nf.liveHead(t); headAfter != headBefore {
		t.Fatalf("the bind moved the live head %s -> %s; it writes no event", headBefore, headAfter)
	}
	published := report.PublishedCheckpoint.Checkpoint.Request
	if published.AcceptedEventHead != headBefore {
		t.Fatalf("published head %s, want the head the bind read %s", published.AcceptedEventHead, headBefore)
	}
}

// Failure mode 2: the witness the bind publishes is the witness the replay
// path computes at the same head (both digests, every table).
func TestCheckpointBindWitnessEqualsReplayPathWitness(t *testing.T) {
	computerID := "computer-bind-witness"
	nf := newNoReplayFixture(t, computerID)
	// The replay path, with room for its scratch, at the head the bind will take.
	nf.volume.set(50 * replayScratchTestGiB)
	nf.rt.replayProbeAfterCapture = nil
	probe, err := nf.rt.ReplayCompleteness(context.Background(), computerID)
	if err != nil {
		t.Fatalf("replay path: %v", err)
	}
	if !probe.Result.Equivalent() {
		t.Fatal("replay path: live and replay are not equivalent")
	}
	// The bind publishes the v3 witness; the replay path's is the same
	// witness read from a tape replay.
	want := tapeReplayWitness(t, nf.checkpointHeadFixture)
	nf.volume.set(replayScratchTestGiB)
	nf.rt.replayProbeAfterCapture = func() { nf.probed = true }
	callsBefore := nf.volume.callCount()

	report := nf.bind(t)

	if nf.probed || nf.volume.callCount() != callsBefore {
		t.Fatal("the bind replayed to get its witness")
	}
	got := report.PublishedCheckpoint.Checkpoint.Request.VMLocalContentWitness
	if err := selfdevprotocol.WitnessContentMatches(want, got); err != nil {
		t.Fatalf("bind witness differs from the replay path's: %v\nbind   %+v\nreplay %+v", err, got, want)
	}
	if !reflect.DeepEqual(report.VMLocalContentWitness, got) {
		t.Fatalf("bind report witness differs from the replay path's:\nreport %+v\nreplay %+v", report.VMLocalContentWitness, want)
	}
}

// Failure mode 3: a restore to a checkpoint the bind published succeeds.
func TestCheckpointBindWithoutReplayIsRestorable(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-bind-restorable")
	report := nf.bind(t)

	result, err := nf.restore(t, report.PublishedCheckpoint.Checkpoint)
	if err != nil {
		t.Fatalf("restore to the no-replay checkpoint refused: %v", err)
	}
	if !result.WitnessMatched {
		t.Fatalf("restore did not match the witness: %#v", result)
	}
}

// Failure mode 4: the early derivability gate is withdrawn, not the check. A
// live store with a row replay cannot produce is checkpointed (the bind no
// longer proves derivability), and the restore to it refuses.
func TestCheckpointBindOfNonDerivableStoreRefusesAtRestore(t *testing.T) {
	ctx := context.Background()
	nf := newNoReplayFixture(t, "computer-bind-non-derivable")
	if _, err := nf.live.DB().ExecContext(ctx, "CREATE TABLE checkpoint_live_only (id VARCHAR(64) PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := nf.live.DB().ExecContext(ctx, "INSERT INTO checkpoint_live_only (id) VALUES ('drift')"); err != nil {
		t.Fatal(err)
	}

	report := nf.bind(t)

	if len(nf.platform.published) != 1 {
		t.Fatalf("%d checkpoints published, want the bind to publish its one", len(nf.platform.published))
	}
	result, err := nf.restore(t, report.PublishedCheckpoint.Checkpoint)
	if err == nil {
		t.Fatalf("restore to a checkpoint over a non-derivable row succeeded: %#v", result)
	}
	if !strings.Contains(err.Error(), "does not match checkpoint") {
		t.Fatalf("restore refused for another reason: %v", err)
	}
	if result.WitnessMatched {
		t.Fatalf("restore reported a witness match over a non-derivable row: %#v", result)
	}
}

// A computer whose tape holds no snapshot binds the same checkpoint (restore
// then replays its bounded chain from genesis), as the probe does.
func TestCheckpointBindWithoutASnapshotRestoresFromGenesis(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-bind-no-snapshot")
	nf.base.snapshotDigest = ""

	report := nf.bind(t)

	result, err := nf.restore(t, report.PublishedCheckpoint.Checkpoint)
	if err != nil || !result.WitnessMatched {
		t.Fatalf("restore to the checkpoint with no snapshot: %v %#v", err, result)
	}
}

// The frontend identity is part of the bind; a release that cannot be read
// refuses before anything is published or replayed.
func TestCheckpointBindWithoutReplayRefusesAnInvalidRelease(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-bind-no-release")
	_, _, err := nf.rt.bindCheckpointAtHead(context.Background(), nf.rt.cfg.ComputerID, "not-a-digest", []updater.ManifestFile{{Path: "index.html", SHA256: "not-a-digest"}}, nil)
	if err == nil {
		t.Fatal("a bind with an invalid release identity succeeded")
	}
	if nf.probed || nf.volume.callCount() != 0 {
		t.Fatal("the refused bind reached the replay probe")
	}
}
