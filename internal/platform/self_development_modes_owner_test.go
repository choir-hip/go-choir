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
// (docs/archive/problems/guest-can-arm-own-selfdev-mode-2026-10-15.md). The
// guest's event:append capability writes no mode at all.

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

func modeWriter(mode SelfDevelopmentMode) string {
	if mode.Receipt == nil {
		return ""
	}
	value, _ := mode.Receipt.KindFields["mode_writer"].(string)
	return value
}

func TestGuestCapabilityCannotSetSelfDevelopmentMode(t *testing.T) {
	f := newModeAuthorityFixture(t)
	// From no row (canonical apply, generation 0): the guest writes nothing.
	for name, request := range map[string]SetSelfDevelopmentModeRequest{
		"set apply":        {Mode: SelfDevelopmentModeApply, IdempotencyKey: "guest-apply"},
		"set off":          {Mode: SelfDevelopmentModeOff, IdempotencyKey: "guest-off"},
		"legacy arm":       {Mode: "accept_once", IdempotencyKey: "guest-arm-once"},
		"legacy consume":   {Mode: "propose_only", IdempotencyKey: "accept-once-consumed:op-guest:" + strings.Repeat("d", 64)},
		"legacy audit":     {Mode: "audit_only", IdempotencyKey: "guest-audit"},
		"legacy standing":  {Mode: "standing_consensus", IdempotencyKey: "guest-standing"},
		"legacy qualified": {Mode: "qualified_consensus", IdempotencyKey: "guest-qc"},
	} {
		t.Run("no row/"+name, func(t *testing.T) {
			if code, _ := f.guest(t, request); code != http.StatusForbidden {
				t.Fatalf("guest %s status=%d, want 403", name, code)
			}
			if got := f.current(t); got.Mode != SelfDevelopmentModeApply || got.Generation != 0 {
				t.Fatalf("guest %s changed the mode: %+v", name, got)
			}
		})
	}
	// From an owner-set off: the guest cannot raise it, and cannot consume.
	code, off := f.owner(t, SetSelfDevelopmentModeRequest{Mode: SelfDevelopmentModeOff, IdempotencyKey: "owner-off"})
	if code != http.StatusOK || off.Mode != SelfDevelopmentModeOff {
		t.Fatalf("owner off status=%d mode=%+v", code, off)
	}
	for name, request := range map[string]SetSelfDevelopmentModeRequest{
		"raise to apply": {Mode: SelfDevelopmentModeApply, ExpectedGeneration: off.Generation, IdempotencyKey: "guest-apply-2"},
		"re-set off":     {Mode: SelfDevelopmentModeOff, ExpectedGeneration: off.Generation, IdempotencyKey: "guest-off-2"},
	} {
		t.Run("from off/"+name, func(t *testing.T) {
			if code, _ := f.guest(t, request); code != http.StatusForbidden {
				t.Fatalf("guest %s status=%d, want 403", name, code)
			}
			if got := f.current(t); got.Mode != SelfDevelopmentModeOff || got.Generation != off.Generation {
				t.Fatalf("guest %s changed the mode: %+v", name, got)
			}
		})
	}
}

func TestOwnerSetsOffAndApplyOnly(t *testing.T) {
	f := newModeAuthorityFixture(t)
	generation := uint64(0)
	for i, request := range []SetSelfDevelopmentModeRequest{
		{Mode: SelfDevelopmentModeOff, IdempotencyKey: "owner-1"},
		{Mode: SelfDevelopmentModeApply, IdempotencyKey: "owner-2"},
		{Mode: SelfDevelopmentModeOff, IdempotencyKey: "owner-3"},
	} {
		request.ExpectedGeneration = generation
		code, mode := f.owner(t, request)
		if code != http.StatusOK || mode.Mode != request.Mode || mode.Generation != generation+1 || modeWriter(mode) != ModeWriterHostInternal {
			t.Fatalf("owner set %d (%s) status=%d mode=%+v writer=%q", i, request.Mode, code, mode, modeWriter(mode))
		}
		generation = mode.Generation
	}
	for _, retired := range []string{"audit_only", "propose_only", "accept_once", "qualified_consensus", "standing_consensus"} {
		code, _ := f.owner(t, SetSelfDevelopmentModeRequest{Mode: retired, ExpectedGeneration: generation, IdempotencyKey: "owner-retired-" + retired})
		if code != http.StatusBadRequest {
			t.Fatalf("owner wrote retired mode %s: status=%d, want 400", retired, code)
		}
	}
	// The guest still reads the mode.
	r := httptest.NewRequest(http.MethodGet, "/internal/computers/self-development/mode?computer_id="+f.computerID, nil)
	r.RemoteAddr = "172.16.0.2:40000"
	r.Header.Set("Authorization", "Bearer "+f.guestToken)
	w := httptest.NewRecorder()
	f.handler.HandleSelfDevelopmentMode(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("guest mode read status=%d", w.Code)
	}
}
