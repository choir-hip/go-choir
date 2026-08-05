package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	runMetadataAssignmentID = "supervision_assignment_id"
	runMetadataAttemptID    = "supervision_attempt_id"
	runMetadataObservedBase = "supervision_observed_base"
)

func supervisedCoSuperAgentID(assignmentID string) string {
	return "cosuper:assignment:" + strings.TrimSpace(assignmentID)
}

func supervisionObservedBase(snapshot types.SupervisionProjectionSnapshot) (computerevent.SupervisionObservedBase, error) {
	base := computerevent.SupervisionObservedBase{CanonicalEventHead: snapshot.ObservedCanonicalEventHead, IntentRevisionID: snapshot.IntentRevisionID, ArtifactHeadRevisionID: snapshot.ArtifactHeadRevisionID}
	if strings.TrimSpace(base.CanonicalEventHead) == "" || strings.TrimSpace(base.IntentRevisionID) == "" || strings.TrimSpace(base.ArtifactHeadRevisionID) == "" {
		return computerevent.SupervisionObservedBase{}, fmt.Errorf("supervision projection lacks observed base")
	}
	return base, nil
}

func supervisionExpected(snapshot types.SupervisionProjectionSnapshot) computerevent.SupervisionExpected {
	head, lifecycle, intent, artifact := snapshot.CanonicalEventHead, uint64(snapshot.LifecycleVersion), snapshot.IntentRevisionID, snapshot.ArtifactHeadRevisionID
	return computerevent.SupervisionExpected{CanonicalEventHead: &head, LifecycleVersion: &lifecycle, IntentRevisionID: &intent, ArtifactHeadRevisionID: &artifact}
}

func storedSupervisionObservedBase(metadata map[string]any, fallback computerevent.SupervisionObservedBase) computerevent.SupervisionObservedBase {
	values, ok := metadata[runMetadataObservedBase].(map[string]string)
	if !ok {
		return fallback
	}
	base := computerevent.SupervisionObservedBase{CanonicalEventHead: values["canonical_event_head"], IntentRevisionID: values["intent_revision_id"], ArtifactHeadRevisionID: values["artifact_head_revision_id"]}
	if strings.TrimSpace(base.CanonicalEventHead) == "" || strings.TrimSpace(base.IntentRevisionID) == "" || strings.TrimSpace(base.ArtifactHeadRevisionID) == "" {
		return fallback
	}
	return base
}

func (rt *Runtime) supervisionSnapshotForRun(ctx context.Context, rec *types.RunRecord) (types.SupervisionProjectionSnapshot, bool, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return types.SupervisionProjectionSnapshot{}, false, nil
	}
	snapshot, err := rt.store.GetSupervisionProjectionSnapshot(ctx, rec.OwnerID, rec.SandboxID, trajectoryIDForRun(rec))
	if err == nil {
		return snapshot, true, nil
	}
	if err == store.ErrNotFound {
		return types.SupervisionProjectionSnapshot{}, false, nil
	}
	return types.SupervisionProjectionSnapshot{}, false, fmt.Errorf("load supervision projection: %w", err)
}

