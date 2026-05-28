package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// ToolLoopProvider extends the Provider interface with tool-calling
// capabilities. When the LLM returns a tool_use stop reason, the
// tool-calling loop needs to:
//  1. Parse the tool calls from the response
//  2. Execute them via the ToolRegistry
//  3. Feed the results back into the next LLM call
//
// This interface separates the tool-loop orchestration (owned by the
// runtime) from the LLM API mechanics (owned by the provider). The
// BridgeProvider implements this interface when wrapping a real LLM
// provider; the StubProvider implements it with optional tool simulation.
type ToolLoopProvider interface {
	Provider

	// CallWithTools sends a request with tool definitions and conversation
	// history, returning a response that may contain tool calls. This is the
	// primitive used by the tool-calling loop: each iteration calls
	// CallWithTools, inspects the stop reason, and either executes tools
	// or returns the final text.
	CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error)
}

// ToolLoopRequest is the request shape for the tool-calling loop. It carries
// the full conversation history including prior tool results, the available
// tool definitions, and the system prompt.
type ToolLoopRequest struct {
	// Provider is the provider identifier for gateway-routed requests.
	Provider string `json:"provider,omitempty"`

	// Model is the per-run model resolved from runtime/user-computer policy.
	Model string `json:"model,omitempty"`

	// ReasoningEffort is the provider-specific per-run reasoning control.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	// System is the system prompt (potentially including the tool catalog).
	System string `json:"system"`

	// Messages is the conversation history in Anthropic Messages format.
	// Each entry is a raw JSON message object with role and content fields.
	Messages []json.RawMessage `json:"messages"`

	// ToolDefinitions is the list of available tool schemas.
	ToolDefinitions []ToolDefinition `json:"tool_definitions"`

	// ToolChoice optionally constrains provider tool selection for this call.
	// Supported values are provider-dependent. Shared OpenAI-compatible modes
	// are "auto", "none", and "required"; "function:<name>" means the next
	// provider call must select that exact tool when the adapter supports exact
	// tool choice.
	ToolChoice string `json:"tool_choice,omitempty"`

	// MaxTokens is the maximum output tokens for this call.
	MaxTokens int `json:"max_tokens"`
}

// ToolLoopResponse is the response from a single LLM call in the tool-calling
// loop. It may contain text output, tool calls, or both, depending on the
// stop reason.
type ToolLoopResponse struct {
	// ID is the provider-assigned response identifier.
	ID string `json:"id"`

	// StopReason is why the model stopped: "tool_use", "end_turn", "max_tokens",
	// or other provider-specific reasons.
	StopReason string `json:"stop_reason"`

	// Text is the concatenated text content from the response. May be empty
	// if the model only produced tool calls.
	Text string `json:"text"`

	// ReasoningContent is hidden provider context returned by reasoning models.
	// Some OpenAI-compatible tool loops require this field to be passed back on
	// the next assistant turn. It is not user-facing answer text.
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// ToolCalls contains the tool invocation requests from the provider.
	// Non-empty only when StopReason is "tool_use".
	ToolCalls []types.ToolCall `json:"tool_calls,omitempty"`

	// Usage contains token usage information.
	Usage TokenUsage `json:"usage"`

	// Model is the model that produced the response.
	Model string `json:"model"`
}

// TokenUsage tracks token counts for a tool-loop response.
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// InjectUserTurnsFunc allows the runtime to splice additional user turns into a
// running loop between model iterations. This is used for runtime-owned inbox
// delivery: queued messages are threaded in as normal user turns rather than
// polled by the agent.
type InjectUserTurnsFunc func(finalCheckpoint bool) ([]json.RawMessage, error)

// ToolLoopMemoryHooks lets the runtime persist and rebuild provider context
// around the tool loop without making the tool loop depend on a storage layer.
type ToolLoopMemoryHooks struct {
	BeforeProviderCall func(ctx context.Context, messages []json.RawMessage) ([]json.RawMessage, error)
	AfterAppendMessage func(ctx context.Context, role string, msg json.RawMessage) error
	OnProviderError    func(ctx context.Context, messages []json.RawMessage, err error) ([]json.RawMessage, bool, error)
}

type toolLoopOptions struct {
	memoryHooks       ToolLoopMemoryHooks
	llmConfig         LLMSelection
	initialToolChoice string
	terminalTools     map[string]bool
}

