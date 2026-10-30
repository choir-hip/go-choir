package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The Jev bearer follows its computer across boots
// (docs/archive/problems/l2a-merge-recall-on-the-paper-2026-10-14.md, finding 2).
// A computer keeps its gateway token across boots, but its guest address
// comes from the host port of each boot. The Jev route pins the bearer to
// the first address that used it, so after a cold boot onto a new port
// every Jev call from that computer was refused ("Jev bearer is not
// authorized for this VM"): intake labels and the support judge both.
// Ways the repair can fail, written before the code:
//  1. after a boot onto a new address the pin stays on the old one, so Jev
//     refuses the computer for the life of its credential (the defect);
//  2. the Jev route itself moves the pin, so a bearer copied to another VM
//     takes Jev over by calling it (the pin's purpose is lost); only host
//     lifecycle control may move it;
//  3. a peer address that is not a Choir guest address (loopback, public)
//     is accepted, and Jev is pinned to an address no VM of this computer
//     has; or it fails the ensure, so the credential itself stops being
//     reconciled (a development runtime on loopback);
//  4. an ensure without a peer address (an older vmctl) changes the pin;
//  5. a revoked, expired or conflicting credential has its pin moved;
//  6. the moved pin is lost on a gateway restart, and the old address works
//     again;
//  7. the ensure response echoes the raw token (covered by the existing
//     ensure handler test).

const (
	firstBootPeer  = "10.200.0.2"
	secondBootPeer = "10.200.1.2"
)

func TestEnsureCredentialForPeerMovesTheJevPinToTheHostAttestedAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway-identities.json")
	reg := NewIdentityRegistry(time.Hour)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	cred, err := reg.IssueCredential("computer-repin")
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.BindPeer("computer-repin", firstBootPeer); err != nil {
		t.Fatalf("first boot binds: %v", err)
	}
	// Failure mode 1, the defect as it is: the next boot's address is refused.
	if err := reg.BindPeer("computer-repin", secondBootPeer); err == nil {
		t.Fatal("a second address was accepted before any re-pin; the pin guards nothing")
	}

	result, err := reg.EnsureCredentialForPeer(cred.RawToken, secondBootPeer)
	if err != nil {
		t.Fatalf("ensure with the new boot's address: %v", err)
	}
	if result.Status != "already_active" {
		t.Fatalf("status = %q, want already_active", result.Status)
	}
	if err := reg.BindPeer("computer-repin", secondBootPeer); err != nil {
		t.Fatalf("the new boot's address after the host re-pinned it: %v", err)
	}
	if err := reg.BindPeer("computer-repin", firstBootPeer); err == nil {
		t.Fatal("the old address still works after the re-pin")
	}

	// Failure mode 6: the pin survives a gateway restart.
	restarted := NewIdentityRegistry(time.Hour)
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	if err := restarted.BindPeer("computer-repin", secondBootPeer); err != nil {
		t.Fatalf("the re-pinned address after a gateway restart: %v", err)
	}
	if err := restarted.BindPeer("computer-repin", firstBootPeer); err == nil {
		t.Fatal("the old address works again after a gateway restart")
	}
}

func TestEnsureCredentialForPeerImportsWithThePin(t *testing.T) {
	reg := NewIdentityRegistry(time.Hour)
	raw := "computer-imported:host-held-token"
	result, err := reg.EnsureCredentialForPeer(raw, secondBootPeer)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "imported" {
		t.Fatalf("status = %q, want imported", result.Status)
	}
	if err := reg.BindPeer("computer-imported", firstBootPeer); err == nil {
		t.Fatal("an imported credential took its pin from the first Jev caller, not from the host")
	}
}

