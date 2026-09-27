package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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

// The bound-assignment deadline is a derivable wake, not a selection-gated
// sweep: when the armed assigned_engineering_fate_deadline fires on an expired
// bound+active assignment, the dispatcher handler cancels it with no
// management-desk selection. A not-yet-expired assignment must no-op.
func TestDeadlineWakeCancelsExpiredBoundAssignmentWithoutManagementSelection(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)

	// Two live assignments on one trajectory collide on the lifecycle
	// optimistic-concurrency lock, so each case gets its own seed/trajectory.
	openAndBind := func(owner string, assignmentID string) (types.EngineeringAssignmentBinding, error) {
		seed, err := store.SeedEngineeringAssignmentAuthority(s, owner, rt.TextureComputerID(), 1)
		if err != nil {
			return types.EngineeringAssignmentBinding{}, err
		}
		open := types.OpenEngineeringAssignmentRequest{
			CommandID: "command-open-" + assignmentID, AssignmentID: assignmentID,
			Binding: types.EngineeringAssignmentBinding{
				OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
				ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
				ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
				ParentWorkItemID: seed.ParentWorkID, AssignedWorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0],
				Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
				ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
				CapabilityDigest: store.DigestEngineeringOpaqueCapability("cap-" + assignmentID), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-" + assignmentID)),
				SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
				SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
				Writable:          true, CapsuleID: "capsule-" + assignmentID,
				NetworkMode: types.EngineeringCapsuleNetworkForbidden, FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
			},
			AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
			AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "deadline expiry"},
		}
		open.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open)
		if err != nil {
			return types.EngineeringAssignmentBinding{}, err
		}
		if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
			return types.EngineeringAssignmentBinding{}, err
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
			CommandID: "command-bind-" + assignmentID, OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
			AssignmentID: assignmentID, Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
			OpaqueCapability: "cap-" + assignmentID, CapsuleID: open.Binding.CapsuleID,
		}
		bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
		if err != nil {
			return types.EngineeringAssignmentBinding{}, err
		}
		if _, err := s.BindEngineeringAssignment(ctx, bind); err != nil {
			return types.EngineeringAssignmentBinding{}, err
		}
		return open.Binding, nil
	}

	// Fresh assignment: deadline not reached, handler must no-op.
	freshBinding, err := openAndBind("owner-deadline-fresh", "assignment-deadline-live")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := s.GetEngineeringAssignment(ctx, freshBinding.OwnerID, freshBinding.ComputerID, "assignment-deadline-live", 1)
	if err != nil || fresh.Disposition != types.EngineeringAssignmentBound || fresh.CapsuleDisposition != types.EngineeringCapsuleActive {
		t.Fatalf("fresh bound+active assignment: %+v err=%v", fresh, err)
	}
	content, err := encodeAssignedEngineeringFateDeadline("assignment-deadline-live", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.HandleAssignedEngineeringFateDeadline(ctx, freshBinding.OwnerID, freshBinding.ComputerID, freshBinding.ParentAgentID, content); err != nil {
		t.Fatalf("live assignment handler: %v", err)
	}
	if after, _ := s.GetEngineeringAssignment(ctx, freshBinding.OwnerID, freshBinding.ComputerID, "assignment-deadline-live", 1); after.Disposition != types.EngineeringAssignmentBound {
		t.Fatalf("not-yet-due assignment was touched: %+v", after)
	}

	// Expired assignment: the same wake cancels it — no management reconcile ran.
	t.Setenv("CHOIR_ASSIGNMENT_DEADLINE", "1ms")
	expiredBinding, err := openAndBind("owner-deadline-expired", "assignment-deadline-expired")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	expiredContent, err := encodeAssignedEngineeringFateDeadline("assignment-deadline-expired", 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.HandleAssignedEngineeringFateDeadline(ctx, expiredBinding.OwnerID, expiredBinding.ComputerID, expiredBinding.ParentAgentID, expiredContent); err != nil {
		t.Fatalf("expired assignment handler: %v", err)
	}
	done, err := s.GetEngineeringAssignment(ctx, expiredBinding.OwnerID, expiredBinding.ComputerID, "assignment-deadline-expired", 1)
	if err != nil {
		t.Fatal(err)
	}
	if done.Disposition != types.EngineeringAssignmentCancelled {
		t.Fatalf("expired assignment not cancelled by derivable wake: %+v", done)
	}
}

// A guest restart terminally cancels the bound assignment (absent capsule) but
// must not strand the cast: the desk reconcile re-opens the cast at the next
// attempt carrying a retry_after_block supersede tuple that names the prior
// attempt's cancel report. A deliberate cancel (different reason) stays dead.
func TestRestartCancelledCastRecastsAtNextAttempt(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)

	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-restart-recast", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignmentID := "assignment-restart-recast"
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + assignmentID, AssignmentID: assignmentID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
			ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
			ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
			ParentWorkItemID: seed.ParentWorkID, AssignedWorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0],
			Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability("cap-" + assignmentID), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-" + assignmentID)),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
			Writable:          true, CapsuleID: "capsule-" + assignmentID,
			NetworkMode: types.EngineeringCapsuleNetworkForbidden, FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "restart recast"},
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
		CommandID: "command-bind-" + assignmentID, OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		AssignmentID: assignmentID, Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: "cap-" + assignmentID, CapsuleID: open.Binding.CapsuleID,
	}
	bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bind); err != nil {
		t.Fatal(err)
	}

	// Restart: the capsule is absent, so the sweep revokes + cancels attempt 1.
	rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx)
	cancelled, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, assignmentID, 1)
	if err != nil || cancelled.Disposition != types.EngineeringAssignmentCancelled {
		t.Fatalf("attempt 1 not restart-cancelled: %+v err=%v", cancelled, err)
	}
	if cancelled.DispositionReason != restartCancelledAssignmentReason {
		t.Fatalf("unexpected cancel reason %q", cancelled.DispositionReason)
	}

	// The reconcile predicate: the latest terminal attempt cancelled by restart
	// is recast-admissible; a deliberate cancel is not.
	attempts, err := s.ListEngineeringAssignments(ctx, seed.OwnerID, seed.ComputerID, seed.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	latest, admissible := latestCancelledForRestartRecast(attempts, assignmentID)
	if !admissible || latest.Binding.Attempt != 1 {
		t.Fatalf("restart-cancelled attempt not marked recast-admissible: latest=%+v admissible=%v", latest, admissible)
	}
	reportRef := rt.restartRecastReportRef(ctx, latest)
	if reportRef == "" {
		t.Fatal("cancel report receipt not resolvable for supersede tuple")
	}

	// Attempt 2 opens with the frozen supersede tuple naming the cancel report.
	open2 := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + assignmentID + ":2", AssignmentID: assignmentID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
			ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
			ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
			ParentWorkItemID:   seed.ParentWorkID,
			AssignedWorkItemID: "work:" + assignmentID + ":attempt-2",
			AssignedAgentID:    agentprofile.Engineering + ":" + assignmentID + ":attempt-2",
			Kind:               types.EngineeringAssignmentImplementation, Attempt: 2,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability("cap-" + assignmentID + "-2"), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-" + assignmentID + "-2")),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
			Writable:          true, CapsuleID: "capsule-" + assignmentID + "-2",
			NetworkMode: types.EngineeringCapsuleNetworkForbidden, FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: agentprofile.Engineering + ":" + assignmentID + ":attempt-2"},
		AssignedWork:  types.WorkItemRecord{WorkItemID: "work:" + assignmentID + ":attempt-2", AssignedAgentID: agentprofile.Engineering + ":" + assignmentID + ":attempt-2", Objective: "restart recast"},
		Supersedes: &types.EngineeringSupersedeTuple{
			SupersedesAssignmentID: assignmentID, SupersedesAttempt: 1,
			PriorReceiptRef: reportRef, SupersedeKind: types.EngineeringSupersedeRetryAfterBlock,
			ReasonEnum: "restart_passivation",
			DeltaDigest: objectgraph.SHA256([]byte(strings.Join([]string{
				"retry_after_block", assignmentID, "1", "2", restartCancelledAssignmentReason,
			}, "\x00"))),
		},
	}
	open2.CommandDigest, err = store.ComputeOpenEngineeringAssignmentDigest(open2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenEngineeringAssignment(ctx, open2); err != nil {
		t.Fatalf("attempt-2 recast open refused: %v", err)
	}
	recast, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, assignmentID, 2)
	if err != nil || recast.Disposition != types.EngineeringAssignmentOpen {
		t.Fatalf("attempt 2 not open after recast: %+v err=%v", recast, err)
	}

	// A deliberate cancel is not recast-admissible: same predicate, wrong reason.
	deliberate := cancelled
	deliberate.DispositionReason = "owner requested cancellation"
	if _, ok := latestCancelledForRestartRecast([]types.EngineeringAssignment{deliberate}, assignmentID); ok {
		t.Fatal("deliberate cancel must not be restart-recast admissible")
	}
}

