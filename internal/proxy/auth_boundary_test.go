package proxy

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Failure modes pinned here
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md):
//
//	F1a a client-supplied X-Authenticated-Computer reaches the guest, which
//	    takes it as the computer binding;
//	F1b a client pre-sets a proxy carrier header (X-Proxy-Trusted-Scopes,
//	    -Email, X-Resolved-Autoputer-URL) and the director copies it;
//	F1c a computer route spelled with a trailing slash misses the proxy's
//	    matchers and reaches the guest through the generic forward;
//	F4  an agent key, `admin` included, arms, approves, rolls back, restores,
//	    rematerializes or bootstraps without the owner;
//	F4' the owner-act gate also refuses the fail-safe reject, or an
//	    owner-minted owner:acts key.

func TestProxyStripsForgedIdentityAndCarrierHeaders(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	var attackerHits atomic.Int32
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attackerHits.Add(1)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer attacker.Close()
	seen := make(chan http.Header, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	handler, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: upstream.URL, AuthPublicKeyPath: "/unused"}, pub)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWTWithEmail(priv, "owner-real", "owner@example.com")})
	forged := map[string]string{
		"X-Authenticated-Computer": "computer-forged",
		"X-Proxy-Trusted-Scopes":   "admin",
		"X-Proxy-Trusted-Email":    "attacker@example.com",
		"X-Proxy-Trusted-User":     "owner-forged",
		"X-Resolved-Autoputer-URL": attacker.URL,
		"X-Internal-Updater":       "true",
		"X-Choir-Operator":         "alice",
		"X-Choir-Operator-Token":   "token-a",
		"X-Choir-Lifecycle-Caller": "forged",
		"X-Internal-Caller":        "true",
	}
	for name, value := range forged {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if attackerHits.Load() != 0 {
		t.Fatal("F1b: a client-supplied X-Resolved-Autoputer-URL redirected the forward")
	}
	got := <-seen
	if got.Get("X-Authenticated-User") != "owner-real" {
		t.Fatalf("X-Authenticated-User = %q, want the session's owner", got.Get("X-Authenticated-User"))
	}
	if got.Get("X-Authenticated-Email") != "owner@example.com" {
		t.Fatalf("F1b: X-Authenticated-Email = %q, want the session's", got.Get("X-Authenticated-Email"))
	}
	for _, name := range []string{"X-Authenticated-Computer", "X-Authenticated-Scopes", "X-Internal-Updater", "X-Choir-Operator", "X-Choir-Operator-Token", "X-Choir-Lifecycle-Caller", "X-Internal-Caller", "X-Proxy-Trusted-Scopes", "X-Proxy-Trusted-Email", "X-Proxy-Trusted-User", "X-Resolved-Autoputer-URL"} {
		if v := got.Get(name); v != "" {
			t.Errorf("F1a/F1b: forged %s reached the guest: %q", name, v)
		}
	}
}

func TestProxyRefusesComputerRoutesInGenericForward(t *testing.T) {
	handler, _, autoputer, store := testProxyEnvWithAuthStore(t)
	var hits atomic.Int32
	autoputer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/computers/") {
			hits.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	})
	user, secret := createTestAPIKey(t, handler, store, "runtime-writer", []string{"write:runtime", "read:runtime"}, nil)
	computer, err := handler.vmctlClient.ResolveDesktopContext(context.Background(), user.ID, "primary")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/computers/" + computer.ComputerID + "/lifecycle/restore/",
		"/api/computers/" + computer.ComputerID + "/lifecycle/rematerialize-from-tape/",
		"/api/computers/" + computer.ComputerID + "/lifecycle/bootstrap-chain/",
		"/api/computers/" + computer.ComputerID + "/lifecycle/replace-workspace/",
		"/api/computers/" + computer.ComputerID + "/events/digest-1",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+secret)
		req.Header.Set("X-Authenticated-Computer", computer.ComputerID)
		rec := httptest.NewRecorder()
		handler.HandleAPI(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("F1c: %s = %d, want 404 (body %s)", path, rec.Code, rec.Body.String())
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("F1c: %d computer-route requests reached the guest through the generic forward", hits.Load())
	}
}

// ownerActEnv is one computer, "computer-a", owned by a fresh user, with a
// guest and a corpusd that count what reaches them.
type ownerActEnv struct {
	handler     *Handler
	store       *auth.Store
	user        *auth.User
	guestHits   *atomic.Int32
	corpusdHits *atomic.Int32
	privateKey  ed25519.PrivateKey
}

