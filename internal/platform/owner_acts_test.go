package platform

import (
	"bytes"
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

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// S1: owner-act receipts on mode writes and the effect_accepted admission
// gate (docs/problems/effect-accepted-admitted-without-authority-2026-10-15.md).
// The failure-mode numbers in comments are that record's.

func newTestOwnerActSigner(t *testing.T) *owneract.Signer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func testOwnerActAuthority(t *testing.T, mode OwnerActMode, signer *owneract.Signer) *OwnerActAuthority {
	t.Helper()
	var verifier *owneract.Verifier
	if signer != nil {
		var err error
		verifier, err = owneract.NewVerifier(signer.PublicKey())
		if err != nil {
			t.Fatal(err)
		}
	}
	return NewOwnerActAuthority(mode, verifier)
}

// mintModeAct signs what the proxy signs for a mode write.
func mintModeAct(t *testing.T, signer *owneract.Signer, computerID, owner string, request SetSelfDevelopmentModeRequest, now time.Time) computerevent.Receipt {
	t.Helper()
	subject, err := OwnerActModeSubject(computerID, request)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := signer.Mint(owneract.Act{
		Kind: OwnerActKindForMode(request.Mode), OwnerUserID: owner, AuthMethod: owneract.AuthMethodAPIKey,
		CredentialRef: "api-key:test", ComputerID: computerID, Subject: subject,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

func encodeAct(t *testing.T, receipt computerevent.Receipt) string {
	t.Helper()
	value, err := owneract.Encode(receipt)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

// ownerWith posts as the proxy does, carrying the given receipt header.
func (f *modeAuthorityFixture) ownerWith(t *testing.T, request SetSelfDevelopmentModeRequest, header string) (int, SelfDevelopmentMode, string) {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/mode?computer_id="+f.computerID, bytes.NewReader(body))
	r.RemoteAddr = "127.0.0.1:50000"
	r.Header.Set("X-Internal-Caller", "true")
	r.Header.Set("X-Authenticated-User", "owner-1")
	if header != "" {
		r.Header.Set(owneract.Header, header)
	}
	w := httptest.NewRecorder()
	f.handler.HandleSelfDevelopmentMode(w, r)
	var mode SelfDevelopmentMode
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &mode); err != nil {
			t.Fatalf("decode mode: %v: %s", err, w.Body.String())
		}
	}
	return w.Code, mode, w.Body.String()
}

func TestOwnerActModeWriteEnforceRefusesMissingForgedExpiredAndReplayed(t *testing.T) {
	f := newModeAuthorityFixture(t)
	signer := newTestOwnerActSigner(t)
	f.modes.ConfigureOwnerActs(testOwnerActAuthority(t, OwnerActEnforce, signer))
	arm := f.arm(SelfDevelopmentModeAcceptOnce, "op-1", 0, "arm-1")
	unchanged := func(label string) {
		t.Helper()
		if mode := f.current(t); mode.Mode != SelfDevelopmentModeOff || mode.Generation != 0 {
			t.Fatalf("%s changed the mode: %+v", label, mode)
		}
	}

	// 1: no receipt (a loopback curl with the hostauth header).
	if code, _, body := f.ownerWith(t, arm, ""); code != http.StatusForbidden {
		t.Fatalf("no receipt: %d %s", code, body)
	}
	unchanged("no receipt")

	// 2: unsigned (no signature at all).
	unsigned := mintModeAct(t, signer, f.computerID, "owner-1", arm, time.Now())
	unsigned.RequiredSigners, unsigned.SignatureSet = nil, nil
	if code, _, body := f.ownerWith(t, arm, encodeAct(t, unsigned)); code != http.StatusForbidden {
		t.Fatalf("unsigned receipt: %d %s", code, body)
	}
	// 2: forged under the pinned key id with another private key.
	_, otherKey, _ := ed25519.GenerateKey(rand.Reader)
	subject, _ := OwnerActModeSubject(f.computerID, arm)
	forged, err := computerevent.NewSignedReceipt(owneract.ReceiptKind, owneract.Issuer, map[string]any{
		"act_kind": owneract.ActSelfDevApprove, "owner_user_id": "owner-1", "auth_method": "session",
		"credential_ref": "session-jwt-sha256:x", "audience": owneract.Audience, "computer_id": f.computerID,
		"nonce": strings.Repeat("ab", 16), "not_after": time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano),
		"subject": subject,
	}, []computerevent.SigningKey{{SignerRef: computerevent.SignerRef{SignerDomain: owneract.SignerDomain, KeyID: signer.KeyID()}, PrivateKey: otherKey}}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if code, _, body := f.ownerWith(t, arm, encodeAct(t, forged)); code != http.StatusForbidden {
		t.Fatalf("forged receipt: %d %s", code, body)
	}
	// 2: a different (unpinned) proxy key.
	if code, _, body := f.ownerWith(t, arm, encodeAct(t, mintModeAct(t, newTestOwnerActSigner(t), f.computerID, "owner-1", arm, time.Now()))); code != http.StatusForbidden {
		t.Fatalf("unpinned key: %d %s", code, body)
	}
	// 3: another computer, and a subject that is not this write.
	if code, _, body := f.ownerWith(t, arm, encodeAct(t, mintModeAct(t, signer, "computer-other", "owner-1", arm, time.Now()))); code != http.StatusForbidden {
		t.Fatalf("other computer: %d %s", code, body)
	}
	other := f.arm(SelfDevelopmentModeAcceptOnce, "op-other", 0, "arm-1")
	if code, _, body := f.ownerWith(t, arm, encodeAct(t, mintModeAct(t, signer, f.computerID, "owner-1", other, time.Now()))); code != http.StatusForbidden {
		t.Fatalf("other subject: %d %s", code, body)
	}
	// 4: expired.
	if code, _, body := f.ownerWith(t, arm, encodeAct(t, mintModeAct(t, signer, f.computerID, "owner-1", arm, time.Now().Add(-6*time.Minute)))); code != http.StatusForbidden {
		t.Fatalf("expired receipt: %d %s", code, body)
	}
	unchanged("refused receipts")

	// 16: the proxy's receipt over this exact write arms, and the
	// ModeReceipt names it.
	valid := mintModeAct(t, signer, f.computerID, "owner-1", arm, time.Now())
	code, armed, body := f.ownerWith(t, arm, encodeAct(t, valid))
	if code != http.StatusOK || armed.Mode != SelfDevelopmentModeAcceptOnce {
		t.Fatalf("valid receipt: %d %s", code, body)
	}
	validBytes, _ := valid.CanonicalBytes()
	if armed.Receipt.KindFields["owner_act_receipt_digest"] != computerevent.DigestBytes(validBytes) ||
		armed.Receipt.KindFields["owner_user_id"] != "owner-1" || armed.Receipt.KindFields["owner_auth_method"] != "api_key" {
		t.Fatalf("ModeReceipt does not name the owner act: %#v", armed.Receipt.KindFields)
	}
	// 18: an identical retry returns the stored mode.
	if code, retried, body := f.ownerWith(t, arm, encodeAct(t, mintModeAct(t, signer, f.computerID, "owner-1", arm, time.Now()))); code != http.StatusOK || retried.Generation != 1 {
		t.Fatalf("idempotent retry: %d %s", code, body)
	}
	// Move on, then replay the first receipt with its own request (5).
	off := SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeOff, ExpectedGeneration: 1, IdempotencyKey: "off-1"}
	if code, _, body := f.ownerWith(t, off, encodeAct(t, mintModeAct(t, signer, f.computerID, "owner-1", off, time.Now()))); code != http.StatusOK {
		t.Fatalf("owner off: %d %s", code, body)
	}
	if code, _, body := f.ownerWith(t, arm, encodeAct(t, valid)); code != http.StatusForbidden || !strings.Contains(body, "already used") {
		t.Fatalf("replayed receipt: %d %s", code, body)
	}
	if mode := f.current(t); mode.Mode != SelfDevelopmentModeOff || mode.Generation != 2 {
		t.Fatalf("replay changed the mode: %+v", mode)
	}

	// 17: the guest's consumption needs no receipt; 10: expiry neither.
	arm3 := f.arm(SelfDevelopmentModeAcceptOnce, "op-3", 2, "arm-3")
	if code, _, body := f.ownerWith(t, arm3, encodeAct(t, mintModeAct(t, signer, f.computerID, "owner-1", arm3, time.Now()))); code != http.StatusOK {
		t.Fatalf("arm 3: %d %s", code, body)
	}
	if code, consumed := f.guest(t, consume(AcceptOnceConsumptionKeyPrefix, "op-3", 3)); code != http.StatusOK || modeWriter(consumed) != ModeWriterGuestConsumption {
		t.Fatalf("guest consumption: %d %+v", code, consumed)
	}
}

func TestOwnerActModeWriteEnforceWithoutKeyRefusesEveryOwnerWrite(t *testing.T) {
	f := newModeAuthorityFixture(t)
	f.modes.ConfigureOwnerActs(testOwnerActAuthority(t, OwnerActEnforce, nil))
	arm := f.arm(SelfDevelopmentModeAcceptOnce, "op-1", 0, "arm-1")
	// 6: enforce with no public key configured.
	if code, _, body := f.ownerWith(t, arm, encodeAct(t, mintModeAct(t, newTestOwnerActSigner(t), f.computerID, "owner-1", arm, time.Now()))); code != http.StatusForbidden {
		t.Fatalf("no key configured: %d %s", code, body)
	}
}

func TestOwnerActModeWriteObserveLogsAndAllows(t *testing.T) {
	f := newModeAuthorityFixture(t)
	signer := newTestOwnerActSigner(t)
	f.modes.ConfigureOwnerActs(testOwnerActAuthority(t, OwnerActObserve, signer))
	arm := f.arm(SelfDevelopmentModeAcceptOnce, "op-1", 0, "arm-1")
	code, armed, body := f.ownerWith(t, arm, "")
	if code != http.StatusOK || armed.Mode != SelfDevelopmentModeAcceptOnce {
		t.Fatalf("observe without receipt: %d %s", code, body)
	}
	if _, named := armed.Receipt.KindFields["owner_act_receipt_digest"]; named {
		t.Fatalf("observe recorded an owner act it never verified: %#v", armed.Receipt.KindFields)
	}
	// A valid receipt is recorded in observe too.
	off := SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeOff, ExpectedGeneration: 1, IdempotencyKey: "off-1"}
	code, offMode, body := f.ownerWith(t, off, encodeAct(t, mintModeAct(t, signer, f.computerID, "owner-1", off, time.Now())))
	if code != http.StatusOK || offMode.Receipt.KindFields["owner_act_receipt_digest"] == nil {
		t.Fatalf("observe with receipt: %d %s", code, body)
	}
}

// --- the effect_accepted admission gate ---

type allowPins struct{}

func (allowPins) ValidateEventPins(context.Context, computerevent.CASRequest) error { return nil }

type admissionFixture struct {
	*modeAuthorityFixture
	cas    *ComputerEventCAS
	signer *owneract.Signer
	store  *Store
}

func newAdmissionFixture(t *testing.T, mode OwnerActMode) *admissionFixture {
	t.Helper()
	f := newModeAuthorityFixture(t)
	signer := newTestOwnerActSigner(t)
	authority := testOwnerActAuthority(t, mode, signer)
	f.modes.ConfigureOwnerActs(authority)
	store := f.modes.store
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cas, err := NewComputerEventCAS(store, "corpusd", computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "admission-test"}, PrivateKey: key,
	}, allowPins{})
	if err != nil {
		t.Fatal(err)
	}
	cas.ConfigureEffectAdmission(authority)
	return &admissionFixture{modeAuthorityFixture: f, cas: cas, signer: signer, store: store}
}

