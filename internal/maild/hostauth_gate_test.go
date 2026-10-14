package maild

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// A loopback caller asserting an owner needs the host-internal credential
// (docs/problems/loopback-is-a-credential-2026-10-14.md).
func TestInternalOwnerRequiresHostCredentialInEnforce(t *testing.T) {
	const token = "maild-gate-test-token-0123456789abcdef0123456789"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	h := &Handler{cfg: &Config{}}
	call := func(presented string) (string, int) {
		var owner string
		req := httptest.NewRequest(http.MethodGet, "/api/email/drafts", nil)
		req.RemoteAddr = "127.0.0.1:8000"
		req.Header.Set("X-Internal-Caller", "true")
		req.Header.Set("X-Authenticated-User", "owner-1")
		if presented != "" {
			req.Header.Set(hostauth.Header, presented)
		}
		rec := httptest.NewRecorder()
		hostauth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			owner, _ = h.authenticatedInternalOwner(w, r)
		})).ServeHTTP(rec, req)
		return owner, rec.Code
	}
	if owner, code := call(""); owner != "" || code != http.StatusForbidden {
		t.Fatalf("no credential: owner=%q code=%d, want refusal 403", owner, code)
	}
	if owner, _ := call(token); owner != "owner-1" {
		t.Fatalf("credential: owner=%q, want owner-1", owner)
	}
}
