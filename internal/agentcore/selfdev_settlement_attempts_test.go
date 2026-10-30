package agentcore

// The crash-boot settlement of a self-development apply, bounded and without
// a replay (docs/archive/design/apply-checkpoint-without-replay-2026-10-26.md, 4.5
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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	if !strings.Contains(got.TerminalError, "did not finish on two boots") || strings.Contains(got.TerminalError, "crashed the computer") {
		t.Fatalf("terminal error %q does not name the cause", got.TerminalError)
	}
	if n := fx.settlementAttempts(t, operation.OperationID); n != 2 {
		t.Fatalf("attempts = %d after the close, want the two crashes only", n)
	}
	// The cause says the apply is recorded on the tape, so it must be, and the
	// applied event must have closed the pending transition: a degraded close
	// that leaves it bound refuses every later update.
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound after the degraded close", pending)
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

// Review 309, finding 5: the bound fails closed. A runtime with no durable
// attempt count cannot bound the settlement, so it refuses to run it: the
// operation stays where it is, nothing is bound, and the updater is not called.
func TestCrashBootSettlementWithoutADurableAttemptCountRefuses(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-unbounded")
	operation := fx.parkAppliedButUnrecorded(t, "op-settle-unbounded")
	fx.rt.cfg.StorePath = "" // no durable count: nothing could bound the settlement
	fx.controlFail.Store(false)
	callsBefore := fx.applyCalls.Load()

	fx.bootAfterCrash(t)
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())

	got, err := fx.operations.Get(context.Background(), fx.computerID, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != selfdev.StateMaterializing || got.CheckpointRef != "" || got.RouteReceipt != "" {
		t.Fatalf("an unbounded settlement ran: %+v", got)
	}
	if len(fx.checkpoints) != 0 {
		t.Fatalf("an unbounded settlement bound %d checkpoints", len(fx.checkpoints))
	}
	if calls := fx.applyCalls.Load(); calls != callsBefore {
		t.Fatalf("the settlement called the updater (%d -> %d)", callsBefore, calls)
	}
}

