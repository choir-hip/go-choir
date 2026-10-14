package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// corpusd internal routes need the host-internal credential
// (docs/problems/loopback-is-a-credential-2026-10-14.md).
func TestTrustedInternalCallerRequiresHostCredentialInEnforce(t *testing.T) {
	const token = "corpusd-gate-test-token-0123456789abcdef01234567"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	gate := func(remote, presented string) bool {
		var got bool
		req := httptest.NewRequest(http.MethodGet, "/internal/platform/control-key", nil)
		req.RemoteAddr = remote
		req.Header.Set("X-Internal-Caller", "true")
		if presented != "" {
			req.Header.Set(hostauth.Header, presented)
		}
		hostauth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = trustedInternalCaller(r) })).
			ServeHTTP(httptest.NewRecorder(), req)
		return got
	}
	if gate("127.0.0.1:7000", "") {
		t.Fatal("loopback + marker without the credential was trusted")
	}
	if !gate("127.0.0.1:7000", token) {
		t.Fatal("loopback + marker + credential was refused")
	}
	if gate("10.200.1.2:7000", token) {
		t.Fatal("a guest address with the credential was trusted")
	}
}
