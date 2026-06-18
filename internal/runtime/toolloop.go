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

type ToolLoopCompletionState struct {
	Messages  []json.RawMessage
	FinalText string
	Attempts  int
}

type ToolLoopCompletionGuardResult struct {
	Continue    bool
	Reason      string
	Instruction string
}

type ToolLoopCompletionGuardFunc func(ctx context.Context, state ToolLoopCompletionState) (ToolLoopCompletionGuardResult, error)

// ToolLoopBudget is a cumulative kill switch for one tool-loop activation. It is
// intentionally role-neutral: callers attach policy, while RunToolLoop enforces
// provider-call, token, and elapsed-time limits uniformly.
type ToolLoopBudget struct {
	Label            string
	MaxProviderCalls int
	MaxInputTokens   int
	MaxOutputTokens  int
	MaxTotalTokens   int
	MaxElapsed       time.Duration
}

type toolLoopOptions struct {
	memoryHooks                   ToolLoopMemoryHooks
	llmConfig                     LLMSelection
	providerPreconditionFallbacks []LLMSelection
	initialToolChoice             string
	terminalTools                 map[string]bool
	completionGuard               ToolLoopCompletionGuardFunc
	budget                        ToolLoopBudget
}

type pendingRequiredTool struct {
	Name        string
	Instruction string
	Attempts    int
}

const maxRequiredNextToolRetries = 2

const maxTokenContinuationRetries = 3

const maxCompletionGuardRetries = 2

var requiredNextToolCallTimeout = 45 * time.Second

const requiredNextToolDefaultMaxTokens = 2048

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

// WithProviderPreconditionFallbacks configures alternate model selections for
// provider request-shape precondition or provider-availability failures. The
// tool loop only uses these after preserving the same tool obligation on the
// original selection first.
func WithProviderPreconditionFallbacks(fallbacks ...LLMSelection) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.providerPreconditionFallbacks = append([]LLMSelection(nil), fallbacks...)
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
// unless the same tool batch yields a recognized mechanical required-next-tool
// protocol. This is for side-effect tools whose successful execution is the
// run's observable result.
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

// WithCompletionGuard lets a caller reject an end_turn as incomplete and append
// an ordinary user turn describing the remaining obligation. The guard does not
// choose a tool; it keeps the tool loop uniform while letting app-level policy
// define what counts as a complete turn.
func WithCompletionGuard(guard ToolLoopCompletionGuardFunc) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.completionGuard = guard
	}
}

