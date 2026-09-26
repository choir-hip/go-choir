package agentcore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Platform-update push path (M9a). A tracking computer fast-forwards its
// platform-tracked surface on a platform-control-signed offer — no owner
// decision, no selfdev operation. The update is a forward transaction on the
// canonical event chain: accepted -> started -> applied -> checkpoint ->
// route promotion under the platform-follow evidence class. Restore stays on
// the existing checkpoint/tape edge; nothing here mints owner authority.

// ErrPlatformUpdateStaleHead marks base-head divergence — the offer bound a
// canonical head the guest has moved past. Callers map it to 409.
var ErrPlatformUpdateStaleHead = errors.New("platform update: base event head is stale")

// PlatformUpdateReport is the apply endpoint's observable result.
type PlatformUpdateReport struct {
	UpdateID                  string `json:"update_id"`
	ComputerID                string `json:"computer_id"`
	AcceptedEventHead         string `json:"accepted_event_head"`
	AppliedEventHead          string `json:"applied_event_head"`
	ReleaseDigest             string `json:"release_digest"`
	PriorReleaseDigest        string `json:"prior_release_digest,omitempty"`
	RouteGeneration           uint64 `json:"route_generation"`
	RouteReceiptID            string `json:"route_receipt_id,omitempty"`
	CheckpointDigest          string `json:"checkpoint_digest"`
	VerifierCertificateDigest string `json:"verifier_certificate_digest"`
	// Checkpoint is the published restore artifact — a subsequent restore
	// presents it to the checkpoint/tape path to return to this head.
	Checkpoint *selfdevprotocol.CheckpointResponse `json:"checkpoint,omitempty"`
	Replayed   bool                                `json:"replayed"`
}

// platformUpdateIdempotencyKey namespaces canonical events for one update.
func platformUpdateIdempotencyKey(kind, updateID string) string {
	return "platform-update-" + kind + "-" + updateID
}

