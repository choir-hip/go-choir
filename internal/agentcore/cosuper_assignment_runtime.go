package agentcore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	coSuperAssignmentMemoryMax = int64(1 << 30)
	coSuperAssignmentCPUQuota  = int64(100000)
	coSuperAssignmentPidsMax   = int64(256)
)

type assignmentCapsuleRuntime interface {
	Spawn(context.Context, capsule.SpawnSpec) (*capsule.Capsule, error)
	MintCapabilityHandle(string, capsule.AgentRole, string, string, time.Duration) (*capsule.Capability, error)
	RevokeCapability(string, string) error
	ForceDestroy(context.Context, string) error
	ExtractGranted(context.Context, string, string) ([]capsule.FileChange, error)
	ResolveGrantedWorktreeDigest(context.Context, string, string) (string, error)
	ResolveExecutionReceipts([]string) ([]capsule.ExecutionReceipt, error)
	AssignmentHandle(string, string) (string, error)
	InspectCapsuleRaw(string) (*capsule.CapsuleDiagnostics, error)
	HasCapsule(string) bool
}

type StartAssignedCoSuperRequest struct {
	Objective        string
	Kind             types.CoSuperAssignmentKind
	Attempt          uint64
	ScopeDigest      string
	SubjectDigest    string
	ParentWorkItemID string
	ToolCallID       string
}

type AssignedCoSuperStart struct {
	Assignment types.CoSuperAssignment
	Run        types.RunRecord
	Replay     bool
}

func deterministicAssignmentIdentity(parent types.RunRecord, req StartAssignedCoSuperRequest) string {
	seed := strings.Join([]string{
		"choir:co-super-assignment:v1", parent.OwnerID, parent.SandboxID,
		metadataStringValue(parent.Metadata, "assignment_trajectory_id"), parent.RunID,
		strings.TrimSpace(req.ParentWorkItemID), strings.TrimSpace(req.ToolCallID),
		fmt.Sprint(req.Attempt), string(req.Kind), strings.TrimSpace(req.ScopeDigest), strings.TrimSpace(req.SubjectDigest), strings.TrimSpace(req.Objective),
	}, "\x00")
	return "assignment-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(seed)).String()
}

func opaqueAssignmentCapability() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "assignment-cap-" + hex.EncodeToString(raw[:]), nil
}

func (rt *Runtime) startAssignedCoSuper(ctx context.Context, parentRunID, ownerID string, req StartAssignedCoSuperRequest) (AssignedCoSuperStart, error) {
	if rt == nil || rt.store == nil || rt.capsuleExecutor == nil {
		return AssignedCoSuperStart{}, fmt.Errorf("assigned CoSuper capsule authority unavailable")
	}
	parent, err := rt.getRunForComputer(ctx, ownerID, parentRunID)
	if err != nil {
		return AssignedCoSuperStart{}, err
	}
	return rt.startAssignedCoSuperForParent(ctx, parent, req)
}

