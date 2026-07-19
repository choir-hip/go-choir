package agentcore

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

type updaterReceiptKeyResolver struct {
	ref computerevent.SignerRef
	key ed25519.PublicKey
}

func (r updaterReceiptKeyResolver) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != r.ref.SignerDomain || keyID != r.ref.KeyID || len(r.key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("updater receipt key refused")
	}
	return append(ed25519.PublicKey(nil), r.key...), nil
}

func (rt *Runtime) reconcileSelfDevelopmentMaterialization(ctx context.Context) {
	if rt == nil || rt.selfdevUpdater == nil || rt.selfdevControl == nil || rt.selfdevRoute == nil || rt.selfdevRouteOwnerID == "" || rt.selfdevRouteDesktopID == "" || rt.selfdevComputerID == "" || rt.selfdevOperations == nil || rt.eventAppender == nil || rt.store == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || strings.TrimSpace(rt.selfdevRealizationID) == "" {
		return
	}
	rt.selfdevMaterializeMu.Lock()
	defer rt.selfdevMaterializeMu.Unlock()
	operations, err := rt.selfdevOperations.ListByStates(ctx, rt.selfdevComputerID, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateRollbackPending)
	if err != nil {
		return
	}
	for _, operation := range operations {
		var operationErr error
		if operation.State == selfdev.StateRollbackPending {
			operationErr = rt.rollbackSelfDevelopmentOperation(ctx, operation)
		} else {
			operationErr = rt.materializeSelfDevelopmentOperation(ctx, operation)
		}
		if operationErr != nil {
			// The durable operation and updater journal retain the recovery point.
			// Startup and idempotent owner retries invoke this reconciler again.
			continue
		}
	}
}

