package provideriface

import "testing"

func TestMaxOutputTokensForSelectionUsesModelCatalog(t *testing.T) {
	if got := MaxOutputTokensForSelection(LLMSelection{Model: "accounts/fireworks/models/deepseek-v4-flash"}); got != 131072 {
		t.Fatalf("deepseek flash max tokens = %d, want 131072", got)
	}
	if got := MaxOutputTokensForSelection(LLMSelection{Model: "gpt-5.5"}); got != 65536 {
		t.Fatalf("gpt-5.5 max tokens = %d, want 65536", got)
	}
	if got := MaxOutputTokensForSelection(LLMSelection{Model: "unknown-model"}); got != 65536 {
		t.Fatalf("unknown model max tokens = %d, want safe default 65536", got)
	}
}

func TestMaxInteractiveOutputTokensForSelectionUsesModelCatalog(t *testing.T) {
	sel := LLMSelection{Provider: "fireworks", Model: "accounts/fireworks/models/deepseek-v4-flash"}
	if got := MaxInteractiveOutputTokensForSelection(sel, "conductor"); got != 0 {
		t.Fatalf("conductor interactive tokens = %d, want 0 to omit Fireworks max_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(sel, "texture"); got != 0 {
		t.Fatalf("texture interactive tokens = %d, want 0 to omit Fireworks max_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "chatgpt", Model: "gpt-5.5"}, "texture"); got != 0 {
		t.Fatalf("ChatGPT interactive tokens = %d, want 0 to omit unsupported max_output_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "chatgpt", Model: "gpt-5.5", MaxTokens: 32768}, "super"); got != 0 {
		t.Fatalf("explicit ChatGPT interactive tokens = %d, want 0 to omit unsupported max_output_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "xiaomi", Model: "mimo-v2.5-pro"}, "super"); got != 0 {
		t.Fatalf("Xiaomi interactive tokens = %d, want 0 to omit OpenAI-compatible chat budget", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "fireworks", Model: "accounts/fireworks/models/deepseek-v4-flash", MaxTokens: 32768}, "super"); got != 32768 {
		t.Fatalf("explicit Fireworks interactive tokens = %d, want 32768", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Model: "us.anthropic.claude-haiku-4-5-20251001-v1:0"}, "super"); got != 8192 {
		t.Fatalf("low-limit model interactive tokens = %d, want 8192", got)
	}
}

func TestResolvedLLMConfigFromMetadata(t *testing.T) {
	cfg := ResolvedLLMConfigFromMetadata(map[string]any{
		"llm_provider":         "fireworks",
		"llm_model":            "accounts/fireworks/models/deepseek-v4-flash",
		"llm_reasoning_effort": "medium",
		"llm_max_tokens":       32768,
		"llm_policy_source":    "policy.toml",
	})
	if cfg.Provider != "fireworks" || cfg.Model != "accounts/fireworks/models/deepseek-v4-flash" || cfg.ReasoningEffort != "medium" {
		t.Fatalf("selection = %+v", cfg)
	}
	if cfg.MaxTokens != 32768 {
		t.Fatalf("max tokens = %d, want 32768", cfg.MaxTokens)
	}
	if cfg.Source != "policy.toml" {
		t.Fatalf("source = %q, want policy.toml", cfg.Source)
	}
	if got := ResolvedLLMConfigFromMetadata(nil); got != (LLMSelection{}) {
		t.Fatalf("nil metadata = %+v, want zero", got)
	}
}
