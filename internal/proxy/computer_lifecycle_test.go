package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestComputerRestartPersistsIntentAndDoesNotReactuateIdempotentRetry(t *testing.T) {
	state, epoch, refreshes := "active", int64(7), 0
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-1", "user_id": "lifecycle-owner", "desktop_id": "primary", "state": state, "epoch": epoch})
		case "/internal/vmctl/refresh":
			refreshes++
			state, epoch = "active", epoch+1
			_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-1", "user_id": "lifecycle-owner", "desktop_id": "primary", "state": state, "epoch": epoch})
		default:
			t.Fatalf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	defer vm.Close()

	var pending *platform.LifecycleControlRequest
	receipt := &computerevent.Receipt{ReceiptKind: "LifecycleReceipt", ReceiptID: "lifecycle-restart-1"}
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request platform.LifecycleControlRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		switch request.Phase {
		case "prepare":
			if pending == nil {
				copy := request
				pending = &copy
				_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{Status: "pending", Action: request.Action, PriorState: request.PriorState, PriorEpoch: request.PriorEpoch})
				return
			}
			_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{Status: "completed", Action: request.Action, Receipt: receipt})
		case "complete":
			if pending == nil || request.RequestCommitment != pending.RequestCommitment || request.ResultingEpoch <= request.PriorEpoch {
				t.Fatalf("completion did not bind durable intent: %+v", request)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{Status: "completed", Action: request.Action, Receipt: receipt})
		default:
			t.Fatalf("unexpected phase %q", request.Phase)
		}
	}))
	defer corpusd.Close()

	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	user, err := store.CreateUser("lifecycle-owner", "lifecycle-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "lifecycle", []string{"computer:lifecycle"}, "computer-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-1/lifecycle/restart", strings.NewReader(`{"idempotency_key":"restart-1"}`))
		request.Header.Set("Authorization", "Bearer "+secret)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		if response.Code != http.StatusCreated && response.Code != http.StatusOK {
			t.Fatalf("attempt %d status=%d body=%s", attempt+1, response.Code, response.Body.String())
		}
	}
	if refreshes != 1 || epoch != 8 {
		t.Fatalf("restart retry reactuated: refreshes=%d epoch=%d", refreshes, epoch)
	}
}

func TestComputerStartResolvesConfiguredDisposableByExactComputerIDAndPreservesOwnership(t *testing.T) {
	const computerID = "computer-platform"
	state, epoch, refreshes, humanUserID := "failed", int64(8), 0, ""
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			got, lookupUser := r.URL.Query().Get("computer_id"), r.URL.Query().Get("user_id")
			if got == computerID && lookupUser == "" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"vm_id": "vm-retained", "computer_id": computerID, "user_id": "universal-wire-platform",
					"desktop_id": "platform", "sandbox_url": "http://guest", "state": state, "epoch": epoch,
				})
				return
			}
			if lookupUser == humanUserID && (got == computerID || got == "computer-other") {
				http.NotFound(w, r)
				return
			}
			t.Fatalf("unexpected lookup query computer=%q user=%q", got, lookupUser)
		case "/internal/vmctl/refresh":
			var request map[string]string
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request["user_id"] != "universal-wire-platform" || request["desktop_id"] != "platform" {
				t.Fatalf("refresh target = %#v", request)
			}
			refreshes++
			state, epoch = "active", epoch+1
			_ = json.NewEncoder(w).Encode(map[string]any{
				"vm_id": "vm-retained", "computer_id": computerID, "user_id": request["user_id"],
				"desktop_id": request["desktop_id"], "sandbox_url": "http://guest", "state": state, "epoch": epoch,
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
		_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{
			Status: "completed", Action: request.Action,
			Receipt: &computerevent.Receipt{ReceiptKind: "LifecycleReceipt", ReceiptID: "platform-start"},
		})
	}))
	defer corpusd.Close()

	handler, privateKey, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.cfg.SelfDevelopmentDisposableComputerID = computerID
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	user, err := store.CreateUser("human-owner", "human-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	humanUserID = user.ID
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "platform lifecycle", []string{"computer:lifecycle"}, computerID, nil)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/lifecycle/start", strings.NewReader(`{"idempotency_key":"platform-start-1"}`))
	request.Header.Set("Authorization", "Bearer "+secret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if refreshes != 1 || state != "active" || epoch != 9 {
		t.Fatalf("refreshes=%d state=%s epoch=%d", refreshes, state, epoch)
	}

	wrongTarget := httptest.NewRequest(http.MethodGet, "/api/computers/computer-other/lifecycle/status", nil)
	wrongTarget.Header.Set("Authorization", "Bearer "+secret)
	wrongTargetResponse := httptest.NewRecorder()
	handler.HandleAPI(wrongTargetResponse, wrongTarget)
	if wrongTargetResponse.Code != http.StatusForbidden {
		t.Fatalf("wrong-target status=%d body=%s", wrongTargetResponse.Code, wrongTargetResponse.Body.String())
	}

	_, otherSecret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "ordinary lifecycle", []string{"computer:lifecycle"}, "computer-other", nil)
	if err != nil {
		t.Fatal(err)
	}
	ordinaryTarget := httptest.NewRequest(http.MethodGet, "/api/computers/computer-other/lifecycle/status", nil)
	ordinaryTarget.Header.Set("Authorization", "Bearer "+otherSecret)
	ordinaryTargetResponse := httptest.NewRecorder()
	handler.HandleAPI(ordinaryTargetResponse, ordinaryTarget)
	if ordinaryTargetResponse.Code != http.StatusNotFound {
		t.Fatalf("non-disposable status=%d body=%s", ordinaryTargetResponse.Code, ordinaryTargetResponse.Body.String())
	}

	cookieTarget := httptest.NewRequest(http.MethodGet, "/api/computers/"+computerID+"/lifecycle/status", nil)
	cookieTarget.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, user.ID)})
	cookieTargetResponse := httptest.NewRecorder()
	handler.HandleAPI(cookieTargetResponse, cookieTarget)
	if cookieTargetResponse.Code != http.StatusNotFound {
		t.Fatalf("cookie target status=%d body=%s", cookieTargetResponse.Code, cookieTargetResponse.Body.String())
	}
}
