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
func (absentAssignmentCapsule) MintCapabilityHandle(string, capsule.AgentRole, string, string, time.Duration, string) (*capsule.Capability, error) {
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

func TestReconcileEngineeringAssignmentCapsulesAfterRestartTerminalizesAbsentCapsule(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-assignment", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignmentID := "assignment-boot-sweep"
	capability := "opaque-boot-sweep"
	capsuleID := "capsule-boot-sweep"
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
			Writable:          true, CapsuleID: capsuleID,
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "bounded delegated assignment"},
	}
	open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	runID := "run:" + assignmentID
	run := types.RunRecord{
		RunID: runID, AgentID: open.Binding.AssignedAgentID, ChannelID: open.Binding.AssignedAgentID,
		RequestedByRunID: open.Binding.ParentRunID, TrajectoryID: open.Binding.TrajectoryID,
		AgentProfile: "engineering", AgentRole: "engineering", OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		State: types.RunPending, Prompt: open.AssignedWork.Objective,
		Metadata: map[string]any{
			"work_item_ids": []string{open.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": open.Binding.AssignedWorkItemID,
			"requested_by_agent_id": open.Binding.ParentAgentID, "requested_by_profile": "management",
			"assignment_id": assignmentID, "assignment_attempt": 1, "assignment_kind": string(open.Binding.Kind),
			"assigned_work_item_id": open.Binding.AssignedWorkItemID, "parent_work_item_id": open.Binding.ParentWorkItemID,
			"parent_decision_id": open.Binding.ParentDecisionID, "parent_control_id": open.Binding.ParentControlID,
			"capsule_id": open.Binding.CapsuleID, "scope_digest": open.Binding.ScopeDigest, "request_digest": open.Binding.RequestDigest,
			"capability_digest": open.Binding.CapabilityDigest, "execution_handle_digest": open.Binding.ExecutionHandleDigest,
			"subject_digest": open.Binding.SubjectDigest, "source_artifact_ref": open.Binding.SourceArtifactRef,
			"source_candidate_id": open.Binding.SourceCandidateID,
		},
	}
	bind := types.BindEngineeringAssignmentRequest{
		CommandID: "command-bind-" + assignmentID + "-1",
		OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: assignmentID,
		Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: capability, CapsuleID: capsuleID,
	}
	bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bind); err != nil {
		t.Fatal(err)
	}
	rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx)
	assignment, err := s.GetEngineeringAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, assignmentID, 1)
	if err != nil || !assignment.Disposition.Terminal() || assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
		t.Fatalf("assignment fate=%+v err=%v", assignment, err)
	}
	got, err := s.GetLifecycleRun(ctx, open.Binding.OwnerID, open.Binding.ComputerID, runID)
	if err != nil || !got.State.Terminal() {
		t.Fatalf("run projection=%+v err=%v", got, err)
	}
}

