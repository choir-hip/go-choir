package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func (rt *Runtime) synthesizeResearcherUpdateOnFailure(ctx context.Context, rec *types.RunRecord, runErr error) error {
	return rt.synthesizeResearcherUpdateIfMissing(ctx, rec, runErr)
}

func (rt *Runtime) synthesizeResearcherUpdateOnCompletion(ctx context.Context, rec *types.RunRecord) error {
	return rt.synthesizeResearcherUpdateIfMissing(ctx, rec, nil)
}

func (rt *Runtime) synthesizeResearcherUpdateIfMissing(ctx context.Context, rec *types.RunRecord, runErr error) error {
	if rt == nil || rt.store == nil || rec == nil || agentProfileForRun(rec) != agentprofile.Researcher {
		return nil
	}
	eventsForRun, err := rt.store.ListEvents(ctx, rec.RunID, 1000)
	if err != nil {
		return err
	}
	toolEvent, toolName, output, ok := latestSuccessfulResearchToolResultOutput(eventsForRun)
	if !ok {
		return nil
	}
	latestSubmit := latestSuccessfulResearchToolSeq(eventsForRun, "update_coagent")
	if latestSubmit > 0 && latestSubmit >= toolEvent.Seq {
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
	results := toolregistry.ExecuteToolBatch(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(rec)), registry, []types.ToolCall{{
		ID:        "runtime-fallback-submit-coagent-update",
		Name:      "update_coagent",
		Arguments: rawArgs,
	}}, emit)
	result := results[0]
	if result.IsError {
		return fmt.Errorf("fallback update_coagent: %s", result.Output)
	}
	return nil
}

func latestSuccessfulResearchToolSeq(events []types.EventRecord, toolNames ...string) int64 {
	wanted := make(map[string]bool, len(toolNames))
	for _, toolName := range toolNames {
		if strings.TrimSpace(toolName) != "" {
			wanted[toolName] = true
		}
	}
	var latest int64
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		tool, _ := payload["tool"].(string)
		isError, _ := payload["is_error"].(bool)
		if wanted[tool] && !isError && ev.Seq > latest {
			latest = ev.Seq
		}
	}
	return latest
}

func latestSuccessfulResearchToolResultOutput(eventsForRun []types.EventRecord) (types.EventRecord, string, map[string]any, bool) {
	wanted := map[string]bool{
		"web_search":                 true,
		"source_search":              true,
		"fetch_url":                  true,
		"import_url_content":         true,
		"import_document_content":    true,
		"read_content_item":          true,
		"read_content_item_selector": true,
		"save_evidence":              true,
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
	summary := fmt.Sprintf("Runtime fallback: researcher returned %s evidence but did not submit a coagent checkpoint before the run completed.", toolName)
	kind := "evidence_update"
	if runErr != nil {
		summary = fmt.Sprintf("Runtime fallback: researcher returned %s evidence but did not submit a coagent checkpoint before the run failed.", toolName)
		kind = "blocker"
	}
	findings := []string{researcherFallbackFinding(toolName, output, runErr != nil)}
	refs := []string{
		"trace:event:" + strings.TrimSpace(toolEvent.EventID),
		"tool:" + strings.TrimSpace(toolName),
	}
	refs = append(refs, researcherFallbackRefs(output)...)
	notes := []string{"This is a runtime-synthesized checkpoint/update so Texture can wake and honestly revise from available evidence instead of waiting indefinitely."}
	if runErr != nil && strings.TrimSpace(runErr.Error()) != "" {
		notes = append(notes, "Run error: "+strings.TrimSpace(runErr.Error()))
	}
	if rec != nil && strings.TrimSpace(rec.RunID) != "" {
		notes = append(notes, "Researcher loop: "+strings.TrimSpace(rec.RunID))
	}
	sourceRefs := append([]string{}, researcherFallbackEvidenceIDs(output)...)
	sourceRefs = append(sourceRefs, refs...)
	sources := coagentSourcesFromTypedEvidenceRefs(sourceRefs)
	args := map[string]any{
		"schema_version": types.CoagentSourcePacketSchemaV1,
		"kind":           kind,
		"summary":        summary,
		"claims":         coagentClaimsFromTexts(findings, sources),
		"sources":        sources,
		"notes":          trimDedupeNonEmpty(notes),
	}
	if rec != nil {
		if textureAgentID := strings.TrimSpace(metadataStringValue(rec.Metadata, "requested_by_agent_id")); isTextureAgentID(textureAgentID) {
			args["agent_id"] = textureAgentID
		} else if channelID := strings.TrimSpace(metadataStringValue(rec.Metadata, runMetadataChannelID)); channelID != "" {
			args["agent_id"] = currentTextureAgentID(channelID)
		}
	}
	return args, nil
}

func researcherFallbackFinding(toolName string, output map[string]any, failed bool) string {
	missingCheckpoint := "before completing."
	if failed {
		missingCheckpoint = "before the runtime deadline/failure."
	}
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
		parts = append(parts, "but the researcher did not convert it into a structured checkpoint "+missingCheckpoint)
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
		parts = append(parts, "but the researcher did not convert it into a structured checkpoint "+missingCheckpoint)
		return strings.Join(parts, " ")
	case "fetch_url":
		url := stringMapValue(output, "url")
		status := intMapValue(output, "status_code")
		if url != "" && status > 0 {
			return fmt.Sprintf("A fetch_url result returned HTTP %d for %s, but the researcher did not convert it into a structured checkpoint %s", status, url, missingCheckpoint)
		}
	case "import_url_content":
		title := stringMapValue(output, "title")
		source := stringMapValue(output, "source_url")
		if title != "" || source != "" {
			return fmt.Sprintf("An import_url_content result returned source material (%s %s), but the researcher did not convert it into a structured checkpoint %s", title, source, missingCheckpoint)
		}
	case "import_document_content", "read_content_item", "read_content_item_selector":
		contentID := stringMapValue(output, "content_id")
		title := firstNonEmpty(stringMapValue(output, "title"), stringMapValue(output, "selector_id"), contentID)
		if title != "" || contentID != "" {
			return fmt.Sprintf("A %s result returned source material (%s %s), but the researcher did not convert it into a structured checkpoint %s", toolName, title, contentID, missingCheckpoint)
		}
	case "save_evidence":
		evidenceID := stringMapValue(output, "evidence_id")
		title := stringMapValue(output, "title")
		if evidenceID != "" || title != "" {
			return fmt.Sprintf("A save_evidence result persisted evidence (%s %s), but the researcher did not deliver its evidence_id through update_coagent %s", title, evidenceID, missingCheckpoint)
		}
	}
	return fmt.Sprintf("A %s result returned, but the researcher did not convert it into a structured checkpoint %s", toolName, missingCheckpoint)
}

func researcherFallbackEvidenceIDs(output map[string]any) []string {
	if evidenceID := stringMapValue(output, "evidence_id"); evidenceID != "" {
		return []string{"evidence:" + evidenceID}
	}
	return nil
}

func researcherFallbackRefs(output map[string]any) []string {
	var refs []string
	if contentID := stringMapValue(output, "content_id"); contentID != "" {
		refs = append(refs, "content_id:"+contentID)
	}
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
