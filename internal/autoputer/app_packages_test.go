package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// memoryAppPackageStore folds events with the real reducer; it stands in for
// the Dolt store so the handler's own rules are what these tests exercise.
type memoryAppPackageStore struct {
	mu     sync.Mutex
	events []apppackage.Event
	data   map[string]apppackage.Datum
}

func (m *memoryAppPackageStore) AppPackageInventory(context.Context, string) (apppackage.Inventory, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return apppackage.Reduce(m.events)
}

func (m *memoryAppPackageStore) RecordAppPackageEvent(_ context.Context, ownerID, _ string, ev apppackage.Event) (apppackage.Inventory, apppackage.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, err := apppackage.Reduce(m.events)
	if err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, err
	}
	ev.Actor = "owner:" + ownerID
	chained, err := apppackage.Chain(inv, ev)
	if err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, err
	}
	next, effect, err := apppackage.Apply(inv, chained)
	if err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, err
	}
	if effect.DeleteData {
		for key, datum := range m.data {
			if datum.Package == ev.Package {
				delete(m.data, key)
			}
		}
	}
	m.events = append(m.events, chained)
	return next, chained, nil
}

func (m *memoryAppPackageStore) PutAppPackageDatum(_ context.Context, _ string, _ string, datum apppackage.Datum) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, err := apppackage.Reduce(m.events)
	if err != nil {
		return err
	}
	stamped, err := apppackage.PrepareDatum(inv, datum)
	if err != nil {
		return err
	}
	if m.data == nil {
		m.data = map[string]apppackage.Datum{}
	}
	m.data[datum.Package+"\x00"+datum.Key] = stamped
	return nil
}

func (m *memoryAppPackageStore) GetAppPackageDatum(_ context.Context, _ string, pkg, key string) (apppackage.Datum, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	datum, ok := m.data[pkg+"\x00"+key]
	if !ok {
		return apppackage.Datum{}, store.ErrNotFound
	}
	return datum, nil
}

// catalogWithHello builds a static fixture into a catalog directory and
// returns the catalog and the build digest.
func catalogWithHello(t *testing.T) (apppackage.BuildStore, string) {
	t.Helper()
	dir := t.TempDir()
	entry := filepath.Join(dir, "frontend", "index.js")
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte("export function mount(t) { t.textContent = 'hello'; return { destroy() {} }; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := apppackage.SourceDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := apppackage.Manifest{
		Schema:   apppackage.ManifestSchemaV1,
		Name:     "hello",
		Version:  "0.1.0",
		Display:  apppackage.Display{Title: "Hello", Icon: "👋"},
		Source:   apppackage.SourceRef{Digest: digest},
		Frontend: apppackage.Frontend{Entry: "frontend/index.js", ABI: apppackage.FrontendABIV1},
		Build:    apppackage.BuildSpec{Recipe: apppackage.RecipeStaticV1, Outputs: []string{"frontend/index.js"}},
		Data:     apppackage.DataSpec{Store: apppackage.DataStoreKVV1, SchemaVersion: 1, ReadsSchemaVersions: []int{1}},
		Restore:  apppackage.RestoreSpec{DataScope: apppackage.DataScopeComputerTape, OnPurge: apppackage.OnPurgeDeleteData},
		Grants:   []json.RawMessage{},
	}
	raw, err := apppackage.MarshalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, apppackage.ManifestFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	_, build, files, err := apppackage.BuildStatic(dir)
	if err != nil {
		t.Fatal(err)
	}
	catalog := apppackage.BuildStore{Root: t.TempDir()}
	buildDigest, err := catalog.Put(build, files)
	if err != nil {
		t.Fatal(err)
	}
	return catalog, buildDigest
}

func newTestAppPackageService(t *testing.T) (*AppPackageService, string) {
	t.Helper()
	catalog, digest := catalogWithHello(t)
	return &AppPackageService{
		Store:      &memoryAppPackageStore{},
		Builds:     apppackage.BuildStore{Root: t.TempDir()},
		Sources:    []apppackage.Source{catalog},
		ComputerID: "computer-pkg-http",
	}, digest
}

func pkgRequest(t *testing.T, svc *AppPackageService, method, path, body string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	if authed {
		req.Header.Set("X-Authenticated-User", "owner@example.com")
	}
	rec := httptest.NewRecorder()
	svc.ServeHTTP(rec, req)
	return rec
}

func mustStatus(t *testing.T, rec *httptest.ResponseRecorder, want int, what string) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("%s: status %d, want %d; body %s", what, rec.Code, want, rec.Body.String())
	}
}

func listPackages(t *testing.T, svc *AppPackageService) appPackageList {
	t.Helper()
	rec := pkgRequest(t, svc, http.MethodGet, "/api/app-packages", "", true)
	mustStatus(t, rec, http.StatusOK, "list")
	var list appPackageList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	return list
}

