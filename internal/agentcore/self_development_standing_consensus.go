package agentcore

// P4: the guest side of autonomous acceptance, now authorized by the mode row
// (docs/design/selfdev-apply-and-revert-2026-11-06.md 3.2 and 3.4;
// docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md §5).
//
// After management's Verify moves an operation to awaiting_approval, the
// runtime makes one bounded attempt on a computer whose canonical mode is
// apply (the owner wrote apply, or wrote nothing): it submits the candidate's
// P5 host build and waits for it (the host's vote is on that build, so the
// build comes before the decision), signs management's verdict as the
// independent-reviewer ballot (guest-core), asks corpusd's decision poster for
// a decision (corpusd casts the capsule-verifier ballot itself, on the build),
// and records the poster's answer through the ordinary decision path. The
// guest never reduces a bundle and never signs the host's seat. Nothing
// parks: every exit of the attempt that does not decide closes the operation
// failed with the reason (closeUndecidedSelfDevelopmentOperation), and every
// boot closes an undecided awaiting_approval operation. There is no resumer.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// standingBallotSigner is the guest-core signer (receiptsigner.Client).
type standingBallotSigner interface {
	PublicKey(ctx context.Context) (computerevent.SignerRef, ed25519.PublicKey, error)
	SignReceipt(ctx context.Context, kind, issuer string, fields map[string]any, issuedAt time.Time) (computerevent.Receipt, error)
}

const (
	// standingConsensusAttempts bounds retries on a moved head.
	standingConsensusAttempts = 3
	// standingConsensusBudget bounds one attempt: P5's 20-minute queue
	// deadline plus its 40-minute build timeout, with margin.
	standingConsensusBudget = 90 * time.Minute
	// defaultStandingHostBuildPoll is how often the driver reads a queued or
	// running host build (P5's own admission poll).
	defaultStandingHostBuildPoll = 15 * time.Second
)

// triggerStandingConsensus starts one bounded attempt in the background, so
// management's Verify returns while the host builds. An attempt that ends
// without a decision closes the operation failed with its reason.
func (rt *Runtime) triggerStandingConsensus(operationID, verifierRunID string) {
	if rt == nil {
		return
	}
	if rt.selfdevControl == nil || rt.selfdevBallotSigner == nil || rt.selfdevOperations == nil || rt.store == nil {
		// Nothing parks: a computer that cannot run the attempt closes the
		// operation with that reason (when it has an operation store at all).
		if rt.selfdevOperations != nil {
			computerID := firstNonEmpty(rt.standingComputerID(), rt.cfg.ComputerID)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				rt.closeUndecidedSelfDevelopmentOperation(ctx, computerID, operationID, "standing consensus: the decision authority is not wired on this computer")
			}()
		}
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), standingConsensusBudget)
		defer cancel()
		if _, err := rt.runStandingConsensus(ctx, operationID, verifierRunID); err != nil {
			log.Printf("agentcore: standing consensus for %s: %v", operationID, err)
			// The attempt's own context may be the thing that ended; the
			// close is a fresh bounded write.
			closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Minute)
			defer closeCancel()
			rt.closeUndecidedSelfDevelopmentOperation(closeCtx, rt.standingComputerID(), operationID, "standing consensus: "+err.Error())
		}
	}()
}

func (rt *Runtime) standingComputerID() string {
	if rt.selfdevComputerID != "" {
		return rt.selfdevComputerID
	}
	if rt.selfdevControl != nil {
		return rt.selfdevControl.ComputerID()
	}
	return ""
}

func (rt *Runtime) standingHostBuildPoll() time.Duration {
	if rt.selfdevStandingPoll > 0 {
		return rt.selfdevStandingPoll
	}
	return defaultStandingHostBuildPoll
}

