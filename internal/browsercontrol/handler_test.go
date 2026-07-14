//go:build comprehensive

package browsercontrol

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func testBrowserSetup(t *testing.T) *Handler {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "browser.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	handler := NewHandler(provideriface.Config{SandboxID: "sandbox-test", StorePath: dbPath}, s, events.NewEventBus())
	t.Cleanup(func() {
		handler.Close()
		_ = s.Close()
	})
	return handler
}

func authenticatedRequest(method, path, body, user string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if user != "" {
		req.Header.Set("X-Authenticated-User", user)
	}
	return req
}

func runtimeHandlerRequest(t *testing.T, handler http.HandlerFunc, method, path, body, user string) *httptest.ResponseRecorder {
	t.Helper()
	req := authenticatedRequest(method, path, body, user)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func allowPrivateSourceFetchForTest(t *testing.T) {
	t.Helper()
	previous := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(previous) })
}

func TestBrowserCapabilitiesRequireAuthAndReportUnavailable(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)

	unauth := authenticatedRequest(http.MethodGet, "/api/browser/capabilities", "", "")
	unauthW := httptest.NewRecorder()
	handler.HandleBrowserCapabilities(unauthW, unauth)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want %d", unauthW.Code, http.StatusUnauthorized)
	}

	req := authenticatedRequest(http.MethodGet, "/api/browser/capabilities", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleBrowserCapabilities(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp browserCapabilitiesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Provider != "obscura" {
		t.Fatalf("provider = %q, want obscura", resp.Provider)
	}
	if resp.Available || resp.Configured || resp.Status != "not_configured" {
		t.Fatalf("unexpected unavailable response: %+v", resp)
	}
	if resp.Substrate != "frontend_iframe" {
		t.Fatalf("substrate = %q, want frontend_iframe", resp.Substrate)
	}
	if resp.Supports["navigate"] || resp.Supports["screenshot"] || resp.Supports["cdp_screenshot"] || resp.Supports["input"] || resp.Supports["cdp"] {
		t.Fatalf("unavailable support matrix should fail closed: %+v", resp.Supports)
	}
	if !resp.LegacyIframeAvailable {
		t.Fatalf("legacy iframe fallback should remain available until backend sessions are implemented")
	}
}

func TestBrowserCapabilitiesDetectConfiguredObscuraBinary(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin

	req := authenticatedRequest(http.MethodGet, "/api/browser/capabilities", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleBrowserCapabilities(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp browserCapabilitiesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Available || !resp.Configured || resp.Mode != "backend" || resp.Status != "ready" {
		t.Fatalf("unexpected available response: %+v", resp)
	}
	if resp.Substrate != "obscura_cli_fetch" {
		t.Fatalf("substrate = %q, want obscura_cli_fetch", resp.Substrate)
	}
	if !resp.Supports["navigate"] || !resp.Supports["text"] || !resp.Supports["html"] || !resp.Supports["links"] {
		t.Fatalf("snapshot support matrix missing expected support: %+v", resp.Supports)
	}
	if resp.Supports["screenshot"] || resp.Supports["cdp_screenshot"] || resp.Supports["bounded_input"] || resp.Supports["input"] || resp.Supports["cdp"] {
		t.Fatalf("unexpected support matrix: %+v", resp.Supports)
	}
}

func TestBrowserCapabilitiesReportOptInCDPScreenshotSubstrate(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin
	handler.cfg.ObscuraCDPScreenshots = true

	req := authenticatedRequest(http.MethodGet, "/api/browser/capabilities", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleBrowserCapabilities(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp browserCapabilitiesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Substrate != "obscura_cli_fetch+obscura_cdp_screenshot" {
		t.Fatalf("substrate = %q, want hybrid cdp screenshot substrate", resp.Substrate)
	}
	if !resp.Supports["screenshot"] || !resp.Supports["cdp_screenshot"] || !resp.Supports["bounded_input"] || !resp.Supports["fill"] || !resp.Supports["click"] {
		t.Fatalf("cdp bounded control support missing: %+v", resp.Supports)
	}
	if resp.Supports["input"] || resp.Supports["cdp"] {
		t.Fatalf("cdp screenshot mode must not claim generic input/cdp: %+v", resp.Supports)
	}
}

func TestBrowserSessionsNavigateThroughOwnerScopedBackendSnapshot(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "links" ]; then
  printf 'https://example.com/learn\tLearn more\n'
elif [ "$mode" = "html" ]; then
  printf '<!doctype html><title>Example Backend Page</title><h1>Example Backend Page</h1>'
else
  printf 'Example Backend Page\n\nSnapshot from fake Obscura\n'
fi
`), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin

	createW := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.OwnerID != "user-alice" || created.Mode != "backend" || created.State != types.BrowserSessionIdle {
		t.Fatalf("unexpected created session: %+v", created)
	}

	otherUserW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodGet, "/api/browser/sessions/"+created.SessionID, "", "user-bob")
	if otherUserW.Code != http.StatusNotFound {
		t.Fatalf("other user status = %d, want %d", otherUserW.Code, http.StatusNotFound)
	}

	navigateW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/path#fragment"}`, "user-alice")
	if navigateW.Code != http.StatusOK {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusOK, navigateW.Body.String())
	}
	var navigated types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&navigated); err != nil {
		t.Fatalf("decode navigate: %v", err)
	}
	if navigated.State != types.BrowserSessionReady {
		t.Fatalf("state = %q, want %q; session: %+v", navigated.State, types.BrowserSessionReady, navigated)
	}
	if navigated.ExecutionScope != "host_process" {
		t.Fatalf("execution_scope = %q, want host_process", navigated.ExecutionScope)
	}
	if navigated.CurrentURL != "https://example.com/path" {
		t.Fatalf("current_url = %q, want normalized URL without fragment", navigated.CurrentURL)
	}
	if navigated.Title != "Example Backend Page" {
		t.Fatalf("title = %q, want first snapshot line", navigated.Title)
	}
	if !strings.Contains(navigated.TextSnapshot, "Snapshot from fake Obscura") {
		t.Fatalf("text_snapshot missing fake output: %q", navigated.TextSnapshot)
	}
	if !strings.Contains(navigated.HTMLSnapshot, "<title>Example Backend Page</title>") {
		t.Fatalf("html_snapshot missing fake output: %q", navigated.HTMLSnapshot)
	}
	if len(navigated.Links) != 1 || navigated.Links[0].URL != "https://example.com/learn" || navigated.Links[0].Text != "Learn more" {
		t.Fatalf("links = %+v, want extracted fake link", navigated.Links)
	}

	traceID := browserSessionTraceID(created.SessionID)
	events, err := handler.store.ListEventsByTrajectory(context.Background(), "user-alice", traceID, 10)
	if err != nil {
		t.Fatalf("list browser trace events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("browser trace event count = %d, want 2", len(events))
	}
	if events[0].Kind != types.EventBrowserSessionCreated || events[1].Kind != types.EventBrowserNavigationCompleted {
		t.Fatalf("browser trace kinds = %q, %q", events[0].Kind, events[1].Kind)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("decode browser completion payload: %v", err)
	}
	if int(payload["links_count"].(float64)) != 1 {
		t.Fatalf("links_count payload = %+v, want 1", payload)
	}
	if int(payload["html_snapshot_bytes"].(float64)) == 0 {
		t.Fatalf("html_snapshot_bytes payload = %+v, want nonzero", payload)
	}
}

func TestBrowserSessionRejectsDirectWorldBinding(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)
	forged := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{"vm_id":"vm-forged"}`, "user-alice")
	if forged.Code != http.StatusBadRequest {
		t.Fatalf("forged vm_id status = %d, want 400; body=%s", forged.Code, forged.Body.String())
	}
}

func TestBrowserSessionNavigateKeepsTextWhenOptionalSnapshotDumpsFail(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "text" ]; then
  printf 'Readable Source Page\n\nPrimary source text from fake Obscura\n'
  exit 0
fi
printf 'fake optional %s dump failed\n' "$mode" >&2
exit 2
`), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin

	createW := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	navigateW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
	if navigateW.Code != http.StatusOK {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusOK, navigateW.Body.String())
	}
	var navigated types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&navigated); err != nil {
		t.Fatalf("decode navigate: %v", err)
	}
	if navigated.State != types.BrowserSessionReady {
		t.Fatalf("state = %q, want %q; session: %+v", navigated.State, types.BrowserSessionReady, navigated)
	}
	if !strings.Contains(navigated.TextSnapshot, "Primary source text from fake Obscura") {
		t.Fatalf("text_snapshot missing primary text: %q", navigated.TextSnapshot)
	}
	if navigated.HTMLSnapshot != "" {
		t.Fatalf("html_snapshot = %q, want empty optional artifact", navigated.HTMLSnapshot)
	}
	if len(navigated.Links) != 0 {
		t.Fatalf("links = %+v, want none after optional dump failure", navigated.Links)
	}
	if len(navigated.SnapshotWarnings) != 2 {
		t.Fatalf("snapshot_warnings = %+v, want links/html warnings", navigated.SnapshotWarnings)
	}
	joinedWarnings := strings.Join(navigated.SnapshotWarnings, "\n")
	if !strings.Contains(joinedWarnings, "links") || !strings.Contains(joinedWarnings, "html") {
		t.Fatalf("snapshot_warnings = %+v, want links/html dump warnings", navigated.SnapshotWarnings)
	}

	events, err := handler.store.ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list browser trace events: %v", err)
	}
	if len(events) != 2 || events[1].Kind != types.EventBrowserNavigationCompleted {
		t.Fatalf("browser trace events = %+v, want completed navigation", events)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("decode browser completion payload: %v", err)
	}
	if int(payload["snapshot_warning_count"].(float64)) != 2 {
		t.Fatalf("snapshot warning payload = %+v, want count 2", payload)
	}
}

