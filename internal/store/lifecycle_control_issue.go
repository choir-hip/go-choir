package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ComputeIssueLifecycleControlDigest returns the canonical replay identity for
// a standalone lifecycle-control issue. Optimistic versions are execution
// preconditions, not command identity.
func ComputeIssueLifecycleControlDigest(req types.IssueLifecycleControlRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.CommandID = strings.TrimSpace(req.CommandID)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.CallerAgentID = strings.TrimSpace(req.CallerAgentID)
	req.CallerRunID = strings.TrimSpace(req.CallerRunID)
	req.ExpectedLifecycleVersion, req.ExpectedCallerLifecycleVersion = 0, 0
	req.Reason = strings.TrimSpace(req.Reason)
	turnLike, err := normalizeTextureTurnDigestRequest(types.ApplyTextureTurnRequest{Controls: req.Controls})
	if err != nil {
		return "", err
	}
	req.Controls = turnLike.Controls
	return lifecycleDigest(req)
}

func normalizeIssueLifecycleControlRequest(req types.IssueLifecycleControlRequest) (types.IssueLifecycleControlRequest, error) {
	req.CommandID = strings.TrimSpace(req.CommandID)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	req.DocumentID = strings.TrimSpace(req.DocumentID)
	req.CallerAgentID = strings.TrimSpace(req.CallerAgentID)
	req.CallerRunID = strings.TrimSpace(req.CallerRunID)
	req.Reason = strings.TrimSpace(req.Reason)
	turnLike, err := normalizeTextureTurnDigestRequest(types.ApplyTextureTurnRequest{Controls: req.Controls})
	if err != nil {
		return req, err
	}
	req.Controls = turnLike.Controls
	return req, nil
}

func validateIssueLifecycleControlShape(req types.IssueLifecycleControlRequest) error {
	if err := validateLifecycleCommand(req.CommandID, req.CommandDigest, req.TrajectoryID); err != nil {
		return err
	}
	if req.DocumentID == "" || req.CallerAgentID == "" || req.CallerRunID == "" || req.ExpectedLifecycleVersion <= 0 || req.ExpectedCallerLifecycleVersion <= 0 {
		return fmt.Errorf("issue lifecycle control: complete document, caller, run, and lifecycle versions are required")
	}
	if len(req.Controls) == 0 {
		return fmt.Errorf("issue lifecycle control: at least one control is required")
	}
	return validateTextureTurnControls(req.DocumentID, req.Controls, nil, "issue lifecycle control")
}

// lifecycleControlAccumulator owns the object/event ordering of control issue.
// Both Texture turns and standalone issue commands feed this single reducer
// path so a turn's existing control semantics remain byte-for-byte ordered.
type lifecycleControlAccumulator struct {
	store *Store
	ctx   context.Context

	ownerID, computerID, documentID, trajectoryID        string
	callerAgentID, callerRunID, commandID, commandDigest string
	operation                                            string
	now                                                  time.Time
	seq                                                  *int64

	conditions     *[]objectgraph.ObjectCondition
	objects        *[]objectgraph.Object
	events         *[]types.LifecycleEvent
	seenConditions map[string]struct{}

	controls        *[]types.CoagentSourcePacket
	targetWorkItems *[]types.WorkItemRecord
	controlIDs      *[]string
	targetWorkIDs   *[]string
}

func newLifecycleControlAccumulator(s *Store, ctx context.Context, ownerID, computerID, documentID, trajectoryID, callerAgentID, callerRunID, commandID, commandDigest, operation string, now time.Time, seq *int64, conditions *[]objectgraph.ObjectCondition, objects *[]objectgraph.Object, events *[]types.LifecycleEvent, controls *[]types.CoagentSourcePacket, targetWorkItems *[]types.WorkItemRecord, controlIDs *[]string, targetWorkIDs *[]string) *lifecycleControlAccumulator {
	seenConditions := make(map[string]struct{}, len(*conditions))
	for _, condition := range *conditions {
		seenConditions[condition.CanonicalID] = struct{}{}
	}
	return &lifecycleControlAccumulator{store: s, ctx: ctx, ownerID: ownerID, computerID: computerID, documentID: documentID, trajectoryID: trajectoryID, callerAgentID: callerAgentID, callerRunID: callerRunID, commandID: commandID, commandDigest: commandDigest, operation: operation, now: now, seq: seq, conditions: conditions, objects: objects, events: events, seenConditions: seenConditions, controls: controls, targetWorkItems: targetWorkItems, controlIDs: controlIDs, targetWorkIDs: targetWorkIDs}
}

