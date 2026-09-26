package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// R3c — the desk-cell carrier path (rlmReductionForDeskCall), not a hand-built
// reduction, drives a management cell's staged Cast into delegated-cast
// admission: the commitment record mints first and openDelegatedCastAssignment
// runs under the caster's trajectory-bound authority. On platforms where the
// capsule executor is a stub, admission stops at the substrate boundary —
// never at the authority layer.

func TestR3cDeskCastReachesDelegatedAdmission(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "user-alice", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)

	casterRun, err := s.GetRun(ctx, seed.ParentRunID)
	if err != nil {
		t.Fatalf("caster run: %v", err)
	}
	// execCtx as the runtime installs for a management run on the cell carrier:
	// RunRecord is the caster's, channel is the caster's channel (the seeded
	// parent agent's own channel).
	execCtx := toolregistry.ExecutionContext{
		RunID:      casterRun.RunID,
		AgentID:    seed.ParentAgentID,
		OwnerID:    seed.OwnerID,
		Profile:    "management",
		Role:       "management",
		ChannelID:  seed.ParentAgentID,
		ComputerID: seed.ComputerID,
		RunRecord:  &casterRun,
	}
	rcCtx := toolregistry.WithExecutionContext(ctx, execCtx)

	reduction := rlmReductionForDeskCall(rcCtx, rt)
	if !reduction.active {
		t.Fatal("desk reduction inactive — management carrier reduction must be active on cells")
	}
	if reduction.rec == nil {
		t.Fatal("desk reduction lost the caster RunRecord; delegated cast admission needs it")
	}
	if reduction.scope.FromRole != "management" {
		t.Fatalf("desk reduction FromRole=%q, want management", reduction.scope.FromRole)
	}

	intent := yaegikernel.StagedIntent{
		LocalID: "cast-1", Kind: yaegikernel.IntentCast, ToDesk: "engineering",
		Objective: "bounded delegated assignment",
	}
	// commitTray path via commitActIntent: mints the commitment, then reaches
	// delegated admission on the caster's own run/work authority.
	_, castErr := reduction.commitActIntent(rcCtx, intent)

	if id := commitmentCanonicalID(t, s, reduction.scope, intent); id == "" {
		t.Fatal("delegated cast commitment absent from the ledger")
	}
	if castErr == nil {
		t.Log("delegated cast opened and spawned on live executor")
		return
	}
	// Admission reached the delegated-authority path — the error names the
	// capsule substrate (preflight/spawn), never an authority or validation
	// rejection of the cast itself.
	for _, rejected := range []string{
		"requires a trajectory-bound caster", "requires objective, kind, and the commitment control",
		"caster has no open work item", "caster holds multiple open work items",
	} {
		if strings.Contains(castErr.Error(), rejected) {
			t.Fatalf("delegated cast rejected at the authority layer: %v", castErr)
		}
	}
	t.Logf("delegated cast reached admission; substrate stop: %v", castErr)
}
