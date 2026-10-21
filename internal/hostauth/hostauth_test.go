package hostauth

// Failure modes this package must hold against (written before the code,
// docs/problems/loopback-is-a-credential-2026-10-14.md):
//
//  1. enforce with no credential loaded (file missing, empty, short):
//     every internal call is refused (fail closed), never allowed.
//  2. a missing or wrong token: refused in enforce, allowed and logged in
//     observe, allowed silently in off (the legacy rollback).
//  3. the token reaches a guest: servers strip the header at ingress, so a
//     reverse proxy cannot forward it; clients attach it only to loopback
//     destinations.
//  4. the token leaks through logs or status: no log line or Status
//     string contains it.
//  5. an ambiguous request (two token headers) is never authorized.
//  6. an unknown mode string fails closed (enforce), not open.
//  7. a request that never passed the middleware is not authorized in
//     enforce.
//  8. observe logging floods the journal: refusals are rate limited per
//     route and reason.

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef"

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })
	return &buf
}

// serveThroughMiddleware runs one request through Middleware and returns
// what the inner handler saw: whether Authorize passed and whether the
// token header was still present.
func serveThroughMiddleware(t *testing.T, header []string) (authorized bool, headerSeen bool) {
	t.Helper()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorized = Authorize(r)
		headerSeen = r.Header.Get(Header) != ""
	})
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8083/internal/vmctl/list", nil)
	req.RemoteAddr = "127.0.0.1:40000"
	for _, v := range header {
		req.Header.Add(Header, v)
	}
	Middleware(inner).ServeHTTP(httptest.NewRecorder(), req)
	return authorized, headerSeen
}

func TestAuthorizeMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mode   Mode
		token  string
		header []string
		want   bool
	}{
		{"enforce right credential", ModeEnforce, testToken, []string{testToken}, true},
		{"enforce missing credential", ModeEnforce, testToken, nil, false},
		{"enforce wrong credential", ModeEnforce, testToken, []string{strings.Repeat("x", len(testToken))}, false},
		{"enforce wrong length credential", ModeEnforce, testToken, []string{"short"}, false},
		{"enforce two headers", ModeEnforce, testToken, []string{testToken, testToken}, false},
		{"enforce no credential loaded refuses right-looking token", ModeEnforce, "", []string{testToken}, false},
		{"enforce no credential loaded refuses missing token", ModeEnforce, "", nil, false},
		{"observe missing credential allowed", ModeObserve, testToken, nil, true},
		{"observe wrong credential allowed", ModeObserve, testToken, []string{"nope"}, true},
		{"observe right credential", ModeObserve, testToken, []string{testToken}, true},
		{"off missing credential allowed", ModeOff, "", nil, true},
		{"off wrong credential allowed", ModeOff, testToken, []string{"nope"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			captureLog(t)
			restore := SetForTesting(tc.mode, tc.token)
			defer restore()
			got, headerSeen := serveThroughMiddleware(t, tc.header)
			if got != tc.want {
				t.Fatalf("Authorize = %v, want %v", got, tc.want)
			}
			if headerSeen {
				t.Fatalf("token header reached the handler; it must be stripped at ingress")
			}
		})
	}
}

func TestRefusalLogsReasonNeverToken(t *testing.T) {
	buf := captureLog(t)
	restore := SetForTesting(ModeEnforce, testToken)
	defer restore()
	wrong := "WRONG-" + strings.Repeat("z", 40)
	if ok, _ := serveThroughMiddleware(t, []string{wrong}); ok {
		t.Fatal("wrong credential authorized")
	}
	out := buf.String()
	if !strings.Contains(out, "enforce: refused") || !strings.Contains(out, "wrong credential") {
		t.Fatalf("refusal log lacks mode and reason: %q", out)
	}
	if strings.Contains(out, testToken) || strings.Contains(out, wrong) {
		t.Fatalf("log leaked a credential: %q", out)
	}
}

func TestObserveLogsWouldRefuseOncePerRoute(t *testing.T) {
	buf := captureLog(t)
	restore := SetForTesting(ModeObserve, testToken)
	defer restore()
	for i := 0; i < 5; i++ {
		if ok, _ := serveThroughMiddleware(t, nil); !ok {
			t.Fatal("observe refused")
		}
	}
	if n := strings.Count(buf.String(), "observe: would refuse"); n != 1 {
		t.Fatalf("observe logged %d times for one route, want 1 (rate limited): %q", n, buf.String())
	}
}

func TestRequestThatSkippedMiddlewareIsNotAuthorizedInEnforce(t *testing.T) {
	captureLog(t)
	restore := SetForTesting(ModeEnforce, testToken)
	defer restore()
	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
	req.Header.Set(Header, testToken)
	if Authorize(req) {
		t.Fatal("a request that never passed the middleware was authorized")
	}
}

