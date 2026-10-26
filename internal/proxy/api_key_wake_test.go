package proxy

// docs/problems/api-key-request-to-a-hibernated-computer-gets-502-2026-10-26.md:
// a bearer request whose exactly-joined computer is hibernated wakes that
// computer through vmctl's resolve (the path `computer start` uses), re-joins
// the realized computer, then serves; a refused or timed-out wake is an honest
// 503, never the opaque 502.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type wakeFixture struct {
	handler  *Handler
	secret   string
	userID   string
	exact    atomic.Int64 // requests the woken guest served
	resolves atomic.Int64
	lookups  atomic.Int64

	mu            sync.Mutex
	computers     map[string]*wakeComputer
	resolveBodies []map[string]string

	resolveStatus int
	resolveBody   string
	resolveDelay  time.Duration
	// resolveComputerID overrides the computer_id the resolve answer names.
	resolveComputerID string
	// resolveLeavesAsleep answers the wake with success but changes nothing.
	resolveLeavesAsleep bool
}

type wakeComputer struct {
	state     string
	desktopID string
	kind      string
	// afterWakeDesktopID, when set, is the desktop the computer reports once
	// woken (a remapped realization).
	afterWakeDesktopID string
	url                string
}

func newWakeFixture(t *testing.T, boundTo string, scopes []string, computers map[string]*wakeComputer) *wakeFixture {
	t.Helper()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	f := &wakeFixture{handler: handler, computers: computers}
	user, err := store.CreateUser("wake-owner-"+strings.ReplaceAll(t.Name(), "/", "-"), "wake-"+strings.ReplaceAll(t.Name(), "/", "-")+"@example.com")
	if err != nil {
		t.Fatal(err)
	}
	f.userID = user.ID
	if scopes == nil {
		scopes = []string{"admin"}
	}
	if boundTo != "" {
		_, f.secret, err = store.CreateComputerScopedAPIKey(t.Context(), user.ID, "wake", scopes, boundTo, nil)
	} else {
		_, f.secret, err = store.CreateAPIKey(t.Context(), user.ID, "wake owner-wide", scopes, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.exact.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"user": r.Header.Get("X-Authenticated-User")})
	}))
	t.Cleanup(guest.Close)
	for _, c := range computers {
		c.url = guest.URL
	}
	f.resolveStatus = http.StatusOK
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/touch":
			_, _ = w.Write([]byte(`{"touched":true}`))
		case "/internal/vmctl/lookup":
			f.lookups.Add(1)
			id := r.URL.Query().Get("computer_id")
			f.mu.Lock()
			c := computers[id]
			var reply map[string]any
			if c != nil {
				desktop := c.desktopID
				if c.state == "active" && c.afterWakeDesktopID != "" {
					desktop = c.afterWakeDesktopID
				}
				kind := c.kind
				if kind == "" {
					kind = "interactive"
				}
				reply = map[string]any{"computer_id": id, "user_id": user.ID, "desktop_id": desktop, "kind": kind,
					"computer_url": c.url, "state": c.state, "epoch": 7}
			}
			f.mu.Unlock()
			if reply == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(reply)
		case "/internal/vmctl/resolve":
			f.resolves.Add(1)
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.resolveBodies = append(f.resolveBodies, body)
			f.mu.Unlock()
			if f.resolveDelay > 0 {
				time.Sleep(f.resolveDelay)
			}
			if f.resolveStatus != http.StatusOK {
				w.WriteHeader(f.resolveStatus)
				_, _ = w.Write([]byte(f.resolveBody))
				return
			}
			// Wake the computer that owns this desktop.
			var woken string
			f.mu.Lock()
			for id, c := range computers {
				if !f.resolveLeavesAsleep && c.desktopID == body["desktop_id"] {
					c.state = "active"
					woken = id
				}
			}
			f.mu.Unlock()
			if f.resolveComputerID != "" {
				woken = f.resolveComputerID
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": woken, "user_id": body["user_id"],
				"desktop_id": body["desktop_id"], "computer_url": guest.URL, "state": "active"})
		default:
			t.Errorf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	t.Cleanup(vmctlServer.Close)
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	return f
}

func (f *wakeFixture) get(path string, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+f.secret)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.handler.HandleAPI(rec, req)
	return rec
}

func decodeWakeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %q", rec.Body.String())
	}
	return body
}

