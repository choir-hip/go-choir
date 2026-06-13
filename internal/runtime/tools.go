package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// ToolFunc is the execution contract for in-process tools. Tools are Go
// function calls, not CLI subprocesses (mission constraint: no CLI loop).
// The function receives the raw JSON arguments from the provider and returns
// a text result or an error.
type ToolFunc func(ctx context.Context, args json.RawMessage) (string, error)

// Tool describes a callable tool plus its LLM-facing schema metadata.
// Adapted from Cogent's Tool struct but simplified for go-choir: no core/tool
// distinction, no Anthropic/OpenAI schema variants (those belong in the
// provider bridge), and no native-session profile tracking.
type Tool struct {
	// Name is the unique tool identifier used in LLM tool_use responses.
	Name string `json:"name"`

	// Description is a human-readable summary of what the tool does,
	// included in the system prompt for LLM tool discovery.
	Description string `json:"description,omitempty"`

	// Parameters is the JSON Schema object describing the tool's input
	// parameters. If nil, defaults to an empty object schema.
	Parameters map[string]any `json:"parameters,omitempty"`

	// Func is the Go function that executes the tool. Must be non-nil.
	Func ToolFunc `json:"-"`
}

// Validate checks that the tool has a name and a non-nil function.
func (t Tool) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("tool name must not be empty")
	}
	if t.Func == nil {
		return fmt.Errorf("tool %q has nil func", t.Name)
	}
	return nil
}

// ToolDefinition is the LLM-facing schema for a tool, without the Go
// function. This is what gets included in API requests and system prompts.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// Definition returns the LLM-facing definition for this tool.
func (t Tool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  cloneSchemaMap(t.Parameters),
	}
}

// ToolRegistry manages the set of available tools for the runtime loop.
// Tools are registered once at startup and looked up by name during the
// tool-calling loop when the LLM returns tool_use stop reasons.
//
// Adapted from Cogent's ToolRegistry but simplified:
//   - No core/activated tool distinction (go-choir sends all tool schemas
//     up front; LLM tool discovery happens through the system prompt catalog).
//   - No Anthropic/OpenAI schema methods (those belong in the provider bridge).
//   - Thread-safe for concurrent lookup during parallel tool execution.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	order []string // sorted names for deterministic catalog output
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// NewToolRegistryWithTools creates a tool registry with the given tools
// pre-registered. Returns an error if any tool fails validation.
func NewToolRegistryWithTools(tools ...Tool) (*ToolRegistry, error) {
	r := NewToolRegistry()
	for _, tool := range tools {
		if err := r.Register(tool); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// MustNewToolRegistry creates a tool registry with the given tools or panics.
func MustNewToolRegistry(tools ...Tool) *ToolRegistry {
	r, err := NewToolRegistryWithTools(tools...)
	if err != nil {
		panic(err)
	}
	return r
}

// Register adds a tool to the registry. Returns an error if the tool fails
// validation or a tool with the same name is already registered.
func (r *ToolRegistry) Register(tool Tool) error {
	if err := tool.Validate(); err != nil {
		return err
	}

	// Default to empty object schema if no parameters specified.
	if len(tool.Parameters) == 0 {
		tool.Parameters = jsonSchemaObject(nil, nil, false)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[tool.Name]; exists {
		return fmt.Errorf("tool %q already registered", tool.Name)
	}
	r.tools[tool.Name] = tool
	r.order = append(r.order, tool.Name)
	sort.Strings(r.order)
	return nil
}

// Lookup returns the tool with the given name, or false if not found.
func (r *ToolRegistry) Lookup(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// Execute runs the named tool with the given arguments. Returns an error
// if the tool is not found or if execution fails.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("tool %q not found", name)
	}
	return tool.Func(ctx, args)
}

// Tools returns all registered tools in sorted order.
func (r *ToolRegistry) Tools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.tools[name])
	}
	return out
}

