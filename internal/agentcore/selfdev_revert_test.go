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
	"time"

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

// An orphaned revert (its rollback_requested event committed, its row never
// created) is finished by the next tap, not refused for good.
func TestRevertFinishesAnOrphanedBaseRestore(t *testing.T) {
	ctx := context.Background()
	fx := joinFixture(t, "rv-orphan", joinTestBaseA, joinTestBaseA)
	baseline := fx.withBaselineRoot(t)
	first := fx.applyPlain(t, "op-rv-orphan-1")
	accepted, found, err := fx.store.EventByDigest(ctx, fx.computerID, first.DecisionEvent)
	if err != nil || !found {
		t.Fatalf("accept event: found=%v err=%v", found, err)
	}
	target, err := fx.rt.servingBaselineManifest(ctx, fx.computerID, baseline)
	if err != nil {
		t.Fatal(err)
	}
	key := "revert:" + first.OperationID + ":" + first.OperationID
	event, err := rollbackRequestedEvent(fx.computerID, "owner", "selfdev-rollback-requested-"+key, strings.Repeat("d", 64), first.TrajectoryID, first.CapsuleID, target.ContentDigest, accepted.ExpectedEffectiveEventHead)
	if err != nil {
		t.Fatal(err)
	}
	fx.waitForStableHead(t)
	if _, err := fx.appender.AppendNew(ctx, event, computerevent.TransitionInput{TargetStateCommitment: accepted.ExpectedEffectiveStateCommitment}, nil); err != nil {
		t.Fatal(err)
	}
	if head, _ := fx.store.Head(ctx, fx.computerID); head == nil || head.PendingTransitionRef == "" {
		t.Fatal("setup: the orphaned request leaves the transition pending")
	}
	code, opened := fx.revert(t, first.OperationID)
	if code != http.StatusCreated || !selfdev.IsBaseRestore(opened) {
		t.Fatalf("revert of an orphaned request status=%d operation=%+v; want it finished", code, opened)
	}
	fx.waitForState(t, opened.OperationID, selfdev.StateRolledBack)
	if head, _ := fx.store.Head(ctx, fx.computerID); head == nil || head.PendingTransitionRef != "" {
		t.Fatalf("head after the finished revert = %+v; want the transition discharged", head)
	}
	if opened.TrajectoryID != first.TrajectoryID {
		t.Fatalf("the revert row's trajectory %q, want the candidate's %q", opened.TrajectoryID, first.TrajectoryID)
	}
}

