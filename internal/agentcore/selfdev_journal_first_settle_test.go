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
	// The fence comes only after a won row CAS: the live apply is untouched.
	if _, found, _ := updater.ReadJournalOutcome(fx.updaterRoot, "selfdev-apply-"+accepted.DecisionEvent); found {
		t.Fatal("a settle that lost the row fenced the live operation's apply")
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	got := fx.waitForState(t, accepted.OperationID, selfdev.StateApplied)
	if got.State != selfdev.StateApplied {
		t.Fatalf("the live operation = %s; want it applied", got.State)
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
	unlockMaterialize := holdForTest(&fx.rt.selfdevMaterializeMu)
	row, err := fx.operations.Get(ctx, fx.computerID, rollback.OperationID)
	if err == nil && row.State == selfdev.StateRollbackPending {
		err = fx.rt.closeSelfDevelopmentSettlementDegraded(ctx, row, true)
	}
	unlockMaterialize()
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

// writeDeadJournal leaves the journal an apply that died mid-flight writes:
// phase past the swap, prior and target named, no outcome.
func (fx *derivableSelfDevFixture) writeDeadJournal(t *testing.T, key, phase, prior, target string) {
	t.Helper()
	sum := sha256.Sum256([]byte(key))
	body := `{"phase":"` + phase + `","request_commitment":"x","prior_release_digest":"` + prior + `","target_release_digest":"` + target + `","started_at":"2026-11-02T00:00:00Z","result":{}}`
	if err := os.WriteFile(filepath.Join(fx.updaterRoot, "operations", hex.EncodeToString(sum[:])+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// materializingWithDeadJournal is an operation the materializer moved to
// materializing whose apply died past its swap: current is whatever the test
// makes it, the bundle and inputs are irrelevant (the rule needs none).
func (fx *derivableSelfDevFixture) materializingWithDeadJournal(t *testing.T, key, phase string, prior, target string) selfdev.Operation {
	t.Helper()
	_, accepted := fx.acceptedWithoutApply(t, key)
	operation, err := fx.operations.Transition(context.Background(), fx.computerID, accepted.OperationID, selfdev.StateAccepted, selfdev.StateMaterializing, nil)
	if err != nil {
		t.Fatal(err)
	}
	fx.writeDeadJournal(t, "selfdev-apply-"+accepted.DecisionEvent, phase, prior, target)
	return operation
}

// The one rule, dead journal, current == prior: nothing installed. Abandon
// (the owner act of a held operation: attempts spent, inputs gone) fences it
// and settles failed, whatever the journal's phase.
func TestAbandonOfADeadJournalPastItsSwapWithThePriorCurrentSettlesFailed(t *testing.T) {
	fx := joinFixture(t, "arbitrate-prior", joinTestBaseA, joinTestBaseA)
	fx.applyPlain(t, "op-arbitrate-prior-1")
	ctx := context.Background()
	prior := fx.currentManifest(t).ContentDigest
	operation := fx.materializingWithDeadJournal(t, "arb-prior", "restart_requested", prior, strings.Repeat("b", 64))
	rec := postAbandon(fx, abandonRequestBody(operation.OperationID, "attempts spent", "abandon-arb-prior"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(ctx, fx.computerID, operation.OperationID)
	if got.State != selfdev.StateFailed || got.TerminalError != selfDevelopmentAbandonTerminalError("attempts spent") {
		t.Fatalf("operation = %s (%q)", got.State, got.TerminalError)
	}
	if journaled, _, _ := updater.ReadJournalOutcome(fx.updaterRoot, "selfdev-apply-"+operation.DecisionEvent); !journaled.Terminal || journaled.Result.Outcome != "refused" {
		t.Fatalf("journal = %+v; want the fence", journaled)
	}
	if p := fx.pendingTransition(t); p != "" {
		t.Fatalf("pending %s still bound", p)
	}
	requireReducerReplay(t, fx)
}

// Dead journal, current == target: installed. A crash boot closes it forward
// from the current pointer, never resuming the apply and never failing it.
func TestCrashBootClosesADeadJournalWhoseTargetIsCurrentForward(t *testing.T) {
	fx := joinFixture(t, "arbitrate-target", joinTestBaseA, joinTestBaseA)
	fx.applyPlain(t, "op-arbitrate-target-1")
	ctx := context.Background()
	current := fx.currentManifest(t).ContentDigest
	operation := fx.materializingWithDeadJournal(t, "arb-target", "pointer_swapped", strings.Repeat("a", 64), current)
	calls := fx.applyCalls.Load()
	fx.bootAfterCrash(t)
	got, _ := fx.operations.Get(ctx, fx.computerID, operation.OperationID)
	if got.State != selfdev.StateApplied || !strings.Contains(got.TerminalError, "current pointer") || got.ReleaseDigest != current {
		t.Fatalf("operation = %s release=%s (%q); want applied from the current pointer", got.State, got.ReleaseDigest, got.TerminalError)
	}
	if fx.applyCalls.Load() != calls {
		t.Fatal("the crash boot called the updater")
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 2 || kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("events = %v; want the first apply and this one recorded applied", kinds)
	}
	if p := fx.pendingTransition(t); p != "" {
		t.Fatalf("pending %s still bound", p)
	}
	requireReducerReplay(t, fx)
}

// Dead journal, current is neither: failed, terminal, the unknown named.
func TestAbandonOfADeadJournalWithAnUnknownCurrentSettlesFailedNamingIt(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-arbitrate-unknown")
	ctx := context.Background()
	operation := fx.materializingWithDeadJournal(t, "arb-unknown", "recovering", strings.Repeat("a", 64), strings.Repeat("b", 64))
	rec := postAbandon(fx, abandonRequestBody(operation.OperationID, "lost", "abandon-arb-unknown"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(ctx, fx.computerID, operation.OperationID)
	if got.State != selfdev.StateFailed || !strings.Contains(got.TerminalError, "the effective release is unknown") {
		t.Fatalf("operation = %s (%q)", got.State, got.TerminalError)
	}
	if _, found, _ := fx.operations.OpenOperation(ctx, fx.computerID); found {
		t.Fatal("still open")
	}
	requireReducerReplay(t, fx)
}

// An older daemon (no key-lock marker): abandon refuses to call a missing
// journal dead, saying why; a terminal journal (d@e's applied rollback) still
// settles.
func TestAnOlderUpdaterDaemonRefusesTheNothingInstalledConclusion(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-old-daemon")
	_, accepted := fx.acceptedWithoutApply(t, "old-daemon")
	if err := os.Remove(filepath.Join(fx.updaterRoot, "operations", ".key-lock-v1")); err != nil {
		t.Fatal(err)
	}
	rec := postAbandon(fx, abandonRequestBody(accepted.OperationID, "x", "abandon-old"))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "predates the per-key lock") {
		t.Fatalf("status=%d body=%s; want 409 naming the old daemon", rec.Code, rec.Body.String())
	}
}

func TestAnOlderUpdaterDaemonStillSettlesDaeFromItsTerminalJournal(t *testing.T) {
	fx, rollback := degradedRollbackProductionShape(t, "old-daemon-dae", removePinnedRelease)
	if err := os.Remove(filepath.Join(fx.updaterRoot, "operations", ".key-lock-v1")); err != nil {
		t.Fatal(err)
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	if got, _ := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID); got.State != selfdev.StateRolledBack {
		t.Fatalf("operation = %s (%q)", got.State, got.TerminalError)
	}
	requireReducerReplay(t, fx)
}

// A crash-held apply the updater is still running arms a retry wake, so the
// terminal journal it writes is read without waiting for another commit.
func TestCrashHeldLiveApplyArmsARetry(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-crash-live")
	_, accepted := fx.acceptedWithoutApply(t, "crash-live")
	if _, err := fx.operations.Transition(context.Background(), fx.computerID, accepted.OperationID, selfdev.StateAccepted, selfdev.StateMaterializing, nil); err != nil {
		t.Fatal(err)
	}
	release := holdApplyKey(t, fx.updaterRoot, "selfdev-apply-"+accepted.DecisionEvent)
	defer release()
	var armed []string
	fx.rt.scheduleActor = func(_ context.Context, _, _, _, kind, target, _, _ string, _ time.Time) error {
		armed = append(armed, kind+":"+target)
		return nil
	}
	fx.bootAfterCrash(t)
	if len(armed) != 1 || armed[0] != selfdevMaterializationRetryKind+":"+accepted.OperationID {
		t.Fatalf("retry wakes armed = %v", armed)
	}
	if got, _ := fx.operations.Get(context.Background(), fx.computerID, accepted.OperationID); got.State != selfdev.StateMaterializing {
		t.Fatalf("a live apply was settled: %s", got.State)
	}
}

// A legacy failed row whose release is installed (its dead journal's target is
// current) and whose pending is still bound: the sweep records it forward (the
// applied event), never as failed.
func TestSweepRecordsAnInstalledOrphanForward(t *testing.T) {
	fx := joinFixture(t, "orphan-installed", joinTestBaseA, joinTestBaseA)
	fx.applyPlain(t, "op-orphan-installed-1")
	current := fx.currentManifest(t).ContentDigest
	operation := fx.materializingWithDeadJournal(t, "orphan-installed", "restart_requested", strings.Repeat("a", 64), current)
	if pending := fx.pendingTransition(t); pending != operation.DecisionEvent {
		t.Fatalf("fixture: pending %q, decision %q", pending, operation.DecisionEvent)
	}
	if _, err := fx.store.DB().Exec(`UPDATE self_development_operations SET state = ? WHERE operation_id = ?`, selfdev.StateFailed, operation.OperationID); err != nil {
		t.Fatal(err)
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	if p := fx.pendingTransition(t); p != "" {
		t.Fatalf("pending %s still bound", p)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 2 || kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("events = %v", kinds)
	}
	requireReducerReplay(t, fx)
}
