package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestVerifierGateRejectsNonVerifierSlot pins the host-side verifier gates the
// P3-parity Definition requires: inspectSelfDevelopmentBundle and
// recordSelfDevelopmentVerification must refuse any run whose co_super_slot is
// not the verifier slot, before any bundle or store work.
func TestVerifierGateRejectsNonVerifierSlot(t *testing.T) {
	for _, slot := range []string{"", "implementation", "researcher"} {
		rec := &types.RunRecord{
			RunID: "run-gate", Metadata: map[string]any{runMetadataCoSuperSlot: slot},
		}
		if _, err := inspectSelfDevelopmentBundle(context.Background(), &CapsuleToolCtx{}, rec, "op", "sha256:abc"); err == nil {
			t.Errorf("inspectSelfDevelopmentBundle accepted slot %q", slot)
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
	_, err := inspectSelfDevelopmentBundle(context.Background(), &CapsuleToolCtx{}, rec, "op", "sha256:abc")
	if err == nil {
		t.Fatal("inspectSelfDevelopmentBundle must fail on missing binding")
	}
	if got := err.Error(); got == "bundle inspection is restricted to the co-super verifier slot" {
		t.Fatalf("verifier slot was refused: %v", err)
	}
	_, err = recordSelfDevelopmentVerification(context.Background(), &CapsuleToolCtx{}, rec, "op", "sha256:abc", "pass", []string{"ref"})
	if err == nil {
		t.Fatal("recordSelfDevelopmentVerification must fail on missing authority")
	}
	if got := err.Error(); got == "verification recording is restricted to the co-super verifier slot" {
		t.Fatalf("verifier slot was refused: %v", err)
	}
}