type pendingRequiredTool struct {
	Name        string
	Instruction string
	Attempts    int
}

const maxRequiredNextToolRetries = 2

var requiredNextToolCallTimeout = 45 * time.Second

// ToolLoopOption configures optional tool-loop behavior.
type ToolLoopOption func(*toolLoopOptions)

// WithToolLoopMemoryHooks configures durable memory callbacks for context
// persistence, compaction, and context-overflow retry.
func WithToolLoopMemoryHooks(hooks ToolLoopMemoryHooks) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.memoryHooks = hooks
	}
}

// WithToolLoopLLMConfig carries the per-run provider/model choice resolved
// from computer-owned model policy into each provider request.
func WithToolLoopLLMConfig(config LLMSelection) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.llmConfig = config
	}
}

// WithInitialToolChoice constrains tool use only on the first provider call.
// This is useful for appagents that must take a tool-mediated action before
// ordinary assistant text can be meaningful, while still allowing later turns
// in the same loop to finish naturally after tool results are appended.
func WithInitialToolChoice(choice string) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.initialToolChoice = strings.TrimSpace(choice)
	}
}

// WithTerminalToolSuccesses makes successful tool calls terminal for this loop
// unless a tool result explicitly declares a next_required_tool. This is for
// side-effect tools whose successful execution is the run's observable result.
func WithTerminalToolSuccesses(names ...string) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		if len(names) == 0 {
			return
		}
		if opts.terminalTools == nil {
			opts.terminalTools = make(map[string]bool, len(names))
		}
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name != "" {
				opts.terminalTools[name] = true
			}
		}
	}
}

// maxToolLoopIterations prevents infinite tool-calling loops. If the LLM
// keeps requesting tool use without reaching an end_turn, we bail out
// after this many iterations. This is a temporary stability ceiling while
// worker leases, cancellation, compaction, and budget backpressure mature
// toward longer or budget-governed execution.
const (
	maxToolLoopIterations = 200
)

var providerRateLimitRetryDelays = []time.Duration{
	5 * time.Second,
	20 * time.Second,
	60 * time.Second,
}

