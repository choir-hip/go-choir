package platform

// P4: standing autonomous acceptance on test computers
// (docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md;
// docs/archive/reports/selfdev-update-architecture-2026-10-15.md §1.3, P4;
// docs/archive/reports/principal-identity-and-signed-acts-2026-10-15.md §6 S2).
//
// corpusd is the decision poster. Under an owner's standing_consensus arm it
// collects the two verification ballots reversible-selfdev-v1 needs, each
// signed by the one domain its seat admits (the capsule-verifier seat cast
// here, platform-control signed, on this operation's P5 host build, so the
// vote binds the bytes the computer applies; the independent-reviewer seat
// by the computer's guest-core key, management's Verify verdict), builds the
// ConsensusBundle, reduces it, and mints a ConsensusDecisionReceipt signed by
// platform-control. The guest's chain writer records that decision as
// effect_accepted with AuthorityRef qualified-consensus:<decision digest>,
// and the admission gate admits it only for a decision this poster minted.

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computertiers"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/owneract"
)

var (
	// ErrConsensusRefused is a decision request the poster will not act on:
	// no standing arm, an expired window, a non-test computer, a ballot that
	// does not verify, or a subject that does not bind.
	ErrConsensusRefused = errors.New("consensus decision refused")
	// ErrConsensusStale is a subject whose heads are no longer corpusd's.
	ErrConsensusStale = errors.New("consensus decision subject is stale")
	// ErrConsensusDissent is a quorum the reducer refused because a seat
	// voted reject (or the seats fell short): no decision is minted.
	ErrConsensusDissent = errors.New("consensus dissent")
	// ErrConsensusUnavailable is a poster that cannot decide right now (no
	// host build service configured).
	ErrConsensusUnavailable = errors.New("consensus decision poster unavailable")
	// ErrConsensusPending is an operation whose host build is still queued
	// or running: no ballot, no decision yet.
	ErrConsensusPending = errors.New("consensus decision pending: the host build is not finished")
)

// HostBuildJobs is P5's host build service as the poster reads it: a job
// only to the computer that owns it (hostbuild.Service).
type HostBuildJobs interface {
	Get(computerID, jobID string) (hostbuild.Job, bool)
}

// maxConsensusRequestBytes bounds a decision request (the frozen bundle's
// canonical bytes and two small receipts).
const maxConsensusRequestBytes = 8 << 20

const consensusSchemaDDL = `
CREATE TABLE IF NOT EXISTS consensus_decisions (
  decision_digest CHAR(64) NOT NULL,
  computer_id VARCHAR(128) NOT NULL,
  operation_id VARCHAR(255) NOT NULL,
  subject_digest CHAR(64) NOT NULL,
  bundle_digest CHAR(64) NOT NULL,
  desired_event_head CHAR(64) NOT NULL,
  effective_event_head CHAR(64) NOT NULL,
  pending_transition_ref VARCHAR(255) NOT NULL,
  desired_state_commitment CHAR(64) NOT NULL,
  effective_state_commitment CHAR(64) NOT NULL,
  policy_digest CHAR(64) NOT NULL,
  standing_mode_receipt_digest CHAR(64) NOT NULL,
  standing_generation BIGINT NOT NULL,
  host_build_job_id CHAR(64) NOT NULL,
  host_build_result_digest CHAR(64) NOT NULL,
  not_after TIMESTAMPTZ NOT NULL,
  decision_json TEXT NOT NULL,
  admitted_event_digest CHAR(64) NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (decision_digest),
  CONSTRAINT consensus_decisions_subject_uq UNIQUE (computer_id, subject_digest, standing_generation)
);
CREATE TABLE IF NOT EXISTS consensus_guest_core_keys (
  computer_id VARCHAR(128) NOT NULL,
  key_id VARCHAR(255) NOT NULL,
  public_key VARCHAR(128) NOT NULL,
  pinned_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id)
);`

// consensusAdmissionGrace tolerates the seconds between the guest's
// decision read and its append, as the platform offer gate does.
const consensusAdmissionGrace = platformUpdateOfferAdmissionGrace

// IsConsensusAuthority reports an effect_accepted authority that names a
// consensus decision.
func IsConsensusAuthority(authorityRef string) bool {
	return strings.HasPrefix(strings.TrimSpace(authorityRef), AuthorityQualifiedConsensus)
}

var (
	platformTiersOnce sync.Once
	platformTiers     computertiers.Tiers
	platformTiersErr  error
)

