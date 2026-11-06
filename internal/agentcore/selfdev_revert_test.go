package agentcore

// R3 revert (docs/design/selfdev-apply-and-revert-2026-11-06.md, 3.5). Ways
// this could fail, written first:
//   - a revert of a non-first apply does not equal the rollback to its prior
//     operation (wrong target, bindings assembled from a stale read);
//   - a revert of a first apply (prior is the genesis checkpoint) is refused,
//     or restores something other than the booted base release, or never
//     records its success and sticks in rollback_pending;
//   - a double request opens two reverts;
//   - a second revert to the same target after a later apply is refused as
//     "already reverted";
//   - an operation that is not applied, or is unknown, is reverted.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

func (fx *derivableSelfDevFixture) withBaselineRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(fx.updaterRoot, "incoming", "baseline-root")
	if err := os.MkdirAll(filepath.Join(root, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "frontend", "index.html"), []byte("<html>booted base</html>"), 0o444); err != nil {
		t.Fatal(err)
	}
	fx.configure(func(rt *Runtime) { rt.packageMigrationBaseline = root })
	return root
}

func (fx *derivableSelfDevFixture) revert(t *testing.T, operationID string) (int, selfdev.Operation) {
	t.Helper()
	fx.waitForStableHead(t)
	recorder := httptest.NewRecorder()
	(&APIHandler{rt: fx.rt}).revertSelfDevelopmentOperation(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")), "owner", fx.computerID, operationID)
	var operation selfdev.Operation
	if recorder.Code == http.StatusOK || recorder.Code == http.StatusCreated {
		if err := json.NewDecoder(recorder.Body).Decode(&operation); err != nil {
			t.Fatalf("revert body: %v", err)
		}
	} else {
		t.Logf("revert status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	return recorder.Code, operation
}

func (fx *derivableSelfDevFixture) rollbackOps(t *testing.T) []selfdev.Operation {
	t.Helper()
	ops, err := fx.operations.ListByStates(context.Background(), fx.computerID, selfdev.StateRolledBack, selfdev.StateRollbackPending)
	if err != nil {
		t.Fatal(err)
	}
	return ops
}

func TestRevertOfANonFirstApplyIsTheRollbackToItsPriorOperation(t *testing.T) {
	fx := joinFixture(t, "rv-prior", joinTestBaseA, joinTestBaseA)
	first := fx.applyPlain(t, "op-rv-prior-1")
	second := fx.applyPlain(t, "op-rv-prior-2")
	fx.setBootedBuildCommit(t, joinTestBaseB)

	code, reverted := fx.revert(t, second.OperationID)
	if code != http.StatusCreated {
		t.Fatalf("revert status=%d", code)
	}
	if selfdev.IsBaseRestore(reverted) || reverted.BaseHead != first.EffectiveHead {
		t.Fatalf("revert of the second apply = %+v; want the rollback to the first (%s)", reverted, first.EffectiveHead)
	}
	fx.requireRolledBackTo(t, reverted, first)
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRollbackApplied] != 1 {
		t.Fatalf("events = %v; want one rollback_applied", kinds)
	}
}

func TestRevertOfAFirstApplyRestoresTheBootedBase(t *testing.T) {
	ctx := context.Background()
	fx := joinFixture(t, "rv-base", joinTestBaseA, joinTestBaseA)
	fx.withBaselineRoot(t)
	first := fx.applyPlain(t, "op-rv-base-1")
	if fx.currentManifest(t).ContentDigest != first.ReleaseDigest {
		t.Fatal("setup: the first apply is not the served release")
	}

	code, opened := fx.revert(t, first.OperationID)
	if code != http.StatusCreated || !selfdev.IsBaseRestore(opened) || selfdev.BaseRestoreSubject(opened) != first.OperationID {
		t.Fatalf("revert status=%d operation=%+v; want a base restore of %s", code, opened, first.OperationID)
	}
	if len(opened.VerifierRefs) != 0 || opened.CheckpointRef != "" || opened.ReleaseDigest != opened.BundleDigest {
		t.Fatalf("a base restore carries no verifier evidence and targets the base manifest digest: %+v", opened)
	}
	done := fx.waitForState(t, opened.OperationID, selfdev.StateRolledBack)
	served := fx.currentManifest(t)
	if served.ContentDigest != opened.ReleaseDigest || served.ContentDigest == first.ReleaseDigest || served.Marker != "genesis-baseline" || served.LayeringEntrypoint != "" {
		t.Fatalf("served release = %+v; want the booted base %s with no overlay", served, opened.ReleaseDigest)
	}
	if done.MaterializationReceipt == "" || done.CheckpointRef != "" || done.RouteReceipt != "" {
		t.Fatalf("the tail binds the baseline release, no checkpoint or route: %+v", done)
	}
	if kinds := fx.countEventKinds(); kinds[computerevent.EventRollbackApplied] != 1 {
		t.Fatalf("events = %v; want one rollback_applied", kinds)
	}
	head, err := fx.store.Head(ctx, fx.computerID)
	if err != nil || head == nil || head.PendingTransitionRef != "" || head.EffectiveEventHead != done.EffectiveHead {
		t.Fatalf("head after the restore = %+v err=%v; want the transition discharged on the operation's effective head", head, err)
	}
	if open, found, err := fx.operations.OpenOperation(ctx, fx.computerID); err != nil || found {
		t.Fatalf("open operation after the restore = %+v found=%v err=%v", open, found, err)
	}
}

func TestRevertDoubleRequestIsOneAct(t *testing.T) {
	fx := joinFixture(t, "rv-double", joinTestBaseA, joinTestBaseA)
	fx.withBaselineRoot(t)
	first := fx.applyPlain(t, "op-rv-double-1")

	code, opened := fx.revert(t, first.OperationID)
	if code != http.StatusCreated {
		t.Fatalf("revert status=%d", code)
	}
	// A second tap, before and after the first settles, starts nothing.
	code, again := fx.revert(t, first.OperationID)
	if code != http.StatusOK || again.OperationID != opened.OperationID {
		t.Fatalf("second tap status=%d operation=%s; want the first act %s", code, again.OperationID, opened.OperationID)
	}
	fx.waitForState(t, opened.OperationID, selfdev.StateRolledBack)
	code, again = fx.revert(t, first.OperationID)
	if code != http.StatusOK || again.OperationID != opened.OperationID {
		t.Fatalf("tap after settle status=%d operation=%s; want the first act %s", code, again.OperationID, opened.OperationID)
	}
	if ops := fx.rollbackOps(t); len(ops) != 1 {
		t.Fatalf("rollback operations = %d, want 1", len(ops))
	}
}

func TestRevertToTheSameTargetAfterALaterApplyIsAccepted(t *testing.T) {
	fx := joinFixture(t, "rv-again", joinTestBaseA, joinTestBaseA)
	fx.withBaselineRoot(t)
	first := fx.applyPlain(t, "op-rv-again-1")
	_, firstRevert := fx.revert(t, first.OperationID)
	fx.waitForState(t, firstRevert.OperationID, selfdev.StateRolledBack)

	second := fx.applyPlain(t, "op-rv-again-2")
	code, secondRevert := fx.revert(t, second.OperationID)
	if code != http.StatusCreated || secondRevert.OperationID == firstRevert.OperationID || !selfdev.IsBaseRestore(secondRevert) {
		t.Fatalf("second revert status=%d operation=%+v; want a new base restore", code, secondRevert)
	}
	fx.waitForState(t, secondRevert.OperationID, selfdev.StateRolledBack)
	if served := fx.currentManifest(t); served.ContentDigest != secondRevert.ReleaseDigest {
		t.Fatalf("served release %s, want the base %s", served.ContentDigest, secondRevert.ReleaseDigest)
	}
}

func TestRevertRefusesWhatIsNotAnAppliedOperation(t *testing.T) {
	fx, first := rollbackFixture(t, "rv-refuse")
	if code, _ := fx.revert(t, "selfdev-unknown"); code != http.StatusNotFound {
		t.Fatalf("unknown operation status=%d, want 404", code)
	}
	rollback := fx.startRollbackTo(t, first, "rollback-rv-refuse")
	fx.requireRolledBackTo(t, rollback, first)
	if code, _ := fx.revert(t, rollback.OperationID); code != http.StatusConflict {
		t.Fatalf("revert of a rollback status=%d, want 409", code)
	}
}
