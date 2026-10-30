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
		if r.URL.Path == "/internal/vmctl/touch" {
			_, _ = w.Write([]byte(`{"touched":true}`))
			return
		}
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

type replayDeadlineResponseWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *replayDeadlineResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}

func TestReplayCompletenessExtendsOnlyRouteWriteDeadline(t *testing.T) {
	const routeTimeout = 2 * time.Second
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer autoputer.Close()

	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": "owner-user",
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()

	handler, privateKey, _ := testProxyEnv(t)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	handler.replayAutoputerHTTP = &http.Client{Timeout: routeTimeout}
	request := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/self-development/replay-completeness", nil)
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	response := &replayDeadlineResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	started := time.Now()

	handler.HandleAPI(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("replay completeness status=%d body=%s", response.Code, response.Body.String())
	}
	if response.deadline.Before(started.Add(routeTimeout)) {
		t.Fatalf("route deadline=%s, want at least %s after start", response.deadline, routeTimeout)
	}
	if response.deadline.After(started.Add(routeTimeout + 2*replayCompletenessWriteGrace)) {
		t.Fatalf("route deadline=%s, unexpectedly beyond route budget plus grace", response.deadline)
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
	if proposalResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unauthenticated proposal status=%d body=%s, want 405 (the start-operation POST is gone)", proposalResponse.Code, proposalResponse.Body.String())
	}
	decision := httptest.NewRequest(http.MethodPost, "/api/computers/computer-decision/self-development/operations/operation-decision/decision", strings.NewReader(`{"decision":"reject"}`))
	decisionResponse := httptest.NewRecorder()
	handler.HandleAPI(decisionResponse, decision)
	if decisionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated decision status=%d body=%s", decisionResponse.Code, decisionResponse.Body.String())
	}
}

