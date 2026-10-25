package toolregistry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

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
	// Waited says the guard waited for an outside event (a capsule job's
	// end) and Instruction reports it: the turn resumes, and the wait is
	// not a retry of the guard's reminder (bounded by maxCompletionGuardWaits).
	Waited bool
}

type ToolLoopCompletionGuardFunc func(ctx context.Context, state ToolLoopCompletionState) (ToolLoopCompletionGuardResult, error)

// ToolLoopBatchWaitFunc runs after a tool batch that was neither terminal
// nor passivating. ok=true with Continue says it waited for an outside event
// (a capsule job the batch's cell looked at) and Instruction reports it: the
// loop appends the instruction and calls the model, with no model turn
// between the batch and the wait. ok=false continues the loop as usual.
type ToolLoopBatchWaitFunc func(ctx context.Context, calls []types.ToolCall, results []types.ToolResult) (ToolLoopCompletionGuardResult, bool, error)

type ToolLoopParkState struct {
	Messages  []json.RawMessage
	FinalText string
	Attempts  int
}

type ToolLoopParkResult struct {
	Continue  bool
	Passivate bool
	Reason    string
}

type ToolLoopParkWaiterFunc func(ctx context.Context, state ToolLoopParkState) (ToolLoopParkResult, error)

var ErrToolLoopPassivated = errors.New("tool loop passivated")

type ToolLoopPassivatedError struct {
	Reason string
}

func (e *ToolLoopPassivatedError) Error() string {
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		return ErrToolLoopPassivated.Error()
	}
	return ErrToolLoopPassivated.Error() + ": " + reason
}

func (e *ToolLoopPassivatedError) Unwrap() error {
	return ErrToolLoopPassivated
}

type toolLoopOptions struct {
	memoryHooks    ToolLoopMemoryHooks
	llmConfig      provideriface.LLMSelection
	conversationID string
	// cacheLabel and cacheScope name the prompt-cache thread
	// (WithToolLoopPromptCache).
	cacheLabel string
	cacheScope string
	// fallbackLLMConfigs are the pool members after llmConfig, in order. A
	// failover-class provider error moves the rest of the run to the first
	// one (docs/model-pools-design-2026-10-12.md).
	fallbackLLMConfigs []provideriface.LLMSelection

	initialToolChoice string
	// providerCallTimeout bounds each model call (0: none beyond ctx). A
	// management task sets it below its deadline (management delegation
	// R2-9), so one slow call cannot take the whole task.
	providerCallTimeout    time.Duration
	terminalToolResults    map[string]func(string) bool
	passivatingToolResults map[string]func(string) bool
	completionGuard        ToolLoopCompletionGuardFunc
	parkWaiter             ToolLoopParkWaiterFunc
	batchWait              ToolLoopBatchWaitFunc
}

type pendingRequiredTool struct {
	Name        string
	Instruction string
	Attempts    int
}

const maxRequiredNextToolRetries = 2

const maxTokenContinuationRetries = 3

const maxCompletionGuardRetries = 2

// maxCompletionGuardWaits bounds the guard's waits for outside events in
// one run (a desk that starts a job per turn), park resumes included. Each
// wait needs a job that ended, so a run cannot spin here without starting
// jobs; the activation budget bounds the run's time. An engineer that works
// in short jobs waits often: faf32 on b@c waited 13 times in 19.5 minutes
// (docs/evidence/selfdev-trace-reviews-2026-10-25.md, Round 3, N1), so 32
// would cancel such a run before a 60-minute activation ends.
const maxCompletionGuardWaits = 128

const maxToolLoopParkResumes = 16

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
func WithToolLoopLLMConfig(config provideriface.LLMSelection) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.llmConfig = config
	}
}

// WithToolLoopFallbackLLMConfigs names the models a run may fail over to,
// in order. The loop walks them forward, each at most once, and never
// returns to a member it left.
func WithToolLoopFallbackLLMConfigs(configs []provideriface.LLMSelection) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.fallbackLLMConfigs = append([]provideriface.LLMSelection(nil), configs...)
	}
}

// WithToolLoopConversationID carries the durable run identity into every
// provider request emitted by the loop.
func WithToolLoopConversationID(conversationID string) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.conversationID = strings.TrimSpace(conversationID)
	}
}