func (rt *Runtime) materializeSelfDevelopmentOperation(ctx context.Context, operation selfdev.Operation) error {
	bundlePath := filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest, "bundle.json")
	rawBundle, err := os.ReadFile(bundlePath)
	if err != nil || computerevent.DigestBytes(rawBundle) != operation.BundleDigest {
		return fmt.Errorf("materializer: frozen bundle unavailable")
	}
	var bundle transaction.TransactionRecord
	decoder := json.NewDecoder(strings.NewReader(string(rawBundle)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil || bundle.Rejected || !computerevent.IsSHA256(bundle.RuntimeArtifactDigest) || bundle.BaseEffectiveEventHead != operation.EffectiveHead || len(bundle.RuntimeFiles) == 0 {
		return fmt.Errorf("materializer: invalid frozen bundle")
	}
	if operation.State == selfdev.StateAccepted {
		idempotency := "selfdev-materialization-started-" + operation.DecisionEvent
		if _, found, lookupErr := rt.store.EventByIdempotency(ctx, operation.ComputerID, idempotency); lookupErr != nil {
			return lookupErr
		} else if !found {
			eventID, eventErr := computerevent.NewEventID()
			if eventErr != nil {
				return eventErr
			}
			event := computerevent.Event{
				SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
				EventKind: computerevent.EventMaterializationStarted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
				IdempotencyKey: idempotency, RequestCommitment: computerevent.ZeroHead,
				TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID, ActorProfile: agentprofile.Super,
				AuthorityRef: "guest-core:choir-updater", PrivacyClass: "owner", PayloadCommitment: computerevent.ZeroHead,
				ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent, ReducerVersion: computerevent.ReducerVersionV1,
			}
			if _, eventErr = rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{}, nil); eventErr != nil {
				return eventErr
			}
		}
		head, headErr := rt.store.Head(ctx, operation.ComputerID)
		if headErr != nil || head == nil {
			return fmt.Errorf("materializer: started head unavailable")
		}
		operation, err = rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, selfdev.StateAccepted, selfdev.StateMaterializing, func(next *selfdev.Operation) error {
			next.DesiredHead, next.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
			return nil
		})
		if err != nil {
			return err
		}
	}
	bundleURI := "artifact+sha256://" + operation.BundleDigest + "/sha256/computer-event-payload/" + operation.BundleDigest
	closure, err := computerversion.NewCodeClosure(bundle.SourceTreeDigest, []computerversion.CodeArtifact{{
		Name: "capsule-effect-bundle.json", SHA256: operation.BundleDigest, URI: bundleURI,
	}}, bundle.Timestamp.UTC())
	if err != nil {
		return err
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "capsule_effect_bundle", ContentSHA256: operation.BundleDigest, ArtifactURI: bundleURI,
	}}, bundle.Timestamp.UTC())
	if err != nil {
		return err
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}

	manifestFiles := make([]updater.ManifestFile, len(bundle.RuntimeFiles))
	for index, file := range bundle.RuntimeFiles {
		manifestFiles[index] = updater.ManifestFile{Path: file.Path, SHA256: file.SHA256, Mode: file.Mode}
	}
	manifest, err := updater.FinalizeManifest(updater.ReleaseManifest{
		Version: updater.ManifestVersion, ComputerID: operation.ComputerID, AcceptedEventHead: operation.DecisionEvent,
		CodeRef: string(version.CodeRef), ArtifactProgramRef: string(version.ArtifactProgramRef),
		EventSchemaVersion: computerevent.SchemaVersionV1, ReducerVersion: computerevent.ReducerVersionV1,
		Marker: "selfdev-" + operation.BundleDigest[:16], Files: manifestFiles,
	})
	if err != nil {
		return err
	}
	applyRequest := updater.ApplyRequest{
		ComputerID: operation.ComputerID, RealizationID: rt.selfdevRealizationID, OperationID: operation.OperationID,
		IdempotencyKey: "selfdev-apply-" + operation.DecisionEvent, AcceptedEventHead: operation.DecisionEvent,
		SourceDir: filepath.Dir(bundlePath), Manifest: manifest,
	}
	applyRequest.RequestCommitment, err = updater.ComputeApplyRequestCommitment(applyRequest)
	if err != nil {
		return err
	}
	result, applyErr := rt.selfdevUpdater.Apply(ctx, applyRequest)
	ref, publicKey, keyErr := rt.selfdevUpdater.PublicKey(ctx)
	if keyErr != nil {
		return keyErr
	}
	resolver := updaterReceiptKeyResolver{ref: ref, key: publicKey}
	if applyErr == nil {
		if result.Outcome != "applied" || result.MaterializationReceipt.Verify(resolver) != nil || result.HealthReceipt.Verify(resolver) != nil {
			return fmt.Errorf("materializer: invalid applied receipts")
		}
		return rt.recordMaterializationApplied(ctx, operation, result, closure, program, computerevent.EventMaterializationApplied, selfdev.StateMaterializing, selfdev.StateApplied, routeledger.TransitionPromote, "computer:self_development:approve")
	}
	if result.RecoveryReceipt != nil && result.RecoveryReceipt.Verify(resolver) == nil {
		return rt.recordMaterializationFailed(ctx, operation, result, applyErr, selfdev.StateMaterializing)
	}
	_, transitionErr := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, selfdev.StateMaterializing, selfdev.StateDegraded, func(next *selfdev.Operation) error {
		next.TerminalError = applyErr.Error()
		return nil
	})
	if transitionErr != nil {
		return transitionErr
	}
	return applyErr
}

