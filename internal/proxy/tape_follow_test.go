package proxy

// The tape follow route (tape_follow.go) hands a computer's whole tape to the
// caller, so its failure modes are authority failures:
//
//  A1  An unauthenticated caller reaches corpusd.
//  A2  A signed-in user reaches another owner's computer (vmctl has no row
//      for the pair, or answers another owner).
//  A3  An API key without admin or computer:lifecycle scope, or bound to
//      another computer, reaches it.
//  A4  A worker (non-interactive) computer is served.
//  A5  The proxy forwards the caller's own internal headers or unknown query
//      parameters, or a computer_id other than the path's, to corpusd.
//  A6  The route touches the computer's activity (keeping it awake) or
//      wakes it: the only vmctl call is the ownership lookup.
//  A7  The body is buffered or cut: the tar passes through byte for byte
//      with its range headers.
//  A8  A write method is served.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type tapeFollowFixture struct {
	handler *Handler
	mu      sync.Mutex
	vmctl   []string // paths
	corpusd []*http.Request
	owner   map[string]string // computer -> owner user id
	kind    string
}

func newTapeFollowFixture(t *testing.T) (*tapeFollowFixture, func(string) string, func(userID string, scopes []string, bound string) string) {
	t.Helper()
	handler, priv, _, store := testProxyEnvWithAuthStore(t)
	f := &tapeFollowFixture{handler: handler, owner: map[string]string{}, kind: "interactive"}
	for _, id := range []string{"user-a", "user-b"} {
		if _, err := store.CreateUser(id, id+"@example.com"); err != nil {
			t.Fatal(err)
		}
	}
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.vmctl = append(f.vmctl, r.URL.Path)
		owner, kind := f.owner[r.URL.Query().Get("computer_id")], f.kind
		f.mu.Unlock()
		if r.URL.Path != "/internal/vmctl/lookup" || owner == "" || (r.URL.Query().Get("user_id") != "" && r.URL.Query().Get("user_id") != owner) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": r.URL.Query().Get("computer_id"), "user_id": owner, "desktop_id": "primary", "kind": kind, "state": "hibernated", "epoch": 3,
		})
	}))
	t.Cleanup(vm.Close)
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.corpusd = append(f.corpusd, r.Clone(context.Background()))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("X-Choir-Source-Head-Seq", "42")
		w.Header().Set("X-Choir-Bundle-Head-Seq", "40")
		w.Header().Set("X-Internal-Leak", "no")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("tarbytes", 1<<14)))
	}))
	t.Cleanup(corpusd.Close)
	handler.cfg.CorpusdURL = corpusd.URL
	cookie := func(userID string) string { return issueTestAccessJWT(priv, userID) }
	key := func(userID string, scopes []string, bound string) string {
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), userID, "follow", scopes, bound, nil)
		if err != nil {
			t.Fatal(err)
		}
		return secret
	}
	return f, cookie, key
}

func (f *tapeFollowFixture) serve(method, path string, set func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if set != nil {
		set(req)
	}
	rec := httptest.NewRecorder()
	f.handler.HandleAPI(rec, req)
	return rec
}

func (f *tapeFollowFixture) counts() (vm int, corp int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.vmctl), len(f.corpusd)
}

