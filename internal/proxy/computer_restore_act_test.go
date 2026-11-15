package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Restore is an accepted act (docs/problems/restore-cannot-finish-inside-its-
// ten-minute-request-2026-11-14.md, failure list 17): the guest answers 202 at
// once, so the proxy forwards restore and rematerialize on the ordinary
// client, not the ten-minute one, and the status read is a GET gated as the
// act is.

// restoreActGuest is a guest that records what the proxy sent and answers as
// the accepted-act routes do.
type restoreActGuest struct {
	server   *httptest.Server
	requests []string
	queries  []string
}

func newRestoreActGuest(t *testing.T) *restoreActGuest {
	t.Helper()
	g := &restoreActGuest{}
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.requests = append(g.requests, r.Method+" "+r.URL.Path)
		g.queries = append(g.queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]any{"act_id": "ra-1", "state": "accepted", "status_url": "/api/computers/computer-a/lifecycle/restore-status?act=ra-1"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"act_id": "ra-1", "state": "replaying"})
	}))
	t.Cleanup(g.server.Close)
	return g
}

func restoreActOwnership(t *testing.T, guestURL, userID string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup" || r.URL.Query().Get("computer_id") != "computer-a" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": userID,
			"state": "active", "computer_url": guestURL,
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRestoreRoutesUseTheOrdinaryClientAnd202(t *testing.T) {
	for _, action := range []string{"restore", "rematerialize-from-tape"} {
		t.Run(action, func(t *testing.T) {
			guest := newRestoreActGuest(t)
			ownership := restoreActOwnership(t, guest.server.URL, "owner-user")
			handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
			handler.vmctlClient = vmctl.NewClient(ownership.URL)
			handler.autoputerHTTP = &http.Client{Timeout: 5 * time.Second}
			// The ten-minute client must not carry these routes any more: this one
			// fails every request, so a route that still used it would answer 502.
			handler.replayAutoputerHTTP = &http.Client{Timeout: time.Nanosecond}

			request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/"+action, strings.NewReader("{}"))
			request.Header.Set("Content-Type", "application/json")
			request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
			response := httptest.NewRecorder()
			handler.HandleAPI(response, request)
			if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"act_id":"ra-1"`) {
				t.Fatalf("%s status=%d body=%s, want the guest's 202 unchanged", action, response.Code, response.Body.String())
			}
			if len(guest.requests) != 1 {
				t.Fatalf("guest requests = %v", guest.requests)
			}
		})
	}
}

func TestRestoreStatusRouteIsAnOwnerAct(t *testing.T) {
	guest := newRestoreActGuest(t)
	handler, privateKey, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("restore-status-owner", "restore-status-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	mint := func(scopes ...string) string {
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, strings.Join(scopes, "+"), scopes, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		return secret
	}
	lifecycleOnly := mint("computer:lifecycle")
	withOwnerActs := mint("computer:lifecycle", "owner:acts")
	readOnly := mint("computer:self_development:read", "owner:acts")
	ownership := restoreActOwnership(t, guest.server.URL, user.ID)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	handler.autoputerHTTP = &http.Client{Timeout: 5 * time.Second}

	send := func(auth func(*http.Request), method string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, "/api/computers/computer-a/lifecycle/restore-status?act=ra-1", nil)
		auth(request)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		return response
	}
	bearer := func(secret string) func(*http.Request) {
		return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+secret) }
	}

	if response := send(func(*http.Request) {}, http.MethodGet); response.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status read = %d, want 401", response.Code)
	}
	if response := send(bearer(lifecycleOnly), http.MethodGet); response.Code != http.StatusForbidden || len(guest.requests) != 0 {
		t.Fatalf("a lifecycle key without owner:acts read the status: %d %s (guest saw %v)", response.Code, response.Body.String(), guest.requests)
	}
	if response := send(bearer(readOnly), http.MethodGet); response.Code != http.StatusForbidden || len(guest.requests) != 0 {
		t.Fatalf("a key without the lifecycle scope read the status: %d (guest saw %v)", response.Code, guest.requests)
	}
	response := send(bearer(withOwnerActs), http.MethodGet)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"state":"replaying"`) {
		t.Fatalf("owner-act key status read = %d %s", response.Code, response.Body.String())
	}
	if len(guest.requests) != 1 || guest.requests[0] != "GET /api/computers/computer-a/lifecycle/restore-status" || guest.queries[0] != "act=ra-1" {
		t.Fatalf("guest saw %v %v, want the GET with its act query", guest.requests, guest.queries)
	}
	if response := send(bearer(withOwnerActs), http.MethodDelete); response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE restore-status = %d, want 405", response.Code)
	}
	if response := send(func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "attacker-user")})
	}, http.MethodGet); response.Code != http.StatusForbidden {
		t.Fatalf("another owner read the status: %d", response.Code)
	}
	if response := send(func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, user.ID)})
	}, http.MethodGet); response.Code != http.StatusOK {
		t.Fatalf("the owner's session read the status: %d %s", response.Code, response.Body.String())
	}
}
