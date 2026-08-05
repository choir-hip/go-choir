package computerevent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func testSupervisionTransaction(t *testing.T) SupervisionTransaction {
	t.Helper()
	transaction := SupervisionTransaction{
		Schema: SupervisionSchemaV1, Reducer: SupervisionReducerV1, DigestRecipe: SupervisionDigestRecipeV1,
		TransactionID: "command-1", TransactionClass: "open_trajectory", OwnerID: "owner-1", ComputerID: testComputerID,
		TrajectoryID: "trajectory-1", CommandID: "command-1", CommandDigest: ZeroHead,
		Actor:    SupervisionActor{ActorID: "texture-1", Role: "texture", AuthorityRef: "authority:test"},
		Expected: SupervisionExpected{},
		Mutations: []SupervisionMutation{
			{Kind: "trajectory_started", Body: json.RawMessage(`{"trajectory_kind":"document","subject_refs":{"artifact":"texture://documents/document-1"},"intent_revision_id":"intent-1","artifact_id":"document-1","artifact_revision_id":"revision-1","texture_actor_id":"texture-1","initial_assignment_ids":["assignment-1"],"objective":"Build the supervised document."}`)},
			{Kind: "intent_revised", Body: json.RawMessage(`{"intent_revision_id":"intent-1","parent_intent_revision_id":null,"intent":"Build the supervised document.","material":false,"affected_targets":[]}`)},
			{Kind: "texture_revision", Body: json.RawMessage(`{"artifact_id":"document-1","revision_id":"revision-1","title":"Supervised document","parent_revision_id":null,"content":"supervised content","source_graph":{},"metadata":{},"metadata_digest":"` + testDigestB + `","narrative_kind":"texture_synthesis","fulfills_intent_revision_id":"intent-1"}`)},
		},
	}
	digest, err := transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	transaction.CommandDigest = digest
	return transaction
}

func TestSupervisionCommandDigestBindsFinalMutationArtifactDigests(t *testing.T) {
	transaction := testSupervisionTransaction(t)
	var body map[string]any
	if err := json.Unmarshal(transaction.Mutations[0].Body, &body); err != nil {
		t.Fatal(err)
	}
	body["referenced_artifacts"] = []any{map[string]any{
		"binding_id":       "binding-1",
		"artifact_digest":  testDigestA,
		"plaintext_digest": testDigestB,
	}}
	transaction.Mutations[0].Body, _ = json.Marshal(body)
	first, err := transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	body["referenced_artifacts"].([]any)[0].(map[string]any)["artifact_digest"] = testDigestC
	transaction.Mutations[0].Body, _ = json.Marshal(body)
	second, err := transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("command digest did not bind final ciphertext artifact digest: %s", first)
	}
}
func testUnboundSupervisionEvent() Event {
	return Event{
		SchemaVersion: SchemaVersionV1, ComputerID: testComputerID,
		EventKind: EventSupervisionTransaction, PrivacyClass: "private",
		ReducerVersion: ReducerVersionV1,
	}
}

func TestSupervisionTransactionClosedAuthorizerAndDigest(t *testing.T) {
	transaction := testSupervisionTransaction(t)
	if err := transaction.Validate(); err != nil {
		t.Fatal(err)
	}

	wrongRole := transaction
	wrongRole.Actor.Role = "cosuper"
	wrongRole.CommandDigest, _ = wrongRole.ComputeCommandDigest()
	if err := wrongRole.Validate(); err == nil || !strings.Contains(err.Error(), "cannot authorize") {
		t.Fatalf("expected role refusal, got %v", err)
	}

	unknownField := transaction
	unknownField.Mutations = append([]SupervisionMutation(nil), transaction.Mutations...)
	unknownField.Mutations[0].Body = json.RawMessage(`{"intent_revision_id":"intent-1","artifact_id":"document-1","artifact_revision_id":"revision-1","texture_actor_id":"texture-1","initial_assignment_ids":["assignment-1"],"objective_artifact_ref":"artifact:sha256:` + testDigestA + `","unfrozen":true}`)
	unknownField.CommandDigest, _ = unknownField.ComputeCommandDigest()
	if err := unknownField.Validate(); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected closed-body refusal, got %v", err)
	}

	changed := transaction
	changed.OwnerID = "owner-2"
	if err := changed.Validate(); err == nil || !strings.Contains(err.Error(), "command_digest mismatch") {
		t.Fatalf("expected digest refusal, got %v", err)
	}
}

