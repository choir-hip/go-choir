package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// P3: only an owner act may set the self-development mode
// (docs/archive/problems/guest-can-arm-own-selfdev-mode-2026-10-15.md). The guest's
// event:append capability may only consume the grant the owner armed.

type modeAuthorityFixture struct {
	handler    *Handler
	modes      *SelfDevelopmentModeCAS
	computerID string
	guestToken string
	now        time.Time
}

func newModeAuthorityFixture(t *testing.T) *modeAuthorityFixture {
	t.Helper()
	store, root := openTestPlatformStore(t)
	t.Cleanup(func() { _ = store.Close() })
	service := NewService(store, filepath.Join(root, "artifacts"), filepath.Join(root, "platform-signing.key"))
	handler := NewHandler(service)
	handler.eventAuth = SignedCapabilityVerifier{Store: store, PublicKey: service.signingKey.Public}
	_, modeKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	modes, err := NewSelfDevelopmentModeCAS(store, computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: "platform-control", KeyID: "mode-owner-test"}, PrivateKey: modeKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.ConfigureSelfDevelopmentModes(modes); err != nil {
		t.Fatal(err)
	}
	f := &modeAuthorityFixture{handler: handler, modes: modes, computerID: "computer-mode-owner", now: time.Now().UTC()}
	// The exact scope set every guest capability carries
	// (credential_envelope.go: exchange, post-revocation, renewal).
	f.guestToken, err = MintComputerCapability(ComputerCapability{
		Version: 1, ComputerID: f.computerID, Scopes: []string{"event:read", "event:pin", "event:append"},
		ExpiresAt: f.now.Add(10 * time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano), Nonce: "guest-nonce",
	}, service.signingKey.Private)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// guest posts from a guest tap address with the guest bearer and no
// host-internal marker: what a computer can send corpusd directly.
func (f *modeAuthorityFixture) guest(t *testing.T, request SetSelfDevelopmentModeRequest) (int, SelfDevelopmentMode) {
	t.Helper()
	return f.post(t, request, func(r *http.Request) {
		r.RemoteAddr = "172.16.0.2:40000"
		r.Header.Set("Authorization", "Bearer "+f.guestToken)
	})
}

// owner posts as the proxy does after it has authenticated the owner's
// session or scoped API key: a host-internal caller.
func (f *modeAuthorityFixture) owner(t *testing.T, request SetSelfDevelopmentModeRequest) (int, SelfDevelopmentMode) {
	t.Helper()
	return f.post(t, request, func(r *http.Request) {
		r.RemoteAddr = "127.0.0.1:50000"
		r.Header.Set("X-Internal-Caller", "true")
		r.Header.Set("X-Authenticated-User", "owner-1")
	})
}

func (f *modeAuthorityFixture) post(t *testing.T, request SetSelfDevelopmentModeRequest, as func(*http.Request)) (int, SelfDevelopmentMode) {
	t.Helper()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/internal/computers/self-development/mode?computer_id="+f.computerID, bytes.NewReader(body))
	as(r)
	w := httptest.NewRecorder()
	f.handler.HandleSelfDevelopmentMode(w, r)
	var mode SelfDevelopmentMode
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &mode); err != nil {
			t.Fatalf("decode mode: %v: %s", err, w.Body.String())
		}
	}
	return w.Code, mode
}

func (f *modeAuthorityFixture) current(t *testing.T) SelfDevelopmentMode {
	t.Helper()
	mode, err := f.modes.Get(context.Background(), f.computerID)
	if err != nil {
		t.Fatal(err)
	}
	return mode
}

func (f *modeAuthorityFixture) arm(mode, operationID string, generation uint64, key string) SetSelfDevelopmentModeRequest {
	digest := strings.Repeat("a", 64)
	pending := ""
	request := SetSelfDevelopmentModeRequest{
		Mode: mode, ExpectedGeneration: generation, IdempotencyKey: key,
		OperationID: operationID, BundleDigest: digest,
		ExpectedDesiredEventHead: digest, ExpectedEffectiveEventHead: digest, ExpectedPendingTransitionRef: &pending,
		ExpectedDesiredStateCommitment: digest, ExpectedEffectiveStateCommitment: digest,
		ExpiresAt: f.now.Add(5 * time.Minute).Truncate(time.Microsecond).Format(time.RFC3339Nano),
	}
	if mode == SelfDevelopmentModeQualifiedConsensus {
		request.PolicyDigest = strings.Repeat("b", 64)
		request.ConsensusReceiptDigest = strings.Repeat("c", 64)
	}
	return request
}

func consume(prefix, operationID string, generation uint64) SetSelfDevelopmentModeRequest {
	return SetSelfDevelopmentModeRequest{
		Mode: SelfDevelopmentModeProposeOnly, ExpectedGeneration: generation,
		IdempotencyKey: prefix + operationID + ":" + strings.Repeat("d", 64),
	}
}

