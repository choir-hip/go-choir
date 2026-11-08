package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Enforce flip S1, section 6.1: the route gate enforces the package rows and
// only counts the built-in ones, so its observability label must say that
// rather than the bare word "observe".
func TestObservabilityRouteGateLabelDoesNotClaimObserve(t *testing.T) {
	rt, _ := testRuntime(t)
	h := &APIHandler{rt: rt}
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/observability", nil)
	req.Header.Set("X-Authenticated-User", "owner-label")
	w := httptest.NewRecorder()
	h.HandleGuestObservability(w, req)
	var resp struct {
		Route *struct {
			Mode string `json:"mode"`
		} `json:"route_app_gate"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Route == nil {
		t.Fatalf("observability = %d %s", w.Code, w.Body.String())
	}
	if resp.Route.Mode == "observe" || !strings.Contains(resp.Route.Mode, "enforce package") {
		t.Fatalf("route_app_gate mode = %q; it enforces package routes and observes built-in ones", resp.Route.Mode)
	}
}
