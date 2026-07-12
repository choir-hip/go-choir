package proxy

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestUniversalWireStoriesReadsCorpusdWithoutVMLifecycle(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var sandboxCalls atomic.Int32
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sandboxCalls.Add(1)
		http.Error(w, "sandbox must not be called", http.StatusInternalServerError)
	}))
	defer sandbox.Close()
	var vmctlCalls atomic.Int32
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vmctlCalls.Add(1)
		http.Error(w, "vmctl must not be called", http.StatusInternalServerError)
	}))
	defer vmctlServer.Close()

	want := platform.UniversalWireStoriesResponse{
		Stories: []types.WireStory{
			{ID: "new", Headline: "Newest", Related: []string{}, Manifest: types.WireSourceManifest{}, Claims: []string{}, Projections: map[string]string{"wire-style": "new"}, SourceState: "corpusd-publication-index"},
			{ID: "old", Headline: "Older", Related: []string{}, Manifest: types.WireSourceManifest{}, Claims: []string{}, Projections: map[string]string{"wire-style": "old"}, SourceState: "corpusd-publication-index"},
		},
		StyleSources: []types.WireStyleSource{},
		Source:       "corpusd-publications",
	}
	var corpusdCalls atomic.Int32
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpusdCalls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/internal/platform/universal-wire/stories" {
			t.Fatalf("corpusd request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatal("corpusd request missing internal caller header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer corpusd.Close()

	handler, err := NewHandler(&Config{SandboxURL: sandbox.URL, CorpusdURL: corpusd.URL, VmctlURL: vmctlServer.URL}, pub)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	req := httptest.NewRequest(http.MethodGet, "/api/universal-wire/stories", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "reader-1")})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	var got platform.UniversalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Stories) != 2 || got.Stories[0].ID != "new" || got.Stories[1].ID != "old" {
		t.Fatalf("stories = %+v, want corpusd order preserved", got.Stories)
	}
	if got.Source != want.Source || got.StyleSources == nil {
		t.Fatalf("response shape = %+v", got)
	}
	if corpusdCalls.Load() != 1 {
		t.Fatalf("corpusd calls = %d, want 1", corpusdCalls.Load())
	}
	if vmctlCalls.Load() != 0 || sandboxCalls.Load() != 0 {
		t.Fatalf("vmctl calls = %d sandbox calls = %d, want zero", vmctlCalls.Load(), sandboxCalls.Load())
	}
}
