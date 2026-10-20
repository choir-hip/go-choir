package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The visual-evidence listing is a read of a frozen candidate, like the
// source-patch export: the proxy classifier must know the suffix (else it
// answers 404 before the guest sees it), the read must not borrow a write
// scope or accept a write method, and the upstream path is not rewritten.
func TestSelfDevelopmentGuestServesVisualEvidenceWithReadScope(t *testing.T) {
	const path = "/api/computers/computer-a/self-development/operations/op-1/visual-evidence"
	if computerID, action, ok := selfDevelopmentGuestRoute(path); !ok || computerID != "computer-a" || action != "read" {
		t.Fatalf("classifier = %q %q %v, want computer-a read", computerID, action, ok)
	}
	for _, bad := range []string{
		"/api/computers/computer-a/self-development/operations/op-1/visual-evidence/extra",
		"/api/computers/computer-a/self-development/operations//visual-evidence",
		"/api/computers/computer-a/self-development/visual-evidence",
	} {
		if _, _, ok := selfDevelopmentGuestRoute(bad); ok {
			t.Errorf("classifier accepted %q", bad)
		}
	}
	var gotPath, gotMethod string
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"operation_id":"op-1","captures":[]}`))
	}))
	defer autoputer.Close()
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("selfdev-visual-user", "selfdev-visual@example.com")
	if err != nil {
		t.Fatal(err)
	}
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": autoputer.URL,
		})
	}))
	defer ownership.Close()
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	call := func(method, scope string) *httptest.ResponseRecorder {
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "visual-"+method+"-"+scope, []string{scope}, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(method, path, nil)
		request.Header.Set("Authorization", "Bearer "+secret)
		response := httptest.NewRecorder()
		handler.HandleAPI(response, request)
		return response
	}
	if response := call(http.MethodGet, "computer:self_development:read"); response.Code != http.StatusOK || gotMethod != http.MethodGet || gotPath != path {
		t.Fatalf("read status=%d upstream=%s %s body=%s", response.Code, gotMethod, gotPath, response.Body.String())
	}
	gotPath = ""
	for _, scope := range []string{"computer:self_development:propose", "computer:self_development:approve"} {
		if response := call(http.MethodGet, scope); response.Code != http.StatusForbidden || gotPath != "" {
			t.Fatalf("%s status=%d upstream=%q, want 403 and no upstream call", scope, response.Code, gotPath)
		}
	}
	if response := call(http.MethodPost, "computer:self_development:read"); response.Code != http.StatusMethodNotAllowed || gotPath != "" {
		t.Fatalf("POST status=%d upstream=%q, want 405", response.Code, gotPath)
	}
}
