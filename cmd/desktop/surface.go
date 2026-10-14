package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// desktopBridgeScriptPath is served by the native app, never the backend.
// The frontend reads window.__CHOIR_BRIDGE to route passkey ceremonies
// through /desktop-auth/start-session (frontend/src/lib/auth.js).
const desktopBridgeScriptPath = "/__choir_desktop/bridge.js"

const desktopBridgeScript = "window.__CHOIR_BRIDGE = true;\n"

// maxInjectedDocumentBytes bounds the HTML documents the app rewrites. The
// Choir index document is under 1 KiB; anything larger passes through
// unmodified rather than being buffered.
const maxInjectedDocumentBytes = 4 << 20

// assetHandler is the Wails asset handler: every request the window makes
// arrives here. The native auth entrypoint and the bridge script are served
// locally; everything else is proxied to the backend through the native
// session (desktop_auth.go), so the window renders the server's platform
// shell or computer surface rather than a copy compiled into the app.
func assetHandler(session *desktopSession) http.Handler {
	proxy := session.proxy()

	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		director(request)
		// Let the Go transport negotiate and transparently decode
		// compression so HTML can be rewritten as plain bytes.
		request.Header.Del("Accept-Encoding")
	}
	modifyResponse := proxy.ModifyResponse
	proxy.ModifyResponse = func(response *http.Response) error {
		if err := modifyResponse(response); err != nil {
			return err
		}
		return injectBridgeIntoDocument(response)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, _ error) {
		if wantsDocument(r) {
			writeOfflinePage(w, session.backend.String())
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "backend request failed"})
	}

	logRequests := os.Getenv("CHOIR_DESKTOP_LOG_REQUESTS") != ""
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if logRequests {
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			w = recorder
			defer func() { log.Printf("request %s %s -> %d", r.Method, r.URL.RequestURI(), recorder.status) }()
		}
		switch {
		case r.URL.Path == "/desktop-auth/start-session":
			session.handleStart(w, r)
		case r.URL.Path == desktopBridgeScriptPath:
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, desktopBridgeScript)
		case isBlockedDesktopAuthRoute(r.URL.Path):
			http.NotFound(w, r)
		default:
			proxy.ServeHTTP(w, r)
		}
	})
}

// injectBridgeIntoDocument adds the bridge <script> to a successful HTML
// document response, before </head>.
func injectBridgeIntoDocument(response *http.Response) error {
	if response.Request == nil || response.Request.Method != http.MethodGet ||
		response.StatusCode != http.StatusOK ||
		!strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/html") ||
		response.Header.Get("Content-Encoding") != "" {
		return nil
	}
	if response.ContentLength > maxInjectedDocumentBytes {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxInjectedDocumentBytes+1))
	if err != nil {
		return fmt.Errorf("read backend document: %w", err)
	}
	if len(body) > maxInjectedDocumentBytes {
		// Too large to rewrite: stream it through unchanged.
		response.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(body), response.Body), response.Body}
		return nil
	}
	_ = response.Body.Close()

	tag := []byte(`<script src="` + desktopBridgeScriptPath + `"></script>`)
	if idx := bytes.Index(bytes.ToLower(body), []byte("</head>")); idx >= 0 {
		rewritten := make([]byte, 0, len(body)+len(tag))
		rewritten = append(rewritten, body[:idx]...)
		rewritten = append(rewritten, tag...)
		body = append(rewritten, body[idx:]...)
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))
	response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	response.Header.Del("ETag")
	log.Printf("surface: %s from backend (%d bytes, bridge injected)", response.Request.URL.Path, len(body))
	return nil
}

// statusRecorder records the response status for CHOIR_DESKTOP_LOG_REQUESTS.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Flush() {
	if flusher, ok := s.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func wantsDocument(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
		return false
	}
	accept := r.Header.Get("Accept")
	return accept == "" || strings.Contains(accept, "text/html")
}

func writeOfflinePage(w http.ResponseWriter, backend string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = fmt.Fprintf(w, offlinePage, html.EscapeString(backend))
}

const offlinePage = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>Choir</title>
<style>
  :root { color-scheme: light dark; }
  body { margin: 0; min-height: 100vh; display: grid; place-items: center;
         font: 15px/1.5 -apple-system, BlinkMacSystemFont, sans-serif;
         background: #050912; color: #F7FAFF; }
  main { max-width: 26rem; padding: 2rem; text-align: center; }
  p { color: #9AA9C0; }
  button { font: inherit; padding: .5rem 1.25rem; border-radius: .5rem;
           border: 1px solid rgba(133,159,211,.4); background: #0D1628; color: inherit; }
</style></head>
<body><main>
  <h1>Choir cannot reach its server</h1>
  <p>The desktop app talks to <code>%s</code>. Check your connection, then try again.</p>
  <button onclick="location.reload()">Try again</button>
</main></body></html>
`
