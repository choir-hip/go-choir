package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Review bunch 22 item 1: the isolation self-test's OOM and seccomp legs are
// host-internal (the guest serves them on /internal/runtime/isolation-selftest).
// The proxy forwards every authenticated /api/* call to the guest with the
// owner injected, so it must refuse the legs on the /api route itself, for a
// cookie session and for an API key with write:runtime alike; the no-leg
// confinement self-test stays an owner route.
func TestIsolationSelfTestLegsAreNotForwarded(t *testing.T) {
	handler, priv, autoputer, store := testProxyEnvWithAuthStore(t)
	reached := map[string]int{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		reached[r.URL.RequestURI()]++
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	autoputer.Config.Handler = mux

	user, key := createTestAPIKey(t, handler, store, "selftest-owner", []string{"read:runtime", "write:runtime"}, nil)
	send := func(cookie bool, target string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(`{}`))
		if cookie {
			req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, user.ID)})
		} else {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		rec := httptest.NewRecorder()
		handler.HandleProtectedAPI(rec, req)
		return rec.Code
	}
	for _, cookie := range []bool{false, true} {
		for _, target := range []string{
			"/api/runtime/isolation-selftest?leg=oom",
			"/api/runtime/isolation-selftest?leg=seccomp",
			"/api/runtime/isolation-selftest?computer_id=c1&leg=oom",
			"/api/runtime/isolation-selftest?leg=",
		} {
			before := total(reached)
			code := send(cookie, target)
			if forwarded := total(reached) != before; code != http.StatusNotFound || forwarded {
				t.Errorf("cookie=%v %s: status %d, forwarded %v; want 404 before the guest", cookie, target, code, forwarded)
			}
		}
		// The owner's confinement self-test (no leg) is still forwarded.
		if code := send(cookie, "/api/runtime/isolation-selftest"); code != http.StatusNoContent {
			t.Errorf("cookie=%v: the no-leg self-test was not forwarded: %d", cookie, code)
		}
	}
	if reached["/api/runtime/isolation-selftest"] != 2 {
		t.Errorf("no-leg self-test reached the guest %d times, want 2: %v", reached["/api/runtime/isolation-selftest"], reached)
	}
}

func total(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}
