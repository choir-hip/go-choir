package agentcore

// The crash-boot settlement of a self-development apply, bounded and without
// a replay (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.5
// and 4.6 slice 1; docs/problems/apply-checkpoint-replay-fills-guest-tmp-2026-10-26.md,
// the 12:3x addendum). b@c's boot settled an apply whose journal was terminal
// `applied`, and the settlement bound a checkpoint by replaying the tape into
// /tmp until the runtime died, on every boot. Written before the code. Ways
// this could fail:
//
//  1. the settlement still replays to bind: it admits a scratch, installs a
//     base, and on a full volume holds the apply instead of recording it;
//  2. the settlement records the apply more than once (two checkpoints);
//  3. a settlement that crashed the computer twice runs a third time: the
//     crash loop the restart rule forbids;
//  4. a settlement that did not crash is counted as if it had (every reconcile
//     pass in one boot spends an attempt), so a transient failure closes the
//     operation;
//  5. closing the operation erases the apply: the materialization-applied
//     event must stay on the tape, no checkpoint is bound, the route is not
//     promoted, and nothing else is applied.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// settlementAttemptKey is the durable attempt key the settlement spends
// under: the reason and the operation (planned_restart.go).
func settlementAttemptKey(operationID string) string {
	return "self_development_settlement " + operationID
}

func (fx *derivableSelfDevFixture) settlementAttempts(t *testing.T, operationID string) int {
	t.Helper()
	raw, err := os.ReadFile(GuestApplyAttemptsPath(fx.rt.cfg.StorePath))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	if err := json.Unmarshal(raw, &counts); err != nil {
		t.Fatal(err)
	}
	return counts[settlementAttemptKey(operationID)]
}

