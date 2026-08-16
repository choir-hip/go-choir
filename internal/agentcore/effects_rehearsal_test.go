package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/trustedoutbox"
)

func TestEffectsRehearsalReversibleProposeConsensusPromoteRestore(t *testing.T) {
	store, input := decisionpolicyValidInput(t)
	ownerRecovery := input
	ownerRecovery.Subject.OwnerRecovery = true
	if _, err := decisionpolicy.Reduce(store, ownerRecovery); !errors.Is(err, decisionpolicy.ErrOwnerRecovery) {
		t.Fatalf("OwnerRecovery error = %v", err)
	}

	receipt, err := decisionpolicy.Reduce(store, input)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.PolicyID != decisionpolicy.PolicyIDReversibleSelfDevV1 || receipt.HumanSeatState != decisionpolicy.HumanSeatAbsent || !receipt.QuorumEvaluation.Met {
		t.Fatalf("reversible receipt = %+v", receipt)
	}
	if err := decisionpolicy.Verify(store, input, receipt); err != nil {
		t.Fatal(err)
	}

	stored := receipt
	stored.ReceiptDigest = ""
	receiptJSON, err := computerevent.CanonicalJSON(stored)
	if err != nil {
		t.Fatal(err)
	}
	receiptArtifact := computerevent.DigestBytes(receiptJSON)
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-binding",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		EventKind: computerevent.EventEffectAccepted, IdempotencyKey: "rehearsal-reversible", RequestCommitment: strings.Repeat("1", 64),
		TrajectoryID: "trajectory-binding", CapsuleID: "capsule-binding", ParentEventID: "operation-binding",
		ActorProfile: "super", AuthorityRef: decisionpolicy.AuthorityRef(receipt), PrivacyClass: "owner",
		ExpectedDesiredEventHead: strings.Repeat("9", 64), ExpectedEffectiveEventHead: strings.Repeat("a", 64),
		ExpectedDesiredStateCommitment: strings.Repeat("b", 64), ExpectedEffectiveStateCommitment: strings.Repeat("c", 64),
		RequireExpectedHead: true, PayloadCommitment: computerevent.ZeroHead, ProposedEffectRef: strings.Repeat("2", 64),
		DecisionRef: strings.Repeat("3", 64), InputArtifactRefs: []string{"artifact:sha256:" + strings.Repeat("d", 64), "artifact:sha256:" + receiptArtifact},
		VerifierRefs: []string{strings.Repeat("4", 64)}, ReducerVersion: computerevent.ReducerVersionV1,
	}
	eventDigest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	transition := computerevent.DurableEvent{
		Request: computerevent.CASRequest{
			Event: event, EventDigest: eventDigest,
			Next: computerevent.Head{DesiredEventHead: strings.Repeat("5", 64), EffectiveEventHead: strings.Repeat("6", 64)},
		},
		Receipt: computerevent.Receipt{ReceiptKind: "EventHeadReceipt", ReceiptID: "receipt-rehearsal", KindFields: map[string]any{"event_digest": eventDigest}},
	}
	operation := selfdev.Operation{
		OperationID: event.ParentEventID, ComputerID: event.ComputerID, TrajectoryID: event.TrajectoryID,
		CapsuleID: event.CapsuleID, BundleDigest: event.ProposedEffectRef, VerifierRefs: append([]string(nil), event.VerifierRefs...),
		State: selfdev.StateAwaitingApproval,
	}
	got, err := verifyFinalizedSelfDevelopmentDecision(operation, transition)
	if err != nil {
		t.Fatalf("qualified consensus decision refused: %v", err)
	}
	if got.AuthorityKind != "qualified-consensus" || got.ConsensusReceiptDigest != receipt.ReceiptDigest {
		t.Fatalf("verified decision = %+v", got)
	}
	unowned := transition
	unowned.Request.Event.AuthorityRef = "nobody:" + receipt.ReceiptDigest
	if _, err := verifyFinalizedSelfDevelopmentDecision(operation, unowned); err == nil {
		t.Fatal("decision with neither owner nor consensus was accepted")
	}

	now := time.Now().UTC()
	makeInputs := func(seed byte) (computerversion.CodeClosure, computerversion.ArtifactProgram) {
		digest := strings.Repeat(string(seed), 64)
		closure, err := computerversion.NewCodeClosure(strings.Repeat(string(seed), 40), []computerversion.CodeArtifact{{Name: "bundle", SHA256: digest, URI: "artifact+sha256://" + digest + "/bundle"}}, now)
		if err != nil {
			t.Fatal(err)
		}
		program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "bundle", ContentSHA256: digest, ArtifactURI: "artifact+sha256://" + digest + "/bundle"}}, now)
		if err != nil {
			t.Fatal(err)
		}
		return closure, program
	}
	oldCode, oldProgram := makeInputs('1')
	newCode, newProgram := makeInputs('2')
	oldVersion := computerversion.ComputerVersion{CodeRef: oldCode.Ref, ArtifactProgramRef: oldProgram.Ref}
	newVersion := computerversion.ComputerVersion{CodeRef: newCode.Ref, ArtifactProgramRef: newProgram.Ref}
	slotID, err := routeledger.RouteSlotID("owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	ledger := routeledger.NewMemoryLedger()
	bootstrap := func(version computerversion.ComputerVersion, generation uint64, key string) {
		t.Helper()
		approval, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, slotID, version, json.RawMessage(`{"approval":true}`), now)
		if err != nil {
			t.Fatal(err)
		}
		certificate, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, version, json.RawMessage(`{"certificate":true}`), now)
		if err != nil {
			t.Fatal(err)
		}
		kind := routeledger.TransitionBootstrap
		old := computerversion.ComputerVersion{}
		if generation > 0 {
			kind = routeledger.TransitionPromote
			old = oldVersion
		}
		if _, _, err := ledger.TransitionWithEvidence(context.Background(), routeledger.TransitionCommand{
			RouteSlotID: slotID, Kind: kind, Old: old, New: version, ExpectedGeneration: generation,
			ApprovalRef: routeledger.ApprovalRef(approval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(certificate.Ref),
			IdempotencyKey: routeledger.IdempotencyKey(key),
		}, []routeledger.AuthorizationEvidence{approval, certificate}); err != nil {
			t.Fatal(err)
		}
	}
	bootstrap(oldVersion, 0, "idempotency:rehearsal-bootstrap")
	bootstrap(newVersion, 1, "idempotency:rehearsal-promote")
	slot, _, err := ledger.Resolve(context.Background(), slotID)
	if err != nil {
		t.Fatal(err)
	}
	if slot.Generation != 2 || slot.Current != newVersion {
		t.Fatalf("promoted slot = %+v", slot)
	}

	computerID := "computer-rehearsal-restore"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if live != nil {
			_ = live.Close()
		}
	}()
	rt, cas, acceptedHead := rematerializeTapeRuntime(t, computerID, storePath, live)
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	priorDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
	targetDigest, targetIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
	pointCurrent(t, updaterRoot, priorDigest)
	rt.selfdevUpdaterRoot = updaterRoot
	ctx := context.Background()
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity, acceptedHead)
	laterID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	later := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: laterID, ComputerID: computerID,
		EventKind: computerevent.EventArtifactProduced, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "rehearsal-later", ActorProfile: "super", AuthorityRef: "owner", PrivacyClass: "owner",
		PayloadCommitment: strings.Repeat("c", 64), ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := rt.eventAppender.AppendNew(ctx, later, computerevent.TransitionInput{}, nil); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(restoreAPIRequest{
		Checkpoint:    checkpoint,
		OperandScopes: []string{selfdevprotocol.RestoreScopeVMLocal, selfdevprotocol.RestoreScopeFrontend},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/lifecycle/restore", bytes.NewReader(body))
	request.Header.Set("X-Authenticated-User", "owner-restore")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(rt).HandleComputersRouter(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("restore status=%d body=%s", response.Code, response.Body.String())
	}
	live = nil
	foundIntent := false
	for _, record := range cas.events {
		if record.Request.Event.EventKind == computerevent.EventRestoreRequested {
			foundIntent = true
			if record.Request.Event.DecisionRef != acceptedHead || record.Request.Event.ProposedEffectRef != checkpoint.Digest {
				t.Fatalf("restore intent bindings=%+v", record.Request.Event)
			}
		}
	}
	if !foundIntent {
		t.Fatal("restore did not append a restore-intent event")
	}
	if rt.store == nil {
		t.Fatal("restore closed the captured store pointer")
	}
	t.Cleanup(func() { _ = rt.store.Close() })
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("restored head: %v %#v", err, head)
	}
	if head.CanonicalEventHead != acceptedHead || head.Sequence != 1 {
		t.Fatalf("restored projection %+v replayed past the checkpoint head", head)
	}
}