func (rt *Runtime) rollbackSelfDevelopmentOperation(ctx context.Context, operation selfdev.Operation) error {
	manifest, releaseDir, err := updater.ReadPinnedManifest(rt.selfdevUpdaterRoot, operation.ReleaseDigest)
	if err != nil {
		return fmt.Errorf("materializer: rollback release unavailable: %w", err)
	}
	version := computerversion.ComputerVersion{CodeRef: computerversion.CodeRef(operation.CodeRef), ArtifactProgramRef: computerversion.ArtifactProgramRef(operation.ArtifactProgramRef)}
	inputs, err := rt.selfdevRoute.ResolveComputerVersionInputs(ctx, version)
	if err != nil {
		return fmt.Errorf("materializer: rollback immutable inputs unavailable: %w", err)
	}
	closure, program := inputs.CodeClosure, inputs.ArtifactProgram
	manifest.AcceptedEventHead = operation.DecisionEvent
	manifest.ContentDigest = ""
	manifest, err = updater.FinalizeManifest(manifest)
	if err != nil {
		return err
	}
	applyRequest := updater.ApplyRequest{
		ComputerID: operation.ComputerID, RealizationID: rt.selfdevRealizationID, OperationID: operation.OperationID,
		IdempotencyKey: "selfdev-rollback-apply-" + operation.DecisionEvent, AcceptedEventHead: operation.DecisionEvent,
		SourceDir: releaseDir, Manifest: manifest,
	}
	applyRequest.RequestCommitment, err = updater.ComputeApplyRequestCommitment(applyRequest)
	if err != nil {
		return err
	}
	result, applyErr := rt.selfdevUpdater.Apply(ctx, applyRequest)
	ref, publicKey, keyErr := rt.selfdevUpdater.PublicKey(ctx)
	if keyErr != nil {
		return keyErr
	}
	resolver := updaterReceiptKeyResolver{ref: ref, key: publicKey}
	if applyErr == nil {
		if result.Outcome != "applied" || result.MaterializationReceipt.Verify(resolver) != nil || result.HealthReceipt.Verify(resolver) != nil {
			return fmt.Errorf("materializer: invalid rollback receipts")
		}
		return rt.recordMaterializationApplied(ctx, operation, result, closure, program, computerevent.EventRollbackApplied, selfdev.StateRollbackPending, selfdev.StateRolledBack, routeledger.TransitionRollback, "computer:self_development:rollback")
	}
	if result.RecoveryReceipt != nil && result.RecoveryReceipt.Verify(resolver) == nil {
		return rt.recordMaterializationFailed(ctx, operation, result, applyErr, selfdev.StateRollbackPending)
	}
	_, transitionErr := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, selfdev.StateRollbackPending, selfdev.StateDegraded, func(next *selfdev.Operation) error {
		next.TerminalError = applyErr.Error()
		return nil
	})
	if transitionErr != nil {
		return errors.Join(applyErr, transitionErr)
	}
	return applyErr
}