// WithToolLoopPromptCache names the thread whose calls share a provider
// prompt cache: label is the desk/role (kept readable in the key), scope
// the desk's identity on its computer (hashed). The loop derives the key
// with PromptCacheKey from what it actually sends, so a new prompt version
// gets a new key. An empty scope sends no key.
func WithToolLoopPromptCache(label, scope string) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.cacheLabel = strings.TrimSpace(label)
		opts.cacheScope = scope
	}
}

// PromptCacheKey is the cache-affinity key for a desk thread at a prompt
// version: "choir-<label>-<scope hash>-<prompt hash>". The scope hash keeps
// owner, computer and agent ids out of provider logs; the prompt hash
// covers the system prompt and the tool definitions, the prefix every call
// of the thread starts with. Byte-identical prefixes give identical keys.
func PromptCacheKey(label, scope, system string, tools []provideriface.ToolDefinition) string {
	if strings.TrimSpace(scope) == "" {
		return ""
	}
	toolsJSON, _ := json.Marshal(tools)
	label = promptCacheLabel(label)
	return "choir-" + label + "-" + shortSHA256(scope) + "-" + shortSHA256(system, string(toolsJSON))
}

// failoverCacheKey is the cache key for a call served by active after a model
// failover. The primary selection keeps the base key; any other selection
// appends "-m" and six hex characters of sha256(provider/model), so every call
// a thread gets from one model shares a key whatever its failover ordinal, and
// different models never share one. The tag adds 8 characters to a key of at
// most 56, within the 64 a provider accepts, and carries no owner or computer
// id.
func failoverCacheKey(base string, primary, active provideriface.LLMSelection) string {
	if base == "" || sameServingSelection(primary, active) {
		return base
	}
	return base + "-m" + shortSHA256(servingSelectionID(active))[:6]
}

func servingSelectionID(sel provideriface.LLMSelection) string {
	return strings.ToLower(strings.TrimSpace(sel.Provider)) + "/" + strings.ToLower(strings.TrimSpace(sel.Model))
}

func sameServingSelection(a, b provideriface.LLMSelection) bool {
	return servingSelectionID(a) == servingSelectionID(b)
}

func promptCacheLabel(label string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(label)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
		if b.Len() >= 24 {
			break
		}
	}
	if b.Len() == 0 {
		return "run"
	}
	return b.String()
}

func shortSHA256(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
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

// WithTerminalToolResult makes a tool result terminal when the predicate
// accepts its structured output (e.g. an eval result carrying a committed
// terminal assignment fate). The decision reads the result payload, not just
// the tool name.
// WithToolLoopProviderCallTimeout bounds each model call of the loop. A
// call that reaches it fails the loop with ErrProviderCallTimeout.
func WithToolLoopProviderCallTimeout(timeout time.Duration) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		if timeout > 0 {
			opts.providerCallTimeout = timeout
		}
	}
}

// ErrProviderCallTimeout marks a model call that reached the loop's
// per-call timeout.
var ErrProviderCallTimeout = errors.New("model call exceeded its per-call timeout")

func WithTerminalToolResult(name string, predicate func(output string) bool) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		if trimmed := strings.TrimSpace(name); trimmed != "" && predicate != nil {
			if opts.terminalToolResults == nil {
				opts.terminalToolResults = make(map[string]func(string) bool)
			}
			opts.terminalToolResults[trimmed] = predicate
		}
	}
}

// WithPassivatingToolResult passivates this activation when a successful call
// to name produces structured output the predicate accepts (e.g. a desk_go_eval
// result whose receipts carry a committed texture_turn intent). The decision
// reads the result payload, not just the tool name.
func WithPassivatingToolResult(name string, predicate func(output string) bool) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		if trimmed := strings.TrimSpace(name); trimmed != "" && predicate != nil {
			if opts.passivatingToolResults == nil {
				opts.passivatingToolResults = make(map[string]func(string) bool)
			}
			opts.passivatingToolResults[trimmed] = predicate
		}
	}
}

