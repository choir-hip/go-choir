package agentcore

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
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

// triggerSelfDevelopmentReconcile is the M7 derivable-continuation wake: the
// appender's post-commit observer calls it once per committed canonical
// event. The pending flag coalesces bursts — the first trigger launches the
// drain, later triggers only leave the flag set, and the drain loops until a
// pass completes with the flag clear. The reconciler's own ListByStates
// query is the sole state gate.
func (rt *Runtime) triggerSelfDevelopmentReconcile() {
	if rt == nil || rt.selfdevOperations == nil || rt.eventAppender == nil || rt.selfdevComputerID == "" {
		return
	}
	if rt.selfdevReconcilePending.CompareAndSwap(false, true) {
		go rt.selfdevReconcileDrain()
	}
}

// selfdevReconcileDrain runs the reconciler until the pending flag stays
// clear — each pass covers every commit that preceded its state listing,
// so one drain collapses a commit burst into one settlement boundary.
func (rt *Runtime) selfdevReconcileDrain() {
	for rt.selfdevReconcilePending.Swap(false) {
		rt.reconcileSelfDevelopmentMaterialization(context.Background())
	}
}

// TriggerSelfDevelopmentReconcile is the derivable-continuation boundary for
// the materializer: durable retry wakes (selfdev_materialization_retry) and
// post-commit observers converge on this single non-blocking arm. Exported for
// actorruntime's deadline dispatch.
func (rt *Runtime) TriggerSelfDevelopmentReconcile() {
	rt.triggerSelfDevelopmentReconcile()
}

// PlannedRestartSelfDevelopmentApply is the planned-restart reason an apply
// writes before it restarts the guest.
const PlannedRestartSelfDevelopmentApply = "self_development_apply"

// SelfDevelopmentApplyMaterializing reports whether a self-development
// operation on this computer is still materializing. The actor handler holds
// work while it is, so the apply checkpoint reads a quiet chain. A read error
// reports false: work is never held on an unknown state.
func (rt *Runtime) SelfDevelopmentApplyMaterializing(ctx context.Context) bool {
	if rt == nil || rt.selfdevOperations == nil || rt.selfdevComputerID == "" {
		return false
	}
	operations, err := rt.selfdevOperations.ListByStates(ctx, rt.selfdevComputerID, selfdev.StateMaterializing)
	return err == nil && len(operations) > 0
}

