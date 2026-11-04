package proxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Package install at the proxy (docs/design/package-install-2026-11-04.md).
// Failure modes pinned:
//
//	F12 an agent key without owner:acts, admin included, installs;
//	F2  a release the store does not list for this owner (missing, withdrawn)
//	    reaches the computer;
//	F1  a supplied record that does not hash to the id reaches the computer;
//	F3  tree bytes the store answers that are not the record's reach it;
//	P1  the computer gets anything but the store's record and verified tree,
//	    or the store is read as anyone but the authenticated owner.

type installProxyEnv struct {
	handler *Handler
	user    string
	mu      sync.Mutex
	calls   []string
	guestIn []byte
	view    func() (int, []byte)
	blob    func() (int, []byte)
	secret  func(t *testing.T, scopes ...string) string
}

func (e *installProxyEnv) record(call string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, call)
}

func newInstallProxyEnv(t *testing.T) *installProxyEnv {
	t.Helper()
	env := &installProxyEnv{}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.record("guest " + r.Method + " " + r.URL.Path)
		env.guestIn, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"adoption_id":"install-1"}`))
	}))
	t.Cleanup(guest.Close)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.record("corpusd " + r.Method + " " + r.URL.Path + " " + r.URL.RawQuery)
		status, body := env.view()
		if strings.Contains(r.URL.Path, "/blobs/") {
			status, body = env.blob()
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(corpusd.Close)
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	user, err := store.CreateUser("install-user", "install@example.com")
	if err != nil {
		t.Fatal(err)
	}
	env.user = user.ID
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": guest.URL,
		})
	}))
	t.Cleanup(ownership.Close)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	env.handler = handler
	var n int
	env.secret = func(t *testing.T, scopes ...string) string {
		t.Helper()
		n++
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "install-key-"+strings.Repeat("x", n), scopes, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		return secret
	}
	return env
}

func (e *installProxyEnv) send(t *testing.T, body, secret string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/self-development/adoptions", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	e.handler.HandleAPI(response, request)
	return response
}

func installRecord(t *testing.T, tree []byte, publisherOwner string) (pkgrelease.Record, string, []byte) {
	t.Helper()
	blob := pkgrelease.Blob{SHA256: computerevent.DigestBytes(tree), Bytes: len(tree)}
	rec := pkgrelease.Record{
		Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: "demo-next", ManifestVersion: "1.0.0"}, Track: publisherOwner,
		Parent: pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: strings.Repeat("1", 40), Tree: pkgrelease.EmptyTree},
		Source: pkgrelease.Source{
			Root: pkgrelease.Root("demo-next"), Tree: strings.Repeat("2", 40), TreePatch: blob, ReviewPatch: blob,
			ChangedPaths: []string{pkgrelease.Root("demo-next") + "/App.svelte"}, BuiltOn: strings.Repeat("1", 40),
		},
		Manifest:  pkgrelease.Manifest{SHA256: strings.Repeat("3", 64), ABI: "choir.app-frontend/svelte-inline/v1"},
		Publisher: pkgrelease.Publisher{OwnerID: publisherOwner, ComputerID: "computer-p"},
		Operation: pkgrelease.Operation{OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	id, err := rec.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := rec.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return rec, id, raw
}

func TestPackageInstallIsTheOwnersActAndCarriesOnlyTheStoresVerifiedRelease(t *testing.T) {
	env := newInstallProxyEnv(t)
	tree := []byte("diff --git a/frontend/src/packages/demo-next/App.svelte b/frontend/src/packages/demo-next/App.svelte\n")
	_, id, raw := installRecord(t, tree, env.user)
	state := pkgrelease.StateListed
	env.view = func() (int, []byte) {
		out, _ := json.Marshal(map[string]any{"release_id": id, "record": json.RawMessage(raw), "listing": map[string]any{"state": state}})
		return http.StatusOK, out
	}
	env.blob = func() (int, []byte) { return http.StatusOK, tree }
	body := `{"release_id":"` + id + `","record":` + string(raw) + `}`
	owner := env.secret(t, "computer:self_development:approve", OwnerActsScope)

	// F12: no owner act, no upstream call.
	for _, scopes := range [][]string{{"computer:self_development:approve"}, {"admin"}} {
		if rec := env.send(t, body, env.secret(t, scopes...)); rec.Code != http.StatusForbidden || len(env.calls) != 0 {
			t.Fatalf("%v = %d, calls %v", scopes, rec.Code, env.calls)
		}
	}
	refused := func(leg, body string, status int, reason string) {
		t.Helper()
		env.calls, env.guestIn = nil, nil
		rec := env.send(t, body, owner)
		var got errorResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != status || got.Reason != reason || env.guestIn != nil {
			t.Fatalf("%s = %d %s, guest got %q", leg, rec.Code, rec.Body.String(), env.guestIn)
		}
	}
	// F2: withdrawn, or not there for this owner.
	state = pkgrelease.StateWithdrawn
	refused("F2 withdrawn", body, http.StatusNotFound, "release_not_published")
	state = pkgrelease.StateListed
	env.view = func() (int, []byte) { return http.StatusNotFound, []byte(`{"error":"package release not published"}`) }
	refused("F2 missing", body, http.StatusNotFound, "release_not_published")
	env.view = func() (int, []byte) {
		out, _ := json.Marshal(map[string]any{"release_id": id, "record": json.RawMessage(raw), "listing": map[string]any{"state": state}})
		return http.StatusOK, out
	}
	// F1: a supplied record that does not hash to the id.
	tampered := strings.Replace(string(raw), `"operation_id":"op-1"`, `"operation_id":"op-z"`, 1)
	refused("F1", `{"release_id":"`+id+`","record":`+tampered+`}`, http.StatusUnprocessableEntity, "release_digest_mismatch")
	// F3: the store's tree bytes are not the record's.
	env.blob = func() (int, []byte) { return http.StatusOK, append(append([]byte{}, tree...), 'x') }
	refused("F3", body, http.StatusConflict, "patch_digest_mismatch")
	env.blob = func() (int, []byte) { return http.StatusOK, tree }

	// Open install (owner, 2026-11-04): another owner's listed release
	// installs; yanked and withdrawn ones are refused; uninstall always works.
	// The forwarded body is exactly {release_id, record, tree_patch, action}:
	// an unrefreshed guest decodes with DisallowUnknownFields.
	_, otherID, otherRaw := installRecord(t, tree, "someone-else")
	viewOf := env.view
	env.view = func() (int, []byte) {
		out, _ := json.Marshal(map[string]any{"release_id": otherID, "record": json.RawMessage(otherRaw), "listing": map[string]any{"state": state}})
		return http.StatusOK, out
	}
	for _, bad := range []string{pkgrelease.StateYanked, pkgrelease.StateWithdrawn} {
		state = bad
		refused("open install "+bad, `{"release_id":"`+otherID+`"}`, http.StatusNotFound, "release_not_published")
	}
	state = pkgrelease.StateListed
	env.guestIn = nil
	if rec := env.send(t, `{"release_id":"`+otherID+`"}`, owner); rec.Code != http.StatusCreated || env.guestIn == nil {
		t.Fatalf("open install of another owner's listed release = %d %s", rec.Code, rec.Body.String())
	}
	var exact map[string]json.RawMessage
	if err := json.Unmarshal(env.guestIn, &exact); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(exact))
	for k := range exact {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "action,record,release_id,tree_patch" {
		t.Fatalf("forwarded body keys = %v", keys)
	}
	// A forged store copy of another owner's release does not verify.
	forged := strings.Replace(string(otherRaw), `"operation_id":"op-1"`, `"operation_id":"op-z"`, 1)
	env.view = func() (int, []byte) {
		out, _ := json.Marshal(map[string]any{"release_id": otherID, "record": json.RawMessage(forged), "listing": map[string]any{"state": state}})
		return http.StatusOK, out
	}
	refused("open install forged", `{"release_id":"`+otherID+`"}`, http.StatusBadGateway, "published_record_digest_mismatch")
	env.guestIn = nil
	if rec := env.send(t, `{"release_id":"`+otherID+`","action":"uninstall"}`, owner); rec.Code != http.StatusCreated || env.guestIn == nil {
		t.Fatalf("cross-owner uninstall = %d %s", rec.Code, rec.Body.String())
	}
	env.view = viewOf

	// P1: installed, with the store's record and verified tree, read as the
	// authenticated owner.
	env.calls = nil
	rec := env.send(t, body, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("install = %d %s", rec.Code, rec.Body.String())
	}
	var forwarded struct {
		ReleaseID string          `json:"release_id"`
		Record    json.RawMessage `json:"record"`
		TreePatch []byte          `json:"tree_patch"`
		Action    string          `json:"action"`
	}
	if err := json.Unmarshal(env.guestIn, &forwarded); err != nil || forwarded.ReleaseID != id || string(forwarded.Record) != string(raw) || string(forwarded.TreePatch) != string(tree) {
		t.Fatalf("P1: guest got %s (%v)", env.guestIn, err)
	}
	calls := strings.Join(env.calls, "\n")
	for _, want := range []string{"corpusd GET /internal/platform/packages/releases/" + id + " owner_id=" + env.user, "/blobs/" + computerevent.DigestBytes(tree) + " owner_id=" + env.user, "guest POST /api/computers/computer-a/self-development/adoptions"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("P1: calls missing %q:\n%s", want, calls)
		}
	}
	// B1 (review): an uninstall reads nothing from the catalog, so a
	// withdrawn release, and one whose object and tree are gone, can always
	// be removed; the computer works from its own install record.
	uninstall := `{"release_id":"` + id + `","action":"uninstall"}`
	for leg, setup := range map[string]func(){
		"listed":    func() {},
		"withdrawn": func() { state = pkgrelease.StateWithdrawn },
		"gone": func() {
			env.view = func() (int, []byte) { return http.StatusNotFound, []byte(`{"error":"package release not published"}`) }
			env.blob = func() (int, []byte) { return http.StatusNotFound, nil }
		},
	} {
		setup()
		env.calls, env.guestIn = nil, nil
		rec := env.send(t, uninstall, owner)
		if rec.Code != http.StatusCreated || string(env.guestIn) != `{"action":"uninstall","release_id":"`+id+`"}` {
			t.Fatalf("uninstall %s = %d %s, guest got %s", leg, rec.Code, rec.Body.String(), env.guestIn)
		}
		for _, call := range env.calls {
			if strings.HasPrefix(call, "corpusd") {
				t.Fatalf("uninstall %s read the catalog: %v", leg, env.calls)
			}
		}
	}
}