// sleepContext waits d or until ctx ends.
func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// closeUndecidedSelfDevelopmentOperation closes an operation still awaiting
// approval failed, with reason on the operation (RecordIntentError and
// TerminalError) and in its document's Texture. Nothing parks
// (docs/design/selfdev-apply-and-revert-2026-11-06.md 3.4). An operation that
// already left awaiting_approval is left alone; one whose decision is on the
// tape but not projected goes through the existing recovery, never this close.
func (rt *Runtime) closeUndecidedSelfDevelopmentOperation(ctx context.Context, computerID, operationID, reason string) {
	if rt == nil || rt.selfdevOperations == nil {
		return
	}
	operation, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		log.Printf("agentcore: close undecided %s: read: %v", operationID, err)
		return
	}
	if operation.State != selfdev.StateAwaitingApproval {
		return
	}
	if rt.store != nil {
		_, decided, lookupErr := rt.store.FinalizedDecisionForOperation(ctx, operation.ComputerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
		if lookupErr != nil {
			log.Printf("agentcore: close undecided %s: decision lookup: %v", operationID, lookupErr)
			return
		}
		if decided {
			if _, _, recoverErr := rt.recoverSelfDevelopmentDecision(ctx, operation); recoverErr != nil {
				log.Printf("agentcore: close undecided %s: its decision is on the tape; recovery: %v", operationID, recoverErr)
			}
			return
		}
	}
	if recordErr := rt.selfdevOperations.RecordIntentError(ctx, computerID, operationID, reason); recordErr != nil {
		log.Printf("agentcore: close undecided %s: record the reason: %v", operationID, recordErr)
	}
	closed, err := rt.selfdevOperations.Transition(ctx, computerID, operationID, selfdev.StateAwaitingApproval, selfdev.StateFailed, func(next *selfdev.Operation) error {
		next.TerminalError = reason
		return nil
	})
	if err != nil {
		log.Printf("agentcore: close undecided %s: %v", operationID, err)
		return
	}
	log.Printf("agentcore: %s closed failed before a decision: %s", operationID, reason)
	rt.notifyTextureOfOwnerAct(ctx, rt.selfdevRouteOwnerID, closed, types.OwnerActFail, "")
}

// standingHostBuild submits the operation's host build and waits until it is
// terminal. It returns the job id the poster will vote on: a done build is
// verified and staged where the materializer reuses it; a build the host
// recorded as failed is returned too (the poster votes reject on it). Any
// other failure is the guest's own and ends the attempt.
func (rt *Runtime) standingHostBuild(ctx context.Context, operation selfdev.Operation, bundle transaction.CapsuleEffectBundle) (string, error) {
	for {
		built, err := rt.selfDevelopmentHostBuild(ctx, operation, bundle)
		var failure hostBuildFailure
		switch {
		case err == nil:
			return built.Job.JobID, nil
		case errors.As(err, &failure) && failure.HostRecorded:
			request, requestErr := rt.selfDevelopmentHostBuildRequest(operation, bundle)
			if requestErr != nil {
				return "", requestErr
			}
			jobID, idErr := request.JobID()
			log.Printf("agentcore: standing consensus %s: host build %s failed (%s); asking for the host's reject", operation.OperationID, jobID, failure.Reason)
			return jobID, idErr
		case errors.As(err, &failure):
			return "", fmt.Errorf("no host build to vote on: %w", err)
		}
		// Pending, or a transient host or transport error: ask again.
		if waitErr := sleepContext(ctx, rt.standingHostBuildPoll()); waitErr != nil {
			return "", fmt.Errorf("host build did not finish: %v (last: %v)", waitErr, err)
		}
	}
}