func TestMarkInternalAttachesTokenOnlyToLoopback(t *testing.T) {
	captureLog(t)
	restore := SetForTesting(ModeEnforce, testToken)
	defer restore()
	for _, target := range []string{"http://127.0.0.1:8083/x", "http://[::1]:8086/x", "http://localhost:8084/x", "http://127.0.0.2:8087/x"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		MarkInternal(req)
		if req.Header.Get("X-Internal-Caller") != "true" || req.Header.Get(Header) != testToken {
			t.Fatalf("%s: loopback request not marked with marker and token", target)
		}
	}
	for _, target := range []string{"http://10.200.0.2:8080/x", "http://172.16.0.1:8084/x", "https://choir.news/x", "http://localhost.evil.example:8083/x",
		// Node B DNATs 127.0.0.1:9xxx to a guest's :8085: never send the secret there.
		"http://127.0.0.1:9117/health", "http://127.0.0.1/x"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set(Header, "stale")
		MarkInternal(req)
		if req.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("%s: marker missing", target)
		}
		if req.Header.Get(Header) != "" {
			t.Fatalf("%s: token attached to a non-loopback destination", target)
		}
	}
}

func TestMarkInternalPortOverride(t *testing.T) {
	captureLog(t)
	t.Setenv(EnvPorts, " 18083 ")
	restore := SetForTesting(ModeEnforce, testToken)
	defer restore()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:18083/x", nil)
	MarkInternal(req)
	if req.Header.Get(Header) != testToken {
		t.Fatal("override port did not receive the credential")
	}
	req = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8083/x", nil)
	MarkInternal(req)
	if req.Header.Get(Header) != "" {
		t.Fatal("default port still received the credential after an override")
	}
}

func TestMarkInternalWithoutCredentialSendsNoToken(t *testing.T) {
	captureLog(t)
	restore := SetForTesting(ModeObserve, "")
	defer restore()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8083/x", nil)
	MarkInternal(req)
	if req.Header.Get(Header) != "" {
		t.Fatal("token header sent with no credential loaded")
	}
}

func TestLoadFromEnv(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good")
	if err := os.WriteFile(good, []byte(testToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("tooshort\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	credDir := filepath.Join(dir, "creds")
	if err := os.MkdirAll(credDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(credDir, CredentialName), []byte(testToken), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, mode, file, credDir string
		wantMode                  Mode
		wantToken                 bool
	}{
		{"unset is off", "", "", "", ModeOff, false},
		{"explicit file trims newline", "enforce", good, "", ModeEnforce, true},
		{"systemd credential directory", "observe", "", credDir, ModeObserve, true},
		{"short token is a load error", "enforce", short, "", ModeEnforce, false},
		{"missing file is a load error", "enforce", filepath.Join(dir, "absent"), "", ModeEnforce, false},
		{"unknown mode fails closed", "enforec", good, "", ModeEnforce, true},
		{"mode is case and space tolerant", " Observe ", good, "", ModeObserve, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvMode, tc.mode)
			t.Setenv(EnvTokenFile, tc.file)
			t.Setenv("CREDENTIALS_DIRECTORY", tc.credDir)
			cfg := loadFromEnv()
			if cfg.mode != tc.wantMode {
				t.Fatalf("mode = %q, want %q", cfg.mode, tc.wantMode)
			}
			if cfg.hasToken != tc.wantToken {
				t.Fatalf("hasToken = %v, want %v (err %v)", cfg.hasToken, tc.wantToken, cfg.loadErr)
			}
			if tc.wantToken && cfg.raw != testToken {
				t.Fatalf("token not trimmed")
			}
			if strings.Contains(cfg.Status(), testToken) {
				t.Fatalf("Status leaked the token: %q", cfg.Status())
			}
		})
	}
}

// SEC-11: the credential reaches the proxy (8082) on loopback, so the
// proxy's internal routes can require it; still never a guest DNAT port.
func TestMarkInternalSendsCredentialToTheProxyPort(t *testing.T) {
	defer SetForTesting(ModeEnforce, "markinternal-proxy-port-token-0123456789abcdef")()
	for url, want := range map[string]bool{
		"http://127.0.0.1:8082/internal/platform/objects": true,
		"http://127.0.0.1:9077/":                          false,
		"http://10.200.1.2:8082/":                         false,
	} {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		MarkInternal(req)
		if got := req.Header.Get(Header) != ""; got != want {
			t.Errorf("MarkInternal(%s) sent the credential = %v, want %v", url, got, want)
		}
	}
}

// SEC-11: AuthorizeAtMost caps the configured mode for one route.
func TestAuthorizeAtMost(t *testing.T) {
	const token = "authorize-at-most-token-0123456789abcdef0123"
	check := func(configured, routeCap Mode, presented string) bool {
		defer SetForTesting(configured, token)()
		var got bool
		req := httptest.NewRequest(http.MethodGet, "/internal/x", nil)
		if presented != "" {
			req.Header.Set(Header, presented)
		}
		Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = AuthorizeAtMost(r, routeCap) })).
			ServeHTTP(httptest.NewRecorder(), req)
		return got
	}
	if check(ModeEnforce, ModeEnforce, "") {
		t.Fatal("enforce/enforce passed without a credential")
	}
	if !check(ModeEnforce, ModeEnforce, token) {
		t.Fatal("enforce/enforce refused the credential")
	}
	if !check(ModeEnforce, ModeObserve, "") {
		t.Fatal("a route capped at observe refused")
	}
	if !check(ModeObserve, ModeEnforce, "") {
		t.Fatal("a route cannot raise the service's observe to enforce")
	}
	if !check(ModeOff, ModeEnforce, "") {
		t.Fatal("hostauth off must stay off")
	}
}
