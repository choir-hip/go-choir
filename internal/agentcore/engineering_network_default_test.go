package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/design/engineering-network-grants-2026-10-10.md §4.0b (owner: "Let's
// do L2"). Failure modes pinned:
//   - implementation casts stay networkless (L2 never reaches engineering);
//   - verification gets a network (the verifier must rebuild offline);
//   - management's {"network":"forbidden"} withhold is ignored, or a spec
//     value other than an explicit withhold changes the mode;
//   - the store refuses an ecosystem_proxy binding, or the grant policy
//     digest does not change with the network mode (L2 invisible in evidence).
func TestEngineeringNetworkDefaultsToL2ForImplementationOnly(t *testing.T) {
	impl, verify := types.EngineeringAssignmentImplementation, types.EngineeringAssignmentVerification
	if got := engineeringCapsuleNetwork(impl, false); got != types.EngineeringCapsuleNetworkEcosystemProxy {
		t.Fatalf("implementation default = %q", got)
	}
	if got := engineeringCapsuleNetwork(impl, true); got != types.EngineeringCapsuleNetworkForbidden {
		t.Fatalf("withheld implementation = %q", got)
	}
	if got := engineeringCapsuleNetwork(verify, false); got != types.EngineeringCapsuleNetworkForbidden {
		t.Fatalf("verification = %q", got)
	}
	for spec, want := range map[string]bool{
		``:                              false,
		`not json`:                      false,
		`{"objective":"x"}`:             false,
		`{"network":"forbidden"}`:       true,
		`{"network":"none"}`:            true,
		`{"network":" OFF "}`:           true,
		`{"network":false}`:             true,
		`{"network":true}`:              false,
		`{"network":"full"}`:            false,
		`{"network":"ecosystem_proxy"}`: false,
	} {
		if got := castWithholdsNetwork(spec); got != want {
			t.Errorf("castWithholdsNetwork(%s) = %v, want %v", spec, got, want)
		}
	}
}

func TestStoreAcceptsAnEcosystemProxyImplementationBinding(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-l2", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-l2")
	open.Binding.NetworkMode = types.EngineeringCapsuleNetworkEcosystemProxy
	if open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatalf("store refused an ecosystem_proxy implementation binding: %v", err)
	}
	got, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, open.AssignmentID, 1)
	if err != nil || got.Binding.NetworkMode != types.EngineeringCapsuleNetworkEcosystemProxy {
		t.Fatalf("stored network mode = %q, %v", got.Binding.NetworkMode, err)
	}
	verbs := []string{"exec", "read_file"}
	l2 := store.ComputeEngineeringGrantPolicyDigest("engineering", verbs, types.EngineeringCapsuleNetworkEcosystemProxy, types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay, true)
	offline := store.ComputeEngineeringGrantPolicyDigest("engineering", verbs, types.EngineeringCapsuleNetworkForbidden, types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay, true)
	if l2 == offline {
		t.Fatal("grant policy digest ignores the network mode")
	}
}

// docs/problems/delegated-cast-authority-dies-with-the-caster-turn-2026-10-10.md
// (follow-up): the bound engineering run must carry the binding's parent
// fields, or the store's exact bind check refuses every cast. Failure mode
// pinned: the run the runtime builds for a records binding (C1e) fails the
// store's bind check.
func TestBuiltEngineeringRunPassesTheExactBindCheck(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-bindrun", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	open := store.SeedEngineeringOpenRequest(seed, 0, "opaque-bindrun")
	opened, err := s.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	agent, err := s.GetAgentByScope(ctx, seed.OwnerID, seed.ComputerID, open.Binding.AssignedAgentID)
	if err != nil {
		t.Fatal(err)
	}
	run := buildAssignedEngineeringRun(opened.Assignment, "run:"+open.AssignmentID, "build it", open.Binding.SourceArtifactRef)
	if err := store.ValidateEngineeringAssignmentRun(opened.Assignment, agent, run); err != nil {
		t.Fatalf("built run fails the exact bind check: %v", err)
	}
	if run.TrajectoryID != seed.TrajectoryID {
		t.Fatalf("built run trajectory = %q, want the binding's %q (kickEngineeringReconcileAfterPersist needs it)", run.TrajectoryID, seed.TrajectoryID)
	}
}
