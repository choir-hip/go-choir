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
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestComputerRestartPreservesOrdinaryUserStopResolveSemantics(t *testing.T) {
	state, epoch, stops, resolves := "active", int64(7), 0, 0
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("user_id") != "ordinary-owner" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-ordinary", "user_id": "ordinary-owner", "desktop_id": "primary",
				"state": state, "epoch": epoch,
			})
		case "/internal/vmctl/stop":
			stops++
			state = "stopped"
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		case "/internal/vmctl/resolve":
			resolves++
			state, epoch = "active", epoch+1
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-ordinary", "user_id": "ordinary-owner", "desktop_id": "primary",
				"state": state, "epoch": epoch,
			})
		default:
			t.Fatalf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	defer vm.Close()
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request platform.LifecycleControlRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Phase == "prepare" {
			_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{
				Status: "pending", Action: request.Action, PriorState: request.PriorState, PriorEpoch: request.PriorEpoch,
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{
			Status: "completed", Action: request.Action,
			Receipt: &computerevent.Receipt{ReceiptKind: "LifecycleReceipt", ReceiptID: "ordinary-restart"},
		})
	}))
	defer corpusd.Close()

	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	user, err := store.CreateUser("ordinary-owner", "ordinary-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "ordinary lifecycle", []string{"computer:lifecycle"}, "computer-ordinary", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-ordinary/lifecycle/restart", strings.NewReader(`{"idempotency_key":"ordinary-restart-1"}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if stops != 1 || resolves != 1 || epoch != 8 {
		t.Fatalf("ordinary restart used stops=%d resolves=%d epoch=%d", stops, resolves, epoch)
	}
}

func TestParseComputerLifecyclePathAcceptsRefresh(t *testing.T) {
	computerID, action, ok := parseComputerLifecyclePath("/api/computers/computer-retained/lifecycle/refresh")
	if !ok || computerID != "computer-retained" || action != "refresh" {
		t.Fatalf("parse refresh = %q %q %v", computerID, action, ok)
	}
	if _, _, ok := parseComputerLifecyclePath("/api/computers/computer-retained/lifecycle/rematerialize-from-tape"); ok {
		t.Fatal("rematerialize must not parse as VM lifecycle control")
	}
}

func TestComputerRefreshUsesVmctlRefreshNotResolve(t *testing.T) {
	state, epoch, stops, resolves, refreshes := "active", int64(268), 0, 0, 0
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("user_id") != "ordinary-owner" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-ordinary", "user_id": "ordinary-owner", "desktop_id": "primary",
				"state": state, "epoch": epoch,
			})
		case "/internal/vmctl/refresh":
			refreshes++
			state, epoch = "active", epoch+1
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-ordinary", "user_id": "ordinary-owner", "desktop_id": "primary",
				"state": state, "epoch": epoch,
			})
		case "/internal/vmctl/stop", "/internal/vmctl/resolve", "/internal/vmctl/rematerialize":
			t.Fatalf("refresh used unexpected vmctl path %s", r.URL.Path)
		default:
			t.Fatalf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	defer vm.Close()
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request platform.LifecycleControlRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Action != "refresh" {
			t.Fatalf("lifecycle action = %q, want refresh", request.Action)
		}
		if request.Phase == "prepare" {
			_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{
				Status: "pending", Action: request.Action, PriorState: request.PriorState, PriorEpoch: request.PriorEpoch,
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{
			Status: "completed", Action: request.Action,
			Receipt: &computerevent.Receipt{ReceiptKind: "LifecycleReceipt", ReceiptID: "ordinary-refresh"},
		})
	}))
	defer corpusd.Close()

	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	user, err := store.CreateUser("ordinary-owner", "ordinary-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "ordinary lifecycle", []string{"computer:lifecycle"}, "computer-ordinary", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-ordinary/lifecycle/refresh", strings.NewReader(`{"idempotency_key":"ordinary-refresh-1"}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if stops != 0 || resolves != 0 || refreshes != 1 || epoch != 269 {
		t.Fatalf("owner refresh used stops=%d resolves=%d refreshes=%d epoch=%d", stops, resolves, refreshes, epoch)
	}
}

func TestComputerRecoverUnholdsStoppedComputerAndStarts(t *testing.T) {
	state, epoch, unholds, resolves := "stopped", int64(7), 0, 0
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("user_id") != "ordinary-owner" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-ordinary", "user_id": "ordinary-owner", "desktop_id": "primary",
				"state": state, "epoch": epoch,
			})
		case "/internal/vmctl/unhold":
			unholds++
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unheld", "computer_id": "computer-ordinary"})
		case "/internal/vmctl/resolve":
			resolves++
			state, epoch = "active", epoch+1
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-ordinary", "user_id": "ordinary-owner", "desktop_id": "primary",
				"state": state, "epoch": epoch,
			})
		default:
			t.Fatalf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	defer vm.Close()
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request platform.LifecycleControlRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Action != "recover" {
			t.Fatalf("lifecycle action = %q, want recover", request.Action)
		}
		if request.Phase == "prepare" {
			_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{
				Status: "pending", Action: request.Action, PriorState: request.PriorState, PriorEpoch: request.PriorEpoch,
			})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{
			Status: "completed", Action: request.Action,
			Receipt: &computerevent.Receipt{ReceiptKind: "LifecycleReceipt", ReceiptID: "ordinary-recover"},
		})
	}))
	defer corpusd.Close()

	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	user, err := store.CreateUser("ordinary-owner", "ordinary-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "ordinary lifecycle", []string{"computer:lifecycle"}, "computer-ordinary", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-ordinary/lifecycle/recover", strings.NewReader(`{"idempotency_key":"ordinary-recover-1"}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if unholds != 1 || resolves != 1 || state != "active" || epoch != 8 {
		t.Fatalf("recover used unholds=%d resolves=%d state=%s epoch=%d", unholds, resolves, state, epoch)
	}
}

func TestColdRecoverCurrentRequiresOwnerAndRejectsCheckpointPassthrough(t *testing.T) {
	createdAt := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	closure, err := computerversion.NewCodeClosure(strings.Repeat("1", 40), []computerversion.CodeArtifact{{
		Name: "autoputer", SHA256: strings.Repeat("a", 64), URI: "nix-store+sha256://" + strings.Repeat("a", 64) + "/nix/store/test-autoputer",
	}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "autoputer", ContentSHA256: strings.Repeat("b", 64), ArtifactURI: "artifact+sha256://" + strings.Repeat("b", 64) + "/autoputer",
	}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	canonicalHead := strings.Repeat("c", 64)
	coldRecoverCalls := 0
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("user_id") != "owner-user" || r.URL.Query().Get("computer_id") != "computer-a" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-a", "user_id": "owner-user", "desktop_id": "primary", "vm_id": "vm-a", "state": "failed",
			})
		case "/internal/vmctl/computer-version-routes/resolve":
			slotID := r.URL.Query().Get("route_slot_id")
			_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{
				Slot:          routeledger.Slot{ID: slotID, Current: version, Generation: 7, LatestReceiptID: "receipt-1"},
				LatestReceipt: routeledger.TransitionReceipt{ID: "receipt-1", RouteSlotID: slotID, New: version, CommittedGeneration: 7},
				CodeClosure:   closure, ArtifactProgram: program,
			})
		case "/internal/vmctl/computers/computer-a/cold-recover":
			coldRecoverCalls++
			var request vmctl.ColdRecoverRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.ComputerID != "computer-a" || request.ExpectedCanonicalHead != canonicalHead || request.ExpectedRouteGeneration != 7 || (request.IdempotencyKey != "bios-cold-recover-computer-a-1" && request.IdempotencyKey != "owner-cold-recover:computer-a:0") {
				t.Fatalf("cold recovery request = %#v", request)
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(vmctl.ColdRecoverResponse{
				Status: "rematerializing", CanonicalHead: canonicalHead, QuarantinePath: "/evidence/data.img.quarantine", FencingToken: vmctl.RecoveryFencingToken{ComputerID: "computer-a"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer vmctlServer.Close()
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/events/head" || r.URL.Query().Get("computer_id") != "computer-a" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(computerevent.Head{ComputerID: "computer-a", CanonicalEventHead: canonicalHead})
	}))
	defer corpusd.Close()
	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/cold-recover", strings.NewReader(`{"idempotency_key":"bios-cold-recover-computer-a-1"}`))
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("cold recovery status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Recovery struct {
			Status string `json:"status"`
		} `json:"recovery"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil || body.Recovery.Status != "rematerializing" {
		t.Fatalf("cold recovery response=%s decode=%v", response.Body.String(), err)
	}
	fallback := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/rematerialize-from-tape", nil)
	fallback.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	fallbackResponse := httptest.NewRecorder()
	handler.HandleAPI(fallbackResponse, fallback)
	if fallbackResponse.Code != http.StatusAccepted || coldRecoverCalls != 2 {
		t.Fatalf("inactive rematerialize fallback status=%d cold calls=%d body=%s", fallbackResponse.Code, coldRecoverCalls, fallbackResponse.Body.String())
	}
	rejected := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/cold-recover", strings.NewReader(`{"idempotency_key":"x","checkpoint_digest":"forbidden"}`))
	rejected.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	rejectedResponse := httptest.NewRecorder()
	handler.HandleAPI(rejectedResponse, rejected)
	if rejectedResponse.Code != http.StatusBadRequest || coldRecoverCalls != 2 {
		t.Fatalf("checkpoint rejection status=%d cold calls=%d body=%s", rejectedResponse.Code, coldRecoverCalls, rejectedResponse.Body.String())
	}
	unauthorized := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/cold-recover", strings.NewReader(`{"idempotency_key":"x"}`))
	unauthorized.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "attacker-user")})
	unauthorizedResponse := httptest.NewRecorder()
	handler.HandleAPI(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusForbidden || coldRecoverCalls != 2 {
		t.Fatalf("unauthorized status=%d cold calls=%d body=%s", unauthorizedResponse.Code, coldRecoverCalls, unauthorizedResponse.Body.String())
	}
}