func (rt *Runtime) startAssignedCoSuperForParent(ctx context.Context, parent types.RunRecord, req StartAssignedCoSuperRequest) (AssignedCoSuperStart, error) {
	req.Objective = strings.TrimSpace(req.Objective)
	if req.Objective == "" || req.Attempt == 0 || strings.TrimSpace(req.ParentWorkItemID) == "" || strings.TrimSpace(req.ToolCallID) == "" ||
		(req.Kind != types.CoSuperAssignmentImplementation && req.Kind != types.CoSuperAssignmentVerification) ||
		!types.ValidSHA256Digest(req.ScopeDigest) || !types.ValidSHA256Digest(req.SubjectDigest) {
		return AssignedCoSuperStart{}, fmt.Errorf("assigned CoSuper requires objective, kind, attempt, scope digest, and immutable subject digest")
	}
	ownerID, computerID := strings.TrimSpace(parent.OwnerID), strings.TrimSpace(parent.SandboxID)
	if ownerID == "" || computerID == "" || parent.AgentID != persistentSuperAgentID(ownerID) ||
		agentprofile.Canonical(parent.AgentProfile) != agentprofile.Super || agentprofile.Canonical(parent.AgentRole) != agentprofile.Super ||
		parent.TrajectoryID != "" || !persistentSuperRunStateAllowedRuntime(parent.State) {
		return AssignedCoSuperStart{}, fmt.Errorf("only the exact non-lifecycle persistent Super may open assigned CoSuper work")
	}
	trajectoryID := metadataStringValue(parent.Metadata, "assignment_trajectory_id")
	parentWorkID := strings.TrimSpace(req.ParentWorkItemID)
	parentControlID := runtimePersistentSuperControlID(parent.Metadata, trajectoryID, parentWorkID)
	if trajectoryID == "" || parentControlID == "" {
		return AssignedCoSuperStart{}, fmt.Errorf("persistent Super run lacks exact lifecycle control binding for selected work")
	}
	parentDecisionID := "decision:" + objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:co-super-decision:v1", ownerID, computerID, parent.RunID, trajectoryID, parentWorkID, parentControlID, strings.TrimSpace(req.ToolCallID),
	}, "\x00")))
	assignmentID := deterministicAssignmentIdentity(parent, req)
	if existing, getErr := rt.store.GetCoSuperAssignment(ctx, ownerID, computerID, assignmentID, req.Attempt); getErr == nil {
		if existing.Disposition == types.CoSuperAssignmentBound || existing.Disposition.Terminal() {
			run := types.RunRecord{}
			if existing.BoundRunID != "" {
				run, _ = rt.store.GetLifecycleRun(ctx, ownerID, computerID, existing.BoundRunID)
			}
			return AssignedCoSuperStart{Assignment: existing, Run: run, Replay: true}, nil
		}
		return AssignedCoSuperStart{}, fmt.Errorf("assigned CoSuper opener was durably committed but not bound; reconcile attempt before retry")
	} else if !errors.Is(getErr, store.ErrNotFound) {
		return AssignedCoSuperStart{}, getErr
	}

	agentID := "co-super:" + assignmentID
	workID := "work:" + assignmentID
	runID := "run:" + assignmentID
	capsuleID := "capsule-" + strings.TrimPrefix(uuid.NewSHA1(uuid.NameSpaceOID, []byte(assignmentID+"\x00"+fmt.Sprint(req.Attempt))).String(), "-")
	opaque, err := opaqueAssignmentCapability()
	if err != nil {
		return AssignedCoSuperStart{}, err
	}
	binding := types.CoSuperAssignmentBinding{
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ParentAgentID: parent.AgentID, ParentRunID: parent.RunID, ParentDecisionID: parentDecisionID,
		ParentControlID: parentControlID, ParentWorkItemID: parentWorkID,
		AssignedWorkItemID: workID, AssignedAgentID: agentID, Kind: req.Kind, Attempt: req.Attempt,
		ScopeDigest: req.ScopeDigest, CapabilityDigest: store.DigestCoSuperOpaqueCapability(opaque), SubjectDigest: req.SubjectDigest,
		Writable: true, CapsuleID: capsuleID, NetworkMode: types.CoSuperCapsuleNetworkForbidden,
		FilesystemMode: types.CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay,
	}
	open := types.OpenCoSuperAssignmentRequest{
		CommandID: "co-super-open:" + assignmentID + fmt.Sprintf(":%d", req.Attempt), AssignmentID: assignmentID, Binding: binding,
		AssignedAgent: types.AgentRecord{AgentID: agentID},
		AssignedWork:  types.WorkItemRecord{WorkItemID: workID, AssignedAgentID: agentID, Objective: req.Objective},
	}
	open.CommandDigest, err = store.ComputeOpenCoSuperAssignmentDigest(open)
	if err != nil {
		return AssignedCoSuperStart{}, err
	}
	opened, err := rt.store.OpenCoSuperAssignment(ctx, open)
	if err != nil {
		return AssignedCoSuperStart{}, err
	}
	cancelOpen := func(cause error) error {
		current, loadErr := rt.store.GetCoSuperAssignment(context.Background(), ownerID, computerID, assignmentID, req.Attempt)
		if loadErr == nil && !current.Disposition.Terminal() {
			cancel := types.CancelCoSuperAssignmentRequest{CommandID: "co-super-open-failed:" + assignmentID, OwnerID: ownerID, ComputerID: computerID,
				AssignmentID: assignmentID, Attempt: req.Attempt, ExpectedLifecycleVersion: current.LifecycleVersion, Reason: cause.Error()}
			cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
			_, _ = rt.store.CancelCoSuperAssignment(context.Background(), cancel)
		}
		return cause
	}
	created, err := rt.capsuleExecutor.Spawn(ctx, capsule.SpawnSpec{CapsuleID: capsuleID, OwnerRunID: runID,
		MemoryMax: coSuperAssignmentMemoryMax, CpuQuota: coSuperAssignmentCPUQuota, CpuPeriod: 100000, PidsMax: coSuperAssignmentPidsMax,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium})
	if err != nil {
		return AssignedCoSuperStart{}, cancelOpen(fmt.Errorf("spawn assigned capsule after durable open: %w", err))
	}
	cleanupCapsule := func(cause error) error {
		_ = rt.capsuleExecutor.RevokeCapability(runID, opaque)
		_ = rt.capsuleExecutor.ForceDestroy(context.Background(), capsuleID)
		return cancelOpen(cause)
	}
	if created.ID != capsuleID || created.State != capsule.StateActive {
		return AssignedCoSuperStart{}, cleanupCapsule(fmt.Errorf("assigned capsule acknowledgement mismatch"))
	}
	if _, err := rt.capsuleExecutor.MintCapabilityHandle(runID, capsule.RoleCoSuper, capsuleID, opaque, 24*time.Hour); err != nil {
		return AssignedCoSuperStart{}, cleanupCapsule(fmt.Errorf("mint exact assignment capability: %w", err))
	}
	run := types.RunRecord{
		RunID: runID, AgentID: agentID, ChannelID: agentID, RequestedByRunID: parent.RunID, TrajectoryID: trajectoryID,
		AgentProfile: agentprofile.CoSuper, AgentRole: agentprofile.CoSuper, OwnerID: ownerID, SandboxID: computerID,
		State: types.RunPending, Prompt: req.Objective,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.CoSuper, runMetadataAgentRole: agentprofile.CoSuper, runMetadataAgentID: agentID,
			runMetadataTrajectoryID: trajectoryID, "work_item_ids": []string{workID}, "lifecycle_work_item_id": workID,
			"requested_by_agent_id": parent.AgentID, "requested_by_profile": agentprofile.Super,
			"assignment_id": assignmentID, "assignment_attempt": req.Attempt, "assignment_kind": string(req.Kind),
			"assigned_work_item_id": workID, "capsule_id": capsuleID,
			"parent_decision_id": parentDecisionID, "parent_control_id": parentControlID,
			"parent_work_item_id": parentWorkID, "scope_digest": req.ScopeDigest,
			"capability_digest": binding.CapabilityDigest, "subject_digest": req.SubjectDigest,
		},
	}
	bind := types.BindCoSuperAssignmentRequest{
		CommandID: "co-super-bind:" + assignmentID + fmt.Sprintf(":%d", req.Attempt), OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: assignmentID, Attempt: req.Attempt, ExpectedLifecycleVersion: opened.Assignment.LifecycleVersion,
		RunID: runID, Run: run, OpaqueCapability: opaque, CapsuleID: capsuleID,
	}
	bind.CommandDigest, err = store.ComputeBindCoSuperAssignmentDigest(bind)
	if err != nil {
		return AssignedCoSuperStart{}, cleanupCapsule(err)
	}
	bound, err := rt.store.BindCoSuperAssignment(ctx, bind)
	if err != nil {
		return AssignedCoSuperStart{}, cleanupCapsule(fmt.Errorf("bind assigned CoSuper activation: %w", err))
	}
	// The lifecycle Bind receipt is durable before this actor wake. No generic
	// lifecycle-Super refusal is removed and no pre-cutover mailbox is written.
	rt.activate(&run)
	return AssignedCoSuperStart{Assignment: bound.Assignment, Run: run}, nil
}

func runtimePersistentSuperControlID(metadata map[string]any, trajectoryID, workItemID string) string {
	raw, ok := metadata["lifecycle_control_bindings"]
	if !ok {
		return ""
	}
	var entries []any
	switch value := raw.(type) {
	case []any:
		entries = value
	case []map[string]any:
		entries = make([]any, len(value))
		for i := range value {
			entries[i] = value[i]
		}
	default:
		return ""
	}
	matched := ""
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		entryTrajectory, _ := entry["trajectory_id"].(string)
		entryWork, _ := entry["target_work_item_id"].(string)
		entryUpdate, _ := entry["update_id"].(string)
		if strings.TrimSpace(entryTrajectory) == trajectoryID && strings.TrimSpace(entryWork) == workItemID && strings.TrimSpace(entryUpdate) != "" {
			if matched != "" {
				return ""
			}
			matched = strings.TrimSpace(entryUpdate)
		}
	}
	return matched
}

func persistentSuperRunStateAllowedRuntime(state types.RunState) bool {
	return state == types.RunPending || state == types.RunRunning || state == types.RunPassivated
}
