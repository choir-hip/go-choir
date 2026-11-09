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

// The owner update at the proxy (docs/design/update-now-2026-11-09.md items
// 5-9, failure modes 2, 4, 8, 9). Ways this could fail:
//   - the route goes to /internal/vmctl/refresh (a markerless crash boot)
//     instead of the deploy-refresh path;
//   - an agent key (computer:lifecycle, admin) or another user restarts the
//     owner's computer;
//   - current, deferred or skipped answer 201 or write a receipt;
//   - the guest's structured refusal is flattened;
//   - the update action is not parsed / registered.

type updateFixture struct {
	handler        *Handler
	secretFor      func(scopes ...string) string
	deployRefresh  *[]map[string]any
	legacyRefresh  *int
	completes      *int
	outcome        *vmctl.OwnerUpdateResult
	outcomeStatus  *int
	signal         *vmctl.UpdateAvailableResponse
	otherUserToken string
	ownerToken     string
}

func newUpdateFixture(t *testing.T) *updateFixture {
	t.Helper()
	state, epoch := "active", int64(40)
	var deployCalls []map[string]any
	legacy, completes := 0, 0
	outcome := &vmctl.OwnerUpdateResult{Outcome: "refreshed", Target: "commit-new"}
	status := http.StatusOK
	signal := &vmctl.UpdateAvailableResponse{}
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			if r.URL.Query().Get("user_id") != "update-owner" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-up", "user_id": "update-owner", "desktop_id": "primary", "state": state, "epoch": epoch})
		case "/internal/vmctl/deploy-refresh":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			deployCalls = append(deployCalls, body)
			if outcome.Outcome == "refreshed" {
				epoch++
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(outcome)
		case "/internal/vmctl/update-available":
			_ = json.NewEncoder(w).Encode(signal)
		case "/internal/vmctl/refresh":
			legacy++
			http.Error(w, "must not be called", http.StatusTeapot)
		case "/internal/vmctl/touch":
			_, _ = w.Write([]byte(`{"touched":true}`))
		default:
			t.Fatalf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	t.Cleanup(vm.Close)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request platform.LifecycleControlRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Action != "update" {
			t.Fatalf("lifecycle action = %q, want update", request.Action)
		}
		if request.Phase == "prepare" {
			_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{Status: "pending", Action: request.Action, PriorState: request.PriorState, PriorEpoch: request.PriorEpoch})
			return
		}
		completes++
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.LifecycleControlResult{Status: "completed", Action: request.Action,
			Receipt: &computerevent.Receipt{ReceiptKind: "LifecycleReceipt", ReceiptID: "update-receipt"}})
	}))
	t.Cleanup(corpusd.Close)

	handler, priv, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()
	handler.vmctlClient = vmctl.NewClient(vm.URL)
	user, err := store.CreateUser("update-owner", "update-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	return &updateFixture{
		handler: handler, deployRefresh: &deployCalls, legacyRefresh: &legacy, completes: &completes,
		outcome: outcome, outcomeStatus: &status, signal: signal,
		ownerToken: issueTestAccessJWT(priv, "update-owner"), otherUserToken: issueTestAccessJWT(priv, "someone-else"),
		secretFor: func(scopes ...string) string {
			_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "update", scopes, "computer-up", nil)
			if err != nil {
				t.Fatal(err)
			}
			return secret
		},
	}
}

func (f *updateFixture) post(token, bearer, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/computers/computer-up/lifecycle/update", strings.NewReader(`{"idempotency_key":"`+key+`"}`))
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	f.handler.HandleAPI(rec, req)
	return rec
}

func TestComputerUpdateParsesAsALifecycleAction(t *testing.T) {
	id, action, ok := parseComputerLifecyclePath("/api/computers/computer-up/lifecycle/update")
	if !ok || id != "computer-up" || action != "update" {
		t.Fatalf("update not parsed: %q %q %v", id, action, ok)
	}
	if !platform.OwnerVMLifecycleAdvancesEpoch("update") {
		t.Fatal("update must advance the epoch")
	}
}

func TestComputerUpdateRefreshedUsesDeployRefreshWithReceipt(t *testing.T) {
	f := newUpdateFixture(t)
	rec := f.post(f.ownerToken, "", "u1")
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "update-receipt") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if *f.legacyRefresh != 0 {
		t.Fatal("the owner update called /internal/vmctl/refresh")
	}
	if len(*f.deployRefresh) != 1 || (*f.deployRefresh)[0]["owner_busy"] != true || (*f.deployRefresh)[0]["target"] != "" {
		t.Fatalf("deploy-refresh calls = %v, want one owner_busy request with no caller-chosen target", *f.deployRefresh)
	}
	if *f.completes != 1 {
		t.Fatalf("completes=%d, want one receipt", *f.completes)
	}
}

func TestComputerUpdateCurrentIs200WithoutReceipt(t *testing.T) {
	f := newUpdateFixture(t)
	*f.outcome = vmctl.OwnerUpdateResult{Outcome: "current", Target: "commit-new"}
	rec := f.post(f.ownerToken, "", "u1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"current"`) || *f.completes != 0 {
		t.Fatalf("status=%d body=%s completes=%d", rec.Code, rec.Body.String(), *f.completes)
	}
}

