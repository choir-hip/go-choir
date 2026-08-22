package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestColdRecoverCurrentReturnsRecoveryStatusAndRejectsCheckpoint(t *testing.T) {
	closure, err := computerversion.NewCodeClosure(strings.Repeat("1", 40), []computerversion.CodeArtifact{{
		Name: "autoputer", SHA256: strings.Repeat("a", 64), URI: "nix-store+sha256://" + strings.Repeat("a", 64) + "/nix/store/test-autoputer",
	}}, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "autoputer", ContentSHA256: strings.Repeat("b", 64), ArtifactURI: "artifact+sha256://" + strings.Repeat("b", 64) + "/autoputer",
	}}, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	canonicalHead := strings.Repeat("c", 64)
	coldCalls := 0

	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("user_id") != "owner" || r.URL.Query().Get("computer_id") != "computer-a" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "user_id": "owner", "desktop_id": "primary", "state": "failed"})
		case "/internal/vmctl/computer-version-routes/resolve":
			slotID := r.URL.Query().Get("route_slot_id")
			_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{
				Slot:          routeledger.Slot{ID: slotID, Current: version, Generation: 3, LatestReceiptID: "receipt-1"},
				LatestReceipt: routeledger.TransitionReceipt{ID: "receipt-1", RouteSlotID: slotID, New: version, CommittedGeneration: 3},
				CodeClosure:   closure, ArtifactProgram: program,
			})
		case "/internal/vmctl/computers/computer-a/cold-recover":
			coldCalls++
			var request vmctl.ColdRecoverRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.ComputerID != "computer-a" || request.ExpectedCanonicalHead != canonicalHead || request.ExpectedRouteGeneration != 3 ||
				(request.IdempotencyKey != "recover-1" && request.IdempotencyKey != "owner-cold-recover:computer-a:0") {
				t.Fatalf("cold recovery request = %#v", request)
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(vmctl.ColdRecoverResponse{Status: "rematerializing", QuarantinePath: "/evidence/quarantine", FencingToken: vmctl.RecoveryFencingToken{ComputerID: "computer-a"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer vm.Close()
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/events/head" || r.Header.Get("X-Internal-Caller") != "true" || r.Header.Get("X-Authenticated-User") != "owner" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(computerevent.Head{ComputerID: "computer-a", CanonicalEventHead: canonicalHead})
	}))
	defer corpusd.Close()

	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()

	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/cold-recover", strings.NewReader(`{"idempotency_key":"recover-1"}`))
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner")})
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), `"status":"rematerializing"`) || coldCalls != 1 {
		t.Fatalf("cold recovery status=%d calls=%d body=%s", response.Code, coldCalls, response.Body.String())
	}

	fallback := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/rematerialize-from-tape", nil)
	fallback.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner")})
	fallbackResponse := httptest.NewRecorder()
	handler.HandleAPI(fallbackResponse, fallback)
	if fallbackResponse.Code != http.StatusAccepted || coldCalls != 2 {
		t.Fatalf("inactive rematerialize fallback status=%d calls=%d body=%s", fallbackResponse.Code, coldCalls, fallbackResponse.Body.String())
	}

	rejected := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/cold-recover", strings.NewReader(`{"idempotency_key":"recover-2","checkpoint_digest":"forbidden"}`))
	rejected.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner")})
	rejectedResponse := httptest.NewRecorder()
	handler.HandleAPI(rejectedResponse, rejected)
	if rejectedResponse.Code != http.StatusBadRequest || coldCalls != 2 {
		t.Fatalf("checkpoint rejection status=%d calls=%d body=%s", rejectedResponse.Code, coldCalls, rejectedResponse.Body.String())
	}

	foreign := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/lifecycle/cold-recover", strings.NewReader(`{"idempotency_key":"recover-3"}`))
	foreign.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "other-owner")})
	foreignResponse := httptest.NewRecorder()
	handler.HandleAPI(foreignResponse, foreign)
	if foreignResponse.Code != http.StatusForbidden || coldCalls != 2 {
		t.Fatalf("foreign owner status=%d calls=%d body=%s", foreignResponse.Code, coldCalls, foreignResponse.Body.String())
	}
}
