package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// P4: the standing arm, the decision poster and the gate's consensus branch
// (docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md). The numbers
// in comments are that record's failure modes.

func mintStandingAct(t *testing.T, signer *owneract.Signer, computerID string, request SetSelfDevelopmentModeRequest, tier string, now time.Time) string {
	t.Helper()
	subject, err := OwnerActModeSubjectWithTier(computerID, request, tier)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := signer.Mint(owneract.Act{
		Kind: OwnerActKindForMode(request.Mode), OwnerUserID: "owner-1", AuthMethod: owneract.AuthMethodAPIKey,
		CredentialRef: "api-key:test", ComputerID: computerID, Subject: subject,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	return encodeAct(t, receipt)
}

func standingRequest(generation uint64, key string, expires time.Time) SetSelfDevelopmentModeRequest {
	return SetSelfDevelopmentModeRequest{
		Mode: SelfDevelopmentModeStandingConsensus, ExpectedGeneration: generation, IdempotencyKey: key,
		PolicyDigest: decisionpolicy.PolicyDigestReversibleSelfDevV1,
		ExpiresAt:    expires.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano),
	}
}

func TestStandingArmIsOwnerOnlyTestTierBoundedAndNeverConsumed(t *testing.T) {
	f := newModeAuthorityFixture(t)
	signer := newTestOwnerActSigner(t)
	// Observe: S1 would let a receipt-less owner write through; the
	// standing arm still needs a verified owner act.
	f.modes.ConfigureOwnerActs(testOwnerActAuthority(t, OwnerActObserve, signer))
	now := time.Now().UTC()
	arm := standingRequest(0, "standing-1", now.Add(2*time.Hour))
	unchanged := func(label string) {
		t.Helper()
		if mode := f.current(t); mode.Mode != SelfDevelopmentModeOff || mode.Generation != 0 {
			t.Fatalf("%s changed the mode: %+v", label, mode)
		}
	}

	// 16: no owner act, an owner act that names no tier, a primary-tier
	// computer, and an act of the wrong kind.
	if code, _, body := f.ownerWith(t, arm, ""); code != http.StatusForbidden {
		t.Fatalf("no owner act (observe): %d %s", code, body)
	}
	if code, _, body := f.ownerWith(t, arm, mintStandingAct(t, signer, f.computerID, arm, "", now)); code != http.StatusForbidden || !strings.Contains(body, "test-tier") {
		t.Fatalf("no tier: %d %s", code, body)
	}
	if code, _, body := f.ownerWith(t, arm, mintStandingAct(t, signer, f.computerID, arm, "primary", now)); code != http.StatusForbidden {
		t.Fatalf("primary tier: %d %s", code, body)
	}
	unchanged("refused arms")
	// 18: the guest cannot arm it.
	if code, _ := f.guest(t, arm); code != http.StatusForbidden {
		t.Fatalf("guest standing arm: %d", code)
	}
	// 19 and 14: other policies, operation bindings, a consensus receipt, a
	// window over 24 hours, a past or non-canonical expiry.
	bad := map[string]SetSelfDevelopmentModeRequest{}
	other := arm
	other.PolicyDigest = decisionpolicy.PolicyDigestHumanRequiredV1
	bad["human-required policy"] = other
	bound := arm
	bound.OperationID = "op-1"
	bad["operation binding"] = bound
	receipt := arm
	receipt.ConsensusReceiptDigest = strings.Repeat("c", 64)
	bad["consensus receipt"] = receipt
	bad["25 hours"] = standingRequest(0, "standing-1", now.Add(25*time.Hour))
	bad["past"] = standingRequest(0, "standing-1", now.Add(-time.Minute))
	nonCanonical := arm
	nonCanonical.ExpiresAt = now.Add(time.Hour).Format(time.RFC1123)
	bad["non-canonical"] = nonCanonical
	for name, request := range bad {
		if code, _, body := f.ownerWith(t, request, mintStandingAct(t, signer, f.computerID, request, ComputerTierTest, now)); code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", name, code, body)
		}
	}
	unchanged("invalid arms")

	// The owner's test-tier arm.
	code, armed, body := f.ownerWith(t, arm, mintStandingAct(t, signer, f.computerID, arm, ComputerTierTest, now))
	if code != http.StatusOK || armed.Mode != SelfDevelopmentModeStandingConsensus || armed.PolicyDigest != decisionpolicy.PolicyDigestReversibleSelfDevV1 {
		t.Fatalf("test-tier arm: %d %s", code, body)
	}
	if armed.Receipt.KindFields["computer_tier"] != ComputerTierTest || armed.Receipt.KindFields["owner_act_receipt_digest"] == nil || modeWriter(armed) != ModeWriterHostInternal {
		t.Fatalf("standing ModeReceipt fields: %#v", armed.Receipt.KindFields)
	}
	// Never consumed: a guest consumption of the standing arm is refused.
	for _, prefix := range []string{AcceptOnceConsumptionKeyPrefix, QualifiedConsensusConsumptionKeyPrefix} {
		if code, _ := f.guest(t, consume(prefix, "op-1", 1)); code != http.StatusForbidden {
			t.Fatalf("guest consumed the standing arm with %s: %d", prefix, code)
		}
	}
	// 11: after the window, the first read writes off.
	f.modes.now = func() time.Time { return now.Add(3 * time.Hour) }
	if mode := f.current(t); mode.Mode != SelfDevelopmentModeOff || modeWriter(mode) != ModeWriterCorpusdExpiry {
		t.Fatalf("expired standing arm: %+v", mode)
	}
}

