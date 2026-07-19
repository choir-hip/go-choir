package computerevent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const (
	testComputerID = "computer-test"
	testDigestA    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testDigestB    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testDigestC    = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

func TestCanonicalJSONUsesUTF16OrderingAndRejectsFloats(t *testing.T) {
	got, err := CanonicalJSON(map[string]any{"\ue000": 1, "😀": 2, "a": "\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":"\n","😀":2,"":1}`
	if string(got) != want {
		t.Fatalf("canonical JSON = %s, want %s", got, want)
	}
	if _, err := CanonicalJSON(map[string]any{"float": 1.5}); err == nil {
		t.Fatal("CanonicalJSON accepted a floating-point protocol value")
	}
}

func TestSignedReceiptRoundTripAndTamperRefusal(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	issuedAt := time.Date(2026, 7, 18, 20, 0, 0, 123, time.UTC)
	receipt, err := NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{"computer_id": testComputerID, "sequence": uint64(1), "event_digest": testDigestA}, []SigningKey{key}, issuedAt)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	var decoded Receipt
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	resolver := staticKeyResolver{key: publicKey}
	if err := decoded.Verify(resolver); err != nil {
		t.Fatalf("round-trip receipt did not verify: %v", err)
	}
	decoded.KindFields["event_digest"] = testDigestB
	if err := decoded.Verify(resolver); err == nil {
		t.Fatal("tampered receipt verified")
	}
	decoded = receipt
	decoded.SignatureSet = append(decoded.SignatureSet, decoded.SignatureSet[0])
	if err := decoded.Verify(resolver); err == nil {
		t.Fatal("receipt with an extra signature verified")
	}
}

func TestReducerAcceptanceApplicationFailureAndRollback(t *testing.T) {
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	head, err := Reduce(nil, genesis, TransitionInput{TargetStateCommitment: testDigestA})
	if err != nil {
		t.Fatal(err)
	}
	if head.DesiredStateCommitment != testDigestA || head.EffectiveStateCommitment != testDigestA || head.PendingTransitionRef != "" {
		t.Fatalf("unexpected genesis head: %+v", head)
	}

	causal := testEvent(t, &head, EventMessageRecorded)
	causalHead, err := Reduce(&head, causal, TransitionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if causalHead.DesiredEventHead != head.DesiredEventHead || causalHead.EffectiveEventHead != head.EffectiveEventHead || causalHead.DesiredStateCommitment != head.DesiredStateCommitment {
		t.Fatalf("causal event changed semantic projection: before=%+v after=%+v", head, causalHead)
	}

	accepted := testEvent(t, &causalHead, EventEffectAccepted)
	accepted.ProposedEffectRef = "artifact:bundle"
	accepted.DecisionRef = "decision:owner"
	accepted.VerifierRefs = []string{"artifact:verifier"}
	acceptedHead, err := Reduce(&causalHead, accepted, TransitionInput{TargetStateCommitment: testDigestB})
	if err != nil {
		t.Fatal(err)
	}
	acceptedDigest, _ := accepted.Digest()
	if acceptedHead.PendingTransitionRef != acceptedDigest || acceptedHead.DesiredEventHead != acceptedDigest || acceptedHead.EffectiveStateCommitment != testDigestA {
		t.Fatalf("acceptance projection = %+v", acceptedHead)
	}
	secondAcceptance := testEvent(t, &acceptedHead, EventEffectAccepted)
	secondAcceptance.ProposedEffectRef = "artifact:other"
	secondAcceptance.DecisionRef = "decision:owner"
	secondAcceptance.VerifierRefs = []string{"artifact:verifier"}
	if _, err := Reduce(&acceptedHead, secondAcceptance, TransitionInput{TargetStateCommitment: testDigestC}); !errors.Is(err, ErrPendingTransition) {
		t.Fatalf("second acceptance error = %v, want ErrPendingTransition", err)
	}

	started := testEvent(t, &acceptedHead, EventMaterializationStarted)
	started.DecisionRef = acceptedHead.PendingTransitionRef
	startedHead, err := Reduce(&acceptedHead, started, TransitionInput{})
	if err != nil {
		t.Fatal(err)
	}
	applied := testEvent(t, &startedHead, EventMaterializationApplied)
	applied.DecisionRef = startedHead.PendingTransitionRef
	applied.ResultingEffectiveCommitment = testDigestB
	appliedHead, err := Reduce(&startedHead, applied, TransitionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if appliedHead.PendingTransitionRef != "" || appliedHead.EffectiveStateCommitment != testDigestB || appliedHead.DesiredStateCommitment != testDigestB {
		t.Fatalf("applied projection = %+v", appliedHead)
	}

	rollback := testEvent(t, &appliedHead, EventRollbackRequested)
	rollback.DecisionRef = "decision:rollback"
	rollbackHead, err := Reduce(&appliedHead, rollback, TransitionInput{TargetStateCommitment: testDigestA})
	if err != nil {
		t.Fatal(err)
	}
	failed := testEvent(t, &rollbackHead, EventMaterializationFailed)
	failed.DecisionRef = rollbackHead.PendingTransitionRef
	if _, err := Reduce(&rollbackHead, failed, TransitionInput{}); err == nil {
		t.Fatal("materialization failure without verified restoration was accepted")
	}
	failedHead, err := Reduce(&rollbackHead, failed, TransitionInput{RestoredPriorEffective: true})
	if err != nil {
		t.Fatal(err)
	}
	if failedHead.PendingTransitionRef != "" || failedHead.DesiredStateCommitment != failedHead.EffectiveStateCommitment || failedHead.EffectiveStateCommitment != testDigestB {
		t.Fatalf("failed materialization projection = %+v", failedHead)
	}
}

func TestAppenderAppendNewBindsCurrentProjection(t *testing.T) {
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
	genesis.Sequence = 99
	genesis.PreviousHead = testDigestB
	genesis.ResultingEffectiveCommitment = testDigestA
	if _, err := appender.AppendNew(context.Background(), genesis, TransitionInput{TargetStateCommitment: testDigestA}, nil); err != nil {
		t.Fatalf("append genesis: %v", err)
	}

	proposed := testEvent(t, nil, EventEffectProposed)
	proposed.Sequence = 1
	proposed.PreviousHead = ZeroHead
	proposed.ProposedEffectRef = testDigestB
	if _, err := appender.AppendNew(context.Background(), proposed, TransitionInput{}, nil); err != nil {
		t.Fatalf("append proposed effect: %v", err)
	}
	if len(cas.records) != 2 {
		t.Fatalf("records = %d, want 2", len(cas.records))
	}
	got := cas.records[1].Request.Event
	if got.Sequence != 2 || got.PreviousHead != cas.records[0].Request.EventDigest ||
		got.ExpectedDesiredEventHead != cas.records[0].Request.Next.DesiredEventHead ||
		got.ExpectedEffectiveEventHead != cas.records[0].Request.Next.EffectiveEventHead {
		t.Fatalf("new event was not bound to current projection: %+v", got)
	}
}

func TestAppenderAppendNewPayloadPinsBundleIntoEvent(t *testing.T) {
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

	payload := []byte(`{"bundle":"frozen"}`)
	proposed := testEvent(t, nil, EventEffectProposed)
	proposed.PrivacyClass = "public"
	_, digest, err := appender.AppendNewPayload(context.Background(), proposed, TransitionInput{}, payload, "application/vnd.choir.capsule-effect+json", "public")
	if err != nil {
		t.Fatal(err)
	}
	if digest != DigestBytes(payload) || len(cas.records) != 2 {
		t.Fatalf("payload digest/records = %q/%d", digest, len(cas.records))
	}
	request := cas.records[1].Request
	if request.Event.ProposedEffectRef != digest || request.Event.PayloadCommitment != digest ||
		len(request.Event.OutputArtifactRefs) != 1 || request.Event.OutputArtifactRefs[0] != digest ||
		len(request.PayloadPinReceiptDigests) != 1 {
		t.Fatalf("payload binding incomplete: %+v", request)
	}
}
func TestAppenderAppendNewPrivatePayloadEncryptsAndBindsEnvelope(t *testing.T) {
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
	keyring, err := NewFilePrivacyKeyring(filepath.Join(t.TempDir(), "keys"))
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := NewPrivateArtifactCipher(keyring)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("private repair prompt")
	started := testEvent(t, nil, EventTrajectoryStarted)
	started.PrivacyClass = "private"
	_, digest, err := appender.AppendNewPrivatePayload(context.Background(), started, TransitionInput{}, payload, "text/plain", cipher)
	if err != nil {
		t.Fatal(err)
	}
	if digest == DigestBytes(payload) || len(cas.records) != 2 {
		t.Fatalf("private envelope digest/records = %q/%d", digest, len(cas.records))
	}
	request := cas.records[1].Request
	if request.Event.PayloadCommitment != digest || request.Event.OutputArtifactRefs[0] != digest || len(request.PayloadPinReceiptDigests) != 1 {
		t.Fatalf("private payload binding incomplete: %+v", request)
	}
}

func TestAppenderRecoversCrashAfterCanonicalCAS(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	projection := &memoryProjection{failFinalizeOnce: true}
	cas := &memoryCAS{signer: signer}
	pinner := memoryPinner{signer: signer}
	verifier := EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}}
	appender, err := NewComputerEventAppender(testComputerID, pinner, projection, cas, verifier)
	if err != nil {
		t.Fatal(err)
	}
	event := testEvent(t, nil, EventGenesisImported)
	event.ResultingEffectiveCommitment = testDigestA
	genesisInput := TransitionInput{TargetStateCommitment: testDigestA}
	bindTestRequest(t, &event, genesisInput, nil)
	_, err = appender.Append(context.Background(), event, genesisInput, nil)
	if !errors.Is(err, ErrNeedsProjectionRepair) {
		t.Fatalf("append error = %v, want projection repair", err)
	}
	if cas.head == nil || projection.head != nil || len(projection.prepared) != 1 {
		t.Fatalf("crash fixture not split across stores: cas=%+v projection=%+v prepared=%d", cas.head, projection.head, len(projection.prepared))
	}
	if err := appender.RecoverPrepared(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cas.head, projection.head) || len(projection.prepared) != 0 {
		t.Fatalf("recovery did not converge: cas=%+v projection=%+v prepared=%d", cas.head, projection.head, len(projection.prepared))
	}
}

func TestAppenderReconstructsEmbeddedProjectionFromDurableChain(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := SigningKey{SignerRef: SignerRef{SignerDomain: "platform-control", KeyID: "platform-1"}, PrivateKey: privateKey}
	cas := &memoryCAS{signer: signer}
	pinner := memoryPinner{signer: signer}
	verifier := EventHeadReceiptVerifier{Keys: staticKeyResolver{key: publicKey}}
	originalProjection := &memoryProjection{}
	original, err := NewComputerEventAppender(testComputerID, pinner, originalProjection, cas, verifier)
	if err != nil {
		t.Fatal(err)
	}
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	genesisInput := TransitionInput{TargetStateCommitment: testDigestA}
	bindTestRequest(t, &genesis, genesisInput, nil)
	if _, err := original.Append(context.Background(), genesis, genesisInput, nil); err != nil {
		t.Fatal(err)
	}
	causal := testEvent(t, originalProjection.head, EventArtifactProduced)
	bindTestRequest(t, &causal, TransitionInput{}, nil)
	if _, err := original.Append(context.Background(), causal, TransitionInput{}, nil); err != nil {
		t.Fatal(err)
	}

	reconstructedProjection := &memoryProjection{}
	reconstructed, err := NewComputerEventAppender(testComputerID, pinner, reconstructedProjection, cas, verifier)
	if err != nil {
		t.Fatal(err)
	}
	if err := reconstructed.Reconstruct(context.Background(), cas); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cas.head, reconstructedProjection.head) {
		t.Fatalf("reconstructed head = %+v, canonical = %+v", reconstructedProjection.head, cas.head)
	}
}

func testEvent(t *testing.T, current *Head, kind EventKind) Event {
	t.Helper()
	id, err := NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := Event{
		SchemaVersion:                    SchemaVersionV1,
		EventID:                          id,
		ComputerID:                       testComputerID,
		Sequence:                         1,
		PreviousHead:                     ZeroHead,
		EventKind:                        kind,
		OccurredAt:                       time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey:                   "idem-" + id,
		RequestCommitment:                testDigestC,
		ActorProfile:                     "trusted-core",
		AuthorityRef:                     "authority:test",
		PayloadCommitment:                testDigestA,
		PrivacyClass:                     "private",
		ReducerVersion:                   ReducerVersionV1,
		ExpectedDesiredEventHead:         ZeroHead,
		ExpectedEffectiveEventHead:       ZeroHead,
		ExpectedDesiredStateCommitment:   ZeroHead,
		ExpectedEffectiveStateCommitment: ZeroHead,
	}
	if current != nil {
		event.Sequence = current.Sequence + 1
		event.PreviousHead = current.CanonicalEventHead
		event.ExpectedDesiredEventHead = current.DesiredEventHead
		event.ExpectedEffectiveEventHead = current.EffectiveEventHead
		event.ExpectedPendingTransitionRef = current.PendingTransitionRef
		event.ExpectedDesiredStateCommitment = current.DesiredStateCommitment
		event.ExpectedEffectiveStateCommitment = current.EffectiveStateCommitment
	}
	return event
}
func bindTestRequest(t *testing.T, event *Event, input TransitionInput, payloadPins []string) {
	t.Helper()
	pinIntent, err := ComputePinIntentCommitment(*event, input)
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := ComputeRequestCommitment(*event, input, pinIntent, payloadPins)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment = commitment
}

type staticKeyResolver struct{ key ed25519.PublicKey }

func (r staticKeyResolver) ResolveReceiptKey(string, string, string, uint64, time.Time) (ed25519.PublicKey, error) {
	return r.key, nil
}

type memoryPinner struct{ signer SigningKey }

func (p memoryPinner) PinEvent(_ context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (PinResult, error) {
	digest := DigestBytes(canonicalEvent)
	receipt, err := NewSignedReceipt("PinReceipt", "corpusd", map[string]any{"computer_id": computerID, "artifact_digest": digest, "request_commitment": requestCommitment}, []SigningKey{p.signer}, time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC))
	return PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

func (p memoryPinner) PinNonPrivatePayload(_ context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (PinResult, error) {
	digest := DigestBytes(payload)
	receipt, err := NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "media_type": mediaType,
		"privacy_class": privacyClass, "pin_intent_commitment": pinIntentCommitment,
	}, []SigningKey{p.signer}, time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC))
	return PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}
func (p memoryPinner) PreparePrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID, mediaType string, plaintext []byte) ([]byte, PrivateArtifactMetadata, error) {
	return cipher.Encrypt(ctx, computerID, eventID, mediaType, "private", plaintext)
}

func (p memoryPinner) PinPrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (PinResult, error) {
	if _, _, err := cipher.Decrypt(ctx, envelope, computerID, eventID); err != nil {
		return PinResult{}, err
	}
	digest := DigestBytes(envelope)
	receipt, err := NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "media_type": PrivateArtifactMediaType,
		"privacy_class": "private", "pin_intent_commitment": pinIntentCommitment,
	}, []SigningKey{p.signer}, time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC))
	return PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

type memoryProjection struct {
	head             *Head
	prepared         []CASRequest
	failFinalizeOnce bool
}

func (p *memoryProjection) Head(context.Context, string) (*Head, error) {
	return cloneHead(p.head), nil
}
func (p *memoryProjection) Prepare(_ context.Context, request CASRequest) error {
	p.prepared = append(p.prepared, request)
	return nil
}
func (p *memoryProjection) Prepared(context.Context, string) ([]CASRequest, error) {
	return append([]CASRequest(nil), p.prepared...), nil
}
func (p *memoryProjection) Finalize(_ context.Context, _ string, digest string, _ Receipt) error {
	if p.failFinalizeOnce {
		p.failFinalizeOnce = false
		return errors.New("injected finalize crash")
	}
	for index, request := range p.prepared {
		if request.EventDigest == digest {
			p.head = cloneHead(&request.Next)
			p.prepared = append(p.prepared[:index], p.prepared[index+1:]...)
			return nil
		}
	}
	return errors.New("prepared event absent")
}
func (p *memoryProjection) DiscardPrepared(_ context.Context, _ string, digest string) error {
	for index, request := range p.prepared {
		if request.EventDigest == digest {
			p.prepared = append(p.prepared[:index], p.prepared[index+1:]...)
		}
	}
	return nil
}

type memoryCAS struct {
	head     *Head
	signer   SigningKey
	receipts map[string]Receipt
	records  []DurableEvent
}

func (c *memoryCAS) Head(context.Context, string) (*Head, error) { return cloneHead(c.head), nil }
func (c *memoryCAS) CompareAndSwap(_ context.Context, request CASRequest) (Receipt, error) {
	if c.receipts == nil {
		c.receipts = make(map[string]Receipt)
	}
	if receipt, ok := c.receipts[request.Event.IdempotencyKey]; ok {
		return receipt, nil
	}
	if (c.head == nil && request.Event.PreviousHead != ZeroHead) || (c.head != nil && request.Event.PreviousHead != c.head.CanonicalEventHead) {
		return Receipt{}, ErrCASConflict
	}
	receipt, err := NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"computer_id": request.Event.ComputerID, "previous_head": request.Event.PreviousHead,
		"event_digest": request.EventDigest, "sequence": request.Event.Sequence,
		"event_kind": request.Event.EventKind, "request_commitment": request.Event.RequestCommitment,
		"pin_receipt_digests": append([]string{request.EventPinReceiptDigest}, request.PayloadPinReceiptDigests...),
		"desired_event_head":  request.Next.DesiredEventHead, "effective_event_head": request.Next.EffectiveEventHead,
		"pending_transition_ref": request.Next.PendingTransitionRef, "desired_state_commitment": request.Next.DesiredStateCommitment,
		"effective_state_commitment": request.Next.EffectiveStateCommitment,
	}, []SigningKey{c.signer}, time.Date(2026, 7, 18, 20, 0, 0, 0, time.UTC))
	if err != nil {
		return Receipt{}, err
	}
	c.head = cloneHead(&request.Next)
	c.receipts[request.Event.IdempotencyKey] = receipt
	c.records = append(c.records, DurableEvent{Request: request, Receipt: receipt})
	return receipt, nil
}

func (c *memoryCAS) Events(_ context.Context, _ string, afterSequence uint64) ([]DurableEvent, error) {
	var records []DurableEvent
	for _, record := range c.records {
		if record.Request.Event.Sequence > afterSequence {
			records = append(records, record)
		}
	}
	return records, nil
}

func cloneHead(head *Head) *Head {
	if head == nil {
		return nil
	}
	copy := *head
	return &copy
}