// runStandingConsensus builds, asks the poster and records its answer. It
// returns the operation after the decision, or the operation unchanged (and
// nil) when it is no longer awaiting approval. Every other exit is an error,
// and the caller closes the operation failed with it.
func (rt *Runtime) runStandingConsensus(ctx context.Context, operationID, verifierRunID string) (selfdev.Operation, error) {
	computerID := rt.standingComputerID()
	if platform.IsPlatformComputer(computerID) {
		return selfdev.Operation{}, fmt.Errorf("a platform computer does not self-develop (owner ruling 2026-11-06)")
	}
	mode, err := rt.selfdevControl.SelfDevelopmentMode(ctx)
	if err != nil {
		return selfdev.Operation{}, err
	}
	// No row is canonical apply at generation 0 with no receipt.
	if canonical := platform.CanonicalSelfDevelopmentMode(mode.Mode); canonical != platform.SelfDevelopmentModeApply {
		return selfdev.Operation{}, fmt.Errorf("the computer's self-development mode is %s", canonical)
	}
	if mode.Receipt == nil {
		if mode.Generation != 0 {
			return selfdev.Operation{}, fmt.Errorf("mode row %d carries no receipt", mode.Generation)
		}
	} else if mode.Receipt.Verify(rt.selfdevControl.KeyResolver()) != nil {
		return selfdev.Operation{}, fmt.Errorf("mode receipt does not verify")
	}
	operation, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		return selfdev.Operation{}, err
	}
	if operation.State != selfdev.StateAwaitingApproval {
		return operation, nil
	}
	rawBundle, bundle, err := rt.standingFrozenBundle(operation)
	if err != nil {
		return operation, err
	}
	// The frozen patch decides the kind (docs/design/package-operation-decision-2026-11-06.md
	// 2.1): a package-only change takes the package seat, which drafts the
	// release itself; anything else takes the host build.
	classified, err := rt.standingClassify(ctx, operation, rawBundle)
	if err != nil {
		return operation, err
	}
	if err := checkOperationKindAgrees(operation, classified.kind == platform.ConsensusKindPackage); err != nil {
		return operation, err
	}
	// The host builds first: its vote is on the build. A package decision has
	// no host build; its vote is on the draft.
	var jobID string
	if classified.kind != platform.ConsensusKindPackage {
		if jobID, err = rt.standingHostBuild(ctx, operation, bundle); err != nil {
			return operation, err
		}
	}
	bootedBase, err := updater.DigestFile(rt.guestImageManifestPath())
	if err != nil {
		return operation, fmt.Errorf("booted guest image manifest unreadable: %w", err)
	}
	handler := NewAPIHandler(rt)
	for attempt := 1; attempt <= standingConsensusAttempts; {
		operation, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
		if err != nil {
			return selfdev.Operation{}, err
		}
		if operation.State != selfdev.StateAwaitingApproval {
			return operation, nil
		}
		if len(operation.VerifierRefs) == 0 {
			return operation, fmt.Errorf("operation carries no verifier reference")
		}
		head, err := rt.store.Head(ctx, computerID)
		if err != nil || head == nil {
			return operation, fmt.Errorf("event head unavailable: %v", err)
		}
		subject := decisionpolicy.EffectSubject{
			ComputerID: computerID, OperationID: operationID, BundleDigest: operation.BundleDigest,
			DesiredEventHead: head.DesiredEventHead, EffectiveEventHead: head.EffectiveEventHead,
			PendingTransitionRef: head.PendingTransitionRef, DesiredStateCommitment: head.DesiredStateCommitment,
			EffectiveStateCommitment: head.EffectiveStateCommitment, EffectClass: decisionpolicy.EffectClassReversible,
		}
		request, err := rt.standingConsensusRequest(ctx, mode, operation, subject, verifierRunID, rawBundle)
		if err != nil {
			return operation, err
		}
		request.BaseImageManifestDigest, request.HostBuildJobID = bootedBase, jobID
		if classified.kind == platform.ConsensusKindPackage {
			request.Kind, request.SourcePatch, request.ClaimedPlatformBase = platform.ConsensusKindPackage, classified.patch, classified.claimedBase
		}
		response, status, askErr := rt.selfdevControl.RequestConsensusDecision(ctx, request)
		pending := subject.PendingTransitionRef
		decision := selfDevelopmentDecisionRequest{
			BundleDigest: operation.BundleDigest, VerifierRef: operation.VerifierRefs[0],
			ExpectedDesiredEventHead: subject.DesiredEventHead, ExpectedEffectiveEventHead: subject.EffectiveEventHead,
			ExpectedPendingTransitionRef: &pending, ExpectedDesiredStateCommitment: subject.DesiredStateCommitment,
			ExpectedEffectiveStateCommitment: subject.EffectiveStateCommitment,
		}
		switch {
		case askErr == nil:
			// Defense in depth: the decision must name the build this guest
			// built and staged, the one the materializer will apply.
			if classified.kind == platform.ConsensusKindPackage {
				if err := rt.verifyPackageDecisionDraft(ctx, classified, response.Decision); err != nil {
					return operation, err
				}
			} else {
				if consensusDecisionKind(response.Decision) != platform.ConsensusKindPlatform {
					return operation, fmt.Errorf("consensus decision is of kind %q, not a platform decision", consensusDecisionKind(response.Decision))
				}
				if named := consensusDecisionHostBuildJob(response.Decision); named != jobID {
					return operation, fmt.Errorf("consensus decision names host build %q, not this operation's %s", named, jobID)
				}
			}
			decision.Decision = "approve"
			decision.IdempotencyKey = "standing-consensus-" + response.DecisionDigest
			decision.ConsensusDecision = response.Decision
		case status == http.StatusConflict && response.Dissent && len(response.BallotDigests) > 0:
			// The host voted reject (its build failed): record it.
			decision.Decision = "reject"
			decision.IdempotencyKey = "standing-consensus-dissent-" + response.BallotDigests[0]
			decision.ConsensusDissent = response.BallotDigests[0]
			decision.Reason = "standing consensus dissent: " + response.Reason
			// The reason goes on the operation before the reject, so the
			// owner reads it from the operation, not from corpusd's log
			// (fix 12, docs/problems/selfdev-round-blockers-cluster-2026-11-03.md);
			// the reject's owner-act record carries it to Texture.
			if recordErr := rt.selfdevOperations.RecordIntentError(ctx, computerID, operationID, decision.Reason); recordErr != nil {
				log.Printf("agentcore: standing consensus %s: record the dissent reason: %v", operationID, recordErr)
			}
		case status == http.StatusConflict && response.Pending:
			// The host's job is not terminal yet; it does not spend an attempt.
			if waitErr := sleepContext(ctx, rt.standingHostBuildPoll()); waitErr != nil {
				return operation, fmt.Errorf("host build did not finish: %v", waitErr)
			}
			continue
		case status == http.StatusConflict:
			// The heads moved between the read and the poster's check.
			attempt++
			if waitErr := sleepContext(ctx, time.Second); waitErr != nil {
				return operation, waitErr
			}
			continue
		default:
			return operation, askErr
		}
		decided, decideStatus, decideErr := handler.decideSelfDevelopment(ctx, "", computerID, operationID, decision)
		if decideErr == nil {
			log.Printf("agentcore: standing consensus %s %s -> %s (host build %s)", decision.Decision, operationID, decided.State, jobID)
			return decided, nil
		}
		// The post-commit reconciler may have projected this same decision
		// onto the operation first (M7 derivable continuation).
		wantActor := response.DecisionDigest
		if decision.Decision == "reject" {
			wantActor = consensusDissentAuthorityPrefix + decision.ConsensusDissent
		}
		if current, getErr := rt.selfdevOperations.Get(ctx, computerID, operationID); getErr == nil &&
			current.State != selfdev.StateAwaitingApproval && current.DecisionEvent != "" && current.DecisionActor == wantActor {
			log.Printf("agentcore: standing consensus %s %s -> %s (projected by the reconciler)", decision.Decision, operationID, current.State)
			return current, nil
		}
		// The canonical head moves under projection-batch appends; the
		// decision's own heads are re-read on the next attempt.
		if decideStatus == http.StatusConflict && strings.Contains(decideErr.Error(), "head") {
			attempt++
			if waitErr := sleepContext(ctx, 250*time.Millisecond); waitErr != nil {
				return operation, waitErr
			}
			continue
		}
		return operation, decideErr
	}
	return selfdev.Operation{}, fmt.Errorf("decision attempt exhausted: the event head kept moving; %d attempts", standingConsensusAttempts)
}

