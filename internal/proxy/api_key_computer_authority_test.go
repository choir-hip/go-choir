package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestRequireAPIKeyComputerTargetFailsClosed(t *testing.T) {
	type authorityReply struct {
		status int
		body   string
	}
	tests := []struct {
		name               string
		auth               AuthResult
		pathComputerID     string
		requestedDesktopID string
		queryDesktopID     string
		headerDesktopID    string
		queryDesktopIDs    []string
		headerDesktopIDs   []string
		queryComputerID    string
		headerComputerID   string
		lookupComputerID   string
		reply              authorityReply
		withoutVMCTL       bool
		wantStatus         int
		wantOK             bool
		wantLookups        int64
	}{
		{name: "owner-wide unnamed", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", Scopes: []string{"admin"}}, wantStatus: http.StatusBadRequest},
		{name: "owner-wide named via query", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", Scopes: []string{"admin"}}, queryComputerID: "computer-a", lookupComputerID: "computer-a", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary","kind":"interactive","state":"active","epoch":7}`}, wantOK: true, wantLookups: 1},
		{name: "owner-wide named via header", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", Scopes: []string{"admin"}}, headerComputerID: "computer-a", lookupComputerID: "computer-a", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary","kind":"interactive","state":"active","epoch":7}`}, wantOK: true, wantLookups: 1},
		{name: "owner-wide named via path", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", Scopes: []string{"admin"}}, pathComputerID: "computer-a", lookupComputerID: "computer-a", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary","kind":"interactive","state":"active","epoch":7}`}, wantOK: true, wantLookups: 1},
		{name: "owner-wide foreign computer", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", Scopes: []string{"admin"}}, queryComputerID: "computer-b", lookupComputerID: "computer-b", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-b","user_id":"other","desktop_id":"primary","kind":"interactive"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "owner-wide worker computer rejected", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", Scopes: []string{"admin"}}, queryComputerID: "computer-w", lookupComputerID: "computer-w", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-w","user_id":"owner","desktop_id":"primary","kind":"worker"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "owner-wide conflicting path and query", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", Scopes: []string{"admin"}}, pathComputerID: "computer-a", queryComputerID: "computer-b", wantStatus: http.StatusForbidden},
		{name: "owner-wide desktop selector mismatch", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", Scopes: []string{"admin"}}, queryComputerID: "computer-a", lookupComputerID: "computer-a", requestedDesktopID: "branch", queryDesktopID: "branch", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary","kind":"interactive"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "path target differs before lookup", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, pathComputerID: "computer-b", wantStatus: http.StatusForbidden},
		{name: "vmctl missing", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, withoutVMCTL: true, wantStatus: http.StatusServiceUnavailable},
		{name: "not found", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, reply: authorityReply{status: http.StatusNotFound}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "foreign owner", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"other","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "empty owner", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "non-exact owner", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":" owner ","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "empty canonical desktop", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner"}`}, wantStatus: http.StatusServiceUnavailable, wantLookups: 1},
		{name: "returned computer conflicts", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-b","user_id":"owner","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "bound worker computer rejected", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-w"}, lookupComputerID: "computer-w", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-w","user_id":"owner","desktop_id":"primary","kind":"worker"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "authority 500", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, reply: authorityReply{status: http.StatusInternalServerError}, wantStatus: http.StatusServiceUnavailable, wantLookups: 1},
		{name: "malformed authority", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, reply: authorityReply{status: http.StatusOK, body: `{`}, wantStatus: http.StatusServiceUnavailable, wantLookups: 1},
		{name: "default desktop mismatch", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, requestedDesktopID: "primary", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"branch"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "query desktop mismatch", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, requestedDesktopID: "branch", queryDesktopID: "branch", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "header desktop mismatch", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, requestedDesktopID: "branch", headerDesktopID: "branch", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "body desktop mismatch", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, requestedDesktopID: "branch", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "query and header conflict", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, requestedDesktopID: "primary", queryDesktopID: "primary", headerDesktopID: "branch", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "duplicate query conflict", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, requestedDesktopID: "primary", queryDesktopIDs: []string{"primary", "branch"}, reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "duplicate header conflict", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, requestedDesktopID: "primary", headerDesktopIDs: []string{"primary", "branch"}, reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary"}`}, wantStatus: http.StatusForbidden, wantLookups: 1},
		{name: "exact owner computer path and desktop", auth: AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, pathComputerID: "computer-a", requestedDesktopID: "primary", queryDesktopID: "primary", headerDesktopID: "primary", reply: authorityReply{status: http.StatusOK, body: `{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary","state":"active","epoch":7}`}, wantOK: true, wantLookups: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var lookups atomic.Int64
			var server *httptest.Server
			h := &Handler{}
			if !test.withoutVMCTL {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					lookups.Add(1)
					expected := test.lookupComputerID
					if expected == "" {
						expected = test.auth.ComputerID
					}
					if r.URL.Path != "/internal/vmctl/lookup" || r.URL.Query().Get("user_id") != test.auth.UserID || r.URL.Query().Get("computer_id") != expected {
						t.Fatalf("lookup target=%s?%s", r.URL.Path, r.URL.RawQuery)
					}
					status := test.reply.status
					if status == 0 {
						status = http.StatusOK
					}
					w.WriteHeader(status)
					_, _ = w.Write([]byte(test.reply.body))
				}))
				defer server.Close()
				h.vmctlClient = vmctl.NewClient(server.URL)
			}
			target := "/api/test"
			if test.queryDesktopID != "" {
				target += "?desktop_id=" + test.queryDesktopID
			}
			req := httptest.NewRequest(http.MethodGet, target, nil)
			query := req.URL.Query()
			for _, value := range test.queryDesktopIDs {
				query.Add("desktop_id", value)
			}
			if test.queryComputerID != "" {
				query.Set("computer_id", test.queryComputerID)
			}
			req.URL.RawQuery = query.Encode()
			if test.headerDesktopID != "" {
				req.Header.Set("X-Choir-Desktop", test.headerDesktopID)
			}
			for _, value := range test.headerDesktopIDs {
				req.Header.Add("X-Choir-Desktop", value)
			}
			if test.headerComputerID != "" {
				req.Header.Set("X-Choir-Computer", test.headerComputerID)
			}
			rec := httptest.NewRecorder()
			resolved, ok := h.requireAPIKeyComputerTarget(rec, req, &test.auth, test.pathComputerID, test.requestedDesktopID)
			if ok != test.wantOK {
				t.Fatalf("ok=%v want=%v status=%d body=%s", ok, test.wantOK, rec.Code, rec.Body.String())
			}
			if test.wantOK {
				expected := test.lookupComputerID
				if expected == "" {
					expected = test.auth.ComputerID
				}
				if resolved == nil || resolved.ComputerID != expected || resolved.UserID != "owner" {
					t.Fatalf("resolved=%+v", resolved)
				}
			} else if rec.Code != test.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, test.wantStatus, rec.Body.String())
			}
			if got := lookups.Load(); got != test.wantLookups {
				t.Fatalf("lookups=%d want=%d", got, test.wantLookups)
			}
		})
	}
}

