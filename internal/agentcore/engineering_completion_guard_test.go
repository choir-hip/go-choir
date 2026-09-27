package agentcore

import (
	"encoding/json"
	"testing"
)

func toolResultMsg(t *testing.T, output string) json.RawMessage {
	m, _ := json.Marshal(map[string]any{
		"role": "user",
		"content": []map[string]any{{
			"type": "tool_result", "tool_use_id": "x", "content": output,
		}},
	})
	return m
}

func TestEngineeringOverlayTerminalFateCommitted(t *testing.T) {
	text, _ := json.Marshal(map[string]any{"role": "assistant", "content": "done"})
	nonTerminal := toolResultMsg(t, `{"exit_code":0,"stdout":"ok"}`)
	terminal := toolResultMsg(t, `{"exit_code":0,"fate_terminal":true}`)
	plainString, _ := json.Marshal(map[string]any{"role": "user", "content": "hello"})

	if engineeringOverlayTerminalFateCommitted([]json.RawMessage{text, nonTerminal}) {
		t.Fatal("non-terminal result must not satisfy the guard")
	}
	if !engineeringOverlayTerminalFateCommitted([]json.RawMessage{nonTerminal, terminal}) {
		t.Fatal("terminal fate result must satisfy the guard")
	}
	if engineeringOverlayTerminalFateCommitted([]json.RawMessage{plainString, text}) {
		t.Fatal("string content must not crash or satisfy the guard")
	}
}
