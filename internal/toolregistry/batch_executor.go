package toolregistry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// slowToolLogThreshold is the elapsed bound above which a single tool call
// logs its name + duration. Below it per-tool logging would flood the
// console on every iteration; above it the call is the dominant cost and
// the stall signature worth naming (sa1 drain pace-collapse residual).
const slowToolLogThreshold = 30 * time.Second

// ExecuteToolBatch executes a provider batch under the authoritative tool
// execution policy and returns results in provider call order.
func ExecuteToolBatch(ctx context.Context, registry *ToolRegistry, calls []types.ToolCall, emit provideriface.EventEmitFunc) []types.ToolResult {
	results := make([]types.ToolResult, len(calls))

	// Stage 1 (pre-dispatch): narrow assigned-Engineering admission grammar
	// (settlement gate item 4). Statically refuse contradictory or forbidden
	// batches with none run.
	if refusalErr := validateEngineeringBatchAdmission(calls); refusalErr != nil {
		for i, call := range calls {
			results[i] = types.ToolResult{
				CallID:  call.ID,
				Output:  fmt.Sprintf("tool_error: %v", refusalErr),
				IsError: true,
			}
		}
		return results
	}

	if shouldExecuteToolsSequentially(calls) {
		for i, call := range calls {
			results[i] = executeOneTool(ctx, registry, call, emit)
		}
		return results
	}

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
	// tool.invoked is published on the bus for the live ticker and is not
	// appended (types.LiveOnly, journal cut R3): the arguments and the start
	// ride the tool.result below.
	argFields := toolArgumentFields(args)
	invokedData := map[string]any{
		"tool":    call.Name,
		"call_id": call.ID,
	}
	for k, v := range argFields {
		invokedData[k] = v
	}
	invokedPayload, err := json.Marshal(invokedData)
	if err != nil {
		invokedPayload = fallbackToolPayload(call.Name, call.ID, nil)
	}
	emit(types.EventToolInvoked, "tool_call", invokedPayload)

	isError := false
	execution := ExecutionContextFrom(ctx)
	execution.ToolCallID = strings.TrimSpace(call.ID)
	callCtx := WithExecutionContext(ctx, execution)
	toolStart := time.Now()
	output, err := registry.Execute(callCtx, call.Name, call.Arguments)
	elapsed := time.Since(toolStart)
	if elapsed > slowToolLogThreshold {
		// A single tool call that holds the serialized store lock for
		// tens of seconds stalls a Management drain carrier's whole
		// iteration. Log which call owns the stall so a slow-path
		// residual names the offending tool, not just "the store".
		log.Printf("tool loop: slow tool call tool=%s call_id=%s dur=%s err=%v", call.Name, call.ID, elapsed.Round(time.Millisecond), err)
	}
	if err != nil {
		output = fmt.Sprintf("tool_error: %v", err)
		isError = true
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
		// What tool.invoked carried and pairing the two gave (journal cut R3).
		// The arguments ride as toolArgumentFields (invalid JSON as a string).
		"started_at":  toolStart.UTC(),
		"duration_ms": elapsed.Milliseconds(),
	}
	if projection != nil {
		resultPayloadData["output_projection"] = projection.Metadata
		resultPayloadData["full_output_len"] = len(projection.DurableOutput)
		resultPayloadData["full_output_sha256"] = toolOutputSHA256Hex(projection.DurableOutput)
		fullOutput, truncated := capDurableToolOutput(projection.DurableOutput)
		resultPayloadData["full_output"] = fullOutput
		resultPayloadData["full_output_truncated"] = truncated
	}
	for k, v := range argFields {
		resultPayloadData[k] = v
	}
	resultPayload, marshalErr := json.Marshal(resultPayloadData)
	if marshalErr != nil {
		// Never let one field erase the only durable record of the call.
		resultPayload = fallbackToolPayload(call.Name, call.ID, map[string]any{
			"is_error":      isError,
			"output_len":    len(visibleOutput),
			"output":        visibleOutput,
			"started_at":    toolStart.UTC(),
			"duration_ms":   elapsed.Milliseconds(),
			"marshal_error": marshalErr.Error(),
		})
	}
	emit(types.EventToolResult, "tool_call", resultPayload)

	return types.ToolResult{CallID: call.ID, Output: visibleOutput, IsError: isError}
}

// toolArgumentFields returns the payload fields that carry a call's
// arguments. Valid JSON is embedded as is under "arguments". A provider can
// emit malformed arguments (canonicalJSON passes invalid JSON through), and a
// json.RawMessage of invalid bytes makes the whole payload fail to marshal, so
// those bytes ride as a string under "arguments_raw" with "arguments_invalid".
func toolArgumentFields(args json.RawMessage) map[string]any {
	if json.Valid(args) {
		return map[string]any{"arguments": args}
	}
	return map[string]any{"arguments_raw": string(args), "arguments_invalid": true}
}

// fallbackToolPayload is the payload when the full one cannot be marshaled: it
// keeps the tool name and call id plus the plain fields in extra.
func fallbackToolPayload(tool, callID string, extra map[string]any) json.RawMessage {
	data := map[string]any{"tool": tool, "call_id": callID}
	for k, v := range extra {
		data[k] = v
	}
	payload, err := json.Marshal(data)
	if err != nil {
		payload, _ = json.Marshal(map[string]any{"tool": tool, "call_id": callID, "marshal_error": err.Error()})
	}
	return payload
}

func shouldExecuteToolsSequentially(calls []types.ToolCall) bool {
	for _, call := range calls {
		if toolRequiresSequentialTurnExecution(call.Name) {
			return true
		}
	}
	return false
}

func toolRequiresSequentialTurnExecution(name string) bool {
	switch strings.TrimSpace(name) {
	case "capsule_go_eval", "desk_go_eval":
		return true
	default:
		return false
	}
}

func validateEngineeringBatchAdmission(calls []types.ToolCall) error {
	evalCount := 0
	for _, call := range calls {
		if strings.TrimSpace(call.Name) == "capsule_go_eval" {
			evalCount++
		}
	}
	if evalCount > 1 {
		return fmt.Errorf("admission_grammar_refusal: at most one capsule_go_eval call allowed per turn (found %d)", evalCount)
	}
	return nil
}
func capToolOutput(output string) string {
	const maxToolOutput = 100 * 1024
	if len(output) <= maxToolOutput {
		return output
	}
	// The cut backs off to a rune boundary so it never splits a UTF-8
	// sequence (LR lever 6, L6-3).
	cut := maxToolOutput
	for cut > 0 && !utf8.RuneStart(output[cut]) {
		cut--
	}
	return output[:cut] + fmt.Sprintf("\n\n[output truncated — %d bytes total, showing first %d bytes]", len(output), cut)
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
