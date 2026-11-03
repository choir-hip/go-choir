package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// An owner-wide key's status read on an account with no computer creates the
// primary computer once (not once per poll); a cookie session's does not.
func TestOwnerWideAPIKeyComputeStatusCreatesFirstComputerOnce(t *testing.T) {
	handler, priv, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("first-computer", "first-computer@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(t.Context(), user.ID, "first computer", []string{"read:runtime"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var resolves atomic.Int64
	release := make(chan struct{})
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			http.NotFound(w, r)
		case "/internal/vmctl/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"ownerships": []any{}})
		case "/internal/vmctl/resolve":
			resolves.Add(1)
			<-release
			_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "c1", "user_id": user.ID, "desktop_id": "primary", "kind": "interactive", "state": "active"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer vmctlServer.Close()
	defer close(release)
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)

	// A cookie session never creates from a status read.
	req := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, user.ID)})
	handler.HandleAPI(httptest.NewRecorder(), req)
	time.Sleep(50 * time.Millisecond)
	if n := resolves.Load(); n != 0 {
		t.Fatalf("cookie status resolved %d times, want 0", n)
	}

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		rec := httptest.NewRecorder()
		handler.HandleAPI(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "being created") {
			t.Fatalf("poll %d: status=%d body=%s", i, rec.Code, rec.Body.String())
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for resolves.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if n := resolves.Load(); n != 1 {
		t.Fatalf("resolves=%d, want exactly 1 across 3 polls", n)
	}
}

func TestFirstComputerGateHoldsOffAfterFailure(t *testing.T) {
	var g firstComputerGate
	now := time.Now()
	if !g.begin("u", now) || g.begin("u", now) {
		t.Fatal("want one flight at a time")
	}
	g.end("u", true, now)
	if g.begin("u", now.Add(time.Second)) {
		t.Fatal("a failure must hold off the next attempt")
	}
	if !g.begin("u", now.Add(firstComputerRetryAfter+time.Second)) {
		t.Fatal("the hold must expire")
	}
	g.end("u", false, now)
	if !g.begin("u", now) {
		t.Fatal("success clears the gate")
	}
}
