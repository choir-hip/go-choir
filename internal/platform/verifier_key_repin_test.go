package platform

import (
	"bytes"
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
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// The verifier-key preflight and the owner's re-pin
// (docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md).
// Ways this could fail, written before the code:
//   - the preflight reports match for a key that the checkpoint authority
//     refuses (or the reverse), so a hold or an apply is wrong;
//   - a key id that does not name its public key is pinned;
//   - the re-pin runs without a proxy-signed owner act, or from a guest
//     caller, or in observe mode without a receipt (the guest could then
//     re-pin itself);
//   - a receipt for another key, another computer or another reason is
//     accepted, or one receipt is used twice;
//   - the re-pin replaces a pin it did not name (a concurrent change);
//   - the old pin stays active, or receipts it signed before the re-pin stop
//     verifying;
//   - the reason is not recorded.

type verifierRepinWorld struct {
	t         *testing.T
	store     *Store
	authority *CheckpointAuthority
	handler   *Handler
	signer    *owneract.Signer
}

func newVerifierRepinWorld(t *testing.T, mode OwnerActMode) *verifierRepinWorld {
	t.Helper()
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
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := owneract.NewVerifier(public)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{checkpointAuthority: authority}
	h.ConfigurePackageOwnerActs(NewOwnerActAuthority(mode, verifier))
	return &verifierRepinWorld{t: t, store: platformStore, authority: authority, handler: h, signer: signer}
}

type testVerifierKey struct {
	id     string
	public string
	key    ed25519.PublicKey
}

func newTestVerifierKey(t *testing.T) testVerifierKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return testVerifierKey{id: selfdevprotocol.VerifierKeyIDFor(public), public: base64.RawStdEncoding.EncodeToString(public), key: public}
}

// pin inserts the active pin exactly as the checkpoint authority's first-use
// branch does.
func (w *verifierRepinWorld) pin(computerID string, key testVerifierKey, sequence uint64, at time.Time) {
	w.t.Helper()
	receipt, err := computerevent.NewSignedReceipt("VerifierKeyPinned", "corpusd", map[string]any{
		"computer_id": computerID, "key_id": key.id, "public_key": key.public, "checkpoint_digest": platformTestDigest('7'),
		"repair": "legacy-import first-use pin", "accepted_event_head": platformTestDigest('8'),
	}, []computerevent.SigningKey{w.authority.cas.signingKey}, at)
	if err != nil {
		w.t.Fatal(err)
	}
	if err := w.store.insertControlKey(w.t.Context(), ControlKey{SignerDomain: selfdevprotocol.VerifierKeyDomain, ComputerID: computerID, KeyID: key.id,
		PublicKey: key.key, Status: "active", ActivationSequence: &sequence, ActivationTime: &at, AuthorizingReceipt: receipt}); err != nil {
		w.t.Fatal(err)
	}
}

func (w *verifierRepinWorld) genesis(computerID string, sequence uint64, at time.Time) {
	w.t.Helper()
	insertTestAppendReceipt(w.t, w.store.db, computerID, "genesis-"+computerID, platformTestDigest('c'), uint64(1), string(computerevent.EventGenesisImported), platformTestDigest('1'), platformTestDigest('d'), nil, at)
	if sequence > 1 {
		insertTestAppendReceipt(w.t, w.store.db, computerID, "head-"+computerID, platformTestDigest('c'), sequence, string(computerevent.EventProjectionBatchRecorded), platformTestDigest('2'), platformTestDigest('d'), nil, at)
	}
}

func (w *verifierRepinWorld) mint(request selfdevprotocol.VerifierKeyRepinRequest, owner string) string {
	w.t.Helper()
	receipt, err := w.signer.Mint(owneract.Act{Kind: owneract.ActVerifierRepin, OwnerUserID: owner, AuthMethod: owneract.AuthMethodSession,
		CredentialRef: "session:test", ComputerID: request.ComputerID, Subject: selfdevprotocol.VerifierKeyRepinSubject(request)}, time.Now())
	if err != nil {
		w.t.Fatal(err)
	}
	encoded, err := owneract.Encode(receipt)
	if err != nil {
		w.t.Fatal(err)
	}
	return encoded
}

