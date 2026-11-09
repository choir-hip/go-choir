package provider

// Thinking is never turned off (owner 2026-11-09). Ways this could fail:
//  - Bedrock Haiku 5.5 omits thinking, or sends enabled+budget_tokens (400)
//    or disabled (drops the prompt cache);
//  - an older Bedrock model, Z.AI or OpenCode messages body gains a thinking
//    field it never had;
//  - an unset, none, minimal, off, disabled or false effort still reaches the
//    wire as an off shape: OpenRouter DeepSeek "none", Synthetic "none"
//    (and "minimal"), Fireworks "none", ChatGPT effort omitted or "none";
//  - the floor raises a configured effort (low must stay low where low thinks);
//  - an empty effort is turned into an off value instead of being omitted.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNoBuilderEmitsThinkingOff(t *testing.T) {
	offs := []string{"none", "minimal", "off", "disabled", "false"}
	for _, e := range offs {
		if got := openRouterReasoningEffort("deepseek/deepseek-v4.1-flash-1017", e); got != "low" {
			t.Errorf("openrouter deepseek %q -> %q, want low", e, got)
		}
		if got := openRouterReasoningEffort("nvidia/nemotron-3.5-super-vl:free", e); got != "minimal" {
			t.Errorf("openrouter crouton %q -> %q, want minimal", e, got)
		}
		// Synthetic: none and low are the non-thinking template, so the
		// lowest thinking-on setting is high.
		if got := syntheticReasoningEffort(e); got != "high" {
			t.Errorf("synthetic %q -> %q, want high", e, got)
		}
		if got := fireworksReasoningEffort(e); got != "low" {
			t.Errorf("fireworks %q -> %q, want low", e, got)
		}
		if got := chatGPTReasoningEffort(e); got != "low" {
			t.Errorf("chatgpt %q -> %q, want low", e, got)
		}
		if got := riverReasoningEffort(e); got != "low" {
			t.Errorf("river %q -> %q, want low", e, got)
		}
	}
	if got := fireworksReasoningEffort(""); got != "" {
		t.Errorf("fireworks empty -> %q, want omitted", got)
	}
	if got := fireworksReasoningEffort("medium"); got != "medium" {
		t.Errorf("fireworks medium changed: %q", got)
	}
	if got := chatGPTReasoningEffort("high"); got != "high" {
		t.Errorf("chatgpt high changed: %q", got)
	}
	if got := openRouterReasoningEffort("deepseek/x", "low"); got != "low" {
		t.Errorf("openrouter low raised: %q", got)
	}
}

func TestAnthropicWireBodiesNeverCarryThinkingOff(t *testing.T) {
	z := &ZAIProvider{}
	oc := &OpenCodeProvider{}
	b, _ := NewBedrockProvider(BedrockConfig{Region: "us-east-1", ModelID: "us.anthropic.claude-haiku-5-5", AuthToken: "tok"})
	for _, e := range []string{"", "low", "none", "off"} {
		req := LLMRequest{ReasoningEffort: e}
		bodies := map[string]any{
			"zai":      z.buildRequestBody(req, "glm"),
			"opencode": oc.buildMessagesRequestBody(req, "m"),
			"bedrock":  b.buildRequestBody(req),
		}
		for name, body := range bodies {
			raw, _ := json.Marshal(body)
			if name != "bedrock" && strings.Contains(string(raw), `"thinking"`) {
				t.Errorf("%s effort %q gained a thinking field: %s", name, e, raw)
			}
			for _, banned := range []string{`"disabled"`, `"enabled"`, "budget_tokens"} {
				if strings.Contains(string(raw), banned) {
					t.Errorf("%s effort %q carries %s: %s", name, e, banned, raw)
				}
			}
		}
	}
}
