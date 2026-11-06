package agentcore

// R2 and the guest half of R1 (docs/design/selfdev-apply-and-revert-2026-11-06.md
// 3.2, 3.3, 3.4): nothing parks, and a platform computer does not self-develop.
//
// Ways these could pass while the product is still wrong, and what each test
// does about it:
//   - the close is asserted on a fabricated state: the operation goes through
//     the real Freeze/Verify fixture and the real trigger, and the terminal
//     reason is read back from the operation row;
//   - a decided operation is swept: the boot close runs between the decision
//     event's append and the operation's transition, and the decision still
//     lands as applied.

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const paperComputerForTest = "computer-1355b83bced1613eb048ee28d66688a8" // the Autopaper paper computer

func TestAttemptThatCannotDecideClosesTheOperationFailedWithItsReason(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	operation := f.verify(t, f.freeze(t, "park-no-bundle", "a candidate whose frozen bundle is gone"))
	// The poster cannot be asked without the frozen bundle (it names the
	// source patch the host builds): a refusal before any decision.
	if err := os.Remove(filepath.Join(f.updaterRoot, "incoming", operation.BundleDigest, "bundle.json")); err != nil {
		t.Fatal(err)
	}
	f.rt.triggerStandingConsensus(operation.OperationID, "run:management-p4")
	closed := f.waitForState(t, operation.OperationID, selfdev.StateFailed)
	for name, got := range map[string]string{"terminal_error": closed.TerminalError, "last_intent_error": closed.LastIntentError} {
		if !strings.Contains(got, "frozen bundle unavailable") {
			t.Fatalf("closed operation %s = %q, want the refusal as the reason", name, got)
		}
	}
	// A closed candidate keeps its bundle: management may propose again.
	if closed.BundleDigest != operation.BundleDigest {
		t.Fatalf("closing lost the bundle: %q", closed.BundleDigest)
	}
	// Closing an operation already out of awaiting_approval is a no-op.
	f.rt.closeUndecidedSelfDevelopmentOperation(ctx, f.computerID, operation.OperationID, "again")
	if again, _ := f.operations.Get(ctx, f.computerID, operation.OperationID); again.TerminalError != closed.TerminalError {
		t.Fatalf("a second close rewrote the reason: %q", again.TerminalError)
	}
}