func (w *verifierRepinWorld) post(path string, body any, internal bool, receipt string) *httptest.ResponseRecorder {
	w.t.Helper()
	raw, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	if internal {
		request.Header.Set("X-Internal-Caller", "true")
	} else {
		request.RemoteAddr = "10.200.1.2:40000"
	}
	if receipt != "" {
		request.Header.Set(owneract.Header, receipt)
	}
	recorder := httptest.NewRecorder()
	switch path {
	case verifierKeyPreflightRoute:
		w.handler.HandleVerifierKeyPreflight(recorder, request)
	case verifierKeyRepinRoute:
		w.handler.HandleVerifierKeyRepin(recorder, request)
	default:
		w.t.Fatalf("unknown route %s", path)
	}
	return recorder
}

func TestVerifierKeyPreflightAnswersThePinState(t *testing.T) {
	w := newVerifierRepinWorld(t, OwnerActEnforce)
	ctx := t.Context()
	now := time.Date(2026, 10, 26, 21, 28, 39, 0, time.UTC)
	pinned, presented := newTestVerifierKey(t), newTestVerifierKey(t)

	state := func(computerID string, key testVerifierKey) selfdevprotocol.VerifierKeyPin {
		t.Helper()
		pin, err := w.authority.VerifierKeyPreflight(ctx, selfdevprotocol.VerifierKeyPreflightRequest{ComputerID: computerID, KeyID: key.id, PublicKey: key.public})
		if err != nil {
			t.Fatal(err)
		}
		return pin
	}
	if got := state("computer-bare", presented); got.State != selfdevprotocol.VerifierKeyUnpinnedNoGenesis || !got.Refuses() {
		t.Fatalf("no pin, no genesis = %+v", got)
	}
	w.genesis("computer-new", 1, now)
	if got := state("computer-new", presented); got.State != selfdevprotocol.VerifierKeyUnpinned || got.Refuses() {
		t.Fatalf("no pin, genesis = %+v (first use pins)", got)
	}
	w.genesis("computer-de", 12744, now)
	w.pin("computer-de", pinned, 12000, now)
	if got := state("computer-de", pinned); got.State != selfdevprotocol.VerifierKeyMatch || got.PinnedKeyID != pinned.id {
		t.Fatalf("pinned key = %+v", got)
	}
	// d@e: the lost disk regenerated the key.
	if got := state("computer-de", presented); got.State != selfdevprotocol.VerifierKeyMismatch || !got.Refuses() || got.PinnedKeyID != pinned.id || got.PresentedKeyID != presented.id {
		t.Fatalf("regenerated key = %+v", got)
	}
	forged := presented
	forged.id = pinned.id
	if got := state("computer-de", forged); got.State != selfdevprotocol.VerifierKeyInvalid {
		t.Fatalf("a key id that does not name its public key = %+v", got)
	}

	// The guest route answers the same, read only.
	if rec := w.post(verifierKeyPreflightRoute, selfdevprotocol.VerifierKeyPreflightRequest{ComputerID: "computer-de", KeyID: presented.id, PublicKey: presented.public}, true, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"mismatch"`) {
		t.Fatalf("preflight route = %d %s", rec.Code, rec.Body.String())
	}
}

func TestVerifierKeyRepinIsTheOwnersActAndReplacesThePin(t *testing.T) {
	w := newVerifierRepinWorld(t, OwnerActEnforce)
	ctx := t.Context()
	pinnedAt := time.Date(2026, 10, 26, 21, 28, 39, 0, time.UTC)
	pinned, presented := newTestVerifierKey(t), newTestVerifierKey(t)
	const computerID = "computer-59c902c5d46caecc0a180a8557164e72"
	w.genesis(computerID, 12744, pinnedAt)
	w.pin(computerID, pinned, 9000, pinnedAt)

	request := selfdevprotocol.VerifierKeyRepinRequest{ComputerID: computerID, OwnerID: "owner-a", KeyID: presented.id, PublicKey: presented.public,
		ReplacesKeyID: pinned.id, Reason: "the lost-disk drills regenerated the verifier key"}

	// A guest caller is refused even with a valid receipt.
	if rec := w.post(verifierKeyRepinRoute, request, false, w.mint(request, "owner-a")); rec.Code != http.StatusForbidden {
		t.Fatalf("guest re-pin = %d %s", rec.Code, rec.Body.String())
	}
	// No receipt; a receipt for another key; another owner; another reason.
	if rec := w.post(verifierKeyRepinRoute, request, true, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("re-pin without a receipt = %d %s", rec.Code, rec.Body.String())
	}
	other := request
	other.KeyID, other.PublicKey = pinned.id, pinned.public
	if rec := w.post(verifierKeyRepinRoute, request, true, w.mint(other, "owner-a")); rec.Code != http.StatusForbidden {
		t.Fatalf("re-pin with another key's receipt = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.post(verifierKeyRepinRoute, request, true, w.mint(request, "owner-b")); rec.Code != http.StatusForbidden {
		t.Fatalf("re-pin with another owner's receipt = %d %s", rec.Code, rec.Body.String())
	}
	reworded := request
	reworded.Reason = "something else"
	if rec := w.post(verifierKeyRepinRoute, reworded, true, w.mint(request, "owner-a")); rec.Code != http.StatusForbidden {
		t.Fatalf("re-pin with a receipt for another reason = %d %s", rec.Code, rec.Body.String())
	}
	// A stale replaces_key_id is a conflict, not a re-pin.
	stale := request
	stale.ReplacesKeyID = presented.id
	if rec := w.post(verifierKeyRepinRoute, stale, true, w.mint(stale, "owner-a")); rec.Code != http.StatusConflict {
		t.Fatalf("re-pin naming the wrong pin = %d %s", rec.Code, rec.Body.String())
	}
	if pin, _ := w.authority.VerifierKeyPreflight(ctx, selfdevprotocol.VerifierKeyPreflightRequest{ComputerID: computerID, KeyID: presented.id, PublicKey: presented.public}); pin.State != selfdevprotocol.VerifierKeyMismatch {
		t.Fatalf("a refused re-pin changed the pin: %+v", pin)
	}

	receipt := w.mint(request, "owner-a")
	rec := w.post(verifierKeyRepinRoute, request, true, receipt)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner re-pin = %d %s", rec.Code, rec.Body.String())
	}
	var response selfdevprotocol.VerifierKeyRepinResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != selfdevprotocol.VerifierKeyRepinned || response.Pin.State != selfdevprotocol.VerifierKeyMatch || response.Pin.PinnedKeyID != presented.id {
		t.Fatalf("re-pin response = %+v", response)
	}
	if response.PinReceipt.ReceiptKind != selfdevprotocol.VerifierKeyPinnedReceiptKind || response.PinReceipt.KindFields["repair"] != selfdevprotocol.VerifierKeyRepinRepair ||
		response.PinReceipt.KindFields["reason"] != request.Reason || response.PinReceipt.KindFields["replaces_key_id"] != pinned.id || response.PinReceipt.KindFields["key_id"] != presented.id {
		t.Fatalf("pin receipt = %+v", response.PinReceipt.KindFields)
	}
	if err := response.PinReceipt.Verify(platformTestKeyResolver{key: w.authority.cas.signingKey.PrivateKey.Public().(ed25519.PublicKey)}); err != nil {
		t.Fatalf("pin receipt does not verify under corpusd's key: %v", err)
	}

	// The new key is the pin; the old one is revoked from the next sequence,
	// and what it signed before still verifies.
	if pin, _ := w.authority.VerifierKeyPreflight(ctx, selfdevprotocol.VerifierKeyPreflightRequest{ComputerID: computerID, KeyID: presented.id, PublicKey: presented.public}); pin.State != selfdevprotocol.VerifierKeyMatch {
		t.Fatalf("after re-pin = %+v", pin)
	}
	old, err := w.store.ControlKey(ctx, selfdevprotocol.VerifierKeyDomain, computerID, pinned.id)
	if err != nil || old.Status != "revoked" || old.ReplacementKeyID != presented.id || old.FirstInvalidSequence == nil || *old.FirstInvalidSequence != 12745 {
		t.Fatalf("old pin = %+v, %v", old, err)
	}
	resolver := ControlKeyResolver{Store: w.store}
	if _, err := resolver.ResolveReceiptKey(selfdevprotocol.VerifierKeyDomain, computerID, pinned.id, 12000, pinnedAt.Add(time.Hour)); err != nil {
		t.Fatalf("a receipt the old key signed before the re-pin stopped verifying: %v", err)
	}
	if _, err := resolver.ResolveReceiptKey(selfdevprotocol.VerifierKeyDomain, computerID, pinned.id, 12745, time.Now().Add(time.Minute)); err == nil {
		t.Fatal("the old key still resolves after the re-pin")
	}
	fresh, err := w.store.ControlKey(ctx, selfdevprotocol.VerifierKeyDomain, computerID, presented.id)
	if err != nil || fresh.Status != "active" || fresh.AuthorizingReceipt.KindFields["reason"] != request.Reason {
		t.Fatalf("new pin = %+v, %v", fresh, err)
	}
	var recorded int
	if err := w.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM owner_act_receipts WHERE computer_id=? AND act_kind=?`, computerID, owneract.ActVerifierRepin).Scan(&recorded); err != nil || recorded != 1 {
		t.Fatalf("owner act recorded %d times, %v", recorded, err)
	}

	// The same receipt again is refused (single use); a new receipt for the
	// key already pinned answers already_pinned and changes nothing.
	if rec := w.post(verifierKeyRepinRoute, request, true, receipt); rec.Code != http.StatusForbidden {
		t.Fatalf("replayed receipt = %d %s", rec.Code, rec.Body.String())
	}
	again := request
	again.ReplacesKeyID = presented.id
	rec = w.post(verifierKeyRepinRoute, again, true, w.mint(again, "owner-a"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), selfdevprotocol.VerifierKeyAlreadyPinned) {
		t.Fatalf("re-pin of the pinned key = %d %s", rec.Code, rec.Body.String())
	}
}