func (rt *Runtime) appendSupervisionAttemptStart(ctx context.Context, requester, child *types.RunRecord, snapshot types.SupervisionProjectionSnapshot) (computerevent.SupervisionObservedBase, error) {
	assignmentID, attemptID := strings.TrimSpace(metadataStringValue(child.Metadata, runMetadataAssignmentID)), strings.TrimSpace(metadataStringValue(child.Metadata, runMetadataAttemptID))
	if assignmentID == "" || attemptID == "" {
		return computerevent.SupervisionObservedBase{}, fmt.Errorf("supervised co-super spawn requires assignment_id and attempt_id")
	}
	lineage, err := rt.store.GetSupervisionAssignmentLineage(ctx, child.OwnerID, child.SandboxID, trajectoryIDForRun(child), assignmentID)
	if err != nil {
		return computerevent.SupervisionObservedBase{}, fmt.Errorf("load supervision assignment lineage: %w", err)
	}
	if lineage.AssignedActorID != agentIDForRun(child) || lineage.AssignedRole != agentprofile.CoSuper || lineage.Status != "open" {
		return computerevent.SupervisionObservedBase{}, fmt.Errorf("supervision assignment %q is not authorized for co-super %s", assignmentID, agentIDForRun(child))
	}
	base, err := supervisionObservedBase(snapshot)
	if err != nil {
		return computerevent.SupervisionObservedBase{}, err
	}
	commandID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:supervision:attempt:"+child.RunID+":"+attemptID)).String()
	binding, err := computerevent.CanonicalJSON(map[string]string{"schema": "choir.supervision_attempt_runtime_binding.v1", "run_id": child.RunID, "assignment_id": assignmentID, "attempt_id": attemptID})
	if err != nil {
		return computerevent.SupervisionObservedBase{}, err
	}
	bindingID := commandID + ":runtime"
	runtimeRef := computerevent.SupervisionArtifactPlaceholder(bindingID)
	attemptKind, ordinal, priorAttemptID := supervisionAttemptShape(lineage.Attempts, attemptID)
	body, err := json.Marshal(map[string]any{
		"assignment_id": assignmentID, "attempt_id": attemptID, "attempt_kind": attemptKind, "ordinal": ordinal,
		"prior_attempt_id": priorAttemptID, "run_id": child.RunID, "observed_base": base, "runtime_receipt_ref": runtimeRef,
	})
	if err != nil {
		return computerevent.SupervisionObservedBase{}, err
	}
	transaction := computerevent.SupervisionTransaction{Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: commandID, TransactionClass: "start_attempt", OwnerID: child.OwnerID, ComputerID: child.SandboxID, TrajectoryID: trajectoryIDForRun(child), CommandID: commandID, Actor: computerevent.SupervisionActor{ActorID: agentIDForRun(requester), Role: "super", AuthorityRef: "run:" + requester.RunID}, Expected: supervisionExpected(snapshot), ObservedBase: &base, Mutations: []computerevent.SupervisionMutation{{Kind: "attempt_started", Body: body}}}
	if _, _, _, err := rt.AppendSupervisionTransactionWithPrivateArtifacts(ctx, transaction, []computerevent.PrivateSupervisionArtifactPayload{{
		BindingID: bindingID, Plaintext: binding, MediaType: computerevent.SupervisionEvidenceMediaTypeV1,
	}}); err != nil {
		return computerevent.SupervisionObservedBase{}, fmt.Errorf("append supervised attempt start: %w", err)
	}
	return base, nil
}

func supervisionAttemptShape(attempts []store.SupervisionAttemptLineage, attemptID string) (string, uint64, *string) {
	for _, attempt := range attempts {
		if attempt.AttemptID != attemptID {
			continue
		}
		if attempt.PriorAttempt == "" {
			return attempt.AttemptKind, attempt.Ordinal, nil
		}
		prior := attempt.PriorAttempt
		return attempt.AttemptKind, attempt.Ordinal, &prior
	}
	if len(attempts) == 0 {
		return "initial", 1, nil
	}
	previous := attempts[len(attempts)-1]
	prior := previous.AttemptID
	return "retry", previous.Ordinal + 1, &prior
}