// A revert of an operation a later revert already undid would reinstall the
// earlier change; it is refused, and the listing says what is still live.
func TestRevertRefusesAnAlreadyUndoneOperation(t *testing.T) {
	fx := joinFixture(t, "rv-undone", joinTestBaseA, joinTestBaseA)
	fx.withBaselineRoot(t)
	a := fx.applyPlain(t, "op-rv-undone-a")
	x := fx.applyPlain(t, "op-rv-undone-x")
	code, undoA := fx.revert(t, a.OperationID)
	if code != http.StatusCreated {
		t.Fatalf("revert of the first apply status=%d", code)
	}
	fx.waitForState(t, undoA.OperationID, selfdev.StateRolledBack)
	if code, _ := fx.revert(t, x.OperationID); code != http.StatusConflict {
		t.Fatalf("revert of an operation already undone status=%d, want 409", code)
	}
	recorder := httptest.NewRecorder()
	(&APIHandler{rt: fx.rt}).listSelfDevelopmentOperations(recorder, httptest.NewRequest(http.MethodGet, "/", nil), fx.computerID)
	var listing struct {
		Live []string `json:"live_operation_ids"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&listing); err != nil {
		t.Fatal(err)
	}
	for _, id := range listing.Live {
		if id == a.OperationID || id == x.OperationID {
			t.Fatalf("live operations %v still name an undone operation", listing.Live)
		}
	}
}

func TestLiveOperationSetFollowsRollbacks(t *testing.T) {
	at := func(n int) string { return time.Date(2026, 11, 6, 10, n, 0, 0, time.UTC).Format(time.RFC3339Nano) }
	h := func(c string) string { return strings.Repeat(c, 64) }
	rows := []selfdev.Operation{
		{OperationID: "selfdev-a", State: selfdev.StateApplied, EffectiveHead: h("1"), CreatedAt: at(1)},
		{OperationID: "selfdev-b", State: selfdev.StateApplied, EffectiveHead: h("2"), CreatedAt: at(2)},
		{OperationID: "rollback-1", State: selfdev.StateRolledBack, BaseHead: h("1"), EffectiveHead: h("3"), CreatedAt: at(3)},
		{OperationID: "selfdev-c", State: selfdev.StateApplied, EffectiveHead: h("4"), CreatedAt: at(4)},
	}
	live := liveOperationSet(rows)
	if live["selfdev-b"] || !live["selfdev-a"] || !live["selfdev-c"] {
		t.Fatalf("live = %v; a and c survive, b was undone", live)
	}
	rows = append(rows, selfdev.Operation{OperationID: "rollback-2", State: selfdev.StateRolledBack, BaseHead: h("9"), EffectiveHead: h("5"), CreatedAt: at(5)})
	if live := liveOperationSet(rows); len(live) != 0 {
		t.Fatalf("live = %v; a restore to a checkpoint with no row undoes all", live)
	}
}

// Two taps at once are one act: the loser gets the winner's row.
func TestRevertConcurrentTapsAreOneAct(t *testing.T) {
	fx := joinFixture(t, "rv-race", joinTestBaseA, joinTestBaseA)
	fx.withBaselineRoot(t)
	first := fx.applyPlain(t, "op-rv-race-1")
	fx.waitForStableHead(t)
	type answer struct {
		code int
		id   string
	}
	results := make(chan answer, 2)
	for i := 0; i < 2; i++ {
		go func() {
			recorder := httptest.NewRecorder()
			(&APIHandler{rt: fx.rt}).revertSelfDevelopmentOperation(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")), "owner", fx.computerID, first.OperationID)
			var op selfdev.Operation
			_ = json.NewDecoder(recorder.Body).Decode(&op)
			results <- answer{recorder.Code, op.OperationID}
		}()
	}
	a, b := <-results, <-results
	for _, got := range []answer{a, b} {
		if got.code != http.StatusOK && got.code != http.StatusCreated {
			t.Fatalf("a concurrent tap got %d, want the same act's row", got.code)
		}
	}
	if a.id == "" || a.id != b.id {
		t.Fatalf("taps answered operations %q and %q; want one act", a.id, b.id)
	}
	fx.waitForState(t, a.id, selfdev.StateRolledBack)
}

// An orphaned request whose booted base moved since is not a permanent wedge:
// the retry binds the row to the event's recorded digest, the drive refuses at
// its own target check, the refusal discharges the pending transition, and the
// next revert works.
func TestRevertOrphanWithAMovedBaseClosesFailedAndUnwedges(t *testing.T) {
	ctx := context.Background()
	fx := joinFixture(t, "rv-moved", joinTestBaseA, joinTestBaseA)
	fx.withBaselineRoot(t)
	first := fx.applyPlain(t, "op-rv-moved-1")
	accepted, found, err := fx.store.EventByDigest(ctx, fx.computerID, first.DecisionEvent)
	if err != nil || !found {
		t.Fatalf("accept event: found=%v err=%v", found, err)
	}
	key := "revert:" + first.OperationID + ":" + first.OperationID
	// The event recorded a base digest that is not today's.
	event, err := rollbackRequestedEvent(fx.computerID, "owner", "selfdev-rollback-requested-"+key, strings.Repeat("d", 64), first.TrajectoryID, first.CapsuleID, strings.Repeat("e", 64), accepted.ExpectedEffectiveEventHead)
	if err != nil {
		t.Fatal(err)
	}
	fx.waitForStableHead(t)
	if _, err := fx.appender.AppendNew(ctx, event, computerevent.TransitionInput{TargetStateCommitment: accepted.ExpectedEffectiveStateCommitment}, nil); err != nil {
		t.Fatal(err)
	}
	code, opened := fx.revert(t, first.OperationID)
	if code != http.StatusCreated {
		t.Fatalf("retry of an orphan with a moved base status=%d, want the row opened", code)
	}
	fx.waitForState(t, opened.OperationID, selfdev.StateFailed)
	if head, _ := fx.store.Head(ctx, fx.computerID); head == nil || head.PendingTransitionRef != "" {
		t.Fatalf("head after the refusal = %+v; want the pending transition discharged", head)
	}
	code, next := fx.revert(t, first.OperationID)
	if code != http.StatusCreated || next.OperationID == opened.OperationID {
		t.Fatalf("the next revert status=%d operation=%s; want a new act", code, next.OperationID)
	}
	fx.waitForState(t, next.OperationID, selfdev.StateRolledBack)
}
