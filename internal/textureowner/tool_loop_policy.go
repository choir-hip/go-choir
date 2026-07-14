package textureowner

import (
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func initialTextureToolChoice(rec *types.RunRecord) string {
	if rec == nil || agentProfileForRun(rec) != agentprofile.Texture {
		return ""
	}
	if metadataStringValue(rec.Metadata, "request_source") == "update_coagent" {
		return "required"
	}
	if metadataIntValue(rec.Metadata, "scheduled_message_seq") > 0 {
		return ""
	}
	if metadataStringValue(rec.Metadata, "request_intent") == "revise" &&
		metadataStringValue(rec.Metadata, "current_author_kind") == string(types.AuthorUser) {
		return "required"
	}
	return ""
}

const (
	defaultTextureActorMaxProviderCalls = 80
	defaultTextureActorMaxTotalTokens   = 1200000
	defaultTextureActorMaxElapsed       = 45 * time.Minute
)

func textureActorToolLoopBudget(rec *types.RunRecord) toolregistry.ToolLoopBudget {
	docID := ""
	if rec != nil {
		docID = strings.TrimSpace(firstNonEmpty(
			metadataStringValue(rec.Metadata, "doc_id"),
			rec.ChannelID,
		))
	}
	label := "texture"
	if docID != "" {
		label = "texture:" + docID
	}
	budget := toolregistry.ToolLoopBudget{
		Label:            label,
		MaxProviderCalls: defaultTextureActorMaxProviderCalls,
		MaxTotalTokens:   defaultTextureActorMaxTotalTokens,
		MaxElapsed:       defaultTextureActorMaxElapsed,
	}
	if rec == nil {
		return budget
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_provider_calls"); value > 0 {
		budget.MaxProviderCalls = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_input_tokens"); value > 0 {
		budget.MaxInputTokens = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_output_tokens"); value > 0 {
		budget.MaxOutputTokens = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_total_tokens"); value > 0 {
		budget.MaxTotalTokens = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_elapsed_seconds"); value > 0 {
		budget.MaxElapsed = time.Duration(value) * time.Second
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_spent_provider_calls"); value > 0 {
		budget.SpentProviderCalls = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_spent_input_tokens"); value > 0 {
		budget.SpentInputTokens = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_spent_output_tokens"); value > 0 {
		budget.SpentOutputTokens = value
	}
	return budget
}
