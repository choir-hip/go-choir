package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func storeSupervisionArtifact(fill byte, bindingID string) computerevent.ReferencedArtifact {
	digest := storeTestDigest(fill)
	return computerevent.ReferencedArtifact{
		Ref: "artifact:sha256:" + digest, ArtifactDigest: digest, PlaintextDigest: digest,
		MediaType: "text/plain", BindingID: bindingID,
		PinReceipt: computerevent.Receipt{ReceiptKind: "PinReceipt"},
	}
}

func storeSupervisionOpenTransaction(t *testing.T, canonicalHead string) computerevent.SupervisionTransaction {
	t.Helper()
	transaction := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1,
		TransactionID: "transaction-open", TransactionClass: "open_trajectory", OwnerID: "owner-supervision", ComputerID: "computer-supervision", TrajectoryID: "trajectory-supervision", CommandID: "command-open", CommandDigest: computerevent.ZeroHead,
		Actor: computerevent.SupervisionActor{ActorID: "texture-1", Role: "texture", AuthorityRef: "authority:texture"}, Expected: computerevent.SupervisionExpected{},
		Mutations: []computerevent.SupervisionMutation{
			{Kind: "trajectory_started", Body: json.RawMessage(`{"trajectory_kind":"document","subject_refs":{"artifact":"texture://documents/document-1"},"intent_revision_id":"intent-1","artifact_id":"document-1","artifact_revision_id":"revision-1","texture_actor_id":"texture-1","initial_assignment_ids":["assignment-1"],"objective":"Build the supervised document."}`)},
			{Kind: "intent_revised", Body: json.RawMessage(`{"intent_revision_id":"intent-1","parent_intent_revision_id":null,"intent":"Build the supervised document.","material":false,"affected_targets":[]}`)},
			{Kind: "texture_revision", Body: json.RawMessage(`{"artifact_id":"document-1","revision_id":"revision-1","title":"Supervised document","parent_revision_id":null,"content":"supervised content","source_graph":{},"metadata":{},"metadata_digest":"` + storeTestDigest('b') + `","narrative_kind":"texture_synthesis","fulfills_intent_revision_id":"intent-1"}`)},
		},
	}
	transaction.CommandDigest, _ = transaction.ComputeCommandDigest()
	return transaction
}

func storeSupervisionRequest(t *testing.T, transaction computerevent.SupervisionTransaction, current *computerevent.Head) computerevent.CASRequest {
	t.Helper()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	transaction.TransactionID = eventID
	canonical, err := transaction.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	sequence, previous := uint64(1), computerevent.ZeroHead
	expectedDesired, expectedEffective := computerevent.ZeroHead, computerevent.ZeroHead
	expectedDesiredState, expectedEffectiveState := computerevent.ZeroHead, computerevent.ZeroHead
	if current != nil {
		sequence = current.Sequence + 1
		previous = current.CanonicalEventHead
		expectedDesired = current.DesiredEventHead
		expectedEffective = current.EffectiveEventHead
		expectedDesiredState = current.DesiredStateCommitment
		expectedEffectiveState = current.EffectiveStateCommitment
	}
	transactionArtifactDigest := computerevent.DigestBytes(canonical)
	event := computerevent.Event{SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: transaction.ComputerID, Sequence: sequence, PreviousHead: previous, EventKind: computerevent.EventSupervisionTransaction, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: transaction.CommandID, ActorProfile: transaction.Actor.Role, AuthorityRef: transaction.Actor.AuthorityRef, InputArtifactRefs: []string{"artifact:sha256:" + transactionArtifactDigest}, PayloadCommitment: transaction.CommandDigest, DecisionRef: transactionArtifactDigest, PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1, TrajectoryID: transaction.TrajectoryID, ExpectedDesiredEventHead: expectedDesired, ExpectedEffectiveEventHead: expectedEffective, ExpectedDesiredStateCommitment: expectedDesiredState, ExpectedEffectiveStateCommitment: expectedEffectiveState}
	input := computerevent.TransitionInput{}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment = commitment
	next, err := computerevent.Reduce(current, event, input)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.CASRequest{Event: event, EventDigest: digest, EventArtifactDigest: digest, EventPinReceiptDigest: storeTestDigest('d'), PinIntentCommitment: pinIntent, Input: input, Next: next, SupervisionTransaction: &transaction}
}

