package apihandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/kernel"
)

func TestPluginsStateReturnsKernelSnapshot(t *testing.T) {
	h := NewHandler(nil)
	child, err := h.kernel.Root().Child("probe")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = h.kernel.Shutdown(ctx)
	}()
	_ = child.OnClose("noop", func(context.Context) error { return nil })

	rec := httptest.NewRecorder()
	h.HandlePluginsState(rec, authenticatedCostsRequest(http.MethodGet, "/api/plugins/state", "owner-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var snap kernel.Snapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Scope.Name != "kernel" || len(snap.Scope.Children) != 1 || snap.Scope.Children[0].Effects[0] != "noop" {
		t.Fatalf("snapshot = %+v", snap)
	}

	rec = httptest.NewRecorder()
	h.HandlePluginsState(rec, authenticatedCostsRequest(http.MethodGet, "/api/plugins/state", ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.HandlePluginsState(rec, authenticatedCostsRequest(http.MethodPost, "/api/plugins/state", "owner-1"))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status %d", rec.Code)
	}
}

// The plugins state names the installed shell providers a deployed proof reads
// (surface mount: an own-track release of menu-default is the active provider
// of contextMenu.renderer; a framed release is listed and not active).
func TestPluginsStateListsInstalledProviders(t *testing.T) {
	h := NewHandler(nil)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = h.kernel.Shutdown(ctx)
	}()
	h.SetSlotSource(func(_ context.Context, ownerID string) []agentcore.InstalledSlot {
		if ownerID != "owner-1" {
			t.Errorf("slot source asked for owner %q", ownerID)
		}
		return []agentcore.InstalledSlot{{Slot: "contextMenu.renderer", Package: "menu-default", ReleaseID: "r1", Isolation: "shell", Priority: 10, Available: true, Eligible: true}}
	})
	rec := httptest.NewRecorder()
	h.HandlePluginsState(rec, authenticatedCostsRequest(http.MethodGet, "/api/plugins/state", "owner-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Scope kernel.ScopeInfo          `json:"scope"`
		Slots []agentcore.InstalledSlot `json:"slots"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Slots) != 1 || out.Slots[0].Package != "menu-default" || !out.Slots[0].Eligible || out.Slots[0].Slot != "contextMenu.renderer" || out.Scope.Name != "kernel" {
		t.Fatalf("state = %s", rec.Body.String())
	}
	// With no source the field is an empty list, never absent.
	h2 := NewHandler(nil)
	rec = httptest.NewRecorder()
	h2.HandlePluginsState(rec, authenticatedCostsRequest(http.MethodGet, "/api/plugins/state", "owner-1"))
	if !strings.Contains(rec.Body.String(), `"slots":[]`) {
		t.Fatalf("no source: %s", rec.Body.String())
	}
}
