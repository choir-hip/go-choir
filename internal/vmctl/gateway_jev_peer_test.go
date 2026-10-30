package vmctl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// vmctl names the computer's current guest address when it reconciles the
// gateway credential, so the gateway moves the Jev pin with the computer
// (docs/archive/problems/l2a-merge-recall-on-the-paper-2026-10-14.md, finding 2).
// Ways it can fail, written before the code:
//  1. the ensure carries no address, so the pin stays on the first boot's
//     address and Jev refuses the computer after every cold boot;
//  2. the 10-minute success throttle swallows the ensure after a reboot
//     onto a new address, so Jev stays refused until some later lookup;
//  3. an address is guessed from a URL that has none (a hostname, an empty
//     URL) and the pin moves somewhere wrong;
//  4. the throttle stops working, so every lookup posts to the gateway.

type ensureCall struct {
	RawToken string `json:"raw_token"`
	PeerIP   string `json:"peer_ip"`
	HasPeer  bool   `json:"-"`
}

func ensureRecorder(t *testing.T) (*httptest.Server, func() []ensureCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []ensureCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provider/v1/credentials/ensure" {
			http.NotFound(w, r)
			return
		}
		var raw map[string]any
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Errorf("decode ensure: %v", err)
		}
		c := ensureCall{}
		c.RawToken, _ = raw["raw_token"].(string)
		if p, ok := raw["peer_ip"]; ok {
			c.HasPeer = true
			c.PeerIP, _ = p.(string)
		}
		mu.Lock()
		calls = append(calls, c)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"computer_id":"vm-jev-peer","status":"already_active"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []ensureCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]ensureCall(nil), calls...)
	}
}

func registryWithVM(t *testing.T, gatewayURL, computerURL string) (*OwnershipRegistry, *VMOwnership) {
	t.Helper()
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL(gatewayURL)
	reg.SetVMManager(&mockVMManager{tokens: map[string]string{"vm-jev-peer": "vm-jev-peer:host-held-token"}})
	now := time.Now()
	own := &VMOwnership{VMID: "vm-jev-peer", UserID: "user-jev-peer", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive,
		ComputerURL: computerURL, State: VMStateActive, CreatedAt: now, LastActiveAt: now, Epoch: 1}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(own.UserID, own.DesktopID)] = own
	reg.vmByID[own.VMID] = own
	reg.mu.Unlock()
	return reg, own
}

func TestGatewayCredentialEnsureNamesTheGuestAddressAndFollowsAReboot(t *testing.T) {
	srv, calls := ensureRecorder(t)
	reg, own := registryWithVM(t, srv.URL, "http://10.200.0.2:8085")

	reg.ensureExistingGatewayCredential(own.VMID)
	got := calls()
	if len(got) != 1 || got[0].PeerIP != "10.200.0.2" {
		t.Fatalf("first ensure = %+v; want one call naming 10.200.0.2", got)
	}

	// Failure mode 4: the same address within the throttle posts nothing.
	reg.ensureExistingGatewayCredential(own.VMID)
	if n := len(calls()); n != 1 {
		t.Fatalf("ensure calls = %d after a repeat on the same address; the throttle stopped working", n)
	}

	// Failure mode 2: a reboot onto a new address is not throttled.
	reg.mu.Lock()
	own.ComputerURL = "http://10.200.1.2:8085"
	reg.mu.Unlock()
	reg.ensureExistingGatewayCredential(own.VMID)
	got = calls()
	if len(got) != 2 || got[1].PeerIP != "10.200.1.2" {
		t.Fatalf("ensure after the reboot = %+v; want a second call naming 10.200.1.2", got)
	}
}

func TestGatewayCredentialEnsureNamesNoAddressWhenTheURLHasNone(t *testing.T) {
	for _, computerURL := range []string{"", "http://autoputer.internal:8085", "::not a url"} {
		srv, calls := ensureRecorder(t)
		reg, own := registryWithVM(t, srv.URL, computerURL)
		reg.ensureExistingGatewayCredential(own.VMID)
		got := calls()
		if len(got) != 1 {
			t.Fatalf("%q: ensure calls = %d, want 1 (the credential is still reconciled)", computerURL, len(got))
		}
		if got[0].HasPeer {
			t.Fatalf("%q: ensure named peer %q from a URL with no address", computerURL, got[0].PeerIP)
		}
	}
}
