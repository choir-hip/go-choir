package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// docs/problems/selfdev-stale-open-operations-block-engineering-2026-11-02.md.
// Ways the changes can fail, written before the code:
//  1. a degraded or rollback_pending rollback whose pinned release is gone is
//     retried forever, or settles with no record on the tape;
//  2. a transient read error settles the operation, losing a rollback that
//     would have worked; a corrupt release is mislabeled as retention;
//  3. the settled operation leaves the head pending, so a later operation is
//     refused or binds to a head that is not live;
//  4. abandon refuses the in-flight states that have no other terminal path,
//     or closes one whose apply is applied or mid-flight;
//  5. abandon closes with no owner decision on the tape, or twice;
//  6. a decision landing during a settle leaves pending bound to a failed
//     operation;
//  7. the engineering admission hold stays console-only.

func removePinnedRelease(t *testing.T, releaseDir string) {
	t.Helper()
	if err := os.Rename(releaseDir, releaseDir+".gone"); err != nil {
		t.Fatal(err)
	}
}

func unreadablePinnedManifest(t *testing.T, releaseDir string) {
	t.Helper()
	manifest := filepath.Join(releaseDir, "release-manifest.json")
	if err := os.Chmod(manifest, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(manifest, 0o444) })
}

func corruptPinnedManifest(t *testing.T, releaseDir string) {
	t.Helper()
	manifest := filepath.Join(releaseDir, "release-manifest.json")
	if err := os.Chmod(manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("not a manifest"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// requireReducerReplay replays the tape's events through the reducer and
// requires every recorded next head, and the store's head, to match.
func requireReducerReplay(t *testing.T, fx *derivableSelfDevFixture) {
	t.Helper()
	var current *computerevent.Head
	for _, durable := range fx.cas.snapshot() {
		next, err := computerevent.Reduce(current, durable.Request.Event, durable.Request.Input)
		if err != nil {
			t.Fatalf("replay of event %d (%s): %v", durable.Request.Event.Sequence, durable.Request.Event.EventKind, err)
		}
		if next != durable.Request.Next {
			t.Fatalf("replay of event %d (%s) produced %+v, the tape recorded %+v", durable.Request.Event.Sequence, durable.Request.Event.EventKind, next, durable.Request.Next)
		}
		current = &next
	}
	head, err := fx.store.Head(context.Background(), fx.computerID)
	if err != nil || head == nil || current == nil || *head != *current {
		t.Fatalf("replayed head %+v != stored head %+v (%v)", current, head, err)
	}
}

// degradedRollbackProductionShape is what a rollback degraded after its
// release applied leaves (d@e): the rollback_requested/rollback_applied events
// are on the tape, pending is empty, desired runs ahead of effective by event
// head, and the operation row is degraded with its route receipt. damage then
// ruins the target's pinned release.
func degradedRollbackProductionShape(t *testing.T, name string, damage func(t *testing.T, releaseDir string)) (*derivableSelfDevFixture, selfdev.Operation) {
	t.Helper()
	fx, first := rollbackFixture(t, name)
	rollback := fx.startRollbackTo(t, first, "rollback-"+name)
	fx.waitForState(t, rollback.OperationID, selfdev.StateRolledBack)
	fx.waitForStableHead(t)
	if _, err := fx.store.DB().Exec(`UPDATE self_development_operations SET state = ?, terminal_error = ? WHERE operation_id = ?`,
		selfdev.StateDegraded, "the settlement did not finish on two boots", rollback.OperationID); err != nil {
		t.Fatal(err)
	}
	head, err := fx.store.Head(context.Background(), fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef != "" || head.DesiredEventHead == head.EffectiveEventHead {
		t.Fatalf("fixture: head = %+v (%v); want pending empty and desired != effective", head, err)
	}
	got, err := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID)
	if err != nil || got.State != selfdev.StateDegraded || got.RouteReceipt == "" {
		t.Fatalf("fixture: operation = %+v (%v); want degraded with a route receipt", got, err)
	}
	damage(t, filepath.Join(fx.updaterRoot, "releases", got.ReleaseDigest))
	return fx, got
}

func TestDegradedRollbackWithUnavailablePinnedReleaseSettlesFailed(t *testing.T) {
	fx, rollback := degradedRollbackProductionShape(t, "stale-gone", removePinnedRelease)
	ctx := context.Background()
	headBefore, _ := fx.store.Head(ctx, fx.computerID)

	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)

	got, err := fx.operations.Get(ctx, fx.computerID, rollback.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != selfdev.StateFailed {
		t.Fatalf("state = %s (%q), want failed: a degraded rollback whose pinned release is gone must settle", got.State, got.TerminalError)
	}
	if want := "pinned release unavailable; the computer stays on its current release"; got.TerminalError != want {
		t.Fatalf("terminal error = %q, want %q", got.TerminalError, want)
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventRecoveryRecorded] != 1 || kinds[computerevent.EventMaterializationFailed] != 0 {
		t.Fatalf("events = %v; want the settlement recorded once and no pending to discharge", kinds)
	}
	headAfter, _ := fx.store.Head(ctx, fx.computerID)
	if headAfter.PendingTransitionRef != "" || headAfter.EffectiveEventHead != headBefore.EffectiveEventHead || headAfter.EffectiveStateCommitment != headBefore.EffectiveStateCommitment {
		t.Fatalf("head moved its effective state or bound pending: before %+v after %+v", headBefore, headAfter)
	}
	if open, found, err := fx.operations.OpenOperation(ctx, fx.computerID); err != nil || found {
		t.Fatalf("open operation after the settlement = %+v found=%v err=%v", open, found, err)
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRecoveryRecorded] != 1 {
		t.Fatalf("events after a second pass = %v", kinds)
	}
	requireReducerReplay(t, fx)

	// The hold is gone: a new operation starts, is decided and applies on the
	// live head (the fixture moved the booted base; put it back so the join admits it).
	fx.setBootedBuildCommit(t, joinTestBaseA)
	next := fx.applyPlain(t, "op-stale-gone-3")
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef != "" || head.DesiredStateCommitment != head.EffectiveStateCommitment {
		t.Fatalf("head after the following operation = %+v (%v)", head, err)
	}
	applied, err := fx.operations.Get(ctx, fx.computerID, next.OperationID)
	if err != nil || applied.State != selfdev.StateApplied || applied.EffectiveHead != head.EffectiveEventHead {
		t.Fatalf("the following operation = %+v (%v); want it applied on the live head", applied, err)
	}
	requireReducerReplay(t, fx)
}

func TestRollbackPendingWithUnavailablePinnedReleaseSettlesFailed(t *testing.T) {
	fx, first := rollbackFixture(t, "stale-pending")
	removePinnedRelease(t, filepath.Join(fx.updaterRoot, "releases", first.ReleaseDigest))
	rollback := fx.startRollbackTo(t, first, "rollback-stale-pending")
	// StartRollback never reads the manifest, so the rollback opens; the
	// materializer then settles it instead of retrying forever.
	got := fx.waitForState(t, rollback.OperationID, selfdev.StateFailed)
	if want := "pinned release unavailable; the computer stays on its current release"; got.TerminalError != want {
		t.Fatalf("terminal error = %q, want %q", got.TerminalError, want)
	}
	kinds := fx.countEventKinds()
	if kinds[computerevent.EventRecoveryRecorded] != 1 || kinds[computerevent.EventMaterializationFailed] != 1 {
		t.Fatalf("events = %v; want one settlement and one discharge of the pending rollback", kinds)
	}
	head, err := fx.store.Head(context.Background(), fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef != "" || head.DesiredEventHead != head.EffectiveEventHead {
		t.Fatalf("head = %+v (%v); pending must be discharged and desired returned to effective", head, err)
	}
	requireReducerReplay(t, fx)
}

func TestDegradedRollbackWithACorruptPinnedReleaseSettlesWithItsOwnCause(t *testing.T) {
	fx, rollback := degradedRollbackProductionShape(t, "stale-corrupt", corruptPinnedManifest)
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	got, err := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID)
	if err != nil || got.State != selfdev.StateFailed {
		t.Fatalf("operation = %+v (%v); want failed", got, err)
	}
	if !strings.HasPrefix(got.TerminalError, "pinned release failed verification: ") || strings.Contains(got.TerminalError, "unavailable") {
		t.Fatalf("terminal error = %q; a corrupt release must not be called retention", got.TerminalError)
	}
	// The tape tells the two causes apart by the settlement's act.
	corrupt := selfdevSettleIdempotency(selfdevSettleActPinnedCorrupt, fx.computerID, rollback.OperationID, "pinned-release-failed-verification")
	if _, found, err := fx.store.EventByIdempotency(context.Background(), fx.computerID, corrupt); err != nil || !found {
		t.Fatalf("no settlement event with the corrupt act (found=%v err=%v)", found, err)
	}
	unavailable := selfdevSettleIdempotency(selfdevSettleActPinnedUnavailable, fx.computerID, rollback.OperationID, "pinned-release-unavailable")
	if _, found, _ := fx.store.EventByIdempotency(context.Background(), fx.computerID, unavailable); found {
		t.Fatal("a corrupt release was recorded with the unavailable act")
	}
	requireReducerReplay(t, fx)
}

func TestDegradedRollbackWithATransientReadErrorStaysDegraded(t *testing.T) {
	fx, rollback := degradedRollbackProductionShape(t, "stale-transient", unreadablePinnedManifest)
	if _, _, err := updater.ReadPinnedManifest(fx.updaterRoot, rollback.ReleaseDigest); err == nil || errors.Is(err, updater.ErrPinnedReleaseUnavailable) || errors.Is(err, updater.ErrPinnedReleaseCorrupt) {
		t.Skipf("this environment reads a mode-0 file (%v); the transient class cannot be provoked", err)
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	got, err := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID)
	if err != nil || got.State != selfdev.StateDegraded {
		t.Fatalf("a transient read error moved the operation to %+v (%v); only a missing or corrupt release settles", got.State, err)
	}
}

func TestPinnedReleaseReadClasses(t *testing.T) {
	fx, first := rollbackFixture(t, "stale-classes")
	dir := filepath.Join(fx.updaterRoot, "releases", first.ReleaseDigest)
	if _, _, err := updater.ReadPinnedManifest(fx.updaterRoot, first.ReleaseDigest); err != nil {
		t.Fatalf("intact release: %v", err)
	}
	// A file the manifest names is missing: unavailable.
	removePinnedRelease(t, dir)
	if _, _, err := updater.ReadPinnedManifest(fx.updaterRoot, first.ReleaseDigest); !errors.Is(err, updater.ErrPinnedReleaseUnavailable) {
		t.Fatalf("missing release = %v; want unavailable", err)
	}
	// A regular file where the release directory should be: ENOTDIR counts as missing.
	if err := os.WriteFile(dir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := updater.ReadPinnedManifest(fx.updaterRoot, first.ReleaseDigest); !errors.Is(err, updater.ErrPinnedReleaseUnavailable) {
		t.Fatalf("release path that is a file = %v; want unavailable", err)
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir+".gone", dir); err != nil {
		t.Fatal(err)
	}
	// Tampered manifest: corrupt, not unavailable.
	corruptPinnedManifest(t, dir)
	_, _, err := updater.ReadPinnedManifest(fx.updaterRoot, first.ReleaseDigest)
	if !errors.Is(err, updater.ErrPinnedReleaseCorrupt) || errors.Is(err, updater.ErrPinnedReleaseUnavailable) {
		t.Fatalf("tampered manifest = %v; want corrupt", err)
	}
}

func abandonRequestBody(operationID, reason, key string) string {
	body, _ := json.Marshal(map[string]string{"operation_id": operationID, "reason": reason, "idempotency_key": key})
	return string(body)
}

func postAbandon(fx *derivableSelfDevFixture, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	(&APIHandler{rt: fx.rt}).abandonSelfDevelopmentOperation(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)), "owner", fx.computerID)
	return recorder
}

func TestAbandonClosesAnAwaitingApprovalOperationWithTheOwnersReason(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-abandon-awaiting")
	operation, _, _ := fx.seedOperationToAwaitingApproval(t, "op-abandon-awaiting")
	ctx := context.Background()
	if open, found, err := fx.operations.OpenOperation(ctx, fx.computerID); err != nil || !found || open.OperationID != operation.OperationID {
		t.Fatalf("fixture: open operation = %+v found=%v err=%v", open, found, err)
	}

	rec := postAbandon(fx, abandonRequestBody(operation.OperationID, "stale for six days", "abandon-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("abandon status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, err := fx.operations.Get(ctx, fx.computerID, operation.OperationID)
	if err != nil || got.State != selfdev.StateFailed || !strings.Contains(got.TerminalError, "stale for six days") || !strings.Contains(got.TerminalError, "abandoned") {
		t.Fatalf("abandoned operation = %+v (%v)", got, err)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRecoveryRecorded] != 1 {
		t.Fatalf("events = %v; want the owner decision on the tape once", kinds)
	}
	if _, found, _ := fx.operations.OpenOperation(ctx, fx.computerID); found {
		t.Fatal("the abandoned operation still holds the computer")
	}
	if again := postAbandon(fx, abandonRequestBody(operation.OperationID, "stale for six days", "abandon-1")); again.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", again.Code, again.Body.String())
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRecoveryRecorded] != 1 {
		t.Fatalf("a replay wrote another event: %v", kinds)
	}
	if other := postAbandon(fx, abandonRequestBody(operation.OperationID, "a different reason", "abandon-1")); other.Code != http.StatusConflict || !strings.Contains(other.Body.String(), "spent") {
		t.Fatalf("same key, other reason status=%d body=%s; want 409 saying the key is spent", other.Code, other.Body.String())
	}
	if again := postAbandon(fx, abandonRequestBody(operation.OperationID, "again", "abandon-2")); again.Code != http.StatusConflict {
		t.Fatalf("abandon of a failed operation status=%d, want 409", again.Code)
	}
	if bad := postAbandon(fx, abandonRequestBody(operation.OperationID, "  ", "abandon-3")); bad.Code != http.StatusBadRequest {
		t.Fatalf("abandon with no reason status=%d, want 400", bad.Code)
	}
	if missing := postAbandon(fx, abandonRequestBody("no-such-op", "x", "abandon-4")); missing.Code != http.StatusNotFound {
		t.Fatalf("abandon of an unknown operation status=%d, want 404", missing.Code)
	}
	requireReducerReplay(t, fx)
}

// The wedge that review found: a rollback_pending whose pinned release is
// unusable, parked where nothing settles it automatically (a transient read
// error here), has only the owner act. It has no applied or mid-flight apply,
// so abandon settles it and a new rollback is possible again.
func TestAbandonSettlesARollbackPendingWithNothingApplied(t *testing.T) {
	fx, first := rollbackFixture(t, "abandon-pending")
	unreadablePinnedManifest(t, filepath.Join(fx.updaterRoot, "releases", first.ReleaseDigest))
	if _, _, err := updater.ReadPinnedManifest(fx.updaterRoot, first.ReleaseDigest); err == nil || errors.Is(err, updater.ErrPinnedReleaseUnavailable) || errors.Is(err, updater.ErrPinnedReleaseCorrupt) {
		t.Skipf("this environment reads a mode-0 file (%v)", err)
	}
	rollback := fx.startRollbackTo(t, first, "rollback-abandon-pending")
	fx.waitForState(t, rollback.OperationID, selfdev.StateRollbackPending)

	rec := postAbandon(fx, abandonRequestBody(rollback.OperationID, "pinned release will not come back", "abandon-pending"))
	if rec.Code != http.StatusOK {
		t.Fatalf("abandon of a rollback_pending status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID)
	if got.State != selfdev.StateFailed || !strings.Contains(got.TerminalError, "pinned release will not come back") {
		t.Fatalf("operation = %+v", got)
	}
	head, err := fx.store.Head(context.Background(), fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef != "" || head.DesiredEventHead != head.EffectiveEventHead {
		t.Fatalf("head = %+v (%v); the pending rollback must be discharged", head, err)
	}
	requireReducerReplay(t, fx)
}

// An apply that the updater journaled applied is a release that is live: a
// settle must not close the row under it.
func TestAbandonRefusesAMaterializingOperationWhoseApplyIsJournaledApplied(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-abandon-applied")
	operation := fx.parkAppliedButUnrecorded(t, "op-abandon-applied")
	rec := postAbandon(fx, abandonRequestBody(operation.OperationID, "no", "abandon-applied"))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "applied release") {
		t.Fatalf("status=%d body=%s; want 409 naming the applied release", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(context.Background(), fx.computerID, operation.OperationID)
	if got.State != selfdev.StateMaterializing {
		t.Fatalf("state = %s, want it untouched", got.State)
	}
}

func TestAbandonRefusesWhileTheMaterializerIsWorking(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-abandon-busy")
	operation := fx.parkAppliedButUnrecorded(t, "op-abandon-busy")
	fx.rt.selfdevMaterializeMu.Lock()
	defer fx.rt.selfdevMaterializeMu.Unlock()
	rec := postAbandon(fx, abandonRequestBody(operation.OperationID, "no", "abandon-busy"))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "materializer is working") {
		t.Fatalf("status=%d body=%s; want 409 while the materializer holds its pass", rec.Code, rec.Body.String())
	}
}

func TestAbandonClosesADegradedOperation(t *testing.T) {
	fx, rollback := degradedRollbackProductionShape(t, "abandon-degraded", unreadablePinnedManifest)
	rec := postAbandon(fx, abandonRequestBody(rollback.OperationID, "owner gave up on it", "abandon-degraded"))
	if rec.Code != http.StatusOK {
		t.Fatalf("abandon of a degraded operation status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID)
	if got.State != selfdev.StateFailed || !strings.Contains(got.TerminalError, "owner gave up on it") {
		t.Fatalf("operation = %+v", got)
	}
	requireReducerReplay(t, fx)
}

// A decision that lands between the caller's checks and the transition leaves
// the head pending on the operation's decision. The settle must discharge it:
// a failed operation never keeps the head's pending transition bound.
func TestSettleDischargesADecisionThatLandedUnderIt(t *testing.T) {
	fx := newDerivableSelfDevFixture(t, "computer-settle-toctou")
	stale := fx.decideWithoutActing(t, "toctou") // the decision is on the tape; the row still reads awaiting_approval
	ctx := context.Background()
	row, err := fx.operations.Get(ctx, fx.computerID, stale.OperationID)
	if err != nil || row.State != selfdev.StateAwaitingApproval || row.DecisionEvent != "" {
		t.Fatalf("fixture: row = %+v (%v)", row, err)
	}
	if fx.pendingTransition(t) == "" {
		t.Fatal("fixture: the decision did not bind a pending transition")
	}

	got, err := fx.rt.settleSelfDevelopmentOperationFailed(ctx, row, selfdevSettleSpec{
		Act: selfdevSettleActAbandon, Actor: "owner:owner", Authority: "external-owner:owner", Reason: "raced", Key: "toctou", TerminalError: "abandoned by the owner: raced",
	})
	if err != nil || got.State != selfdev.StateFailed {
		t.Fatalf("settle = %+v (%v)", got, err)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound to a failed operation", pending)
	}
	head, _ := fx.store.Head(ctx, fx.computerID)
	if head.DesiredEventHead != head.EffectiveEventHead {
		t.Fatalf("desired %s did not return to effective %s", head.DesiredEventHead, head.EffectiveEventHead)
	}
	requireReducerReplay(t, fx)
}

// The hold is recorded on the ledger after the third consecutive deferral.
func TestOpenOperationHoldIsRecordedOnTheLedgerAfterThreeDeferrals(t *testing.T) {
	ctx := context.Background()
	f := newCarryHoldFixture(t, selfdev.StateAwaitingApproval)
	seed, err := store.SeedEngineeringAssignmentAuthorityForDocument(f.s, "owner-carry-hold", f.first.Binding.ComputerID, "beta", 1)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := f.s.OpenEngineeringAssignment(ctx, store.SeedEngineeringOpenRequest(seed, 0, "cap-"+seed.AssignmentIDs[0]))
	if err != nil {
		t.Fatal(err)
	}
	other := opened.Assignment
	cancel := types.CancelEngineeringAssignmentRequest{CommandID: "command-hold-cancel", OwnerID: f.retry.Binding.OwnerID, ComputerID: f.retry.Binding.ComputerID,
		AssignmentID: f.retry.AssignmentID, Attempt: f.retry.Binding.Attempt, ExpectedLifecycleVersion: c1eGet(t, f.s, f.retry).LifecycleVersion, Reason: "test: withdrawn"}
	cancel.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancel)
	if _, err := f.s.CancelEngineeringAssignment(ctx, cancel); err != nil {
		t.Fatal(err)
	}
	err = f.rt.admitEngineeringAssignment(ctx, c1eGet(t, f.s, other))
	var busy *engineeringAdmissionBusy
	if !errors.As(err, &busy) || busy.Why != "operation_open" {
		t.Fatalf("admission = %v; want operation_open", err)
	}
	if busy.OperationID != f.operationID || busy.OperationState != selfdev.StateAwaitingApproval {
		t.Fatalf("the refusal does not carry the blocker: %+v", busy)
	}

	f.rt.kernelMode = true
	f.rt.scheduleActor = func(context.Context, string, string, string, string, string, string, string, time.Time) error {
		return nil
	}
	assignment := c1eGet(t, f.s, other)
	recordID := engineeringHoldRecordID(f.operationID, selfdev.StateAwaitingApproval, assignment.AssignmentID)
	recorded := func() bool {
		ok, err := f.s.CommitmentRecordExists(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, recordID)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	for retry := 0; retry < 2; retry++ {
		if err := f.rt.deferEngineeringAdmission(ctx, assignment, delegatedAssignmentSpawnDeadline{AssignmentID: assignment.AssignmentID, Retry: retry}, "obj", busy); err != nil {
			t.Fatal(err)
		}
		if recorded() {
			t.Fatalf("the hold was recorded after deferral %d; it waits for the third", retry+1)
		}
	}
	for retry := 2; retry < 5; retry++ {
		if err := f.rt.deferEngineeringAdmission(ctx, assignment, delegatedAssignmentSpawnDeadline{AssignmentID: assignment.AssignmentID, Retry: retry}, "obj", busy); err != nil {
			t.Fatal(err)
		}
	}
	if !recorded() {
		t.Fatal("the third deferral recorded nothing on the ledger")
	}
	rec := engineeringHoldRecord(assignment, busy, time.Now().UTC())
	text := rec.Prediction.Hypothesis + " " + rec.Observation.Excerpt
	for _, want := range []string{f.operationID, selfdev.StateAwaitingApproval, "reject", "abandon", assignment.AssignmentID} {
		if !strings.Contains(text, want) {
			t.Fatalf("hold record does not say %q: %s", want, text)
		}
	}
}
