package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Prompt-cache measurement through the real store: a tool-loop turn's
// cached-input count survives persistence and /api/costs' extraction reads
// it back from the stored row, priced at the cached rate. Failure modes: the
// emitter rewrites the payload and drops the field; a stored row from before
// the field existed is counted as a reported miss.
func TestRunEventEmitterKeepsCachedInputTokens(t *testing.T) {
	rt, s := testRuntime(t)
	rec := &types.RunRecord{RunID: "run-cache", AgentID: "research:cache", OwnerID: "owner-1"}
	emit := rt.runEventEmitter(context.Background(), rec)

	hit, _ := json.Marshal(map[string]any{"iteration": 2, "llm_model": "deepseek-v4.1-flash", "llm_provider": "opencode-go",
		"input_tokens": 10000, "output_tokens": 100, "cached_input_tokens": 9000, "cache_reported": true})
	emit(types.EventRunProgress, "tool_loop", hit)
	legacy, _ := json.Marshal(map[string]any{"iteration": 1, "llm_model": "deepseek-v4.1-flash", "llm_provider": "opencode-go",
		"input_tokens": 10000, "output_tokens": 100})
	emit(types.EventRunProgress, "tool_loop", legacy)

	evs, err := s.ListEvents(context.Background(), rec.RunID, 10)
	if err != nil || len(evs) != 2 {
		t.Fatalf("list events: %v (%d)", err, len(evs))
	}
	summary := llmcost.Aggregate(llmcost.ExtractCostEntries(evs))
	if summary.CallCount != 2 || summary.TotalCachedInputTokens != 9000 {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.CacheReportedCallCount != 1 || summary.CacheHitRate != 0.9 {
		t.Fatalf("hit rate = %v over %d reported calls, want 0.9 over 1", summary.CacheHitRate, summary.CacheReportedCallCount)
	}
	uncached := llmcost.EstimateCall("deepseek-v4.1-flash", 20000, 200).USD
	if summary.TotalCost >= uncached {
		t.Fatalf("cached run cost %v should be below the all-uncached %v", summary.TotalCost, uncached)
	}
}
