package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type coSuperAssignmentStoreFixture struct {
	ownerID, computerID, trajectoryID                 string
	parentAgentID, parentRunID, parentWorkID          string
	parentDecisionID, parentControlID                 string
	assignedAgentIDs, assignedWorkIDs, assignedRunIDs []string
}

func installCoSuperAssignmentAuthority(t *testing.T, s *Store, count int) coSuperAssignmentStoreFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	f := coSuperAssignmentStoreFixture{
		ownerID: "owner-assignment", computerID: "computer-assignment", trajectoryID: "trajectory-assignment",
		parentAgentID: "super:owner-assignment", parentRunID: "run-super-assignment", parentWorkID: "work-super-assignment",
		parentDecisionID: "decision:" + objectgraph.SHA256([]byte("decision-assignment")), parentControlID: "control-assignment",
	}
	trajectory := types.TrajectoryRecord{
		TrajectoryID: f.trajectoryID, OwnerID: f.ownerID, ComputerID: f.computerID,
		Kind: types.TrajectoryKindTask, Status: types.TrajectoryLive,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"subject"}},
		SubjectRefs:    map[string]string{"subject": "artifact:subject", "doc_id": "document-assignment"}, LifecycleVersion: 1, ReducerSeq: 1, CreatedAt: now, UpdatedAt: now,
	}
	document := types.Document{DocID: "document-assignment", OwnerID: f.ownerID, ComputerID: f.computerID, Title: "Assignment authority", CurrentRevisionID: "revision-assignment", CreatedAt: now, UpdatedAt: now}
	revision := types.Revision{RevisionID: "revision-assignment", DocID: document.DocID, OwnerID: f.ownerID, ComputerID: f.computerID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", Content: "assignment authority", CreatedAt: now}
	parentAgent := types.AgentRecord{
		AgentID: f.parentAgentID, OwnerID: f.ownerID, ComputerID: f.computerID, SandboxID: f.computerID,
		Profile: "super", Role: "super", ChannelID: f.parentAgentID, ActiveRunID: f.parentRunID,
		LifecycleVersion: 0, CreatedAt: now, UpdatedAt: now,
	}
	parentWork := types.WorkItemRecord{
		WorkItemID: f.parentWorkID, TrajectoryID: f.trajectoryID, OwnerID: f.ownerID, ComputerID: f.computerID,
		Objective: "coordinate delegated assignments", AuthorityProfile: "super", Status: types.WorkItemOpen,
		AssignedAgentID: f.parentAgentID, LifecycleVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	parentRun := types.RunRecord{
		RunID: f.parentRunID, AgentID: f.parentAgentID, ChannelID: f.parentAgentID,
		AgentProfile: "super", AgentRole: "super", OwnerID: f.ownerID, SandboxID: f.computerID,
		State: types.RunRunning, Prompt: "coordinate", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			"assignment_trajectory_id": f.trajectoryID, "work_item_ids": []string{f.parentWorkID},
			"lifecycle_control_bindings": []any{map[string]any{"trajectory_id": f.trajectoryID,
				"target_work_item_id": f.parentWorkID, "update_id": f.parentControlID, "producer_agent_id": "texture:document-assignment"}},
		},
	}
	mustLifecycleObject := func(kind objectgraph.ObjectKind, key string, body any, metadata map[string]any) objectgraph.Object {
		t.Helper()
		obj, err := lifecycleObject(kind, f.ownerID, f.computerID, key, body, metadata, now, now)
		if err != nil {
			t.Fatalf("build %s %s: %v", kind, key, err)
		}
		return obj
	}
	objects := []objectgraph.Object{
		mustLifecycleObject(ogKindTrajectory, f.trajectoryID, trajectory, lifecycleMetadata("trajectory_id", f.trajectoryID, f.computerID, f.trajectoryID, 1)),
		mustLifecycleObject(ogKindAgent, f.parentAgentID, parentAgent, map[string]any{"agent_id": f.parentAgentID, "computer_id": f.computerID}),
		mustLifecycleObject(ogKindWorkItem, f.parentWorkID, parentWork, lifecycleMetadata("work_item_id", f.parentWorkID, f.computerID, f.trajectoryID, 1)),
		mustLifecycleObject(ogKindTexDoc, document.DocID, document, map[string]any{"doc_id": document.DocID, "computer_id": f.computerID}),
		mustLifecycleObject(ogKindTexRev, revision.RevisionID, revision, map[string]any{"revision_id": revision.RevisionID, "doc_id": document.DocID, "computer_id": f.computerID}),
	}
	for i := 0; i < count; i++ {
		agentID := fmt.Sprintf("co-super:assignment-%02d", i)
		workID := fmt.Sprintf("work-cosuper-assignment-%02d", i)
		runID := fmt.Sprintf("run-cosuper-assignment-%02d", i)
		f.assignedAgentIDs = append(f.assignedAgentIDs, agentID)
		f.assignedWorkIDs = append(f.assignedWorkIDs, workID)
		f.assignedRunIDs = append(f.assignedRunIDs, runID)
		// The standard assignment opener creates the lifecycle CoSuper subject
		// and its work atomically. Fixtures reserve only their deterministic IDs.
	}
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: objects}); err != nil {
		t.Fatalf("install assignment authority: %v", err)
	}
	if err := s.CreateRunOG(ctx, parentRun); err != nil {
		t.Fatalf("install non-lifecycle persistent Super run: %v", err)
	}
	return f
}

