package toolregistry

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Prompt-cache measurement: each tool-loop iteration's loop.progress event
// carries the provider's cached-input count and whether it was reported, and
// the run total accumulates it. Failure modes: the progress payload omits the
// field (so /api/costs cannot see hits), the run total drops it, or a
// reported miss is indistinguishable from no report.
func TestRunToolLoopProgressCarriesCachedInputTokens(t *testing.T) {
	provider := newMockToolLoopProvider(&provideriface.ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "done",
		Usage:      provideriface.TokenUsage{InputTokens: 4000, OutputTokens: 20, CachedInputTokens: 3584, CacheReported: true},
		Model:      "deepseek-v4.1-flash",
	})
	var progress []json.RawMessage
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunProgress && phase == "tool_loop" {
			progress = append(progress, payload)
		}
	}
	_, usage, err := RunToolLoop(context.Background(), provider, nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)}, "sys", 4096, emit, nil)
	if err != nil {
		t.Fatal(err)
	}
	if usage.CachedInputTokens != 3584 || !usage.CacheReported {
		t.Fatalf("run usage = %+v", usage)
	}
	if len(progress) != 1 {
		t.Fatalf("progress events = %d", len(progress))
	}
	var p map[string]any
	if err := json.Unmarshal(progress[0], &p); err != nil {
		t.Fatal(err)
	}
	if p["cached_input_tokens"] != float64(3584) || p["cache_reported"] != true {
		t.Fatalf("progress payload = %s", progress[0])
	}
	// The payload, stored as the runtime stores it (phase in the payload),
	// prices through llmcost with the cache.
	p["phase"] = "tool_loop"
	p["llm_model"] = "deepseek-v4.1-flash"
	raw, _ := json.Marshal(p)
	entries := llmcost.ExtractCostEntries([]types.EventRecord{{EventID: "e", Kind: types.EventRunProgress, Payload: raw}})
	if len(entries) != 1 || entries[0].CachedInputTokens != 3584 || !entries[0].CacheReported {
		t.Fatalf("cost entries = %+v", entries)
	}
}

func TestTokenUsageAddAccumulatesCache(t *testing.T) {
	var total provideriface.TokenUsage
	total.Add(provideriface.TokenUsage{InputTokens: 100, OutputTokens: 1})
	total.Add(provideriface.TokenUsage{InputTokens: 200, OutputTokens: 2, CachedInputTokens: 150, CacheReported: true})
	total.Add(provideriface.TokenUsage{InputTokens: 300, OutputTokens: 3, CacheReported: false})
	if total.InputTokens != 600 || total.OutputTokens != 6 || total.CachedInputTokens != 150 || !total.CacheReported {
		t.Fatalf("total = %+v", total)
	}
}