func TestBrowserSessionNavigateUsesHTMLFallbackWhenTextSnapshotEmpty(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "text" ]; then
  exit 0
fi
if [ "$mode" = "html" ]; then
  printf '<!doctype html><title>Readable HTML Fallback</title><main><h1>Readable HTML Fallback</h1><p>Source text recovered from html. This fallback has enough article body text to prove that the HTML-derived source surface is useful without relying on a declared alternate. It includes a second sentence about citations, source windows, and durable inspection so the extraction quality check does not accept a skeletal page title alone.</p><p>The source reader should preserve this prose as the readable browser snapshot while still keeping the raw HTML artifact for debugging.</p><script>ignored()</script></main>'
  exit 0
fi
if [ "$mode" = "links" ]; then
  printf 'https://example.com/source\tSource link\n'
  exit 0
fi
`), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin

	createW := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	navigateW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
	if navigateW.Code != http.StatusOK {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusOK, navigateW.Body.String())
	}
	var navigated types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&navigated); err != nil {
		t.Fatalf("decode navigate: %v", err)
	}
	if navigated.State != types.BrowserSessionReady {
		t.Fatalf("state = %q, want %q; session: %+v", navigated.State, types.BrowserSessionReady, navigated)
	}
	if !strings.Contains(navigated.TextSnapshot, "Source text recovered from html.") {
		t.Fatalf("text_snapshot missing html fallback text: %q", navigated.TextSnapshot)
	}
	if !strings.Contains(navigated.HTMLSnapshot, "<title>Readable HTML Fallback</title>") {
		t.Fatalf("html_snapshot missing raw html: %q", navigated.HTMLSnapshot)
	}
	if len(navigated.Links) != 1 || navigated.Links[0].Text != "Source link" {
		t.Fatalf("links = %+v, want extracted fake link", navigated.Links)
	}
	if len(navigated.SnapshotWarnings) != 1 || !strings.Contains(navigated.SnapshotWarnings[0], "used html readable fallback") {
		t.Fatalf("snapshot_warnings = %+v, want html fallback warning", navigated.SnapshotWarnings)
	}

	events, err := handler.store.ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list browser trace events: %v", err)
	}
	if len(events) != 2 || events[1].Kind != types.EventBrowserNavigationCompleted {
		t.Fatalf("browser trace events = %+v, want completed navigation", events)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("decode browser completion payload: %v", err)
	}
	if int(payload["snapshot_warning_count"].(float64)) != 1 {
		t.Fatalf("snapshot warning payload = %+v, want count 1", payload)
	}
}

func TestBrowserSessionNavigateUsesDeclaredMarkdownAlternateWhenHTMLFallbackLowContent(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	handler := testBrowserSetup(t)
	markdown := strings.Repeat("Similarity search article text recovered from the declared Markdown alternate. ", 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/docs/index.md" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = fmt.Fprintf(w, "# Search\n# Similarity search\n\n%s\n", markdown)
	}))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	htmlShell := fmt.Sprintf(`<!doctype html><html><head><title>%s/docs/</title><link rel="canonical" href="%s/docs/"><link rel="alternate" type="text/markdown" href="index.md"></head><body></body></html>`, server.URL, server.URL)
	if err := os.WriteFile(bin, []byte(fmt.Sprintf(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "text" ]; then
  exit 0
fi
if [ "$mode" = "html" ]; then
  printf %%s %q
  exit 0
fi
if [ "$mode" = "links" ]; then
  exit 0
fi
`, htmlShell)), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin

	createW := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	navigateW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
	if navigateW.Code != http.StatusOK {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusOK, navigateW.Body.String())
	}
	var navigated types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&navigated); err != nil {
		t.Fatalf("decode navigate: %v", err)
	}
	if navigated.State != types.BrowserSessionReady {
		t.Fatalf("state = %q, want %q; session: %+v", navigated.State, types.BrowserSessionReady, navigated)
	}
	if !strings.Contains(navigated.TextSnapshot, "Similarity search article text recovered") {
		t.Fatalf("text_snapshot missing markdown alternate text: %q", navigated.TextSnapshot)
	}
	if !strings.Contains(navigated.HTMLSnapshot, `rel="alternate"`) {
		t.Fatalf("html_snapshot missing original html shell: %q", navigated.HTMLSnapshot)
	}
	if len(navigated.SnapshotWarnings) != 1 || !strings.Contains(navigated.SnapshotWarnings[0], "used declared markdown alternate") {
		t.Fatalf("snapshot_warnings = %+v, want declared markdown alternate warning", navigated.SnapshotWarnings)
	}

	events, err := handler.store.ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list browser trace events: %v", err)
	}
	if len(events) != 2 || events[1].Kind != types.EventBrowserNavigationCompleted {
		t.Fatalf("browser trace events = %+v, want completed navigation", events)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("decode browser completion payload: %v", err)
	}
	if int(payload["snapshot_warning_count"].(float64)) != 1 {
		t.Fatalf("snapshot warning payload = %+v, want count 1", payload)
	}
}