func coSuperOpenRequest(f coSuperAssignmentStoreFixture, index int, assignmentID string, attempt uint64, kind types.CoSuperAssignmentKind, writable bool, capability, capsuleID string) types.OpenCoSuperAssignmentRequest {
	binding := types.CoSuperAssignmentBinding{
		OwnerID: f.ownerID, ComputerID: f.computerID, TrajectoryID: f.trajectoryID,
		ParentAgentID: f.parentAgentID, ParentRunID: f.parentRunID,
		ParentDecisionID: f.parentDecisionID, ParentControlID: f.parentControlID,
		ParentWorkItemID: f.parentWorkID, AssignedWorkItemID: f.assignedWorkIDs[index], AssignedAgentID: f.assignedAgentIDs[index],
		Kind: kind, Attempt: attempt, ScopeDigest: objectgraph.SHA256([]byte("scope:" + assignmentID)),
		CapabilityDigest: DigestCoSuperOpaqueCapability(capability), SubjectDigest: objectgraph.SHA256([]byte("subject:" + assignmentID)),
		Writable: writable, CapsuleID: capsuleID,
		NetworkMode:    types.CoSuperCapsuleNetworkForbidden,
		FilesystemMode: types.CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay,
	}
	req := types.OpenCoSuperAssignmentRequest{
		CommandID: "command-open-" + assignmentID + fmt.Sprintf("-%d", attempt), AssignmentID: assignmentID, Binding: binding,
		AssignedAgent: types.AgentRecord{AgentID: binding.AssignedAgentID},
		AssignedWork: types.WorkItemRecord{WorkItemID: binding.AssignedWorkItemID, AssignedAgentID: binding.AssignedAgentID,
			Objective: "bounded delegated assignment"},
	}
	req.CommandDigest, _ = ComputeOpenCoSuperAssignmentDigest(req)
	return req
}

func bindCoSuperRequest(open types.OpenCoSuperAssignmentRequest, runID, capability string) types.BindCoSuperAssignmentRequest {
	run := types.RunRecord{
		RunID: runID, AgentID: open.Binding.AssignedAgentID, ChannelID: open.Binding.AssignedAgentID,
		RequestedByRunID: open.Binding.ParentRunID, TrajectoryID: open.Binding.TrajectoryID,
		AgentProfile: "co-super", AgentRole: "co-super", OwnerID: open.Binding.OwnerID, SandboxID: open.Binding.ComputerID,
		State: types.RunPending, Prompt: open.AssignedWork.Objective,
		Metadata: map[string]any{
			"work_item_ids": []string{open.Binding.AssignedWorkItemID}, "lifecycle_work_item_id": open.Binding.AssignedWorkItemID,
			"requested_by_agent_id": open.Binding.ParentAgentID, "requested_by_profile": "super",
			"assignment_id": open.AssignmentID, "assignment_attempt": open.Binding.Attempt, "assignment_kind": string(open.Binding.Kind),
			"assigned_work_item_id": open.Binding.AssignedWorkItemID, "parent_work_item_id": open.Binding.ParentWorkItemID,
			"parent_decision_id": open.Binding.ParentDecisionID, "parent_control_id": open.Binding.ParentControlID,
			"capsule_id": open.Binding.CapsuleID, "scope_digest": open.Binding.ScopeDigest,
			"capability_digest": open.Binding.CapabilityDigest, "subject_digest": open.Binding.SubjectDigest,
			"coordination_contract_id":     open.Binding.CoordinationContractID,
			"coordination_contract_digest": open.Binding.CoordinationContractDigest,
		},
	}
	req := types.BindCoSuperAssignmentRequest{
		CommandID: "command-bind-" + open.AssignmentID + fmt.Sprintf("-%d", open.Binding.Attempt),
		OwnerID:   open.Binding.OwnerID, ComputerID: open.Binding.ComputerID, AssignmentID: open.AssignmentID,
		Attempt: open.Binding.Attempt, ExpectedLifecycleVersion: 1, RunID: runID, Run: run,
		OpaqueCapability: capability, CapsuleID: open.Binding.CapsuleID,
	}
	req.CommandDigest, _ = ComputeBindCoSuperAssignmentDigest(req)
	return req
}

