package modelcatalog

import (
	"fmt"
	"strings"
)

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

	// Modalities names the input modalities Choir sends this model. It is
	// filled by SupportedModels from the vision default and the text-only
	// opt-out list (TextOnlyModels); catalog entries do not set it.
	Modalities []string `json:"modalities,omitempty"`

	// AdapterModalities names modalities Choir's adapter for this provider
	// serializes. Every adapter serializes text and images (user turns).
	AdapterModalities []string `json:"adapter_modalities,omitempty"`

	// TextOnlyReason is why the model is opted out of image input; empty
	// for a vision model.
	TextOnlyReason string `json:"text_only_reason,omitempty"`

	// RecommendedFor names role/purpose hints for model policy UIs.
	RecommendedFor []string `json:"recommended_for,omitempty"`
}

// Input modalities.
const (
	ModalityText  = "text"
	ModalityImage = "image"
)

// TextOnlyModels is the explicit image opt-out list: catalogued model id to
// the reason it may not receive an image. Owner 2026-10-15: vision is on by
// default for every model the catalog configures; a model goes here only
// when it is known text-only or its image support through our route is
// UNVERIFIED. Delete a line once scripts/vision_probe.py --model <id>
// passes on staging (docs/problems/models-configured-text-only-2026-10-15.md).
func TextOnlyModels() map[string]string {
	const zaiText = "believed text-only: Z.AI ships vision as separate GLM-*V models (UNVERIFIED for this version)"
	const unverified = "UNVERIFIED: no doc or code confirms image input through OpenCode for this model"
	return map[string]string{
		"glm-5.2":                         zaiText,
		"glm-5.1":                         zaiText,
		"glm-5-turbo":                     zaiText,
		"glm-5.3-flash":                   unverified,
		"muse-spark-1.3-contributor":      unverified,
		"muse-spark-1.3-contributor-free": unverified,
		"hy3":                             unverified,
		"qwen3.7-max":                     unverified,
		"cat-jam-free":                    unverified,
		"longcat-2.5-preview-free":        unverified,
		"step-5-preview-free":             unverified,
	}
}

// SupportsImage reports whether Choir may send an image to modelID: the
// model is catalogued (configured) and not on the text-only opt-out list.
// An uncatalogued id is refused: Choir knows nothing about it.
func SupportsImage(modelID string) bool {
	return ImageRefusal(modelID) == ""
}

// ImageRefusal explains why modelID may not receive an image, or "" when
// it may.
func ImageRefusal(modelID string) string {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID != modelID {
			continue
		}
		if model.TextOnlyReason == "" {
			return ""
		}
		return fmt.Sprintf("model %q is opted out of image input in the Choir model catalog (%s)", modelID, model.TextOnlyReason)
	}
	return fmt.Sprintf("model %q is not in the Choir model catalog; image input requires a catalogued model", modelID)
}

// withModalities applies the vision default and the opt-out list.
func withModalities(models []ModelInfo) []ModelInfo {
	textOnly := TextOnlyModels()
	for i := range models {
		models[i].AdapterModalities = []string{ModalityText, ModalityImage}
		if reason, ok := textOnly[models[i].ID]; ok {
			models[i].Modalities = []string{ModalityText}
			models[i].TextOnlyReason = reason
			continue
		}
		models[i].Modalities = []string{ModalityText, ModalityImage}
	}
	return models
}

