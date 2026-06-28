package llmcost

import (
	"encoding/json"
	"math"
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

func makeExecutionEvent(eventID, runID, model, provider string, inputTokens, outputTokens int, ts time.Time) types.EventRecord {
	payload, _ := json.Marshal(map[string]any{
		"phase":      "execution",
		"status":     "completed",
		"model":      model,
		"provider":   provider,
		"tokens_in":  inputTokens,
		"tokens_out": outputTokens,
	})
	return types.EventRecord{
		EventID:   eventID,
		RunID:     runID,
		Kind:      types.EventRunProgress,
		Phase:     "execution",
		Payload:   payload,
		Timestamp: ts,
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

func TestExtractCostEntriesExecution(t *testing.T) {
	t.Parallel()
	events := []types.EventRecord{
		makeExecutionEvent("ev-1", "run-1", "claude-3.5-sonnet", "anthropic", 5000, 2000, time.Unix(1700000000, 0)),
	}
	entries := ExtractCostEntries(events)
	if len(entries) != 1 {
		t.Fatalf("ExtractCostEntries: got %d entries, want 1", len(entries))
	}
	if entries[0].Provider != "anthropic" {
		t.Fatalf("entry 0 provider: got %q, want anthropic", entries[0].Provider)
	}
}

func TestExtractCostEntriesSkipsZeroTokenEvents(t *testing.T) {
	t.Parallel()
	// A tool_loop event with no token data (pre-cost-tracking) should be skipped.
	payload, _ := json.Marshal(map[string]any{
		"phase":     "tool_loop",
		"iteration": 1,
		"model":     "gpt-4o",
	})
	events := []types.EventRecord{{
		EventID: "ev-1",
		RunID:   "run-1",
		Kind:    types.EventRunProgress,
		Phase:   "tool_loop",
		Payload: payload,
	}}
	entries := ExtractCostEntries(events)
	if len(entries) != 0 {
		t.Fatalf("ExtractCostEntries: got %d entries, want 0 for zero-token event", len(entries))
	}
}

func TestExtractCostEntriesSkipsBudgetUsage(t *testing.T) {
	t.Parallel()
	// Budget-usage events are cumulative and must not be extracted as per-call.
	payload, _ := json.Marshal(map[string]any{
		"phase":                "tool_loop_budget_usage",
		"input_tokens":         5000,
		"output_tokens":        2000,
		"activation_input_tokens":  1000,
		"activation_output_tokens": 500,
	})
	events := []types.EventRecord{{
		EventID: "ev-1",
		RunID:   "run-1",
		Kind:    types.EventRunProgress,
		Phase:   "tool_loop_budget_usage",
		Payload: payload,
	}}
	entries := ExtractCostEntries(events)
	if len(entries) != 0 {
		t.Fatalf("ExtractCostEntries: got %d entries, want 0 for budget-usage event", len(entries))
	}
}

func TestExtractCostEntriesSkipsExecutionStarted(t *testing.T) {
	t.Parallel()
	payload, _ := json.Marshal(map[string]any{
		"phase":    "execution",
		"status":   "started",
		"provider": "gateway",
	})
	events := []types.EventRecord{{
		EventID: "ev-1",
		RunID:   "run-1",
		Kind:    types.EventRunProgress,
		Phase:   "execution",
		Payload: payload,
	}}
	entries := ExtractCostEntries(events)
	if len(entries) != 0 {
		t.Fatalf("ExtractCostEntries: got %d entries, want 0 for execution-started event", len(entries))
	}
}

func TestAggregateTotal(t *testing.T) {
	t.Parallel()
	entries := []CostEntry{
		{RunID: "run-1", TrajectoryID: "traj-1", Provider: "openai", Model: "gpt-4o", InputTokens: 1000, OutputTokens: 500, Timestamp: time.Unix(1700000000, 0), Cost: EstimateCall("gpt-4o", 1000, 500)},
		{RunID: "run-1", TrajectoryID: "traj-1", Provider: "openai", Model: "gpt-4o", InputTokens: 2000, OutputTokens: 1000, Timestamp: time.Unix(1700000060, 0), Cost: EstimateCall("gpt-4o", 2000, 1000)},
		{RunID: "run-2", TrajectoryID: "traj-2", Provider: "anthropic", Model: "claude-3.5-sonnet", InputTokens: 5000, OutputTokens: 2000, Timestamp: time.Unix(1700000120, 0), Cost: EstimateCall("claude-3.5-sonnet", 5000, 2000)},
	}
	summary := Aggregate(entries)
	if summary.CallCount != 3 {
		t.Fatalf("CallCount: got %d, want 3", summary.CallCount)
	}
	if summary.TotalInputTokens != 8000 {
		t.Fatalf("TotalInputTokens: got %d, want 8000", summary.TotalInputTokens)
	}
	if summary.TotalOutputTokens != 3500 {
		t.Fatalf("TotalOutputTokens: got %d, want 3500", summary.TotalOutputTokens)
	}
	want := 0.0125 + 0.025 + (3.0*5000/1_000_000 + 15.0*2000/1_000_000)
	if math.Abs(summary.TotalCost-want) > 0.0000001 {
		t.Fatalf("TotalCost: got %.8f, want %.8f", summary.TotalCost, want)
	}
}

func TestAggregateByProvider(t *testing.T) {
	t.Parallel()
	entries := []CostEntry{
		{RunID: "run-1", Provider: "openai", Model: "gpt-4o", InputTokens: 1000, OutputTokens: 500, Timestamp: time.Unix(1700000000, 0), Cost: EstimateCall("gpt-4o", 1000, 500)},
		{RunID: "run-2", Provider: "anthropic", Model: "claude-3.5-sonnet", InputTokens: 5000, OutputTokens: 2000, Timestamp: time.Unix(1700000120, 0), Cost: EstimateCall("claude-3.5-sonnet", 5000, 2000)},
	}
	summary := Aggregate(entries)
	openai, ok := summary.ByProvider["openai"]
	if !ok {
		t.Fatal("ByProvider[openai] missing")
	}
	if openai.CallCount != 1 {
		t.Fatalf("ByProvider[openai] CallCount: got %d, want 1", openai.CallCount)
	}
	anthropic, ok := summary.ByProvider["anthropic"]
	if !ok {
		t.Fatal("ByProvider[anthropic] missing")
	}
	if anthropic.CallCount != 1 {
		t.Fatalf("ByProvider[anthropic] CallCount: got %d, want 1", anthropic.CallCount)
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

func TestAggregateByTrajectory(t *testing.T) {
	t.Parallel()
	entries := []CostEntry{
		{RunID: "run-1", TrajectoryID: "traj-1", Provider: "openai", Model: "gpt-4o", InputTokens: 1000, OutputTokens: 500, Timestamp: time.Unix(1700000000, 0), Cost: EstimateCall("gpt-4o", 1000, 500)},
		{RunID: "run-2", TrajectoryID: "traj-1", Provider: "openai", Model: "gpt-4o", InputTokens: 500, OutputTokens: 200, Timestamp: time.Unix(1700000060, 0), Cost: EstimateCall("gpt-4o", 500, 200)},
	}
	summary := Aggregate(entries)
	traj1, ok := summary.ByTrajectory["traj-1"]
	if !ok {
		t.Fatal("ByTrajectory[traj-1] missing")
	}
	if traj1.CallCount != 2 {
		t.Fatalf("ByTrajectory[traj-1] CallCount: got %d, want 2", traj1.CallCount)
	}
}

func TestAggregateByDay(t *testing.T) {
	t.Parallel()
	// 2023-11-14 UTC and 2023-11-15 UTC.
	entries := []CostEntry{
		{RunID: "run-1", Provider: "openai", Model: "gpt-4o", InputTokens: 1000, OutputTokens: 500, Timestamp: time.Unix(1699977600, 0), Cost: EstimateCall("gpt-4o", 1000, 500)},  // 2023-11-14 16:00 UTC
		{RunID: "run-2", Provider: "openai", Model: "gpt-4o", InputTokens: 500, OutputTokens: 200, Timestamp: time.Unix(1700064000, 0), Cost: EstimateCall("gpt-4o", 500, 200)},    // 2023-11-15 16:00 UTC
	}
	summary := Aggregate(entries)
	day1, ok := summary.ByDay["2023-11-14"]
	if !ok {
		t.Fatal("ByDay[2023-11-14] missing")
	}
	if day1.CallCount != 1 {
		t.Fatalf("ByDay[2023-11-14] CallCount: got %d, want 1", day1.CallCount)
	}
	day2, ok := summary.ByDay["2023-11-15"]
	if !ok {
		t.Fatal("ByDay[2023-11-15] missing")
	}
	if day2.CallCount != 1 {
		t.Fatalf("ByDay[2023-11-15] CallCount: got %d, want 1", day2.CallCount)
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

func TestAggregateEmpty(t *testing.T) {
	t.Parallel()
	summary := Aggregate(nil)
	if summary.CallCount != 0 {
		t.Fatalf("CallCount: got %d, want 0", summary.CallCount)
	}
	if summary.TotalCost != 0 {
		t.Fatalf("TotalCost: got %.4f, want 0", summary.TotalCost)
	}
}

func TestExtractCostEntriesIgnoresNonProgressEvents(t *testing.T) {
	t.Parallel()
	events := []types.EventRecord{
		{EventID: "ev-1", Kind: types.EventRunStarted, Payload: json.RawMessage(`{}`)},
		{EventID: "ev-2", Kind: types.EventRunCompleted, Payload: json.RawMessage(`{}`)},
	}
	entries := ExtractCostEntries(events)
	if len(entries) != 0 {
		t.Fatalf("ExtractCostEntries: got %d entries, want 0 for non-progress events", len(entries))
	}
}