// WithCompletionGuard installs an app-level completion obligation evaluated
// each time the model ends its turn. Returning Continue keeps the loop alive
// with a reminder (bounded by maxCompletionGuardRetries, then the run errors);
// returning Continue=false accepts the turn as the run's result.
func WithCompletionGuard(guard ToolLoopCompletionGuardFunc) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.completionGuard = guard
	}
}

// WithToolBatchWait installs a wait run after each non-terminal tool batch
// (ToolLoopBatchWaitFunc). Its waits count against the completion guard's
// wait bound (maxCompletionGuardWaits), not its retries.
func WithToolBatchWait(wait ToolLoopBatchWaitFunc) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.batchWait = wait
	}
}

// WithParkWaiter lets a caller suspend normal completion until a durable signal
// or idle deadline. The waiter must not call the provider; on Continue=true the
// loop injects runtime-owned user turns and resumes provider calls only after
// new context is available.
func WithParkWaiter(waiter ToolLoopParkWaiterFunc) ToolLoopOption {
	return func(opts *toolLoopOptions) {
		opts.parkWaiter = waiter
	}
}

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
func RunToolLoop(ctx context.Context, provider provideriface.ToolLoopProvider, registry *ToolRegistry, initialMessages []json.RawMessage, systemPrompt string, maxTokens int, emit provideriface.EventEmitFunc, injectUserTurns InjectUserTurnsFunc, opts ...ToolLoopOption) (string, provideriface.TokenUsage, error) {
	var totalUsage provideriface.TokenUsage
	options := toolLoopOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	messages := make([]json.RawMessage, len(initialMessages))
	copy(messages, initialMessages)

	toolDefs := []provideriface.ToolDefinition{}
	if registry != nil {
		toolDefs = registry.Definitions()
		systemPrompt = BuildSystemPrompt(systemPrompt, registry)
	}
	cacheKey := PromptCacheKey(options.cacheLabel, options.cacheScope, systemPrompt, toolDefs)
	forceInitialToolChoiceRetry := false
	relaxInitialExactToolChoice := false
	initialToolChoiceAttempts := 0
	activeLLMConfig := options.llmConfig
	var requiredNextTool *pendingRequiredTool
	var maxTokenContinuationAttempts int
	var completionGuardAttempts int
	var completionGuardWaits int
	var parkAttempts int
	var partialTextFragments []string

	appendMessageWithContext := func(appendCtx context.Context, role string, msg json.RawMessage) error {
		messages = append(messages, msg)
		if options.memoryHooks.AfterAppendMessage != nil {
			if err := options.memoryHooks.AfterAppendMessage(appendCtx, role, msg); err != nil {
				return err
			}
		}
		return nil
	}
	appendMessage := func(role string, msg json.RawMessage) error {
		return appendMessageWithContext(ctx, role, msg)
	}
	appendInjected := func(injected []json.RawMessage) error {
		for _, msg := range injected {
			if err := appendMessage(types.RunMemoryRoleRuntimeInjection, msg); err != nil {
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
		text := requiredInitialToolChoiceReminderText(requiredName, reason)
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

	transientAttempts := 0
	nextFallback := 0
	baseConversationID := options.conversationID
	baseCacheKey := cacheKey
	for i := 0; ; i++ {
		if options.memoryHooks.BeforeProviderCall != nil {
			rebuilt, err := options.memoryHooks.BeforeProviderCall(ctx, messages)
			if err != nil {
				return "", totalUsage, fmt.Errorf("tool loop memory before iteration %d: %w", i, err)
			}
			messages = rebuilt
		}

		req := provideriface.ToolLoopRequest{
			Provider:        activeLLMConfig.Provider,
			Model:           activeLLMConfig.Model,
			ReasoningEffort: activeLLMConfig.ReasoningEffort,
			ConversationID:  options.conversationID,
			CacheKey:        cacheKey,

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

		// The request-side line is published on the bus and not appended
		// (journal cut R1, types.LiveOnly); what a reader of the pair used
		// to take from it rides the response line below.
		lastUserText := extractLastUserMessage(messages)
		systemSHA256 := toolOutputSHA256Hex(systemPrompt)
		lastUserSHA256 := toolOutputSHA256Hex(lastUserText)
		callMessages, callTools := len(messages), len(req.ToolDefinitions)
		callStarted := time.Now()
		if emit != nil {
			preCallPayload, _ := json.Marshal(map[string]any{
				"iteration":            i + 1,
				"phase":                "provider_call_started",
				"messages":             callMessages,
				"tools":                callTools,
				"tool_names":           toolDefinitionNames(req.ToolDefinitions),
				"system_chars":         len(systemPrompt),
				"system_sha256":        systemSHA256,
				"system_preview":       truncatePromptSnippet(systemPrompt, 2000),
				"last_user_chars":      len(lastUserText),
				"last_user_sha256":     lastUserSHA256,
				"last_user_text":       truncatePromptSnippet(lastUserText, 4000),
				"message_roles":        toolLoopMessageRoles(messages),
				"max_tokens":           req.MaxTokens,
				"max_tokens_requested": req.MaxTokens > 0,
				"llm_provider":         activeLLMConfig.Provider,
				"llm_model":            activeLLMConfig.Model,
				"llm_reasoning_effort": activeLLMConfig.ReasoningEffort,
				"tool_choice":          req.ToolChoice,
				"model_policy":         "run_metadata",
			})
			emit(types.EventRunProgress, types.ProgressPhaseProviderCall, preCallPayload)
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
		} else if options.providerCallTimeout > 0 {
			providerCtx, cancelProviderCall = context.WithTimeout(ctx, options.providerCallTimeout)
		}
		resp, err := CallToolLoopProviderWithRetries(providerCtx, provider, req, emit)
		providerCallErr := providerCtx.Err()
		if cancelProviderCall != nil {
			cancelProviderCall()
		}
		if err != nil && requiredTimeout == nil && options.providerCallTimeout > 0 && ctx.Err() == nil && errors.Is(providerCallErr, context.DeadlineExceeded) {
			return "", totalUsage, fmt.Errorf("%w (%s): %v", ErrProviderCallTimeout, options.providerCallTimeout, err)
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
			// Model failover: a model that still fails after its retries
			// hands the rest of the run to the next pool member. The cache
			// breaks; the work continues. The loop walks the fallbacks in
			// order and never goes back, so each member is tried at most
			// once and the run fails only when the last one fails. Going
			// only to the first fallback stranded a deepseek thread on
			// cat-jam behind the same open circuit while luna was healthy
			// (docs/problems/free-model-429-opens-provider-circuit-stub-
			// stories-2026-10-14.md). Never on our own cancellation.
			if nextFallback < len(options.fallbackLLMConfigs) && ctx.Err() == nil && IsModelFailoverError(err) {
				from := activeLLMConfig
				activeLLMConfig = fillFallbackSelection(options.fallbackLLMConfigs[nextFallback], from)
				nextFallback++
				if baseConversationID != "" {
					options.conversationID = baseConversationID + "#failover"
					if nextFallback > 1 {
						options.conversationID += strconv.Itoa(nextFallback)
					}
				}
				if baseCacheKey != "" {
					// The key doubles as OpenCode's session: a failed-over
					// thread leaves the session pinned to the failing route.
					// It is keyed by the serving selection, not the failover
					// ordinal, so a model that is failover1 on one call and
					// failover2 on the next keeps one cache key.
					cacheKey = failoverCacheKey(baseCacheKey, options.llmConfig, activeLLMConfig)
				}
				messages = stripProviderReplay(messages)
				transientAttempts = 0
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"reason":         "model_failover",
						"from_provider":  from.Provider,
						"from_model":     from.Model,
						"to_provider":    activeLLMConfig.Provider,
						"to_model":       activeLLMConfig.Model,
						"provider_error": err.Error(),
						"members_left":   len(options.fallbackLLMConfigs) - nextFallback,
					})
					emit(types.EventRunRetry, "model_failover", payload)
				}
				continue
			}
			// Transient provider outage (gateway 429/5xx or an equivalent
			// marker on another provider): a bound assignment's activation
			// must not terminalize on a recoverable call. Bounded in-loop
			// retry with a short sleep; the durable run record is untouched
			// so a retry costs one iteration of wall time, not a state.
			if isTransientProviderError(err) && transientAttempts < 3 {
				transientAttempts++
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"reason":         "transient_provider_error",
						"attempt":        transientAttempts,
						"provider_error": err.Error(),
					})
					emit(types.EventRunRetry, "transient_provider", payload)
				}
				select {
				case <-ctx.Done():
					return "", totalUsage, fmt.Errorf("tool loop iteration %d: %w", i, ctx.Err())
				case <-time.After(15 * time.Second):
				}
				continue
			}
			return "", totalUsage, fmt.Errorf("tool loop iteration %d: %w", i, err)
		}

		// Accumulate token usage.
		totalUsage.Add(resp.Usage)

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
			"input_tokens":         resp.Usage.InputTokens,
			"output_tokens":        resp.Usage.OutputTokens,
			// Prompt-cache measurement: the part of input_tokens served
			// from the provider's cache, and whether the provider reported
			// it at all (llmcost prices and rates only reported calls).
			"cached_input_tokens": resp.Usage.CachedInputTokens,
			"cache_reported":      resp.Usage.CacheReported,
			// What the request-side line gave a reader of the pair: how long
			// the call took (retries included), the request's shape, and the
			// hashes that show the prompt prefix changed between turns.
			"provider_call_ms": time.Since(callStarted).Milliseconds(),
			"messages":         callMessages,
			"tools":            callTools,
			"system_sha256":    systemSHA256,
			"last_user_sha256": lastUserSHA256,
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
			if err := appendMessageWithContext(ctx, "assistant", assistantMsg); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist assistant message: %w", err)
			}
			// The tools run under the model that answered this call, so a
			// record they write names it (after a failover too).
			toolResults := ExecuteToolBatch(WithActiveModel(ctx, activeLLMConfig.Provider, activeLLMConfig.Model), registry, resp.ToolCalls, emit)
			toolResultMsg, _ := json.Marshal(map[string]any{
				"role":    "user",
				"content": buildToolResultContent(toolResults),
			})
			if err := appendMessageWithContext(ctx, "user", toolResultMsg); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist tool result message: %w", err)
			}
			activeRequired := requiredNextTool
			requiredCalled := requiredToolCalled(activeRequired, resp.ToolCalls)
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
					if terminalTools := successfulTerminalToolNamesWithResults(resp.ToolCalls, toolResults, options.terminalToolResults); len(terminalTools) > 0 {
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
			} else if terminalTools := successfulTerminalToolNamesWithResults(resp.ToolCalls, toolResults, options.terminalToolResults); len(terminalTools) > 0 {
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
			if requiredNextTool == nil {
				if passivatingTools := successfulTerminalToolNamesWithResults(resp.ToolCalls, toolResults, options.passivatingToolResults); len(passivatingTools) > 0 {
					if emit != nil {
						payload, _ := json.Marshal(map[string]any{
							"iteration": i + 1,
							"tools":     passivatingTools,
							"reason":    "durable_transition_success",
						})
						emit(types.EventRunProgress, "passivating_tool_success", payload)
					}
					return combinedFinalText(resp.Text), totalUsage, &ToolLoopPassivatedError{Reason: "durable_transition_success"}
				}
			}

			if requiredNextTool == nil && options.batchWait != nil {
				waited, ok, err := options.batchWait(ctx, resp.ToolCalls, toolResults)
				if err != nil {
					return "", totalUsage, fmt.Errorf("tool loop batch wait: %w", err)
				}
				if ok && waited.Continue {
					completionGuardWaits++
					if completionGuardWaits > maxCompletionGuardWaits {
						return "", totalUsage, fmt.Errorf("tool loop: completion guard waited %d times", maxCompletionGuardWaits)
					}
					if err := appendCompletionGuardReminder(waited); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist batch wait instruction: %w", err)
					}
				}
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
					if guardResult.Waited {
						completionGuardWaits++
						if completionGuardWaits > maxCompletionGuardWaits {
							return "", totalUsage, fmt.Errorf("tool loop: completion guard waited %d times", maxCompletionGuardWaits)
						}
					} else {
						completionGuardAttempts++
						if completionGuardAttempts > maxCompletionGuardRetries {
							return "", totalUsage, fmt.Errorf("tool loop: completion guard %q was not satisfied after %d retries", strings.TrimSpace(guardResult.Reason), maxCompletionGuardRetries)
						}
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
			if options.parkWaiter != nil {
				if parkAttempts >= maxToolLoopParkResumes {
					return "", totalUsage, fmt.Errorf("tool loop: park waiter resumed %d times without completing", maxToolLoopParkResumes)
				}
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"attempt": parkAttempts + 1,
						"reason":  "idle_before_completion",
					})
					emit(types.EventRunProgress, "park_wait_started", payload)
				}
				parkResult, err := options.parkWaiter(ctx, ToolLoopParkState{
					Messages:  append([]json.RawMessage(nil), messages...),
					FinalText: resp.Text,
					Attempts:  parkAttempts,
				})
				if err != nil {
					return "", totalUsage, fmt.Errorf("tool loop park waiter: %w", err)
				}
				if emit != nil {
					payload, _ := json.Marshal(map[string]any{
						"attempt":  parkAttempts + 1,
						"continue": parkResult.Continue,
						"reason":   strings.TrimSpace(parkResult.Reason),
					})
					emit(types.EventRunProgress, "park_wait_finished", payload)
				}
				if parkResult.Continue {
					if injectUserTurns == nil {
						return "", totalUsage, fmt.Errorf("tool loop park waiter continued without an injector")
					}
					if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist parked assistant message: %w", err)
					}
					injected, err := injectUserTurns(true)
					if err != nil {
						return "", totalUsage, fmt.Errorf("tool loop inject turns after park: %w", err)
					}
					if len(injected) == 0 {
						return "", totalUsage, fmt.Errorf("tool loop park waiter continued without injected turns")
					}
					if err := appendInjected(injected); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist injected turns after park: %w", err)
					}
					parkAttempts++
					continue
				}
				if parkResult.Passivate {
					if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
						return "", totalUsage, fmt.Errorf("tool loop persist passivated assistant message: %w", err)
					}
					return combinedFinalText(resp.Text), totalUsage, &ToolLoopPassivatedError{Reason: parkResult.Reason}
				}
			}
			if err := appendAssistantText(resp.Text, resp.ReasoningContent); err != nil {
				return "", totalUsage, fmt.Errorf("tool loop persist assistant final message: %w", err)
			}
			// Normal completion — return the text.
			return combinedFinalText(resp.Text), totalUsage, nil

		case "max_tokens":
			if req.ToolChoice != "" && len(resp.ToolCalls) == 0 {
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
			if maxTokenContinuationAttempts > maxTokenContinuationRetries {
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

func isProviderPreconditionError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "412") ||
		strings.Contains(text, "precondition failed") ||
		strings.Contains(text, "thinking mode does not support this tool_choice")
}