// SupportedModels returns the list of models Choir knows how to route through
// platform-owned providers. A new entry takes images by default; opt a
// text-only model out in TextOnlyModels.
func SupportedModels() []ModelInfo {
	return withModalities([]ModelInfo{
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
			ID:                  "glm-5.2",
			DisplayName:         "GLM-5.2",
			Provider:            "zai",
			MaxOutputTokens:     131072,
			ContextWindowTokens: 1_000_000,
			RecommendedFor:      []string{"management", "engineering", "verifier"},
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
			ID:                  "deepseek-v4.1-flash",
			DisplayName:         "DeepSeek V4.1 Flash",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{"engineering", "verifier"},
		},
		{
			ID:                  "glm-5.3-flash",
			DisplayName:         "GLM-5.3 Flash",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{"engineering", "verifier"},
		},
		{
			ID:                  "muse-spark-1.3-contributor",
			DisplayName:         "Muse Spark 1.3 Contributor",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{"engineering", "verifier"},
		},
		{
			ID:                  "muse-spark-1.3-contributor-free",
			DisplayName:         "Muse Spark 1.3 Contributor Free",
			Provider:            "opencode-zen",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{"engineering", "verifier"},
		},
		{
			ID:                  "hy3",
			DisplayName:         "HY3",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{"engineering"},
		},
		{
			ID:                  "qwen3.7-max",
			DisplayName:         "Qwen 3.7 Max",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
			RecommendedFor:      []string{"engineering", "verifier"},
		},
		// Free opencode-go models (owner 2026-10-13: "add these to our pool,
		// and see how much usage we can get out of them"). Chat completions,
		// probed with a tool call on 2026-10-13.
		{
			ID:                  "cat-jam-free",
			DisplayName:         "Cat Jam (free)",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: 262144,
		},
		{
			ID:                  "longcat-2.5-preview-free",
			DisplayName:         "LongCat 2.5 Preview (free)",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
		},
		{
			ID:                  "step-5-preview-free",
			DisplayName:         "Step 5 Preview (free)",
			Provider:            "opencode-go",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
		},

		{
			ID:                  "gpt-6-luna",
			DisplayName:         "GPT-6 Luna",
			Provider:            "chatgpt",
			MaxOutputTokens:     65536,
			ContextWindowTokens: DefaultContextWindowTokens,
		},
	})
}

// DefaultPool names the pool a spec without a model resolves to.
const DefaultPool = "default"

// PoolSpecPrefix marks a model spec that names a pool instead of a model.
const PoolSpecPrefix = "pool:"

// PoolMember is one (provider, model) pair in a pool.
type PoolMember struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// Pools returns the named model pools, each an ordered list of catalog
// entries. A thread is assigned one member and fails over to the next in
// order. Owner 2026-10-12: every desk runs on a mix of deepseek and luna
// (docs/model-pools-design-2026-10-12.md). Owner 2026-10-13: the free
// opencode-go models join the pool to measure how much they carry and
// whether they meet the bar; gpt-6-luna sits last so a thread that fails
// over from it wraps to deepseek. step-5-preview-free stays catalog-only
// until its upstream answers (429 "endpoint unavailable" when probed).
// longcat-2.5-preview-free was measured and left the pool on 2026-10-14:
// 21.8 s median and 115 s p90 per call on the paper computer, up to 932 s,
// against 2.5 s and 10 s for gpt-6-luna; a management turn on it held the
// paper for 38 minutes (docs/evidence/model-latency-paper-2026-10-14.md).
func Pools() map[string][]PoolMember {
	return map[string][]PoolMember{
		DefaultPool: {
			{Provider: "opencode-go", Model: "deepseek-v4.1-flash"},
			{Provider: "opencode-go", Model: "cat-jam-free"},
			{Provider: "chatgpt", Model: "gpt-6-luna"},
		},
	}
}

// Lookup returns the catalog entry for (provider, model).
func Lookup(provider, modelID string) (ModelInfo, bool) {
	provider, modelID = strings.TrimSpace(provider), strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.Provider == provider && model.ID == modelID {
			return model, true
		}
	}
	return ModelInfo{}, false
}

// ProviderCatalogued reports whether the catalog lists any model for
// provider. Providers it does not list are not validated by model.
func ProviderCatalogued(provider string) bool {
	provider = strings.TrimSpace(provider)
	for _, model := range SupportedModels() {
		if model.Provider == provider {
			return true
		}
	}
	return false
}

// MaxOutputTokensForModel returns the maximum output tokens for a model ID.
// Falls back to DefaultMaxOutputTokens if the model is not found in the catalog.
func MaxOutputTokensForModel(modelID string) int {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID == modelID && model.MaxOutputTokens > 0 {
			return model.MaxOutputTokens
		}
	}
	return DefaultMaxOutputTokens
}

// ContextWindowTokensForModel returns the advertised context window size for a
// model ID. Falls back to DefaultContextWindowTokens if the model is not found
// or does not advertise a window.
func ContextWindowTokensForModel(modelID string) int {
	modelID = strings.TrimSpace(modelID)
	for _, model := range SupportedModels() {
		if model.ID == modelID && model.ContextWindowTokens > 0 {
			return model.ContextWindowTokens
		}
	}
	return DefaultContextWindowTokens
}