// Definitions returns the LLM-facing definitions for all registered tools.
func (r *ToolRegistry) Definitions() []ToolDefinition {
	tools := r.Tools()
	out := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		out = append(out, tool.Definition())
	}
	return out
}

// Catalog returns a compact one-line-per-tool description suitable for
// inclusion in the system prompt. The LLM reads this to know what tools
// are available and calls them by name. Adapted from Cogent's Catalog()
// but without the core/activated distinction.
func (r *ToolRegistry) Catalog() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	b.WriteString("Available tools:\n")
	for _, name := range r.order {
		tool := r.tools[name]
		desc := tool.Description
		if len(desc) > 80 {
			desc = desc[:80] + "..."
		}
		fmt.Fprintf(&b, "- %s — %s\n", name, desc)
	}
	return b.String()
}

// Size returns the number of registered tools.
func (r *ToolRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// --- Schema helpers ---

// jsonSchemaObject creates a JSON Schema object with the given properties,
// required fields, and additionalProperties setting.
func jsonSchemaObject(properties map[string]any, required []string, additionalProperties bool) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": additionalProperties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// cloneSchemaMap deep-clones a JSON Schema map.
func cloneSchemaMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneSchemaValue(v)
	}
	return out
}

func cloneSchemaValue(v any) any {
	switch vv := v.(type) {
	case map[string]any:
		return cloneSchemaMap(vv)
	case []any:
		out := make([]any, len(vv))
		for i, item := range vv {
			out[i] = cloneSchemaValue(item)
		}
		return out
	default:
		return v
	}
}

// buildSystemPromptWithTools constructs the system prompt for the tool-calling
// loop by appending the tool catalog to the base system prompt. This gives
// the LLM visibility into available tools without requiring separate tool
// schema negotiation on each turn.
func buildSystemPromptWithTools(basePrompt string, registry *ToolRegistry) string {
	if registry == nil || registry.Size() == 0 {
		return basePrompt
	}
	return basePrompt + "\n\n" + registry.Catalog()
}

// executeTools runs a batch of tool calls from the LLM response in parallel,
// emitting events for each invocation. Returns the tool results for feeding
// back into the LLM conversation. Adapted from Cogent's executeTools but
// simplified for go-choir: no steer draining, no consecutive-error tracking,
// and no tool activation (all tools are always available).
func executeTools(ctx context.Context, registry *ToolRegistry, calls []types.ToolCall, emit EventEmitFunc) []types.ToolResult {
	results := make([]types.ToolResult, len(calls))
	skipped := plannedToolSkips(ctx, calls)

	if shouldExecuteToolsSequentially(calls) {
		for i, call := range calls {
			results[i] = executeOneTool(ctx, registry, call, skipped[i], emit)
		}
		executeRequiredToolTransitions(ctx, registry, calls, results, emit)
		return results
	}

	// Execute tool calls in parallel — results collected in order.
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, c types.ToolCall) {
			defer wg.Done()
			results[idx] = executeOneTool(ctx, registry, c, skipped[idx], emit)
		}(i, call)
	}
	wg.Wait()

	executeRequiredToolTransitions(ctx, registry, calls, results, emit)

	return results
}

func executeOneTool(ctx context.Context, registry *ToolRegistry, call types.ToolCall, skipReason string, emit EventEmitFunc) types.ToolResult {
	// Emit full tool inputs: Trace is owner-scoped and is the proof surface
	// for workflow tests, so summaries are not enough.
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

	// Emit tool.result event after execution.
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

	return types.ToolResult{
		CallID:  call.ID,
		Output:  visibleOutput,
		IsError: isError,
	}
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
	case "bash",
		"write_file",
		"edit_vtext",
		"spawn_agent",
		"cancel_agent",
		"request_super_execution",
		"request_email_draft",
		"request_worker_vm",
		"product_api_request",
		"delegate_worker_vm",
		"start_worker_delegation",
		"observe_worker_delegation",
		"finish_worker_delegation",
		"cancel_worker_delegation",
		"publish_app_change_package",
		"update_coagent",
		"save_evidence":
		return true
	default:
		return false
	}
}

