package store

import (
	"context"
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
	if spec := req.PacketSpec; spec != nil {
		if strings.TrimSpace(spec.TargetAgentID) == "" {
			return fmt.Errorf("commit lifecycle act: packet target_agent_id is required")
		}
		if addressee := strings.TrimSpace(rec.Addressee); addressee != "" {
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

	result := types.LifecycleResult{Receipt: receipt, Trajectory: trajectory, Agent: &callerAgent, Events: events}
	if packet.UpdateID != "" {
		result.Update = &packet
		result.Controls = []types.CoagentSourcePacket{packet}
	}
	return s.commitLifecycleTransition(ctx, ownerID, computerID, req.CommandID, req.CommandDigest, conditions, objects, result, edges...)
}