// TestRevokedAssignmentResumeCommitsStagedProposalEvidence pins the
// post-revoke terminal resume: when a bound report's commit fails after the
// saga revoked the capsule (e.g. a transient validation or CAS failure), the
// retry must commit the staged pending proposal — the granted executor
// evidence minted while the capsule was frozen — rather than re-binding the
// raw execution receipts through the late path. Raw receipts lack granted
// refs, so a late-bound commit cannot satisfy the timely evidence contract;
// the staged proposal is the only evidence-bearing resume source.
// Regression: m11-recast-desk-terminal-report-uncommittable-2026-09-27.
func TestRevokedAssignmentResumeCommitsStagedProposalEvidence(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	rt.assignmentRuntime = absentAssignmentCapsule{}
	rt.capsuleExecutor = capsule.NewExecutor(t.TempDir(), t.TempDir(), t.TempDir(), 0)
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-resume-proposal", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignmentID := "assignment-resume-proposal"
	open := types.OpenEngineeringAssignmentRequest{
		CommandID: "command-open-" + assignmentID, AssignmentID: assignmentID,
		Binding: types.EngineeringAssignmentBinding{
			OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, TrajectoryID: seed.TrajectoryID,
			ParentAgentID: seed.ParentAgentID, ParentRunID: seed.ParentRunID,
			ParentDecisionID: seed.ParentDecisionID, ParentControlID: seed.ParentControlID,
			ParentWorkItemID: seed.ParentWorkID, AssignedWorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0],
			Kind: types.EngineeringAssignmentImplementation, Attempt: 1,
			ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)), RequestDigest: objectgraph.SHA256([]byte("request:" + assignmentID)),
			CapabilityDigest: store.DigestEngineeringOpaqueCapability("cap-" + assignmentID), ExecutionHandleDigest: objectgraph.SHA256([]byte("cap-" + assignmentID)),
			SubjectDigest:     objectgraph.SHA256([]byte("subject:" + assignmentID)),
			SourceArtifactRef: "capsule-source-git:commit:" + objectgraph.SHA256([]byte("subject:"+assignmentID)),
			Writable:          true, CapsuleID: "capsule-" + assignmentID,
			NetworkMode:    types.EngineeringCapsuleNetworkForbidden,
			FilesystemMode: types.EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
		},
		AssignedAgent: types.AgentRecord{AgentID: seed.AssignedAgentIDs[0]},
		AssignedWork:  types.WorkItemRecord{WorkItemID: seed.AssignedWorkIDs[0], AssignedAgentID: seed.AssignedAgentIDs[0], Objective: "resume staged proposal"},
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
		CommandID: "command-bind-" + assignmentID,
		OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: assignmentID,
		Attempt: 1, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: "cap-" + assignmentID, CapsuleID: open.Binding.CapsuleID,
	}
	bind.CommandDigest, err = store.ComputeBindEngineeringAssignmentDigest(bind)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := s.BindEngineeringAssignment(ctx, bind)
	if err != nil {
		t.Fatal(err)
	}

	// The desk mutated the subject: the frozen digest differs from the binding
	// subject, so the proposal carries a subject_bytes mutation overlay.
	frozenDigest := objectgraph.SHA256([]byte("frozen-subject:" + assignmentID))
	command := types.EngineeringRecordedCommand{
		CommandID: "observed-command", CommandDigest: objectgraph.SHA256([]byte("mutating command")),
		ExecutionRef: "capsule-exec:sha256:" + strings.Repeat("1", 64), ExitCode: 0,
	}
	bareReport := types.EngineeringAssignmentReport{
		Result: types.EngineeringResultCompleted, Verdict: types.EngineeringVerdictNone, Summary: "mutating desk work",
		Commands: []types.EngineeringRecordedCommand{command},
	}
	propositionDigest, err := store.ComputeTerminalPropositionDigest(open.Binding.SubjectDigest,
		bareReport.Result, bareReport.Verdict, bareReport.Commands, bareReport.Outputs, bareReport.EvidenceRefs)
	if err != nil {
		t.Fatal(err)
	}
	reportID := store.TerminalReportID(open.Binding.OwnerID, open.Binding.ComputerID, assignmentID, 1, propositionDigest)

	// The staged proposal is the shape the live saga persists while frozen:
	// the authored report plus granted executor evidence bound at freeze time.
	// ObservedSubjectDigest stays the authored (binding) digest — the
	// subject-change overlay lives on Mutations.
	grantedRef := "capsule-granted-exec:sha256:" + strings.Repeat("2", 64)
	stagedReport := bareReport
	stagedReport.ReportID = reportID
	stagedReport.PropositionDigest = propositionDigest
	stagedReport.ExecutorReceiptRefs = []string{grantedRef}
	stagedReport.Mutations = []types.EngineeringRecordedMutation{{
		MutationID: "assignment-overlay:test", Kind: "assignment_overlay",
		BeforeDigest: open.Binding.SubjectDigest, AfterDigest: frozenDigest,
		EvidenceRef: "capsule-diff:test", SubjectBytesChanged: true,
	}}
	stagedReport.CandidateArtifactRef = "capsule-subject:" + frozenDigest

	freezeIntentRef := "capsule-freeze-intent:" + propositionDigest
	freeze := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: "command-freeze-" + assignmentID, OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		AssignmentID: assignmentID, Attempt: 1, ExpectedLifecycleVersion: bound.Assignment.LifecycleVersion,
		Disposition: types.EngineeringCapsuleFreezeRequested, IntentRef: freezeIntentRef,
		PendingProposal: &types.EngineeringPendingProposal{
			PropositionDigest: propositionDigest, Report: stagedReport,
			FreezeIntentRef: freezeIntentRef, CreatedAt: time.Now().UTC(),
		},
	}
	freeze.CommandDigest, err = store.ComputeSetEngineeringCapsuleDispositionDigest(freeze)
	if err != nil {
		t.Fatal(err)
	}
	freezeResult, err := s.SetEngineeringCapsuleDisposition(ctx, freeze)
	if err != nil {
		t.Fatal(err)
	}
	frozenAck := freeze
	frozenAck.CommandID, frozenAck.ExpectedLifecycleVersion = "command-frozen-"+assignmentID, freezeResult.Assignment.LifecycleVersion
	frozenAck.Disposition, frozenAck.AckRef = types.EngineeringCapsuleFrozen, "capsule-fate:sha256:"+strings.Repeat("3", 64)
	frozenAck.PendingProposal = freezeResult.Assignment.PendingProposal
	frozenAck.CommandDigest, err = store.ComputeSetEngineeringCapsuleDispositionDigest(frozenAck)
	if err != nil {
		t.Fatal(err)
	}
	frozenResult, err := s.SetEngineeringCapsuleDisposition(ctx, frozenAck)
	if err != nil {
		t.Fatal(err)
	}

	// The saga's terminal revoke: the assignment reaches revoked with the
	// staged proposal still pending — the state a first commit failure leaves.
	revokeIntent := assignedEngineeringTerminalRevokeIntent(frozenResult.Assignment)
	revoke := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: "command-revoke-req-" + assignmentID, OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		AssignmentID: assignmentID, Attempt: 1, ExpectedLifecycleVersion: frozenResult.Assignment.LifecycleVersion,
		Disposition: types.EngineeringCapsuleRevokeRequested, IntentRef: revokeIntent,
	}
	revoke.CommandDigest, err = store.ComputeSetEngineeringCapsuleDispositionDigest(revoke)
	if err != nil {
		t.Fatal(err)
	}
	revokeReqResult, err := s.SetEngineeringCapsuleDisposition(ctx, revoke)
	if err != nil {
		t.Fatal(err)
	}
	revokeAck := revoke
	revokeAck.CommandID, revokeAck.ExpectedLifecycleVersion = "command-revoked-"+assignmentID, revokeReqResult.Assignment.LifecycleVersion
	revokeAck.Disposition, revokeAck.AckRef = types.EngineeringCapsuleRevoked, "capsule-revoke:sha256:"+strings.Repeat("4", 64)
	revokeAck.CommandDigest, err = store.ComputeSetEngineeringCapsuleDispositionDigest(revokeAck)
	if err != nil {
		t.Fatal(err)
	}
	revokedResult, err := s.SetEngineeringCapsuleDisposition(ctx, revokeAck)
	if err != nil {
		t.Fatal(err)
	}
	if revokedResult.Assignment.CapsuleDisposition != types.EngineeringCapsuleRevoked || revokedResult.Assignment.PendingProposal == nil {
		t.Fatalf("assignment did not reach revoked-with-pending-proposal: %+v", revokedResult.Assignment)
	}

	// The retry submits the bare report (the worker's authored claims) — the
	// resume must substitute the staged proposal's granted evidence, not the
	// late raw-receipt path.
	result, err := rt.recordAssignedEngineeringReport(ctx, &run, "tool-call:complete", bareReport)
	if err != nil {
		t.Fatalf("revoked resume refused staged proposal evidence: %v", err)
	}
	if result.Report == nil || result.Report.Late {
		t.Fatalf("resumed report committed late: %+v", result.Report)
	}
	if len(result.Report.ExecutorReceiptRefs) != 1 || result.Report.ExecutorReceiptRefs[0] != grantedRef {
		t.Fatalf("resumed report lost granted executor evidence: %+v", result.Report.ExecutorReceiptRefs)
	}
	if result.Report.ObservedSubjectDigest != frozenDigest {
		t.Fatalf("resumed report observed subject = %s, want frozen %s", result.Report.ObservedSubjectDigest, frozenDigest)
	}
	if result.Assignment.Disposition != types.EngineeringAssignmentCompleted {
		t.Fatalf("resumed assignment disposition = %s, want completed", result.Assignment.Disposition)
	}
}

