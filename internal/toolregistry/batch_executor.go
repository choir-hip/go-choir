package toolregistry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ExecuteToolBatch executes a provider batch under the authoritative tool
// execution policy and returns results in provider call order.
func ExecuteToolBatch(ctx context.Context, registry *ToolRegistry, calls []types.ToolCall, emit provideriface.EventEmitFunc) []types.ToolResult {
	results := make([]types.ToolResult, len(calls))
	skipped := plannedToolSkips(ctx, calls)

	if shouldExecuteToolsSequentially(calls) {
		profile := ExecutionContextFrom(ctx).Profile
		successfulTextureEditCallID := ""
		for i, call := range calls {
			skipReason := skipped[i]
			if skipReason == "" && profile == agentprofile.Texture && isTextureWriteToolName(call.Name) && successfulTextureEditCallID != "" {
				skipReason = fmt.Sprintf("tool_notice:duplicate Texture write tool %s in this Texture turn skipped after call %s; one canonical document mutation is allowed per revision run", call.Name, successfulTextureEditCallID)
			}
			results[i] = executeOneTool(ctx, registry, call, skipReason, emit)
			if skipReason == "" && profile == agentprofile.Texture && isTextureWriteToolName(call.Name) && !results[i].IsError && IsStructuredToolSuccess(results[i].Output) {
				successfulTextureEditCallID = call.ID
			}
		}
		return results
	}

	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(index int, call types.ToolCall) {
			defer wg.Done()
			results[index] = executeOneTool(ctx, registry, call, skipped[index], emit)
		}(i, call)
	}
	wg.Wait()
	return results
}

func executeOneTool(ctx context.Context, registry *ToolRegistry, call types.ToolCall, skipReason string, emit provideriface.EventEmitFunc) types.ToolResult {
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

	isError := false
	output := skipReason
	if skipReason != "" {
		if strings.HasPrefix(skipReason, "tool_notice:") {
			output = strings.TrimSpace(strings.TrimPrefix(skipReason, "tool_notice:"))
		} else {
			isError = true
		}
	} else {
		var err error
		output, err = registry.Execute(ctx, call.Name, call.Arguments)
		if err != nil {
			output = fmt.Sprintf("tool_error: %v", err)
			isError = true
		}
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
	case "bash", "write_file", "patch_texture", "rewrite_texture", "spawn_agent", "cancel_agent", "request_super_execution", "request_email_draft", "product_api_request", "publish_app_change_package", "update_coagent", "save_evidence":
		return true
	default:
		return false
	}
}

func isTextureWriteToolName(name string) bool {
	switch strings.TrimSpace(name) {
	case "patch_texture", "rewrite_texture":
		return true
	default:
		return false
	}
}

func plannedToolSkips(ctx context.Context, calls []types.ToolCall) map[int]string {
	profile := ExecutionContextFrom(ctx).Profile
	if profile == "" || len(calls) == 0 {
		return nil
	}
	skipped := make(map[int]string)
	setSkip := func(index int, reason string) {
		if _, exists := skipped[index]; !exists {
			skipped[index] = reason
		}
	}

	if profile == agentprofile.Conductor {
		firstTexture := -1
		for i, call := range calls {
			if call.Name != "spawn_agent" {
				continue
			}
			if toolCallSpawnProfile(call) == agentprofile.Texture {
				if firstTexture == -1 {
					firstTexture = i
				} else {
					setSkip(i, "tool_notice: conductor already routed this prompt to texture; duplicate texture route skipped")
				}
			}
		}
		if firstTexture != -1 {
			for i, call := range calls {
				if i == firstTexture || call.Name != "spawn_agent" {
					continue
				}
				setSkip(i, "tool_notice: conductor routed this prompt to texture; texture owns downstream researcher/super requests")
			}
		}
	}
	planSideEffectToolSkips(profile, calls, setSkip)
	if len(skipped) == 0 {
		return nil
	}
	return skipped
}

