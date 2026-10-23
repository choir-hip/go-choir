package provideriface

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
)

// LLMSelection is the effective provider/model/reasoning tuple used for a run.
// It contains no provider secrets; credentials remain platform/server-owned.
type LLMSelection struct {
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	MaxTokens       int    `json:"max_tokens,omitempty"`
	Source          string `json:"source,omitempty"`
}

const (
	runMetadataLLMProvider        = "llm_provider"
	runMetadataLLMModel           = "llm_model"
	runMetadataLLMReasoningEffort = "llm_reasoning_effort"
	runMetadataLLMMaxTokens       = "llm_max_tokens"
	runMetadataLLMPolicySource    = "llm_policy_source"
)

// MaxOutputTokensForSelection returns the model catalog maximum for the
// selected model.
func MaxOutputTokensForSelection(sel LLMSelection) int {
	return modelcatalog.MaxOutputTokensForModel(sel.Model)
}

// MaxInteractiveOutputTokensForSelection returns the per-call generation budget
// requested by foreground agent loops. This is intentionally separate from the
// model catalog maximum: catalog limits describe capability, while request
// budgets are provider protocol choices. OpenAI-compatible chat completions
// paths behave best when ordinary agent loops omit explicit generation budgets;
// the ChatGPT Codex Responses endpoint rejects max_output_tokens, so ChatGPT
// loops also omit explicit output budgets.
func MaxInteractiveOutputTokensForSelection(sel LLMSelection, role string) int {
	provider := strings.ToLower(strings.TrimSpace(sel.Provider))
	if provider == "chatgpt" {
		return 0
	}
	if sel.MaxTokens > 0 {
		return sel.MaxTokens
	}
	// OpenAI-compatible chat-completions providers behave best when ordinary
	// agent loops omit the explicit generation budget.
	if provider == "opencode-go" || provider == "opencode-zen" || provider == "synthetic" {
		return 0
	}
	return MaxOutputTokensForSelection(sel)
}

// ResolvedLLMConfigFromMetadata extracts the effective LLM selection from run
// metadata.
func ResolvedLLMConfigFromMetadata(metadata map[string]any) LLMSelection {
	if metadata == nil {
		return LLMSelection{}
	}
	return LLMSelection{
		Provider:        strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMProvider)),
		Model:           strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMModel)),
		ReasoningEffort: strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMReasoningEffort)),
		MaxTokens:       metadataIntValue(metadata, runMetadataLLMMaxTokens),
		Source:          strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMPolicySource)),
	}
}

// FallbackLLMConfigsFromMetadata parses the run's failover order
// (llm_fallbacks: provider/model, comma-separated). Each fallback keeps the
// primary's reasoning effort and output budget.
func FallbackLLMConfigsFromMetadata(metadata map[string]any, primary LLMSelection) []LLMSelection {
	raw := metadataStringValue(metadata, "llm_fallbacks")
	if raw == "" {
		return nil
	}
	out := make([]LLMSelection, 0, 2)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		slash := strings.Index(part, "/")
		if slash <= 0 || slash == len(part)-1 {
			continue
		}
		out = append(out, LLMSelection{
			Provider:        part[:slash],
			Model:           part[slash+1:],
			ReasoningEffort: primary.ReasoningEffort,
			MaxTokens:       primary.MaxTokens,
			Source:          "model_failover",
		})
	}
	return out
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
		n, _ := value.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}
