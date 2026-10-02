package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// CommitLifecycleAct (record-native RN3): one command mints the commitment
// record AND its derived directive packet atomically. The record is the
// authored act; the packet is delivery state derived from it
// (UpdateID = recordID + ":packet"). An addressed act either commits both or
// fails whole — a ledger record without its packet recreates the
// envelope-era split-brain this station retires.

// ogEdgeRecordPacket is the provenance edge from the minted directive packet
// to the commitment record it derives from.
const ogEdgeRecordPacket = objectgraph.EdgeKind("commitment_record_packet")

// ComputeCommitLifecycleActDigest returns the canonical replay identity.
// Optimistic versions are execution preconditions, not command identity.
func ComputeCommitLifecycleActDigest(req types.CommitLifecycleActRequest) (string, error) {
	req.OwnerID, req.ComputerID, req.CommandDigest = "", "", ""
	req.CommandID = strings.TrimSpace(req.CommandID)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	req.CallerAgentID = strings.TrimSpace(req.CallerAgentID)
	req.CallerRunID = strings.TrimSpace(req.CallerRunID)
	req.ExpectedLifecycleVersion, req.ExpectedCallerLifecycleVersion = 0, 0
	req.Reason = strings.TrimSpace(req.Reason)
	// Provenance commit time is store-adjacent observation metadata, not the
	// authored act identity. A re-reduced cell stamps a fresh wall time but
	// must replay its already committed record and derived packet.
	req.Record.Provenance.CommittedAt = ""
	if req.PacketSpec != nil {
		spec := *req.PacketSpec
		spec.TargetAgentID = strings.TrimSpace(spec.TargetAgentID)
		spec.TrajectoryID = strings.TrimSpace(spec.TrajectoryID)
		spec.ChannelID = strings.TrimSpace(spec.ChannelID)
		spec.WorkItemID = strings.TrimSpace(spec.WorkItemID)
		spec.Direction = types.LifecyclePacketDirection(strings.TrimSpace(string(spec.Direction)))
		if spec.Direction == "" {
			spec.Direction = types.LifecyclePacketDirectionDirective
		}
		if spec.Direction == types.LifecyclePacketDirectionProducerReport {
			var err error
			spec.WorkDisposition, err = normalizeUpdateWorkDisposition(spec.WorkDisposition)
			if err != nil {
				return "", err
			}
		}
		req.PacketSpec = &spec
	}
	return lifecycleDigest(req)
}

func validateCommitLifecycleActShape(req types.CommitLifecycleActRequest) error {
	if req.CallerAgentID == "" || req.CallerRunID == "" {
		return fmt.Errorf("commit lifecycle act: caller agent and run are required")
	}
	rec := req.Record
	if strings.TrimSpace(rec.RecordID) == "" {
		return fmt.Errorf("commit lifecycle act: record_id is required")
	}
	kind := rec.RecordKind()
	switch kind {
	case types.CommitmentKindPrecommit, types.CommitmentKindReport,
		types.CommitmentKindResolve, types.CommitmentKindDisagreement:
	case types.CommitmentKindDirective:
		if rec.Directive == nil || !types.IsValidDirectiveSubtype(rec.Directive.Subtype) {
			return fmt.Errorf("commit lifecycle act: directive record requires a valid subtype")
		}
		if rec.Directive.Subtype == types.CommitmentDirectiveNote && strings.TrimSpace(rec.Directive.Body) == "" {
			return fmt.Errorf("commit lifecycle act: note directive requires a body")
		}
	default:
		return fmt.Errorf("commit lifecycle act: record kind %q is not a mintable act", rec.Kind)
	}
	if rec.Provenance.AgentID != req.CallerAgentID {
		return fmt.Errorf("commit lifecycle act: record provenance agent must equal the caller agent")
	}
	addressee := strings.TrimSpace(rec.Addressee)
	if (addressee == "") != (req.PacketSpec == nil) {
		return fmt.Errorf("commit lifecycle act: packet_spec is required exactly for addressed records")
	}
	if spec := req.PacketSpec; spec != nil {
		if spec.Direction != types.LifecyclePacketDirectionDirective &&
			spec.Direction != types.LifecyclePacketDirectionProducerReport {
			return fmt.Errorf("commit lifecycle act: packet direction %q is invalid", spec.Direction)
		}
		if spec.Direction == types.LifecyclePacketDirectionProducerReport {
			if strings.TrimSpace(spec.WorkItemID) == "" {
				return fmt.Errorf("commit lifecycle act: producer report work_item_id is required")
			}
			if err := validateUpdateWorkConsequence(spec.WorkDisposition, spec.WorkItemID, "commit lifecycle act producer report"); err != nil {
				return err
			}
		}
		if strings.TrimSpace(spec.TargetAgentID) == "" {
			return fmt.Errorf("commit lifecycle act: packet target_agent_id is required")
		}
		if addressee != "" {
			target := strings.TrimSpace(spec.TargetAgentID)
			// Agent-scoped addressees ("profile:suffix") pin the exact agent;
			// bare desk names pin the target's canonical profile — the runtime
			// resolver, not the model payload, owns name→agent resolution.
			if strings.Contains(addressee, ":") {
				if addressee != target {
					return fmt.Errorf("commit lifecycle act: record addressee %q disagrees with packet target %q", addressee, target)
				}
			} else if target == "" || !strings.HasPrefix(target, addressee+":") {
				return fmt.Errorf("commit lifecycle act: record addressee %q disagrees with packet target %q", addressee, target)
			}
		}
		digest, err := ComputeLifecycleUpdatePayloadDigest(spec.Packet, spec.Content)
		if err != nil {
			return err
		}
		if digest != spec.PayloadDigest {
			return fmt.Errorf("commit lifecycle act: packet payload digest mismatch: %w", ErrLifecycleCommandConflict)
		}
	}
	return nil
}