// Historical versions executed a hidden request_worker_vm -> delegate_worker_vm
// transition here. That made super block inside one tool loop while the worker
// ran, which prevented supervision, VText clarification, cancellation, and
// concurrent observation. Worker delegation is now explicit and asynchronous:
// request_worker_vm leases, start_worker_delegation starts, and observe/finish
// collect evidence. This hook intentionally does no worker handoff.
func executeRequiredToolTransitions(ctx context.Context, registry *ToolRegistry, calls []types.ToolCall, results []types.ToolResult, emit EventEmitFunc) {
	return
}

func capToolOutput(output string) string {
	const maxToolOutput = 100 * 1024 // 100KB
	if len(output) <= maxToolOutput {
		return output
	}
	return output[:maxToolOutput] + fmt.Sprintf(
		"\n\n[output truncated — %d bytes total, showing first %d bytes]",
		len(output), maxToolOutput)
}

type toolOutputProjection struct {
	ModelOutput   string
	DurableOutput string
	Metadata      map[string]any
}

func toolProjectionResultJSON(modelOutput any, durableOutput any, metadata map[string]any) (string, error) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	return toolResultJSON(map[string]any{
		"__choir_tool_projection": true,
		"model_output":            modelOutput,
		"durable_output":          durableOutput,
		"projection":              metadata,
	})
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
	return &toolOutputProjection{
		ModelOutput:   modelOutput,
		DurableOutput: durableOutput,
		Metadata:      metadata,
	}
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
	return output[:maxDurableToolOutput] + fmt.Sprintf(
		"\n\n[durable output truncated — %d bytes total, showing first %d bytes]",
		len(output), maxDurableToolOutput), true
}

func toolOutputSHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

type requiredWorkerDelegation struct {
	Args json.RawMessage
}

func buildRequiredWorkerDelegation(ctx context.Context, output string) (requiredWorkerDelegation, bool) {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		return requiredWorkerDelegation{}, false
	}
	nextTool, _ := decoded["next_required_tool"].(string)
	if strings.TrimSpace(nextTool) != "delegate_worker_vm" {
		return requiredWorkerDelegation{}, false
	}
	args := map[string]any{}
	if rawArgs, _ := decoded["next_required_args"].(map[string]any); rawArgs != nil {
		for key, value := range rawArgs {
			args[key] = value
		}
	}
	if _, ok := args["profile"]; !ok {
		args["profile"] = AgentProfileVSuper
	}
	objective := requiredWorkerDelegationObjective(ctx, decoded)
	if objective == "" {
		return requiredWorkerDelegation{}, false
	}
	args["objective"] = objective
	raw, err := json.Marshal(args)
	if err != nil {
		return requiredWorkerDelegation{}, false
	}
	return requiredWorkerDelegation{Args: raw}, true
}

func requiredWorkerDelegationObjective(ctx context.Context, requestOutput map[string]any) string {
	var parts []string
	if purpose, _ := requestOutput["purpose"].(string); strings.TrimSpace(purpose) != "" {
		parts = append(parts, "Worker VM purpose:\n"+strings.TrimSpace(purpose))
	}
	if handle, _ := requestOutput["handle"].(map[string]any); handle != nil {
		if purpose, _ := handle["purpose"].(string); strings.TrimSpace(purpose) != "" {
			parts = append(parts, "Worker VM purpose:\n"+strings.TrimSpace(purpose))
		}
	}
	if rec := ctxRunRecord(ctx); rec != nil && strings.TrimSpace(rec.Prompt) != "" {
		parts = append(parts, "Full parent super objective:\n"+strings.TrimSpace(rec.Prompt))
	}
	if len(parts) == 0 {
		return ""
	}
	return "The parent super leased this worker VM and the runtime is completing the required delegation transition.\n\n" + strings.Join(parts, "\n\n")
}

