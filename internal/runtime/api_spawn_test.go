//go:build comprehensive

package runtime

import (
	"net/http"
	"testing"
)

// TestSpawnRouteNotBrowserRegistered verifies that /api/agent/spawn is not
// registered on the browser-public route table.
func TestSpawnRouteNotBrowserRegistered(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)

	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/agent/spawn", "{}", "user-alice")
	if w.Code != http.StatusNotFound {
		t.Fatalf("/api/agent/spawn registered publicly: got status %d, want 404", w.Code)
	}
}