func TestTapeFollowOwnerSessionStreamsOwnComputer(t *testing.T) {
	f, cookie, _ := newTapeFollowFixture(t)
	f.owner["computer-a"] = "user-a"
	rec := f.serve(http.MethodGet, "/api/computers/computer-a/tape/bundle?branch=main&have_seq=7&have_head="+strings.Repeat("ab", 32)+"&max_events=50&computer_id=computer-b&evil=1", func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "choir_access", Value: cookie("user-a")})
		r.Header.Set("X-Authenticated-User", "user-b")
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 8<<14 || rec.Header().Get("X-Choir-Source-Head-Seq") != "42" || rec.Header().Get("X-Choir-Bundle-Head-Seq") != "40" || rec.Header().Get("X-Internal-Leak") != "" {
		t.Fatalf("body %d bytes, headers %v (A7)", rec.Body.Len(), rec.Header())
	}
	vm, corp := f.counts()
	if vm != 1 || corp != 1 {
		t.Fatalf("vmctl calls=%d corpusd calls=%d, want 1 and 1", vm, corp)
	}
	if f.vmctl[0] != "/internal/vmctl/lookup" {
		t.Fatalf("vmctl call %s: the route must only look ownership up (A6)", f.vmctl[0])
	}
	up := f.corpusd[0]
	q := up.URL.Query()
	if up.URL.Path != "/internal/computers/tape/bundle" || q.Get("computer_id") != "computer-a" || q.Get("have_seq") != "7" || q.Get("max_events") != "50" || q.Get("branch") != "main" || q.Has("evil") {
		t.Fatalf("upstream %s (A5)", up.URL.String())
	}
	if up.Header.Get("X-Authenticated-User") != "user-a" || up.Header.Get("X-Internal-Caller") != "true" {
		t.Fatalf("upstream headers %v (A5)", up.Header)
	}
}

func TestTapeFollowRefusals(t *testing.T) {
	f, cookie, key := newTapeFollowFixture(t)
	f.owner["computer-a"] = "user-a"
	f.owner["computer-b"] = "user-b"
	admin := key("user-a", []string{"admin"}, "")
	lifecycle := key("user-a", []string{"computer:lifecycle"}, "computer-a")
	texture := key("user-a", []string{"read:texture", "read:runtime"}, "")
	boundB := key("user-a", []string{"admin"}, "computer-b")
	bearer := func(secret string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+secret) }
	}
	session := func(user string) func(*http.Request) {
		return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "choir_access", Value: cookie(user)}) }
	}
	cases := []struct {
		name   string
		method string
		path   string
		set    func(*http.Request)
		want   int
	}{
		{"unauthenticated (A1)", http.MethodGet, "/api/computers/computer-a/tape/bundle", nil, http.StatusUnauthorized},
		{"bad bearer (A1)", http.MethodGet, "/api/computers/computer-a/tape/bundle", bearer("choir_sk_nope"), http.StatusUnauthorized},
		{"another owner's session (A2)", http.MethodGet, "/api/computers/computer-b/tape/bundle", session("user-a"), http.StatusForbidden},
		{"no such computer (A2)", http.MethodGet, "/api/computers/computer-z/tape/bundle", session("user-a"), http.StatusForbidden},
		{"another owner's computer by owner-wide key (A2)", http.MethodGet, "/api/computers/computer-b/tape/bundle", bearer(admin), http.StatusForbidden},
		{"key without the scope (A3)", http.MethodGet, "/api/computers/computer-a/tape/bundle", bearer(texture), http.StatusForbidden},
		{"key bound to another computer (A3)", http.MethodGet, "/api/computers/computer-a/tape/bundle", bearer(boundB), http.StatusForbidden},
		{"write method (A8)", http.MethodPost, "/api/computers/computer-a/tape/bundle", session("user-a"), http.StatusMethodNotAllowed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, before := f.counts()
			rec := f.serve(c.method, c.path, c.set)
			if rec.Code != c.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, c.want, rec.Body.String())
			}
			if _, after := f.counts(); after != before {
				t.Fatal("a refused request reached corpusd")
			}
		})
	}
	for _, c := range []struct {
		name string
		set  func(*http.Request)
	}{{"owner-wide admin key", bearer(admin)}, {"bound lifecycle key", bearer(lifecycle)}, {"owner session", session("user-a")}} {
		t.Run("allowed: "+c.name, func(t *testing.T) {
			rec := f.serve(http.MethodGet, "/api/computers/computer-a/tape/bundle", c.set)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	f.mu.Lock()
	f.kind = "worker"
	f.mu.Unlock()
	if rec := f.serve(http.MethodGet, "/api/computers/computer-a/tape/bundle", session("user-a")); rec.Code != http.StatusForbidden {
		t.Fatalf("worker computer: status=%d (A4)", rec.Code)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.vmctl {
		if p != "/internal/vmctl/lookup" {
			t.Fatalf("vmctl call %s: the route must only look ownership up (A6)", p)
		}
	}
}