// RunToolLoop executes the tool-calling loop: call the LLM, execute any
// requested tools, feed results back, and repeat until the model returns
// end_turn or the context is cancelled.
//
// This is adapted from Cogent's runToolLoop but simplified for go-choir:
//   - Runtime-owned memory hooks can persist/rebuild conversation state.
//   - No steer/interrupt mechanism (runs are atomic from the runtime's
//     perspective; steering belongs in the appagent layer).
//   - Tool execution emits observable events through the event bus.
//
// Returns the final text result, total token usage, and any error.
func RunToolLoop(ctx context.Context, provider ToolLoopProvider, registry *ToolRegistry, initialMessages []json.RawMessage, systemPrompt string, maxTokens int, emit EventEmitFunc, injectUserTurns InjectUserTurnsFunc, opts ...ToolLoopOption) (string, TokenUsage, error) {
	var totalUsage TokenUsage
	options := toolLoopOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	messages := make([]json.RawMessage, len(initialMessages))
	copy(messages, initialMessages)

	toolDefs := []ToolDefinition{}
	if registry != nil {
		toolDefs = registry.Definitions()
		systemPrompt = buildSystemPromptWithTools(systemPrompt, registry)
	}
	forceInitialToolChoiceRetry := false
	var requiredNextTool *pendingRequiredTool

	appendMessage := func(role string, msg json.RawMessage) error {
		messages = append(messages, msg)
		if options.memoryHooks.AfterAppendMessage != nil {
			if err := options.memoryHooks.AfterAppendMessage(ctx, role, msg); err != nil {
				return err
			}
		}
		return nil
	}
	appendInjected := func(injected []json.RawMessage) error {
		for _, msg := range injected {
			if err := appendMessage(runMemoryMessageRole(msg), msg); err != nil {
				return err
			}
		}
		return nil
	}
	appendAssistantText := func(text string, reasoningContent string) error {
		if text == "" {
			return nil
		}
		msg := map[string]any{
			"role":    "assistant",
			"content": buildAssistantContent(text, nil),
		}
		if strings.TrimSpace(reasoningContent) != "" {
			msg["reasoning_content"] = reasoningContent
		}
		assistantMsg, _ := json.Marshal(msg)
		return appendMessage("assistant", assistantMsg)
	}
	appendRequiredNextToolReminder := func(required pendingRequiredTool, reason string) error {
		text := requiredNextToolReminderText(required, reason)
		msg, _ := json.Marshal(map[string]any{
			"role": "user",
			"content": []map[string]string{{
				"type": "text",
				"text": text,
			}},
		})
		return appendMessage("user", msg)
	}

	for i := 0; i < maxToolLoopIterations; i++ {
		if options.memoryHooks.BeforeProviderCall != nil {
			rebuilt, err := options.memoryHooks.BeforeProviderCall(ctx, messages)
			if err != nil {
				return "", totalUsage, fmt.Errorf("tool loop memory before iteration %d: %w", i, err)
			}
			messages = rebuilt
		}

		req := ToolLoopRequest{
			Provider:        options.llmConfig.Provider,
			Model:           options.llmConfig.Model,
			ReasoningEffort: options.llmConfig.ReasoningEffort,
			System:          systemPrompt,
			Messages:        messages,
			ToolDefinitions: toolDefs,
			MaxTokens:       maxTokens,
		}
		if len(toolDefs) > 0 && requiredNextTool != nil {
			req.ToolChoice = exactRequiredToolChoice(requiredNextTool.Name)
		} else if len(toolDefs) > 0 && options.initialToolChoice != "" && (i == 0 || forceInitialToolChoiceRetry) {
			req.ToolChoice = options.initialToolChoice
		}
		forceInitialToolChoiceRetry = false

		if emit != nil {
			lastUserText := extractLastUserMessage(messages)
			preCallPayload, _ := json.Marshal(map[string]any{
				"iteration":            i + 1,
				"phase":                "provider_call_started",
				"messages":             len(messages),
				"tools":                len(toolDefs),
				"tool_names":           toolDefinitionNames(toolDefs),
				"system_chars":         len(systemPrompt),
				"system_sha256":        toolOutputSHA256Hex(systemPrompt),
				"system_preview":       truncatePromptSnippet(systemPrompt, 2000),
				"last_user_chars":      len(lastUserText),
				"last_user_sha256":     toolOutputSHA256Hex(lastUserText),
				"last_user_text":       truncatePromptSnippet(lastUserText, 4000),
				"message_roles":        toolLoopMessageRoles(messages),
				"max_tokens":           req.MaxTokens,
				"max_tokens_requested": req.MaxTokens > 0,
				"llm_provider":         options.llmConfig.Provider,
				"llm_model":            options.llmConfig.Model,
				"llm_reasoning_effort": options.llmConfig.ReasoningEffort,
				"tool_choice":          req.ToolChoice,
				"model_policy":         "run_metadata",
			})
			emit(types.EventRunProgress, "provider_call", preCallPayload)
		}

		// Call the LLM with current conversation state. Required continuation
		// turns are narrow function-call obligations; bound them separately so
		// an exact-tool prompt cannot hold a supervisor-visible chain open for
		// the gateway's full inference timeout.
		providerCtx := ctx
		var cancelProviderCall context.CancelFunc
		var requiredTimeout *pendingRequiredTool
		if requiredNextTool != nil && requiredNextToolCallTimeout > 0 {
			requiredTimeout = requiredNextTool
			providerCtx, cancelProviderCall = context.WithTimeout(ctx, requiredNextToolCallTimeout)
		}
		resp, err := callToolLoopProviderWithRetries(providerCtx, provider, req, emit)
		providerCallErr := providerCtx.Err()
		if cancelProviderCall != nil {
			cancelProviderCall()
		}
		if err != nil {
			if requiredTimeout != nil && ctx.Err() == nil && providerCallErr != nil {
				requiredTimeout.Attempts++
				if requiredTimeout.Attempts > maxRequiredNextToolRetries {
					return "", totalUsage, fmt.Errorf("tool loop: required next tool %q was not called after %d retries", requiredTimeout.Name, maxRequiredNextToolRetries)
				}
				requiredNextTool = requiredTimeout
				if appendErr := appendRequiredNextToolReminder(*requiredTimeout, "provider_timed_out_before_required_tool"); appendErr != nil {
					return "", totalUsage, fmt.Errorf("tool loop persist required-next-tool timeout retry: %w", appendErr)
				}
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"required_tool": requiredTimeout.Name,
						"attempt":       requiredTimeout.Attempts,
						"reason":        "provider_timed_out_before_required_tool",
						"timeout_ms":    requiredNextToolCallTimeout.Milliseconds(),
					})
					emit(types.EventRunRetry, "required_next_tool", payload)
				}
				continue
			}
			if options.memoryHooks.OnProviderError != nil {
				rebuilt, retry, hookErr := options.memoryHooks.OnProviderError(ctx, messages, err)
				if hookErr != nil {
					return "", totalUsage, hookErr
				}
				if retry {
					messages = rebuilt
					continue
				}
			}
			return "", totalUsage, fmt.Errorf("tool loop iteration %d: %w", i, err)
		}

		// Accumulate token usage.
		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens

		// Emit progress event for this iteration.
		progressPayload, _ := json.Marshal(map[string]any{
			"iteration":            i + 1,
			"stop_reason":          resp.StopReason,
			"tool_calls":           len(resp.ToolCalls),
			"tool_call_names":      toolCallNames(resp.ToolCalls),
			"response_text_chars":  len(resp.Text),
			"response_text":        truncatePromptSnippet(resp.Text, 2000),
			"model":                resp.Model,
			"llm_provider":         options.llmConfig.Provider,
			"llm_model":            options.llmConfig.Model,
			"llm_reasoning_effort": options.llmConfig.ReasoningEffort,
			"model_policy":         "run_metadata",
		})
		emit(types.EventRunProgress, "tool_loop", progressPayload)

		switch resp.StopReason {
		case "tool_use":
			if len(resp.ToolCalls) == 0 {
				return "", totalUsage, fmt.Errorf("tool loop: provider returned tool_use without tool calls")
			}

			// Append the assistant's response (with tool calls) to conversation.
			assistantPayload := map[string]any{
				"role":    "assistant",
				"content": buildAssistantContent(resp.Text, resp.ToolCalls),
			}
			if strings.TrimSpace(resp.ReasoningContent) != "" {
				assistantPayload["reasoning_content"] = resp.ReasoningContent
			}
			assistantMsg, _ := json.Marshal(assistantPayload)
			if err := appendMessage("assistant", assistantMsg); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist assistant message: %w", err)
			}

			activeRequired := requiredNextTool
			requiredCalled := requiredToolCalled(activeRequired, resp.ToolCalls)

			// Execute tools and collect results.
			toolResults := executeTools(ctx, registry, resp.ToolCalls, emit)

			// Append tool results as a user message (per Anthropic Messages API convention).
			toolResultMsg, _ := json.Marshal(map[string]any{
				"role":    "user",
				"content": buildToolResultContent(toolResults),
			})
			if err := appendMessage("user", toolResultMsg); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist tool result message: %w", err)
			}
			if injectUserTurns != nil {
				injected, err := injectUserTurns(false)
				if err != nil {
					return "", totalUsage, fmt.Errorf("tool loop inject turns after tools: %w", err)
				}
				if err := appendInjected(injected); err != nil {
					return "", totalUsage, fmt.Errorf("tool loop persist injected turns after tools: %w", err)
				}
			}
			if activeRequired != nil && requiredCalled {
				requiredNextTool = nil
			}
			if next, ok := extractRequiredNextTool(toolResults); ok {
				if requiredToolSucceeded(next.Name, resp.ToolCalls, toolResults) {
					if emit != nil {
						payload, _ := json.Marshal(map[string]any{
							"required_tool": next.Name,
							"reason":        "tool_result_declared_required_next_tool_satisfied_in_batch",
						})
						emit(types.EventRunProgress, "required_next_tool_satisfied", payload)
					}
					if terminalTools := successfulTerminalToolNames(resp.ToolCalls, toolResults, options.terminalTools); len(terminalTools) > 0 {
						if emit != nil {
							payload, _ := json.Marshal(map[string]any{
								"iteration": i + 1,
								"tools":     terminalTools,
								"reason":    "terminal_tool_success",
							})
							emit(types.EventRunProgress, "terminal_tool_success", payload)
						}
						return resp.Text, totalUsage, nil
					}
				} else {
					requiredNextTool = &next
					if err := appendRequiredNextToolReminder(next, "tool_result_declared_required_next_tool"); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist required-next-tool reminder: %w", err)
					}
					if emit != nil {
						payload, _ := json.Marshal(map[string]any{
							"required_tool": next.Name,
							"reason":        "tool_result_declared_required_next_tool",
						})
						emit(types.EventRunRetry, "required_next_tool", payload)
					}
				}
			} else if activeRequired != nil && !requiredCalled {
				activeRequired.Attempts++
				if activeRequired.Attempts > maxRequiredNextToolRetries {
					return "", totalUsage, fmt.Errorf("tool loop: required next tool %q was not called after %d retries", activeRequired.Name, maxRequiredNextToolRetries)
				}
				requiredNextTool = activeRequired
				if err := appendRequiredNextToolReminder(*activeRequired, "model_called_different_tool"); err != nil {
					return "", totalUsage, fmt.Errorf("tool loop persist required-next-tool retry: %w", err)
				}
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"required_tool": activeRequired.Name,
						"attempt":       activeRequired.Attempts,
						"reason":        "model_called_different_tool",
					})
					emit(types.EventRunRetry, "required_next_tool", payload)
				}
			} else if terminalTools := successfulTerminalToolNames(resp.ToolCalls, toolResults, options.terminalTools); len(terminalTools) > 0 {
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"iteration": i + 1,
						"tools":     terminalTools,
						"reason":    "terminal_tool_success",
					})
					emit(types.EventRunProgress, "terminal_tool_success", payload)
				}
				return resp.Text, totalUsage, nil
			}

			log.Printf("tool loop: iteration %d, executed %d tools, continuing", i+1, len(resp.ToolCalls))

		case "end_turn", "":
			if requiredNextTool != nil && len(toolDefs) > 0 {
				requiredNextTool.Attempts++
				if requiredNextTool.Attempts > maxRequiredNextToolRetries {
					return "", totalUsage, fmt.Errorf("tool loop: required next tool %q was not called after %d retries", requiredNextTool.Name, maxRequiredNextToolRetries)
				}
				if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
					return "", totalUsage, fmt.Errorf("tool loop persist assistant ignored required-next-tool text: %w", err)
				}
				if err := appendRequiredNextToolReminder(*requiredNextTool, "model_ended_turn_without_required_tool"); err != nil {
					return "", totalUsage, fmt.Errorf("tool loop persist required-next-tool retry: %w", err)
				}
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"required_tool": requiredNextTool.Name,
						"attempt":       requiredNextTool.Attempts,
						"reason":        "model_ended_turn_without_required_tool",
					})
					emit(types.EventRunRetry, "required_next_tool", payload)
				}
				continue
			}
			if injectUserTurns != nil {
				injected, err := injectUserTurns(true)
				if err != nil {
					return "", totalUsage, fmt.Errorf("tool loop final inbox checkpoint: %w", err)
				}
				if len(injected) > 0 {
					if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist assistant final message: %w", err)
					}
					if err := appendInjected(injected); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist final injected turns: %w", err)
					}
					continue
				}
			}
			if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist assistant final message: %w", err)
			}
			// Normal completion — return the text.
			return resp.Text, totalUsage, nil

		case "max_tokens":
			if req.ToolChoice != "" && len(resp.ToolCalls) == 0 && i < maxToolLoopIterations-1 {
				var retryAttempt int
				if requiredNextTool != nil {
					requiredNextTool.Attempts++
					retryAttempt = requiredNextTool.Attempts
					if requiredNextTool.Attempts > maxRequiredNextToolRetries {
						return "", totalUsage, fmt.Errorf("tool loop: required next tool %q was not called after %d retries", requiredNextTool.Name, maxRequiredNextToolRetries)
					}
				}
				if emit != nil {
					retryPayload, _ := json.Marshal(map[string]any{
						"iteration":   i + 1,
						"reason":      "required_tool_not_called",
						"tool_choice": req.ToolChoice,
						"max_tokens":  req.MaxTokens,
						"attempt":     retryAttempt,
					})
					emit(types.EventRunRetry, "required_tool_call", retryPayload)
				}
				reminderMsg, _ := json.Marshal(map[string]any{
					"role": "user",
					"content": []map[string]string{{
						"type": "text",
						"text": "The previous model turn stopped at max_tokens without calling a required tool. Call exactly one available tool now. Do not write prose.",
					}},
				})
				if err := appendMessage("user", reminderMsg); err != nil {
					return "", totalUsage, fmt.Errorf("tool loop persist required-tool retry message: %w", err)
				}
				forceInitialToolChoiceRetry = true
				continue
			}
			return resp.Text, totalUsage, fmt.Errorf("tool loop: model stopped at max_tokens (iteration %d)", i+1)

		default:
			return "", totalUsage, fmt.Errorf("tool loop: unsupported stop reason %q (iteration %d)", resp.StopReason, i+1)
		}
	}

	return "", totalUsage, fmt.Errorf("tool loop: exceeded %d iterations without end_turn", maxToolLoopIterations)
}

