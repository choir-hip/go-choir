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

// The Synthetic provider end to end through the gateway handler, against a
// fake upstream: the catalogued 1017 model routes to it and its answer comes
// back; a model outside the catalog is refused before any upstream call; an
// upstream 429 (Synthetic's concurrency limit) reaches the caller as a
// sanitized status 429 error, the form the run's own failure handling reads.
func TestSyntheticRoutesThroughTheGateway(t *testing.T) {
	const model = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"
	var upstream int
	status := http.StatusOK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = fmt.Fprint(w, `{"error":"Too many concurrent requests"}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":"c1","model":%q,"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}],"usage":{"prompt_tokens":10,"completion_tokens":2}}`, model)
	}))
	defer server.Close()
	syn, err := provider.NewSyntheticProvider(provider.SyntheticConfig{APIKey: "syn-test-key", ModelID: model, BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	mp := provider.NewMultiProvider()
	mp.Register("synthetic", syn)
	reg := NewIdentityRegistry(time.Hour)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-synthetic")

	post := func(model string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(ProviderRequest{
			Provider: "synthetic",
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
	before := upstream
	if w := post("hf:deepseek-ai/DeepSeek-Nope"); w.Code == http.StatusOK || upstream != before {
		t.Fatalf("uncatalogued model: %d, upstream calls %d -> %d", w.Code, before, upstream)
	}
	status = http.StatusTooManyRequests
	w := post(model)
	if w.Code == http.StatusOK || !strings.Contains(w.Body.String(), "status 429") || strings.Contains(w.Body.String(), "concurrent") {
		t.Fatalf("upstream 429: %d %s", w.Code, w.Body.String())
	}
}