func (rt *Runtime) recordMaterializationApplied(ctx context.Context, operation selfdev.Operation, result updater.ApplyResult, closure computerversion.CodeClosure, program computerversion.ArtifactProgram, eventKind computerevent.EventKind, expectedState, nextState string, routeKind routeledger.TransitionKind, decisionScope string) error {
	payload, err := computerevent.CanonicalJSON(result)
	if err != nil {
		return err
	}
	receiptBytes, err := result.MaterializationReceipt.CanonicalBytes()
	if err != nil {
		return err
	}
	receiptDigest := computerevent.DigestBytes(receiptBytes)
	idempotency := "selfdev-" + string(eventKind) + "-" + operation.DecisionEvent
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, operation.ComputerID, idempotency); lookupErr != nil {
		return lookupErr
	} else if !found {
		head, headErr := rt.store.Head(ctx, operation.ComputerID)
		if headErr != nil || head == nil {
			return fmt.Errorf("materializer: desired projection unavailable")
		}
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return eventErr
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
			EventKind: eventKind, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: idempotency, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ActorProfile: agentprofile.Super, AuthorityRef: "guest-core:choir-updater", PrivacyClass: "owner",
			ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent,
			ResultingEffectiveCommitment: head.DesiredStateCommitment, OutputArtifactRefs: []string{receiptDigest, result.ReleaseDigest},
			ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, _, eventErr = rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.materialization-result+json", "owner"); eventErr != nil {
			return eventErr
		}
	}
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		return fmt.Errorf("materializer: applied projection unavailable")
	}
	eventReceipt, found, err := rt.store.EventReceiptByIdempotency(ctx, operation.ComputerID, idempotency)
	if err != nil || !found {
		return fmt.Errorf("materializer: applied event receipt unavailable")
	}
	appliedEventHead, _ := eventReceipt.KindFields["event_digest"].(string)
	if !computerevent.IsSHA256(appliedEventHead) {
		return fmt.Errorf("materializer: applied event receipt is not head-bound")
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	reconstructionDigest, err := selfdevprotocol.Digest(struct {
		Version       computerversion.ComputerVersion `json:"computer_version"`
		EffectiveHead string                          `json:"effective_event_head"`
		ReleaseDigest string                          `json:"release_digest"`
	}{version, head.EffectiveEventHead, result.ReleaseDigest})
	if err != nil {
		return err
	}
	verifierDecision := "pass"
	if eventKind == computerevent.EventRollbackApplied {
		verifierDecision = "rollback_prior_verified"
	}
	if len(operation.VerifierRefs) == 0 {
		return fmt.Errorf("materializer: verifier evidence unavailable")
	}
	verifierCertificate, err := rt.selfdevUpdater.SignVerifierCertificate(ctx, selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: operation.ComputerID, OperationID: operation.OperationID,
		BundleDigest: operation.BundleDigest, VerificationEventDigest: operation.VerifierRefs[0],
		VerifierEvidenceRefs: operation.VerifierRefs, DecisionEventHead: operation.DecisionEvent,
		CodeRef: string(version.CodeRef), ArtifactProgramRef: string(version.ArtifactProgramRef),
		ReleaseDigest: result.ReleaseDigest, Decision: verifierDecision,
	})
	if err != nil {
		return err
	}
	verifierJSON, err := computerevent.CanonicalJSON(verifierCertificate.Certificate)
	if err != nil {
		return err
	}
	verifierDigest := computerevent.DigestBytes(verifierJSON)
	checkpoint, err := rt.selfdevControl.PublishCheckpoint(ctx, selfdevprotocol.CheckpointRequest{
		ComputerID: operation.ComputerID, IdempotencyKey: "selfdev-checkpoint-" + operation.DecisionEvent,
		ComputerVersion: version, AcceptedEventHead: appliedEventHead, EffectiveEventHead: head.EffectiveEventHead,
		EffectiveStateCommitment: head.EffectiveStateCommitment, EventHeadReceiptID: eventReceipt.ReceiptID,
		ReleaseDigest: result.ReleaseDigest, ReconstructionDigest: reconstructionDigest,
		MaterializationReceiptDigest: receiptDigest, VerifierCertificateDigest: verifierDigest,
		VerifierCertificate: verifierCertificate, ReducerVersion: head.ReducerVersion,
	})
	if err != nil {
		return err
	}
	checkpointRef := "checkpoint:sha256:" + checkpoint.Checkpoint.Digest
	checkpointEventIdempotency := "selfdev-checkpoint-published-" + operation.DecisionEvent
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, operation.ComputerID, checkpointEventIdempotency); lookupErr != nil {
		return lookupErr
	} else if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return eventErr
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
			EventKind: computerevent.EventCheckpointPublished, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: checkpointEventIdempotency, RequestCommitment: computerevent.ZeroHead,
			TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ActorProfile: agentprofile.Super, AuthorityRef: "platform-control:checkpoint",
			PayloadCommitment: computerevent.ZeroHead, PrivacyClass: "owner",
			ProposedEffectRef: checkpoint.Checkpoint.Digest, DecisionRef: operation.DecisionEvent, ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, eventErr = rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{}, nil); eventErr != nil {
			return eventErr
		}
	}
	checkpointEventReceipt, found, err := rt.store.EventReceiptByIdempotency(ctx, operation.ComputerID, checkpointEventIdempotency)
	if err != nil || !found {
		return fmt.Errorf("materializer: checkpoint event receipt unavailable")
	}
	checkpointEventHead, _ := checkpointEventReceipt.KindFields["event_digest"].(string)
	if !computerevent.IsSHA256(checkpointEventHead) {
		return fmt.Errorf("materializer: checkpoint event receipt is not head-bound")
	}
	routeSlotID, err := routeledger.RouteSlotID(rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID)
	if err != nil {
		return err
	}
	currentRoute, err := rt.selfdevRoute.ResolveComputerVersionRoute(ctx, routeSlotID)
	if err != nil {
		return err
	}
	routeIdempotency := routeledger.IdempotencyKey("idempotency:selfdev-route:" + operation.DecisionEvent)
	oldVersion, expectedGeneration := currentRoute.Slot.Current, currentRoute.Slot.Generation
	if currentRoute.Slot.Current == version {
		if currentRoute.LatestReceipt.IdempotencyKey != routeIdempotency || currentRoute.LatestReceipt.New != version {
			return fmt.Errorf("materializer: current route already changed by another transition")
		}
		oldVersion, expectedGeneration = currentRoute.LatestReceipt.Old, currentRoute.LatestReceipt.ExpectedGeneration
	}
	createdAt := checkpoint.Receipt.IssuedAt
	acceptedPayload := selfdevprotocol.AcceptedEventAuthorizationEvidence{
		Version: 1, ComputerID: operation.ComputerID, AcceptedOrRollbackEventDigest: appliedEventHead,
		EventHeadReceiptID: eventReceipt.ReceiptID, EffectiveEventHead: head.EffectiveEventHead,
		OldComputerVersion: oldVersion, NewComputerVersion: version,
		DecisionActor: operation.DecisionActor, DecisionScope: decisionScope,
	}
	acceptedJSON, err := computerevent.CanonicalJSON(acceptedPayload)
	if err != nil {
		return err
	}
	approvalEvidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, routeSlotID, version, acceptedJSON, createdAt)
	if err != nil {
		return err
	}
	checkpointReceiptDigest, err := selfdevprotocol.Digest(checkpoint.Receipt)
	if err != nil {
		return err
	}
	promotionPayload := selfdevprotocol.PromotionJoinEvidence{
		Version: 1, ComputerID: operation.ComputerID, EventHeadReceiptID: checkpointEventReceipt.ReceiptID,
		CheckpointReceiptDigest: checkpointReceiptDigest, MaterializationReceiptDigest: receiptDigest,
		VerifierCertificateDigest: verifierDigest, OldComputerVersion: oldVersion, NewComputerVersion: version,
	}
	promotionJSON, err := computerevent.CanonicalJSON(promotionPayload)
	if err != nil {
		return err
	}
	promotionEvidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, routeSlotID, version, promotionJSON, createdAt)
	if err != nil {
		return err
	}
	command := routeledger.TransitionCommand{
		RouteSlotID: routeSlotID, Kind: routeKind, Old: oldVersion, New: version,
		ExpectedGeneration: expectedGeneration, ApprovalRef: routeledger.ApprovalRef(approvalEvidence.Ref),
		PromotionCertificateRef: routeledger.PromotionCertificateRef(promotionEvidence.Ref),
		IdempotencyKey:          routeIdempotency,
	}
	if routeKind == routeledger.TransitionRollback {
		command.RollbackTargetReceiptID = routeledger.ReceiptID(operation.RouteReceipt)
	}
	authorizationWindow := time.Now().UTC().Truncate(time.Minute)
	projectionRequest := selfdevprotocol.RouteProjectionRequest{
		ComputerID: operation.ComputerID, IdempotencyKey: fmt.Sprintf("selfdev-route-certificate-%s-%d", operation.DecisionEvent, authorizationWindow.Unix()),
		Checkpoint: checkpoint, CodeClosure: closure, ArtifactProgram: program,
		CanonicalEventHead: checkpointEventHead, EventHeadReceiptID: checkpointEventReceipt.ReceiptID,
		ApprovalEvidence: approvalEvidence, PromotionEvidence: promotionEvidence, Command: command,
		DecisionActor: operation.DecisionActor, DecisionScope: decisionScope,
		ExpiresAt: authorizationWindow.Add(5 * time.Minute).Format(time.RFC3339Nano),
	}
	authorization, err := rt.selfdevControl.PublishRouteProjection(ctx, projectionRequest)
	if err != nil {
		return err
	}
	route, err := rt.selfdevRoute.ApplySelfDevelopmentRouteProjection(ctx, selfdevprotocol.ApplyRouteProjectionRequest{Projection: projectionRequest, Authorization: authorization})
	if err != nil || route.TransitionReceipt == nil {
		return fmt.Errorf("materializer: route projection failed: %w", err)
	}
	routeCertificateDigest := authorization.Receipt.ArtifactDigest
	routeGeneration := route.Slot.Generation
	routeEventIdempotency := "selfdev-route-projection-updated-" + operation.DecisionEvent
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, operation.ComputerID, routeEventIdempotency); lookupErr != nil {
		return lookupErr
	} else if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return eventErr
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
			EventKind: computerevent.EventRouteProjectionUpdated, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: routeEventIdempotency, RequestCommitment: computerevent.ZeroHead,
			TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ActorProfile: agentprofile.Super, AuthorityRef: "vmctl:route-cas",
			PayloadCommitment: computerevent.ZeroHead, PrivacyClass: "owner",
			ProposedEffectRef: routeCertificateDigest, DecisionRef: operation.DecisionEvent, ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, eventErr = rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{}, nil); eventErr != nil {
			return eventErr
		}
	}
	finalHead, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || finalHead == nil {
		return fmt.Errorf("materializer: route projection event head unavailable")
	}
	_, err = rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, expectedState, nextState, func(next *selfdev.Operation) error {
		next.DesiredHead, next.EffectiveHead = finalHead.DesiredEventHead, finalHead.EffectiveEventHead
		next.MaterializationReceipt, next.CheckpointRef, next.RouteCertificate, next.RouteGeneration, next.RouteReceipt = receiptDigest, checkpointRef, routeCertificateDigest, &routeGeneration, string(route.TransitionReceipt.ID)
		next.ReleaseDigest, next.CodeRef, next.ArtifactProgramRef = result.ReleaseDigest, string(version.CodeRef), string(version.ArtifactProgramRef)
		return nil
	})
	return err
}

