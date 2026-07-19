package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

func TestKernelCapabilityReceiptRequiresReadScopeRoute(t *testing.T) {
	target, ok := parseSelfDevelopmentTarget("/api/computers/computer-a/self-development/kernel-capabilities", http.MethodGet)
	if !ok || target.ComputerID != "computer-a" || target.RequiredScope != "computer:self_development:read" {
		t.Fatalf("target = %+v accepted=%v", target, ok)
	}
	if _, ok := parseSelfDevelopmentTarget("/api/computers/computer-a/self-development/kernel-capabilities", http.MethodPost); ok {
		t.Fatal("kernel capability mutation route was accepted")
	}
}

func TestPublicDecisionDelegatesModeAuthorityToGuest(t *testing.T) {
	var modeReads, decisionPosts int
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-Computer") != "computer-decision" ||
			r.Header.Get("X-Authenticated-User") != "owner-decision" {
			t.Fatal("guest request lost authenticated binding")
		}
		var decision map[string]any
		if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&decision) != nil ||
			decision["idempotency_key"] != "decision-replay" {
			t.Fatalf("invalid guest decision request: %#v", decision)
		}
		decisionPosts++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"operation_id":"operation-decision","state":"rejected"}`))
	}))
	defer guest.Close()
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-decision", "desktop_id": "primary", "user_id": "owner-decision",
			"state": "active", "sandbox_url": guest.URL,
		})
	}))
	defer ownership.Close()
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modeReads++
		http.Error(w, "proxy must not own decision mode state", http.StatusServiceUnavailable)
	}))
	defer corpusd.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	handler.cfg.CorpusdURL = corpusd.URL
	user, err := store.CreateUser("owner-decision", "owner-decision@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "selfdev-decision", []string{"computer:self_development:approve"}, "computer-decision", nil)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"decision":"reject","idempotency_key":"decision-replay"}`
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-decision/self-development/operations/operation-decision/decision", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK || modeReads != 0 || decisionPosts != 1 {
		t.Fatalf("decision status=%d mode_reads=%d decision_posts=%d body=%s", response.Code, modeReads, decisionPosts, response.Body.String())
	}
}

func TestPublicProposalInjectsCurrentModeReceiptForGuestAuthority(t *testing.T) {
	receipt := &computerevent.Receipt{ReceiptKind: "ModeReceipt", ReceiptID: "receipt-mode-propose"}
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(selfDevelopmentModeProjection{
			ComputerID: "computer-propose", Mode: "propose_only", Generation: 4, Receipt: receipt,
		})
	}))
	defer corpusd.Close()
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var start proxiedSelfDevelopmentStart
		if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&start) != nil {
			t.Fatal("invalid guest proposal request")
		}
		if start.IdempotencyKey != "proposal-mode" || start.Prompt != "change runtime" ||
			start.ModeReceipt == nil || start.ModeReceipt.ReceiptID != receipt.ReceiptID {
			t.Fatalf("guest proposal binding = %+v", start)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"operation_id":"operation-propose"}`))
	}))
	defer guest.Close()
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-propose", "desktop_id": "primary", "user_id": "owner-propose",
			"state": "active", "sandbox_url": guest.URL,
		})
	}))
	defer ownership.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	user, err := store.CreateUser("owner-propose", "owner-propose@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "selfdev-propose", []string{"computer:self_development:propose"}, "computer-propose", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-propose/self-development/operations", strings.NewReader(`{"idempotency_key":"proposal-mode","prompt":"change runtime"}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("proposal status=%d body=%s", response.Code, response.Body.String())
	}
}
