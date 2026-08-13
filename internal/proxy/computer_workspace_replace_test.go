package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestParseComputerLifecyclePathRejectsWorkspaceReplace(t *testing.T) {
	if _, _, ok := parseComputerLifecyclePath("/api/computers/computer-a/lifecycle/replace-workspace"); ok {
		t.Fatal("workspace replace must not use VM lifecycle control")
	}
	if _, ok := computerWorkspaceReplaceComputerID("/api/computers/computer-a/lifecycle/replace-workspace"); !ok {
		t.Fatal("workspace replace path should parse")
	}
	if _, _, ok := parseComputerLifecyclePath("/api/computers/computer-a/lifecycle/rematerialize-from-tape"); ok {
		t.Fatal("tape rematerialize must not use VM lifecycle control")
	}
	if _, ok := computerRematerializeComputerID("/api/computers/computer-a/lifecycle/rematerialize-from-tape"); !ok {
		t.Fatal("tape rematerialize path should parse")
	}
	for path, want := range map[string]bool{
		"/api/computers/computer-a/lifecycle/replace-workspace":        true,
		"/api/computers//lifecycle/replace-workspace":                  false,
		"/api/computers/a/b/lifecycle/replace-workspace":               false,
		"/api/computers/a/lifecycle/replace-workspace/extra":           false,
		"/api/computers/computer-a/self-development/replace-workspace": false,
		"/api/computers/computer-a/self-development/genesis":           false,
	} {
		if _, got := computerWorkspaceReplaceComputerID(path); got != want {
			t.Errorf("path %q accepted=%v, want %v", path, got, want)
		}
	}
}

func TestWorkspaceReplaceForwardsOwnedComputerAndTrustedBinding(t *testing.T) {
	var gotUser, gotComputer, gotPath, gotMethod string
	var corpusdCalls, stops, resolves int
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-Authenticated-User")
		gotComputer = r.Header.Get("X-Authenticated-Computer")
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "appended_event": false, "published_checkpoint": false, "store_closed": true,
		})
	}))
	defer autoputer.Close()

	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("computer_id") != "computer-a" || r.URL.Query().Get("user_id") != "owner-user" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user",
				"state": "active", "computer_url": autoputer.URL,
			})
		case "/internal/vmctl/stop":
			stops++
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/internal/vmctl/resolve":
			resolves++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user",
				"state": "active", "computer_url": autoputer.URL,
			})
		default:
			t.Fatalf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	defer ownership.Close()

	corpusd := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		corpusdCalls++
		t.Fatal("workspace replace must not call lifecycle control")
	}))
	defer corpusd.Close()

	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/replace-workspace", nil)
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("workspace replace status=%d body=%s", response.Code, response.Body.String())
	}
	if gotUser != "owner-user" || gotComputer != "computer-a" || gotPath != request.URL.Path || gotMethod != http.MethodPost {
		t.Fatalf("upstream binding user=%q computer=%q path=%q method=%q", gotUser, gotComputer, gotPath, gotMethod)
	}
	if corpusdCalls != 0 || stops != 0 || resolves != 0 {
		t.Fatalf("workspace replace used corpusd=%d stops=%d resolves=%d", corpusdCalls, stops, resolves)
	}

	attacker := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/replace-workspace", nil)
	attacker.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "attacker-user")})
	attackerResponse := httptest.NewRecorder()
	handler.HandleAPI(attackerResponse, attacker)
	if attackerResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-owner replace status=%d body=%s", attackerResponse.Code, attackerResponse.Body.String())
	}
}

func TestRematerializeFromTapeForwardsOwnedComputerAndTrustedBinding(t *testing.T) {
	var gotUser, gotComputer, gotPath, gotMethod string
	var corpusdCalls, stops, resolves int
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-Authenticated-User")
		gotComputer = r.Header.Get("X-Authenticated-Computer")
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "witness_matched": true, "original_denied": true, "store_closed": true,
		})
	}))
	defer autoputer.Close()

	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("computer_id") != "computer-a" || r.URL.Query().Get("user_id") != "owner-user" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user",
				"state": "active", "computer_url": autoputer.URL,
			})
		case "/internal/vmctl/stop":
			stops++
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/internal/vmctl/resolve":
			resolves++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user",
				"state": "active", "computer_url": autoputer.URL,
			})
		default:
			t.Fatalf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	defer ownership.Close()

	corpusd := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		corpusdCalls++
		t.Fatal("tape rematerialize must not call lifecycle control")
	}))
	defer corpusd.Close()

	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/rematerialize-from-tape", strings.NewReader(`{"checkpoint":{"digest":"abc"}}`))
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("tape rematerialize status=%d body=%s", response.Code, response.Body.String())
	}
	if gotUser != "owner-user" || gotComputer != "computer-a" || gotPath != request.URL.Path || gotMethod != http.MethodPost {
		t.Fatalf("upstream binding user=%q computer=%q path=%q method=%q", gotUser, gotComputer, gotPath, gotMethod)
	}
	if corpusdCalls != 0 || stops != 0 || resolves != 0 {
		t.Fatalf("tape rematerialize used corpusd=%d stops=%d resolves=%d", corpusdCalls, stops, resolves)
	}
}

func TestWorkspaceReplaceRequiresLifecycleScope(t *testing.T) {
	var guestCalls int
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guestCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "store_closed": true})
	}))
	defer autoputer.Close()

	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("replace-owner", "replace-owner@example.com")
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

	denied := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/replace-workspace", nil)
	denied.Header.Set("Authorization", "Bearer "+readSecret)
	deniedResponse := httptest.NewRecorder()
	handler.HandleAPI(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden || guestCalls != 0 {
		t.Fatalf("read-scope replace status=%d guest_calls=%d body=%s", deniedResponse.Code, guestCalls, deniedResponse.Body.String())
	}

	allowed := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/replace-workspace", nil)
	allowed.Header.Set("Authorization", "Bearer "+lifecycleSecret)
	allowedResponse := httptest.NewRecorder()
	handler.HandleAPI(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusOK || guestCalls != 1 {
		t.Fatalf("lifecycle-scope replace status=%d guest_calls=%d body=%s", allowedResponse.Code, guestCalls, allowedResponse.Body.String())
	}
}

func TestWorkspaceReplaceRejectsGenesisPath(t *testing.T) {
	handler, _, _ := testProxyEnv(t)
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/genesis", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("genesis status=%d body=%s", response.Code, response.Body.String())
	}
}
