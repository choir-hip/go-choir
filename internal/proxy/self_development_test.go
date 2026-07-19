package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
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
		VerifierRef:              "verifier-ref",
		ExpectedDesiredEventHead: digest('b'), ExpectedEffectiveEventHead: digest('c'),
		ExpectedPendingTransitionRef:   &pending,
		ExpectedDesiredStateCommitment: digest('d'), ExpectedEffectiveStateCommitment: digest('e'),
	}
	consumptionKey, err := consumedModeIdempotency("operation-1", decision)
	if err != nil {
		t.Fatal(err)
	}
	receipt := &computerevent.Receipt{ReceiptKind: "ModeReceipt", KindFields: map[string]any{
		"old_mode": "accept_once", "new_mode": "propose_only", "consumed_operation_id": "operation-1",
		"consumed_bundle_digest":              decision.BundleDigest,
		"consumed_desired_event_head":         decision.ExpectedDesiredEventHead,
		"consumed_effective_event_head":       decision.ExpectedEffectiveEventHead,
		"consumed_pending_transition_ref":     pending,
		"consumed_desired_state_commitment":   decision.ExpectedDesiredStateCommitment,
		"consumed_effective_state_commitment": decision.ExpectedEffectiveStateCommitment,
		"idempotency_key":                     consumptionKey,
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
		VerifierRef:              "verifier-ref",
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
		wantKey, keyErr := consumedModeIdempotency("operation-1", decision)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
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

func TestPublicTerminalDecisionReplayPrecedesLaterModeAuthority(t *testing.T) {
	digest := func(fill byte) string { return strings.Repeat(string(fill), 64) }
	pending := ""
	decision := proxiedSelfDevelopmentDecision{
		Decision: "reject", IdempotencyKey: "decision-replay", BundleDigest: digest('a'), VerifierRef: digest('b'), Reason: "owner rejected",
		ExpectedDesiredEventHead: digest('c'), ExpectedEffectiveEventHead: digest('d'), ExpectedPendingTransitionRef: &pending,
		ExpectedDesiredStateCommitment: digest('e'), ExpectedEffectiveStateCommitment: digest('f'),
	}
	decisionBytes, err := computerevent.CanonicalJSON(decision)
	if err != nil {
		t.Fatal(err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	event := computerevent.Event{
		SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: "computer-replay",
		Sequence: 1, PreviousHead: computerevent.ZeroHead, EventKind: computerevent.EventEffectRejected,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: "event-replay", RequestCommitment: computerevent.ZeroHead,
		TrajectoryID: "trajectory-replay", CapsuleID: "capsule-replay", ParentEventID: "operation-replay",
		ActorProfile: "super", AuthorityRef: "external-owner:owner", PrivacyClass: "owner",
		ExpectedDesiredEventHead: decision.ExpectedDesiredEventHead, ExpectedEffectiveEventHead: decision.ExpectedEffectiveEventHead,
		ExpectedDesiredStateCommitment: decision.ExpectedDesiredStateCommitment, ExpectedEffectiveStateCommitment: decision.ExpectedEffectiveStateCommitment,
		RequireExpectedHead: true, PayloadCommitment: digest('1'), ProposedEffectRef: decision.BundleDigest,
		DecisionRef: computerevent.DigestBytes(decisionBytes), VerifierRefs: []string{decision.VerifierRef}, ReducerVersion: computerevent.ReducerVersionV1,
	}
	eventDigest, err := event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	operation := selfdev.Operation{
		OperationID: "operation-replay", ComputerID: "computer-replay", BundleDigest: decision.BundleDigest,
		VerifierRefs: []string{decision.VerifierRef}, DecisionEvent: eventDigest, State: selfdev.StateRejected,
	}
	var decisionPosts int
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-Computer") != "computer-replay" {
			t.Fatal("guest request lost computer binding")
		}
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/operations/operation-replay"):
			_ = json.NewEncoder(w).Encode(operation)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/events/"+eventDigest):
			_ = json.NewEncoder(w).Encode(event)
		case r.Method == http.MethodPost:
			decisionPosts++
			http.Error(w, "unexpected decision mutation", http.StatusConflict)
		default:
			http.NotFound(w, r)
		}
	}))
	defer guest.Close()
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-replay", "desktop_id": "primary", "user_id": "owner-replay",
			"state": "active", "sandbox_url": guest.URL,
		})
	}))
	defer ownership.Close()
	var modeReads int
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		modeReads++
		http.Error(w, "later mode unavailable", http.StatusServiceUnavailable)
	}))
	defer corpusd.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	handler.cfg.CorpusdURL = corpusd.URL
	user, err := store.CreateUser("owner-replay", "owner-replay@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "selfdev-replay", []string{"computer:self_development:approve"}, "computer-replay", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(decision)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-replay/self-development/operations/operation-replay/decision", strings.NewReader(string(body)))
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK || modeReads != 0 || decisionPosts != 0 {
		t.Fatalf("terminal replay status=%d mode_reads=%d decision_posts=%d body=%s", response.Code, modeReads, decisionPosts, response.Body.String())
	}
	var replayed selfdev.Operation
	if json.Unmarshal(response.Body.Bytes(), &replayed) != nil || replayed.DecisionEvent != eventDigest {
		t.Fatalf("terminal replay operation = %+v", replayed)
	}
	changed := decision
	changed.Reason = "different rejection"
	changedBody, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	changedRequest := httptest.NewRequest(http.MethodPost, "/api/computers/computer-replay/self-development/operations/operation-replay/decision", strings.NewReader(string(changedBody)))
	changedRequest.Header.Set("Authorization", "Bearer "+secret)
	changedResponse := httptest.NewRecorder()
	handler.HandleAPI(changedResponse, changedRequest)
	if changedResponse.Code != http.StatusConflict || modeReads != 0 || decisionPosts != 0 {
		t.Fatalf("changed terminal replay status=%d mode_reads=%d decision_posts=%d body=%s", changedResponse.Code, modeReads, decisionPosts, changedResponse.Body.String())
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
