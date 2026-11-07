package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The clean rebase (owner ruling 2026-11-04, recorded in
// docs/design/u4-stack-rebase-build-2026-10-26.md, "Owner ruling": "I want
// all this, and clean rebases without approval"). Before the rebase notice
// goes to Texture, the runtime tries the applied change's own patch bytes on
// the booted base (sourceproj.Reapply, the host builder's `git apply`). When
// they apply, it opens a platform-authored operation whose bundle is the
// approved bundle with only its base and identity fields changed, records a
// verification of that mechanical fact, has the host build it (the frontend
// build with the package check, and the Go build: the floor), and then
// accepts it under a CleanRebaseReceipt instead of an owner act. corpusd
// admits that acceptance only when the receipt, both bundles on the tape, the
// prior decision and the host build agree (internal/platform). The
// materializer applies it as a host build, with the planned restart and the
// rollback of any other operation. A conflict, a failed build or a failed
// step mints the rebase notice as before.
//
// Bounds:
//   - one attempt per (operation, booted base) pair: the operation's
//     idempotency key; a failed or refused attempt falls through to the
//     notice and is never retried for that pair;
//   - only while no notice has been minted for the pair (the model route
//     owns a pair once it has a notice);
//   - on every computer, whatever its tier (owner ruling 2026-11-04: a test
//     computer must not work differently from the others, or it is no test);
//     the paper and other always-busy computers take it through the same
//     marker-backed planned restart, and never with a live capsule, a
//     streaming snapshot or another open operation (cleanRebaseQuietHold);
//   - no resumer: an attempt is driven only by the process that started it.
//     A pre-decision clean-rebase operation found open with no live driver
//     (a crash or planned restart killed it) is closed failed, "interrupted
//     by a restart", and the pair goes to the notice.

const (
	cleanRebaseOperationPrefix = "clean-rebase-"
	cleanRebaseCapsulePrefix   = "clean-rebase-capsule-"
	// defaultCleanRebaseBudget bounds one attempt: the host build's queue
	// deadline and build timeout, with margin (as standing consensus).
	defaultCleanRebaseBudget = 90 * time.Minute
	// cleanRebaseDecisionAttempts bounds retries on a moved canonical head.
	cleanRebaseDecisionAttempts = 5
)

// cleanRebaseIdempotencyKey is the pair's operation key: one per pair.
func cleanRebaseIdempotencyKey(operationID, booted string) string {
	return "clean-rebase:" + strings.TrimSpace(operationID) + ":" + strings.TrimSpace(booted)
}

func cleanRebaseOperationID(key string) string {
	return cleanRebaseOperationPrefix + computerevent.DigestBytes([]byte(key))[:32]
}

// isCleanRebaseOperation reports a platform-authored clean-rebase operation.
func isCleanRebaseOperation(operationID string) bool {
	return strings.HasPrefix(strings.TrimSpace(operationID), cleanRebaseOperationPrefix)
}

// cleanRebaseQuietHold says why the computer cannot take a restart now, or "".
// The apply restarts the guest. Running runs resume across a planned restart
// (the marker), but a live engineering capsule lives in the executor's memory
// and a streaming snapshot is discarded; neither survives it. A test computer
// rarely has either; an owner's computer or the paper often does. The check
// fails closed: an unreadable count holds.
func (rt *Runtime) cleanRebaseQuietHold(ctx context.Context) string {
	work, err := rt.plannedRestartBusy(ctx, true)
	if err != nil {
		return "the work a restart would cut cannot be counted: " + err.Error()
	}
	switch {
	case work.LiveCapsules > 0:
		return fmt.Sprintf("%d live engineering capsule(s) a restart would destroy", work.LiveCapsules)
	case work.SnapshotInFlight:
		return "a snapshot is streaming; a restart would discard it"
	}
	return ""
}

func (rt *Runtime) cleanRebaseBudget() time.Duration {
	if rt.cleanRebaseAttemptBudget > 0 {
		return rt.cleanRebaseAttemptBudget
	}
	return defaultCleanRebaseBudget
}

func (rt *Runtime) cleanRebaseSetLive(key string, live bool) bool {
	rt.cleanRebaseMu.Lock()
	defer rt.cleanRebaseMu.Unlock()
	if rt.cleanRebaseLive == nil {
		rt.cleanRebaseLive = map[string]bool{}
	}
	if live && rt.cleanRebaseLive[key] {
		return false
	}
	if live {
		rt.cleanRebaseLive[key] = true
	} else {
		delete(rt.cleanRebaseLive, key)
	}
	return true
}

func (rt *Runtime) cleanRebaseIsLive(key string) bool {
	rt.cleanRebaseMu.Lock()
	defer rt.cleanRebaseMu.Unlock()
	return rt.cleanRebaseLive[key]
}

// cleanRebaseBeforeNotice runs inside the notice timer's fire, before a
// notice is minted. hold says the notice must wait (an attempt is in flight,
// or another selfdev operation is open);
// otherwise the notice goes as today, and why (when set) says why the clean
// path did not take the pair.
func (rt *Runtime) cleanRebaseBeforeNotice(ctx context.Context, ownerID, computerID, operationID, stackBase, booted string) (hold bool, why string) {
	if rt.selfdevOperations == nil || rt.store == nil {
		return false, "no operation store"
	}
	// The model route owns a pair once a notice exists for it.
	if rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.StackRebaseNoticeID(operationID, booted, 1)); err != nil {
		return true, "read the pair's first notice: " + err.Error()
	} else if rec != nil {
		return false, "the pair already has a rebase notice"
	}
	// A package operation is not part of the source stack: there is nothing
	// to re-apply onto a moved base (docs/design/package-operation-decision-2026-11-06.md 2.5).
	if named, err := rt.selfdevOperations.Get(ctx, computerID, operationID); err == nil && named.IsPackage() {
		return false, "the operation " + operationID + " is a package operation, outside the source stack"
	}
	key := cleanRebaseIdempotencyKey(operationID, booted)
	existing, found, err := rt.selfdevOperations.GetByIdempotency(ctx, computerID, key)
	if err != nil {
		return true, "read the clean rebase operation: " + err.Error()
	}
	if found {
		return rt.cleanRebaseExisting(ctx, key, existing)
	}
	if open, isOpen, err := rt.selfdevOperations.OpenOperation(ctx, computerID); err != nil {
		return true, "read the open operation: " + err.Error()
	} else if isOpen {
		// Hold, do not mint: at stack_rematerialized ordinary Casts are
		// admitted, so an open operation is routine work, and a notice minted
		// now could be claimed by a rebase assignment whose Freeze then hits
		// selfdev.ErrOperationOpen, spending the pair. The timer rechecks.
		return true, fmt.Sprintf("operation %s is %s; a clean rebase opens no second operation, and the notice waits", open.OperationID, open.State)
	}
	if err := rt.selfDevelopmentProposalAuthorized(ctx); err != nil {
		return false, "the computer's mode does not authorize a proposal: " + err.Error()
	}
	prior, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil || prior.State != selfdev.StateApplied {
		return false, fmt.Sprintf("the applied operation %s cannot be read as applied (%v)", operationID, err)
	}
	base, patch, err := rt.boundSourcePatch(prior)
	if err != nil {
		return false, "the applied change's patch cannot be read: " + err.Error()
	}
	if base != stackBase {
		return false, fmt.Sprintf("the applied change's base %s is not the stack base %s", base, stackBase)
	}
	verdict, err := sourceproj.Reapply(ctx, rt.releaseRepo(), stackBase, booted, patch)
	if err != nil {
		return false, "the re-application could not be checked: " + err.Error()
	}
	if !verdict.Clean {
		return false, verdict.Reason
	}
	if why := rt.cleanRebaseQuietHold(ctx); why != "" {
		// Hold, do not spend the pair: the timer rechecks.
		return true, "a clean rebase waits: " + why
	}
	if !rt.cleanRebaseSetLive(key, true) {
		return true, "a clean re-application of " + operationID + " is in flight"
	}
	attempt := cleanRebaseAttempt{key: key, ownerID: ownerID, computerID: computerID, prior: prior, stackBase: stackBase, booted: booted, patch: patch}
	start := rt.cleanRebaseStart
	if start == nil {
		start = func(run func()) { go run() }
	}
	start(func() {
		defer rt.cleanRebaseSetLive(key, false)
		ctx, cancel := context.WithTimeout(context.Background(), rt.cleanRebaseBudget())
		defer cancel()
		if err := rt.runCleanRebase(ctx, attempt); err != nil {
			log.Printf("clean rebase: %s onto %s: %v; the rebase notice follows", operationID, booted, err)
		}
	})
	return true, fmt.Sprintf("the applied change %s re-applies cleanly onto %s; a clean rebase started", operationID, booted)
}

