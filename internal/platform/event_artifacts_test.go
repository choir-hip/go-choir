package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	productstore "github.com/yusefmosiah/go-choir/internal/store"
)

func TestEventArtifactPinsBindEveryEventArtifactReference(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{
		store:         &Store{},
		artifactsRoot: t.TempDir(),
		signingKey:    &SigningKey{Private: privateKey, Public: publicKey, KeyID: "platform-test"},
	}
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: publicKey})
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("public observation")
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion:                    computerevent.SchemaVersionV1,
		EventID:                          eventID,
		ComputerID:                       "computer-test",
		Sequence:                         1,
		PreviousHead:                     computerevent.ZeroHead,
		EventKind:                        computerevent.EventGenesisImported,
		OccurredAt:                       time.Date(2026, 7, 18, 22, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey:                   "genesis-test",
		ActorProfile:                     "trusted-core",
		AuthorityRef:                     "authority:test",
		InputArtifactRefs:                []string{"sha256:" + computerevent.DigestBytes(payload)},
		PayloadCommitment:                platformTestDigest('a'),
		PrivacyClass:                     "public",
		ReducerVersion:                   computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead:         computerevent.ZeroHead,
		ExpectedEffectiveEventHead:       computerevent.ZeroHead,
		ExpectedDesiredStateCommitment:   computerevent.ZeroHead,
		ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment:     platformTestDigest('b'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: platformTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	payloadPin, err := artifacts.pinPayload(context.Background(), event.ComputerID, payload, "application/octet-stream", "public", pinIntent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := artifacts.pinPayload(context.Background(), event.ComputerID, []byte("raw secret"), "text/plain", "private", pinIntent); err == nil {
		t.Fatal("raw private payload was pinned without an encrypted envelope")
	}
	payloadReceipt, err := payloadPin.Receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	payloadReceiptDigest := computerevent.DigestBytes(payloadReceipt)
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, []string{payloadReceiptDigest})
	if err != nil {
		t.Fatal(err)
	}
	eventJSON, err := event.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	eventPin, err := artifacts.PinEvent(context.Background(), event.ComputerID, eventJSON, event.RequestCommitment)
	if err != nil {
		t.Fatal(err)
	}
	eventReceipt, err := eventPin.Receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{
		Event: event, EventDigest: eventPin.ArtifactDigest, EventArtifactDigest: eventPin.ArtifactDigest,
		EventPinReceiptDigest: computerevent.DigestBytes(eventReceipt), PayloadPinReceiptDigests: []string{payloadReceiptDigest},
		PinIntentCommitment: pinIntent, Input: input,
	}
	if err := artifacts.ValidateEventPins(context.Background(), request); err != nil {
		t.Fatalf("valid pin set refused: %v", err)
	}
	request.Event.InputArtifactRefs = []string{"sha256:" + platformTestDigest('d')}
	if err := artifacts.ValidateEventPins(context.Background(), request); err == nil {
		t.Fatal("pin set for an unrelated artifact was accepted")
	}
}

func TestPrivatePayloadAppendCompletesDirectedCommitmentGraph(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	if service.signingKey == nil {
		t.Fatal("platform signing key unavailable")
	}
	resolver := platformTestKeyResolver{key: service.signingKey.Public}
	artifacts, err := NewEventArtifactService(service, resolver)
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(platformStore, "corpusd", service.computerEventSigningKey(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	token, err := MintComputerCapability(ComputerCapability{
		Version: 1, ComputerID: "computer-private", Scopes: []string{"event:read", "event:pin", "event:append"},
		ExpiresAt: now.Add(4 * time.Minute).Format(time.RFC3339Nano), Nonce: "private-http-test",
	}, service.signingKey.Private)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service)
	if err := handler.ConfigureComputerEvents(cas, artifacts, SignedCapabilityVerifier{
		Store: platformStore, PublicKey: service.signingKey.Public, Now: func() time.Time { return now },
	}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/computers/events/head", handler.HandleComputerEventHead)
	mux.HandleFunc("/internal/computers/events/pin", handler.HandleComputerEventPin)
	mux.HandleFunc("/internal/computers/events/append", handler.HandleComputerEventAppend)
	mux.HandleFunc("/internal/computers/checkpoints", handler.HandleComputerCheckpoint)
	server := httptest.NewServer(mux)
	defer server.Close()
	eventClient, err := computerevent.NewHTTPClient(server.URL, server.Client(), func(context.Context) (string, error) {
		return token, nil
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := productstore.Open(filepath.Join(root, "product", "choir.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer projection.Close()
	cipher, err := computerevent.LoadGuestPrivateArtifactCipher(filepath.Join(t.TempDir(), "privacy-key"), "computer-private", true)
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("private model response")
	envelope, metadata, err := eventClient.PreparePrivatePayload(context.Background(), cipher, "computer-private", eventID, "text/plain", plaintext)
	if err != nil || metadata.EventID != eventID {
		t.Fatalf("prepare private payload = %#v, %v", metadata, err)
	}
	tamperedEventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	var tamperedMetadataEnvelope map[string]any
	if err := json.Unmarshal(envelope, &tamperedMetadataEnvelope); err != nil {
		t.Fatal(err)
	}
	tamperedMetadataEnvelope["metadata"].(map[string]any)["event_id"] = tamperedEventID
	tamperedMetadataJSON, err := computerevent.CanonicalJSON(tamperedMetadataEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eventClient.PinPrivatePayload(context.Background(), cipher, "computer-private", tamperedEventID, tamperedMetadataJSON, platformTestDigest('f')); err == nil {
		t.Fatal("AEAD-unauthenticated private envelope metadata was pinned")
	}
	var tamperedCiphertextEnvelope map[string]any
	if err := json.Unmarshal(envelope, &tamperedCiphertextEnvelope); err != nil {
		t.Fatal(err)
	}
	tamperedCiphertextEnvelope["ciphertext"] = "AAAAAAAAAAAAAAAAAAAAAA"
	tamperedCiphertextJSON, err := computerevent.CanonicalJSON(tamperedCiphertextEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eventClient.PinPrivatePayload(context.Background(), cipher, "computer-private", eventID, tamperedCiphertextJSON, platformTestDigest('f')); err == nil {
		t.Fatal("AEAD-unauthenticated private ciphertext was pinned")
	}
	verifierPublicKey, verifierPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	verifierSigningKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "verifier-control", KeyID: "verifier-test"}, PrivateKey: verifierPrivateKey}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-private",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventGenesisImported,
		OccurredAt:     time.Date(2026, 7, 18, 23, 30, 0, 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey: "private-genesis", ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		OutputArtifactRefs: []string{"sha256:" + computerevent.DigestBytes(envelope)},
		PayloadCommitment:  computerevent.DigestBytes(plaintext), PrivacyClass: "private",
		VerifierRefs: []string{
			"updater-key:updater-test:sha256:" + platformTestDigest('7'),
			"verifier-key:" + verifierSigningKey.KeyID + ":sha256:" + computerevent.DigestBytes(verifierPublicKey),
			"release:sha256:" + platformTestDigest('e'),
		},
		ReducerVersion:           computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: platformTestDigest('e'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: event.ResultingEffectiveCommitment}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	payloadPin, err := eventClient.PinPrivatePayload(context.Background(), cipher, event.ComputerID, event.EventID, envelope, pinIntent)
	if err != nil {
		t.Fatal(err)
	}
	payloadReceipt, err := payloadPin.Receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	payloadReceiptDigest := computerevent.DigestBytes(payloadReceipt)
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, []string{payloadReceiptDigest})
	if err != nil {
		t.Fatal(err)
	}
	appender, err := computerevent.NewComputerEventAppender(
		event.ComputerID, eventClient, projection, eventClient,
		computerevent.EventHeadReceiptVerifier{Keys: resolver},
	)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := appender.Append(context.Background(), event, input, []string{payloadReceiptDigest})
	if err != nil {
		t.Fatalf("private payload HTTP append failed: %v", err)
	}
	head, err := eventClient.Head(context.Background(), event.ComputerID)
	if err != nil || head == nil || head.CanonicalEventHead != receipt.KindFields["event_digest"] {
		t.Fatalf("canonical head = %#v, %v; want appended event", head, err)
	}
	checkpointCode, err := computerversion.NewCodeClosure(platformTestDigest('4'), []computerversion.CodeArtifact{{
		Name: "bundle", SHA256: platformTestDigest('5'), URI: "artifact+sha256://" + platformTestDigest('5') + "/bundle",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	checkpointProgram, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "capsule_effect_bundle", ContentSHA256: platformTestDigest('5'), ArtifactURI: "artifact+sha256://" + platformTestDigest('5') + "/bundle",
	}}, now)
	if err != nil {
		t.Fatal(err)
	}
	checkpointVersion := computerversion.ComputerVersion{CodeRef: checkpointCode.Ref, ArtifactProgramRef: checkpointProgram.Ref}
	verifierRequest := selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: event.ComputerID, OperationID: "genesis-baseline",
		BundleDigest: platformTestDigest('5'), VerificationEventDigest: head.CanonicalEventHead,
		VerifierEvidenceRefs: []string{head.CanonicalEventHead}, DecisionEventHead: head.CanonicalEventHead,
		CodeRef: string(checkpointVersion.CodeRef), ArtifactProgramRef: string(checkpointVersion.ArtifactProgramRef),
		ReleaseDigest: platformTestDigest('e'), Decision: "genesis_baseline",
	}
	verifierCertificate, err := selfdevprotocol.NewVerifierCertificate(verifierRequest, verifierSigningKey, now)
	if err != nil {
		t.Fatal(err)
	}
	verifierResponse := selfdevprotocol.VerifierCertificateResponse{
		Request: verifierRequest, Certificate: verifierCertificate,
		PublicKey: base64.RawStdEncoding.EncodeToString(verifierPublicKey),
	}
	verifierJSON, _ := computerevent.CanonicalJSON(verifierCertificate)
	checkpointRequest := selfdevprotocol.CheckpointRequest{
		ComputerID: event.ComputerID, IdempotencyKey: "checkpoint-private",
		ComputerVersion:   checkpointVersion,
		AcceptedEventHead: head.CanonicalEventHead, EffectiveEventHead: head.EffectiveEventHead,
		EffectiveStateCommitment: head.EffectiveStateCommitment, EventHeadReceiptID: receipt.ReceiptID,
		ReleaseDigest: platformTestDigest('e'), ReconstructionDigest: platformTestDigest('f'),
		MaterializationReceiptDigest: platformTestDigest('1'), VerifierCertificateDigest: computerevent.DigestBytes(verifierJSON),
		VerifierCertificate: verifierResponse, VerifierTrustBootstrap: true, ReducerVersion: head.ReducerVersion,
	}
	checkpointBody, err := computerevent.CanonicalJSON(checkpointRequest)
	if err != nil {
		t.Fatal(err)
	}
	checkpointHTTP, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/internal/computers/checkpoints", bytes.NewReader(checkpointBody))
	if err != nil {
		t.Fatal(err)
	}
	checkpointHTTP.Header.Set("Authorization", "Bearer "+token)
	checkpointResponse, err := server.Client().Do(checkpointHTTP)
	if err != nil {
		t.Fatal(err)
	}
	defer checkpointResponse.Body.Close()
	var published selfdevprotocol.CheckpointResponse
	if err := json.NewDecoder(checkpointResponse.Body).Decode(&published); err != nil {
		t.Fatal(err)
	}
	expectedCheckpointJSON, _ := computerevent.CanonicalJSON(checkpointRequest)
	publishedCheckpointJSON, _ := computerevent.CanonicalJSON(published.Checkpoint.Request)
	if checkpointResponse.StatusCode != http.StatusCreated || !bytes.Equal(expectedCheckpointJSON, publishedCheckpointJSON) || published.Receipt.Verify(service.signingKey.Public) != nil {
		t.Fatalf("checkpoint response status=%d response=%+v", checkpointResponse.StatusCode, published)
	}
	oldVersion := computerversion.ComputerVersion{CodeRef: computerversion.CodeRef("code:sha256:" + platformTestDigest('6')), ArtifactProgramRef: computerversion.ArtifactProgramRef("artifact-program:sha256:" + platformTestDigest('7'))}
	checkpointReceiptDigest, _ := selfdevprotocol.Digest(published.Receipt)
	acceptedPayload := selfdevprotocol.AcceptedEventAuthorizationEvidence{
		Version: 1, ComputerID: event.ComputerID, AcceptedOrRollbackEventDigest: head.CanonicalEventHead,
		EventHeadReceiptID: receipt.ReceiptID, EffectiveEventHead: head.EffectiveEventHead,
		OldComputerVersion: oldVersion, NewComputerVersion: checkpointVersion,
		DecisionActor: "owner", DecisionScope: "computer:self_development:approve",
	}
	acceptedJSON, _ := computerevent.CanonicalJSON(acceptedPayload)
	approval, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, "computer:owner:primary", checkpointVersion, acceptedJSON, now)
	joinPayload := selfdevprotocol.PromotionJoinEvidence{
		Version: 1, ComputerID: event.ComputerID, EventHeadReceiptID: receipt.ReceiptID,
		CheckpointReceiptDigest: checkpointReceiptDigest, MaterializationReceiptDigest: checkpointRequest.MaterializationReceiptDigest,
		VerifierCertificateDigest: checkpointRequest.VerifierCertificateDigest, OldComputerVersion: oldVersion, NewComputerVersion: checkpointVersion,
	}
	joinJSON, _ := computerevent.CanonicalJSON(joinPayload)
	promotion, _ := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, "computer:owner:primary", checkpointVersion, joinJSON, now)
	command := routeledger.TransitionCommand{
		RouteSlotID: "computer:owner:primary", Kind: routeledger.TransitionPromote, Old: oldVersion, New: checkpointVersion, ExpectedGeneration: 1,
		ApprovalRef: routeledger.ApprovalRef(approval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(promotion.Ref),
		IdempotencyKey: "idempotency:selfdev-route",
	}
	projectionRequest := selfdevprotocol.RouteProjectionRequest{
		ComputerID: event.ComputerID, IdempotencyKey: "route-certificate-private", Checkpoint: published,
		CanonicalEventHead: head.CanonicalEventHead, EventHeadReceiptID: receipt.ReceiptID,
		CodeClosure: checkpointCode, ArtifactProgram: checkpointProgram, ApprovalEvidence: approval, PromotionEvidence: promotion,
		Command: command, DecisionActor: "owner", DecisionScope: "computer:self_development:approve",
		ExpiresAt: now.Add(2 * time.Minute).Format(time.RFC3339Nano),
	}
	projectionCertificate, err := handler.checkpointAuthority.PublishRouteProjection(t.Context(), projectionRequest)
	if err != nil || projectionCertificate.Receipt.Verify(service.signingKey.Public) != nil || projectionCertificate.Certificate.RouteTransitionCommand != command {
		t.Fatalf("route projection certificate=%+v err=%v", projectionCertificate, err)
	}
	tampered := checkpointRequest
	tampered.ReleaseDigest = platformTestDigest('3')
	tamperedBody, _ := computerevent.CanonicalJSON(tampered)
	tamperedHTTP, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/internal/computers/checkpoints", bytes.NewReader(tamperedBody))
	tamperedHTTP.Header.Set("Authorization", "Bearer "+token)
	tamperedResponse, err := server.Client().Do(tamperedHTTP)
	if err != nil {
		t.Fatal(err)
	}
	defer tamperedResponse.Body.Close()
	if tamperedResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("checkpoint with substituted release/certificate join status=%d, want 400", tamperedResponse.StatusCode)
	}

	mismatchedEvent := event
	mismatchedEvent.EventID, err = computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	mismatchedEvent.Sequence = 2
	mismatchedEvent.PreviousHead = head.CanonicalEventHead
	mismatchedEvent.EventKind = computerevent.EventArtifactProduced
	mismatchedEvent.OccurredAt = time.Date(2026, 7, 18, 23, 30, 1, 0, time.UTC).Format(time.RFC3339Nano)
	mismatchedEvent.IdempotencyKey = "mismatched-private-envelope"
	mismatchedEvent.RequestCommitment = ""
	mismatchedPinIntent, err := computerevent.ComputePinIntentCommitment(mismatchedEvent, computerevent.TransitionInput{})
	if err != nil {
		t.Fatal(err)
	}
	mismatchedPayloadPin, err := eventClient.PinPrivatePayload(context.Background(), cipher, event.ComputerID, event.EventID, envelope, mismatchedPinIntent)
	if err != nil {
		t.Fatal(err)
	}
	mismatchedPayloadReceipt, err := mismatchedPayloadPin.Receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	mismatchedPayloadReceiptDigest := computerevent.DigestBytes(mismatchedPayloadReceipt)
	mismatchedEvent.RequestCommitment, err = computerevent.ComputeRequestCommitment(
		mismatchedEvent, computerevent.TransitionInput{}, mismatchedPinIntent, []string{mismatchedPayloadReceiptDigest},
	)
	if err != nil {
		t.Fatal(err)
	}
	mismatchedEventJSON, err := mismatchedEvent.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	mismatchedEventPin, err := eventClient.PinEvent(context.Background(), event.ComputerID, mismatchedEventJSON, mismatchedEvent.RequestCommitment)
	if err != nil {
		t.Fatal(err)
	}
	mismatchedEventReceipt, err := mismatchedEventPin.Receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.ValidateEventPins(context.Background(), computerevent.CASRequest{
		Event: mismatchedEvent, EventDigest: mismatchedEventPin.ArtifactDigest, EventArtifactDigest: mismatchedEventPin.ArtifactDigest,
		EventPinReceiptDigest:    computerevent.DigestBytes(mismatchedEventReceipt),
		PayloadPinReceiptDigests: []string{mismatchedPayloadReceiptDigest}, PinIntentCommitment: mismatchedPinIntent,
	}); err == nil {
		t.Fatal("private envelope from another event was accepted")
	}
}

func TestMintComputerCapabilityRejectsUnknownScopeAndNonCanonicalExpiry(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	capability := ComputerCapability{
		Version: 1, ComputerID: "computer-test", Scopes: []string{"event:append"},
		ExpiresAt: time.Date(2026, 7, 18, 22, 5, 0, 0, time.UTC).Format(time.RFC3339Nano),
		Nonce:     "nonce", RevocationEpoch: 0,
	}
	if _, err := MintComputerCapability(capability, privateKey); err != nil {
		t.Fatalf("valid capability was not minted: %v", err)
	}
	capability.Scopes = []string{"event:append", "host:root"}
	if _, err := MintComputerCapability(capability, privateKey); err == nil {
		t.Fatal("capability with unknown scope was minted")
	}
	capability.Scopes = []string{"event:append"}
	capability.ExpiresAt = "2026-07-18T22:05:00+00:00"
	if _, err := MintComputerCapability(capability, privateKey); err == nil {
		t.Fatal("capability with non-canonical expiry was minted")
	}
}

func TestCredentialEnvelopeIsCanonicalScopedAndDeterministicForRetry(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{signingKey: newSigningKey(privateKey)}
	issuedAt := time.Date(2026, 7, 18, 22, 0, 0, 0, time.UTC)
	expiresAt := issuedAt.Add(4 * time.Minute)
	first, err := service.buildCredentialEnvelope("computer-test", "realization-1", "issue-1", platformTestDigest('a'), 3, issuedAt, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := service.buildCredentialEnvelope("computer-test", "realization-1", "issue-1", platformTestDigest('a'), 3, issuedAt, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if first.Bearer != retry.Bearer || first.Nonce != retry.Nonce || first.Signature != retry.Signature {
		t.Fatal("idempotent credential issuance produced different credential material")
	}
	encoded, err := computerevent.CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := service.verifyCredentialEnvelope(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if verified.ComputerID != "computer-test" || verified.RealizationID != "realization-1" || verified.RevocationEpoch != 3 {
		t.Fatalf("verified envelope = %+v", verified)
	}
	first.ComputerID = "computer-other"
	tampered, err := computerevent.CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.verifyCredentialEnvelope(tampered); err == nil {
		t.Fatal("credential envelope accepted after computer identity tampering")
	}
}

func TestCredentialEnvelopeExchangeRefusesReplay(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	now := time.Now().UTC().Truncate(time.Microsecond)
	envelope, _, err := service.MintComputerCredentialEnvelope(
		context.Background(), "computer-replay", "realization-replay", "issue-replay", now.Add(4*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := computerevent.CanonicalJSON(envelope)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.exchangeComputerCredentialEnvelope(context.Background(), raw)
	if err != nil {
		t.Fatalf("first exchange refused: %v", err)
	}
	if result.Capability == "" || result.PostRevocationCapability != "" {
		t.Fatal("pre-genesis credential exchange returned an invalid capability set")
	}
	if len(result.PendingLifecycleReceipts) != 0 {
		t.Fatal("pre-genesis exchange scheduled a revocation event before an event head exists")
	}
	if replay, err := service.exchangeComputerCredentialEnvelope(context.Background(), raw); err == nil || replay.Capability != "" {
		t.Fatalf("consumed envelope replay = %#v, %v; want refusal without bearer", replay, err)
	}
	if encoded := base64.RawURLEncoding.EncodeToString(raw); encoded == "" {
		t.Fatal("canonical bootstrap encoding unavailable")
	}
}

func TestCapabilityRenewalReturnsCurrentAndPostRevocationPair(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	result, err := service.RenewComputerCapability(context.Background(), "computer-renew")
	if err != nil {
		t.Fatal(err)
	}
	decode := func(token string) ComputerCapability {
		t.Helper()
		parts := strings.Split(token, ".")
		if len(parts) != 2 {
			t.Fatalf("malformed renewed capability")
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err != nil {
			t.Fatal(err)
		}
		var capability ComputerCapability
		if err := json.Unmarshal(payload, &capability); err != nil {
			t.Fatal(err)
		}
		return capability
	}
	current := decode(result.Capability)
	next := decode(result.PostRevocationCapability)
	if current.ComputerID != "computer-renew" || next.ComputerID != current.ComputerID ||
		current.RevocationEpoch != 0 || next.RevocationEpoch != 1 ||
		current.ExpiresAt != next.ExpiresAt || current.Nonce == next.Nonce {
		t.Fatalf("renewed capability pair does not fate-share correctly: current=%+v next=%+v", current, next)
	}
}

type platformTestKeyResolver struct{ key ed25519.PublicKey }

func (r platformTestKeyResolver) ResolveReceiptKey(string, string, string, uint64, time.Time) (ed25519.PublicKey, error) {
	return r.key, nil
}

func platformTestDigest(value byte) string {
	buffer := make([]byte, 64)
	for index := range buffer {
		buffer[index] = value
	}
	return string(buffer)
}

func TestCheckpointVerifierEvidenceRequiresPinnedCoSuperPass(t *testing.T) {
	platformStore, root := openTestPlatformStore(t)
	service := NewService(platformStore, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(platformStore, "corpusd", service.computerEventSigningKey(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewCheckpointAuthority(cas, service)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 19, 7, 0, 0, 0, time.UTC)
	payload := map[string]any{
		"schema_version": 1, "operation_id": "operation-verify", "bundle_digest": platformTestDigest('a'),
		"decision": "pass", "verifier_refs": []string{"evidence:independent"}, "verifier_run_id": "run-verifier",
	}
	rawPayload, _ := computerevent.CanonicalJSON(payload)
	payloadDigest := computerevent.DigestBytes(rawPayload)
	eventID, _ := computerevent.NewEventID()
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-verify",
		Sequence: 2, PreviousHead: platformTestDigest('b'), EventKind: computerevent.EventVerificationRecorded,
		OccurredAt: now.Format(time.RFC3339Nano), IdempotencyKey: "verification-event", RequestCommitment: platformTestDigest('c'),
		TrajectoryID: "trajectory-verify", CapsuleID: "capsule-verify", ActorProfile: "co-super",
		AuthorityRef: "guest-core:self-development-verifier", OutputArtifactRefs: []string{"sha256:" + payloadDigest},
		PayloadCommitment: payloadDigest, PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: platformTestDigest('b'), ExpectedEffectiveEventHead: platformTestDigest('b'),
		ExpectedDesiredStateCommitment: platformTestDigest('f'), ExpectedEffectiveStateCommitment: platformTestDigest('f'),
	}
	rawEvent, err := event.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	eventDigest := computerevent.DigestBytes(rawEvent)
	if err := service.writeBlob(filepath.Join("sha256", "computer-event", eventDigest), rawEvent); err != nil {
		t.Fatal(err)
	}
	if err := service.writeBlob(filepath.Join("sha256", "computer-event-payload", payloadDigest), rawPayload); err != nil {
		t.Fatal(err)
	}
	if _, err := platformStore.db.Exec(`INSERT INTO computer_event_append_receipts (computer_id,idempotency_key,request_commitment,sequence,previous_head,event_kind,event_digest,event_artifact_ref,event_pin_receipt_digest,pin_receipt_digests_json,event_head_receipt_id,event_head_receipt_json,event_head_receipt_digest,desired_event_head,effective_event_head,desired_state_commitment,effective_state_commitment,pending_transition_ref,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		event.ComputerID, event.IdempotencyKey, platformTestDigest('c'), event.Sequence, event.PreviousHead, string(event.EventKind), eventDigest, "artifact://sha256/"+eventDigest, platformTestDigest('d'), "[]", "receipt-verifier", "{}", platformTestDigest('e'), eventDigest, event.PreviousHead, platformTestDigest('f'), platformTestDigest('f'), nil, now); err != nil {
		t.Fatal(err)
	}
	request := selfdevprotocol.CheckpointRequest{ComputerID: event.ComputerID, VerifierCertificate: selfdevprotocol.VerifierCertificateResponse{Request: selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: event.ComputerID, OperationID: "operation-verify", BundleDigest: platformTestDigest('a'),
		VerificationEventDigest: eventDigest, VerifierEvidenceRefs: []string{"evidence:independent"}, DecisionEventHead: platformTestDigest('1'),
		CodeRef: "code:verify", ArtifactProgramRef: "artifact:verify", ReleaseDigest: platformTestDigest('2'), Decision: "pass",
	}}}
	if err := authority.verifyVerifierEvidence(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	request.VerifierCertificate.Request.BundleDigest = platformTestDigest('9')
	if err := authority.verifyVerifierEvidence(t.Context(), request); err == nil {
		t.Fatal("verifier evidence accepted a substituted bundle")
	}
}
