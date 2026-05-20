package sandbox

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleBootstrapReturnsSandboxIdentity(t *testing.T) {
	cfg := Config{Port: "0", SandboxID: "sandbox-test-001"}
	h := NewHandler(cfg.SandboxID)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp BootstrapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode bootstrap response: %v", err)
	}

	if resp.SandboxID != "sandbox-test-001" {
		t.Errorf("expected sandbox_id %q, got %q", "sandbox-test-001", resp.SandboxID)
	}
	if resp.Bootstrap != "placeholder-shell-v1" {
		t.Errorf("expected bootstrap %q, got %q", "placeholder-shell-v1", resp.Bootstrap)
	}
}

func TestHandleBootstrapEchoesUserContext(t *testing.T) {
	cfg := Config{Port: "0", SandboxID: "sandbox-test-001"}
	h := NewHandler(cfg.SandboxID)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("X-Authenticated-User", "user-alice@example.com")
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	var resp BootstrapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode bootstrap response: %v", err)
	}

	if resp.User != "user-alice@example.com" {
		t.Errorf("expected user %q, got %q", "user-alice@example.com", resp.User)
	}
}

func TestHandleBootstrapEchoesRequestPath(t *testing.T) {
	cfg := Config{Port: "0", SandboxID: "sandbox-test-001"}
	h := NewHandler(cfg.SandboxID)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap?detail=full", nil)
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	var resp BootstrapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode bootstrap response: %v", err)
	}

	if resp.Path != "/api/shell/bootstrap" {
		t.Errorf("expected path %q, got %q", "/api/shell/bootstrap", resp.Path)
	}
	if resp.Method != "GET" {
		t.Errorf("expected method %q, got %q", "GET", resp.Method)
	}
	if resp.Query != "detail=full" {
		t.Errorf("expected query %q, got %q", "detail=full", resp.Query)
	}
}

func TestHandleBootstrapRejectsNonGet(t *testing.T) {
	cfg := Config{Port: "0", SandboxID: "sandbox-test-001"}
	h := NewHandler(cfg.SandboxID)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/shell/bootstrap", nil)
			w := httptest.NewRecorder()
			h.HandleBootstrap(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected status 405 for %s, got %d", method, w.Code)
			}
		})
	}
}

func TestHandleBootstrapReturnsJSONContentType(t *testing.T) {
	cfg := Config{Port: "0", SandboxID: "sandbox-test-001"}
	h := NewHandler(cfg.SandboxID)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type to contain application/json, got %q", ct)
	}
}

func TestHandleErrorReturnsNon2xx(t *testing.T) {
	cfg := Config{Port: "0", SandboxID: "sandbox-test-001"}
	h := NewHandler(cfg.SandboxID)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/error", nil)
	w := httptest.NewRecorder()
	h.HandleError(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.SandboxID != "sandbox-test-001" {
		t.Errorf("expected sandbox_id %q, got %q", "sandbox-test-001", resp.SandboxID)
	}
	if resp.StatusCode != 500 {
		t.Errorf("expected status_code 500, got %d", resp.StatusCode)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleErrorReturnsJSONContentType(t *testing.T) {
	cfg := Config{Port: "0", SandboxID: "sandbox-test-001"}
	h := NewHandler(cfg.SandboxID)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/error", nil)
	w := httptest.NewRecorder()
	h.HandleError(w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type to contain application/json, got %q", ct)
	}
}

func TestBootstrapNoUserContextWithoutHeader(t *testing.T) {
	cfg := Config{Port: "0", SandboxID: "sandbox-test-001"}
	h := NewHandler(cfg.SandboxID)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	// No X-Authenticated-User header set.
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	var resp BootstrapResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode bootstrap response: %v", err)
	}

	if resp.User != "" {
		t.Errorf("expected empty user when no header set, got %q", resp.User)
	}
}