// isTransientProviderError reports whether a provider call error is a
// recoverable outage rather than a request failure. Typed errors that expose
// Transient() bool (gatewayHTTPStatusError: 429 + 5xx) are authoritative;
// the string fallback covers providers that flatten status into the message
// (matches the existing precondition-detection convention in this file).
// IsModelFailoverError reports whether a provider error, after its retries,
// should move a run to the next model in its pool: rate limits and usage
// limits, 5xx and timeouts, and a model the provider or the catalog does
// not serve. A malformed request of ours is not one.
func IsModelFailoverError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if isTransientProviderError(err) || IsProviderRateLimitError(err) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"usage limit", "usage_limit", "quota", "not in the catalog", "unsupported model",
		"model_not_found", "model not found", "does not exist", "timeout", "timed out",
		"circuit open", "breaker open",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// fillFallbackSelection keeps the run's reasoning effort and output budget
// on the fallback model unless the fallback names its own.
func fillFallbackSelection(next, from provideriface.LLMSelection) provideriface.LLMSelection {
	if next.ReasoningEffort == "" {
		next.ReasoningEffort = from.ReasoningEffort
	}
	if next.MaxTokens <= 0 {
		next.MaxTokens = from.MaxTokens
	}
	if next.Source == "" {
		next.Source = "model_failover"
	}
	return next
}