// WithToolLoopBudget applies cumulative loop limits. Zero-valued fields are
// unbounded for that dimension; at least one positive field enables the budget.
func WithToolLoopBudget(budget ToolLoopBudget) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		budget.Label = strings.TrimSpace(budget.Label)
		opts.budget = budget
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
	relaxInitialExactToolChoice := false
	initialToolChoiceAttempts := 0
	activeLLMConfig := options.llmConfig
	preconditionFallbackIndex := 0
	var requiredNextTool *pendingRequiredTool
	var maxTokenContinuationAttempts int
	var completionGuardAttempts int
	var partialTextFragments []string
	loopStartedAt := time.Now()

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
	combinedFinalText := func(finalText string) string {
		parts := make([]string, 0, len(partialTextFragments)+1)
		for _, part := range partialTextFragments {
			if strings.TrimSpace(part) != "" {
				parts = append(parts, part)
			}
		}
		if strings.TrimSpace(finalText) != "" {
			parts = append(parts, finalText)
		}
		return strings.Join(parts, "\n")
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
	appendRequiredInitialToolChoiceReminder := func(requiredName, reason string) error {
		text := fmt.Sprintf("The previous model turn did not call the required initial tool %q (%s). Call exactly that available tool now. Do not write prose and do not call any other tool.", requiredName, reason)
		msg, _ := json.Marshal(map[string]any{
			"role": "user",
			"content": []map[string]string{{
				"type": "text",
				"text": text,
			}},
		})
		return appendMessage("user", msg)
	}
	appendCompletionGuardReminder := func(result ToolLoopCompletionGuardResult) error {
		text := strings.TrimSpace(result.Instruction)
		if text == "" {
			text = "The previous model turn ended before satisfying an app-level completion obligation. Continue with the next legitimate tool action or record an audit-worthy blocker/decision before ending."
		}
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
		if err := checkToolLoopBudgetBeforeProvider(options.budget, i, loopStartedAt); err != nil {
			emitToolLoopBudgetExhausted(emit, options.budget, i, totalUsage, err)
			return "", totalUsage, err
		}
		if options.memoryHooks.BeforeProviderCall != nil {
			rebuilt, err := options.memoryHooks.BeforeProviderCall(ctx, messages)
			if err != nil {
				return "", totalUsage, fmt.Errorf("tool loop memory before iteration %d: %w", i, err)
			}
			messages = rebuilt
		}

		req := ToolLoopRequest{
			Provider:        activeLLMConfig.Provider,
			Model:           activeLLMConfig.Model,
			ReasoningEffort: activeLLMConfig.ReasoningEffort,
			System:          systemPrompt,
			Messages:        messages,
			ToolDefinitions: toolDefs,
			MaxTokens:       maxTokens,
		}
		initialToolChoiceApplied := false
		if len(toolDefs) > 0 && requiredNextTool != nil {
			req.ToolChoice = exactRequiredToolChoice(requiredNextTool.Name)
			if req.MaxTokens <= 0 {
				// Normal Fireworks agent turns intentionally omit max_tokens, but
				// forced continuation turns are only supposed to emit a tool call.
				// Give that narrow call a finite budget so provider defaults cannot
				// spend the whole required-tool timeout in open-ended reasoning.
				req.MaxTokens = requiredNextToolDefaultMaxTokens
			}
		} else if len(toolDefs) > 0 && options.initialToolChoice != "" && (i == 0 || forceInitialToolChoiceRetry) {
			initialToolChoiceApplied = true
			req.ToolChoice = options.initialToolChoice
			if relaxInitialExactToolChoice && isExactRequiredToolChoice(req.ToolChoice) {
				req.ToolChoice = "required"
			}
		}
		if initialToolChoiceApplied {
			if name, ok := exactRequiredToolChoiceName(options.initialToolChoice); ok {
				req.ToolDefinitions = toolDefinitionsMatchingName(toolDefs, name)
			}
		}
		forceInitialToolChoiceRetry = false

		if emit != nil {
			lastUserText := extractLastUserMessage(messages)
			preCallPayload, _ := json.Marshal(map[string]any{
				"iteration":                            i + 1,
				"phase":                                "provider_call_started",
				"messages":                             len(messages),
				"tools":                                len(req.ToolDefinitions),
				"tool_names":                           toolDefinitionNames(req.ToolDefinitions),
				"system_chars":                         len(systemPrompt),
				"system_sha256":                        toolOutputSHA256Hex(systemPrompt),
				"system_preview":                       truncatePromptSnippet(systemPrompt, 2000),
				"last_user_chars":                      len(lastUserText),
				"last_user_sha256":                     toolOutputSHA256Hex(lastUserText),
				"last_user_text":                       truncatePromptSnippet(lastUserText, 4000),
				"message_roles":                        toolLoopMessageRoles(messages),
				"max_tokens":                           req.MaxTokens,
				"max_tokens_requested":                 req.MaxTokens > 0,
				"llm_provider":                         activeLLMConfig.Provider,
				"llm_model":                            activeLLMConfig.Model,
				"llm_reasoning_effort":                 activeLLMConfig.ReasoningEffort,
				"tool_choice":                          req.ToolChoice,
				"model_policy":                         "run_metadata",
				"provider_precondition_fallback_count": len(options.providerPreconditionFallbacks),
				"provider_precondition_fallback_index": preconditionFallbackIndex,
				"tool_loop_budget":                     toolLoopBudgetPayload(options.budget),
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
			if isExactInitialToolChoicePreconditionError(req.ToolChoice, err) && !relaxInitialExactToolChoice && i == 0 {
				relaxInitialExactToolChoice = true
				forceInitialToolChoiceRetry = true
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"reason":              "exact_initial_tool_choice_precondition",
						"tool_choice":         req.ToolChoice,
						"retry_tool_choice":   "required",
						"provider_error":      err.Error(),
						"provider_error_kind": "precondition_failed",
					})
					emit(types.EventRunRetry, "provider_tool_choice", payload)
				}
				continue
			}
			if isProviderModelFallbackError(err) && preconditionFallbackIndex < len(options.providerPreconditionFallbacks) {
				next := options.providerPreconditionFallbacks[preconditionFallbackIndex]
				preconditionFallbackIndex++
				if !sameLLMSelection(activeLLMConfig, next) && strings.TrimSpace(next.Provider) != "" && strings.TrimSpace(next.Model) != "" {
					activeLLMConfig = next
					forceInitialToolChoiceRetry = strings.TrimSpace(req.ToolChoice) != ""
					if emit != nil {
						payload, _ := json.Marshal(map[string]any{
							"reason":          providerModelFallbackReason(err),
							"tool_choice":     req.ToolChoice,
							"from_provider":   req.Provider,
							"from_model":      req.Model,
							"to_provider":     next.Provider,
							"to_model":        next.Model,
							"to_reasoning":    next.ReasoningEffort,
							"fallback_index":  preconditionFallbackIndex - 1,
							"fallback_count":  len(options.providerPreconditionFallbacks),
							"provider_error":  err.Error(),
							"fallback_source": next.Source,
						})
						emit(types.EventRunRetry, "provider_model_fallback", payload)
					}
					continue
				}
			}
			return "", totalUsage, fmt.Errorf("tool loop iteration %d: %w", i, err)
		}

		// Accumulate token usage.
		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		if err := checkToolLoopBudgetAfterProvider(options.budget, totalUsage); err != nil {
			emitToolLoopBudgetExhausted(emit, options.budget, i+1, totalUsage, err)
			return "", totalUsage, err
		}

		// Emit progress event for this iteration.
		progressPayload, _ := json.Marshal(map[string]any{
			"iteration":            i + 1,
			"stop_reason":          resp.StopReason,
			"tool_calls":           len(resp.ToolCalls),
			"tool_call_names":      toolCallNames(resp.ToolCalls),
			"response_text_chars":  len(resp.Text),
			"response_text":        truncatePromptSnippet(resp.Text, 2000),
			"model":                resp.Model,
			"llm_provider":         activeLLMConfig.Provider,
			"llm_model":            activeLLMConfig.Model,
			"llm_reasoning_effort": activeLLMConfig.ReasoningEffort,
			"model_policy":         "run_metadata",
		})
		emit(types.EventRunProgress, "tool_loop", progressPayload)

		switch resp.StopReason {
		case "tool_use":
			if len(resp.ToolCalls) == 0 {
				return "", totalUsage, fmt.Errorf("tool loop: provider returned tool_use without tool calls")
			}
			if initialToolChoiceApplied {
				if requiredName, ok := exactRequiredToolChoiceName(options.initialToolChoice); ok && !toolCallsExactlyMatchName(resp.ToolCalls, requiredName) {
					initialToolChoiceAttempts++
					if initialToolChoiceAttempts > maxRequiredNextToolRetries {
						return "", totalUsage, fmt.Errorf("tool loop: required initial tool %q was not called after %d retries", requiredName, maxRequiredNextToolRetries)
					}
					if err := appendRequiredInitialToolChoiceReminder(requiredName, "model_called_different_initial_tool"); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist initial tool-choice retry: %w", err)
					}
					forceInitialToolChoiceRetry = true
					if emit != nil {
						payload, _ := json.Marshal(map[string]any{
							"required_tool": requiredName,
							"called_tools":  toolCallNames(resp.ToolCalls),
							"tool_choice":   req.ToolChoice,
							"reason":        "model_called_different_initial_tool",
							"attempt":       initialToolChoiceAttempts,
						})
						emit(types.EventRunRetry, "initial_tool_choice", payload)
					}
					continue
				}
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
			if initialToolChoiceApplied {
				if requiredName, ok := exactRequiredToolChoiceName(options.initialToolChoice); ok && !requiredToolSucceeded(requiredName, resp.ToolCalls, toolResults) {
					initialToolChoiceAttempts++
					if initialToolChoiceAttempts > maxRequiredNextToolRetries {
						return "", totalUsage, fmt.Errorf("tool loop: required initial tool %q did not succeed after %d retries", requiredName, maxRequiredNextToolRetries)
					}
					if err := appendRequiredInitialToolChoiceReminder(requiredName, "required_initial_tool_failed"); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist failed initial tool-choice retry: %w", err)
					}
					forceInitialToolChoiceRetry = true
					if emit != nil {
						payload, _ := json.Marshal(map[string]any{
							"required_tool": requiredName,
							"tool_choice":   req.ToolChoice,
							"reason":        "required_initial_tool_failed",
							"attempt":       initialToolChoiceAttempts,
						})
						emit(types.EventRunRetry, "initial_tool_choice", payload)
					}
					continue
				}
			}
			if activeRequired != nil && requiredCalled {
				requiredNextTool = nil
			}
			if next, ok := extractRequiredNextTool(resp.ToolCalls, toolResults); ok {
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
						return combinedFinalText(resp.Text), totalUsage, nil
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
				return combinedFinalText(resp.Text), totalUsage, nil
			}

			log.Printf("tool loop: iteration %d, executed %d tools, continuing", i+1, len(resp.ToolCalls))

		case "end_turn", "":
			if initialToolChoiceApplied {
				if requiredName, ok := exactRequiredToolChoiceName(options.initialToolChoice); ok && len(toolDefs) > 0 {
					initialToolChoiceAttempts++
					if initialToolChoiceAttempts > maxRequiredNextToolRetries {
						return "", totalUsage, fmt.Errorf("tool loop: required initial tool %q was not called after %d retries", requiredName, maxRequiredNextToolRetries)
					}
					if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist assistant ignored initial tool-choice text: %w", err)
					}
					if err := appendRequiredInitialToolChoiceReminder(requiredName, "model_ended_turn_without_required_initial_tool"); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist initial tool-choice retry: %w", err)
					}
					forceInitialToolChoiceRetry = true
					if emit != nil {
						payload, _ := json.Marshal(map[string]any{
							"required_tool": requiredName,
							"tool_choice":   req.ToolChoice,
							"reason":        "model_ended_turn_without_required_initial_tool",
							"attempt":       initialToolChoiceAttempts,
						})
						emit(types.EventRunRetry, "initial_tool_choice", payload)
					}
					continue
				}
			}
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
			if options.completionGuard != nil {
				guardResult, err := options.completionGuard(ctx, ToolLoopCompletionState{
					Messages:  append([]json.RawMessage(nil), messages...),
					FinalText: resp.Text,
					Attempts:  completionGuardAttempts,
				})
				if err != nil {
					return "", totalUsage, fmt.Errorf("tool loop completion guard: %w", err)
				}
				if guardResult.Continue {
					completionGuardAttempts++
					if completionGuardAttempts > maxCompletionGuardRetries {
						return "", totalUsage, fmt.Errorf("tool loop: completion guard %q was not satisfied after %d retries", strings.TrimSpace(guardResult.Reason), maxCompletionGuardRetries)
					}
					if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist guarded assistant final message: %w", err)
					}
					if err := appendCompletionGuardReminder(guardResult); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist completion guard reminder: %w", err)
					}
					if emit != nil {
						payload, _ := json.Marshal(map[string]any{
							"attempt": completionGuardAttempts,
							"reason":  strings.TrimSpace(guardResult.Reason),
						})
						emit(types.EventRunRetry, "completion_guard", payload)
					}
					continue
				}
			}
			if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist assistant final message: %w", err)
			}
			// Normal completion — return the text.
			return combinedFinalText(resp.Text), totalUsage, nil

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
			if strings.TrimSpace(resp.Text) == "" {
				return combinedFinalText(resp.Text), totalUsage, fmt.Errorf("tool loop: model stopped at max_tokens without text (iteration %d)", i+1)
			}
			maxTokenContinuationAttempts++
			if maxTokenContinuationAttempts > maxTokenContinuationRetries || i >= maxToolLoopIterations-1 {
				return combinedFinalText(resp.Text), totalUsage, fmt.Errorf("tool loop: model stopped at max_tokens after %d continuation attempts (iteration %d)", maxTokenContinuationAttempts-1, i+1)
			}
			partialTextFragments = append(partialTextFragments, resp.Text)
			if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist max_tokens partial assistant: %w", err)
			}
			continuationMsg, _ := json.Marshal(map[string]any{
				"role": "user",
				"content": []map[string]string{{
					"type": "text",
					"text": "The previous model turn stopped because the provider ended the output. Continue from that partial response. Keep the continuation concise; do not restart, repeat, or produce a giant inventory. If more source evidence is required, call the available tools; otherwise finish the remaining answer.",
				}},
			})
			if err := appendMessage("user", continuationMsg); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist max_tokens continuation message: %w", err)
			}
			if emit != nil {
				payload, _ := json.Marshal(map[string]any{
					"iteration": i + 1,
					"attempt":   maxTokenContinuationAttempts,
					"reason":    "provider_output_stopped_with_partial_text",
				})
				emit(types.EventRunRetry, "max_tokens_continuation", payload)
			}
			continue

		default:
			return "", totalUsage, fmt.Errorf("tool loop: unsupported stop reason %q (iteration %d)", resp.StopReason, i+1)
		}
	}

	return "", totalUsage, fmt.Errorf("tool loop: exceeded %d iterations without end_turn", maxToolLoopIterations)
}