func (rt *Runtime) recoverSupervisedUpdate(ctx context.Context, rec *types.RunRecord, packet types.CoagentSourcePacketPayload, commandID, targetAgentID, channelID string) (types.CoagentSourcePacket, bool, error) {
	if rt == nil || rt.eventAppender == nil || rt.store == nil || rec == nil {
		return types.CoagentSourcePacket{}, false, nil
	}
	_, transaction, found, err := rt.eventAppender.RecoverFinalizedSupervisionTransaction(ctx, commandID)
	if err != nil || !found {
		return types.CoagentSourcePacket{}, found, err
	}
	profile := agentprofile.Canonical(agentProfileForRun(rec))
	expectedActorRole := profile
	if profile == agentprofile.CoSuper {
		expectedActorRole = "cosuper"
	}
	if transaction.OwnerID != rec.OwnerID || transaction.ComputerID != rec.SandboxID ||
		transaction.TrajectoryID != trajectoryIDForRun(rec) ||
		transaction.Actor.ActorID != agentIDForRun(rec) ||
		transaction.Actor.Role != expectedActorRole ||
		transaction.Actor.AuthorityRef != "run:"+rec.RunID {
		return types.CoagentSourcePacket{}, true, fmt.Errorf("%w: finalized supervised update scope mismatch", computerevent.ErrSupervisionIdempotencyConflict)
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, rec.OwnerID, rec.SandboxID, trajectoryIDForRun(rec))
	if err != nil {
		return types.CoagentSourcePacket{}, true, fmt.Errorf("recover supervised update trajectory: %w", err)
	}
	deliveries, err := rt.store.ListSupervisionDeliveryEvents(ctx, rec.OwnerID, rec.SandboxID, trajectory.TrajectoryID)
	if err != nil {
		return types.CoagentSourcePacket{}, true, fmt.Errorf("recover supervised update delivery: %w", err)
	}
	expectedPacket, err := computerevent.CanonicalJSON(normalizeCoagentSourcePacketPayload(packet))
	if err != nil {
		return types.CoagentSourcePacket{}, true, err
	}
	for _, delivery := range deliveries {
		if delivery.CommandID != commandID {
			continue
		}
		canonicalTarget := strings.TrimSpace(targetAgentID)
		if canonicalTarget == "" {
			switch delivery.Kind {
			case "attempt_result":
				canonicalTarget = persistentSuperAgentID(rec.OwnerID)
			case "researcher_packet_recorded":
				canonicalTarget = textureAgentIDForTrajectory(trajectory)
			case "actor_message_recorded":
				var body canonicalSupervisionDeliveryBody
				if err := json.Unmarshal(delivery.Body, &body); err != nil || body.ToActorID == nil {
					return types.CoagentSourcePacket{}, true, fmt.Errorf("%w: finalized supervised update recipient is not explicit", computerevent.ErrNeedsProjectionRepair)
				}
				canonicalTarget = strings.TrimSpace(*body.ToActorID)
			}
		}
		update, addressed, err := rt.canonicalSupervisionDeliveryUpdate(ctx, trajectory, canonicalTarget, delivery)
		if err != nil {
			return types.CoagentSourcePacket{}, true, err
		}
		if !addressed || update.AgentID != agentIDForRun(rec) {
			return types.CoagentSourcePacket{}, true, fmt.Errorf("%w: finalized supervised update recipient mismatch", computerevent.ErrSupervisionIdempotencyConflict)
		}
		if targetAgentID != "" && update.TargetAgentID != strings.TrimSpace(targetAgentID) {
			return types.CoagentSourcePacket{}, true, fmt.Errorf("%w: finalized supervised update recipient mismatch", computerevent.ErrSupervisionIdempotencyConflict)
		}
		if transaction.TransactionClass == "record_message" && channelID != "" && update.ChannelID != strings.TrimSpace(channelID) {
			return types.CoagentSourcePacket{}, true, fmt.Errorf("%w: finalized supervised update channel mismatch", computerevent.ErrSupervisionIdempotencyConflict)
		}
		acceptedPacket, err := computerevent.CanonicalJSON(normalizeCoagentSourcePacketPayload(update.Packet))
		if err != nil {
			return types.CoagentSourcePacket{}, true, err
		}
		if string(acceptedPacket) != string(expectedPacket) {
			return types.CoagentSourcePacket{}, true, fmt.Errorf("%w: finalized supervised update payload mismatch", computerevent.ErrSupervisionIdempotencyConflict)
		}
		return update, true, nil
	}
	return types.CoagentSourcePacket{}, true, fmt.Errorf("%w: finalized supervised update has no delivery", computerevent.ErrNeedsProjectionRepair)
}

