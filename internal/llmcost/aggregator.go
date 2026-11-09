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

	// Desk is the desk whose run made the call ("management", "app:notes"),
	// from the event when it names one and otherwise filled by the caller from
	// the run record (AttributeDesks). Empty when it cannot be told.
	Desk string

	// Provider is the LLM provider name.
	Provider string

	// Model is the model identifier returned or configured for the call.
	Model string

	// InputTokens is the prompt token count for this call.
	InputTokens int

	// OutputTokens is the completion token count for this call.
	OutputTokens int

	// CachedInputTokens is the part of InputTokens the provider served from
	// its prompt cache. Zero on events recorded before 2026-10-14.
	CachedInputTokens int

	// CacheReported is true when the provider reported a cache count for
	// this call (a reported 0 is a miss; an unreported call is unknown and
	// stays out of the hit rate).
	CacheReported bool

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
//     llm_provider, input_tokens, output_tokens, and (since 2026-10-14)
//     cached_input_tokens and cache_reported.
//   - phase "jev_judgment": one decision-model (Jev) call made for a run,
//     with the same fields.
//   - phase "execution": gateway provider completion with model, provider,
//     tokens_in, tokens_out, tokens_cached, cache_reported.
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
		case "tool_loop", "jev_judgment":
			// jev_judgment is one decision-model (Jev) call the runtime
			// made for a run, such as judging its revision's citations; it
			// carries the same fields as a tool-loop iteration.
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
	cachedTokens := payloadInt(payload, "cached_input_tokens")
	write5m := payloadInt(payload, "cache_write_5m_tokens")
	write1h := payloadInt(payload, "cache_write_1h_tokens")
	// Skip events that carry no token data — these are pre-cost-tracking
	// events or non-LLM progress updates.
	if inputTokens == 0 && outputTokens == 0 {
		return CostEntry{}, false
	}
	if model == "" {
		return CostEntry{}, false
	}
	provider := payloadString(payload, "llm_provider")
	cost := EstimateCallWithCacheWrites(model, inputTokens, cachedTokens, write5m, write1h, outputTokens)
	if provider == "" {
		provider = cost.Provider
	}
	return CostEntry{
		RunID:             ev.RunID,
		TrajectoryID:      ev.TrajectoryID,
		AgentID:           ev.AgentID,
		Desk:              strings.TrimSpace(payloadString(payload, "desk")),
		Provider:          provider,
		Model:             model,
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		CachedInputTokens: cost.CachedInputTokens,
		CacheReported:     payloadBool(payload, "cache_reported") || cost.CachedInputTokens > 0,
		Timestamp:         ev.Timestamp,
		Cost:              cost,
		EventID:           ev.EventID,
	}, true
}

// SystemPackage is the package name spend by a built-in desk is grouped under.
const SystemPackage = "system"

// Unattributed groups spend whose desk could not be told.
const Unattributed = "unattributed"

// PackageOfDesk is the package a desk's spend is grouped under: the package of
// an app desk (app:<pkg>), SystemPackage for a built-in desk.
func PackageOfDesk(desk string) string {
	desk = strings.TrimSpace(desk)
	switch {
	case desk == "":
		return Unattributed
	case strings.HasPrefix(desk, "app:"):
		return strings.TrimPrefix(desk, "app:")
	}
	return SystemPackage
}

