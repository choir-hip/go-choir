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

// The owner's `computer start` is the one act that clears vmctl's crash-loop
// hold: the proxy marks that resolve owner_start, and `status` shows the hold
// and its cause. A request wake never carries the mark (api_key_wake_test.go).
func TestComputerStartOfACrashHeldComputerIsTheOwnersStart(t *testing.T) {
	state, epoch, held := "stopped", int64(7), true
	var resolveBodies []map[string]any
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("user_id") != "crash-owner" {
				http.NotFound(w, r)
				return
			}
			reply := map[string]any{
				"computer_id": "computer-crash-owner", "user_id": "crash-owner", "desktop_id": "primary",
				"state": state, "epoch": epoch,
			}
			if held {
				reply["held"], reply["hold_reason"], reply["held_by"] = true, "crash loop: 3 crashes in 30m", vmctl.CrashLoopHeldBy
			}
			_ = json.NewEncoder(w).Encode(reply)
		case "/internal/vmctl/resolve":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			resolveBodies = append(resolveBodies, body)
			state, epoch, held = "active", epoch+1, false
			_ = json.NewEncoder(w).Encode(map[string]any{
				"computer_id": "computer-crash-owner", "user_id": "crash-owner", "desktop_id": "primary",
				"state": state, "epoch": epoch,
			})
		case "/internal/vmctl/touch":
			_, _ = w.Write([]byte(`{"touched":true}`))
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
			Receipt: &computerevent.Receipt{ReceiptKind: "LifecycleReceipt", ReceiptID: "crash-start"},
		})
	}))
	defer corpusd.Close()

	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	user, err := store.CreateUser("crash-owner", "crash-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "crash lifecycle", []string{"computer:lifecycle"}, "computer-crash-owner", nil)
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+secret)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		return response
	}

	status := do(http.MethodGet, "/api/computers/computer-crash-owner/lifecycle/status", "")
	if status.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", status.Code, status.Body.String())
	}
	var shown map[string]any
	if err := json.Unmarshal(status.Body.Bytes(), &shown); err != nil {
		t.Fatal(err)
	}
	if shown["held"] != true || shown["hold_reason"] != "crash loop: 3 crashes in 30m" || shown["state"] != "stopped" {
		t.Fatalf("status of a crash-held computer = %v, want held with the cause", shown)
	}

	response := do(http.MethodPost, "/api/computers/computer-crash-owner/lifecycle/start", `{"idempotency_key":"crash-start-1"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", response.Code, response.Body.String())
	}
	if len(resolveBodies) != 1 || resolveBodies[0]["owner_start"] != true {
		t.Fatalf("start resolved with %v, want one resolve marked owner_start", resolveBodies)
	}
}
