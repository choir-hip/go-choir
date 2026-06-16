package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func (rt *Runtime) synthesizeResearcherUpdateOnFailure(ctx context.Context, rec *types.RunRecord, runErr error) error {
	if rt == nil || rt.store == nil || rec == nil || agentProfileForRun(rec) != AgentProfileResearcher {
		return nil
	}
	eventsForRun, err := rt.store.ListEvents(ctx, rec.RunID, 1000)
	if err != nil {
		return err
	}
	if hasSuccessfulToolResult(eventsForRun, "update_coagent") {
		return nil
	}
	toolEvent, toolName, output, ok := latestSuccessfulResearchToolResultOutput(eventsForRun)
	if !ok {
		return nil
	}
	registry := rt.toolRegistryForRun(rec)
	if registry == nil {
		return nil
	}
	updateArgs, err := researcherFallbackUpdateArgs(rec, runErr, toolEvent, toolName, output)
	if err != nil {
		return err
	}
	rawArgs, err := json.Marshal(updateArgs)
	if err != nil {
		return err
	}
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		rt.emitEvent(ctx, rec, kind, events.CauseSupervisorRecovery, payload)
	}
	result := executeOneTool(WithToolExecutionContext(ctx, rec), registry, types.ToolCall{
		ID:        "runtime-fallback-submit-coagent-update",
		Name:      "update_coagent",
		Arguments: rawArgs,
	}, "", emit)
	if result.IsError {
		return fmt.Errorf("fallback update_coagent: %s", result.Output)
	}
	return nil
}

func latestSuccessfulResearchToolResultOutput(eventsForRun []types.EventRecord) (types.EventRecord, string, map[string]any, bool) {
	wanted := map[string]bool{
		"web_search":         true,
		"source_search":      true,
		"fetch_url":          true,
		"import_url_content": true,
	}
	var latest types.EventRecord
	var latestTool string
	var latestOutput map[string]any
	found := false
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload, ok := decodeToolResultPayload(ev)
		if !ok || payload.IsError || !wanted[payload.Tool] {
			continue
		}
		var output map[string]any
		if err := json.Unmarshal([]byte(payload.Output), &output); err != nil {
			output = map[string]any{"raw_output": strings.TrimSpace(payload.Output)}
		}
		latest = ev
		latestTool = payload.Tool
		latestOutput = output
		found = true
	}
	return latest, latestTool, latestOutput, found
}

func researcherFallbackUpdateArgs(rec *types.RunRecord, runErr error, toolEvent types.EventRecord, toolName string, output map[string]any) (map[string]any, error) {
	updateID := "runtime-research-checkpoint-" + strings.TrimSpace(toolEvent.EventID)
	if updateID == "runtime-research-checkpoint-" {
		updateID = "runtime-research-checkpoint-" + uuid.NewString()
	}
	summary := fmt.Sprintf("Runtime fallback: researcher returned %s evidence but did not submit a coagent checkpoint before the run failed.", toolName)
	findings := []string{researcherFallbackFinding(toolName, output)}
	refs := []string{
		"trace:event:" + strings.TrimSpace(toolEvent.EventID),
		"tool:" + strings.TrimSpace(toolName),
	}
	refs = append(refs, researcherFallbackRefs(output)...)
	notes := []string{"This is a runtime-synthesized blocker/update so Texture can wake and honestly revise from available evidence instead of waiting indefinitely."}
	if runErr != nil && strings.TrimSpace(runErr.Error()) != "" {
		notes = append(notes, "Run error: "+strings.TrimSpace(runErr.Error()))
	}
	if rec != nil && strings.TrimSpace(rec.RunID) != "" {
		notes = append(notes, "Researcher loop: "+strings.TrimSpace(rec.RunID))
	}
	return map[string]any{
		"update_id": updateID,
		"kind":      "blocker",
		"summary":   summary,
		"findings":  findings,
		"refs":      trimDedupeNonEmpty(refs),
		"notes":     trimDedupeNonEmpty(notes),
	}, nil
}

func researcherFallbackFinding(toolName string, output map[string]any) string {
	switch toolName {
	case "web_search":
		query := stringMapValue(output, "query")
		provider := stringMapValue(output, "provider")
		resultCount := intMapValue(output, "result_count")
		if resultCount == 0 {
			resultCount = len(anySliceMapValue(output, "results"))
		}
		parts := []string{"A web_search result returned"}
		if query != "" {
			parts = append(parts, "for query "+strconvQuote(query))
		}
		if resultCount > 0 {
			parts = append(parts, fmt.Sprintf("with %d visible result(s)", resultCount))
		}
		if provider != "" {
			parts = append(parts, "via "+provider)
		}
		parts = append(parts, "but the researcher did not convert it into a structured checkpoint before the runtime deadline/failure.")
		return strings.Join(parts, " ")
	case "source_search":
		query := stringMapValue(output, "query")
		resultCount := intMapValue(output, "result_count")
		if resultCount == 0 {
			resultCount = len(anySliceMapValue(output, "results"))
		}
		parts := []string{"A source_search result returned"}
		if query != "" {
			parts = append(parts, "for query "+strconvQuote(query))
		}
		if resultCount > 0 {
			parts = append(parts, fmt.Sprintf("with %d visible source-service item(s)", resultCount))
		}
		parts = append(parts, "but the researcher did not convert it into a structured checkpoint before the runtime deadline/failure.")
		return strings.Join(parts, " ")
	case "fetch_url":
		url := stringMapValue(output, "url")
		status := intMapValue(output, "status_code")
		if url != "" && status > 0 {
			return fmt.Sprintf("A fetch_url result returned HTTP %d for %s, but the researcher did not convert it into a structured checkpoint before the runtime deadline/failure.", status, url)
		}
	case "import_url_content":
		title := stringMapValue(output, "title")
		source := stringMapValue(output, "source_url")
		if title != "" || source != "" {
			return fmt.Sprintf("An import_url_content result returned source material (%s %s), but the researcher did not convert it into a structured checkpoint before the runtime deadline/failure.", title, source)
		}
	}
	return fmt.Sprintf("A %s result returned, but the researcher did not convert it into a structured checkpoint before the runtime deadline/failure.", toolName)
}

func researcherFallbackRefs(output map[string]any) []string {
	var refs []string
	if url := stringMapValue(output, "url"); url != "" {
		refs = append(refs, url)
	}
	if source := stringMapValue(output, "source_url"); source != "" {
		refs = append(refs, source)
	}
	for _, result := range anySliceMapValue(output, "results") {
		item, _ := result.(map[string]any)
		if item == nil {
			continue
		}
		if url := stringMapValue(item, "url"); url != "" {
			refs = append(refs, url)
		}
		if itemID := stringMapValue(item, "item_id"); itemID != "" {
			refs = append(refs, "source_service_item:"+itemID)
		}
		if sourceID := stringMapValue(item, "source_id"); sourceID != "" {
			refs = append(refs, "source:"+sourceID)
		}
		if len(refs) >= 5 {
			break
		}
	}
	return refs
}

func anySliceMapValue(m map[string]any, key string) []any {
	switch value := m[key].(type) {
	case []any:
		return value
	default:
		return nil
	}
}

func strconvQuote(value string) string {
	encoded, err := json.Marshal(strings.TrimSpace(value))
	if err != nil {
		return strings.TrimSpace(value)
	}
	return string(encoded)
}