func TestStandingArmRefusesAPlatformComputer(t *testing.T) {
	f := newModeAuthorityFixture(t)
	signer := newTestOwnerActSigner(t)
	f.modes.ConfigureOwnerActs(testOwnerActAuthority(t, OwnerActEnforce, signer))
	f.computerID = "computer-1355b83bced1613eb048ee28d66688a8" // the Autopaper paper computer
	arm := standingRequest(0, "standing-paper", time.Now().Add(time.Hour))
	if code, _, body := f.ownerWith(t, arm, mintStandingAct(t, signer, f.computerID, arm, ComputerTierTest, time.Now())); code != http.StatusForbidden || !strings.Contains(body, "platform computer") {
		t.Fatalf("platform computer arm: %d %s", code, body)
	}
}

// --- the decision poster ---

// stubHostBuilds stands in for P5's hostbuild.Service: job records by id,
// visible only to the computer that owns them (H13).
type stubHostBuilds struct{ jobs map[string]hostbuild.Job }

func (s *stubHostBuilds) Get(computerID, jobID string) (hostbuild.Job, bool) {
	job, ok := s.jobs[jobID]
	if !ok || job.ComputerID != computerID {
		return hostbuild.Job{}, false
	}
	return job, true
}

type consensusFixture struct {
	*admissionFixture
	actuator   *ConsensusActuator
	builds     *stubHostBuilds
	guestKey   computerevent.SigningKey
	head       computerevent.Head
	bundle     []byte
	baseCommit string
	patchSHA   string
	bootedBase string
	modeKey    computerevent.SigningKey
	armedMode  SelfDevelopmentMode
}

