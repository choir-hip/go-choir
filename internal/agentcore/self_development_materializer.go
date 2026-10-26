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

// selfdevInterruptedByRestart is the visible fate of an operation whose desk
// run a crash boot closed.
const selfdevInterruptedByRestart = "interrupted by a restart"

// closeSelfDevelopmentOperationsAfterCrash fails every pre-decision operation
// at a crash boot (owner rule, AGENTS.md "Restarts End Work (Crash) Or Resume
// It"). Desk tools drive requested through verified; the crash boot closes
// that desk run, so nothing would advance the operation again, and the busy
// probe would count it forever
// (docs/problems/selfdev-zombie-operations-pin-owner-computer-2026-10-10.md).
// awaiting_approval waits on the owner and stays. The post-decision states,
// and ops whose owner decision committed before the crash, are marked
// interrupted: the materializer settles them from the updater journal and
// never applies them (L1a-3b). A planned boot closes nothing.
func (rt *Runtime) closeSelfDevelopmentOperationsAfterCrash(ctx context.Context) {
	if rt == nil || rt.selfdevOperations == nil {
		return
	}
	if _, planned := rt.BootWasPlannedRestart(); planned {
		return
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" {
		return
	}
	rt.markSelfDevelopmentOperationsInterruptedAfterCrash(ctx)
	operations, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.PreDecisionStates...)
	if err != nil {
		log.Printf("selfdev: crash boot: list pre-decision operations: %v", err)
		return
	}
	for _, operation := range operations {
		if _, err := rt.selfdevOperations.Transition(ctx, computerID, operation.OperationID, operation.State, selfdev.StateFailed, func(next *selfdev.Operation) error {
			next.TerminalError = selfdevInterruptedByRestart
			return nil
		}); err != nil {
			log.Printf("selfdev: crash boot: close %s from %s: %v", operation.OperationID, operation.State, err)
			continue
		}
		log.Printf("selfdev: crash boot: %s closed from %s: %s", operation.OperationID, operation.State, selfdevInterruptedByRestart)
	}
}

// selfdevApplyInterruptedByRestart is the updater-result outcome a crash boot
// records for a decided operation the updater never started.
const selfdevApplyInterruptedByRestart = "interrupted_by_restart"

var errSelfDevJournalMidFlight = errors.New("updater journal is mid-flight; effective state unknown")

// markSelfDevelopmentOperationsInterruptedAfterCrash records the operations a
// crash boot found decided but unsettled: every post-decision state, and ops
// awaiting approval whose owner decision already committed. An op the owner
// decides after this boot is new work and materializes normally.
func (rt *Runtime) markSelfDevelopmentOperationsInterruptedAfterCrash(ctx context.Context) {
	computerID := strings.TrimSpace(rt.selfdevComputerID)
	if computerID == "" {
		computerID = strings.TrimSpace(rt.TextureComputerID())
	}
	if computerID == "" {
		return
	}
	operations, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateAwaitingApproval, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateRollbackPending, selfdev.StateDegraded)
	if err != nil {
		log.Printf("selfdev: crash boot: list post-decision operations: %v", err)
		return
	}
	interrupted := make(map[string]bool, len(operations))
	for _, operation := range operations {
		if operation.State == selfdev.StateAwaitingApproval {
			if rt.store == nil {
				continue
			}
			if _, found, lookupErr := rt.store.FinalizedDecisionForOperation(ctx, operation.ComputerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID); lookupErr != nil || !found {
				continue
			}
		}
		interrupted[operation.OperationID] = true
		log.Printf("selfdev: crash boot: %s (%s) interrupted; settles from the updater journal, never re-applied", operation.OperationID, operation.State)
	}
	rt.selfdevMaterializeMu.Lock()
	rt.selfdevCrashInterrupted = interrupted
	rt.selfdevSettlementSpent = map[string]bool{}
	rt.selfdevMaterializeMu.Unlock()
}

