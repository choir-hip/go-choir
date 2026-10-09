package platform

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/keyescrow"
)

// TestRealizationKeyDelivery locks station SH slice 3: a realization of an
// existing computer receives the escrowed key sealed to its own recipient
// key, only with the capability minted from its own (current) credential
// envelope, and only after a transparency entry is recorded.
func TestRealizationKeyDelivery(t *testing.T) {
	store, root := openTestPlatformStore(t)
	ctx := context.Background()
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	handler := NewHandler(service)
	escrowPrivate, escrowPublic, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.ConfigureKeyEscrow(escrowPrivate, ""); err != nil {
		t.Fatal(err)
	}
	handler.eventAuth = SignedCapabilityVerifier{Store: store, PublicKey: service.signingKey.Public}

	const computerID = "computer-delivery"
	seedHead := func(id string) {
		t.Helper()
		if _, err := store.db.ExecContext(ctx, `INSERT INTO computer_event_heads (computer_id,sequence,canonical_event_head,desired_event_head,effective_event_head,desired_state_commitment,effective_state_commitment,pending_transition_ref,reducer_version,credential_revocation_epoch,created_at,updated_at) VALUES (?,?,?,?,?,?,?,NULL,1,0,?,?)`,
			id, 7, strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("b", 64), time.Now(), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	seedHead(computerID)
	dek := bytes.Repeat([]byte{0x5a}, 32)
	wrapped, err := keyescrow.SealDEK(escrowPublic, computerID, dek)
	if err != nil {
		t.Fatal(err)
	}
	wrappedJSON, _ := json.Marshal(wrapped)
	if err := store.UpsertKeyEscrow(ctx, computerID, keyescrow.ProtectorCustodian, wrappedJSON, wrapped.KeyDigest); err != nil {
		t.Fatal(err)
	}

	realize := func(id, realizationID string) (ComputerCredentialEnvelope, string) {
		t.Helper()
		envelope, _, err := service.MintComputerCredentialEnvelope(ctx, id, realizationID, "issue-"+realizationID, time.Now().UTC().Add(4*time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := computerevent.CanonicalJSON(envelope)
		result, err := service.exchangeComputerCredentialEnvelope(ctx, raw)
		if err != nil {
			t.Fatal(err)
		}
		return envelope, result.Capability
	}
	recipientPrivate, recipientPublic, err := keyescrow.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	deliver := func(id, commitment, token string) (int, realizationKeyDeliveryResponse) {
		t.Helper()
		body, _ := json.Marshal(realizationKeyDeliveryRequest{
			ComputerID: id, RequestCommitment: commitment,
			RecipientPublicKey: base64.RawStdEncoding.EncodeToString(recipientPublic[:]),
		})
		response := callKeyEscrowHandler(t, handler.HandleRealizationKeyDelivery, http.MethodPost, "/internal/computers/keys/realization-delivery", body, map[string]string{"Authorization": "Bearer " + token})
		var decoded realizationKeyDeliveryResponse
		_ = json.Unmarshal(response.Body.Bytes(), &decoded)
		return response.Code, decoded
	}

	envelopeA, tokenA := realize(computerID, "realization-a")

	// Front-running: a valid capability for the same computer that was not
	// minted from this issuance cannot claim its delivery.
	foreign, err := MintComputerCapability(ComputerCapability{
		Version: 1, ComputerID: computerID, Scopes: []string{"event:read", "event:pin", "event:append"},
		ExpiresAt: time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano), Nonce: "some-other-nonce",
	}, service.signingKey.Private)
	if err != nil {
		t.Fatal(err)
	}
	if code, _ := deliver(computerID, envelopeA.RequestCommitment, foreign); code != http.StatusForbidden {
		t.Fatalf("delivery to a capability from another issuance = %d, want 403", code)
	}

	seqBefore, _, _ := store.KeyEscrowTransparencyHead(ctx)
	code, delivered := deliver(computerID, envelopeA.RequestCommitment, tokenA)
	if code != http.StatusOK {
		t.Fatalf("delivery status = %d, want 200", code)
	}
	if seqAfter, _, _ := store.KeyEscrowTransparencyHead(ctx); seqAfter != seqBefore+1 {
		t.Fatalf("delivery must append exactly one transparency entry: before=%d after=%d", seqBefore, seqAfter)
	}
	record, err := keyescrow.ParseWrappedKey([]byte(delivered.WrappedKey))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := keyescrow.OpenDEK(recipientPrivate, record, computerID)
	if err != nil || !bytes.Equal(opened, dek) || delivered.KeyDigest != wrapped.KeyDigest {
		t.Fatalf("recipient could not open the delivered key: %v digest=%s", err, delivered.KeyDigest)
	}

	// Crash-safe re-delivery with the same issuance proof is allowed.
	if code, _ := deliver(computerID, envelopeA.RequestCommitment, tokenA); code != http.StatusOK {
		t.Fatalf("re-delivery to the same issuance = %d, want 200", code)
	}

	// A later issuance supersedes realization A.
	realize(computerID, "realization-b")
	if code, _ := deliver(computerID, envelopeA.RequestCommitment, tokenA); code != http.StatusConflict {
		t.Fatalf("delivery to a superseded issuance = %d, want 409", code)
	}

	// No chain: the guest creates its own key; nothing to deliver.
	envelopeC, tokenC := realize("computer-no-chain", "realization-c")
	if code, _ := deliver("computer-no-chain", envelopeC.RequestCommitment, tokenC); code != http.StatusConflict {
		t.Fatalf("delivery before genesis = %d, want 409", code)
	}

	// No escrow: nothing to deliver.
	seedHead("computer-no-escrow")
	envelopeD, tokenD := realize("computer-no-escrow", "realization-d")
	if code, _ := deliver("computer-no-escrow", envelopeD.RequestCommitment, tokenD); code != http.StatusNotFound {
		t.Fatalf("delivery without escrow = %d, want 404", code)
	}
}
