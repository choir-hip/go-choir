package trustedoutbox

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

func testDigest(seed string) string { return computerevent.DigestBytes([]byte(seed)) }

func mustSign(t *testing.T, ballot decisionpolicy.BallotAttestation) decisionpolicy.BallotAttestation {
	t.Helper()
	if err := ballot.Sign(); err != nil {
		t.Fatal(err)
	}
	return ballot
}

func emailConsensus(t *testing.T, policyDigest string, withHuman bool) (decisionpolicy.ConsensusInput, decisionpolicy.QualifiedConsensusReceipt) {
	t.Helper()
	store := decisionpolicy.MustEffectsPolicyStore()
	policy, _, err := store.Get(policyDigest)
	if err != nil {
		t.Fatal(err)
	}
	subject := decisionpolicy.EffectSubject{
		ComputerID: "computer-test", OperationID: "operation-test", BundleDigest: testDigest("bundle"),
		DesiredEventHead: testDigest("desired"), EffectiveEventHead: testDigest("effective"),
		DesiredStateCommitment: testDigest("desired-state"), EffectiveStateCommitment: testDigest("effective-state"),
		EffectClass: decisionpolicy.EffectClassIrreversible, Recipient: "owner@example.com",
		PayloadDigest: testDigest("payload"), Actuator: decisionpolicy.ActuatorTrustedOutbox,
		AcceptanceInbox: "accept@example.com", ExternalSends: 1,
	}
	manifest := decisionpolicy.SeatManifest{Seats: []decisionpolicy.Seat{
		{SeatID: "cosuper-author", IndependenceDomain: "authoring", Kind: "agent_profile", EligibilityProof: "assigned-cosuper"},
		{SeatID: "capsule-verifier", IndependenceDomain: "verification", Kind: "independent_verifier", EligibilityProof: "capsule-exec-receipts"},
		{SeatID: "independent-reviewer", IndependenceDomain: "verification", Kind: "agent_profile", EligibilityProof: "not-authoring-cosuper"},
		{SeatID: "external-effects-reviewer", IndependenceDomain: "external_effects", Kind: "independent_verifier", EligibilityProof: "not-authoring-not-verification-signer"},
	}}
	if withHuman {
		manifest.Seats = append(manifest.Seats, decisionpolicy.Seat{
			SeatID: "owner-human", IndependenceDomain: "owner_human", Kind: "owner_human", EligibilityProof: "owner-present",
		})
	}
	subjectDigest, err := subject.Digest()
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest, err := manifest.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection := decisionpolicy.PolicySelectionReceipt{
		ReceiptKind: decisionpolicy.ReceiptKindPolicySelection, PolicyDigest: policyDigest,
		SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
		SelectedAtHead: testDigest("head"), SelectedSequence: 4,
	}
	selectionDigest, err := selection.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection.SelectionDigest = selectionDigest
	sign := func(id, seat, domain, signer string) decisionpolicy.BallotAttestation {
		return mustSign(t, decisionpolicy.BallotAttestation{
			BallotID: id, SeatID: seat, EligibilityProofDigest: testDigest(seat + "-elig"),
			IndependenceDomain: domain, PolicyDigest: policyDigest,
			SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
			PolicySelectionDigest: selectionDigest, Vote: decisionpolicy.VoteAccept,
			WindowID: selectionDigest, SignerProvenance: signer,
		})
	}
	ballots := []decisionpolicy.BallotAttestation{
		sign("b-author", "cosuper-author", "authoring", "signer-author"),
		sign("b-verifier", "capsule-verifier", "verification", "signer-verifier"),
		sign("b-reviewer", "independent-reviewer", "verification", "signer-reviewer"),
		sign("b-external", "external-effects-reviewer", "external_effects", "signer-external"),
	}
	if withHuman {
		ballots = append(ballots, sign("b-human", "owner-human", "owner_human", "signer-owner"))
	}
	input := decisionpolicy.ConsensusInput{
		Policy: policy, Manifest: manifest, Subject: subject, Selection: selection, Ballots: ballots,
		Now: time.Date(2026, 8, 16, 1, 30, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}
	receipt, err := decisionpolicy.Reduce(store, input)
	if err != nil {
		t.Fatal(err)
	}
	return input, receipt
}

func TestDispatchRecordsIntentBeforeProviderAndIdempotentReplay(t *testing.T) {
	store := decisionpolicy.MustEffectsPolicyStore()
	provider := &RecordingProvider{}
	box := New(store, provider)
	input, receipt := emailConsensus(t, decisionpolicy.PolicyDigestIrreversibleEmailV1, false)
	payload := []byte("payload")
	if computerevent.DigestBytes(payload) != input.Subject.PayloadDigest {
		payload = []byte("payload")
		input.Subject.PayloadDigest = computerevent.DigestBytes(payload)
	}
	// payload digest in subject is testDigest("payload") == DigestBytes([]byte("payload"))
	first, err := box.Dispatch(DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
		Now: input.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Greened || first.ProviderOutcome != ProviderAccepted || first.IntentDigest == "" {
		t.Fatalf("consequence = %+v", first)
	}
	if len(provider.Sends) != 1 {
		t.Fatalf("provider sends = %d", len(provider.Sends))
	}
	retry, err := box.Dispatch(DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
		Now: input.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if retry.Digest != first.Digest || len(provider.Sends) != 1 {
		t.Fatalf("retry was not idempotent: %+v sends=%d", retry, len(provider.Sends))
	}
}

func TestDispatchRefusesModeOffAndUnarmedLiveProvider(t *testing.T) {
	store := decisionpolicy.MustEffectsPolicyStore()
	input, receipt := emailConsensus(t, decisionpolicy.PolicyDigestIrreversibleEmailV1, false)
	box := New(store, &RecordingProvider{})
	_, err := box.Dispatch(DispatchRequest{
		Mode: platform.SelfDevelopmentModeOff, Input: input, Receipt: receipt, Payload: []byte("payload"),
	})
	if !errors.Is(err, ErrModeOff) {
		t.Fatalf("mode off error = %v", err)
	}
	live := New(store, liveProvider{})
	_, err = live.Dispatch(DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
	})
	if !errors.Is(err, ErrNotArmed) {
		t.Fatalf("unarmed live error = %v", err)
	}
}

type liveProvider struct{}

func (liveProvider) Send(Intent) (ProviderResult, error) {
	return ProviderResult{Outcome: ProviderAccepted, ProviderDeliveryID: "live"}, nil
}

func TestDispatchRefusesReversibleReceipt(t *testing.T) {
	store := decisionpolicy.MustEffectsPolicyStore()
	policy, _, err := store.Get(decisionpolicy.PolicyDigestReversibleSelfDevV1)
	if err != nil {
		t.Fatal(err)
	}
	subject := decisionpolicy.EffectSubject{
		ComputerID: "computer-test", OperationID: "operation-test", BundleDigest: testDigest("bundle"),
		DesiredEventHead: testDigest("desired"), EffectiveEventHead: testDigest("effective"),
		DesiredStateCommitment: testDigest("desired-state"), EffectiveStateCommitment: testDigest("effective-state"),
		EffectClass: decisionpolicy.EffectClassReversible,
	}
	manifest := decisionpolicy.SeatManifest{Seats: []decisionpolicy.Seat{
		{SeatID: "cosuper-author", IndependenceDomain: "authoring", Kind: "agent_profile", EligibilityProof: "assigned-cosuper"},
		{SeatID: "capsule-verifier", IndependenceDomain: "verification", Kind: "independent_verifier", EligibilityProof: "capsule-exec-receipts"},
		{SeatID: "independent-reviewer", IndependenceDomain: "verification", Kind: "agent_profile", EligibilityProof: "not-authoring-cosuper"},
	}}
	subjectDigest, err := subject.Digest()
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest, err := manifest.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection := decisionpolicy.PolicySelectionReceipt{
		ReceiptKind: decisionpolicy.ReceiptKindPolicySelection, PolicyDigest: decisionpolicy.PolicyDigestReversibleSelfDevV1,
		SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
		SelectedAtHead: testDigest("head"), SelectedSequence: 4,
	}
	sel, err := selection.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection.SelectionDigest = sel
	sign := func(id, seat, domain, signer string) decisionpolicy.BallotAttestation {
		return mustSign(t, decisionpolicy.BallotAttestation{
			BallotID: id, SeatID: seat, EligibilityProofDigest: testDigest(seat + "-elig"),
			IndependenceDomain: domain, PolicyDigest: decisionpolicy.PolicyDigestReversibleSelfDevV1,
			SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
			PolicySelectionDigest: sel, Vote: decisionpolicy.VoteAccept, WindowID: sel, SignerProvenance: signer,
		})
	}
	input := decisionpolicy.ConsensusInput{
		Policy: policy, Manifest: manifest, Subject: subject, Selection: selection,
		Now: time.Date(2026, 8, 16, 1, 30, 0, 0, time.UTC).Format(time.RFC3339Nano),
		Ballots: []decisionpolicy.BallotAttestation{
			sign("b-author", "cosuper-author", "authoring", "signer-author"),
			sign("b-verifier", "capsule-verifier", "verification", "signer-verifier"),
			sign("b-reviewer", "independent-reviewer", "verification", "signer-reviewer"),
		},
	}
	receipt, err := decisionpolicy.Reduce(store, input)
	if err != nil {
		t.Fatal(err)
	}
	box := New(store, &RecordingProvider{})
	_, err = box.Dispatch(DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
	})
	if !errors.Is(err, ErrNotIrreversible) {
		t.Fatalf("error = %v, want %v", err, ErrNotIrreversible)
	}
}

func TestUnknownOutcomeDoesNotGreen(t *testing.T) {
	store := decisionpolicy.MustEffectsPolicyStore()
	provider := &RecordingProvider{Result: ProviderResult{Outcome: ProviderUnknown}}
	box := New(store, provider)
	input, receipt := emailConsensus(t, decisionpolicy.PolicyDigestIrreversibleEmailV1, false)
	cons, err := box.Dispatch(DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
	})
	if !errors.Is(err, ErrUnknownOutcome) || cons.Greened {
		t.Fatalf("unknown outcome greened: cons=%+v err=%v", cons, err)
	}
}