// cleanRebaseExisting decides from the pair's operation: in flight here (hold),
// owned by the materializer (hold), left by a restart (closed, then the
// notice), or ended (the notice).
func (rt *Runtime) cleanRebaseExisting(ctx context.Context, key string, op selfdev.Operation) (bool, string) {
	switch op.State {
	case selfdev.StateAccepted, selfdev.StateMaterializing:
		// A decided clean rebase belongs to the materializer. One that has
		// sat decided past twice the attempt budget is wedged: the owner gets
		// the notice (the operation itself is left to the materializer's own
		// settlement; nothing here closes a decided operation).
		if updated, err := time.Parse(time.RFC3339Nano, op.UpdatedAt); err == nil && rt.rebaseClock().Sub(updated) > 2*rt.cleanRebaseBudget() {
			return false, "the clean rebase " + op.OperationID + " has been " + op.State + " since " + op.UpdatedAt + "; wedged, so the notice"
		}
		return true, "the clean rebase " + op.OperationID + " is " + op.State
	case selfdev.StateApplied:
		return true, "the clean rebase " + op.OperationID + " is applied"
	case selfdev.StateRequested, selfdev.StateExecuting, selfdev.StateFrozen, selfdev.StateVerified, selfdev.StateAwaitingApproval:
		if rt.cleanRebaseIsLive(key) {
			return true, "the clean rebase " + op.OperationID + " is in flight (" + op.State + ")"
		}
		if op.State == selfdev.StateAwaitingApproval {
			// A decision that committed before the restart belongs to the
			// materializer's crash settlement, never to a resumer here.
			if _, decided, err := rt.store.FinalizedDecisionForOperation(ctx, op.ComputerID, op.OperationID, op.TrajectoryID, op.CapsuleID); err != nil {
				return true, "read the clean rebase decision: " + err.Error()
			} else if decided {
				return true, "the clean rebase " + op.OperationID + " is decided"
			}
		}
		if _, err := rt.selfdevOperations.Transition(ctx, op.ComputerID, op.OperationID, op.State, selfdev.StateFailed, func(next *selfdev.Operation) error {
			next.TerminalError = selfdevInterruptedByRestart
			return nil
		}); err != nil {
			return true, "close the interrupted clean rebase: " + err.Error()
		}
		log.Printf("clean rebase: %s closed from %s: %s", op.OperationID, op.State, selfdevInterruptedByRestart)
		return false, "the clean rebase " + op.OperationID + " was " + selfdevInterruptedByRestart
	default:
		reason := op.TerminalError
		if reason == "" {
			reason = op.LastIntentError
		}
		return false, "the clean rebase " + op.OperationID + " ended " + op.State + ": " + reason
	}
}