func augmentWorkerRequestWithDelegation(requestOutput, delegateOutput string, delegateIsError bool) string {
	var decoded map[string]any
	if err := json.Unmarshal([]byte(requestOutput), &decoded); err != nil {
		return requestOutput
	}
	status := "worker_delegated"
	if delegateIsError {
		status = "worker_delegation_failed"
	}
	decoded["delegation_status"] = status
	decoded["chained_required_tool"] = "delegate_worker_vm"
	decoded["chained_delegation_is_error"] = delegateIsError
	var parsed any
	if err := json.Unmarshal([]byte(delegateOutput), &parsed); err == nil {
		decoded["chained_delegation_output"] = parsed
		if parsedMap, ok := parsed.(map[string]any); ok {
			propagateChainedWorkerDelegation(decoded, parsedMap)
		}
	} else {
		decoded["chained_delegation_output"] = delegateOutput
	}
	out, err := json.Marshal(decoded)
	if err != nil {
		return requestOutput
	}
	return string(out)
}

func propagateChainedWorkerDelegation(requestOutput, delegateOutput map[string]any) {
	if status, _ := delegateOutput["status"].(string); strings.TrimSpace(status) != "" {
		requestOutput["delegation_status"] = strings.TrimSpace(status)
	}
	for _, key := range []string{
		"app_change_packages",
		"completion_blocker",
		"terminal_error",
		"reviewable_package_observed",
		"worker_update_checkpoint",
		"worker_event_error",
		"worker_event_summary",
		"worker_spawned_profiles",
		"worker_channel_message_count",
		"worker_child_run_ids",
		"worker_child_statuses",
		"worker_child_status_errors",
	} {
		if value, ok := delegateOutput[key]; ok {
			requestOutput[key] = value
		}
	}
	if _, ok := requestOutput["app_change_packages"]; !ok {
		requestOutput["app_change_packages"] = []any{}
	}
	if requestOutput["completion_blocker"] != nil || requestOutput["terminal_error"] != nil || requestOutput["delegation_status"] == "worker_run_incomplete" {
		requestOutput["delegation_incomplete"] = true
	}
}

