package provider

import (
	"strings"
	"testing"
)

// The Responses stream reports each function call twice — once as
// response.function_call_arguments.done and again as response.output_item.done.
// The parser appended both, so every OpenAI-routed desk executed each cell
// twice under one call id (Minesweeper demo, management runs 2026-10-10:
// every call_id invoked 2x; a cast committed, then its duplicate failed).
// Failure modes pinned: a call reported by both events is returned twice; a
// call reported only by output_item.done (no argument events) is dropped;
// two distinct calls collapse into one.
func TestResponsesStreamReturnsEachFunctionCallOnce(t *testing.T) {
	events := []string{
		`{"type":"response.created","response":{"id":"resp_1","model":"gpt-5.6-luna"}}`,
		`{"type":"response.output_item.added","item":{"type":"function_call","id":"fc_1","call_id":"call_A","name":"desk_go_eval","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"source\":"}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"\"println(1)\"}"}`,
		`{"type":"response.function_call_arguments.done","item_id":"fc_1","arguments":"{\"source\":\"println(1)\"}"}`,
		`{"type":"response.output_item.done","item":{"type":"function_call","id":"fc_1","call_id":"call_A","name":"desk_go_eval","arguments":"{\"source\":\"println(1)\"}"}}`,
		`{"type":"response.output_item.done","item":{"type":"function_call","id":"fc_2","call_id":"call_B","name":"desk_go_eval","arguments":"{\"source\":\"println(2)\"}"}}`,
		`{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-luna","usage":{"input_tokens":10,"output_tokens":5}}}`,
	}
	var body strings.Builder
	for _, event := range events {
		body.WriteString("data: " + event + "\n\n")
	}
	result, err := parseOpenAIStream(strings.NewReader(body.String()), "gpt-5.6-luna", "chatgpt", func(StreamChunk) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("tool calls = %+v, want call_A and call_B once each", result.ToolCalls)
	}
	if result.ToolCalls[0].ID != "call_A" || result.ToolCalls[1].ID != "call_B" {
		t.Fatalf("tool call order = %s, %s", result.ToolCalls[0].ID, result.ToolCalls[1].ID)
	}
	if !strings.Contains(string(result.ToolCalls[0].Arguments), "println(1)") || !strings.Contains(string(result.ToolCalls[1].Arguments), "println(2)") {
		t.Fatalf("arguments = %s / %s", result.ToolCalls[0].Arguments, result.ToolCalls[1].Arguments)
	}
}
