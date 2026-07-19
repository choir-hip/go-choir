package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

	writeRequest := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/mode", nil)
	writeRequest.Header.Set("Authorization", "Bearer "+secret)
	writeResponse := httptest.NewRecorder()
	handler.HandleAPI(writeResponse, writeRequest)
	if writeResponse.Code != http.StatusForbidden || calls != 1 {
		t.Fatalf("read-only key write response=%d calls=%d", writeResponse.Code, calls)
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
	decision := proxiedSelfDevelopmentDecision{
		Decision: "approve", IdempotencyKey: "decision-1", BundleDigest: digest('a'),
		ExpectedDesiredEventHead: digest('b'), ExpectedEffectiveEventHead: digest('c'),
		ExpectedDesiredStateCommitment: digest('d'), ExpectedEffectiveStateCommitment: digest('e'),
	}
	receipt := &computerevent.Receipt{ReceiptKind: "ModeReceipt", KindFields: map[string]any{
		"old_mode": "accept_once", "new_mode": "off", "consumed_operation_id": "operation-1",
		"consumed_bundle_digest":              decision.BundleDigest,
		"consumed_desired_event_head":         decision.ExpectedDesiredEventHead,
		"consumed_effective_event_head":       decision.ExpectedEffectiveEventHead,
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