func plannedToolSkips(ctx context.Context, calls []types.ToolCall) map[int]string {
	profile := canonicalAgentProfile(stringFromToolContext(ctx, toolCtxProfile))
	if profile == "" || len(calls) == 0 {
		return nil
	}
	skipped := make(map[int]string)
	setSkip := func(index int, reason string) {
		if _, exists := skipped[index]; !exists {
			skipped[index] = reason
		}
	}

	switch profile {
	case AgentProfileConductor:
		firstVText := -1
		for i, call := range calls {
			if call.Name != "spawn_agent" {
				continue
			}
			if toolCallSpawnProfile(call) == AgentProfileVText {
				if firstVText == -1 {
					firstVText = i
				} else {
					setSkip(i, "tool_notice: conductor already routed this prompt to vtext; duplicate vtext route skipped")
				}
			}
		}
		if firstVText != -1 {
			for i, call := range calls {
				if i == firstVText || call.Name != "spawn_agent" {
					continue
				}
				setSkip(i, "tool_notice: conductor routed this prompt to vtext; vtext owns downstream researcher/super requests")
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
	seenVSuperSpawn := map[string]int{}
	seenCoagentUpdate := map[string]int{}
	seenExport := map[string]int{}
	seenBash := map[string]int{}
	seenDelegateWorker := map[string]int{}
	seenVTextResearcherSpawn := map[string]int{}
	firstVTextEdit := -1

	for i, call := range calls {
		switch call.Name {
		case "edit_vtext":
			if profile != AgentProfileVText {
				continue
			}
			if firstVTextEdit != -1 {
				setSkip(i, fmt.Sprintf("tool_notice:duplicate edit_vtext in this VText turn skipped after call %s; one canonical document mutation is allowed per revision run", calls[firstVTextEdit].ID))
				continue
			}
			firstVTextEdit = i
		case "bash":
			if profile != AgentProfileSuper && profile != AgentProfileVSuper && profile != AgentProfileCoSuper {
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
			case AgentProfileVText:
				key, ok := toolCallVTextResearcherSpawnKey(call)
				if !ok {
					continue
				}
				if previous, exists := seenVTextResearcherSpawn[key]; exists {
					setSkip(i, fmt.Sprintf("tool_notice: duplicate vtext researcher spawn for %s already planned in this turn at call %s; one worker for this exact objective is enough", key, calls[previous].ID))
					continue
				}
				seenVTextResearcherSpawn[key] = i
			case AgentProfileVSuper:
				key, ok := toolCallVSuperCoSuperSpawnKey(call)
				if !ok {
					continue
				}
				if previous, exists := seenVSuperSpawn[key]; exists {
					setSkip(i, fmt.Sprintf("tool_error: duplicate spawn_agent for %s already planned in this turn at call %s; reuse that child instead of launching or reusing it again", key, calls[previous].ID))
					continue
				}
				seenVSuperSpawn[key] = i
			}
		case "publish_app_change_package":
			if profile != AgentProfileSuper && profile != AgentProfileVSuper && profile != AgentProfileCoSuper {
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
		case "delegate_worker_vm", "start_worker_delegation":
			if profile != AgentProfileSuper {
				continue
			}
			key := normalizedToolCallArgs(call)
			if key == "" {
				continue
			}
			if previous, exists := seenDelegateWorker[key]; exists {
				if call.Name == "start_worker_delegation" {
					notice, _ := json.Marshal(map[string]any{
						"status":           "duplicate_start_ignored",
						"state":            "pending",
						"deduped":          true,
						"dedupe_reason":    "start_worker_delegation_already_planned_in_turn",
						"previous_call_id": calls[previous].ID,
						"next_tools":       []string{"observe_worker_delegation", "finish_worker_delegation", "cancel_worker_delegation"},
					})
					setSkip(i, "tool_notice:"+string(notice))
					continue
				}
				setSkip(i, fmt.Sprintf("tool_error: duplicate %s payload already planned in this turn at call %s; wait for the first worker result instead of starting the same worker delegation twice", call.Name, calls[previous].ID))
				continue
			}
			seenDelegateWorker[key] = i
		}
	}
}

func toolCallVSuperCoSuperSpawnKey(call types.ToolCall) (string, bool) {
	var in struct {
		Role      string `json:"role"`
		Profile   string `json:"profile"`
		Slot      string `json:"slot"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal(call.Arguments, &in); err != nil {
		return "", false
	}
	profile := canonicalAgentProfile(in.Profile)
	if profile == "" {
		profile = canonicalAgentProfile(in.Role)
	}
	if profile != AgentProfileCoSuper {
		return "", false
	}
	slot := normalizeVSuperCoSuperSlot(in.Slot)
	if slot == "" {
		return "", false
	}
	return profile + ":" + slot + ":" + strings.TrimSpace(in.ChannelID), true
}

func toolCallVTextResearcherSpawnKey(call types.ToolCall) (string, bool) {
	var in struct {
		Role      string `json:"role"`
		Profile   string `json:"profile"`
		ChannelID string `json:"channel_id"`
		Objective string `json:"objective"`
	}
	if err := json.Unmarshal(call.Arguments, &in); err != nil {
		return "", false
	}
	profile := canonicalAgentProfile(in.Profile)
	if profile == "" {
		profile = canonicalAgentProfile(in.Role)
	}
	if profile != AgentProfileResearcher {
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
	profile := canonicalAgentProfile(in.Profile)
	if profile == "" {
		profile = canonicalAgentProfile(in.Role)
	}
	return profile
}