// platformComputer reports a computer on the platform tier, which no
// standing arm may name whatever the proxy attests. An unreadable tier file
// refuses every standing arm (fail closed).
func platformComputer(computerID string) (bool, error) {
	platformTiersOnce.Do(func() { platformTiers, platformTiersErr = computertiers.Load() })
	if platformTiersErr != nil {
		return true, platformTiersErr
	}
	return platformTiers.PlatformComputer(computerID), nil
}

// standingArmRefusal is the authority a standing_consensus arm needs: the
// owner (host-internal), a verified owner act of kind selfdev.autonomy.arm
// whose subject the proxy signed with computer_tier "test", and a computer
// that is not a platform computer. It applies in observe as in enforce.
func standingArmRefusal(writer, computerID string, ownerAct *owneract.Verified) error {
	if writer != ModeWriterHostInternal {
		return ErrSelfDevelopmentModeOwnerRequired
	}
	if ownerAct == nil {
		return fmt.Errorf("%w: standing_consensus requires a verified owner act (in observe too)", ErrOwnerActRefused)
	}
	if ownerAct.ActKind != owneract.ActSelfDevAutonomyArm {
		return fmt.Errorf("%w: standing_consensus requires act %s", ErrOwnerActRefused, owneract.ActSelfDevAutonomyArm)
	}
	if tier := ownerAct.SubjectString("computer_tier"); tier != ComputerTierTest {
		return fmt.Errorf("%w: standing autonomy is allowed only on test-tier computers (owner act names tier %q)", ErrOwnerActRefused, tier)
	}
	if platform, err := platformComputer(computerID); platform || err != nil {
		return fmt.Errorf("%w: standing autonomy is refused on a platform computer", ErrOwnerActRefused)
	}
	return nil
}

// ConsensusRequest is the guest's request for a decision on one operation.
type ConsensusRequest struct {
	OperationID string                       `json:"operation_id"`
	Subject     decisionpolicy.EffectSubject `json:"subject"`
	// ReviewerBallot is management's Verify verdict, signed by guest-core.
	ReviewerBallot computerevent.Receipt `json:"reviewer_ballot"`
	// ReviewerPublicKey is the guest-core public key (base64, raw std),
	// pinned on first use per computer.
	ReviewerPublicKey string `json:"reviewer_public_key"`
	// Bundle is the frozen bundle's canonical bytes: the poster reads the
	// source patch sha and base commit the host build job is named by.
	Bundle []byte `json:"bundle"`
	// BaseImageManifestDigest is the computer's booted base, the last part
	// of P5's job identity (the host refused any base it had not deployed).
	BaseImageManifestDigest string `json:"base_image_manifest_digest"`
	// HostBuildJobID is the job the guest built; it must be the job the
	// poster derives (a legible refusal, not an input to the identity).
	HostBuildJobID string `json:"host_build_job_id,omitempty"`
}

// ConsensusResponse is a decision, or the dissent that stopped one.
type ConsensusResponse struct {
	Decision       *computerevent.Receipt `json:"decision,omitempty"`
	DecisionDigest string                 `json:"decision_digest,omitempty"`
	Dissent        bool                   `json:"dissent,omitempty"`
	Pending        bool                   `json:"pending,omitempty"`
	HostBuildJobID string                 `json:"host_build_job_id,omitempty"`
	Reason         string                 `json:"reason,omitempty"`
	BallotDigests  []string               `json:"ballot_digests,omitempty"`
}

// ConsensusActuator is corpusd's decision poster.
type ConsensusActuator struct {
	store      *Store
	modes      *SelfDevelopmentModeCAS
	signingKey computerevent.SigningKey
	policies   *decisionpolicy.Store
	hostBuilds HostBuildJobs
	now        func() time.Time
	locks      [64]sync.Mutex
}

// NewConsensusActuator wires the poster. hostBuilds may be nil: the poster
// then refuses every request as unavailable.
func NewConsensusActuator(store *Store, modes *SelfDevelopmentModeCAS, signingKey computerevent.SigningKey, hostBuilds HostBuildJobs) (*ConsensusActuator, error) {
	if store == nil || store.db == nil || modes == nil || signingKey.SignerDomain != "platform-control" || signingKey.KeyID == "" || len(signingKey.PrivateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("consensus actuator: store, mode authority and platform-control signer are required")
	}
	return &ConsensusActuator{
		store: store, modes: modes, signingKey: signingKey, policies: decisionpolicy.MustReversibleSelfDevV1Store(),
		hostBuilds: hostBuilds, now: func() time.Time { return time.Now().UTC() },
	}, nil
}

