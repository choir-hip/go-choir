package textureowner

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type textureAuditTestPinner struct {
	key             computerevent.SigningKey
	failPrivate     bool
	privateAttempts int
}

func (p *textureAuditTestPinner) PinEvent(_ context.Context, computerID string, canonical []byte, requestCommitment string) (computerevent.PinResult, error) {
	digest := computerevent.DigestBytes(canonical)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "request_commitment": requestCommitment,
	}, []computerevent.SigningKey{p.key}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

func (p *textureAuditTestPinner) PreparePrivatePayload(ctx context.Context, cipher *computerevent.PrivateArtifactCipher, computerID, eventID, mediaType string, plaintext []byte) ([]byte, computerevent.PrivateArtifactMetadata, error) {
	return cipher.Encrypt(ctx, computerID, eventID, mediaType, "private", plaintext)
}

func (p *textureAuditTestPinner) PinPrivatePayload(ctx context.Context, cipher *computerevent.PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (computerevent.PinResult, error) {
	p.privateAttempts++
	if p.failPrivate {
		return computerevent.PinResult{}, errors.New("injected private pin failure")
	}
	if _, _, err := cipher.Decrypt(ctx, envelope, computerID, eventID); err != nil {
		return computerevent.PinResult{}, err
	}
	digest := computerevent.DigestBytes(envelope)
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", map[string]any{
		"computer_id": computerID, "artifact_digest": digest, "request_commitment": pinIntentCommitment,
	}, []computerevent.SigningKey{p.key}, time.Now().UTC())
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, err
}

type textureAuditTestCAS struct {
	key        computerevent.SigningKey
	projection computerevent.ProjectionStore
}

func (c textureAuditTestCAS) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	return c.projection.Head(ctx, computerID)
}

func (c textureAuditTestCAS) CompareAndSwap(_ context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt("EventHeadReceipt", "corpusd", map[string]any{
		"event_digest": request.EventDigest,
	}, []computerevent.SigningKey{c.key}, time.Now().UTC())
}

type textureAuditTestReceiptVerifier struct{}

func (textureAuditTestReceiptVerifier) VerifyEventHeadReceipt(context.Context, computerevent.Receipt, computerevent.CASRequest) error {
	return nil
}

func auditedTextureAPISetup(t *testing.T, failPrivate bool) (*agentcore.Runtime, *Handler, *textureAuditTestPinner) {
	t.Helper()
	var pinner *textureAuditTestPinner
	rt, handler := testAPISetupWithOptions(t, func(productStore *store.Store, dir string) []agentcore.RuntimeOption {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signingKey := computerevent.SigningKey{
			SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "texture-audit-test"},
			PrivateKey: privateKey,
		}
		pinner = &textureAuditTestPinner{key: signingKey, failPrivate: failPrivate}
		appender, err := computerevent.NewComputerEventAppender(
			"sandbox-test",
			pinner,
			productStore,
			textureAuditTestCAS{key: signingKey, projection: productStore},
			textureAuditTestReceiptVerifier{},
		)
		if err != nil {
			t.Fatal(err)
		}
		genesisID, err := computerevent.NewEventID()
		if err != nil {
			t.Fatal(err)
		}
		genesis := computerevent.Event{
			SchemaVersion:                computerevent.SchemaVersionV1,
			EventID:                      genesisID,
			ComputerID:                   "sandbox-test",
			EventKind:                    computerevent.EventGenesisImported,
			OccurredAt:                   time.Now().UTC().Format(time.RFC3339Nano),
			IdempotencyKey:               "genesis",
			ActorProfile:                 "super",
			AuthorityRef:                 "owner",
			PrivacyClass:                 "owner",
			PayloadCommitment:            strings.Repeat("a", 64),
			ProposedEffectRef:            strings.Repeat("b", 64),
			ResultingEffectiveCommitment: strings.Repeat("a", 64),
			ReducerVersion:               computerevent.ReducerVersionV1,
		}
		if _, err := appender.AppendNew(t.Context(), genesis, computerevent.TransitionInput{TargetStateCommitment: strings.Repeat("a", 64)}, nil); err != nil {
			t.Fatal(err)
		}
		cipher, err := computerevent.LoadGuestPrivateArtifactCipher(filepath.Join(dir, "privacy-key.json"), "sandbox-test", true)
		if err != nil {
			t.Fatal(err)
		}
		return []agentcore.RuntimeOption{
			agentcore.WithComputerEventAppender(appender),
			agentcore.WithPrivateArtifactCipher(cipher),
		}
	})
	return rt, handler, pinner
}