func TestBrowserSessionNavigateUsesDeclaredMarkdownAlternateFromCanonicalShell(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	handler := testBrowserSetup(t)
	markdown := strings.Repeat("Similarity search article text recovered from the canonical page Markdown alternate. ", 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html><html><head><title>Search - Qdrant</title><link rel="alternate" type="text/markdown" href="%s/docs/index.md"></head><body><main><h1>Search</h1></main></body></html>`, serverURLFromRequest(r))
		case "/docs/index.md":
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			_, _ = fmt.Fprintf(w, "# Search\n# Similarity search\n\n%s\n", markdown)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	htmlShell := fmt.Sprintf(`<!doctype html><html><head><title>%s/source</title><link rel="canonical" href="%s/docs/"><meta http-equiv="refresh" content="0; url=%s/docs/"></head><body></body></html>`, server.URL, server.URL, server.URL)
	if err := os.WriteFile(bin, []byte(fmt.Sprintf(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "text" ]; then
  exit 0
fi
if [ "$mode" = "html" ]; then
  printf %%s %q
  exit 0
fi
if [ "$mode" = "links" ]; then
  exit 0
fi
`, htmlShell)), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin

	createW := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	navigateW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
	if navigateW.Code != http.StatusOK {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusOK, navigateW.Body.String())
	}
	var navigated types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&navigated); err != nil {
		t.Fatalf("decode navigate: %v", err)
	}
	if navigated.State != types.BrowserSessionReady {
		t.Fatalf("state = %q, want %q; session: %+v", navigated.State, types.BrowserSessionReady, navigated)
	}
	if !strings.Contains(navigated.TextSnapshot, "canonical page Markdown alternate") {
		t.Fatalf("text_snapshot missing canonical markdown alternate text: %q", navigated.TextSnapshot)
	}
	if !strings.Contains(navigated.HTMLSnapshot, `http-equiv="refresh"`) {
		t.Fatalf("html_snapshot missing original redirect shell: %q", navigated.HTMLSnapshot)
	}
	if len(navigated.SnapshotWarnings) != 1 || !strings.Contains(navigated.SnapshotWarnings[0], "used declared markdown alternate") {
		t.Fatalf("snapshot_warnings = %+v, want declared markdown alternate warning", navigated.SnapshotWarnings)
	}
}

