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
// Ways the three changes can fail, written before the code:
//  1. a degraded rollback whose pinned release is gone is retried forever, or
//     settles with no record on the tape;
//  2. a transient read error (not "gone") settles the operation, losing a
//     rollback that would have worked;
//  3. the settled operation leaves the head pending, so a later operation is
//     refused or binds to a head that is not live;
//  4. abandon closes an operation that is in flight (materializing,
//     rollback_pending), racing the updater;
//  5. abandon closes with no owner decision on the tape, or twice;
//  6. the engineering admission hold stays console-only.

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

// degradedRollbackWithPinnedRelease starts a rollback to the first applied
// operation, damages that operation's pinned release with hide, and parks the
// rollback degraded with a route receipt (a completed prior promote, lost
// mid-rollback).
func degradedRollbackWithPinnedRelease(t *testing.T, name string, hide func(t *testing.T, releaseDir string)) (*derivableSelfDevFixture, selfdev.Operation, selfdev.Operation) {
	t.Helper()
	fx, first := rollbackFixture(t, name)
	hide(t, filepath.Join(fx.updaterRoot, "releases", first.ReleaseDigest))
	rollback := fx.startRollbackTo(t, first, "rollback-"+name)
	fx.waitForState(t, rollback.OperationID, selfdev.StateRollbackPending)
	if _, err := fx.operations.Transition(context.Background(), fx.computerID, rollback.OperationID, selfdev.StateRollbackPending, selfdev.StateDegraded, func(next *selfdev.Operation) error {
		next.RouteReceipt = strings.Repeat("a", 64)
		next.TerminalError = "the settlement did not finish on two boots"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return fx, first, rollback
}

func TestDegradedRollbackWithUnavailablePinnedReleaseSettlesFailed(t *testing.T) {
	fx, _, rollback := degradedRollbackWithPinnedRelease(t, "stale-gone", removePinnedRelease)
	ctx := context.Background()

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
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRecoveryRecorded] != 1 {
		t.Fatalf("events = %v; want the settlement recorded once on the tape", kinds)
	}
	if pending := fx.pendingTransition(t); pending != "" {
		t.Fatalf("pending transition %s still bound after the settlement", pending)
	}
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil || head.DesiredEventHead != head.EffectiveEventHead {
		t.Fatalf("head after the settlement = %+v (%v); desired must not run ahead of effective", head, err)
	}
	if open, found, err := fx.operations.OpenOperation(ctx, fx.computerID); err != nil || found {
		t.Fatalf("open operation after the settlement = %+v found=%v err=%v", open, found, err)
	}
	// A pass after the settlement changes nothing.
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRecoveryRecorded] != 1 {
		t.Fatalf("events after a second pass = %v", kinds)
	}
}

func TestFollowingOperationBindsToTheLiveHeadAfterTheSettlement(t *testing.T) {
	fx, _, rollback := degradedRollbackWithPinnedRelease(t, "stale-follow", removePinnedRelease)
	ctx := context.Background()
	fx.rt.reconcileSelfDevelopmentMaterialization(ctx)
	fx.waitForState(t, rollback.OperationID, selfdev.StateFailed)

	// The hold is gone, and a new operation starts, is decided and applies
	// (the fixture moved the booted base; put it back so the join admits it).
	fx.setBootedBuildCommit(t, joinTestBaseA)
	next := fx.applyPlain(t, "op-stale-follow-3")
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef != "" || head.DesiredStateCommitment != head.EffectiveStateCommitment {
		t.Fatalf("head after the following operation = %+v (%v)", head, err)
	}
	applied, err := fx.operations.Get(ctx, fx.computerID, next.OperationID)
	if err != nil || applied.State != selfdev.StateApplied || applied.EffectiveHead != head.EffectiveEventHead {
		t.Fatalf("the following operation = %+v (%v); want it applied on the live head", applied, err)
	}
}

func TestDegradedRollbackWithATransientReadErrorStaysDegraded(t *testing.T) {
	fx, _, rollback := degradedRollbackWithPinnedRelease(t, "stale-transient", unreadablePinnedManifest)
	if _, _, err := updater.ReadPinnedManifest(fx.updaterRoot, rollback.ReleaseDigest); err == nil || errors.Is(err, updater.ErrPinnedReleaseUnavailable) {
		t.Skipf("this environment reads a mode-0 file (%v); the transient class cannot be provoked", err)
	}
	fx.rt.reconcileSelfDevelopmentMaterialization(context.Background())
	got, err := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID)
	if err != nil || got.State != selfdev.StateDegraded {
		t.Fatalf("a transient read error moved the operation to %+v (%v); only an unavailable release settles", got.State, err)
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
	// The same key and reason replay; no second event.
	if again := postAbandon(fx, abandonRequestBody(operation.OperationID, "stale for six days", "abandon-1")); again.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", again.Code, again.Body.String())
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRecoveryRecorded] != 1 {
		t.Fatalf("a replay wrote another event: %v", kinds)
	}
	// A settled operation cannot be abandoned under a new key.
	if again := postAbandon(fx, abandonRequestBody(operation.OperationID, "again", "abandon-2")); again.Code != http.StatusConflict {
		t.Fatalf("abandon of a failed operation status=%d, want 409", again.Code)
	}
	// And a request with no reason is refused before any act.
	if bad := postAbandon(fx, abandonRequestBody(operation.OperationID, "  ", "abandon-3")); bad.Code != http.StatusBadRequest {
		t.Fatalf("abandon with no reason status=%d, want 400", bad.Code)
	}
}

func TestAbandonRefusesAnOperationInFlight(t *testing.T) {
	fx, first := rollbackFixture(t, "abandon-flight")
	// A rollback whose pinned release is hidden stays rollback_pending.
	removePinnedRelease(t, filepath.Join(fx.updaterRoot, "releases", first.ReleaseDigest))
	rollback := fx.startRollbackTo(t, first, "rollback-abandon-flight")
	fx.waitForState(t, rollback.OperationID, selfdev.StateRollbackPending)

	rec := postAbandon(fx, abandonRequestBody(rollback.OperationID, "no", "abandon-flight"))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "in flight") {
		t.Fatalf("abandon of a rollback_pending operation status=%d body=%s; want 409 naming it in flight", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID)
	if got.State != selfdev.StateRollbackPending {
		t.Fatalf("state = %s, want it untouched", got.State)
	}
}

func TestAbandonClosesADegradedOperation(t *testing.T) {
	fx, _, rollback := degradedRollbackWithPinnedRelease(t, "abandon-degraded", unreadablePinnedManifest)
	rec := postAbandon(fx, abandonRequestBody(rollback.OperationID, "owner gave up on it", "abandon-degraded"))
	if rec.Code != http.StatusOK {
		t.Fatalf("abandon of a degraded operation status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := fx.operations.Get(context.Background(), fx.computerID, rollback.OperationID)
	if got.State != selfdev.StateFailed || !strings.Contains(got.TerminalError, "owner gave up on it") {
		t.Fatalf("operation = %+v", got)
	}
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
	recordID := engineeringHoldRecordID(f.operationID, assignment.AssignmentID)
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
