package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ComputeApplyTextureTurnWithSourceGraphDigest returns the canonical replay
// identity for a Texture turn. Inbound and control slices deliberately retain
// caller order; reordering either is a conflicting command.
func ComputeApplyTextureTurnWithSourceGraphDigest(req types.ApplyTextureTurnRequest, graph TextureSourceGraphWriteSet) (string, error) {
	normalized, err := normalizeTextureTurnDigestRequest(req)
	if err != nil {
		return "", err
	}
	return lifecycleDigest(struct {
		Request     types.ApplyTextureTurnRequest `json:"request"`
		SourceGraph TextureSourceGraphWriteSet    `json:"source_graph"`
	}{Request: normalized, SourceGraph: normalizeApplyLifecycleSourceGraphDigest(graph)})
}

func ComputeApplyTextureTurnDigest(req types.ApplyTextureTurnRequest) (string, error) {
	return ComputeApplyTextureTurnWithSourceGraphDigest(req, TextureSourceGraphWriteSet{})
}

func normalizeTextureTurnDigestRequest(req types.ApplyTextureTurnRequest) (types.ApplyTextureTurnRequest, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.CommandID = strings.TrimSpace(req.CommandID)
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	req.CallerAgentID = strings.TrimSpace(req.CallerAgentID)
	req.CallerRunID = strings.TrimSpace(req.CallerRunID)
	// Optimistic reducer versions are execution preconditions, not semantic
	// command identity. Exact retry after the first commit reconstructs current
	// versions and must still find the original receipt by stable digest.
	req.ExpectedLifecycleVersion, req.ExpectedCallerLifecycleVersion = 0, 0
	req.OwnerInstructions = append([]types.TextureTurnOwnerInstruction(nil), req.OwnerInstructions...)
	for i := range req.OwnerInstructions {
		req.OwnerInstructions[i].InstructionID = strings.TrimSpace(req.OwnerInstructions[i].InstructionID)
		req.OwnerInstructions[i].RequestID = strings.TrimSpace(req.OwnerInstructions[i].RequestID)
	}
	req.ExpectedHeadRevisionID = strings.TrimSpace(req.ExpectedHeadRevisionID)
	req.CallerWorkItemID = strings.TrimSpace(req.CallerWorkItemID)
	req.CallerWorkDisposition = types.WorkItemStatus(strings.TrimSpace(string(req.CallerWorkDisposition)))
	req.Outcome = types.TextureTurnOutcome(strings.TrimSpace(string(req.Outcome)))
	req.Reason = strings.TrimSpace(req.Reason)
	if req.SubjectRefs != nil {
		normalizedRefs := make(map[string]string, len(req.SubjectRefs))
		for key, value := range req.SubjectRefs {
			key, value = strings.TrimSpace(key), strings.TrimSpace(value)
			if key != "" && value != "" {
				normalizedRefs[key] = value
			}
		}
		req.SubjectRefs = normalizedRefs
	}
	if err := normalizeApplyLifecycleRevisionDigest(&req.Revision); err != nil {
		return req, err
	}
	req.Inbound = append([]types.TextureTurnInboundDisposition(nil), req.Inbound...)
	for i := range req.Inbound {
		in := &req.Inbound[i]
		in.TargetAgentID = strings.TrimSpace(in.TargetAgentID)
		in.ProducerAgentID = strings.TrimSpace(in.ProducerAgentID)
		in.ProducerUpdateID = strings.TrimSpace(in.ProducerUpdateID)
		in.UpdateID = strings.TrimSpace(in.UpdateID)
		in.ProducerWorkItemID = strings.TrimSpace(in.ProducerWorkItemID)
		in.WorkResultRef = strings.TrimSpace(in.WorkResultRef)
		in.Reason = strings.TrimSpace(in.Reason)
		var err error
		in.WorkDisposition, err = normalizeUpdateWorkDisposition(in.WorkDisposition)
		if err != nil {
			return req, err
		}
	}
	req.Controls = append([]types.TextureTurnControl(nil), req.Controls...)
	for i := range req.Controls {
		control := &req.Controls[i]
		control.ControlID = strings.TrimSpace(control.ControlID)
		control.TargetAgentID = strings.TrimSpace(control.TargetAgentID)
		control.TargetWorkItemID = strings.TrimSpace(control.TargetWorkItemID)
		control.PayloadDigest = strings.TrimSpace(control.PayloadDigest)
		control.Content = strings.TrimSpace(control.Content)
		control.Packet.SchemaVersion = strings.TrimSpace(control.Packet.SchemaVersion)
		control.Packet.Kind = strings.TrimSpace(control.Packet.Kind)
		control.Packet.Summary = strings.TrimSpace(control.Packet.Summary)
		if control.OpenAgent != nil {
			agent := *control.OpenAgent
			agent.AgentID = strings.TrimSpace(agent.AgentID)
			agent.OwnerID, agent.ComputerID, agent.SandboxID = "", "", ""
			agent.Profile, agent.Role, agent.ChannelID = strings.TrimSpace(agent.Profile), strings.TrimSpace(agent.Role), strings.TrimSpace(agent.ChannelID)
			agent.ActiveRunID = ""
			agent.LifecycleVersion, agent.LastReducerSeq = 0, 0
			agent.CreatedAt, agent.UpdatedAt = time.Time{}, time.Time{}
			control.OpenAgent = &agent
		}
		if control.OpenWork != nil {
			work := *control.OpenWork
			work.WorkItemID = strings.TrimSpace(work.WorkItemID)
			work.TrajectoryID, work.OwnerID, work.ComputerID = "", "", ""
			work.Objective = strings.TrimSpace(work.Objective)
			work.Reason = strings.TrimSpace(work.Reason)
			work.AuthorityProfile = strings.TrimSpace(work.AuthorityProfile)
			work.AssignedAgentID = strings.TrimSpace(work.AssignedAgentID)
			work.ObjectiveFingerprint, work.CreatedByRunID, work.ResultRef = "", "", ""
			work.Status, work.LifecycleVersion, work.LastReducerSeq = "", 0, 0
			work.CreatedAt, work.UpdatedAt = time.Time{}, time.Time{}
			if work.Details != nil {
				work.Details = cloneTextureTurnDetails(work.Details)
				delete(work.Details, "requested_by_profile")
				delete(work.Details, "requested_by_agent_id")
				delete(work.Details, "requested_by_run_id")
			}
			control.OpenWork = &work
		}
	}
	return req, nil
}