// ApplyPlatformUpdate verifies a platform-signed offer and applies it through
// the guest updater under canonical head binding. Verification-first:
// signature, structure, expiry, lineage policy, and head binding all gate
// before any mutation. The offer's canonical digest becomes DecisionRef on the
// accepted event — the event chain, not the HTTP body, is the receipt.
func (rt *Runtime) ApplyPlatformUpdate(ctx context.Context, offer selfdevprotocol.PlatformUpdateOffer) (PlatformUpdateReport, error) {
	report := PlatformUpdateReport{UpdateID: offer.UpdateID, ComputerID: offer.ComputerID}
	if rt == nil || rt.selfdevUpdater == nil || rt.selfdevControl == nil || rt.selfdevVerifier == nil || rt.selfdevRoute == nil || rt.eventAppender == nil || rt.store == nil {
		return report, fmt.Errorf("platform update: runtime surface incomplete")
	}
	if offer.ComputerID != rt.selfdevComputerID {
		return report, fmt.Errorf("platform update: offer binds a different computer")
	}
	if offer.Realization != rt.selfdevRealizationID {
		return report, fmt.Errorf("platform update: offer binds a different realization")
	}
	if err := selfdevprotocol.PlatformUpdateOfferFromRequest(offer, time.Now().UTC()); err != nil {
		return report, err
	}
	offerDigest, err := offer.Digest()
	if err != nil {
		return report, err
	}
	authorization := offer.Authorization
	if authorization.Kind != selfdevprotocol.ReceiptKindPlatformUpdate ||
		authorization.ComputerID != offer.ComputerID ||
		authorization.ArtifactDigest != offerDigest {
		return report, fmt.Errorf("platform update: authorization does not bind this offer")
	}
	if err := authorization.Verify(rt.selfdevControl.PublicKey()); err != nil {
		return report, fmt.Errorf("platform update: signature refused: %w", err)
	}
	computerID := offer.ComputerID
	appliedKey := platformUpdateIdempotencyKey("applied", offer.UpdateID)
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, computerID, appliedKey); lookupErr != nil {
		return report, lookupErr
	} else if found {
		// Replay: the update already committed. Return the recorded outcome —
		// the event chain is the receipt; we do not re-apply.
		report.Replayed = true
		return report, nil
	}
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		return report, fmt.Errorf("platform update: canonical head unavailable")
	}
	// Resume gate: a platform update restarts its own guest, killing this
	// handler mid-flight after the accepted event commits. A re-pushed
	// identical offer may continue only when the pending transition is this
	// update's own accepted event — any other pending transition is stale.
	acceptedKey := platformUpdateIdempotencyKey("accepted", offer.UpdateID)
	var acceptedDigest string
	resuming := false
	if head.PendingTransitionRef != "" {
		pendingEvent, found, evErr := rt.store.EventByIdempotency(ctx, computerID, acceptedKey)
		if evErr != nil {
			return report, evErr
		}
		if !found {
			return report, ErrPlatformUpdateStaleHead
		}
		pendingDigest, digestErr := pendingEvent.Digest()
		if digestErr != nil {
			return report, digestErr
		}
		if pendingDigest != head.PendingTransitionRef {
			return report, ErrPlatformUpdateStaleHead
		}
		acceptedDigest = pendingDigest
		resuming = true
	} else if head.CanonicalEventHead != offer.BaseEventHead {
		return report, ErrPlatformUpdateStaleHead
	}

	if !resuming {
		// Target commitment: what the accepted event binds the desired state to.
		// Deterministic over the offer — both verifier and auditor recompute it.
		targetCommitment, targetErr := selfdevprotocol.Digest(struct {
			ComputerID    string `json:"computer_id"`
			UpdateID      string `json:"update_id"`
			OfferDigest   string `json:"offer_digest"`
			ContentDigest string `json:"content_digest"`
		}{computerID, offer.UpdateID, offerDigest, offer.Manifest.ContentDigest})
		if targetErr != nil {
			return report, targetErr
		}

		// Canonical binding: the accepted event commits desired state BEFORE the
		// updater mutates anything. AcceptedEventHead must be a real head.
		acceptedEventID, idErr := computerevent.NewEventID()
		if idErr != nil {
			return report, idErr
		}
		acceptedEvent := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: acceptedEventID, ComputerID: computerID,
			EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: acceptedKey,
			ActorProfile:   agentprofile.Management, AuthorityRef: "platform-control:update",
			PrivacyClass: "owner", PayloadCommitment: computerevent.ZeroHead,
			ProposedEffectRef: offer.Manifest.ContentDigest, DecisionRef: offerDigest,
			VerifierRefs:                     offer.VerifierRefs,
			RequireExpectedHead:              true,
			PreviousHead:                     head.CanonicalEventHead,
			ExpectedDesiredEventHead:         head.DesiredEventHead,
			ExpectedEffectiveEventHead:       head.EffectiveEventHead,
			ExpectedDesiredStateCommitment:   head.DesiredStateCommitment,
			ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
			ReducerVersion:                   computerevent.ReducerVersionV1,
		}
		if _, appendErr := rt.eventAppender.AppendNew(ctx, acceptedEvent, computerevent.TransitionInput{TargetStateCommitment: targetCommitment}, nil); appendErr != nil {
			return report, fmt.Errorf("platform update: accepted event refused: %w", appendErr)
		}
		head, err = rt.store.Head(ctx, computerID)
		if err != nil || head == nil || head.PendingTransitionRef == "" {
			return report, fmt.Errorf("platform update: accepted head projection unavailable")
		}
		acceptedDigest = head.CanonicalEventHead
	}
	report.AcceptedEventHead = acceptedDigest

	// Materialization started — mirrors the selfdev event surface so tapes
	// carry the same transition shape for updates and owner decisions. On a
	// resume the event already committed; dedup on the idempotency key.
	startedKey := platformUpdateIdempotencyKey("started", offer.UpdateID)
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, computerID, startedKey); lookupErr != nil {
		return report, lookupErr
	} else if !found {
		startedEventID, idErr := computerevent.NewEventID()
		if idErr != nil {
			return report, idErr
		}
		startedEvent := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: startedEventID, ComputerID: computerID,
			EventKind: computerevent.EventMaterializationStarted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: startedKey,
			ActorProfile:   agentprofile.Management, AuthorityRef: "guest-core:choir-updater",
			PrivacyClass: "owner", PayloadCommitment: computerevent.ZeroHead,
			ProposedEffectRef: offerDigest, DecisionRef: acceptedDigest,
			ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, appendErr := rt.eventAppender.AppendNew(ctx, startedEvent, computerevent.TransitionInput{}, nil); appendErr != nil {
			return report, fmt.Errorf("platform update: materialization-start refused: %w", appendErr)
		}
	}
	// Finalize the manifest with the real accepted head, then stage payload
	// into the updater's trusted incoming store.
	manifest := offer.Manifest
	manifest.AcceptedEventHead = acceptedDigest
	manifest, err = updater.FinalizeManifest(manifest)
	if err != nil {
		return report, err
	}
	releaseDigest := manifest.ContentDigest
	incomingDir, err := rt.stagePlatformUpdatePayload(offer, releaseDigest)
	if err != nil {
		return report, err
	}

	applyRequest := updater.ApplyRequest{
		ComputerID: computerID, RealizationID: rt.selfdevRealizationID,
		OperationID:       "platform-update-" + offer.UpdateID,
		IdempotencyKey:    platformUpdateIdempotencyKey("apply", offer.UpdateID),
		AcceptedEventHead: acceptedDigest,
		SourceDir:         incomingDir, Manifest: manifest,
	}
	applyRequest.RequestCommitment, err = updater.ComputeApplyRequestCommitment(applyRequest)
	if err != nil {
		return report, err
	}
	result, applyErr := rt.selfdevUpdater.Apply(ctx, applyRequest)
	ref, publicKey, keyErr := rt.selfdevUpdater.PublicKey(ctx)
	if keyErr != nil {
		return report, keyErr
	}
	resolver := updaterReceiptKeyResolver{ref: ref, key: publicKey}
	if applyErr != nil {
		if result.RecoveryReceipt != nil && result.RecoveryReceipt.Verify(resolver) == nil {
			return report, rt.recordPlatformUpdateFailed(ctx, offer, offerDigest, acceptedDigest, result, applyErr)
		}
		return report, fmt.Errorf("platform update: apply failed: %w", applyErr)
	}
	if result.Outcome != "applied" || result.MaterializationReceipt.Verify(resolver) != nil || result.HealthReceipt.Verify(resolver) != nil {
		return report, fmt.Errorf("platform update: invalid applied receipts")
	}
	report.ReleaseDigest = result.ReleaseDigest
	report.PriorReleaseDigest = result.PriorReleaseDigest

	head, err = rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		return report, fmt.Errorf("platform update: applied projection unavailable")
	}
	// Applied event closes the pending transition.
	appliedEventID, err := computerevent.NewEventID()
	if err != nil {
		return report, err
	}
	resultPayload, _ := json.Marshal(result)
	appliedEvent := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: appliedEventID, ComputerID: computerID,
		EventKind: computerevent.EventMaterializationApplied, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: appliedKey,
		ActorProfile:   agentprofile.Management, AuthorityRef: "guest-core:choir-updater",
		PrivacyClass:      "owner",
		ProposedEffectRef: offerDigest, DecisionRef: acceptedDigest,
		ResultingEffectiveCommitment: head.DesiredStateCommitment,
		ReducerVersion:               computerevent.ReducerVersionV1,
	}
	if _, _, err = rt.eventAppender.AppendNewPayload(ctx, appliedEvent, computerevent.TransitionInput{}, resultPayload, "application/vnd.choir.platform-update-result+json", "owner"); err != nil {
		return report, fmt.Errorf("platform update: applied event refused: %w", err)
	}
	head, err = rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		return report, fmt.Errorf("platform update: post-apply head unavailable")
	}
	appliedEventHead := head.CanonicalEventHead
	report.AppliedEventHead = appliedEventHead

	appliedReceipt, found, err := rt.store.EventReceiptByIdempotency(ctx, computerID, appliedKey)
	if err != nil || !found {
		return report, fmt.Errorf("platform update: applied event receipt unavailable")
	}
	receiptDigest, err := selfdevprotocol.Digest(result.MaterializationReceipt)
	if err != nil {
		return report, err
	}

	// Verifier certificate over the platform-attached evidence — same signer
	// domain the selfdev path uses; the offer's verifier refs are the input.
	version := computerversion.ComputerVersion{CodeRef: offer.CodeClosure.Ref, ArtifactProgramRef: offer.ArtifactProgram.Ref}
	reconstructionDigest, err := selfdevprotocol.Digest(struct {
		Version       computerversion.ComputerVersion `json:"computer_version"`
		EffectiveHead string                          `json:"effective_event_head"`
		ReleaseDigest string                          `json:"release_digest"`
	}{version, head.EffectiveEventHead, result.ReleaseDigest})
	if err != nil {
		return report, err
	}
	verifierCertificate, err := rt.selfdevVerifier.SignVerifierCertificate(ctx, selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: computerID, OperationID: applyRequest.OperationID,
		BundleDigest: offerDigest, VerificationEventDigest: offer.VerifierRefs[0],
		VerifierEvidenceRefs: offer.VerifierRefs, DecisionEventHead: acceptedDigest,
		CodeRef: string(version.CodeRef), ArtifactProgramRef: string(version.ArtifactProgramRef),
		ReleaseDigest: result.ReleaseDigest, Decision: "pass",
	})
	if err != nil {
		return report, fmt.Errorf("platform update: verifier certificate refused: %w", err)
	}
	verifierJSON, err := computerevent.CanonicalJSON(verifierCertificate.Certificate)
	if err != nil {
		return report, err
	}
	verifierDigest := computerevent.DigestBytes(verifierJSON)
	report.VerifierCertificateDigest = verifierDigest

	pinned, _, err := updater.ReadPinnedManifest(rt.selfdevUpdaterRoot, result.ReleaseDigest)
	if err != nil {
		return report, fmt.Errorf("platform update: applied release unavailable: %w", err)
	}
	witness, frontend, err := rt.checkpointRestoreBindings(ctx, computerID, result.ReleaseDigest, pinned.Files)
	if err != nil {
		return report, err
	}
	checkpoint, err := rt.selfdevControl.PublishCheckpoint(ctx, selfdevprotocol.CheckpointRequest{
		ComputerID: computerID, IdempotencyKey: "platform-update-checkpoint-" + offer.UpdateID,
		ComputerVersion: version, AcceptedEventHead: appliedEventHead, EffectiveEventHead: head.EffectiveEventHead,
		EffectiveStateCommitment: head.EffectiveStateCommitment, EventHeadReceiptID: appliedReceipt.ReceiptID,
		ReleaseDigest: result.ReleaseDigest, ReconstructionDigest: reconstructionDigest,
		MaterializationReceiptDigest: receiptDigest, VerifierCertificateDigest: verifierDigest,
		VerifierCertificate: verifierCertificate, ReducerVersion: head.ReducerVersion,
		VMLocalContentWitness: witness, FrontendIdentity: frontend,
	})
	if err != nil {
		return report, fmt.Errorf("platform update: checkpoint refused: %w", err)
	}
	report.CheckpointDigest = checkpoint.Checkpoint.Digest
	report.Checkpoint = &checkpoint

	// Checkpoint-published causal event — needed for the promotion join's
	// event-head receipt, same as the selfdev path.
	checkpointEventIdempotency := platformUpdateIdempotencyKey("checkpoint-published", offer.UpdateID)
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, computerID, checkpointEventIdempotency); lookupErr != nil {
		return report, lookupErr
	} else if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return report, eventErr
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: computerevent.EventCheckpointPublished, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: checkpointEventIdempotency,
			ActorProfile:   agentprofile.Management, AuthorityRef: "platform-control:checkpoint",
			PayloadCommitment: computerevent.ZeroHead, PrivacyClass: "owner",
			ProposedEffectRef: checkpoint.Checkpoint.Digest, DecisionRef: acceptedDigest,
			ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, eventErr = rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{}, nil); eventErr != nil {
			return report, eventErr
		}
	}
	checkpointEventReceipt, found, err := rt.store.EventReceiptByIdempotency(ctx, computerID, checkpointEventIdempotency)
	if err != nil || !found {
		return report, fmt.Errorf("platform update: checkpoint event receipt unavailable")
	}
	checkpointEventHead, _ := checkpointEventReceipt.KindFields["event_digest"].(string)
	if !computerevent.IsSHA256(checkpointEventHead) {
		return report, fmt.Errorf("platform update: checkpoint event receipt is not head-bound")
	}

	// Route promotion under the platform-follow evidence class — distinct
	// scope + endpoint from the owner self-dev projection path.
	routeSlotID, err := routeledger.RouteSlotID(rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID)
	if err != nil {
		return report, err
	}
	currentRoute, err := rt.selfdevRoute.ResolveComputerVersionRouteOrAbsent(ctx, routeSlotID)
	if err != nil {
		return report, err
	}
	routeIdempotency := routeledger.IdempotencyKey("idempotency:platform-update-route:" + offer.UpdateID)
	transitionKind := routeledger.TransitionPromote
	oldVersion, expectedGeneration := currentRoute.Slot.Current, currentRoute.Slot.Generation
	switch {
	case currentRoute.RouteAbsent:
		// Fresh computers carry no route slot yet; the first platform update
		// bootstraps the slot (gen 1) with this update's pinned version.
		transitionKind = routeledger.TransitionBootstrap
		oldVersion, expectedGeneration = computerversion.ComputerVersion{}, 0
	case currentRoute.Slot.Current == version:
		if currentRoute.LatestReceipt.IdempotencyKey != routeIdempotency || currentRoute.LatestReceipt.New != version {
			return report, fmt.Errorf("platform update: current route already changed by another transition")
		}
		transitionKind = currentRoute.LatestReceipt.Kind
		oldVersion, expectedGeneration = currentRoute.LatestReceipt.Old, currentRoute.LatestReceipt.ExpectedGeneration
	}
	createdAt := checkpoint.Receipt.IssuedAt
	acceptedPayload := selfdevprotocol.AcceptedEventAuthorizationEvidence{
		Version: 1, ComputerID: computerID, AcceptedOrRollbackEventDigest: appliedEventHead,
		EventHeadReceiptID: appliedReceipt.ReceiptID, EffectiveEventHead: head.EffectiveEventHead,
		OldComputerVersion: oldVersion, NewComputerVersion: version,
		DecisionActor: selfdevprotocol.PlatformUpdateFollowActor, DecisionScope: selfdevprotocol.PlatformUpdateFollowScope,
	}
	acceptedJSON, err := computerevent.CanonicalJSON(acceptedPayload)
	if err != nil {
		return report, err
	}
	approvalEvidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, routeSlotID, version, acceptedJSON, createdAt)
	if err != nil {
		return report, err
	}
	checkpointReceiptDigest, err := selfdevprotocol.Digest(checkpoint.Receipt)
	if err != nil {
		return report, err
	}
	promotionPayload := selfdevprotocol.PromotionJoinEvidence{
		Version: 1, ComputerID: computerID, EventHeadReceiptID: checkpointEventReceipt.ReceiptID,
		CheckpointReceiptDigest: checkpointReceiptDigest, MaterializationReceiptDigest: receiptDigest,
		VerifierCertificateDigest: verifierDigest, OldComputerVersion: oldVersion, NewComputerVersion: version,
	}
	promotionJSON, err := computerevent.CanonicalJSON(promotionPayload)
	if err != nil {
		return report, err
	}
	promotionEvidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, routeSlotID, version, promotionJSON, createdAt)
	if err != nil {
		return report, err
	}
	command := routeledger.TransitionCommand{
		RouteSlotID: routeSlotID, Kind: transitionKind, Old: oldVersion, New: version,
		ExpectedGeneration: expectedGeneration, ApprovalRef: routeledger.ApprovalRef(approvalEvidence.Ref),
		PromotionCertificateRef: routeledger.PromotionCertificateRef(promotionEvidence.Ref),
		IdempotencyKey:          routeIdempotency,
	}
	authorizationWindow := time.Now().UTC().Truncate(time.Minute)
	projectionRequest := selfdevprotocol.RouteProjectionRequest{
		ComputerID: computerID, IdempotencyKey: fmt.Sprintf("platform-update-route-certificate-%s-%d", offer.UpdateID, authorizationWindow.Unix()),
		Checkpoint: checkpoint, CodeClosure: offer.CodeClosure, ArtifactProgram: offer.ArtifactProgram,
		CanonicalEventHead: checkpointEventHead, EventHeadReceiptID: checkpointEventReceipt.ReceiptID,
		ApprovalEvidence: approvalEvidence, PromotionEvidence: promotionEvidence, Command: command,
		DecisionActor: selfdevprotocol.PlatformUpdateFollowActor, DecisionScope: selfdevprotocol.PlatformUpdateFollowScope,
		ExpiresAt: authorizationWindow.Add(5 * time.Minute).Format(time.RFC3339Nano),
	}
	routeAuthorization, err := rt.selfdevControl.PublishRouteProjection(ctx, projectionRequest)
	if err != nil {
		return report, fmt.Errorf("platform update: route projection refused: %w", err)
	}
	route, err := rt.selfdevRoute.ApplyPlatformFollowRouteProjection(ctx, selfdevprotocol.ApplyRouteProjectionRequest{Projection: projectionRequest, Authorization: routeAuthorization})
	if err != nil || route.TransitionReceipt == nil {
		return report, fmt.Errorf("platform update: route promotion failed: %w", err)
	}
	report.RouteGeneration = route.Slot.Generation
	report.RouteReceiptID = string(route.TransitionReceipt.ID)

	// Route-projection causal event — mirrors the selfdev tape shape.
	routeEventIdempotency := platformUpdateIdempotencyKey("route-projection-updated", offer.UpdateID)
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, computerID, routeEventIdempotency); lookupErr != nil {
		return report, lookupErr
	} else if !found {
		eventID, eventErr := computerevent.NewEventID()
		if eventErr != nil {
			return report, eventErr
		}
		event := computerevent.Event{
			SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: computerID,
			EventKind: computerevent.EventRouteProjectionUpdated, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey: routeEventIdempotency,
			ActorProfile:   agentprofile.Management, AuthorityRef: "vmctl:route-cas",
			PayloadCommitment: computerevent.ZeroHead, PrivacyClass: "owner",
			ProposedEffectRef: routeAuthorization.Receipt.ArtifactDigest, DecisionRef: acceptedDigest,
			ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, eventErr = rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{}, nil); eventErr != nil {
			return report, eventErr
		}
	}
	return report, nil
}

