package maild

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealthReportsSafeMaildState(t *testing.T) {
	store, cfg := newTestStore(t)
	cfg.ResendAPIKey = "re_test"
	cfg.WebhookSecret = "whsec_test"
	handler := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "ok" || resp.Service != "maild" {
		t.Fatalf("unexpected status/service: %+v", resp)
	}
	if !resp.ResendAPIKeyConfigured || !resp.WebhookSecretConfigured || !resp.RootOwnerIDConfigured {
		t.Fatalf("expected config booleans true, got %+v", resp)
	}
	if resp.Stats.Aliases != 1 {
		t.Fatalf("aliases = %d, want 1", resp.Stats.Aliases)
	}
}

func TestHandleHealthRejectsNonGET(t *testing.T) {
	store, cfg := newTestStore(t)
	handler := NewHandler(cfg, store)

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	handler.HandleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}