// TestAppPackageHandlerRequiresAuth covers F29.
func TestAppPackageHandlerRequiresAuth(t *testing.T) {
	svc, digest := newTestAppPackageService(t)
	hex := strings.TrimPrefix(digest, "sha256:")
	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/app-packages", ""},
		{http.MethodGet, "/api/app-packages/catalog", ""},
		{http.MethodPost, "/api/app-packages/install", `{"build_digest":"` + digest + `"}`},
		{http.MethodPost, "/api/app-packages/hello/enable", ""},
		{http.MethodGet, "/api/app-packages/hello/data/count", ""},
		{http.MethodPut, "/api/app-packages/hello/data/count", `{"value":1}`},
		{http.MethodGet, "/api/app-packages/builds/" + hex + "/frontend/index.js", ""},
	} {
		rec := pkgRequest(t, svc, route.method, route.path, route.body, false)
		mustStatus(t, rec, http.StatusUnauthorized, route.method+" "+route.path)
	}
}

// TestAppPackageHandlerInstallTakesDigestOnly covers F28 and the install path.
func TestAppPackageHandlerInstallTakesDigestOnly(t *testing.T) {
	svc, digest := newTestAppPackageService(t)

	// Bytes in the request are refused before anything is looked up.
	for _, body := range []string{
		`{"build_digest":"` + digest + `","files":{"frontend/index.js":"ZXZpbA=="}}`,
		`{"build_digest":"` + digest + `","build":{}}`,
		`{"manifest":{}}`,
		`not json`,
		`{"build_digest":"sha256:nothex"}`,
	} {
		rec := pkgRequest(t, svc, http.MethodPost, "/api/app-packages/install", body, true)
		mustStatus(t, rec, http.StatusBadRequest, "install body "+body)
	}
	missing := "sha256:" + strings.Repeat("ab", 32)
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/install", `{"build_digest":"`+missing+`"}`, true), http.StatusNotFound, "install of an unknown build")
	if svc.Builds.Has(missing) || len(listPackages(t, svc).Packages) != 0 {
		t.Fatal("a refused install left state behind")
	}

	// The catalog lists what can be installed.
	rec := pkgRequest(t, svc, http.MethodGet, "/api/app-packages/catalog", "", true)
	mustStatus(t, rec, http.StatusOK, "catalog")
	if !strings.Contains(rec.Body.String(), digest) {
		t.Fatalf("catalog does not offer the fixture: %s", rec.Body.String())
	}

	rec = pkgRequest(t, svc, http.MethodPost, "/api/app-packages/install", `{"build_digest":"`+digest+`"}`, true)
	mustStatus(t, rec, http.StatusOK, "install")
	if !svc.Builds.Has(digest) {
		t.Fatal("install did not store the build before recording it")
	}
	list := listPackages(t, svc)
	if len(list.Packages) != 1 || list.Packages[0].Name != "hello" || list.Packages[0].Status != string(apppackage.StatusInstalled) || list.Packages[0].BuildDigest != digest {
		t.Fatalf("after install: %+v", list.Packages)
	}
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/install", `{"build_digest":"`+digest+`"}`, true), http.StatusConflict, "second install")
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/hello/disable", "", true), http.StatusConflict, "disable while installed")
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/missing/enable", "", true), http.StatusNotFound, "enable of an absent package")
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/hello/explode", "", true), http.StatusNotFound, "unknown lifecycle op")
	mustStatus(t, pkgRequest(t, svc, http.MethodGet, "/api/app-packages/hello/enable", "", true), http.StatusMethodNotAllowed, "GET lifecycle op")
}