func TestBootCloseClosesAnUndecidedCandidateAndSkipsADecidedUnprojectedOne(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()

	// An undecided candidate: the boot close fails it with the restart reason.
	// (One operation is open per computer, so the two cases run in turn.)
	undecided := f.verify(t, f.freeze(t, "park-undecided", "never decided"))
	f.rt.closeUndecidedSelfDevelopmentOperationsAtBoot(ctx, f.computerID)
	got, err := f.operations.Get(ctx, f.computerID, undecided.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != selfdev.StateFailed || got.TerminalError != selfdevUndecidedInterruptedByRestart || got.LastIntentError != selfdevUndecidedInterruptedByRestart {
		t.Fatalf("the sweep left the undecided operation at %s %q / %q, want failed %q", got.State, got.TerminalError, got.LastIntentError, selfdevUndecidedInterruptedByRestart)
	}

	// A decided one: its decision event is on the tape but the operation row
	// has not caught up (the sweep runs inside the handler's window).
	decided := f.verify(t, f.freeze(t, "park-decided", "decided, not yet projected"))
	swept := false
	f.rt.selfdevDecisionBeforeTransition = func(ctx context.Context, operationID string) {
		if operationID != decided.OperationID {
			return
		}
		current, err := f.operations.Get(ctx, f.computerID, operationID)
		if err != nil || current.State != selfdev.StateAwaitingApproval {
			t.Errorf("the operation was not awaiting approval inside the window: %+v %v", current, err)
		}
		f.rt.closeUndecidedSelfDevelopmentOperationsAtBoot(ctx, f.computerID)
		swept = true
	}
	_, err = f.rt.runStandingConsensus(ctx, decided.OperationID, "run:management-p4")
	f.rt.selfdevDecisionBeforeTransition = nil
	if err != nil {
		t.Fatalf("consensus on the decided candidate: %v", err)
	}
	if !swept {
		t.Fatal("the boot close never ran inside the decision window")
	}
	// The sweep did not fail the decided operation: its decision stands.
	f.waitForState(t, decided.OperationID, selfdev.StateApplied)
}

func TestPlatformComputerDoesNotSelfDevelopOnTheGuestSide(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	// A runtime that is the paper computer, sharing the fixture's mode
	// authority: the freeze check and the attempt refuse before any other
	// work, whatever the mode row says.
	paper := &Runtime{selfdevControl: f.rt.selfdevControl, selfdevComputerID: paperComputerForTest}
	if err := paper.selfDevelopmentProposalAuthorized(ctx); err == nil || !strings.Contains(err.Error(), "platform computer does not self-develop") {
		t.Fatalf("freeze on the paper computer: %v", err)
	}
	if _, err := paper.runStandingConsensus(ctx, "op-paper", "run:paper"); err == nil || !strings.Contains(err.Error(), "platform computer does not self-develop") {
		t.Fatalf("attempt on the paper computer: %v", err)
	}
	// An ordinary computer with no row (apply) may propose, and off may not.
	if err := f.rt.selfDevelopmentProposalAuthorized(ctx); err != nil {
		t.Fatalf("freeze with no mode row: %v", err)
	}
	f.writeMode(t, "off")
	if err := f.rt.selfDevelopmentProposalAuthorized(ctx); err == nil || !strings.Contains(err.Error(), "off") {
		t.Fatalf("freeze under mode off: %v", err)
	}
	f.writeMode(t, "apply")
	if err := f.rt.selfDevelopmentProposalAuthorized(ctx); err != nil {
		t.Fatalf("freeze under mode apply: %v", err)
	}
}

// The boot close tells Texture, as the attempt close does: the owner reading
// the document learns the candidate closed, and why.
func TestBootCloseWakesTheCandidatesTexture(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	trajectoryID, inbox := seedOwnerActDocument(t, f.store, "owner", f.computerID, "doc-boot-close")
	f.trajectoryID = trajectoryID
	operation := f.verify(t, f.freeze(t, "park-boot-wake", "closed at boot"))
	f.rt.closeUndecidedSelfDevelopmentOperationsAtBoot(ctx, f.computerID)
	got := ownerActRecords(t, f.store, "owner", f.computerID, inbox)
	if len(got) != 1 {
		t.Fatalf("the boot close left %d owner-act records, want 1: %+v", len(got), got)
	}
	checkOwnerActRecord(t, got[0], inbox, operation.OperationID, types.OwnerActFail)
	if !strings.Contains(got[0].Directive.Body, "before a decision") || !strings.Contains(got[0].Directive.Body, selfdevUndecidedInterruptedByRestart) {
		t.Fatalf("the wake does not say the candidate closed before a decision, and why: %q", got[0].Directive.Body)
	}
}

// A computer that cannot run the attempt at all (no decision authority wired)
// still closes the operation instead of parking it.
func TestTriggerWithoutADecisionAuthorityClosesTheOperation(t *testing.T) {
	f := newP4Fixture(t)
	operation := f.verify(t, f.freeze(t, "park-no-authority", "no decision authority"))
	bare := &Runtime{selfdevOperations: f.rt.selfdevOperations, store: f.rt.store, selfdevComputerID: f.computerID}
	bare.triggerStandingConsensus(operation.OperationID, "run:management-p4")
	closed := f.waitForState(t, operation.OperationID, selfdev.StateFailed)
	if !strings.Contains(closed.TerminalError, "decision authority") || !strings.Contains(closed.LastIntentError, "decision authority") {
		t.Fatalf("closed without the reason: terminal=%q intent=%q", closed.TerminalError, closed.LastIntentError)
	}
}

// Reject is the fail-safe decision: the owner can reject a still-undecided
// candidate whatever the mode, off included.
func TestOwnerRejectWorksOnAnOffComputer(t *testing.T) {
	f := newP4Fixture(t)
	ctx := context.Background()
	operation := f.verify(t, f.freeze(t, "park-reject-off", "rejected under off"))
	f.writeMode(t, "off")
	request := ownerActDecisionRequest(t, f, operation, "reject", "reject-under-off")
	handler := &APIHandler{rt: f.rt}
	decided, status, err := handler.decideSelfDevelopment(ctx, "owner", f.computerID, operation.OperationID, request)
	if status != http.StatusOK || err != nil || decided.State != selfdev.StateRejected {
		t.Fatalf("reject under mode off answered %d %v: %+v", status, err, decided)
	}
}
