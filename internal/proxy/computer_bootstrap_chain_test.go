package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestParseComputerBootstrapChainPath(t *testing.T) {
	for path, want := range map[string]bool{
		"/api/computers/computer-a/lifecycle/bootstrap-chain":        true,
		"/api/computers//lifecycle/bootstrap-chain":                  false,
		"/api/computers/a/b/lifecycle/bootstrap-chain":               false,
		"/api/computers/a/lifecycle/bootstrap-chain/extra":           false,
		"/api/computers/computer-a/self-development/bootstrap-chain": false,
		"/api/computers/computer-a/self-development/genesis":         false,
	} {
		if _, got := computerBootstrapChainComputerID(path); got != want {
			t.Errorf("path %q accepted=%v, want %v", path, got, want)
		}
	}
}

func TestBootstrapChainForwardsOwnedComputerAndTrustedBinding(t *testing.T) {
	var gotUser, gotComputer, gotPath, gotMethod string
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-Authenticated-User")
		gotComputer = r.Header.Get("X-Authenticated-Computer")
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "appended_event": true, "published_checkpoint": false,
		})
	}))
	defer autoputer.Close()

	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup" || r.URL.Query().Get("computer_id") != "computer-a" || r.URL.Query().Get("user_id") != "owner-user" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user",
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()

	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/bootstrap-chain", nil)
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("bootstrap-chain status=%d body=%s", response.Code, response.Body.String())
	}
	if gotUser != "owner-user" || gotComputer != "computer-a" || gotPath != request.URL.Path || gotMethod != http.MethodPost {
		t.Fatalf("upstream binding user=%q computer=%q path=%q method=%q", gotUser, gotComputer, gotPath, gotMethod)
	}

	attacker := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/bootstrap-chain", nil)
	attacker.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "attacker-user")})
	attackerResponse := httptest.NewRecorder()
	handler.HandleAPI(attackerResponse, attacker)
	if attackerResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-owner bootstrap status=%d body=%s", attackerResponse.Code, attackerResponse.Body.String())
	}
}

func TestBootstrapChainRequiresLifecycleScope(t *testing.T) {
	var guestCalls int
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guestCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "appended_event": true})
	}))
	defer autoputer.Close()

	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("bootstrap-owner", "bootstrap-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, readSecret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "read", []string{"computer:self_development:read"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, lifecycleSecret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "lifecycle", []string{"computer:lifecycle"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup" || r.URL.Query().Get("computer_id") != "computer-a" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	denied := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/bootstrap-chain", nil)
	denied.Header.Set("Authorization", "Bearer "+readSecret)
	deniedResponse := httptest.NewRecorder()
	handler.HandleAPI(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden || guestCalls != 0 {
		t.Fatalf("read-scope bootstrap status=%d guest_calls=%d body=%s", deniedResponse.Code, guestCalls, deniedResponse.Body.String())
	}

	allowed := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/bootstrap-chain", nil)
	allowed.Header.Set("Authorization", "Bearer "+lifecycleSecret)
	allowedResponse := httptest.NewRecorder()
	handler.HandleAPI(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusOK || guestCalls != 1 {
		t.Fatalf("lifecycle-scope bootstrap status=%d guest_calls=%d body=%s", allowedResponse.Code, guestCalls, allowedResponse.Body.String())
	}
}

func TestBootstrapChainDoesNotTouchSelfDevGenesis(t *testing.T) {
	handler, _, _ := testProxyEnv(t)
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/genesis", nil)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("selfdev genesis status=%d body=%s", response.Code, response.Body.String())
	}
}
