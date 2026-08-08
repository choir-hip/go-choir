package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func ComputeQueueLifecycleOwnerInstructionDigest(req types.QueueLifecycleOwnerInstructionRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.ExpectedLifecycleVersion = 0
	req.CommandID = strings.TrimSpace(req.CommandID)
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.InstructionID = strings.TrimSpace(req.InstructionID)
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	req.TargetAgentID = strings.TrimSpace(req.TargetAgentID)
	req.TargetWorkItemID = strings.TrimSpace(req.TargetWorkItemID)
	req.ExpectedHeadRevisionID = strings.TrimSpace(req.ExpectedHeadRevisionID)
	req.Kind = types.LifecycleOwnerInstructionKind(strings.TrimSpace(string(req.Kind)))
	req.Content = strings.TrimSpace(req.Content)
	return lifecycleDigest(req)
}

func (s *Store) QueueLifecycleOwnerInstruction(ctx context.Context, req types.QueueLifecycleOwnerInstructionRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID, req.CommandDigest = strings.TrimSpace(req.CommandID), strings.TrimSpace(req.CommandDigest)
	req.RequestID, req.InstructionID = strings.TrimSpace(req.RequestID), strings.TrimSpace(req.InstructionID)
	req.DocumentID, req.TrajectoryID = strings.TrimSpace(req.DocumentID), strings.TrimSpace(req.TrajectoryID)
	req.TargetAgentID, req.TargetWorkItemID = strings.TrimSpace(req.TargetAgentID), strings.TrimSpace(req.TargetWorkItemID)
	req.ExpectedHeadRevisionID = strings.TrimSpace(req.ExpectedHeadRevisionID)
	req.Kind = types.LifecycleOwnerInstructionKind(strings.TrimSpace(string(req.Kind)))
	req.Content = strings.TrimSpace(req.Content)
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return types.LifecycleResult{}, err
	}
	if req.RequestID == "" || req.InstructionID == "" || req.DocumentID == "" || req.TargetAgentID == "" ||
		req.TargetWorkItemID == "" || req.ExpectedLifecycleVersion <= 0 || req.ExpectedHeadRevisionID == "" || req.Content == "" ||
		(req.Kind != types.LifecycleOwnerTell && req.Kind != types.LifecycleOwnerCorrect) {
		return types.LifecycleResult{}, fmt.Errorf("lifecycle owner instruction: complete request, target, head, kind, and content are required")
	}
	computed, digestErr := ComputeQueueLifecycleOwnerInstructionDigest(req)
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
	if trajectory.Status != types.TrajectoryLive || trajectory.LifecycleVersion != req.ExpectedLifecycleVersion ||
		strings.TrimSpace(trajectory.SubjectRefs["doc_id"]) != req.DocumentID {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	documentObj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, req.DocumentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	document, err := decodeLifecycleObject[types.Document](documentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if document.OwnerID != ownerID || document.ComputerID != computerID || document.TrajectoryID != req.TrajectoryID ||
		document.CurrentRevisionID != req.ExpectedHeadRevisionID || req.TargetAgentID != "texture:"+req.DocumentID {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	agentObj, err := s.lifecycleGetObject(ctx, ogKindAgent, ownerID, computerID, req.TargetAgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	agent, err := decodeLifecycleObject[types.AgentRecord](agentObj)
	if err != nil || agent.OwnerID != ownerID || agent.ComputerID != computerID || agent.LifecycleVersion <= 0 ||
		agent.Profile != "texture" || agent.Role != "texture" || agent.ChannelID != req.DocumentID {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	workObj, work, err := s.lifecycleWorkObject(ctx, ownerID, computerID, req.TargetWorkItemID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if work.OwnerID != ownerID || work.ComputerID != computerID || work.TrajectoryID != req.TrajectoryID || work.Status != types.WorkItemOpen || work.AssignedAgentID != req.TargetAgentID || work.AuthorityProfile != "texture" {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	now := time.Now().UTC()
	nextSeq := trajectory.ReducerSeq + 1
	instruction := types.LifecycleOwnerInstruction{
		Schema: types.LifecycleOwnerInstructionSchemaV1, InstructionID: req.InstructionID, RequestID: req.RequestID,
		OwnerID: ownerID, ComputerID: computerID, DocumentID: req.DocumentID, TrajectoryID: req.TrajectoryID,
		TargetAgentID: req.TargetAgentID, TargetWorkItemID: req.TargetWorkItemID, HeadRevisionID: req.ExpectedHeadRevisionID,
		Kind: req.Kind, Content: req.Content, Status: types.LifecycleOwnerInstructionPending,
		LifecycleVersion: 1, ReducerSeq: nextSeq, CreatedAt: now,
	}
	instructionKey := req.TrajectoryID + "\x00" + req.InstructionID
	instructionObj, err := lifecycleObject(ogKindOwnerInstruction, ownerID, computerID, instructionKey, instruction,
		lifecycleMetadata("instruction_id", req.InstructionID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = nextSeq, trajectory.LifecycleVersion+1, now
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory,
		lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, nextSeq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID, TrajectoryID: req.TrajectoryID,
		WorkItemID: req.TargetWorkItemID, Kind: types.LifecycleOwnerInstructionQueued,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: nextSeq, CommandID: req.CommandID,
		CommandDigest: req.CommandDigest, RequestID: req.RequestID, ArtifactRefs: []string{instructionObj.CanonicalID}, CreatedAt: now,
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event,
		lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, nextSeq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest,
		types.LifecycleQueueOwnerInstruction, nextSeq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash},
		{CanonicalID: agentObj.CanonicalID, Exists: true, ExpectedContentHash: agentObj.ContentHash},
		{CanonicalID: instructionObj.CanonicalID}, {CanonicalID: eventObj.CanonicalID}, {CanonicalID: receiptObj.CanonicalID},
	}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions,
		[]objectgraph.Object{trajectoryUpdated, instructionObj, eventObj, receiptObj}, types.LifecycleResult{
			Receipt: receipt, Trajectory: trajectory, OwnerInstruction: &instruction, Events: []types.LifecycleEvent{event},
			Document: &document,
		})
}

func (s *Store) GetLifecycleOwnerInstruction(ctx context.Context, ownerID, computerID, trajectoryID, instructionID string) (types.LifecycleOwnerInstruction, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return types.LifecycleOwnerInstruction{}, err
	}
	_, instruction, err := s.lifecycleOwnerInstructionObject(ctx, ownerID, computerID, trajectoryID, instructionID)
	return instruction, err
}

func (s *Store) ListPendingLifecycleOwnerInstructions(ctx context.Context, ownerID, computerID, trajectoryID, targetAgentID string, limit int) ([]types.LifecycleOwnerInstruction, error) {
	out, err := s.ListPendingLifecycleOwnerInstructionsForHead(ctx, ownerID, computerID, trajectoryID, targetAgentID, "")
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ListPendingLifecycleOwnerInstructionsForHead returns the complete ordered
// occurrence set. An empty head selects every pending occurrence. Runtime turn
// construction and reducer completeness CAS deliberately share this one
// unbounded reader so pagination cannot strand the 101st instruction.
func (s *Store) ListPendingLifecycleOwnerInstructionsForHead(ctx context.Context, ownerID, computerID, trajectoryID, targetAgentID, headRevisionID string) ([]types.LifecycleOwnerInstruction, error) {
	ownerID, computerID, err := normalizeLifecycleScope(ownerID, computerID)
	if err != nil {
		return nil, err
	}
	trajectoryID, targetAgentID, headRevisionID = strings.TrimSpace(trajectoryID), strings.TrimSpace(targetAgentID), strings.TrimSpace(headRevisionID)
	if trajectoryID == "" || targetAgentID == "" {
		return nil, fmt.Errorf("owner instructions: trajectory_id and target_agent_id are required")
	}
	objs, err := s.lifecycleTransitionObjects(ctx, ogKindOwnerInstruction, trajectoryID, ownerID, computerID)
	if err != nil {
		return nil, err
	}
	out := make([]types.LifecycleOwnerInstruction, 0)
	for _, obj := range objs {
		instruction, decodeErr := decodeLifecycleObject[types.LifecycleOwnerInstruction](obj)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if instruction.TargetAgentID == targetAgentID && instruction.Status == types.LifecycleOwnerInstructionPending &&
			(headRevisionID == "" || instruction.HeadRevisionID == headRevisionID) {
			out = append(out, instruction)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReducerSeq == out[j].ReducerSeq {
			return out[i].InstructionID < out[j].InstructionID
		}
		return out[i].ReducerSeq < out[j].ReducerSeq
	})
	return out, nil
}

func (s *Store) lifecycleOwnerInstructionObject(ctx context.Context, ownerID, computerID, trajectoryID, instructionID string) (objectgraph.Object, types.LifecycleOwnerInstruction, error) {
	key := strings.TrimSpace(trajectoryID) + "\x00" + strings.TrimSpace(instructionID)
	obj, err := s.lifecycleGetObject(ctx, ogKindOwnerInstruction, ownerID, computerID, key)
	if err != nil {
		return objectgraph.Object{}, types.LifecycleOwnerInstruction{}, err
	}
	instruction, err := decodeLifecycleObject[types.LifecycleOwnerInstruction](obj)
	return obj, instruction, err
}