// standingFrozenBundle reads the operation's frozen bundle bytes and decodes
// them as the materializer does.
func (rt *Runtime) standingFrozenBundle(operation selfdev.Operation) ([]byte, transaction.CapsuleEffectBundle, error) {
	raw, err := os.ReadFile(filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest, "bundle.json"))
	if err != nil {
		return nil, transaction.CapsuleEffectBundle{}, fmt.Errorf("frozen bundle unavailable: %w", err)
	}
	if computerevent.DigestBytes(raw) != operation.BundleDigest {
		return nil, transaction.CapsuleEffectBundle{}, fmt.Errorf("frozen bundle bytes do not hash to the operation's bundle digest")
	}
	var bundle transaction.CapsuleEffectBundle
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return nil, transaction.CapsuleEffectBundle{}, fmt.Errorf("frozen bundle: %w", err)
	}
	return raw, bundle, nil
}

// consensusDecisionHostBuildJob is the host build job a decision names.
func consensusDecisionHostBuildJob(decision *computerevent.Receipt) string {
	if decision == nil {
		return ""
	}
	built, _ := decision.KindFields["host_build"].(map[string]any)
	job, _ := built["job_id"].(string)
	return job
}

// selfDevelopmentDecidedByConsensus reports whether the operation's
// finalized decision cites a consensus decision (P4), or a clean rebase
// receipt (owner ruling 2026-11-04): such an operation applies only as its
// host build, the build the decision was made on. An operation with no finalized decision
// on the tape is an error, never "not consensus" (fail closed).
func (rt *Runtime) selfDevelopmentDecidedByConsensus(ctx context.Context, operation selfdev.Operation) (bool, error) {
	if rt.store == nil {
		return false, fmt.Errorf("materializer: no event store to read the decision")
	}
	transition, found, err := rt.store.FinalizedDecisionForOperation(ctx, operation.ComputerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, fmt.Errorf("materializer: operation %s has no finalized decision on the tape", operation.OperationID)
	}
	_, consensus := decisionpolicy.ReceiptDigestFromAuthority(transition.Request.Event.AuthorityRef)
	_, cleanRebase := selfdevprotocol.CleanRebaseReceiptDigestFromAuthority(transition.Request.Event.AuthorityRef)
	return consensus || cleanRebase, nil
}