func newConsensusFixture(t *testing.T, mode OwnerActMode) *consensusFixture {
	t.Helper()
	af := newAdmissionFixture(t, mode)
	head := af.seedHead(t, af.computerID)
	builds := &stubHostBuilds{jobs: map[string]hostbuild.Job{}}
	actuator, err := NewConsensusActuator(af.store, af.modes, af.modes.signingKey, builds)
	if err != nil {
		t.Fatal(err)
	}
	_, guestPrivate, _ := ed25519.GenerateKey(rand.Reader)
	f := &consensusFixture{admissionFixture: af, actuator: actuator, builds: builds, head: head,
		guestKey: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: decisionpolicy.SignerDomainGuestCore, KeyID: "guest-core-test"}, PrivateKey: guestPrivate},
		modeKey:  af.modes.signingKey, baseCommit: strings.Repeat("a", 40), bootedBase: digestOf('b')}
	sum := sha256.Sum256([]byte("diff --git a/x b/x\n"))
	f.patchSHA = hex.EncodeToString(sum[:])
	f.bundle, err = computerevent.CanonicalJSON(map[string]any{
		"computer_id": af.computerID, "content_digest": digestOf('c'),
		"source_patch_sha256": f.patchSHA, "source_patch_base_commit": f.baseCommit,
		"test_receipts": []string{"capsule-exec:sha256:" + digestOf('7')},
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// hostRequest is P5's job identity for an operation of this fixture.
func (f *consensusFixture) hostRequest(operationID string) hostbuild.Request {
	return hostbuild.Request{
		ComputerID: f.computerID, OperationID: operationID, BundleDigest: computerevent.DigestBytes(f.bundle),
		BaseCommit: f.baseCommit, SourcePatchSHA256: f.patchSHA, BaseImageManifestDigest: f.bootedBase,
	}
}

// recordBuild records a host build job for the request in the given state:
// done (signed by key), failed (with a P5 reason code) or queued.
func (f *consensusFixture) recordBuild(t *testing.T, request hostbuild.Request, state string, key computerevent.SigningKey) hostbuild.Job {
	t.Helper()
	id, err := request.JobID()
	if err != nil {
		t.Fatal(err)
	}
	job := hostbuild.Job{
		JobID: id, ComputerID: request.ComputerID, OperationID: request.OperationID, BundleDigest: request.BundleDigest,
		BaseCommit: request.BaseCommit, SourcePatchSHA256: request.SourcePatchSHA256, BaseImageManifestDigest: request.BaseImageManifestDigest,
		State: state, CreatedAt: time.Now().UTC(),
	}
	switch state {
	case hostbuild.StateFailed:
		job.Reason, job.Detail = hostbuild.ReasonBuildFailed, "builder: apply source patch: patch does not apply"
	case hostbuild.StateDone:
		result := hostbuild.Result{
			JobID: id, ComputerID: request.ComputerID, OperationID: request.OperationID, BundleDigest: request.BundleDigest,
			BaseCommit: request.BaseCommit, SourcePatchSHA256: request.SourcePatchSHA256, BaseImageManifestDigest: request.BaseImageManifestDigest,
			CodeCommit: strings.Repeat("e", 40), RuntimePath: "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-autoputer-0.1.0",
			LayeringEntrypoint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-autoputer-0.1.0/bin/autoputer",
			ClosureDigest:      digestOf('1'), BuilderReceiptDigest: digestOf('2'), BuiltAt: "2026-10-15T00:00:00Z",
		}
		digest, err := result.Digest()
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := selfdevprotocol.NewAuthorityReceipt(selfdevprotocol.ReceiptKindHostBuild, request.ComputerID, id, digest, "corpusd", key, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		job.Result, job.Receipt = &result, &receipt
	}
	f.builds.jobs[id] = job
	return job
}

// armStanding arms the standing mode as the owner, through the proxy's act.
func (f *consensusFixture) armStanding(t *testing.T, expires time.Time) SelfDevelopmentMode {
	t.Helper()
	current := f.current(t)
	arm := standingRequest(current.Generation, "standing-"+expires.Format("150405.000000"), expires)
	code, mode, body := f.ownerWith(t, arm, mintStandingAct(t, f.signer, f.computerID, arm, ComputerTierTest, time.Now()))
	if code != http.StatusOK {
		t.Fatalf("arm standing: %d %s", code, body)
	}
	f.armedMode = mode
	return mode
}

func (f *consensusFixture) subject(operationID string) decisionpolicy.EffectSubject {
	return decisionpolicy.EffectSubject{
		ComputerID: f.computerID, OperationID: operationID, BundleDigest: computerevent.DigestBytes(f.bundle),
		DesiredEventHead: f.head.DesiredEventHead, EffectiveEventHead: f.head.EffectiveEventHead, PendingTransitionRef: f.head.PendingTransitionRef,
		DesiredStateCommitment: f.head.DesiredStateCommitment, EffectiveStateCommitment: f.head.EffectiveStateCommitment,
		EffectClass: decisionpolicy.EffectClassReversible,
	}
}

func (f *consensusFixture) reviewer(t *testing.T, key computerevent.SigningKey, seat, issuer string, subject decisionpolicy.EffectSubject) computerevent.Receipt {
	t.Helper()
	digest, _ := subject.Digest()
	ballot, err := decisionpolicy.NewBallot(seat, decisionpolicy.VoteAccept, decisionpolicy.PolicyDigestReversibleSelfDevV1, digest,
		subject.ComputerID, subject.OperationID, decisionpolicy.DeskManagement, "", []string{"verifier:test"}, time.Now(), 0)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := decisionpolicy.SignBallot(ballot, key, issuer, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func (f *consensusFixture) request(t *testing.T, operationID string) ConsensusRequest {
	t.Helper()
	subject := f.subject(operationID)
	jobID, err := f.hostRequest(operationID).JobID()
	if err != nil {
		t.Fatal(err)
	}
	return ConsensusRequest{
		OperationID: operationID, Subject: subject,
		ReviewerBallot:          f.reviewer(t, f.guestKey, decisionpolicy.SeatIndependentReviewer, decisionpolicy.IssuerGuestManagement, subject),
		ReviewerPublicKey:       base64.RawStdEncoding.EncodeToString(f.guestKey.PrivateKey.Public().(ed25519.PublicKey)),
		Bundle:                  f.bundle,
		BaseImageManifestDigest: f.bootedBase, HostBuildJobID: jobID,
	}
}

func (f *consensusFixture) decide(t *testing.T, request ConsensusRequest) (ConsensusResponse, error) {
	t.Helper()
	return f.actuator.Decide(context.Background(), f.computerID, request)
}

func TestConsensusPosterMintsADecisionFromTwoSignedBallots(t *testing.T) {
	f := newConsensusFixture(t, OwnerActEnforce)
	for _, op := range []string{"op-1", "op-4"} {
		f.recordBuild(t, f.hostRequest(op), hostbuild.StateDone, f.modeKey)
	}
	// 11: no standing arm.
	if _, err := f.decide(t, f.request(t, "op-1")); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("no arm: %v", err)
	}
	f.armStanding(t, time.Now().Add(2*time.Hour))
	response, err := f.decide(t, f.request(t, "op-1"))
	if err != nil {
		t.Fatalf("decision: %v", err)
	}
	if response.Decision == nil || response.DecisionDigest == "" || len(response.BallotDigests) != 2 {
		t.Fatalf("response = %+v", response)
	}
	// The decision is platform-control signed and names the arm, the
	// policy, the reducer's receipt, both signed ballots and the host build
	// it voted on (30).
	if err := response.Decision.Verify(platformTestKeyResolver{key: f.modeKey.PrivateKey.Public().(ed25519.PublicKey)}); err != nil {
		t.Fatalf("decision signature: %v", err)
	}
	raw, _ := response.Decision.CanonicalBytes()
	if computerevent.DigestBytes(raw) != response.DecisionDigest {
		t.Fatal("decision digest is not the receipt's canonical digest")
	}
	fields := response.Decision.KindFields
	armRaw, _ := f.armedMode.Receipt.CanonicalBytes()
	if fields["standing_mode_receipt_digest"] != computerevent.DigestBytes(armRaw) || fields["policy_digest"] != decisionpolicy.PolicyDigestReversibleSelfDevV1 ||
		fields["computer_tier"] != ComputerTierTest || fields["operation_id"] != "op-1" {
		t.Fatalf("decision fields: %#v", fields)
	}
	jobID, _ := f.hostRequest("op-1").JobID()
	done := f.builds.jobs[jobID]
	resultDigest, _ := done.Result.Digest()
	built, _ := fields["host_build"].(map[string]any)
	if built["job_id"] != jobID || built["result_digest"] != resultDigest || built["closure_digest"] != done.Result.ClosureDigest {
		t.Fatalf("decision host_build: %#v", built)
	}
	ballots, _ := fields["ballots"].([]any)
	if len(ballots) != 2 {
		t.Fatalf("decision carries %d ballots", len(ballots))
	}
	// The capsule-verifier ballot is corpusd's own, platform-control signed,
	// and its evidence names the build.
	hostBallot, _ := ballots[0].(map[string]any)
	hostFields := hostBallot // kind fields are flattened into the receipt
	if hostBallot["issuer"] != decisionpolicy.IssuerHostBuild || hostFields["seat"] != decisionpolicy.SeatCapsuleVerifier ||
		!strings.Contains(fmt.Sprint(hostFields["evidence_refs"]), "host-build-job:"+jobID) {
		t.Fatalf("host ballot: %#v", hostBallot)
	}
	// A retried request gets the same decision.
	again, err := f.decide(t, f.request(t, "op-1"))
	if err != nil || again.DecisionDigest != response.DecisionDigest {
		t.Fatalf("retry: %v %s", err, again.DecisionDigest)
	}

	// 1: the guest offers its own key as the capsule-verifier seat: the
	// poster casts that seat itself; a guest-signed ballot never stands in.
	f.recordBuild(t, f.hostRequest("op-2"), hostbuild.StateDone, f.modeKey)
	forged := f.request(t, "op-2")
	forged.ReviewerBallot = f.reviewer(t, f.guestKey, decisionpolicy.SeatCapsuleVerifier, decisionpolicy.IssuerHostBuild, forged.Subject)
	if _, err := f.decide(t, forged); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("guest-signed capsule-verifier ballot: %v", err)
	}
	// 3: another guest-core key after the first use pinned one.
	_, otherPrivate, _ := ed25519.GenerateKey(rand.Reader)
	other := computerevent.SigningKey{SignerRef: f.guestKey.SignerRef, PrivateKey: otherPrivate}
	swapped := f.request(t, "op-2")
	swapped.ReviewerBallot = f.reviewer(t, other, decisionpolicy.SeatIndependentReviewer, decisionpolicy.IssuerGuestManagement, swapped.Subject)
	swapped.ReviewerPublicKey = base64.RawStdEncoding.EncodeToString(otherPrivate.Public().(ed25519.PublicKey))
	if _, err := f.decide(t, swapped); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "pinned") {
		t.Fatalf("swapped guest-core key: %v", err)
	}
	// 4: a reviewer ballot for another operation; heads that are not
	// corpusd's; bundle bytes that do not hash to the subject.
	mismatched := f.request(t, "op-2")
	mismatched.ReviewerBallot = f.request(t, "op-3").ReviewerBallot
	if _, err := f.decide(t, mismatched); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("reviewer ballot for another operation: %v", err)
	}
	stale := f.request(t, "op-2")
	stale.Subject.DesiredEventHead = digestOf('9')
	if _, err := f.decide(t, stale); !errors.Is(err, ErrConsensusStale) {
		t.Fatalf("stale heads: %v", err)
	}
	wrongBytes := f.request(t, "op-2")
	wrongBytes.Bundle = append([]byte(" "), f.bundle...)
	if _, err := f.decide(t, wrongBytes); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("bundle bytes: %v", err)
	}

	// 24: no host build for this operation; a job id that is not the one
	// corpusd derives; another booted base (another job identity).
	if _, err := f.decide(t, f.request(t, "op-5")); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "no host build") {
		t.Fatalf("no host build: %v", err)
	}
	wrongJob := f.request(t, "op-2")
	wrongJob.HostBuildJobID, _ = f.hostRequest("op-1").JobID()
	if _, err := f.decide(t, wrongJob); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("another operation's job id: %v", err)
	}
	otherBase := f.request(t, "op-2")
	otherBase.BaseImageManifestDigest = digestOf('8')
	otherBase.HostBuildJobID = ""
	if _, err := f.decide(t, otherBase); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("another booted base: %v", err)
	}
	// 25: a job of the same identity on another computer is invisible.
	otherComputer := f.hostRequest("op-6")
	otherComputer.ComputerID = "computer-b"
	f.recordBuild(t, otherComputer, hostbuild.StateDone, f.modeKey)
	if _, err := f.decide(t, f.request(t, "op-6")); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("another computer's build: %v", err)
	}
	// 26: a job built from a patch the bundle does not name.
	otherPatch := f.hostRequest("op-7")
	otherPatch.SourcePatchSHA256 = digestOf('6')
	f.recordBuild(t, otherPatch, hostbuild.StateDone, f.modeKey)
	stray := f.request(t, "op-7")
	stray.HostBuildJobID, _ = otherPatch.JobID()
	if _, err := f.decide(t, stray); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("a build of another patch: %v", err)
	}
	// 10: a bundle that names no source patch.
	patchless := f.request(t, "op-2")
	patchless.Bundle, _ = computerevent.CanonicalJSON(map[string]any{"computer_id": f.computerID, "content_digest": digestOf('c')})
	patchless.Subject.BundleDigest = computerevent.DigestBytes(patchless.Bundle)
	patchless.ReviewerBallot = f.reviewer(t, f.guestKey, decisionpolicy.SeatIndependentReviewer, decisionpolicy.IssuerGuestManagement, patchless.Subject)
	if _, err := f.decide(t, patchless); !errors.Is(err, ErrConsensusRefused) || !strings.Contains(err.Error(), "source patch") {
		t.Fatalf("patchless bundle: %v", err)
	}
	// 29: a done job whose result is not platform-control signed.
	_, rogueKey, _ := ed25519.GenerateKey(rand.Reader)
	f.recordBuild(t, f.hostRequest("op-8"), hostbuild.StateDone, computerevent.SigningKey{SignerRef: f.modeKey.SignerRef, PrivateKey: rogueKey})
	if _, err := f.decide(t, f.request(t, "op-8")); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("forged host build receipt: %v", err)
	}
	// 27: a queued build: pending, no ballot, no decision.
	f.recordBuild(t, f.hostRequest("op-9"), hostbuild.StateQueued, f.modeKey)
	pending, err := f.decide(t, f.request(t, "op-9"))
	if !errors.Is(err, ErrConsensusPending) || !pending.Pending {
		t.Fatalf("queued build: %v %+v", err, pending)
	}
	// 7, 28: a failed build: the host's signed reject, dissent, no decision.
	f.recordBuild(t, f.hostRequest("op-2"), hostbuild.StateFailed, f.modeKey)
	dissent, err := f.decide(t, f.request(t, "op-2"))
	if !errors.Is(err, ErrConsensusDissent) || !dissent.Dissent || !strings.Contains(dissent.Reason, hostbuild.ReasonBuildFailed) || len(dissent.BallotDigests) != 2 {
		t.Fatalf("host reject: %v %+v", err, dissent)
	}
	var count int
	if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM consensus_decisions WHERE operation_id IN ('op-2','op-5','op-6','op-7','op-8','op-9')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("a refusal or dissent minted a decision: %d %v", count, err)
	}
	// No host build service configured: unavailable, never a decision.
	f.actuator.hostBuilds = nil
	if _, err := f.decide(t, f.request(t, "op-4")); !errors.Is(err, ErrConsensusUnavailable) {
		t.Fatalf("no host build service: %v", err)
	}
	f.actuator.hostBuilds = f.builds

	// 11: the window closes; the poster refuses.
	f.actuator.SetClock(func() time.Time { return time.Now().Add(3 * time.Hour) })
	f.modes.now = func() time.Time { return time.Now().Add(3 * time.Hour) }
	if _, err := f.decide(t, f.request(t, "op-4")); !errors.Is(err, ErrConsensusRefused) {
		t.Fatalf("expired window: %v", err)
	}
}

