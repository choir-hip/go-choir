package llmcost

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// CostEntry is a single LLM call's cost-relevant metadata extracted from a
// trace event. One entry corresponds to one provider call.
type CostEntry struct {
	// RunID is the run (cycle) this call belongs to.
	RunID string

	// TrajectoryID is the trajectory this call belongs to, when available.
	TrajectoryID string

	// AgentID is the agent that made the call, when available.
	AgentID string

	// Provider is the LLM provider name.
	Provider string

	// Model is the model identifier returned or configured for the call.
	Model string

	// InputTokens is the prompt token count for this call.
	InputTokens int

	// OutputTokens is the completion token count for this call.
	OutputTokens int

	// Timestamp is when the event was recorded.
	Timestamp time.Time

	// Cost is the estimated USD cost for this call.
	Cost Cost

	// EventID is the trace event that produced this entry.
	EventID string
}

// ExtractCostEntries scans trace events and returns one CostEntry per LLM call
// found. It reads the extensible payload fields that the runtime emits on
// progress events:
//
//   - phase "tool_loop": per-iteration provider call with llm_model,
//     llm_provider, input_tokens, output_tokens.
//   - phase "execution": gateway provider completion with model, provider,
//     tokens_in, tokens_out.
//
// Cumulative budget-usage events (phase "tool_loop_budget_usage") are
// intentionally excluded to avoid double-counting, since they report
// accumulated totals rather than per-call deltas.
func ExtractCostEntries(events []types.EventRecord) []CostEntry {
	var entries []CostEntry
	for _, ev := range events {
		if ev.Kind != types.EventRunProgress {
			continue
		}
		payload := parsePayload(ev.Payload)
		switch strings.TrimSpace(payloadString(payload, "phase")) {
		case "tool_loop":
			entry, ok := extractFromToolLoopEvent(ev, payload)
			if ok {
				entries = append(entries, entry)
			}
		case "execution":
			entry, ok := extractFromExecutionEvent(ev, payload)
			if ok {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

func extractFromToolLoopEvent(ev types.EventRecord, payload map[string]any) (CostEntry, bool) {
	model := firstNonEmptyString(
		payloadString(payload, "model"),
		payloadString(payload, "llm_model"),
	)
	inputTokens := payloadInt(payload, "input_tokens")
	outputTokens := payloadInt(payload, "output_tokens")
	// Skip events that carry no token data — these are pre-cost-tracking
	// events or non-LLM progress updates.
	if inputTokens == 0 && outputTokens == 0 {
		return CostEntry{}, false
	}
	if model == "" {
		return CostEntry{}, false
	}
	provider := payloadString(payload, "llm_provider")
	cost := EstimateCall(model, inputTokens, outputTokens)
	if provider == "" {
		provider = cost.Provider
	}
	return CostEntry{
		RunID:        ev.RunID,
		TrajectoryID: ev.TrajectoryID,
		AgentID:      ev.AgentID,
		Provider:     provider,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Timestamp:    ev.Timestamp,
		Cost:         cost,
		EventID:      ev.EventID,
	}, true
}

func extractFromExecutionEvent(ev types.EventRecord, payload map[string]any) (CostEntry, bool) {
	// Only the "completed" status execution event carries token counts.
	status := payloadString(payload, "status")
	if status != "completed" {
		return CostEntry{}, false
	}
	model := payloadString(payload, "model")
	inputTokens := payloadInt(payload, "tokens_in")
	outputTokens := payloadInt(payload, "tokens_out")
	if inputTokens == 0 && outputTokens == 0 {
		return CostEntry{}, false
	}
	if model == "" {
		return CostEntry{}, false
	}
	provider := payloadString(payload, "provider")
	cost := EstimateCall(model, inputTokens, outputTokens)
	if provider == "" {
		provider = cost.Provider
	}
	return CostEntry{
		RunID:        ev.RunID,
		TrajectoryID: ev.TrajectoryID,
		AgentID:      ev.AgentID,
		Provider:     provider,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Timestamp:    ev.Timestamp,
		Cost:         cost,
		EventID:      ev.EventID,
	}, true
}

// CostSummary is an aggregated cost view across a set of LLM calls.
type CostSummary struct {
	// TotalCost is the summed estimated USD cost.
	TotalCost float64 `json:"total_cost_usd"`

	// TotalInputTokens is the summed input token count.
	TotalInputTokens int `json:"total_input_tokens"`

	// TotalOutputTokens is the summed output token count.
	TotalOutputTokens int `json:"total_output_tokens"`

	// CallCount is the number of LLM calls included.
	CallCount int `json:"call_count"`

	// UnpricedCallCount is the number of calls whose model had no pricing
	// entry. These calls contributed tokens but zero USD to TotalCost.
	UnpricedCallCount int `json:"unpriced_call_count"`

	// ByProvider breaks the summary down by LLM provider.
	ByProvider map[string]CostSummary `json:"by_provider,omitempty"`

	// ByModel breaks the summary down by model identifier.
	ByModel map[string]CostSummary `json:"by_model,omitempty"`

	// ByRun breaks the summary down by run (cycle) ID.
	ByRun map[string]CostSummary `json:"by_run,omitempty"`

	// ByTrajectory breaks the summary down by trajectory ID.
	ByTrajectory map[string]CostSummary `json:"by_trajectory,omitempty"`

	// ByDay breaks the summary down by UTC date (YYYY-MM-DD).
	ByDay map[string]CostSummary `json:"by_day,omitempty"`
}

// Aggregate computes a CostSummary from a set of cost entries. The summary
// includes breakdowns by provider, model, run, trajectory, and day. All
// breakdown maps are populated; callers that only need the total can ignore
// them.
func Aggregate(entries []CostEntry) CostSummary {
	summary := CostSummary{
		ByProvider:   make(map[string]CostSummary),
		ByModel:      make(map[string]CostSummary),
		ByRun:        make(map[string]CostSummary),
		ByTrajectory: make(map[string]CostSummary),
		ByDay:        make(map[string]CostSummary),
	}
	for _, entry := range entries {
		summary.TotalCost += entry.Cost.USD
		summary.TotalInputTokens += entry.InputTokens
		summary.TotalOutputTokens += entry.OutputTokens
		summary.CallCount++
		if !entry.Cost.Found {
			summary.UnpricedCallCount++
		}
		addToBucket(summary.ByProvider, entry.Provider, entry)
		addToBucket(summary.ByModel, entry.Model, entry)
		addToBucket(summary.ByRun, entry.RunID, entry)
		addToBucket(summary.ByTrajectory, entry.TrajectoryID, entry)
		addToBucket(summary.ByDay, entry.Timestamp.UTC().Format("2006-01-02"), entry)
	}
	return summary
}

func addToBucket(bucket map[string]CostSummary, key string, entry CostEntry) {
	if key == "" {
		return
	}
	sub := bucket[key]
	sub.TotalCost += entry.Cost.USD
	sub.TotalInputTokens += entry.InputTokens
	sub.TotalOutputTokens += entry.OutputTokens
	sub.CallCount++
	if !entry.Cost.Found {
		sub.UnpricedCallCount++
	}
	bucket[key] = sub
}

// --- payload helpers (local copies to avoid importing runtime) ---

func parsePayload(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func payloadString(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func payloadInt(payload map[string]any, key string) int {
	switch value := payload[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case int64:
		return int(value)
	case string:
		n := 0
		for _, ch := range strings.TrimSpace(value) {
			if ch < '0' || ch > '9' {
				return 0
			}
			n = n*10 + int(ch-'0')
		}
		return n
	default:
		return 0
	}
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