func (rt *Runtime) appendSupervisedUpdate(ctx context.Context, rec *types.RunRecord, packet types.CoagentSourcePacketPayload, commandID, targetAgentID, channelID string) (string, error) {
	if recovered, found, err := rt.recoverSupervisedUpdate(ctx, rec, packet, commandID, targetAgentID, channelID); err != nil {
		return "", err
	} else if found {
		if err := rt.dispatchSupervisionUpdate(ctx, rec, recovered.TargetAgentID, recovered.UpdateID); err != nil {
			return "", err
		}
		return recovered.UpdateID, nil
	}
	snapshot, supervised, err := rt.supervisionSnapshotForRun(ctx, rec)
	if err != nil || !supervised {
		return "", err
	}
	currentBase, err := supervisionObservedBase(snapshot)
	if err != nil {
		return "", err
	}
	observedBase := storedSupervisionObservedBase(rec.Metadata, currentBase)
	packetBytes, err := computerevent.CanonicalJSON(packet)
	if err != nil {
		return "", err
	}
	profile := agentprofile.Canonical(agentProfileForRun(rec))
	transaction := computerevent.SupervisionTransaction{Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: commandID, OwnerID: rec.OwnerID, ComputerID: rec.SandboxID, TrajectoryID: trajectoryIDForRun(rec), CommandID: commandID, Expected: supervisionExpected(snapshot), ObservedBase: &observedBase}
	switch profile {
	case agentprofile.CoSuper:
		canonicalTargetAgentID := persistentSuperAgentID(rec.OwnerID)
		if strings.TrimSpace(targetAgentID) != canonicalTargetAgentID {
			return "", fmt.Errorf("supervised CoSuper result must target persistent Super %q", canonicalTargetAgentID)
		}
		targetAgentID = canonicalTargetAgentID
		assignmentID, attemptID := strings.TrimSpace(metadataStringValue(rec.Metadata, runMetadataAssignmentID)), strings.TrimSpace(metadataStringValue(rec.Metadata, runMetadataAttemptID))
		lineage, err := rt.store.GetSupervisionAssignmentLineage(ctx, rec.OwnerID, rec.SandboxID, trajectoryIDForRun(rec), assignmentID)
		if err != nil {
			return "", fmt.Errorf("load supervision assignment lineage: %w", err)
		}
		if lineage.AssignedActorID != agentIDForRun(rec) || lineage.AssignedRole != agentprofile.CoSuper {
			return "", fmt.Errorf("supervision assignment %q is not authorized for co-super %s", assignmentID, agentIDForRun(rec))
		}
		late, started := false, false
		for _, attempt := range lineage.Attempts {
			if attempt.AttemptID == attemptID {
				late, started = attempt.Status == "cancelled", true
				break
			}
		}
		if !started {
			return "", fmt.Errorf("supervision attempt %q is not canonical for assignment %q", attemptID, assignmentID)
		}
		resultID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:supervision:result:"+commandID)).String()
		bindingID := resultID + ":packet"
		packetRef := computerevent.SupervisionArtifactPlaceholder(bindingID)
		outcome := "succeeded"
		if packet.Kind == "blocker" {
			outcome = "blocked"
		}
		body, err := json.Marshal(map[string]any{"assignment_id": assignmentID, "attempt_id": attemptID, "result_id": resultID, "outcome": outcome, "result_artifact_ref": packetRef, "evidence_refs": []string{}, "observed_base": observedBase, "delivered_after_cancellation": late})
		if err != nil {
			return "", err
		}
		transaction.TransactionClass, transaction.Actor, transaction.Mutations = "return_result", computerevent.SupervisionActor{ActorID: agentIDForRun(rec), Role: "cosuper", AuthorityRef: "run:" + rec.RunID}, []computerevent.SupervisionMutation{{Kind: "attempt_result", Body: body}}
		if _, _, _, err := rt.AppendSupervisionTransactionWithPrivateArtifacts(ctx, transaction, []computerevent.PrivateSupervisionArtifactPayload{{BindingID: bindingID, Plaintext: packetBytes, MediaType: computerevent.SupervisionEvidenceMediaTypeV1}}); err != nil {
			return "", fmt.Errorf("append supervised attempt result: %w", err)
		}
		if err := rt.dispatchSupervisionUpdate(ctx, rec, targetAgentID, resultID); err != nil {
			return "", err
		}
		return resultID, nil
	case agentprofile.Researcher:
		trajectory, err := rt.store.GetLifecycleTrajectory(ctx, rec.OwnerID, rec.SandboxID, trajectoryIDForRun(rec))
		if err != nil {
			return "", fmt.Errorf("load supervised researcher trajectory: %w", err)
		}
		canonicalTargetAgentID := textureAgentIDForTrajectory(trajectory)
		if canonicalTargetAgentID == "" {
			return "", fmt.Errorf("supervised researcher trajectory has no Texture target")
		}
		if strings.TrimSpace(targetAgentID) != canonicalTargetAgentID {
			return "", fmt.Errorf("supervised researcher packet must target trajectory Texture %q", canonicalTargetAgentID)
		}
		targetAgentID = canonicalTargetAgentID
		packetID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:supervision:research:"+commandID)).String()
		bindingID := packetID + ":packet"
		packetRef := computerevent.SupervisionArtifactPlaceholder(bindingID)
		body, err := json.Marshal(map[string]any{"packet_id": packetID, "researcher_id": agentIDForRun(rec), "obligation_id": "run:" + rec.RunID, "packet_artifact_ref": packetRef, "source_artifact_refs": []string{}, "uncertainty_artifact_ref": packetRef, "conflict_refs": []string{}})
		if err != nil {
			return "", err
		}
		transaction.TransactionClass, transaction.Actor, transaction.Mutations = "record_research", computerevent.SupervisionActor{ActorID: agentIDForRun(rec), Role: "researcher", AuthorityRef: "run:" + rec.RunID}, []computerevent.SupervisionMutation{{Kind: "researcher_packet_recorded", Body: body}}
		if _, _, _, err := rt.AppendSupervisionTransactionWithPrivateArtifacts(ctx, transaction, []computerevent.PrivateSupervisionArtifactPayload{{BindingID: bindingID, Plaintext: packetBytes, MediaType: computerevent.SupervisionEvidenceMediaTypeV1}}); err != nil {
			return "", fmt.Errorf("append supervised researcher packet: %w", err)
		}
		if err := rt.dispatchSupervisionUpdate(ctx, rec, targetAgentID, packetID); err != nil {
			return "", err
		}
		return packetID, nil
	case agentprofile.Super, agentprofile.Texture:
		messageID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:supervision:message:"+commandID)).String()
		bindingID := messageID + ":packet"
		packetRef := computerevent.SupervisionArtifactPlaceholder(bindingID)
		toRole := ""
		if strings.TrimSpace(targetAgentID) == persistentSuperAgentID(rec.OwnerID) {
			toRole = agentprofile.Super
		} else {
			target, err := rt.store.GetAgentByScope(ctx, rec.OwnerID, rec.SandboxID, targetAgentID)
			if err != nil {
				return "", fmt.Errorf("resolve supervised message target: %w", err)
			}
			toRole = agentprofile.Canonical(firstNonEmpty(target.Profile, target.Role))
		}
		if !canonicalSupervisionTargetProfileSupported(toRole) {
			return "", fmt.Errorf("supervised message target profile %q cannot consume canonical packets", toRole)
		}
		var toActorID *string
		if targetAgentID = strings.TrimSpace(targetAgentID); targetAgentID != "" {
			toActorID = &targetAgentID
		}
		body, err := json.Marshal(map[string]any{"message_id": messageID, "from_actor_id": agentIDForRun(rec), "to_role": toRole, "to_actor_id": toActorID, "channel_id": channelID, "payload_artifact_ref": packetRef, "material": false})
		if err != nil {
			return "", err
		}
		transaction.TransactionClass, transaction.Actor, transaction.Mutations = "record_message", computerevent.SupervisionActor{ActorID: agentIDForRun(rec), Role: profile, AuthorityRef: "run:" + rec.RunID}, []computerevent.SupervisionMutation{{Kind: "actor_message_recorded", Body: body}}
		if _, _, _, err := rt.AppendSupervisionTransactionWithPrivateArtifacts(ctx, transaction, []computerevent.PrivateSupervisionArtifactPayload{{BindingID: bindingID, Plaintext: packetBytes, MediaType: computerevent.SupervisionEvidenceMediaTypeV1}}); err != nil {
			return "", fmt.Errorf("append supervised actor message: %w", err)
		}
		if err := rt.dispatchSupervisionUpdate(ctx, rec, targetAgentID, messageID); err != nil {
			return "", err
		}
		return messageID, nil
	default:
		return "", fmt.Errorf("supervised update_coagent is not available to %s", profile)
	}
}