func newOwnerActEnv(t *testing.T) *ownerActEnv {
	t.Helper()
	env := &ownerActEnv{guestHits: &atomic.Int32{}, corpusdHits: &atomic.Int32{}}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.guestHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(guest.Close)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.corpusdHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"computer_id":"computer-a","mode":"accept_once","generation":1}`))
	}))
	t.Cleanup(corpusd.Close)
	handler, priv, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	user, err := store.CreateUser("owner-act-user", "owner-act@example.com")
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": guest.URL, "vm_id": "vm-a",
		})
	}))
	t.Cleanup(ownership.Close)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	env.handler, env.store, env.user, env.privateKey = handler, store, user, priv
	return env
}

func (e *ownerActEnv) key(t *testing.T, scopes ...string) string {
	t.Helper()
	_, secret, err := e.store.CreateComputerScopedAPIKey(context.Background(), e.user.ID, "owner-act", scopes, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

func (e *ownerActEnv) send(t *testing.T, method, path, body, secret string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
		req.Header.Set("X-Choir-Computer", "computer-a")
	} else {
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(e.privateKey, e.user.ID)})
	}
	rec := httptest.NewRecorder()
	e.handler.HandleAPI(rec, req)
	return rec
}

func TestOwnerActsRefuseAgentAndAdminKeys(t *testing.T) {
	const base = "/api/computers/computer-a"
	acts := []struct {
		name, method, path, body, scope string
		upstream                        string // "guest" or "corpusd"
	}{
		{"mode set", http.MethodPut, base + "/self-development/mode", `{"mode":"accept_once"}`, "computer:self_development:mode", "corpusd"},
		{"approve", http.MethodPost, base + "/self-development/operations/op-1/decision", `{"decision":"approve"}`, "computer:self_development:approve", "guest"},
		{"unparseable decision", http.MethodPost, base + "/self-development/operations/op-1/decision", `not json`, "computer:self_development:approve", "guest"},
		{"rollback", http.MethodPost, base + "/self-development/rollbacks", `{}`, "computer:self_development:rollback", "guest"},
		// U4 slice 8: the owner's rebase retry is an approval-class act.
		{"rebase retry", http.MethodPost, base + "/self-development/rebase-retry", `{}`, "computer:self_development:approve", "guest"},
		{"restore", http.MethodPost, base + "/lifecycle/restore", `{}`, "computer:lifecycle", "guest"},
		{"rematerialize", http.MethodPost, base + "/lifecycle/rematerialize-from-tape", `{}`, "computer:lifecycle", "guest"},
		{"bootstrap-chain", http.MethodPost, base + "/lifecycle/bootstrap-chain", `{}`, "computer:lifecycle", "guest"},
	}
	for _, act := range acts {
		act := act
		t.Run(act.name, func(t *testing.T) {
			env := newOwnerActEnv(t)
			hits := env.guestHits
			if act.upstream == "corpusd" {
				hits = env.corpusdHits
			}
			for _, scopes := range [][]string{{act.scope}, {"admin"}, {"admin", "manage:keys", act.scope}} {
				rec := env.send(t, act.method, act.path, act.body, env.key(t, scopes...))
				if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "owner act") {
					t.Fatalf("F4: %s with key scopes %v = %d %s, want 403 owner act", act.name, scopes, rec.Code, rec.Body.String())
				}
			}
			if hits.Load() != 0 {
				t.Fatalf("F4: %s reached %s %d times without the owner", act.name, act.upstream, hits.Load())
			}
			if act.name == "unparseable decision" {
				return
			}
			// An owner-minted owner:acts key, with the route's scope, passes.
			if rec := env.send(t, act.method, act.path, act.body, env.key(t, act.scope, OwnerActsScope)); rec.Code == http.StatusForbidden {
				t.Fatalf("F4': owner:acts key refused for %s: %d %s", act.name, rec.Code, rec.Body.String())
			}
			// The owner's own session passes.
			if rec := env.send(t, act.method, act.path, act.body, ""); rec.Code == http.StatusForbidden {
				t.Fatalf("F4': owner session refused for %s: %d %s", act.name, rec.Code, rec.Body.String())
			}
			if hits.Load() != 2 {
				t.Fatalf("F4': %s reached %s %d times, want 2 (owner:acts key, session)", act.name, act.upstream, hits.Load())
			}
		})
	}
}

func TestOwnerActGateLetsAgentKeysReject(t *testing.T) {
	env := newOwnerActEnv(t)
	rec := env.send(t, http.MethodPost, "/api/computers/computer-a/self-development/operations/op-1/decision",
		`{"decision":"reject","reason":"withdrawn by the agent"}`, env.key(t, "computer:self_development:approve"))
	if rec.Code != http.StatusOK || env.guestHits.Load() != 1 {
		t.Fatalf("F4': agent reject = %d (guest hits %d) %s, want forwarded", rec.Code, env.guestHits.Load(), rec.Body.String())
	}
}

// F5: a guest lists objects through the proxy (where corpusd sees the proxy as
// an internal caller) only for its own owner and computer, and lists no edges.
func TestGuestObjectListThroughProxyIsOwnerScoped(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	og := platform.NewObjectGraphHandler(svc, mem)
	corpusMux := http.NewServeMux()
	corpusMux.HandleFunc("/internal/platform/objects", og.HandleObjects)
	corpusMux.HandleFunc("/internal/platform/edges", og.HandleEdges)
	corpusd := httptest.NewServer(corpusMux)
	defer corpusd.Close()
	guests := map[string]vmctl.GuestOwnership{
		"10.200.1.2": {Found: true, UserID: "paper-owner", ComputerID: "computer-paper"},
	}
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

	get := func(path string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "10.200.1.2:40000"
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, path := range []string{
		"/internal/platform/objects?owner=reader-owner&computer=computer-reader",
		"/internal/platform/objects?owner=paper-owner&computer=computer-reader",
		"/internal/platform/objects?kind=choir.web_capture",
		"/internal/platform/objects",
		"/internal/platform/edges",
		"/internal/platform/edges?from=x",
	} {
		if code := get(path); code != http.StatusForbidden {
			t.Errorf("F5: guest %s through the proxy = %d, want 403", path, code)
		}
	}
	if code := get("/internal/platform/objects?kind=choir.web_capture&owner=paper-owner&computer=computer-paper&limit=5"); code != http.StatusOK {
		t.Fatalf("own-computer capture list through the proxy = %d, want 200", code)
	}
}