// C1e B1 deleted the guest's start-operation POST, so the proxy no longer maps
// POST .../self-development/operations to a "propose" action: it answers 405
// before auth, the guest is never called, and no scope (the retired
// computer:self_development:propose, or admin) reopens it
// (docs/archive/problems/surface-broken-calls-2026-10-14.md, item 10).
func TestSelfDevelopmentOperationsPostIsRefusedAtTheProxy(t *testing.T) {
	guestCalls := 0
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guestCalls++
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer autoputer.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("selfdev-propose-user", "selfdev-propose@example.com")
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

	for _, scope := range []string{"computer:self_development:propose", "admin"} {
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, scope, []string{scope}, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/operations", strings.NewReader(`{"idempotency_key":"proposal-mode","prompt":"change runtime"}`))
		request.Header.Set("Authorization", "Bearer "+secret)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s operations POST status=%d body=%s, want 405", scope, response.Code, response.Body.String())
		}
	}
	if guestCalls != 0 {
		t.Fatalf("guest called %d times for a refused operations POST", guestCalls)
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

// Listing the operations collection is a read: a read-scoped key lists, and a
// key holding only the retired propose scope may not read them through this
// route.
func TestSelfDevelopmentGuestListsOperationsWithReadScope(t *testing.T) {
	var gotPath, gotMethod, gotQuery string
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod, gotQuery = r.URL.Path, r.Method, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"operations":[]}`))
	}))
	defer autoputer.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("selfdev-list-user", "selfdev-list@example.com")
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
	list := func(scope string) *httptest.ResponseRecorder {
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "list-"+scope, []string{scope}, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/self-development/operations?state=awaiting_approval", nil)
		request.Header.Set("Authorization", "Bearer "+secret)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		return response
	}
	if response := list("computer:self_development:read"); response.Code != http.StatusOK || gotMethod != http.MethodGet ||
		gotPath != "/api/computers/computer-a/self-development/operations" || gotQuery != "state=awaiting_approval" {
		t.Fatalf("read list status=%d upstream=%s %s?%s body=%s", response.Code, gotMethod, gotPath, gotQuery, response.Body.String())
	}
	if response := list("computer:self_development:propose"); response.Code != http.StatusForbidden {
		t.Fatalf("propose-only list status=%d body=%s, want 403", response.Code, response.Body.String())
	}
}

// An owner's single approval (accept_once) binds the computer's event heads
// and state commitments; without a public read of them only a host-side
// harness could approve. Failure modes pinned: the head is read with a
// write scope or for another computer; the route accepts writes; the read
// reaches anything but corpusd's event head for exactly this computer.
func TestSelfDevelopmentHeadIsAReadOfThisComputersEventHead(t *testing.T) {
	var calls int
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/internal/computers/events/head" || r.URL.Query().Get("computer_id") != "computer-a" {
			t.Fatalf("corpusd target = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "canonical_event_head": strings.Repeat("a", 64), "sequence": 7})
	}))
	defer corpusd.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	user, err := store.CreateUser("selfdev-head-user", "selfdev-head@example.com")
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID, "state": "active"})
	}))
	defer ownership.Close()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	call := func(method, computerID, scope string) *httptest.ResponseRecorder {
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "head-"+scope+method+computerID, []string{scope}, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(method, "/api/computers/"+computerID+"/self-development/head", nil)
		request.Header.Set("Authorization", "Bearer "+secret)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		return response
	}
	if response := call(http.MethodGet, "computer-a", "computer:self_development:read"); response.Code != http.StatusOK || calls != 1 || !strings.Contains(response.Body.String(), `"sequence":7`) {
		t.Fatalf("read head status=%d calls=%d body=%s", response.Code, calls, response.Body.String())
	}
	if response := call(http.MethodGet, "computer-a", "computer:self_development:propose"); response.Code != http.StatusForbidden || calls != 1 {
		t.Fatalf("propose-scope head status=%d calls=%d", response.Code, calls)
	}
	if response := call(http.MethodGet, "computer-b", "computer:self_development:read"); response.Code != http.StatusForbidden || calls != 1 {
		t.Fatalf("other-computer head status=%d calls=%d", response.Code, calls)
	}
	if response := call(http.MethodPut, "computer-a", "admin"); response.Code != http.StatusMethodNotAllowed || calls != 1 {
		t.Fatalf("head write status=%d calls=%d", response.Code, calls)
	}
}

// The source-patch export is a read of a frozen candidate. Failure modes
// pinned: the proxy classifier does not know the suffix and answers 404
// before the guest sees it; the read borrows a write scope (propose or
// approve) or accepts a write method; the upstream path is rewritten.
func TestSelfDevelopmentGuestServesSourcePatchWithReadScope(t *testing.T) {
	const path = "/api/computers/computer-a/self-development/operations/op-1/source-patch"
	if computerID, action, ok := selfDevelopmentGuestRoute(path); !ok || computerID != "computer-a" || action != "read" {
		t.Fatalf("classifier = %q %q %v, want computer-a read", computerID, action, ok)
	}
	var gotPath, gotMethod string
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"operation_id":"op-1","patch_base64":""}`))
	}))
	defer autoputer.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("selfdev-patch-user", "selfdev-patch@example.com")
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
	call := func(method, scope string) *httptest.ResponseRecorder {
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "patch-"+method+"-"+scope, []string{scope}, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(method, path, nil)
		request.Header.Set("Authorization", "Bearer "+secret)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		return response
	}
	if response := call(http.MethodGet, "computer:self_development:read"); response.Code != http.StatusOK || gotMethod != http.MethodGet || gotPath != path {
		t.Fatalf("read status=%d upstream=%s %s body=%s", response.Code, gotMethod, gotPath, response.Body.String())
	}
	gotPath = ""
	for _, scope := range []string{"computer:self_development:propose", "computer:self_development:approve"} {
		if response := call(http.MethodGet, scope); response.Code != http.StatusForbidden || gotPath != "" {
			t.Fatalf("%s status=%d upstream=%q, want 403 and no upstream call", scope, response.Code, gotPath)
		}
	}
	if response := call(http.MethodPost, "computer:self_development:read"); response.Code != http.StatusMethodNotAllowed || gotPath != "" {
		t.Fatalf("POST status=%d upstream=%q, want 405", response.Code, gotPath)
	}
}