// acceptedFor builds the guest's effect_accepted for a decision.
func (f *consensusFixture) acceptedFor(t *testing.T, operationID, decisionDigest string, inputs []string) computerevent.CASRequest {
	t.Helper()
	return accepted(t, f.head, decisionpolicy.DecisionAuthorityRef(decisionDigest), operationID, computerevent.DigestBytes(f.bundle), inputs, digestOf('d'))
}

func (f *consensusFixture) armRef(t *testing.T) string {
	t.Helper()
	raw, err := f.armedMode.Receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	return "artifact:sha256:" + computerevent.DigestBytes(raw)
}

func TestConsensusGateAdmitsOnlyADecisionThePosterMinted(t *testing.T) {
	// Observe: the S1 branches only log; the consensus branch enforces.
	f := newConsensusFixture(t, OwnerActObserve)
	f.recordBuild(t, f.hostRequest("op-1"), hostbuild.StateDone, f.modeKey)
	f.armStanding(t, time.Now().Add(2*time.Hour))
	response, err := f.decide(t, f.request(t, "op-1"))
	if err != nil {
		t.Fatal(err)
	}
	d := response.DecisionDigest
	good := []string{f.armRef(t), "artifact:sha256:" + d}

	// 8: a digest the poster never minted; a guest-reduced receipt digest;
	// the decision cited for another operation or bundle; the decision not
	// carried as an input; input 0 not the standing arm.
	refused(t, "random digest", f.append(t, f.acceptedFor(t, "op-1", digestOf('e'), []string{f.armRef(t), "artifact:sha256:" + digestOf('e')})))
	qualified, _ := response.Decision.KindFields["qualified_consensus_receipt_digest"].(string)
	refused(t, "guest-reduced receipt digest", f.append(t, f.acceptedFor(t, "op-1", qualified, []string{f.armRef(t), "artifact:sha256:" + qualified})))
	refused(t, "other operation", f.append(t, f.acceptedFor(t, "op-2", d, good)))
	otherBundle := accepted(t, f.head, decisionpolicy.DecisionAuthorityRef(d), "op-1", digestOf('b'), good, digestOf('d'))
	refused(t, "other bundle", f.append(t, otherBundle))
	refused(t, "decision not carried", f.append(t, f.acceptedFor(t, "op-1", d, []string{f.armRef(t)})))
	refused(t, "input 0 not the arm", f.append(t, f.acceptedFor(t, "op-1", d, []string{"artifact:sha256:" + digestOf('f'), "artifact:sha256:" + d})))
	// Other computer: the same decision on computer B.
	otherHead := f.seedHead(t, "computer-b")
	onB := accepted(t, otherHead, decisionpolicy.DecisionAuthorityRef(d), "op-1", computerevent.DigestBytes(f.bundle), good, digestOf('d'))
	refused(t, "decision for computer A on computer B", f.append(t, onB))

	// The poster's decision, cited as the guest's decision path cites it.
	admit := f.acceptedFor(t, "op-1", d, good)
	if err := f.append(t, admit); err != nil {
		t.Fatalf("minted decision refused: %v", err)
	}
	var admitted string
	if err := f.store.db.QueryRow(`SELECT COALESCE(admitted_event_digest, '') FROM consensus_decisions WHERE decision_digest=?`, d).Scan(&admitted); err != nil || admitted != admit.EventDigest {
		t.Fatalf("decision not marked: %q %v", admitted, err)
	}
	// Idempotent retry of the admitted append.
	if err := f.append(t, admit); err != nil {
		t.Fatalf("idempotent retry refused: %v", err)
	}
	// 9: a second event citing the same decision.
	tx, err := f.store.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	second := f.acceptedFor(t, "op-1", d, good)
	if reason, _, err := consensusDecisionRefusal(context.Background(), tx, second.Event, second.EventDigest, time.Now()); err != nil || !strings.Contains(reason, "already admitted") {
		t.Fatalf("second admission: %q %v", reason, err)
	}
}