// standingConsensusRequest signs management's reviewer ballot over this
// subject and carries the frozen bundle's bytes.
func (rt *Runtime) standingConsensusRequest(ctx context.Context, mode platform.SelfDevelopmentMode, operation selfdev.Operation, subject decisionpolicy.EffectSubject, verifierRunID string, bundle []byte) (platform.ConsensusRequest, error) {
	subjectDigest, err := subject.Digest()
	if err != nil {
		return platform.ConsensusRequest{}, err
	}
	evidence := append([]string{}, operation.VerifierRefs...)
	if strings.TrimSpace(verifierRunID) != "" {
		evidence = append(evidence, "run:"+strings.TrimPrefix(verifierRunID, "run:"))
	}
	now := time.Now().UTC()
	ballot, err := decisionpolicy.NewBallot(decisionpolicy.SeatIndependentReviewer, decisionpolicy.VoteAccept, decisionpolicy.PolicyDigestReversibleSelfDevV1, subjectDigest,
		subject.ComputerID, operation.OperationID, decisionpolicy.DeskManagement, "", evidence, now, 0)
	if err != nil {
		return platform.ConsensusRequest{}, err
	}
	fields, err := ballot.Fields()
	if err != nil {
		return platform.ConsensusRequest{}, err
	}
	signed, err := rt.selfdevBallotSigner.SignReceipt(ctx, decisionpolicy.BallotReceiptKind, decisionpolicy.IssuerGuestManagement, fields, now)
	if err != nil {
		return platform.ConsensusRequest{}, fmt.Errorf("sign reviewer ballot: %w", err)
	}
	_, publicKey, err := rt.selfdevBallotSigner.PublicKey(ctx)
	if err != nil {
		return platform.ConsensusRequest{}, fmt.Errorf("guest-core public key: %w", err)
	}
	return platform.ConsensusRequest{
		OperationID: operation.OperationID, Subject: subject, ReviewerBallot: signed,
		ReviewerPublicKey: base64.RawStdEncoding.EncodeToString(publicKey), Bundle: bundle,
	}, nil
}

