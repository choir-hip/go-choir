package autoputer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// P2b route failure modes, written before the code (CLAUDE.md "Testing"). The
// manifest, schema and registration modes are in internal/apppackage.
//
//	R1  the route needs a setting to be reached (it has none: MA4.5 deleted
//	    app_packages.objects).
//	R4  an unauthenticated caller, or one that is not the host side of the
//	    guest boundary, reaches the objects.
//	R5  a path names a kind outside the package's namespace
//	    (choir.texture_revision, another package's kind, a traversal) and the
//	    write reaches that kind.
//	R6  an unknown package or an unregistered kind is served.
//	R7  a body that fails the schema, is not JSON, is not an object, or is over
//	    the cap is written.
//	R8  a kind that does not admit owner edits is written by the owner.
//	R9  one owner reads another owner's objects.
//	R10 a write does not round-trip, or an overwrite creates a second object.
//	R11 a method outside GET and PUT does something (DELETE, POST).
//	R12 a written object does not survive a reboot of the store.
//	R13 the route works only while the writer gate is in observe mode, or fails
//	    under enforce.
//	R14 a bad object id (path segment, empty, too long) is written.

const (
	routeOwner    = "owner-a@example.com"
	routeComputer = "computer-a"
)

type routeFixture struct {
	t    *testing.T
	path string
	db   *choirstore.Store
	h    *appPackageObjects
}

func registerRouteFixture(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "apppackage", "testdata", "p2btest.choir-package.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := apppackage.ParseManifest(raw, apppackage.OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	if err := apppackage.RegisterManifest(m); err != nil {
		t.Fatal(err)
	}
}

func newRouteFixture(t *testing.T) *routeFixture {
	t.Helper()
	registerRouteFixture(t)
	path := filepath.Join(t.TempDir(), "runtime.db")
	db, err := choirstore.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	f := &routeFixture{t: t, path: path, db: db, h: &appPackageObjects{store: db, computerID: routeComputer}}
	t.Cleanup(func() { _ = f.db.Close() })
	return f
}

func (f *routeFixture) do(method, path, owner, body string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if owner != "" {
		req.Header.Set("X-Authenticated-User", owner)
	}
	w := httptest.NewRecorder()
	f.h.serve(w, req)
	return w
}

const noteURL = "/api/app-packages/p2btest/objects/note"
const goodNote = `{"title":"first","priority":2}`