func TestEffectsRehearsalIrreversibleProposeConsensusOutboxNoLiveSend(t *testing.T) {
	if platform.SelfDevelopmentModeOff != "off" {
		t.Fatalf("default mode constant drifted: %q", platform.SelfDevelopmentModeOff)
	}

	reversibleStore, reversibleInput := decisionpolicyValidInput(t)
	reversibleInput.Subject.Recipient = "owner@example.com"
	reversibleInput.Subject.PayloadDigest = computerevent.DigestBytes([]byte("payload"))
	reversibleInput.Subject.Actuator = decisionpolicy.ActuatorTrustedOutbox
	if _, err := decisionpolicy.Reduce(reversibleStore, reversibleInput); !errors.Is(err, decisionpolicy.ErrReversiblePolicyIrreversibleSubject) {
		t.Fatalf("reversible policy accepted irreversible subject: %v", err)
	}

	store := decisionpolicy.MustEffectsPolicyStore()
	input, receipt := rehearsalEmailConsensus(t, store, decisionpolicy.PolicyDigestIrreversibleEmailV1, false)
	if receipt.PolicyID != decisionpolicy.PolicyIDIrreversibleEmailV1 || receipt.HumanSeatState != decisionpolicy.HumanSeatAbsent || receipt.QuorumEvaluation.GlobalAccepts != 3 {
		t.Fatalf("irreversible receipt = %+v quorum=%+v", receipt, receipt.QuorumEvaluation)
	}

	humanStore := decisionpolicy.MustEffectsPolicyStore()
	humanInput, _ := rehearsalEmailConsensus(t, humanStore, decisionpolicy.PolicyDigestHumanRequiredV1, true)
	humanInput.Manifest.Seats = humanInput.Manifest.Seats[:4]
	humanInput.Ballots = humanInput.Ballots[:4]
	manifestDigest, err := humanInput.Manifest.Digest()
	if err != nil {
		t.Fatal(err)
	}
	humanInput.Selection.SeatManifestDigest = manifestDigest
	sel, err := humanInput.Selection.Digest()
	if err != nil {
		t.Fatal(err)
	}
	humanInput.Selection.SelectionDigest = sel
	for i := range humanInput.Ballots {
		humanInput.Ballots[i].SeatManifestDigest = manifestDigest
		humanInput.Ballots[i].PolicySelectionDigest = sel
		humanInput.Ballots[i].WindowID = sel
		if err := humanInput.Ballots[i].Sign(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := decisionpolicy.Reduce(humanStore, humanInput); !errors.Is(err, decisionpolicy.ErrMissingRequiredSeat) && !errors.Is(err, decisionpolicy.ErrHumanSeatAbsent) {
		t.Fatalf("human-required absent seat error = %v", err)
	}

	provider := &trustedoutbox.RecordingProvider{}
	box := trustedoutbox.New(store, provider)
	if box.Armed {
		t.Fatal("outbox default Armed was true")
	}
	first, err := box.Dispatch(trustedoutbox.DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
		Now: input.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Greened || first.ProviderOutcome != trustedoutbox.ProviderAccepted || first.IntentDigest == "" || len(provider.Sends) != 1 {
		t.Fatalf("consequence = %+v sends=%d", first, len(provider.Sends))
	}

	if _, err := box.Dispatch(trustedoutbox.DispatchRequest{
		Mode: platform.SelfDevelopmentModeOff, Input: input, Receipt: receipt, Payload: []byte("payload"),
	}); !errors.Is(err, trustedoutbox.ErrModeOff) {
		t.Fatalf("mode off error = %v", err)
	}

	live := trustedoutbox.New(store, rehearsalLiveProvider{})
	if _, err := live.Dispatch(trustedoutbox.DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: input, Receipt: receipt, Payload: []byte("payload"),
	}); !errors.Is(err, trustedoutbox.ErrNotArmed) {
		t.Fatalf("unarmed live error = %v", err)
	}

	unknownBox := trustedoutbox.New(store, &trustedoutbox.RecordingProvider{Result: trustedoutbox.ProviderResult{Outcome: trustedoutbox.ProviderUnknown}})
	unknownInput, unknownReceipt := rehearsalEmailConsensus(t, decisionpolicy.MustEffectsPolicyStore(), decisionpolicy.PolicyDigestIrreversibleEmailV1, false)
	cons, err := unknownBox.Dispatch(trustedoutbox.DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: unknownInput, Receipt: unknownReceipt, Payload: []byte("payload"),
	})
	if !errors.Is(err, trustedoutbox.ErrUnknownOutcome) || cons.Greened {
		t.Fatalf("unknown outcome greened: cons=%+v err=%v", cons, err)
	}

	crashStore := decisionpolicy.MustEffectsPolicyStore()
	crashProvider := &trustedoutbox.RecordingProvider{}
	crashBox := trustedoutbox.New(crashStore, crashProvider)
	crashInput, crashReceipt := rehearsalEmailConsensus(t, crashStore, decisionpolicy.PolicyDigestIrreversibleEmailV1, false)
	accepted, err := crashBox.Dispatch(trustedoutbox.DispatchRequest{
		Mode: platform.SelfDevelopmentModeQualifiedConsensus, Input: crashInput, Receipt: crashReceipt, Payload: []byte("payload"),
	})
	if err != nil {
		t.Fatal(err)
	}
	crashBox.SimulateCrashAfterAccept(accepted.IdempotencyKey, trustedoutbox.ProviderResult{Outcome: trustedoutbox.ProviderAccepted, ProviderDeliveryID: accepted.ProviderDeliveryID})
	recovered, err := crashBox.Reconcile(accepted.IdempotencyKey)
	if err != nil || !recovered.Greened || recovered.ProviderDeliveryID != accepted.ProviderDeliveryID {
		t.Fatalf("reconcile = %+v err=%v", recovered, err)
	}
	if !strings.Contains(recovered.CrashWindow, "reconciled") {
		t.Fatalf("crash window = %q", recovered.CrashWindow)
	}
}

type rehearsalLiveProvider struct{}

func (rehearsalLiveProvider) Send(trustedoutbox.Intent) (trustedoutbox.ProviderResult, error) {
	return trustedoutbox.ProviderResult{Outcome: trustedoutbox.ProviderAccepted, ProviderDeliveryID: "live"}, nil
}

func rehearsalEmailConsensus(t *testing.T, store *decisionpolicy.Store, policyDigest string, withHuman bool) (decisionpolicy.ConsensusInput, decisionpolicy.QualifiedConsensusReceipt) {
	t.Helper()
	policy, _, err := store.Get(policyDigest)
	if err != nil {
		t.Fatal(err)
	}
	digest := func(seed string) string { return computerevent.DigestBytes([]byte(seed)) }
	subject := decisionpolicy.EffectSubject{
		ComputerID: "computer-test", OperationID: "operation-test", BundleDigest: digest("bundle"),
		DesiredEventHead: digest("desired"), EffectiveEventHead: digest("effective"),
		DesiredStateCommitment: digest("desired-state"), EffectiveStateCommitment: digest("effective-state"),
		EffectClass: decisionpolicy.EffectClassIrreversible, Recipient: "owner@example.com",
		PayloadDigest: digest("payload"), Actuator: decisionpolicy.ActuatorTrustedOutbox,
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
		SelectedAtHead: digest("head"), SelectedSequence: 4,
	}
	selectionDigest, err := selection.Digest()
	if err != nil {
		t.Fatal(err)
	}
	selection.SelectionDigest = selectionDigest
	sign := func(id, seat, domain, signer string) decisionpolicy.BallotAttestation {
		b := decisionpolicy.BallotAttestation{
			BallotID: id, SeatID: seat, EligibilityProofDigest: digest(seat + "-elig"),
			IndependenceDomain: domain, PolicyDigest: policyDigest,
			SeatManifestDigest: manifestDigest, SubjectDigest: subjectDigest,
			PolicySelectionDigest: selectionDigest, Vote: decisionpolicy.VoteAccept,
			WindowID: selectionDigest, SignerProvenance: signer,
		}
		if err := b.Sign(); err != nil {
			t.Fatal(err)
		}
		return b
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
