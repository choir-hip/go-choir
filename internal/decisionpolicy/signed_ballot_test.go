package decisionpolicy

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// P4 failure modes 1-6 at the ballot verifier
// (docs/archive/problems/p4-standing-consensus-autonomy-2026-10-15.md).

type pinnedKeys map[string]ed25519.PublicKey

func (p pinnedKeys) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	key, ok := p[domain+"/"+keyID]
	if !ok {
		return nil, fmt.Errorf("unpinned %s/%s", domain, keyID)
	}
	return key, nil
}

func newBallotKey(t *testing.T, domain, id string) computerevent.SigningKey {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: domain, KeyID: id}, PrivateKey: private}
}

func (p pinnedKeys) pin(key computerevent.SigningKey) {
	p[key.SignerDomain+"/"+key.KeyID] = key.PrivateKey.Public().(ed25519.PublicKey)
}

func TestSignedBallotSeatAllowlistAndBinding(t *testing.T) {
	now := time.Now().UTC()
	host := newBallotKey(t, SignerDomainHostBuild, "authority-1")
	guest := newBallotKey(t, SignerDomainGuestCore, "guest-core-1")
	pins := pinnedKeys{}
	pins.pin(host)
	pins.pin(guest)
	subject := strings.Repeat("5", 64)
	want := func(seat string) BallotExpectation {
		return BallotExpectation{Seat: seat, PolicyDigest: PolicyDigestReversibleSelfDevV1, SubjectDigest: subject, ComputerID: "computer-a", OperationID: "op-1", Now: now}
	}
	ballot := func(seat, desk string) Ballot {
		b, err := NewBallot(seat, VoteAccept, PolicyDigestReversibleSelfDevV1, subject, "computer-a", "op-1", desk, "", []string{"bundle:x"}, now, 0)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	sign := func(b Ballot, key computerevent.SigningKey, issuer string) computerevent.Receipt {
		r, err := SignBallot(b, key, issuer, now)
		if err != nil {
			t.Fatal(err)
		}
		return roundTrip(t, r)
	}

	// The two seats, each by its own domain, verify.
	hostBallot := sign(ballot(SeatCapsuleVerifier, ""), host, IssuerHostBuild)
	if _, err := VerifyBallot(hostBallot, pins, want(SeatCapsuleVerifier)); err != nil {
		t.Fatalf("host build capsule-verifier ballot: %v", err)
	}
	reviewer := sign(ballot(SeatIndependentReviewer, DeskManagement), guest, IssuerGuestManagement)
	verified, err := VerifyBallot(reviewer, pins, want(SeatIndependentReviewer))
	if err != nil {
		t.Fatalf("guest-core reviewer ballot: %v", err)
	}
	if verified.Signer.SignerDomain != SignerDomainGuestCore || verified.Digest == "" {
		t.Fatalf("verified = %+v", verified)
	}

	// 1: the guest signs the capsule-verifier seat itself.
	forgedSeat := sign(ballot(SeatCapsuleVerifier, ""), guest, IssuerHostBuild)
	if _, err := VerifyBallot(forgedSeat, pins, want(SeatCapsuleVerifier)); !errors.Is(err, ErrBallotSigner) {
		t.Fatalf("guest-core capsule-verifier ballot: %v", err)
	}
	// 1: a platform-control-domain ballot under the guest's issuer.
	if _, err := VerifyBallot(sign(ballot(SeatCapsuleVerifier, ""), host, IssuerGuestManagement), pins, want(SeatCapsuleVerifier)); !errors.Is(err, ErrBallotSigner) {
		t.Fatalf("wrong issuer: %v", err)
	}
	// 2: a platform-control key nobody pinned, and a forged signature under the
	// pinned key id.
	stranger := newBallotKey(t, SignerDomainHostBuild, "authority-1")
	if _, err := VerifyBallot(sign(ballot(SeatCapsuleVerifier, ""), stranger, IssuerHostBuild), pins, want(SeatCapsuleVerifier)); !errors.Is(err, ErrBallotSigner) {
		t.Fatalf("forged host signature: %v", err)
	}
	unpinned := newBallotKey(t, SignerDomainHostBuild, "authority-2")
	if _, err := VerifyBallot(sign(ballot(SeatCapsuleVerifier, ""), unpinned, IssuerHostBuild), pins, want(SeatCapsuleVerifier)); !errors.Is(err, ErrBallotSigner) {
		t.Fatalf("unpinned host key: %v", err)
	}
	unsigned := hostBallot
	unsigned.RequiredSigners, unsigned.SignatureSet = nil, nil
	if _, err := VerifyBallot(unsigned, pins, want(SeatCapsuleVerifier)); !errors.Is(err, ErrBallotSigner) {
		t.Fatalf("unsigned: %v", err)
	}
	// 3: a reviewer ballot from a guest-core key that is not the pinned one.
	otherGuest := newBallotKey(t, SignerDomainGuestCore, "guest-core-1")
	if _, err := VerifyBallot(sign(ballot(SeatIndependentReviewer, DeskManagement), otherGuest, IssuerGuestManagement), pins, want(SeatIndependentReviewer)); !errors.Is(err, ErrBallotSigner) {
		t.Fatalf("unpinned guest key: %v", err)
	}
	// 4: another subject, policy, computer, operation; the reviewer must be
	// management.
	for name, mutate := range map[string]func(*BallotExpectation){
		"subject":   func(w *BallotExpectation) { w.SubjectDigest = strings.Repeat("6", 64) },
		"policy":    func(w *BallotExpectation) { w.PolicyDigest = PolicyDigestHumanRequiredV1 },
		"computer":  func(w *BallotExpectation) { w.ComputerID = "computer-b" },
		"operation": func(w *BallotExpectation) { w.OperationID = "op-2" },
	} {
		w := want(SeatCapsuleVerifier)
		mutate(&w)
		if _, err := VerifyBallot(hostBallot, pins, w); !errors.Is(err, ErrBallotBinding) {
			t.Fatalf("%s mismatch: %v", name, err)
		}
	}
	if _, err := VerifyBallot(sign(ballot(SeatIndependentReviewer, "engineering"), guest, IssuerGuestManagement), pins, want(SeatIndependentReviewer)); !errors.Is(err, ErrBallotBinding) {
		t.Fatalf("engineering as reviewer: %v", err)
	}
	// A ballot for one seat offered for the other.
	if _, err := VerifyBallot(reviewer, pins, want(SeatCapsuleVerifier)); err == nil {
		t.Fatal("reviewer ballot accepted for the capsule-verifier seat")
	}
	// 5: past not_after; another audience.
	late := want(SeatCapsuleVerifier)
	late.Now = now.Add(MaxBallotTTL + time.Minute)
	if _, err := VerifyBallot(hostBallot, pins, late); !errors.Is(err, ErrBallotBinding) {
		t.Fatalf("expired: %v", err)
	}
	away := ballot(SeatCapsuleVerifier, "")
	away.Audience = "maild"
	if _, err := VerifyBallot(sign(away, host, IssuerHostBuild), pins, want(SeatCapsuleVerifier)); !errors.Is(err, ErrBallotBinding) {
		t.Fatalf("other audience: %v", err)
	}
	// The author seat is never signable.
	if _, err := VerifyBallot(hostBallot, pins, want(SeatAuthor)); !errors.Is(err, ErrBallotSigner) {
		t.Fatalf("author seat: %v", err)
	}
}

// 6 and 7 at the reducer: two verified signed ballots reach quorum; a
// signed reject is dissent.
func TestSignedBallotsReduceUnderReversibleSelfDev(t *testing.T) {
	now := time.Now().UTC()
	store := MustReversibleSelfDevV1Store()
	policy, _, err := store.Get(PolicyDigestReversibleSelfDevV1)
	if err != nil {
		t.Fatal(err)
	}
	host := newBallotKey(t, SignerDomainHostBuild, "authority-1")
	guest := newBallotKey(t, SignerDomainGuestCore, "guest-core-1")
	pins := pinnedKeys{}
	pins.pin(host)
	pins.pin(guest)
	subject := EffectSubject{ComputerID: "computer-a", OperationID: "op-1", BundleDigest: strings.Repeat("b", 64),
		DesiredEventHead: strings.Repeat("1", 64), EffectiveEventHead: strings.Repeat("1", 64),
		DesiredStateCommitment: strings.Repeat("2", 64), EffectiveStateCommitment: strings.Repeat("2", 64), EffectClass: EffectClassReversible}
	subjectDigest, _ := subject.Digest()
	manifest := SeatManifestForPolicy(policy, nil)
	manifestDigest, _ := manifest.Digest()
	selection := PolicySelectionReceipt{ReceiptKind: ReceiptKindPolicySelection, PolicyDigest: PolicyDigestReversibleSelfDevV1,
		SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest, SelectedAtHead: strings.Repeat("7", 64), SelectedSequence: 1}
	selectionDigest, _ := selection.Digest()
	selection.SelectionDigest = selectionDigest
	attest := func(seat, vote string, key computerevent.SigningKey, issuer, desk string) BallotAttestation {
		b, err := NewBallot(seat, vote, PolicyDigestReversibleSelfDevV1, subjectDigest, "computer-a", "op-1", desk, "", nil, now, 0)
		if err != nil {
			t.Fatal(err)
		}
		r, err := SignBallot(b, key, issuer, now)
		if err != nil {
			t.Fatal(err)
		}
		v, err := VerifyBallot(r, pins, BallotExpectation{Seat: seat, PolicyDigest: PolicyDigestReversibleSelfDevV1, SubjectDigest: subjectDigest, ComputerID: "computer-a", OperationID: "op-1", Now: now})
		if err != nil {
			t.Fatal(err)
		}
		a, err := AttestationFromSignedBallot(v, manifestDigest, selectionDigest)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	bundle := ConsensusBundle{Manifest: manifest, Subject: subject, Selection: selection, Now: now.Format(time.RFC3339Nano),
		Ballots: []BallotAttestation{
			attest(SeatCapsuleVerifier, VoteAccept, host, IssuerHostBuild, ""),
			attest(SeatIndependentReviewer, VoteAccept, guest, IssuerGuestManagement, DeskManagement),
		}}
	receipt, err := store.ReduceBundle(bundle)
	if err != nil {
		t.Fatalf("two signed accepts: %v", err)
	}
	if !receipt.QuorumEvaluation.Met || receipt.HumanSeatState != HumanSeatAbsent || receipt.QuorumEvaluation.GlobalAccepts != 2 {
		t.Fatalf("receipt = %+v", receipt)
	}
	// One seat alone (what a guest can sign) is below quorum.
	alone := bundle
	alone.Ballots = bundle.Ballots[1:]
	if _, err := store.ReduceBundle(alone); err == nil {
		t.Fatal("the guest's reviewer ballot alone reached quorum")
	}
	// 7: the host builder's signed reject is dissent.
	dissent := bundle
	dissent.Ballots = []BallotAttestation{attest(SeatCapsuleVerifier, VoteReject, host, IssuerHostBuild, ""), bundle.Ballots[1]}
	if _, err := store.ReduceBundle(dissent); !errors.Is(err, ErrUnresolvedDissent) {
		t.Fatalf("host reject: %v", err)
	}
	// 6: the same seat twice still lacks the other required seat.
	twice := bundle
	twice.Ballots = []BallotAttestation{bundle.Ballots[1], attest(SeatIndependentReviewer, VoteAccept, guest, IssuerGuestManagement, DeskManagement)}
	if _, err := store.ReduceBundle(twice); err == nil {
		t.Fatal("two reviewer ballots reached quorum without the host seat")
	}
}

func roundTrip(t *testing.T, r computerevent.Receipt) computerevent.Receipt {
	t.Helper()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var out computerevent.Receipt
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
