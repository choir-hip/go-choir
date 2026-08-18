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

func TestSelfDevelopmentModeRequiresExactComputerScope(t *testing.T) {
	var calls int
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/internal/computers/self-development/mode" || r.URL.Query().Get("computer_id") != "computer-a" {
			t.Fatalf("corpusd target = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("X-Internal-Caller") != "true" || r.Header.Get("Authorization") != "" {
			t.Fatalf("corpusd authority headers = %#v", r.Header)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "mode": "off", "generation": 0})
	}))
	defer corpusd.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	user, err := store.CreateUser("selfdev-mode-user", "selfdev-mode@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "selfdev", []string{"computer:self_development:read"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup" || r.URL.Query().Get("user_id") != user.ID || r.URL.Query().Get("computer_id") != "computer-a" {
			t.Fatalf("ownership lookup = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID, "state": "active",
		})
	}))
	defer ownership.Close()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	request := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/self-development/mode", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK || calls != 1 {
		t.Fatalf("authorized response=%d calls=%d body=%s", response.Code, calls, response.Body.String())
	}

	wrongTarget := httptest.NewRequest(http.MethodGet, "/api/computers/computer-b/self-development/mode", nil)
	wrongTarget.Header.Set("Authorization", "Bearer "+secret)
	wrongResponse := httptest.NewRecorder()
	handler.HandleAPI(wrongResponse, wrongTarget)
	if wrongResponse.Code != http.StatusForbidden || calls != 1 {
		t.Fatalf("wrong-target response=%d calls=%d", wrongResponse.Code, calls)
	}

	writeRequest := httptest.NewRequest(http.MethodPut, "/api/computers/computer-a/self-development/mode", nil)
	writeRequest.Header.Set("Authorization", "Bearer "+secret)
	writeResponse := httptest.NewRecorder()
	handler.HandleAPI(writeResponse, writeRequest)
	if writeResponse.Code != http.StatusForbidden || calls != 1 {
		t.Fatalf("read-only key write response=%d calls=%d", writeResponse.Code, calls)
	}
}

func TestSelfDevelopmentModeCookieRequiresTargetOwnership(t *testing.T) {
	var corpusdCalls int
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpusdCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "mode": "off", "generation": 0})
	}))
	defer corpusd.Close()
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("computer_id") != "computer-a" {
			t.Fatalf("ownership target = %q", r.URL.Query().Get("computer_id"))
		}
		if r.URL.Query().Get("user_id") != "owner-user" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user", "state": "active",
		})
	}))
	defer ownership.Close()
	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	attacker := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/self-development/mode", nil)
	attacker.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "attacker-user")})
	attackerResponse := httptest.NewRecorder()
	handler.HandleAPI(attackerResponse, attacker)
	if attackerResponse.Code != http.StatusForbidden || corpusdCalls != 0 {
		t.Fatalf("cross-owner mode response=%d corpusd_calls=%d", attackerResponse.Code, corpusdCalls)
	}

	owner := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/self-development/mode", nil)
	owner.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	ownerResponse := httptest.NewRecorder()
	handler.HandleAPI(ownerResponse, owner)
	if ownerResponse.Code != http.StatusOK || corpusdCalls != 1 {
		t.Fatalf("owner mode response=%d corpusd_calls=%d body=%s", ownerResponse.Code, corpusdCalls, ownerResponse.Body.String())
	}
}

func TestSelfDevelopmentModePathRequiresSingleEscapedComputerID(t *testing.T) {
	for path, want := range map[string]bool{
		"/api/computers/computer-a/self-development/mode": true,
		"/api/computers//self-development/mode":           false,
		"/api/computers/a/b/self-development/mode":        false,
		"/api/computers/a/self-development/mode/extra":    false,
	} {
		if _, got := selfDevelopmentModeComputerID(path); got != want {
			t.Fatalf("path %q accepted=%v, want %v", path, got, want)
		}
	}
}
func TestReplayCompletenessPathUsesOwnedComputerAndTrustedBinding(t *testing.T) {
	var gotUser, gotComputer, gotPath string
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-Authenticated-User")
		gotComputer = r.Header.Get("X-Authenticated-Computer")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer autoputer.Close()

	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("computer_id") != "computer-a" || r.URL.Query().Get("user_id") != "owner-user" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user",
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()

	handler, privateKey, _ := testProxyEnv(t)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	request := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/self-development/replay-completeness", nil)
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("replay completeness status=%d body=%s", response.Code, response.Body.String())
	}
	if gotUser != "owner-user" || gotComputer != "computer-a" || gotPath != request.URL.Path {
		t.Fatalf("upstream binding user=%q computer=%q path=%q", gotUser, gotComputer, gotPath)
	}

	for path, want := range map[string]bool{
		"/api/computers/computer-a/self-development/replay-completeness": true,
		"/api/computers//self-development/replay-completeness":           false,
		"/api/computers/a/b/self-development/replay-completeness":        false,
		"/api/computers/a/self-development/replay-completeness/extra":    false,
	} {
		if _, got := selfDevelopmentReplayCompletenessComputerID(path); got != want {
			t.Errorf("path %q accepted=%v, want %v", path, got, want)
		}
	}
}

