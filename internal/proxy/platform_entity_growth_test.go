package proxy

// The public entity growth read (docs/problems/related-stories-no-entity-
// layer-2026-10-26.md, slice c0 §7). Ways it can fail, written before the
// code: it needs a session or calls a computer; it forwards without the
// internal marker or drops the window; a POST is served; a corpusd failure
// reaches the reader as corpusd's error text.

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestPlatformEntityGrowthIsPublicAndForwardsTheWindow(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("growth reached a computer: %s %s", r.Method, r.URL.Path)
	}))
	defer guest.Close()
	var gotInternal, gotQuery string
	fail := false
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/entities/growth" {
			t.Fatalf("corpusd path: %s", r.URL.Path)
		}
		if fail {
			http.Error(w, `{"error":"secret detail"}`, http.StatusInternalServerError)
			return
		}
		gotInternal, gotQuery = r.Header.Get("X-Internal-Caller"), r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(types.EntityGrowthResponse{Days: []types.EntityGrowthDay{{Day: "2026-10-26", StoriesReviewed: 3}}, Stoplist: []string{}})
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
	w := do(http.MethodGet, types.EntityGrowthPath+"?days=7")
	if w.Code != http.StatusOK || gotInternal != "true" || gotQuery != "days=7" {
		t.Fatalf("status %d internal %q query %q body %s", w.Code, gotInternal, gotQuery, w.Body.String())
	}
	var got types.EntityGrowthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got.Days) != 1 || got.Days[0].StoriesReviewed != 3 {
		t.Fatalf("body %s", w.Body.String())
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	if w := do(http.MethodPost, types.EntityGrowthPath); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d", w.Code)
	}
	fail = true
	if w := do(http.MethodGet, types.EntityGrowthPath); w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("corpusd failure = %d %s", w.Code, w.Body.String())
	}
}
