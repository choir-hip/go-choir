package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

type textureAuditPinner struct {
	rollbackTestPinner
	envelopes map[string][]byte
}

func (p textureAuditPinner) PreparePrivatePayload(ctx context.Context, cipher *computerevent.PrivateArtifactCipher, computerID, eventID, mediaType string, plaintext []byte) ([]byte, computerevent.PrivateArtifactMetadata, error) {
	return cipher.Encrypt(ctx, computerID, eventID, mediaType, "private", plaintext)
}

func (p textureAuditPinner) PinPrivatePayload(ctx context.Context, cipher *computerevent.PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (computerevent.PinResult, error) {
	if _, _, err := cipher.Decrypt(ctx, envelope, computerID, eventID); err != nil {
		return computerevent.PinResult{}, err
	}
	digest := computerevent.DigestBytes(envelope)
	p.envelopes[digest] = append([]byte(nil), envelope...)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "request_commitment": pinIntentCommitment,
	}, []computerevent.SigningKey{p.key}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

func TestRecordTextureAuditAppendsOneIdempotentPrivateEvent(t *testing.T) {
	ctx := context.Background()
	computerID := "computer-texture-audit"
	productStore, err := choirstore.Open(t.TempDir() + "/runtime.db")
	if err != nil {
		t.Fatal(err)
	}
	defer productStore.Close()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signingKey := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "texture-audit"}, PrivateKey: privateKey}
	pinner := textureAuditPinner{rollbackTestPinner: rollbackTestPinner{key: signingKey}, envelopes: map[string][]byte{}}
	appender, err := computerevent.NewComputerEventAppender(computerID, pinner, productStore, rollbackTestCAS{key: signingKey, projection: productStore}, rollbackTestReceiptVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	genesisID, _ := computerevent.NewEventID()
	genesis := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: genesisID, ComputerID: computerID,
		EventKind: computerevent.EventGenesisImported, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "genesis", ActorProfile: "super", AuthorityRef: "owner", PrivacyClass: "owner",
		PayloadCommitment: strings.Repeat("a", 64), ProposedEffectRef: strings.Repeat("b", 64),
		ResultingEffectiveCommitment: strings.Repeat("a", 64), ReducerVersion: computerevent.ReducerVersionV1,
	}
	if _, err := appender.AppendNew(ctx, genesis, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("a", 64)}, nil); err != nil {
		t.Fatal(err)
	}
	cipher, err := computerevent.LoadGuestPrivateArtifactCipher(t.TempDir()+"/privacy-key.json", computerID, true)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{store: productStore, eventAppender: appender, privateArtifactCipher: cipher}
	entry := TextureAuditEntry{
		Action: "revision_committed", OwnerID: "owner-texture", ComputerID: computerID,
		TrajectoryID: "trajectory-texture", DocumentID: "document-texture", RevisionID: "revision-texture",
		CommandID: "command-texture", CommandDigest: strings.Repeat("c", 64), LifecycleVersion: 2,
	}
	if err := rt.RecordTextureAudit(ctx, entry); err != nil {
		t.Fatal(err)
	}
	accepted, found, err := productStore.EventByIdempotency(ctx, computerID, textureAuditIdempotencyKey(entry))
	if err != nil || !found {
		t.Fatalf("accepted audit event = %#v, %v", accepted, err)
	}
	if accepted.EventKind != computerevent.EventLifecycleObserved || accepted.ActorProfile != "texture" || accepted.AuthorityRef != "texture:context" || accepted.PrivacyClass != "private" {
		t.Fatalf("accepted audit envelope = %#v", accepted)
	}
	if len(accepted.OutputArtifactRefs) != 1 || len(pinner.envelopes) != 1 {
		t.Fatalf("private audit artifacts = refs %#v, envelopes %d", accepted.OutputArtifactRefs, len(pinner.envelopes))
	}
	headBeforeReplay, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.RecordTextureAudit(ctx, entry); err != nil {
		t.Fatal(err)
	}
	headAfterReplay, err := productStore.Head(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if headBeforeReplay.Sequence != headAfterReplay.Sequence || headBeforeReplay.CanonicalEventHead != headAfterReplay.CanonicalEventHead {
		t.Fatalf("idempotent replay advanced head: before %#v after %#v", headBeforeReplay, headAfterReplay)
	}
	entry.CommandDigest = strings.Repeat("d", 64)
	if err := rt.RecordTextureAudit(ctx, entry); err == nil {
		t.Fatal("changed audit command was accepted")
	}
	otherOwner := entry
	otherOwner.OwnerID = "owner-texture-2"
	otherOwner.DocumentID = "document-texture-2"
	otherOwner.RevisionID = "revision-texture-2"
	otherOwner.CommandDigest = strings.Repeat("c", 64)
	if textureAuditIdempotencyKey(otherOwner) == textureAuditIdempotencyKey(entry) {
		t.Fatal("owner-scoped audit identities collided")
	}
	if err := rt.RecordTextureAudit(ctx, otherOwner); err != nil {
		t.Fatalf("same command for another owner: %v", err)
	}
	acceptedOther, found, err := productStore.EventByIdempotency(ctx, computerID, textureAuditIdempotencyKey(otherOwner))
	if err != nil || !found || acceptedOther.DecisionRef == accepted.DecisionRef {
		t.Fatalf("other owner audit event = %#v, %v", acceptedOther, err)
	}
}