type cleanRebaseAttempt struct {
	key, ownerID, computerID string
	prior                    selfdev.Operation
	stackBase, booted        string
	patch                    []byte
}

// runCleanRebase drives one attempt to a decision. Any error after the
// operation opened settles it failed with that error, so the next timer fire
// mints the notice.
func (rt *Runtime) runCleanRebase(ctx context.Context, a cleanRebaseAttempt) (err error) {
	op, err := rt.selfdevOperations.Start(ctx, selfdev.StartRequest{
		ComputerID: a.computerID, IdempotencyKey: a.key, PromptArtifactRef: a.prior.PromptArtifactRef,
		OperationID: cleanRebaseOperationID(a.key), TrajectoryID: a.prior.TrajectoryID,
	})
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() {
		if err == nil {
			return
		}
		rt.failCleanRebase(context.WithoutCancel(ctx), op.OperationID, err)
	}()
	capsuleID := cleanRebaseCapsulePrefix + strings.TrimPrefix(op.OperationID, cleanRebaseOperationPrefix)
	if op, err = rt.selfdevOperations.Transition(ctx, a.computerID, op.OperationID, selfdev.StateRequested, selfdev.StateExecuting, func(next *selfdev.Operation) error {
		next.CapsuleID = capsuleID
		return nil
	}); err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	priorRaw, err := os.ReadFile(filepath.Join(rt.selfdevUpdaterRoot, "incoming", a.prior.BundleDigest, "bundle.json"))
	if err != nil || computerevent.DigestBytes(priorRaw) != a.prior.BundleDigest {
		return fmt.Errorf("the applied bundle %s is not retained intact (%v)", a.prior.BundleDigest, err)
	}
	var bundle transaction.CapsuleEffectBundle
	decoder := json.NewDecoder(bytes.NewReader(priorRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return fmt.Errorf("the applied bundle does not decode: %w", err)
	}
	if bundle.SourcePatchSHA256 != computerevent.DigestBytes(a.patch) || bundle.SourcePatchBaseCommit != a.stackBase {
		return errors.New("the applied bundle does not bind the patch and base that were checked")
	}
	// The approved bundle, with only the fields a clean re-application may
	// change (selfdevprotocol.CleanRebaseBundleFields).
	bundle.BaseEventHead, bundle.TrajectoryRef, bundle.CapsuleIdentity = op.BaseHead, op.TrajectoryID, capsuleID
	bundle.SourcePatchBaseCommit = a.booted
	bundle.VerifierReceipts = []string{}
	if bundle.ReleaseBinary != nil && bundle.ReleaseBinary.Origin == capsule.ReleaseBinaryReusedRunning {
		// The binary it reused is not running any more, and on the host-built
		// path no capsule binary is a release input.
		bundle.ReleaseBinary = nil
	}
	if bundle.ContentDigest, err = bundle.ComputeContentDigest(); err != nil {
		return err
	}
	if err := bundle.Validate(false); err != nil {
		return fmt.Errorf("the re-applied bundle: %w", err)
	}
	draft, err := computerevent.CanonicalJSON(bundle)
	if err != nil {
		return err
	}
	if err := rt.stageCleanRebaseBundle(a.prior.BundleDigest, bundle.ContentDigest, draft, a.patch); err != nil {
		return err
	}
	head, err := rt.store.Head(ctx, a.computerID)
	if err != nil || head == nil || head.PendingTransitionRef != "" {
		return fmt.Errorf("the event head is unavailable or a transition is pending (%v)", err)
	}
	if op, err = rt.selfdevOperations.Transition(ctx, a.computerID, op.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
		next.BundleDigest = bundle.ContentDigest
		next.DesiredHead, next.EffectiveHead = head.DesiredEventHead, head.EffectiveEventHead
		return nil
	}); err != nil {
		return fmt.Errorf("freeze: %w", err)
	}
	// The verification is the mechanical fact, recorded by the guest core's
	// own verdict path: the approved patch applies unchanged to the booted
	// base. The prior operation's verification stands for the content.
	toolCtx := &CapsuleToolCtx{
		AgentRunID: "clean-rebase:" + op.OperationID, ComputerID: a.computerID, OwnerID: a.ownerID,
		EventAppender: rt.eventAppender, OperationStore: rt.selfdevOperations, UpdaterRoot: rt.selfdevUpdaterRoot, EventProjection: rt.store,
	}
	refs := []string{"clean-rebase-of:" + a.prior.OperationID, "reapply:" + a.stackBase + ".." + a.booted}
	if len(a.prior.VerifierRefs) > 0 {
		refs = append(refs, "prior-verification:"+a.prior.VerifierRefs[0])
	}
	if _, err := recordSelfDevelopmentVerdict(ctx, toolCtx, op.TrajectoryID, op.OperationID, op.BundleDigest, "pass", refs, map[string]any{
		"verifier_desk": "platform", "kind": "clean_rebase", "rebase_of": a.prior.OperationID, "booted_base": a.booted, "stack_base": a.stackBase,
	}); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	if op, err = rt.selfdevOperations.Get(ctx, a.computerID, op.OperationID); err != nil {
		return err
	}
	if op.State != selfdev.StateAwaitingApproval {
		return fmt.Errorf("verification left the operation %s", op.State)
	}
	// The floor: the host builds the re-applied bundle on the booted base
	// before any decision. A build the host refuses or fails ends here.
	_, finalBundle, err := rt.standingFrozenBundle(op)
	if err != nil {
		return err
	}
	built, err := rt.cleanRebaseHostBuild(ctx, op, finalBundle)
	if err != nil {
		return err
	}
	if built.Job.BaseCommit != a.booted {
		return fmt.Errorf("the host built base %s, not the booted base %s", built.Job.BaseCommit, a.booted)
	}
	// The base must not have moved during the attempt.
	if now := rt.bootedBaseCommit(); now != a.booted {
		return fmt.Errorf("the booted base moved to %s during the attempt", now)
	}
	resultDigest, err := built.Job.Result.Digest()
	if err != nil {
		return err
	}
	// The host build can take an hour; an owner's engineering capsule or a
	// snapshot may have started since the attempt began. The decision commits
	// the restart, so wait for a quiet moment, within the attempt's budget.
	for {
		why := rt.cleanRebaseQuietHold(ctx)
		if why == "" {
			break
		}
		if waitErr := sleepContext(ctx, rt.standingHostBuildPoll()); waitErr != nil {
			return fmt.Errorf("the computer stayed busy before the decision: %s (%v)", why, waitErr)
		}
	}
	// The base must still be the one the build was made on.
	if now := rt.bootedBaseCommit(); now != a.booted {
		return fmt.Errorf("the booted base moved to %s while waiting for a quiet moment", now)
	}
	priorDecision, found, err := rt.store.FinalizedDecisionForOperation(ctx, a.computerID, a.prior.OperationID, a.prior.TrajectoryID, a.prior.CapsuleID)
	if err != nil || !found || priorDecision.Request.Event.EventKind != computerevent.EventEffectAccepted {
		return fmt.Errorf("the applied operation's acceptance is not on the tape (%v)", err)
	}
	receipt := selfdevprotocol.CleanRebaseReceipt{
		Schema: selfdevprotocol.CleanRebaseReceiptSchema, ComputerID: a.computerID,
		OperationID: op.OperationID, BundleDigest: op.BundleDigest,
		RebaseOf: a.prior.OperationID, PriorBundleDigest: a.prior.BundleDigest, PriorDecisionEvent: priorDecision.Request.EventDigest,
		SourcePatchSHA256: computerevent.DigestBytes(a.patch), StackBase: a.stackBase, BootedBase: a.booted,
		HostBuildJobID: built.Job.JobID, HostBuildResultDigest: resultDigest,
		Ruling: selfdevprotocol.CleanRebaseRuling, Statement: selfdevprotocol.CleanRebaseStatement(a.prior.OperationID, a.booted),
	}
	decided, err := rt.acceptCleanRebase(ctx, op, receipt)
	if err != nil {
		return fmt.Errorf("accept: %w", err)
	}
	log.Printf("clean rebase: %s accepted (%s); host build %s", decided.OperationID, receipt.Statement, built.Job.JobID)
	rt.triggerSelfDevelopmentReconcile()
	return nil
}

