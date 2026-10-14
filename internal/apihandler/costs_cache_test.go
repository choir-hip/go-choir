package apihandler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// GET /api/costs shows prompt-cache hits: total cached input tokens, the
// token-weighted hit rate over calls that reported caching, savings, and the
// same per model. Failure modes: the JSON omits the fields; the hit rate is
// diluted by calls recorded before the field existed.
func TestHandleCostsReportsCachedInputTokens(t *testing.T) {
	t.Parallel()
	s, handler := testCostsSetup(t)
	ctx := context.Background()
	for i, p := range []map[string]any{
		{"phase": "tool_loop", "llm_model": "deepseek-v4.1-flash", "llm_provider": "opencode-go", "input_tokens": 8000, "output_tokens": 50, "cached_input_tokens": 6000, "cache_reported": true},
		{"phase": "tool_loop", "llm_model": "deepseek-v4.1-flash", "llm_provider": "opencode-go", "input_tokens": 2000, "output_tokens": 50, "cached_input_tokens": 0, "cache_reported": true},
		{"phase": "tool_loop", "llm_model": "deepseek-v4.1-flash", "llm_provider": "opencode-go", "input_tokens": 5000, "output_tokens": 50},
	} {
		raw, _ := json.Marshal(p)
		ev := types.EventRecord{EventID: "cache-ev-" + string(rune('a'+i)), RunID: "cache-run", OwnerID: "user-alice",
			Kind: types.EventRunProgress, Phase: "tool_loop", Payload: raw, Timestamp: time.Now().UTC()}
		if err := s.AppendEvent(ctx, &ev); err != nil {
			t.Fatalf("append: %v", err)
		}
	}

	w := httptest.NewRecorder()
	handler.HandleCosts(w, authenticatedCostsRequest(http.MethodGet, "/api/costs", "user-alice"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	summary, _ := raw["summary"].(map[string]any)
	if summary["total_cached_input_tokens"] != float64(6000) {
		t.Fatalf("total_cached_input_tokens = %v; summary = %v", summary["total_cached_input_tokens"], summary)
	}
	if summary["cache_hit_rate"] != 0.6 || summary["cache_reported_call_count"] != float64(2) {
		t.Fatalf("cache_hit_rate = %v over %v calls, want 0.6 over 2", summary["cache_hit_rate"], summary["cache_reported_call_count"])
	}
	byModel, _ := summary["by_model"].(map[string]any)
	flash, _ := byModel["deepseek-v4.1-flash"].(map[string]any)
	if flash["total_cached_input_tokens"] != float64(6000) {
		t.Fatalf("by_model flash = %v", flash)
	}
}
