package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// docs/problems/recovering-computer-shows-raw-json-2026-10-09.md.
// Failure modes pinned: a browser page load shows raw JSON; the page never
// retries; API callers lose the structured refusal.
func TestComputerSurfaceResolveErrorIsAPageForBrowsers(t *testing.T) {
	refusal := &vmctl.RecoveryRefusalError{Kind: "projection_base_missing", Reason: "required base is missing for an existing chain", RetryAfterSeconds: 60}

	page := httptest.NewRequest(http.MethodGet, "/", nil)
	page.Header.Set("Accept", "text/html,application/xhtml+xml")
	rec := httptest.NewRecorder()
	writeComputerSurfaceResolveError(rec, page, refusal)
	body := rec.Body.String()
	if rec.Code != http.StatusServiceUnavailable || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("browser refusal: status=%d type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Retry-After") == "" || !strings.Contains(body, `http-equiv="refresh"`) {
		t.Fatalf("browser refusal page does not retry: %s", body)
	}
	if strings.Contains(body, `"error"`) || !strings.Contains(body, "restored") {
		t.Fatalf("browser refusal page is not plain words: %s", body)
	}

	rec = httptest.NewRecorder()
	writeComputerSurfaceResolveError(rec, page, context.DeadlineExceeded)
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") || !strings.Contains(rec.Body.String(), `content="5"`) {
		t.Fatalf("browser timeout page: type=%q body=%s", rec.Header().Get("Content-Type"), rec.Body.String())
	}

	api := httptest.NewRequest(http.MethodGet, "/", nil)
	api.Header.Set("Accept", "application/json")
	rec = httptest.NewRecorder()
	writeComputerSurfaceResolveError(rec, api, refusal)
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") || !strings.Contains(rec.Body.String(), "projection_base_missing") {
		t.Fatalf("API caller lost the structured refusal: type=%q body=%s", rec.Header().Get("Content-Type"), rec.Body.String())
	}
}

// docs/problems/owner-computer-stranded-after-vmctl-restart-2026-10-10.md.
// Failure modes pinned: a running-but-unanswering computer shows a page that
// only reloads, so the owner has no way to restart it; the restart page
// reloads under a click in flight; other refusals offer a restart.
func TestComputerSurfaceOffersRestartOnlyForUnansweringComputer(t *testing.T) {
	page := httptest.NewRequest(http.MethodGet, "/", nil)
	page.Header.Set("Accept", "text/html")

	rec := httptest.NewRecorder()
	writeComputerSurfaceResolveError(rec, page, &vmctl.RecoveryRefusalError{Kind: "guest_reattach_pending", Reason: "computer is still running but did not answer reattach", RetryAfterSeconds: 15})
	body := rec.Body.String()
	if !strings.Contains(body, "data-computer-restart") || !strings.Contains(body, "/lifecycle/restart") {
		t.Fatalf("unanswering computer page offers no restart: %s", body)
	}
	if strings.Contains(body, `http-equiv="refresh"`) || !strings.Contains(body, "clearTimeout(reload)") {
		t.Fatalf("restart page reload is not cancelled by the click: %s", body)
	}
	if strings.Contains(body, "cold-recover") {
		t.Fatalf("restart page runs a cold recovery: %s", body)
	}

	rec = httptest.NewRecorder()
	writeComputerSurfaceResolveError(rec, page, &vmctl.RecoveryRefusalError{Kind: "projection_base_missing", RetryAfterSeconds: 60})
	if strings.Contains(rec.Body.String(), "data-computer-restart") {
		t.Fatalf("a restoring computer was offered a restart: %s", rec.Body.String())
	}
}
