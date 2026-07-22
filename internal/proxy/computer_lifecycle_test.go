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
