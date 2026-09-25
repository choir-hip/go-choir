package provideriface

import "testing"

func TestMaxInteractiveOutputTokensForSelectionUsesModelCatalog(t *testing.T) {
	// OpenAI-compatible chat-completions providers omit the explicit
	// generation budget; ChatGPT's Responses endpoint rejects it outright.
	sel := LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}
	if got := MaxInteractiveOutputTokensForSelection(sel, "conductor"); got != 0 {
		t.Fatalf("conductor interactive tokens = %d, want 0 to omit chat-completions budget", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(sel, "texture"); got != 0 {
		t.Fatalf("texture interactive tokens = %d, want 0 to omit chat-completions budget", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "opencode-zen", Model: "muse-spark-1.3-contributor-free"}, "management"); got != 0 {
		t.Fatalf("OpenCode Zen interactive tokens = %d, want 0 to omit chat-completions budget", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "chatgpt", Model: "gpt-5.5"}, "texture"); got != 0 {
		t.Fatalf("ChatGPT interactive tokens = %d, want 0 to omit unsupported max_output_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "chatgpt", Model: "gpt-5.5", MaxTokens: 32768}, "management"); got != 0 {
		t.Fatalf("explicit ChatGPT interactive tokens = %d, want 0 to omit unsupported max_output_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash", MaxTokens: 32768}, "management"); got != 32768 {
		t.Fatalf("explicit OpenCode Go interactive tokens = %d, want 32768", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Model: "us.anthropic.claude-haiku-4-5-20251001-v1:0"}, "management"); got != 8192 {
		t.Fatalf("low-limit model interactive tokens = %d, want 8192", got)
	}
}
