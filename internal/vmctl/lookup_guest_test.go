package vmctl

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Binding a guest's source address to its computer (B12). Every host
// service that admits a guest's write (corpusd captures and verdicts, the
// proxy's guest routes, maild) asks this lookup which computer owns the
// address. Ways it can fail, written before the fix:
//   - a stopped, hibernated or failed computer keeps its old computer URL,
//     and a new computer reuses the tap address; the lookup walks a map in
//     random order and answers with whichever ownership it meets first, so
//     the live guest's writes are refused as another computer's at random
//     (staging c2-paper-3: 10.200.44.2 was claimed by a computer stopped at
//     05:01 and by the paper computer booted at 12:35, and about half of
//     its capture writes got 403 "internal caller required");
//   - two running computers claim one address, and the lookup picks one:
//     it must refuse to bind rather than guess;
//   - an address only a stopped computer claims binds to it.

func lookupGuest(t *testing.T, h *Handler, addr string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"remote_addr": addr})
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/lookup-guest", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5000"
	rec := httptest.NewRecorder()
	h.HandleLookupGuestOwnership(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("lookup %s = %d %s", addr, rec.Code, rec.Body)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func seedOwnership(reg *OwnershipRegistry, user, computer, url string, state VMState) {
	reg.ownerships[ownershipKey(user, PrimaryDesktopID)] = &VMOwnership{
		VMID: "vm-" + computer, ComputerID: computer, UserID: user, DesktopID: PrimaryDesktopID,
		ComputerURL: url, State: state,
	}
}

func TestLookupGuestBindsTheLiveComputerNotAStaleOneAtTheSameAddress(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1")
	seedOwnership(reg, "user-stale-1", "computer-stopped", "http://10.200.44.2:8085", VMStateStopped)
	seedOwnership(reg, "user-stale-2", "computer-hibernated", "http://10.200.44.2:8085", VMStateHibernated)
	seedOwnership(reg, "user-stale-3", "computer-failed", "http://10.200.44.2:8085", VMStateFailed)
	seedOwnership(reg, "user-paper", "computer-paper", "http://10.200.44.2:8085", VMStateActive)
	h := NewHandler(reg)
	// Map order is random per range; enough rounds that a lookup which
	// answers with the first match it meets fails here.
	for i := 0; i < 64; i++ {
		got := lookupGuest(t, h, "10.200.44.2:40000")
		if got["found"] != true || got["computer_id"] != "computer-paper" || got["user_id"] != "user-paper" {
			t.Fatalf("round %d: lookup bound %v; want the live computer-paper", i, got)
		}
	}
}

func TestLookupGuestRefusesAnAddressOnlyStaleComputersClaim(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1")
	seedOwnership(reg, "user-stale", "computer-stopped", "http://10.200.43.2:8085", VMStateStopped)
	h := NewHandler(reg)
	if got := lookupGuest(t, h, "10.200.43.2:40000"); got["found"] != false {
		t.Fatalf("a stopped computer's address bound %v", got)
	}
}

func TestLookupGuestRefusesAnAddressTwoRunningComputersClaim(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1")
	seedOwnership(reg, "user-a", "computer-a", "http://10.200.45.2:8085", VMStateActive)
	seedOwnership(reg, "user-b", "computer-b", "http://10.200.45.2:8085", VMStateDegraded)
	h := NewHandler(reg)
	if got := lookupGuest(t, h, "10.200.45.2:40000"); got["found"] != false {
		t.Fatalf("an address two running computers claim bound %v; want no binding", got)
	}
}

// A computer resuming from stop boots with the URL of its last boot until
// this boot completes; that old address may be a running computer's now.
// The running computer keeps its binding.
func TestLookupGuestPrefersTheRunningComputerOverABootingOneWithAnOldURL(t *testing.T) {
	reg := NewOwnershipRegistry("http://127.0.0.1")
	seedOwnership(reg, "user-resuming", "computer-resuming", "http://10.200.46.2:8085", VMStateBooting)
	seedOwnership(reg, "user-running", "computer-running", "http://10.200.46.2:8085", VMStateActive)
	h := NewHandler(reg)
	for i := 0; i < 32; i++ {
		if got := lookupGuest(t, h, "10.200.46.2:40000"); got["computer_id"] != "computer-running" {
			t.Fatalf("round %d: lookup bound %v; want computer-running", i, got)
		}
	}
	// Alone, a booting computer binds at its URL.
	solo := NewOwnershipRegistry("http://127.0.0.1")
	seedOwnership(solo, "user-booting", "computer-booting", "http://10.200.47.2:8085", VMStateBooting)
	if got := lookupGuest(t, NewHandler(solo), "10.200.47.2:40000"); got["computer_id"] != "computer-booting" {
		t.Fatalf("a booting computer alone at its address bound %v", got)
	}
}