// stagePlatformUpdatePayload writes the signed offer's files into the
// updater's root-owned incoming store under the finalized release digest.
// Bytes were already digest-verified against the manifest during offer
// validation; staging rewrites them deterministically for the updater's own
// verifyManifest pass.
func (rt *Runtime) stagePlatformUpdatePayload(offer selfdevprotocol.PlatformUpdateOffer, releaseDigest string) (string, error) {
	incomingDir := filepath.Join(rt.selfdevUpdaterRoot, "incoming", releaseDigest)
	_ = os.RemoveAll(incomingDir)
	if err := os.MkdirAll(incomingDir, 0o700); err != nil {
		return "", fmt.Errorf("platform update: incoming store: %w", err)
	}
	for _, file := range offer.Files {
		clean := filepath.Clean(file.Path)
		if filepath.IsAbs(clean) || clean == ".." || len(clean) >= 3 && clean[:3] == "../" {
			return "", fmt.Errorf("platform update: payload path %q escapes the incoming store", file.Path)
		}
		raw, err := base64.StdEncoding.DecodeString(file.Bytes)
		if err != nil {
			return "", fmt.Errorf("platform update: payload file %q does not decode", file.Path)
		}
		target := filepath.Join(incomingDir, clean)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, raw, fs.FileMode(file.Mode)&0o777); err != nil {
			return "", fmt.Errorf("platform update: stage %q: %w", file.Path, err)
		}
	}
	return incomingDir, nil
}

