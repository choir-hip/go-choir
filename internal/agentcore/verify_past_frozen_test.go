package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Verify on an operation past frozen, failure modes written before the code:
//
//	V1  Verify on an applied operation (the engineer's final report after an
//	    install review) fails as a digest mismatch, with the draft digest or
//	    the finalized one, so the model retries digests instead of stopping;
//	V2  that refusal stamps last_intent_error on the applied operation, a
//	    stale false alarm the owner reads as its blocker;
//	V3  an operation that reaches applied keeps a stale last_intent_error;
//	V4  Verify on a frozen operation changes (still refuses a wrong digest
//	    and stamps it).

func TestVerifyOnAnAppliedOperationIsRefusedClearlyWithoutAStamp(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, run := retainedFixture(t, "owner-verify-applied")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateFrozen, "")
	if _, err := rt.cancelBoundEngineeringRun(withBuildBudgetEnd(ctx), run, activationBudgetExceededReason); err != nil {
		t.Fatal(err)
	}
	computerID := assignment.Binding.ComputerID
	// V4: while frozen, a wrong digest is the old refusal and it is stamped.
	wrong := yaegikernel.StagedIntent{LocalID: "v1", TargetRef: assignment.AssignmentID, Decision: "pass", BundleDigest: strings.Repeat("0", 64)}
	if _, err := verifyReduction(rt, assignment).commitVerifyIntent(ctx, wrong); err == nil || !strings.Contains(err.Error(), "bundle digest does not match") {
		t.Fatalf("V4: frozen verify = %v", err)
	}
	if got := operationState(t, rt, computerID, operationID).LastIntentError; !strings.Contains(got, "bundle digest does not match") {
		t.Fatalf("V4: frozen refusal not stamped: %q", got)
	}
	// The operation is applied (its stamp cleared by the apply).
	if _, err := rt.store.DB().Exec(`UPDATE self_development_operations SET state=?, last_intent_error='' WHERE computer_id=? AND operation_id=?`, selfdev.StateApplied, computerID, operationID); err != nil {
		t.Fatal(err)
	}
	// V1, V2: the draft digest and the finalized digest both get the clear refusal.
	for name, digest := range map[string]string{"finalized": strings.Repeat("f", 64), "draft": strings.Repeat("0", 64)} {
		intent := yaegikernel.StagedIntent{LocalID: "v1", TargetRef: assignment.AssignmentID, Decision: "pass", BundleDigest: digest}
		_, err := verifyReduction(rt, assignment).commitVerifyIntent(ctx, intent)
		if err == nil || !strings.Contains(err.Error(), "its candidate was already verified and decided") || !strings.Contains(err.Error(), "needs no Verify") || strings.Contains(err.Error(), "digest") {
			t.Fatalf("V1 %s: %v", name, err)
		}
		if got := operationState(t, rt, computerID, operationID).LastIntentError; got != "" {
			t.Fatalf("V2 %s: an applied operation was stamped: %q", name, got)
		}
	}
}

func TestApplyClearsAStaleIntentError(t *testing.T) {
	ctx := context.Background()
	rt, _, assignment, _ := retainedFixture(t, "owner-apply-clears")
	operationID := seedAssignmentOperation(t, rt, assignment, selfdev.StateMaterializing, "verify: verified bundle draft binding mismatch")
	computerID := assignment.Binding.ComputerID
	if got := operationState(t, rt, computerID, operationID).LastIntentError; got == "" {
		t.Fatal("seed lost its stamp")
	}
	if _, err := rt.selfdevOperations.Transition(ctx, computerID, operationID, selfdev.StateMaterializing, selfdev.StateApplied, nil); err != nil {
		t.Fatal(err)
	}
	if got := operationState(t, rt, computerID, operationID); got.State != selfdev.StateApplied || got.LastIntentError != "" {
		t.Fatalf("V3: %s %q", got.State, got.LastIntentError)
	}
}