func modeWriter(mode SelfDevelopmentMode) string {
	if mode.Receipt == nil {
		return ""
	}
	value, _ := mode.Receipt.KindFields["mode_writer"].(string)
	return value
}

func TestGuestCapabilityCannotSetSelfDevelopmentMode(t *testing.T) {
	f := newModeAuthorityFixture(t)
	// Failure modes 1-3 and 4 from mode off.
	for name, request := range map[string]SetSelfDevelopmentModeRequest{
		"arm accept_once":         f.arm(SelfDevelopmentModeAcceptOnce, "op-guest", 0, "guest-arm-once"),
		"arm qualified_consensus": f.arm(SelfDevelopmentModeQualifiedConsensus, "op-guest", 0, "guest-arm-qc"),
		"raise to propose_only":   {Mode: SelfDevelopmentModeProposeOnly, IdempotencyKey: "guest-propose"},
		"raise to audit_only":     {Mode: SelfDevelopmentModeAuditOnly, IdempotencyKey: "guest-audit"},
		"re-set off":              {Mode: SelfDevelopmentModeOff, IdempotencyKey: "guest-off"},
		"consume nothing":         consume("accept-once-consumed:", "op-guest", 0),
	} {
		t.Run("from off/"+name, func(t *testing.T) {
			if code, _ := f.guest(t, request); code != http.StatusForbidden {
				t.Fatalf("guest %s status=%d, want 403", name, code)
			}
			if got := f.current(t); got.Mode != SelfDevelopmentModeOff || got.Generation != 0 {
				t.Fatalf("guest %s changed the mode: %+v", name, got)
			}
		})
	}

	// From an owner-set propose_only: failure modes 1, 2, 4 and 5.
	code, proposed := f.owner(t, SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeProposeOnly, IdempotencyKey: "owner-propose"})
	if code != http.StatusOK || proposed.Mode != SelfDevelopmentModeProposeOnly {
		t.Fatalf("owner propose_only status=%d mode=%+v", code, proposed)
	}
	g := proposed.Generation
	for name, request := range map[string]SetSelfDevelopmentModeRequest{
		"arm accept_once":         f.arm(SelfDevelopmentModeAcceptOnce, "op-guest", g, "guest-arm-once-2"),
		"arm qualified_consensus": f.arm(SelfDevelopmentModeQualifiedConsensus, "op-guest", g, "guest-arm-qc-2"),
		"lower to off":            {Mode: SelfDevelopmentModeOff, ExpectedGeneration: g, IdempotencyKey: "guest-off-2"},
		"lower to audit_only":     {Mode: SelfDevelopmentModeAuditOnly, ExpectedGeneration: g, IdempotencyKey: "guest-audit-2"},
		"consume nothing":         consume("accept-once-consumed:", "op-guest", g),
	} {
		t.Run("from propose_only/"+name, func(t *testing.T) {
			if code, _ := f.guest(t, request); code != http.StatusForbidden {
				t.Fatalf("guest %s status=%d, want 403", name, code)
			}
			if got := f.current(t); got.Mode != SelfDevelopmentModeProposeOnly || got.Generation != g {
				t.Fatalf("guest %s changed the mode: %+v", name, got)
			}
		})
	}
}

