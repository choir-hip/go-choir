package provideriface

import "context"

// TextStreamRequest is one streamed, tool-less text call: a system prompt and
// one user message, answered as text deltas. The Autoradio desk's spoken
// answer is the only caller (docs/design/autoradio-desk-fast-turns-2026-11-11.md
// section 6). It carries no tools by construction.
type TextStreamRequest struct {
	Provider        string
	Model           string
	ReasoningEffort string
	// ConversationID and CacheKey are the provider session and prompt-cache
	// keys, as on ToolLoopRequest.
	ConversationID string
	CacheKey       string
	System         string
	// StableUser is the part of the user message that is byte-identical from
	// turn to turn (for the Autoradio desk, the page block): it comes first,
	// is its own content block, and carries the provider's explicit cache
	// breakpoint where the provider needs one. User is the volatile rest.
	StableUser string
	User       string
	// MaxTokens is zero for the desk: no token budget steers the answer (owner
	// 2026-11-11). Zero omits the field where the provider's API allows and
	// otherwise sends the model's own ceiling from the catalog.
	MaxTokens int
	// ReasoningOff asks the provider for reasoning off for this call. It is the
	// one narrow exemption from "thinking is never off" (owner 2026-11-09),
	// for the autoradio role only; see modelpolicy.ReasoningOff. Nothing else
	// sets it: ToolLoopRequest has no such field.
	ReasoningOff bool
}

// TextStreamResponse is what a finished stream reports.
type TextStreamResponse struct {
	Text       string
	Model      string
	StopReason string
	Usage      TokenUsage
	// Applied is what the gateway said it applied of ReasoningOff and the
	// stable-prefix cache break: "off" or "on" for reasoning; "accepted",
	// "ignored" or "none" for the cache break; "unknown" for both when the
	// gateway is older than the header.
	AppliedReasoning, AppliedCacheBreak string
}

// TextStreamProvider streams a text answer. onDelta is called, in order, with
// each non-empty text delta; it is never called with reasoning. An error
// returned before onDelta was called means the call produced nothing, and the
// caller may try another model without the listener hearing anything.
type TextStreamProvider interface {
	StreamText(ctx context.Context, req TextStreamRequest, onDelta func(string)) (*TextStreamResponse, error)
}