// verifyStandingConsensusDecision checks, before the guest records it, that
// a decision is the poster's (platform-control signed), for this computer,
// operation, bundle and heads, under the current mode, inside its window.
// corpusd's gate repeats the binding where it counts; this is the guest's
// legible refusal and defense in depth.
func verifyStandingConsensusDecision(resolver computerevent.KeyResolver, currentMode platform.SelfDevelopmentMode, computerID, operationID string, request selfDevelopmentDecisionRequest) ([]byte, string, error) {
	decision := request.ConsensusDecision
	if decision == nil {
		return nil, "", fmt.Errorf("standing consensus approval requires the poster's decision")
	}
	now := time.Now().UTC()
	if platform.CanonicalSelfDevelopmentMode(currentMode.Mode) != platform.SelfDevelopmentModeApply {
		return nil, "", fmt.Errorf("the computer's self-development mode is not apply")
	}
	if decision.ReceiptKind != decisionpolicy.DecisionReceiptKind || decision.Issuer != "corpusd" ||
		len(decision.RequiredSigners) != 1 || decision.RequiredSigners[0].SignerDomain != "platform-control" || decision.Verify(resolver) != nil {
		return nil, "", fmt.Errorf("consensus decision is not the poster's")
	}
	field := func(name string) string {
		value, _ := decision.KindFields[name].(string)
		return value
	}
	// The mode receipt's digest, or for a computer with no row the default
	// apply digest (generation 0).
	modeBytes, err := selfDevelopmentModeAuthorityBytes(currentMode)
	if err != nil {
		return nil, "", err
	}
	generation, _ := decision.KindFields["standing_generation"].(json.Number)
	if generation == "" {
		if number, isFloat := decision.KindFields["standing_generation"].(float64); isFloat {
			generation = json.Number(strconv.FormatUint(uint64(number), 10))
		}
	}
	if field("computer_id") != computerID || field("operation_id") != operationID ||
		field("standing_mode_receipt_digest") != computerevent.DigestBytes(modeBytes) ||
		generation.String() != strconv.FormatUint(currentMode.Generation, 10) {
		return nil, "", fmt.Errorf("consensus decision does not bind this computer, operation and mode")
	}
	notAfter, err := time.Parse(time.RFC3339Nano, field("not_after"))
	if err != nil || !now.Before(notAfter) {
		return nil, "", fmt.Errorf("consensus decision has expired")
	}
	raw, err := json.Marshal(decision.KindFields["subject"])
	if err != nil {
		return nil, "", err
	}
	var subject decisionpolicy.EffectSubject
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&subject); err != nil {
		return nil, "", fmt.Errorf("consensus decision subject: %w", err)
	}
	if subject.ComputerID != computerID || subject.OperationID != operationID || subject.BundleDigest != request.BundleDigest ||
		subject.DesiredEventHead != request.ExpectedDesiredEventHead || subject.EffectiveEventHead != request.ExpectedEffectiveEventHead ||
		request.ExpectedPendingTransitionRef == nil || subject.PendingTransitionRef != strings.TrimSpace(*request.ExpectedPendingTransitionRef) ||
		subject.DesiredStateCommitment != request.ExpectedDesiredStateCommitment || subject.EffectiveStateCommitment != request.ExpectedEffectiveStateCommitment {
		return nil, "", fmt.Errorf("consensus decision subject does not bind this decision")
	}
	decisionBytes, err := decision.CanonicalBytes()
	if err != nil {
		return nil, "", err
	}
	return decisionBytes, computerevent.DigestBytes(decisionBytes), nil
}

// checkOperationKindAgrees refuses a decision whose kind disagrees with the
// operation's recorded kind: a package decision on a platform row would host
// build a package patch as a platform release, and the reverse would publish a
// platform change as a package.
func checkOperationKindAgrees(operation selfdev.Operation, decisionIsPackage bool) error {
	if decisionIsPackage != operation.IsPackage() {
		return fmt.Errorf("the decision is of kind %s but operation %s is of kind %s", kindWord(decisionIsPackage), operation.OperationID, kindWord(operation.IsPackage()))
	}
	return nil
}

func kindWord(isPackage bool) string {
	if isPackage {
		return selfdev.KindPackage
	}
	return "platform"
}