// SetClock overrides the poster's clock (tests).
func (a *ConsensusActuator) SetClock(now func() time.Time) { a.now = now }

// StandingArm is the current standing mode, verified for autonomy.
type standingArm struct {
	mode          SelfDevelopmentMode
	receiptDigest string
	expiresAt     time.Time
}

// currentStandingArm reads the mode (lazy expiry applies) and requires an
// unexpired, test-tier standing arm under an allowed policy.
func (a *ConsensusActuator) currentStandingArm(ctx context.Context, computerID string) (standingArm, error) {
	mode, err := a.modes.Get(ctx, computerID)
	if err != nil {
		return standingArm{}, err
	}
	return standingArmFromMode(mode, a.now())
}

func standingArmFromMode(mode SelfDevelopmentMode, now time.Time) (standingArm, error) {
	if mode.Mode != SelfDevelopmentModeStandingConsensus || mode.Receipt == nil {
		return standingArm{}, fmt.Errorf("%w: no standing_consensus arm (mode %s)", ErrConsensusRefused, mode.Mode)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, mode.ExpiresAt)
	if err != nil || !now.Before(expiresAt) {
		return standingArm{}, fmt.Errorf("%w: the standing window has expired", ErrConsensusRefused)
	}
	if tier, _ := mode.Receipt.KindFields["computer_tier"].(string); tier != ComputerTierTest {
		return standingArm{}, fmt.Errorf("%w: the standing arm is not test-tier", ErrConsensusRefused)
	}
	if !decisionpolicy.StandingPolicyAllowed(mode.PolicyDigest) {
		return standingArm{}, fmt.Errorf("%w: the standing arm's policy may not be autonomous", ErrConsensusRefused)
	}
	raw, err := mode.Receipt.CanonicalBytes()
	if err != nil {
		return standingArm{}, err
	}
	return standingArm{mode: mode, receiptDigest: computerevent.DigestBytes(raw), expiresAt: expiresAt.UTC()}, nil
}

// guestCoreKeyPin resolves the one guest-core key pinned for a computer.
type guestCoreKeyPin struct {
	keyID  string
	public ed25519.PublicKey
}

func (p guestCoreKeyPin) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != decisionpolicy.SignerDomainGuestCore || keyID != p.keyID {
		return nil, fmt.Errorf("guest-core key %s/%s is not the computer's pinned key", domain, keyID)
	}
	return p.public, nil
}

// pinGuestCoreKey pins the computer's guest-core key on first use and
// refuses any other key afterwards (attribution, not a trust anchor: S4
// computer identity keys are the repair).
func (a *ConsensusActuator) pinGuestCoreKey(ctx context.Context, computerID, keyID, encoded string) (guestCoreKeyPin, error) {
	public, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(public) != ed25519.PublicKeySize || strings.TrimSpace(keyID) == "" {
		return guestCoreKeyPin{}, fmt.Errorf("%w: reviewer public key", ErrConsensusRefused)
	}
	var pinnedID, pinnedKey string
	err = a.store.db.QueryRowContext(ctx, `SELECT key_id, public_key FROM consensus_guest_core_keys WHERE computer_id=?`, computerID).Scan(&pinnedID, &pinnedKey)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := a.store.db.ExecContext(ctx, `INSERT INTO consensus_guest_core_keys (computer_id, key_id, public_key, pinned_at) VALUES (?, ?, ?, ?)`,
			computerID, keyID, base64.RawStdEncoding.EncodeToString(public), a.now()); err != nil {
			return guestCoreKeyPin{}, fmt.Errorf("consensus actuator: pin guest-core key: %w", err)
		}
		log.Printf("consensus: pinned guest-core key %s for computer %s (first use)", keyID, computerID)
	case err != nil:
		return guestCoreKeyPin{}, err
	case pinnedID != keyID || pinnedKey != base64.RawStdEncoding.EncodeToString(public):
		return guestCoreKeyPin{}, fmt.Errorf("%w: the reviewer ballot's guest-core key is not the key pinned for this computer", ErrConsensusRefused)
	}
	return guestCoreKeyPin{keyID: keyID, public: ed25519.PublicKey(public)}, nil
}