// settleSelfDevelopmentOperationAfterCrash closes an interrupted operation
// with what the updater journal says happened, without calling the updater:
// no journal means nothing changed (failed, interrupted by a restart); a
// terminal journal is recorded through the normal path; a mid-flight journal
// holds the operation, visible, for an owner act.
func (rt *Runtime) settleSelfDevelopmentOperationAfterCrash(ctx context.Context, operation selfdev.Operation, rollback bool) error {
	key := "selfdev-apply-" + operation.DecisionEvent
	if rollback {
		key = "selfdev-rollback-apply-" + operation.DecisionEvent
	}
	journaled, found, err := updater.ReadJournalOutcome(rt.selfdevUpdaterRoot, key)
	switch {
	case err != nil:
		return fmt.Errorf("updater journal unreadable: %w", err)
	case !found && operation.State == selfdev.StateDegraded:
		return nil // closed degraded already (the settlement crashed twice): an owner act decides
	case !found:
		return rt.recordMaterializationFailed(ctx, operation, updater.ApplyResult{Outcome: selfdevApplyInterruptedByRestart}, errors.New(selfdevInterruptedByRestart), operation.State)
	case !journaled.Terminal:
		return errSelfDevJournalMidFlight
	}
	// A terminal journal is recorded through the normal path, which binds a
	// checkpoint and promotes the route. The step is bounded: a settlement
	// that started on two crash boots and never finished closes the operation
	// instead of running a third time.
	if spentErr := rt.spendSelfDevelopmentSettlementAttempt(operation.OperationID); spentErr != nil {
		if errors.Is(spentErr, ErrGuestApplyAttemptsSpent) {
			return rt.closeSelfDevelopmentSettlementDegraded(ctx, operation)
		}
		return spentErr
	}
	if rollback {
		return rt.rollbackSelfDevelopmentOperation(ctx, operation, true)
	}
	return rt.materializeSelfDevelopmentOperation(ctx, operation, true)
}

// selfdevSettlementAttemptReason and selfdevSettlementAttemptLimit bound the
// crash-boot settlement of one operation across boots
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.5): the
// attempt is spent durably before the work, so a settlement that crashes the
// computer is counted, and the second crash is the last.
const (
	selfdevSettlementAttemptReason = "self_development_settlement"
	selfdevSettlementAttemptLimit  = 2
)

// selfdevSettlementUnfinished is the cause on an operation whose settlement
// started on two crash boots and did not finish on either. The count is starts
// (a boot that crashed, or one the base source or control plane failed), so the
// cause says what was counted, not that the computer crashed. last is the state
// the operation was in when the bound closed it.
func selfdevSettlementUnfinished(last string) string {
	return fmt.Sprintf("the settlement did not finish on two boots (last: %s): the apply is recorded on the tape, its checkpoint and route promotion are not; an owner act (rollback, or a manual re-bind) decides", last)
}

// selfdevSettlementUnfinishedNothingInstalled is the cause on an operation
// closed before it installed anything.
func selfdevSettlementUnfinishedNothingInstalled(last string) string {
	return fmt.Sprintf("the settlement did not finish on two boots (last: %s): nothing was installed; the operation is failed", last)
}

// spendSelfDevelopmentSettlementAttempt durably counts this boot's start of
// the operation's settlement, once per boot: every commit triggers a
// reconcile pass, and a pass that retries after an error is not a crash. It
// returns ErrGuestApplyAttemptsSpent once the limit is spent.
func (rt *Runtime) spendSelfDevelopmentSettlementAttempt(operationID string) error {
	if rt.selfdevSettlementSpent[operationID] {
		return nil
	}
	if err := spendSettlementAttempt(rt.cfg.StorePath, selfdevSettlementAttemptReason, operationID, selfdevSettlementAttemptLimit); err != nil {
		return err
	}
	if rt.selfdevSettlementSpent == nil {
		rt.selfdevSettlementSpent = map[string]bool{}
	}
	rt.selfdevSettlementSpent[operationID] = true
	return nil
}