func TestCoSuperAssignmentCommandsReplayAndDigestConflict(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 1)
	open := coSuperOpenRequest(f, 0, "assignment-command-contract", 1, types.CoSuperAssignmentImplementation, true, "cap-command-contract", "capsule-command-contract")
	if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if replay, err := s.OpenCoSuperAssignment(ctx, open); err != nil || !replay.Replay {
		t.Fatalf("open replay: %+v, %v", replay, err)
	}
	openConflict := open
	openConflict.Binding.SubjectDigest = objectgraph.SHA256([]byte("changed-open"))
	openConflict.CommandDigest, _ = ComputeOpenCoSuperAssignmentDigest(openConflict)
	if _, err := s.OpenCoSuperAssignment(ctx, openConflict); !errors.Is(err, ErrCoSuperAssignmentCommandConflict) {
		t.Fatalf("open conflict: %v", err)
	}

	bind := bindCoSuperRequest(open, f.assignedRunIDs[0], "cap-command-contract")
	if _, err := s.BindCoSuperAssignment(ctx, bind); err != nil {
		t.Fatal(err)
	}
	if replay, err := s.BindCoSuperAssignment(ctx, bind); err != nil || !replay.Replay {
		t.Fatalf("bind replay: %+v, %v", replay, err)
	}
	bindConflict := bind
	bindConflict.OpaqueCapability = "other-capability"
	bindConflict.CommandDigest, _ = ComputeBindCoSuperAssignmentDigest(bindConflict)
	if _, err := s.BindCoSuperAssignment(ctx, bindConflict); !errors.Is(err, ErrCoSuperAssignmentCommandConflict) {
		t.Fatalf("bind conflict: %v", err)
	}

	report := assignmentReportRequest(open, 2, "report-command-contract", open.Binding.SubjectDigest, types.CoSuperResultPartial, types.CoSuperVerdictNone)
	if _, err := s.RecordCoSuperAssignmentReport(ctx, report); err != nil {
		t.Fatal(err)
	}
	if replay, err := s.RecordCoSuperAssignmentReport(ctx, report); err != nil || !replay.Replay || replay.Report == nil {
		t.Fatalf("report replay: %+v, %v", replay, err)
	}
	reportConflict := report
	reportConflict.Report.ObservedSubjectDigest = objectgraph.SHA256([]byte("other-report-subject"))
	reportConflict.CommandDigest, _ = ComputeRecordCoSuperAssignmentReportDigest(reportConflict)
	if _, err := s.RecordCoSuperAssignmentReport(ctx, reportConflict); !errors.Is(err, ErrCoSuperAssignmentCommandConflict) {
		t.Fatalf("report conflict: %v", err)
	}

	fate := types.SetCoSuperCapsuleDispositionRequest{
		CommandID: "command-fate-contract", OwnerID: f.ownerID, ComputerID: f.computerID,
		AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 3,
		Disposition: types.CoSuperCapsuleFreezeRequested, IntentRef: "intent:fate-contract",
	}
	fate.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(fate)
	if _, err := s.SetCoSuperCapsuleDisposition(ctx, fate); err != nil {
		t.Fatal(err)
	}
	if replay, err := s.SetCoSuperCapsuleDisposition(ctx, fate); err != nil || !replay.Replay {
		t.Fatalf("fate replay: %+v, %v", replay, err)
	}
	fateConflict := fate
	fateConflict.IntentRef = "intent:changed"
	fateConflict.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(fateConflict)
	if _, err := s.SetCoSuperCapsuleDisposition(ctx, fateConflict); !errors.Is(err, ErrCoSuperAssignmentCommandConflict) {
		t.Fatalf("fate conflict: %v", err)
	}

	cancel := types.CancelCoSuperAssignmentRequest{
		CommandID: "command-cancel-contract", OwnerID: f.ownerID, ComputerID: f.computerID,
		AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 4, Reason: "cancel contract test",
	}
	cancel.CommandDigest, _ = ComputeCancelCoSuperAssignmentDigest(cancel)
	if _, err := s.CancelCoSuperAssignment(ctx, cancel); err != nil {
		t.Fatal(err)
	}
	if replay, err := s.CancelCoSuperAssignment(ctx, cancel); err != nil || !replay.Replay {
		t.Fatalf("cancel replay: %+v, %v", replay, err)
	}
	cancelConflict := cancel
	cancelConflict.Reason = "changed reason"
	cancelConflict.CommandDigest, _ = ComputeCancelCoSuperAssignmentDigest(cancelConflict)
	if _, err := s.CancelCoSuperAssignment(ctx, cancelConflict); !errors.Is(err, ErrCoSuperAssignmentCommandConflict) {
		t.Fatalf("cancel conflict: %v", err)
	}
}

func TestCoSuperAssignmentsAllowManyParallelAttemptsAndReplay(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 13)
	requests := make([]types.OpenCoSuperAssignmentRequest, 12)
	for i := range requests {
		requests[i] = coSuperOpenRequest(f, i, fmt.Sprintf("assignment-%02d", i), 1, types.CoSuperAssignmentImplementation, true,
			fmt.Sprintf("opaque-capability-%02d", i), fmt.Sprintf("capsule-%02d", i))
	}
	first := requests[0]
	var wg sync.WaitGroup
	errs := make(chan error, len(requests))
	for _, req := range requests {
		req := req
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := s.OpenCoSuperAssignment(ctx, req)
			if err != nil {
				errs <- err
				return
			}
			if result.Replay || result.Assignment.LifecycleVersion != 1 {
				errs <- fmt.Errorf("unexpected concurrent open result: %+v", result)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent many-assignment open: %v", err)
	}
	attemptTwo := coSuperOpenRequest(f, 12, first.AssignmentID, 2, types.CoSuperAssignmentVerification, true, "opaque-attempt-two", "capsule-attempt-two")
	if _, err := s.OpenCoSuperAssignment(ctx, attemptTwo); err != nil {
		t.Fatalf("open second attempt: %v", err)
	}
	assignments, err := s.ListCoSuperAssignments(ctx, f.ownerID, f.computerID, f.trajectoryID)
	if err != nil || len(assignments) != 13 {
		t.Fatalf("many assignment list = %d, %v", len(assignments), err)
	}
	replay, err := s.OpenCoSuperAssignment(ctx, first)
	if err != nil || !replay.Replay || replay.Assignment.AssignmentID != first.AssignmentID {
		t.Fatalf("open replay: %+v, %v", replay, err)
	}
	conflict := first
	conflict.AssignmentID = "changed-assignment"
	conflict.CommandDigest, _ = ComputeOpenCoSuperAssignmentDigest(conflict)
	if _, err := s.OpenCoSuperAssignment(ctx, conflict); !errors.Is(err, ErrCoSuperAssignmentCommandConflict) {
		t.Fatalf("command conflict = %v", err)
	}
	duplicate := first
	duplicate.CommandID = "command-open-duplicate-attempt"
	duplicate.CommandDigest, _ = ComputeOpenCoSuperAssignmentDigest(duplicate)
	if _, err := s.OpenCoSuperAssignment(ctx, duplicate); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("duplicate assignment attempt = %v, want CAS conflict", err)
	}
}

