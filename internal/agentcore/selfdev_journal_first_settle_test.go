package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// docs/problems/selfdev-stale-open-operations-block-engineering-2026-11-02.md,
// "Journal-first settlement": one test per table row the round-3 change
// implements. Failure modes 8-11 are listed in selfdev_stale_ops_settle_test.go.

// holdApplyKey takes the updater's per-key lock as a running Apply would.
func holdApplyKey(t *testing.T, root, key string) func() {
	t.Helper()
	sum := sha256.Sum256([]byte(key))
	f, err := os.OpenFile(filepath.Join(root, "operations", hex.EncodeToString(sum[:])+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }
}

// acceptedWithoutApply is the state a decided operation is in once its row
// moved, with no pass run: accepted, pending bound to its decision, no journal.
func (fx *derivableSelfDevFixture) acceptedWithoutApply(t *testing.T, key string) (stale, accepted selfdev.Operation) {
	t.Helper()
	stale = fx.decideWithoutActing(t, key)
	row, err := fx.operations.Get(context.Background(), fx.computerID, stale.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	accepted, found, err := fx.rt.recoverSelfDevelopmentDecision(context.Background(), row)
	if err != nil || !found || accepted.State != selfdev.StateAccepted {
		t.Fatalf("fixture: recover = %+v found=%v err=%v", accepted, found, err)
	}
	return row, accepted
}

// Row 8 (A), decide wins: a settle working from a row read before the decide
// moved it loses the row CAS and discharges nothing; the decided operation
// then applies normally.
func TestSettleThatLosesTheRowToADecideDischargesNothing(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-lost-to-decide")
	ctx := context.Background()
	stale, accepted := fx.acceptedWithoutApply(t, "lost-to-decide")
	pending := fx.pendingTransition(t)
	if pending == "" || pending != accepted.DecisionEvent {
		t.Fatalf("fixture: pending %q, decision %q", pending, accepted.DecisionEvent)
	}
	_, err := fx.rt.settleSelfDevelopmentOperation(ctx, stale, selfdevSettleSpec{
		Act: selfdevSettleActAbandon, Actor: "owner:owner", Authority: "external-owner:owner", Reason: "raced", Key: "lost", TerminalError: "abandoned by the owner: raced",
	})
	if err == nil {
		t.Fatal("a settle from a stale awaiting_approval row succeeded over an accepted operation")
	}
	if got := fx.pendingTransition(t); got != pending {
		t.Fatalf("pending = %q, want the decision %q still bound", got, pending)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 0 || kinds[computerevent.EventRecoveryRecorded] != 0 {
		t.Fatalf("a lost settle wrote %v", kinds)
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	if got := fx.waitForState(t, accepted.OperationID, selfdev.StateApplied); got.State != selfdev.StateApplied {
		t.Fatalf("the decided operation = %+v", got)
	}
	requireReducerReplay(t, fx)
}

// Row 8 (A), the materializer wins: a settle from a stale accepted row loses
// the row CAS against materializing; pending stays with the live row, which
// reaches a terminal state on the next pass (nothing wedged).
func TestSettleThatLosesTheRowToTheMaterializerLeavesItLive(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-lost-to-materializer")
	ctx := context.Background()
	_, accepted := fx.acceptedWithoutApply(t, "lost-to-materializer")
	if _, err := fx.operations.Transition(ctx, fx.computerID, accepted.OperationID, selfdev.StateAccepted, selfdev.StateMaterializing, nil); err != nil {
		t.Fatal(err)
	}
	pending := fx.pendingTransition(t)
	if _, err := fx.rt.settleSelfDevelopmentOperation(ctx, accepted, selfdevSettleSpec{
		Act: selfdevSettleActAbandon, Actor: "owner:owner", Authority: "external-owner:owner", Reason: "raced", Key: "lost", TerminalError: "abandoned by the owner: raced",
	}); err == nil {
		t.Fatal("a settle from a stale accepted row succeeded over a materializing operation")
	}
	if got := fx.pendingTransition(t); got != pending {
		t.Fatalf("pending = %q, want %q: the live row lost its pending", got, pending)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("a lost settle discharged pending: %v", kinds)
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	got, _ := fx.operations.Get(ctx, fx.computerID, accepted.OperationID)
	if got.State != selfdev.StateFailed && got.State != selfdev.StateApplied {
		t.Fatalf("the live operation = %s; want it terminal after a pass", got.State)
	}
	if p := fx.pendingTransition(t); p != "" {
		t.Fatalf("pending %s bound after the operation reached %s", p, got.State)
	}
	requireReducerReplay(t, fx)
}

// Row 9 (B): an apply the updater is running holds the key, even before it
// journals anything (staging); abandon is refused. Once it lets go with no
// journal, abandon fences the key and settles failed, and the fence answers
// any apply that arrives after.
func TestAbandonOfAStagedButUnjournaledApplyIsRefusedThenFenced(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-abandon-staging")
	ctx := context.Background()
	_, accepted := fx.acceptedWithoutApply(t, "staging")
	// The materializer moved the row and called the updater; the client gave
	// up after two minutes while the updater is still staging.
	if _, err := fx.operations.Transition(ctx, fx.computerID, accepted.OperationID, selfdev.StateAccepted, selfdev.StateMaterializing, nil); err != nil {
		t.Fatal(err)
	}
	key := "selfdev-apply-" + accepted.DecisionEvent
	release := holdApplyKey(t, fx.updaterRoot, key)
	rec := postAbandon(fx, abandonRequestBody(accepted.OperationID, "stuck", "abandon-staging"))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "in flight") {
		release()
		t.Fatalf("abandon under a staging apply status=%d body=%s; want 409 in flight", rec.Code, rec.Body.String())
	}
	if _, found, _ := updater.ReadJournalOutcome(fx.updaterRoot, key); found {
		release()
		t.Fatal("a fence was written under a running apply")
	}
	release()

	rec = postAbandon(fx, abandonRequestBody(accepted.OperationID, "stuck", "abandon-staging"))
	if rec.Code != http.StatusOK {
		t.Fatalf("abandon after the apply let go status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(ctx, fx.computerID, accepted.OperationID)
	if got.State != selfdev.StateFailed {
		t.Fatalf("operation = %+v", got)
	}
	journaled, found, err := updater.ReadJournalOutcome(fx.updaterRoot, key)
	if err != nil || !found || !journaled.Terminal || journaled.Result.Outcome != "refused" {
		t.Fatalf("journal after abandon = %+v found=%v err=%v; want the terminal fence", journaled, found, err)
	}
	if p := fx.pendingTransition(t); p != "" {
		t.Fatalf("pending %s still bound", p)
	}
	requireReducerReplay(t, fx)
}

// Row 10 (C), d@e's production shape: a rollback whose apply journaled
// applied and recorded rollback_applied (pending empty, desired != effective),
// whose tail failed and closed it degraded, and whose pinned target release
// is gone. The journal is read first, so the tail runs over the release the
// rollback installed and the operation lands rolled_back, with its checkpoint.
func TestDegradedRollbackWithAppliedJournalAndGonePinnedReleaseRecordsForward(t *testing.T) {
	fx, first := rollbackFixture(t, "dae")
	ctx := context.Background()
	fx.controlFail.Store(true)
	rollback := fx.startRollbackTo(t, first, "rollback-dae")
	key := "selfdev-rollback-apply-" + rollback.DecisionEvent
	deadline := time.Now().Add(30 * time.Second)
	for {
		journaled, found, _ := updater.ReadJournalOutcome(fx.updaterRoot, key)
		if found && journaled.Terminal && fx.countEventKinds()[computerevent.EventRollbackApplied] == 1 {
			if journaled.Result.Outcome != "applied" {
				t.Fatalf("rollback journal = %+v", journaled)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the rollback never journaled applied with its event recorded")
		}
		time.Sleep(50 * time.Millisecond)
	}
	fx.rt.eventAppender.SetPostCommitObserver(func(computerevent.EventKind) {})
	fx.rt.selfdevMaterializeMu.Lock()
	row, err := fx.operations.Get(ctx, fx.computerID, rollback.OperationID)
	if err == nil && row.State == selfdev.StateRollbackPending {
		err = fx.rt.closeSelfDevelopmentSettlementDegraded(ctx, row, true)
	}
	fx.rt.selfdevMaterializeMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	row, _ = fx.operations.Get(ctx, fx.computerID, rollback.OperationID)
	head, _ := fx.store.Head(ctx, fx.computerID)
	if row.State != selfdev.StateDegraded || row.ReleaseDigest != first.ReleaseDigest || head.PendingTransitionRef != "" || head.DesiredEventHead == head.EffectiveEventHead {
		t.Fatalf("fixture is not d@e's shape: row %s/%s head %+v", row.State, row.ReleaseDigest, head)
	}
	journaled, _, _ := updater.ReadJournalOutcome(fx.updaterRoot, key)
	if journaled.Result.ReleaseDigest == first.ReleaseDigest {
		t.Fatalf("fixture: the rollback installed the pinned digest itself")
	}
	removePinnedRelease(t, filepath.Join(fx.updaterRoot, "releases", first.ReleaseDigest))
	fx.controlFail.Store(false)

	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)

	got, _ := fx.operations.Get(ctx, fx.computerID, rollback.OperationID)
	if got.State != selfdev.StateRolledBack || got.CheckpointRef == "" || got.ReleaseDigest != journaled.Result.ReleaseDigest {
		t.Fatalf("operation = %s checkpoint=%q release=%s (%q); want rolled_back with its tail", got.State, got.CheckpointRef, got.ReleaseDigest, got.TerminalError)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRollbackApplied] != 1 || kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("events = %v", kinds)
	}
	if _, found, _ := fx.operations.OpenOperation(ctx, fx.computerID); found {
		t.Fatal("the rollback still holds the computer")
	}
	requireReducerReplay(t, fx)
}

// Row 10 (C), the tail cannot run and the owner closes it: the same shape with
// the control plane still refusing; the materializer retries, abandon settles
// forward from the journal.
func TestAbandonOfADegradedRollbackWithAStuckTailSettlesForward(t *testing.T) {
	fx, first := rollbackFixture(t, "dae-stuck")
	ctx := context.Background()
	fx.controlFail.Store(true)
	rollback := fx.startRollbackTo(t, first, "rollback-dae-stuck")
	deadline := time.Now().Add(30 * time.Second)
	for fx.countEventKinds()[computerevent.EventRollbackApplied] != 1 {
		if time.Now().After(deadline) {
			t.Fatal("the rollback never recorded rollback_applied")
		}
		time.Sleep(50 * time.Millisecond)
	}
	removePinnedRelease(t, filepath.Join(fx.updaterRoot, "releases", first.ReleaseDigest))
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	if got, _ := fx.operations.Get(ctx, fx.computerID, rollback.OperationID); got.State != selfdev.StateRollbackPending {
		t.Fatalf("with the tail refused the rollback is %s; want it retried, still rollback_pending", got.State)
	}
	rec := postAbandon(fx, abandonRequestBody(rollback.OperationID, "control plane will not take it", "abandon-dae-stuck"))
	if rec.Code != http.StatusOK {
		t.Fatalf("abandon status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(ctx, fx.computerID, rollback.OperationID)
	if got.State != selfdev.StateRolledBack || got.TerminalError != selfDevelopmentAbandonForwardNote("control plane will not take it") {
		t.Fatalf("operation = %s (%q)", got.State, got.TerminalError)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("a settle-forward discharged as failed: %v", kinds)
	}
	requireReducerReplay(t, fx)
}

// Row 11: a decision that lands after a settle moved the row leaves pending
// bound to a failed operation; the reconciler's sweep discharges it.
func TestOrphanedPendingOfAFailedOperationIsDischargedByTheSweep(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-orphan-pending")
	ctx := context.Background()
	operation, bundleDigest, _ := fx.seedOperationToAwaitingApproval(t, "orphan")
	fx.rt.eventAppender.SetPostCommitObserver(func(computerevent.EventKind) {})
	if _, err := fx.operations.Transition(ctx, fx.computerID, operation.OperationID, selfdev.StateAwaitingApproval, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.TerminalError = "abandoned by the owner: orphan"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	fx.appendDecision(t, operation, bundleDigest, "decision-orphan")
	if fx.pendingTransition(t) == "" {
		t.Fatal("fixture: the late decision bound no pending")
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	if p := fx.pendingTransition(t); p != "" {
		t.Fatalf("pending %s still bound to a failed operation", p)
	}
	head, _ := fx.store.Head(ctx, fx.computerID)
	if head.DesiredEventHead != head.EffectiveEventHead {
		t.Fatalf("desired %s did not return to effective %s", head.DesiredEventHead, head.EffectiveEventHead)
	}
	requireReducerReplay(t, fx)
}

// Dead past the swap on a crash boot: held, and the owner's abandon hands it
// back to the materializer for the updater's resume (202), once.
func TestAbandonReadmitsACrashInterruptedApplyDeadPastItsSwap(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-abandon-interrupted")
	ctx := context.Background()
	_, accepted := fx.acceptedWithoutApply(t, "interrupted")
	if _, err := fx.operations.Transition(ctx, fx.computerID, accepted.OperationID, selfdev.StateAccepted, selfdev.StateMaterializing, nil); err != nil {
		t.Fatal(err)
	}
	key := "selfdev-apply-" + accepted.DecisionEvent
	sum := sha256.Sum256([]byte(key))
	if err := os.WriteFile(filepath.Join(fx.updaterRoot, "operations", hex.EncodeToString(sum[:])+".json"),
		[]byte(`{"phase":"pointer_swapped","request_commitment":"x","target_release_digest":"y","result":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fx.bootAfterCrash(t)
	if got, _ := fx.operations.Get(ctx, fx.computerID, accepted.OperationID); got.State != selfdev.StateMaterializing || !fx.rt.selfdevCrashInterrupted[accepted.OperationID] {
		t.Fatalf("after the crash boot the operation is %s (interrupted=%v); want held", got.State, fx.rt.selfdevCrashInterrupted[accepted.OperationID])
	}
	rec := postAbandon(fx, abandonRequestBody(accepted.OperationID, "finish it", "abandon-interrupted"))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("abandon of an interrupted apply status=%d body=%s; want 202", rec.Code, rec.Body.String())
	}
	fx.rt.selfdevMaterializeMu.Lock()
	still := fx.rt.selfdevCrashInterrupted[accepted.OperationID]
	fx.rt.selfdevMaterializeMu.Unlock()
	if still {
		t.Fatal("the operation is still held as crash-interrupted")
	}
	// The resume reaches a terminal journal (here the updater refuses the
	// mismatched commitment) and the operation settles from it.
	got := fx.waitForState(t, accepted.OperationID, selfdev.StateFailed, selfdev.StateApplied)
	// The row moves first and the discharge follows it in the same pass.
	fx.rt.selfdevMaterializeMu.Lock()
	pending := fx.pendingTransition(t)
	fx.rt.selfdevMaterializeMu.Unlock()
	if pending != "" {
		t.Fatalf("pending %s bound after the operation reached %s", pending, got.State)
	}
	requireReducerReplay(t, fx)
}
