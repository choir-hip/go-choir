package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
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

func (rt *Runtime) appendSupervisedUpdate(ctx context.Context, rec *types.RunRecord, packet types.CoagentSourcePacketPayload, commandID, targetAgentID, channelID string) (string, error) {
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
	bindingID := commandID + ":packet"
	packetRef := computerevent.SupervisionArtifactPlaceholder(bindingID)
	profile := agentprofile.Canonical(agentProfileForRun(rec))
	transaction := computerevent.SupervisionTransaction{Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: commandID, OwnerID: rec.OwnerID, ComputerID: rec.SandboxID, TrajectoryID: trajectoryIDForRun(rec), CommandID: commandID, Expected: supervisionExpected(snapshot), ObservedBase: &observedBase}
	switch profile {
	case agentprofile.CoSuper:
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
		return resultID, nil
	case agentprofile.Researcher:
		packetID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:supervision:research:"+commandID)).String()
		body, err := json.Marshal(map[string]any{"packet_id": packetID, "researcher_id": agentIDForRun(rec), "obligation_id": "run:" + rec.RunID, "packet_artifact_ref": packetRef, "source_artifact_refs": []string{}, "uncertainty_artifact_ref": packetRef, "conflict_refs": []string{}})
		if err != nil {
			return "", err
		}
		transaction.TransactionClass, transaction.Actor, transaction.Mutations = "record_research", computerevent.SupervisionActor{ActorID: agentIDForRun(rec), Role: "researcher", AuthorityRef: "run:" + rec.RunID}, []computerevent.SupervisionMutation{{Kind: "researcher_packet_recorded", Body: body}}
		if _, _, _, err := rt.AppendSupervisionTransactionWithPrivateArtifacts(ctx, transaction, []computerevent.PrivateSupervisionArtifactPayload{{BindingID: bindingID, Plaintext: packetBytes, MediaType: computerevent.SupervisionEvidenceMediaTypeV1}}); err != nil {
			return "", fmt.Errorf("append supervised researcher packet: %w", err)
		}
		return packetID, nil
	case agentprofile.Super, agentprofile.Texture:
		messageID := uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:supervision:message:"+commandID)).String()
		toRole := "super"
		if target, err := rt.store.GetAgentByScope(ctx, rec.OwnerID, rec.SandboxID, targetAgentID); err == nil {
			toRole = agentprofile.Canonical(target.Profile)
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
		return messageID, nil
	default:
		return "", fmt.Errorf("supervised update_coagent is not available to %s", profile)
	}
}
