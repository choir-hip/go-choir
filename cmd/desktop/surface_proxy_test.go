package main

// Failure modes for serving the backend's surface in the desktop window
// (written before the code, per the repo testing rule):
//
//  1. The backend's HTML document reaches the renderer without the desktop
//     bridge flag, so the passkey path falls back to navigator.credentials,
//     which WKWebView cannot satisfy.
//  2. A gzip-encoded HTML response is rewritten as compressed bytes, or keeps
//     a Content-Encoding header that no longer matches the body.
//  3. Content-Length is stale after injection and the document is truncated.
//  4. Non-HTML responses (JS chunks, JSON) are altered.
//  5. Deep links ("/?app=files", "/some/path") are served from a local copy
//     instead of the backend, re-creating the frozen embedded UI.
//  6. When the backend is unreachable the window shows raw JSON instead of a
//     page that says so; API callers still need JSON.
//  7. The bridge script itself is fetched from the backend instead of being
//     served by the native app.

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const testSurfaceHTML = `<!DOCTYPE html><html><head><title>Choir</title>` +
	`<script type="module" src="/assets/index-abc.js"></script></head>` +
	`<body><div id="app"></div></body></html>`

func surfaceBackend(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/assets/"):
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = io.WriteString(w, "console.log('<head></head>')")
		case strings.HasPrefix(r.URL.Path, "/api/"):
			writeJSON(w, http.StatusOK, map[string]string{"html": "</head>"})
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				w.Header().Set("Content-Encoding", "gzip")
				gz := gzip.NewWriter(w)
				_, _ = io.WriteString(gz, testSurfaceHTML)
				_ = gz.Close()
				return
			}
			_, _ = io.WriteString(w, testSurfaceHTML)
		}
	}))
}

func TestDesktopSurfaceComesFromBackendWithBridgeFlag(t *testing.T) {
	backend := surfaceBackend(t)
	defer backend.Close()
	handler := assetHandler(mustDesktopSession(t, backend.URL))

	for _, target := range []string{"/", "/?app=files", "/some/deep/path"} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.Header.Set("Accept-Encoding", "gzip, deflate, br")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", target, response.Code)
		}
		if got := response.Header().Get("Content-Encoding"); got != "" {
			t.Fatalf("%s: renderer received Content-Encoding %q for a rewritten body", target, got)
		}
		body := response.Body.String()
		if !strings.Contains(body, `<div id="app">`) {
			t.Fatalf("%s: body is not the backend document: %q", target, body)
		}
		tag := `<script src="` + desktopBridgeScriptPath + `"></script>`
		if strings.Count(body, tag) != 1 || strings.Index(body, tag) > strings.Index(body, "</head>") {
			t.Fatalf("%s: bridge script not injected once into <head>: %q", target, body)
		}
		if got := response.Header().Get("Content-Length"); got != "" && got != strconv.Itoa(len(body)) {
			t.Fatalf("%s: Content-Length = %s, body length = %d", target, got, len(body))
		}
	}
}

func TestDesktopSurfaceLeavesNonHTMLUntouched(t *testing.T) {
	backend := surfaceBackend(t)
	defer backend.Close()
	handler := assetHandler(mustDesktopSession(t, backend.URL))

	for target, want := range map[string]string{
		"/assets/index-abc.js": "console.log('<head></head>')",
		"/api/shell/bootstrap": "{\"html\":\"\\u003c/head\\u003e\"}\n",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.String() != want {
			t.Fatalf("%s: status = %d body = %q, want %q", target, response.Code, response.Body.String(), want)
		}
	}
}

func TestDesktopBridgeScriptIsServedNatively(t *testing.T) {
	backendHits := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendHits++
		http.NotFound(w, r)
	}))
	defer backend.Close()

	request := httptest.NewRequest(http.MethodGet, desktopBridgeScriptPath, nil)
	response := httptest.NewRecorder()
	assetHandler(mustDesktopSession(t, backend.URL)).ServeHTTP(response, request)

	if response.Code != http.StatusOK || backendHits != 0 {
		t.Fatalf("status = %d, backend hits = %d", response.Code, backendHits)
	}
	if !strings.HasPrefix(response.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("content type = %q", response.Header().Get("Content-Type"))
	}
	if !strings.Contains(response.Body.String(), "__CHOIR_BRIDGE = true") {
		t.Fatalf("bridge script body = %q", response.Body.String())
	}
}

func TestDesktopBackendUnreachableShowsOfflinePage(t *testing.T) {
	backend := httptest.NewServer(http.NotFoundHandler())
	backendURL := backend.URL
	backend.Close()
	handler := assetHandler(mustDesktopSession(t, backendURL))

	page := httptest.NewRequest(http.MethodGet, "/", nil)
	page.Header.Set("Accept", "text/html,application/xhtml+xml")
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, page)
	if pageResponse.Code != http.StatusBadGateway ||
		!strings.HasPrefix(pageResponse.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(pageResponse.Body.String(), "cannot reach") {
		t.Fatalf("document: status = %d type = %q body = %q",
			pageResponse.Code, pageResponse.Header().Get("Content-Type"), pageResponse.Body.String())
	}

	api := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	apiResponse := httptest.NewRecorder()
	handler.ServeHTTP(apiResponse, api)
	if apiResponse.Code != http.StatusBadGateway ||
		!strings.HasPrefix(apiResponse.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("api: status = %d type = %q", apiResponse.Code, apiResponse.Header().Get("Content-Type"))
	}
}