func requiredToolCalled(required *pendingRequiredTool, calls []types.ToolCall) bool {
	if required == nil || strings.TrimSpace(required.Name) == "" {
		return false
	}
	for _, call := range calls {
		if call.Name == required.Name {
			return true
		}
	}
	return false
}

func requiredToolSucceeded(name string, calls []types.ToolCall, results []types.ToolResult) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	limit := len(calls)
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		if calls[i].Name == name && !results[i].IsError {
			return true
		}
	}
	return false
}

func exactRequiredToolChoice(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "required"
	}
	return "function:" + name
}

func extractRequiredNextTool(results []types.ToolResult) (pendingRequiredTool, bool) {
	for _, result := range results {
		if result.IsError {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(result.Output), &decoded); err != nil {
			continue
		}
		name := firstNonEmpty(
			stringMapValue(decoded, "next_required_tool"),
			stringMapValue(decoded, "next_tool"),
		)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if required, ok := decoded["delegation_required"].(bool); ok && !required {
			continue
		}
		instruction := strings.TrimSpace(firstNonEmpty(
			stringMapValue(decoded, "next_instruction"),
			requiredNextToolArgsInstruction(decoded),
		))
		return pendingRequiredTool{Name: name, Instruction: instruction}, true
	}
	return pendingRequiredTool{}, false
}

