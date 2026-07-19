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

func TestConsumedModeReceiptAuthorizesOnlyExactCrashedDecisionRetry(t *testing.T) {
	digest := func(fill byte) string { return strings.Repeat(string(fill), 64) }
	pending := ""
	decision := proxiedSelfDevelopmentDecision{
		Decision: "approve", IdempotencyKey: "decision-1", BundleDigest: digest('a'),
		ExpectedDesiredEventHead: digest('b'), ExpectedEffectiveEventHead: digest('c'),
		ExpectedPendingTransitionRef:   &pending,
		ExpectedDesiredStateCommitment: digest('d'), ExpectedEffectiveStateCommitment: digest('e'),
	}
	receipt := &computerevent.Receipt{ReceiptKind: "ModeReceipt", KindFields: map[string]any{
		"old_mode": "accept_once", "new_mode": "propose_only", "consumed_operation_id": "operation-1",
		"consumed_bundle_digest":              decision.BundleDigest,
		"consumed_desired_event_head":         decision.ExpectedDesiredEventHead,
		"consumed_effective_event_head":       decision.ExpectedEffectiveEventHead,
		"consumed_pending_transition_ref":     pending,
		"consumed_desired_state_commitment":   decision.ExpectedDesiredStateCommitment,
		"consumed_effective_state_commitment": decision.ExpectedEffectiveStateCommitment,
		"idempotency_key":                     "accept-once-consumed:operation-1:7:decision-1",
	}}
	if !consumedModeReceiptMatches(receipt, "operation-1", decision) {
		t.Fatal("exact consumed decision receipt was refused")
	}
	changed := decision
	changed.ExpectedEffectiveEventHead = digest('f')
	if consumedModeReceiptMatches(receipt, "operation-1", changed) {
		t.Fatal("consumed receipt authorized a changed decision")
	}
}

func TestConsumeAcceptOncePostsDeterministicIdempotencyAndReturnsExactReceipt(t *testing.T) {
	digest := func(fill byte) string { return strings.Repeat(string(fill), 64) }
	pending := ""
	decision := proxiedSelfDevelopmentDecision{
		Decision: "approve", IdempotencyKey: "decision-1", BundleDigest: digest('a'),
		ExpectedDesiredEventHead: digest('b'), ExpectedEffectiveEventHead: digest('c'),
		ExpectedPendingTransitionRef:   &pending,
		ExpectedDesiredStateCommitment: digest('d'), ExpectedEffectiveStateCommitment: digest('e'),
	}
	mode := selfDevelopmentModeProjection{
		ComputerID: "computer-1", Mode: "accept_once", Generation: 7, OperationID: "operation-1",
		BundleDigest: decision.BundleDigest, ExpectedDesiredEventHead: decision.ExpectedDesiredEventHead,
		ExpectedEffectiveEventHead:       decision.ExpectedEffectiveEventHead,
		ExpectedPendingTransitionRef:     pending,
		ExpectedDesiredStateCommitment:   decision.ExpectedDesiredStateCommitment,
		ExpectedEffectiveStateCommitment: decision.ExpectedEffectiveStateCommitment,
	}
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Fatal("invalid mode consumption request")
		}
		wantKey := consumedModeIdempotency("operation-1", 7, "decision-1")
		if request["mode"] != "propose_only" || request["idempotency_key"] != wantKey || request["expected_generation"] != float64(7) {
			t.Fatalf("mode consumption request = %#v", request)
		}
		receipt := &computerevent.Receipt{ReceiptKind: "ModeReceipt", KindFields: map[string]any{
			"old_mode": "accept_once", "new_mode": "propose_only", "consumed_operation_id": "operation-1",
			"consumed_bundle_digest": decision.BundleDigest, "consumed_desired_event_head": decision.ExpectedDesiredEventHead,
			"consumed_effective_event_head":       decision.ExpectedEffectiveEventHead,
			"consumed_pending_transition_ref":     pending,
			"consumed_desired_state_commitment":   decision.ExpectedDesiredStateCommitment,
			"consumed_effective_state_commitment": decision.ExpectedEffectiveStateCommitment,
			"idempotency_key":                     wantKey,
		}}
		_ = json.NewEncoder(w).Encode(selfDevelopmentModeProjection{ComputerID: "computer-1", Mode: "propose_only", Generation: 8, Receipt: receipt})
	}))
	defer corpusd.Close()
	handler := &Handler{cfg: &Config{CorpusdURL: corpusd.URL}, corpusd: corpusd.Client()}
	receipt, err := handler.consumeSelfDevelopmentMode(context.Background(), "computer-1", "owner-1", mode, decision)
	if err != nil || !consumedModeReceiptMatches(&receipt, "operation-1", decision) {
		t.Fatalf("consumed receipt = %+v err=%v", receipt, err)
	}
}