// stripProviderReplay drops provider-specific reasoning replay from the
// history: one provider's reasoning items are not valid input to another.
func stripProviderReplay(messages []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, 0, len(messages))
	for _, msg := range messages {
		var decoded map[string]any
		if json.Unmarshal(msg, &decoded) == nil {
			if _, ok := decoded["reasoning_content"]; ok {
				delete(decoded, "reasoning_content")
				if encoded, err := json.Marshal(decoded); err == nil {
					out = append(out, encoded)
					continue
				}
			}
		}
		out = append(out, msg)
	}
	return out
}

func isTransientProviderError(err error) bool {
	if err == nil {
		return false
	}
	var transient interface{ Transient() bool }
	if errors.As(err, &transient) && transient.Transient() {
		return true
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"status 429", "status 500", "status 502", "status 503", "status 504",
		"too many requests", "service unavailable", "bad gateway", "gateway timeout",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func toolDefinitionsMatchingName(defs []provideriface.ToolDefinition, name string) []provideriface.ToolDefinition {
	name = strings.TrimSpace(name)
	if name == "" {
		return defs
	}
	for _, def := range defs {
		if def.Name == name {
			return []provideriface.ToolDefinition{def}
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

func requiredNextToolProtocolAllowed(string, string, map[string]any) bool {
	return false
}

func successfulTerminalToolNamesWithResults(calls []types.ToolCall, results []types.ToolResult, terminalResults map[string]func(string) bool) []string {
	if len(terminalResults) == 0 {
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
		if name == "" || results[i].IsError || !IsStructuredToolSuccess(results[i].Output) {
			continue
		}
		if pred, ok := terminalResults[name]; !ok || !pred(results[i].Output) {
			continue
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

func IsStructuredToolSuccess(output string) bool {
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

func requiredInitialToolChoiceReminderText(requiredName, reason string) string {
	requiredName = strings.TrimSpace(requiredName)
	reason = strings.TrimSpace(reason)
	if requiredName == "patch_texture" && reason == "required_initial_tool_failed" {
		return "The previous required initial patch_texture call failed. Review the tool error above and fix the issue. If the change spans multiple sections or is a full-document draft (especially v0→v1 or v1→v2), call rewrite_texture instead. Otherwise, call patch_texture again with corrected arguments. Common fixes: use the current base_revision_id from the document outline, use append_block for new paragraphs, or use update_block_text only when the structured outline gives the exact block_id to update."
	}
	return fmt.Sprintf("The previous model turn did not call the required initial tool %q (%s). Call exactly that available tool now. Do not write prose and do not call any other tool.", requiredName, reason)
}

func CallToolLoopProviderWithRetries(ctx context.Context, provider provideriface.ToolLoopProvider, req provideriface.ToolLoopRequest, emit provideriface.EventEmitFunc) (*provideriface.ToolLoopResponse, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		resp, err := provider.CallWithTools(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !IsProviderRateLimitError(err) || attempt >= len(providerRateLimitRetryDelays) {
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

func IsProviderRateLimitError(err error) bool {
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

func toolDefinitionNames(defs []provideriface.ToolDefinition) []string {
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

// --- provideriface.ToolLoopProvider adapter for providers that don't natively support it ---

// toolLoopAdapter wraps a basic Provider to implement provideriface.ToolLoopProvider by
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
	provideriface.Provider
}

// CallWithTools implements provideriface.ToolLoopProvider by delegating to the underlying
// Provider's Execute method. The adapter extracts the last user message as
// the prompt and returns a single-turn end_turn response.
func (a *toolLoopAdapter) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
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

	return &provideriface.ToolLoopResponse{
		StopReason: "end_turn",
		Text:       result,
		Usage:      provideriface.TokenUsage{},
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

// AsToolLoopProvider converts a Provider to a provideriface.ToolLoopProvider. If the
// provider already implements provideriface.ToolLoopProvider, it is returned directly.
// Otherwise, it is wrapped in a toolLoopAdapter that converts tool-loop
// calls into simple provider calls.
func AsToolLoopProvider(p provideriface.Provider) provideriface.ToolLoopProvider {
	if tlp, ok := p.(provideriface.ToolLoopProvider); ok {
		return tlp
	}
	return &toolLoopAdapter{Provider: p}
}

func stringMapValue(m map[string]any, key string) string {
	value := m[key]
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	}
	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.String {
		return strings.TrimSpace(rv.String())
	}
	return ""
}

func runMemoryMessageRole(msg json.RawMessage) string {
	var parsed struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(msg, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Role)
}

func truncatePromptSnippet(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit]) + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
