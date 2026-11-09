package provider

import (
	"encoding/json"
	"testing"
)

// Bedrock answers 400 when a tool_use has no tool_result in the very next
// message ("tool_use ids were found without tool_result blocks immediately
// after", the paper's gateway journal, 2026-11-09): a carried history can end
// a run on a tool call whose result was never appended, and the next wake
// appends a plain user message. Ways the repair can fail:
//  1. an unanswered tool_use followed by a user text message still reaches the wire;
//  2. an unanswered tool_use followed by an assistant message, or by nothing;
//  3. one of two calls is answered and the other is not;
//  4. an answered call gets a second, made-up result;
//  5. the made-up result lands after a text block (results must come first);
//  6. a tool_result whose call is not in the previous message reaches the wire;
//  7. the output differs between two builds of the same history (cache miss).
func TestBedrockEveryToolUseIsAnsweredInTheNextMessage(t *testing.T) {
	p, _ := NewBedrockProvider(BedrockConfig{Region: "us-east-1", ModelID: "us.anthropic.claude-haiku-5-5", AuthToken: "tok"})
	use := func(ids ...string) Message {
		m := Message{Role: "assistant"}
		for _, id := range ids {
			m.Content = append(m.Content, Block{Type: "tool_use", ID: id, Name: "eval", Input: json.RawMessage(`{}`)})
		}
		return m
	}
	msgs := []Message{
		{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "gone", Text: "stray"}, {Type: "text", Text: "wake"}}},
		use("a"),
		{Role: "user", Content: []Block{{Type: "text", Text: "a later wake"}}},
		use("b", "c"),
		{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "c", Text: "ok"}}},
		use("d"),
		{Role: "assistant", Content: []Block{{Type: "text", Text: "carried on"}}},
		{Role: "user", Content: []Block{{Type: "text", Text: "go on"}}},
		use("e"),
	}
	build := func() []byte {
		raw, _ := json.Marshal(p.buildRequestBody(LLMRequest{Model: "us.anthropic.claude-haiku-5-5", Messages: msgs, ReasoningEffort: "low"}))
		return raw
	}
	raw := build()
	if string(raw) != string(build()) {
		t.Fatal("two builds of the same history differ")
	}
	var got struct {
		Messages []struct {
			Role    string           `json:"role"`
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	var prev map[string]bool
	for i, m := range got.Messages {
		results := map[string]int{}
		seenOther := false
		for _, c := range m.Content {
			if c["type"] == "tool_result" {
				if seenOther {
					t.Fatalf("message %d: a tool_result comes after another block", i)
				}
				id, _ := c["tool_use_id"].(string)
				results[id]++
				if m.Role != "user" || !prev[id] {
					t.Fatalf("message %d: tool_result %q answers no call in the previous message", i, id)
				}
			} else {
				seenOther = true
			}
		}
		for id := range prev {
			if results[id] != 1 {
				t.Fatalf("message %d: call %q has %d results in the next message, want 1", i, id, results[id])
			}
		}
		prev = map[string]bool{}
		for _, c := range m.Content {
			if c["type"] == "tool_use" {
				prev[c["id"].(string)] = true
			}
		}
	}
	if len(prev) != 0 {
		t.Fatalf("the history ends on unanswered calls %v", prev)
	}
	if last := got.Messages[len(got.Messages)-1]; last.Role != "user" {
		t.Fatalf("the history ends on a %s message", last.Role)
	}
}