func TestReconcileSkipsTerminalUnboundCapsule(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-assignment", rt.TextureComputerID(), 2)
	if err != nil {
		t.Fatal(err)
	}
	openReq := func(assignmentID, capsuleID, capability string, index int) types.OpenEngineeringAssignmentRequest {
		req := types.OpenEngineeringAssignmentRequest{
			CommandID: "command-open-" + assignmentID + "-1", AssignmentID: assignmentID,
			Binding: types.EngineeringAssignmentBinding{
				OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
				ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
				ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
				ParentWorkItemID: seed.ParentWorkID, AssignedWorkItemID: seed.AssignedWorkIDs[index], AssignedAgentID: seed.AssignedAgentIDs[index],
				Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
				ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
				CapabilityDigest: store.DigestEngineeringOpaqueCapability(capability), ExecutionHandleDigest: objectgraph.SHA256([]byte(capability)),
				SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
				SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
				Writable:          true, CapsuleID: capsuleID,
				NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
				FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
			},
			AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[index]},
			AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[index], AssignedAgentID: seed.AssignedAgentIDs[index], Objective: "bounded delegated assignment"},
		}
		req.CommandDigest, _ = store.ComputeOpenEngineeringAssignmentDigest(req)
		return req
	}
	bindReq := func(open types.OpenEngineeringAssignmentRequest, runID, capability string) types.BindEngineeringAssignmentRequest {
		run := types.RunRecord{
			RunID: runID, AgentID: open.Binding.AssignedAgentID, ChannelID: open.Binding.AssignedAgentID,
			RequestedByRunID: open.Binding.ParentRunID, TrajectoryID: open.Binding.TrajectoryID,
			AgentProfile: "engineering", AgentRole: "engineering", OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
			State: types.RunPending, Prompt: open.AssignedWork.Objective,
			Metadata: map[string]any{
				"work_item_ids": []string{open.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": open.Binding.AssignedWorkItemID,
				"requested_by_agent_id": open.Binding.ParentAgentID, "requested_by_profile": "management",
				"assignment_id": open.AssignmentID, "assignment_attempt": 1, "assignment_kind": string(open.Binding.Kind),
				"assigned_work_item_id": open.Binding.AssignedWorkItemID, "parent_work_item_id": open.Binding.ParentWorkItemID,
				"parent_decision_id": open.Binding.ParentDecisionID, "parent_control_id": open.Binding.ParentControlID,
				"capsule_id": open.Binding.CapsuleID, "scope_digest": open.Binding.ScopeDigest, "request_digest": open.Binding.RequestDigest,
				"capability_digest": open.Binding.CapabilityDigest, "execution_handle_digest": open.Binding.ExecutionHandleDigest,
				"subject_digest": open.Binding.SubjectDigest, "source_artifact_ref": open.Binding.SourceArtifactRef,
				"source_candidate_id": open.Binding.SourceCandidateID,
			},
		}
		req := types.BindEngineeringAssignmentRequest{
			CommandID: "command-bind-" + open.AssignmentID + "-1",
			OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: open.AssignmentID,
			Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
			OpaqueCapability: capability, CapsuleID: open.Binding.CapsuleID,
		}
		req.CommandDigest, _ = store.ComputeBindEngineeringAssignmentDigest(req)
		return req
	}

	openA := openReq("assignment-a", "capsule-a", "cap-a", 0)
	if _, err := s.OpenEngineeringAssignment(ctx, openA); err != nil {
		t.Fatal(err)
	}
	cancelA := types.CancelEngineeringAssignmentRequest{
		CommandID: "command-cancel-a-1", OwnerID: openA.Binding.OwnerID, ComputerID: openA.Binding.ComputerID,
		AssignmentID: "assignment-a", Attempt: 1, ExpectedLifecycleVersion: 1, Reason: "spawn failed before bind",
	}
	cancelA.CommandDigest, _ = store.ComputeCancelEngineeringAssignmentDigest(cancelA)
	if _, err := s.CancelEngineeringAssignment(ctx, cancelA); err != nil {
		t.Fatal(err)
	}

	openB := openReq("assignment-b", "capsule-b", "cap-b", 1)
	if _, err := s.OpenEngineeringAssignment(ctx, openB); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bindReq(openB, "run:assignment-b", "cap-b")); err != nil {
		t.Fatal(err)
	}

	rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx)

	a, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, "assignment-a", 1)
	if err != nil || !a.Disposition.Terminal() || a.CapsuleDisposition != types.EngineeringCapsuleUnbound {
		t.Fatalf("terminal-unbound assignment changed: %+v err=%v", a, err)
	}
	b, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, "assignment-b", 1)
	if err != nil || !b.Disposition.Terminal() || b.CapsuleDisposition != types.EngineeringCapsuleRevoked {
		t.Fatalf("bound assignment not reconciled: %+v err=%v", b, err)
	}
	runB, err := s.GetLifecycleRun(ctx, seed.OwnerID, seed.ComputerID, "run:assignment-b")
	if err != nil || !runB.State.Terminal() {
		t.Fatalf("bound run not terminal: %+v err=%v", runB, err)
	}
}