func (budget ToolLoopBudget) active() bool {
	return budget.MaxProviderCalls > 0 ||
		budget.MaxInputTokens > 0 ||
		budget.MaxOutputTokens > 0 ||
		budget.MaxTotalTokens > 0 ||
		budget.MaxElapsed > 0
}

func toolLoopBudgetLabel(budget ToolLoopBudget) string {
	if label := strings.TrimSpace(budget.Label); label != "" {
		return label
	}
	return "tool_loop"
}

func checkToolLoopBudgetBeforeProvider(budget ToolLoopBudget, providerCalls int, startedAt time.Time) error {
	if !budget.active() {
		return nil
	}
	label := toolLoopBudgetLabel(budget)
	if budget.MaxProviderCalls > 0 && providerCalls >= budget.MaxProviderCalls {
		return fmt.Errorf("tool loop budget %q exhausted: provider calls %d reached max %d", label, providerCalls, budget.MaxProviderCalls)
	}
	if budget.MaxElapsed > 0 && time.Since(startedAt) >= budget.MaxElapsed {
		return fmt.Errorf("tool loop budget %q exhausted: elapsed time reached max %s", label, budget.MaxElapsed)
	}
	return nil
}

func checkToolLoopBudgetAfterProvider(budget ToolLoopBudget, usage TokenUsage) error {
	if !budget.active() {
		return nil
	}
	label := toolLoopBudgetLabel(budget)
	if budget.MaxInputTokens > 0 && usage.InputTokens > budget.MaxInputTokens {
		return fmt.Errorf("tool loop budget %q exhausted: input tokens %d exceeded max %d", label, usage.InputTokens, budget.MaxInputTokens)
	}
	if budget.MaxOutputTokens > 0 && usage.OutputTokens > budget.MaxOutputTokens {
		return fmt.Errorf("tool loop budget %q exhausted: output tokens %d exceeded max %d", label, usage.OutputTokens, budget.MaxOutputTokens)
	}
	total := usage.InputTokens + usage.OutputTokens
	if budget.MaxTotalTokens > 0 && total > budget.MaxTotalTokens {
		return fmt.Errorf("tool loop budget %q exhausted: total tokens %d exceeded max %d", label, total, budget.MaxTotalTokens)
	}
	return nil
}