// CommitLifecycleAct atomically mints the commitment record and its derived
// directive packet. Record-only acts (no addressee) mint the record alone —
// ledger-only is the correct shape for unaddressed acts.
func (s *Store) CommitLifecycleAct(ctx context.Context, req types.CommitLifecycleActRequest) (types.LifecycleResult, error) {
	ownerID, computerID, err := normalizeLifecycleScope(req.OwnerID, req.ComputerID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.OwnerID, req.ComputerID = ownerID, computerID
	req.CommandID = strings.TrimSpace(req.CommandID)
	req.CommandDigest = strings.TrimSpace(req.CommandDigest)
	req.TrajectoryID = strings.TrimSpace(req.TrajectoryID)
	if req.PacketSpec != nil {
		spec := *req.PacketSpec
		spec.TargetAgentID = strings.TrimSpace(spec.TargetAgentID)
		spec.TrajectoryID = strings.TrimSpace(spec.TrajectoryID)
		spec.ChannelID = strings.TrimSpace(spec.ChannelID)
		spec.WorkItemID = strings.TrimSpace(spec.WorkItemID)
		spec.Direction = types.LifecyclePacketDirection(strings.TrimSpace(string(spec.Direction)))
		if spec.Direction == "" {
			spec.Direction = types.LifecyclePacketDirectionDirective
		}
		if spec.Direction == types.LifecyclePacketDirectionProducerReport {
			spec.WorkDisposition, err = normalizeUpdateWorkDisposition(spec.WorkDisposition)
			if err != nil {
				return types.LifecycleResult{}, err
			}
		}
		req.PacketSpec = &spec
	}
	if req.CommandID == "" {
		return types.LifecycleResult{}, fmt.Errorf("commit lifecycle act: command_id is required")
	}
	if err := validateCommitLifecycleActShape(req); err != nil {
		return types.LifecycleResult{}, err
	}
	computed, digestErr := ComputeCommitLifecycleActDigest(req)
	if err := requireLifecycleDigest(req.CommandDigest, computed, digestErr); err != nil {
		return types.LifecycleResult{}, err
	}

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, replayErr := s.replayLifecycleCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest); found || replayErr != nil {
		return replay, replayErr
	}
	if req.PacketSpec != nil && req.PacketSpec.Direction == types.LifecyclePacketDirectionProducerReport {
		return s.commitLifecycleProducerReportAct(ctx, req, ownerID, computerID)
	}

	// Caller pins: the issuing agent object is the authority surface; a live
	// trajectory pin applies only when the caller is trajectory-scoped.
	callerAgentObj, callerAgent, err := s.textureTurnAgentObject(ctx, ownerID, computerID, req.CallerAgentID)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("commit lifecycle act caller agent %s: %w", req.CallerAgentID, err)
	}
	if callerAgent.LifecycleVersion != req.ExpectedCallerLifecycleVersion ||
		callerAgent.OwnerID != ownerID || callerAgent.ComputerID != computerID {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: callerAgentObj.CanonicalID, Exists: true, ExpectedContentHash: callerAgentObj.ContentHash},
	}
	if _, _, runErr := s.textureTurnRunObject(ctx, ownerID, computerID, req.CallerRunID); runErr != nil {
		return types.LifecycleResult{}, fmt.Errorf("commit lifecycle act caller run %s: %w", req.CallerRunID, runErr)
	}
	var trajectory types.TrajectoryRecord
	var trajectoryObj objectgraph.Object
	if req.TrajectoryID != "" {
		trajectoryObj, trajectory, err = s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
		if err != nil {
			return types.LifecycleResult{}, fmt.Errorf("commit lifecycle act trajectory %s: %w", req.TrajectoryID, err)
		}
		if trajectory.Status != types.TrajectoryLive || trajectory.LifecycleVersion != req.ExpectedLifecycleVersion ||
			trajectory.OwnerID != ownerID || trajectory.ComputerID != computerID {
			return types.LifecycleResult{}, ErrConcurrentStateChange
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash})
	}

	now := time.Now().UTC()
	seq := trajectory.ReducerSeq
	var objects []objectgraph.Object
	var edges []objectgraph.Edge
	var events []types.LifecycleEvent
	var eventObjs []objectgraph.Object

	// 1. The commitment record object. Idempotent mint: the deterministic
	// canonical id + not-exists condition makes a replayed act mint nothing.
	rec := req.Record
	recordObj, err := lifecycleObject(ogKindCommitmentRecord, ownerID, computerID, rec.RecordID, rec, map[string]any{
		"record_id":   rec.RecordID,
		"agent_id":    rec.Provenance.AgentID,
		"model_id":    rec.Provenance.ModelID,
		"discrepancy": string(rec.Discrepancy),
		"schema_id":   rec.SchemaID,
		"record_kind": string(rec.RecordKind()),
		"addressee":   strings.TrimSpace(rec.Addressee),
		"created_at":  now.UTC().Format(time.RFC3339Nano),
		"updated_at":  now.UTC().Format(time.RFC3339Nano),
	}, now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: recordObj.CanonicalID})
	objects = append(objects, recordObj)

	var packet types.CoagentSourcePacket
	if spec := req.PacketSpec; spec != nil {
		targetAgentObj, targetAgent, agentErr := s.textureTurnAgentObject(ctx, ownerID, computerID, strings.TrimSpace(spec.TargetAgentID))
		if agentErr != nil {
			return types.LifecycleResult{}, fmt.Errorf("commit lifecycle act target agent %s: %w", spec.TargetAgentID, agentErr)
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: targetAgentObj.CanonicalID, Exists: true, ExpectedContentHash: targetAgentObj.ContentHash})
		channelID := strings.TrimSpace(spec.ChannelID)
		if channelID == "" {
			channelID = strings.TrimSpace(targetAgent.ChannelID)
		}
		// Delivery trajectory is the target's scope: the explicit spec value
		// wins; otherwise resolve through the target desk's channel document.
		// Computer-scoped targets (persistent Management) carry "".
		targetTrajectory := strings.TrimSpace(spec.TrajectoryID)
		if targetTrajectory == "" && channelID != "" {
			if _, targetDoc, docErr := s.textureTurnDocumentObject(ctx, ownerID, computerID, channelID); docErr == nil {
				targetTrajectory = strings.TrimSpace(targetDoc.TrajectoryID)
			}
		}
		if channelID != "" && strings.TrimSpace(targetAgent.ChannelID) != "" && strings.TrimSpace(targetAgent.ChannelID) != channelID {
			return types.LifecycleResult{}, fmt.Errorf("commit lifecycle act: packet channel %q mismatches target channel %q: %w", spec.ChannelID, targetAgent.ChannelID, ErrLifecycleInvalidTransition)
		}
		seq++
		packetID := rec.RecordID + ":packet"
		updateKey := targetTrajectory + "\x00" + targetAgent.AgentID + "\x00" + req.CallerAgentID + "\x00" + packetID
		packet = types.CoagentSourcePacket{
			UpdateID: packetID, ProducerUpdateID: packetID, OwnerID: ownerID, ComputerID: computerID,
			AgentID: req.CallerAgentID, TargetAgentID: targetAgent.AgentID, ChannelID: channelID,
			MessageSeq: seq, TrajectoryID: targetTrajectory, Direction: types.LifecyclePacketDirectionDirective,
			Role: callerAgent.Role, SourceRunID: req.CallerRunID, SourceRecordID: rec.RecordID,
			PayloadDigest: spec.PayloadDigest, Disposition: types.UpdatePending,
			LifecycleVersion: 1, ReducerSeq: seq, Packet: spec.Packet, Content: spec.Content,
			CreatedAt: now,
		}
		updateMeta := lifecycleMetadata("update_id", packet.UpdateID, computerID, targetTrajectory, seq)
		updateMeta["producer_update_id"] = packet.ProducerUpdateID
		updateMeta["target_agent_id"] = packet.TargetAgentID
		updateMeta["direction"] = string(packet.Direction)
		updateMeta["record_id"] = rec.RecordID
		packetObj, objErr := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, updateKey, packet, updateMeta, now, now)
		if objErr != nil {
			return types.LifecycleResult{}, objErr
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: packetObj.CanonicalID})
		objects = append(objects, packetObj)
		edges = append(edges, objectgraph.Edge{
			EdgeID:    "packet-record:" + packetObj.CanonicalID,
			FromID:    packetObj.CanonicalID,
			ToID:      recordObj.CanonicalID,
			Kind:      ogEdgeRecordPacket,
			Metadata:  []byte(`{}`),
			CreatedAt: now,
		})
		seq++
		event := types.LifecycleEvent{
			EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID,
			TrajectoryID: targetTrajectory, UpdateID: packet.UpdateID,
			Kind: types.LifecycleControlQueued, ReducerVersion: types.LifecycleReducerVersion,
			ReducerSeq: seq, CommandID: req.CommandID, CommandDigest: req.CommandDigest,
			Reason: "directive packet derived from " + rec.RecordID, CreatedAt: now,
		}
		eventObj, eventErr := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event,
			lifecycleMetadata("event_id", event.EventID, computerID, targetTrajectory, seq), now, now)
		if eventErr != nil {
			return types.LifecycleResult{}, eventErr
		}
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
		objects = append(objects, eventObj)
		events = append(events, event)
		eventObjs = append(eventObjs, eventObj)
	}

	// Caller version bump mirrors IssueLifecycleControl: the act consumes one
	// caller lifecycle version so a replayed cell cannot mint divergent acts
	// under one activation identity.
	callerAgent.LastReducerSeq, callerAgent.LifecycleVersion, callerAgent.UpdatedAt = seq, callerAgent.LifecycleVersion+1, now
	callerAgentUpdated, err := lifecycleObject(ogKindAgent, ownerID, computerID, callerAgent.AgentID, callerAgent,
		lifecycleMetadata("agent_id", callerAgent.AgentID, computerID, req.TrajectoryID, seq), callerAgentObj.CreatedAt, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	objects = append(objects, callerAgentUpdated)
	if req.TrajectoryID != "" {
		trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = seq, trajectory.LifecycleVersion+1, now
		trajectoryUpdated, trajErr := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory,
			lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, seq), trajectoryObj.CreatedAt, now)
		if trajErr != nil {
			return types.LifecycleResult{}, trajErr
		}
		objects = append(objects, trajectoryUpdated)
	}

	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleCommitAct, seq, eventObjs)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, receiptObj)

	result := types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Agent: &callerAgent, Events: events, RecordCanonicalID: recordObj.CanonicalID}
	if packet.UpdateID != "" {
		result.Update = &packet
		result.Controls = []types.CoagentSourcePacket{packet}
	}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, result, edges...)
}