func (rt *Runtime) reconcileSelfDevelopmentMaterialization(ctx context.Context) {
	if rt == nil || rt.maintenanceHeld() || rt.selfdevUpdater == nil || rt.selfdevVerifier == nil || rt.selfdevControl == nil || rt.selfdevRoute == nil || rt.selfdevRouteOwnerID == "" || rt.selfdevRouteDesktopID == "" || rt.selfdevComputerID == "" || rt.selfdevOperations == nil || rt.eventAppender == nil || rt.store == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || strings.TrimSpace(rt.selfdevRealizationID) == "" {
		return
	}
	rt.selfdevMaterializeMu.Lock()
	defer rt.selfdevMaterializeMu.Unlock()
	operations, err := rt.selfdevOperations.ListByStates(ctx, rt.selfdevComputerID, selfdev.StateAwaitingApproval, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateRollbackPending, selfdev.StateDegraded)
	if err != nil {
		return
	}
	for _, operation := range operations {
		if operation.State == selfdev.StateAwaitingApproval {
			recovered, found, recoveryErr := rt.recoverSelfDevelopmentDecision(ctx, operation)
			if recoveryErr != nil || !found {
				// Still at the owner-decision boundary (M7): mint the
				// observation claim addressed to the management desk so its
				// acting pack shows the op awaiting decision without an API
				// poll. Idempotent on the deterministic record id.
				rt.notifySelfDevelopmentDecisionBoundary(ctx, operation)
				continue
			}
			operation = recovered
			if operation.State == selfdev.StateRejected {
				continue
			}
		}
		var operationErr error
		if operation.State == selfdev.StateRollbackPending ||
			// A degraded operation carrying a route receipt completed a prior
			// promote and was lost mid-rollback; a degraded operation without
			// one was lost mid-apply. The route receipt — only ever written by
			// recordMaterializationApplied — discriminates the two.
			(operation.State == selfdev.StateDegraded && strings.TrimSpace(operation.RouteReceipt) != "") {
			operationErr = rt.rollbackSelfDevelopmentOperation(ctx, operation)
		} else {
			operationErr = rt.materializeSelfDevelopmentOperation(ctx, operation)
		}
		if operationErr != nil {
			// The durable operation and updater journal retain the recovery
			// point, but nothing else re-fires this reconciler unless another
			// commit happens — a silent failure here is a permanent wedge
			// (assessment wedge 4). Re-arm a durable retry wake addressed to
			// the management mailbox so the obligation survives the failed
			// pass. Idempotent: the retry handler re-runs the same drain,
			// which re-drives from the retained journal phase.
			log.Printf("selfdev materializer: operation %s in %s needs retry: %v",
				operation.OperationID, operation.State, operationErr)
			if rt.scheduleActor != nil {
				if armErr := rt.scheduleActor(context.Background(), rt.selfdevRouteOwnerID, rt.selfdevComputerID,
					"management:"+rt.selfdevRouteOwnerID, selfdevMaterializationRetryKind,
					operation.OperationID, "", "", time.Now().UTC().Add(60*time.Second)); armErr != nil {
					log.Printf("selfdev materializer: retry wake for %s: %v", operation.OperationID, armErr)
				}
			}
			continue
		}
	}
}
func (rt *Runtime) recoverSelfDevelopmentDecision(ctx context.Context, operation selfdev.Operation) (selfdev.Operation, bool, error) {
	transition, found, err := rt.store.FinalizedDecisionForOperation(ctx, operation.ComputerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if err != nil || !found {
		return operation, found, err
	}
	decision, err := verifyFinalizedSelfDevelopmentDecision(operation, transition)
	if err != nil {
		return operation, false, err
	}
	recovered, err := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, selfdev.StateAwaitingApproval, decision.NextState, func(next *selfdev.Operation) error {
		next.DecisionEvent = transition.Request.EventDigest
		next.DecisionReceipt = transition.Receipt.ReceiptID
		next.DecisionActor = decision.Actor
		next.DesiredHead = transition.Request.Next.DesiredEventHead
		next.EffectiveHead = transition.Request.Next.EffectiveEventHead
		next.ModeReceipt = decision.ModeReceiptDigest
		return nil
	})
	if err != nil {
		current, getErr := rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
		if getErr == nil && current.DecisionEvent == transition.Request.EventDigest &&
			current.DecisionReceipt == transition.Receipt.ReceiptID &&
			selfDevelopmentDecisionStateDescends(current.State, decision.NextState) {
			if _, verifyErr := verifyFinalizedSelfDevelopmentDecision(current, transition); verifyErr == nil {
				return current, true, nil
			}
		}
		return operation, false, err
	}
	return recovered, true, nil
}

func selfDevelopmentBundleMatchesOperation(bundle transaction.CapsuleEffectBundle, operation selfdev.Operation) bool {
	return bundle.ComputerID == operation.ComputerID &&
		bundle.TrajectoryRef == operation.TrajectoryID &&
		bundle.CapsuleIdentity == operation.CapsuleID &&
		bundle.BaseEventHead == operation.BaseHead
}