func TestGuestCapabilityConsumesOnlyTheArmedGrant(t *testing.T) {
	f := newModeAuthorityFixture(t)
	code, armed := f.owner(t, f.arm(SelfDevelopmentModeAcceptOnce, "op-1", 0, "owner-arm-once"))
	if code != http.StatusOK || armed.Mode != SelfDevelopmentModeAcceptOnce || modeWriter(armed) != ModeWriterHostInternal {
		t.Fatalf("owner arm accept_once status=%d mode=%+v writer=%q", code, armed, modeWriter(armed))
	}
	// Failure modes 5 and 6: the wrong operation, the wrong kind of key, and
	// an arming request dressed as a consumption are refused.
	for name, request := range map[string]SetSelfDevelopmentModeRequest{
		"other operation":   consume("accept-once-consumed:", "op-2", armed.Generation),
		"operation prefix":  consume("accept-once-consumed:", "op-", armed.Generation),
		"wrong key kind":    consume("qualified-consensus-consumed:", "op-1", armed.Generation),
		"digest missing":    {Mode: SelfDevelopmentModeProposeOnly, ExpectedGeneration: armed.Generation, IdempotencyKey: "accept-once-consumed:op-1:"},
		"to off":            {Mode: SelfDevelopmentModeOff, ExpectedGeneration: armed.Generation, IdempotencyKey: "accept-once-consumed:op-1:" + strings.Repeat("d", 64)},
		"re-arm same grant": f.arm(SelfDevelopmentModeAcceptOnce, "op-1", armed.Generation, "accept-once-consumed:op-1:"+strings.Repeat("d", 64)),
	} {
		t.Run(name, func(t *testing.T) {
			if code, _ := f.guest(t, request); code != http.StatusForbidden {
				t.Fatalf("guest %s status=%d, want 403", name, code)
			}
			if got := f.current(t); got.Mode != SelfDevelopmentModeAcceptOnce || got.Generation != armed.Generation {
				t.Fatalf("guest %s changed the armed grant: %+v", name, got)
			}
		})
	}

	// Allowed 7 and 8: the decision handler's consumption, and its retry.
	consumption := consume("accept-once-consumed:", "op-1", armed.Generation)
	code, consumed := f.guest(t, consumption)
	if code != http.StatusOK || consumed.Mode != SelfDevelopmentModeProposeOnly || consumed.Generation != armed.Generation+1 {
		t.Fatalf("guest consumption status=%d mode=%+v", code, consumed)
	}
	if modeWriter(consumed) != ModeWriterGuestConsumption || consumed.Receipt.KindFields["consumed_operation_id"] != "op-1" {
		t.Fatalf("consumption receipt = %+v", consumed.Receipt.KindFields)
	}
	code, retried := f.guest(t, consumption)
	if code != http.StatusOK || retried.Receipt == nil || retried.Receipt.ReceiptID != consumed.Receipt.ReceiptID {
		t.Fatalf("idempotent consumption retry status=%d mode=%+v", code, retried)
	}

	// qualified_consensus consumption.
	code, armedQC := f.owner(t, f.arm(SelfDevelopmentModeQualifiedConsensus, "op-3", consumed.Generation, "owner-arm-qc"))
	if code != http.StatusOK || armedQC.Mode != SelfDevelopmentModeQualifiedConsensus {
		t.Fatalf("owner arm qualified_consensus status=%d mode=%+v", code, armedQC)
	}
	if code, _ := f.guest(t, consume("accept-once-consumed:", "op-3", armedQC.Generation)); code != http.StatusForbidden {
		t.Fatalf("accept_once key consumed a qualified_consensus grant: status=%d", code)
	}
	code, consumedQC := f.guest(t, consume("qualified-consensus-consumed:", "op-3", armedQC.Generation))
	if code != http.StatusOK || consumedQC.Mode != SelfDevelopmentModeProposeOnly || modeWriter(consumedQC) != ModeWriterGuestConsumption {
		t.Fatalf("guest qualified_consensus consumption status=%d mode=%+v", code, consumedQC)
	}
}

func TestOwnerStillSetsEveryMode(t *testing.T) {
	f := newModeAuthorityFixture(t)
	generation := uint64(0)
	for i, request := range []SetSelfDevelopmentModeRequest{
		{Mode: SelfDevelopmentModeAuditOnly, IdempotencyKey: "owner-1"},
		{Mode: SelfDevelopmentModeProposeOnly, IdempotencyKey: "owner-2"},
		f.arm(SelfDevelopmentModeAcceptOnce, "op-1", 0, "owner-3"),
		f.arm(SelfDevelopmentModeQualifiedConsensus, "op-1", 0, "owner-4"),
		{Mode: SelfDevelopmentModeOff, IdempotencyKey: "owner-5"},
	} {
		request.ExpectedGeneration = generation
		code, mode := f.owner(t, request)
		if code != http.StatusOK || mode.Mode != request.Mode || mode.Generation != generation+1 || modeWriter(mode) != ModeWriterHostInternal {
			t.Fatalf("owner set %d (%s) status=%d mode=%+v writer=%q", i, request.Mode, code, mode, modeWriter(mode))
		}
		generation = mode.Generation
	}
	// Allowed 11: the guest still reads the mode.
	r := httptest.NewRequest(http.MethodGet, "/internal/computers/self-development/mode?computer_id="+f.computerID, nil)
	r.RemoteAddr = "172.16.0.2:40000"
	r.Header.Set("Authorization", "Bearer "+f.guestToken)
	w := httptest.NewRecorder()
	f.handler.HandleSelfDevelopmentMode(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("guest mode read status=%d", w.Code)
	}
}

func TestSelfDevelopmentModeExpiryIsRecordedAsCorpusdWrite(t *testing.T) {
	f := newModeAuthorityFixture(t)
	start := time.Date(2026, 7, 19, 1, 0, 0, 0, time.UTC)
	f.now = start
	f.modes.now = func() time.Time { return start }
	if code, _ := f.owner(t, f.arm(SelfDevelopmentModeAcceptOnce, "op-1", 0, "owner-arm")); code != http.StatusOK {
		t.Fatalf("owner arm status=%d", code)
	}
	f.modes.now = func() time.Time { return start.Add(10 * time.Minute) }
	expired := f.current(t)
	if expired.Mode != SelfDevelopmentModeOff || modeWriter(expired) != ModeWriterCorpusdExpiry {
		t.Fatalf("expired mode = %+v writer=%q", expired, modeWriter(expired))
	}
}