func TestBearerRequestWakesHibernatedJoinedComputer(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
	})
	rec := f.get("/api/shell/bootstrap", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if f.resolves.Load() != 1 || f.exact.Load() != 1 {
		t.Fatalf("resolves=%d guest=%d want 1/1", f.resolves.Load(), f.exact.Load())
	}
	if got := f.resolveBodies[0]; got["user_id"] != f.userID || got["desktop_id"] != "primary" {
		t.Fatalf("wake named %v, want the joined user and desktop", got)
	}
}

func TestBearerRequestWakesStoppedJoinedComputer(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "stopped", desktopID: "primary"},
	})
	if rec := f.get("/api/shell/bootstrap", nil); rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if f.resolves.Load() != 1 {
		t.Fatalf("resolves=%d want 1", f.resolves.Load())
	}
}

func TestBearerPromptBarWakesHibernatedJoinedComputer(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/prompt-bar", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Authorization", "Bearer "+f.secret)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.handler.HandleAPI(rec, req)
	if rec.Code == http.StatusBadGateway {
		t.Fatalf("prompt-bar answered the opaque 502: %s", rec.Body.String())
	}
	if f.resolves.Load() != 1 {
		t.Fatalf("prompt-bar status=%d resolves=%d want 1 wake; body=%s", rec.Code, f.resolves.Load(), rec.Body.String())
	}
}

func TestBearerWakeMustRejoinTheSameRealization(t *testing.T) {
	t.Run("woken desktop differs from the join", func(t *testing.T) {
		f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
			"computer-a": {state: "hibernated", desktopID: "primary", afterWakeDesktopID: "branch"},
		})
		rec := f.get("/api/shell/bootstrap", nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if body := decodeWakeBody(t, rec); body["error"] != "computer_waking_failed" || body["state"] != "hibernated" {
			t.Fatalf("body=%v", body)
		}
		if f.exact.Load() != 0 {
			t.Fatal("a mismatching realization was served")
		}
	})
	t.Run("vmctl woke another computer", func(t *testing.T) {
		f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
			"computer-a": {state: "hibernated", desktopID: "primary"},
		})
		f.resolveComputerID = "computer-other"
		rec := f.get("/api/shell/bootstrap", nil)
		if rec.Code != http.StatusServiceUnavailable || f.exact.Load() != 0 {
			t.Fatalf("status=%d guest=%d body=%s", rec.Code, f.exact.Load(), rec.Body.String())
		}
		if body := decodeWakeBody(t, rec); body["error"] != "computer_waking_failed" {
			t.Fatalf("body=%v", body)
		}
	})
	t.Run("still not active after the wake", func(t *testing.T) {
		f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
			"computer-a": {state: "hibernated", desktopID: "primary"},
		})
		f.resolveLeavesAsleep = true
		rec := f.get("/api/shell/bootstrap", nil)
		if rec.Code != http.StatusServiceUnavailable || f.exact.Load() != 0 {
			t.Fatalf("status=%d guest=%d body=%s", rec.Code, f.exact.Load(), rec.Body.String())
		}
		if body := decodeWakeBody(t, rec); body["error"] != "computer_waking_failed" {
			t.Fatalf("body=%v", body)
		}
	})
}

func TestBearerRefusedWakeIsAnHonest503(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
	})
	f.resolveStatus = http.StatusServiceUnavailable
	f.resolveBody = `{"error":"host admission refused: no room for this computer"}`
	rec := f.get("/api/shell/bootstrap", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503 body=%s", rec.Code, rec.Body.String())
	}
	body := decodeWakeBody(t, rec)
	if body["error"] != "computer_waking_failed" || body["state"] != "hibernated" {
		t.Fatalf("body=%v", body)
	}
	if cause, _ := body["cause"].(string); !strings.Contains(cause, "no room") {
		t.Fatalf("cause=%q does not carry vmctl's refusal", cause)
	}
	if strings.Contains(rec.Body.String(), "127.0.0.1") {
		t.Fatalf("cause leaks an internal address: %s", rec.Body.String())
	}
	if f.exact.Load() != 0 {
		t.Fatal("the guest was reached after a refused wake")
	}
	// The same honest body on the prompt bar, whose old answer was
	// "failed to resolve user autoputer".
	req := httptest.NewRequest(http.MethodPost, "/api/prompt-bar", strings.NewReader(`{"prompt":"hi"}`))
	req.Header.Set("Authorization", "Bearer "+f.secret)
	req.Header.Set("Content-Type", "application/json")
	pb := httptest.NewRecorder()
	f.handler.HandleAPI(pb, req)
	if pb.Code != http.StatusServiceUnavailable || decodeWakeBody(t, pb)["error"] != "computer_waking_failed" {
		t.Fatalf("prompt-bar status=%d body=%s", pb.Code, pb.Body.String())
	}
}

