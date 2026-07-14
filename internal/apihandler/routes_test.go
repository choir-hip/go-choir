package apihandler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func TestRegisterRoutesPreservesCanonicalTable(t *testing.T) {
	t.Parallel()

	srv := server.NewServer("apihandler-routes-test", "0")
	RegisterRoutes(srv, runtime.NewAPIHandler(nil), NewHandler(nil), false)

	for _, path := range []string{
		"/health",
		"/api/prompt-bar",
		"/api/prompt-bar/submissions/submission-1",
		"/api/agent/loops",
		"/api/agent/cancel",
		"/api/model-policy/resolve",
		"/api/costs",
		"/api/podcast/subscriptions/refresh",
		"/api/podcast/subscriptions",
		"/api/podcast/search",
		"/api/content/items",
		"/api/content/import-url",
		"/api/ws",
		"/api/browser/capabilities",
		"/api/browser/sessions",
		"/api/browser/sessions/session-1",
		"/api/desktop/state",
		"/api/media/progress",
		"/api/media/recents",
		"/api/preferences/theme",
		"/api/computers/computer-1",
		"/api/app-change-packages",
		"/api/app-change-packages/package-1",
		"/api/adoptions",
		"/api/adoptions/adoption-1",
		"/api/candidate-package-intakes/intake-1/review",
		"/api/trajectories",
		"/api/trajectories/trajectory-1",
		"/api/run-acceptances",
		"/api/run-acceptances/synthesize",
		"/api/run-acceptances/acceptance-1",
		"/api/evals/texture-prompt",
		"/internal/runtime/app-change-packages",
		"/internal/runtime/app-change-packages/package-1",
		"/internal/runtime/channel-casts",
		"/internal/runtime/refresh",
		"/internal/runtime/runs",
		"/internal/runtime/runs/run-1",
		"/internal/texture/documents/document-1",
		"/internal/texture/revisions/revision-1",
		"/internal/texture/proposals",
		"/api/texture/documents",
		"/api/texture/documents/document-1",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if !registeredRouteResponds(srv, path) {
				t.Fatalf("canonical route %q returned the server's unregistered 404", path)
			}
		})
	}

	for _, path := range []string{
		"/api/agent/spawn",
		"/api/agent/status",
		"/api/events",
		"/api/universal-wire/stories",
		"/internal/runtime/objectgraph/web-captures",
	} {
		if registeredRouteResponds(srv, path) {
			t.Fatalf("non-canonical route %q is registered", path)
		}
	}
}

func TestRegisterRoutesGatesTestAPIs(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/prompts",
		"/api/prompts/role-1",
		"/api/test/texture/worker-update",
	} {
		disabled := server.NewServer("apihandler-routes-test-disabled", "0")
		RegisterRoutes(disabled, runtime.NewAPIHandler(nil), NewHandler(nil), false)
		if registeredRouteResponds(disabled, path) {
			t.Fatalf("test route %q registered while disabled", path)
		}

		enabled := server.NewServer("apihandler-routes-test-enabled", "0")
		RegisterRoutes(enabled, runtime.NewAPIHandler(nil), NewHandler(nil), true)
		if !registeredRouteResponds(enabled, path) {
			t.Fatalf("test route %q not registered while enabled", path)
		}
	}
}

func registeredRouteResponds(srv *server.Server, path string) (registered bool) {
	defer func() {
		if recover() != nil {
			registered = true
		}
	}()

	req := httptest.NewRequest(http.MethodPost, path, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w.Code != http.StatusNotFound
}
