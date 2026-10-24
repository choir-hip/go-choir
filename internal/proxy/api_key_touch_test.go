package proxy

// Gap K (docs/problems/idle-sweep-hibernates-a-computer-just-after-its-apply-restart-2026-10-24.md):
// an authenticated API-key request refreshes the computer's idle clock through
// vmctl's touch route, best-effort, throttled, and never at the request's cost.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type touchFixture struct {
	handler  *Handler
	touches  atomic.Int64
	lookups  atomic.Int64
	touched  chan struct{}
	lastBody atomic.Value
}

func newTouchFixture(t *testing.T, touchStatus int, touchDelay time.Duration) *touchFixture {
	t.Helper()
	f := &touchFixture{touched: make(chan struct{}, 16)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			f.lookups.Add(1)
			_, _ = w.Write([]byte(`{"computer_id":"computer-a","user_id":"owner","desktop_id":"primary","kind":"interactive","state":"active","epoch":7}`))
		case "/internal/vmctl/touch":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.lastBody.Store(body)
			f.touches.Add(1)
			f.touched <- struct{}{}
			time.Sleep(touchDelay)
			w.WriteHeader(touchStatus)
			_, _ = w.Write([]byte(`{"touched":true}`))
		default:
			t.Errorf("unexpected vmctl path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	f.handler = &Handler{vmctlClient: vmctl.NewClient(server.URL)}
	return f
}

func (f *touchFixture) request(t *testing.T, auth AuthResult) (*resolvedComputerTarget, bool, int) {
	t.Helper()
	rec := httptest.NewRecorder()
	target, ok := f.handler.requireAPIKeyComputerTarget(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil), &auth, "", "")
	return target, ok, rec.Code
}

func (f *touchFixture) waitTouch(t *testing.T) {
	t.Helper()
	select {
	case <-f.touched:
	case <-time.After(3 * time.Second):
		t.Fatal("no touch reached vmctl")
	}
}

func TestAPIKeyRequestsTouchTheComputerOncePerThrottleWindow(t *testing.T) {
	f := newTouchFixture(t, http.StatusOK, 0)
	auth := AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}
	for i := 0; i < 5; i++ {
		if _, ok, _ := f.request(t, auth); !ok {
			t.Fatalf("request %d refused", i)
		}
	}
	f.waitTouch(t)
	time.Sleep(100 * time.Millisecond)
	if got := f.touches.Load(); got != 1 {
		t.Fatalf("5 requests inside the window sent %d touches, want 1", got)
	}
	if body, _ := f.lastBody.Load().(map[string]string); body["user_id"] != "owner" || body["computer_id"] != "computer-a" {
		t.Fatalf("touch body = %v", body)
	}

	// Past the window, the next request touches again.
	f.handler.apiKeyTouchSent.Range(func(k, _ any) bool {
		f.handler.apiKeyTouchSent.Store(k, time.Now().Add(-apiKeyTouchThrottle-time.Second))
		return true
	})
	if _, ok, _ := f.request(t, auth); !ok {
		t.Fatal("request after the window refused")
	}
	f.waitTouch(t)
	if got := f.touches.Load(); got != 2 {
		t.Fatalf("touches = %d, want 2", got)
	}
}

func TestOwnerWideKeyTouchesTheNamedComputer(t *testing.T) {
	f := newTouchFixture(t, http.StatusOK, 0)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/x?computer_id=computer-a", nil)
	if _, ok := f.handler.requireAPIKeyComputerTarget(rec, req, &AuthResult{AuthMethod: "api_key", UserID: "owner"}, "", ""); !ok {
		t.Fatalf("owner-wide request refused: %d", rec.Code)
	}
	f.waitTouch(t)
	if body, _ := f.lastBody.Load().(map[string]string); body["computer_id"] != "computer-a" {
		t.Fatalf("touch body = %v", body)
	}
}

func TestATouchFailureOrStallNeverFailsOrDelaysTheRequest(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		delay  time.Duration
	}{{"vmctl 500", http.StatusInternalServerError, 0}, {"vmctl 404", http.StatusNotFound, 0}, {"vmctl stalls", http.StatusOK, 600 * time.Millisecond}} {
		f := newTouchFixture(t, tc.status, tc.delay)
		start := time.Now()
		target, ok, _ := f.request(t, AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"})
		if !ok || target == nil || target.ComputerID != "computer-a" || target.State != "active" {
			t.Fatalf("%s: target=%+v ok=%v", tc.name, target, ok)
		}
		if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
			t.Errorf("%s: request took %s", tc.name, elapsed)
		}
		f.waitTouch(t)
	}
}

func TestRefusedAndCookieRequestsSendNoTouch(t *testing.T) {
	f := newTouchFixture(t, http.StatusOK, 0)
	// A key bound to another computer, a foreign path target and a cookie
	// session are all refused or skipped before any touch.
	rec := httptest.NewRecorder()
	if _, ok := f.handler.requireAPIKeyComputerTarget(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil), &AuthResult{AuthMethod: "api_key", UserID: "owner", ComputerID: "computer-a"}, "computer-b", ""); ok {
		t.Fatal("foreign path target admitted")
	}
	if _, ok := f.handler.requireAPIKeyComputerTarget(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/x", nil), &AuthResult{AuthMethod: "cookie", UserID: "owner"}, "", ""); !ok {
		t.Fatal("cookie request refused")
	}
	time.Sleep(150 * time.Millisecond)
	if got := f.touches.Load(); got != 0 {
		t.Fatalf("touches = %d, want 0", got)
	}
}
