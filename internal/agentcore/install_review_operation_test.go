package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The package self-development operation's install review (design 2.2, 7.1):
// the draft, the checked build and the gate run while the operation is still
// accepted; a review management owes keeps it accepted (errInstallReviewPending),
// and only a passed gate moves it to materializing.

type opReviewWorld struct {
	*packageOpWorld
	signer *reviewSigner
	clock  atomic.Int64
}

func newOpReviewWorld(t *testing.T, name string) *opReviewWorld {
	t.Helper()
	w := &opReviewWorld{packageOpWorld: newPackageOpWorld(t, name), signer: newReviewSigner(t)}
	manifest := strings.ReplaceAll(strings.ReplaceAll(draftManifest, `"reader"`, `"`+newPackageName+`"`), "Reader", "Demo new")
	manifest = strings.Replace(manifest, `"capabilities": []`, `"capabilities": [{"grant":"ui.read.kind","args":["notes"]}]`, 1)
	w.files[newPackageRoot+"/choir-package.json"] = manifest
	w.clock.Store(time.Now().UnixNano())
	w.fx.configure(func(rt *Runtime) {
		rt.packageBuilder = &manifestBuilder{manifest: manifest}
		rt.installReviewSigner = w.signer
		rt.installReviewNow = func() time.Time { return time.Unix(0, w.clock.Load()) }
	})
	return w
}

func (w *opReviewWorld) advance(d time.Duration) { w.clock.Add(int64(d)) }

func (w *opReviewWorld) owner() string { return w.fx.rt.selfdevRouteOwnerID }

