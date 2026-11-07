package shotsandbox

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixtureDist(t *testing.T) string {
	t.Helper()
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("<html>shell</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dist, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "assets", "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dist
}

func get(t *testing.T, h http.Handler, method, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The served frontend: the SPA shell, assets with their types, no escape from
// the dist directory, the /auth stub, and /api to the autoputer as the owner
// whatever identity the page claimed (choir-shot.mjs isProxied, serve, proxy).
func TestFrontendHandlerServesProxiesAndStubsAuth(t *testing.T) {
	var gotUser, gotPath atomic.Value
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser.Store(r.Header.Get("X-Authenticated-User"))
		gotPath.Store(r.URL.RequestURI())
		io.WriteString(w, "upstream")
	}))
	defer upstream.Close()
	port, _ := strconv.Atoi(upstream.URL[strings.LastIndex(upstream.URL, ":")+1:])
	h := NewFrontendHandler(fixtureDist(t), port, "owner")

	if rec := get(t, h, "GET", "/", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "shell") || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("/ = %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	if rec := get(t, h, "GET", "/some/spa/route", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "shell") {
		t.Fatalf("an unknown route must answer the SPA shell: %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, "GET", "/assets/app.js", nil); rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("asset = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := get(t, h, "GET", "/assets/missing.js", nil); rec.Code != 404 {
		t.Fatalf("a missing asset = %d, want 404 (not the shell)", rec.Code)
	}
	if rec := get(t, h, "GET", "/../../etc/passwd", nil); strings.Contains(rec.Body.String(), "root:") {
		t.Fatal("path traversal read outside the dist directory")
	}
	if rec := get(t, h, "GET", "/auth/session", nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"authenticated":true`) || !strings.Contains(rec.Body.String(), `"owner"`) {
		t.Fatalf("/auth/session = %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, "GET", "/auth/login", nil); rec.Code != 404 {
		t.Fatalf("other /auth urls = %d, want 404", rec.Code)
	}
	rec := get(t, h, "GET", "/api/thing?x=1", map[string]string{"X-Authenticated-User": "mallory"})
	if rec.Code != 200 || rec.Body.String() != "upstream" {
		t.Fatalf("/api = %d %q", rec.Code, rec.Body.String())
	}
	if gotUser.Load() != "owner" || gotPath.Load() != "/api/thing?x=1" {
		t.Fatalf("upstream saw user %v path %v; the identity is the server's, not the page's", gotUser.Load(), gotPath.Load())
	}
}

// A dead autoputer is a 502 naming it, not a hang or a blank page.
func TestFrontendHandlerUpstreamDownIsBadGateway(t *testing.T) {
	h := NewFrontendHandler(fixtureDist(t), 1, "owner")
	rec := get(t, h, "GET", "/api/x", nil)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "autoputer") {
		t.Fatalf("down upstream = %d %q", rec.Code, rec.Body.String())
	}
}

func TestResetDesktopPutsAnEmptyDesktopAndFailsOnRefusal(t *testing.T) {
	var body, method atomic.Value
	status := atomic.Int32{}
	status.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body.Store(string(b))
		method.Store(r.Method + " " + r.URL.Path + " " + r.Header.Get("X-Choir-Driver"))
		w.WriteHeader(int(status.Load()))
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := resetDesktop(ctx, srv.Client(), srv.URL); err != nil {
		t.Fatal(err)
	}
	if method.Load() != "PUT /api/desktop/state true" || !strings.Contains(body.Load().(string), `"windows":[]`) {
		t.Fatalf("reset sent %v %v", method.Load(), body.Load())
	}
	status.Store(500)
	if err := resetDesktop(ctx, srv.Client(), srv.URL); err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("a refused reset must fail the view, got %v", err)
	}
}

func TestAutoputerEnvIsScrubbedAndPrivate(t *testing.T) {
	t.Setenv("RUNTIME_GATEWAY_TOKEN_FILE", "/secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-x")
	env := strings.Join(autoputerEnv(Spec{StorePath: "/w/state", FilesRoot: "/w/files", WorkDir: "/w", Fontconfig: "/f.conf", PassEnv: []string{"X=1"}}, 4242), "\n")
	for _, want := range []string{"RUNTIME_STORE_PATH=/w/state", "AUTOPUTER_FILES_ROOT=/w/files", "AUTOPUTER_PORT=4242", "CHOIR_GUEST_TRUST_LOOPBACK=1", "RUNTIME_LLM_PROVIDER=stub", "HOME=/w/home", "TMPDIR=/w/tmp", "FONTCONFIG_FILE=/f.conf", "X=1"} {
		if !strings.Contains(env, want) {
			t.Errorf("env lacks %s:\n%s", want, env)
		}
	}
	if strings.Contains(env, "/secret") || strings.Contains(env, "sk-ant") || strings.Contains(env, "GATEWAY") {
		t.Fatalf("the candidate autoputer inherited a credential:\n%s", env)
	}
}

func TestTailBufferKeepsTheLastBytes(t *testing.T) {
	tail := &tailBuffer{max: 10}
	tail.Write([]byte("0123456789abcdef"))
	if got := tail.String(); got != "6789abcdef" {
		t.Fatalf("tail = %q", got)
	}
}
