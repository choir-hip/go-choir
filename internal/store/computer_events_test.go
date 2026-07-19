package store

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"reflect"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestComputerEventProjectionSurvivesPrepareAndFinalizeRestarts(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	productStore, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-test",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventGenesisImported,
		OccurredAt:     time.Date(2026, 7, 18, 23, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		IdempotencyKey: "genesis-test", ActorProfile: "trusted-core", AuthorityRef: "authority:test",
		PayloadCommitment: storeTestDigest('a'), PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: computerevent.ZeroHead, ExpectedEffectiveEventHead: computerevent.ZeroHead,
		ExpectedDesiredStateCommitment: computerevent.ZeroHead, ExpectedEffectiveStateCommitment: computerevent.ZeroHead,
		ResultingEffectiveCommitment: storeTestDigest('b'),
	}
	input := computerevent.TransitionInput{TargetStateCommitment: storeTestDigest('b')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	commitment, err := computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment = commitment
	next, err := computerevent.Reduce(nil, event, input)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{
		Event: event, EventDigest: digest, EventArtifactDigest: digest,
		EventPinReceiptDigest: storeTestDigest('c'), Input: input, Next: next,
		PinIntentCommitment: pinIntent,
	}
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := productStore.Close(); err != nil {
		t.Fatal(err)
	}

	productStore, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := productStore.Prepared(context.Background(), event.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 1 {
		t.Fatalf("prepared request count = %d, want 1", len(prepared))
	}
	preparedEvent, preparedErr := prepared[0].Event.CanonicalBytes()
	originalEvent, originalErr := request.Event.CanonicalBytes()
	if preparedErr != nil || originalErr != nil || !bytes.Equal(preparedEvent, originalEvent) || prepared[0].EventDigest != request.EventDigest || prepared[0].EventArtifactDigest != request.EventArtifactDigest || prepared[0].EventPinReceiptDigest != request.EventPinReceiptDigest || !reflect.DeepEqual(prepared[0].Input, request.Input) || !reflect.DeepEqual(prepared[0].Next, request.Next) {
		t.Fatalf("prepared request after restart = %+v", prepared[0])
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{"computer_id": event.ComputerID, "event_digest": digest, "sequence": uint64(1)}, []computerevent.SigningKey{{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}}, time.Date(2026, 7, 18, 23, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.Finalize(context.Background(), event.ComputerID, digest, receipt); err != nil {
		t.Fatal(err)
	}
	if err := productStore.Close(); err != nil {
		t.Fatal(err)
	}

	productStore, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	head, err := productStore.Head(context.Background(), event.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(head, &next) {
		t.Fatalf("finalized head after restart = %+v, want %+v", head, next)
	}
	prepared, err = productStore.Prepared(context.Background(), event.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) != 0 {
		t.Fatalf("finalized event remained prepared: %+v", prepared)
	}
}

func TestComputerEventProjectionPersistsRevocationEpochAcrossRestart(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	productStore, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	initial := computerevent.Head{
		ComputerID: "computer-revocation", Sequence: 1, CanonicalEventHead: storeTestDigest('a'),
		DesiredEventHead: storeTestDigest('a'), EffectiveEventHead: storeTestDigest('a'),
		DesiredStateCommitment: storeTestDigest('b'), EffectiveStateCommitment: storeTestDigest('b'),
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := productStore.db.Exec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, initial.ComputerID, initial.Sequence, initial.CanonicalEventHead, initial.DesiredEventHead, initial.EffectiveEventHead, initial.DesiredStateCommitment, initial.EffectiveStateCommitment, initial.ReducerVersion, initial.CredentialRevocationEpoch, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: initial.ComputerID,
		Sequence: 2, PreviousHead: initial.CanonicalEventHead, EventKind: computerevent.EventKeyRevoked,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "revoke-test",
		ActorProfile: "trusted-core", AuthorityRef: "authority:test", PayloadCommitment: storeTestDigest('c'),
		PrivacyClass: "private", ReducerVersion: computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: initial.DesiredEventHead, ExpectedEffectiveEventHead: initial.EffectiveEventHead,
		ExpectedDesiredStateCommitment: initial.DesiredStateCommitment, ExpectedEffectiveStateCommitment: initial.EffectiveStateCommitment,
	}
	input := computerevent.TransitionInput{}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	next, err := computerevent.Reduce(&initial, event, input)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{
		Event: event, EventDigest: digest, EventArtifactDigest: digest,
		EventPinReceiptDigest: storeTestDigest('d'), Input: input, Next: next, PinIntentCommitment: pinIntent,
	}
	if err := productStore.Prepare(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{"computer_id": event.ComputerID, "event_digest": digest, "sequence": uint64(2)}, []computerevent.SigningKey{{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "test"}, PrivateKey: privateKey}}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := productStore.Finalize(context.Background(), event.ComputerID, digest, receipt); err != nil {
		t.Fatal(err)
	}
	if err := productStore.Close(); err != nil {
		t.Fatal(err)
	}
	productStore, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	restarted, err := productStore.Head(context.Background(), event.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restarted, &next) || restarted.CredentialRevocationEpoch != 1 {
		t.Fatalf("revoked head after restart = %+v, want %+v", restarted, next)
	}
}

func storeTestDigest(value byte) string {
	buffer := make([]byte, 64)
	for index := range buffer {
		buffer[index] = value
	}
	return string(buffer)
}
