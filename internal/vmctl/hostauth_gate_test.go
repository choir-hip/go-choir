package vmctl

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// gateThroughMiddleware runs a request from remoteAddr through the server's
// credential middleware and reports what gate(r) decided.
func gateThroughMiddleware(t *testing.T, gate func(*http.Request) bool, remoteAddr, token string) bool {
	t.Helper()
	var got bool
	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/list", nil)
	req.RemoteAddr = remoteAddr
	if token != "" {
		req.Header.Set(hostauth.Header, token)
	}
	hostauth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = gate(r) })).
		ServeHTTP(httptest.NewRecorder(), req)
	return got
}

// Loopback is not a credential (docs/problems/loopback-is-a-credential-2026-10-14.md).
func TestIsInternalCallerRequiresHostCredentialInEnforce(t *testing.T) {
	const token = "vmctl-gate-test-token-0123456789abcdef0123456789"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	if gateThroughMiddleware(t, isInternalCaller, "127.0.0.1:5000", "") {
		t.Fatal("loopback without the credential was internal")
	}
	if gateThroughMiddleware(t, isInternalCaller, "127.0.0.1:5000", "wrong-wrong-wrong-wrong-wrong-wrong-wrong") {
		t.Fatal("loopback with a wrong credential was internal")
	}
	if !gateThroughMiddleware(t, isInternalCaller, "127.0.0.1:5000", token) {
		t.Fatal("loopback with the credential was refused")
	}
	if !gateThroughMiddleware(t, isInternalCaller, "@", token) {
		t.Fatal("unix socket with the credential was refused")
	}
	if gateThroughMiddleware(t, isInternalCaller, "10.200.3.2:41000", token) {
		t.Fatal("a guest address with the credential was internal: the transport check must still hold")
	}
}

func TestIsInternalCallerObserveAllowsLoopbackWithoutCredential(t *testing.T) {
	restore := hostauth.SetForTesting(hostauth.ModeObserve, "vmctl-gate-test-token-0123456789abcdef0123456789")
	defer restore()
	if !gateThroughMiddleware(t, isInternalCaller, "127.0.0.1:5000", "") {
		t.Fatal("observe mode refused a loopback caller")
	}
}
