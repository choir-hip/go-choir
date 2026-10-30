package proxy

// The owner's checkpoint surface (slice 3 of
// docs/archive/design/apply-checkpoint-without-replay-2026-10-26.md), written before
// the code. Ways it could fail:
//
//  1. the listing is forwarded without the lifecycle scope, or an API key for
//     another computer reads it;
//  2. the owner act that reopens a closed cadence is reachable by an API key
//     without owner:acts, or by GET;
//  3. the deleted manual bind is still given the dedicated long budget and a
//     lifecycle-scope check, as if it were a live route.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestComputerCheckpointsListingForwardsUnderTheLifecycleScope(t *testing.T) {
	var guestMethod, guestPath, guestQuery string
	var guestCalls int
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guestCalls++
		guestMethod, guestPath, guestQuery = r.Method, r.URL.Path, r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{"cadence": map[string]any{"state": "on"}, "checkpoints": []any{}})
	}))
	defer autoputer.Close()

	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("checkpoints-owner", "checkpoints-owner@example.com")
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
	_, otherSecret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "other", []string{"computer:lifecycle"}, "computer-b", nil)
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": r.URL.Query().Get("computer_id"), "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	get := func(secret, query string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/lifecycle/checkpoints"+query, nil)
		request.Header.Set("Authorization", "Bearer "+secret)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		return response
	}
	if response := get(readSecret, ""); response.Code != http.StatusForbidden || guestCalls != 0 {
		t.Fatalf("read-scope listing = %d (guest calls %d)", response.Code, guestCalls)
	}
	if response := get(otherSecret, ""); response.Code == http.StatusOK || guestCalls != 0 {
		t.Fatalf("another computer's key read the listing = %d (guest calls %d)", response.Code, guestCalls)
	}
	response := get(lifecycleSecret, "?checkpoint_digest=abc")
	if response.Code != http.StatusOK || guestCalls != 1 {
		t.Fatalf("lifecycle-scope listing = %d (guest calls %d) %s", response.Code, guestCalls, response.Body.String())
	}
	if guestMethod != http.MethodGet || guestPath != "/api/computers/computer-a/lifecycle/checkpoints" || guestQuery != "checkpoint_digest=abc" {
		t.Fatalf("guest saw %s %s?%s", guestMethod, guestPath, guestQuery)
	}
}

func TestResumeCheckpointsIsAnOwnerActOnTheProxy(t *testing.T) {
	var guestCalls int
	var guestMethod string
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guestCalls++
		guestMethod = r.Method
		_ = json.NewEncoder(w).Encode(map[string]any{"state": "on"})
	}))
	defer autoputer.Close()

	handler, privateKey, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("resume-owner", "resume-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, lifecycleSecret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "lifecycle", []string{"computer:lifecycle"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, ownerSecret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "owner", []string{"computer:lifecycle", "owner:acts"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup" {
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

	post := func(secret string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/resume-checkpoints", nil)
		request.Header.Set("Authorization", "Bearer "+secret)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		return response
	}
	if response := post(lifecycleSecret); response.Code != http.StatusForbidden || guestCalls != 0 {
		t.Fatalf("a lifecycle key without owner:acts reopened the cadence: %d (guest calls %d)", response.Code, guestCalls)
	}
	if response := post(ownerSecret); response.Code != http.StatusOK || guestCalls != 1 || guestMethod != http.MethodPost {
		t.Fatalf("owner act = %d (guest calls %d, method %s) %s", response.Code, guestCalls, guestMethod, response.Body.String())
	}
	get := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/lifecycle/resume-checkpoints", nil)
	get.Header.Set("Authorization", "Bearer "+ownerSecret)
	getResponse := httptest.NewRecorder()
	handler.HandleAPI(getResponse, get)
	if getResponse.Code != http.StatusMethodNotAllowed || guestCalls != 1 {
		t.Fatalf("GET resume = %d (guest calls %d)", getResponse.Code, guestCalls)
	}
	_ = privateKey
}
