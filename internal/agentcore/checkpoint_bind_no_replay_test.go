package agentcore

// The checkpoint bind without a replay
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.1 and 4.6
// slice 1). The bind reads the head and the live witness under one append
// hold, pins the advertised base and publishes. It installs no base, replays
// no tape and writes no scratch: the replay proved derivability early, and
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
//  5. the bind forgets to pin the advertised base, so a restore to an old
//     checkpoint has no start point;
//  6. a base source that cannot answer is treated as "no base": the
//     checkpoint is published unpinned.

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// countingBaseSource counts the blobs a caller downloads: an installed base
// is a download.
type countingBaseSource struct {
	*restoreBaseFake
	mu        sync.Mutex
	downloads int
}

func (c *countingBaseSource) DownloadBlob(ctx context.Context, computerID, baseRef string, dst io.Writer) error {
	c.mu.Lock()
	c.downloads++
	c.mu.Unlock()
	return c.restoreBaseFake.DownloadBlob(ctx, computerID, baseRef, dst)
}

func (c *countingBaseSource) downloadCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.downloads
}

// noReplayFixture is a checkpoint fixture whose volume cannot hold any replay
// scratch (any probe refuses by name) and whose probe seam records a probe,
// so a bind that still replays is caught on its first step.
type noReplayFixture struct {
	*checkpointHeadFixture
	volume *fakeScratchVolume
	source *countingBaseSource
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
	nf.source = &countingBaseSource{restoreBaseFake: fx.base}
	fx.rt.restoreBaseSource = nf.source
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

// Failure modes 1 and 5: the bind creates no scratch and installs no base,
// and still pins the advertised base under the checkpoint's head.
func TestCheckpointBindCreatesNoScratchAndInstallsNoBase(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-bind-no-scratch")
	headBefore := nf.liveHead(t)

	report := nf.bind(t)

	if nf.probed {
		t.Fatal("the bind opened a replay probe")
	}
	if calls := nf.volume.callCount(); calls != 0 {
		t.Fatalf("the bind admitted a scratch against the volume %d times; it needs none", calls)
	}
	if downloads := nf.source.downloadCount(); downloads != 0 {
		t.Fatalf("the bind downloaded %d base blobs; it installs no base", downloads)
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
	want := nf.base.descriptor.BlobSHA256 + " " + projectionbase.CheckpointPinReference(headBefore)
	pinned := false
	for _, pin := range nf.base.pins {
		if pin == want {
			pinned = true
		}
	}
	if !pinned {
		t.Fatalf("base pins %v do not pin the advertised base as %q", nf.base.pins, want)
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
	want, err := selfdevprotocol.WitnessFromObservationSets(probe.Live, probe.Replay, probe.Result)
	if err != nil {
		t.Fatalf("replay path witness: %v", err)
	}
	nf.volume.set(replayScratchTestGiB)
	nf.rt.replayProbeAfterCapture = func() { nf.probed = true }
	callsBefore := nf.volume.callCount()

	report := nf.bind(t)

	if nf.probed || nf.volume.callCount() != callsBefore {
		t.Fatal("the bind replayed to get its witness")
	}
	got := report.PublishedCheckpoint.Checkpoint.Request.VMLocalContentWitness
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bind witness differs from the replay path's:\nbind   %+v\nreplay %+v", got, want)
	}
	if !reflect.DeepEqual(report.VMLocalContentWitness, want) {
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

// Failure mode 5: a computer the platform never published a base for binds
// unpinned (restore then replays its bounded chain from genesis), as the
// probe did.
func TestCheckpointBindWithoutAdvertisedBaseBindsUnpinned(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-bind-no-base")
	nf.base.emptyWatermark = true

	report := nf.bind(t)

	if len(nf.base.pins) != 0 {
		t.Fatalf("pins %v with no advertised base", nf.base.pins)
	}
	result, err := nf.restore(t, report.PublishedCheckpoint.Checkpoint)
	if err != nil || !result.WitnessMatched {
		t.Fatalf("restore to the unpinned checkpoint: %v %#v", err, result)
	}
}

// Failure mode 6: a base source that fails to answer is not "no base". The
// checkpoint is not published unpinned; the bind fails and the caller retries.
func TestCheckpointBindRefusesWhenTheBaseSourceCannotAnswer(t *testing.T) {
	nf := newNoReplayFixture(t, "computer-bind-source-down")
	nf.base.watermarkErr = errors.New("corpusd unreachable")

	_, err := nf.rt.BindCheckpointRestoreSet(context.Background(), nf.rt.cfg.ComputerID)
	if err == nil {
		t.Fatal("a bind that could not read the advertised base published a checkpoint")
	}
	if len(nf.platform.published) != 0 {
		t.Fatalf("%d checkpoints published without a base answer", len(nf.platform.published))
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