func TestCoSuperAssignmentRejectsCrossScopeAndAuthorityMismatches(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 3)
	base := coSuperOpenRequest(f, 0, "assignment-scope", 1, types.CoSuperAssignmentImplementation, true, "opaque-scope", "capsule-scope")
	for name, mutate := range map[string]func(*types.OpenCoSuperAssignmentRequest){
		"owner": func(r *types.OpenCoSuperAssignmentRequest) {
			r.Binding.OwnerID, r.Binding.ParentAgentID = "other-owner", "super:other-owner"
		},
		"computer":       func(r *types.OpenCoSuperAssignmentRequest) { r.Binding.ComputerID = "other-computer" },
		"trajectory":     func(r *types.OpenCoSuperAssignmentRequest) { r.Binding.TrajectoryID = "other-trajectory" },
		"decision":       func(r *types.OpenCoSuperAssignmentRequest) { r.Binding.ParentDecisionID = "other-decision" },
		"control":        func(r *types.OpenCoSuperAssignmentRequest) { r.Binding.ParentControlID = "other-control" },
		"parent work":    func(r *types.OpenCoSuperAssignmentRequest) { r.Binding.ParentWorkItemID = f.assignedWorkIDs[1] },
		"assigned work":  func(r *types.OpenCoSuperAssignmentRequest) { r.Binding.AssignedWorkItemID = f.assignedWorkIDs[1] },
		"assigned agent": func(r *types.OpenCoSuperAssignmentRequest) { r.Binding.AssignedAgentID = f.assignedAgentIDs[1] },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.CommandID = "command-invalid-" + name
			mutate(&candidate)
			candidate.CommandDigest, _ = ComputeOpenCoSuperAssignmentDigest(candidate)
			if _, err := s.OpenCoSuperAssignment(ctx, candidate); err == nil {
				t.Fatal("cross-scope assignment accepted")
			}
		})
	}
}

func TestCoSuperAssignmentAcceptsPassivatedPersistentSuperControlRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 1)
	run, err := s.GetRunByOwnerOG(ctx, f.ownerID, f.parentRunID)
	if err != nil {
		t.Fatal(err)
	}
	run.State = types.RunPassivated
	run.UpdatedAt = time.Now().UTC()
	if err := s.UpdateRunOG(ctx, run); err != nil {
		t.Fatal(err)
	}
	req := coSuperOpenRequest(f, 0, "assignment-passivated-parent", 1, types.CoSuperAssignmentVerification, true, "cap-passivated", "capsule-passivated")
	if _, err := s.OpenCoSuperAssignment(ctx, req); err != nil {
		t.Fatalf("passivated persistent Super control run should remain exact authority: %v", err)
	}
}

func TestCoSuperAssignmentRejectsGenericLifecycleSuperSubstitute(t *testing.T) {
	t.Run("lifecycle run", func(t *testing.T) {
		s := openTestStore(t)
		ctx := context.Background()
		f := installCoSuperAssignmentAuthority(t, s, 1)
		persistentRunObj, err := s.getRunObjectByOwnerOG(ctx, f.ownerID, f.parentRunID)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ogStore.DeleteObject(ctx, persistentRunObj.CanonicalID); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		lifecycleRun := types.RunRecord{
			RunID: f.parentRunID, AgentID: f.parentAgentID, TrajectoryID: f.trajectoryID,
			AgentProfile: "super", AgentRole: "super", OwnerID: f.ownerID, SandboxID: f.computerID,
			State: types.RunRunning, CreatedAt: now, UpdatedAt: now,
			Metadata: map[string]any{"assignment_trajectory_id": f.trajectoryID, "parent_work_item_id": f.parentWorkID,
				"parent_decision_id": f.parentDecisionID, "parent_control_id": f.parentControlID},
		}
		obj, err := lifecycleObject(ogKindRun, f.ownerID, f.computerID, f.parentRunID, lifecycleRun,
			map[string]any{"run_id": f.parentRunID, "computer_id": f.computerID, "trajectory_id": f.trajectoryID}, now, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ogStore.PutObject(ctx, obj); err != nil {
			t.Fatal(err)
		}
		req := coSuperOpenRequest(f, 0, "assignment-lifecycle-super-run", 1, types.CoSuperAssignmentImplementation, true, "cap", "capsule")
		if _, err := s.OpenCoSuperAssignment(ctx, req); err == nil {
			t.Fatal("generic lifecycle Super run substituted for persistent Super authority")
		}
	})
	t.Run("lifecycle agent", func(t *testing.T) {
		s := openTestStore(t)
		ctx := context.Background()
		f := installCoSuperAssignmentAuthority(t, s, 1)
		agentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, f.ownerID, f.computerID, f.parentAgentID)
		if err != nil {
			t.Fatal(err)
		}
		agent, err := decodeLifecycleObject[types.AgentRecord](agentObj)
		if err != nil {
			t.Fatal(err)
		}
		agent.LifecycleVersion = 1
		updated, err := lifecycleObject(ogKindAgent, f.ownerID, f.computerID, f.parentAgentID, agent,
			map[string]any{"agent_id": f.parentAgentID, "computer_id": f.computerID, "trajectory_id": f.trajectoryID}, agentObj.CreatedAt, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ogStore.PutObject(ctx, updated); err != nil {
			t.Fatal(err)
		}
		req := coSuperOpenRequest(f, 0, "assignment-lifecycle-super-agent", 1, types.CoSuperAssignmentImplementation, true, "cap", "capsule")
		if _, err := s.OpenCoSuperAssignment(ctx, req); !errors.Is(err, ErrCoSuperAssignmentInvalid) {
			t.Fatalf("lifecycle Super agent error = %v", err)
		}
	})
}

func TestCoSuperAssignmentBindClaimsCapabilityAndCapsuleUniquely(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 6)
	opens := []types.OpenCoSuperAssignmentRequest{
		coSuperOpenRequest(f, 0, "assignment-bind-0", 1, types.CoSuperAssignmentImplementation, true, "shared-capability", "shared-capsule"),
		coSuperOpenRequest(f, 1, "assignment-bind-1", 1, types.CoSuperAssignmentImplementation, true, "shared-capability", "capsule-one"),
		coSuperOpenRequest(f, 2, "assignment-bind-2", 1, types.CoSuperAssignmentImplementation, true, "capability-two", "shared-capsule"),
		coSuperOpenRequest(f, 3, "assignment-bind-3", 1, types.CoSuperAssignmentImplementation, true, "capability-three", "capsule-three"),
		coSuperOpenRequest(f, 4, "assignment-bind-4", 1, types.CoSuperAssignmentImplementation, true, "capability-four", "capsule-four"),
		coSuperOpenRequest(f, 5, "assignment-bind-5", 1, types.CoSuperAssignmentImplementation, true, "capability-five", "capsule-five"),
	}
	opens[2].Binding.CoordinationContractID = "coordination-future-only"
	opens[2].Binding.CoordinationContractDigest = objectgraph.SHA256([]byte("coordination-future-only"))
	opens[2].CommandDigest, _ = ComputeOpenCoSuperAssignmentDigest(opens[2])
	for _, open := range opens {
		if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
			t.Fatalf("open %s: %v", open.AssignmentID, err)
		}
	}
	firstBind := bindCoSuperRequest(opens[0], f.assignedRunIDs[0], "shared-capability")
	bound, err := s.BindCoSuperAssignment(ctx, firstBind)
	if err != nil || bound.Assignment.Disposition != types.CoSuperAssignmentBound {
		t.Fatalf("bind first: %+v, %v", bound, err)
	}
	replay, err := s.BindCoSuperAssignment(ctx, firstBind)
	if err != nil || !replay.Replay || replay.Assignment.BoundRunID != f.assignedRunIDs[0] {
		t.Fatalf("bind replay: %+v, %v", replay, err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(opens[1], f.assignedRunIDs[1], "shared-capability")); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("duplicate capability error = %v", err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(opens[2], f.assignedRunIDs[2], "capability-two")); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("duplicate capsule error = %v", err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(opens[3], f.assignedRunIDs[3], "capability-three")); err != nil {
		t.Fatalf("independent bind: %v", err)
	}
	wrongRun := bindCoSuperRequest(opens[4], f.assignedRunIDs[4], "capability-four")
	wrongRun.Run.AgentID = opens[3].Binding.AssignedAgentID
	wrongRun.CommandDigest, _ = ComputeBindCoSuperAssignmentDigest(wrongRun)
	if _, err := s.BindCoSuperAssignment(ctx, wrongRun); !errors.Is(err, ErrCoSuperAssignmentInvalid) {
		t.Fatalf("cross-assignment run error = %v", err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(opens[5], f.assignedRunIDs[3], "capability-five")); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("duplicate run claim error = %v", err)
	}
	snapshot, err := s.ogStore.ReadObjectSnapshot(ctx, f.ownerID, f.computerID)
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range snapshot {
		if strings.Contains(string(obj.Body), "shared-capability") {
			t.Fatalf("opaque capability leaked into durable object %s", obj.CanonicalID)
		}
	}
}