// closeSelfDevelopmentSettlementDegraded closes an operation whose settlement
// started on two crash boots without finishing. An operation that reached
// materializing or later has installed something: it closes `degraded`, which
// means applied with an unsettled tail and stays open to an owner act; the
// applied release and the materialization-applied event stay, no checkpoint is
// bound and the route is not promoted. An operation still `accepted` installed
// nothing and has no accepted -> degraded edge: it settles `failed`. An
// operation already degraded keeps the cause it has.
func (rt *Runtime) closeSelfDevelopmentSettlementDegraded(ctx context.Context, operation selfdev.Operation) error {
	switch operation.State {
	case selfdev.StateDegraded:
		return nil
	case selfdev.StateAccepted:
		// recordMaterializationFailed also discharges the accepted
		// transition on the tape, as the unstarted-apply settlement does.
		cause := selfdevSettlementUnfinishedNothingInstalled(operation.State)
		if err := rt.recordMaterializationFailed(ctx, operation, updater.ApplyResult{Outcome: selfdevApplyInterruptedByRestart}, errors.New(cause), operation.State); err != nil {
			return fmt.Errorf("fail the accepted operation whose settlement did not finish: %w", err)
		}
		log.Printf("selfdev: crash boot: %s failed from %s: %s", operation.OperationID, operation.State, cause)
		return nil
	}
	cause := selfdevSettlementUnfinished(operation.State)
	if _, err := rt.selfdevOperations.Transition(ctx, operation.ComputerID, operation.OperationID, operation.State, selfdev.StateDegraded, func(next *selfdev.Operation) error {
		next.TerminalError = cause
		return nil
	}); err != nil {
		return fmt.Errorf("close the operation whose settlement did not finish: %w", err)
	}
	log.Printf("selfdev: crash boot: %s closed degraded from %s: %s", operation.OperationID, operation.State, cause)
	return nil
}

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
		// A degraded operation carrying a route receipt completed a prior
		// promote and was lost mid-rollback; a degraded operation without one
		// was lost mid-apply. The route receipt — only ever written by
		// recordMaterializationApplied — discriminates the two.
		rollback := operation.State == selfdev.StateRollbackPending ||
			(operation.State == selfdev.StateDegraded && strings.TrimSpace(operation.RouteReceipt) != "")
		if rt.selfdevCrashInterrupted[operation.OperationID] {
			// Decided before a crash boot: settle from the journal, never
			// apply, and arm no retry (a retry would only settle again).
			if settleErr := rt.settleSelfDevelopmentOperationAfterCrash(ctx, operation, rollback); settleErr != nil {
				log.Printf("selfdev materializer: operation %s in %s interrupted by a restart, held: %v",
					operation.OperationID, operation.State, settleErr)
			}
			continue
		}
		var operationErr error
		if rollback {
			operationErr = rt.rollbackSelfDevelopmentOperation(ctx, operation, false)
		} else {
			operationErr = rt.materializeSelfDevelopmentOperation(ctx, operation, false)
		}
		if errors.Is(operationErr, ErrGuestApplyAttemptsSpent) {
			// The apply crashed this guest every time it ran; retrying it
			// is the loop the attempt bound exists to stop. The operation
			// stays in its state, visible, for an owner act.
			log.Printf("selfdev materializer: operation %s in %s held: %v",
				operation.OperationID, operation.State, operationErr)
			continue
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
		mismatch := "get (" + fmt.Sprint(getErr) + ")"
		if getErr == nil {
			mismatch = selfDevelopmentDecisionNotCarried(current, transition, decision.NextState)
		}
		if mismatch == "" {
			return current, true, nil
		}
		if errors.Is(err, selfdev.ErrConflict) {
			log.Printf("selfdev decision recovery: operation %s lost its transition and the operation does not carry the finalized decision: %s mismatch (operation state=%q decision_event=%q decision_receipt=%q; decision event=%q receipt=%q next_state=%q): %v",
				operation.OperationID, mismatch, current.State, current.DecisionEvent, current.DecisionReceipt,
				transition.Request.EventDigest, transition.Receipt.ReceiptID, decision.NextState, err)
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

// journalOnly records the updater journal's terminal outcome without calling
// the updater (crash-boot settlement, L1a-3b).
func (rt *Runtime) materializeSelfDevelopmentOperation(ctx context.Context, operation selfdev.Operation, journalOnly bool) error {
	bundlePath := filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest, "bundle.json")
	rawBundle, err := os.ReadFile(bundlePath)
	if err != nil {
		return fmt.Errorf("materializer: frozen bundle unavailable")
	}
	if computerevent.DigestBytes(rawBundle) != operation.BundleDigest {
		return fmt.Errorf("materializer: frozen bundle digest does not match the operation")
	}
	var bundle transaction.CapsuleEffectBundle
	decoder := json.NewDecoder(strings.NewReader(string(rawBundle)))
	decoder.DisallowUnknownFields()
	switch {
	case decoder.Decode(&bundle) != nil:
		return fmt.Errorf("materializer: invalid frozen bundle: does not decode")
	case bundle.Validate(true) != nil:
		return fmt.Errorf("materializer: invalid frozen bundle: incomplete")
	case !selfDevelopmentBundleMatchesOperation(bundle, operation):
		return fmt.Errorf("materializer: invalid frozen bundle: names another computer, trajectory, capsule or base")
	case len(operation.VerifierRefs) == 0 || !selfDevelopmentContainsString(bundle.VerifierReceipts, operation.VerifierRefs[0]):
		return fmt.Errorf("materializer: invalid frozen bundle: does not carry the operation's verifier receipt")
	}

	// P5: a candidate that touched Go input runs only as a host-built layered
	// release. The host builds while the operation stays accepted (desks keep
	// working); a failed build fails the operation and changes nothing. A
	// crash-boot settlement reads the journal only and never asks the host
	// (G9).
	//
	// P4 (docs/problems/p4-standing-consensus-autonomy-2026-10-15.md §5): an
	// operation a consensus decision accepted applies only as the host build
	// corpusd voted on, frontend-only or not; the capsule's files are never
	// its release.
	var hostBuilt *hostBuiltRelease
	hostBuildRequired := false
	if !journalOnly {
		consensusDecided, decidedErr := rt.selfDevelopmentDecidedByConsensus(ctx, operation)
		if decidedErr != nil {
			return decidedErr
		}
		hostBuildRequired = consensusDecided || len(selfDevelopmentBundleBinaryInputs(bundle)) > 0
	}
	if hostBuildRequired {
		built, buildErr := rt.selfDevelopmentHostBuild(ctx, operation, bundle)
		var failure hostBuildFailure
		switch {
		case buildErr == nil:
			hostBuilt = &built
		case errors.As(buildErr, &failure) && operation.State == selfdev.StateAccepted:
			log.Printf("selfdev materializer: operation %s: %v; the computer keeps its current release", operation.OperationID, failure)
			return rt.recordMaterializationFailed(ctx, operation, updater.ApplyResult{Outcome: "host_build_failed"}, failure, operation.State)
		default:
			return buildErr
		}
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
	release := updater.ReleaseManifest{
		Version: updater.ManifestVersion, ComputerID: operation.ComputerID, AcceptedEventHead: operation.DecisionEvent,
		CodeRef: string(version.CodeRef), ArtifactProgramRef: string(version.ArtifactProgramRef),
		EventSchemaVersion: computerevent.SchemaVersionV1, ReducerVersion: computerevent.ReducerVersionV1,
		Marker: "selfdev-" + operation.BundleDigest[:16], Files: manifestFiles,
	}
	// The plain release joins its base: the base commit the candidate was
	// authored against is the manifest's BaseCommit, and the updater refuses
	// the apply when it is not the booted base. Not gated on any switch (a
	// security provenance check; docs/problems/
	// selfdev-apply-has-no-base-join-2026-10-26.md). A host-built release
	// takes the host's recorded base below.
	sourceDir := filepath.Dir(bundlePath)
	if hostBuilt != nil {
		// The layered release: the host's closure, receipts and the layer's
		// own frontend. The capsule-built binary is not a release input.
		release = layeredSelfDevelopmentManifest(release, *hostBuilt)
		sourceDir = hostBuilt.Dir
	} else {
		baseCommit, baseErr := rt.plainReleaseBaseCommit(operation, bundle)
		switch {
		case baseErr == nil:
			release.BaseCommit = baseCommit
		case journalOnly:
			// A crash-boot settlement reads the journal, which is the
			// authority; the claim only shapes a request it never sends.
			log.Printf("selfdev materializer: operation %s: %v (journal-only settlement continues)", operation.OperationID, baseErr)
		default:
			// The join cannot be proven: fail closed before the updater is
			// asked. Nothing was installed, so the operation settles failed.
			return rt.settleSelfDevelopmentRefusal(ctx, operation, baseErr)
		}
	}
	manifest, err := updater.FinalizeManifest(release)
	if err != nil {
		return err
	}
	applyRequest := updater.ApplyRequest{
		ComputerID: operation.ComputerID, RealizationID: rt.selfdevRealizationID, OperationID: operation.OperationID,
		IdempotencyKey: "selfdev-apply-" + operation.DecisionEvent, AcceptedEventHead: operation.DecisionEvent,
		SourceDir: sourceDir, Manifest: manifest,
	}
	applyRequest.RequestCommitment, err = updater.ComputeApplyRequestCommitment(applyRequest)
	if err != nil {
		return err
	}
	result, applyErr, err := rt.applyOrReadJournal(ctx, applyRequest, PlannedRestartSelfDevelopmentApply, operation.OperationID, journalOnly)
	if err != nil {
		return err
	}
	if journaled, found, journalErr := updater.ReadJournalOutcome(rt.selfdevUpdaterRoot, applyRequest.IdempotencyKey); journalErr == nil && found && journaled.Terminal {
		// The journal is the durable authority: a terminal outcome supersedes
		// whatever the live Apply returned (including a transport error that
		// lost the response or an epoch-fenced replay refusal).
		result, applyErr = journaled.Result, nil
		switch journaled.Result.Outcome {
		case "applied":
		case "refused":
			// A pre-mutation refusal keeps its type: the branch below settles
			// the operation degraded with the refusal as its reason. Losing
			// the type left it materializing, desks held, replaying the same
			// journaled refusal (docs/problems/
			// selfdev-apply-has-no-base-join-2026-10-26.md).
			applyErr = fmt.Errorf("%w: %s", updater.ErrApplyRefused, journaled.Failure)
		default:
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
		if err := rt.recordMaterializationApplied(ctx, operation, result, closure, program, computerevent.EventMaterializationApplied, operation.State, selfdev.StateApplied, routeledger.TransitionPromote, "computer:self_development:approve", ""); err != nil {
			return err
		}
		// A live apply wakes the candidate's Texture (SD-08). A crash
		// boot's journal-only settlement does not: a restart never starts
		// a Texture turn.
		if !journalOnly {
			rt.notifyTextureOfOwnerAct(ctx, rt.selfdevRouteOwnerID, operation, types.OwnerActApply, "")
		}
		return nil
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
	return rt.settleSelfDevelopmentRefusal(ctx, operation, applyErr)
}

// selfdevReasonBaseUnverifiable is the typed reason on an apply refused
// because the booted base is named by a manifest that cannot be read or
// carries no valid build commit: the join cannot be proven, so it fails closed.
const selfdevReasonBaseUnverifiable = "base_unverifiable"

// settleSelfDevelopmentRefusal settles an operation whose apply was refused
// before any mutation. A typed refusal installs nothing, so the operation is
// failed, a settled state: `degraded` means "applied, with an unsettled tail"
// and stays open (selfdev.OpenStates), so settling there would hold every new
// round and engineering admission forever. The failure is recorded like any
// other failed materialization (a materialization_failed event discharges the
// pending transition; the prior effective state trivially holds), with the
// refusal's own text, which names both commits for a moved base, as the
// operation's reason (docs/problems/
// selfdev-apply-has-no-base-join-2026-10-26.md, review 311).
func (rt *Runtime) settleSelfDevelopmentRefusal(ctx context.Context, operation selfdev.Operation, refusal error) error {
	log.Printf("selfdev materializer: operation %s refused before any change, settled failed: %v", operation.OperationID, refusal)
	return rt.recordMaterializationFailed(ctx, operation, updater.ApplyResult{Outcome: "refused"}, refusal, operation.State)
}

// plainReleaseBaseCommit is the base commit a plain (not host-built) release
// declares: the one its frozen bundle's source patch was authored against, or
// "" when no claim applies, each case named:
//   - the bundle records no source patch: it has no base to join (the
//     candidate is not derived from source), so the release claims none;
//   - no booted base image manifest is wired, or it names a dev build
//     (build_commit=local, or a dirty tree): nothing can be proven against
//     it, and refusing would refuse every candidate on that build, so the
//     release claims none;
//   - a manifest is wired but cannot be read, or carries no valid build
//     commit: that is not a dev build, it is a join that cannot be proven, so
//     it fails closed with the typed reason base_unverifiable (an error
//     wrapping updater.ErrApplyRefused, which settles the operation failed);
//   - otherwise the base is claimed exactly as recorded (lower-cased). A
//     short or malformed commit is claimed too, so the updater refuses it
//     with the reason rather than a prefix match passing it.
func (rt *Runtime) plainReleaseBaseCommit(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle) (string, error) {
	base := strings.ToLower(strings.TrimSpace(bundle.SourcePatchBaseCommit))
	if base == "" {
		log.Printf("selfdev materializer: operation %s: the bundle records no source patch base; the release claims no base commit", operation.OperationID)
		return "", nil
	}
	booted, err := rt.bootedBaseForJoin()
	if err != nil {
		return "", fmt.Errorf("%w: %s: %v", updater.ErrApplyRefused, selfdevReasonBaseUnverifiable, err)
	}
	if booted == "" {
		log.Printf("selfdev materializer: operation %s: no booted base manifest wired, or a dev build; the release claims no base commit", operation.OperationID)
		return "", nil
	}
	return base, nil
}

// rollbackApplyManifest is the manifest a rollback applies: the pinned prior
// release re-finalized under the rollback's decision event.
func rollbackApplyManifest(pinned updater.ReleaseManifest, decisionEvent string) (updater.ReleaseManifest, error) {
	pinned.AcceptedEventHead = decisionEvent
	if rollbackDropsBaseClaim(pinned) {
		// A plain release's base claim is a freeze-time provenance check on
		// installing a new candidate. A rollback returns to a release the
		// computer already ran and is never refused by it. A layered
		// release keeps the base it was built on: its closure resolves
		// against that base.
		pinned.BaseCommit = ""
	}
	pinned.ContentDigest = ""
	return updater.FinalizeManifest(pinned)
}

// rollbackDropsBaseClaim reports whether rolling back to the pinned release
// drops a base claim: a plain release that carries one.
func rollbackDropsBaseClaim(pinned updater.ReleaseManifest) bool {
	return pinned.BaseCommit != "" && pinned.ClosureDigest == "" && pinned.BaseImageManifestDigest == "" && pinned.LayeringEntrypoint == ""
}

// rollbackDroppedClaimNote is the operation's terminal result when a rollback
// to a plain release dropped that release's base claim by design. The rolled
// back computer runs a release authored on a base that may no longer be the
// booted one; the note makes that visible (review 311).
func rollbackDroppedClaimNote(pinned updater.ReleaseManifest) string {
	return "rolled back to a plain release; its base claim dropped by design (base commit " + pinned.BaseCommit + " is not joined to the booted base)"
}

func (rt *Runtime) rollbackSelfDevelopmentOperation(ctx context.Context, operation selfdev.Operation, journalOnly bool) error {
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
	terminalNote := ""
	if rollbackDropsBaseClaim(manifest) {
		terminalNote = rollbackDroppedClaimNote(manifest)
	}
	manifest, err = rollbackApplyManifest(manifest, operation.DecisionEvent)
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
	result, applyErr, err := rt.applyOrReadJournal(ctx, applyRequest, "self_development_rollback", operation.OperationID, journalOnly)
	if err != nil {
		return err
	}
	if journaled, found, journalErr := updater.ReadJournalOutcome(rt.selfdevUpdaterRoot, applyRequest.IdempotencyKey); journalErr == nil && found && journaled.Terminal {
		result, applyErr = journaled.Result, nil
		switch journaled.Result.Outcome {
		case "applied":
		case "refused":
			// Keep the refusal's type, as the apply path does: replacing it
			// with a plain error left the rollback pending, re-arming a retry
			// wake every 60 s and holding every deploy (review 311).
			applyErr = fmt.Errorf("%w: %s", updater.ErrApplyRefused, journaled.Failure)
		default:
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
		return rt.recordMaterializationApplied(ctx, operation, result, closure, program, computerevent.EventRollbackApplied, operation.State, selfdev.StateRolledBack, routeledger.TransitionRollback, "computer:self_development:rollback", terminalNote)
	}
	if result.RecoveryReceipt != nil && result.RecoveryReceipt.Verify(resolver) == nil {
		return rt.recordMaterializationFailed(ctx, operation, result, applyErr, operation.State)
	}
	if !errors.Is(applyErr, updater.ErrApplyRefused) {
		return applyErr
	}
	// A refused rollback installed nothing: it settles failed, like a refused
	// apply, and the applied release it was to leave stays as it was.
	return rt.settleSelfDevelopmentRefusal(ctx, operation, applyErr)
}

func (rt *Runtime) recordMaterializationApplied(ctx context.Context, operation selfdev.Operation, result updater.ApplyResult, closure computerversion.CodeClosure, program computerversion.ArtifactProgram, eventKind computerevent.EventKind, expectedState, nextState string, routeKind routeledger.TransitionKind, decisionScope, terminalNote string) error {
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
	if head, err := rt.store.Head(ctx, operation.ComputerID); err != nil || head == nil {
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
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
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
		Version: 1, ComputerID: operation.ComputerID, OperationID: evidence.OperationID,
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
	// The authority CAS-gates on the live head, and desks, projection sweeps
	// and key rotations keep advancing it. The checkpoint names the head its
	// witness was taken at, read and published while appends are held
	// (bindCheckpointAtHead); the certificate is bound to the decision event,
	// not the head, so a re-bind republishes it safely.
	_, published, err := rt.bindCheckpointAtHead(ctx, operation.ComputerID, result.ReleaseDigest, pinned.Files, func(ctx context.Context, binding checkpointBinding) (selfdevprotocol.CheckpointResponse, error) {
		head := binding.Head
		if head == nil || !computerevent.IsSHA256(head.CanonicalEventHead) {
			return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("materializer: applied projection unavailable")
		}
		checkpointHeadReceipt, found, err := rt.store.EventReceiptByDigest(ctx, operation.ComputerID, head.CanonicalEventHead)
		if err != nil || !found {
			return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("materializer: current-head event receipt unavailable")
		}
		reconstructionDigest, err := selfdevprotocol.Digest(struct {
			Version       computerversion.ComputerVersion `json:"computer_version"`
			EffectiveHead string                          `json:"effective_event_head"`
			ReleaseDigest string                          `json:"release_digest"`
		}{version, head.EffectiveEventHead, result.ReleaseDigest})
		if err != nil {
			return selfdevprotocol.CheckpointResponse{}, err
		}
		return rt.selfdevControl.PublishCheckpoint(ctx, selfdevprotocol.CheckpointRequest{
			ComputerID: operation.ComputerID, IdempotencyKey: "selfdev-checkpoint-" + operation.DecisionEvent + "-" + head.CanonicalEventHead[:16],
			ComputerVersion: version, AcceptedEventHead: head.CanonicalEventHead, EffectiveEventHead: head.EffectiveEventHead,
			EffectiveStateCommitment: head.EffectiveStateCommitment, EventHeadReceiptID: checkpointHeadReceipt.ReceiptID,
			ReleaseDigest: result.ReleaseDigest, ReconstructionDigest: reconstructionDigest,
			MaterializationReceiptDigest: receiptDigest, VerifierCertificateDigest: verifierDigest,
			VerifierCertificate: verifierCertificate, ReducerVersion: head.ReducerVersion,
			VMLocalContentWitness: binding.Witness, FrontendIdentity: binding.Frontend,
		})
	})
	if err != nil {
		return err
	}
	checkpoint := *published
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
		if terminalNote != "" {
			next.TerminalError = terminalNote
		}
		return nil
	})
	if err == nil {
		// U1: the source repository follows the release that just became
		// effective. A trigger only; the cast gate and boot reconcile again.
		rt.reconcileSourceProjectionLogged(ctx, string(eventKind))
	}
	return err
}

type selfdevVerificationEvidence struct {
	// OperationID is the operation the recorded payload names, which the
	// verifier certificate must carry (the checkpoint authority joins it
	// verbatim). It is the operation's own id, except for a rollback, whose
	// payload names the operation that was verified, the rollback's target.
	OperationID          string
	VerifiedBundleDigest string
	Refs                 []string
}

// rollbackTargetHops bounds how far a rollback follows rollback targets back
// to the operation its verification payload names (a rollback to a rollback
// to ...). A longer chain is refused.
const rollbackTargetHops = 16

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
		SchemaVersion int      `json:"schema_version"`
		OperationID   string   `json:"operation_id"`
		BundleDigest  string   `json:"bundle_digest"`
		Decision      string   `json:"decision"`
		VerifierRefs  []string `json:"verifier_refs"`
		VerifierRunID string   `json:"verifier_run_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.SchemaVersion != 1 ||
		!rt.verificationPayloadNamesOperation(ctx, operation, payload.OperationID) ||
		payload.Decision != "pass" || len(payload.VerifierRefs) == 0 || payload.VerifierRunID == "" ||
		!computerevent.IsSHA256(payload.BundleDigest) {
		return selfdevVerificationEvidence{}, fmt.Errorf("materializer: verification payload refused")
	}
	return selfdevVerificationEvidence{OperationID: payload.OperationID, VerifiedBundleDigest: payload.BundleDigest, Refs: payload.VerifierRefs}, nil
}

func (rt *Runtime) recordMaterializationFailed(ctx context.Context, operation selfdev.Operation, result updater.ApplyResult, applyErr error, expectedState string) error {
	payload, err := computerevent.CanonicalJSON(result)
	if err != nil {
		return err
	}
	// A crash-boot settlement of an apply the updater never started has no
	// recovery receipt: nothing changed, so nothing was restored.
	recoveryDigest := ""
	if result.RecoveryReceipt != nil {
		recoveryBytes, err := result.RecoveryReceipt.CanonicalBytes()
		if err != nil {
			return err
		}
		recoveryDigest = computerevent.DigestBytes(recoveryBytes)
	}
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

// applyOrReadJournal runs the apply, announcing its planned restart, or with
// journalOnly reads the terminal outcome the updater already journaled for
// the request without calling the updater. err reports a refused or absent
// attempt; applyErr is the apply's own result error.
func (rt *Runtime) applyOrReadJournal(ctx context.Context, request updater.ApplyRequest, reason, target string, journalOnly bool) (updater.ApplyResult, error, error) {
	if journalOnly {
		journaled, found, err := updater.ReadJournalOutcome(rt.selfdevUpdaterRoot, request.IdempotencyKey)
		if err != nil {
			return updater.ApplyResult{}, nil, err
		}
		if !found || !journaled.Terminal {
			return updater.ApplyResult{}, nil, errSelfDevJournalMidFlight
		}
		if journaled.Result.Outcome != "applied" {
			return journaled.Result, errors.New(journaled.Failure), nil
		}
		return journaled.Result, nil, nil
	}
	restartPlanned, spentErr := rt.markPlannedRestart(reason, target)
	if spentErr != nil {
		return updater.ApplyResult{}, nil, spentErr
	}
	result, applyErr := rt.selfdevUpdater.Apply(ctx, request)
	restartPlanned(applyErr)
	return result, applyErr, nil
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

// verificationPayloadNamesOperation reports whether a recorded verification
// payload that names payloadOperationID is the evidence for this operation:
// its own, or, for a rollback, the operation it rolls back to. A rollback
// operation carries its target's verifier ref (selfdev.Store.StartRollback), so
// its payload names the operation that was verified. The target is the
// operation whose effective head is the rollback's BaseHead; it must be
// settled (applied or rolled back) and cite the same verifier ref and bundle.
// A target that is itself a rollback cites the ref it copied, so the chain is
// followed back, bounded, to the operation the payload names. Anything else
// stays refused: an apply never cites another operation's payload.
// (docs/problems/selfdev-rollback-cannot-record-success-2026-10-26.md)
func (rt *Runtime) verificationPayloadNamesOperation(ctx context.Context, operation selfdev.Operation, payloadOperationID string) bool {
	if payloadOperationID == operation.OperationID {
		return true
	}
	rollback := operation.State == selfdev.StateRollbackPending ||
		(operation.State == selfdev.StateDegraded && strings.TrimSpace(operation.RouteReceipt) != "")
	if !rollback || len(operation.VerifierRefs) == 0 || strings.TrimSpace(operation.BaseHead) == "" || rt.selfdevOperations == nil {
		return false
	}
	head := operation.BaseHead
	for hop := 0; hop < rollbackTargetHops; hop++ {
		target, err := rt.selfdevOperations.GetByEffectiveHead(ctx, operation.ComputerID, head)
		if err != nil || target.OperationID == operation.OperationID ||
			len(target.VerifierRefs) == 0 || target.VerifierRefs[0] != operation.VerifierRefs[0] ||
			target.BundleDigest != operation.BundleDigest {
			return false
		}
		if target.OperationID == payloadOperationID {
			return true
		}
		// Only a rollback is followed further back; its base head is the
		// effective head of the operation it rolled back to.
		if !strings.HasPrefix(target.OperationID, "rollback-") || strings.TrimSpace(target.BaseHead) == "" {
			return false
		}
		head = target.BaseHead
	}
	return false
}