func TestOwnerWideAdminAPIKeyRequiresNamedTargetBeforeDesktopRouteFamilies(t *testing.T) {
	handler, _, sandbox, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("owner-wide-admin", "owner-wide-admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(context.Background(), user.ID, "owner-wide admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var ownershipCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ownershipCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	var downstreamCalls atomic.Int64
	sandbox.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	})
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	}))
	defer corpusd.Close()
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()

	// These routes select a desktop, not a path-named computer: an owner-wide
	// key must name its target explicitly (computer_id query/header) and fails
	// closed with 400 before any ownership lookup or downstream call.
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "bootstrap", method: http.MethodGet, path: "/api/shell/bootstrap"},
		{name: "protected HTTP", method: http.MethodGet, path: "/api/base/delta"},
		{name: "websocket", method: http.MethodGet, path: "/api/ws"},
		{name: "super console websocket", method: http.MethodGet, path: "/api/super-console/ws"},
		{name: "compute recovery", method: http.MethodPost, path: "/api/compute/recovery", body: `{"action":"wake_current_computer","desktop_id":"primary"}`},
		{name: "execution identity", method: http.MethodGet, path: "/api/acceptance/execution-identity"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			req.Header.Set("Authorization", "Bearer "+secret)
			rec := httptest.NewRecorder()
			handler.HandleAPI(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
			if ownershipCalls.Load() != 0 || downstreamCalls.Load() != 0 {
				t.Fatalf("ownership_calls=%d downstream_calls=%d want zero", ownershipCalls.Load(), downstreamCalls.Load())
			}
		})
	}
}

