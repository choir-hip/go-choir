package agentcore

// P4: the guest side of standing autonomous acceptance
// (docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md §5).
//
// After management's Verify moves an operation to awaiting_approval, and
// only while the owner's standing_consensus arm is the computer's mode, the
// runtime makes one bounded attempt: it submits the candidate's P5 host
// build and waits for it (under a standing arm the host's vote is on that
// build, so the build comes before the decision), signs management's verdict
// as the independent-reviewer ballot (guest-core), asks corpusd's decision
// poster for a decision (corpusd casts the capsule-verifier ballot itself,
// on the build), and records the poster's answer through the ordinary
// decision path. The guest never reduces a bundle and never signs the
// host's seat. There is no resumer: a crash mid-attempt leaves the
// operation at awaiting_approval for the owner.

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
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
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
// management's Verify returns while the host builds.
func (rt *Runtime) triggerStandingConsensus(operationID, verifierRunID string) {
	if rt == nil || rt.selfdevControl == nil || rt.selfdevBallotSigner == nil || rt.selfdevOperations == nil || rt.store == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), standingConsensusBudget)
		defer cancel()
		if _, err := rt.runStandingConsensus(ctx, operationID, verifierRunID); err != nil {
			log.Printf("agentcore: standing consensus for %s: %v", operationID, err)
			computerID := rt.standingComputerID()
			if recordErr := rt.selfdevOperations.RecordIntentError(ctx, computerID, operationID, "standing consensus: "+err.Error()); recordErr != nil {
				log.Printf("agentcore: record standing consensus error on %s: %v", operationID, recordErr)
			}
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
// nil) when the computer is not under a standing arm.
func (rt *Runtime) runStandingConsensus(ctx context.Context, operationID, verifierRunID string) (selfdev.Operation, error) {
	computerID := rt.standingComputerID()
	mode, err := rt.selfdevControl.SelfDevelopmentMode(ctx)
	if err != nil {
		return selfdev.Operation{}, err
	}
	if mode.Mode != platform.SelfDevelopmentModeStandingConsensus {
		return selfdev.Operation{}, nil
	}
	if mode.Receipt == nil || mode.Receipt.Verify(rt.selfdevControl.KeyResolver()) != nil {
		return selfdev.Operation{}, fmt.Errorf("standing mode receipt does not verify")
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
	// The host builds first: its vote is on the build.
	jobID, err := rt.standingHostBuild(ctx, operation, bundle)
	if err != nil {
		return operation, err
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
			if named := consensusDecisionHostBuildJob(response.Decision); named != jobID {
				return operation, fmt.Errorf("consensus decision names host build %q, not this operation's %s", named, jobID)
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
	return selfdev.Operation{}, fmt.Errorf("event head kept moving; %d attempts", standingConsensusAttempts)
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
// finalized decision cites a consensus decision (P4): such an operation
// applies only as its host build. An operation with no finalized decision
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
	return consensus, nil
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
	ballot, err := decisionpolicy.NewBallot(decisionpolicy.SeatIndependentReviewer, decisionpolicy.VoteAccept, mode.PolicyDigest, subjectDigest,
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
// operation, bundle and heads, under the current standing arm, inside its
// window. corpusd's gate repeats the binding where it counts; this is the
// guest's legible refusal and defense in depth.
func verifyStandingConsensusDecision(resolver computerevent.KeyResolver, currentMode platform.SelfDevelopmentMode, computerID, operationID string, request selfDevelopmentDecisionRequest) ([]byte, string, error) {
	decision := request.ConsensusDecision
	if decision == nil {
		return nil, "", fmt.Errorf("standing consensus approval requires the poster's decision")
	}
	now := time.Now().UTC()
	expiresAt, err := time.Parse(time.RFC3339Nano, currentMode.ExpiresAt)
	if err != nil || !now.Before(expiresAt) {
		return nil, "", fmt.Errorf("the standing window has expired")
	}
	if decision.ReceiptKind != decisionpolicy.DecisionReceiptKind || decision.Issuer != "corpusd" ||
		len(decision.RequiredSigners) != 1 || decision.RequiredSigners[0].SignerDomain != "platform-control" || decision.Verify(resolver) != nil {
		return nil, "", fmt.Errorf("consensus decision is not the poster's")
	}
	field := func(name string) string {
		value, _ := decision.KindFields[name].(string)
		return value
	}
	armBytes, err := currentMode.Receipt.CanonicalBytes()
	if err != nil {
		return nil, "", err
	}
	if field("computer_id") != computerID || field("operation_id") != operationID ||
		field("standing_mode_receipt_digest") != computerevent.DigestBytes(armBytes) {
		return nil, "", fmt.Errorf("consensus decision does not bind this computer, operation and standing arm")
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
