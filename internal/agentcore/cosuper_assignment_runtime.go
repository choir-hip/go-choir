package agentcore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
	CandidateID      string
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
		"choir:co-super-assignment:v2", parent.RunID, strings.TrimSpace(req.ToolCallID),
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
	req.Objective, req.CandidateID = strings.TrimSpace(req.Objective), strings.TrimSpace(req.CandidateID)
	if req.Objective == "" || strings.TrimSpace(req.ParentWorkItemID) == "" || strings.TrimSpace(req.ToolCallID) == "" ||
		(req.Kind != types.CoSuperAssignmentImplementation && req.Kind != types.CoSuperAssignmentVerification) {
		return AssignedCoSuperStart{}, fmt.Errorf("assigned CoSuper requires objective, kind, parent work, and authenticated tool-call identity")
	}
	if (req.Kind == types.CoSuperAssignmentVerification) != (req.CandidateID != "") {
		return AssignedCoSuperStart{}, fmt.Errorf("verification requires one exact candidate_id and implementation forbids it")
	}
	ownerID, computerID := strings.TrimSpace(parent.OwnerID), strings.TrimSpace(parent.ComputerID)
	if ownerID == "" || computerID == "" || parent.AgentID != persistentSuperAgentID(ownerID) ||
		agentprofile.Canonical(parent.AgentProfile) != agentprofile.Super || agentprofile.Canonical(parent.AgentRole) != agentprofile.Super ||
		parent.TrajectoryID != "" || !persistentSuperRunStateAllowedRuntime(parent.State) {
		return AssignedCoSuperStart{}, fmt.Errorf("only the exact non-lifecycle persistent Super may open assigned CoSuper work")
	}
	trajectoryID := metadataStringValue(parent.Metadata, "assignment_trajectory_id")
	parentWorkID := strings.TrimSpace(req.ParentWorkItemID)
	if trajectoryID == "" {
		return AssignedCoSuperStart{}, fmt.Errorf("persistent Super run lacks exact lifecycle trajectory binding")
	}
	attempt := uint64(1) // one authenticated tool call is one runtime-derived attempt
	assignmentID := deterministicAssignmentIdentity(parent, req)
	requestDigest := objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:co-super-request:v1", req.Objective, string(req.Kind), req.CandidateID, parentWorkID,
	}, "\x00")))
	// Replay is resolved before reading mutable current source/work projections.
	// The authenticated provider call identity is the authority; changed semantic
	// arguments conflict under that same identity.
	if existing, getErr := rt.store.GetCoSuperAssignment(ctx, ownerID, computerID, assignmentID, attempt); getErr == nil {
		if existing.Binding.ParentRunID != parent.RunID || existing.Binding.ParentWorkItemID != parentWorkID ||
			existing.Binding.Kind != req.Kind || existing.Binding.RequestDigest != requestDigest ||
			existing.Binding.SourceCandidateID != req.CandidateID {
			return AssignedCoSuperStart{}, store.ErrCoSuperAssignmentCommandConflict
		}
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
	parentControlID := runtimePersistentSuperControlID(parent.Metadata, trajectoryID, parentWorkID)
	if parentControlID == "" {
		return AssignedCoSuperStart{}, fmt.Errorf("persistent Super run lacks exact lifecycle control binding for selected work")
	}
	parentDecisionID := "decision:" + objectgraph.SHA256([]byte(strings.Join([]string{
		"choir:co-super-decision:v2", ownerID, computerID, parent.RunID, trajectoryID, parentWorkID, parentControlID, strings.TrimSpace(req.ToolCallID),
	}, "\x00")))
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return AssignedCoSuperStart{}, fmt.Errorf("derive assignment scope: %w", err)
	}
	var deliveredControl *types.CoagentSourcePacket
	var parentWork *types.WorkItemRecord
	for i := range snapshot.Updates {
		if snapshot.Updates[i].UpdateID == parentControlID && snapshot.Updates[i].TargetWorkItemID == parentWorkID &&
			snapshot.Updates[i].DeliveredToRunID == parent.RunID && snapshot.Updates[i].DeliveredAt != nil {
			copy := snapshot.Updates[i]
			deliveredControl = &copy
			break
		}
	}
	for i := range snapshot.WorkItems {
		if snapshot.WorkItems[i].WorkItemID == parentWorkID && snapshot.WorkItems[i].AssignedAgentID == parent.AgentID {
			copy := snapshot.WorkItems[i]
			parentWork = &copy
			break
		}
	}
	if deliveredControl == nil || parentWork == nil {
		return AssignedCoSuperStart{}, fmt.Errorf("derive assignment scope: exact delivered control/work join unavailable")
	}
	scopeBytes, err := json.Marshal(struct {
		Control types.CoagentSourcePacket `json:"control"`
		Work    types.WorkItemRecord      `json:"work"`
	}{*deliveredControl, *parentWork})
	if err != nil {
		return AssignedCoSuperStart{}, err
	}
	scopeDigest := objectgraph.SHA256(scopeBytes)
	sourceArtifactRef := ""
	if req.Kind == types.CoSuperAssignmentVerification {
		candidate, candidateErr := rt.store.GetCoSuperSubjectCandidate(ctx, ownerID, computerID, req.CandidateID)
		if candidateErr != nil || candidate.TrajectoryID != trajectoryID || candidate.ArtifactRef == "" {
			return AssignedCoSuperStart{}, fmt.Errorf("verification candidate is unavailable or outside exact trajectory authority")
		}
		implementation, loadErr := rt.store.GetCoSuperAssignment(ctx, ownerID, computerID, candidate.AssignmentID, candidate.Attempt)
		if loadErr != nil || implementation.Binding.Kind != types.CoSuperAssignmentImplementation ||
			implementation.Binding.ParentAgentID != parent.AgentID || implementation.Binding.ParentWorkItemID != parentWorkID ||
			implementation.Binding.TrajectoryID != trajectoryID || implementation.Disposition != types.CoSuperAssignmentCompleted {
			return AssignedCoSuperStart{}, fmt.Errorf("verification candidate is not an exact completed implementation artifact")
		}
		sourceArtifactRef = candidate.ArtifactRef
	}
	preflight, err := rt.capsuleExecutor.PreflightSourceSnapshot(ctx, sourceArtifactRef)
	if err != nil {
		return AssignedCoSuperStart{}, fmt.Errorf("preflight immutable assignment subject: %w", err)
	}
	subjectDigest := "sha256:" + strings.TrimPrefix(preflight.SubjectDigest, "sha256:")
	if req.Kind == types.CoSuperAssignmentVerification {
		candidate, _ := rt.store.GetCoSuperSubjectCandidate(ctx, ownerID, computerID, req.CandidateID)
		if candidate.SubjectDigest != subjectDigest || candidate.ArtifactRef != preflight.ArtifactRef {
			return AssignedCoSuperStart{}, fmt.Errorf("verification candidate artifact digest mismatch")
		}
	}

	agentID := "co-super:" + assignmentID
	workID := "work:" + assignmentID
	runID := "run:" + assignmentID
	capsuleID := "capsule-" + strings.TrimPrefix(uuid.NewSHA1(uuid.NameSpaceOID, []byte(assignmentID+"\x00"+fmt.Sprint(attempt))).String(), "-")
	opaque, err := opaqueAssignmentCapability()
	if err != nil {
		return AssignedCoSuperStart{}, err
	}
	binding := types.CoSuperAssignmentBinding{
		OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID,
		ParentAgentID: parent.AgentID, ParentRunID: parent.RunID, ParentDecisionID: parentDecisionID,
		ParentControlID: parentControlID, ParentWorkItemID: parentWorkID,
		AssignedWorkItemID: workID, AssignedAgentID: agentID, Kind: req.Kind, Attempt: attempt,
		ScopeDigest: scopeDigest, RequestDigest: requestDigest, CapabilityDigest: store.DigestCoSuperOpaqueCapability(opaque),
		ExecutionHandleDigest: objectgraph.SHA256([]byte(opaque)), SubjectDigest: subjectDigest,
		SourceArtifactRef: preflight.ArtifactRef, SourceCandidateID: req.CandidateID,
		Writable: true, CapsuleID: capsuleID, NetworkMode: types.CoSuperCapsuleNetworkForbidden,
		FilesystemMode: types.CoSuperCapsuleFilesystemAssignmentLocalWritableOverlay,
	}
	open := types.OpenCoSuperAssignmentRequest{
		CommandID: "co-super-open:" + assignmentID + fmt.Sprintf(":%d", attempt), AssignmentID: assignmentID, Binding: binding,
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
		current, loadErr := rt.store.GetCoSuperAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
		if loadErr == nil && !current.Disposition.Terminal() {
			cancel := types.CancelCoSuperAssignmentRequest{CommandID: "co-super-open-failed:" + assignmentID, OwnerID: ownerID, ComputerID: computerID,
				AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: current.LifecycleVersion, Reason: cause.Error()}
			cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
			_, _ = rt.store.CancelCoSuperAssignment(context.Background(), cancel)
		}
		return cause
	}
	created, err := rt.capsuleExecutor.Spawn(ctx, capsule.SpawnSpec{CapsuleID: capsuleID, OwnerRunID: runID,
		MemoryMax: coSuperAssignmentMemoryMax, CpuQuota: coSuperAssignmentCPUQuota, CpuPeriod: 100000, PidsMax: coSuperAssignmentPidsMax,
		WorkingDir: "/workspace/platform", Tier: capsule.TierMedium,
		SourceArtifactRef: preflight.ArtifactRef, ExpectedSubjectDigest: preflight.SubjectDigest})
	if err != nil {
		return AssignedCoSuperStart{}, cancelOpen(fmt.Errorf("spawn assigned capsule after durable open: %w", err))
	}
	cleanupCapsule := func(cause error) error {
		current, loadErr := rt.store.GetCoSuperAssignment(context.Background(), ownerID, computerID, assignmentID, attempt)
		if loadErr != nil {
			return fmt.Errorf("%w (load opened assignment for capsule cleanup: %v)", cause, loadErr)
		}
		intent := "capsule-revoke-intent:" + objectgraph.SHA256([]byte(current.AssignmentID+"\x00pre-bind\x00"+cause.Error()))
		requested, fateErr := rt.store.SetCoSuperCapsuleDisposition(context.Background(), coSuperFateRequest(current, types.CoSuperCapsuleRevokeRequested, intent, ""))
		if fateErr != nil {
			return fmt.Errorf("%w (persist pre-bind capsule revoke intent: %v)", cause, fateErr)
		}
		_ = rt.capsuleExecutor.RevokeCapability(runID, opaque)
		if rt.capsuleExecutor.HasCapsule(capsuleID) {
			if destroyErr := rt.capsuleExecutor.ForceDestroy(context.Background(), capsuleID); destroyErr != nil {
				return fmt.Errorf("%w (destroy after pre-bind revoke intent: %v)", cause, destroyErr)
			}
		}
		if rt.capsuleExecutor.HasCapsule(capsuleID) {
			return fmt.Errorf("%w (pre-bind capsule continued after executor acknowledgement)", cause)
		}
		receipt, receiptErr := rt.capsuleExecutor.PersistRevocationReceipt(runID, requested.Assignment.Binding.CapabilityDigest, capsuleID, intent)
		if receiptErr != nil {
			return fmt.Errorf("%w (persist structured pre-bind revoke acknowledgement: %v)", cause, receiptErr)
		}
		fateAck, fateAckErr := coSuperFateAckRequest(requested.Assignment, types.CoSuperCapsuleRevoked, intent, receipt.ReceiptRef, "", "", receipt.OccurredAt, receipt.CapsuleAbsent)
		if fateAckErr != nil {
			return fmt.Errorf("%w (invalid revoke receipt occurred_at: %v)", cause, fateAckErr)
		}
		acked, fateErr := rt.store.SetCoSuperCapsuleDisposition(context.Background(), fateAck)
		if fateErr != nil {
			return fmt.Errorf("%w (persist pre-bind capsule revoke acknowledgement: %v)", cause, fateErr)
		}
		cancel := types.CancelCoSuperAssignmentRequest{CommandID: "co-super-open-failed:" + assignmentID, OwnerID: ownerID, ComputerID: computerID,
			AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: acked.Assignment.LifecycleVersion, Reason: cause.Error()}
		cancel.CommandDigest, _ = store.ComputeCancelCoSuperAssignmentDigest(cancel)
		if _, cancelErr := rt.store.CancelCoSuperAssignment(context.Background(), cancel); cancelErr != nil {
			return fmt.Errorf("%w (cancel pre-bind assignment after revoke ack: %v)", cause, cancelErr)
		}
		return cause
	}
	if created.ID != capsuleID || created.State != capsule.StateActive {
		return AssignedCoSuperStart{}, cleanupCapsule(fmt.Errorf("assigned capsule acknowledgement mismatch"))
	}
	spawnedAt := time.Now().UTC()
	capability, err := rt.capsuleExecutor.MintCapabilityHandle(runID, capsule.RoleCoSuper, capsuleID, opaque, 24*time.Hour)
	grantedAt := time.Now().UTC()
	if err != nil {
		return AssignedCoSuperStart{}, cleanupCapsule(fmt.Errorf("mint exact assignment capability: %w", err))
	}
	compiledVerbs := make([]string, 0, len(capsule.RoleVerbSets[capsule.RoleCoSuper]))
	for verb, allowed := range capsule.RoleVerbSets[capsule.RoleCoSuper] {
		if allowed {
			compiledVerbs = append(compiledVerbs, verb)
		}
	}
	slices.Sort(compiledVerbs)
	actualVerbs := make([]string, 0, len(capability.Verbs))
	for verb, allowed := range capability.Verbs {
		if !allowed {
			return AssignedCoSuperStart{}, cleanupCapsule(fmt.Errorf("minted assignment capability contains a disabled verb"))
		}
		actualVerbs = append(actualVerbs, verb)
	}
	slices.Sort(actualVerbs)
	if capability.AgentRole != capsule.RoleCoSuper || capability.AgentRunID != runID || capability.CapsuleID != capsuleID || capability.TargetCapsule != capsuleID ||
		capability.Handle != opaque || !slices.Equal(actualVerbs, compiledVerbs) || len(capability.ExternalAccess) != 0 || strings.TrimSpace(capability.KeyID) == "" ||
		len(capability.Signature) == 0 || !capability.ExpiresAt.After(grantedAt) || capability.ExpiresAt.After(grantedAt.Add(24*time.Hour+time.Second)) {
		return AssignedCoSuperStart{}, cleanupCapsule(fmt.Errorf("minted assignment capability acknowledgement mismatch"))
	}
	capabilityBytes, err := json.Marshal(capability)
	if err != nil {
		return AssignedCoSuperStart{}, cleanupCapsule(fmt.Errorf("digest minted assignment capability: %w", err))
	}
	grantAttestation := &types.CoSuperGrantPolicyAttestation{
		Role: string(capability.AgentRole), GrantedVerbs: actualVerbs,
		VerbSetDigest:          store.ComputeCoSuperGrantVerbSetDigest(actualVerbs),
		PolicyDigest:           store.ComputeCoSuperGrantPolicyDigest(string(capability.AgentRole), actualVerbs, binding.NetworkMode, binding.FilesystemMode, binding.Writable),
		SignedCapabilityDigest: objectgraph.SHA256(capabilityBytes), SpawnAcknowledged: true, ActiveAcknowledged: true, GrantAcknowledged: true,
		SpawnedAt: spawnedAt, GrantedAt: grantedAt,
	}
	run := types.RunRecord{
		RunID: runID, AgentID: agentID, ChannelID: agentID, RequestedByRunID: parent.RunID, TrajectoryID: trajectoryID,
		AgentProfile: agentprofile.CoSuper, AgentRole: agentprofile.CoSuper, OwnerID: ownerID, ComputerID: computerID,
		State: types.RunPending, Prompt: req.Objective,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.CoSuper, runMetadataAgentRole: agentprofile.CoSuper, runMetadataAgentID: agentID,
			runMetadataTrajectoryID: trajectoryID, "work_item_ids": []string{workID}, "lifecycle_work_item_id": workID,
			"requested_by_agent_id": parent.AgentID, "requested_by_profile": agentprofile.Super,
			"assignment_id": assignmentID, "assignment_attempt": attempt, "assignment_kind": string(req.Kind),
			"assigned_work_item_id": workID, "capsule_id": capsuleID,
			"parent_decision_id": parentDecisionID, "parent_control_id": parentControlID,
			"parent_work_item_id": parentWorkID, "scope_digest": scopeDigest, "request_digest": requestDigest,
			"capability_digest": binding.CapabilityDigest, "execution_handle_digest": binding.ExecutionHandleDigest, "subject_digest": subjectDigest,
			"source_artifact_ref": preflight.ArtifactRef, "source_candidate_id": req.CandidateID,
		},
	}
	bind := types.BindCoSuperAssignmentRequest{
		CommandID: "co-super-bind:" + assignmentID + fmt.Sprintf(":%d", attempt), OwnerID: ownerID, ComputerID: computerID,
		AssignmentID: assignmentID, Attempt: attempt, ExpectedLifecycleVersion: opened.Assignment.LifecycleVersion,
		RunID: runID, Run: run, OpaqueCapability: opaque, CapsuleID: capsuleID, GrantPolicyAttestation: grantAttestation,
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