func successfulTerminalToolNames(calls []types.ToolCall, results []types.ToolResult, terminalTools map[string]bool) []string {
	if len(terminalTools) == 0 {
		return nil
	}
	limit := len(calls)
	if len(results) < limit {
		limit = len(results)
	}
	names := make([]string, 0, limit)
	seen := make(map[string]bool, limit)
	for i := 0; i < limit; i++ {
		name := strings.TrimSpace(calls[i].Name)
		if name == "" || !terminalTools[name] || results[i].IsError || !isStructuredToolSuccess(results[i].Output) {
			continue
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

func isStructuredToolSuccess(output string) bool {
	var decoded map[string]any
	return json.Unmarshal([]byte(strings.TrimSpace(output)), &decoded) == nil && len(decoded) > 0
}

func requiredNextToolArgsInstruction(decoded map[string]any) string {
	args, _ := decoded["next_required_args"].(map[string]any)
	if args == nil {
		args, _ = decoded["start_args"].(map[string]any)
	}
	if len(args) == 0 {
		return ""
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	return "Use these required arguments as the base: " + string(encoded)
}

func requiredNextToolReminderText(required pendingRequiredTool, reason string) string {
	var b strings.Builder
	b.WriteString("Runtime-required continuation: call ")
	b.WriteString(required.Name)
	b.WriteString(" now. Do not end the turn and do not write a prose summary before this tool call.")
	if strings.TrimSpace(required.Instruction) != "" {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(required.Instruction))
	}
	if strings.TrimSpace(reason) != "" {
		b.WriteString("\n\nReason: ")
		b.WriteString(reason)
	}
	return b.String()
}

func callToolLoopProviderWithRetries(ctx context.Context, provider ToolLoopProvider, req ToolLoopRequest, emit EventEmitFunc) (*ToolLoopResponse, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		resp, err := provider.CallWithTools(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isProviderRateLimitError(err) || attempt >= len(providerRateLimitRetryDelays) {
			return nil, lastErr
		}
		delay := providerRateLimitRetryDelays[attempt]
		payload, _ := json.Marshal(map[string]any{
			"reason":   "provider_rate_limit",
			"attempt":  attempt + 1,
			"delay_ms": delay.Milliseconds(),
			"error":    err.Error(),
		})
		if emit != nil {
			emit(types.EventRunRetry, "provider_rate_limit", payload)
		}
		if err := sleepContext(ctx, delay); err != nil {
			return nil, err
		}
	}
}

func isProviderRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "429") ||
		strings.Contains(text, "too many requests") ||
		strings.Contains(text, "rate limit") ||
		strings.Contains(text, "rate_limited")
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func toolDefinitionNames(defs []ToolDefinition) []string {
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		name := strings.TrimSpace(def.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func toolCallNames(calls []types.ToolCall) []string {
	names := make([]string, 0, len(calls))
	for _, call := range calls {
		name := strings.TrimSpace(call.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func toolLoopMessageRoles(messages []json.RawMessage) []string {
	roles := make([]string, 0, len(messages))
	for _, raw := range messages {
		role := runMemoryMessageRole(raw)
		if role == "" {
			role = "unknown"
		}
		roles = append(roles, role)
	}
	return roles
}

// buildAssistantContent constructs the content blocks for an assistant
// message that may contain text and tool calls.
func buildAssistantContent(text string, toolCalls []types.ToolCall) []any {
	var content []any

	// Add text content if present.
	if text != "" {
		content = append(content, map[string]string{
			"type": "text",
			"text": text,
		})
	}

	// Add tool_use blocks for each tool call.
	for _, call := range toolCalls {
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    call.ID,
			"name":  call.Name,
			"input": json.RawMessage(call.Arguments),
		})
	}

	return content
}

// buildToolResultContent constructs the content blocks for a user message
// containing tool results, following the Anthropic Messages API convention.
func buildToolResultContent(results []types.ToolResult) []any {
	content := make([]any, 0, len(results))
	for _, result := range results {
		entry := map[string]any{
			"type":        "tool_result",
			"tool_use_id": result.CallID,
			"content":     result.Output,
		}
		if result.IsError {
			entry["is_error"] = true
		}
		content = append(content, entry)
	}
	return content
}

// --- ToolLoopProvider adapter for providers that don't natively support it ---

// toolLoopAdapter wraps a basic Provider to implement ToolLoopProvider by
// converting tool-loop calls into the simpler Provider.Execute interface.
// This is used when a provider (like the StubProvider or BridgeProvider)
// doesn't directly implement CallWithTools.
//
// The adapter converts the tool-loop request into a RunRecord-like call
// through the Provider.Execute method. It does NOT support actual tool-calling
// (it ignores tool definitions and always returns end_turn), so it should
// only be used when the runtime wants the executeTask path without the
// tool-calling loop.
type toolLoopAdapter struct {
	Provider
}

// CallWithTools implements ToolLoopProvider by delegating to the underlying
// Provider's Execute method. The adapter extracts the last user message as
// the prompt and returns a single-turn end_turn response.
func (a *toolLoopAdapter) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	// Extract the last user message as the prompt for the simple provider.
	prompt := extractLastUserMessage(req.Messages)

	task := &types.RunRecord{
		Prompt: prompt,
	}

	var capturedText string
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		// Capture delta text for the response.
		if kind == types.EventRunDelta {
			var delta struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(payload, &delta); err == nil && delta.Text != "" {
				capturedText += delta.Text
			}
		}
	}

	err := a.Execute(ctx, task, emit)
	if err != nil {
		return nil, err
	}

	result := capturedText
	if result == "" {
		result = task.Result
	}

	return &ToolLoopResponse{
		StopReason: "end_turn",
		Text:       result,
		Usage:      TokenUsage{},
	}, nil
}

// extractLastUserMessage finds the last user-role message in the conversation
// history and returns its text content. Falls back to an empty string.
func extractLastUserMessage(messages []json.RawMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		var msg struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		}
		if err := json.Unmarshal(messages[i], &msg); err != nil {
			continue
		}
		if msg.Role == "user" {
			return extractTextFromContent(msg.Content)
		}
	}
	return ""
}

// extractTextFromContent extracts text from a message content field, which
// may be a string, an array of content blocks, or null.
func extractTextFromContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var text string
		for _, item := range v {
			if block, ok := item.(map[string]any); ok {
				if blockType, _ := block["type"].(string); blockType == "text" {
					if t, _ := block["text"].(string); t != "" {
						text += t
					}
				}
				// Skip tool_result blocks when extracting text.
			}
		}
		return text
	default:
		return ""
	}
}

// asToolLoopProvider converts a Provider to a ToolLoopProvider. If the
// provider already implements ToolLoopProvider, it is returned directly.
// Otherwise, it is wrapped in a toolLoopAdapter that converts tool-loop
// calls into simple provider calls.
func asToolLoopProvider(p Provider) ToolLoopProvider {
	if tlp, ok := p.(ToolLoopProvider); ok {
		return tlp
	}
	return &toolLoopAdapter{Provider: p}
}
