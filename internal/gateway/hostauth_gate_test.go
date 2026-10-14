package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// The credential issue/ensure and operator routes need the host-internal
// credential, not just loopback and a header anyone can send
// (docs/problems/loopback-is-a-credential-2026-10-14.md).
func TestCredentialCallerRequiresHostCredentialInEnforce(t *testing.T) {
	const token = "gateway-gate-test-token-0123456789abcdef01234567"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	gate := func(remote, presented string) bool {
		var got bool
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/credentials/ensure", nil)
		req.RemoteAddr = remote
		req.Header.Set(internalCallerHeader, "true")
		if presented != "" {
			req.Header.Set(hostauth.Header, presented)
		}
		hostauth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = isAuthorizedCredentialCaller(r) })).
			ServeHTTP(httptest.NewRecorder(), req)
		return got
	}
	if gate("127.0.0.1:6000", "") {
		t.Fatal("loopback + marker without the credential was authorized")
	}
	if gate("127.0.0.1:6000", "not-the-token-not-the-token-not-the-token") {
		t.Fatal("wrong credential was authorized")
	}
	if !gate("127.0.0.1:6000", token) {
		t.Fatal("loopback + marker + credential was refused")
	}
	if gate("10.200.1.2:6000", token) {
		t.Fatal("a guest address with the credential was authorized")
	}
}