// release is the draft's release id, once the operation has drafted.
func (w *opReviewWorld) release(t *testing.T, operation selfdev.Operation) string {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if draft, found, err := w.fx.rt.loadPackageDraft(operation); err == nil && found {
			return draft.ReleaseID
		}
		if time.Now().After(deadline) {
			t.Fatal("the operation never drafted")
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// waitOwed waits for the owed review on the release to be open.
func (w *opReviewWorld) waitOwed(t *testing.T, release string) installReviewOwed {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		owed, found, err := w.fx.rt.installReviewOwedOn(context.Background(), w.owner(), w.fx.computerID, release)
		if err == nil && found && owed.Open {
			return owed
		}
		if time.Now().After(deadline) {
			t.Fatalf("no open review on %s (found=%v err=%v)", release, found, err)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (w *opReviewWorld) judge(t *testing.T, release string, accept bool, reason string) (map[string]any, error) {
	t.Helper()
	run := types.RunRecord{RunID: "run-mgmt-op", OwnerID: w.owner(), ComputerID: w.fx.computerID, AgentID: persistentManagementAgentID(w.owner()), AgentProfile: agentprofile.Management}
	reduce := &rlmCallReduction{active: true, mb: w.fx.rt, st: w.fx.rt.store, ledger: w.fx.rt.store, rec: &run, scope: ReductionScope{
		RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, DeskAgentID: run.AgentID,
	}}
	decision := "reject"
	if accept {
		decision = "accept"
	}
	return reduce.commitInstallVerdictIntent(context.Background(), yaegikernel.StagedIntent{
		LocalID: "v1", Kind: yaegikernel.IntentInstallVerdict, TargetRef: release, Decision: decision, PWithinCeiling: 0.85, Reason: reason,
	})
}

func (w *opReviewWorld) pass() { w.fx.rt.reconcileSelfDevelopmentMaterialization(context.Background()) }

func TestPackageOperationPendsInAcceptedAndInstallsOnAnAcceptingVerdict(t *testing.T) {
	w := newOpReviewWorld(t, "review-accept")
	operation := w.propose(t, "op-pkg-review-accept")
	release := w.release(t, operation)
	owed := w.waitOwed(t, release)
	// FM4 and the reorder: the operation is still accepted, nothing is
	// materializing, published or installed, and the owed record binds the
	// operation.
	w.pass()
	pending, err := w.fx.operations.Get(context.Background(), w.fx.computerID, operation.OperationID)
	if err != nil || pending.State != selfdev.StateAccepted {
		t.Fatalf("operation = %s, %v; want accepted while the review is owed", pending.State, err)
	}
	if kinds := w.fx.countEventKinds(); kinds[computerevent.EventMaterializationStarted] != 0 || kinds[computerevent.EventPackageApplied] != 0 {
		t.Fatalf("events while the review is owed = %v", kinds)
	}
	if w.pub.count() != 0 || len(w.installs(t)) != 0 {
		t.Fatalf("published %d / installed %d before the verdict", w.pub.count(), len(w.installs(t)))
	}
	if owed.Facts.Path != types.InstallReviewPathOperation || owed.Facts.OperationID != operation.OperationID {
		t.Fatalf("owed facts = %+v", owed.Facts)
	}
	// Management's packet is served from the operation's persisted draft.
	payload, _ := json.Marshal(yaegikernel.InstallPacketPayload{ReleaseID: release})
	if _, err := w.fx.rt.installPacketEgress(context.Background(), agentprofile.Management, toolregistry.ExecutionContext{OwnerID: w.owner(), ComputerID: w.fx.computerID}, payload); err != nil {
		t.Fatalf("packet = %v", err)
	}
	// The verdict is management's own, on its own package operation (FM4):
	// the operation continues through publish, install and package_applied.
	if out, err := w.judge(t, release, true, ""); err != nil || out["verdict"] != installreview.VerdictAccept {
		t.Fatalf("verdict = %v, %v", out, err)
	}
	applied := w.fx.waitForState(t, operation.OperationID, selfdev.StateApplied)
	if applied.PackageReleaseID != release || w.pub.count() != 1 || len(w.installs(t)) != 1 {
		t.Fatalf("applied = %+v, publishes %d, installs %d", applied, w.pub.count(), len(w.installs(t)))
	}
	if kinds := w.fx.countEventKinds(); kinds[computerevent.EventMaterializationStarted] != 1 || kinds[computerevent.EventPackageApplied] != 1 {
		t.Fatalf("events = %v", kinds)
	}
}

func TestPackageOperationClosesFailedWhenManagementDoesNotJudgeInTime(t *testing.T) {
	w := newOpReviewWorld(t, "review-timeout")
	operation := w.propose(t, "op-pkg-review-timeout")
	release := w.release(t, operation)
	w.waitOwed(t, release)
	// FM1: past 45 minutes the operation fails and nothing is sealed.
	w.advance(types.InstallReviewOwedDeadline + time.Minute)
	w.pass()
	failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, installReviewNotJudged) {
		t.Fatalf("terminal error = %q", failed.TerminalError)
	}
	if w.pub.count() != 0 || len(w.installs(t)) != 0 {
		t.Fatal("a timed-out review published or installed")
	}
	if recs, _ := w.fx.rt.store.ListCommitmentRecords(context.Background(), w.owner(), w.fx.computerID, installreview.AddresseePrefix+release, 50); len(recs) != 0 {
		t.Fatalf("a timeout sealed %d record(s)", len(recs))
	}
	// The owed record closed with the operation: management cannot judge a
	// release that will never install.
	if _, err := w.judge(t, release, true, ""); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("late verdict = %v", err)
	}
	payload, _ := json.Marshal(yaegikernel.InstallPacketPayload{ReleaseID: release})
	if _, err := w.fx.rt.installPacketEgress(context.Background(), agentprofile.Management, toolregistry.ExecutionContext{OwnerID: w.owner(), ComputerID: w.fx.computerID}, payload); err == nil {
		t.Fatal("a packet was served for a closed review")
	}
}

func TestPackageOperationFailsWithManagementsReasonOnARejection(t *testing.T) {
	w := newOpReviewWorld(t, "review-reject")
	operation := w.propose(t, "op-pkg-review-reject")
	release := w.release(t, operation)
	w.waitOwed(t, release)
	if out, err := w.judge(t, release, false, "it reads notes it has no need of"); err != nil || out["verdict"] != installreview.VerdictReject {
		t.Fatalf("verdict = %v, %v", out, err)
	}
	failed := w.fx.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if !strings.Contains(failed.TerminalError, "it reads notes it has no need of") {
		t.Fatalf("terminal error = %q; want management's reason", failed.TerminalError)
	}
	if w.pub.count() != 0 || len(w.installs(t)) != 0 {
		t.Fatal("a rejected package was published or installed")
	}
}
