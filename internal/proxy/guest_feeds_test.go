package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// X4, X9, X19 at the proxy: whatever owner a guest names, corpusd sees the
// owner vmctl bound; an unbound caller, a write to a feed and a read of the
// verdict route stay refused.
func TestGuestFeedsAreScopedToTheBoundOwner(t *testing.T) {
	var seen []string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		seen = append(seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery+" "+string(raw))
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "next_cursor": ""})
	}))
	defer corpusd.Close()
	guests := map[string]vmctl.GuestOwnership{"10.200.1.2": {Found: true, UserID: "bound-owner", ComputerID: "computer-1"}}
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RemoteAddr string `json:"remote_addr"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(guests[strings.Split(req.RemoteAddr, ":")[0]])
	}))
	defer vmctlServer.Close()
	handler, _, _ := testProxyEnv(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)
	send := func(method, path, body, from string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	for _, path := range []string{"/internal/platform/opeds", "/internal/platform/notices"} {
		seen = nil
		if rec := send(http.MethodGet, path+"?owner_id=victim&after=2026-01-01T00:00:00Z", "", "10.200.1.2"); rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, rec.Code, rec.Body)
		}
		if len(seen) != 1 || !strings.Contains(seen[0], "owner_id=bound-owner") || strings.Contains(seen[0], "victim") {
			t.Fatalf("corpusd saw %v, want the bound owner only", seen)
		}
		for _, from := range []string{"10.200.77.2", "203.0.113.9"} {
			if rec := send(http.MethodGet, path, "", from); rec.Code != http.StatusForbidden {
				t.Fatalf("unbound GET %s from %s = %d", path, from, rec.Code)
			}
		}
		if rec := send(http.MethodPost, path, "{}", "10.200.1.2"); rec.Code != http.StatusForbidden {
			t.Fatalf("a guest POST to %s = %d, want 403", path, rec.Code)
		}
	}

	seen = nil
	verdict := `{"owner_id":"victim","version_id":"pub-x-v1","run":1,"verdict":"accepted","verdict_hash":"h","tape_record_id":"r"}`
	if rec := send(http.MethodPost, "/internal/platform/oped-verdicts", verdict, "10.200.1.2"); rec.Code != http.StatusOK {
		t.Fatalf("verdict post = %d %s", rec.Code, rec.Body)
	}
	if len(seen) != 1 || !strings.Contains(seen[0], `"owner_id":"bound-owner"`) || strings.Contains(seen[0], "victim") {
		t.Fatalf("corpusd saw %v", seen)
	}
	if rec := send(http.MethodGet, "/internal/platform/oped-verdicts", "", "10.200.1.2"); rec.Code != http.StatusForbidden {
		t.Fatalf("a guest read of the verdict route = %d, want 403", rec.Code)
	}
	if rec := send(http.MethodPost, "/internal/platform/oped-verdicts", verdict, "203.0.113.9"); rec.Code != http.StatusForbidden {
		t.Fatalf("an unbound verdict post = %d, want 403", rec.Code)
	}
	if rec := send(http.MethodPost, "/internal/platform/oped-verdicts", "not json", "10.200.1.2"); rec.Code != http.StatusForbidden {
		t.Fatalf("a malformed verdict post = %d, want refused", rec.Code)
	}
}
