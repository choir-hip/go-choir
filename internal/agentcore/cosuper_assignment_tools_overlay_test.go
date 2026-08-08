package agentcore

import (
	"context"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
	"testing"
)

type fixedAssignmentHandleResolver struct{ runID, capsuleID, handle string }

func (f fixedAssignmentHandleResolver) AssignmentHandle(r, c string) (string, error) {
	if r != f.runID || c != f.capsuleID {
		return "", fmt.Errorf("unexpected binding")
	}
	return f.handle, nil
}

type fixedAssignmentLookup struct{ assignment types.CoSuperAssignment }

func (f fixedAssignmentLookup) GetCoSuperAssignment(_ context.Context, o, c, id string, n uint64) (types.CoSuperAssignment, error) {
	a := f.assignment
	if o != a.Binding.OwnerID || c != a.Binding.ComputerID || id != a.AssignmentID || n != a.Binding.Attempt {
		return a, fmt.Errorf("unexpected lookup")
	}
	return a, nil
}
func TestAssignedCoSuperToolOverlayIsExactRunOnly(t *testing.T) {
	ctx := context.Background()
	exec := capsule.NewExecutor("", "", "", 0)
	rt := &Runtime{capsuleExecutor: exec}
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	base := rt.ToolRegistryForProfile(agentprofile.CoSuper)
	legacy := &types.RunRecord{RunID: "legacy", AgentID: "co-super:legacy", AgentProfile: "co-super", AgentRole: "co-super"}
	got, h, err := rt.assignedCoSuperToolOverlay(ctx, legacy, base)
	if err == nil || h != "" || got != nil {
		t.Fatalf("unassigned overlay=(%p,%q,%v), want hard refusal", got, h, err)
	}
	o, c, tr, r, cap, id, opaque := "owner", "computer", "trajectory", "run", "capsule", "assignment", "opaque"
	b := types.CoSuperAssignmentBinding{OwnerID: o, ComputerID: c, TrajectoryID: tr, ParentAgentID: "super:owner", ParentRunID: "parent", ParentDecisionID: "decision:" + objectgraph.SHA256([]byte("decision")), ParentControlID: "control", ParentWorkItemID: "parent-work", AssignedWorkItemID: "assigned-work", AssignedAgentID: "co-super:assigned", Kind: types.CoSuperAssignmentImplementation, Attempt: 1, ScopeDigest: objectgraph.SHA256([]byte("scope")), RequestDigest: objectgraph.SHA256([]byte("request")), CapabilityDigest: objectgraph.SHA256([]byte("capability")), ExecutionHandleDigest: objectgraph.SHA256([]byte("handle")), SubjectDigest: objectgraph.SHA256([]byte("subject")), SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject")), Writable: true, CapsuleID: cap, NetworkMode: types.CoSuperCapsuleNetworkForbidden, FilesystemMode: types.CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay}
	a := types.CoSuperAssignment{AssignmentID: id, Binding: b, Disposition: types.CoSuperAssignmentBound, CapsuleDisposition: types.CoSuperCapsuleActive, BoundRunID: r}
	rt.assignmentLookup = fixedAssignmentLookup{a}
	rt.assignmentHandleResolver = fixedAssignmentHandleResolver{r, cap, opaque}
	run := &types.RunRecord{RunID: r, AgentID: b.AssignedAgentID, TrajectoryID: tr, AgentProfile: "co-super", AgentRole: "co-super", OwnerID: o, SandboxID: c, Metadata: map[string]any{"assignment_id": id, "assignment_attempt": 1, "assignment_kind": string(b.Kind), "assigned_work_item_id": b.AssignedWorkItemID, "capsule_id": cap, "capability_digest": b.CapabilityDigest, "execution_handle_digest": b.ExecutionHandleDigest, "request_digest": b.RequestDigest, "source_artifact_ref": b.SourceArtifactRef, "source_candidate_id": b.SourceCandidateID}}
	overlay, resolved, err := rt.assignedCoSuperToolOverlay(ctx, run, base)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != opaque {
		t.Fatal("handle mismatch")
	}
	for _, n := range []string{"capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir", "record_assignment_result"} {
		if _, ok := overlay.Lookup(n); !ok {
			t.Errorf("missing %s", n)
		}
	}
	for _, n := range []string{"read_file", "glob", "grep", "save_evidence", "verify_model_capability", "update_coagent", "commit_transaction", "append_computer_event", "materialize_self_development", "create_checkpoint"} {
		if _, ok := overlay.Lookup(n); ok {
			t.Errorf("host tool %s", n)
		}
	}
}
