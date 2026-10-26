package agentcore

// Failure modes of the replay probe's scratch, written before the fix
// (docs/problems/apply-checkpoint-replay-fills-guest-tmp-2026-10-26.md):
//
//  1. The scratch cannot fit: the probe started anyway, filled the volume
//     and Dolt's Fatalf killed the runtime. It must refuse by name before it
//     captures anything or writes any scratch.
//  2. The scratch runs out while the probe replays (the live store shares the
//     volume and keeps writing): the probe must stop by name, not Fatalf, and
//     remove its scratch.
//  3. A probe killed in flight left its root behind, and nothing removed it:
//     the next start must remove every stale root, old location and new.
//  4. (Withdrawn: the apply's checkpoint bind no longer replays, so it has no
//     scratch to hold on; checkpoint_bind_no_replay_test.go and
//     selfdev_settlement_attempts_test.go prove that on a full volume.)
//  5. The probe's base-plus-tail replay (the incremental path) must give the
//     same witness as a full replay from genesis on the same chain.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

const replayScratchTestGiB = uint64(1) << 30

// fakeScratchVolume reports a volume of 100 GiB whose free space a test sets.
type fakeScratchVolume struct {
	mu    sync.Mutex
	avail uint64
	calls int
	dirs  []string
}

func (v *fakeScratchVolume) set(avail uint64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.avail = avail
}

func (v *fakeScratchVolume) usage(dir string) (persistentdisk.Usage, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls++
	v.dirs = append(v.dirs, dir)
	total := 100 * replayScratchTestGiB
	return persistentdisk.Usage{TotalBytes: total, UsedBytes: total - v.avail, AvailBytes: v.avail}, nil
}

// readDirs copies the directories usage was asked about, under the lock: the
// probe's watchdog goroutine can still be calling usage when the test reads.
func (v *fakeScratchVolume) readDirs() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]string(nil), v.dirs...)
}

func (v *fakeScratchVolume) callCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.calls
}

// replayScratchRuntime is a runtime with a genesis chain plus n updates and
// a real base at sequence 1, so the probe installs a base and replays a tail.
func replayScratchRuntime(t *testing.T, computerID string, updates int) (*Runtime, *replayEventCAS, string) {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.Close() })
	rt, cas, _ := rematerializeTapeRuntime(t, computerID, storePath, live)
	appendReplayScratchUpdates(t, rt, computerID, updates)
	seedRestoreBase(t, context.Background(), rt, cas, computerID, 1)
	return rt, cas, storePath
}

func appendReplayScratchUpdates(t *testing.T, rt *Runtime, computerID string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		eventID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		commitment := fmt.Sprintf("%064x", 0xf00+i)
		update := computerevent.Event{SchemaVersion: 1, EventID: eventID, ComputerID: computerID, EventKind: computerevent.EventResearchUpdate,
			OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: fmt.Sprintf("scratch-update-%d", i), ActorProfile: "research",
			AuthorityRef: "typed-update", PayloadCommitment: strings.Repeat("0", 64), PrivacyClass: "owner",
			ResultingEffectiveCommitment: commitment, ReducerVersion: 1}
		if _, err := rt.eventAppender.AppendNew(context.Background(), update, computerevent.TransitionInput{TargetStateCommitment: commitment}, nil); err != nil {
			t.Fatalf("append update %d: %v", i, err)
		}
	}
}

