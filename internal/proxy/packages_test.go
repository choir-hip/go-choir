package proxy

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Library S2 at the proxy (docs/design/library-package-propagation-2026-10-26.md
// 4.1, 14), written before the code. Failure modes pinned:
//
//	L6   publish without the owner's act (an agent key, admin included) reaches
//	     the guest or corpusd; a desk's guest files a package object through
//	     the generic routes; the receipt does not bind the release;
//	R4   the publisher or track corpusd is handed is a body field or the
//	     guest's say, not the authenticated owner and the computer it owns;
//	R1   a draft whose record does not hash to its id, or whose patches do
//	     not hash to the record, reaches corpusd;
//	L4   a catalog read names an owner the caller chose (the proxy must name
//	     the authenticated owner and only it).

type packageProxyEnv struct {
	handler *Handler
	user    string
	secret  func(t *testing.T, scopes ...string) string
	mu      sync.Mutex
	calls   []string
	draft   func(query string) []byte
	head    string // corpusd's track head answer; "" is 404
	receipt string // the owner-act header corpusd saw on the publish
}

func (e *packageProxyEnv) record(call string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, call)
}

func (e *packageProxyEnv) joined() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Join(e.calls, "\n")
}

func (e *packageProxyEnv) send(t *testing.T, method, path, body, secret string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+secret)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	e.handler.HandleAPI(response, request)
	return response
}