func assignmentReportRequest(open types.OpenCoSuperAssignmentRequest, version int64, reportID string, observedDigest string, result types.CoSuperAssignmentResultKind, verdict types.CoSuperAssignmentVerdict) types.RecordCoSuperAssignmentReportRequest {
	req := types.RecordCoSuperAssignmentReportRequest{
		CommandID: "command-report-" + reportID, OwnerID: open.Binding.OwnerID, ComputerID: open.Binding.ComputerID,
		AssignmentID: open.AssignmentID, Attempt: open.Binding.Attempt, ExpectedLifecycleVersion: version,
		Report: types.CoSuperAssignmentReport{
			ReportID: reportID, Result: result, Verdict: verdict, ObservedSubjectDigest: observedDigest,
			Commands: []types.CoSuperRecordedCommand{{CommandID: "observed-command", CommandDigest: objectgraph.SHA256([]byte("command")), ExecutionRef: "receipt:execution"}},
			Outputs:  []types.CoSuperRecordedOutput{{OutputID: "output", Kind: "evidence", Digest: objectgraph.SHA256([]byte("output")), Ref: "artifact:output"}},
		},
	}
	req.CommandDigest, _ = ComputeRecordCoSuperAssignmentReportDigest(req)
	return req
}

func TestVerificationSubjectChangeCreatesCandidateAndCannotCertifyOriginal(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 3)
	immutable := coSuperOpenRequest(f, 0, "assignment-verify-immutable", 1, types.CoSuperAssignmentVerification, true, "verify-capability-0", "capsule-verify-0")
	changed := coSuperOpenRequest(f, 1, "assignment-verify-changed", 1, types.CoSuperAssignmentVerification, true, "verify-capability-1", "capsule-verify-1")
	changedSameBytes := coSuperOpenRequest(f, 2, "assignment-verify-changed-same-bytes", 1, types.CoSuperAssignmentVerification, true, "verify-capability-2", "capsule-verify-2")
	for index, open := range []types.OpenCoSuperAssignmentRequest{immutable, changed, changedSameBytes} {
		if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
			t.Fatal(err)
		}
		if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(open, f.assignedRunIDs[index], fmt.Sprintf("verify-capability-%d", index))); err != nil {
			t.Fatal(err)
		}
	}
	passReq := assignmentReportRequest(immutable, 2, "report-immutable", immutable.Binding.SubjectDigest, types.CoSuperResultCompleted, types.CoSuperVerdictPass)
	pass, err := s.RecordCoSuperAssignmentReport(ctx, passReq)
	if err != nil || pass.Report == nil || !pass.Report.CertifiesOriginalSubject || pass.Candidate != nil {
		t.Fatalf("immutable verification result: %+v, %v", pass, err)
	}
	newDigest := objectgraph.SHA256([]byte("changed subject bytes"))
	changedReq := assignmentReportRequest(changed, 2, "report-changed", newDigest, types.CoSuperResultCompleted, types.CoSuperVerdictPass)
	changedReq.Report.CertifiesOriginalSubject = true // authored claim must be ignored by the reducer
	changedReq.Report.Mutations = []types.CoSuperRecordedMutation{{
		MutationID: "subject-mutation", Kind: "subject_bytes", BeforeDigest: changed.Binding.SubjectDigest,
		AfterDigest: newDigest, EvidenceRef: "receipt:mutation", SubjectBytesChanged: true,
	}}
	changedReq.CommandDigest, _ = ComputeRecordCoSuperAssignmentReportDigest(changedReq)
	result, err := s.RecordCoSuperAssignmentReport(ctx, changedReq)
	if err != nil || result.Report == nil || result.Report.CertifiesOriginalSubject || result.Candidate == nil ||
		result.Candidate.SubjectDigest != newDigest || result.Report.CandidateID == "" {
		t.Fatalf("changed verification result: %+v, %v", result, err)
	}
	if result.Assignment.Binding.SubjectDigest != changed.Binding.SubjectDigest {
		t.Fatal("changed verification rewrote original subject identity")
	}
	replay, err := s.RecordCoSuperAssignmentReport(ctx, changedReq)
	if err != nil || !replay.Replay || replay.Candidate == nil || replay.Candidate.CandidateID != result.Candidate.CandidateID {
		t.Fatalf("changed report replay: %+v, %v", replay, err)
	}
	sameBytesReq := assignmentReportRequest(changedSameBytes, 2, "report-changed-same-bytes", newDigest, types.CoSuperResultCompleted, types.CoSuperVerdictPass)
	sameBytesReq.Report.Mutations = []types.CoSuperRecordedMutation{{
		MutationID: "subject-mutation-same-bytes", Kind: "subject_bytes", BeforeDigest: changedSameBytes.Binding.SubjectDigest,
		AfterDigest: newDigest, EvidenceRef: "receipt:mutation-same-bytes", SubjectBytesChanged: true,
	}}
	sameBytesReq.CommandDigest, _ = ComputeRecordCoSuperAssignmentReportDigest(sameBytesReq)
	sameBytesResult, err := s.RecordCoSuperAssignmentReport(ctx, sameBytesReq)
	if err != nil || sameBytesResult.Candidate == nil || sameBytesResult.Candidate.CandidateID == result.Candidate.CandidateID ||
		sameBytesResult.Candidate.SubjectDigest != result.Candidate.SubjectDigest {
		t.Fatalf("assignment-specific candidate identity for same bytes: first=%+v second=%+v err=%v", result.Candidate, sameBytesResult.Candidate, err)
	}
}

