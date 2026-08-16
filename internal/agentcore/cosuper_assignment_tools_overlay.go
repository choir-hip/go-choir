package agentcore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func (rt *Runtime) assignedCoSuperToolOverlay(ctx context.Context, rec *types.RunRecord, base *toolregistry.ToolRegistry) (*toolregistry.ToolRegistry, string, error) {
	if rec == nil || agentprofile.Canonical(agentProfileForRun(rec)) != agentprofile.CoSuper {
		return base, "", nil
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	if assignmentID == "" && attempt == 0 {
		return nil, "", fmt.Errorf("unassigned CoSuper cannot execute")
	}
	if assignmentID == "" || attempt == 0 || rt.capsuleExecutor == nil {
		return nil, "", fmt.Errorf("assigned CoSuper tool overlay binding unavailable")
	}
	lookup := rt.assignmentLookup
	if lookup == nil {
		lookup = rt.store
	}
	assignment, err := lookup.GetCoSuperAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, attempt)
	if err != nil {
		return nil, "", err
	}
	if assignment.Disposition != types.CoSuperAssignmentBound || assignment.CapsuleDisposition != types.CoSuperCapsuleActive ||
		assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedAgentID != rec.AgentID ||
		assignment.Binding.TrajectoryID != rec.TrajectoryID || assignment.Binding.ComputerID != rec.ComputerID ||
		metadataStringValue(rec.Metadata, "capsule_id") != assignment.Binding.CapsuleID ||
		metadataStringValue(rec.Metadata, "capability_digest") != assignment.Binding.CapabilityDigest ||
		metadataStringValue(rec.Metadata, "execution_handle_digest") != assignment.Binding.ExecutionHandleDigest ||
		metadataStringValue(rec.Metadata, "assigned_work_item_id") != assignment.Binding.AssignedWorkItemID ||
		metadataStringValue(rec.Metadata, "assignment_kind") != string(assignment.Binding.Kind) ||
		metadataStringValue(rec.Metadata, "request_digest") != assignment.Binding.RequestDigest ||
		metadataStringValue(rec.Metadata, "source_artifact_ref") != assignment.Binding.SourceArtifactRef ||
		metadataStringValue(rec.Metadata, "source_candidate_id") != assignment.Binding.SourceCandidateID {
		return nil, "", fmt.Errorf("assigned CoSuper durable run binding mismatch")
	}
	resolver := rt.assignmentHandleResolver
	if resolver == nil {
		resolver = rt.capsuleExecutor
	}
	handle, err := resolver.AssignmentHandle(rec.RunID, assignment.Binding.CapsuleID)
	if err != nil || strings.TrimSpace(handle) == "" {
		return nil, "", fmt.Errorf("assigned CoSuper runtime capability unavailable: %w", err)
	}
	// Never clone the static profile registry. Assignment authority is a fresh
	// exact closed set so no read_file/glob/grep/evidence/model host callback
	// can cross the durable assignment boundary by registry inheritance.
	// update_coagent is the Super report channel. Freeze/inspect/verify are
	// capsule-bound worker authority, not host mutation or owner decision.
	registry, err := buildAssignedCoSuperRegistry(rt)
	if err != nil {
		return nil, "", err
	}
	return registry, handle, nil
}

func (rt *Runtime) assignedCoSuperCapsuleToolCtx(rec *types.RunRecord, handle string) *CapsuleToolCtx {
	toolCtx := &CapsuleToolCtx{
		Executor: rt.capsuleExecutor, AgentRunID: rec.RunID, ComputerID: rec.ComputerID,
		Role: capsule.RoleCoSuper, CapsuleHandle: handle,
		EventAppender: rt.eventAppender, TransactionBuilder: rt.capsuleBuilder,
		OperationStore: rt.selfdevOperations, UpdaterRoot: rt.selfdevUpdaterRoot,
		ValidateCurrentObligation: func(callCtx context.Context) error {
			return rt.validateAssignedCoSuperExecution(callCtx, rec)
		},
	}
	if rt != nil && rt.store != nil {
		toolCtx.EventProjection = rt.store
	}
	return toolCtx
}

func (rt *Runtime) validateAssignedCoSuperExecution(ctx context.Context, rec *types.RunRecord) error {
	if rec == nil || rt == nil || rt.store == nil || rt.capsuleExecutor == nil {
		return fmt.Errorf("assigned CoSuper execution authority unavailable")
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	assignment, err := rt.store.GetCoSuperAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, attempt)
	if err != nil {
		return err
	}
	if assignment.Disposition != types.CoSuperAssignmentBound || assignment.CapsuleDisposition != types.CoSuperCapsuleActive ||
		assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedWorkItemID != metadataStringValue(rec.Metadata, "assigned_work_item_id") {
		return fmt.Errorf("assignment fate is not active and bound")
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, rec.OwnerID, rec.ComputerID, assignment.Binding.TrajectoryID)
	if err != nil || trajectory.Status != types.TrajectoryLive {
		return fmt.Errorf("trajectory obligation is not live: %w", err)
	}
	if _, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, rec.OwnerID, rec.ComputerID, assignment.Binding.TrajectoryID); intentErr == nil {
		return fmt.Errorf("trajectory cancellation is already authoritative")
	} else if !errors.Is(intentErr, store.ErrNotFound) {
		return fmt.Errorf("trajectory cancellation authority unavailable: %w", intentErr)
	}
	work, err := rt.store.GetLifecycleWorkItem(ctx, rec.OwnerID, rec.ComputerID, assignment.Binding.AssignedWorkItemID)
	if err != nil || work.Status != types.WorkItemOpen || work.AssignedAgentID != rec.AgentID {
		return fmt.Errorf("assigned work obligation is not open: %w", err)
	}
	storedRun, err := rt.getRunForComputer(ctx, rec.OwnerID, rec.RunID)
	if err != nil || !storedRun.State.Active() || storedRun.AgentID != rec.AgentID || storedRun.TrajectoryID != assignment.Binding.TrajectoryID {
		return fmt.Errorf("assigned run obligation is not active: %w", err)
	}
	handle, err := rt.capsuleExecutor.AssignmentHandle(rec.RunID, assignment.Binding.CapsuleID)
	if err != nil || strings.TrimSpace(handle) == "" || !rt.capsuleExecutor.HasCapsule(assignment.Binding.CapsuleID) {
		return fmt.Errorf("assignment capsule capability is absent: %w", err)
	}
	diagnostics, err := rt.capsuleExecutor.InspectCapsuleRaw(assignment.Binding.CapsuleID)
	if err != nil || diagnostics.ID != assignment.Binding.CapsuleID || diagnostics.State != capsule.StateActive {
		return fmt.Errorf("assignment capsule is not active: %w", err)
	}
	return nil
}