func newPackageProxyEnv(t *testing.T) *packageProxyEnv {
	t.Helper()
	env := &packageProxyEnv{}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.record("guest " + r.Method + " " + r.URL.Path + "?" + r.URL.RawQuery + " owner=" + r.Header.Get("X-Authenticated-User") + " computer=" + r.Header.Get("X-Authenticated-Computer"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(env.draft(r.URL.RawQuery))
	}))
	t.Cleanup(guest.Close)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		line := "corpusd " + r.Method + " " + r.URL.Path + " internal=" + r.Header.Get("X-Internal-Caller") + " query=" + r.URL.RawQuery
		if r.Method == http.MethodPost {
			line += " " + string(body)
			env.mu.Lock()
			env.receipt = r.Header.Get(owneract.Header)
			env.mu.Unlock()
		}
		env.record(line)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"digest":"x"}`))
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"release_id":"r","published":true}`))
		case strings.HasSuffix(r.URL.Path, "/head"):
			if env.head == "" {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"no head","reason":"no_track_head"}`))
				return
			}
			_, _ = w.Write([]byte(env.head))
		default:
			_, _ = w.Write([]byte(`{"packages":[]}`))
		}
	}))
	t.Cleanup(corpusd.Close)
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	user, err := store.CreateUser("package-user", "package@example.com")
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
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "package-key-"+strings.Repeat("x", n), scopes, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		return secret
	}
	return env
}

var (
	proxyTreePatch   = []byte("diff --git a/frontend/src/packages/reader/App.svelte b/frontend/src/packages/reader/App.svelte\nnew\n")
	proxyReviewPatch = []byte("diff --git a/frontend/src/packages/reader/App.svelte b/frontend/src/packages/reader/App.svelte\nreview\n")
	proxyParentPatch = []byte("diff --git a/frontend/src/packages/reader/App.svelte b/frontend/src/packages/reader/App.svelte\nparent\n")
)

func blobRef(raw []byte) pkgrelease.Blob {
	return pkgrelease.Blob{SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
}

// packageDraftFor is the guest's draft for owner: a well-formed record on
// the base with its three patches. mutate runs after the id is taken (to
// tamper) when after is true, before it otherwise.
func packageDraftFor(t *testing.T, owner string, mutate func(*pkgrelease.Record), after bool, blobs ...[]byte) []byte {
	t.Helper()
	tree, review, parent := proxyTreePatch, proxyReviewPatch, proxyParentPatch
	if len(blobs) == 3 {
		tree, review, parent = blobs[0], blobs[1], blobs[2]
	}
	parentRef := blobRef(parent)
	record := pkgrelease.Record{
		Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: "reader", ManifestVersion: "1.0.0"}, Track: owner,
		Parent:      pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: strings.Repeat("1", 40), Tree: strings.Repeat("2", 40)},
		ParentPatch: &parentRef,
		Source: pkgrelease.Source{
			Root: "frontend/src/packages/reader", Tree: strings.Repeat("3", 40), TreePatch: blobRef(proxyTreePatch), ReviewPatch: blobRef(proxyReviewPatch),
			ChangedPaths: []string{"frontend/src/packages/reader/App.svelte"}, BuiltOn: strings.Repeat("1", 40),
		},
		Manifest:  pkgrelease.Manifest{SHA256: strings.Repeat("4", 64), ABI: "choir.app-frontend/svelte-inline/v1"},
		Publisher: pkgrelease.Publisher{OwnerID: owner, ComputerID: "computer-a"},
		Operation: pkgrelease.Operation{OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	if mutate != nil && !after {
		mutate(&record)
	}
	id, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil && after {
		mutate(&record)
	}
	raw, err := record.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(map[string]any{
		"release_id": id, "record": json.RawMessage(raw),
		"tree_patch_base64":   base64.StdEncoding.EncodeToString(tree),
		"review_patch_base64": base64.StdEncoding.EncodeToString(review),
		"parent_patch_base64": base64.StdEncoding.EncodeToString(parent),
	})
	return out
}

func TestPackagePublishIsTheOwnersActAndOnlyAVerifiedDraftReachesCorpusd(t *testing.T) {
	env := newPackageProxyEnv(t)
	env.draft = func(string) []byte { return packageDraftFor(t, env.user, nil, false) }
	const path = "/api/computers/computer-a/packages/releases"
	body := `{"operation_id":"op-1","package":"reader"}`

	// L6: no owner:acts, admin included: nothing upstream is called.
	for _, scopes := range [][]string{{"computer:self_development:approve"}, {"admin"}} {
		if rec := env.send(t, http.MethodPost, path, body, env.secret(t, scopes...)); rec.Code != http.StatusForbidden || env.joined() != "" {
			t.Fatalf("%v publish = %d, calls %v; want 403 and no upstream call", scopes, rec.Code, env.joined())
		}
	}
	owner := env.secret(t, "computer:self_development:approve", OwnerActsScope)
	if rec := env.send(t, http.MethodPost, path, `{"operation_id":"op-1","package":"../x"}`, owner); rec.Code != http.StatusBadRequest {
		t.Fatalf("a package that is not a name = %d", rec.Code)
	}

	refused := func(name string, draft func(string) []byte) {
		t.Helper()
		env.mu.Lock()
		env.calls = nil
		env.mu.Unlock()
		env.draft = draft
		rec := env.send(t, http.MethodPost, path, body, owner)
		if rec.Code != http.StatusBadGateway || strings.Contains(env.joined(), "corpusd PUT") || strings.Contains(env.joined(), "corpusd POST") {
			t.Fatalf("%s = %d %s, calls %v; want 502 and no write to corpusd", name, rec.Code, rec.Body.String(), env.joined())
		}
	}
	// R1: the record does not hash to its id; a patch does not hash to the
	// record.
	refused("tampered record", func(string) []byte {
		return packageDraftFor(t, env.user, func(r *pkgrelease.Record) { r.Declarations.TestCommand = "smuggled" }, true)
	})
	refused("tampered patch", func(string) []byte {
		return packageDraftFor(t, env.user, nil, false, []byte("other bytes"), proxyReviewPatch, proxyParentPatch)
	})
	// R4: the draft names another owner, another computer, another track.
	refused("foreign owner", func(string) []byte { return packageDraftFor(t, "someone-else", nil, false) })
	refused("foreign computer", func(string) []byte {
		return packageDraftFor(t, env.user, func(r *pkgrelease.Record) { r.Publisher.ComputerID = "computer-z" }, false)
	})
	refused("another package", func(string) []byte {
		return packageDraftFor(t, env.user, func(r *pkgrelease.Record) {
			r.Package.Name, r.Source.Root, r.Source.ChangedPaths = "image", "frontend/src/packages/image", []string{"frontend/src/packages/image/App.svelte"}
		}, false)
	})

	// The owner's act: the head is read for the authenticated owner, the
	// guest drafts on it, the patches and then the record go to corpusd with
	// the publisher the proxy authorized and a receipt binding the release.
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	env.handler.ownerActSigner = signer
	env.mu.Lock()
	env.calls = nil
	env.mu.Unlock()
	headID, headTree := strings.Repeat("c", 64), strings.Repeat("5", 40)
	env.head = `{"release_id":"` + headID + `","tree":"` + headTree + `","seq":1}`
	var draftQuery string
	env.draft = func(query string) []byte { draftQuery = query; return packageDraftFor(t, env.user, nil, false) }
	rec := env.send(t, http.MethodPost, path, `{"operation_id":"op-1","package":"reader","test_command":"npm test"}`, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", rec.Code, rec.Body.String())
	}
	calls := env.joined()
	for _, want := range []string{
		"corpusd GET /internal/platform/packages/tracks/reader/head internal=true query=owner_id=" + env.user,
		"guest GET /api/computers/computer-a/packages/operations/op-1/release-draft?",
		"owner=" + env.user + " computer=computer-a",
		"corpusd PUT /internal/platform/packages/blobs/" + computerevent.DigestBytes(proxyTreePatch) + " internal=true",
		"corpusd PUT /internal/platform/packages/blobs/" + computerevent.DigestBytes(proxyReviewPatch) + " internal=true",
		"corpusd PUT /internal/platform/packages/blobs/" + computerevent.DigestBytes(proxyParentPatch) + " internal=true",
		"corpusd POST /internal/platform/packages/releases internal=true",
		`"owner_id":"` + env.user + `"`, `"computer_id":"computer-a"`,
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("publish calls missing %q:\n%s", want, calls)
		}
	}
	for _, want := range []string{"package=reader", "head_release_id=" + headID, "head_tree=" + headTree, "test_command=npm+test"} {
		if !strings.Contains(draftQuery, want) {
			t.Fatalf("the draft query %q lacks %q", draftQuery, want)
		}
	}
	receipt, err := owneract.Decode(env.receipt)
	if err != nil {
		t.Fatalf("L6 no owner-act receipt on the publish: %v", err)
	}
	verifier, _ := owneract.NewVerifier(signer.PublicKey())
	verified, err := verifier.Verify(receipt, "computer-a", owneract.ActPackagePublish, time.Now())
	var draft struct {
		ReleaseID string `json:"release_id"`
	}
	_ = json.Unmarshal(packageDraftFor(t, env.user, nil, false), &draft)
	if err != nil || verified.OwnerUserID != env.user || verified.SubjectString("release_id") != draft.ReleaseID || verified.SubjectString("scopes") != pkgrelease.ScopeOwner {
		t.Fatalf("L6 receipt = %+v %v", verified, err)
	}
}

func TestPackageCatalogReadsNameTheAuthenticatedOwner(t *testing.T) {
	env := newPackageProxyEnv(t)
	reader := env.secret(t, "computer:self_development:read")
	id := strings.Repeat("a", 64)
	for path, want := range map[string]string{
		"/api/catalog/packages?owner_id=evil&q=read":     "corpusd GET /internal/platform/packages/catalog internal=true query=owner_id=" + env.user + "&q=read",
		"/api/catalog/packages/reader/releases":          "corpusd GET /internal/platform/packages/catalog/reader/releases internal=true query=owner_id=" + env.user,
		"/api/catalog/releases/" + id + "?owner_id=evil": "corpusd GET /internal/platform/packages/releases/" + id + " internal=true query=owner_id=" + env.user,
	} {
		env.mu.Lock()
		env.calls = nil
		env.mu.Unlock()
		if rec := env.send(t, http.MethodGet, path, "", reader); rec.Code != http.StatusOK || !strings.Contains(env.joined(), want) || strings.Contains(env.joined(), "evil") {
			t.Fatalf("L4 %s = %d, calls %v; want %q", path, rec.Code, env.joined(), want)
		}
	}
	if rec := env.send(t, http.MethodGet, "/api/catalog/packages", "", env.secret(t, "computer:self_development:approve")); rec.Code != http.StatusForbidden {
		t.Fatalf("a catalog read without the read scope = %d", rec.Code)
	}
	if rec := env.send(t, http.MethodGet, "/api/catalog/releases/not-a-sha", "", reader); rec.Code != http.StatusNotFound {
		t.Fatalf("a malformed release id = %d", rec.Code)
	}
	if rec := env.send(t, http.MethodPost, "/api/catalog/packages", "{}", reader); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("a catalog write = %d", rec.Code)
	}
}

// L6: a bound guest cannot file either package kind through the proxy's
// generic object relay.
func TestGuestCannotFilePackageObjectsThroughTheProxy(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	og := platform.NewObjectGraphHandler(svc, mem)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/platform/objects", og.HandleObjects)
	corpusd := httptest.NewServer(mux)
	defer corpusd.Close()
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(vmctl.GuestOwnership{Found: true, UserID: "owner-a", ComputerID: "computer-a1"})
	}))
	defer vmctlServer.Close()
	handler, _, _ := testProxyEnv(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)
	for _, kind := range []objectgraph.ObjectKind{platform.PackageReleaseKind, platform.ReleaseListingKind} {
		raw, _ := json.Marshal(map[string]any{"kind": kind, "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "k", "metadata": map[string]any{}})
		req := httptest.NewRequest(http.MethodPost, "/internal/platform/objects", bytes.NewReader(raw))
		req.RemoteAddr = "10.200.1.2:40000"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("guest created a %s object through the proxy: %d %s", kind, rec.Code, rec.Body)
		}
	}
}