// Review 309, finding 4: an operation still `accepted` has installed nothing,
// so a settlement that crashed twice settles it `failed` (there is no
// accepted -> degraded edge, and degraded means applied with an unsettled
// tail). A materializing operation has installed something and stays degraded.
func TestSettlementThatCrashedTwiceSettlesAnAcceptedOperationFailed(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-accepted")
	ctx := context.Background()
	operation := fx.decideWithoutActing(t, "op-settle-accepted")
	accepted, found, err := fx.rt.recoverSelfDevelopmentDecision(ctx, operation)
	if err != nil || !found || accepted.State != selfdev.StateAccepted {
		t.Fatalf("recover the decision: %+v found=%v err=%v; want an accepted operation", accepted, found, err)
	}

	if err := fx.rt.closeSelfDevelopmentSettlementDegraded(ctx, accepted, false); err != nil {
		t.Fatalf("close an accepted operation: %v", err)
	}

	got, err := fx.operations.Get(ctx, fx.computerID, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != selfdev.StateFailed {
		t.Fatalf("accepted operation closed as %s, want %s", got.State, selfdev.StateFailed)
	}
	if !strings.Contains(got.TerminalError, "did not finish on two boots") {
		t.Fatalf("terminal error %q does not name the cause", got.TerminalError)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound after the operation failed", pending)
	}
}

// Review 315 (Devin, nit 2): every state before materializing installed
// nothing, so it settles failed, not only `accepted`.
func TestSettlementThatCrashedTwiceSettlesAnOperationStillAwaitingApprovalFailed(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-awaiting")
	ctx := context.Background()
	operation, _, _ := fx.seedOperationToAwaitingApproval(t, "op-settle-awaiting")

	if err := fx.rt.closeSelfDevelopmentSettlementDegraded(ctx, operation, false); err != nil {
		t.Fatalf("close an operation awaiting approval: %v", err)
	}

	got, err := fx.operations.Get(ctx, fx.computerID, operation.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != selfdev.StateFailed || !strings.Contains(got.TerminalError, "did not finish on two boots") {
		t.Fatalf("operation = %s %q, want failed with the cause", got.State, got.TerminalError)
	}
}

// Review 323 (DeepSeek, High): the selfdev settlement spent its attempt before
// the applied event, so two boots on which the store refused the append spent
// the bound; the third boot closed the operation degraded with no applied event,
// the pending transition still bound (every later offer refused) and a cause
// ("the apply is recorded on the tape") that was not true. The applied event is
// recorded first, outside the bound; only the tail is bounded.
func TestCrashBootSettlementRecordsAppliedAfterTwoFailedAppendsWithoutSpendingTheBound(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-append-fails")
	real := appendEventPayload
	defer func() { appendEventPayload = real }()
	failing := true
	appendEventPayload = func(rt *Runtime, ctx context.Context, event computerevent.Event, input computerevent.TransitionInput, payload []byte, mediaType, privacyClass string) (computerevent.Receipt, string, error) {
		if failing && event.EventKind == computerevent.EventMaterializationApplied {
			return computerevent.Receipt{}, "", errors.New("store mid-replay: append unavailable")
		}
		return real(rt, ctx, event, input, payload, mediaType, privacyClass)
	}
	operation := fx.parkAppliedButUnrecorded(t, "op-settle-append-fails")
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("the applied event committed while appends were refused: %v", kinds)
	}
	fx.controlFail.Store(false)

	fx.bootAfterCrash(t)
	fx.bootAfterCrash(t)
	if n := fx.settlementAttempts(t, operation.OperationID); n != 0 {
		t.Fatalf("two failed applied appends spent %d settlement attempts", n)
	}

	failing = false
	fx.bootAfterCrash(t)

	applied := fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if !strings.HasPrefix(applied.CheckpointRef, "checkpoint:sha256:") {
		t.Fatalf("third boot did not finish the settlement: %+v", applied)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound", pending)
	}
	if n := fx.settlementAttempts(t, operation.OperationID); n != 1 {
		t.Fatalf("settlement attempts = %d, want the one tail", n)
	}
}

// writeTerminalRefusedJournal is the journal the updater leaves for a
// pre-mutation refusal (base join, state compat, source trust, staging).
func (fx *derivableSelfDevFixture) writeTerminalRefusedJournal(t *testing.T, key string) {
	t.Helper()
	fx.writeTerminalRefusedJournalWithFailure(t, key, "base commit does not match the booted base")
}

func (fx *derivableSelfDevFixture) writeTerminalRefusedJournalWithFailure(t *testing.T, key, failure string) {
	t.Helper()
	sum := sha256.Sum256([]byte(key))
	dir := filepath.Join(fx.updaterRoot, "operations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	failureField := ""
	if failure != "" {
		failureField = `"failure":"` + failure + `",`
	}
	body := `{"phase":"refused","request_commitment":"x","target_release_digest":"y",` + failureField + `"result":{"outcome":"refused"}}`
	if err := os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Review 323 (Devin, Medium; the selfdev half): a journaled refusal installed
// nothing and has no tail. It settles `failed` and discharges the pending
// transition on a crash boot, spending no attempt, even when the bound for the
// operation is already spent or cannot be kept.
func TestCrashBootSettlementOfAJournaledRefusalFailsEvenWithTheBoundSpent(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(fx *derivableSelfDevFixture, t *testing.T, operationID string)
	}{
		{"bound spent", func(fx *derivableSelfDevFixture, t *testing.T, operationID string) {
			fx.rt.cfg.StorePath = filepath.Join(t.TempDir(), "runtime.db")
			raw, _ := json.Marshal(map[string]int{settlementAttemptKey(operationID): selfdevSettlementAttemptLimit})
			if err := os.WriteFile(GuestApplyAttemptsPath(fx.rt.cfg.StorePath), raw, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"no durable count", func(fx *derivableSelfDevFixture, t *testing.T, operationID string) {
			fx.rt.cfg.StorePath = ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newDerivableSelfDevFixture(t, "computer-settle-refused-"+strings.ReplaceAll(tc.name, " ", "-"))
			operation := fx.decideWithoutActing(t, "op-settle-refused")
			fx.writeTerminalRefusedJournal(t, "selfdev-apply-"+fx.decisionDigest(t, "op-settle-refused"))
			tc.setup(fx, t, operation.OperationID)
			callsBefore := fx.applyCalls.Load()

			fx.bootAfterCrash(t)

			got := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
			if !strings.Contains(got.TerminalError, "base commit does not match") {
				t.Fatalf("terminal error %q does not carry the refusal", got.TerminalError)
			}
			if pending := fx.pendingTransition(t); pending != "" {
				t.Fatalf("pending transition %s still bound after a refused settlement", pending)
			}
			if calls := fx.applyCalls.Load(); calls != callsBefore {
				t.Fatalf("the settlement called the updater (%d -> %d)", callsBefore, calls)
			}
		})
	}
}

// Review 332 (nit 2): a journaled refusal with no reason still settles with a
// named cause, not "...: " with nothing after it.
func TestCrashBootSettlementOfAJournaledRefusalWithNoReasonNamesTheCause(t *testing.T) {
	if got := journalFailureCause("", "refused"); got == "" || !strings.Contains(got, "refused") {
		t.Fatalf("empty failure fell back to %q, want a named cause", got)
	}
	if got := journalFailureCause("  ", "refused"); got == "" {
		t.Fatal("a blank failure fell back to nothing")
	}
	if got := journalFailureCause("base moved", "refused"); got != "base moved" {
		t.Fatalf("a stated failure was replaced: %q", got)
	}

	fx := newDerivableSelfDevFixture(t, "computer-settle-refused-no-reason")
	operation := fx.decideWithoutActing(t, "op-settle-refused-no-reason")
	fx.writeTerminalRefusedJournalWithFailure(t, "selfdev-apply-"+fx.decisionDigest(t, "op-settle-refused-no-reason"), "")
	fx.useDurableAttemptCount(t)

	fx.bootAfterCrash(t)

	got := fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if strings.TrimSpace(got.TerminalError) == "" || strings.HasSuffix(strings.TrimSpace(got.TerminalError), ":") || !strings.Contains(got.TerminalError, "no reason") {
		t.Fatalf("terminal error %q does not name the cause of a refusal with no reason", got.TerminalError)
	}
}

// Review 332 (nit 3): an `applied` journal on an operation that never reached
// materializing is a contradiction (the updater only runs after the
// materializing transition). The settlement records nothing and spends nothing:
// the operation is held, visible, for an owner act, rather than getting an
// applied event for an apply the operation never started.
func TestCrashBootSettlementRefusesAnAppliedJournalOnAnOperationThatNeverMaterialized(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-applied-on-accepted")
	ctx := context.Background()
	operation := fx.decideWithoutActing(t, "op-settle-applied-on-accepted")
	accepted, found, err := fx.rt.recoverSelfDevelopmentDecision(ctx, operation)
	if err != nil || !found || accepted.State != selfdev.StateAccepted {
		t.Fatalf("recover the decision: %+v found=%v err=%v", accepted, found, err)
	}
	key := "selfdev-apply-" + fx.decisionDigest(t, "op-settle-applied-on-accepted")
	sum := sha256.Sum256([]byte(key))
	dir := filepath.Join(fx.updaterRoot, "operations")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := `{"phase":"applied","request_commitment":"x","target_release_digest":"y","result":{"outcome":"applied"}}`
	if err := os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	fx.useDurableAttemptCount(t)

	err = fx.rt.settleSelfDevelopmentOperationAfterCrash(ctx, accepted, false)

	if !errors.Is(err, errSelfDevSettlementStateMismatch) {
		t.Fatalf("settlement of an applied journal on an accepted operation = %v, want errSelfDevSettlementStateMismatch", err)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventMaterializationApplied] != 0 {
		t.Fatalf("an applied event was recorded for an operation that never materialized: %v", kinds)
	}
	if n := fx.settlementAttempts(t, operation.OperationID); n != 0 {
		t.Fatalf("the refused settlement spent %d attempts", n)
	}
	got, err := fx.operations.Get(ctx, fx.computerID, operation.OperationID)
	if err != nil || got.State != selfdev.StateAccepted {
		t.Fatalf("operation = %+v (%v), want it held accepted", got, err)
	}
}
