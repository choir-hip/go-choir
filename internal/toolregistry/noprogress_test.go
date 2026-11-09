package toolregistry

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// scriptedProvider answers every model call with next(n), n counting from 1,
// and records how many runtime notices the conversation carried.
type scriptedProvider struct {
	provideriface.Provider
	calls  atomic.Int32
	next   func(n int) *provideriface.ToolLoopResponse
	notice atomic.Int32
}

func (p *scriptedProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	n := int(p.calls.Add(1))
	if n > 200 {
		return nil, errors.New("runaway: loop not stopped")
	}
	raw, _ := json.Marshal(req.Messages)
	p.notice.Store(int32(strings.Count(string(raw), "Runtime notice: you made the same")))
	return p.next(n), nil
}

func toolUse(args string) *provideriface.ToolLoopResponse {
	return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: "m",
		ToolCalls: []types.ToolCall{{ID: "c", Name: "list_dir", Arguments: json.RawMessage(args)}}}
}

func runScripted(t *testing.T, p *scriptedProvider, out func(call int) string) (string, error) {
	t.Helper()
	var executed atomic.Int32
	reg := NewToolRegistry()
	if err := reg.Register(Tool{Name: "list_dir", Func: func(context.Context, json.RawMessage) (string, error) {
		return out(int(executed.Add(1))), nil
	}}); err != nil {
		t.Fatal(err)
	}
	text, _, err := RunToolLoop(context.Background(), p, reg, []json.RawMessage{json.RawMessage(`{"role":"user","content":"go"}`)}, "system", 0, noEmit, nil)
	return text, err
}

func TestNoProgressSameCallSameResultIsNoticedOnceThenStopped(t *testing.T) {
	p := &scriptedProvider{next: func(int) *provideriface.ToolLoopResponse { return toolUse(`{"path":"/work"}`) }}
	_, err := runScripted(t, p, func(int) string { return "[]" })
	var npe *NoProgressError
	if !errors.As(err, &npe) || !errors.Is(err, ErrNoProgress) {
		t.Fatalf("err = %v; want NoProgressError", err)
	}
	if got := p.calls.Load(); got != 2*noRepeatLimit {
		t.Fatalf("model calls = %d; want %d", got, 2*noRepeatLimit)
	}
	if p.notice.Load() != 1 {
		t.Fatalf("notices shown = %d; want exactly 1", p.notice.Load())
	}
	if want := "stopped: list_dir repeated 8 times with the same result"; err.Error() != want {
		t.Fatalf("message %q; want %q", err.Error(), want)
	}
	if IsModelFailoverError(err) || IsProviderRateLimitError(err) {
		t.Fatalf("a no-progress stop must not be a failover or quota error: %v", err)
	}
}

func TestNoProgressChangingResultIsProgress(t *testing.T) {
	p := &scriptedProvider{next: func(n int) *provideriface.ToolLoopResponse {
		if n > 12 {
			return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "m"}
		}
		return toolUse(`{"path":"/work"}`)
	}}
	// The poll answers "pending" four times and changes on the fifth try;
	// every later answer is new, so it is never stopped.
	text, err := runScripted(t, p, func(n int) string {
		if n <= 4 {
			return "pending"
		}
		return "ready-" + string(rune('a'+n))
	})
	if err != nil || text != "done" {
		t.Fatalf("run = %q, %v; want done", text, err)
	}
}

func TestNoProgressDistinctCallsNeverTrip(t *testing.T) {
	p := &scriptedProvider{next: func(n int) *provideriface.ToolLoopResponse {
		if n > 30 {
			return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "m"}
		}
		return toolUse(`{"path":"/work/` + string(rune('a'+n%26)) + string(rune('a'+n/26)) + `"}`)
	}}
	text, err := runScripted(t, p, func(int) string { return "[]" })
	if err != nil || text != "done" || p.notice.Load() != 0 {
		t.Fatalf("run = %q, %v, notices %d; want done, nil, 0", text, err, p.notice.Load())
	}
}

func TestNoProgressArgumentKeyOrderIsCanonical(t *testing.T) {
	p := &scriptedProvider{next: func(n int) *provideriface.ToolLoopResponse {
		if n%2 == 0 {
			return toolUse(`{"a":1,"b":2}`)
		}
		return toolUse(`{"b":2,"a":1}`)
	}}
	_, err := runScripted(t, p, func(int) string { return "same" })
	if !errors.Is(err, ErrNoProgress) {
		t.Fatalf("err = %v; want ErrNoProgress", err)
	}
}