func TestSupervisionAttemptRetryLineageIsClosed(t *testing.T) {
	newAttempt := func(kind string, ordinal uint64, prior *string) SupervisionTransaction {
		body, err := json.Marshal(map[string]any{
			"assignment_id": "assignment-1", "attempt_id": "attempt-2", "attempt_kind": kind, "ordinal": ordinal,
			"prior_attempt_id": prior, "run_id": "run-2",
			"observed_base":       map[string]string{"canonical_event_head": testDigestA, "intent_revision_id": "intent-1", "artifact_head_revision_id": "revision-1"},
			"runtime_receipt_ref": "artifact:sha256:" + testDigestB,
		})
		if err != nil {
			t.Fatal(err)
		}
		transaction := SupervisionTransaction{
			Schema: SupervisionSchemaV1, Reducer: SupervisionReducerV1, DigestRecipe: SupervisionDigestRecipeV1,
			TransactionID: "attempt-command", TransactionClass: "start_attempt", OwnerID: "owner-1", ComputerID: testComputerID,
			TrajectoryID: "trajectory-1", CommandID: "attempt-command",
			Actor:     SupervisionActor{ActorID: "super-1", Role: "super", AuthorityRef: "authority:super"},
			Mutations: []SupervisionMutation{{Kind: "attempt_started", Body: body}},
		}
		transaction.CommandDigest, err = transaction.ComputeCommandDigest()
		if err != nil {
			t.Fatal(err)
		}
		return transaction
	}
	prior := "attempt-1"
	if err := newAttempt("retry", 2, &prior).Validate(); err != nil {
		t.Fatalf("valid retry lineage rejected: %v", err)
	}
	if err := newAttempt("retry", 1, &prior).Validate(); err == nil || !strings.Contains(err.Error(), "retry attempt") {
		t.Fatalf("retry ordinal one = %v, want refusal", err)
	}
	if err := newAttempt("initial", 1, &prior).Validate(); err == nil || !strings.Contains(err.Error(), "initial attempt") {
		t.Fatalf("initial attempt with prior = %v, want refusal", err)
	}
}

func TestSupervisionTransactionRejectsNonFrozenAuthorableMutations(t *testing.T) {
	transaction := testSupervisionTransaction(t)
	assignment := SupervisionMutation{Kind: "assignment_opened", Body: json.RawMessage(`{"assignment_id":"assignment-1","assigned_actor_id":"cosuper-1","assigned_role":"cosuper","parent_decision_id":"decision-1","intent_revision_id":"intent-1","observed_base":{"canonical_event_head":"` + ZeroHead + `","intent_revision_id":"intent-1","artifact_head_revision_id":"revision-1"},"scope_digest":"` + testDigestA + `","capability_digest":"` + testDigestB + `","policy_digest":"` + testDigestC + `","obligation_ids":["obligation-1"],"idempotency_commitment":"` + testDigestA + `"}`)}
	transaction.Mutations = append(transaction.Mutations, assignment)
	transaction.CommandDigest, _ = transaction.ComputeCommandDigest()
	if err := transaction.Validate(); err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("Texture open_trajectory authoring assignment = %v, want refusal", err)
	}

	for _, tc := range []struct {
		name, class, role, kind string
	}{
		{name: "update", class: "return_update", role: "cosuper", kind: "update_recorded"},
		{name: "terminal delivery", class: "record_terminal_delivery", role: "runtime", kind: "terminal_delivery_observed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rejected := testSupervisionTransaction(t)
			rejected.TransactionClass = tc.class
			rejected.Actor.Role = tc.role
			rejected.Mutations = []SupervisionMutation{{Kind: tc.kind, Body: json.RawMessage(`{}`)}}
			rejected.CommandDigest, _ = rejected.ComputeCommandDigest()
			if err := rejected.Validate(); err == nil || !strings.Contains(err.Error(), "unknown transaction_class") {
				t.Fatalf("%s authorable transaction = %v, want closed-vocabulary refusal", tc.kind, err)
			}
		})
	}
}

