package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
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
		return base, "", nil // legacy/unassigned CoSuper sees no capsule schemas
	}
	if assignmentID == "" || attempt == 0 || rt.capsuleExecutor == nil || base == nil {
		return nil, "", fmt.Errorf("assigned CoSuper tool overlay binding unavailable")
	}
	lookup := rt.assignmentLookup
	if lookup == nil {
		lookup = rt.store
	}
	assignment, err := lookup.GetCoSuperAssignment(ctx, rec.OwnerID, rec.SandboxID, assignmentID, attempt)
	if err != nil {
		return nil, "", err
	}
	if assignment.Disposition != types.CoSuperAssignmentBound || assignment.CapsuleDisposition != types.CoSuperCapsuleActive ||
		assignment.BoundRunID != rec.RunID || assignment.Binding.AssignedAgentID != rec.AgentID ||
		assignment.Binding.TrajectoryID != rec.TrajectoryID || assignment.Binding.ComputerID != rec.SandboxID ||
		metadataStringValue(rec.Metadata, "capsule_id") != assignment.Binding.CapsuleID ||
		metadataStringValue(rec.Metadata, "capability_digest") != assignment.Binding.CapabilityDigest ||
		metadataStringValue(rec.Metadata, "assigned_work_item_id") != assignment.Binding.AssignedWorkItemID ||
		metadataStringValue(rec.Metadata, "assignment_kind") != string(assignment.Binding.Kind) {
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
	registry, err := toolregistry.NewToolRegistryWithTools(base.Tools()...)
	if err != nil {
		return nil, "", err
	}
	if err := RegisterCapsuleLocalTools(registry, rt); err != nil {
		return nil, "", err
	}
	return registry, handle, nil
}