func emitToolLoopBudgetExhausted(emit EventEmitFunc, budget ToolLoopBudget, providerCalls int, usage TokenUsage, cause error) {
	if emit == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"reason":         "tool_loop_budget_exhausted",
		"error":          cause.Error(),
		"provider_calls": providerCalls,
		"input_tokens":   usage.InputTokens,
		"output_tokens":  usage.OutputTokens,
		"total_tokens":   usage.InputTokens + usage.OutputTokens,
		"budget":         toolLoopBudgetPayload(budget),
	})
	emit(types.EventRunProgress, "tool_loop_budget", payload)
}

func toolLoopBudgetPayload(budget ToolLoopBudget) map[string]any {
	payload := map[string]any{
		"active": budget.active(),
	}
	if label := strings.TrimSpace(budget.Label); label != "" {
		payload["label"] = label
	}
	if budget.MaxProviderCalls > 0 {
		payload["max_provider_calls"] = budget.MaxProviderCalls
	}
	if budget.MaxInputTokens > 0 {
		payload["max_input_tokens"] = budget.MaxInputTokens
	}
	if budget.MaxOutputTokens > 0 {
		payload["max_output_tokens"] = budget.MaxOutputTokens
	}
	if budget.MaxTotalTokens > 0 {
		payload["max_total_tokens"] = budget.MaxTotalTokens
	}
	if budget.MaxElapsed > 0 {
		payload["max_elapsed_ms"] = budget.MaxElapsed.Milliseconds()
	}
	return payload
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

func isExactRequiredToolChoice(choice string) bool {
	_, ok := exactRequiredToolChoiceName(choice)
	return ok
}

func exactRequiredToolChoiceName(choice string) (string, bool) {
	name, ok := strings.CutPrefix(strings.TrimSpace(choice), "function:")
	if !ok {
		return "", false
	}
	name = strings.TrimSpace(name)
	return name, name != ""
}

func toolCallsExactlyMatchName(calls []types.ToolCall, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(calls) == 0 {
		return false
	}
	for _, call := range calls {
		if strings.TrimSpace(call.Name) != name {
			return false
		}
	}
	return true
}

func isExactInitialToolChoicePreconditionError(choice string, err error) bool {
	if !isExactRequiredToolChoice(choice) || err == nil {
		return false
	}
	return isProviderPreconditionError(err)
}

func isInitialToolChoicePreconditionError(choice string, err error) bool {
	if strings.TrimSpace(choice) == "" || err == nil {
		return false
	}
	return isProviderPreconditionError(err)
}

func isProviderPreconditionError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "412") ||
		strings.Contains(text, "precondition failed") ||
		strings.Contains(text, "thinking mode does not support this tool_choice")
}