func TestOwnerWideAdminAPIKeyControlsAnyOwnedComputer(t *testing.T) {
	handler, _, sandbox, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("owner-wide-user", "owner-wide-user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(context.Background(), user.ID, "owner-wide admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var ownershipCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup" {
			http.Error(w, "unexpected", http.StatusInternalServerError)
			return
		}
		ownershipCalls.Add(1)
		computerID := r.URL.Query().Get("computer_id")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": computerID, "user_id": user.ID, "desktop_id": "primary", "kind": "interactive", "state": "active", "epoch": 7,
		})
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	var downstreamCalls atomic.Int64
	sandbox.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls.Add(1)
		http.Error(w, "bounded downstream stop", http.StatusInternalServerError)
	})
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": r.URL.Query().Get("computer_id"), "mode": "off", "generation": 0})
	}))
	defer corpusd.Close()
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()

	// One owner-wide key reaches every owned computer; the named target in the
	// path drives the exact ownership join.
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		computerID string
	}{
		{name: "lifecycle computer A", method: http.MethodGet, path: "/api/computers/computer-a/lifecycle/status", computerID: "computer-a"},
		{name: "lifecycle computer B", method: http.MethodGet, path: "/api/computers/computer-b/lifecycle/status", computerID: "computer-b"},
		{name: "self development computer B", method: http.MethodGet, path: "/api/computers/computer-b/self-development/mode", computerID: "computer-b"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := ownershipCalls.Load()
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			req.Header.Set("Authorization", "Bearer "+secret)
			rec := httptest.NewRecorder()
			handler.HandleAPI(rec, req)
			if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized || rec.Code == http.StatusBadRequest {
				t.Fatalf("owner-wide key denied on owned computer: status=%d body=%s", rec.Code, rec.Body.String())
			}
			if ownershipCalls.Load() != before+1 {
				t.Fatalf("guard lookups delta=%d want=1 status=%d body=%s", ownershipCalls.Load()-before, rec.Code, rec.Body.String())
			}
		})
	}
	if downstreamCalls.Load() == 0 {
		t.Fatal("owner-wide positive route matrix never progressed beyond the ownership guard")
	}
}

func TestOwnerWideAdminAPIKeyForeignComputerDenied(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("owner-wide-foreign", "owner-wide-foreign@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(context.Background(), user.ID, "owner-wide admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var ownershipCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ownershipCalls.Add(1)
		// A foreign computer has no ownership row for this user.
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/computers/computer-foreign/lifecycle/status", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if ownershipCalls.Load() != 1 {
		t.Fatalf("ownership_calls=%d want=1", ownershipCalls.Load())
	}
}

func TestOwnerWideAdminAPIKeyWorkerComputerDenied(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("owner-wide-worker", "owner-wide-worker@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(context.Background(), user.ID, "owner-wide admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var ownershipCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ownershipCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": r.URL.Query().Get("computer_id"), "user_id": user.ID, "desktop_id": "primary", "kind": "worker",
		})
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/computers/computer-worker/lifecycle/status", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if ownershipCalls.Load() != 1 {
		t.Fatalf("ownership_calls=%d want=1", ownershipCalls.Load())
	}
}

