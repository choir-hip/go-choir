package proxy

// The public read of one entity (platform_entity_page.go). Ways it can fail,
// written before the code: it needs a session or calls a computer; a
// malformed id reaches corpusd; the window of stories is dropped or an
// unbounded one forwarded as is; corpusd's 404 reaches the reader as a
// failure or its error text leaks; a POST is served.

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestPlatformEntityPageIsPublicChecksTheIdAndMapsCorpusdsAnswers(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("an entity read reached a computer: %s %s", r.Method, r.URL.Path)
	}))
	defer guest.Close()
	known := types.MintedEntityID("Halvorsen Dynamics")
	var paths []string
	var gotInternal, gotQuery string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		gotInternal, gotQuery = r.Header.Get("X-Internal-Caller"), r.URL.RawQuery
		switch r.URL.Path {
		case "/internal/platform/entities/" + known:
			_ = json.NewEncoder(w).Encode(platform.EntityPage{EntityID: known, Name: "Halvorsen Dynamics", Type: "organization", Mentions: 2, StoryList: []platform.EntityPageStory{}, CoEntities: []types.EntityRef{}})
		case "/internal/platform/entities/" + types.MintedEntityID("Nobody"):
			http.Error(w, `{"error":"secret detail"}`, http.StatusNotFound)
		default:
			http.Error(w, `{"error":"secret detail"}`, http.StatusInternalServerError)
		}
	}))
	defer corpusd.Close()
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: guest.URL,
		AuthPublicKeyPath: "/unused/in/test", CorpusdURL: corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	do := func(method, target string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.HandleAPI(w, httptest.NewRequest(method, target, nil))
		return w
	}
	w := do(http.MethodGet, types.EntityPagePrefix+known+"?stories=7")
	var got platform.EntityPage
	if w.Code != http.StatusOK || gotInternal != "true" || gotQuery != "stories=7" || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.Mentions != 2 {
		t.Fatalf("status %d internal %q query %q body %s", w.Code, gotInternal, gotQuery, w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=60" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	// The growth read still answers at its own path.
	n := len(paths)
	do(http.MethodGet, types.EntityGrowthPath)
	if paths[len(paths)-1] != "/internal/platform/entities/growth" || len(paths) != n+1 {
		t.Fatalf("growth path = %v", paths)
	}
	// A malformed id never reaches corpusd.
	n = len(paths)
	for _, bad := range []string{"ent:zz", "ent:../x", "wd:Q0", "x"} {
		if w := do(http.MethodGet, types.EntityPagePrefix+bad); w.Code != http.StatusNotFound {
			t.Fatalf("%s = %d", bad, w.Code)
		}
	}
	if len(paths) != n {
		t.Fatalf("a malformed id reached corpusd: %v", paths[n:])
	}
	if w := do(http.MethodGet, types.EntityPagePrefix+types.MintedEntityID("Nobody")); w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("corpusd 404 = %d %s", w.Code, w.Body.String())
	}
	if w := do(http.MethodPost, types.EntityPagePrefix+known); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", w.Code)
	}
	if w := do(http.MethodGet, types.EntityPagePrefix+types.MintedEntityID("Else")); w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("corpusd failure = %d %s", w.Code, w.Body.String())
	}
}