func TestFetchBrowserDeclaredAlternateTextInfersHTMLFromURL(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = fmt.Fprint(w, `<!doctype html><html><head><title>Readable alternate</title></head><body><main><p>Recovered article prose without raw markup.</p></main></body></html>`)
	}))
	defer server.Close()

	result, err := fetchBrowserDeclaredAlternateText(context.Background(), server.Client(), server.URL+"/article.html")
	if err != nil {
		t.Fatalf("fetch declared alternate: %v", err)
	}
	if !strings.Contains(result.Text, "Recovered article prose without raw markup.") {
		t.Fatalf("text = %q, want readable article prose", result.Text)
	}
	if strings.Contains(result.Text, "<main>") {
		t.Fatalf("text = %q, want HTML markup removed", result.Text)
	}
}

func serverURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func TestBrowserSessionNavigateFailsWhenTextSnapshotFails(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "text" ]; then
  printf 'fake text dump failed\n' >&2
  exit 2
fi
printf 'optional artifact should not matter\n'
`), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin

	createW := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	navigateW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
	if navigateW.Code != http.StatusBadGateway {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusBadGateway, navigateW.Body.String())
	}
	var navigated types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&navigated); err != nil {
		t.Fatalf("decode navigate: %v", err)
	}
	if navigated.State != types.BrowserSessionError {
		t.Fatalf("state = %q, want %q; session: %+v", navigated.State, types.BrowserSessionError, navigated)
	}
	if !strings.Contains(navigated.Error, "text fetch failed") {
		t.Fatalf("error = %q, want text fetch failure", navigated.Error)
	}
	if navigated.TextSnapshot != "" {
		t.Fatalf("text_snapshot = %q, want empty on text failure", navigated.TextSnapshot)
	}
}

func TestBrowserSessionNavigateFailsClosedWhenBackendUnavailable(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)

	createW := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.State != types.BrowserSessionUnavailable {
		t.Fatalf("created state = %q, want unavailable", created.State)
	}

	navigateW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com"}`, "user-alice")
	if navigateW.Code != http.StatusServiceUnavailable {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusServiceUnavailable, navigateW.Body.String())
	}
	var blocked types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&blocked); err != nil {
		t.Fatalf("decode blocked: %v", err)
	}
	if blocked.State != types.BrowserSessionUnavailable || blocked.Error == "" {
		t.Fatalf("unexpected blocked session: %+v", blocked)
	}
	events, err := handler.store.ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list blocked browser trace events: %v", err)
	}
	if len(events) != 2 || events[1].Kind != types.EventBrowserNavigationFailed {
		t.Fatalf("blocked browser trace events = %+v, want create + navigation failed", events)
	}
}