func storeBootstrapSupervisionComputer(t *testing.T, productStore *Store) *computerevent.Head {
	t.Helper()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-supervision",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventGenesisImported,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "genesis-supervision",
		ActorProfile: "trusted-core", AuthorityRef: "authority:test", PayloadCommitment: storeTestDigest('a'),
		PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: storeTestDigest('b'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: storeTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := computerevent.Reduce(nil, event, input)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{Event: event, EventDigest: digest, EventArtifactDigest: digest, EventPinReceiptDigest: storeTestDigest('c'), PinIntentCommitment: pinIntent, Input: input, Next: next}
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	finalizeSupervisionRequest(t, productStore, request)
	head, err := productStore.Head(context.Background(), event.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	return head
}

func finalizeSupervisionRequest(t *testing.T, productStore *Store, request computerevent.CASRequest) {
	t.Helper()
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{"computer_id": request.Event.ComputerID, "event_digest": request.EventDigest, "sequence": request.Event.Sequence}, []computerevent.SigningKey{{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.Finalize(context.Background(), request.Event.ComputerID, request.EventDigest, receipt); err != nil {
		t.Fatal(err)
	}
}

func TestSupervisionProjectionSnapshotSeparatesGlobalAndTrajectoryHeads(t *testing.T) {
	productStore := openTestStore(t)
	head := storeBootstrapSupervisionComputer(t, productStore)
	transactionA := storeSupervisionOpenTransaction(t, head.CanonicalEventHead)
	requestA := storeSupervisionRequest(t, transactionA, head)
	finalizeSupervisionRequest(t, productStore, requestA)
	headA, err := productStore.Head(context.Background(), transactionA.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	transactionB := storeSupervisionOpenTransaction(t, headA.CanonicalEventHead)
	transactionB.TrajectoryID = "trajectory-other"
	transactionB.CommandID = "command-open-other"
	transactionB.CommandDigest, err = transactionB.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	requestB := storeSupervisionRequest(t, transactionB, headA)
	finalizeSupervisionRequest(t, productStore, requestB)
	headB, err := productStore.Head(context.Background(), transactionB.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := productStore.GetSupervisionProjectionSnapshot(context.Background(), transactionA.OwnerID, transactionA.ComputerID, transactionA.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.CanonicalEventHead != headB.CanonicalEventHead {
		t.Fatalf("global snapshot head = %q, want %q", snapshot.CanonicalEventHead, headB.CanonicalEventHead)
	}
	if snapshot.ObservedCanonicalEventHead != headA.CanonicalEventHead {
		t.Fatalf("trajectory-local observed head = %q, want %q", snapshot.ObservedCanonicalEventHead, headA.CanonicalEventHead)
	}
}
func TestSupervisionProjectionFinalizesEventAndDerivedStateAtomically(t *testing.T) {
	productStore := openTestStore(t)
	genesis := storeBootstrapSupervisionComputer(t, productStore)
	transaction := storeSupervisionOpenTransaction(t, genesis.CanonicalEventHead)
	request := storeSupervisionRequest(t, transaction, genesis)
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	prepared, err := productStore.Prepared(context.Background(), transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 1 || prepared[0].SupervisionTransaction == nil || prepared[0].SupervisionTransaction.CommandDigest != transaction.CommandDigest {
		t.Fatalf("prepared supervision transaction not recoverable: %+v", prepared)
	}
	var commandStatus, commandEventDigest string
	if err := productStore.db.QueryRow(`SELECT status, COALESCE(event_digest, '') FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, transaction.ComputerID, transaction.CommandID).Scan(&commandStatus, &commandEventDigest); err != nil || commandStatus != "prepared" || commandEventDigest != request.EventDigest {
		t.Fatalf("prepared command binding = status=%q event=%q err=%v", commandStatus, commandEventDigest, err)
	}
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatalf("prepared retry did not verify matching command binding: %v", err)
	}
	finalizeSupervisionRequest(t, productStore, request)
	stateID, err := lifecycleCanonicalID(ogKindSupervisionState, transaction.OwnerID, transaction.ComputerID, transaction.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := productStore.ogStore.GetObject(context.Background(), stateID)
	if err != nil {
		t.Fatal(err)
	}
	var state supervisionProjectionState
	if err := json.Unmarshal(obj.Body, &state); err != nil {
		t.Fatal(err)
	}
	if state.LifecycleVersion != 1 || state.CanonicalEventHead != request.EventDigest {
		t.Fatalf("unexpected supervision projection: %+v", state)
	}
	document, err := productStore.GetLifecycleDocument(context.Background(), transaction.OwnerID, transaction.ComputerID, "document-1")
	if err != nil {
		t.Fatal(err)
	}
	revision, err := productStore.GetLifecycleRevision(context.Background(), transaction.OwnerID, transaction.ComputerID, "revision-1")
	if err != nil {
		t.Fatal(err)
	}
	if document.CurrentRevisionID != revision.RevisionID || revision.Content != "supervised content" {

		t.Fatalf("unexpected Texture projection: document=%+v revision=%+v", document, revision)
	}
	headAfterOpen, err := productStore.Head(context.Background(), transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	intentID, artifactHead := "intent-1", "revision-1"
	lifecycleVersion := uint64(1)
	optionsArtifact := storeSupervisionArtifact('e', "decision-options")
	proposalArtifact := storeSupervisionArtifact('f', "decision-proposal")
	decision := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1,
		TransactionID: "transaction-decision", TransactionClass: "propose_decision", OwnerID: transaction.OwnerID, ComputerID: transaction.ComputerID, TrajectoryID: transaction.TrajectoryID, CommandID: "command-decision", CommandDigest: computerevent.ZeroHead,
		Actor:               computerevent.SupervisionActor{ActorID: "super-1", Role: "super", AuthorityRef: "authority:super"},
		Expected:            computerevent.SupervisionExpected{CanonicalEventHead: &headAfterOpen.CanonicalEventHead, LifecycleVersion: &lifecycleVersion, IntentRevisionID: &intentID, ArtifactHeadRevisionID: &artifactHead},
		Mutations:           []computerevent.SupervisionMutation{{Kind: "super_decision_proposed", Body: json.RawMessage(`{"decision_id":"decision-1","options_artifact_ref":"` + optionsArtifact.Ref + `","selected_option_id":"assign","proposal_artifact_ref":"` + proposalArtifact.Ref + `","evidence_refs":["` + optionsArtifact.Ref + `"],"dissent_ids":[],"reserved_authority":"none"}`)}},
		ReferencedArtifacts: []computerevent.ReferencedArtifact{optionsArtifact, proposalArtifact},
	}
	decision.CommandDigest, _ = decision.ComputeCommandDigest()
	finalizeSupervisionRequest(t, productStore, storeSupervisionRequest(t, decision, headAfterOpen))
	headAfterDecision, err := productStore.Head(context.Background(), transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleVersion = 2
	assignment := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1,
		TransactionID: "transaction-assignment", TransactionClass: "open_assignment", OwnerID: transaction.OwnerID, ComputerID: transaction.ComputerID, TrajectoryID: transaction.TrajectoryID, CommandID: "command-assignment", CommandDigest: computerevent.ZeroHead,
		Actor:     computerevent.SupervisionActor{ActorID: "super-1", Role: "super", AuthorityRef: "authority:super"},
		Expected:  computerevent.SupervisionExpected{CanonicalEventHead: &headAfterDecision.CanonicalEventHead, LifecycleVersion: &lifecycleVersion, IntentRevisionID: &intentID, ArtifactHeadRevisionID: &artifactHead},
		Mutations: []computerevent.SupervisionMutation{{Kind: "assignment_opened", Body: json.RawMessage(`{"assignment_id":"assignment-1","assigned_actor_id":"cosuper-1","assigned_role":"cosuper","parent_decision_id":"decision-1","intent_revision_id":"intent-1","observed_base":{"canonical_event_head":"` + headAfterDecision.CanonicalEventHead + `","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1"},"scope_digest":"` + storeTestDigest('a') + `","capability_digest":"` + storeTestDigest('b') + `","policy_digest":"` + storeTestDigest('c') + `","obligation_ids":["obligation-1"],"idempotency_commitment":"` + storeTestDigest('a') + `"}`)}},
	}
	assignment.CommandDigest, _ = assignment.ComputeCommandDigest()
	finalizeSupervisionRequest(t, productStore, storeSupervisionRequest(t, assignment, headAfterDecision))
	headAfterAssignment, err := productStore.Head(context.Background(), transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	work, err := productStore.GetLifecycleWorkItem(context.Background(), transaction.OwnerID, transaction.ComputerID, "assignment-1")
	if err != nil || work.TrajectoryID != transaction.TrajectoryID || work.AssignedAgentID != "cosuper-1" {
		t.Fatalf("assignment projection work=%+v err=%v", work, err)
	}
	observedHead := headAfterAssignment.CanonicalEventHead
	lifecycleVersion = 3
	startAttempt := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1,
		TransactionID: "transaction-attempt", TransactionClass: "start_attempt", OwnerID: transaction.OwnerID, ComputerID: transaction.ComputerID, TrajectoryID: transaction.TrajectoryID, CommandID: "command-attempt", CommandDigest: computerevent.ZeroHead,
		Actor:     computerevent.SupervisionActor{ActorID: "super-1", Role: "super", AuthorityRef: "authority:super"},
		Expected:  computerevent.SupervisionExpected{CanonicalEventHead: &observedHead, LifecycleVersion: &lifecycleVersion, IntentRevisionID: &intentID, ArtifactHeadRevisionID: &artifactHead},
		Mutations: []computerevent.SupervisionMutation{{Kind: "attempt_started", Body: json.RawMessage(`{"assignment_id":"assignment-1","attempt_id":"attempt-1","attempt_kind":"initial","ordinal":1,"prior_attempt_id":null,"run_id":"run-1","observed_base":{"canonical_event_head":"` + observedHead + `","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1"},"runtime_receipt_ref":"artifact:sha256:` + storeTestDigest('a') + `"}`)}},
	}
	startAttempt.CommandDigest, _ = startAttempt.ComputeCommandDigest()
	startRequest := storeSupervisionRequest(t, startAttempt, headAfterAssignment)
	finalizeSupervisionRequest(t, productStore, startRequest)

	headAfterAttempt, err := productStore.Head(context.Background(), transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleVersion = 4
	message := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1,
		TransactionID: "transaction-message", TransactionClass: "record_message", OwnerID: transaction.OwnerID, ComputerID: transaction.ComputerID, TrajectoryID: transaction.TrajectoryID, CommandID: "command-message", CommandDigest: computerevent.ZeroHead,
		Actor:     computerevent.SupervisionActor{ActorID: "super-1", Role: "super", AuthorityRef: "authority:super"},
		Expected:  computerevent.SupervisionExpected{CanonicalEventHead: &headAfterAttempt.CanonicalEventHead, LifecycleVersion: &lifecycleVersion, IntentRevisionID: &intentID, ArtifactHeadRevisionID: &artifactHead},
		Mutations: []computerevent.SupervisionMutation{{Kind: "actor_message_recorded", Body: json.RawMessage(`{"message_id":"message-1","from_actor_id":"super-1","to_role":"texture","to_actor_id":null,"channel_id":"supervision","payload_artifact_ref":"artifact:sha256:` + storeTestDigest('a') + `","material":false}`)}},
	}
	message.CommandDigest, _ = message.ComputeCommandDigest()
	messageRequest := storeSupervisionRequest(t, message, headAfterAttempt)
	finalizeSupervisionRequest(t, productStore, messageRequest)

	headAfterMessage, err := productStore.Head(context.Background(), transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	lifecycleVersion = 5
	result := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1,
		TransactionID: "transaction-result", TransactionClass: "return_result", OwnerID: transaction.OwnerID, ComputerID: transaction.ComputerID, TrajectoryID: transaction.TrajectoryID, CommandID: "command-result", CommandDigest: computerevent.ZeroHead,
		Actor:        computerevent.SupervisionActor{ActorID: "cosuper-1", Role: "cosuper", AuthorityRef: "authority:assignment-1"},
		Expected:     computerevent.SupervisionExpected{CanonicalEventHead: &headAfterMessage.CanonicalEventHead, LifecycleVersion: &lifecycleVersion, IntentRevisionID: &intentID, ArtifactHeadRevisionID: &artifactHead},
		ObservedBase: &computerevent.SupervisionObservedBase{CanonicalEventHead: observedHead, IntentRevisionID: intentID, ArtifactHeadRevisionID: artifactHead},
		Mutations:    []computerevent.SupervisionMutation{{Kind: "attempt_result", Body: json.RawMessage(`{"assignment_id":"assignment-1","attempt_id":"attempt-1","result_id":"result-1","outcome":"succeeded","result_artifact_ref":"artifact:sha256:` + storeTestDigest('a') + `","evidence_refs":["artifact:sha256:` + storeTestDigest('a') + `"],"observed_base":{"canonical_event_head":"` + observedHead + `","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1"},"delivered_after_cancellation":false}`)}},
	}
	result.CommandDigest, _ = result.ComputeCommandDigest()
	resultRequest := storeSupervisionRequest(t, result, headAfterMessage)
	finalizeSupervisionRequest(t, productStore, resultRequest)
	stateObject, err := productStore.ogStore.GetObject(context.Background(), stateID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stateObject.Body, &state); err != nil {
		t.Fatal(err)
	}
	if !state.OpenRebaseObligations["rebase:result-1"] || state.Statuses["result"]["result-1"] != "returned" {
		t.Fatalf("stale concurrent result was not retained with a rebase obligation: %+v", state)
	}
}
func TestReserveFrozenSupervisionInputsAtomicallyPersistsExactPlan(t *testing.T) {
	productStore := openTestStore(t)
	transaction := storeSupervisionOpenTransaction(t, computerevent.ZeroHead)
	envelope := []byte("encrypted-private-input")
	artifactDigest := computerevent.DigestBytes(envelope)
	artifactRef, err := computerevent.ArtifactRefFromDigest(artifactDigest)
	if err != nil {
		t.Fatal(err)
	}
	var startBody map[string]any
	if err := json.Unmarshal(transaction.Mutations[0].Body, &startBody); err != nil {
		t.Fatal(err)
	}
	startBody["subject_refs"].(map[string]any)["input_artifact_ref"] = artifactRef.String()
	transaction.Mutations[0].Body, err = json.Marshal(startBody)
	if err != nil {
		t.Fatal(err)
	}
	logicalDigest := storeTestDigest('a')
	plaintextDigest := storeTestDigest('b')
	transaction.ReferencedArtifacts = []computerevent.ReferencedArtifact{{
		Ref: artifactRef.String(), ArtifactDigest: artifactDigest, PlaintextDigest: plaintextDigest,
		LogicalPlaintextDigest: logicalDigest, MediaType: "application/json", BindingID: "command-open:input",
	}}
	transaction.CommandDigest, err = transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	plan := computerevent.FrozenSupervisionPlan{
		Transaction: transaction,
		PrivateInputs: []computerevent.FrozenPrivateSupervisionInput{{
			BindingID: "command-open:input", MediaType: "application/json", Envelope: envelope,
			ArtifactDigest: artifactDigest, PlaintextDigest: plaintextDigest,
		}},
	}
	if _, _, finalized, err := productStore.ReserveFrozenSupervisionInputs(context.Background(), transaction.ComputerID, transaction.CommandID, transaction.CommandDigest, plan); err != nil || finalized {
		t.Fatalf("reserve frozen inputs: finalized=%v err=%v", finalized, err)
	}
	var status string
	if err := productStore.db.QueryRow(`SELECT status FROM computer_supervision_commands WHERE computer_id=? AND command_id=?`, transaction.ComputerID, transaction.CommandID).Scan(&status); err != nil || status != "input_frozen" {
		t.Fatalf("frozen input status = %q, %v", status, err)
	}
	if _, _, _, err := productStore.ReserveFrozenSupervisionInputs(context.Background(), transaction.ComputerID, transaction.CommandID, transaction.CommandDigest, plan); err != nil {
		t.Fatalf("exact frozen input retry: %v", err)
	}
	changed := plan
	changed.PrivateInputs = append([]computerevent.FrozenPrivateSupervisionInput(nil), plan.PrivateInputs...)
	changed.Transaction.ReferencedArtifacts = append([]computerevent.ReferencedArtifact(nil), plan.Transaction.ReferencedArtifacts...)
	changed.PrivateInputs[0].Envelope = []byte("different-encrypted-private-input")
	changed.PrivateInputs[0].ArtifactDigest = computerevent.DigestBytes(changed.PrivateInputs[0].Envelope)
	changedRef, err := computerevent.ArtifactRefFromDigest(changed.PrivateInputs[0].ArtifactDigest)
	if err != nil {
		t.Fatal(err)
	}
	changed.Transaction.ReferencedArtifacts[0].ArtifactDigest = changed.PrivateInputs[0].ArtifactDigest
	changed.Transaction.ReferencedArtifacts[0].Ref = changedRef.String()
	changed.Transaction.Mutations = append([]computerevent.SupervisionMutation(nil), plan.Transaction.Mutations...)
	changed.Transaction.Mutations[0].Body = json.RawMessage(strings.ReplaceAll(
		string(changed.Transaction.Mutations[0].Body), artifactRef.String(), changedRef.String(),
	))
	if _, _, _, err := productStore.ReserveFrozenSupervisionInputs(context.Background(), transaction.ComputerID, transaction.CommandID, transaction.CommandDigest, changed); !errors.Is(err, computerevent.ErrSupervisionIdempotencyConflict) {
		t.Fatalf("changed frozen input error = %v, want idempotency conflict", err)
	}
}

func TestPrepareRejectsDriftedExistingSupervisionCommandBinding(t *testing.T) {
	productStore := openTestStore(t)
	genesis := storeBootstrapSupervisionComputer(t, productStore)
	transaction := storeSupervisionOpenTransaction(t, genesis.CanonicalEventHead)
	request := storeSupervisionRequest(t, transaction, genesis)
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := productStore.db.Exec(`UPDATE computer_supervision_commands SET status='reserved', event_digest=NULL WHERE computer_id=? AND command_id=?`, transaction.ComputerID, transaction.CommandID); err != nil {
		t.Fatal(err)
	}
	if err := productStore.Prepare(context.Background(), request); !errors.Is(err, computerevent.ErrNeedsProjectionRepair) {
		t.Fatalf("drifted prepared binding error = %v, want repair refusal", err)
	}
}

func TestSupervisionProjectionFanoutCancellationLateDissentAndSettlementProperties(t *testing.T) {
	for fanout := 3; fanout <= 8; fanout++ {
		t.Run(fmt.Sprintf("fanout-%d", fanout), func(t *testing.T) {
			state := supervisionProjectionState{
				CanonicalEventHead: "current-head", IntentRevisionID: "intent-1", ArtifactHeadRevisionID: "revision-1",
				Entities: map[string]map[string]json.RawMessage{"attempt_started": {}},
				Statuses: map[string]map[string]string{
					"assignment": {"assignment-1": "open"},
					"attempt":    {},
					"result":     {},
				},
				OpenRebaseObligations: map[string]bool{}, OpenCompensationObligations: map[string]bool{},
				OpenFindings: map[string]bool{}, OpenDissents: map[string]bool{},
			}
			for ordinal := 1; ordinal <= fanout; ordinal++ {
				attemptID := fmt.Sprintf("attempt-%d", ordinal)
				state.Statuses["attempt"][attemptID] = "open"
				state.Entities["attempt_started"][attemptID] = json.RawMessage(`{"assignment_id":"assignment-1","ordinal":1}`)
			}
			transaction := computerevent.SupervisionTransaction{TransactionClass: "return_result"}
			apply := func(kind, body string) {
				t.Helper()
				decoded, err := supervisionBodyMap(json.RawMessage(body))
				if err != nil {
					t.Fatal(err)
				}
				if err := applySupervisionMutation(&state, transaction, kind, decoded); err != nil {
					t.Fatalf("apply %s: %v", kind, err)
				}
			}

			last := fmt.Sprintf("attempt-%d", fanout)
			apply("attempt_result", `{"assignment_id":"assignment-1","attempt_id":"`+last+`","result_id":"result-last","outcome":"succeeded","result_artifact_ref":"artifact:sha256:`+storeTestDigest('a')+`","evidence_refs":["artifact:sha256:`+storeTestDigest('a')+`"],"observed_base":{"canonical_event_head":"old-head","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1"},"delivered_after_cancellation":false}`)
			apply("assignment_cancelled", `{"assignment_id":"assignment-1","reason_artifact_ref":"artifact:sha256:`+storeTestDigest('b')+`","active_attempt_ids":[`+supervisionOpenAttemptIDs(fanout)+`]}`)
			apply("attempt_result", `{"assignment_id":"assignment-1","attempt_id":"attempt-1","result_id":"result-late","outcome":"succeeded","result_artifact_ref":"artifact:sha256:`+storeTestDigest('c')+`","evidence_refs":["artifact:sha256:`+storeTestDigest('c')+`"],"observed_base":{"canonical_event_head":"old-head","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1"},"delivered_after_cancellation":true}`)
			apply("dissent_recorded", `{"dissent_id":"dissent-1","subject":{"kind":"assignment","id":"assignment-1"},"stance_artifact_ref":"artifact:sha256:`+storeTestDigest('a')+`","evidence_refs":["artifact:sha256:`+storeTestDigest('a')+`"]}`)

			if state.Statuses["assignment"]["assignment-1"] != "cancelled" || state.Statuses["result"]["result-last"] != "returned" || state.Statuses["result"]["result-late"] != "late" {
				t.Fatalf("out-of-order/cancelled fan-out did not retain results: %+v", state.Statuses)
			}
			if !state.OpenRebaseObligations["rebase:result-last"] || !state.OpenRebaseObligations["rebase:result-late"] || !state.OpenDissents["dissent-1"] {
				t.Fatalf("semantic rebase or dissent was lost: rebases=%+v dissents=%+v", state.OpenRebaseObligations, state.OpenDissents)
			}
			if err := state.settlementReady(); err == nil {
				t.Fatal("settlement accepted cancelled/late fan-out with unresolved dispositions, rebase, and dissent")
			}
		})
	}
}

func TestSupervisionProjectionBindsRetryToSameAssignment(t *testing.T) {
	state := supervisionProjectionState{
		Entities: map[string]map[string]json.RawMessage{
			"attempt_started": {"attempt-1": json.RawMessage(`{"assignment_id":"assignment-1","ordinal":1}`)},
		},
		Statuses: map[string]map[string]string{
			"assignment": {"assignment-1": "open", "assignment-2": "open"},
			"attempt":    {"attempt-1": "returned"},
		},
	}
	retry, err := supervisionBodyMap(json.RawMessage(`{"assignment_id":"assignment-1","attempt_id":"attempt-2","attempt_kind":"retry","ordinal":2,"prior_attempt_id":"attempt-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, computerevent.SupervisionTransaction{TransactionClass: "start_attempt"}, "attempt_started", retry); err != nil {
		t.Fatalf("same-assignment retry rejected: %v", err)
	}
	wrongAssignment, err := supervisionBodyMap(json.RawMessage(`{"assignment_id":"assignment-2","attempt_id":"attempt-3","attempt_kind":"retry","ordinal":2,"prior_attempt_id":"attempt-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, computerevent.SupervisionTransaction{TransactionClass: "start_attempt"}, "attempt_started", wrongAssignment); err == nil || !strings.Contains(err.Error(), "retry attempt lineage") {
		t.Fatalf("cross-assignment retry = %v, want refusal", err)
	}
}

func TestSupervisionProjectionRejectsStaleSemanticHead(t *testing.T) {
	current, stale := "current-head", "stale-head"
	state := supervisionProjectionState{CanonicalEventHead: current, LifecycleVersion: 2, IntentRevisionID: "intent-1", ArtifactHeadRevisionID: "revision-1"}
	transaction := computerevent.SupervisionTransaction{TransactionClass: "revise_artifact", Expected: computerevent.SupervisionExpected{CanonicalEventHead: &stale}}
	if err := validateSupervisionExpected(transaction, state, true, current); err == nil || !strings.Contains(err.Error(), "stale canonical event head") {
		t.Fatalf("stale semantic head error = %v", err)
	}
	transaction.Expected.CanonicalEventHead = &current
	if err := validateSupervisionExpected(transaction, state, true, current); err == nil || !strings.Contains(err.Error(), "complete semantic expectations") {
		t.Fatalf("missing revise_artifact expectations error = %v", err)
	}
}

func TestSupervisionProjectionRequiresExactTextureRevisionLineage(t *testing.T) {
	state := supervisionProjectionState{
		ArtifactID: "document-1", ArtifactHeadRevisionID: "revision-current", IntentRevisionID: "intent-current",
		Entities: map[string]map[string]json.RawMessage{}, Statuses: map[string]map[string]string{},
		OpenRebaseObligations: map[string]bool{}, OpenCompensationObligations: map[string]bool{},
		OpenFindings: map[string]bool{}, OpenDissents: map[string]bool{},
	}
	mutation := func(parent, intent string) map[string]any {
		body, err := supervisionBodyMap(json.RawMessage(`{"artifact_id":"document-1","revision_id":"revision-next","title":"Next","parent_revision_id":"` + parent + `","content":"next","source_graph":{},"metadata":{},"metadata_digest":"` + storeTestDigest('a') + `","narrative_kind":"texture_synthesis","fulfills_intent_revision_id":"` + intent + `"}`))
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	transaction := computerevent.SupervisionTransaction{TransactionClass: "revise_artifact"}
	if err := applySupervisionMutation(&state, transaction, "texture_revision", mutation("revision-stale", "intent-current")); err == nil || !strings.Contains(err.Error(), "parent is stale") {
		t.Fatalf("stale parent error = %v", err)
	}
	if err := applySupervisionMutation(&state, transaction, "texture_revision", mutation("revision-current", "intent-stale")); err == nil || !strings.Contains(err.Error(), "intent is stale") {
		t.Fatalf("stale intent error = %v", err)
	}
	if err := applySupervisionMutation(&state, transaction, "texture_revision", mutation("revision-current", "intent-current")); err != nil {
		t.Fatalf("current lineage rejected: %v", err)
	}
	if state.ArtifactHeadRevisionID != "revision-next" {
		t.Fatalf("artifact head = %q, want revision-next", state.ArtifactHeadRevisionID)
	}
}

func TestSupervisionProjectionReplayReconstructsFieldEquivalentState(t *testing.T) {
	var states [2]supervisionProjectionState
	for index, name := range []string{"original", "rebuilt"} {
		t.Run(name, func(t *testing.T) {
			productStore := openTestStore(t)
			genesis := storeBootstrapSupervisionComputer(t, productStore)
			transaction := storeSupervisionOpenTransaction(t, genesis.CanonicalEventHead)
			finalizeSupervisionRequest(t, productStore, storeSupervisionRequest(t, transaction, genesis))
			state := loadSupervisionProjectionState(t, productStore, transaction)
			state.CanonicalEventHead = ""
			state.CreatedAt = time.Time{}
			state.UpdatedAt = time.Time{}
			states[index] = state
		})
	}
	if !reflect.DeepEqual(states[0], states[1]) {
		t.Fatalf("replayed projection fields differ:\noriginal=%+v\nrebuilt=%+v", states[0], states[1])
	}
}

func loadSupervisionProjectionState(t *testing.T, productStore *Store, transaction computerevent.SupervisionTransaction) supervisionProjectionState {
	t.Helper()
	stateID, err := lifecycleCanonicalID(ogKindSupervisionState, transaction.OwnerID, transaction.ComputerID, transaction.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	object, err := productStore.ogStore.GetObject(context.Background(), stateID)
	if err != nil {
		t.Fatal(err)
	}
	var state supervisionProjectionState
	if err := json.Unmarshal(object.Body, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func supervisionOpenAttemptIDs(fanout int) string {
	ids := make([]string, fanout-1)
	for ordinal := 1; ordinal < fanout; ordinal++ {
		ids[ordinal-1] = fmt.Sprintf(`"attempt-%d"`, ordinal)
	}
	return strings.Join(ids, ",")
}

func TestSupervisionProjectionRefusesInvalidTransitionBeforePrepare(t *testing.T) {
	productStore := openTestStore(t)
	genesis := storeBootstrapSupervisionComputer(t, productStore)
	openTransaction := storeSupervisionOpenTransaction(t, genesis.CanonicalEventHead)
	openRequest := storeSupervisionRequest(t, openTransaction, genesis)
	if err := productStore.Prepare(context.Background(), openRequest); err != nil {
		t.Fatal(err)
	}
	finalizeSupervisionRequest(t, productStore, openRequest)
	head, err := productStore.Head(context.Background(), openTransaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	artifact := "revision-1"
	transaction := computerevent.SupervisionTransaction{Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1, DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: "transaction-invalid", TransactionClass: "start_attempt", OwnerID: openTransaction.OwnerID, ComputerID: openTransaction.ComputerID, TrajectoryID: openTransaction.TrajectoryID, CommandID: "command-invalid", CommandDigest: computerevent.ZeroHead, Actor: computerevent.SupervisionActor{ActorID: "super-1", Role: "super", AuthorityRef: "authority:super"}, Expected: computerevent.SupervisionExpected{CanonicalEventHead: &head.CanonicalEventHead, LifecycleVersion: func() *uint64 { v := uint64(1); return &v }(), IntentRevisionID: func() *string { v := "intent-1"; return &v }(), ArtifactHeadRevisionID: &artifact}, ObservedBase: &computerevent.SupervisionObservedBase{CanonicalEventHead: head.CanonicalEventHead, IntentRevisionID: "intent-1", ArtifactHeadRevisionID: "revision-1"}, Mutations: []computerevent.SupervisionMutation{{Kind: "attempt_started", Body: json.RawMessage(`{"assignment_id":"missing-assignment","attempt_id":"attempt-1","attempt_kind":"initial","ordinal":1,"prior_attempt_id":null,"run_id":"run-1","observed_base":{"canonical_event_head":"` + head.CanonicalEventHead + `","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1"},"runtime_receipt_ref":"artifact:sha256:` + storeTestDigest('a') + `"}`)}}}
	transaction.CommandDigest, _ = transaction.ComputeCommandDigest()
	request := storeSupervisionRequest(t, transaction, head)
	err = productStore.Prepare(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "assignment is not open") {
		t.Fatalf("expected semantic preflight refusal, got %v", err)
	}
	after, err := productStore.Head(context.Background(), transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if after.CanonicalEventHead != head.CanonicalEventHead || after.Sequence != head.Sequence {
		t.Fatalf("head changed on refused transition: before=%+v after=%+v", head, after)
	}
	prepared, err := productStore.Prepared(context.Background(), transaction.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 0 {
		t.Fatalf("refused transition left prepared rows: %+v", prepared)
	}
}

func TestSupervisionProjectionRefusesUnknownDispositionAndSettlesExactClosedState(t *testing.T) {
	state := supervisionProjectionState{
		CanonicalEventHead: "current-head", IntentRevisionID: "intent-1", ArtifactHeadRevisionID: "revision-1",
		Entities: map[string]map[string]json.RawMessage{
			"disposition_recorded": {
				"disposition-assignment": json.RawMessage(`{"disposition_id":"disposition-assignment","target":{"kind":"assignment","id":"assignment-1"},"prior_disposition_id":null,"value":"cancelled","rationale_artifact_ref":"artifact:sha256:` + storeTestDigest('b') + `"}`),
			},
		},
		Statuses:              map[string]map[string]string{"assignment": {"assignment-1": "cancelled"}},
		ReferencedArtifacts:   map[string]bool{},
		OpenRebaseObligations: map[string]bool{}, OpenCompensationObligations: map[string]bool{}, OpenFindings: map[string]bool{}, OpenDissents: map[string]bool{},
	}
	transaction := computerevent.SupervisionTransaction{TransactionClass: "record_disposition"}
	unknown, err := supervisionBodyMap(json.RawMessage(`{"disposition_id":"disposition-unknown","target":{"kind":"result","id":"missing"},"prior_disposition_id":null,"value":"rejected","rationale_artifact_ref":"artifact:sha256:` + storeTestDigest('a') + `","evidence_refs":[],"compensation_obligation_id":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, transaction, "disposition_recorded", unknown); err == nil || !strings.Contains(err.Error(), "unknown disposition target") {
		t.Fatalf("unknown disposition target = %v, want refusal", err)
	}
	snapshotDigest, err := supervisionSettlementSnapshotDigest(state)
	if err != nil {
		t.Fatal(err)
	}
	fabricated, err := supervisionBodyMap(json.RawMessage(`{"proposal_id":"proposal-fabricated","canonical_event_head":"current-head","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1","assignment_ids":["assignment-1"],"attempt_ids":[],"result_ids":[],"update_ids":[],"disposition_ids":["disposition-assignment"],"finding_ids":[],"dissent_ids":[],"rebase_obligation_ids":[],"compensation_obligation_ids":[],"evidence_refs":["artifact:sha256:` + storeTestDigest('b') + `"],"owner_attention_ids":[],"snapshot_digest":"` + snapshotDigest + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, computerevent.SupervisionTransaction{TransactionClass: "propose_settlement"}, "settlement_proposed", fabricated); err == nil || !strings.Contains(err.Error(), "not retained") {
		t.Fatalf("fabricated settlement evidence = %v, want retained-ref refusal", err)
	}
	supervisionRecordReferencedArtifacts(&state, computerevent.SupervisionTransaction{
		ReferencedArtifacts: []computerevent.ReferencedArtifact{{Ref: "artifact:sha256:" + storeTestDigest('b')}},
	})
	snapshotDigest, err = supervisionSettlementSnapshotDigest(state)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := supervisionBodyMap(json.RawMessage(`{"proposal_id":"proposal-1","canonical_event_head":"current-head","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1","assignment_ids":["assignment-1"],"attempt_ids":[],"result_ids":[],"update_ids":[],"disposition_ids":["disposition-assignment"],"finding_ids":[],"dissent_ids":[],"rebase_obligation_ids":[],"compensation_obligation_ids":[],"evidence_refs":["artifact:sha256:` + storeTestDigest('b') + `"],"owner_attention_ids":[],"snapshot_digest":"` + snapshotDigest + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, computerevent.SupervisionTransaction{TransactionClass: "propose_settlement"}, "settlement_proposed", proposal); err != nil {
		t.Fatalf("closed settlement proposal rejected: %v", err)
	}
	if state.SettlementProposalID != "proposal-1" {
		t.Fatalf("settlement proposal not recorded: %+v", state)
	}
	settlement, err := supervisionBodyMap(json.RawMessage(`{"settlement_id":"settlement-1","proposal_id":"proposal-1","owner_decision_id":"owner-accept-1","settlement_artifact_ref":"artifact:sha256:` + storeTestDigest('c') + `","snapshot_digest":"` + snapshotDigest + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	settlementTransaction := computerevent.SupervisionTransaction{TransactionClass: "settle_trajectory", Actor: computerevent.SupervisionActor{ActorID: "owner-1", Role: "owner"}}
	if err := applySupervisionMutation(&state, settlementTransaction, "trajectory_settled", settlement); err == nil || !strings.Contains(err.Error(), "owner settlement acceptance") {
		t.Fatalf("settlement without owner acceptance error = %v", err)
	}
	acceptanceRaw := json.RawMessage(`{"decision_id":"owner-accept-1","proposal_id":"proposal-1","owner_actor_id":"owner-1","decision_artifact_ref":"artifact:sha256:` + storeTestDigest('d') + `","scope_digest":"` + storeTestDigest('e') + `","decision":"accept","settlement_snapshot_digest":"` + snapshotDigest + `"}`)
	acceptance, err := supervisionBodyMap(acceptanceRaw)
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, computerevent.SupervisionTransaction{TransactionClass: "record_owner_decision"}, "owner_decision_recorded", acceptance); err != nil {
		t.Fatalf("fresh owner acceptance rejected: %v", err)
	}
	if state.Entities["owner_decision_recorded"] == nil {
		state.Entities["owner_decision_recorded"] = map[string]json.RawMessage{}
	}
	state.Entities["owner_decision_recorded"]["owner-accept-1"] = acceptanceRaw
	if err := applySupervisionMutation(&state, settlementTransaction, "trajectory_settled", settlement); err != nil {
		t.Fatalf("closed settlement rejected: %v", err)
	}
	if !state.Settled {
		t.Fatalf("settlement was not recorded: %+v", state)
	}
}

func TestSupervisionProjectionMaterialRebaseValidatesBeliefAndArtifactPremises(t *testing.T) {
	state := supervisionProjectionState{
		IntentRevisionID: "intent-1", ArtifactID: "document-1", ArtifactHeadRevisionID: "revision-1",
		Entities: map[string]map[string]json.RawMessage{
			"super_belief_recorded": {"belief-1": json.RawMessage(`{"belief_id":"belief-1","belief_artifact_ref":"artifact:sha256:` + storeTestDigest('a') + `"}`)},
		},
		Statuses: map[string]map[string]string{}, OpenRebaseObligations: map[string]bool{},
		OpenCompensationObligations: map[string]bool{}, OpenFindings: map[string]bool{}, OpenDissents: map[string]bool{},
	}
	beliefDigest, err := supervisionTargetStateDigest(state, "belief", "belief-1")
	if err != nil {
		t.Fatal(err)
	}
	artifactDigest, err := supervisionTargetStateDigest(state, "artifact_premise", "document-1")
	if err != nil {
		t.Fatal(err)
	}
	stale, err := supervisionBodyMap(json.RawMessage(`{"intent_revision_id":"intent-2","parent_intent_revision_id":"intent-1","intent":"revised","material":true,"affected_targets":[{"kind":"belief","id":"belief-1","prior_intent_revision_id":"intent-0","state_digest":"` + beliefDigest + `","rebase_obligation_id":"rebase-stale"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, computerevent.SupervisionTransaction{TransactionClass: "revise_intent"}, "intent_revised", stale); err == nil || !strings.Contains(err.Error(), "target intent is stale") {
		t.Fatalf("stale material target intent = %v, want refusal", err)
	}
	if state.IntentRevisionID != "intent-1" || len(state.OpenRebaseObligations) != 0 {
		t.Fatalf("stale material target mutated state: %+v", state)
	}
	body, err := supervisionBodyMap(json.RawMessage(`{"intent_revision_id":"intent-2","parent_intent_revision_id":"intent-1","intent":"revised","material":true,"affected_targets":[{"kind":"belief","id":"belief-1","prior_intent_revision_id":"intent-1","state_digest":"` + beliefDigest + `","rebase_obligation_id":"rebase-belief"},{"kind":"artifact_premise","id":"document-1","prior_intent_revision_id":"intent-1","state_digest":"` + artifactDigest + `","rebase_obligation_id":"rebase-artifact"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, computerevent.SupervisionTransaction{TransactionClass: "revise_intent"}, "intent_revised", body); err != nil {
		t.Fatalf("exact material rebase rejected: %v", err)
	}
	unknown, err := supervisionBodyMap(json.RawMessage(`{"intent_revision_id":"intent-3","parent_intent_revision_id":"intent-2","intent":"revised again","material":true,"affected_targets":[{"kind":"belief","id":"missing","prior_intent_revision_id":"intent-2","state_digest":"` + storeTestDigest('b') + `","rebase_obligation_id":"rebase-missing"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applySupervisionMutation(&state, computerevent.SupervisionTransaction{TransactionClass: "revise_intent"}, "intent_revised", unknown); err == nil || !strings.Contains(err.Error(), "unknown supervision belief") {
		t.Fatalf("unknown material target = %v, want refusal", err)
	}
}
