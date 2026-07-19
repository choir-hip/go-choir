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
	state, epoch, stops, starts := "active", int64(7), 0, 0
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-1", "desktop_id": "primary", "state": state, "epoch": epoch})
		case "/internal/vmctl/stop":
			stops++
			state = "stopped"
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/internal/vmctl/resolve":
			starts++
			state, epoch = "active", epoch+1
			_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-1", "desktop_id": "primary", "state": state, "epoch": epoch})
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
	if stops != 1 || starts != 1 || epoch != 8 {
		t.Fatalf("restart retry reactuated: stops=%d starts=%d epoch=%d", stops, starts, epoch)
	}
}