func cloneTextureTurnDetails(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func textureTurnRevisionEmpty(rev types.Revision) bool {
	zero := types.Revision{}
	left, _ := json.Marshal(rev)
	right, _ := json.Marshal(zero)
	return string(left) == string(right)
}

func validateTextureTurnShape(req types.ApplyTextureTurnRequest, graph TextureSourceGraphWriteSet) error {
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return err
	}
	if req.DocumentID == "" || req.CallerAgentID == "" || req.CallerRunID == "" || req.ExpectedHeadRevisionID == "" || req.CallerWorkItemID == "" ||
		req.ExpectedLifecycleVersion <= 0 || req.ExpectedCallerLifecycleVersion <= 0 {
		return fmt.Errorf("apply Texture turn: complete document, caller, run, caller work, lifecycle version, and head identity are required")
	}
	if req.CallerWorkDisposition != types.WorkItemOpen && req.CallerWorkDisposition != types.WorkItemCompleted {
		return fmt.Errorf("apply Texture turn: caller work disposition must be open or completed")
	}
	if req.CallerWorkDisposition == types.WorkItemCompleted && req.Outcome != types.TextureTurnRevision {
		return fmt.Errorf("apply Texture turn: completed caller work requires revision outcome")
	}
	if docRef, ok := req.SubjectRefs["doc_id"]; ok && docRef != req.DocumentID {
		return fmt.Errorf("apply Texture turn: subject_refs.doc_id cannot change document authority")
	}
	switch req.Outcome {
	case types.TextureTurnRevision:
		if strings.TrimSpace(req.Revision.RevisionID) == "" || req.Revision.AuthorKind != types.AuthorAppAgent || strings.TrimSpace(req.Revision.AuthorLabel) == "" {
			return fmt.Errorf("apply Texture turn: revision outcome requires a nonempty appagent-authored revision; caller authority proves Texture")
		}
		if value := strings.TrimSpace(req.Revision.DocID); value != "" && value != req.DocumentID {
			return fmt.Errorf("apply Texture turn: revision doc authority conflicts: %w", ErrLifecycleCommandConflict)
		}
		if value := strings.TrimSpace(req.Revision.ParentRevisionID); value != "" && value != req.ExpectedHeadRevisionID {
			return fmt.Errorf("apply Texture turn: revision parent authority conflicts: %w", ErrLifecycleCommandConflict)
		}
		if value := strings.TrimSpace(req.Revision.OwnerID); value != "" && value != req.OwnerID {
			return fmt.Errorf("apply Texture turn: revision owner authority conflicts: %w", ErrLifecycleCommandConflict)
		}
		if value := strings.TrimSpace(req.Revision.ComputerID); value != "" && value != req.ComputerID {
			return fmt.Errorf("apply Texture turn: revision computer authority conflicts: %w", ErrLifecycleCommandConflict)
		}
		if value := strings.TrimSpace(req.Revision.TrajectoryID); value != "" && value != req.TrajectoryID {
			return fmt.Errorf("apply Texture turn: revision trajectory authority conflicts: %w", ErrLifecycleCommandConflict)
		}
	case types.TextureTurnNoSemanticChange, types.TextureTurnWait, types.TextureTurnBlock:
		if !textureTurnRevisionEmpty(req.Revision) {
			return fmt.Errorf("apply Texture turn: non-revision outcome cannot carry revision")
		}
		if len(graph.SourceEntities) != 0 || len(graph.SourceRefs) != 0 {
			return fmt.Errorf("apply Texture turn: non-revision outcome cannot carry a source graph")
		}
		if req.Reason == "" {
			return fmt.Errorf("apply Texture turn: non-revision outcome requires reason")
		}
	default:
		return fmt.Errorf("apply Texture turn: exactly one supported outcome is required")
	}
	seenInbound := map[string]struct{}{}
	seenWork := map[string]struct{}{}
	for _, in := range req.Inbound {
		key := in.TargetAgentID + "\x00" + in.ProducerAgentID + "\x00" + in.ProducerUpdateID
		if in.TargetAgentID != req.CallerAgentID || in.ProducerAgentID == "" || in.ProducerUpdateID == "" || in.UpdateID == "" || in.ProducerWorkItemID == "" {
			return fmt.Errorf("apply Texture turn: inbound requires exact caller target, producer/update identity, and producer work")
		}
		if _, duplicate := seenInbound[key]; duplicate {
			return fmt.Errorf("apply Texture turn: duplicate inbound identity")
		}
		seenInbound[key] = struct{}{}
		if _, duplicate := seenWork[in.ProducerWorkItemID]; duplicate && in.WorkDisposition != types.WorkItemOpen {
			return fmt.Errorf("apply Texture turn: duplicate terminal producer work consequence")
		}
		if in.WorkDisposition != types.WorkItemOpen {
			seenWork[in.ProducerWorkItemID] = struct{}{}
		}
		switch in.Disposition {
		case types.UpdateIncorporated:
			if in.WorkDisposition != types.WorkItemOpen && in.WorkDisposition != types.WorkItemCompleted {
				return fmt.Errorf("apply Texture turn: incorporated inbound requires open or completed producer work")
			}
			if in.WorkDisposition == types.WorkItemCompleted && in.WorkResultRef == "" {
				return fmt.Errorf("apply Texture turn: completed producer work requires result ref")
			}
		case types.UpdateRejected:
			if in.Reason == "" || (in.WorkDisposition != types.WorkItemOpen && in.WorkDisposition != types.WorkItemRefused) {
				return fmt.Errorf("apply Texture turn: rejected inbound requires reason and open or refused producer work")
			}
		default:
			return fmt.Errorf("apply Texture turn: inbound must explicitly incorporate or reject")
		}
	}
	seenControl := map[string]struct{}{}
	persistentSuperOpenerCount := 0
	for _, control := range req.Controls {
		if _, terminalInSameTurn := seenWork[control.TargetWorkItemID]; terminalInSameTurn {
			return fmt.Errorf("apply Texture turn: control target work cannot be settled by the same turn")
		}
		if control.ControlID == "" || control.TargetAgentID == "" || control.TargetWorkItemID == "" || control.PayloadDigest == "" {
			return fmt.Errorf("apply Texture turn: control requires identity, target, target work, and payload digest")
		}
		if _, duplicate := seenControl[control.ControlID]; duplicate {
			return fmt.Errorf("apply Texture turn: duplicate control identity")
		}
		seenControl[control.ControlID] = struct{}{}
		if control.Packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 || control.Packet.Kind == "" {
			return fmt.Errorf("apply Texture turn: control requires a normalized v1 packet")
		}
		payloadDigest, err := ComputeLifecycleUpdatePayloadDigest(control.Packet, control.Content)
		if err != nil || payloadDigest != control.PayloadDigest {
			return fmt.Errorf("apply Texture turn: control payload digest mismatch: %w", ErrLifecycleCommandConflict)
		}
		if control.OpenAgent != nil && control.OpenWork == nil {
			return fmt.Errorf("apply Texture turn: agent opener requires its work in the same turn")
		}
		if control.OpenWork != nil {
			if control.OpenWork.WorkItemID != control.TargetWorkItemID || control.Packet.Kind == "" {
				return fmt.Errorf("apply Texture turn: opener requires exact work and first typed control packet")
			}
			if control.OpenAgent == nil {
				persistentSuperOpenerCount++
				if control.Packet.Kind != "execution_request" || len(control.Packet.Actions) == 0 {
					return fmt.Errorf("apply Texture turn: persistent-Super opener requires execution_request actions")
				}
			} else if control.OpenAgent.AgentID != control.TargetAgentID || control.OpenAgent.Profile != agentprofile.Researcher || control.OpenAgent.Role != agentprofile.Researcher || control.OpenAgent.ChannelID != req.DocumentID || control.OpenWork.AssignedAgentID != control.TargetAgentID || control.OpenWork.AuthorityProfile != agentprofile.Researcher {
				return fmt.Errorf("apply Texture turn: Researcher opener requires exact runtime-derived agent and work binding")
			}
		}
	}
	if persistentSuperOpenerCount > 1 {
		return fmt.Errorf("apply Texture turn: at most one persistent-Super opener is allowed")
	}
	return nil
}

