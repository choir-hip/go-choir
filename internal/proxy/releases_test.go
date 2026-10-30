package proxy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/release"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// P7/P8 at the proxy (docs/archive/reports/p6-p8-release-adopt-2026-10-15.md).
// Failure modes pinned:
//   R3/A11 publish and adopt are the owner's acts: an agent key, `admin`
//          included, without owner:acts is refused before any upstream call;
//   R4     the publisher the proxy hands corpusd is the authenticated owner
//          and the computer it owns, never a body field;
//   R1/R2  a draft whose record does not hash to its id, or whose patch does
//          not hash to the record, never reaches corpusd;
//   reads use the read scope and never write.

type releaseProxyEnv struct {
	handler *Handler
	user    string
	secret  func(t *testing.T, scopes ...string) string
	guest   *httptest.Server
	corpusd *httptest.Server
	mu      sync.Mutex
	calls   []string
	draft   func() []byte
	adopted []byte
	store   *auth.Store
}

func (e *releaseProxyEnv) record(call string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, call)
}

func (e *releaseProxyEnv) send(t *testing.T, method, path, body, secret string) *httptest.ResponseRecorder {
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

func newReleaseProxyEnv(t *testing.T) *releaseProxyEnv {
	t.Helper()
	env := &releaseProxyEnv{}
	env.guest = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.record("guest " + r.Method + " " + r.URL.Path + " owner=" + r.Header.Get("X-Authenticated-User") + " computer=" + r.Header.Get("X-Authenticated-Computer"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/release-draft"):
			_, _ = w.Write(env.draft())
		case strings.HasSuffix(r.URL.Path, "/adoptions") && r.Method == http.MethodPost:
			env.adopted, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"adoption_id":"adoption-1"}`))
		default:
			_, _ = w.Write([]byte(`{"adoptions":[]}`))
		}
	}))
	t.Cleanup(env.guest.Close)
	env.corpusd = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		env.record("corpusd " + r.Method + " " + r.URL.Path + " internal=" + r.Header.Get("X-Internal-Caller") + " query=" + r.URL.RawQuery + " " + string(body))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"digest":"x"}`))
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"release_id":"r","published":true}`))
		case r.URL.Path == "/internal/platform/releases":
			_, _ = w.Write([]byte(`{"releases":[],"count":0}`))
		default:
			_, _ = w.Write([]byte(`{"release_id":"r","record":{}}`))
		}
	}))
	t.Cleanup(env.corpusd.Close)
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = env.corpusd.URL
	user, err := store.CreateUser("release-user", "release@example.com")
	if err != nil {
		t.Fatal(err)
	}
	env.user = user.ID
	env.store = store
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": env.guest.URL,
		})
	}))
	t.Cleanup(ownership.Close)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	env.handler = handler
	var n int
	env.secret = func(t *testing.T, scopes ...string) string {
		t.Helper()
		n++
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "release-key-"+strings.Repeat("x", n), scopes, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		return secret
	}
	return env
}

func draftFor(t *testing.T, owner string, patch []byte, mutate func(*release.Record)) []byte {
	t.Helper()
	record := release.Record{
		Schema:    release.Schema,
		Publisher: release.Publisher{ComputerID: "computer-a", OwnerID: owner},
		Operation: release.Operation{
			OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), ReleaseDigest: strings.Repeat("c", 64),
			CodeRef: "code:sha256:" + strings.Repeat("d", 64), DecisionEvent: strings.Repeat("e", 64),
		},
		Source: release.Source{
			BaseCommit: strings.Repeat("1", 40), PatchSHA256: computerevent.DigestBytes(patch), PatchBytes: len(patch),
			TreeHash: strings.Repeat("2", 40), ChangedPaths: []string{"frontend/public/x.txt"},
		},
	}
	id, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(&record)
	}
	raw, err := record.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(map[string]any{
		"release_id": id, "record": json.RawMessage(raw), "patch_sha256": computerevent.DigestBytes(patch),
		"patch_bytes": len(patch), "patch_base64": base64.StdEncoding.EncodeToString(patch),
	})
	return out
}

func TestReleasePublishIsTheOwnersActAndOnlyVerifiedSourceReachesCorpusd(t *testing.T) {
	env := newReleaseProxyEnv(t)
	patch := []byte("diff --git a/frontend/public/x.txt b/frontend/public/x.txt\n+token\n")
	env.draft = func() []byte { return draftFor(t, env.user, patch, nil) }
	const path = "/api/computers/computer-a/releases"
	body := `{"operation_id":"op-1","test_command":"test -f frontend/public/x.txt"}`

	// R3/A11: no owner:acts, admin included.
	for _, scopes := range [][]string{{"computer:self_development:approve"}, {"admin"}} {
		if rec := env.send(t, http.MethodPost, path, body, env.secret(t, scopes...)); rec.Code != http.StatusForbidden || len(env.calls) != 0 {
			t.Fatalf("%v publish = %d, calls %v; want 403 and no upstream call", scopes, rec.Code, env.calls)
		}
	}
	owner := env.secret(t, "computer:self_development:approve", OwnerActsScope)
	// R1: the draft's record does not hash to its id.
	env.draft = func() []byte {
		return draftFor(t, env.user, patch, func(r *release.Record) { r.Declarations.TestCommand = "smuggled" })
	}
	if rec := env.send(t, http.MethodPost, path, body, owner); rec.Code != http.StatusBadGateway || strings.Contains(strings.Join(env.calls, "\n"), "corpusd") {
		t.Fatalf("tampered draft = %d %s, calls %v; want 502 and corpusd untouched", rec.Code, rec.Body.String(), env.calls)
	}
	// R4: the draft names another owner than the one who acts.
	env.calls = nil
	env.draft = func() []byte { return draftFor(t, "someone-else", patch, nil) }
	if rec := env.send(t, http.MethodPost, path, body, owner); rec.Code != http.StatusBadGateway || strings.Contains(strings.Join(env.calls, "\n"), "corpusd") {
		t.Fatalf("foreign-owner draft = %d, calls %v", rec.Code, env.calls)
	}

	env.calls = nil
	env.draft = func() []byte { return draftFor(t, env.user, patch, nil) }
	rec := env.send(t, http.MethodPost, path, body, owner)
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", rec.Code, rec.Body.String())
	}
	calls := strings.Join(env.calls, "\n")
	for _, want := range []string{
		"guest GET /api/computers/computer-a/self-development/operations/op-1/release-draft owner=" + env.user + " computer=computer-a",
		"corpusd PUT /internal/platform/releases/blobs/" + computerevent.DigestBytes(patch) + " internal=true",
		"corpusd POST /internal/platform/releases internal=true",
		`"owner_id":"` + env.user + `"`, `"computer_id":"computer-a"`,
	} {
		if !strings.Contains(calls, want) {
			t.Fatalf("publish calls missing %q:\n%s", want, calls)
		}
	}
}

func TestReleaseAdoptionIsTheOwnersActAndReadsUseTheReadScope(t *testing.T) {
	env := newReleaseProxyEnv(t)
	env.draft = func() []byte { return []byte(`{}`) }
	const adopt = "/api/computers/computer-a/self-development/adoptions"
	if computerID, action, ok := selfDevelopmentGuestRoute(adopt); !ok || computerID != "computer-a" || action != "adoptions" {
		t.Fatalf("classifier = %q %q %v", computerID, action, ok)
	}
	body := `{"release_id":"` + strings.Repeat("a", 64) + `","record":{}}`
	for _, scopes := range [][]string{{"computer:self_development:approve"}, {"admin"}, {"computer:self_development:read"}} {
		if rec := env.send(t, http.MethodPost, adopt, body, env.secret(t, scopes...)); rec.Code != http.StatusForbidden || len(env.calls) != 0 {
			t.Fatalf("%v adopt = %d, calls %v", scopes, rec.Code, env.calls)
		}
	}
	rec := env.send(t, http.MethodPost, adopt, body, env.secret(t, "computer:self_development:approve", OwnerActsScope))
	if rec.Code != http.StatusCreated || string(env.adopted) != body {
		t.Fatalf("owner adopt = %d %s, guest got %s", rec.Code, rec.Body.String(), env.adopted)
	}
	reader := env.secret(t, "computer:self_development:read")
	for _, path := range []string{adopt, adopt + "/adoption-1", "/api/computers/computer-a/self-development/operations/op-1/release-draft"} {
		if rec := env.send(t, http.MethodGet, path, "", reader); rec.Code != http.StatusOK {
			t.Fatalf("read %s = %d %s", path, rec.Code, rec.Body.String())
		}
	}
	if rec := env.send(t, http.MethodGet, "/api/releases?limit=5", "", reader); rec.Code != http.StatusOK || !strings.Contains(strings.Join(env.calls, "\n"), "corpusd GET /internal/platform/releases internal=true") {
		t.Fatalf("release list = %d, calls %v", rec.Code, env.calls)
	}
	if rec := env.send(t, http.MethodGet, "/api/releases/"+strings.Repeat("a", 64), "", reader); rec.Code != http.StatusOK {
		t.Fatalf("release show = %d %s", rec.Code, rec.Body.String())
	}
	if rec := env.send(t, http.MethodPost, "/api/releases", "{}", env.secret(t, "admin")); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("release list write = %d", rec.Code)
	}
}