func digestOf(b byte) string { return strings.Repeat(string([]byte{b}), 64) }

// seedHead gives a computer a current head with no pending transition.
func (f *admissionFixture) seedHead(t *testing.T, computerID string) computerevent.Head {
	t.Helper()
	head := computerevent.Head{
		ComputerID: computerID, Sequence: 1, CanonicalEventHead: digestOf('1'),
		DesiredEventHead: digestOf('1'), EffectiveEventHead: digestOf('1'),
		DesiredStateCommitment: digestOf('2'), EffectiveStateCommitment: digestOf('2'),
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	now := time.Now().UTC()
	if _, err := f.store.db.Exec(`INSERT INTO computer_event_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, pending_transition_ref, reducer_version, credential_revocation_epoch, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, 0, ?, ?)`,
		head.ComputerID, head.Sequence, head.CanonicalEventHead, head.DesiredEventHead, head.EffectiveEventHead, head.DesiredStateCommitment, head.EffectiveStateCommitment, head.ReducerVersion, now, now); err != nil {
		t.Fatal(err)
	}
	return head
}

// accepted builds an effect_accepted at the head and the CAS request a
// guest would send.
func accepted(t *testing.T, head computerevent.Head, authority, operationID, bundle string, inputs []string, decisionRef string) computerevent.CASRequest {
	t.Helper()
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: head.ComputerID,
		Sequence: head.Sequence + 1, PreviousHead: head.CanonicalEventHead, EventKind: computerevent.EventEffectAccepted,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "accept-" + eventID,
		ParentEventID: operationID, ActorProfile: "management", AuthorityRef: authority, PrivacyClass: "owner",
		InputArtifactRefs: inputs, PayloadCommitment: computerevent.ZeroHead,
		ProposedEffectRef: bundle, DecisionRef: decisionRef, VerifierRefs: []string{"verifier:test"},
		ReducerVersion:           computerevent.ReducerVersionV1,
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedPendingTransitionRef:   head.PendingTransitionRef,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		RequireExpectedHead: true,
	}
	input := computerevent.TransitionInput{TargetStateCommitment: digestOf('3')}
	pinIntent, err := computerevent.ComputePinIntentCommitment(event, input)
	if err != nil {
		t.Fatal(err)
	}
	event.RequestCommitment, err = computerevent.ComputeRequestCommitment(event, input, pinIntent, nil)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	next, err := computerevent.Reduce(&head, event, input)
	if err != nil {
		t.Fatal(err)
	}
	return computerevent.CASRequest{
		Event: event, EventDigest: digest, EventArtifactDigest: digest, EventPinReceiptDigest: digestOf('9'),
		PinIntentCommitment: pinIntent, Input: input, Next: next,
	}
}

