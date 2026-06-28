package runtime

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// payloadString extracts a trimmed string value from a payload map.
func payloadString(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

// payloadBool extracts a bool value from a payload map.
func payloadBool(payload map[string]any, key string) bool {
	value, _ := payload[key].(bool)
	return value
}

// payloadInt64 extracts an int64 value from a payload map.
func payloadInt64(payload map[string]any, key string) int64 {
	switch value := payload[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		return n
	default:
		return 0
	}
}

// payloadAnySliceLen returns the length of an []any field in a payload map.
func payloadAnySliceLen(payload map[string]any, key string) int {
	raw, ok := payload[key].([]any)
	if !ok {
		return 0
	}
	return len(raw)
}

// parseTracePayload unmarshals a JSON RawMessage into a map.
func parseTracePayload(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

// parseTraceToolOutput extracts the output field from a trace payload as a map.
func parseTraceToolOutput(payload map[string]any) map[string]any {
	raw := payload["output"]
	switch value := raw.(type) {
	case map[string]any:
		return value
	case string:
		var out map[string]any
		if err := json.Unmarshal([]byte(value), &out); err == nil && out != nil {
			return out
		}
	}
	return map[string]any{}
}

// traceSearchProviderStats holds per-provider search statistics.
type traceSearchProviderStats struct {
	Provider     string `json:"provider"`
	Endpoint     string `json:"endpoint,omitempty"`
	Attempts     int    `json:"attempts"`
	Successes    int    `json:"successes"`
	RateLimits   int    `json:"rate_limits"`
	Errors       int    `json:"errors"`
	ResultCount  int    `json:"result_count"`
	AvgLatencyMs int64  `json:"avg_latency_ms,omitempty"`
	LastStatus   string `json:"last_status,omitempty"`
	LastError    string `json:"last_error,omitempty"`
}

// traceSearchSummary holds aggregate search statistics across providers.
type traceSearchSummary struct {
	Queries    int                        `json:"queries"`
	Attempts   int                        `json:"attempts"`
	Successes  int                        `json:"successes"`
	RateLimits int                        `json:"rate_limits"`
	Providers  []traceSearchProviderStats `json:"providers"`
}

// buildTraceSearchSummary aggregates web_search tool events into a summary.
func buildTraceSearchSummary(events []types.EventRecord) traceSearchSummary {
	type agg struct {
		traceSearchProviderStats
		latencyTotal int64
	}
	byProvider := make(map[string]*agg)
	summary := traceSearchSummary{}
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		if payloadString(payload, "tool") != "web_search" {
			continue
		}
		summary.Queries++
		if isError, _ := payload["is_error"].(bool); isError {
			provider := "unknown"
			entry := byProvider[provider]
			if entry == nil {
				entry = &agg{traceSearchProviderStats: traceSearchProviderStats{Provider: provider}}
				byProvider[provider] = entry
			}
			entry.Attempts++
			entry.Errors++
			entry.LastStatus = "error"
			entry.LastError = traceExcerpt(payloadString(payload, "output"), 240)
			continue
		}
		output := parseTraceToolOutput(payload)
		attempts, ok := output["attempts"].([]any)
		if !ok || len(attempts) == 0 {
			provider := payloadString(output, "provider")
			if provider == "" {
				provider = "unknown"
			}
			results := payloadAnySliceLen(output, "results")
			entry := byProvider[provider]
			if entry == nil {
				entry = &agg{traceSearchProviderStats: traceSearchProviderStats{Provider: provider}}
				byProvider[provider] = entry
			}
			entry.Attempts++
			entry.Successes++
			entry.ResultCount += results
			entry.LastStatus = "success"
			continue
		}
		for _, rawAttempt := range attempts {
			attempt, _ := rawAttempt.(map[string]any)
			if attempt == nil {
				continue
			}
			provider := payloadString(attempt, "provider")
			if provider == "" {
				provider = "unknown"
			}
			entry := byProvider[provider]
			if entry == nil {
				entry = &agg{traceSearchProviderStats: traceSearchProviderStats{Provider: provider}}
				byProvider[provider] = entry
			}
			if endpoint := payloadString(attempt, "endpoint"); endpoint != "" {
				entry.Endpoint = endpoint
			}
			status := payloadString(attempt, "status")
			if status == "" {
				status = "unknown"
			}
			entry.Attempts++
			entry.LastStatus = status
			latency := payloadInt64(attempt, "latency_ms")
			entry.latencyTotal += latency
			switch status {
			case "success":
				entry.Successes++
				entry.ResultCount += int(payloadInt64(attempt, "results"))
			case "rate_limited":
				entry.RateLimits++
				entry.LastError = payloadString(attempt, "error")
			default:
				entry.Errors++
				entry.LastError = payloadString(attempt, "error")
			}
		}
	}

	providers := make([]traceSearchProviderStats, 0, len(byProvider))
	for _, entry := range byProvider {
		if entry.Attempts > 0 && entry.latencyTotal > 0 {
			entry.AvgLatencyMs = entry.latencyTotal / int64(entry.Attempts)
		}
		summary.Attempts += entry.Attempts
		summary.Successes += entry.Successes
		summary.RateLimits += entry.RateLimits
		providers = append(providers, entry.traceSearchProviderStats)
	}
	summary.Providers = providers
	return summary
}

// traceRunMetadataString reads a string metadata field from a run record.
func traceRunMetadataString(run types.RunRecord, key string) string {
	if run.Metadata == nil {
		return ""
	}
	value, _ := run.Metadata[key].(string)
	return strings.TrimSpace(value)
}

// traceTrajectoryIDForRun returns the trajectory ID for a run, falling back
// to channel ID and then run ID.
func traceTrajectoryIDForRun(run types.RunRecord) string {
	if trajectoryID := traceRunMetadataString(run, runMetadataTrajectoryID); trajectoryID != "" {
		return trajectoryID
	}
	if channelID := strings.TrimSpace(run.ChannelID); channelID != "" {
		return channelID
	}
	return run.RunID
}

// traceRunProfile returns the agent profile for a run, falling back to
// metadata and task type.
func traceRunProfile(run types.RunRecord) string {
	if profile := strings.TrimSpace(run.AgentProfile); profile != "" {
		return profile
	}
	if profile := traceRunMetadataString(run, runMetadataAgentProfile); profile != "" {
		return profile
	}
	if taskType := traceRunMetadataString(run, "type"); taskType != "" {
		return taskType
	}
	return "loop"
}

// traceRunRole returns the agent role for a run, falling back to profile.
func traceRunRole(run types.RunRecord) string {
	if role := strings.TrimSpace(run.AgentRole); role != "" {
		return role
	}
	if role := traceRunMetadataString(run, runMetadataAgentRole); role != "" {
		return role
	}
	return traceRunProfile(run)
}

// traceExcerpt returns a trimmed, single-spaced excerpt of text, truncated
// to max characters with an ellipsis.
func traceExcerpt(text string, max int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if normalized == "" {
		return ""
	}
	if len(normalized) <= max {
		return normalized
	}
	return normalized[:max-1] + "…"
}
