package provider

import (
	"encoding/json"
	"regexp"
	"testing"
)

// Five shapes Bedrock answers 400 to on Haiku 5.5 (direct probes, 2026-11-09):
// (1) a tool_use id outside ^[a-zA-Z0-9_-]+$, (2) a thinking block without a
// signature, (3) an empty text block, (4) a tool without input_schema,
// (5) temperature. A transcript from a crouton/OpenRouter/DeepSeek run can
// carry 1 to 4; none may reach the wire. (5) is unreachable: the Bedrock body
// has no sampling fields; the test pins that.
func TestBedrockHaiku55NoRejectedShapeReachesTheWire(t *testing.T) {
	p, _ := NewBedrockProvider(BedrockConfig{Region: "us-east-1", ModelID: "us.anthropic.claude-haiku-5-5", AuthToken: "tok"})
	build := func() []byte {
		unsigned := `[{"type":"thinking","thinking":"foreign reasoning"}]`
		msgs := []Message{
			{Role: "user", Content: []Block{{Type: "text", Text: ""}, {Type: "text", Text: "wake"}}},
			{Role: "assistant", ReasoningContent: unsigned, Content: []Block{{Type: "text", Text: ""}, {Type: "tool_use", ID: "functions.eval:0", Name: "eval", Input: json.RawMessage(`{}`)}}},
			{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "functions.eval:0", Text: "ok"}}},
			{Role: "assistant", Content: []Block{{Type: "text", Text: ""}}},
			{Role: "user", Content: []Block{{Type: "text", Text: "go on"}}},
		}
		raw, _ := json.Marshal(p.buildRequestBody(LLMRequest{Model: "us.anthropic.claude-haiku-5-5", Messages: msgs, ReasoningEffort: "low",
			Tools: []ToolDef{{Name: "eval", Description: "no schema"}}}))
		return raw
	}
	raw := build()
	var got struct {
		Temperature any              `json:"temperature"`
		TopP        any              `json:"top_p"`
		TopK        any              `json:"top_k"`
		Tools       []map[string]any `json:"tools"`
		Messages    []struct {
			Role    string           `json:"role"`
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Temperature != nil || got.TopP != nil || got.TopK != nil {
		t.Fatalf("sampling params on the wire: %s", raw)
	}
	if len(got.Tools) != 1 || got.Tools[0]["input_schema"] == nil {
		t.Fatalf("tool without input_schema: %s", raw)
	}
	idRE := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	var useID, resID any
	for i, m := range got.Messages {
		if len(m.Content) == 0 {
			t.Fatalf("message %d has empty content: %s", i, raw)
		}
		for _, c := range m.Content {
			switch c["type"] {
			case "text":
				if s, _ := c["text"].(string); s == "" {
					t.Fatalf("empty text block in message %d: %s", i, raw)
				}
			case "thinking", "redacted_thinking":
				if s, _ := c["signature"].(string); s == "" {
					t.Fatalf("unsigned thinking block in message %d: %s", i, raw)
				}
			case "tool_use":
				useID = c["id"]
			case "tool_result":
				resID = c["tool_use_id"]
			}
		}
	}
	if s, _ := useID.(string); !idRE.MatchString(s) || useID != resID {
		t.Fatalf("tool ids use=%v result=%v", useID, resID)
	}
	if string(build()) != string(raw) {
		t.Fatal("body is not byte-stable across builds (cache prefix would break)")
	}
	if bedrockToolID("call_00_AbC") != "call_00_AbC" || bedrockToolID("a.b") == bedrockToolID("a:b") {
		t.Fatal("id mapping must keep valid ids and keep distinct inputs distinct")
	}
}