// U4 slice 8 (review 328, DeepSeek F1): the owner's rebase retry reaches the
// guest through the proxy, POST only, as an owner act. Before the route was
// wired every /self-development/ path it did not know answered 409 "effects
// are disabled", so the act was unreachable by the owner.
func TestRebaseRetryRouteIsWiredThroughTheProxyAsAnOwnerAct(t *testing.T) {
	const path = "/api/computers/computer-a/self-development/rebase-retry"
	computerID, action, ok := selfDevelopmentGuestRoute(path)
	if !ok || computerID != "computer-a" || action != "rebase-retry" {
		t.Fatalf("route = %q %q %v; want computer-a rebase-retry", computerID, action, ok)
	}
	for method, want := range map[string]bool{http.MethodPost: true, http.MethodGet: false, http.MethodPut: false, http.MethodDelete: false} {
		if got := selfDevelopmentGuestMethodAllowed(method, action); got != want {
			t.Fatalf("%s allowed = %v, want %v", method, got, want)
		}
	}
	if !isSelfDevelopmentGuestPath(path) {
		t.Fatal("the dispatcher does not route the retry to the guest handler")
	}
	// Deeper or empty paths are not the route.
	for _, bad := range []string{
		"/api/computers/computer-a/self-development/rebase-retry/extra",
		"/api/computers//self-development/rebase-retry",
		"/api/computers/a/b/self-development/rebase-retry",
	} {
		if _, _, ok := selfDevelopmentGuestRoute(bad); ok {
			t.Fatalf("%s routed", bad)
		}
	}
}

func TestRebaseRetryThroughTheProxyForwardsOnlyTheOwnersPost(t *testing.T) {
	const path = "/api/computers/computer-a/self-development/rebase-retry"
	env := newOwnerActEnv(t)
	// Unauthenticated: nothing reaches the guest.
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	env.handler.HandleAPI(rec, req)
	if rec.Code != http.StatusUnauthorized || env.guestHits.Load() != 0 {
		t.Fatalf("unauthenticated = %d (guest hits %d)", rec.Code, env.guestHits.Load())
	}
	// GET is refused at the proxy.
	if rec := env.send(t, http.MethodGet, path, "", ""); rec.Code != http.StatusMethodNotAllowed || env.guestHits.Load() != 0 {
		t.Fatalf("GET = %d (guest hits %d)", rec.Code, env.guestHits.Load())
	}
	// An agent key with the approve scope, and an admin key, are refused as non-owners.
	for _, scopes := range [][]string{{"computer:self_development:approve"}, {"admin"}} {
		if rec := env.send(t, http.MethodPost, path, `{}`, env.key(t, scopes...)); rec.Code != http.StatusForbidden || env.guestHits.Load() != 0 {
			t.Fatalf("key %v = %d (guest hits %d)", scopes, rec.Code, env.guestHits.Load())
		}
	}
	// The owner's session reaches the guest, and its answer comes back.
	if rec := env.send(t, http.MethodPost, path, `{}`, ""); rec.Code != http.StatusOK || env.guestHits.Load() != 1 {
		t.Fatalf("owner POST = %d (guest hits %d) %s", rec.Code, env.guestHits.Load(), rec.Body.String())
	}
	// Another user's session does not own the computer.
	other, err := env.store.CreateUser("not-the-owner", "other@example.com")
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(env.privateKey, other.ID)})
	rec = httptest.NewRecorder()
	env.handler.HandleAPI(rec, req)
	if rec.Code != http.StatusForbidden || env.guestHits.Load() != 1 {
		t.Fatalf("another user = %d (guest hits %d)", rec.Code, env.guestHits.Load())
	}
}
