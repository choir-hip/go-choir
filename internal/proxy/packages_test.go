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

	"github.com/yusefmosiah/go-choir/internal/auth"
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
	// guestStatus is the guest's status (0 is 200).
	guestStatus int
	// guestMedia answers the guest's .../media/<sha> route by digest;
	// corpusdMedia answers corpusd's release media read (bytes and type).
	guestMedia   map[string][]byte
	corpusdMedia map[string][2]string
	// corpusdBlobs answers corpusd's release blob read (bytes and type).
	corpusdBlobs map[string][2]string
	// answer is corpusd's body for a catalog or release read; "" is the
	// empty catalog.
	answer string
	store  *auth.Store
	// addresses is maild's lookup: owner id -> numbered address. An owner
	// not in it has no number. maildDown makes maild answer 503;
	// maildAsked records each lookup's ids.
	addresses  map[string]string
	maildDown  bool
	maildAsked [][]string
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
	env.guestMedia = map[string][]byte{computerevent.DigestBytes(defaultProxyShot(t)): defaultProxyShot(t)}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.record("guest " + r.Method + " " + r.URL.Path + "?" + r.URL.RawQuery + " owner=" + r.Header.Get("X-Authenticated-User") + " computer=" + r.Header.Get("X-Authenticated-Computer"))
		if _, digest, ok := strings.Cut(r.URL.Path, "/media/"); ok {
			raw, found := env.guestMedia[digest]
			if !found {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write(raw)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if env.guestStatus != 0 {
			w.WriteHeader(env.guestStatus)
		}
		_, _ = w.Write(env.draft(r.URL.RawQuery))
	}))
	t.Cleanup(guest.Close)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		line := "corpusd " + r.Method + " " + r.URL.Path + " internal=" + r.Header.Get("X-Internal-Caller") + " query=" + r.URL.RawQuery
		if r.Method == http.MethodPost || strings.HasPrefix(r.URL.Path, "/internal/platform/listings/") {
			line += " " + string(body)
			env.mu.Lock()
			env.receipt = r.Header.Get(owneract.Header)
			env.mu.Unlock()
		}
		env.record(line)
		if _, digest, ok := strings.Cut(r.URL.Path, "/blobs/"); ok && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/") {
			answer, found := env.corpusdBlobs[digest]
			if !found {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"package blob not found"}`))
				return
			}
			w.Header().Set("Content-Type", answer[1])
			_, _ = w.Write([]byte(answer[0]))
			return
		}
		if _, digest, ok := strings.Cut(r.URL.Path, "/media/"); ok && r.Method == http.MethodGet {
			answer, found := env.corpusdMedia[digest]
			if !found {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"package media not found"}`))
				return
			}
			w.Header().Set("Content-Type", answer[1])
			_, _ = w.Write([]byte(answer[0]))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/internal/platform/listings/"):
			_, _ = w.Write([]byte(`{"release_id":"r","changed":true}`))
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
		case env.answer != "":
			_, _ = w.Write([]byte(env.answer))
		default:
			_, _ = w.Write([]byte(`{"packages":[]}`))
		}
	}))
	t.Cleanup(corpusd.Close)
	maild := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			OwnerIDs []string `json:"owner_ids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if r.URL.Path != "/internal/maild/public-addresses" || r.Method != http.MethodPost || r.Header.Get("X-Internal-Caller") != "true" {
			t.Errorf("maild call %s %s internal=%q", r.Method, r.URL.Path, r.Header.Get("X-Internal-Caller"))
		}
		env.mu.Lock()
		env.maildAsked = append(env.maildAsked, in.OwnerIDs)
		down := env.maildDown
		out := map[string]string{}
		for _, id := range in.OwnerIDs {
			if a, ok := env.addresses[id]; ok {
				out[id] = a
			}
		}
		env.mu.Unlock()
		if down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"addresses": out})
	}))
	t.Cleanup(maild.Close)
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.MaildURL = maild.URL
	handler.cfg.CorpusdURL = corpusd.URL
	user, err := store.CreateUser("package-user", "package@example.com")
	if err != nil {
		t.Fatal(err)
	}
	env.user = user.ID
	env.store = store
	env.addresses = map[string]string{user.ID: "000@choir.example"}
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

// defaultProxyShot is the screenshot every test draft names; the guest
// serves it unless a test replaces guestMedia.
func defaultProxyShot(t *testing.T) []byte { return proxyPNG(t, 4, 4) }

func defaultProxyShotItem(t *testing.T) pkgrelease.Media {
	raw := defaultProxyShot(t)
	return pkgrelease.Media{Kind: pkgrelease.MediaKindScreenshot, Name: "home.png", MediaType: pkgrelease.MediaTypePNG, SHA256: computerevent.DigestBytes(raw), Bytes: len(raw), Width: 4, Height: 4}
}

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
		// Every release carries a screenshot; a test of the gate clears it.
		Media: []pkgrelease.Media{defaultProxyShotItem(t)},
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
	// Review 274 F1: a draft for another operation than the owner named.
	refused("another operation", func(string) []byte {
		return packageDraftFor(t, env.user, func(r *pkgrelease.Record) { r.Operation.OperationID = "op-2" }, false)
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
	rec := env.send(t, http.MethodPost, path, `{"operation_id":"op-1","package":"reader","test_command":"npm test","confirm_paths":"`+strings.Repeat("e", 64)+`"}`, owner)
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
		// The owner's confirmation of what becomes public reaches corpusd.
		`"confirm_paths":"` + strings.Repeat("e", 64) + `"`,
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
	if err != nil || verified.OwnerUserID != env.user || verified.SubjectString("release_id") != draft.ReleaseID || verified.SubjectString("scopes") != pkgrelease.ScopeOwner+","+pkgrelease.ScopePublic {
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

// Withdraw and reinstate at the proxy (the owner's ruling of 2026-11-03:
// a published release is public, and share is no longer an act). Failure
// modes pinned:
//
//	A2   a listing act without owner:acts (admin included) reaches the guest
//	     or corpusd; the receipt does not bind the release and the change;
//	P4   a share (a scope set, named owners or a paths confirmation) reaches
//	     corpusd instead of answering share_retired with the reason; a
//	     listing act still asks the computer's retired packages.share gate;
//	R4   the owner corpusd is told is a body field, not the authenticated
//	     owner; a computer the key does not own names the act;
//	L4   a history read forwards a caller-chosen owner, or drops the track
//	     and the page cursor.
func TestPackageListingActIsTheOwnersActAndShareIsRetired(t *testing.T) {
	env := newPackageProxyEnv(t)
	env.draft = func(string) []byte { return []byte(`{}`) }
	id := strings.Repeat("a", 64)
	path := "/api/catalog/records/" + id + "/listing"
	withdraw := `{"computer_id":"computer-a","state":"withdrawn","reason":"bad build"}`
	reset := func() {
		env.mu.Lock()
		env.calls = nil
		env.receipt = ""
		env.mu.Unlock()
	}

	// A2: no owner:acts, admin included: nothing upstream.
	for _, scopes := range [][]string{{"computer:self_development:approve"}, {"admin"}} {
		if rec := env.send(t, http.MethodPatch, path, withdraw, env.secret(t, scopes...)); rec.Code != http.StatusForbidden || env.joined() != "" {
			t.Fatalf("A2 %v withdraw = %d, calls %v", scopes, rec.Code, env.joined())
		}
	}
	owner := env.secret(t, "computer:self_development:approve", OwnerActsScope)
	// P4: every share shape answers share_retired, and nothing upstream.
	for _, body := range []string{
		`{"computer_id":"computer-a","scopes":["owner","shared"],"shared_with":{"owners":["owner-b"]},"confirm_paths":"` + strings.Repeat("f", 64) + `"}`,
		`{"computer_id":"computer-a","scopes":["owner","public"]}`,
		`{"computer_id":"computer-a","scopes":["owner"]}`,
		`{"computer_id":"computer-a","state":"withdrawn","shared_with":{"owners":["owner-b"]}}`,
	} {
		reset()
		rec := env.send(t, http.MethodPatch, path, body, owner)
		var got struct {
			Error  string `json:"error"`
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if rec.Code != http.StatusGone || got.Reason != pkgrelease.ReasonShareRetired || !strings.Contains(got.Error, "public") || env.joined() != "" {
			t.Fatalf("P4 share %s = %d %s, calls %v; want 410 share_retired and no upstream call", body, rec.Code, rec.Body.String(), env.joined())
		}
	}
	for name, body := range map[string]string{
		"neither":              `{"computer_id":"computer-a"}`,
		"no computer":          `{"state":"withdrawn"}`,
		"an owner in the body": `{"computer_id":"computer-a","state":"withdrawn","owner_id":"evil"}`,
	} {
		reset()
		if rec := env.send(t, http.MethodPatch, path, body, owner); rec.Code != http.StatusBadRequest || env.joined() != "" {
			t.Fatalf("%s = %d %s, calls %v; want 400 and no upstream call", name, rec.Code, rec.Body.String(), env.joined())
		}
	}
	reset()
	if rec := env.send(t, http.MethodPatch, "/api/catalog/records/not-a-sha/listing", withdraw, owner); rec.Code != http.StatusNotFound || env.joined() != "" {
		t.Fatalf("a malformed id = %d, calls %v", rec.Code, env.joined())
	}
	// R4: a computer the key does not own.
	reset()
	if rec := env.send(t, http.MethodPatch, path, strings.Replace(withdraw, "computer-a", "computer-z", 1), owner); rec.Code == http.StatusOK || strings.Contains(env.joined(), "corpusd PUT") {
		t.Fatalf("R4 another computer = %d, calls %v", rec.Code, env.joined())
	}

	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	env.handler.ownerActSigner = signer
	verifier, _ := owneract.NewVerifier(signer.PublicKey())
	for _, c := range []struct {
		body   string
		change platform.ListingChange
	}{
		{withdraw, platform.ListingChange{State: "withdrawn", Reason: "bad build"}},
		{`{"computer_id":"computer-a","state":"listed"}`, platform.ListingChange{State: "listed"}},
	} {
		reset()
		rec := env.send(t, http.MethodPatch, path, c.body, owner)
		if rec.Code != http.StatusOK {
			t.Fatalf("act %s = %d %s", c.body, rec.Code, rec.Body.String())
		}
		calls := env.joined()
		if strings.Contains(calls, "share-gate") || strings.Contains(calls, "guest ") {
			t.Fatalf("P4 the act asked the retired share gate:\n%s", calls)
		}
		for _, want := range []string{
			"corpusd PUT /internal/platform/listings/" + id + " internal=true",
			`"owner_id":"` + env.user + `"`, `"computer_id":"computer-a"`,
		} {
			if !strings.Contains(calls, want) {
				t.Fatalf("act %s calls missing %q:\n%s", c.body, want, calls)
			}
		}
		receipt, err := owneract.Decode(env.receipt)
		if err != nil {
			t.Fatalf("A2 no receipt on the act: %v", err)
		}
		verified, err := verifier.Verify(receipt, "computer-a", owneract.ActPackageListing, time.Now())
		if err != nil || verified.OwnerUserID != env.user {
			t.Fatalf("A2 receipt = %+v %v", verified, err)
		}
		for key, want := range platform.PackageListingSubject(id, c.change) {
			if verified.SubjectString(key) != want {
				t.Fatalf("A2 receipt subject %s = %q, want %q", key, verified.SubjectString(key), want)
			}
		}
	}
}

func TestPackageHistoryForwardsTheTrackAndThePage(t *testing.T) {
	env := newPackageProxyEnv(t)
	reader := env.secret(t, "computer:self_development:read")
	rec := env.send(t, http.MethodGet, "/api/catalog/packages/reader/releases?track=owner-x&limit=2&before_seq=3&owner_id=evil", "", reader)
	calls := env.joined()
	if rec.Code != http.StatusOK || strings.Contains(calls, "evil") {
		t.Fatalf("L4 history = %d, calls %v", rec.Code, calls)
	}
	for _, want := range []string{"owner_id=" + env.user, "track=owner-x", "limit=2", "before_seq=3"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("history calls %v lack %q", calls, want)
		}
	}
}

// The app store shows who made an app by their numbered Choir address, the
// account's real Choir email, never the sign-in email (owner, 2026-11-04).
// The proxy asks maild's host-internal lookup for the owners of the releases
// in the answer. There is no fallback: maild down, or a listed release whose
// publisher has no number, fails the read visibly; and a publisher with no
// number cannot publish.
func TestCatalogAnswersCarryThePublishersNumberedAddressAndNeverFallBack(t *testing.T) {
	env := newPackageProxyEnv(t)
	reader := env.secret(t, "computer:self_development:read")
	maker, err := env.store.CreateUser("maker", "maker@example.com")
	if err != nil {
		t.Fatal(err)
	}
	env.addresses[maker.ID] = "007@choir.example"
	env.addresses["tester"] = "10000042@choir.example"
	calls := func() int {
		env.mu.Lock()
		defer env.mu.Unlock()
		return len(env.maildAsked)
	}
	noEmail := func(label string, rec *httptest.ResponseRecorder) {
		t.Helper()
		body := rec.Body.String()
		if strings.Contains(body, "@example.com") || strings.Contains(body, "publisher_email") || strings.Contains(body, "maker@") || strings.Contains(body, "package@") {
			t.Fatalf("%s shows a real sign-in email: %s", label, body)
		}
	}
	id := strings.Repeat("a", 64)
	release := func(owner string) string {
		return `{"release_id":"` + id + `","record":{"publisher":{"owner_id":"` + owner + `","computer_id":"computer-x"}},"media":[]}`
	}

	// The shelf: each track in the answer, test accounts included.
	env.answer = `{"count":3,"reader":"` + env.user + `","packages":[{"package":"reader","track":"` + maker.ID + `","releases":1},{"package":"reader","track":"` + env.user + `","releases":1},{"package":"t","track":"tester","releases":1}]}`
	rec := env.send(t, http.MethodGet, "/api/catalog/packages", "", reader)
	var catalog struct {
		Packages []struct {
			PublisherAddress string `json:"publisher_address"`
			Own              bool   `json:"own"`
		} `json:"packages"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &catalog) != nil || len(catalog.Packages) != 3 {
		t.Fatalf("catalog = %d %s", rec.Code, rec.Body.String())
	}
	noEmail("catalog", rec)
	for i, want := range []struct {
		addr string
		own  bool
	}{{"007@choir.example", false}, {"000@choir.example", true}, {"10000042@choir.example", false}} {
		if got := catalog.Packages[i]; got.PublisherAddress != want.addr || got.Own != want.own {
			t.Fatalf("catalog entry %d = %+v, want %+v", i, got, want)
		}
	}
	if calls() != 1 {
		t.Fatalf("maild asked %v, want one call", env.maildAsked)
	}
	env.send(t, http.MethodGet, "/api/catalog/packages", "", reader)
	if calls() != 1 {
		t.Fatalf("the cache did not answer a repeat: %v", env.maildAsked)
	}

	// A release, and a history of releases.
	env.answer = release(maker.ID)
	rec = env.send(t, http.MethodGet, "/api/catalog/releases/"+id, "", reader)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"publisher_address":"007@choir.example"`) || !strings.Contains(rec.Body.String(), `"own":false`) {
		t.Fatalf("release = %d %s", rec.Code, rec.Body.String())
	}
	noEmail("release", rec)
	env.answer = `{"package":"reader","track":"` + maker.ID + `","count":1,"releases":[{"release_id":"` + id + `","record":{"publisher":{"owner_id":"` + maker.ID + `"}}}]}`
	rec = env.send(t, http.MethodGet, "/api/catalog/packages/reader/releases?track="+maker.ID, "", reader)
	if rec.Code != http.StatusOK || strings.Count(rec.Body.String(), `"publisher_address":"007@choir.example"`) != 1 {
		t.Fatalf("history = %d %s", rec.Code, rec.Body.String())
	}
	noEmail("history", rec)

	// B1: a track named in the request with no visible release resolves
	// nothing and asks maild nothing.
	before := calls()
	for _, track := range []string{maker.ID, env.user, "anyone-at-all"} {
		env.answer = `{"package":"reader","track":"` + track + `","releases":[],"count":0}`
		rec = env.send(t, http.MethodGet, "/api/catalog/packages/anything/releases?track="+track, "", reader)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "publisher_address") {
			t.Fatalf("B1 an empty history for track %s resolves an address: %d %s", track, rec.Code, rec.Body.String())
		}
	}
	if calls() != before {
		t.Fatalf("B1 maild was asked about a request parameter: %v", env.maildAsked[before:])
	}

	// No fallback (1): a listed release whose publisher has no number is a
	// defect, named in a visible error, never an omitted field.
	env.answer = release("numberless-owner")
	rec = env.send(t, http.MethodGet, "/api/catalog/releases/"+id, "", reader)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "publisher_address_missing") || !strings.Contains(rec.Body.String(), "numberless-owner") || strings.Contains(rec.Body.String(), "publisher_address\":") {
		t.Fatalf("a numberless publisher = %d %s, want 502 publisher_address_missing naming the owner", rec.Code, rec.Body.String())
	}
	// No fallback (2): maild down fails the read, never drops the field.
	env.mu.Lock()
	env.maildDown = true
	env.mu.Unlock()
	env.handler.publisherAddresses = publisherAddressCache{}
	env.answer = release(maker.ID)
	rec = env.send(t, http.MethodGet, "/api/catalog/releases/"+id, "", reader)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "publisher_address_unavailable") {
		t.Fatalf("maild down = %d %s, want 502 publisher_address_unavailable", rec.Code, rec.Body.String())
	}
	env.answer = ""
}

// Publishing requires the publisher's account to have a numbered address, and
// the address service to answer: a release is listed with its maker shown by
// that address.
func TestPublishNeedsThePublishersNumberedAddress(t *testing.T) {
	env := newPackageProxyEnv(t)
	owner := env.secret(t, "computer:self_development:approve", OwnerActsScope)
	path := "/api/computers/computer-a/packages/releases"
	body := `{"operation_id":"op-1","package":"reader"}`
	env.draft = func(string) []byte { return packageDraftFor(t, env.user, nil, false) }

	env.mu.Lock()
	delete(env.addresses, env.user)
	env.mu.Unlock()
	env.handler.publisherAddresses = publisherAddressCache{}
	rec := env.send(t, http.MethodPost, path, body, owner)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "publisher_address_missing") || strings.Contains(env.joined(), "guest GET") || strings.Contains(env.joined(), "corpusd POST") {
		t.Fatalf("publish without a number = %d %s, calls:\n%s", rec.Code, rec.Body.String(), env.joined())
	}

	env.mu.Lock()
	env.addresses[env.user] = "000@choir.example"
	env.maildDown = true
	env.mu.Unlock()
	rec = env.send(t, http.MethodPost, path, body, owner)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "publisher_address_unavailable") || strings.Contains(env.joined(), "corpusd POST") {
		t.Fatalf("publish with maild down = %d %s", rec.Code, rec.Body.String())
	}
}
