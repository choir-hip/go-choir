// Package provideriface defines the provider interface contracts used by the
// runtime and provider bridges. These types were extracted from internal/runtime
// so that provider implementations (internal/provider, internal/gatewayruntime)
// can depend on the interface contracts without importing the full runtime
// package.
//
// All types here are concurrency-free: interfaces, plain structs, and function
// types. The concurrency substrate lives in internal/actor (the durable actor
// runtime) and internal/actorruntime (the adapter).
package provideriface

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// EventEmitFunc is a callback for emitting incremental events during task
// execution. Providers call this to report progress, deltas, or other
// lifecycle updates before the task completes.
type EventEmitFunc func(kind types.EventKind, phase string, payload json.RawMessage)

// ProviderPolicy describes the runtime-visible provider/model policy shown in
// Settings. It is read-only observability metadata, not a mutable config API.
type ProviderPolicy struct {
	ActiveProvider              string   `json:"active_provider"`
	DefaultModel                string   `json:"default_model,omitempty"`
	ModelSelection              string   `json:"model_selection"`
	SupportsPerRunModelOverride bool     `json:"supports_per_run_model_override"`
	Notes                       []string `json:"notes,omitempty"`
}

// Provider is the interface for executing a runtime task. The stub provider
// simulates execution; the real Bedrock/Z.AI bridge (via the provider package's
// BridgeProvider) implements this interface for real upstream calls.
type Provider interface {
	// Execute runs the task to completion, emitting incremental events via
	// the callback. Returns nil on success or an error describing the failure.
	// The runtime transitions the task to failed/blocked on error and remains
	// available for later runs (VAL-RUNTIME-008).
	Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error

	// ProviderName returns the name of the provider for observability
	// (e.g., "stub", "bedrock", "zai"). This is used in health responses
	// and event payloads to distinguish real providers from stubs.
	ProviderName() string
}

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

// ToolDefinition is the LLM-facing schema for a tool, without the Go
// function. This is what gets included in API requests and system prompts.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// Config holds the runtime configuration. It is a pure data struct with no
// concurrency behavior. Extracted from internal/runtime so the actorruntime
// adapter can load and use config without importing the old runtime.
type Config struct {
	// SandboxID is the stable identity of this sandbox instance.
	SandboxID string

	// StorePath is the marker path used to derive the embedded Dolt workspace
	// for task/event persistence. Retired SQLite content is never imported.
	StorePath string

	// PromptRoot is the sandbox-owned filesystem root for editable role prompts.
	PromptRoot string

	// SkillsRoot is the repo-owned filesystem root for natural-language skills
	// that should be summarized into selected agent prompts.
	SkillsRoot string

	// ProviderTimeout is the simulated work duration for the stub provider.
	ProviderTimeout time.Duration
	// ActivationBudget is the maximum wall-clock residency of one activation.
	// The runtime persists a terminal progress-deadline outcome when it expires.
	ActivationBudget time.Duration

	// SupervisionInterval is legacy reserved configuration. The old polling
	// supervisor has been deleted and this value is currently unused.
	SupervisionInterval time.Duration

	// ResearcherCount is the configured researcher worker count for this VM.
	ResearcherCount int

	// TextureWakeDebounce is the coalescing window for addressed worker findings
	// before the runtime schedules the next texture synthesis.
	TextureWakeDebounce time.Duration

	// TextureActorParkIdle enables default park-on-idle for Texture revision
	// actors when positive. Hand-constructed test configs leave this zero unless
	// they opt into parked lifecycle behavior explicitly.
	TextureActorParkIdle time.Duration

	// VmctlURL is the host-side lifecycle URL used by product desktop controls.
	// Runtime agents receive no vmctl or worker-computer authority.
	VmctlURL string

	// MaildURL is the host-side mail service URL. Texture-originated Email
	// appagent draft requests use this only to persist reviewable drafts; it
	// must not expose raw send authority to runtime agents.
	MaildURL string

	// WirePublishURL is the host-mediated proxy route for autonomous Universal
	// Wire platform publication. Platform VM sandboxes call this instead of
	// corpusd directly.
	WirePublishURL string

	// CorpusdURL is an optional direct corpusd endpoint for local publish
	// tests or host-colocated sandboxes when WirePublishURL is unset.
	CorpusdURL string

	// LLMProvider is the explicitly selected provider for runtime LLM calls.
	// Empty means no provider is selected by this runtime config.
	LLMProvider string

	// LLMModel is the explicitly selected model for runtime LLM calls.
	LLMModel string

	// LLMReasoningEffort is the provider-specific reasoning effort for runtime
	// LLM calls.
	LLMReasoningEffort string

	// ModelPolicyPath is the computer-owned editable text file that maps agent
	// roles to provider/model/reasoning selections. Environment LLM settings
	// remain the platform fallback; this file is the user-computer policy.
	ModelPolicyPath string

	// ObscuraPath is an optional path or executable name for the backend browser
	// provider. Empty means the backend browser substrate is not configured.
	ObscuraPath string

	// ObscuraCDPScreenshots enables the opt-in CDP screenshot substrate for the
	// backend Browser app. The default remains CLI snapshot extraction only.
	ObscuraCDPScreenshots bool

	// EnableTestAPIs exposes local-only browser test hooks. These endpoints are
	// disabled by default and should never be enabled on deployed environments.
	EnableTestAPIs bool

	// RunMemoryContextThresholdTokens controls automatic context compaction for
	// tool-loop runs. Zero means derive from the selected model context window.
	// The estimator is intentionally approximate.
	RunMemoryContextThresholdTokens int

	// RunMemoryKeepRecentTokens controls how much recent raw context is kept
	// alongside each run-memory compaction checkpoint.
	RunMemoryKeepRecentTokens int

	// QdrantURL is the Qdrant instance URL for semantic routing and indexing.
	// Defaults to the node-b local instance.
	QdrantURL string

	// OllamaURL is the Ollama instance URL for embedding generation.
	// Defaults to localhost:11434.
	OllamaURL string

	// OllamaEmbeddingModel is the model name for Ollama embeddings.
	OllamaEmbeddingModel string

	// QdrantDedupThreshold is the cosine similarity score at which a newly
	// ingested capture is treated as a semantic duplicate of an existing
	// capture and dropped before processor dispatch. Defaults to the spike-5
	// baseline (0.7862); tune via QDRANT_DEDUP_THRESHOLD. A threshold of 0
	// disables semantic dedup.
	QdrantDedupThreshold float32

	// TracePersistenceEnabled mounts the Dolt-backed trace observability store
	// (internal/trace) into the runtime router so trace events are persisted
	// alongside the existing event recording. This is additive: existing
	// request handling and event bus publishing are unchanged. When false (the
	// default), no trace store is mounted and events are not projected into the
	// observability schema. Enabled via RUNTIME_TRACE_PERSISTENCE_ENABLED.
	TracePersistenceEnabled bool
}
