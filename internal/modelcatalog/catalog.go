package modelcatalog

import "strings"

const (
	DefaultMaxOutputTokens     = 65536
	DefaultContextWindowTokens = 200000
)

// ModelInfo describes a supported model and its associated provider.
type ModelInfo struct {
	// ID is the model identifier used in API requests (provider-specific).
	ID string `json:"id"`

	// DisplayName is a human-readable name for logging and UI.
	DisplayName string `json:"display_name"`

	// Provider is the provider name that serves this model (e.g., "zai",
	// "fireworks", "bedrock").
	Provider string `json:"provider"`

	// MaxOutputTokens is the maximum output tokens for this model.
	MaxOutputTokens int `json:"max_output_tokens"`

	// ContextWindowTokens is the advertised or platform-assumed input context
	// window used for runtime pressure and compaction policy.
	ContextWindowTokens int `json:"context_window_tokens,omitempty"`

	// Modalities names upstream content modalities known for this model.
	Modalities []string `json:"modalities,omitempty"`

	// AdapterModalities names modalities Choir currently knows how to serialize
	// for this provider adapter.
	AdapterModalities []string `json:"adapter_modalities,omitempty"`

	// RecommendedFor names role/purpose hints for model policy UIs.
	RecommendedFor []string `json:"recommended_for,omitempty"`
}

// SupportedModels returns the list of models Choir knows how to route through
// platform-owned providers.
func SupportedModels() []ModelInfo {
	return []ModelInfo{
		{
			ID:              "us.anthropic.claude-haiku-4-5-20251001-v1:0",
			DisplayName:     "Claude Haiku 4.5",
			Provider:        "bedrock",
			MaxOutputTokens: 8192,
		},
		{
			ID:              "us.anthropic.claude-sonnet-4-6",
			DisplayName:     "Claude Sonnet 4.6",
			Provider:        "bedrock",
			MaxOutputTokens: 65536,
		},
		{
			ID:              "us.anthropic.claude-opus-4-6-v1",
			DisplayName:     "Claude Opus 4.6",
			Provider:        "bedrock",
			MaxOutputTokens: 32768,
		},
		{
			ID:              "glm-5.1",
			DisplayName:     "GLM-5.1",
			Provider:        "zai",
			MaxOutputTokens: 131072,
		},
		{
			ID:              "glm-5-turbo",
			DisplayName:     "GLM-5-Turbo",
			Provider:        "zai",
			MaxOutputTokens: 131072,
		},
		{
			ID:                  "deepseek-v4-pro",
			DisplayName:         "DeepSeek V4 Pro",
			Provider:            "deepseek",
			MaxOutputTokens:     131072,
			ContextWindowTokens: 1_000_000,
			Modalities:          []string{"text"},
			AdapterModalities:   []string{"text"},
			RecommendedFor:      []string{"super", "vsuper", "cosuper_coding", "verifier"},
		},
		{
			ID:                  "deepseek-v4-flash",
			DisplayName:         "DeepSeek V4 Flash",
			Provider:            "deepseek",
			MaxOutputTokens:     131072,
			ContextWindowTokens: 1_000_000,
			Modalities:          []string{"text"},
			AdapterModalities:   []string{"text"},
			RecommendedFor:      []string{"conductor", "texture", "researcher"},
		},
		{
			ID:                  "mimo-v2.5-pro",
			DisplayName:         "MiMo V2.5 Pro",
			Provider:            "xiaomi",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 1_000_000,
			Modalities:          []string{"text"},
			AdapterModalities:   []string{"text"},
			RecommendedFor:      []string{"super", "vsuper", "cosuper_coding", "verifier"},
		},
		{
			ID:                  "mimo-v2.5",
			DisplayName:         "MiMo V2.5",
			Provider:            "xiaomi",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 1_000_000,
			Modalities:          []string{"text", "image"},
			AdapterModalities:   []string{"text", "image"},
			RecommendedFor:      []string{"verifier_multimodal"},
		},
		{
			ID:                "accounts/fireworks/models/deepseek-v4-pro",
			DisplayName:       "DeepSeek V4 Pro",
			Provider:          "fireworks",
			MaxOutputTokens:   131072,
			Modalities:        []string{"text"},
			AdapterModalities: []string{"text"},
			RecommendedFor:    []string{"super", "vsuper", "cosuper_coding", "verifier"},
		},
		{
			ID:                "accounts/fireworks/models/deepseek-v4-flash",
			DisplayName:       "DeepSeek V4 Flash",
			Provider:          "fireworks",
			MaxOutputTokens:   131072,
			Modalities:        []string{"text"},
			AdapterModalities: []string{"text"},
			RecommendedFor:    []string{"conductor", "texture", "researcher"},
		},
		{
			ID:                "accounts/fireworks/models/kimi-k2p6",
			DisplayName:       "Kimi K2.6",
			Provider:          "fireworks",
			MaxOutputTokens:   131072,
			Modalities:        []string{"text", "image"},
			AdapterModalities: []string{"text", "image"},
			RecommendedFor:    []string{"verifier_multimodal"},
		},
		{
			ID:              "gpt-5.5",
			DisplayName:     "GPT-5.5",
			Provider:        "chatgpt",
			MaxOutputTokens: 65536,
		},
		{
			ID:              "gpt-5.4",
			DisplayName:     "GPT-5.4",
			Provider:        "chatgpt",
			MaxOutputTokens: 65536,
		},
		{
			ID:              "gpt-5.4-mini",
			DisplayName:     "GPT-5.4 Mini",
			Provider:        "chatgpt",
			MaxOutputTokens: 65536,
		},
	}
}

func MaxOutputTokensForModel(modelID string) int {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID == modelID && model.MaxOutputTokens > 0 {
			return model.MaxOutputTokens
		}
	}
	return DefaultMaxOutputTokens
}

func ContextWindowTokensForModel(modelID string) int {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID == modelID && model.ContextWindowTokens > 0 {
			return model.ContextWindowTokens
		}
	}
	return DefaultContextWindowTokens
}