func TestSupervisionEventBindingRequiresTransactionArtifact(t *testing.T) {
	transaction := testSupervisionTransaction(t)
	canonical, err := transaction.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	event := testEvent(t, nil, EventSupervisionTransaction)
	event.EventID = transaction.TransactionID
	event.TrajectoryID = transaction.TrajectoryID
	event.IdempotencyKey = transaction.CommandID
	event.DecisionRef = DigestBytes(canonical)
	event.PayloadCommitment = transaction.CommandDigest
	event.ActorProfile = transaction.Actor.Role
	event.PrivacyClass = "private"
	event.AuthorityRef = transaction.Actor.AuthorityRef
	event.InputArtifactRefs = []string{"artifact:sha256:" + DigestBytes(canonical)}
	if err := ValidateSupervisionEventBinding(event, transaction); err != nil {
		t.Fatal(err)
	}

	event.InputArtifactRefs = nil
	if err := ValidateSupervisionEventBinding(event, transaction); err == nil || !strings.Contains(err.Error(), "artifact reference") {
		t.Fatalf("expected missing artifact refusal, got %v", err)
	}
}

func TestAppendNewSupervisionTransactionPinsAndBindsClosedEnvelope(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	projection := &memoryProjection{}
	cas := &memoryCAS{signer: signer}
	appender, err := NewComputerEventAppender(testComputerID, memoryPinner{signer: signer}, projection, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	transaction := testSupervisionTransaction(t)
	event := testUnboundSupervisionEvent()
	cipher, err := newPrivateArtifactCipher(testComputerID, base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x55}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	_, artifactDigest, err := appender.AppendNewSupervisionTransaction(context.Background(), event, TransitionInput{}, transaction, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if !IsSHA256(artifactDigest) {
		t.Fatalf("invalid encrypted artifact digest %q", artifactDigest)
	}
	if len(cas.records) != 2 {
		t.Fatalf("canonical event count = %d, want 2", len(cas.records))
	}
	appended := cas.records[1].Request
	if appended.Event.EventKind != EventSupervisionTransaction || appended.SupervisionTransaction == nil || appended.Event.PayloadCommitment != transaction.CommandDigest || appended.Event.DecisionRef != artifactDigest {
		t.Fatalf("unexpected supervision append: %+v", appended)
	}
}

func TestAppendNewSupervisionTransactionRecoversPreparedProjectionAfterFinalizeFailure(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	projection := &memoryProjection{}
	cas := &memoryCAS{signer: signer}
	appender, err := NewComputerEventAppender(testComputerID, memoryPinner{signer: signer}, projection, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	projection.failFinalizeOnce = true
	cipher, err := newPrivateArtifactCipher(testComputerID, base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x77}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := appender.AppendNewSupervisionTransaction(context.Background(), testUnboundSupervisionEvent(), TransitionInput{}, testSupervisionTransaction(t), cipher); err == nil || !strings.Contains(err.Error(), ErrNeedsProjectionRepair.Error()) {
		t.Fatalf("finalize crash error = %v, want projection repair", err)
	}
	if len(projection.prepared) != 1 || projection.prepared[0].SupervisionTransaction == nil || cas.head == nil {
		t.Fatalf("prepared supervision crash lost recovery plan: prepared=%+v head=%+v", projection.prepared, cas.head)
	}
	if err := appender.RecoverPrepared(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(projection.prepared) != 0 || projection.head == nil || projection.head.CanonicalEventHead != cas.head.CanonicalEventHead {
		t.Fatalf("recovered supervision projection diverged: projection=%+v canonical=%+v", projection.head, cas.head)
	}
}

func TestAppendNewSupervisionTransactionRetriesBeforePrivatePinning(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	pinner := &countingSupervisionPinner{memoryPinner: memoryPinner{signer: signer}}
	projection := &memoryProjection{}
	cas := &memoryCAS{signer: signer}
	appender, err := NewComputerEventAppender(testComputerID, pinner, projection, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}

	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	transaction := testSupervisionTransaction(t)
	event := testUnboundSupervisionEvent()
	cipher, err := newPrivateArtifactCipher(testComputerID, base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x66}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	first, artifact, err := appender.AppendNewSupervisionTransaction(context.Background(), event, TransitionInput{}, transaction, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if pinner.privatePins != 1 || len(cas.records) != 2 {
		t.Fatalf("first command private pins=%d events=%d", pinner.privatePins, len(cas.records))
	}
	retry, retryArtifact, err := appender.AppendNewSupervisionTransaction(context.Background(), event, TransitionInput{}, transaction, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if retryArtifact != artifact || retry.KindFields["event_digest"] != first.KindFields["event_digest"] || pinner.privatePins != 1 || len(cas.records) != 2 {
		t.Fatalf("exact retry repinned or changed receipt: artifact=%q/%q pins=%d events=%d", retryArtifact, artifact, pinner.privatePins, len(cas.records))
	}

	changed := transaction
	changed.Mutations = append([]SupervisionMutation(nil), transaction.Mutations...)
	changed.Mutations[1].Body = json.RawMessage(`{"intent_revision_id":"intent-1","parent_intent_revision_id":null,"intent":"Changed payload.","material":false,"affected_targets":[]}`)
	if _, _, err := appender.AppendNewSupervisionTransaction(context.Background(), event, TransitionInput{}, changed, cipher); err == nil || !strings.Contains(strings.ToLower(err.Error()), "idempot") {
		t.Fatalf("changed command payload error = %v, want idempotency conflict", err)
	}
	if pinner.privatePins != 1 || len(cas.records) != 2 {
		t.Fatalf("changed command payload reached pin/CAS: pins=%d events=%d", pinner.privatePins, len(cas.records))
	}
}

func TestAppendWithPrivateArtifactsReservesBeforeEncryptionAndReplaysWithoutRepinning(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	projection := &memoryProjection{}
	pinner := &reservationGuardPinner{
		countingSupervisionPinner: countingSupervisionPinner{memoryPinner: memoryPinner{signer: signer}},
		projection:                projection, commandID: "command-1",
	}
	cas := &memoryCAS{signer: signer}
	appender, err := NewComputerEventAppender(testComputerID, pinner, projection, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}

	transaction := testSupervisionTransaction(t)
	bindingID := transaction.CommandID + ":objective"
	var startBody map[string]any
	if err := json.Unmarshal(transaction.Mutations[0].Body, &startBody); err != nil {
		t.Fatal(err)
	}
	subjectRefs := startBody["subject_refs"].(map[string]any)
	subjectRefs["objective_artifact_ref"] = SupervisionArtifactPlaceholder(bindingID)
	transaction.Mutations[0].Body, err = json.Marshal(startBody)
	if err != nil {
		t.Fatal(err)
	}
	transaction.CommandDigest = ZeroHead
	payloads := []PrivateSupervisionArtifactPayload{{
		BindingID: bindingID, Plaintext: []byte(`{"objective":"reserved before encryption"}`),
		MediaType: SupervisionEvidenceMediaTypeV1,
	}}
	first, artifactDigest, firstArtifacts, err := appender.AppendNewSupervisionTransactionWithPrivateArtifacts(
		context.Background(), testUnboundSupervisionEvent(), TransitionInput{}, transaction, payloads, cipherForTest(t, 0x91),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !pinner.reservationObserved || pinner.privatePins != 2 || len(firstArtifacts) != 1 {
		t.Fatalf("reservation/pin receipt mismatch: reserved=%v pins=%d artifacts=%d", pinner.reservationObserved, pinner.privatePins, len(firstArtifacts))
	}
	retry, retryArtifactDigest, retryArtifacts, err := appender.AppendNewSupervisionTransactionWithPrivateArtifacts(
		context.Background(), testUnboundSupervisionEvent(), TransitionInput{}, transaction, payloads, cipherForTest(t, 0x91),
	)
	if err != nil {
		t.Fatal(err)
	}
	if pinner.privatePins != 2 || retryArtifactDigest != artifactDigest || len(retryArtifacts) != 1 ||
		retryArtifacts[0].ArtifactDigest != firstArtifacts[0].ArtifactDigest ||
		retry.KindFields["event_digest"] != first.KindFields["event_digest"] {
		t.Fatalf("accepted retry encrypted, repinned, or changed identity: pins=%d refs=%+v/%+v", pinner.privatePins, firstArtifacts, retryArtifacts)
	}
}

func TestAppendWithPrivateArtifactsRecoversInputFrozenPlanWithoutRebuildingDependencies(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	projection := &memoryProjection{}
	pinner := &failFirstReservedInputPinner{reservationGuardPinner: reservationGuardPinner{
		countingSupervisionPinner: countingSupervisionPinner{memoryPinner: memoryPinner{signer: signer}},
		projection:                projection, commandID: "command-1",
	}, fail: true}
	cas := &memoryCAS{signer: signer}
	appender, err := NewComputerEventAppender(testComputerID, pinner, projection, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}

	transaction := testSupervisionTransaction(t)
	evidenceBinding := transaction.CommandID + ":evidence"
	manifestBinding := transaction.CommandID + ":manifest"
	var startBody map[string]any
	if err := json.Unmarshal(transaction.Mutations[0].Body, &startBody); err != nil {
		t.Fatal(err)
	}
	subjectRefs := startBody["subject_refs"].(map[string]any)
	subjectRefs["evidence_artifact_ref"] = SupervisionArtifactPlaceholder(evidenceBinding)
	subjectRefs["manifest_artifact_ref"] = SupervisionArtifactPlaceholder(manifestBinding)
	transaction.Mutations[0].Body, err = json.Marshal(startBody)
	if err != nil {
		t.Fatal(err)
	}
	transaction.CommandDigest = ZeroHead
	source := "source-before-crash"
	finalizeCalls := 0
	payloads := []PrivateSupervisionArtifactPayload{
		{BindingID: evidenceBinding, Plaintext: []byte(`{"evidence":"stable"}`), MediaType: SupervisionEvidenceMediaTypeV1},
		{
			BindingID: manifestBinding, Plaintext: []byte(`{"manifest":"logical"}`), MediaType: SupervisionEvidenceMediaTypeV1,
			Finalize: func(replacements map[string]string) ([]byte, map[string]string, error) {
				finalizeCalls++
				raw, err := CanonicalJSON(map[string]string{
					"evidence_ref": replacements[SupervisionArtifactPlaceholder(evidenceBinding)],
					"source":       source,
				})
				return raw, nil, err
			},
		},
	}
	cipher := cipherForTest(t, 0x92)
	if _, _, _, err := appender.AppendNewSupervisionTransactionWithPrivateArtifacts(
		context.Background(), testUnboundSupervisionEvent(), TransitionInput{}, transaction, payloads, cipher,
	); err == nil || !strings.Contains(err.Error(), "injected input pin crash") {
		t.Fatalf("first append error = %v, want injected crash", err)
	}
	plan, found := projection.frozenPlans[transaction.CommandID]
	if !found || len(plan.PrivateInputs) != 2 || finalizeCalls != 1 || pinner.privatePins != 0 {
		t.Fatalf("frozen input plan = found=%v inputs=%d finalize=%d pins=%d", found, len(plan.PrivateInputs), finalizeCalls, pinner.privatePins)
	}
	source = "source-after-crash"
	recovered, found, err := appender.RecoverFrozenSupervisionTransaction(context.Background(), transaction.CommandID)
	if err != nil || !found {
		t.Fatalf("recover frozen transaction: found=%v err=%v", found, err)
	}
	receipt, err := appender.ResumeFrozenSupervisionTransaction(context.Background(), recovered, cipher)
	if err != nil {
		t.Fatalf("resume frozen transaction: %v", err)
	}
	if finalizeCalls != 1 || pinner.privatePins != 3 || receipt.KindFields["event_digest"] == "" {
		t.Fatalf("resume rebuilt inputs or missed append: finalize=%d pins=%d receipt=%+v", finalizeCalls, pinner.privatePins, receipt)
	}
}

func cipherForTest(t *testing.T, fill byte) *PrivateArtifactCipher {
	t.Helper()
	cipher, err := newPrivateArtifactCipher(testComputerID, base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}
func TestAppendNewSupervisionTransactionRetriesFrozenPlanAfterPinPersistenceCrash(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	pinner := &countingSupervisionPinner{memoryPinner: memoryPinner{signer: signer}}
	projection := &memoryProjection{failPinRecordOnce: true}
	cas := &memoryCAS{signer: signer}
	appender, err := NewComputerEventAppender(testComputerID, pinner, projection, cas, EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}})
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatal(err)
	}
	transaction := testSupervisionTransaction(t)
	cipher, err := newPrivateArtifactCipher(testComputerID, base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{0x88}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := appender.AppendNewSupervisionTransaction(context.Background(), testUnboundSupervisionEvent(), TransitionInput{}, transaction, cipher); err == nil {
		t.Fatal("pin receipt persistence crash was accepted")
	}
	plan, found := projection.frozenPlans[transaction.CommandID]
	if !found || plan.PinReceipt != nil || pinner.privatePins != 1 || len(cas.records) != 1 {
		t.Fatalf("crash did not leave one frozen unprepared plan: plan=%+v pins=%d records=%d", plan, pinner.privatePins, len(cas.records))
	}
	if pending, err := appender.HasPendingSupervisionReservation(context.Background(), transaction.CommandID); err != nil || !pending {
		t.Fatalf("frozen command pending reservation = %v, %v", pending, err)
	}
	frozen, found, err := appender.RecoverFrozenSupervisionTransaction(context.Background(), transaction.CommandID)
	if err != nil || !found || frozen.TransactionID != plan.EventID {
		t.Fatalf("recover frozen transaction: found=%v transaction=%+v err=%v", found, frozen, err)
	}
	receipt, err := appender.ResumeFrozenSupervisionTransaction(context.Background(), frozen, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if pinner.privatePins != 1 || len(cas.records) != 2 {
		t.Fatalf("frozen recovery duplicated private pin or event: pins=%d records=%d", pinner.privatePins, len(cas.records))
	}
	appended := cas.records[1].Request
	if appended.Event.EventID != plan.EventID || appended.Event.OccurredAt != plan.OccurredAt || appended.Event.DecisionRef != plan.ArtifactDigest || receipt.KindFields["event_digest"] != appended.EventDigest {
		t.Fatalf("recovery did not reuse frozen event identity, artifact, and receipt: request=%+v", appended)
	}
	if pending, err := appender.HasPendingSupervisionReservation(context.Background(), transaction.CommandID); err != nil || pending {
		t.Fatalf("finalized command pending reservation = %v, %v", pending, err)
	}
	finalReceipt, accepted, found, err := appender.RecoverFinalizedSupervisionTransaction(context.Background(), transaction.CommandID)
	if err != nil || !found || accepted.TransactionID != plan.EventID || finalReceipt.KindFields["event_digest"] != receipt.KindFields["event_digest"] {
		t.Fatalf("recover finalized transaction: found=%v transaction=%+v receipt=%+v err=%v", found, accepted, finalReceipt, err)
	}
}

func TestProjectionImportBindsCanonicalManifestDigest(t *testing.T) {
	manifest := map[string]any{"schema": "choir.supervision_projection_import.v1", "objects": []any{map[string]any{"canonical_id": "document-1"}}, "projection_digest": ""}
	canonical, err := CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	digest := DigestBytes(canonical)
	manifest["projection_digest"] = digest
	artifactCanonical, err := CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"import_ref": "artifact:sha256:" + digest, "import_digest": digest,
		"import_artifact_plaintext_digest": DigestBytes(artifactCanonical),
		"source_dolt_commit":               "dolt-commit-1", "source_projection_digest": testDigestA,
		"legacy_lifecycle_watermark": 1, "object_count": 1, "edge_count": 0, "refusal_count": 0,
		"quiescence_receipt_ref": "artifact:sha256:" + testDigestB,
		"drain_receipt_refs":     []string{"artifact:sha256:" + testDigestC}, "manifest": manifest,
	})
	if err != nil {
		t.Fatal(err)
	}
	transaction := SupervisionTransaction{
		Schema: SupervisionSchemaV1, Reducer: SupervisionReducerV1, DigestRecipe: SupervisionDigestRecipeV1,
		TransactionID: "import-1", TransactionClass: "projection_import", OwnerID: "owner-1", ComputerID: testComputerID,
		TrajectoryID: "trajectory-1", CommandID: "import-1",
		Actor:     SupervisionActor{ActorID: "runtime", Role: "runtime", AuthorityRef: "authority:projection-import"},
		Mutations: []SupervisionMutation{{Kind: "projection_imported", Body: body}},
	}
	transaction.CommandDigest, err = transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.Validate(); err != nil {
		t.Fatalf("valid import rejected: %v", err)
	}
	var changed map[string]any
	if err := json.Unmarshal(body, &changed); err != nil {
		t.Fatal(err)
	}
	changed["manifest"] = map[string]any{"schema": "changed", "objects": []any{map[string]any{"canonical_id": "document-1"}}}
	transaction.Mutations[0].Body, err = json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	transaction.CommandDigest, err = transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	if err := transaction.Validate(); err == nil || !strings.Contains(err.Error(), "does not bind manifest") {
		t.Fatalf("changed manifest validation error = %v", err)
	}
}

type countingSupervisionPinner struct {
	memoryPinner
	privatePins int
	pins        map[string]PinResult
}

func (p *countingSupervisionPinner) PinPrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (PinResult, error) {
	key := eventID + ":" + DigestBytes(envelope)
	if pin, ok := p.pins[key]; ok {
		return pin, nil
	}
	pin, err := p.memoryPinner.PinPrivatePayload(ctx, cipher, computerID, eventID, envelope, pinIntentCommitment)
	if err == nil {
		if p.pins == nil {
			p.pins = make(map[string]PinResult)
		}
		p.pins[key] = pin
		p.privatePins++
	}
	return pin, err
}

type reservationGuardPinner struct {
	countingSupervisionPinner
	projection          *memoryProjection
	commandID           string
	reservationObserved bool
}

func (p *reservationGuardPinner) PinPrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (PinResult, error) {
	if p.projection == nil || p.projection.reservations[p.commandID] == "" {
		return PinResult{}, errors.New("private payload reached pinning before command reservation")
	}
	p.reservationObserved = true
	return p.countingSupervisionPinner.PinPrivatePayload(ctx, cipher, computerID, eventID, envelope, pinIntentCommitment)
}

type failFirstReservedInputPinner struct {
	reservationGuardPinner
	fail bool
}

func (p *failFirstReservedInputPinner) PinPrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (PinResult, error) {
	if p.fail {
		if p.projection == nil || p.projection.reservations[p.commandID] == "" {
			return PinResult{}, errors.New("private payload reached pinning before command reservation")
		}
		p.reservationObserved = true
		p.fail = false
		return PinResult{}, errors.New("injected input pin crash")
	}
	return p.reservationGuardPinner.PinPrivatePayload(ctx, cipher, computerID, eventID, envelope, pinIntentCommitment)
}
