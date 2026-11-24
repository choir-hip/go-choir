package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// What can go wrong: reading the data-volume report of a hibernated computer
// wakes it (an investigator's read woke the drill computer on 2026-11-24 and
// consumed its 1.5 GB snapshot), or the no-wake rule also stops the same
// read of an active computer.
func noWakeFixture(t *testing.T, state string) (h *Handler, secret string, guestCalls, otherVMCTLCalls *atomic.Int64) {
	t.Helper()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("no-wake-owner", "no-wake-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	const computerID = "computer-no-wake"
	_, secret, err = store.CreateComputerScopedAPIKey(t.Context(), user.ID, "no wake", []string{"read:runtime"}, computerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	guestCalls, otherVMCTLCalls = new(atomic.Int64), new(atomic.Int64)
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guestCalls.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(guest.Close)
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			url := guest.URL
			if state != "active" {
				url = ""
			}
			reported := state
			if state == "active-no-url" {
				reported = "active"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": computerID, "user_id": user.ID, "desktop_id": "primary", "vm_id": "vm-nw",
				"computer_url": url, "state": reported, "epoch": 3,
			})
		case "/internal/vmctl/touch":
			_, _ = w.Write([]byte(`{"touched":true}`))
		default:
			otherVMCTLCalls.Add(1)
			http.Error(w, "a wake or a resolve", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(vm.Close)
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	return handler, secret, guestCalls, otherVMCTLCalls
}

func TestDataVolumeDetailReadDoesNotWakeAHibernatedComputer(t *testing.T) {
	handler, secret, guestCalls, otherCalls := noWakeFixture(t, string(vmctl.VMStateHibernated))
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability?data_volume=detail", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d want 409 body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hibernated") || !strings.Contains(rec.Body.String(), "not woken") {
		t.Fatalf("the refusal does not say why: %s", rec.Body.String())
	}
	if otherCalls.Load() != 0 || guestCalls.Load() != 0 {
		t.Fatalf("vmctl wake/resolve calls=%d guest calls=%d, want 0/0", otherCalls.Load(), guestCalls.Load())
	}
}

func TestDataVolumeDetailReadOfAnActiveComputerIsServed(t *testing.T) {
	handler, secret, guestCalls, _ := noWakeFixture(t, "active")
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability?data_volume=detail", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusOK || guestCalls.Load() != 1 {
		t.Fatalf("status=%d guest calls=%d body=%s", rec.Code, guestCalls.Load(), rec.Body.String())
	}
}

func TestPlainObservabilityReadReachesTheWakePathAndIsNotRefused(t *testing.T) {
	// Only the diagnostic read is exempt: the plain read is what health and
	// the shell use, and it keeps its wake.
	handler, secret, _, otherCalls := noWakeFixture(t, string(vmctl.VMStateHibernated))
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if otherCalls.Load() == 0 || rec.Code == http.StatusConflict || strings.Contains(rec.Body.String(), "not woken") {
		t.Fatalf("a plain read no longer reaches the wake path or was refused as a diagnostic read; wake calls=%d status=%d body=%s", otherCalls.Load(), rec.Code, rec.Body.String())
	}
}

// The no-wake rule is held at the call site: the diagnostic read never calls
// the waking resolver, whatever the computer's state.
func TestDataVolumeDetailNeverCallsTheWakingResolver(t *testing.T) {
	for _, state := range []string{string(vmctl.VMStateHibernated), "active-no-url", "active"} {
		handler, secret, _, _ := noWakeFixture(t, state)
		handler.testWakingResolverHook = func() { t.Errorf("state %s: the diagnostic read reached the waking resolver", state) }
		req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability?data_volume=detail", nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		handler.HandleAPI(httptest.NewRecorder(), req)
	}
}

func TestDataVolumeDetailHeadAndAnActiveComputerWithNoURLDoNotWakeEither(t *testing.T) {
	for name, c := range map[string]struct{ state, method string }{
		"HEAD of a hibernated computer":         {string(vmctl.VMStateHibernated), http.MethodHead},
		"GET of an active computer with no URL": {"active-no-url", http.MethodGet},
	} {
		t.Run(name, func(t *testing.T) {
			handler, secret, guestCalls, otherCalls := noWakeFixture(t, c.state)
			req := httptest.NewRequest(c.method, "/api/runtime/observability?data_volume=detail", nil)
			req.Header.Set("Authorization", "Bearer "+secret)
			rec := httptest.NewRecorder()
			handler.HandleAPI(rec, req)
			if rec.Code != http.StatusConflict || otherCalls.Load() != 0 || guestCalls.Load() != 0 {
				t.Fatalf("status=%d vmctl wake/resolve calls=%d guest calls=%d body=%s", rec.Code, otherCalls.Load(), guestCalls.Load(), rec.Body.String())
			}
		})
	}
}