// parkAppliedButUnrecorded is the state a crash leaves: the updater applied
// the release and journaled it terminal `applied`, recording it failed (the
// control plane refused the checkpoint), so the operation is parked
// materializing with a `materialization-applied` event or none. No further
// commit triggers a pass. The caller then simulates boots.
func (fx *derivableSelfDevFixture) parkAppliedButUnrecorded(t *testing.T, key string) selfdev.Operation {
	t.Helper()
	fx.controlFail.Store(true)
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, key)
	fx.appendDecision(t, operation, bundleDigest, "decision-"+key)
	fx.waitForState(t, operation.OperationID, selfdev.StateMaterializing)
	applyKey := "selfdev-apply-" + fx.decisionDigest(t, key)
	deadline := time.Now().Add(30 * time.Second)
	for {
		journaled, found, err := updater.ReadJournalOutcome(fx.updaterRoot, applyKey)
		if err == nil && found && journaled.Terminal {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("apply journal never became terminal (found=%v err=%v)", found, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	// The guest dies here: no commit triggers a pass any more. Wait until the
	// passes already queued have drained and the updater call count is stable.
	fx.rt.eventAppender.SetPostCommitObserver(func(computerevent.EventKind) {})
	for stable, last := 0, int32(-1); stable < 3; {
		fx.rt.selfdevMaterializeMu.Lock()
		calls := fx.applyCalls.Load()
		fx.rt.selfdevMaterializeMu.Unlock()
		if calls == last {
			stable++
		} else {
			stable, last = 0, calls
		}
		time.Sleep(200 * time.Millisecond)
	}
	fx.rt.cfg.StorePath = filepath.Join(t.TempDir(), "runtime.db")
	return operation
}

// bootAfterCrash is one crash boot: the runtime marks the decided operations
// interrupted, then the reconciler runs.
func (fx *derivableSelfDevFixture) bootAfterCrash(t *testing.T) {
	t.Helper()
	fx.rt.closeSelfDevelopmentOperationsAfterCrash(context.Background())
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
}

// Failure modes 1 and 2 (the 12:3x path): a crash-boot settlement of an
// operation whose journal is terminal `applied` records it with one bind and
// no replay, on a volume that has no room for a replay.
func TestCrashBootSettlementRecordsAppliedJournalWithOneBindAndNoReplay(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-no-replay")
	// After the fixture: its unix sockets need a short TMPDIR.
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	operation := fx.parkAppliedButUnrecorded(t, "op-settle-no-replay")
	if got := len(fx.checkpoints); got != 0 {
		t.Fatalf("%d checkpoints bound while the control plane was down", got)
	}
	volume := &fakeScratchVolume{avail: replayScratchTestGiB} // no room for any replay
	fx.rt.replayScratchUsage = volume.usage
	probed := false
	fx.rt.replayProbeAfterCapture = func() { probed = true }
	fx.controlFail.Store(false)
	callsBefore := fx.applyCalls.Load()

	fx.bootAfterCrash(t)

	applied := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if !strings.HasPrefix(applied.CheckpointRef, "checkpoint:sha256:") || applied.RouteReceipt == "" {
		t.Fatalf("operation applied without its checkpoint and route: %+v", applied)
	}
	if probed || volume.callCount() != 0 {
		t.Fatalf("the settlement replayed to bind (probe=%v, volume reads=%d)", probed, volume.callCount())
	}
	if roots := replayScratchRoots(t, tmp); len(roots) != 0 {
		t.Fatalf("the settlement left scratch in TMPDIR: %v", roots)
	}
	if got := len(fx.checkpoints); got != 1 {
		t.Fatalf("%d checkpoints bound, want exactly one", got)
	}
	if calls := fx.applyCalls.Load(); calls != callsBefore {
		t.Fatalf("the settlement called the updater (%d -> %d)", callsBefore, calls)
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventMaterializationApplied] != 1 || kinds[computerevent.EventCheckpointPublished] != 1 {
		t.Fatalf("events = %v; want one applied and one checkpoint_published", kinds)
	}
	if n := fx.settlementAttempts(t, operation.OperationID); n != 1 {
		t.Fatalf("settlement attempts = %d, want the one this boot spent", n)
	}
}

// Failure mode 3 and 5: a settlement that crashed the computer twice closes
// the operation `degraded` instead of running a third time. Each failed
// settlement below leaves the footprint a crash leaves: an attempt spent, no
// terminal record.
func TestCrashBootSettlementThatCrashedTwiceClosesDegraded(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-crashed-twice")
	operation := fx.parkAppliedButUnrecorded(t, "op-settle-twice")
	callsBefore := fx.applyCalls.Load()

	fx.bootAfterCrash(t) // boot 1: the settlement starts and does not finish
	if n := fx.settlementAttempts(t, operation.OperationID); n != 1 {
		t.Fatalf("after boot 1 attempts = %d, want 1", n)
	}
	fx.bootAfterCrash(t) // boot 2
	if n := fx.settlementAttempts(t, operation.OperationID); n != 2 {
		t.Fatalf("after boot 2 attempts = %d, want 2", n)
	}
	current, err := fx.operations.Get(context.Background(), fx.computerID, operation.OperationID)
	if err != nil || current.State != selfdev.StateMaterializing {
		t.Fatalf("operation after two failed settlements = %+v (%v), want it still materializing", current, err)
	}

	fx.controlFail.Store(false) // boot 3 could now finish, and must not try
	fx.bootAfterCrash(t)

	got, err := fx.operations.Get(context.Background(), fx.computerID, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != selfdev.StateDegraded {
		t.Fatalf("operation state = %s, want %s after the settlement crashed twice", got.State, selfdev.StateDegraded)
	}
	if !strings.Contains(got.TerminalError, "crashed the computer twice") {
		t.Fatalf("terminal error %q does not name the cause", got.TerminalError)
	}
	if n := fx.settlementAttempts(t, operation.OperationID); n != 2 {
		t.Fatalf("attempts = %d after the close, want the two crashes only", n)
	}
	if len(fx.checkpoints) != 0 || got.CheckpointRef != "" || got.RouteReceipt != "" {
		t.Fatalf("the closed settlement bound or promoted: checkpoints=%d ref=%q route=%q", len(fx.checkpoints), got.CheckpointRef, got.RouteReceipt)
	}
	if calls := fx.applyCalls.Load(); calls != callsBefore {
		t.Fatalf("the settlement called the updater (%d -> %d)", callsBefore, calls)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 1 {
		t.Fatalf("events = %v; the applied event must stay on the tape", kinds)
	}

	// Later passes and later crash boots leave the closed operation closed.
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	fx.bootAfterCrash(t)
	again, err := fx.operations.Get(context.Background(), fx.computerID, operation.OperationID)
	if err != nil || again.State != selfdev.StateDegraded || again.TerminalError != got.TerminalError {
		t.Fatalf("closed operation changed: %+v (%v)", again, err)
	}
	if len(fx.checkpoints) != 0 {
		t.Fatalf("a later boot bound %d checkpoints for the closed operation", len(fx.checkpoints))
	}
}

// Failure mode 4: the attempt is spent once per boot. Every reconcile pass in
// one boot (any commit triggers one) must not spend another, or a transient
// failure would close the operation in one boot.
func TestCrashBootSettlementSpendsOneAttemptPerBoot(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-one-per-boot")
	operation := fx.parkAppliedButUnrecorded(t, "op-settle-per-boot")

	fx.bootAfterCrash(t)
	for i := 0; i < 4; i++ {
		fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	}

	if n := fx.settlementAttempts(t, operation.OperationID); n != 1 {
		t.Fatalf("attempts = %d after five passes in one boot, want 1", n)
	}
	current, err := fx.operations.Get(context.Background(), fx.computerID, operation.OperationID)
	if err != nil || current.State != selfdev.StateMaterializing {
		t.Fatalf("operation after five passes = %+v (%v), want it still materializing", current, err)
	}
}