func seedAuditDocument(t *testing.T, handler *Handler, suffix string) (types.Document, types.Revision) {
	t.Helper()
	now := time.Now().UTC().Add(-time.Minute)
	doc := types.Document{
		DocID: "audit-doc-" + suffix, OwnerID: "user-1", ComputerID: "sandbox-test",
		Title: "A", CreatedAt: now, UpdatedAt: now,
	}
	if err := handler.Store.CreateDocument(t.Context(), doc); err != nil {
		t.Fatal(err)
	}
	revision := types.Revision{
		RevisionID: "audit-revision-" + suffix, DocID: doc.DocID, OwnerID: doc.OwnerID,
		AuthorKind: types.AuthorUser, AuthorLabel: doc.OwnerID, Content: "base",
		Citations: json.RawMessage("[]"), Metadata: json.RawMessage("{}"), CreatedAt: now,
	}
	if err := handler.Store.CreateRevision(t.Context(), revision); err != nil {
		t.Fatal(err)
	}
	return doc, revision
}

func TestTextureAuditFailureDoesNotFailCommittedRevision(t *testing.T) {
	rt, handler, pinner := auditedTextureAPISetup(t, true)
	doc, base := seedAuditDocument(t, handler, "failure")
	request := textureRequest(t, http.MethodPost, "/api/texture/documents/"+doc.DocID+"/revisions", textureCreateRevisionRequest{
		Content: "committed despite audit outage", ParentRevisionID: base.RevisionID,
	})
	response := httptest.NewRecorder()
	handler.HandleTextureRevisions(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("revision status = %d body=%s", response.Code, response.Body.String())
	}
	stored, err := handler.Store.GetDocument(t.Context(), doc.DocID, doc.OwnerID)
	if err != nil || stored.CurrentRevisionID == "" || stored.CurrentRevisionID == base.RevisionID {
		t.Fatalf("committed document = %#v, %v", stored, err)
	}
	if pinner.privateAttempts != 1 {
		t.Fatalf("private audit attempts = %d, want 1", pinner.privateAttempts)
	}
	head, err := rt.Store().Head(t.Context(), rt.TextureSandboxID())
	if err != nil || head.Sequence != 1 {
		t.Fatalf("canonical head after failed audit = %#v, %v", head, err)
	}
}

func TestTextureAuditRecordsTitleCycleMergeAndRestore(t *testing.T) {
	rt, handler, pinner := auditedTextureAPISetup(t, false)
	doc, base := seedAuditDocument(t, handler, "coverage")
	assertSequence := func(want uint64) {
		t.Helper()
		head, err := rt.Store().Head(t.Context(), rt.TextureSandboxID())
		if err != nil || head.Sequence != want {
			t.Fatalf("canonical head = %#v, %v; want sequence %d", head, err, want)
		}
	}
	assertSequence(1)
	for _, title := range []string{"B", "A"} {
		request := textureRequest(t, http.MethodPut, "/api/texture/documents/"+doc.DocID, textureUpdateDocRequest{Title: title})
		response := httptest.NewRecorder()
		handler.handleTextureUpdateDocument(response, request, doc.DocID)
		if response.Code != http.StatusOK {
			t.Fatalf("title %q status = %d body=%s", title, response.Code, response.Body.String())
		}
		time.Sleep(time.Millisecond)
	}
	assertSequence(3)

	mergeRequest := textureRequest(t, http.MethodPost, "/api/texture/documents/"+doc.DocID+"/accept-merge", textureAcceptMergeRequest{
		Content: "merged", TargetRevisionID: base.RevisionID,
	})
	mergeResponse := httptest.NewRecorder()
	handler.HandleTextureAcceptMerge(mergeResponse, mergeRequest)
	if mergeResponse.Code != http.StatusCreated {
		t.Fatalf("merge status = %d body=%s", mergeResponse.Code, mergeResponse.Body.String())
	}
	assertSequence(4)

	restoreRequest := textureRequest(t, http.MethodPost, "/api/texture/documents/"+doc.DocID+"/restore", textureRestoreRevisionRequest{RevisionID: base.RevisionID})
	restoreResponse := httptest.NewRecorder()
	handler.HandleTextureRestoreRevision(restoreResponse, restoreRequest)
	if restoreResponse.Code != http.StatusCreated {
		t.Fatalf("restore status = %d body=%s", restoreResponse.Code, restoreResponse.Body.String())
	}
	assertSequence(5)
	if pinner.privateAttempts != 4 {
		t.Fatalf("private audit attempts = %d, want 4", pinner.privateAttempts)
	}
}