func TestExactBoundAdminAPIKeyPassesGuardForAllComputerRouteFamilies(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("exact-bound-admin", "exact-bound-admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	const computerID = "computer-exact-bound"
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "bound admin", []string{"admin"}, computerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var guardLookups atomic.Int64
	var afterGuardCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/vmctl/lookup" && r.URL.Query().Get("computer_id") == computerID {
			guardLookups.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": computerID, "user_id": user.ID, "desktop_id": "primary", "state": "active", "epoch": 7,
			})
			return
		}
		afterGuardCalls.Add(1)
		http.Error(w, "bounded downstream stop", http.StatusInternalServerError)
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		afterGuardCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": computerID, "mode": "off", "generation": 0})
	}))
	defer corpusd.Close()
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "bootstrap", method: http.MethodGet, path: "/api/shell/bootstrap"},
		{name: "protected HTTP", method: http.MethodGet, path: "/api/base/delta"},
		{name: "websocket", method: http.MethodGet, path: "/api/ws"},
		{name: "super console websocket", method: http.MethodGet, path: "/api/super-console/ws"},
		{name: "compute status", method: http.MethodGet, path: "/api/compute/status"},
		{name: "compute recovery", method: http.MethodPost, path: "/api/compute/recovery", body: `{"action":"wake_current_computer","desktop_id":"primary"}`},
		{name: "execution identity", method: http.MethodGet, path: "/api/acceptance/execution-identity"},
		{name: "lifecycle", method: http.MethodGet, path: "/api/computers/" + computerID + "/lifecycle/status"},
		{name: "self development", method: http.MethodGet, path: "/api/computers/" + computerID + "/self-development/mode"},
		{name: "texture publication", method: http.MethodPost, path: "/api/platform/texture/publications", body: `{"doc_id":"doc-a"}`},
		{name: "publication proposal", method: http.MethodPost, path: "/api/platform/publications/pub-a/proposals", body: `{"doc_id":"doc-a"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := guardLookups.Load()
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			req.Header.Set("Authorization", "Bearer "+secret)
			rec := httptest.NewRecorder()
			handler.HandleAPI(rec, req)
			if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
				t.Fatalf("exact bound request failed authority guard: status=%d body=%s", rec.Code, rec.Body.String())
			}
			if guardLookups.Load() != before+1 {
				t.Fatalf("guard lookups delta=%d want=1 status=%d body=%s", guardLookups.Load()-before, rec.Code, rec.Body.String())
			}
		})
	}
	if afterGuardCalls.Load() == 0 {
		t.Fatal("positive route matrix never progressed beyond the ownership guard")
	}
}

func TestComputeRecoveryBodyDesktopMismatchStopsBeforeRecovery(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("recovery-body-mismatch", "recovery-body-mismatch@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(t.Context(), user.ID, "recovery", []string{"write:runtime"}, "computer-recovery", nil)
	if err != nil {
		t.Fatal(err)
	}
	var guardLookups atomic.Int64
	var downstreamCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/vmctl/lookup" && r.URL.Query().Get("computer_id") == "computer-recovery" {
			guardLookups.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-recovery", "user_id": user.ID, "desktop_id": "primary", "state": "failed", "epoch": 8,
			})
			return
		}
		downstreamCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer","desktop_id":"branch"}`))
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if guardLookups.Load() != 1 || downstreamCalls.Load() != 0 {
		t.Fatalf("guard_lookups=%d downstream_calls=%d want 1/0", guardLookups.Load(), downstreamCalls.Load())
	}
}

func TestUnboundAdminAPIKeyExcludesNonComputerRouteFamiliesFromGuard(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("non-computer-admin", "non-computer-admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(t.Context(), user.ID, "unbound manager", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var vmctlCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vmctlCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer corpusd.Close()
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer maild.Close()
	handler.cfg.MaildURL = maild.URL
	handler.maild = maild.Client()

	tests := []struct {
		name          string
		path          string
		authenticated bool
	}{
		{name: "universal wire", path: "/api/universal-wire/stories", authenticated: true},
		{name: "published texture", path: "/api/texture/documents/doc-a?read_owner=" + vmctl.UniversalWirePlatformOwnerID, authenticated: true},
		{name: "mail", path: "/api/email/status", authenticated: true},
		{name: "public publication resolve", path: "/api/platform/publications/resolve?route=/article"},
		{name: "public publication export", path: "/api/platform/publications/export?route=/article&format=json"},
		{name: "public retrieval search", path: "/api/platform/retrieval/search?q=article"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.authenticated {
				req.Header.Set("Authorization", "Bearer "+secret)
			}
			rec := httptest.NewRecorder()
			handler.HandleAPI(rec, req)
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				t.Fatalf("non-computer route was authority-blocked: status=%d body=%s", rec.Code, rec.Body.String())
			}
			if vmctlCalls.Load() != 0 {
				t.Fatalf("vmctl calls=%d want zero", vmctlCalls.Load())
			}
		})
	}
}