func TestCancelledAssignmentAcceptsLateResultWithoutReopeningOrCertifying(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 1)
	open := coSuperOpenRequest(f, 0, "assignment-cancel-late", 1, types.CoSuperAssignmentVerification, true, "late-capability", "capsule-late")
	if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(open, f.assignedRunIDs[0], "late-capability")); err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelCoSuperAssignmentRequest{
		CommandID: "command-cancel-late", OwnerID: f.ownerID, ComputerID: f.computerID,
		AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 2,
		Reason: "parent withdrew assignment",
	}
	cancel.CommandDigest, _ = ComputeCancelCoSuperAssignmentDigest(cancel)
	cancelled, err := s.CancelCoSuperAssignment(ctx, cancel)
	if err != nil || cancelled.Assignment.Disposition != types.CoSuperAssignmentCancelled {
		t.Fatalf("cancel: %+v, %v", cancelled, err)
	}
	lateReq := assignmentReportRequest(open, 3, "report-late", open.Binding.SubjectDigest, types.CoSuperResultCompleted, types.CoSuperVerdictPass)
	late, err := s.RecordCoSuperAssignmentReport(ctx, lateReq)
	if err != nil || late.Report == nil || !late.Report.Late || late.Report.CertifiesOriginalSubject ||
		late.Assignment.Disposition != types.CoSuperAssignmentCancelled || late.Assignment.TerminalAt == nil {
		t.Fatalf("late result: %+v, %v", late, err)
	}
	stored, err := s.GetCoSuperAssignment(ctx, f.ownerID, f.computerID, open.AssignmentID, 1)
	if err != nil || stored.Disposition != types.CoSuperAssignmentCancelled || stored.LifecycleVersion != 4 {
		t.Fatalf("stored late assignment: %+v, %v", stored, err)
	}
}

func TestCapsuleFreezeAcknowledgementPermitsBoundTerminalReport(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 1)
	open := coSuperOpenRequest(f, 0, "assignment-freeze-only", 1, types.CoSuperAssignmentVerification, true, "cap-freeze-only", "capsule-freeze-only")
	if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(open, f.assignedRunIDs[0], "cap-freeze-only")); err != nil {
		t.Fatal(err)
	}
	intent := types.SetCoSuperCapsuleDispositionRequest{
		CommandID: "command-freeze-only-intent", OwnerID: f.ownerID, ComputerID: f.computerID,
		AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 2,
		Disposition: types.CoSuperCapsuleFreezeRequested, IntentRef: "intent:freeze-only",
	}
	intent.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(intent)
	if _, err := s.SetCoSuperCapsuleDisposition(ctx, intent); err != nil {
		t.Fatal(err)
	}
	ack := intent
	ack.CommandID, ack.ExpectedLifecycleVersion = "command-freeze-only-ack", 3
	ack.Disposition, ack.AckRef = types.CoSuperCapsuleFrozen, "receipt:freeze-only"
	ack.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(ack)
	result, err := s.SetCoSuperCapsuleDisposition(ctx, ack)
	if err != nil || result.Assignment.CapsuleDisposition != types.CoSuperCapsuleFrozen || result.Assignment.Disposition != types.CoSuperAssignmentBound {
		t.Fatalf("freeze-only assignment: %+v, %v", result, err)
	}
	delayed := assignmentReportRequest(open, 4, "report-after-freeze", open.Binding.SubjectDigest, types.CoSuperResultCompleted, types.CoSuperVerdictPass)
	reported, err := s.RecordCoSuperAssignmentReport(ctx, delayed)
	if err != nil || reported.Report == nil || reported.Report.Late || !reported.Report.CertifiesOriginalSubject ||
		reported.Assignment.Disposition != types.CoSuperAssignmentCompleted || reported.Assignment.CapsuleDisposition != types.CoSuperCapsuleFrozen {
		t.Fatalf("frozen terminal report did not preserve exact certification: %+v, %v", reported, err)
	}
}

