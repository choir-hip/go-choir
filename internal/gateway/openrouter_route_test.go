package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// The OpenRouter provider end to end through the gateway handler, against a
// fake upstream: the catalogued crouton model routes to it with the
// gateway's key and its answer comes back; a model outside the catalog is
// refused before any upstream call; a 402 reaches the caller sanitized.
func TestOpenRouterRoutesThroughTheGateway(t *testing.T) {
	const model = "nvidia/nemotron-3.5-super-vl:free"
	var upstream int
	var gotAuth, gotPath string
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream++
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = fmt.Fprint(w, `{"error":{"message":"Insufficient credits","code":402}}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":"gen-1","model":%q,"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`, model)
	}))
	defer server.Close()
	or, err := provider.NewOpenRouterProvider(provider.OpenRouterConfig{APIKey: "or-test-key", ModelID: model, BaseURL: server.URL + "/api/v1"})
	if err != nil {
		t.Fatal(err)
	}
	mp := provider.NewMultiProvider()
	mp.Register("openrouter", or)
	reg := NewIdentityRegistry(time.Hour)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-openrouter")

	post := func(model string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(ProviderRequest{
			Provider: "openrouter",
			Model:    model,
			Messages: []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "Hi"}}}},
		})
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(string(body)))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.HandleInference(w, req)
		return w
	}

	if w := post(model); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("catalogued model: %d %s", w.Code, w.Body.String())
	}
	if gotAuth != "Bearer or-test-key" || gotPath != "/api/v1/chat/completions" {
		t.Fatalf("upstream auth %q path %q", gotAuth, gotPath)
	}
	before := upstream
	if w := post("stealth/not-a-model"); w.Code == http.StatusOK || upstream != before {
		t.Fatalf("uncatalogued model: %d, upstream calls %d -> %d", w.Code, before, upstream)
	}
	status = http.StatusPaymentRequired
	w := post(model)
	if w.Code == http.StatusOK || !strings.Contains(w.Body.String(), "status 402") || strings.Contains(w.Body.String(), "Insufficient") || strings.Contains(w.Body.String(), "or-test-key") {
		t.Fatalf("upstream 402: %d %s", w.Code, w.Body.String())
	}
}