func (a *lifecycleControlAccumulator) addCondition(condition objectgraph.ObjectCondition) {
	if _, exists := a.seenConditions[condition.CanonicalID]; exists {
		return
	}
	a.seenConditions[condition.CanonicalID] = struct{}{}
	*a.conditions = append(*a.conditions, condition)
}

func (a *lifecycleControlAccumulator) appendEvent(kind types.LifecycleEventKind, workItemID, updateID string, refs []string, reason string) error {
	*a.events = append(*a.events, types.LifecycleEvent{
		EventID: a.commandID + ":" + fmt.Sprintf("%d", len(*a.events)+1), OwnerID: a.ownerID, ComputerID: a.computerID,
		TrajectoryID: a.trajectoryID, WorkItemID: workItemID, UpdateID: updateID, Kind: kind,
		ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: *a.seq, CommandID: a.commandID,
		CommandDigest: a.commandDigest, ArtifactRefs: refs, Reason: reason, CreatedAt: a.now,
	})
	return nil
}

func (a *lifecycleControlAccumulator) issue(controls []types.TextureTurnControl) error {
	for _, control := range controls {
		if err := a.issueOne(control); err != nil {
			return err
		}
	}
	return nil
}

func (a *lifecycleControlAccumulator) issueOne(control types.TextureTurnControl) error {
	var binding LifecycleTextureControlTargetBinding
	var targetAgentObj objectgraph.Object
	var targetAgent types.AgentRecord
	if control.OpenAgent != nil {
		targetAgent = *control.OpenAgent
		targetAgent.OwnerID, targetAgent.ComputerID, targetAgent.ComputerID = a.ownerID, a.computerID, a.computerID
		targetAgent.LifecycleVersion, targetAgent.LastReducerSeq = 1, *a.seq+1
		targetAgent.CreatedAt, targetAgent.UpdatedAt = a.now, a.now
		if targetAgent.AgentID != control.TargetAgentID || targetAgent.Profile != agentprofile.Research || targetAgent.Role != agentprofile.Research || targetAgent.ChannelID != a.documentID || targetAgent.ActiveRunID != "" {
			return ErrLifecycleInvalidTransition
		}
		targetCanonicalID, err := lifecycleCanonicalID(ogKindAgent, a.ownerID, a.computerID, targetAgent.AgentID)
		if err != nil {
			return err
		}
		if _, err := a.store.lifecycleGraph().GetObject(a.ctx, targetCanonicalID); err == nil {
			return ErrLifecycleCommandConflict
		} else if !errors.Is(err, objectgraph.ErrNotFound) {
			return err
		}
		targetAgentMeta := lifecycleMetadata("agent_id", targetAgent.AgentID, a.computerID, a.trajectoryID, *a.seq+1)
		targetAgentMeta["channel_id"] = targetAgent.ChannelID
		targetAgentObj, err = lifecycleObject(ogKindAgent, a.ownerID, a.computerID, targetAgent.AgentID, targetAgent, targetAgentMeta, a.now, a.now)
		if err != nil {
			return err
		}
		a.addCondition(objectgraph.ObjectCondition{CanonicalID: targetAgentObj.CanonicalID})
		*a.objects = append(*a.objects, targetAgentObj)
		binding = LifecycleTextureControlTargetBinding{TargetAgent: targetAgent, TargetProfile: agentprofile.Research}
	} else {
		validatorWorkID := control.TargetWorkItemID
		if control.OpenWork != nil {
			validatorWorkID = ""
		}
		var err error
		binding, err = a.store.ValidateLifecycleTextureControlTarget(a.ctx, LifecycleTextureControlTargetRequest{
			OwnerID: a.ownerID, ComputerID: a.computerID, DocumentID: a.documentID, TrajectoryID: a.trajectoryID,
			CallerAgentID: a.callerAgentID, CallerRunID: a.callerRunID, TargetAgentID: control.TargetAgentID, TargetWorkItemID: validatorWorkID,
		})
		if err != nil {
			return err
		}
		var targetErr error
		targetAgentObj, targetAgent, targetErr = a.store.textureTurnAgentObject(a.ctx, a.ownerID, a.computerID, control.TargetAgentID)
		if targetErr != nil || !reflect.DeepEqual(targetAgent, binding.TargetAgent) {
			if targetErr != nil {
				return targetErr
			}
			return ErrConcurrentStateChange
		}
		a.addCondition(objectgraph.ObjectCondition{CanonicalID: targetAgentObj.CanonicalID, Exists: true, ExpectedContentHash: targetAgentObj.ContentHash})
	}
	if binding.TargetProfile == agentprofile.Management && (control.Packet.Kind != "execution_request" || len(control.Packet.Actions) == 0) {
		return fmt.Errorf("%s: persistent-Management control requires execution_request actions", a.operation)
	}
	if binding.TargetRun != nil {
		targetRunObj, targetRun, err := a.store.textureTurnRunObject(a.ctx, a.ownerID, a.computerID, binding.TargetRun.RunID)
		if err != nil || !reflect.DeepEqual(targetRun, *binding.TargetRun) {
			if err != nil {
				return err
			}
			return ErrConcurrentStateChange
		}
		a.addCondition(objectgraph.ObjectCondition{CanonicalID: targetRunObj.CanonicalID, Exists: true, ExpectedContentHash: targetRunObj.ContentHash})
	}

	var workObj objectgraph.Object
	var work types.WorkItemRecord
	if control.OpenWork == nil {
		if binding.TargetWorkItem == nil {
			return ErrLifecycleInvalidTransition
		}
		work = *binding.TargetWorkItem
		var rawWork types.WorkItemRecord
		var err error
		workObj, rawWork, err = a.store.lifecycleWorkObject(a.ctx, a.ownerID, a.computerID, work.WorkItemID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(rawWork, work) {
			return ErrConcurrentStateChange
		}
		a.addCondition(objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash})
	} else {
		openerProfile := binding.TargetProfile
		switch openerProfile {
		case agentprofile.Management:
			if control.OpenAgent != nil || binding.TargetAgent.AgentID != agentprofile.Management+":"+a.ownerID {
				return ErrLifecycleInvalidTransition
			}
		case agentprofile.Research:
			if control.OpenAgent == nil || binding.TargetAgent.AgentID != control.TargetAgentID {
				return ErrLifecycleInvalidTransition
			}
		default:
			return ErrLifecycleInvalidTransition
		}
		var err error
		work, err = normalizeLifecycleWork(*control.OpenWork, a.ownerID, a.computerID, a.trajectoryID, a.now)
		if err != nil {
			return err
		}
		if work.AssignedAgentID != control.TargetAgentID || work.AuthorityProfile != openerProfile || work.WorkItemID != control.TargetWorkItemID {
			return ErrLifecycleInvalidTransition
		}
		work.CreatedByRunID = a.callerRunID
		work.Details = cloneTextureTurnDetails(work.Details)
		if work.Details == nil {
			work.Details = map[string]any{}
		}
		work.Details["requested_by_profile"] = agentprofile.Texture
		work.Details["requested_by_agent_id"] = a.callerAgentID
		work.Details["requested_by_run_id"] = a.callerRunID
		workCanonicalID, err := lifecycleCanonicalID(ogKindWorkItem, a.ownerID, a.computerID, work.WorkItemID)
		if err != nil {
			return err
		}
		existingObj, err := a.store.lifecycleGraph().GetObject(a.ctx, workCanonicalID)
		switch {
		case err == nil:
			existing, decodeErr := decodeLifecycleObject[types.WorkItemRecord](existingObj)
			if decodeErr != nil {
				return decodeErr
			}
			validated, validateErr := a.store.ValidateLifecycleTextureControlTarget(a.ctx, LifecycleTextureControlTargetRequest{
				OwnerID: a.ownerID, ComputerID: a.computerID, DocumentID: a.documentID, TrajectoryID: a.trajectoryID,
				CallerAgentID: a.callerAgentID, CallerRunID: a.callerRunID, TargetAgentID: control.TargetAgentID, TargetWorkItemID: control.TargetWorkItemID,
			})
			if validateErr != nil || validated.TargetWorkItem == nil || !textureTurnWorkEquivalent(existing, work) {
				return ErrLifecycleCommandConflict
			}
			work, workObj = existing, existingObj
			a.addCondition(objectgraph.ObjectCondition{CanonicalID: existingObj.CanonicalID, Exists: true, ExpectedContentHash: existingObj.ContentHash})
		case errors.Is(err, objectgraph.ErrNotFound):
			*a.seq++
			work.LifecycleVersion, work.LastReducerSeq = 1, *a.seq
			workObj, err = lifecycleObject(ogKindWorkItem, a.ownerID, a.computerID, work.WorkItemID, work,
				lifecycleMetadata("work_item_id", work.WorkItemID, a.computerID, a.trajectoryID, *a.seq), a.now, a.now)
			if err != nil {
				return err
			}
			a.addCondition(objectgraph.ObjectCondition{CanonicalID: workObj.CanonicalID})
			*a.objects = append(*a.objects, workObj)
			if err := a.appendEvent(types.LifecycleWorkOpened, work.WorkItemID, "", []string{control.TargetAgentID}, ""); err != nil {
				return err
			}
		default:
			return err
		}
	}

	updateKey := a.trajectoryID + "\x00" + control.TargetAgentID + "\x00" + a.callerAgentID + "\x00" + control.ControlID
	updateCanonicalID, err := lifecycleCanonicalID(ogKindWorkerUpdate, a.ownerID, a.computerID, updateKey)
	if err != nil {
		return err
	}
	if _, err := a.store.lifecycleGraph().GetObject(a.ctx, updateCanonicalID); err == nil {
		return ErrLifecycleCommandConflict
	} else if !errors.Is(err, objectgraph.ErrNotFound) {
		return err
	}
	*a.seq++
	packet := types.CoagentSourcePacket{
		UpdateID: control.ControlID, ProducerUpdateID: control.ControlID, OwnerID: a.ownerID, ComputerID: a.computerID,
		AgentID: a.callerAgentID, TargetAgentID: control.TargetAgentID, ChannelID: a.documentID,
		MessageSeq: *a.seq, TrajectoryID: a.trajectoryID, Direction: types.LifecyclePacketDirectionControl,
		TargetWorkItemID: control.TargetWorkItemID, Role: agentprofile.Texture, SourceRunID: a.callerRunID,
		PayloadDigest: control.PayloadDigest, Disposition: types.UpdatePending, LifecycleVersion: 1, ReducerSeq: *a.seq,
		Packet: control.Packet, Content: control.Content, CreatedAt: a.now,
	}
	if packet.Packet.Kind == "execution_request" && control.TargetAgentID == agentprofile.Management+":"+a.ownerID {
		ordinal, ordinalErr := a.store.nextArrivalOrdinal(a.ctx, a.ownerID, a.computerID)
		if ordinalErr != nil {
			return fmt.Errorf("%s: allocate arrival ordinal: %w", a.operation, ordinalErr)
		}
		packet.ArrivalOrdinal = ordinal
	}
	updateMeta := lifecycleMetadata("update_id", packet.UpdateID, a.computerID, a.trajectoryID, *a.seq)
	updateMeta["producer_update_id"], updateMeta["target_agent_id"] = packet.ProducerUpdateID, packet.TargetAgentID
	controlObj, err := lifecycleObject(ogKindWorkerUpdate, a.ownerID, a.computerID, updateKey, packet, updateMeta, a.now, a.now)
	if err != nil {
		return err
	}
	a.addCondition(objectgraph.ObjectCondition{CanonicalID: controlObj.CanonicalID})
	*a.objects = append(*a.objects, controlObj)
	if err := a.appendEvent(types.LifecycleControlQueued, work.WorkItemID, packet.UpdateID, nil, ""); err != nil {
		return err
	}
	*a.controls = append(*a.controls, packet)
	*a.targetWorkItems = append(*a.targetWorkItems, work)
	*a.controlIDs = append(*a.controlIDs, packet.UpdateID)
	*a.targetWorkIDs = append(*a.targetWorkIDs, work.WorkItemID)
	return nil
}