func TestRewarmAssignedEngineeringReconcilesAbsentCapsuleWithoutWake(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	var wakes []string
	rt.SetDispatchActor(func(_ context.Context, _, _, _, kind, content, _, _ string) error {
		wakes = append(wakes, kind+":"+content)
		return nil
	})
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-assignment", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignmentID := "assignment-boot-absent"
	capability := "opaque-boot-absent"
	capsuleID := "capsule-boot-absent"
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
			Writable:          true, CapsuleID: capsuleID,
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "bounded delegated assignment"},
	}
	open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	runID := "run:" + assignmentID
	run := types.RunRecord{
		RunID: runID, AgentID: open.Binding.AssignedAgentID, ChannelID: open.Binding.AssignedAgentID,
		RequestedByRunID: open.Binding.ParentRunID, TrajectoryID: open.Binding.TrajectoryID,
		AgentProfile: "engineering", AgentRole: "engineering", OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		State: types.RunPending, Prompt: open.AssignedWork.Objective,
		Metadata: map[string]any{
			"work_item_ids": []string{open.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": open.Binding.AssignedWorkItemID,
			"requested_by_agent_id": open.Binding.ParentAgentID, "requested_by_profile": "management",
			"assignment_id": assignmentID, "assignment_attempt": 1, "assignment_kind": string(open.Binding.Kind),
			"assigned_work_item_id": open.Binding.AssignedWorkItemID, "parent_work_item_id": open.Binding.ParentWorkItemID,
			"parent_decision_id": open.Binding.ParentDecisionID, "parent_control_id": open.Binding.ParentControlID,
			"capsule_id": open.Binding.CapsuleID, "scope_digest": open.Binding.ScopeDigest, "request_digest": open.Binding.RequestDigest,
			"capability_digest": open.Binding.CapabilityDigest, "execution_handle_digest": open.Binding.ExecutionHandleDigest,
			"subject_digest": open.Binding.SubjectDigest, "source_artifact_ref": open.Binding.SourceArtifactRef,
			"source_candidate_id": open.Binding.SourceCandidateID,
		},
	}
	bind := types.BindEngineeringAssignmentRequest{
		CommandID: "command-bind-" + assignmentID + "-1",
		OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: assignmentID,
		Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: capability, CapsuleID: capsuleID,
	}
	bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bind); err != nil {
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
		t.Fatalf("bound Engineering run missing from lifecycle pending index: %+v", listed)
	}
	if err := rt.ReconcileLifecycleWorkAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, open.Binding.AssignedAgentID, open.Binding.TrajectoryID, open.Binding.AssignedWorkItemID); err != nil {
		t.Fatalf("reconcile assigned Engineering work wake: %v", err)
	}
	for _, wake := range wakes {
		if strings.HasPrefix(wake, "initial_dispatch:") {
			t.Fatalf("rewarm re-dispatched assigned Engineering after restart: %v", wakes)
		}
	}
	assignment, err := s.GetEngineeringAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, assignmentID, 1)
	if err != nil || !assignment.Disposition.Terminal() || assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked {
		t.Fatalf("assignment fate=%+v err=%v", assignment, err)
	}
	got, err := s.GetLifecycleRun(ctx, open.Binding.OwnerID, open.Binding.ComputerID, runID)
	if err != nil || !got.State.Terminal() {
		t.Fatalf("run projection=%+v err=%v", got, err)
	}
}