// Decide is the poster: it decides one operation under the standing arm.
func (a *ConsensusActuator) Decide(ctx context.Context, computerID string, request ConsensusRequest) (ConsensusResponse, error) {
	if a == nil {
		return ConsensusResponse{}, ErrConsensusUnavailable
	}
	computerID = strings.TrimSpace(computerID)
	lock := &a.locks[computerEventLockIndex(computerID)]
	lock.Lock()
	defer lock.Unlock()

	arm, err := a.currentStandingArm(ctx, computerID)
	if err != nil {
		return ConsensusResponse{}, err
	}
	subject := request.Subject
	switch {
	case computerID == "" || subject.ComputerID != computerID:
		return ConsensusResponse{}, fmt.Errorf("%w: subject is for another computer", ErrConsensusRefused)
	case strings.TrimSpace(request.OperationID) == "" || subject.OperationID != request.OperationID:
		return ConsensusResponse{}, fmt.Errorf("%w: subject is for another operation", ErrConsensusRefused)
	case subject.EffectClass != decisionpolicy.EffectClassReversible || subject.Irreversible() || subject.OwnerRecovery:
		return ConsensusResponse{}, fmt.Errorf("%w: subject is not a reversible computer-local effect", ErrConsensusRefused)
	case !computerevent.IsSHA256(subject.BundleDigest) || computerevent.DigestBytes(request.Bundle) != subject.BundleDigest:
		return ConsensusResponse{}, fmt.Errorf("%w: bundle bytes do not hash to the subject's bundle digest", ErrConsensusRefused)
	}
	head, err := readComputerEventHead(ctx, a.store.db, computerID, false)
	if err != nil {
		return ConsensusResponse{}, err
	}
	if head == nil || head.DesiredEventHead != subject.DesiredEventHead || head.EffectiveEventHead != subject.EffectiveEventHead ||
		head.PendingTransitionRef != subject.PendingTransitionRef || head.DesiredStateCommitment != subject.DesiredStateCommitment ||
		head.EffectiveStateCommitment != subject.EffectiveStateCommitment {
		return ConsensusResponse{}, fmt.Errorf("%w: subject heads are not corpusd's current head", ErrConsensusStale)
	}
	subjectDigest, err := subject.Digest()
	if err != nil {
		return ConsensusResponse{}, err
	}
	if existing, found, err := a.existingDecision(ctx, computerID, subjectDigest, arm.mode.Generation); err != nil || found {
		return existing, err
	}
	now := a.now().UTC().Truncate(time.Microsecond)
	policyDigest := arm.mode.PolicyDigest
	want := func(seat string) decisionpolicy.BallotExpectation {
		return decisionpolicy.BallotExpectation{Seat: seat, PolicyDigest: policyDigest, SubjectDigest: subjectDigest, ComputerID: computerID, OperationID: request.OperationID, Now: now}
	}

	// The independent-reviewer seat: management's Verify verdict, signed by
	// the computer's pinned guest-core key.
	if len(request.ReviewerBallot.RequiredSigners) != 1 {
		return ConsensusResponse{}, fmt.Errorf("%w: reviewer ballot must carry one signature", ErrConsensusRefused)
	}
	pin, err := a.pinGuestCoreKey(ctx, computerID, request.ReviewerBallot.RequiredSigners[0].KeyID, request.ReviewerPublicKey)
	if err != nil {
		return ConsensusResponse{}, err
	}
	reviewer, err := decisionpolicy.VerifyBallot(request.ReviewerBallot, pin, want(decisionpolicy.SeatIndependentReviewer))
	if err != nil {
		return ConsensusResponse{}, fmt.Errorf("%w: reviewer ballot: %v", ErrConsensusRefused, err)
	}

	// The capsule-verifier seat: corpusd's vote on its own P5 build of this
	// operation, signed by platform-control, a key no guest holds.
	hostVote, err := a.castHostBuildBallot(computerID, request, policyDigest, subjectDigest, now)
	if errors.Is(err, ErrConsensusPending) {
		return ConsensusResponse{Pending: true, Reason: err.Error(), HostBuildJobID: hostVote.job.JobID}, err
	}
	if err != nil {
		return ConsensusResponse{}, err
	}
	host, err := decisionpolicy.VerifyBallot(hostVote.ballot, platformControlKey{keyID: a.signingKey.KeyID, public: a.signingKey.PrivateKey.Public().(ed25519.PublicKey)}, want(decisionpolicy.SeatCapsuleVerifier))
	if err != nil {
		return ConsensusResponse{}, fmt.Errorf("%w: host build ballot: %v", ErrConsensusRefused, err)
	}
	ballotDigests := []string{host.Digest, reviewer.Digest}

	// The bundle: seat manifest from the policy, the selection bound to the
	// owner's standing arm (the policy was selected before any output),
	// and the reducer's join records for the two verified ballots.
	policy, _, err := a.policies.Get(policyDigest)
	if err != nil {
		return ConsensusResponse{}, fmt.Errorf("%w: policy: %v", ErrConsensusRefused, err)
	}
	manifest := decisionpolicy.SeatManifestForPolicy(policy, map[string]string{
		decisionpolicy.SeatAuthor:              "engineering desk, author of the bundle (recused from verification)",
		decisionpolicy.SeatCapsuleVerifier:     "corpusd host build " + hostVote.job.JobID + " (platform-control " + host.Signer.KeyID + "): the bundle's source patch built at its base commit on the computer's booted base",
		decisionpolicy.SeatIndependentReviewer: "management Verify verdict, guest-core " + reviewer.Signer.KeyID,
	})
	manifestDigest, err := manifest.Digest()
	if err != nil {
		return ConsensusResponse{}, err
	}
	selection := decisionpolicy.PolicySelectionReceipt{
		ReceiptKind: decisionpolicy.ReceiptKindPolicySelection, PolicyDigest: policyDigest,
		SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
		SelectedAtHead: arm.receiptDigest, SelectedSequence: arm.mode.Generation,
	}
	if selection.SelectionDigest, err = selection.Digest(); err != nil {
		return ConsensusResponse{}, err
	}
	var attestations []decisionpolicy.BallotAttestation
	for _, verified := range []decisionpolicy.VerifiedBallot{host, reviewer} {
		attestation, err := decisionpolicy.AttestationFromSignedBallot(verified, manifestDigest, selection.SelectionDigest)
		if err != nil {
			return ConsensusResponse{}, err
		}
		attestations = append(attestations, attestation)
	}
	bundle := decisionpolicy.ConsensusBundle{Manifest: manifest, Subject: subject, Selection: selection, Ballots: attestations, Now: now.Format(time.RFC3339Nano)}
	qualified, err := a.policies.ReduceBundle(bundle)
	if err != nil {
		reason := err.Error()
		if hostVote.reason != "" {
			reason += ": host build: " + hostVote.reason
		}
		log.Printf("consensus: computer=%s operation=%s dissent: %s", computerID, request.OperationID, reason)
		return ConsensusResponse{Dissent: true, Reason: reason, BallotDigests: ballotDigests, HostBuildJobID: hostVote.job.JobID}, fmt.Errorf("%w: %s", ErrConsensusDissent, reason)
	}

	notAfter := now.Add(decisionpolicy.MaxBallotTTL)
	if arm.expiresAt.Before(notAfter) {
		notAfter = arm.expiresAt
	}
	fields := map[string]any{
		"computer_id": computerID, "operation_id": request.OperationID,
		"subject": subject, "subject_digest": subjectDigest,
		"policy_id": policy.PolicyID, "policy_digest": policyDigest,
		"qualified_consensus_receipt": qualified, "qualified_consensus_receipt_digest": qualified.ReceiptDigest,
		"ballots":                      []computerevent.Receipt{host.Receipt, reviewer.Receipt},
		"ballot_receipt_digests":       ballotDigests,
		"standing_mode_receipt_digest": arm.receiptDigest, "standing_generation": arm.mode.Generation,
		"computer_tier": ComputerTierTest, "not_after": notAfter.Format(time.RFC3339Nano),
		"host_build": map[string]any{
			"job_id": hostVote.job.JobID, "result_digest": hostVote.resultDigest,
			"closure_digest": hostVote.job.Result.ClosureDigest, "builder_receipt_digest": hostVote.job.Result.BuilderReceiptDigest,
			"code_commit": hostVote.job.Result.CodeCommit,
		},
	}
	normalized, err := normalizeFields(fields)
	if err != nil {
		return ConsensusResponse{}, err
	}
	decision, err := computerevent.NewSignedReceipt(decisionpolicy.DecisionReceiptKind, "corpusd", normalized, []computerevent.SigningKey{a.signingKey}, now)
	if err != nil {
		return ConsensusResponse{}, err
	}
	raw, err := decision.CanonicalBytes()
	if err != nil {
		return ConsensusResponse{}, err
	}
	digest := computerevent.DigestBytes(raw)
	_, err = a.store.db.ExecContext(ctx, `INSERT INTO consensus_decisions (decision_digest, computer_id, operation_id, subject_digest, bundle_digest, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment, policy_digest, standing_mode_receipt_digest, standing_generation, host_build_job_id, host_build_result_digest, not_after, decision_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		digest, computerID, request.OperationID, subjectDigest, subject.BundleDigest, subject.DesiredEventHead, subject.EffectiveEventHead,
		subject.PendingTransitionRef, subject.DesiredStateCommitment, subject.EffectiveStateCommitment, policyDigest,
		arm.receiptDigest, arm.mode.Generation, hostVote.job.JobID, hostVote.resultDigest, notAfter, string(raw), now)
	if err != nil {
		return ConsensusResponse{}, fmt.Errorf("consensus actuator: record decision: %w", err)
	}
	log.Printf("consensus: computer=%s operation=%s bundle=%s decision=%s host_build=%s host=%s reviewer=%s", computerID, request.OperationID, subject.BundleDigest, digest, hostVote.job.JobID, host.Digest, reviewer.Digest)
	return ConsensusResponse{Decision: &decision, DecisionDigest: digest, BallotDigests: ballotDigests, HostBuildJobID: hostVote.job.JobID}, nil
}

// platformControlKey resolves corpusd's own platform-control key, the only
// key the capsule-verifier seat admits.
type platformControlKey struct {
	keyID  string
	public ed25519.PublicKey
}

func (k platformControlKey) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != decisionpolicy.SignerDomainHostBuild || keyID != k.keyID {
		return nil, fmt.Errorf("%s/%s is not corpusd's platform-control key", domain, keyID)
	}
	return k.public, nil
}

// hostBuildVote is the capsule-verifier ballot corpusd cast on a host build.
type hostBuildVote struct {
	ballot       computerevent.Receipt
	job          hostbuild.Job
	resultDigest string
	reason       string
}

// bundleSourcePatch is what the poster reads from the frozen bundle bytes
// (transaction.CapsuleEffectBundle's fields) to name the host build job.
type bundleSourcePatch struct {
	SourcePatchSHA256     string `json:"source_patch_sha256"`
	SourcePatchBaseCommit string `json:"source_patch_base_commit"`
}

// castHostBuildBallot is the capsule-verifier seat. The job is P5's, named
// by the identity corpusd derives (the route's computer, the request's
// operation, the subject's bundle, the bundle's own source patch sha and
// base commit, the booted base), never by a job id the guest picks: a job
// done and verified under platform-control is an accept naming the build; a
// failed job is a reject with its reason code; a queued or running job is
// pending; no job is a refusal.
func (a *ConsensusActuator) castHostBuildBallot(computerID string, request ConsensusRequest, policyDigest, subjectDigest string, now time.Time) (hostBuildVote, error) {
	if a.hostBuilds == nil {
		return hostBuildVote{}, fmt.Errorf("%w: no host build service configured", ErrConsensusUnavailable)
	}
	var fields bundleSourcePatch
	if err := json.Unmarshal(request.Bundle, &fields); err != nil {
		return hostBuildVote{}, fmt.Errorf("%w: bundle bytes do not decode", ErrConsensusRefused)
	}
	build := hostbuild.Request{
		ComputerID: computerID, OperationID: request.OperationID, BundleDigest: request.Subject.BundleDigest,
		BaseCommit: strings.ToLower(strings.TrimSpace(fields.SourcePatchBaseCommit)), SourcePatchSHA256: strings.TrimSpace(fields.SourcePatchSHA256),
		BaseImageManifestDigest: strings.TrimSpace(request.BaseImageManifestDigest),
	}
	if !computerevent.IsSHA256(build.SourcePatchSHA256) || build.BaseCommit == "" {
		return hostBuildVote{}, fmt.Errorf("%w: the bundle names no source patch and base commit; the host cannot build it", ErrConsensusRefused)
	}
	if err := build.Validate(); err != nil {
		return hostBuildVote{}, fmt.Errorf("%w: %v", ErrConsensusRefused, err)
	}
	jobID, err := build.JobID()
	if err != nil {
		return hostBuildVote{}, err
	}
	if named := strings.TrimSpace(request.HostBuildJobID); named != "" && named != jobID {
		return hostBuildVote{}, fmt.Errorf("%w: host build job %s is not this operation's (%s)", ErrConsensusRefused, named, jobID)
	}
	job, ok := a.hostBuilds.Get(computerID, jobID)
	if !ok {
		return hostBuildVote{}, fmt.Errorf("%w: no host build for this operation (job %s); submit it first", ErrConsensusRefused, jobID)
	}
	vote := hostBuildVote{job: job}
	var choice string
	evidence := []string{"host-build-job:" + jobID}
	switch job.State {
	case hostbuild.StateQueued, hostbuild.StateRunning:
		return vote, fmt.Errorf("%w: host build %s is %s", ErrConsensusPending, jobID, job.State)
	case hostbuild.StateFailed:
		choice = decisionpolicy.VoteReject
		vote.reason = strings.TrimSpace(job.Reason + ": " + job.Detail)
		evidence = append(evidence, "host-build-reason:"+job.Reason)
	case hostbuild.StateDone:
		if err := hostbuild.VerifyJob(job, build, a.signingKey.PrivateKey.Public().(ed25519.PublicKey)); err != nil {
			return hostBuildVote{}, fmt.Errorf("%w: host build %s: %v", ErrConsensusRefused, jobID, err)
		}
		if vote.resultDigest, err = job.Result.Digest(); err != nil {
			return hostBuildVote{}, err
		}
		choice = decisionpolicy.VoteAccept
		evidence = append(evidence, "host-build-result:sha256:"+vote.resultDigest, "closure:sha256:"+job.Result.ClosureDigest,
			"builder-receipt:sha256:"+job.Result.BuilderReceiptDigest, "code-commit:"+job.Result.CodeCommit)
	default:
		return hostBuildVote{}, fmt.Errorf("%w: host build %s has unknown state %q", ErrConsensusRefused, jobID, job.State)
	}
	ballot, err := decisionpolicy.NewBallot(decisionpolicy.SeatCapsuleVerifier, choice, policyDigest, subjectDigest,
		computerID, request.OperationID, "", "", evidence, now, 0)
	if err != nil {
		return hostBuildVote{}, err
	}
	if vote.ballot, err = decisionpolicy.SignBallot(ballot, a.signingKey, decisionpolicy.IssuerHostBuild, now); err != nil {
		return hostBuildVote{}, err
	}
	return vote, nil
}

// existingDecision returns the decision already minted for this subject
// under this arm, so a retried request gets the same decision.
func (a *ConsensusActuator) existingDecision(ctx context.Context, computerID, subjectDigest string, generation uint64) (ConsensusResponse, bool, error) {
	var digest, raw string
	err := a.store.db.QueryRowContext(ctx, `SELECT decision_digest, decision_json FROM consensus_decisions WHERE computer_id=? AND subject_digest=? AND standing_generation=?`, computerID, subjectDigest, generation).Scan(&digest, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return ConsensusResponse{}, false, nil
	}
	if err != nil {
		return ConsensusResponse{}, false, err
	}
	var decision computerevent.Receipt
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return ConsensusResponse{}, false, err
	}
	return ConsensusResponse{Decision: &decision, DecisionDigest: digest}, true, nil
}

// normalizeFields renders kind fields through JSON so the signed fields are
// exactly what a reader decodes.
func normalizeFields(fields map[string]any) (map[string]any, error) {
	raw, err := computerevent.CanonicalJSON(fields)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var out map[string]any
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// consensusDecisionRefusal is the admission gate's consensus branch: an
// effect_accepted citing qualified-consensus:<d> is admitted only when d is
// a decision this poster minted for exactly this event, still inside its
// window, under a standing arm that is still the computer's current mode.
func consensusDecisionRefusal(ctx context.Context, tx *sql.Tx, event computerevent.Event, eventDigest string, now time.Time) (string, func() error, error) {
	digest, ok := decisionpolicy.ReceiptDigestFromAuthority(event.AuthorityRef)
	if !ok {
		return "qualified-consensus authority names no decision digest", nil, nil
	}
	var row struct {
		computerID, operationID, bundle, desired, effective, pending, desiredCommitment, effectiveCommitment, standingDigest, admitted string
		generation                                                                                                                     uint64
		notAfter                                                                                                                       time.Time
	}
	err := tx.QueryRowContext(ctx, `SELECT computer_id, operation_id, bundle_digest, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment, standing_mode_receipt_digest, COALESCE(admitted_event_digest, ''), standing_generation, not_after FROM consensus_decisions WHERE decision_digest=?`, digest).
		Scan(&row.computerID, &row.operationID, &row.bundle, &row.desired, &row.effective, &row.pending, &row.desiredCommitment, &row.effectiveCommitment, &row.standingDigest, &row.admitted, &row.generation, &row.notAfter)
	if errors.Is(err, sql.ErrNoRows) {
		return "qualified-consensus names no decision corpusd's poster minted", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	switch {
	case row.computerID != event.ComputerID:
		return "the consensus decision was minted for another computer", nil, nil
	case row.operationID != event.ParentEventID || row.bundle != event.ProposedEffectRef:
		return "the consensus decision is for another operation or bundle", nil, nil
	case row.desired != event.ExpectedDesiredEventHead || row.effective != event.ExpectedEffectiveEventHead ||
		row.pending != event.ExpectedPendingTransitionRef || row.desiredCommitment != event.ExpectedDesiredStateCommitment ||
		row.effectiveCommitment != event.ExpectedEffectiveStateCommitment:
		return "the consensus decision does not bind this event's heads and commitments", nil, nil
	case now.After(row.notAfter.UTC().Add(consensusAdmissionGrace)):
		return "the consensus decision has expired", nil, nil
	case row.admitted != "" && row.admitted != eventDigest:
		return "the consensus decision already admitted another event", nil, nil
	}
	cited := false
	for i, ref := range event.InputArtifactRefs {
		parsed, err := computerevent.ParseArtifactRef(ref)
		if err != nil {
			return "an input artifact is not an artifact reference", nil, nil
		}
		if i == 0 && parsed.Digest().String() != row.standingDigest {
			return "input artifact 0 is not the standing arm the decision was made under", nil, nil
		}
		if parsed.Digest().String() == digest {
			cited = true
		}
	}
	if !cited {
		return "the event does not carry the consensus decision it cites", nil, nil
	}
	mode, found, err := readSelfDevelopmentMode(ctx, tx, event.ComputerID, false)
	if err != nil {
		return "", nil, err
	}
	if !found || mode.Generation != row.generation {
		return "the standing arm the decision was made under is no longer the computer's mode", nil, nil
	}
	if _, err := standingArmFromMode(mode, now); err != nil {
		return "the standing arm no longer authorizes autonomy: " + err.Error(), nil, nil
	}
	return "", func() error {
		_, err := tx.ExecContext(ctx, `UPDATE consensus_decisions SET admitted_event_digest=? WHERE decision_digest=? AND (admitted_event_digest IS NULL OR admitted_event_digest=?)`, eventDigest, digest, eventDigest)
		return err
	}, nil
}

// ConfigureConsensusActuator installs the decision poster's route.
func (h *Handler) ConfigureConsensusActuator(actuator *ConsensusActuator) error {
	if h == nil || actuator == nil {
		return fmt.Errorf("corpusd handler: consensus actuator is required")
	}
	h.consensus = actuator
	return nil
}

// HandleSelfDevelopmentConsensus is POST
// /internal/computers/self-development/consensus?computer_id=: the guest of
// that computer (event:append) asks the poster for a decision.
func (h *Handler) HandleSelfDevelopmentConsensus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.consensus == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "consensus decision poster unavailable"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if computerID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "computer_id is required"})
		return
	}
	if !trustedInternalCaller(r) && (h.eventAuth == nil || h.eventAuth.Authorize(r, computerID, "event:append") != nil) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "consensus request capability refused"})
		return
	}
	var request ConsensusRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxConsensusRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid consensus request"})
		return
	}
	response, err := h.consensus.Decide(r.Context(), computerID, request)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, response)
	case errors.Is(err, ErrConsensusDissent), errors.Is(err, ErrConsensusPending):
		writeJSON(w, http.StatusConflict, response)
	case errors.Is(err, ErrConsensusStale):
		writeJSON(w, http.StatusConflict, apiError{Error: err.Error()})
	case errors.Is(err, ErrConsensusRefused):
		log.Printf("consensus: computer=%s operation=%s refused: %v", computerID, request.OperationID, err)
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
	case errors.Is(err, ErrConsensusUnavailable):
		log.Printf("consensus: computer=%s operation=%s unavailable: %v", computerID, request.OperationID, err)
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: err.Error()})
	default:
		log.Printf("consensus: computer=%s operation=%s failed: %v", computerID, request.OperationID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "consensus decision failed"})
	}
}