// recordPlatformUpdateFailed commits a materialization_failed event when the
// updater restored the prior release — the pending transition resolves back
// to the effective state, keeping the tape auditable.
func (rt *Runtime) recordPlatformUpdateFailed(ctx context.Context, offer selfdevprotocol.PlatformUpdateOffer, offerDigest, acceptedDigest string, result updater.ApplyResult, cause error) error {
	failedKey := platformUpdateIdempotencyKey("failed", offer.UpdateID)
	if _, found, lookupErr := rt.store.EventByIdempotency(ctx, offer.ComputerID, failedKey); lookupErr != nil {
		return lookupErr
	} else if found {
		return fmt.Errorf("platform update: apply failed and prior release was restored: %w", cause)
	}
	head, headErr := rt.store.Head(ctx, offer.ComputerID)
	if headErr != nil || head == nil {
		return fmt.Errorf("platform update: failed-transition head unavailable")
	}
	eventID, eventErr := computerevent.NewEventID()
	if eventErr != nil {
		return eventErr
	}
	resultPayload, _ := json.Marshal(result)
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: offer.ComputerID,
		EventKind: computerevent.EventMaterializationFailed, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: failedKey,
		ActorProfile:   agentprofile.Management, AuthorityRef: "guest-core:choir-updater",
		PrivacyClass:      "owner",
		ProposedEffectRef: offerDigest, DecisionRef: acceptedDigest,
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, _, err := rt.eventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{RestoredPriorEffective: true}, resultPayload, "application/vnd.choir.platform-update-result+json", "owner"); err != nil {
		return fmt.Errorf("platform update: failed event refused: %w", err)
	}
	return fmt.Errorf("platform update: apply failed and prior release was restored: %w", cause)
}

// HandleInternalPlatformUpdate serves POST /internal/runtime/platform-update —
// the guest apply endpoint the platform's signed offer reaches through the
// vmctl autoputer proxy. Internal-caller gated; the offer signature, not the
// transport, is the authority.
func (h *APIHandler) HandleInternalPlatformUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal runtime endpoints are not publicly accessible"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "platform update surface unavailable"})
		return
	}
	var request struct {
		Offer selfdevprotocol.PlatformUpdateOffer `json:"offer"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid platform update offer"})
		return
	}
	report, err := h.rt.ApplyPlatformUpdate(r.Context(), request.Offer)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrPlatformUpdateStaleHead) {
			status = http.StatusConflict
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, report)
}
