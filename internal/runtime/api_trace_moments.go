package runtime

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func buildTraceMomentSummaries(events []types.EventRecord, agentIndex map[string]traceAgentNode) []traceMomentSummary {
	moments := make([]traceMomentSummary, 0, len(events))
	for _, ev := range events {
		if ev.Kind == types.EventRunDelta {
			continue
		}
		moments = append(moments, buildTraceMomentSummary(ev, agentIndex))
	}
	sort.Slice(moments, func(i, j int) bool {
		return moments[i].StreamSeq < moments[j].StreamSeq
	})
	return moments
}

func buildTraceMomentSummary(ev types.EventRecord, agentIndex map[string]traceAgentNode) traceMomentSummary {
	payload := parseTracePayload(ev.Payload)
	agent := agentIndex[strings.TrimSpace(ev.AgentID)]
	channelID := strings.TrimSpace(ev.ChannelID)
	if ch := payloadString(payload, "channel_id"); ch != "" {
		channelID = ch
	}
	return traceMomentSummary{
		MomentID:     ev.EventID,
		StreamSeq:    ev.StreamSeq,
		Timestamp:    formatTraceTime(ev.Timestamp),
		Kind:         ev.Kind,
		Phase:        strings.TrimSpace(ev.Phase),
		RunID:        strings.TrimSpace(ev.RunID),
		AgentID:      strings.TrimSpace(ev.AgentID),
		AgentLabel:   traceNonEmpty(agent.Label, shortTraceID(ev.AgentID)),
		AgentProfile: agent.Profile,
		AgentRole:    agent.Role,
		ChannelID:    channelID,
		Summary:      traceEventSummary(ev, payload),
		Tone:         traceEventTone(ev),
		HasDetail:    true,
		MessageSeq:   payloadInt64(payload, "cursor"),
	}
}

func traceEventSummary(ev types.EventRecord, payload map[string]any) string {
	switch ev.Kind {
	case types.EventToolResult:
		tool := traceNonEmpty(payloadString(payload, "tool"), "tool")
		if isError, _ := payload["is_error"].(bool); isError {
			return fmt.Sprintf("%s returned error", tool)
		}
		return fmt.Sprintf("%s returned", tool)
	case types.EventChannelMessage:
		content := traceExcerpt(payloadString(payload, "content"), 96)
		return fmt.Sprintf("%s: %s", traceNonEmpty(payloadString(payload, "from"), "agent"), traceNonEmpty(content, "message"))
	case types.EventAppChangePackagePublished:
		return "published app package"
	case types.EventAppAdoptionVerified:
		return "app adoption verified"
	case types.EventAppAdoptionPromoted:
		return "app adoption promoted"
	case types.EventRunStarted:
		return "loop started"
	case types.EventRunCompleted:
		return "loop completed"
	case types.EventRunFailed, types.EventRunBlocked, types.EventRunCancelled:
		if errText := payloadString(payload, "error"); errText != "" {
			return errText
		}
		return string(ev.Kind)
	default:
		if status := payloadString(payload, "status"); status != "" {
			return status
		}
		if phase := payloadString(payload, "phase"); phase != "" {
			return phase
		}
		return string(ev.Kind)
	}
}

func traceEventTone(ev types.EventRecord) string {
	switch ev.Kind {
	case types.EventRunFailed, types.EventRunBlocked, types.EventRunCancelled:
		return "error"
	case types.EventRunCompleted, types.EventAppAdoptionVerified, types.EventAppAdoptionPromoted:
		return "success"
	case types.EventChannelMessage:
		return "message"
	case types.EventToolResult:
		return "tool"
	default:
		return "neutral"
	}
}

func formatTraceTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func parseTraceTime(value string) time.Time {
	ts, _ := time.Parse(time.RFC3339Nano, value)
	return ts
}

func shortTraceID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) <= 8 {
		return value
	}
	return value[:8]
}

func traceNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}