// A Complete reduce that failed after the freeze ack leaves the assignment
// bound with a frozen capsule and a staged pending proposal. The restart
// reconcile must claim that state for the reducer-authored resume BEFORE the
// restart-cancel branch can destroy the work evidence; on darwin the resume
// itself cannot finish (the executor stub refuses granted-capsule work), so
// the strand is preserved for the next sweep instead of being cancelled.
func TestReconcileResumesStrandedFrozenProposalBeforeRestartCancel(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-assignment", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignmentID := "assignment-strand-frozen"
	capability := "opaque-strand-frozen"
	capsuleID := "capsule-strand-frozen"
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
			Writable:          true, CapsuleID: capsuleID,
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "bounded delegated assignment"},
	}
	open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	runID := "run:" + assignmentID
	run := types.RunRecord{
		RunID: runID, AgentID: open.Binding.AssignedAgentID, ChannelID: open.Binding.AssignedAgentID,
		RequestedByRunID: open.Binding.ParentRunID, TrajectoryID: open.Binding.TrajectoryID,
		AgentProfile: "engineering", AgentRole: "engineering", OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		State: types.RunPending, Prompt: open.AssignedWork.Objective,
		Metadata: map[string]any{
			"work_item_ids": []string{open.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": open.Binding.AssignedWorkItemID,
			"requested_by_agent_id": open.Binding.ParentAgentID, "requested_by_profile": "management",
			"assignment_id": assignmentID, "assignment_attempt": 1, "assignment_kind": string(open.Binding.Kind),
			"assigned_work_item_id": open.Binding.AssignedWorkItemID, "parent_work_item_id": open.Binding.ParentWorkItemID,
			"parent_decision_id": open.Binding.ParentDecisionID, "parent_control_id": open.Binding.ParentControlID,
			"capsule_id": open.Binding.CapsuleID, "scope_digest": open.Binding.ScopeDigest, "request_digest": open.Binding.RequestDigest,
			"capability_digest": open.Binding.CapabilityDigest, "execution_handle_digest": open.Binding.ExecutionHandleDigest,
			"subject_digest": open.Binding.SubjectDigest, "source_artifact_ref": open.Binding.SourceArtifactRef,
		},
	}
	bind := types.BindEngineeringAssignmentRequest{
		CommandID: "command-bind-" + assignmentID + "-1",
		OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: assignmentID,
		Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: capability, CapsuleID: capsuleID,
	}
	bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bind); err != nil {
		t.Fatal(err)
	}
	// Stage the exact proposal a Complete reduce stages: the terminal report
	// content and the freeze intent keyed to its proposition digest.
	report := types.EngineeringAssignmentReport{
		Result: types.EngineeringResultCompleted, Verdict: types.EngineeringVerdictNone, Summary: "resumable stranded work",
		ObservedSubjectDigest: open.Binding.SubjectDigest,
		Commands:              []types.EngineeringRecordedCommand{{CommandID: "observed-command", CommandDigest: objectgraph.SHA256([]byte("command")), ExecutionRef: "receipt:execution"}},
		Outputs:               []types.EngineeringRecordedOutput{{OutputID: "output", Kind: "evidence", Digest: objectgraph.SHA256([]byte("output")), Ref: "artifact:output"}},
	}
	propositionDigest, digestErr := store.ComputeTerminalPropositionDigest(open.Binding.SubjectDigest,
		report.Result, report.Verdict, report.Commands, report.Outputs, report.EvidenceRefs)
	if digestErr != nil {
		t.Fatal(digestErr)
	}
	freeze := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: "command-freeze-" + assignmentID, OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		AssignmentID: assignmentID, Attempt: 1, ExpectedLifecycleVersion: 2,
		Disposition: types.EngineeringCapsuleFreezeRequested,
		IntentRef:   "capsule-freeze-intent:" + propositionDigest,
		PendingProposal: &types.EngineeringPendingProposal{
			PropositionDigest: propositionDigest, Report: report,
			FreezeIntentRef: "capsule-freeze-intent:" + propositionDigest, CreatedAt: time.Now().UTC(),
		},
	}
	freeze.CommandDigest, err = store.ComputeSetEngineeringCapsuleDispositionDigest(freeze)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetEngineeringCapsuleDisposition(ctx, freeze); err != nil {
		t.Fatal(err)
	}
	ack := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: "command-frozen-" + assignmentID, OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		AssignmentID: assignmentID, Attempt: 1, ExpectedLifecycleVersion: 3,
		Disposition: types.EngineeringCapsuleFrozen, IntentRef: "capsule-freeze-intent:" + propositionDigest,
		AckRef: "capsule-fate:sha256:" + propositionDigest,
	}
	ack.CommandDigest, err = store.ComputeSetEngineeringCapsuleDispositionDigest(ack)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetEngineeringCapsuleDisposition(ctx, ack); err != nil {
		t.Fatal(err)
	}

	rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx)
	assignment, err := s.GetEngineeringAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, assignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if assignment.Disposition != types.EngineeringAssignmentBound || assignment.CapsuleDisposition != types.EngineeringCapsuleFrozen {
		t.Fatalf("stranded frozen assignment was not preserved for resume: %+v", assignment)
	}
	if assignment.PendingProposal == nil || assignment.PendingProposal.PropositionDigest != propositionDigest {
		t.Fatalf("stranded proposal was dropped: %+v", assignment.PendingProposal)
	}
	runState, err := s.GetLifecycleRun(ctx, open.Binding.OwnerID, open.Binding.ComputerID, runID)
	if err != nil || runState.State.Terminal() {
		t.Fatalf("resume branch must not terminalize the bound run before the fate lands: %+v err=%v", runState, err)
	}
	// Idempotent: a second reconcile keeps the strand resumable rather than
	// minting a second resume report.
	rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx)
	again, err := s.GetEngineeringAssignment(ctx, open.Binding.OwnerID, open.Binding.ComputerID, assignmentID, 1)
	if err != nil || again.Disposition != types.EngineeringAssignmentBound || again.CapsuleDisposition != types.EngineeringCapsuleFrozen {
		t.Fatalf("second reconcile changed the strand: %+v err=%v", again, err)
	}
}

