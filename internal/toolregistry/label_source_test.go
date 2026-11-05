package toolregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// MA3.6: a model call is logged with the resolved provider and model and the
// run's label set, and is not gated. The durable per-call record (the
// tool_loop progress event) and the log line both carry them.
func TestModelCallIsLoggedWithProviderModelAndLabels(t *testing.T) {
	provider := newMockToolLoopProvider(&provideriface.ToolLoopResponse{
		StopReason: "end_turn", Text: "done", Model: "test-model",
		Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
	})
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)

	var call map[string]any
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunProgress && phase == "tool_loop" {
			if err := json.Unmarshal(payload, &call); err != nil {
				t.Fatal(err)
			}
		}
	}
	ctx := WithExecutionContext(context.Background(), ExecutionContext{RunID: "run-labels"})
	ctx = WithLabelSource(ctx, func() []string { return []string{"world_untrusted", "owner_private:capture"} })
	if _, _, err := RunToolLoop(ctx, provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)}, "sys", 0, emit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "fireworks", Model: "deepseek-v4-flash"})); err != nil {
		t.Fatal(err)
	}
	if call["llm_provider"] != "fireworks" || call["llm_model"] != "deepseek-v4-flash" {
		t.Fatalf("call record = %v", call)
	}
	labels, _ := call["labels"].([]any)
	if len(labels) != 2 || labels[0] != "owner_private:capture" || labels[1] != "world_untrusted" {
		t.Fatalf("labels in the call record = %v", call["labels"])
	}
	line := buf.String()
	if !strings.Contains(line, "model call: run=run-labels provider=fireworks model=deepseek-v4-flash labels=owner_private:capture,world_untrusted") {
		t.Fatalf("log = %q", line)
	}
}

func TestAModelCallWithNoLabelSourceLogsNoLabels(t *testing.T) {
	if got := ModelCallLine("r", "p", "m", nil); got != "model call: run=r provider=p model=m labels=-" {
		t.Fatal(got)
	}
	if LabelsFrom(context.Background()) != nil {
		t.Fatal("labels from a bare context")
	}
}
