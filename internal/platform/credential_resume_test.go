package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// S3 re-credential on resume
// (docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md, "What the
// platform allows"). A snapshot resume keeps the realization (same
// realization_id and realization epoch) but the guest's frozen capability may
// be long expired. This pins the platform rules the host relies on, with no
// platform change:
//   - the boot issuance key replayed hours later is refused (its expiry derives
//     from the recorded issue time), so a resume needs a fresh key;
//   - a fresh key for the same realization is issued;
//   - the exchanged capability carries the head's current credential
//     revocation epoch and authorizes appends without any old bearer;
//   - the post-revocation capability is minted at epoch+1 (the boot protocol
//     advances the revocation epoch on consumption);
//   - consumption is one-shot: a replayed exchange is refused;
//   - an old capability past the renewal grace is refused renewal (the case
//     the resume must not depend on).
func TestResumeEnvelopeForSameRealizationIsIssuedExchangedAndConsumed(t *testing.T) {
	store, root := openTestPlatformStore(t)
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	handler := NewHandler(service)
	now := time.Now().UTC().Truncate(time.Microsecond)
	handler.eventAuth = SignedCapabilityVerifier{Store: store, PublicKey: service.signingKey.Public}
	const computerID = "computer-resume"
	const realizationID = "vm-resume-epoch-7"
	digest := platformTestDigest('b')
	if _, err := store.db.ExecContext(t.Context(), `INSERT INTO computer_event_heads (
		computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head,
		desired_state_commitment, effective_state_commitment, pending_transition_ref,
		reducer_version, credential_revocation_epoch, created_at, updated_at
	) VALUES (?, 4, ?, ?, ?, ?, ?, NULL, ?, 3, ?, ?)`,
		computerID, digest, digest, digest, digest, digest, computerevent.ReducerVersionV1, now, now,
	); err != nil {
		t.Fatal(err)
	}
	issue := func(key string) (int, ComputerCredentialEnvelope) {
		t.Helper()
		body, _ := json.Marshal(computerCredentialIssueRequest{ComputerID: computerID, RealizationID: realizationID, IdempotencyKey: key})
		request := httptest.NewRequest(http.MethodPost, "/internal/computers/credentials/issue", bytes.NewReader(body))
		request.Header.Set("X-Internal-Caller", "true")
		response := httptest.NewRecorder()
		handler.HandleComputerCredentialIssue(response, request)
		var out computerCredentialIssueResponse
		_ = json.Unmarshal(response.Body.Bytes(), &out)
		return response.Code, out.Envelope
	}
	bootKey := "guest-credential:" + realizationID + ":7"
	if code, _ := issue(bootKey); code != http.StatusCreated {
		t.Fatalf("boot issuance status=%d", code)
	}
	// The computer hibernated hours ago: move the boot issuance back in time.
	if _, err := store.db.ExecContext(t.Context(), `UPDATE computer_lifecycle_receipts SET completed_at=? WHERE computer_id=? AND idempotency_key=?`,
		now.Add(-3*time.Hour), computerID, bootKey); err != nil {
		t.Fatal(err)
	}
	if code, _ := issue(bootKey); code == http.StatusCreated {
		t.Fatal("boot issuance key replayed hours later was accepted; a resume must not rely on it")
	}

	code, envelope := issue("guest-credential-resume:" + realizationID + ":7:0123456789abcdef")
	if code != http.StatusCreated {
		t.Fatalf("fresh-key issuance for the same realization status=%d", code)
	}
	if envelope.RealizationID != realizationID || envelope.RevocationEpoch != 3 {
		t.Fatalf("resume envelope binding = realization %q epoch %d", envelope.RealizationID, envelope.RevocationEpoch)
	}
	raw, err := computerevent.CanonicalJSON(envelope)
	if err != nil {
		t.Fatal(err)
	}
	exchange := httptest.NewRecorder()
	handler.HandleComputerCredentialExchange(exchange, httptest.NewRequest(http.MethodPost, "/internal/computers/credentials/exchange", bytes.NewReader(raw)))
	if exchange.Code != http.StatusCreated {
		t.Fatalf("exchange status=%d", exchange.Code)
	}
	var result CredentialExchangeResult
	if err := json.Unmarshal(exchange.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	authorized := func(token, scope string) error {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Authorization", "Bearer "+token)
		return handler.eventAuth.Authorize(request, computerID, scope)
	}
	if err := authorized(result.Capability, "event:append"); err != nil {
		t.Fatalf("exchanged capability does not authorize appends at the current epoch: %v", err)
	}
	if err := authorized(result.PostRevocationCapability, "event:append"); err == nil {
		t.Fatal("post-revocation capability authorized before key_revoked advanced the epoch")
	}

	consume := httptest.NewRequest(http.MethodPost, "/internal/computers/credentials/consume",
		strings.NewReader(`{"computer_id":"`+computerID+`","nonce":"`+envelope.Nonce+`","request_commitment":"`+envelope.RequestCommitment+`"}`))
	consume.Header.Set("Authorization", "Bearer "+result.Capability)
	consumed := httptest.NewRecorder()
	handler.HandleComputerCredentialConsume(consumed, consume)
	if consumed.Code != http.StatusOK {
		t.Fatalf("consume status=%d body=%s", consumed.Code, consumed.Body.String())
	}
	var consumption CredentialExchangeResult
	if err := json.Unmarshal(consumed.Body.Bytes(), &consumption); err != nil {
		t.Fatal(err)
	}
	if len(consumption.PendingLifecycleReceipts) != 1 || consumption.PendingLifecycleReceipts[0].KindFields["action"] != "credential_envelope_consumed" {
		t.Fatalf("consumption must hand the guest one key_revoked receipt, got %d", len(consumption.PendingLifecycleReceipts))
	}
	replay := httptest.NewRecorder()
	handler.HandleComputerCredentialExchange(replay, httptest.NewRequest(http.MethodPost, "/internal/computers/credentials/exchange", bytes.NewReader(raw)))
	if replay.Code == http.StatusCreated {
		t.Fatal("a consumed resume envelope was exchanged again")
	}

	stale, err := MintComputerCapability(ComputerCapability{
		Version: 1, ComputerID: computerID, Scopes: []string{"event:read", "event:pin", "event:append"},
		ExpiresAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano), RevocationEpoch: 3, Nonce: "frozen",
	}, service.signingKey.Private)
	if err != nil {
		t.Fatal(err)
	}
	renew := httptest.NewRequest(http.MethodPost, "/internal/computers/credentials/renew", strings.NewReader(`{"computer_id":"`+computerID+`"}`))
	renew.Header.Set("Authorization", "Bearer "+stale)
	renewed := httptest.NewRecorder()
	handler.HandleComputerCredentialRenew(renewed, renew)
	if renewed.Code != http.StatusForbidden {
		t.Fatalf("frozen capability two hours past expiry renewal status=%d, want 403", renewed.Code)
	}
}