func TestAssignmentOutcomeAndCapsuleFateRemainSeparateAcrossRestart(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 3)
	opens := []types.OpenCoSuperAssignmentRequest{
		coSuperOpenRequest(f, 0, "assignment-complete-freeze", 1, types.CoSuperAssignmentImplementation, true, "cap-freeze", "capsule-freeze"),
		coSuperOpenRequest(f, 1, "assignment-cancel-revoke", 1, types.CoSuperAssignmentImplementation, true, "cap-revoke", "capsule-revoke"),
		coSuperOpenRequest(f, 2, "assignment-failed-active", 1, types.CoSuperAssignmentImplementation, true, "cap-failed", "capsule-failed"),
	}
	caps := []string{"cap-freeze", "cap-revoke", "cap-failed"}
	for index, open := range opens {
		if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
			t.Fatal(err)
		}
		if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(open, f.assignedRunIDs[index], caps[index])); err != nil {
			t.Fatal(err)
		}
	}
	completedReport := assignmentReportRequest(opens[0], 2, "report-completed", opens[0].Binding.SubjectDigest, types.CoSuperResultCompleted, types.CoSuperVerdictNone)
	completed, err := s.RecordCoSuperAssignmentReport(ctx, completedReport)
	if err != nil || completed.Assignment.Disposition != types.CoSuperAssignmentCompleted || completed.Assignment.CapsuleDisposition != types.CoSuperCapsuleActive {
		t.Fatalf("completed assignment outcome/fate: %+v, %v", completed, err)
	}
	freezeIntent := types.SetCoSuperCapsuleDispositionRequest{
		CommandID: "command-freeze-intent", OwnerID: f.ownerID, ComputerID: f.computerID,
		AssignmentID: opens[0].AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 3,
		Disposition: types.CoSuperCapsuleFreezeRequested, IntentRef: "intent:freeze",
	}
	freezeIntent.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(freezeIntent)
	requested, err := s.SetCoSuperCapsuleDisposition(ctx, freezeIntent)
	if err != nil || requested.Assignment.CapsuleDisposition != types.CoSuperCapsuleFreezeRequested || requested.Assignment.Disposition != types.CoSuperAssignmentCompleted {
		t.Fatalf("freeze intent: %+v, %v", requested, err)
	}
	freezeAck := freezeIntent
	freezeAck.CommandID, freezeAck.ExpectedLifecycleVersion = "command-freeze-ack", 4
	freezeAck.Disposition, freezeAck.AckRef = types.CoSuperCapsuleFrozen, "receipt:freeze"
	freezeAck.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(freezeAck)
	frozen, err := s.SetCoSuperCapsuleDisposition(ctx, freezeAck)
	if err != nil || frozen.Assignment.CapsuleDisposition != types.CoSuperCapsuleFrozen || frozen.Assignment.Disposition != types.CoSuperAssignmentCompleted {
		t.Fatalf("freeze ack: %+v, %v", frozen, err)
	}

	cancel := types.CancelCoSuperAssignmentRequest{
		CommandID: "command-cancel-revoke", OwnerID: f.ownerID, ComputerID: f.computerID,
		AssignmentID: opens[1].AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 2, Reason: "parent cancelled",
	}
	cancel.CommandDigest, _ = ComputeCancelCoSuperAssignmentDigest(cancel)
	cancelled, err := s.CancelCoSuperAssignment(ctx, cancel)
	if err != nil || cancelled.Assignment.Disposition != types.CoSuperAssignmentCancelled || cancelled.Assignment.CapsuleDisposition != types.CoSuperCapsuleActive {
		t.Fatalf("cancel leaves capsule fate separate: %+v, %v", cancelled, err)
	}
	revokeIntent := types.SetCoSuperCapsuleDispositionRequest{
		CommandID: "command-revoke-intent", OwnerID: f.ownerID, ComputerID: f.computerID,
		AssignmentID: opens[1].AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 3,
		Disposition: types.CoSuperCapsuleRevokeRequested, IntentRef: "intent:revoke",
	}
	revokeIntent.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(revokeIntent)
	if _, err := s.SetCoSuperCapsuleDisposition(ctx, revokeIntent); err != nil {
		t.Fatal(err)
	}
	missingAck := revokeIntent
	missingAck.CommandID, missingAck.ExpectedLifecycleVersion, missingAck.Disposition = "command-revoke-no-ack", 4, types.CoSuperCapsuleRevoked
	missingAck.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(missingAck)
	if _, err := s.SetCoSuperCapsuleDisposition(ctx, missingAck); !errors.Is(err, ErrCoSuperAssignmentInvalid) {
		t.Fatalf("revocation without ack = %v", err)
	}
	revokeAck := missingAck
	revokeAck.CommandID, revokeAck.AckRef = "command-revoke-ack", "receipt:revoke"
	revokeAck.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(revokeAck)
	revoked, err := s.SetCoSuperCapsuleDisposition(ctx, revokeAck)
	if err != nil || revoked.Assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked || revoked.Assignment.Disposition != types.CoSuperAssignmentCancelled {
		t.Fatalf("receipt-backed revoke: %+v, %v", revoked, err)
	}

	failedReport := assignmentReportRequest(opens[2], 2, "report-failed", opens[2].Binding.SubjectDigest, types.CoSuperResultFailed, types.CoSuperVerdictNone)
	failed, err := s.RecordCoSuperAssignmentReport(ctx, failedReport)
	if err != nil || failed.Assignment.Disposition != types.CoSuperAssignmentFailed || failed.Assignment.CapsuleDisposition != types.CoSuperCapsuleActive {
		t.Fatalf("failed outcome keeps active capsule fate: %+v, %v", failed, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	replay, err := s.SetCoSuperCapsuleDisposition(ctx, freezeAck)
	if err != nil || !replay.Replay || replay.Assignment.CapsuleDisposition != types.CoSuperCapsuleFrozen {
		t.Fatalf("restart standard-receipt replay: %+v, %v", replay, err)
	}
	receiptObj, err := s.lifecycleGetObject(ctx, ogKindLifecycleCmd, f.ownerID, f.computerID, freezeAck.CommandID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := decodeLifecycleObject[types.LifecycleCommandReceipt](receiptObj)
	if err != nil || receipt.Kind != types.LifecycleSetCoSuperCapsuleDisposition {
		t.Fatalf("assignment command did not use standard lifecycle receipt: %+v, %v", receipt, err)
	}
	objects, err := s.ogStore.ReadObjectSnapshot(ctx, f.ownerID, f.computerID)
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range objects {
		if obj.ObjectKind == objectgraph.ObjectKind("choir.co_super_assignment_command") {
			t.Fatal("parallel co-super command tape persisted")
		}
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, f.ownerID, f.computerID, f.trajectoryID)
	if err != nil || len(snapshot.CoSuperAssignments) != 3 || len(snapshot.Events) < 13 {
		t.Fatalf("restart lifecycle snapshot assignments/events = %d/%d, %v", len(snapshot.CoSuperAssignments), len(snapshot.Events), err)
	}
	page, err := s.ListLifecycleEventPage(ctx, f.ownerID, f.computerID, f.trajectoryID, 1, 100)
	if err != nil || len(page.Events) != len(snapshot.Events) {
		t.Fatalf("assignment events absent from lifecycle paging: %d/%d, %v", len(page.Events), len(snapshot.Events), err)
	}
}

func TestOpenedAssignmentCapsuleCleanupIntentAckBeforeCancel(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 1)
	open := coSuperOpenRequest(f, 0, "assignment-prebind-cleanup", 1, types.CoSuperAssignmentImplementation, true, "cap", "capsule-prebind")
	if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	intent := types.SetCoSuperCapsuleDispositionRequest{CommandID: "prebind-revoke-intent", OwnerID: f.ownerID, ComputerID: f.computerID, AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 1, Disposition: types.CoSuperCapsuleRevokeRequested, IntentRef: "intent:prebind"}
	intent.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(intent)
	requested, err := s.SetCoSuperCapsuleDisposition(ctx, intent)
	if err != nil || requested.Assignment.Disposition != types.CoSuperAssignmentOpen {
		t.Fatalf("intent: %+v %v", requested, err)
	}
	ack := intent
	ack.CommandID = "prebind-revoke-ack"
	ack.ExpectedLifecycleVersion = 2
	ack.Disposition = types.CoSuperCapsuleRevoked
	ack.AckRef = "ack:absent"
	ack.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(ack)
	revoked, err := s.SetCoSuperCapsuleDisposition(ctx, ack)
	if err != nil || revoked.Assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
		t.Fatalf("ack: %+v %v", revoked, err)
	}
	cancel := types.CancelCoSuperAssignmentRequest{CommandID: "cancel-prebind", OwnerID: f.ownerID, ComputerID: f.computerID, AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 3, Reason: "mint failed"}
	cancel.CommandDigest, _ = ComputeCancelCoSuperAssignmentDigest(cancel)
	cancelled, err := s.CancelCoSuperAssignment(ctx, cancel)
	if err != nil || cancelled.Assignment.Disposition != types.CoSuperAssignmentCancelled || cancelled.Assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
		t.Fatalf("cancel: %+v %v", cancelled, err)
	}
}