// TestAppPackageHandlerServesOnlyEnabledBuilds covers F26 and F27, and the
// list record the desktop loads from.
func TestAppPackageHandlerServesOnlyEnabledBuilds(t *testing.T) {
	svc, digest := newTestAppPackageService(t)
	hex := strings.TrimPrefix(digest, "sha256:")
	entry := "/api/app-packages/builds/" + hex + "/frontend/index.js"

	mustStatus(t, pkgRequest(t, svc, http.MethodGet, entry, "", true), http.StatusNotFound, "serve before install")
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/install", `{"build_digest":"`+digest+`"}`, true), http.StatusOK, "install")
	mustStatus(t, pkgRequest(t, svc, http.MethodGet, entry, "", true), http.StatusNotFound, "serve while installed but not enabled")
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/hello/enable", "", true), http.StatusOK, "enable")

	rec := pkgRequest(t, svc, http.MethodGet, entry, "", true)
	mustStatus(t, rec, http.StatusOK, "serve while enabled")
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Fatalf("content type %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "export function mount") {
		t.Fatalf("served body: %s", rec.Body.String())
	}
	if rec.Header().Get("X-Choir-Package-Digest") == "" {
		t.Fatal("served file does not name its digest")
	}

	list := listPackages(t, svc)
	if len(list.Packages) != 1 {
		t.Fatalf("list: %+v", list)
	}
	pkg := list.Packages[0]
	if pkg.Status != string(apppackage.StatusEnabled) || pkg.Frontend.URL != entry || !apppackage.IsDigest(pkg.Frontend.SHA256) || pkg.Frontend.ABI != apppackage.FrontendABIV1 || !pkg.Available {
		t.Fatalf("list record: %+v", pkg)
	}
	if list.Seq != 2 || !apppackage.IsDigest(list.Head) {
		t.Fatalf("list head: seq=%d head=%q", list.Seq, list.Head)
	}

	mustStatus(t, pkgRequest(t, svc, http.MethodGet, "/api/app-packages/builds/"+hex+"/frontend/other.js", "", true), http.StatusNotFound, "path outside the receipt")
	mustStatus(t, pkgRequest(t, svc, http.MethodGet, "/api/app-packages/builds/"+hex+"/../build.json", "", true), http.StatusNotFound, "path escape")
	mustStatus(t, pkgRequest(t, svc, http.MethodGet, "/api/app-packages/builds/"+strings.Repeat("cd", 32)+"/frontend/index.js", "", true), http.StatusNotFound, "unbound digest")

	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/hello/disable", "", true), http.StatusOK, "disable")
	mustStatus(t, pkgRequest(t, svc, http.MethodGet, entry, "", true), http.StatusNotFound, "serve after disable")
}

func TestAppPackageHandlerData(t *testing.T) {
	svc, digest := newTestAppPackageService(t)
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/install", `{"build_digest":"`+digest+`"}`, true), http.StatusOK, "install")
	mustStatus(t, pkgRequest(t, svc, http.MethodPut, "/api/app-packages/hello/data/count", `{"value":1}`, true), http.StatusConflict, "write while not enabled")
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/hello/enable", "", true), http.StatusOK, "enable")

	mustStatus(t, pkgRequest(t, svc, http.MethodGet, "/api/app-packages/hello/data/count", "", true), http.StatusNotFound, "read of an unwritten key")
	mustStatus(t, pkgRequest(t, svc, http.MethodPut, "/api/app-packages/hello/data/count", `{"value":3}`, true), http.StatusOK, "write")
	rec := pkgRequest(t, svc, http.MethodGet, "/api/app-packages/hello/data/count", "", true)
	mustStatus(t, rec, http.StatusOK, "read")
	var got struct {
		Value         json.RawMessage `json:"value"`
		SchemaVersion int             `json:"schema_version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || string(got.Value) != "3" || got.SchemaVersion != 1 {
		t.Fatalf("read back %s (%v)", rec.Body.String(), err)
	}
	mustStatus(t, pkgRequest(t, svc, http.MethodPut, "/api/app-packages/hello/data/Bad-Key", `{"value":3}`, true), http.StatusBadRequest, "bad key")
	mustStatus(t, pkgRequest(t, svc, http.MethodPut, "/api/app-packages/hello/data/count", `{"value":3,"extra":1}`, true), http.StatusBadRequest, "unknown field")
	mustStatus(t, pkgRequest(t, svc, http.MethodPut, "/api/app-packages/hello/data/count", `{}`, true), http.StatusBadRequest, "missing value")

	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/hello/purge", "", true), http.StatusConflict, "purge while enabled")
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/hello/disable", "", true), http.StatusOK, "disable")
	mustStatus(t, pkgRequest(t, svc, http.MethodPost, "/api/app-packages/hello/purge", "", true), http.StatusOK, "purge")
	mustStatus(t, pkgRequest(t, svc, http.MethodGet, "/api/app-packages/hello/data/count", "", true), http.StatusNotFound, "read after purge")
	if len(listPackages(t, svc).Packages) != 0 {
		t.Fatal("purged package still listed")
	}
}

func TestAppPackageStatusForErrors(t *testing.T) {
	for err, want := range map[error]int{
		apppackage.ErrNotInstalled:      http.StatusNotFound,
		apppackage.ErrAlreadyInstalled:  http.StatusConflict,
		apppackage.ErrInvalidTransition: http.StatusConflict,
		apppackage.ErrNotEnabled:        http.StatusConflict,
		apppackage.ErrIncompatibleData:  http.StatusConflict,
		apppackage.ErrUnsupported:       http.StatusUnprocessableEntity,
		apppackage.ErrInvalidDatum:      http.StatusBadRequest,
		apppackage.ErrBuildNotFound:     http.StatusNotFound,
		store.ErrNotFound:               http.StatusNotFound,
		errors.New("disk on fire"):      http.StatusInternalServerError,
	} {
		if got := appPackageStatusFor(err); got != want {
			t.Errorf("%v -> %d, want %d", err, got, want)
		}
	}
}