func TestConsensusGateStopsWhenTheWindowClosesOrTheOwnerDisarms(t *testing.T) {
	f := newConsensusFixture(t, OwnerActEnforce)
	f.recordBuild(t, f.hostRequest("op-1"), hostbuild.StateDone, f.modeKey)
	f.armStanding(t, time.Now().Add(2*time.Hour))
	response, err := f.decide(t, f.request(t, "op-1"))
	if err != nil {
		t.Fatal(err)
	}
	event := f.acceptedFor(t, "op-1", response.DecisionDigest, []string{f.armRef(t), "artifact:sha256:" + response.DecisionDigest})
	tx, err := f.store.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// 13: past the decision's own not_after (30 minutes) plus grace.
	if reason, _, _ := consensusDecisionRefusal(context.Background(), tx, event.Event, event.EventDigest, time.Now().Add(40*time.Minute)); !strings.Contains(reason, "expired") {
		t.Fatalf("expired decision: %q", reason)
	}
	_ = tx.Rollback()

	// 12: the owner disarms between the decision and the append.
	off := SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeProposeOnly, ExpectedGeneration: f.armedMode.Generation, IdempotencyKey: "disarm"}
	if code, _, body := f.ownerWith(t, off, encodeAct(t, mintModeAct(t, f.signer, f.computerID, "owner-1", off, time.Now()))); code != http.StatusOK {
		t.Fatalf("disarm: %d %s", code, body)
	}
	refused(t, "after the owner disarmed", f.append(t, event))

	// 12: re-armed with a short window; a decision inside it; the window
	// closes before the append.
	g := newConsensusFixture(t, OwnerActEnforce)
	g.recordBuild(t, g.hostRequest("op-1"), hostbuild.StateDone, g.modeKey)
	g.armStanding(t, time.Now().Add(10*time.Minute))
	decided, err := g.decide(t, g.request(t, "op-1"))
	if err != nil {
		t.Fatal(err)
	}
	late := g.acceptedFor(t, "op-1", decided.DecisionDigest, []string{g.armRef(t), "artifact:sha256:" + decided.DecisionDigest})
	g.cas.ownerActs.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	refused(t, "after the window closed", g.append(t, late))
}