func decodeObject(t *testing.T, w *httptest.ResponseRecorder) choirstore.PackageObject {
	t.Helper()
	var out struct {
		Object choirstore.PackageObject `json:"object"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return out.Object
}

func (f *routeFixture) countNotes(owner string) int {
	f.t.Helper()
	objs, err := f.db.ListPackageObjects(objectgraph.WithReader(context.Background(), objectgraph.WriterOwner), "choir.app.p2btest.note", owner, 100)
	if err != nil {
		f.t.Fatal(err)
	}
	return len(objs)
}

// R1. The route has no switch: an authenticated owner reaches it on a fresh
// computer, with no setting stored.
func TestRouteIsOpenToTheOwnerWithNoSetting(t *testing.T) {
	f := newRouteFixture(t)
	if w := f.do(http.MethodPut, noteURL+"/n1", routeOwner, goodNote); w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("PUT on a computer with no setting: %d %s", w.Code, w.Body.String())
	}
	if w := f.do(http.MethodGet, noteURL+"/n1", routeOwner, ""); w.Code != http.StatusOK {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	if _, known := choirstore.KnownComputerSettings["app_packages.objects"]; known {
		t.Fatal("app_packages.objects is still a known setting")
	}
}

// R4
func TestRouteRefusesACallerThatIsNotTheHostSideOwner(t *testing.T) {
	f := newRouteFixture(t)
	if w := f.do(http.MethodPut, noteURL+"/n1", "", goodNote); w.Code != http.StatusUnauthorized {
		t.Fatalf("no identity: %d", w.Code)
	}
	// An in-guest process (loopback) is not the owner, header or no header.
	req := httptest.NewRequest(http.MethodPut, noteURL+"/n1", strings.NewReader(goodNote))
	req.RemoteAddr = "127.0.0.1:4000"
	req.Header.Set("X-Authenticated-User", routeOwner)
	w := httptest.NewRecorder()
	f.h.serve(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("loopback caller: %d, want 401", w.Code)
	}
	if n := f.countNotes(routeOwner); n != 0 {
		t.Fatalf("a refused caller wrote %d objects", n)
	}
}

// R5, R6
func TestRouteCannotNameAKindOutsideThePackageNamespace(t *testing.T) {
	f := newRouteFixture(t)
	for _, path := range []string{
		"/api/app-packages/p2btest/objects/choir.texture_revision/x",
		"/api/app-packages/p2btest/objects/choir.app.mail.message/x",
		"/api/app-packages/texture/objects/revision/x",
		"/api/app-packages/p2btest/objects/../x/y",
		"/api/app-packages/p2btest/objects/Note/x",
		"/api/app-packages/p2btest/objects/note/x/extra",
		"/api/app-packages/p2btest/objects",
		"/api/app-packages/p2btest/notobjects/note/x",
		"/api/app-packages/p2bnone/objects/note/x",
		"/api/app-packages/p2btest/objects/never/x",
	} {
		if w := f.do(http.MethodPut, path, routeOwner, goodNote); w.Code != http.StatusNotFound {
			t.Errorf("PUT %s: %d %s, want 404", path, w.Code, w.Body.String())
		}
		if w := f.do(http.MethodGet, path, routeOwner, ""); w.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", path, w.Code)
		}
	}
	// The write path itself refuses a built-in kind even if a caller skips the route.
	_, err := f.db.PutPackageObject(context.Background(), "choir.texture_revision", routeOwner, "x", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("the store wrote a built-in kind through the package seam")
	}
}

// R7
func TestRouteValidatesTheBodyAgainstTheSchemaBeforeWriting(t *testing.T) {
	f := newRouteFixture(t)
	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"a missing required field": {`{"title":"x"}`, http.StatusUnprocessableEntity},
		"the wrong type":           {`{"title":1,"priority":1}`, http.StatusUnprocessableEntity},
		"too long":                 {`{"title":"` + strings.Repeat("x", 21) + `","priority":1}`, http.StatusUnprocessableEntity},
		"outside the enum":         {`{"title":"x","priority":1,"state":"nope"}`, http.StatusUnprocessableEntity},
		"an array":                 {`[]`, http.StatusUnprocessableEntity},
		"not JSON":                 {`{`, http.StatusBadRequest},
		"an empty body":            {``, http.StatusBadRequest},
		"over the cap":             {`{"title":"x","priority":1,"pad":"` + strings.Repeat("p", appPackageObjectMaxBody) + `"}`, http.StatusRequestEntityTooLarge},
	} {
		if w := f.do(http.MethodPut, noteURL+"/n1", routeOwner, tc.body); w.Code != tc.want {
			t.Errorf("%s: %d %s, want %d", name, w.Code, w.Body.String(), tc.want)
		}
	}
	if n := f.countNotes(routeOwner); n != 0 {
		t.Fatalf("%d objects written by refused bodies", n)
	}
}

// R8
func TestRouteKeepsAKindWithoutOwnerEditsReadOnly(t *testing.T) {
	f := newRouteFixture(t)
	url := "/api/app-packages/p2btest/objects/ledger"
	if w := f.do(http.MethodPut, url+"/l1", routeOwner, `{"entry":"x"}`); w.Code != http.StatusForbidden {
		t.Fatalf("owner write of a desk-only kind: %d %s, want 403", w.Code, w.Body.String())
	}
	if w := f.do(http.MethodGet, url, routeOwner, ""); w.Code != http.StatusOK {
		t.Fatalf("a read of a desk-only kind: %d", w.Code)
	}
	if w := f.do(http.MethodGet, url+"/l1", routeOwner, ""); w.Code != http.StatusNotFound {
		t.Fatalf("the refused write left an object: %d", w.Code)
	}
}

// R9, R10
func TestRouteRoundTripsAndScopesToTheOwner(t *testing.T) {
	f := newRouteFixture(t)

	put := f.do(http.MethodPut, noteURL+"/n1", routeOwner, `{"title":"first","priority":2,"extra":{"k":1}}`)
	if put.Code != http.StatusOK {
		t.Fatalf("put: %d %s", put.Code, put.Body.String())
	}
	if obj := decodeObject(t, put); obj.ID != "n1" || obj.Kind != "choir.app.p2btest.note" {
		t.Fatalf("put response = %+v", obj)
	}
	get := f.do(http.MethodGet, noteURL+"/n1", routeOwner, "")
	if get.Code != http.StatusOK {
		t.Fatalf("get: %d", get.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(decodeObject(t, get).Body, &body); err != nil || body["title"] != "first" {
		t.Fatalf("body = %v, %v", body, err)
	}

	// An overwrite is the same object.
	if w := f.do(http.MethodPut, noteURL+"/n1", routeOwner, `{"title":"second","priority":3}`); w.Code != http.StatusOK {
		t.Fatalf("overwrite: %d", w.Code)
	}
	if n := f.countNotes(routeOwner); n != 1 {
		t.Fatalf("an overwrite left %d objects, want 1", n)
	}
	if w := f.do(http.MethodPut, noteURL+"/n2", routeOwner, goodNote); w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	list := f.do(http.MethodGet, noteURL, routeOwner, "")
	var listed struct {
		Objects []choirstore.PackageObject `json:"objects"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil || len(listed.Objects) != 2 {
		t.Fatalf("list = %s (%v)", list.Body.String(), err)
	}

	// Owner b sees nothing of owner a's, by list or by id, and writes their own.
	if w := f.do(http.MethodGet, noteURL+"/n1", "owner-b@example.com", ""); w.Code != http.StatusNotFound {
		t.Fatalf("owner b read owner a's object: %d", w.Code)
	}
	if n := f.countNotes("owner-b@example.com"); n != 0 {
		t.Fatalf("owner b lists %d of owner a's objects", n)
	}
	if w := f.do(http.MethodPut, noteURL+"/n1", "owner-b@example.com", `{"title":"b's","priority":1}`); w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	again := f.do(http.MethodGet, noteURL+"/n1", routeOwner, "")
	var a map[string]any
	_ = json.Unmarshal(decodeObject(t, again).Body, &a)
	if a["title"] != "second" {
		t.Fatalf("owner b's write changed owner a's object: %v", a)
	}

	if w := f.do(http.MethodGet, noteURL+"?limit=0", routeOwner, ""); w.Code != http.StatusBadRequest {
		t.Fatalf("limit=0: %d", w.Code)
	}
	if w := f.do(http.MethodGet, noteURL+"?limit=1", routeOwner, ""); w.Code != http.StatusOK {
		t.Fatalf("limit=1: %d", w.Code)
	}
}

// R11
func TestRouteOnlyReadsAndWrites(t *testing.T) {
	f := newRouteFixture(t)
	if w := f.do(http.MethodPut, noteURL+"/n1", routeOwner, goodNote); w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	for _, m := range []string{http.MethodDelete, http.MethodPost, http.MethodPatch} {
		if w := f.do(m, noteURL+"/n1", routeOwner, goodNote); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d, want 405", m, w.Code)
		}
	}
	if w := f.do(http.MethodPut, noteURL, routeOwner, goodNote); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT on the collection: %d, want 405", w.Code)
	}
	if n := f.countNotes(routeOwner); n != 1 {
		t.Fatalf("objects = %d, want 1", n)
	}
}