// armAndConsume runs the owner's approval as the tracer does: the proxy's
// receipt arms accept_once at the head, then the guest consumes it. It
// returns the consumed ModeReceipt's artifact reference.
func (f *admissionFixture) armAndConsume(t *testing.T, head computerevent.Head, operationID, bundle string, withReceipt bool) string {
	t.Helper()
	pending := head.PendingTransitionRef
	current := f.current(t)
	arm := SetSelfDevelopmentModeRequest{
		Mode: SelfDevelopmentModeAcceptOnce, ExpectedGeneration: current.Generation, IdempotencyKey: "arm-" + operationID,
		OperationID: operationID, BundleDigest: bundle,
		ExpectedDesiredEventHead: head.DesiredEventHead, ExpectedEffectiveEventHead: head.EffectiveEventHead,
		ExpectedPendingTransitionRef:   &pending,
		ExpectedDesiredStateCommitment: head.DesiredStateCommitment, ExpectedEffectiveStateCommitment: head.EffectiveStateCommitment,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano),
	}
	header := ""
	if withReceipt {
		header = encodeAct(t, mintModeAct(t, f.signer, f.computerID, "owner-1", arm, time.Now()))
	}
	if code, _, body := f.ownerWith(t, arm, header); code != http.StatusOK {
		t.Fatalf("arm %s: %d %s", operationID, code, body)
	}
	code, consumed := f.guest(t, consume(AcceptOnceConsumptionKeyPrefix, operationID, current.Generation+1))
	if code != http.StatusOK {
		t.Fatalf("consume %s: %d", operationID, code)
	}
	raw, err := consumed.Receipt.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	return "artifact:sha256:" + computerevent.DigestBytes(raw)
}