func TestBrowserSessionCloseIsOwnerScopedIdempotentAndPreventsNavigation(t *testing.T) {
	t.Parallel()
	handler := testBrowserSetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
case "$mode" in
  links) printf 'https://example.com/learn\tLearn more\n' ;;
  html) printf '<title>Example Backend Page</title>' ;;
  *) printf 'Example Backend Page\n' ;;
esac
`), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	handler.cfg.ObscuraPath = bin

	createW := runtimeHandlerRequest(t, handler.HandleBrowserSessionsRoot, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	otherCloseW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/close", `{}`, "user-bob")
	if otherCloseW.Code != http.StatusNotFound {
		t.Fatalf("other user close status = %d, want %d", otherCloseW.Code, http.StatusNotFound)
	}

	closeW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/close", `{}`, "user-alice")
	if closeW.Code != http.StatusOK {
		t.Fatalf("close status = %d, want %d; body: %s", closeW.Code, http.StatusOK, closeW.Body.String())
	}
	var closed types.BrowserSessionRecord
	if err := json.NewDecoder(closeW.Body).Decode(&closed); err != nil {
		t.Fatalf("decode close: %v", err)
	}
	if closed.State != types.BrowserSessionClosed {
		t.Fatalf("closed state = %q, want %q", closed.State, types.BrowserSessionClosed)
	}

	closeAgainW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/close", `{}`, "user-alice")
	if closeAgainW.Code != http.StatusOK {
		t.Fatalf("close again status = %d, want %d; body: %s", closeAgainW.Code, http.StatusOK, closeAgainW.Body.String())
	}

	navigateW := runtimeHandlerRequest(t, handler.HandleBrowserSessionRouter, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com"}`, "user-alice")
	if navigateW.Code != http.StatusConflict {
		t.Fatalf("navigate closed status = %d, want %d; body: %s", navigateW.Code, http.StatusConflict, navigateW.Body.String())
	}

	events, err := handler.store.ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list close trace events: %v", err)
	}
	if len(events) != 2 || events[0].Kind != types.EventBrowserSessionCreated || events[1].Kind != types.EventBrowserSessionClosed {
		t.Fatalf("close trace events = %+v, want create + single close", events)
	}
}
func TestHandlerCloseClosesEveryCDPSession(t *testing.T) {
	t.Parallel()
	first := &browserCDPSession{}
	second := &browserCDPSession{}
	handler := NewHandler(provideriface.Config{}, nil, nil)
	handler.browserCDP["first"] = first
	handler.browserCDP["second"] = second

	handler.Close()
	handler.Close()

	if !first.closed || !second.closed {
		t.Fatalf("Close left browser CDP sessions open: first=%v second=%v", first.closed, second.closed)
	}
	if len(handler.browserCDP) != 0 {
		t.Fatalf("Close left %d browser CDP sessions registered", len(handler.browserCDP))
	}
}