// R12: the object is on the store's tape, not in memory.
func TestRouteObjectsSurviveReopeningTheStore(t *testing.T) {
	f := newRouteFixture(t)
	if w := f.do(http.MethodPut, noteURL+"/n1", routeOwner, goodNote); w.Code != http.StatusOK {
		t.Fatalf("put: %d %s", w.Code, w.Body.String())
	}
	if err := f.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := choirstore.Open(f.path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	f.db, f.h = db, &appPackageObjects{store: db, computerID: routeComputer}
	w := f.do(http.MethodGet, noteURL+"/n1", routeOwner, "")
	if w.Code != http.StatusOK {
		t.Fatalf("after reopening: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(string(decodeObject(t, w).Body), `"first"`) {
		t.Fatalf("body after reopening = %s", w.Body.String())
	}
}

// R13: the route works under the enforcing gate (the writer it declares is the
// owner, which the kind admits). That another desk is refused is P2a's test.
func TestRouteWorksUnderTheEnforcingWriterGate(t *testing.T) {
	f := newRouteFixture(t)
	// The setting write above declares the shell as its writer in the store
	// (MA3.2); the gate is enforced after it for the object write.
	t.Cleanup(objectgraph.SetModeForTest(objectgraph.WriterGateEnforce))
	if w := f.do(http.MethodPut, noteURL+"/n1", routeOwner, goodNote); w.Code != http.StatusOK {
		t.Fatalf("owner write under enforce: %d %s", w.Code, w.Body.String())
	}
}

// R14
func TestRouteRefusesABadObjectID(t *testing.T) {
	f := newRouteFixture(t)
	for _, id := range []string{".hidden", "-x", "a%20b", "a%00b", strings.Repeat("i", 129), "a:b", "a%3Fb"} {
		if w := f.do(http.MethodPut, noteURL+"/"+id, routeOwner, goodNote); w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
			t.Errorf("id %q: %d, want 400 or 404", id, w.Code)
		}
	}
	if n := f.countNotes(routeOwner); n != 0 {
		t.Fatalf("%d objects written under bad ids", n)
	}
}

// MA3.10: the registered package-objects route is gated by app identity: it
// serves the package's own app and refuses another package or an undeclared
// caller before anything is read.
func TestAppPackageObjectsRouteRefusesOtherApps(t *testing.T) {
	mux := http.NewServeMux()
	RegisterAppPackageObjectRoutes(muxAdapter{mux}, nil, "computer-a")
	for who, want := range map[string]int{"other": http.StatusForbidden, "": http.StatusForbidden} {
		req := httptest.NewRequest(http.MethodGet, "/api/app-packages/notes/objects/note", nil)
		if who != "" {
			req.Header.Set("X-Choir-App", who)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("caller %q = %d, want %d", who, rec.Code, want)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/app-packages/notes/objects/note", nil)
	req.Header.Set("X-Choir-App", "notes")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Errorf("the owning package was refused by the app gate")
	}
}

type muxAdapter struct{ *http.ServeMux }

func (m muxAdapter) HandleFunc(p string, h http.HandlerFunc) { m.ServeMux.HandleFunc(p, h) }