func TestCrashWindowReconcileFindsAcceptedSend(t *testing.T) {
	store := decisionpolicy.MustEffectsPolicyStore()
	provider := &RecordingProvider{}
	box := New(store, provider)
	input, receipt := emailConsensus(t, decisionpolicy.PolicyDigestIrreversibleEmailV1, false)
	cons, err := box.Dispatch(DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
	})
	if err != nil {
		t.Fatal(err)
	}
	box.SimulateCrashAfterAccept(cons.IdempotencyKey, ProviderResult{Outcome: ProviderAccepted, ProviderDeliveryID: cons.ProviderDeliveryID})
	recovered, err := box.Reconcile(cons.IdempotencyKey)
	if err != nil || !recovered.Greened || recovered.ProviderDeliveryID != cons.ProviderDeliveryID {
		t.Fatalf("reconcile = %+v err=%v", recovered, err)
	}
	if !strings.Contains(recovered.CrashWindow, "reconciled") {
		t.Fatalf("crash window = %q", recovered.CrashWindow)
	}
}

func TestRevocationCheckedImmediatelyBeforeDispatch(t *testing.T) {
	store := decisionpolicy.MustEffectsPolicyStore()
	box := New(store, &RecordingProvider{})
	input, receipt := emailConsensus(t, decisionpolicy.PolicyDigestIrreversibleEmailV1, false)
	box.Revoked[receipt.PolicyDigest] = true
	_, err := box.Dispatch(DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
	})
	if !errors.Is(err, ErrRevoked) {
		t.Fatalf("error = %v, want %v", err, ErrRevoked)
	}
}
