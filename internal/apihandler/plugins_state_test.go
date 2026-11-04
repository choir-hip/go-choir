package apihandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