// stageCleanRebaseBundle publishes incoming/<digest>/ with the draft, the
// patch bytes, and the prior candidate's screen evidence (the bundle keeps
// its Screens and Video fields, so their files go with it).
func (rt *Runtime) stageCleanRebaseBundle(priorDigest, digest string, draft, patch []byte) error {
	incoming := filepath.Join(rt.selfdevUpdaterRoot, "incoming")
	tmp, err := os.MkdirTemp(incoming, ".clean-rebase-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, "source.patch"), patch, 0o444); err != nil {
		return err
	}
	evidence := filepath.Join(incoming, priorDigest, "evidence")
	if _, statErr := os.Stat(evidence); statErr == nil {
		if err := copyTree(evidence, filepath.Join(tmp, "evidence")); err != nil {
			return fmt.Errorf("copy the screen evidence: %w", err)
		}
	}
	return publishFrozenDraft(tmp, filepath.Join(incoming, digest), draft)
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o444)
	})
}

// cleanRebaseHostBuild submits the build and waits for a terminal answer
// within the attempt's budget.
func (rt *Runtime) cleanRebaseHostBuild(ctx context.Context, op selfdev.Operation, bundle transaction.CapsuleEffectBundle) (hostBuiltRelease, error) {
	for {
		built, err := rt.selfDevelopmentHostBuild(ctx, op, bundle)
		var failure hostBuildFailure
		switch {
		case err == nil:
			return built, nil
		case errors.As(err, &failure):
			return hostBuiltRelease{}, failure
		}
		if waitErr := sleepContext(ctx, rt.standingHostBuildPoll()); waitErr != nil {
			return hostBuiltRelease{}, fmt.Errorf("the host build did not finish: %v (last: %v)", waitErr, err)
		}
	}
}