func replayScratchRoots(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, replayScratchPrefix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

// The probe's scratch lives on the live store's volume, never in the RAM
// tmpfs /tmp.
func TestReplayScratchLivesBesideTheStoreNotInTmp(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	rt, _, storePath := replayScratchRuntime(t, "computer-scratch-where", 1)
	volume := &fakeScratchVolume{avail: 50 * replayScratchTestGiB}
	rt.replayScratchUsage = volume.usage
	var during []string
	rt.replayProbeAfterCapture = func() {}
	probe, err := rt.openReplayProbe(context.Background(), "computer-scratch-where")
	if err != nil {
		t.Fatalf("probe with room refused: %v", err)
	}
	during = append(during, probe.tempRoot)
	probe.close()
	parent := filepath.Join(filepath.Dir(storePath), replayScratchDirName)
	if len(during) != 1 || filepath.Dir(during[0]) != parent || !strings.HasPrefix(filepath.Base(during[0]), replayScratchPrefix) {
		t.Fatalf("probe root = %v, want a %s* root under %s", during, replayScratchPrefix, parent)
	}
	if roots := replayScratchRoots(t, tmp); len(roots) != 0 {
		t.Fatalf("probe wrote scratch into TMPDIR: %v", roots)
	}
	if roots := replayScratchRoots(t, parent); len(roots) != 0 {
		t.Fatalf("closed probe left its root: %v", roots)
	}
	for _, dir := range volume.readDirs() {
		if dir != parent {
			t.Fatalf("free space read on %s, want the scratch parent %s", dir, parent)
		}
	}
}

// Failure mode 1: a scratch that cannot fit refuses by name before the live
// capture and before any scratch exists.
func TestReplayProbeRefusesScratchItCannotHold(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	rt, _, storePath := replayScratchRuntime(t, "computer-scratch-full", 2)
	volume := &fakeScratchVolume{avail: replayScratchTestGiB} // under the 2 GiB reserve
	rt.replayScratchUsage = volume.usage
	captured := false
	rt.replayProbeAfterCapture = func() { captured = true }

	_, err := rt.ReplayCompleteness(context.Background(), "computer-scratch-full")
	if !errors.Is(err, ErrReplayScratchInsufficient) {
		t.Fatalf("probe on a full volume: %v, want ErrReplayScratchInsufficient", err)
	}
	if captured {
		t.Fatal("the probe captured live state (holding appends) before refusing")
	}
	if volume.callCount() == 0 {
		t.Fatal("the probe never read free space")
	}
	parent := filepath.Join(filepath.Dir(storePath), replayScratchDirName)
	if roots := append(replayScratchRoots(t, parent), replayScratchRoots(t, tmp)...); len(roots) != 0 {
		t.Fatalf("refused probe left scratch: %v", roots)
	}
}

// The estimate counts the base install peak (download plus unpack) and the
// tail; free space that covers the reserve but not the base refuses.
func TestReplayScratchEstimateCountsTheBaseInstallPeak(t *testing.T) {
	rt, _, _ := replayScratchRuntime(t, "computer-scratch-estimate", 3)
	descriptor := rt.restoreBaseSource.(*restoreBaseFake).descriptor
	need, err := rt.replayScratchEstimate(context.Background(), "computer-scratch-estimate")
	if err != nil {
		t.Fatal(err)
	}
	want := 2*uint64(descriptor.BlobSizeBytes) + 3*replayScratchPerEventBytes
	if need != want {
		t.Fatalf("estimate = %d, want 2 x blob (%d) + 3 tail events x %d = %d", need, descriptor.BlobSizeBytes, replayScratchPerEventBytes, want)
	}
	volume := &fakeScratchVolume{avail: replayScratchReserveBytes + need - 1}
	rt.replayScratchUsage = volume.usage
	if _, err := rt.ReplayCompleteness(context.Background(), "computer-scratch-estimate"); !errors.Is(err, ErrReplayScratchInsufficient) {
		t.Fatalf("one byte short of estimate plus reserve: %v, want ErrReplayScratchInsufficient", err)
	}
	volume.set(replayScratchReserveBytes + need)
	if _, err := rt.ReplayCompleteness(context.Background(), "computer-scratch-estimate"); err != nil {
		t.Fatalf("exactly estimate plus reserve refused: %v", err)
	}
}

// Failure mode 2: the volume runs out while the probe replays. The probe
// stops by name and removes its scratch; the runtime lives.
func TestReplayProbeStopsWhenScratchRunsOutInFlight(t *testing.T) {
	rt, cas, storePath := replayScratchRuntime(t, "computer-scratch-inflight", 4)
	volume := &fakeScratchVolume{avail: 50 * replayScratchTestGiB}
	rt.replayScratchUsage = volume.usage
	rt.replayScratchPoll = time.Millisecond
	cas.onEvents = func() {
		// The replay has started: the live store eats the volume.
		volume.set(replayScratchTestGiB / 2)
		seen := volume.callCount()
		deadline := time.Now().Add(5 * time.Second)
		for volume.callCount() < seen+2 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
	}
	_, err := rt.ReplayCompleteness(context.Background(), "computer-scratch-inflight")
	if !errors.Is(err, ErrReplayScratchInsufficient) {
		t.Fatalf("probe that ran out of scratch in flight: %v, want ErrReplayScratchInsufficient", err)
	}
	parent := filepath.Join(filepath.Dir(storePath), replayScratchDirName)
	if roots := replayScratchRoots(t, parent); len(roots) != 0 {
		t.Fatalf("stopped probe left scratch: %v", roots)
	}
	// The live store still serves.
	if head, err := rt.store.Head(context.Background(), "computer-scratch-inflight"); err != nil || head == nil {
		t.Fatalf("live store after the stopped probe: %v %+v", err, head)
	}
}

// Failure mode 3: roots a dead probe left, in the new and the old location,
// are removed when the runtime starts; nothing else is touched.
func TestStaleReplayScratchIsRemovedAtStart(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("RUNTIME_MAINTENANCE_HOLD", "1") // Start returns after its first phases
	storeDir := t.TempDir()
	storePath := filepath.Join(storeDir, "state")
	parent := filepath.Join(storeDir, replayScratchDirName)
	stale := []string{
		filepath.Join(parent, replayScratchPrefix+"111"),
		filepath.Join(tmp, replayScratchPrefix+"786065405"),
	}
	keep := []string{
		filepath.Join(parent, "unrelated"),
		filepath.Join(tmp, "choir-overlay-probe-1"),
		storePath,
	}
	for _, dir := range append(append([]string{}, stale...), keep...) {
		if err := os.MkdirAll(filepath.Join(dir, "runtime.texture", "texture", ".dolt", "noms"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "runtime.texture", "texture", ".dolt", "noms", "journal.idx"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rt := &Runtime{cfg: provideriface.Config{ComputerID: "computer-scratch-boot", StorePath: storePath}}
	rt.Start(context.Background())
	for _, dir := range stale {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("stale probe root %s survived Start: %v", dir, err)
		}
	}
	for _, dir := range keep {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("Start removed %s, which is not probe scratch: %v", dir, err)
		}
	}
}

// Failure mode 5: the probe's base-plus-tail replay gives the witness a full
// genesis replay gives on the same chain.
func TestReplayProbeBaseTailWitnessEqualsGenesisWitness(t *testing.T) {
	ctx := context.Background()
	const computerID = "computer-scratch-witness"
	rt, cas, _ := replayScratchRuntime(t, computerID, 5)

	fromBase, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatalf("base probe: %v", err)
	}
	if fromBase.BaseSequence != 1 || fromBase.TailEventsApplied != 5 {
		t.Fatalf("base probe replayed base=%d tail=%d, want base 1 and a 5-event tail", fromBase.BaseSequence, fromBase.TailEventsApplied)
	}
	rt.restoreBaseSource = restoreBaseFakeFor(rt.restoreBaseSource.(*restoreBaseFake).descriptor, nil, cas.events)
	rt.restoreBaseSource.(*restoreBaseFake).emptyWatermark = true
	fromGenesis, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatalf("genesis probe: %v", err)
	}
	if fromGenesis.BaseSequence != 0 {
		t.Fatalf("genesis probe used a base: %d", fromGenesis.BaseSequence)
	}
	baseWitness, err := selfdevprotocol.WitnessFromObservationSets(fromBase.Live, fromBase.Replay, fromBase.Result)
	if err != nil {
		t.Fatalf("base witness: %v", err)
	}
	genesisWitness, err := selfdevprotocol.WitnessFromObservationSets(fromGenesis.Live, fromGenesis.Replay, fromGenesis.Result)
	if err != nil {
		t.Fatalf("genesis witness: %v", err)
	}
	if !reflect.DeepEqual(baseWitness, genesisWitness) {
		t.Fatalf("base-plus-tail witness differs from the genesis witness:\nbase    %+v\ngenesis %+v", baseWitness, genesisWitness)
	}
}
