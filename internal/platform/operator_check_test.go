package platform

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// The proxy's platform-computer gate asks this route and nothing else for
// the roster. Failure list A2, A3, A5, A10
// (docs/design/platform-operator-account-2026-11-06.md, "Failure list (build)").
func TestPlatformOperatorCheckExactMatchAndFailClosed(t *testing.T) {
	const op = "1111aaaa-2222-3333-4444-55555555beef"
	ask := func(h *Handler, id, remote string) (int, bool) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, PlatformOperatorCheckRoute+"?user_id="+url.QueryEscape(id), nil)
		if remote != "" {
			r.RemoteAddr = remote
		}
		rec := httptest.NewRecorder()
		h.HandlePlatformOperatorCheck(rec, r)
		var out PlatformOperatorCheck
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out.Operator
	}
	h := &Handler{}
	// A2: unset and "none" admit nobody.
	if n, err := h.ConfigurePlatformOperatorUsers(""); n != 0 || err != nil {
		t.Fatalf("empty: %d %v", n, err)
	}
	if code, ok := ask(h, op, "127.0.0.1:1"); code != 200 || ok {
		t.Fatalf("empty roster admitted: %d %v", code, ok)
	}
	// A3: one malformed entry grants nobody, the well-formed neighbour included.
	if _, err := h.ConfigurePlatformOperatorUsers(op + ",NOT-AN-ID"); err == nil {
		t.Fatal("malformed entry parsed")
	}
	if _, ok := ask(h, op, "127.0.0.1:1"); ok {
		t.Fatal("malformed roster admitted the well-formed entry")
	}
	// The member is admitted; near matches are not.
	if n, err := h.ConfigurePlatformOperatorUsers(op); n != 1 || err != nil {
		t.Fatalf("roster: %d %v", n, err)
	}
	if code, ok := ask(h, op, "127.0.0.1:1"); code != 200 || !ok {
		t.Fatalf("member refused: %d %v", code, ok)
	}
	for _, near := range []string{"1111AAAA-2222-3333-4444-55555555BEEF", " " + op, op + " ", op[:8], op + "x", "", "*"} {
		if _, ok := ask(h, near, "127.0.0.1:1"); ok {
			t.Fatalf("near match %q admitted", near)
		}
	}
	// A10: not from a non-loopback address.
	if code, _ := ask(h, op, "203.0.113.9:4000"); code != http.StatusForbidden {
		t.Fatalf("remote caller got %d", code)
	}
	// Reads only.
	rec := httptest.NewRecorder()
	h.HandlePlatformOperatorCheck(rec, httptest.NewRequest(http.MethodPost, PlatformOperatorCheckRoute+"?user_id="+op, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST got %d", rec.Code)
	}
}
