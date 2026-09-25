package llmcost

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func makeToolLoopEvent(eventID, runID, trajectoryID, agentID, model, provider string, inputTokens, outputTokens int, ts time.Time) types.EventRecord {
	payload, _ := json.Marshal(map[string]any{
		"phase":                "tool_loop",
		"iteration":            1,
		"stop_reason":          "end_turn",
		"model":                model,
		"llm_provider":         provider,
		"llm_model":            model,
		"llm_reasoning_effort": "medium",
		"input_tokens":         inputTokens,
		"output_tokens":        outputTokens,
	})
	return types.EventRecord{
		EventID:      eventID,
		RunID:        runID,
		TrajectoryID: trajectoryID,
		AgentID:      agentID,
		Kind:         types.EventRunProgress,
		Phase:        "tool_loop",
		Payload:      payload,
		Timestamp:    ts,
	}
}

func TestExtractCostEntriesToolLoop(t *testing.T) {
	t.Parallel()
	events := []types.EventRecord{
		makeToolLoopEvent("ev-1", "run-1", "traj-1", "agent-a", "gpt-4o", "openai", 1000, 500, time.Unix(1700000000, 0)),
		makeToolLoopEvent("ev-2", "run-1", "traj-1", "agent-a", "gpt-4o", "openai", 2000, 1000, time.Unix(1700000060, 0)),
	}
	entries := ExtractCostEntries(events)
	if len(entries) != 2 {
		t.Fatalf("ExtractCostEntries: got %d entries, want 2", len(entries))
	}
	if entries[0].Model != "gpt-4o" {
		t.Fatalf("entry 0 model: got %q, want gpt-4o", entries[0].Model)
	}
	if entries[0].InputTokens != 1000 || entries[0].OutputTokens != 500 {
		t.Fatalf("entry 0 tokens: got in=%d out=%d, want 1000/500", entries[0].InputTokens, entries[0].OutputTokens)
	}
	if !entries[0].Cost.Found {
		t.Fatal("entry 0 cost should be found")
	}
}
func TestAggregateByRun(t *testing.T) {
	t.Parallel()
	entries := []CostEntry{
		{RunID: "run-1", Provider: "openai", Model: "gpt-4o", InputTokens: 1000, OutputTokens: 500, Timestamp: time.Unix(1700000000, 0), Cost: EstimateCall("gpt-4o", 1000, 500)},
		{RunID: "run-1", Provider: "openai", Model: "gpt-4o", InputTokens: 2000, OutputTokens: 1000, Timestamp: time.Unix(1700000060, 0), Cost: EstimateCall("gpt-4o", 2000, 1000)},
		{RunID: "run-2", Provider: "openai", Model: "gpt-4o", InputTokens: 500, OutputTokens: 200, Timestamp: time.Unix(1700000120, 0), Cost: EstimateCall("gpt-4o", 500, 200)},
	}
	summary := Aggregate(entries)
	run1, ok := summary.ByRun["run-1"]
	if !ok {
		t.Fatal("ByRun[run-1] missing")
	}
	if run1.CallCount != 2 {
		t.Fatalf("ByRun[run-1] CallCount: got %d, want 2", run1.CallCount)
	}
	if run1.TotalInputTokens != 3000 {
		t.Fatalf("ByRun[run-1] TotalInputTokens: got %d, want 3000", run1.TotalInputTokens)
	}
	run2, ok := summary.ByRun["run-2"]
	if !ok {
		t.Fatal("ByRun[run-2] missing")
	}
	if run2.CallCount != 1 {
		t.Fatalf("ByRun[run-2] CallCount: got %d, want 1", run2.CallCount)
	}
}
func TestAggregateUnpricedCallCount(t *testing.T) {
	t.Parallel()
	entries := []CostEntry{
		{RunID: "run-1", Provider: "openai", Model: "gpt-4o", InputTokens: 1000, OutputTokens: 500, Timestamp: time.Unix(1700000000, 0), Cost: EstimateCall("gpt-4o", 1000, 500)},
		{RunID: "run-2", Provider: "", Model: "unknown-model", InputTokens: 1000, OutputTokens: 500, Timestamp: time.Unix(1700000060, 0), Cost: EstimateCall("unknown-model", 1000, 500)},
	}
	summary := Aggregate(entries)
	if summary.UnpricedCallCount != 1 {
		t.Fatalf("UnpricedCallCount: got %d, want 1", summary.UnpricedCallCount)
	}
}