func TestEnsureCredentialForPeerRefusesWhatItMustNotMove(t *testing.T) {
	reg := NewIdentityRegistry(time.Hour)
	cred, err := reg.IssueCredential("computer-guarded")
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.BindPeer("computer-guarded", firstBootPeer); err != nil {
		t.Fatal(err)
	}
	// Failure mode 3: only a Choir guest address can hold the pin.
	for _, peer := range []string{"127.0.0.1", "8.8.8.8", "10.200.0.1", "not-an-ip"} {
		if _, err := reg.EnsureCredentialForPeer(cred.RawToken, peer); err == nil {
			t.Fatalf("peer %q accepted", peer)
		}
	}
	// Failure mode 4: no peer keeps the pin where it is.
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, ""); err != nil {
		t.Fatalf("ensure without a peer: %v", err)
	}
	if _, err := reg.EnsureCredential(cred.RawToken); err != nil {
		t.Fatalf("ensure (old call): %v", err)
	}
	if err := reg.BindPeer("computer-guarded", firstBootPeer); err != nil {
		t.Fatalf("the pin moved without a peer or with a refused one: %v", err)
	}
	// Failure mode 5: a conflicting or revoked credential moves nothing.
	if _, err := reg.EnsureCredentialForPeer("computer-guarded:other-token", secondBootPeer); err == nil {
		t.Fatal("a conflicting credential was ensured")
	}
	reg.RevokeCredential("computer-guarded")
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, secondBootPeer); err == nil {
		t.Fatal("a revoked credential was ensured")
	}
	reg.mu.Lock()
	pinned := reg.identities["computer-guarded"].PeerIP
	reg.mu.Unlock()
	if pinned != firstBootPeer {
		t.Fatalf("pin = %q after refused ensures, want %q", pinned, firstBootPeer)
	}
}

// The product path end to end through the handlers: a computer judged from
// its first boot, rebooted onto a new address, is refused until vmctl's
// ensure carries the new address; the Jev route never moves the pin.
func TestJevRouteFollowsTheComputerAfterTheHostReEnsuresItsCredential(t *testing.T) {
	h, reg, _ := setupHandlerWithRateLimit(t, 100, time.Minute)
	h.SetJevTransport(&JevTransport{enabled: true, apiKey: "test-key", httpClient: &http.Client{Transport: &decisionsRoundTripper{}},
		receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "receipts.jsonl"))})
	cred, err := reg.IssueCredential("computer-reboots")
	if err != nil {
		t.Fatal(err)
	}
	judge := func(peer string) int {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"state":     map[string]any{"capture": "x"},
			"questions": map[string]any{"q": map[string]any{"type": "choice", "choices": []string{"yes", "no"}}},
		})
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/judgments", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		req.RemoteAddr = peer + ":40000"
		rec := httptest.NewRecorder()
		h.HandleJudgment(rec, req)
		return rec.Code
	}
	ensure := func(body string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/credentials/ensure", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Caller", "true")
		req.Host = "localhost:8084"
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		h.HandleEnsureCredential(rec, req)
		return rec.Code
	}

	if code := judge(firstBootPeer); code != http.StatusOK {
		t.Fatalf("first boot = %d", code)
	}
	if code := judge(secondBootPeer); code != http.StatusForbidden {
		t.Fatalf("after a reboot onto a new address, before the host re-ensures = %d; want 403", code)
	}
	// Failure mode 2: calling Jev from the new address did not move the pin.
	if code := judge(firstBootPeer); code != http.StatusOK {
		t.Fatalf("a refused Jev call moved the pin: first address now = %d", code)
	}
	// A non-guest peer still ensures the credential but moves nothing.
	if code := ensure(`{"raw_token":"` + cred.RawToken + `","peer_ip":"127.0.0.1"}`); code != http.StatusOK {
		t.Fatalf("ensure with a non-guest peer = %d; the credential must still be ensured", code)
	}
	if code := judge(secondBootPeer); code != http.StatusForbidden {
		t.Fatalf("a non-guest peer moved the pin: new address = %d", code)
	}
	if code := ensure(`{"raw_token":"` + cred.RawToken + `","peer_ip":"` + secondBootPeer + `"}`); code != http.StatusOK {
		t.Fatalf("ensure with the new boot's address = %d", code)
	}
	if code := judge(secondBootPeer); code != http.StatusOK {
		t.Fatalf("after the host re-ensured, the new address = %d", code)
	}
	if code := judge(firstBootPeer); code != http.StatusForbidden {
		t.Fatalf("after the re-pin, the old address = %d; want 403", code)
	}
}

