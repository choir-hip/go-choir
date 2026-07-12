package toolregistry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ExecuteToolBatch executes independent tool calls concurrently and returns
// their results in provider call order. Runtime callers with app-specific batch
// policy must supply their policy executor instead.
func ExecuteToolBatch(ctx context.Context, registry *ToolRegistry, calls []types.ToolCall, emit provideriface.EventEmitFunc) []types.ToolResult {
	results := make([]types.ToolResult, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(index int, call types.ToolCall) {
			defer wg.Done()
			results[index] = executeOneTool(ctx, registry, call, emit)
		}(i, call)
	}
	wg.Wait()
	return results
}

func executeOneTool(ctx context.Context, registry *ToolRegistry, call types.ToolCall, emit provideriface.EventEmitFunc) types.ToolResult {
	args := json.RawMessage(strings.TrimSpace(string(call.Arguments)))
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	invokedPayload, _ := json.Marshal(map[string]any{
		"tool":      call.Name,
		"call_id":   call.ID,
		"arguments": args,
	})
	emit(types.EventToolInvoked, "tool_call", invokedPayload)

	output, err := registry.Execute(ctx, call.Name, call.Arguments)
	isError := err != nil
	if err != nil {
		output = fmt.Sprintf("tool_error: %v", err)
	}

	visibleOutput := output
	var projection *toolOutputProjection
	if !isError {
		projection = parseToolOutputProjection(output)
		if projection != nil {
			visibleOutput = projection.ModelOutput
		}
	}
	visibleOutput = capToolOutput(visibleOutput)

	resultPayloadData := map[string]any{
		"tool":       call.Name,
		"call_id":    call.ID,
		"is_error":   isError,
		"output_len": len(visibleOutput),
		"output":     visibleOutput,
	}
	if projection != nil {
		resultPayloadData["output_projection"] = projection.Metadata
		resultPayloadData["full_output_len"] = len(projection.DurableOutput)
		resultPayloadData["full_output_sha256"] = toolOutputSHA256Hex(projection.DurableOutput)
		fullOutput, truncated := capDurableToolOutput(projection.DurableOutput)
		resultPayloadData["full_output"] = fullOutput
		resultPayloadData["full_output_truncated"] = truncated
	}
	resultPayload, _ := json.Marshal(resultPayloadData)
	emit(types.EventToolResult, "tool_call", resultPayload)

	return types.ToolResult{CallID: call.ID, Output: visibleOutput, IsError: isError}
}

func capToolOutput(output string) string {
	const maxToolOutput = 100 * 1024
	if len(output) <= maxToolOutput {
		return output
	}
	return output[:maxToolOutput] + fmt.Sprintf("\n\n[output truncated — %d bytes total, showing first %d bytes]", len(output), maxToolOutput)
}

type toolOutputProjection struct {
	ModelOutput   string
	DurableOutput string
	Metadata      map[string]any
}

func parseToolOutputProjection(output string) *toolOutputProjection {
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		return nil
	}
	rawSentinel, ok := decoded["__choir_tool_projection"]
	if !ok {
		return nil
	}
	var sentinel bool
	if err := json.Unmarshal(rawSentinel, &sentinel); err != nil || !sentinel {
		return nil
	}
	modelOutput := marshalRawProjectionValue(decoded["model_output"])
	if strings.TrimSpace(modelOutput) == "" {
		modelOutput = output
	}
	durableOutput := marshalRawProjectionValue(decoded["durable_output"])
	if strings.TrimSpace(durableOutput) == "" {
		durableOutput = output
	}
	metadata := map[string]any{}
	if raw := decoded["projection"]; len(raw) > 0 {
		_ = json.Unmarshal(raw, &metadata)
	}
	return &toolOutputProjection{ModelOutput: modelOutput, DurableOutput: durableOutput, Metadata: metadata}
}

func marshalRawProjectionValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var compact any
	if err := json.Unmarshal(raw, &compact); err != nil {
		return strings.TrimSpace(string(raw))
	}
	data, err := json.Marshal(compact)
	if err != nil {
		return strings.TrimSpace(string(raw))
	}
	return string(data)
}

func capDurableToolOutput(output string) (string, bool) {
	const maxDurableToolOutput = 512 * 1024
	if len(output) <= maxDurableToolOutput {
		return output, false
	}
	return output[:maxDurableToolOutput] + fmt.Sprintf("\n\n[durable output truncated — %d bytes total, showing first %d bytes]", len(output), maxDurableToolOutput), true
}

func toolOutputSHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