func (rt *Runtime) materializeSelfDevelopmentOperation(ctx context.Context, operation selfdev.Operation) error {
	bundlePath := filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest, "bundle.json")
	rawBundle, err := os.ReadFile(bundlePath)
	if err != nil || computerevent.DigestBytes(rawBundle) != operation.BundleDigest {
		return fmt.Errorf("materializer: frozen bundle unavailable")
	}
	var bundle transaction.CapsuleEffectBundle
	decoder := json.NewDecoder(strings.NewReader(string(rawBundle)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil || bundle.Validate(true) != nil ||
		!selfDevelopmentBundleMatchesOperation(bundle, operation) ||
		len(operation.VerifierRefs) == 0 || !selfDevelopmentContainsString(bundle.VerifierReceipts, operation.VerifierRefs[0]) {
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
				TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID, ActorProfile: agentprofile.Management,
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
	frozenAt, err := time.Parse(time.RFC3339Nano, operation.CreatedAt)
	if err != nil {
		return fmt.Errorf("materializer: invalid frozen operation time")
	}
	sourceTreeDigest := strings.TrimPrefix(bundle.SourceTreeRef, "source-tree:sha256:")
	closure, err := computerversion.NewCodeClosure(sourceTreeDigest, []computerversion.CodeArtifact{{
		Name: "capsule-effect-bundle.json", SHA256: operation.BundleDigest, URI: bundleURI,
	}}, frozenAt.UTC())
	if err != nil {
		return err
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "capsule_effect_bundle", ContentSHA256: operation.BundleDigest, ArtifactURI: bundleURI,
	}}, frozenAt.UTC())
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
	restartPlanned := rt.markPlannedRestart(PlannedRestartSelfDevelopmentApply, operation.OperationID)
	result, applyErr := rt.selfdevUpdater.Apply(ctx, applyRequest)
	restartPlanned(applyErr)
	if journaled, found, journalErr := updater.ReadJournalOutcome(rt.selfdevUpdaterRoot, applyRequest.IdempotencyKey); journalErr == nil && found && journaled.Terminal {
		// The journal is the durable authority: a terminal outcome supersedes
		// whatever the live Apply returned (including a transport error that
		// lost the response or an epoch-fenced replay refusal).
		result, applyErr = journaled.Result, nil
		if journaled.Result.Outcome != "applied" {
			applyErr = errors.New(journaled.Failure)
		}
	}
	ref, publicKey, keyErr := rt.selfdevUpdater.PublicKey(ctx)
	if keyErr != nil {
		return keyErr
	}
	resolver := updaterReceiptKeyResolver{ref: ref, key: publicKey}
	if applyErr == nil {
		if result.Outcome != "applied" || result.MaterializationReceipt.Verify(resolver) != nil || result.HealthReceipt.Verify(resolver) != nil {
			return fmt.Errorf("materializer: invalid applied receipts")
		}
		return rt.recordMaterializationApplied(ctx, operation, result, closure, program, computerevent.EventMaterializationApplied, operation.State, selfdev.StateApplied, routeledger.TransitionPromote, "computer:self_development:approve")
	}
	if result.RecoveryReceipt != nil && result.RecoveryReceipt.Verify(resolver) == nil {
		return rt.recordMaterializationFailed(ctx, operation, result, applyErr, operation.State)
	}
	// Only a typed refusal is terminal: the updater judged the request itself
	// invalid. Transport failures (daemon down, socket error) stay
	// materializing and the caller re-arms a retry wake — the journal may
	// already record a completed swap the response never delivered.
	if !errors.Is(applyErr, updater.ErrApplyRefused) {
		return applyErr
	}
	if operation.State == selfdev.StateDegraded {
		// Re-refused while degraded and the journal still has no terminal
		// outcome: nothing will repair this pass's verdict; staying silent
		// keeps the terminal row instead of arming a retry every 60s.
		return nil
	}
	_, transitionErr := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, selfdev.StateDegraded, func(next *selfdev.Operation) error {
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
	restartPlanned := rt.markPlannedRestart("self_development_rollback", operation.OperationID)
	result, applyErr := rt.selfdevUpdater.Apply(ctx, applyRequest)
	restartPlanned(applyErr)
	if journaled, found, journalErr := updater.ReadJournalOutcome(rt.selfdevUpdaterRoot, applyRequest.IdempotencyKey); journalErr == nil && found && journaled.Terminal {
		result, applyErr = journaled.Result, nil
		if journaled.Result.Outcome != "applied" {
			applyErr = errors.New(journaled.Failure)
		}
	}
	ref, publicKey, keyErr := rt.selfdevUpdater.PublicKey(ctx)
	if keyErr != nil {
		return keyErr
	}
	resolver := updaterReceiptKeyResolver{ref: ref, key: publicKey}
	if applyErr == nil {
		if result.Outcome != "applied" || result.MaterializationReceipt.Verify(resolver) != nil || result.HealthReceipt.Verify(resolver) != nil {
			return fmt.Errorf("materializer: invalid rollback receipts")
		}
		return rt.recordMaterializationApplied(ctx, operation, result, closure, program, computerevent.EventRollbackApplied, operation.State, selfdev.StateRolledBack, routeledger.TransitionRollback, "computer:self_development:rollback")
	}
	if result.RecoveryReceipt != nil && result.RecoveryReceipt.Verify(resolver) == nil {
		return rt.recordMaterializationFailed(ctx, operation, result, applyErr, operation.State)
	}
	if !errors.Is(applyErr, updater.ErrApplyRefused) {
		return applyErr
	}
	if operation.State == selfdev.StateDegraded {
		// Re-refused while degraded with a non-terminal journal: stay
		// terminal rather than arming a retry loop.
		return nil
	}
	_, transitionErr := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, selfdev.StateDegraded, func(next *selfdev.Operation) error {
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
			ActorProfile: agentprofile.Management, AuthorityRef: "guest-core:choir-updater", PrivacyClass: "owner",
			ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent,
			ResultingEffectiveCommitment: head.DesiredStateCommitment,
			ReducerVersion:               computerevent.ReducerVersionV1,
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
	// The applied event's own receipt stays bound to the approval/promotion
	// evidence below (the route join names the applied event, not the head).
	// The checkpoint request must pin the POST-APPEND head: every event on
	// this chain (including the projection batches the per-commit sweep keeps
	// minting) advances canonical_event_head, so a request that names the
	// applied event's digest fails the authority's FOR UPDATE equality check
	// on every retry — the wedge observed on computer-5352d5a8 2026-09-29.
	appliedEventHead, _ := eventReceipt.KindFields["event_digest"].(string)
	if !computerevent.IsSHA256(appliedEventHead) {
		return fmt.Errorf("materializer: applied event receipt is not head-bound")
	}
	checkpointHeadReceipt, found, err := rt.store.EventReceiptByDigest(ctx, operation.ComputerID, head.CanonicalEventHead)
	if err != nil || !found {
		return fmt.Errorf("materializer: current-head event receipt unavailable")
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
	// The certificate's evidence binding must name what the verifier actually
	// attested: the recorded verification payload carries the *draft* bundle
	// digest (pre-finalization) and the cited evidence refs, while the
	// operation row holds the post-verification finalized digest and the
	// verification event digest. The checkpoint authority joins certificate
	// fields against the payload verbatim; operation fields refuse
	// deterministically (wedge-10).
	evidence, err := rt.verificationEvidence(ctx, operation)
	if err != nil {
		return err
	}
	verifierCertificate, err := rt.selfdevVerifier.SignVerifierCertificate(ctx, selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: operation.ComputerID, OperationID: operation.OperationID,
		BundleDigest: evidence.VerifiedBundleDigest, VerificationEventDigest: operation.VerifierRefs[0],
		VerifierEvidenceRefs: evidence.Refs, DecisionEventHead: operation.DecisionEvent,
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
	pinned, _, err := updater.ReadPinnedManifest(rt.selfdevUpdaterRoot, result.ReleaseDigest)
	if err != nil {
		return fmt.Errorf("materializer: checkpoint release unavailable: %w", err)
	}
	witness, frontend, err := rt.checkpointRestoreBindings(ctx, operation.ComputerID, result.ReleaseDigest, pinned.Files)
	if err != nil {
		return err
	}
	// The authority CAS-gates on the live head, but arbitrary tape events
	// (per-boot key_revoked rotations, projection sweeps, concurrent cells)
	// keep advancing it between the head read above and publish. Re-read the
	// head + receipt + reconstruction digest and retry; the certificate is
	// bound to the decision event, not the current head, so republishing is
	// safe. Bounded at three attempts — a persistently-moving head means a
	// livelock loop, which is worse than surfacing the error.
	var checkpoint selfdevprotocol.CheckpointResponse
	published := false
	for attempt := 0; attempt < 3 && !published; attempt++ {
		if attempt > 0 {
			head, err = rt.store.Head(ctx, operation.ComputerID)
			if err != nil || head == nil {
				return fmt.Errorf("materializer: applied projection unavailable on retry")
			}
			checkpointHeadReceipt, found, err = rt.store.EventReceiptByDigest(ctx, operation.ComputerID, head.CanonicalEventHead)
			if err != nil || !found {
				return fmt.Errorf("materializer: current-head event receipt unavailable on retry")
			}
			reconstructionDigest, err = selfdevprotocol.Digest(struct {
				Version       computerversion.ComputerVersion `json:"computer_version"`
				EffectiveHead string                          `json:"effective_event_head"`
				ReleaseDigest string                          `json:"release_digest"`
			}{version, head.EffectiveEventHead, result.ReleaseDigest})
			if err != nil {
				return err
			}
		}
		checkpoint, err = rt.selfdevControl.PublishCheckpoint(ctx, selfdevprotocol.CheckpointRequest{
			ComputerID: operation.ComputerID, IdempotencyKey: "selfdev-checkpoint-" + operation.DecisionEvent + "-" + head.CanonicalEventHead[:16],
			ComputerVersion: version, AcceptedEventHead: head.CanonicalEventHead, EffectiveEventHead: head.EffectiveEventHead,
			EffectiveStateCommitment: head.EffectiveStateCommitment, EventHeadReceiptID: checkpointHeadReceipt.ReceiptID,
			ReleaseDigest: result.ReleaseDigest, ReconstructionDigest: reconstructionDigest,
			MaterializationReceiptDigest: receiptDigest, VerifierCertificateDigest: verifierDigest,
			VerifierCertificate: verifierCertificate, ReducerVersion: head.ReducerVersion,
			VMLocalContentWitness: witness, FrontendIdentity: frontend,
		})
		if err != nil && strings.Contains(err.Error(), "head CAS conflict") {
			continue
		}
		published = true
	}
	if !published {
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
			ActorProfile: agentprofile.Management, AuthorityRef: "platform-control:checkpoint",
			PayloadCommitment: computerevent.ZeroHead, PrivacyClass: "owner",
			ProposedEffectRef: checkpoint.Checkpoint.Digest, DecisionRef: operation.DecisionEvent, ReducerVersion: computerevent.ReducerVersionV1,
		}
		if _, eventErr = rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{}, nil); eventErr != nil {
			return eventErr
		}
	}
	// The route projection request must name the live canonical head: the
	// authority rejects any digest that is not the current head (and the
	// promotion join mirrors request.EventHeadReceiptID into the
	// certificate). Re-read after the checkpoint_published append instead of
	// pinning that event — any drift between append and publish makes a
	// pinned digest permanently stale, the same wedge class as the
	// checkpoint head binding above.
	routeHead, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || routeHead == nil {
		return fmt.Errorf("materializer: route projection head unavailable")
	}
	routeHeadReceipt, found, err := rt.store.EventReceiptByDigest(ctx, operation.ComputerID, routeHead.CanonicalEventHead)
	if err != nil || !found {
		return fmt.Errorf("materializer: route head event receipt unavailable")
	}
	routeSlotID, err := routeledger.RouteSlotID(rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID)
	if err != nil {
		return err
	}
	currentRoute, err := rt.selfdevRoute.ResolveComputerVersionRouteOrAbsent(ctx, routeSlotID)
	if err != nil {
		return err
	}
	routeIdempotency := routeledger.IdempotencyKey("idempotency:selfdev-route:" + operation.DecisionEvent)
	oldVersion, expectedGeneration := currentRoute.Slot.Current, currentRoute.Slot.Generation
	// A chain-bootstrapped computer has no route slot row: Resolve returns
	// RouteAbsent with no current version. The only legal first transition is
	// TransitionBootstrap — promote/rollback both require an existing slot.
	// Detect that here rather than letting the authority surface "route is
	// absent" as a terminal refusal.
	effectiveRouteKind := routeKind
	effectiveScope, effectiveActor := decisionScope, operation.DecisionActor
	if currentRoute.RouteAbsent {
		if routeKind == routeledger.TransitionRollback {
			return fmt.Errorf("materializer: cannot rollback to an absent route slot")
		}
		// Bootstrap is a platform-scope act: the self-dev projection endpoint
		// refuses TransitionBootstrap by design (vmctl/self_development_route.go),
		// and only ApplyPlatformFollowRouteProjection accepts it. The first
		// materialization of a chain-bootstrapped computer binds its initial
		// route under the platform-follow evidence class.
		effectiveRouteKind = routeledger.TransitionBootstrap
		effectiveScope = selfdevprotocol.PlatformUpdateFollowScope
		effectiveActor = selfdevprotocol.PlatformUpdateFollowActor
	}
	createdAt := checkpoint.Receipt.IssuedAt
	// RouteProjectionFromRequest recomputes this payload from the minted
	// checkpoint's request fields and refuses any byte deviation — bind it
	// from the published checkpoint verbatim, never from the head read.
	acceptedPayload := selfdevprotocol.AcceptedEventAuthorizationEvidence{
		Version: 1, ComputerID: operation.ComputerID, AcceptedOrRollbackEventDigest: checkpoint.Checkpoint.Request.AcceptedEventHead,
		EventHeadReceiptID: checkpoint.Checkpoint.Request.EventHeadReceiptID, EffectiveEventHead: checkpoint.Checkpoint.Request.EffectiveEventHead,
		OldComputerVersion: oldVersion, NewComputerVersion: version,
		DecisionActor: effectiveActor, DecisionScope: effectiveScope,
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
		Version: 1, ComputerID: operation.ComputerID, EventHeadReceiptID: routeHeadReceipt.ReceiptID,
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
		RouteSlotID: routeSlotID, Kind: effectiveRouteKind, Old: oldVersion, New: version,
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
		CanonicalEventHead: routeHead.CanonicalEventHead, EventHeadReceiptID: routeHeadReceipt.ReceiptID,
		ApprovalEvidence: approvalEvidence, PromotionEvidence: promotionEvidence, Command: command,
		DecisionActor: effectiveActor, DecisionScope: effectiveScope,
		ExpiresAt: authorizationWindow.Add(5 * time.Minute).Format(time.RFC3339Nano),
	}
	authorization, err := rt.selfdevControl.PublishRouteProjection(ctx, projectionRequest)
	if err != nil {
		return err
	}
	var route vmctl.RouteResolution
	if effectiveRouteKind == routeledger.TransitionBootstrap {
		route, err = rt.selfdevRoute.ApplyPlatformFollowRouteProjection(ctx, selfdevprotocol.ApplyRouteProjectionRequest{Projection: projectionRequest, Authorization: authorization})
	} else {
		route, err = rt.selfdevRoute.ApplySelfDevelopmentRouteProjection(ctx, selfdevprotocol.ApplyRouteProjectionRequest{Projection: projectionRequest, Authorization: authorization})
	}
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
			ActorProfile: agentprofile.Management, AuthorityRef: "vmctl:route-cas",
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

type selfdevVerificationEvidence struct {
	VerifiedBundleDigest string
	Refs                 []string
}

// verificationEvidence reads the recorded verification event's output
// payload and returns the exact fields the checkpoint authority joins on:
// the draft bundle digest the verifier signed off (pre-finalization) and
// the evidence refs it cited. The operation row's BundleDigest/VerifierRefs
// diverge deliberately — finalization rewrites the bundle (adding the
// verifier receipt) and VerifierRefs stores the event digest — so the
// recorded payload is the only sound certificate source.
func (rt *Runtime) verificationEvidence(ctx context.Context, operation selfdev.Operation) (selfdevVerificationEvidence, error) {
	if rt.eventPayloadReader == nil {
		return selfdevVerificationEvidence{}, fmt.Errorf("materializer: event payload reader unavailable")
	}
	event, found, err := rt.store.EventByDigest(ctx, operation.ComputerID, operation.VerifierRefs[0])
	if err != nil || !found || event.EventKind != computerevent.EventVerificationRecorded || len(event.OutputArtifactRefs) != 1 {
		return selfdevVerificationEvidence{}, fmt.Errorf("materializer: verification event unavailable")
	}
	payloadRef, err := computerevent.ParseArtifactRef(event.OutputArtifactRefs[0])
	if err != nil {
		return selfdevVerificationEvidence{}, err
	}
	raw, err := rt.eventPayloadReader.FetchPayload(ctx, operation.ComputerID, payloadRef.Digest().String())
	if err != nil {
		return selfdevVerificationEvidence{}, fmt.Errorf("materializer: verification payload unavailable: %w", err)
	}
	var payload struct {
		OperationID   string   `json:"operation_id"`
		BundleDigest  string   `json:"bundle_digest"`
		Decision      string   `json:"decision"`
		VerifierRefs  []string `json:"verifier_refs"`
		VerifierRunID string   `json:"verifier_run_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.OperationID != operation.OperationID ||
		payload.Decision != "pass" || len(payload.VerifierRefs) == 0 || payload.VerifierRunID == "" ||
		!computerevent.IsSHA256(payload.BundleDigest) {
		return selfdevVerificationEvidence{}, fmt.Errorf("materializer: verification payload refused")
	}
	return selfdevVerificationEvidence{VerifiedBundleDigest: payload.BundleDigest, Refs: payload.VerifierRefs}, nil
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
			ActorProfile: agentprofile.Management, AuthorityRef: "guest-core:choir-updater", PrivacyClass: "owner",
			ProposedEffectRef: operation.BundleDigest, DecisionRef: operation.DecisionEvent,
			ReducerVersion: computerevent.ReducerVersionV1,
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

// selfdevBoundaryObserverAgentID is the committing-agent identity stamped on
// reconciler-minted boundary records — system-committed acts addressed to the
// management desk, distinct from any cell-authored provenance.
const selfdevBoundaryObserverAgentID = "selfdev-reconciler"

// notifySelfDevelopmentDecisionBoundary mints the management observation of
// an op parked at the owner-decision boundary (M7): a score-free commitment
// record addressed to the computer's management desk so its acting pack
// shows the pending decision without an API poll. The deterministic record
// id makes re-emission a no-op; emission is observational — a ledger failure
// is logged, never fatal to the reconcile pass.
func (rt *Runtime) notifySelfDevelopmentDecisionBoundary(ctx context.Context, operation selfdev.Operation) {
	if rt == nil || rt.store == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rec := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    fmt.Sprintf("selfdev-observation:%s:awaiting-decision", operation.OperationID),
		Discrepancy: types.DiscrepancyUnresolved,
		Prediction: types.CommitmentPrediction{
			Hypothesis:  fmt.Sprintf("self-development operation %s (trajectory %s, capsule %s) is verified and awaits the owner decision boundary", operation.OperationID, operation.TrajectoryID, operation.CapsuleID),
			CommittedAt: now,
		},
		Observation: types.CommitmentObservation{
			Excerpt:    fmt.Sprintf("operation %s verified frozen bundle %s; no decided transition projected yet", operation.OperationID, operation.BundleDigest),
			SourceRef:  operation.OperationID,
			ObservedAt: now,
		},
		Provenance: types.CommitmentProvenance{
			AgentID:     selfdevBoundaryObserverAgentID,
			ContextRef:  operation.TrajectoryID,
			CommittedAt: now,
		},
		Addressee:    persistentManagementAgentID(rt.selfdevRouteOwnerID),
		EvidenceRefs: []string{operation.OperationID},
	}
	if _, err := rt.store.AppendCommitmentRecord(ctx, rt.selfdevRouteOwnerID, operation.ComputerID, rec); err != nil {
		log.Printf("selfdev reconcile: decision-boundary observation for %s: %v", operation.OperationID, err)
	}
}