func (f *admissionFixture) append(t *testing.T, request computerevent.CASRequest) error {
	t.Helper()
	_, err := f.cas.CompareAndSwap(context.Background(), request)
	return err
}

func refused(t *testing.T, label string, err error) {
	t.Helper()
	if !errors.Is(err, ErrEffectAdmissionRefused) {
		t.Fatalf("%s: want admission refusal, got %v", label, err)
	}
}

func TestEffectAdmissionEnforceTracesOwnerApprovalToTheProxyReceipt(t *testing.T) {
	f := newAdmissionFixture(t, OwnerActEnforce)
	head := f.seedHead(t, f.computerID)
	bundle := digestOf('b')
	consumedRef := f.armAndConsume(t, head, "op-1", bundle, true)

	// 7: a ModeReceipt corpusd never issued.
	refused(t, "fabricated ModeReceipt", f.append(t, accepted(t, head, "external-owner:owner-1", "op-1", bundle, []string{"artifact:sha256:" + digestOf('f')}, digestOf('d'))))
	// 15: no ModeReceipt at all, and an unknown authority.
	refused(t, "no input", f.append(t, accepted(t, head, "external-owner:owner-1", "op-1", bundle, nil, digestOf('d'))))
	refused(t, "unknown authority", f.append(t, accepted(t, head, "guest:self", "op-1", bundle, []string{consumedRef}, digestOf('d'))))
	// 9: the consumption is for op-1; the event claims op-2, or another bundle.
	refused(t, "other operation", f.append(t, accepted(t, head, "external-owner:owner-1", "op-2", bundle, []string{consumedRef}, digestOf('d'))))
	refused(t, "other bundle", f.append(t, accepted(t, head, "external-owner:owner-1", "op-1", digestOf('c'), []string{consumedRef}, digestOf('d'))))
	// 11: the arm's owner act was owner-1's.
	refused(t, "other owner", f.append(t, accepted(t, head, "external-owner:owner-2", "op-1", bundle, []string{consumedRef}, digestOf('d'))))
	// 13: consensus authority until S2.
	refused(t, "qualified consensus", f.append(t, accepted(t, head, "qualified-consensus:"+digestOf('e'), "op-1", bundle, []string{consumedRef}, digestOf('d'))))
	// 8: citing the arm itself rather than its consumption.
	var armDigest string
	if err := f.store.db.QueryRow(`SELECT receipt_digest FROM self_development_mode_receipts WHERE computer_id=? AND new_mode=?`, f.computerID, SelfDevelopmentModeAcceptOnce).Scan(&armDigest); err != nil {
		t.Fatal(err)
	}
	refused(t, "arm as input", f.append(t, accepted(t, head, "external-owner:owner-1", "op-1", bundle, []string{"artifact:sha256:" + armDigest}, digestOf('d'))))

	// 19: the owner's own approval is admitted and marks the consumption.
	good := accepted(t, head, "external-owner:owner-1", "op-1", bundle, []string{consumedRef}, digestOf('d'))
	if err := f.append(t, good); err != nil {
		t.Fatalf("owner approval refused: %v", err)
	}
	var admitted string
	if err := f.store.db.QueryRow(`SELECT COALESCE(admitted_event_digest, '') FROM self_development_mode_receipts WHERE receipt_digest=?`, strings.TrimPrefix(consumedRef, "artifact:sha256:")).Scan(&admitted); err != nil || admitted != good.EventDigest {
		t.Fatalf("consumption not marked: %q %v", admitted, err)
	}
	// 22: an idempotent retry of the admitted append returns its receipt.
	if err := f.append(t, good); err != nil {
		t.Fatalf("idempotent retry refused: %v", err)
	}
	// 12: a second event citing the same consumption.
	tx, err := f.store.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	second := accepted(t, head, "external-owner:owner-1", "op-1", bundle, []string{consumedRef}, digestOf('d'))
	reason, _, err := f.cas.ownerActs.effectAcceptedRefusal(context.Background(), tx, second.Event, second.EventDigest)
	if err != nil || !strings.Contains(reason, "already admitted") {
		t.Fatalf("second admission of one consumption: %q %v", reason, err)
	}
}

