package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type absentAssignmentCapsule struct{}

func (absentAssignmentCapsule) Spawn(context.Context, capsule.SpawnSpec) (*capsule.Capsule, error) {
	return nil, fmt.Errorf("spawn unavailable after restart")
}
func (absentAssignmentCapsule) MintCapabilityHandle(string, capsule.AgentRole, string, string, time.Duration) (*capsule.Capability, error) {
	return nil, fmt.Errorf("mint unavailable after restart")
}
func (absentAssignmentCapsule) RevokeCapability(string, string) error { return nil }
func (absentAssignmentCapsule) ForceDestroy(context.Context, string) error {
	return fmt.Errorf("force destroy unavailable after restart")
}
func (absentAssignmentCapsule) ExtractGranted(context.Context, string, string) ([]capsule.FileChange, error) {
	return nil, fmt.Errorf("extract unavailable after restart")
}
func (absentAssignmentCapsule) ResolveGrantedWorktreeDigest(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("digest unavailable after restart")
}
func (absentAssignmentCapsule) ResolveExecutionReceipts([]string) ([]capsule.ExecutionReceipt, error) {
	return nil, fmt.Errorf("receipts unavailable after restart")
}
func (absentAssignmentCapsule) AssignmentHandle(string, string) (string, error) {
	return "", fmt.Errorf("capsule assignment capability unavailable")
}
func (absentAssignmentCapsule) InspectCapsuleRaw(string) (*capsule.CapsuleDiagnostics, error) {
	return nil, fmt.Errorf("capsule not found")
}
func (absentAssignmentCapsule) HasCapsule(string) bool { return false }
func (absentAssignmentCapsule) CleanupOrphanedCapsule(context.Context, string) error {
	return nil
}
func (absentAssignmentCapsule) PersistRevocationReceipt(agentRunID, capabilityDigest, capsuleID, intentRef string) (capsule.CapsuleRevocationReceipt, error) {
	receipt := capsule.CapsuleRevocationReceipt{
		AgentRunID: agentRunID, AssignmentCapabilityDigest: capabilityDigest, CapsuleID: capsuleID,
		IntentRef: intentRef, Disposition: "revoked", CapsuleAbsent: true, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	unsigned, err := json.Marshal(receipt)
	if err != nil {
		return capsule.CapsuleRevocationReceipt{}, err
	}
	receipt.ReceiptRef = "capsule-revoke:" + objectgraph.SHA256(unsigned)
	return receipt, nil
}

func TestRewarmAssignedCoSuperReconcilesAbsentCapsuleWithoutWake(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	var wakes []string
	rt.SetDispatchActor(func(_ context.Context, _, _, _, kind, content, _, _ string) error {
		wakes = append(wakes, kind+":"+content)
		return nil
	})
	seed, err := store.SeedCoSuperAssignmentAuthority(s, "owner-assignment", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignmentID := "assignment-boot-absent"
	capability := "opaque-boot-absent"
	capsuleID := "capsule-boot-absent"
	open := types.OpenCoSuperAssignmentRequest{
		CommandID: "command-open-" + assignmentID + "-1", AssignmentID: assignmentID,
		Binding: types.CoSuperAssignmentBinding{
			OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
			ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
			ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
			ParentWorkItemID: seed.ParentWorkID, AssignedWorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0],
			Kind: types.CoSuperAssignmentImplementation, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
			CapabilityDigest: store.DigestCoSuperOpaqueCapability(capability), ExecutionHandleDigest: objectgraph.SHA256([]byte(capability)),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
			Writable:          true, CapsuleID: capsuleID,
			NetworkMode:    types.CoSuperCapsuleNetworkForbidden,
			FilesystemMode: types.CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "bounded delegated assignment"},
	}
	open.CommandDigest, err = store.ComputeOpenCoSuperAssignmentDigest(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	runID := "run:" + assignmentID
	run := types.RunRecord{
		RunID: runID, AgentID: open.Binding.AssignedAgentID, ChannelID: open.Binding.AssignedAgentID,
		RequestedByRunID: open.Binding.ParentRunID, TrajectoryID: open.Binding.TrajectoryID,
		AgentProfile: "co-super", AgentRole: "co-super", OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		State: types.RunPending, Prompt: open.AssignedWork.Objective,
		Metadata: map[string]any{
			"work_item_ids": []string{open.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": open.Binding.AssignedWorkItemID,
			"requested_by_agent_id": open.Binding.ParentAgentID, "requested_by_profile": "super",
			"assignment_id": assignmentID, "assignment_attempt": 1, "assignment_kind": string(open.Binding.Kind),
			"assigned_work_item_id": open.Binding.AssignedWorkItemID, "parent_work_item_id": open.Binding.ParentWorkItemID,
			"parent_decision_id": open.Binding.ParentDecisionID, "parent_control_id": open.Binding.ParentControlID,
			"capsule_id": open.Binding.CapsuleID, "scope_digest": open.Binding.ScopeDigest, "request_digest": open.Binding.RequestDigest,
			"capability_digest": open.Binding.CapabilityDigest, "execution_handle_digest": open.Binding.ExecutionHandleDigest,
			"subject_digest": open.Binding.SubjectDigest, "source_artifact_ref": open.Binding.SourceArtifactRef,
			"source_candidate_id": open.Binding.SourceCandidateID,
		},
	}
	bind := types.BindCoSuperAssignmentRequest{
		CommandID: "command-bind-" + assignmentID + "-1",
		OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: assignmentID,
		Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: capability, CapsuleID: capsuleID,
	}
	bind.CommandDigest, err = store.ComputeBindCoSuperAssignmentDigest(bind)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bind); err != nil {
		t.Fatal(err)
	}
	listed, listErr := s.ListLifecycleRunsByState(ctx, "", rt.TextureComputerID(), types.RunPending)
	if listErr != nil {
		t.Fatal(listErr)
	}
	found := false
	for _, rec := range listed {
		if rec.RunID == runID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("bound CoSuper run missing from lifecycle pending index: %+v", listed)
	}
	runBefore, loadErr := s.GetLifecycleRun(ctx, open.Binding.OwnerID, open.Binding.ComputerID, runID)
	if loadErr != nil {
		t.Fatalf("load bound run: %v", loadErr)
	}
	eligible, eligibilityErr := rt.lifecycleActivationBindingsEligible(ctx, &runBefore)
	if eligibilityErr != nil || !eligible {
		t.Fatalf("assigned CoSuper should remain eligible for boot reconcile: eligible=%t err=%v run=%+v", eligible, eligibilityErr, runBefore)
	}
	rt.rewarmInterruptedLifecycleActivations(ctx)
	rt.sweepOpenWorkItemActors(ctx)
	for _, wake := range wakes {
		if strings.HasPrefix(wake, "initial_dispatch:") {
			t.Fatalf("rewarm re-dispatched assigned CoSuper after restart: %v", wakes)
		}
	}
	assignment, err := s.GetCoSuperAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, assignmentID, 1)
	if err != nil || !assignment.Disposition.Terminal() || assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
		t.Fatalf("assignment fate=%+v err=%v", assignment, err)
	}
	got, err := s.GetLifecycleRun(ctx, open.Binding.OwnerID, open.Binding.ComputerID, runID)
	if err != nil || !got.State.Terminal() {
		t.Fatalf("run projection=%+v err=%v", got, err)
	}
}