func TestBearerWakeTimeoutIsAnHonest503(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
	})
	f.resolveDelay = 400 * time.Millisecond
	f.handler.cfg.VmctlTimeout = 80 * time.Millisecond
	rec := f.get("/api/shell/bootstrap", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503 body=%s", rec.Code, rec.Body.String())
	}
	body := decodeWakeBody(t, rec)
	if body["error"] != "computer_waking_failed" || body["state"] != "hibernated" {
		t.Fatalf("body=%v", body)
	}
	if cause, _ := body["cause"].(string); !strings.Contains(cause, "timed out") {
		t.Fatalf("cause=%q does not name the timeout", cause)
	}
}

func TestBearerStateThatCannotBeWokenNamesTheState(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "failed", desktopID: "primary"},
	})
	rec := f.get("/api/shell/bootstrap", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if body := decodeWakeBody(t, rec); body["error"] != "computer_waking_failed" || body["state"] != "failed" {
		t.Fatalf("body=%v", body)
	}
	if f.resolves.Load() != 0 {
		t.Fatal("a failed computer was woken by a request")
	}
}

func TestConcurrentBearerRequestsShareOneWake(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
	})
	f.resolveDelay = 200 * time.Millisecond
	const n = 8
	codes := make(chan int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- f.get("/api/shell/bootstrap", nil).Code
		}()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK {
			t.Fatalf("a concurrent request got %d", code)
		}
	}
	if f.resolves.Load() != 1 {
		t.Fatalf("resolves=%d want exactly one wake for %d concurrent requests", f.resolves.Load(), n)
	}
	if f.exact.Load() != n {
		t.Fatalf("guest served %d want %d", f.exact.Load(), n)
	}
}

func TestOwnerWideBearerKeyStillNamesItsTargetBeforeAnyWake(t *testing.T) {
	f := newWakeFixture(t, "", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
	})
	rec := f.get("/api/shell/bootstrap", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unnamed owner-wide status=%d body=%s", rec.Code, rec.Body.String())
	}
	if f.resolves.Load() != 0 || f.lookups.Load() != 0 {
		t.Fatalf("resolves=%d lookups=%d before naming a target", f.resolves.Load(), f.lookups.Load())
	}
	rec = f.get("/api/shell/bootstrap", map[string]string{"X-Choir-Computer": "computer-a"})
	if rec.Code != http.StatusOK || f.resolves.Load() != 1 {
		t.Fatalf("named owner-wide status=%d resolves=%d body=%s", rec.Code, f.resolves.Load(), rec.Body.String())
	}
}

func TestOwnerWideBearerKeyWakesOnlyTheComputerItNames(t *testing.T) {
	f := newWakeFixture(t, "", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
		"computer-b": {state: "hibernated", desktopID: "branch"},
	})
	rec := f.get("/api/shell/bootstrap", map[string]string{"X-Choir-Computer": "computer-b"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if f.resolves.Load() != 1 || f.resolveBodies[0]["desktop_id"] != "branch" {
		t.Fatalf("resolves=%d bodies=%v want one wake of computer-b's desktop", f.resolves.Load(), f.resolveBodies)
	}
	f.mu.Lock()
	aState := f.computers["computer-a"].state
	f.mu.Unlock()
	if aState != "hibernated" {
		t.Fatalf("computer-a state=%q; naming computer-b woke it", aState)
	}
}

func TestBearerKeyBoundToOneComputerCannotWakeAnother(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
		"computer-b": {state: "hibernated", desktopID: "branch"},
	})
	// A path that names computer-b with a key bound to computer-a.
	req := httptest.NewRequest(http.MethodGet, "/api/computers/computer-b/lifecycle/status", nil)
	req.Header.Set("Authorization", "Bearer "+f.secret)
	rec := httptest.NewRecorder()
	f.handler.HandleAPI(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	// A selector for another desktop is denied before any wake.
	if rec = f.get("/api/shell/bootstrap", map[string]string{"X-Choir-Desktop": "branch"}); rec.Code != http.StatusForbidden {
		t.Fatalf("desktop selector status=%d body=%s", rec.Code, rec.Body.String())
	}
	if f.resolves.Load() != 0 {
		t.Fatalf("resolves=%d; a key bound to computer-a woke something while denied", f.resolves.Load())
	}
	// A header naming computer-b does not retarget a bound key: the only wake
	// is computer-a's own desktop.
	f.get("/api/shell/bootstrap", map[string]string{"X-Choir-Computer": "computer-b"})
	if f.resolves.Load() != 1 || f.resolveBodies[0]["desktop_id"] != "primary" {
		t.Fatalf("resolves=%d bodies=%v want one wake of computer-a's desktop", f.resolves.Load(), f.resolveBodies)
	}
	f.mu.Lock()
	bState := f.computers["computer-b"].state
	f.mu.Unlock()
	if bState != "hibernated" {
		t.Fatalf("computer-b state=%q", bState)
	}
}

