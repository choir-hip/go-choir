package agentcore

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type fixedAssignmentHandleResolver struct{ runID, capsuleID, handle string }

func (f fixedAssignmentHandleResolver) AssignmentHandle(r, c string) (string, error) {
	if r != f.runID || c != f.capsuleID {
		return "", fmt.Errorf("unexpected binding")
	}
	return f.handle, nil
}

type fixedAssignmentLookup struct{ assignment types.EngineeringAssignment }

func (f fixedAssignmentLookup) GetEngineeringAssignment(_ context.Context, o, c, id string, n uint64) (types.EngineeringAssignment, error) {
	a := f.assignment
	if o != a.Binding.OwnerID || c != a.Binding.ComputerID || id != a.AssignmentID || n != a.Binding.Attempt {
		return a, fmt.Errorf("unexpected lookup")
	}
	return a, nil
}
func TestAssignedEngineeringToolOverlayIsExactRunOnly(t *testing.T) {
	ctx := context.Background()
	exec := capsule.NewExecutor("", "", "", 0)
	rt := &Runtime{capsuleExecutor: exec}
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	base := rt.ToolRegistryForProfile(agentprofile.Engineering)
	legacy := &types.RunRecord{RunID: "legacy", AgentID: "engineering:legacy", AgentProfile: "engineering", AgentRole: "engineering"}
	got, h, err := rt.assignedEngineeringToolOverlay(ctx, legacy, base)
	if err == nil || h != "" || got != nil {
		t.Fatalf("unassigned overlay=(%p,%q,%v), want hard refusal", got, h, err)
	}
	o, c, tr, r, cap, id, opaque := "owner", "computer", "trajectory", "run", "capsule", "assignment", "opaque"
	b := types.EngineeringAssignmentBinding{OwnerID: o, ComputerID: c, TrajectoryID: tr, ParentAgentID: "management:owner", ParentRunID: "parent", ParentDecisionID: "decision:" + objectgraph.SHA256([]byte("decision")), ParentControlID: "control", ParentWorkItemID: "parent-work", AssignedWorkItemID: "assigned-work", AssignedAgentID: "engineering:assigned", Kind: types.EngineeringAssignmentImplementation, Attempt: 1, ScopeDigest: objectgraph.SHA256([]byte("scope")), RequestDigest: objectgraph.SHA256([]byte("request")), CapabilityDigest: objectgraph.SHA256([]byte("capability")), ExecutionHandleDigest: objectgraph.SHA256([]byte("handle")), SubjectDigest: objectgraph.SHA256([]byte("subject")), SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject")), Writable: true, CapsuleID: cap, NetworkMode: types.EngineeringCapsuleNetworkForbidden, FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay}
	a := types.EngineeringAssignment{AssignmentID: id, Binding: b, Disposition: types.EngineeringAssignmentBound, CapsuleDisposition: types.EngineeringCapsuleActive, BoundRunID: r}
	rt.assignmentLookup = fixedAssignmentLookup{a}
	rt.assignmentHandleResolver = fixedAssignmentHandleResolver{r, cap, opaque}
	run := &types.RunRecord{RunID: r, AgentID: b.AssignedAgentID, TrajectoryID: tr, AgentProfile: "engineering", AgentRole: "engineering", OwnerID: o, ComputerID: c, Metadata: map[string]any{"assignment_id": id, "assignment_attempt": 1, "assignment_kind": string(b.Kind), "assigned_work_item_id": b.AssignedWorkItemID, "capsule_id": cap, "capability_digest": b.CapabilityDigest, "execution_handle_digest": b.ExecutionHandleDigest, "request_digest": b.RequestDigest, "source_artifact_ref": b.SourceArtifactRef, "source_candidate_id": b.SourceCandidateID}}
	overlay, resolved, err := rt.assignedEngineeringToolOverlay(ctx, run, base)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != opaque {
		t.Fatal("handle mismatch")
	}
	if got, want := registryToolNames(overlay), []string{"capsule_go_eval"}; !slices.Equal(got, want) {
		t.Errorf("assigned overlay tools = %v, want exact %v", got, want)
	}
	for _, n := range []string{"capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir", "read_file", "glob", "grep", "save_evidence", "verify_model_capability", "append_computer_event", "materialize_self_development", "create_checkpoint", "propose_effect", "finalize_effect", "update_coagent", "commit_transaction", "inspect_self_development_bundle", "record_self_development_verification", "record_assignment_result"} {
		if _, ok := overlay.Lookup(n); ok {
			t.Errorf("forbidden tool %s", n)
		}
	}
}
