package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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
	assignmentID, capability := "assignment-l2", "opaque-l2"
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + assignmentID + "-1", AssignmentID: assignmentID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
			ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
			ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
			ParentWorkItemID: seed.ParentWorkID, AssignedWorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0],
			Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability(capability), ExecutionHandleDigest: objectgraph.SHA256([]byte(capability)),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
			Writable:          true, CapsuleID: "capsule-l2",
			NetworkMode:    types.EngineeringCapsuleNetworkEcosystemProxy,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "pip install and replicate"},
	}
	open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatalf("store refused an ecosystem_proxy implementation binding: %v", err)
	}
	got, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, assignmentID, 1)
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

// docs/problems/delegated-cast-authority-dies-with-the-caster-turn-2026-10-10.md:
// each Texture request opens another management work item, so the second
// cast was refused "caster holds multiple open work items". Failure modes
// pinned: a run that names its work item still gets the ambiguity refusal;
// the named item is ignored for another open item; a closed or foreign item
// is chosen.
func TestDelegatedCastParentWorkIsTheCastingTurnsOwnItem(t *testing.T) {
	const caster = "management:owner"
	items := []types.WorkItemRecord{
		{WorkItemID: "first-request", AssignedAgentID: caster, Status: types.WorkItemOpen},
		{WorkItemID: "second-request", AssignedAgentID: caster, Status: types.WorkItemOpen},
		{WorkItemID: "done-request", AssignedAgentID: caster, Status: types.WorkItemCompleted},
		{WorkItemID: "texture-item", AssignedAgentID: "texture:doc", Status: types.WorkItemOpen},
	}
	if work, err := delegatedCastParentWork(items, caster, "second-request"); err != nil || work == nil || work.WorkItemID != "second-request" {
		t.Fatalf("named item: %+v, %v", work, err)
	}
	for _, named := range []string{"done-request", "texture-item", "missing"} {
		if work, err := delegatedCastParentWork(items, caster, named); err != nil || work != nil {
			t.Fatalf("named %s: got %+v, %v; want no parent work", named, work, err)
		}
	}
	if _, err := delegatedCastParentWork(items, caster, ""); err == nil {
		t.Fatal("unnamed run with two open items was not refused")
	}
	if work, err := delegatedCastParentWork(items[1:], caster, ""); err != nil || work == nil || work.WorkItemID != "second-request" {
		t.Fatalf("unnamed run with one open item: %+v, %v", work, err)
	}
}