func isProviderAvailabilityError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "402") ||
		strings.Contains(text, "payment required")
}

func isProviderModelFallbackError(err error) bool {
	return isProviderPreconditionError(err) || isProviderAvailabilityError(err)
}

func providerModelFallbackReason(err error) string {
	if isProviderAvailabilityError(err) {
		return "provider_availability_fallback"
	}
	return "provider_precondition_fallback"
}

func sameLLMSelection(a, b LLMSelection) bool {
	return strings.TrimSpace(a.Provider) == strings.TrimSpace(b.Provider) &&
		strings.TrimSpace(a.Model) == strings.TrimSpace(b.Model) &&
		strings.TrimSpace(a.ReasoningEffort) == strings.TrimSpace(b.ReasoningEffort) &&
		a.MaxTokens == b.MaxTokens
}

func toolDefinitionsMatchingName(defs []ToolDefinition, name string) []ToolDefinition {
	name = strings.TrimSpace(name)
	if name == "" {
		return defs
	}
	for _, def := range defs {
		if def.Name == name {
			return []ToolDefinition{def}
		}
	}
	return defs
}

func extractRequiredNextTool(calls []types.ToolCall, results []types.ToolResult) (pendingRequiredTool, bool) {
	limit := len(results)
	if len(calls) < limit {
		limit = len(calls)
	}
	for i := 0; i < limit; i++ {
		result := results[i]
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
		if !requiredNextToolProtocolAllowed(calls[i].Name, name, decoded) {
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

func requiredNextToolProtocolAllowed(producerTool, nextTool string, decoded map[string]any) bool {
	producerTool = strings.TrimSpace(producerTool)
	nextTool = strings.TrimSpace(nextTool)
	switch producerTool {
	case "request_worker_vm":
		return nextTool == "start_worker_delegation" && decoded["start_args"] != nil
	default:
		return false
	}
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
