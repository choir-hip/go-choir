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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
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
