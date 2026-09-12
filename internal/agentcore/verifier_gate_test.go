package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestVerifierGateRejectsNonVerifierSlot pins the host-side verifier gate the
// P3-parity Definition requires: recordSelfDevelopmentVerification must
// refuse any run whose co_super_slot is not the verifier slot, before any
// bundle or store work. (The in-cell InspectBundle slot gate lives in
// yaegikernel and is covered by TestVerifierSlotExports; the former host-side
// inspect body was deleted as orphaned - the in-cell inspector is the sole
// implementation.)
func TestVerifierGateRejectsNonVerifierSlot(t *testing.T) {
	for _, slot := range []string{"", "implementation", "researcher"} {
		rec := &types.RunRecord{
			RunID: "run-gate", Metadata: map[string]any{runMetadataCoSuperSlot: slot},
		}
		if _, err := recordSelfDevelopmentVerification(context.Background(), &CapsuleToolCtx{}, rec, "op", "sha256:abc", "pass", []string{"ref"}); err == nil {
			t.Errorf("recordSelfDevelopmentVerification accepted slot %q", slot)
		}
	}
}

// TestVerifierGateReachesBindingCheck proves a verifier-slot run passes the
// slot gate and reaches the next binding check (exact operation/digest), not
// the export table. The failure must be the binding mismatch, not the slot
// refusal.
func TestVerifierGateReachesBindingCheck(t *testing.T) {
	rec := &types.RunRecord{
		RunID: "run-gate", Metadata: map[string]any{runMetadataCoSuperSlot: "verifier"},
	}
	_, err := recordSelfDevelopmentVerification(context.Background(), &CapsuleToolCtx{}, rec, "op", "sha256:abc", "pass", []string{"ref"})
	if err == nil {
		t.Fatal("recordSelfDevelopmentVerification must fail on missing authority")
	}
	if got := err.Error(); got == "verification recording is restricted to the co-super verifier slot" {
		t.Fatalf("verifier slot was refused: %v", err)
	}
}