// SEC-05 findings 13 and 14: the Jev route keeps its first-use pin in every
// mode but strict (its pre-SEC-05 boundary; observe must not open it), and
// strict refuses an unpinned bearer, so Jev cannot write a pin that opens the
// other routes there. A mismatch is Jev's own 403 in every mode.
func TestJevRouteKeepsItsFirstUsePinInEveryModeButStrict(t *testing.T) {
	cases := []struct {
		mode     BindingMode
		wantCode int
		wantPin  bool
	}{
		{BindingOff, http.StatusOK, true},
		{BindingObserve, http.StatusOK, true},
		{BindingEnforce, http.StatusOK, true},
		{BindingStrict, http.StatusForbidden, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			h, reg, _ := setupHandlerWithRateLimit(t, 100, time.Minute)
			h.SetBearerBinding(tc.mode)
			h.SetJevTransport(&JevTransport{enabled: true, apiKey: "test-key", httpClient: &http.Client{Transport: &decisionsRoundTripper{}},
				receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "receipts.jsonl"))})
			cred, err := reg.IssueCredential("computer-jev")
			if err != nil {
				t.Fatal(err)
			}
			judge := func(peer string) int {
				t.Helper()
				body, _ := json.Marshal(map[string]any{
					"state":     map[string]any{"capture": "x"},
					"questions": map[string]any{"q": map[string]any{"type": "choice", "choices": []string{"yes", "no"}}},
				})
				req := httptest.NewRequest(http.MethodPost, "/provider/v1/judgments", bytes.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+cred.RawToken)
				req.RemoteAddr = peer + ":40000"
				rec := httptest.NewRecorder()
				h.HandleJudgment(rec, req)
				return rec.Code
			}
			pin := func() string {
				t.Helper()
				state, p, err := reg.CheckPeer("computer-jev", "10.200.9.9", false)
				if err != nil {
					t.Fatal(err)
				}
				if state == PeerUnpinned {
					return ""
				}
				return p
			}

			if code := judge(firstBootPeer); code != tc.wantCode {
				t.Fatalf("unpinned Jev call in %s = %d, want %d", tc.mode, code, tc.wantCode)
			}
			switch got := pin(); {
			case tc.wantPin && got != firstBootPeer:
				t.Fatalf("%s: first use did not pin (pin %q)", tc.mode, got)
			case !tc.wantPin && got != "":
				t.Fatalf("%s: the Jev route wrote a pin (%q) the mode forbids", tc.mode, got)
			}
			if tc.wantPin {
				// Finding 14: a copied bearer from another guest is refused
				// before the host's ensure, as it was before SEC-05.
				if code := judge(secondBootPeer); code != http.StatusForbidden {
					t.Fatalf("%s: a copied bearer from another guest before the host's ensure = %d, want 403", tc.mode, code)
				}
			}
			if tc.mode == BindingStrict {
				// The refused Jev call did not open inference either.
				if code := inferFrom(h, cred.RawToken, firstBootPeer); code != http.StatusTooManyRequests {
					t.Fatalf("strict: inference after a Jev call from an unpinned bearer = %d, want 429", code)
				}
			}

			// Once the host attests an address, a mismatch is Jev's 403 in
			// every mode and the pin does not move.
			if _, err := reg.EnsureCredentialForPeer(cred.RawToken, firstBootPeer); err != nil {
				t.Fatal(err)
			}
			if code := judge(secondBootPeer); code != http.StatusForbidden {
				t.Fatalf("%s: a Jev call from another address = %d, want 403", tc.mode, code)
			}
			if got := pin(); got != firstBootPeer {
				t.Fatalf("%s: a refused Jev call moved the pin to %q", tc.mode, got)
			}
			if code := judge(firstBootPeer); code != http.StatusOK {
				t.Fatalf("%s: the attested address = %d", tc.mode, code)
			}
		})
	}
}