func TestComputerUpdateDeferredAndSkippedAre409WithStructuredRefusal(t *testing.T) {
	f := newUpdateFixture(t)
	*f.outcomeStatus = http.StatusConflict
	*f.outcome = vmctl.OwnerUpdateResult{Outcome: "deferred", Reason: "busy: {...}", ReasonCode: "live_capsule",
		Refusal: json.RawMessage(`{"outcome":"busy","live_capsules":1}`)}
	rec := f.post(f.ownerToken, "", "u1")
	var body map[string]any
	if rec.Code != http.StatusConflict || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body["outcome"] != "deferred" || body["reason"] != "live_capsule" {
		t.Fatalf("deferred: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if detail, _ := body["detail"].(map[string]any); detail["live_capsules"] != float64(1) {
		t.Fatalf("refusal detail not passed through: %v", body)
	}
	*f.outcome = vmctl.OwnerUpdateResult{Outcome: "skipped", Reason: "ring R3 waits: ring walk w is at R0"}
	rec = f.post(f.ownerToken, "", "u2")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"skipped"`) || !strings.Contains(rec.Body.String(), "waits") {
		t.Fatalf("skipped: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if *f.completes != 0 {
		t.Fatalf("an early outcome wrote a receipt (%d)", *f.completes)
	}
}

// FM4: only the owner session or an owner_acts key; computer:lifecycle and
// admin keys and other users cannot.
func TestComputerUpdateRequiresOwnerAct(t *testing.T) {
	f := newUpdateFixture(t)
	for label, secret := range map[string]string{
		"lifecycle key": f.secretFor("computer:lifecycle"),
		"admin key":     f.secretFor("admin"),
	} {
		rec := f.post("", secret, "u-"+label)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: status=%d body=%s, want 403", label, rec.Code, rec.Body.String())
		}
	}
	if rec := f.post(f.otherUserToken, "", "u-other"); rec.Code != http.StatusNotFound {
		t.Fatalf("another user's session: status=%d, want 404", rec.Code)
	}
	if len(*f.deployRefresh) != 0 {
		t.Fatalf("a denied caller reached vmctl: %v", *f.deployRefresh)
	}
	// The owner_acts key alone passes (no computer:lifecycle needed).
	if rec := f.post("", f.secretFor(OwnerActsScope), "u-owner-key"); rec.Code != http.StatusCreated {
		t.Fatalf("owner_acts key: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestComputerUpdateSignalRoute(t *testing.T) {
	f := newUpdateFixture(t)
	get := func(token, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/computers/computer-up/update", nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		f.handler.HandleAPI(rec, req)
		return rec
	}
	rec := get(f.ownerToken, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"update_available":null`) {
		t.Fatalf("idle: %d %s", rec.Code, rec.Body.String())
	}
	*f.signal = vmctl.UpdateAvailableResponse{Available: true, Target: "commit-new", Since: "2026-11-09T10:00:00Z"}
	rec = get(f.ownerToken, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"target":"commit-new"`) {
		t.Fatalf("available: %d %s", rec.Code, rec.Body.String())
	}
	if rec = get("", f.secretFor("read:runtime")); rec.Code != http.StatusOK {
		t.Fatalf("read:runtime key: %d %s", rec.Code, rec.Body.String())
	}
	if rec = get("", f.secretFor("computer:lifecycle")); rec.Code != http.StatusForbidden {
		t.Fatalf("key without read:runtime: %d", rec.Code)
	}
	if rec = get(f.otherUserToken, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("another user: %d", rec.Code)
	}
	if rec = get("", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", rec.Code)
	}
}

// Review follow-up: a vmctl failure or a plain refusal is a named answer, not a
// 502 the chip would read as "the restart is running".
func TestComputerUpdateFailedAndCodesAreNamedNotBadGateway(t *testing.T) {
	f := newUpdateFixture(t)
	*f.outcomeStatus = http.StatusInternalServerError
	*f.outcome = vmctl.OwnerUpdateResult{Outcome: "failed", Reason: "boot failed", ReasonCode: "failed"}
	rec := f.post(f.ownerToken, "", "f1")
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"reason":"failed"`) {
		t.Fatalf("failed: %d %s", rec.Code, rec.Body.String())
	}
	*f.outcomeStatus = http.StatusConflict
	*f.outcome = vmctl.OwnerUpdateResult{Outcome: "deferred", Reason: "a deploy refresh of this computer is already in flight", ReasonCode: "refresh_in_flight"}
	rec = f.post(f.ownerToken, "", "f2")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"reason":"refresh_in_flight"`) {
		t.Fatalf("in flight: %d %s", rec.Code, rec.Body.String())
	}
	*f.outcomeStatus = http.StatusNotFound
	*f.outcome = vmctl.OwnerUpdateResult{Outcome: "skipped", Reason: "no computer"}
	if rec = f.post(f.ownerToken, "", "f3"); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"skipped"`) {
		t.Fatalf("no computer: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOwnerUpdateClientMapsAPlainConflictToASkip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"route does not join"}`))
	}))
	defer srv.Close()
	res, err := vmctl.NewClient(srv.URL).OwnerUpdateContext(context.Background(), "u", "primary")
	if err != nil || res.Outcome != "skipped" || res.Reason != "route does not join" {
		t.Fatalf("plain 409: %+v err=%v", res, err)
	}
}