// ApplyTextureTurn is the empty-source-graph view of the same atomic command.
func (s *Store) ApplyTextureTurn(ctx context.Context, req types.ApplyTextureTurnRequest) (types.LifecycleResult, error) {
	return s.ApplyTextureTurnWithSourceGraph(ctx, req, TextureSourceGraphWriteSet{})
}

// ApplyTextureTurnWithSourceGraph atomically commits the exact lifecycle
// Texture caller's one semantic outcome, inbound dispositions, target controls,
// source graph, events, and receipt. This store method never wakes an actor;
// wake/reconstruction is a post-success caller contract.
func (s *Store) ApplyTextureTurnWithSourceGraph(ctx context.Context, req types.ApplyTextureTurnRequest, sourceGraph TextureSourceGraphWriteSet) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	suppliedRevision := req.Revision
	suppliedExpectedLifecycleVersion := req.ExpectedLifecycleVersion
	suppliedExpectedCallerLifecycleVersion := req.ExpectedCallerLifecycleVersion
	normalized, err := normalizeTextureTurnDigestRequest(req)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	// Restore runtime scope, authority-shaped revision fields, and the supplied
	// digest after canonical replay normalization. The reducer validates rather
	// than silently overwriting any conflicting revision authority.
	normalized.OwnerID, normalized.ComputerID = ownerID, computerID
	normalized.CommandDigest = strings.TrimSpace(req.CommandDigest)
	normalized.Revision = suppliedRevision
	normalized.ExpectedLifecycleVersion = suppliedExpectedLifecycleVersion
	normalized.ExpectedCallerLifecycleVersion = suppliedExpectedCallerLifecycleVersion
	req = normalized
	if err := validateTextureTurnShape(req, sourceGraph); err != nil {
		return types.LifecycleResult{}, err
	}
	computed, digestErr := ComputeApplyTextureTurnWithSourceGraphDigest(req, sourceGraph)
	if err := requireLifecycleDigest(req.CommandDigest, computed, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}

	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	documentObj, document, err := s.textureTurnDocumentObject(ctx, ownerID, computerID, req.DocumentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	headObj, head, err := s.textureTurnRevisionObject(ctx, ownerID, computerID, req.ExpectedHeadRevisionID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	callerAgentObj, callerAgent, err := s.textureTurnAgentObject(ctx, ownerID, computerID, req.CallerAgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	callerRunObj, callerRun, err := s.textureTurnRunObject(ctx, ownerID, computerID, req.CallerRunID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	callerWorkObj, callerWork, err := s.lifecycleWorkObject(ctx, ownerID, computerID, req.CallerWorkItemID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive || trajectory.OwnerID != ownerID || trajectory.ComputerID != computerID ||
		trajectory.TrajectoryID != req.TrajectoryID || trajectory.LifecycleVersion != req.ExpectedLifecycleVersion ||
		strings.TrimSpace(trajectory.SubjectRefs["doc_id"]) != req.DocumentID || document.DocID != req.DocumentID ||
		document.OwnerID != ownerID || document.ComputerID != computerID || document.TrajectoryID != req.TrajectoryID ||
		document.ArchivedAt != nil || document.CurrentRevisionID != req.ExpectedHeadRevisionID || head.RevisionID != req.ExpectedHeadRevisionID ||
		head.DocID != req.DocumentID || head.OwnerID != ownerID || head.ComputerID != computerID || callerAgent.AgentID != req.CallerAgentID ||
		callerAgent.OwnerID != ownerID || callerAgent.ComputerID != computerID || callerAgent.Profile != agentprofile.Texture ||
		callerAgent.Role != agentprofile.Texture || callerAgent.ChannelID != req.DocumentID || callerAgent.ActiveRunID != req.CallerRunID ||
		callerAgent.LifecycleVersion != req.ExpectedCallerLifecycleVersion || callerRun.RunID != req.CallerRunID ||
		callerRun.AgentID != req.CallerAgentID || callerRun.OwnerID != ownerID || callerRun.SandboxID != computerID ||
		callerRun.TrajectoryID != req.TrajectoryID || callerRun.AgentProfile != agentprofile.Texture || callerRun.AgentRole != agentprofile.Texture ||
		callerRun.ChannelID != req.DocumentID || !callerRun.State.Active() || callerWork.WorkItemID != req.CallerWorkItemID ||
		callerWork.OwnerID != ownerID || callerWork.ComputerID != computerID || callerWork.TrajectoryID != req.TrajectoryID ||
		callerWork.Status != types.WorkItemOpen || strings.TrimSpace(callerWork.AssignedAgentID) != req.CallerAgentID ||
		callerWork.AuthorityProfile != agentprofile.Texture {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	type ownerInstructionBinding struct {
		object     objectgraph.Object
		workObject objectgraph.Object
		record     types.LifecycleOwnerInstruction
	}
	ownerInstructions := make([]ownerInstructionBinding, 0, len(req.OwnerInstructions))
	causalRequestIDs := make([]string, 0, len(req.OwnerInstructions))
	ownerInstructionIDs := make([]string, 0, len(req.OwnerInstructions))
	seenInstructions := make(map[string]bool, len(req.OwnerInstructions))
	for _, binding := range req.OwnerInstructions {
		if binding.InstructionID == "" || binding.RequestID == "" || seenInstructions[binding.InstructionID] {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		seenInstructions[binding.InstructionID] = true
		instructionObj, instruction, instructionErr := s.lifecycleOwnerInstructionObject(ctx, ownerID, computerID, req.TrajectoryID, binding.InstructionID)
		if instructionErr != nil {
			return types.LifecycleResult{}, instructionErr
		}
		instructionWorkObj, instructionWork, workErr := s.lifecycleWorkObject(ctx, ownerID, computerID, instruction.TargetWorkItemID)
		if workErr != nil {
			return types.LifecycleResult{}, workErr
		}
		if instruction.Status != types.LifecycleOwnerInstructionPending || instruction.RequestID != binding.RequestID ||
			instruction.DocumentID != req.DocumentID || instruction.TrajectoryID != req.TrajectoryID ||
			instruction.TargetAgentID != req.CallerAgentID || instruction.HeadRevisionID != req.ExpectedHeadRevisionID ||
			instructionWork.OwnerID != ownerID || instructionWork.ComputerID != computerID ||
			instructionWork.Status != types.WorkItemOpen || instructionWork.TrajectoryID != req.TrajectoryID ||
			instructionWork.AssignedAgentID != req.CallerAgentID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		ownerInstructions = append(ownerInstructions, ownerInstructionBinding{object: instructionObj, workObject: instructionWorkObj, record: instruction})
		ownerInstructionIDs = append(ownerInstructionIDs, instruction.InstructionID)
		causalRequestIDs = append(causalRequestIDs, instruction.RequestID)
	}
	// Every outcome consumes one exact ordered same-head owner occurrence set.
	// Checking only revision outcomes lets a late tell race a wait/block/no-change
	// turn and strand that occurrence behind an already committed activation.
	// Read the complete object set while trajectoryMu is held rather than using
	// the externally bounded list API: completeness is an atomic precondition.
	pendingInstructionObjects, pendingErr := s.lifecycleTransitionObjects(ctx, ogKindOwnerInstruction, req.TrajectoryID, ownerID, computerID)
	if pendingErr != nil {
		return types.LifecycleResult{}, pendingErr
	}
	expected := make([]types.LifecycleOwnerInstruction, 0, len(pendingInstructionObjects))
	for _, obj := range pendingInstructionObjects {
		instruction, decodeErr := decodeLifecycleObject[types.LifecycleOwnerInstruction](obj)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if instruction.Status == types.LifecycleOwnerInstructionPending &&
			instruction.TargetAgentID == req.CallerAgentID &&
			instruction.HeadRevisionID == req.ExpectedHeadRevisionID {
			expected = append(expected, instruction)
		}
	}
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].ReducerSeq == expected[j].ReducerSeq {
			return expected[i].InstructionID < expected[j].InstructionID
		}
		return expected[i].ReducerSeq < expected[j].ReducerSeq
	})
	if len(expected) != len(req.OwnerInstructions) {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	for i := range expected {
		if expected[i].InstructionID != req.OwnerInstructions[i].InstructionID || expected[i].RequestID != req.OwnerInstructions[i].RequestID {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
	}
	now := time.Now().UTC()
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: headObj.CanonicalID, Exists: true, ExpectedContentHash: headObj.ContentHash},
		{CanonicalID: callerAgentObj.CanonicalID, Exists: true, ExpectedContentHash: callerAgentObj.ContentHash},
		{CanonicalID: callerRunObj.CanonicalID, Exists: true, ExpectedContentHash: callerRunObj.ContentHash},
		{CanonicalID: callerWorkObj.CanonicalID, Exists: true, ExpectedContentHash: callerWorkObj.ContentHash},
	}
	seenOwnerConditions := make(map[string]bool, len(conditions)+len(ownerInstructions)*2)
	for _, condition := range conditions {
		seenOwnerConditions[condition.CanonicalID] = true
	}
	for _, binding := range ownerInstructions {
		for _, condition := range []objectgraph.ObjectCondition{
			{CanonicalID: binding.object.CanonicalID, Exists: true, ExpectedContentHash: binding.object.ContentHash},
			{CanonicalID: binding.workObject.CanonicalID, Exists: true, ExpectedContentHash: binding.workObject.ContentHash},
		} {
			if !seenOwnerConditions[condition.CanonicalID] {
				conditions = append(conditions, condition)
				seenOwnerConditions[condition.CanonicalID] = true
			}
		}
	}
	objects := make([]objectgraph.Object, 0)
	edges := make([]objectgraph.Edge, 0)
	events := make([]types.LifecycleEvent, 0, 1+len(req.Inbound)*2+len(req.Controls)*2)
	seq := trajectory.ReducerSeq
	appendEvent := func(kind types.LifecycleEventKind, workItemID, updateID string, refs []string, reason string) error {
		seq++
		event := types.LifecycleEvent{
			EventID: req.CommandID + ":" + fmt.Sprintf("%d", len(events)+1), OwnerID: ownerID, ComputerID: computerID,
			TrajectoryID: req.TrajectoryID, WorkItemID: workItemID, UpdateID: updateID, Kind: kind,
			ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: seq, CommandID: req.CommandID,
			CommandDigest: req.CommandDigest, RequestIDs: append([]string(nil), causalRequestIDs...), ArtifactRefs: refs, Reason: reason, CreatedAt: now,
		}
		if len(causalRequestIDs) == 1 {
			event.RequestID = causalRequestIDs[0]
		}
		events = append(events, event)
		return nil
	}

	var resultRevision *types.Revision
	resultDocument := document
	artifactRefs := []string{req.DocumentID, req.ExpectedHeadRevisionID}
	if req.Outcome == types.TextureTurnRevision {
		revision := req.Revision
		revision.RevisionID = strings.TrimSpace(revision.RevisionID)
		revision.OwnerID, revision.ComputerID, revision.TrajectoryID = ownerID, computerID, req.TrajectoryID
		revision.DocID, revision.ParentRevisionID, revision.CreatedAt = req.DocumentID, req.ExpectedHeadRevisionID, now
		revision, _, _, err = prepareTextureRevisionV2(revision)
		if err != nil {
			return types.LifecycleResult{}, fmt.Errorf("apply Texture turn: prepare revision: %w", err)
		}
		resultDocument, revision, err = commitTextureHeadAuthority(document, &head, revision, now)
		if errors.Is(err, ErrStaleDocumentHead) {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		if err != nil {
			return types.LifecycleResult{}, err
		}
		revisionMeta := lifecycleMetadata("revision_id", revision.RevisionID, computerID, req.TrajectoryID, seq+1)
		revisionMeta["doc_id"], revisionMeta["revision_hash"] = req.DocumentID, revision.RevisionHash
		revisionObj, buildErr := lifecycleObject(ogKindTexRev, ownerID, computerID, revision.RevisionID, revision, revisionMeta, now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		documentMeta := lifecycleMetadata("doc_id", req.DocumentID, computerID, req.TrajectoryID, seq+1)
		documentMeta["current_revision_id"] = revision.RevisionID
		documentUpdated, buildErr := lifecycleObject(ogKindTexDoc, ownerID, computerID, req.DocumentID, resultDocument, documentMeta, documentObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: revisionObj.CanonicalID})
		objects = append(objects, documentUpdated, revisionObj)
		edgeMetadata := json.RawMessage(`{}`)
		documentEdgeID, buildErr := objectgraph.BuildEdgeID(revisionObj.CanonicalID, documentUpdated.CanonicalID, ogEdgeDocRevision, edgeMetadata)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		parentEdgeID, buildErr := objectgraph.BuildEdgeID(revisionObj.CanonicalID, headObj.CanonicalID, ogEdgeRevParent, edgeMetadata)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		edges = append(edges,
			objectgraph.Edge{EdgeID: documentEdgeID, FromID: revisionObj.CanonicalID, ToID: documentUpdated.CanonicalID, Kind: ogEdgeDocRevision, Metadata: edgeMetadata, CreatedAt: now},
			objectgraph.Edge{EdgeID: parentEdgeID, FromID: revisionObj.CanonicalID, ToID: headObj.CanonicalID, Kind: ogEdgeRevParent, Metadata: edgeMetadata, CreatedAt: now},
		)
		if manifestErr := validateTextureTurnSourceManifest(revision, sourceGraph, req.CallerRunID); manifestErr != nil {
			return types.LifecycleResult{}, fmt.Errorf("apply Texture turn source manifest: %w", manifestErr)
		}
		sourceObjects, sourceConditions, sourceErr := s.lifecycleSourceGraphBatch(ctx, revision, sourceGraph, now)
		if sourceErr != nil {
			return types.LifecycleResult{}, fmt.Errorf("apply Texture turn source graph: %w", sourceErr)
		}
		objects = append(objects, sourceObjects...)
		conditions = append(conditions, sourceConditions...)
		resultRevision = &revision
		artifactRefs = []string{req.DocumentID, revision.RevisionID}
	}
	if err := appendEvent(types.LifecycleTextureTurnCommitted, "", "", artifactRefs, req.Reason); err != nil {
		return types.LifecycleResult{}, err
	}
	for i := range ownerInstructions {
		binding := &ownerInstructions[i]
		binding.record.Status = types.LifecycleOwnerInstructionConsumed
		binding.record.LifecycleVersion++
		binding.record.ReducerSeq = seq
		binding.record.ConsumedAt = &now
		key := binding.record.TrajectoryID + "\x00" + binding.record.InstructionID
		updated, buildErr := lifecycleObject(ogKindOwnerInstruction, ownerID, computerID, key, binding.record,
			lifecycleMetadata("instruction_id", binding.record.InstructionID, computerID, req.TrajectoryID, seq), binding.object.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		objects = append(objects, updated)
	}

	// The caller Texture assignment is a separate obligation, never a synthetic
	// self-addressed update. Its exact open record participates in every turn CAS;
	// completed is permitted only with a committed revision result.
	if req.CallerWorkDisposition == types.WorkItemCompleted {
		seq++
		callerWork.Status = types.WorkItemCompleted
		callerWork.ResultRef = resultDocument.CurrentRevisionID
		callerWork.LifecycleVersion++
		callerWork.LastReducerSeq, callerWork.UpdatedAt = seq, now
		callerWorkUpdated, buildErr := lifecycleObject(ogKindWorkItem, ownerID, computerID, callerWork.WorkItemID, callerWork,
			lifecycleMetadata("work_item_id", callerWork.WorkItemID, computerID, req.TrajectoryID, seq), callerWorkObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		objects = append(objects, callerWorkUpdated)
		if err := appendEventAtCurrent(&events, req, ownerID, computerID, seq, types.LifecycleWorkSettled, callerWork.WorkItemID, "", []string{callerWork.ResultRef}, req.Reason, now); err != nil {
			return types.LifecycleResult{}, err
		}
	}

	// Every inbound disposition points at the canonical head resulting from this
	// turn. Non-revision outcomes reuse the existing head; they never create a
	// synthetic revision merely to obtain a disposition reference.
	outcomeRef := resultDocument.CurrentRevisionID
	inboundIDs := make([]string, 0, len(req.Inbound))
	seenConditions := map[string]struct{}{}
	for _, condition := range conditions {
		seenConditions[condition.CanonicalID] = struct{}{}
	}
	addCondition := func(condition objectgraph.ObjectCondition) {
		if _, exists := seenConditions[condition.CanonicalID]; exists {
			return
		}
		seenConditions[condition.CanonicalID] = struct{}{}
		conditions = append(conditions, condition)
	}
	for _, disposition := range req.Inbound {
		updateKey := req.TrajectoryID + "\x00" + disposition.TargetAgentID + "\x00" + disposition.ProducerAgentID + "\x00" + disposition.ProducerUpdateID
		updateObj, update, getErr := s.textureTurnUpdateObject(ctx, ownerID, computerID, updateKey)
		if getErr != nil {
			return types.LifecycleResult{}, getErr
		}
		producerWorkID, targetWorkID, bindingErr := ResolveLifecyclePacketWorkBindings(update)
		if bindingErr != nil || update.Direction != types.LifecyclePacketDirectionProducerReport ||
			(targetWorkID != "" && targetWorkID != req.CallerWorkItemID) ||
			producerWorkID != disposition.ProducerWorkItemID || update.UpdateID != disposition.UpdateID ||
			update.ProducerUpdateID != disposition.ProducerUpdateID || update.AgentID != disposition.ProducerAgentID ||
			update.TargetAgentID != req.CallerAgentID || update.TrajectoryID != req.TrajectoryID || update.Disposition != types.UpdatePending {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		seq++
		update.Disposition, update.DispositionRef, update.DispositionReason = disposition.Disposition, outcomeRef, disposition.Reason
		update.LifecycleVersion++
		update.ReducerSeq = seq
		updateMeta := lifecycleMetadata("update_id", update.UpdateID, computerID, req.TrajectoryID, seq)
		updateMeta["producer_update_id"], updateMeta["target_agent_id"] = update.ProducerUpdateID, update.TargetAgentID
		updatedObj, buildErr := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, updateKey, update, updateMeta, updateObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		addCondition(objectgraph.ObjectCondition{CanonicalID: updateObj.CanonicalID, Exists: true, ExpectedContentHash: updateObj.ContentHash})
		objects = append(objects, updatedObj)
		inboundIDs = append(inboundIDs, update.UpdateID)
		eventKind := types.LifecycleUpdateApplied
		if disposition.Disposition == types.UpdateRejected {
			eventKind = types.LifecycleUpdateRejected
		}
		if err := appendEventAtCurrent(&events, req, ownerID, computerID, seq, eventKind, disposition.ProducerWorkItemID, update.UpdateID, artifactRefs, disposition.Reason, now); err != nil {
			return types.LifecycleResult{}, err
		}

		workObj, work, workErr := s.lifecycleWorkObject(ctx, ownerID, computerID, disposition.ProducerWorkItemID)
		if workErr != nil {
			return types.LifecycleResult{}, workErr
		}
		if work.TrajectoryID != req.TrajectoryID || work.OwnerID != ownerID || work.ComputerID != computerID ||
			work.Status != types.WorkItemOpen || strings.TrimSpace(work.AssignedAgentID) != disposition.ProducerAgentID {
			return types.LifecycleResult{}, ErrLifecycleInvalidTransition
		}
		addCondition(objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash})
		if disposition.WorkDisposition == types.WorkItemOpen {
			continue
		}
		seq++
		work.Status = disposition.WorkDisposition
		work.ResultRef, work.Reason = disposition.WorkResultRef, disposition.Reason
		if disposition.WorkDisposition == types.WorkItemRefused {
			work.ResultRef = outcomeRef
		}
		work.LifecycleVersion++
		work.LastReducerSeq, work.UpdatedAt = seq, now
		workUpdated, buildErr := lifecycleObject(ogKindWorkItem, ownerID, computerID, work.WorkItemID, work,
			lifecycleMetadata("work_item_id", work.WorkItemID, computerID, req.TrajectoryID, seq), workObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		objects = append(objects, workUpdated)
		workEventKind := types.LifecycleWorkSettled
		if work.Status == types.WorkItemRefused {
			workEventKind = types.LifecycleWorkRefused
		}
		if err := appendEventAtCurrent(&events, req, ownerID, computerID, seq, workEventKind, work.WorkItemID, update.UpdateID, []string{work.ResultRef}, work.Reason, now); err != nil {
			return types.LifecycleResult{}, err
		}
	}

	controlPackets := make([]types.CoagentSourcePacket, 0, len(req.Controls))
	targetWorkItems := make([]types.WorkItemRecord, 0, len(req.Controls))
	controlIDs := make([]string, 0, len(req.Controls))
	targetWorkIDs := make([]string, 0, len(req.Controls))
	for _, control := range req.Controls {
		var binding LifecycleTextureControlTargetBinding
		var targetAgentObj objectgraph.Object
		var targetAgent types.AgentRecord
		if control.OpenAgent != nil {
			targetAgent = *control.OpenAgent
			targetAgent.OwnerID, targetAgent.ComputerID, targetAgent.SandboxID = ownerID, computerID, computerID
			targetAgent.LifecycleVersion, targetAgent.LastReducerSeq = 1, seq+1
			targetAgent.CreatedAt, targetAgent.UpdatedAt = now, now
			if targetAgent.AgentID != control.TargetAgentID || targetAgent.Profile != agentprofile.Researcher || targetAgent.Role != agentprofile.Researcher || targetAgent.ChannelID != req.DocumentID || targetAgent.ActiveRunID != "" {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
			targetCanonicalID, buildErr := lifecycleCanonicalID(ogKindAgent, ownerID, computerID, targetAgent.AgentID)
			if buildErr != nil {
				return types.LifecycleResult{}, buildErr
			}
			if _, getErr := s.lifecycleGraph().GetObject(ctx, targetCanonicalID); getErr == nil {
				return types.LifecycleResult{}, ErrLifecycleCommandConflict
			} else if !errors.Is(getErr, objectgraph.ErrNotFound) {
				return types.LifecycleResult{}, getErr
			}
			targetAgentMeta := lifecycleMetadata("agent_id", targetAgent.AgentID, computerID, req.TrajectoryID, seq+1)
			targetAgentMeta["channel_id"] = targetAgent.ChannelID
			targetAgentObj, buildErr = lifecycleObject(ogKindAgent, ownerID, computerID, targetAgent.AgentID, targetAgent,
				targetAgentMeta, now, now)
			if buildErr != nil {
				return types.LifecycleResult{}, buildErr
			}
			addCondition(objectgraph.ObjectCondition{CanonicalID: targetAgentObj.CanonicalID})
			objects = append(objects, targetAgentObj)
			binding = LifecycleTextureControlTargetBinding{TargetAgent: targetAgent, TargetProfile: agentprofile.Researcher}
		} else {
			validatorWorkID := control.TargetWorkItemID
			if control.OpenWork != nil {
				validatorWorkID = ""
			}
			var validateErr error
			binding, validateErr = s.ValidateLifecycleTextureControlTarget(ctx, LifecycleTextureControlTargetRequest{
				OwnerID: ownerID, ComputerID: computerID, DocumentID: req.DocumentID, TrajectoryID: req.TrajectoryID,
				CallerAgentID: req.CallerAgentID, CallerRunID: req.CallerRunID, TargetAgentID: control.TargetAgentID,
				TargetWorkItemID: validatorWorkID,
			})
			if validateErr != nil {
				return types.LifecycleResult{}, validateErr
			}
			var targetErr error
			targetAgentObj, targetAgent, targetErr = s.textureTurnAgentObject(ctx, ownerID, computerID, control.TargetAgentID)
			if targetErr != nil || !reflect.DeepEqual(targetAgent, binding.TargetAgent) {
				if targetErr != nil {
					return types.LifecycleResult{}, targetErr
				}
				return types.LifecycleResult{}, ErrConcurrentStateChange
			}
			addCondition(objectgraph.ObjectCondition{CanonicalID: targetAgentObj.CanonicalID, Exists: true, ExpectedContentHash: targetAgentObj.ContentHash})
		}
		if binding.TargetRun != nil {
			targetRunObj, targetRun, runErr := s.textureTurnRunObject(ctx, ownerID, computerID, binding.TargetRun.RunID)
			if runErr != nil || !reflect.DeepEqual(targetRun, *binding.TargetRun) {
				if runErr != nil {
					return types.LifecycleResult{}, runErr
				}
				return types.LifecycleResult{}, ErrConcurrentStateChange
			}
			addCondition(objectgraph.ObjectCondition{CanonicalID: targetRunObj.CanonicalID, Exists: true, ExpectedContentHash: targetRunObj.ContentHash})
		}
		var workObj objectgraph.Object
		var work types.WorkItemRecord
		if control.OpenWork == nil {
			if binding.TargetWorkItem == nil {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
			work = *binding.TargetWorkItem
			var rawWork types.WorkItemRecord
			workObj, rawWork, err = s.lifecycleWorkObject(ctx, ownerID, computerID, work.WorkItemID)
			if err != nil {
				return types.LifecycleResult{}, err
			}
			if !reflect.DeepEqual(rawWork, work) {
				return types.LifecycleResult{}, ErrConcurrentStateChange
			}
			addCondition(objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash})
		} else {
			openerProfile := binding.TargetProfile
			switch openerProfile {
			case agentprofile.Super:
				if control.OpenAgent != nil || binding.TargetAgent.AgentID != agentprofile.Super+":"+ownerID {
					return types.LifecycleResult{}, ErrLifecycleInvalidTransition
				}
			case agentprofile.Researcher:
				if control.OpenAgent == nil || binding.TargetAgent.AgentID != control.TargetAgentID {
					return types.LifecycleResult{}, ErrLifecycleInvalidTransition
				}
			default:
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
			work, err = normalizeLifecycleWork(*control.OpenWork, ownerID, computerID, req.TrajectoryID, now)
			if err != nil {
				return types.LifecycleResult{}, err
			}
			if work.AssignedAgentID != control.TargetAgentID || work.AuthorityProfile != openerProfile || work.WorkItemID != control.TargetWorkItemID {
				return types.LifecycleResult{}, ErrLifecycleInvalidTransition
			}
			work.CreatedByRunID = req.CallerRunID
			work.Details = cloneTextureTurnDetails(work.Details)
			if work.Details == nil {
				work.Details = map[string]any{}
			}
			work.Details["requested_by_profile"] = agentprofile.Texture
			work.Details["requested_by_agent_id"] = req.CallerAgentID
			work.Details["requested_by_run_id"] = req.CallerRunID
			workCanonicalID, buildErr := lifecycleCanonicalID(ogKindWorkItem, ownerID, computerID, work.WorkItemID)
			if buildErr != nil {
				return types.LifecycleResult{}, buildErr
			}
			existingObj, getErr := s.lifecycleGraph().GetObject(ctx, workCanonicalID)
			switch {
			case getErr == nil:
				existing, decodeErr := decodeLifecycleObject[types.WorkItemRecord](existingObj)
				if decodeErr != nil {
					return types.LifecycleResult{}, decodeErr
				}
				validated, validateErr := s.ValidateLifecycleTextureControlTarget(ctx, LifecycleTextureControlTargetRequest{
					OwnerID: ownerID, ComputerID: computerID, DocumentID: req.DocumentID, TrajectoryID: req.TrajectoryID,
					CallerAgentID: req.CallerAgentID, CallerRunID: req.CallerRunID, TargetAgentID: control.TargetAgentID,
					TargetWorkItemID: control.TargetWorkItemID,
				})
				if validateErr != nil || validated.TargetWorkItem == nil || !textureTurnWorkEquivalent(existing, work) {
					return types.LifecycleResult{}, ErrLifecycleCommandConflict
				}
				work, workObj = existing, existingObj
				addCondition(objectgraph.ObjectCondition{CanonicalID: existingObj.CanonicalID, Exists: true, ExpectedContentHash: existingObj.ContentHash})
			case errors.Is(getErr, objectgraph.ErrNotFound):
				seq++
				work.LifecycleVersion, work.LastReducerSeq = 1, seq
				workObj, buildErr = lifecycleObject(ogKindWorkItem, ownerID, computerID, work.WorkItemID, work,
					lifecycleMetadata("work_item_id", work.WorkItemID, computerID, req.TrajectoryID, seq), now, now)
				if buildErr != nil {
					return types.LifecycleResult{}, buildErr
				}
				addCondition(objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID})
				objects = append(objects, workObj)
				if err := appendEventAtCurrent(&events, req, ownerID, computerID, seq, types.LifecycleWorkOpened, work.WorkItemID, "", []string{control.TargetAgentID}, "", now); err != nil {
					return types.LifecycleResult{}, err
				}
			default:
				return types.LifecycleResult{}, getErr
			}
		}

		updateKey := req.TrajectoryID + "\x00" + control.TargetAgentID + "\x00" + req.CallerAgentID + "\x00" + control.ControlID
		updateCanonicalID, buildErr := lifecycleCanonicalID(ogKindWorkerUpdate, ownerID, computerID, updateKey)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		if _, getErr := s.lifecycleGraph().GetObject(ctx, updateCanonicalID); getErr == nil {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		} else if !errors.Is(getErr, objectgraph.ErrNotFound) {
			return types.LifecycleResult{}, getErr
		}
		seq++
		packet := types.CoagentSourcePacket{
			UpdateID: control.ControlID, ProducerUpdateID: control.ControlID, OwnerID: ownerID, ComputerID: computerID,
			AgentID: req.CallerAgentID, TargetAgentID: control.TargetAgentID, ChannelID: req.DocumentID,
			MessageSeq: seq, TrajectoryID: req.TrajectoryID, Direction: types.LifecyclePacketDirectionControl,
			TargetWorkItemID: control.TargetWorkItemID, Role: agentprofile.Texture, SourceRunID: req.CallerRunID,
			PayloadDigest: control.PayloadDigest, Disposition: types.UpdatePending, LifecycleVersion: 1, ReducerSeq: seq,
			Packet: control.Packet, Content: control.Content, CreatedAt: now,
		}
		updateMeta := lifecycleMetadata("update_id", packet.UpdateID, computerID, req.TrajectoryID, seq)
		updateMeta["producer_update_id"], updateMeta["target_agent_id"] = packet.ProducerUpdateID, packet.TargetAgentID
		controlObj, buildErr := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, updateKey, packet, updateMeta, now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		addCondition(objectgraph.ObjectCondition{CanonicalID: controlObj.CanonicalID})
		objects = append(objects, controlObj)
		if err := appendEventAtCurrent(&events, req, ownerID, computerID, seq, types.LifecycleControlQueued, work.WorkItemID, packet.UpdateID, nil, "", now); err != nil {
			return types.LifecycleResult{}, err
		}
		controlPackets = append(controlPackets, packet)
		targetWorkItems = append(targetWorkItems, work)
		controlIDs = append(controlIDs, packet.UpdateID)
		targetWorkIDs = append(targetWorkIDs, work.WorkItemID)
	}

	if trajectory.SubjectRefs == nil {
		trajectory.SubjectRefs = make(map[string]string)
	}
	for key, value := range req.SubjectRefs {
		trajectory.SubjectRefs[key] = value
	}
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = seq, trajectory.LifecycleVersion+1, now
	callerAgent.LastReducerSeq, callerAgent.LifecycleVersion, callerAgent.UpdatedAt = seq, callerAgent.LifecycleVersion+1, now
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory,
		lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, seq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	callerAgentUpdated, err := lifecycleObject(ogKindAgent, ownerID, computerID, callerAgent.AgentID, callerAgent,
		lifecycleMetadata("agent_id", callerAgent.AgentID, computerID, req.TrajectoryID, seq), callerAgentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	objects = append(objects, trajectoryUpdated, callerAgentUpdated)
	eventObjs := make([]objectgraph.Object, 0, len(events))
	for _, event := range events {
		eventObj, buildErr := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event,
			lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, event.ReducerSeq), now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		addCondition(objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
		eventObjs = append(eventObjs, eventObj)
		objects = append(objects, eventObj)
	}
	turnRecord := &types.TextureTurnRecord{
		Outcome: req.Outcome, PriorHeadRevisionID: req.ExpectedHeadRevisionID, HeadRevisionID: resultDocument.CurrentRevisionID,
		InboundUpdateIDs: inboundIDs, ControlUpdateIDs: controlIDs, TargetWorkItemIDs: targetWorkIDs,
		CallerWorkItemID: callerWork.WorkItemID, CallerWorkDisposition: req.CallerWorkDisposition,
		OwnerInstructionIDs: ownerInstructionIDs, CausalRequestIDs: causalRequestIDs, Reason: req.Reason,
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID,
		req.CommandDigest, types.LifecycleApplyTextureTurn, seq, eventObjs)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	addCondition(objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, receiptObj)
	result := types.LifecycleResult{
		Receipt: receipt, Trajectory: trajectory, WorkItem: &callerWork, Agent: &callerAgent, Events: events, Document: &resultDocument,
		Revision: resultRevision, TextureTurn: turnRecord, Controls: controlPackets, TargetWorkItems: targetWorkItems,
	}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, result, edges...)
}

func appendEventAtCurrent(events *[]types.LifecycleEvent, req types.ApplyTextureTurnRequest, ownerID, computerID string, seq int64,
	kind types.LifecycleEventKind, workItemID, updateID string, refs []string, reason string, now time.Time) error {
	*events = append(*events, types.LifecycleEvent{
		EventID: req.CommandID + ":" + fmt.Sprintf("%d", len(*events)+1), OwnerID: ownerID, ComputerID: computerID,
		TrajectoryID: req.TrajectoryID, WorkItemID: workItemID, UpdateID: updateID, Kind: kind,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: seq, CommandID: req.CommandID,
		CommandDigest: req.CommandDigest, ArtifactRefs: refs, Reason: reason, CreatedAt: now,
	})
	return nil
}

func (s *Store) textureTurnDocumentObject(ctx context.Context, ownerID, computerID, docID string) (objectgraph.Object, types.Document, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, docID)
	if err != nil {
		return objectgraph.Object{}, types.Document{}, err
	}
	rec, err := decodeLifecycleObject[types.Document](obj)
	return obj, rec, err
}

func (s *Store) textureTurnRevisionObject(ctx context.Context, ownerID, computerID, revisionID string) (objectgraph.Object, types.Revision, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindTexRev, ownerID, computerID, revisionID)
	if err != nil {
		return objectgraph.Object{}, types.Revision{}, err
	}
	rec, err := decodeLifecycleObject[types.Revision](obj)
	return obj, rec, err
}

func (s *Store) textureTurnAgentObject(ctx context.Context, ownerID, computerID, agentID string) (objectgraph.Object, types.AgentRecord, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, agentID)
	if err != nil {
		return objectgraph.Object{}, types.AgentRecord{}, err
	}
	rec, err := decodeLifecycleObject[types.AgentRecord](obj)
	return obj, rec, err
}

func (s *Store) textureTurnRunObject(ctx context.Context, ownerID, computerID, runID string) (objectgraph.Object, types.RunRecord, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindRun, ownerID, computerID, runID)
	if err != nil {
		return objectgraph.Object{}, types.RunRecord{}, err
	}
	rec, err := decodeLifecycleObject[types.RunRecord](obj)
	return obj, rec, err
}

func (s *Store) textureTurnUpdateObject(ctx context.Context, ownerID, computerID, updateKey string) (objectgraph.Object, types.CoagentSourcePacket, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindWorkerUpdate, ownerID, computerID, updateKey)
	if err != nil {
		return objectgraph.Object{}, types.CoagentSourcePacket{}, err
	}
	rec, err := decodeLifecycleObject[types.CoagentSourcePacket](obj)
	return obj, rec, err
}

func textureTurnWorkEquivalent(existing, requested types.WorkItemRecord) bool {
	existing.CreatedAt, existing.UpdatedAt = time.Time{}, time.Time{}
	requested.CreatedAt, requested.UpdatedAt = time.Time{}, time.Time{}
	existing.LifecycleVersion, existing.LastReducerSeq = 0, 0
	requested.LifecycleVersion, requested.LastReducerSeq = 0, 0
	left, _ := json.Marshal(existing)
	right, _ := json.Marshal(requested)
	return string(left) == string(right)
}

// GetAppliedTextureTurnByCallerRun returns the durable Texture-turn receipt
// whose stored result names the exact caller activation. It is the recovery
// authority for legacy mutation projections after process-local failures.
func (s *Store) GetAppliedTextureTurnByCallerRun(ctx context.Context, ownerID, computerID, trajectoryID, runID string) (types.LifecycleResult, error) {
	objects, err := s.lifecycleTransitionObjects(ctx, ogKindLifecycleCmd, strings.TrimSpace(trajectoryID), strings.TrimSpace(ownerID), strings.TrimSpace(computerID))
	if err != nil {
		return types.LifecycleResult{}, err
	}
	for i := len(objects) - 1; i >= 0; i-- {
		receipt, decodeErr := decodeLifecycleObject[types.LifecycleCommandReceipt](objects[i])
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		stored := receipt.StoredResult
		if receipt.Kind != types.LifecycleApplyTextureTurn || stored == nil || stored.TextureTurn == nil || stored.Agent == nil || strings.TrimSpace(stored.Agent.ActiveRunID) != strings.TrimSpace(runID) {
			continue
		}
		return types.LifecycleResult{Receipt: receipt, Trajectory: stored.Trajectory, Schema: stored.Schema, WorkItem: stored.WorkItem, Agent: stored.Agent, Update: stored.Update, Events: stored.Events, Document: stored.Document, Revision: stored.Revision, TextureTurn: stored.TextureTurn, Controls: stored.Controls, TargetWorkItems: stored.TargetWorkItems}, nil
	}
	return types.LifecycleResult{}, ErrNotFound
}