// TestAssignedEngineeringFatePendingSignature pins the strand predicate: a bound
// assignment with a staged proposal on any pending fate disposition
// (freeze_requested, frozen, revoke_requested) owes a continuation; terminal,
// active, or proposal-less states do not.
func TestAssignedEngineeringFatePendingSignature(t *testing.T) {
	base := types.EngineeringAssignment{AssignmentID: "assignment-p", Disposition: types.EngineeringAssignmentBound,
		Binding: types.EngineeringAssignmentBinding{Attempt: 1}}
	withProposal := func(a types.EngineeringAssignment) types.EngineeringAssignment {
		a.PendingProposal = &types.EngineeringPendingProposal{PropositionDigest: "sha256:prop"}
		return a
	}
	if assignedEngineeringFatePending(withProposal(base)) {
		t.Fatal("active capsule must not be pending")
	}
	for _, disposition := range []types.EngineeringCapsuleDisposition{
		types.EngineeringCapsuleFreezeRequested, types.EngineeringCapsuleFrozen, types.EngineeringCapsuleRevokeRequested,
		types.EngineeringCapsuleRevoked,
	} {
		stranded := withProposal(base)
		stranded.CapsuleDisposition = disposition
		if !assignedEngineeringFatePending(stranded) {
			t.Fatalf("disposition %s must be pending", disposition)
		}
	}
	completed := withProposal(base)
	completed.Disposition = types.EngineeringAssignmentCompleted
	if assignedEngineeringFatePending(completed) {
		t.Fatal("terminal disposition must not be pending")
	}
	bare := base
	bare.CapsuleDisposition = types.EngineeringCapsuleFrozen
	if assignedEngineeringFatePending(bare) {
		t.Fatal("frozen without a proposal must not be pending (restart-cancel owns it)")
	}
}

// TestAssignedEngineeringTerminalRevokeIntentRoundTrip pins the deterministic
// revoke intent the terminal saga mints: a stranded revoke_requested strand
// must recompute the exact intent so the same terminal report can re-enter
// the saga (P5-review F3).
func TestAssignedEngineeringTerminalRevokeIntentRoundTrip(t *testing.T) {
	assignment := types.EngineeringAssignment{
		AssignmentID: "assignment-9ec36ecb", Disposition: types.EngineeringAssignmentBound,
		BoundRunID: "run:assignment-9ec36ecb",
		Binding:    types.EngineeringAssignmentBinding{Attempt: 1, CapsuleID: "capsule-9ec36ecb"},
	}
	first := assignedEngineeringTerminalRevokeIntent(assignment)
	if first == "" || !strings.HasPrefix(first, "capsule-revoke-intent:") {
		t.Fatalf("intent = %q", first)
	}
	if again := assignedEngineeringTerminalRevokeIntent(assignment); again != first {
		t.Fatalf("intent not deterministic: %q vs %q", again, first)
	}
	mutated := assignment
	mutated.Binding.Attempt = 2
	if mutatedIntent := assignedEngineeringTerminalRevokeIntent(mutated); mutatedIntent == first {
		t.Fatal("attempt must be part of the intent identity")
	}
}