// AttributeDesks fills the Desk of every entry that has none from deskOf, which
// maps a run id to the desk that ran it (the run record's profile). Entries
// whose run is unknown keep an empty desk and aggregate as Unattributed.
func AttributeDesks(entries []CostEntry, deskOf func(runID string) string) {
	seen := map[string]string{}
	for i := range entries {
		if entries[i].Desk != "" || entries[i].RunID == "" {
			continue
		}
		desk, ok := seen[entries[i].RunID]
		if !ok {
			desk = strings.TrimSpace(deskOf(entries[i].RunID))
			seen[entries[i].RunID] = desk
		}
		entries[i].Desk = desk
	}
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
	cachedTokens := payloadInt(payload, "tokens_cached")
	if inputTokens == 0 && outputTokens == 0 {
		return CostEntry{}, false
	}
	if model == "" {
		return CostEntry{}, false
	}
	provider := payloadString(payload, "provider")
	cost := EstimateCallWithCache(model, inputTokens, cachedTokens, outputTokens)
	if provider == "" {
		provider = cost.Provider
	}
	return CostEntry{
		RunID:             ev.RunID,
		TrajectoryID:      ev.TrajectoryID,
		AgentID:           ev.AgentID,
		Desk:              strings.TrimSpace(payloadString(payload, "desk")),
		Provider:          provider,
		Model:             model,
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		CachedInputTokens: cost.CachedInputTokens,
		CacheReported:     payloadBool(payload, "cache_reported") || cost.CachedInputTokens > 0,
		Timestamp:         ev.Timestamp,
		Cost:              cost,
		EventID:           ev.EventID,
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

	// TotalCachedInputTokens is the part of TotalInputTokens the providers
	// served from their prompt caches.
	TotalCachedInputTokens int `json:"total_cached_input_tokens"`

	// CacheReportedCallCount is the number of calls whose provider reported
	// a cache count (hit or miss).
	CacheReportedCallCount int `json:"cache_reported_call_count"`

	// CacheReportedInputTokens is the input tokens of those calls: the
	// denominator of CacheHitRate.
	CacheReportedInputTokens int `json:"cache_reported_input_tokens"`

	// CacheHitRate is TotalCachedInputTokens / CacheReportedInputTokens:
	// the token-weighted prompt-cache hit rate over calls that reported
	// caching. Zero when no call reported.
	CacheHitRate float64 `json:"cache_hit_rate"`

	// CacheSavingsUSD is what the cached tokens would have cost more at
	// the uncached input rate.
	CacheSavingsUSD float64 `json:"cache_savings_usd"`

	// ByProvider breaks the summary down by LLM provider.
	ByProvider map[string]CostSummary `json:"by_provider,omitempty"`

	// ByModel breaks the summary down by model identifier.
	ByModel map[string]CostSummary `json:"by_model,omitempty"`

	// ByDesk breaks the summary down by the desk whose run made the call
	// (MA3.9): spend is attributed and watched, never capped (R8).
	ByDesk map[string]CostSummary `json:"by_desk,omitempty"`

	// ByPackage groups the same spend by package: an app desk's package, or
	// "system" for the built-in desks.
	ByPackage map[string]CostSummary `json:"by_package,omitempty"`

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
		ByDesk:       make(map[string]CostSummary),
		ByPackage:    make(map[string]CostSummary),
		ByRun:        make(map[string]CostSummary),
		ByTrajectory: make(map[string]CostSummary),
		ByDay:        make(map[string]CostSummary),
	}
	for _, entry := range entries {
		summary.add(entry)
		addToBucket(summary.ByProvider, entry.Provider, entry)
		addToBucket(summary.ByModel, entry.Model, entry)
		desk := entry.Desk
		if desk == "" {
			desk = Unattributed
		}
		addToBucket(summary.ByDesk, desk, entry)
		addToBucket(summary.ByPackage, PackageOfDesk(entry.Desk), entry)
		addToBucket(summary.ByRun, entry.RunID, entry)
		addToBucket(summary.ByTrajectory, entry.TrajectoryID, entry)
		addToBucket(summary.ByDay, entry.Timestamp.UTC().Format("2006-01-02"), entry)
	}
	summary.finishHitRate()
	for _, bucket := range []map[string]CostSummary{summary.ByProvider, summary.ByModel, summary.ByDesk, summary.ByPackage, summary.ByRun, summary.ByTrajectory, summary.ByDay} {
		for key, sub := range bucket {
			sub.finishHitRate()
			bucket[key] = sub
		}
	}
	return summary
}

func (s *CostSummary) add(entry CostEntry) {
	s.TotalCost += entry.Cost.USD
	s.TotalInputTokens += entry.InputTokens
	s.TotalOutputTokens += entry.OutputTokens
	s.TotalCachedInputTokens += entry.CachedInputTokens
	s.CacheSavingsUSD += entry.Cost.CacheSavingsUSD
	s.CallCount++
	if !entry.Cost.Found {
		s.UnpricedCallCount++
	}
	if entry.CacheReported {
		s.CacheReportedCallCount++
		s.CacheReportedInputTokens += entry.InputTokens
	}
}

func (s *CostSummary) finishHitRate() {
	s.CacheHitRate = 0
	if s.CacheReportedInputTokens > 0 {
		s.CacheHitRate = float64(s.TotalCachedInputTokens) / float64(s.CacheReportedInputTokens)
	}
}

func addToBucket(bucket map[string]CostSummary, key string, entry CostEntry) {
	if key == "" {
		return
	}
	sub := bucket[key]
	sub.add(entry)
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

// payloadBool reads a JSON bool or a "true"/"1" string.
func payloadBool(payload map[string]any, key string) bool {
	switch value := payload[key].(type) {
	case bool:
		return value
	case string:
		v := strings.TrimSpace(value)
		return v == "1" || strings.EqualFold(v, "true")
	default:
		return false
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