// TestExecutionAttestationPinsFrozenFinalSubject pins the certified-subject
// contract: an execution attestation binds each command to the granted
// receipt's certified *final* subject (the frozen worktree digest), never the
// receipt's per-command post-tree. A mutating multi-command run legitimately
// records earlier worktree states on intermediate receipts; the attestation
// must still certify the frozen final subject. Per-command post-trees remain
// on the raw execution receipts.
// Regression: m11-recast-desk-terminal-report-uncommittable-2026-09-27.
func TestExecutionAttestationPinsFrozenFinalSubject(t *testing.T) {
	subject := objectgraph.SHA256([]byte("binding-subject"))
	frozen := objectgraph.SHA256([]byte("frozen-final"))
	intermediate := objectgraph.SHA256([]byte("intermediate-worktree"))
	commandText := "echo out && write file"
	assignment := types.EngineeringAssignment{
		AssignmentID: "assignment-att", BoundRunID: "run-att",
		Binding: types.EngineeringAssignmentBinding{CapsuleID: "capsule-att", SubjectDigest: subject},
	}
	receipt := capsule.ExecutionReceipt{
		AgentRunID: "run-att", CapsuleID: "capsule-att", Command: commandText,
		GrantedReceiptRef: "capsule-granted-exec:sha256:" + strings.Repeat("5", 64),
		SourceTreeDigest:  subject, WorktreeDigest: intermediate,
		StdoutDigest: objectgraph.SHA256([]byte("out")), StderrDigest: objectgraph.SHA256([]byte("")),
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	command := types.EngineeringRecordedCommand{
		CommandID: "cmd-1", CommandDigest: objectgraph.SHA256([]byte(commandText)), ExecutionRef: "capsule-exec:sha256:x",
	}
	att, err := engineeringExecutionAttestationFromReceipt(assignment, "report-x", command, receipt, frozen)
	if err != nil {
		t.Fatalf("attestation mint: %v", err)
	}
	if att.FinalSubjectDigest != frozen || att.WorktreeDigest != frozen {
		t.Fatalf("attestation certifies per-command worktree %s instead of frozen final %s", att.WorktreeDigest, frozen)
	}
	if att.SourceSubjectDigest != subject {
		t.Fatalf("attestation source = %s, want binding subject %s", att.SourceSubjectDigest, subject)
	}
	if att.WorktreeDigest == intermediate {
		t.Fatal("attestation WorktreeDigest retained the per-command post-tree")
	}
}