func TestVerifierKeyRepinRefusesWithoutAReceiptEvenInObserveMode(t *testing.T) {
	w := newVerifierRepinWorld(t, OwnerActObserve)
	now := time.Date(2026, 10, 26, 21, 28, 39, 0, time.UTC)
	pinned, presented := newTestVerifierKey(t), newTestVerifierKey(t)
	w.genesis("computer-observe", 10, now)
	w.pin("computer-observe", pinned, 5, now)
	request := selfdevprotocol.VerifierKeyRepinRequest{ComputerID: "computer-observe", OwnerID: "owner-a", KeyID: presented.id, PublicKey: presented.public, ReplacesKeyID: pinned.id, Reason: "observe"}
	if rec := w.post(verifierKeyRepinRoute, request, true, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("observe-mode re-pin without a receipt = %d %s", rec.Code, rec.Body.String())
	}
	// No pin to replace: the next verifier checkpoint pins on first use.
	request.ComputerID = "computer-unpinned"
	w.genesis("computer-unpinned", 3, now)
	if rec := w.post(verifierKeyRepinRoute, request, true, w.mint(request, "owner-a")); rec.Code != http.StatusConflict {
		t.Fatalf("re-pin of an unpinned computer = %d %s", rec.Code, rec.Body.String())
	}
}