func planSideEffectToolSkips(profile string, calls []types.ToolCall, setSkip func(index int, reason string)) {
	seenSuperSpawn := map[string]int{}
	seenCoagentUpdate := map[string]int{}
	seenExport := map[string]int{}
	seenBash := map[string]int{}
	seenTextureResearcherSpawn := map[string]int{}

	for i, call := range calls {
		switch call.Name {
		case "patch_texture", "rewrite_texture":
		case "bash":
			if profile != agentprofile.Super && profile != agentprofile.CoSuper {
				continue
			}
			key := normalizedToolCallArgs(call)
			if key == "" {
				continue
			}
			if previous, exists := seenBash[key]; exists {
				setSkip(i, fmt.Sprintf("tool_error: duplicate bash command already planned in this turn at call %s; wait for the first result instead of running it twice", calls[previous].ID))
				continue
			}
			seenBash[key] = i
		case "spawn_agent":
			switch profile {
			case agentprofile.Texture:
				key, ok := toolCallTextureResearcherSpawnKey(call)
				if !ok {
					continue
				}
				if previous, exists := seenTextureResearcherSpawn[key]; exists {
					setSkip(i, fmt.Sprintf("tool_notice: duplicate texture researcher spawn for %s already planned in this turn at call %s; one worker for this exact objective is enough", key, calls[previous].ID))
					continue
				}
				seenTextureResearcherSpawn[key] = i
			case agentprofile.Super:
				key, ok := toolCallSuperCoSuperSpawnKey(call)
				if !ok {
					continue
				}
				if previous, exists := seenSuperSpawn[key]; exists {
					setSkip(i, fmt.Sprintf("tool_error: duplicate spawn_agent for %s already planned in this turn at call %s; reuse that child instead of launching or reusing it again", key, calls[previous].ID))
					continue
				}
				seenSuperSpawn[key] = i
			}
		case "publish_app_change_package":
			if profile != agentprofile.Super && profile != agentprofile.CoSuper {
				continue
			}
			key := normalizedToolCallArgs(call)
			if key == "" {
				continue
			}
			if previous, exists := seenExport[key]; exists {
				setSkip(i, fmt.Sprintf("tool_error: duplicate publish_app_change_package payload already planned in this turn at call %s; one package publication attempt per candidate state is allowed", calls[previous].ID))
				continue
			}
			seenExport[key] = i
		case "update_coagent":
			key := normalizedToolCallArgs(call)
			if key == "" {
				continue
			}
			if previous, exists := seenCoagentUpdate[key]; exists {
				setSkip(i, fmt.Sprintf("tool_error: duplicate update_coagent payload already planned in this turn at call %s; one addressed durable update is enough", calls[previous].ID))
				continue
			}
			seenCoagentUpdate[key] = i
		}
	}
}

func toolCallSuperCoSuperSpawnKey(call types.ToolCall) (string, bool) {
	var in struct {
		Role      string `json:"role"`
		Profile   string `json:"profile"`
		Slot      string `json:"slot"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal(call.Arguments, &in); err != nil {
		return "", false
	}
	profile := agentprofile.Canonical(in.Profile)
	if profile == "" {
		profile = agentprofile.Canonical(in.Role)
	}
	if profile != agentprofile.CoSuper {
		return "", false
	}
	slot := normalizeCoSuperSlot(in.Slot)
	if slot == "" {
		return "", false
	}
	return profile + ":" + slot + ":" + strings.TrimSpace(in.ChannelID), true
}

func toolCallTextureResearcherSpawnKey(call types.ToolCall) (string, bool) {
	var in struct {
		Role      string `json:"role"`
		Profile   string `json:"profile"`
		ChannelID string `json:"channel_id"`
		Objective string `json:"objective"`
	}
	if err := json.Unmarshal(call.Arguments, &in); err != nil {
		return "", false
	}
	profile := agentprofile.Canonical(in.Profile)
	if profile == "" {
		profile = agentprofile.Canonical(in.Role)
	}
	if profile != agentprofile.Researcher {
		return "", false
	}
	channelID := strings.TrimSpace(in.ChannelID)
	objective := strings.Join(strings.Fields(strings.TrimSpace(in.Objective)), " ")
	if objective == "" {
		return "", false
	}
	return profile + ":" + channelID + ":" + objective, true
}

func normalizedToolCallArgs(call types.ToolCall) string {
	raw := strings.TrimSpace(string(call.Arguments))
	if raw == "" {
		return "{}"
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return raw
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return raw
	}
	return string(encoded)
}

func toolCallSpawnProfile(call types.ToolCall) string {
	var in struct {
		Role    string `json:"role"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal(call.Arguments, &in); err != nil {
		return ""
	}
	profile := agentprofile.Canonical(in.Profile)
	if profile == "" {
		profile = agentprofile.Canonical(in.Role)
	}
	return profile
}

func normalizeCoSuperSlot(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "implementation", "implementer", "worker", "writer", "builder":
		return "implementation"
	case "verifier", "verification", "reviewer", "review", "checker", "tester":
		return "verifier"
	default:
		return ""
	}
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