func TestOwnerActRoutesNeverWakeAComputer(t *testing.T) {
	const base = "/api/computers/computer-a"
	acts := []struct{ method, path, body string }{
		{http.MethodPost, base + "/lifecycle/replace-workspace", `{}`},
		{http.MethodPost, base + "/lifecycle/rematerialize-from-tape", `{}`},
		{http.MethodPost, base + "/lifecycle/restore", `{}`},
		{http.MethodPost, base + "/lifecycle/import-residue-snapshot", `{}`},
		{http.MethodPost, base + "/lifecycle/bootstrap-chain", `{}`},
		{http.MethodPost, base + "/lifecycle/settle-producer-reports", `{}`},
		{http.MethodPost, base + "/lifecycle/resume-checkpoints", `{}`},
		{http.MethodPost, base + "/self-development/rollbacks", `{}`},
		{http.MethodPost, base + "/self-development/rebase-retry", `{}`},
		{http.MethodPost, base + "/self-development/adoptions", `{}`},
		{http.MethodPost, base + "/self-development/operations/op1/decision", `{}`},
		{http.MethodPost, base + "/releases", `{"operation_id":"op1"}`},
	}
	for _, act := range acts {
		t.Run(act.method+" "+act.path, func(t *testing.T) {
			f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
				"computer-a": {state: "hibernated", desktopID: "primary"},
			})
			req := httptest.NewRequest(act.method, act.path, strings.NewReader(act.body))
			req.Header.Set("Authorization", "Bearer "+f.secret)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			f.handler.HandleAPI(rec, req)
			if f.resolves.Load() != 0 || f.exact.Load() != 0 {
				t.Fatalf("owner act woke or reached a hibernated computer (status=%d resolves=%d guest=%d)", rec.Code, f.resolves.Load(), f.exact.Load())
			}
		})
	}
}

func TestReadRoutesWakeTheJoinedComputer(t *testing.T) {
	for _, path := range []string{
		"/api/computers/computer-a/lifecycle/producer-reports",
		"/api/computers/computer-a/self-development/operations",
	} {
		t.Run(path, func(t *testing.T) {
			f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
				"computer-a": {state: "hibernated", desktopID: "primary"},
			})
			rec := f.get(path, nil)
			if f.resolves.Load() != 1 {
				t.Fatalf("status=%d resolves=%d want one wake; body=%s", rec.Code, f.resolves.Load(), rec.Body.String())
			}
		})
	}
}

func TestWakeRefusedByRecoveryRefusalRendersTheWakeBody(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
	})
	f.resolveStatus = http.StatusServiceUnavailable
	f.resolveBody = `{"error":"computer recovery blocked","reason":"snapshot voided","kind":"projection_base_missing","retry_after_seconds":30}`
	rec := f.get("/api/shell/bootstrap", nil)
	body := decodeWakeBody(t, rec)
	if rec.Code != http.StatusServiceUnavailable || body["error"] != "computer_waking_failed" || body["state"] != "hibernated" {
		t.Fatalf("status=%d body=%v", rec.Code, body)
	}
	if body["kind"] != "projection_base_missing" || rec.Header().Get("Retry-After") != "30" {
		t.Fatalf("refusal detail lost: body=%v retry-after=%q", body, rec.Header().Get("Retry-After"))
	}
}

func TestWakeCauseDoesNotEchoVmctlText(t *testing.T) {
	f := newWakeFixture(t, "computer-a", nil, map[string]*wakeComputer{
		"computer-a": {state: "hibernated", desktopID: "primary"},
	})
	f.resolveStatus = http.StatusInternalServerError
	f.resolveBody = `{"error":"secret-internal-detail /var/lib/vmctl/x"}`
	rec := f.get("/api/shell/bootstrap", nil)
	if strings.Contains(rec.Body.String(), "secret-internal-detail") || strings.Contains(rec.Body.String(), "/var/lib") {
		t.Fatalf("vmctl text echoed: %s", rec.Body.String())
	}
}