// commitLifecycleProducerReportAct mints a report record and its upward
// producer packet as one transition. Its authority and late-evidence rules
// deliberately match QueueLifecycleUpdate: the record-native cutover changes
// the transaction boundary, not who may report or how terminal trajectories
// retain historical evidence.
func (s *Store) commitLifecycleProducerReportAct(ctx context.Context, req types.CommitLifecycleActRequest, ownerID, computerID string) (types.LifecycleResult, error) {
	spec := req.PacketSpec
	if spec == nil || req.TrajectoryID == "" {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	trajectoryObj, trajectory, err := s.lifecycleTrajectoryObject(ctx, ownerID, computerID, req.TrajectoryID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	lateEvidenceOnly := trajectory.Status != types.TrajectoryLive
	documentID := strings.TrimSpace(trajectory.SubjectRefs["doc_id"])
	channelID := strings.TrimSpace(spec.ChannelID)
	if documentID == "" || channelID != documentID ||
		(strings.TrimSpace(spec.TrajectoryID) != "" && strings.TrimSpace(spec.TrajectoryID) != req.TrajectoryID) ||
		strings.TrimSpace(spec.TargetAgentID) != "texture:"+documentID {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	documentObj, err := s.lifecycleGetObject(ctx, ogKindTexDoc, ownerID, computerID, documentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	document, err := decodeLifecycleObject[types.Document](documentObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if document.DocID != documentID || document.OwnerID != ownerID || document.ComputerID != computerID || document.TrajectoryID != req.TrajectoryID {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	targetAgentObj, targetAgent, err := s.textureTurnAgentObject(ctx, ownerID, computerID, spec.TargetAgentID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if targetAgent.AgentID != spec.TargetAgentID || targetAgent.OwnerID != ownerID || targetAgent.ComputerID != computerID ||
		targetAgent.LifecycleVersion <= 0 || targetAgent.Profile != "texture" || targetAgent.Role != "texture" || targetAgent.ChannelID != documentID {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	callerAgentObj, callerAgent, err := s.textureTurnAgentObject(ctx, ownerID, computerID, req.CallerAgentID)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("commit lifecycle act caller agent %s: %w", req.CallerAgentID, err)
	}
	producerRunObj, err := s.lifecycleGetObject(ctx, ogKindRun, ownerID, computerID, req.CallerRunID)
	if err != nil {
		return types.LifecycleResult{}, fmt.Errorf("commit lifecycle act caller run %s: %w", req.CallerRunID, err)
	}
	producerRun, err := decodeLifecycleObject[types.RunRecord](producerRunObj)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	boundWorkItemIDs, bindingErr := lifecycleActivationWorkItemIDs(producerRun.Metadata)
	if bindingErr != nil || producerRun.RunID != req.CallerRunID || producerRun.OwnerID != ownerID ||
		producerRun.ComputerID != computerID || producerRun.TrajectoryID != req.TrajectoryID ||
		producerRun.AgentID != req.CallerAgentID || strings.TrimSpace(producerRun.AgentProfile) == "" ||
		strings.TrimSpace(producerRun.AgentProfile) != strings.TrimSpace(producerRun.AgentRole) ||
		strings.TrimSpace(producerRun.AgentRole) != strings.TrimSpace(callerAgent.Profile) ||
		strings.TrimSpace(producerRun.ChannelID) != documentID || !producerRun.State.Valid() ||
		!containsLifecycleIdentity(boundWorkItemIDs, spec.WorkItemID) {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	workObj, work, err := s.lifecycleWorkObject(ctx, ownerID, computerID, spec.WorkItemID)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if work.TrajectoryID != req.TrajectoryID || work.AssignedAgentID != req.CallerAgentID ||
		work.AuthorityProfile != producerRun.AgentProfile {
		return types.LifecycleResult{}, ErrLifecycleInvalidTransition
	}
	if !lateEvidenceOnly && (trajectory.LifecycleVersion != req.ExpectedLifecycleVersion ||
		callerAgent.LifecycleVersion != req.ExpectedCallerLifecycleVersion || work.Status != types.WorkItemOpen) {
		return types.LifecycleResult{}, ErrConcurrentStateChange
	}

	packetID := req.Record.RecordID + ":packet"
	updateKey := req.TrajectoryID + "\x00" + spec.TargetAgentID + "\x00" + req.CallerAgentID + "\x00" + packetID
	updateCanonicalID, err := lifecycleCanonicalID(ogKindWorkerUpdate, ownerID, computerID, updateKey)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	if existing, getErr := s.lifecycleGraph().GetObject(ctx, updateCanonicalID); getErr == nil {
		stored, decodeErr := decodeLifecycleObject[types.CoagentSourcePacket](existing)
		if decodeErr != nil {
			return types.LifecycleResult{}, decodeErr
		}
		if stored.UpdateID != packetID || stored.ProducerUpdateID != packetID ||
			stored.Direction != types.LifecyclePacketDirectionProducerReport ||
			stored.SourceRecordID != req.Record.RecordID || stored.PayloadDigest != spec.PayloadDigest ||
			stored.ProducerWorkItemID != spec.WorkItemID || stored.WorkItemID != spec.WorkItemID ||
			stored.WorkDisposition != spec.WorkDisposition {
			return types.LifecycleResult{}, ErrLifecycleCommandConflict
		}
		return types.LifecycleResult{Trajectory: trajectory, Agent: &callerAgent, Update: &stored, Replay: true}, nil
	} else if !errors.Is(getErr, objectgraph.ErrNotFound) {
		return types.LifecycleResult{}, getErr
	}

	now := time.Now().UTC()
	seq := trajectory.ReducerSeq
	conditions := []objectgraph.ObjectCondition{
		{CanonicalID: trajectoryObj.CanonicalID, Exists: true, ExpectedContentHash: trajectoryObj.ContentHash},
		{CanonicalID: documentObj.CanonicalID, Exists: true, ExpectedContentHash: documentObj.ContentHash},
		{CanonicalID: targetAgentObj.CanonicalID, Exists: true, ExpectedContentHash: targetAgentObj.ContentHash},
		{CanonicalID: producerRunObj.CanonicalID, Exists: true, ExpectedContentHash: producerRunObj.ContentHash},
		{CanonicalID: workObj.CanonicalID, Exists: true, ExpectedContentHash: workObj.ContentHash},
	}
	if !lateEvidenceOnly {
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: callerAgentObj.CanonicalID, Exists: true, ExpectedContentHash: callerAgentObj.ContentHash})
	}
	rec := req.Record
	recordObj, err := lifecycleObject(ogKindCommitmentRecord, ownerID, computerID, rec.RecordID, rec, map[string]any{
		"record_id": rec.RecordID, "agent_id": rec.Provenance.AgentID, "model_id": rec.Provenance.ModelID,
		"discrepancy": string(rec.Discrepancy), "schema_id": rec.SchemaID, "record_kind": string(rec.RecordKind()),
		"addressee": strings.TrimSpace(rec.Addressee), "created_at": now.UTC().Format(time.RFC3339Nano), "updated_at": now.UTC().Format(time.RFC3339Nano),
	}, now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: recordObj.CanonicalID})

	disposition, dispositionRef, dispositionReason := types.UpdatePending, "", ""
	var sequenceUpdated objectgraph.Object
	if lateEvidenceOnly {
		var sequenceCondition objectgraph.ObjectCondition
		seq, sequenceUpdated, sequenceCondition, err = s.nextPostTerminalSequence(ctx, ownerID, computerID, trajectory, now)
		if err != nil {
			return types.LifecycleResult{}, err
		}
		conditions = append(conditions, sequenceCondition)
		disposition, dispositionRef, dispositionReason = types.UpdateLate, lifecycleTerminalTrajectoryRef(req.TrajectoryID), "trajectory is terminal"
	} else {
		seq++
	}
	packet := types.CoagentSourcePacket{
		UpdateID: packetID, ProducerUpdateID: packetID, OwnerID: ownerID, ComputerID: computerID,
		AgentID: req.CallerAgentID, TargetAgentID: spec.TargetAgentID, ChannelID: channelID,
		MessageSeq: seq, TrajectoryID: req.TrajectoryID, Direction: types.LifecyclePacketDirectionProducerReport,
		ProducerWorkItemID: spec.WorkItemID, WorkItemID: spec.WorkItemID, WorkDisposition: spec.WorkDisposition,
		Role: producerRun.AgentRole, SourceRunID: req.CallerRunID, SourceRecordID: rec.RecordID,
		PayloadDigest: spec.PayloadDigest, Disposition: disposition, DispositionRef: dispositionRef, DispositionReason: dispositionReason,
		LifecycleVersion: 1, ReducerSeq: seq, Packet: spec.Packet, Content: spec.Content, CreatedAt: now,
	}
	updateMeta := lifecycleMetadata("update_id", packet.UpdateID, computerID, req.TrajectoryID, seq)
	updateMeta["producer_update_id"], updateMeta["target_agent_id"], updateMeta["direction"], updateMeta["record_id"] = packet.ProducerUpdateID, packet.TargetAgentID, string(packet.Direction), rec.RecordID
	packetObj, err := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, updateKey, packet, updateMeta, now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: packetObj.CanonicalID})
	eventKind := types.LifecycleUpdateQueued
	if lateEvidenceOnly {
		eventKind = types.LifecycleUpdateLate
	}
	event := types.LifecycleEvent{
		EventID: req.CommandID + ":1", OwnerID: ownerID, ComputerID: computerID, TrajectoryID: req.TrajectoryID,
		UpdateID: packet.UpdateID, Kind: eventKind, ReducerVersion: types.LifecycleReducerVersion, ReducerSeq: seq,
		CommandID: req.CommandID, CommandDigest: req.CommandDigest, Reason: dispositionReason, CreatedAt: now,
	}
	eventObj, err := lifecycleObject(ogKindLifecycleEvent, ownerID, computerID, event.EventID, event,
		lifecycleMetadata("event_id", event.EventID, computerID, req.TrajectoryID, seq), now, now)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: eventObj.CanonicalID})
	objects := []objectgraph.Object{recordObj, packetObj, eventObj}
	if lateEvidenceOnly {
		objects = append(objects, sequenceUpdated)
	} else {
		trajectory.ReducerSeq, trajectory.LifecycleVersion, trajectory.UpdatedAt = seq, trajectory.LifecycleVersion+1, now
		targetAgent.LastReducerSeq, targetAgent.LifecycleVersion, targetAgent.UpdatedAt = seq, targetAgent.LifecycleVersion+1, now
		callerAgent.LastReducerSeq, callerAgent.LifecycleVersion, callerAgent.UpdatedAt = seq, callerAgent.LifecycleVersion+1, now
		trajectoryUpdated, buildErr := lifecycleObject(ogKindTrajectory, ownerID, computerID, req.TrajectoryID, trajectory,
			lifecycleMetadata("trajectory_id", req.TrajectoryID, computerID, req.TrajectoryID, seq), trajectoryObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		targetUpdated, buildErr := lifecycleObject(ogKindAgent, ownerID, computerID, targetAgent.AgentID, targetAgent,
			lifecycleMetadata("agent_id", targetAgent.AgentID, computerID, req.TrajectoryID, seq), targetAgentObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		callerUpdated, buildErr := lifecycleObject(ogKindAgent, ownerID, computerID, callerAgent.AgentID, callerAgent,
			lifecycleMetadata("agent_id", callerAgent.AgentID, computerID, req.TrajectoryID, seq), callerAgentObj.CreatedAt, now)
		if buildErr != nil {
			return types.LifecycleResult{}, buildErr
		}
		objects = append(objects, trajectoryUpdated, targetUpdated, callerUpdated)
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(now, ownerID, computerID, req.TrajectoryID, req.CommandID, req.CommandDigest, types.LifecycleCommitAct, seq, []objectgraph.Object{eventObj})
	if err != nil {
		return types.LifecycleResult{}, err
	}
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: receiptObj.CanonicalID})
	objects = append(objects, receiptObj)
	edge := objectgraph.Edge{EdgeID: "packet-record:" + packetObj.CanonicalID, FromID: packetObj.CanonicalID, ToID: recordObj.CanonicalID, Kind: ogEdgeRecordPacket, Metadata: []byte(`{}`), CreatedAt: now}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects,
		types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Agent: &callerAgent, Update: &packet, Controls: []types.CoagentSourcePacket{packet}, Events: []types.LifecycleEvent{event}, RecordCanonicalID: recordObj.CanonicalID}, edge)
}