// acceptCleanRebase appends the decision under the receipt's authority and
// moves the operation to accepted. The canonical head moves under projection
// batches, so a head conflict is retried a few times.
func (rt *Runtime) acceptCleanRebase(ctx context.Context, op selfdev.Operation, receipt selfdevprotocol.CleanRebaseReceipt) (selfdev.Operation, error) {
	if rt.selfdevControl == nil || rt.eventAppender == nil {
		return op, errors.New("the decision authority is unavailable")
	}
	receiptBytes, receiptDigest, err := receipt.Canonical()
	if err != nil {
		return op, err
	}
	mode, err := rt.selfdevControl.SelfDevelopmentMode(ctx)
	if err != nil {
		return op, fmt.Errorf("read the mode: %w", err)
	}
	// No mode row is canonical apply at generation 0 with no receipt.
	if mode.Receipt == nil && mode.Generation != 0 {
		return op, errors.New("the current mode receipt is missing")
	}
	if mode.Receipt != nil && mode.Receipt.Verify(rt.selfdevControl.KeyResolver()) != nil {
		return op, errors.New("the current mode receipt does not verify")
	}
	modeBytes, err := selfDevelopmentModeAuthorityBytes(mode)
	if err != nil {
		return op, err
	}
	idempotency := "selfdev-decision-" + computerevent.DigestBytes([]byte(op.ComputerID+"\x00"+op.OperationID+"\x00clean-rebase:"+receiptDigest))
	var lastErr error
	for attempt := 0; attempt < cleanRebaseDecisionAttempts; attempt++ {
		event, found, err := rt.store.EventByIdempotency(ctx, op.ComputerID, idempotency)
		if err != nil {
			return op, err
		}
		if !found {
			head, err := rt.store.Head(ctx, op.ComputerID)
			if err != nil || head == nil {
				return op, fmt.Errorf("the event head is unavailable (%v)", err)
			}
			eventID, err := computerevent.NewEventID()
			if err != nil {
				return op, err
			}
			event = computerevent.Event{
				SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: op.ComputerID,
				EventKind: computerevent.EventEffectAccepted, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: idempotency,
				RequestCommitment: computerevent.ZeroHead, TrajectoryID: op.TrajectoryID, CapsuleID: op.CapsuleID,
				ParentEventID: op.OperationID, PreviousHead: head.CanonicalEventHead,
				ActorProfile: agentprofile.Management, AuthorityRef: selfdevprotocol.CleanRebaseAuthority(receiptDigest), PrivacyClass: "owner",
				ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
				ExpectedPendingTransitionRef: head.PendingTransitionRef, ExpectedDesiredStateCommitment: head.DesiredStateCommitment,
				ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment, RequireExpectedHead: true,
				PayloadCommitment: computerevent.ZeroHead, ProposedEffectRef: op.BundleDigest, DecisionRef: receiptDigest,
				VerifierRefs: []string{op.VerifierRefs[0]}, ReducerVersion: computerevent.ReducerVersionV1,
			}
			target, err := computerevent.CanonicalJSON(map[string]string{"base_head": op.BaseHead, "bundle_digest": op.BundleDigest})
			if err != nil {
				return op, err
			}
			payloads := []computerevent.EventPayload{
				{Content: modeBytes, MediaType: "application/vnd.choir.mode-receipt+json", PrivacyClass: "owner", Direction: computerevent.EventPayloadInput},
				{Content: receiptBytes, MediaType: selfdevprotocol.CleanRebaseReceiptMediaType, PrivacyClass: "owner", Direction: computerevent.EventPayloadInput},
			}
			_, digests, appendErr := rt.eventAppender.AppendNewPayloadSet(ctx, event, computerevent.TransitionInput{TargetStateCommitment: computerevent.DigestBytes(target)}, payloads, rt.privateArtifactCipher)
			if appendErr == nil && (len(digests) < 2 || digests[1] != receiptDigest) {
				appendErr = errors.New("the receipt artifact binding does not match")
			}
			if appendErr != nil {
				lastErr = appendErr
				if strings.Contains(appendErr.Error(), "head") || strings.Contains(appendErr.Error(), "CAS") {
					if waitErr := sleepContext(ctx, 250*time.Millisecond); waitErr != nil {
						return op, waitErr
					}
					continue
				}
				return op, appendErr
			}
			// The appended event as the chain holds it (sequence and head).
			if event, found, err = rt.store.EventByIdempotency(ctx, op.ComputerID, idempotency); err != nil || !found {
				return op, fmt.Errorf("the decision event is not projected (%v)", err)
			}
		}
		eventDigest, err := event.Digest()
		if err != nil {
			return op, err
		}
		transition, ok, err := rt.store.FinalizedDecisionForOperation(ctx, op.ComputerID, op.OperationID, op.TrajectoryID, op.CapsuleID)
		if err != nil || !ok || transition.Request.EventDigest != eventDigest {
			return op, fmt.Errorf("the decision transition is not projected (%v)", err)
		}
		decision, err := verifyFinalizedSelfDevelopmentDecision(op, transition)
		if err != nil || decision.NextState != selfdev.StateAccepted {
			return op, fmt.Errorf("the decision does not bind: %v", err)
		}
		decided, err := rt.selfdevOperations.Transition(ctx, op.ComputerID, op.OperationID, selfdev.StateAwaitingApproval, selfdev.StateAccepted, func(next *selfdev.Operation) error {
			next.DecisionActor = decision.Actor
			next.DecisionEvent = eventDigest
			next.DesiredHead, next.EffectiveHead = transition.Request.Next.DesiredEventHead, transition.Request.Next.EffectiveEventHead
			next.DecisionReceipt = transition.Receipt.ReceiptID
			next.ModeReceipt = decision.ModeReceiptDigest
			return nil
		})
		if err != nil {
			// The post-commit observer may have projected the same decision.
			if current, getErr := rt.selfdevOperations.Get(ctx, op.ComputerID, op.OperationID); getErr == nil && selfDevelopmentDecisionNotCarried(current, transition, selfdev.StateAccepted) == "" {
				return current, nil
			}
			return op, err
		}
		return decided, nil
	}
	return op, fmt.Errorf("the event head kept moving: %v", lastErr)
}

// failCleanRebase settles an attempt's operation failed with its reason, from
// whatever pre-decision state it reached. A decided operation is left to the
// materializer.
func (rt *Runtime) failCleanRebase(ctx context.Context, operationID string, cause error) {
	computerID := rt.cfg.ComputerID
	op, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		log.Printf("clean rebase: %s: read to settle failed: %v", operationID, err)
		return
	}
	switch op.State {
	case selfdev.StateRequested, selfdev.StateExecuting, selfdev.StateFrozen, selfdev.StateVerified, selfdev.StateAwaitingApproval:
	default:
		return
	}
	if op.State == selfdev.StateAwaitingApproval {
		if _, decided, err := rt.store.FinalizedDecisionForOperation(ctx, op.ComputerID, op.OperationID, op.TrajectoryID, op.CapsuleID); err != nil || decided {
			return
		}
	}
	reason := "clean rebase: " + strings.Join(strings.Fields(cause.Error()), " ")
	if _, err := rt.selfdevOperations.Transition(ctx, computerID, operationID, op.State, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.TerminalError = reason
		return nil
	}); err != nil {
		log.Printf("clean rebase: %s: settle failed: %v", operationID, err)
	}
}
