package routekinds

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// MA3.10 failure modes, written first.
//
//	F1 a table row serves a kind whose custodian the row does not name (a
//	   route reaching into another app's kinds unlisted).
//	F2 a built-in route refuses a call from another app or from none (the
//	   advisory phase must count and allow).
//	F3 a built-in route lets a foreign call through without counting it.
//	F4 a package route serves a call from another package, or from none.
//	F5 a desk's in-process call (a declared reader) is counted as an app UI.
//	F6 the owner app, or an admitted surface, is counted.
//	F7 a registered pattern has no row (the gate panics at registration).

func serve(pattern, path, app string, viaQuery bool, ctxReader objectgraph.Writer) *httptest.ResponseRecorder {
	hit := false
	h := Gate(pattern, func(w http.ResponseWriter, r *http.Request) { hit = true; w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if app != "" {
		if viaQuery {
			q := req.URL.Query()
			q.Set(AppQuery, app)
			req.URL.RawQuery = q.Encode()
		} else {
			req.Header.Set(AppHeader, app)
		}
	}
	if ctxReader != "" {
		req = req.WithContext(objectgraph.WithReader(req.Context(), ctxReader))
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	_ = hit
	return rec
}

func TestTableKindsAreServedByNamedCustodians(t *testing.T) {
	for _, row := range Table {
		for _, k := range row.Kinds {
			c, ok := objectgraph.CustodianOf(k)
			if !ok {
				t.Errorf("%v serves %s, which is not registered", row.Patterns, k)
				continue
			}
			found := false
			for _, w := range row.Custodians {
				found = found || w == c
			}
			if !found {
				t.Errorf("%v serves %s, kept by %s, which the row does not name (F1)", row.Patterns, k, c)
			}
		}
	}
}

func TestBuiltInRoutesCountAndAllowForeignCalls(t *testing.T) {
	apps := []struct{ owner, pattern, path string }{
		{"texture", "/api/texture/", "/api/texture/documents/d1"},
		{"texture", "/api/texture/documents", "/api/texture/documents"},
		{"podcast", "/api/podcast/subscriptions", "/api/podcast/subscriptions"},
		{"podcast", "/api/podcast/search", "/api/podcast/search"},
		{"autopaper", "/api/autopaper/events", "/api/autopaper/events"},
		{"autopaper", "/api/autopaper/sources/schedules", "/api/autopaper/sources/schedules"},
		{"shell", "/api/trajectories/", "/api/trajectories/t1"},
	}
	for _, tc := range apps {
		ResetCounts()
		if rec := serve(tc.pattern, tc.path, tc.owner, false, ""); rec.Code != http.StatusNoContent || len(Counts()) != 0 {
			t.Errorf("%s: owner %q = %d, counts %v (F6)", tc.pattern, tc.owner, rec.Code, Counts())
		}
		if rec := serve(tc.pattern, tc.path, tc.owner, true, ""); rec.Code != http.StatusNoContent || len(Counts()) != 0 {
			t.Errorf("%s: owner by query = %d, counts %v (F6)", tc.pattern, rec.Code, Counts())
		}
		if rec := serve(tc.pattern, tc.path, "some-other-app", false, ""); rec.Code != http.StatusNoContent {
			t.Errorf("%s: foreign app refused in observe = %d (F2)", tc.pattern, rec.Code)
		}
		if rec := serve(tc.pattern, tc.path, "", false, ""); rec.Code != http.StatusNoContent {
			t.Errorf("%s: undeclared app refused in observe = %d (F2)", tc.pattern, rec.Code)
		}
		c := Counts()
		if c[tc.pattern+"|some-other-app|wrong_app"] != 1 || c[tc.pattern+"|undeclared|undeclared_app"] != 1 {
			t.Errorf("%s: counts = %v, want one wrong_app and one undeclared_app (F3)", tc.pattern, c)
		}
		ResetCounts()
		if rec := serve(tc.pattern, tc.path, "", false, objectgraph.WriterManagement); rec.Code != http.StatusNoContent || len(Counts()) != 0 {
			t.Errorf("%s: desk call counted as an app UI (F5): %v", tc.pattern, Counts())
		}
	}
}

func TestOwnerRootAdmitsOpsSurfacesNotTexture(t *testing.T) {
	ResetCounts()
	for _, app := range []string{"shell", "compute-monitor", "pulse"} {
		serve("/api/trajectories/", "/api/trajectories/t", app, false, "")
	}
	if len(Counts()) != 0 {
		t.Fatalf("admitted surfaces counted: %v", Counts())
	}
	serve("/api/trajectories/", "/api/trajectories/t", "texture", false, "")
	if Counts()["/api/trajectories/|texture|wrong_app"] != 1 {
		t.Fatalf("texture on the owner root not counted: %v", Counts())
	}
}

func TestPackageRouteRefusesOtherPackagesAndUndeclared(t *testing.T) {
	ResetCounts()
	p := "/api/app-packages/notes/objects/note"
	if rec := serve(PackageRoutePrefix, p, "notes", false, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("owning package = %d", rec.Code)
	}
	if rec := serve(PackageRoutePrefix, p, "other", false, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("other package = %d, want 403 (F4)", rec.Code)
	}
	if rec := serve(PackageRoutePrefix, p, "", false, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("undeclared = %d, want 403 (F4)", rec.Code)
	}
	if rec := serve(PackageRoutePrefix, "/api/app-packages/", "notes", false, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("no package segment = %d, want 403", rec.Code)
	}
	c := Counts()
	if c[PackageRoutePrefix+"|other|wrong_app"] != 1 || c[PackageRoutePrefix+"|undeclared|undeclared_app"] != 1 {
		t.Fatalf("counts = %v", c)
	}
}

func TestGateWithoutRowPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Gate registered a pattern with no row (F7)")
		}
	}()
	Gate("/api/not-in-the-table", func(http.ResponseWriter, *http.Request) {})
}