func TestReplayCompletenessUsesDedicatedUpstreamTimeout(t *testing.T) {
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/computers/computer-a/self-development/replay-completeness" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		select {
		case <-time.After(80 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer autoputer.Close()

	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("computer_id") != "computer-a" || r.URL.Query().Get("user_id") != "owner-user" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user",
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()

	handler, privateKey, _ := testProxyEnv(t)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	handler.autoputerHTTP = &http.Client{Timeout: 5 * time.Millisecond}

	handler.replayAutoputerHTTP = &http.Client{Timeout: 5 * time.Millisecond}
	timedOutRequest := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/self-development/replay-completeness", nil)
	timedOutRequest.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	timedOutResponse := httptest.NewRecorder()
	handler.HandleAPI(timedOutResponse, timedOutRequest)
	if timedOutResponse.Code != http.StatusBadGateway {
		t.Fatalf("timed-out replay completeness status=%d body=%s", timedOutResponse.Code, timedOutResponse.Body.String())
	}

	handler.replayAutoputerHTTP = &http.Client{Timeout: 500 * time.Millisecond}
	request := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/self-development/replay-completeness", nil)
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("replay completeness status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPublicGenesisRemainsEffectsDisabled(t *testing.T) {
	handler, _, autoputer := testProxyEnv(t)
	defer autoputer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/genesis", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "self-development effects are disabled") {
		t.Fatalf("genesis status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPublicProposalAndDecisionRequireAuthThenForwardGuestRefuse(t *testing.T) {
	handler, _, autoputer := testProxyEnv(t)
	defer autoputer.Close()
	proposal := httptest.NewRequest(http.MethodPost, "/api/computers/computer-propose/self-development/operations", strings.NewReader(`{"idempotency_key":"proposal-mode","prompt":"change runtime"}`))
	proposalResponse := httptest.NewRecorder()
	handler.HandleAPI(proposalResponse, proposal)
	if proposalResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated proposal status=%d body=%s", proposalResponse.Code, proposalResponse.Body.String())
	}
	decision := httptest.NewRequest(http.MethodPost, "/api/computers/computer-decision/self-development/operations/operation-decision/decision", strings.NewReader(`{"decision":"reject"}`))
	decisionResponse := httptest.NewRecorder()
	handler.HandleAPI(decisionResponse, decision)
	if decisionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated decision status=%d body=%s", decisionResponse.Code, decisionResponse.Body.String())
	}
}

func TestSelfDevelopmentGuestForwardsProposeAndPassesModeOffRefuse(t *testing.T) {
	var gotUser, gotComputer, gotPath, gotMethod string
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = r.Header.Get("X-Authenticated-User")
		gotComputer = r.Header.Get("X-Authenticated-Computer")
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"current signed mode does not authorize proposal"}`))
	}))
	defer autoputer.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("selfdev-propose-user", "selfdev-propose@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "propose", []string{"computer:self_development:propose"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)

	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/operations", strings.NewReader(`{"idempotency_key":"proposal-mode","prompt":"change runtime"}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("mode-off proposal status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "does not authorize proposal") {
		t.Fatalf("guest refuse not forwarded: %s", response.Body.String())
	}
	if gotUser != user.ID || gotComputer != "computer-a" || gotPath != "/api/computers/computer-a/self-development/operations" || gotMethod != http.MethodPost {
		t.Fatalf("upstream binding user=%q computer=%q path=%q method=%q", gotUser, gotComputer, gotPath, gotMethod)
	}

	denied := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/operations", strings.NewReader(`{"idempotency_key":"proposal-mode","prompt":"change runtime"}`))
	_, readSecret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "read2", []string{"computer:self_development:read"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	denied.Header.Set("Authorization", "Bearer "+readSecret)
	deniedResponse := httptest.NewRecorder()
	handler.HandleAPI(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("read-scope propose status=%d body=%s", deniedResponse.Code, deniedResponse.Body.String())
	}
}

func TestSelfDevelopmentGuestForwardsDecisionWithApproveScope(t *testing.T) {
	var gotPath string
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"current signed mode does not authorize proposal"}`))
	}))
	defer autoputer.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("selfdev-approve-user", "selfdev-approve@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "approve", []string{"computer:self_development:approve"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/operations/operation-a/decision", strings.NewReader(`{"decision":"reject"}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusConflict || gotPath != "/api/computers/computer-a/self-development/operations/operation-a/decision" {
		t.Fatalf("decision status=%d path=%q body=%s", response.Code, gotPath, response.Body.String())
	}
}
