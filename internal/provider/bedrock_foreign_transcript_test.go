package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

// A mid-run transcript written by another model (DeepSeek style: assistant
// turns carry tool calls and plain-text reasoning_content, no thinking
// blocks) must reach Haiku 5.5 in a shape the Messages API accepts. Docs
// (platform.claude.com/docs/en/build-with-claude/thinking-steering-and-cost,
// "Turn validation"): "Assistant turns don't need to start with a thinking
// block... History assembled from mixed sources doesn't need thinking blocks
// reinserted"; only legacy manual-budget models require the final assistant
// turn to start with one. So the body must carry no thinking config, must
// carry output_config.effort (high is valid: Haiku 5.5 supports all five
// levels), a positive max_tokens, every tool_use answered first in the next
// message, no empty text blocks and at most 4 cache breakpoints.
func TestBedrockHaiku55ForeignTranscriptBody(t *testing.T) {
	p, err := NewBedrockProvider(BedrockConfig{Region: "us-east-1", ModelID: "us.anthropic.claude-haiku-5-5", AuthToken: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	msgs := []Message{{Role: "user", Content: []Block{{Type: "text", Text: "wake"}}}}
	for i := 0; i < 7; i++ {
		id := "call_" + string(rune('a'+i))
		msgs = append(msgs,
			Message{Role: "assistant", ReasoningContent: "plain text reasoning from another model", Content: []Block{{Type: "tool_use", ID: id, Name: "texture_op", Input: json.RawMessage(`{"op":"read"}`)}}},
			Message{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: id, Text: "ok"}}},
		)
	}
	msgs = append(msgs,
		Message{Role: "assistant", Content: []Block{{Type: "tool_use", ID: "call_z", Name: "texture_op", Input: json.RawMessage(`{}`)}}},
		Message{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "call_z", Text: ""}}})
	body := p.buildRequestBody(LLMRequest{
		Model: "us.anthropic.claude-haiku-5-5", System: "sys", ReasoningEffort: "high", Messages: msgs,
		Tools: []ToolDef{{Name: "texture_op", Description: "d", InputSchema: map[string]any{"type": "object"}}},
	})
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		MaxTokens    int             `json:"max_tokens"`
		Thinking     json.RawMessage `json:"thinking"`
		Version      string          `json:"anthropic_version"`
		OutputConfig struct {
			Effort string `json:"effort"`
		} `json:"output_config"`
		Messages []struct {
			Role    string           `json:"role"`
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.MaxTokens <= 0 || got.Thinking == nil || got.Version != "bedrock-2023-05-31" || got.OutputConfig.Effort != "high" {
		t.Fatalf("bad envelope: %s", raw)
	}
	if strings.Count(string(raw), `"cache_control"`) > 4 {
		t.Fatalf("more than 4 cache breakpoints: %s", raw)
	}
	for i, m := range got.Messages {
		for _, c := range m.Content {
			if c["type"] == "text" && c["text"] == nil {
				t.Fatalf("message %d has an empty text block", i)
			}
			if c["type"] == "thinking" {
				t.Fatalf("message %d carries a thinking block from a foreign transcript", i)
			}
			if c["type"] == "tool_use" {
				if i+1 >= len(got.Messages) || got.Messages[i+1].Role != "user" || got.Messages[i+1].Content[0]["tool_use_id"] != c["id"] {
					t.Fatalf("tool_use %v at message %d is not answered first in the next message", c["id"], i)
				}
			}
		}
		if i > 0 && m.Role == got.Messages[i-1].Role {
			t.Fatalf("messages %d and %d share a role", i-1, i)
		}
	}
}