func TestConsensusRouteIsTheGuestsAndAnswersTyped(t *testing.T) {
	f := newConsensusFixture(t, OwnerActEnforce)
	f.recordBuild(t, f.hostRequest("op-1"), hostbuild.StateDone, f.modeKey)
	f.recordBuild(t, f.hostRequest("op-2"), hostbuild.StateFailed, f.modeKey)
	f.recordBuild(t, f.hostRequest("op-3"), hostbuild.StateRunning, f.modeKey)
	if err := f.handler.ConfigureConsensusActuator(f.actuator); err != nil {
		t.Fatal(err)
	}
	post := func(request ConsensusRequest, computerID, bearer string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(request)
		r := httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/consensus?computer_id="+computerID, bytes.NewReader(body))
		r.RemoteAddr = "172.16.0.2:40000"
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		f.handler.HandleSelfDevelopmentConsensus(w, r)
		return w
	}
	if w := post(f.request(t, "op-1"), f.computerID, ""); w.Code != http.StatusForbidden {
		t.Fatalf("no capability: %d", w.Code)
	}
	if w := post(f.request(t, "op-1"), "computer-other", f.guestToken); w.Code != http.StatusForbidden {
		t.Fatalf("another computer's route: %d", w.Code)
	}
	if w := post(f.request(t, "op-1"), f.computerID, f.guestToken); w.Code != http.StatusForbidden {
		t.Fatalf("no standing arm: %d %s", w.Code, w.Body.String())
	}
	f.armStanding(t, time.Now().Add(time.Hour))
	w := post(f.request(t, "op-1"), f.computerID, f.guestToken)
	var response ConsensusResponse
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Decision == nil {
		t.Fatalf("decision over the route: %d %s", w.Code, w.Body.String())
	}
	w = post(f.request(t, "op-2"), f.computerID, f.guestToken)
	if w.Code != http.StatusConflict || json.Unmarshal(w.Body.Bytes(), &response) != nil || !response.Dissent {
		t.Fatalf("dissent over the route: %d %s", w.Code, w.Body.String())
	}
	response = ConsensusResponse{}
	w = post(f.request(t, "op-3"), f.computerID, f.guestToken)
	if w.Code != http.StatusConflict || json.Unmarshal(w.Body.Bytes(), &response) != nil || !response.Pending || response.Dissent {
		t.Fatalf("pending over the route: %d %s", w.Code, w.Body.String())
	}
}
