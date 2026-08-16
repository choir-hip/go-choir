package decisionpolicy

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func testDigest(seed string) string {
	return computerevent.DigestBytes([]byte(seed))
}

func validSubject() EffectSubject {
	d := testDigest("bundle")
	return EffectSubject{
		ComputerID: "computer-test", OperationID: "operation-test", BundleDigest: d,
		DesiredEventHead: testDigest("desired"), EffectiveEventHead: testDigest("effective"),
		PendingTransitionRef: "", DesiredStateCommitment: testDigest("desired-state"),
		EffectiveStateCommitment: testDigest("effective-state"),
		EffectClass:              EffectClassReversible,
	}
}

func validManifest() SeatManifest {
	return SeatManifest{Seats: []Seat{
		{SeatID: "cosuper-author", IndependenceDomain: "authoring", Kind: "agent_profile", EligibilityProof: "assigned-cosuper"},
		{SeatID: "capsule-verifier", IndependenceDomain: "verification", Kind: "independent_verifier", EligibilityProof: "capsule-exec-receipts"},
		{SeatID: "independent-reviewer", IndependenceDomain: "verification", Kind: "agent_profile", EligibilityProof: "not-authoring-cosuper"},
	}}
}

func mustSign(t *testing.T, ballot BallotAttestation) BallotAttestation {
	t.Helper()
	if err := ballot.Sign(); err != nil {
		t.Fatal(err)
	}
	return ballot
}

func validInput(t *testing.T) (*Store, ConsensusInput) {
	t.Helper()
	store := MustReversibleSelfDevV1Store()
	policy, _, err := store.Get(PolicyDigestReversibleSelfDevV1)
	if err != nil {
		t.Fatal(err)
	}
	subject := validSubject()
	manifest := validManifest()
	subjectDigest, err := subject.Digest()
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest, err := manifest.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection := PolicySelectionReceipt{
		ReceiptKind: ReceiptKindPolicySelection, PolicyDigest: PolicyDigestReversibleSelfDevV1,
		SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
		SelectedAtHead: testDigest("head"), SelectedSequence: 4,
	}
	selectionDigest, err := selection.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection.SelectionDigest = selectionDigest
	ballot := func(id, seat, domain, signer, vote string) BallotAttestation {
		return mustSign(t, BallotAttestation{
			BallotID: id, SeatID: seat, EligibilityProofDigest: testDigest(seat + "-elig"),
			IndependenceDomain: domain, PolicyDigest: PolicyDigestReversibleSelfDevV1,
			SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
			PolicySelectionDigest: selectionDigest, Vote: vote, WindowID: selectionDigest,
			SignerProvenance: signer,
		})
	}
	now := time.Date(2026, 8, 16, 0, 50, 0, 0, time.UTC).Format(time.RFC3339Nano)
	return store, ConsensusInput{
		Policy: policy, Manifest: manifest, Subject: subject, Selection: selection, Now: now,
		Ballots: []BallotAttestation{
			ballot("b-author", "cosuper-author", "authoring", "signer-author", VoteAccept),
			ballot("b-verifier", "capsule-verifier", "verification", "signer-verifier", VoteAccept),
			ballot("b-reviewer", "independent-reviewer", "verification", "signer-reviewer", VoteAccept),
		},
	}
}

func TestReversibleSelfDevV1FileDigest(t *testing.T) {
	store := MustReversibleSelfDevV1Store()
	if !store.Known(PolicyDigestReversibleSelfDevV1) {
		t.Fatal("frozen reversible-selfdev-v1 digest is not registered")
	}
	if got := computerevent.DigestBytes([]byte(strings.TrimSpace(string(ReversibleSelfDevV1Bytes())))); got != PolicyDigestReversibleSelfDevV1 {
		t.Fatalf("embedded policy digest %s, want %s", got, PolicyDigestReversibleSelfDevV1)
	}
}

func TestReduceQualifiedConsensusReceiptForReversibleSelfDevV1(t *testing.T) {
	store, input := validInput(t)
	receipt, err := Reduce(store, input)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.PolicyID != PolicyIDReversibleSelfDevV1 || receipt.PolicyDigest != PolicyDigestReversibleSelfDevV1 {
		t.Fatalf("receipt policy = %s %s", receipt.PolicyID, receipt.PolicyDigest)
	}
	if !receipt.QuorumEvaluation.Met || receipt.QuorumEvaluation.GlobalAccepts != 2 {
		t.Fatalf("quorum = %+v", receipt.QuorumEvaluation)
	}
	if receipt.HumanSeatState != HumanSeatAbsent {
		t.Fatalf("human_seat_state = %s", receipt.HumanSeatState)
	}
	if err := Verify(store, input, receipt); err != nil {
		t.Fatal(err)
	}
}

func TestReduceRefuseMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Store, *ConsensusInput)
		want   error
	}{
		{name: "no policy", mutate: func(s *Store, in *ConsensusInput) {
			in.Selection.PolicyDigest = strings.Repeat("0", 64)
		}, want: ErrNoPolicy},
		{name: "revoked", mutate: func(s *Store, in *ConsensusInput) {
			s.Revoke(PolicyDigestReversibleSelfDevV1)
		}, want: ErrRevokedPolicy},
		{name: "missing selection", mutate: func(_ *Store, in *ConsensusInput) {
			in.Selection.ReceiptKind = ""
		}, want: ErrMissingSelection},
		{name: "ballot not joined", mutate: func(_ *Store, in *ConsensusInput) {
			in.Ballots[1].PolicySelectionDigest = testDigest("other-selection")
			_ = in.Ballots[1].Sign()
		}, want: ErrBallotNotJoined},
		{name: "missing required seat", mutate: func(_ *Store, in *ConsensusInput) {
			in.Ballots = in.Ballots[:2]
		}, want: ErrMissingRequiredSeat},
		{name: "below quorum abstain", mutate: func(_ *Store, in *ConsensusInput) {
			in.Ballots[2].Vote = VoteAbstain
			_ = in.Ballots[2].Sign()
		}, want: ErrBelowQuorum},
		{name: "unresolved dissent", mutate: func(_ *Store, in *ConsensusInput) {
			in.Ballots[1].Vote = VoteReject
			_ = in.Ballots[1].Sign()
		}, want: ErrUnresolvedDissent},
		{name: "seat dropped", mutate: func(_ *Store, in *ConsensusInput) {
			in.Manifest.Seats[1].Recused = true
		}, want: ErrSeatDropped},
		{name: "independence fabricated", mutate: func(_ *Store, in *ConsensusInput) {
			in.Ballots[2].SignerProvenance = in.Ballots[0].SignerProvenance
			_ = in.Ballots[2].Sign()
		}, want: ErrIndependenceFabricated},
		{name: "reversible policy irreversible subject", mutate: func(_ *Store, in *ConsensusInput) {
			in.Subject.Recipient = "owner@example.com"
			in.Subject.PayloadDigest = testDigest("payload")
			in.Subject.Actuator = ActuatorTrustedOutbox
		}, want: ErrReversiblePolicyIrreversibleSubject},
		{name: "owner recovery", mutate: func(_ *Store, in *ConsensusInput) {
			in.Subject.OwnerRecovery = true
		}, want: ErrOwnerRecovery},
		{name: "ballot without attestation", mutate: func(_ *Store, in *ConsensusInput) {
			in.Ballots[1].Attestation = ""
			in.Ballots[1].BallotDigest = ""
		}, want: ErrBallotAttestation},
		{name: "selection sequence zero", mutate: func(_ *Store, in *ConsensusInput) {
			in.Selection.SelectedSequence = 0
		}, want: ErrSelectionAfterOutputs},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, input := validInput(t)
			tc.mutate(store, &input)
			_, err := Reduce(store, input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestReduceHumanRequiredSeatAbsent(t *testing.T) {
	store, input := validInput(t)
	input.Policy.HumanSeat = HumanSeatRequired
	raw, err := computerevent.CanonicalJSON(input.Policy)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := store.Put(raw)
	if err != nil {
		t.Fatal(err)
	}
	input.Selection.PolicyDigest = digest
	selDigest, err := input.Selection.Digest()
	if err != nil {
		t.Fatal(err)
	}
	input.Selection.SelectionDigest = selDigest
	for i := range input.Ballots {
		input.Ballots[i].PolicyDigest = digest
		input.Ballots[i].PolicySelectionDigest = selDigest
		input.Ballots[i].WindowID = selDigest
		if err := input.Ballots[i].Sign(); err != nil {
			t.Fatal(err)
		}
	}
	_, err = Reduce(store, input)
	if !errors.Is(err, ErrHumanSeatAbsent) {
		t.Fatalf("error = %v, want %v", err, ErrHumanSeatAbsent)
	}
}