func TestReplayRecordedReportAfterRevokeReturnsOriginalReceipt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installCoSuperAssignmentAuthority(t, s, 1)
	open := coSuperOpenRequest(f, 0, "assignment-report-replay", 1, types.CoSuperAssignmentVerification, true, "cap", "capsule")
	if _, err := s.OpenCoSuperAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindCoSuperAssignment(ctx, bindCoSuperRequest(open, f.assignedRunIDs[0], "cap")); err != nil {
		t.Fatal(err)
	}
	req := assignmentReportRequest(open, 2, "report-replay", open.Binding.SubjectDigest, types.CoSuperResultCompleted, types.CoSuperVerdictPass)
	changedCAS := req
	changedCAS.ExpectedLifecycleVersion = 99
	changedDigest, _ := ComputeRecordCoSuperAssignmentReportDigest(changedCAS)
	if changedDigest != req.CommandDigest {
		t.Fatal("CAS precondition changed semantic report occurrence digest")
	}
	reported, err := s.RecordCoSuperAssignmentReport(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	intent := types.SetCoSuperCapsuleDispositionRequest{CommandID: "replay-revoke-intent", OwnerID: f.ownerID, ComputerID: f.computerID, AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 3, Disposition: types.CoSuperCapsuleRevokeRequested, IntentRef: "intent:replay"}
	intent.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(intent)
	x, err := s.SetCoSuperCapsuleDisposition(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	ack := intent
	ack.CommandID = "replay-revoke-ack"
	ack.ExpectedLifecycleVersion = x.Assignment.LifecycleVersion
	ack.Disposition = types.CoSuperCapsuleRevoked
	ack.AckRef = "ack:replay"
	ack.CommandDigest, _ = ComputeSetCoSuperCapsuleDispositionDigest(ack)
	if _, err := s.SetCoSuperCapsuleDisposition(ctx, ack); err != nil {
		t.Fatal(err)
	}
	replay, err := s.ReplayRecordedCoSuperAssignmentReport(ctx, f.ownerID, f.computerID, open.AssignmentID, 1, req.Report.ReportID, req.CommandID)
	if err != nil || !replay.Replay || replay.Report == nil || replay.Receipt.CommandDigest != reported.Receipt.CommandDigest || replay.Assignment.CapsuleDisposition != types.CoSuperCapsuleRevoked {
		t.Fatalf("replay: %+v %v", replay, err)
	}
}