func canonicalSupervisionTargetProfileSupported(profile string) bool {
	switch agentprofile.Canonical(profile) {
	case agentprofile.Super, agentprofile.Researcher, agentprofile.Texture:
		return true
	default:
		return false
	}
}

func (rt *Runtime) dispatchSupervisionUpdate(ctx context.Context, source *types.RunRecord, targetAgentID, recordID string) error {
	if rt == nil || source == nil || rt.dispatchActor == nil {
		return fmt.Errorf("dispatch canonical supervision update: actor runtime unavailable")
	}
	targetAgentID = strings.TrimSpace(targetAgentID)
	recordID = strings.TrimSpace(recordID)
	if targetAgentID == "" || recordID == "" {
		return fmt.Errorf("dispatch canonical supervision update: target and record identity are required")
	}
	if err := rt.dispatchActor(ctx, source.OwnerID, source.SandboxID, targetAgentID, "coagent_result", recordID, trajectoryIDForRun(source), agentIDForRun(source)); err != nil {
		rt.scheduleCanonicalSupervisionSweep(context.WithoutCancel(ctx))
		return fmt.Errorf("dispatch canonical supervision update: %w", err)
	}
	return nil
}

func (rt *Runtime) acknowledgeCanonicalSupervisionDeliveries(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rec == nil || metadataStringValue(rec.Metadata, "supervision_delivery_authority") != "canonical" {
		return nil
	}
	deliveryIDs := metadataStringSlice(rec.Metadata["supervision_delivery_ids"])
	if len(deliveryIDs) == 0 {
		return fmt.Errorf("canonical supervision completion has no delivery identities")
	}
	consumed, err := rt.consumedCanonicalSupervisionUpdateIDs(ctx, rec.OwnerID, rec.SandboxID, rec.AgentID)
	if err != nil {
		return err
	}
	pending, err := rt.canonicalSupervisionUpdatesForAgentWithConsumed(ctx, rec.OwnerID, rec.SandboxID, rec.AgentID, trajectoryIDForRun(rec), 0, consumed)
	if err != nil {
		return err
	}
	pendingIDs := make(map[string]bool, len(pending))
	for _, update := range pending {
		pendingIDs[strings.TrimSpace(update.UpdateID)] = true
	}
	unacknowledged := make([]string, 0, len(deliveryIDs))
	for _, deliveryID := range deliveryIDs {
		deliveryID = strings.TrimSpace(deliveryID)
		if deliveryID == "" || consumed[deliveryID] {
			continue
		}
		if !pendingIDs[deliveryID] {
			return fmt.Errorf("canonical supervision delivery %q is not pending for run %s", deliveryID, rec.RunID)
		}
		unacknowledged = append(unacknowledged, deliveryID)
	}
	if len(unacknowledged) == 0 {
		return nil
	}
	sort.Strings(unacknowledged)
	snapshot, err := rt.store.GetSupervisionProjectionSnapshot(ctx, rec.OwnerID, rec.SandboxID, trajectoryIDForRun(rec))
	if err != nil {
		return err
	}
	observedBase, err := supervisionObservedBase(snapshot)
	if err != nil {
		return err
	}
	mutations := make([]computerevent.SupervisionMutation, 0, len(unacknowledged))
	for _, deliveryID := range unacknowledged {
		body, err := json.Marshal(map[string]any{
			"message_id": deliveryID, "target_actor_id": rec.AgentID, "run_id": rec.RunID,
		})
		if err != nil {
			return err
		}
		mutations = append(mutations, computerevent.SupervisionMutation{Kind: "actor_message_acknowledged", Body: body})
	}
	commandID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join(append([]string{"choir:supervision:ack", rec.RunID}, unacknowledged...), "\x00"))).String()
	transaction := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1,
		DigestRecipe:  computerevent.SupervisionDigestRecipeV1,
		TransactionID: commandID, OwnerID: rec.OwnerID, ComputerID: rec.SandboxID,
		TrajectoryID: trajectoryIDForRun(rec), CommandID: commandID,
		Expected: supervisionExpected(snapshot), ObservedBase: &observedBase,
		TransactionClass: "acknowledge_message",
		Actor: computerevent.SupervisionActor{
			ActorID: rec.AgentID, Role: agentprofile.Canonical(agentProfileForRun(rec)),
			AuthorityRef: "run:" + rec.RunID,
		},
		Mutations: mutations,
	}
	if _, _, err := rt.AppendSupervisionTransaction(ctx, transaction); err != nil {
		return fmt.Errorf("acknowledge canonical supervision deliveries: %w", err)
	}
	return nil
}