func TestOwnerWideAdminComputeRecoveryDeniedBeforeMissingVMCTLAndBodyDecode(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("owner-wide-recovery-nil-vmctl", "owner-wide-recovery-nil-vmctl@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(t.Context(), user.ID, "owner-wide admin", []string{"admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	handler.vmctlClient = nil
	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{`))
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestBoundAPIKeyUsesExactJoinedSandboxWithoutDesktopReresolve(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("exact-sandbox-owner", "exact-sandbox-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	const computerID = "computer-exact-sandbox"
	_, secret, err := store.CreateComputerScopedAPIKey(t.Context(), user.ID, "exact sandbox", []string{"read:runtime"}, computerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var exactCalls atomic.Int64
	exactSandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exactCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"user": r.Header.Get("X-Authenticated-User")})
	}))
	defer exactSandbox.Close()
	var wrongSandboxCalls atomic.Int64
	wrongSandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrongSandboxCalls.Add(1)
		http.Error(w, "wrong sandbox", http.StatusTeapot)
	}))
	defer wrongSandbox.Close()
	var desktopResolveCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/vmctl/lookup" && r.URL.Query().Get("computer_id") == computerID {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": computerID, "user_id": user.ID, "desktop_id": "primary", "vm_id": "vm-exact",
				"sandbox_url": exactSandbox.URL, "state": "active", "epoch": 7,
			})
			return
		}
		desktopResolveCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-wrong", "user_id": user.ID, "desktop_id": "primary", "vm_id": "vm-wrong",
			"sandbox_url": wrongSandbox.URL, "state": "active", "epoch": 1,
		})
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if exactCalls.Load() != 1 || desktopResolveCalls.Load() != 0 || wrongSandboxCalls.Load() != 0 {
		t.Fatalf("exact=%d desktop_resolve=%d wrong=%d want 1/0/0", exactCalls.Load(), desktopResolveCalls.Load(), wrongSandboxCalls.Load())
	}
}

func TestBoundAPIKeyComputeStatusNeverListsOrExposesSiblingComputers(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("bound-status-owner", "bound-status-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	const computerID = "computer-bound-status"
	_, secret, err := store.CreateComputerScopedAPIKey(t.Context(), user.ID, "status", []string{"read:runtime"}, computerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var computerLookups atomic.Int64
	var desktopLookups atomic.Int64
	var listCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/vmctl/lookup" && r.URL.Query().Get("computer_id") == computerID:
			computerLookups.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": computerID, "user_id": user.ID, "desktop_id": "primary", "vm_id": "vm-status",
				"state": "active", "epoch": 9,
			})
		case r.URL.Path == "/internal/vmctl/lookup" && r.URL.Query().Get("desktop_id") == "primary":
			desktopLookups.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": computerID, "user_id": user.ID, "desktop_id": "primary", "vm_id": "vm-status",
				"state": "active", "epoch": 9,
			})
		case r.URL.Path == "/internal/vmctl/list":
			listCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"computers": []any{
				map[string]any{"computer_id": computerID, "user_id": user.ID, "desktop_id": "primary"},
				map[string]any{"computer_id": "computer-sibling", "user_id": user.ID, "desktop_id": "sibling"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response computeStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Computers) != 1 || response.Computers[0].DesktopID != "primary" {
		t.Fatalf("computers=%+v want only bound primary", response.Computers)
	}
	if computerLookups.Load() != 1 || desktopLookups.Load() != 1 || listCalls.Load() != 0 {
		t.Fatalf("computer_lookups=%d desktop_lookups=%d list_calls=%d want 1/1/0", computerLookups.Load(), desktopLookups.Load(), listCalls.Load())
	}
}

func TestBoundAPIKeyDesktopMismatchDeniedAcrossComputerRouteFamilies(t *testing.T) {
	handler, _, sandbox, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("desktop-mismatch-owner", "desktop-mismatch-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	const computerID = "computer-desktop-mismatch"
	_, secret, err := store.CreateComputerScopedAPIKey(t.Context(), user.ID, "bound admin", []string{"admin"}, computerID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var guardLookups atomic.Int64
	var downstreamCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/vmctl/lookup" && r.URL.Query().Get("computer_id") == computerID {
			guardLookups.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": computerID, "user_id": user.ID, "desktop_id": "primary", "vm_id": "vm-primary",
				"sandbox_url": sandbox.URL, "state": "active", "epoch": 3,
			})
			return
		}
		downstreamCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	sandbox.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	})
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	}))
	defer corpusd.Close()
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "bootstrap", method: http.MethodGet, path: "/api/shell/bootstrap?desktop_id=branch"},
		{name: "protected HTTP", method: http.MethodGet, path: "/api/base/delta?desktop_id=branch"},
		{name: "websocket", method: http.MethodGet, path: "/api/ws?desktop_id=branch"},
		{name: "super console websocket", method: http.MethodGet, path: "/api/super-console/ws?desktop_id=branch"},
		{name: "compute status", method: http.MethodGet, path: "/api/compute/status?desktop_id=branch"},
		{name: "execution identity", method: http.MethodGet, path: "/api/acceptance/execution-identity?desktop_id=branch"},
		{name: "texture publication", method: http.MethodPost, path: "/api/platform/texture/publications?desktop_id=branch", body: `{"doc_id":"doc-a"}`},
		{name: "publication proposal", method: http.MethodPost, path: "/api/platform/publications/pub-a/proposals?desktop_id=branch", body: `{"doc_id":"doc-a"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := guardLookups.Load()
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			req.Header.Set("Authorization", "Bearer "+secret)
			rec := httptest.NewRecorder()
			handler.HandleAPI(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if guardLookups.Load() != before+1 || downstreamCalls.Load() != 0 {
				t.Fatalf("guard lookup delta=%d downstream=%d want 1/0", guardLookups.Load()-before, downstreamCalls.Load())
			}
		})
	}

	for _, path := range []string{
		"/api/computers/computer-other/lifecycle/status",
		"/api/computers/computer-other/self-development/mode",
	} {
		before := guardLookups.Load()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		rec := httptest.NewRecorder()
		handler.HandleAPI(rec, req)
		if rec.Code != http.StatusForbidden || guardLookups.Load() != before || downstreamCalls.Load() != 0 {
			t.Fatalf("path=%s status=%d lookup_delta=%d downstream=%d", path, rec.Code, guardLookups.Load()-before, downstreamCalls.Load())
		}
	}
}

func TestOwnerWideAdminAPIKeyComputeStatusListsEveryOwnedInteractiveComputer(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("owner-wide-status", "owner-wide-status@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(t.Context(), user.ID, "owner-wide status", []string{"read:runtime"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	const primaryID = "computer-ow-primary"
	const branchID = "computer-ow-branch"
	var listCalls atomic.Int64
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/internal/vmctl/lookup" && r.URL.Query().Get("desktop_id") == "primary":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": primaryID, "user_id": user.ID, "desktop_id": "primary", "vm_id": "vm-ow-primary",
				"kind": "interactive", "state": "active", "epoch": 11,
			})
		case r.URL.Path == "/internal/vmctl/list":
			listCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"ownerships": []any{
				map[string]any{"computer_id": primaryID, "user_id": user.ID, "desktop_id": "primary", "kind": "interactive"},
				map[string]any{"computer_id": branchID, "user_id": user.ID, "desktop_id": "branch", "kind": "interactive"},
				map[string]any{"computer_id": "computer-ow-worker", "user_id": user.ID, "desktop_id": "worker", "kind": "worker"},
				map[string]any{"computer_id": "computer-foreign", "user_id": "somebody-else", "desktop_id": "primary", "kind": "interactive"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer vmctlServer.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleAPI(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response computeStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	desktopIDs := make([]string, 0, len(response.Computers))
	for _, c := range response.Computers {
		desktopIDs = append(desktopIDs, c.DesktopID)
	}
	if len(desktopIDs) != 2 || desktopIDs[0] != "primary" || desktopIDs[1] != "branch" {
		t.Fatalf("computers=%v want [primary branch] only (worker and foreign filtered)", desktopIDs)
	}
	if listCalls.Load() != 1 {
		t.Fatalf("list_calls=%d want=1", listCalls.Load())
	}
}
