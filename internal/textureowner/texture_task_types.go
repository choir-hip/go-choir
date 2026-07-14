package textureowner

import (
	"encoding/json"
	"strconv"
	"strings"
)

const (
	textureAgentRevisionTaskType = "texture_agent_revision"
)

const (
	textureRevisionRoleInput              = "input"
	textureRevisionRoleCanonical          = "canonical"
	textureInputOriginUserPrompt          = "user_prompt"
	textureInputOriginProcessorHandoff    = "processor_handoff"
	textureInputOriginReconcilerHandoff   = "reconciler_handoff"
	textureMetadataPromptUnixTS           = "prompt_unix_ts"
	canonicalTextureSourcePathMetadataKey = "canonical_texture_source_path"
	textureAvailableSourceEntitiesKey     = "texture_available_source_entities"
	runMetadataExplicitResearcher         = "explicit_researcher_request"
	runMetadataProcessorKey               = "processor_key"
	runMetadataReconcilerScope            = "reconciler_scope"
)

func textureInputOriginForCaller(profile string) string {
	switch strings.TrimSpace(profile) {
	case "processor":
		return textureInputOriginProcessorHandoff
	case "reconciler":
		return textureInputOriginReconcilerHandoff
	default:
		return textureInputOriginUserPrompt
	}
}

var durableMetadataKeys = []string{
	"seed_prompt",
	runMetadataExplicitResearcher,
	"source_path",
	canonicalTextureSourcePathMetadataKey,
	"import_manifest",
	"migration_manifest",
	"conductor_loop_id",
	"trajectory_id",
	"artifact_kind",
	"revision_role",
	"input_origin",
	"texture_version_stage",
	"source_network_cycle_id",
	"source_network_request_id",
	"source_network_request_kind",
	"ingestion_handoff_cycle_id",
	"ingestion_handoff_request_id",
	"ingestion_handoff_request_kind",
	"source_item_ids",
	"processor_key",
	"reconciler_scope",
	"selected_style_sources",
	"selected_style_rationale",
	"owner_email",
	"llm_policy_overlay_id",
	textureAvailableSourceEntitiesKey,
}

func isTextureAgentRevisionTaskType(value string) bool {
	switch strings.TrimSpace(value) {
	case textureAgentRevisionTaskType:
		return true
	default:
		return false
	}
}

func metadataStringValue(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataIntValue(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		number, _ := value.Int64()
		return int(number)
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(value))
		return number
	default:
		return 0
	}
}

func textureAgentIDMatchesDoc(agentID, docID string) bool {
	agentID = strings.TrimSpace(agentID)
	docID = strings.TrimSpace(docID)
	return agentID != "" && docID != "" && agentID == currentTextureAgentID(docID)
}

func docIDFromTextureAgentID(agentID string) string {
	const prefix = "texture:"
	agentID = strings.TrimSpace(agentID)
	if !strings.HasPrefix(agentID, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(agentID, prefix))
}

type explicitInitialTextureDecision struct {
	DecisionKind string
	Reason       string
	EvidenceRefs []string
	NextAction   string
}

func explicitNoWorkerDecisionRequestFromPrompt(prompt string) (explicitInitialTextureDecision, bool) {
	text := strings.TrimSpace(prompt)
	if !texturePromptExplicitlyRequestsNoWorkerDecision(text) {
		return explicitInitialTextureDecision{}, false
	}
	lower := strings.ToLower(text)
	reason := extractDelimitedPromptValue(text, lower, "exact reason ", []string{", evidence ref", ", evidence refs", ", next action", ". then "})
	if reason == "" {
		reason = extractDelimitedPromptValue(text, lower, "reason ", []string{", evidence ref", ", evidence refs", ", next action", ". then "})
	}
	if reason == "" {
		return explicitInitialTextureDecision{}, false
	}
	evidence := extractDelimitedPromptValue(text, lower, "evidence ref ", []string{", next action", ". then "})
	if evidence == "" {
		evidence = extractDelimitedPromptValue(text, lower, "evidence refs ", []string{", next action", ". then "})
	}
	nextAction := extractDelimitedPromptValue(text, lower, "next action ", []string{". then ", " then "})
	return explicitInitialTextureDecision{
		DecisionKind: "no_worker_needed",
		Reason:       strings.TrimSpace(reason),
		EvidenceRefs: splitPromptRefs(evidence),
		NextAction:   strings.TrimSpace(nextAction),
	}, true
}

func extractDelimitedPromptValue(original, lower, marker string, delimiters []string) string {
	start := strings.Index(lower, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := len(original)
	tailLower := lower[start:]
	for _, delimiter := range delimiters {
		if index := strings.Index(tailLower, delimiter); index >= 0 && start+index < end {
			end = start + index
		}
	}
	return strings.Trim(strings.TrimSpace(original[start:end]), " ,")
}

func splitPromptRefs(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func texturePromptExplicitlyRequestsDecisionNote(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	if strings.Contains(text, "record_texture_decision") {
		return true
	}
	if strings.Contains(text, "decision_kind") && strings.Contains(text, "off-document") && strings.Contains(text, "decision") {
		return true
	}
	if strings.Contains(text, "record") && strings.Contains(text, "off-document") && strings.Contains(text, "decision note") {
		return true
	}
	return strings.Contains(text, "record") && strings.Contains(text, "texture decision")
}

func texturePromptExplicitlyRequestsNoWorkerDecision(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	if strings.Contains(text, "decision_kind") && strings.Contains(text, "no_worker_needed") {
		return true
	}
	if strings.Contains(text, "no-worker") && strings.Contains(text, "decision") {
		return true
	}
	if strings.Contains(text, "no worker") && strings.Contains(text, "decision") {
		return true
	}
	return strings.Contains(text, "no research or execution worker") && texturePromptExplicitlyRequestsDecisionNote(text)
}