func (rt *Runtime) recordMaterializationFailed(ctx context.Context, operation selfdev.Operation, result updater.ApplyResult, applyErr error, expectedState string) error {
	payload, err := computerevent.CanonicalJSON(result)
	if err != nil {
		return err
	}
	recoveryBytes, err := result.RecoveryReceipt.CanonicalBytes()
	if err != nil {
		return err
	}
	recoveryDigest := computerevent.DigestBytes(recoveryBytes)
	idempotency := "selfdev-materialization-failed-" + operation.DecisionEvent
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, operation.ComputerID, idempotency); lookupErr != nil {
		return lookupErr
	} else if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return eventErr
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: operation.ComputerID,
			EventKind: computerevent.EventMaterializationFailed, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: idempotency, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
			ActorProfile: agentprofile.Super, AuthorityRef: "guest-core:choir-updater", PrivacyClass: "owner",
			ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent,
			OutputArtifactRefs: []string{recoveryDigest}, ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, _, eventErr = rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{RestoredPriorEffective: true}, payload, "application/vnd.choir.materialization-result+json", "owner"); eventErr != nil {
			return eventErr
		}
	}
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		return fmt.Errorf("materializer: recovery projection unavailable")
	}
	_, err = rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, expectedState, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.MaterializationReceipt = recoveryDigest
		next.DesiredHead, next.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
		next.TerminalError = applyErr.Error()
		return nil
	})
	return err
}

func bundleDigestFromRelease(releaseDigest, fallback string) string {
	if computerevent.IsSHA256(releaseDigest) {
		return releaseDigest
	}
	return fallback
}