func TestEffectAdmissionEnforceRefusesAnArmWithoutAnOwnerAct(t *testing.T) {
	f := newAdmissionFixture(t, OwnerActEnforce)
	head := f.seedHead(t, f.computerID)
	bundle := digestOf('b')
	// 10: an arm made while observe let a receipt-less write through.
	f.modes.ConfigureOwnerActs(testOwnerActAuthority(t, OwnerActObserve, f.signer))
	consumedRef := f.armAndConsume(t, head, "op-1", bundle, false)
	refused(t, "arm without owner act", f.append(t, accepted(t, head, "external-owner:owner-1", "op-1", bundle, []string{consumedRef}, digestOf('d'))))
}

func TestEffectAdmissionPlatformUpdateNeedsAnOfferCorpusdMinted(t *testing.T) {
	f := newAdmissionFixture(t, OwnerActEnforce)
	head := f.seedHead(t, f.computerID)
	content := digestOf('c')
	offerDigest := digestOf('0')
	// 14: an offer corpusd never minted.
	refused(t, "unminted offer", f.append(t, accepted(t, head, AuthorityPlatformUpdate, "", content, nil, offerDigest)))
	offer := selfdevprotocol.PlatformUpdateOffer{ComputerID: f.computerID, UpdateID: "u-1", BaseEventHead: head.CanonicalEventHead,
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339Nano)}
	offer.Manifest.ContentDigest = content
	if err := recordPlatformUpdateOfferIssued(context.Background(), f.store.db, offer, offerDigest, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	other := offer
	other.ComputerID = "computer-other"
	if err := recordPlatformUpdateOfferIssued(context.Background(), f.store.db, other, digestOf('5'), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	// 14: minted for another computer; for another content digest.
	refused(t, "other computer's offer", f.append(t, accepted(t, head, AuthorityPlatformUpdate, "", content, nil, digestOf('5'))))
	refused(t, "other content", f.append(t, accepted(t, head, AuthorityPlatformUpdate, "", digestOf('a'), nil, offerDigest)))
	// 20: a minted offer still applies.
	if err := f.append(t, accepted(t, head, AuthorityPlatformUpdate, "", content, nil, offerDigest)); err != nil {
		t.Fatalf("platform update refused: %v", err)
	}
}

func TestEffectAdmissionObserveLogsAndAdmits(t *testing.T) {
	f := newAdmissionFixture(t, OwnerActObserve)
	head := f.seedHead(t, f.computerID)
	// A guest-appended acceptance with no traceable authority is only logged.
	if err := f.append(t, accepted(t, head, "external-owner:x", "op-1", digestOf('b'), []string{"artifact:sha256:" + digestOf('f')}, digestOf('d'))); err != nil {
		t.Fatalf("observe refused: %v", err)
	}
}

func TestEffectAdmissionRefusalIsA403AtTheAppendRoute(t *testing.T) {
	f := newAdmissionFixture(t, OwnerActEnforce)
	head := f.seedHead(t, f.computerID)
	service := NewService(f.store, filepath.Join(t.TempDir(), "artifacts"), filepath.Join(t.TempDir(), "platform-signing.key"))
	artifacts, err := NewEventArtifactService(service, platformTestKeyResolver{key: service.signingKey.Public})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.handler.ConfigureComputerEvents(f.cas, artifacts, f.handler.eventAuth); err != nil {
		t.Fatal(err)
	}
	request := accepted(t, head, "external-owner:x", "op-1", digestOf('b'), []string{"artifact:sha256:" + digestOf('f')}, digestOf('d'))
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/internal/computers/events/append", bytes.NewReader(body))
	r.RemoteAddr = "172.16.0.2:40000"
	r.Header.Set("Authorization", "Bearer "+f.guestToken)
	w := httptest.NewRecorder()
	f.handler.HandleComputerEventAppend(w, r)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "admission refused") {
		t.Fatalf("guest-appended effect_accepted with no authority: %d %s", w.Code, w.Body.String())
	}
}
