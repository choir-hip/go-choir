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
	User           string
	MaxTokens      int
}

// TextStreamResponse is what a finished stream reports.
type TextStreamResponse struct {
	Text       string
	Model      string
	StopReason string
	Usage      TokenUsage
}

// TextStreamProvider streams a text answer. onDelta is called, in order, with
// each non-empty text delta; it is never called with reasoning. An error
// returned before onDelta was called means the call produced nothing, and the
// caller may try another model without the listener hearing anything.
type TextStreamProvider interface {
	StreamText(ctx context.Context, req TextStreamRequest, onDelta func(string)) (*TextStreamResponse, error)
}
