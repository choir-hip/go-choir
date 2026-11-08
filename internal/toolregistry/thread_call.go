package toolregistry

import (
	"context"
	"encoding/json"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The thread call (owner, 2026-11-08: "Compaction shouldn't be pinned to a
// model, it should use the same model as the thread for caching. There
// shouldn't be a 'compactor'."). The loop offers its memory hooks a way to
// ask the thread itself: the request the next turn would send, unchanged,
// plus one final user message. The system prompt, tool definitions, messages,
// answering member (after any failover), conversation id and cache key are
// the loop's own, so the provider's prompt cache serves the prefix and a
// failed-over run is asked of the member that answers it.

type threadCallKey struct{}

// ThreadCallFunc asks the thread one more question and returns the reply.
type ThreadCallFunc func(ctx context.Context, instruction string) (*provideriface.ToolLoopResponse, error)

// withThreadCall installs fn on ctx for the memory hooks.
func withThreadCall(ctx context.Context, fn ThreadCallFunc) context.Context {
	return context.WithValue(ctx, threadCallKey{}, fn)
}

// ThreadCall returns the thread call the loop offered ctx. It is absent
// outside a loop's memory hooks.
func ThreadCall(ctx context.Context) (ThreadCallFunc, bool) {
	if ctx == nil {
		return nil, false
	}
	fn, ok := ctx.Value(threadCallKey{}).(ThreadCallFunc)
	return fn, ok && fn != nil
}

// threadInstructionMessage is the one user message appended to the thread.
func threadInstructionMessage(instruction string) json.RawMessage {
	msg, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": []any{map[string]string{"type": "text", "text": instruction}},
	})
	return msg
}