// IssueLifecycleControl atomically queues one or more downward Texture controls
// without manufacturing an otherwise-empty Texture turn.
func (s *Store) IssueLifecycleControl(ctx context.Context, req types.IssueLifecycleControlRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	normalized, err := normalizeIssueLifecycleControlRequest(req)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	normalized.OwnerID, normalized.ComputerID = ownerID, computerID
	normalized.CommandDigest = strings.TrimSpace(req.CommandDigest)
	req = normalized
	if err := validateIssueLifecycleControlShape(req); err != nil {
		return types.LifecycleResult{}, err
	}
	computed, digestErr := ComputeIssueLifecycleControlDigest(req)
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
	callerAgentObj, callerAgent, err := s.textureTurnAgentObject(ctx, ownerID, computerID, req.CallerAgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	callerRunObj, callerRun, err := s.textureTurnRunObject(ctx, ownerID, computerID, req.CallerRunID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if trajectory.Status != types.TrajectoryLive || trajectory.OwnerID != ownerID || trajectory.ComputerID != computerID || trajectory.TrajectoryID != req.TrajectoryID || trajectory.LifecycleVersion != req.ExpectedLifecycleVersion || strings.TrimSpace(trajectory.SubjectRefs["doc_id"]) != req.DocumentID || document.DocID != req.DocumentID || document.OwnerID != ownerID || document.ComputerID != computerID || document.TrajectoryID != req.TrajectoryID || document.ArchivedAt != nil || callerAgent.AgentID != req.CallerAgentID || callerAgent.OwnerID != ownerID || callerAgent.ComputerID != computerID || callerAgent.Profile != agentprofile.Texture || callerAgent.Role != agentprofile.Texture || callerAgent.ChannelID != req.DocumentID || callerAgent.ActiveRunID != req.CallerRunID || callerAgent.LifecycleVersion != req.ExpectedCallerLifecycleVersion || callerRun.RunID != req.CallerRunID || callerRun.AgentID != req.CallerAgentID || callerRun.OwnerID != ownerID || callerRun.ComputerID != computerID || callerRun.TrajectoryID != req.TrajectoryID || callerRun.AgentProfile != agentprofile.Texture || callerRun.AgentRole != agentprofile.Texture || callerRun.ChannelID != req.DocumentID || !callerRun.State.Active() {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}

	now := time.Now().UTC()
	conditions := []objectgraph.ObjectCondition{{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash}, {CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash}, {CanonicalID: callerAgentObj.CanonicalID, Exists: true, ExpectedContentHash: callerAgentObj.ContentHash}, {CanonicalID: callerRunObj.CanonicalID, Exists: true, ExpectedContentHash: callerRunObj.ContentHash}}
	objects := make([]objectgraph.Object, 0, len(req.Controls)*3+3)
	events := make([]types.LifecycleEvent, 0, len(req.Controls)*2)
	seq := trajectory.ReducerSeq
	controls := make([]types.CoagentSourcePacket, 0, len(req.Controls))
	targetWorkItems := make([]types.WorkItemRecord, 0, len(req.Controls))
	controlIDs := make([]string, 0, len(req.Controls))
	targetWorkIDs := make([]string, 0, len(req.Controls))
	accumulator := newLifecycleControlAccumulator(s, ctx, ownerID, computerID, req.DocumentID, req.TrajectoryID, req.CallerAgentID, req.CallerRunID, req.CommandID, req.CommandDigest, "issue lifecycle control", now, &seq, &conditions, &objects, &events, &controls, &targetWorkItems, &controlIDs, &targetWorkIDs)
	if err := accumulator.issue(req.Controls); err != nil {
		return types.LifecycleResult{}, err
	}

	trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = seq, trajectory.LifecycleVersion+1, now
	callerAgent.LastReducerSeq, callerAgent.LifecycleVersion, callerAgent.UpdatedAt = seq, callerAgent.LifecycleVersion+1, now
	trajectoryUpdated, err := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory, lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, seq), trajectoryObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	callerAgentUpdated, err := lifecycleObject(ogKindAgent, ownerID, computerID, callerAgent.AgentID, callerAgent, lifecycleMetadata("agent_id", callerAgent.AgentID, computerID, req.TrajectoryID, seq), callerAgentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	objects = append(objects, trajectoryUpdated, callerAgentUpdated)
	eventObjs := make([]objectgraph.Object, 0, len(events))
	for _, event := range events {
		eventObj, buildErr := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event, lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, event.ReducerSeq), now, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		accumulator.addCondition(objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
		eventObjs = append(eventObjs, eventObj)
		objects = append(objects, eventObj)
	}
	turnRecord := &types.TextureTurnRecord{ControlUpdateIDs: controlIDs, TargetWorkItemIDs: targetWorkIDs, Reason: req.Reason}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleIssueControl, seq, eventObjs)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	accumulator.addCondition(objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, receiptObj)
	result := types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Agent: &callerAgent, Events: events, Document: &document, TextureTurn: turnRecord, Controls: controls, TargetWorkItems: targetWorkItems}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, result)
}
