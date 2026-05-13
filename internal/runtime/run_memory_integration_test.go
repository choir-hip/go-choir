package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRuntimeRunMemoryThresholdCompaction(t *testing.T) {
	registry := testRunMemoryRegistry(t)
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{ID: "call-1", Name: "echo", Arguments: json.RawMessage(`{"text":"hello"}`)},
			},
			Usage: TokenUsage{InputTokens: 10, OutputTokens: 5},
			Model: "test-model",
		},
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "finished after compaction",
			Usage:      TokenUsage{InputTokens: 8, OutputTokens: 4},
			Model:      "test-model",
		},
	)
	rt, s := testRuntimeWithProviderAndRegistry(t, provider, registry)
	rt.cfg.RunMemoryContextThresholdTokens = 1
	rt.cfg.RunMemoryKeepRecentTokens = 1

	rec, err := rt.StartRun(context.Background(), "remember this through the tool call", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("state: got %s, want completed", done.State)
	}

	entries, err := s.ListRunMemoryEntries(context.Background(), "user-alice", rec.RunID)
	if err != nil {
		t.Fatalf("list run memory: %v", err)
	}
	foundCompaction := false
	for _, entry := range entries {
		if entry.Kind == types.RunMemoryEntryCompaction {
			foundCompaction = true
			if entry.Summary == "" {
				t.Fatalf("compaction summary should not be empty")
			}
			if entry.TokensBefore == 0 {
				t.Fatalf("tokens_before should be recorded")
			}
		}
	}
	if !foundCompaction {
		t.Fatalf("expected run-memory compaction entry, got %+v", entries)
	}

	events, err := s.ListEvents(context.Background(), rec.RunID, 100)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	kinds := map[types.EventKind]bool{}
	for _, ev := range events {
		kinds[ev.Kind] = true
	}
	if !kinds[types.EventRunCompactionStarted] || !kinds[types.EventRunCompactionCompleted] {
		t.Fatalf("missing compaction events: %+v", kinds)
	}
}

func TestRuntimeRunMemoryOverflowRetriesOnceThenCompletes(t *testing.T) {
	registry := testRunMemoryRegistry(t)
	provider := &runtimeOverflowProvider{
		failuresBeforeSuccess: 1,
	}
	rt, s := testRuntimeWithProviderAndRegistry(t, provider, registry)
	rt.cfg.RunMemoryContextThresholdTokens = 100000
	rt.cfg.RunMemoryKeepRecentTokens = 1

	rec, err := rt.StartRun(context.Background(), "a long prompt that will overflow once", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("state: got %s, want completed", done.State)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 2 {
		t.Fatalf("provider calls: got %d, want 2", got)
	}

	events, err := s.ListEvents(context.Background(), rec.RunID, 100)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	kinds := map[types.EventKind]bool{}
	for _, ev := range events {
		kinds[ev.Kind] = true
	}
	if !kinds[types.EventRunRetry] || !kinds[types.EventRunCompactionCompleted] {
		t.Fatalf("missing retry/compaction events: %+v", kinds)
	}
}

func TestRuntimeRunMemoryOverflowFailureBlocksRun(t *testing.T) {
	registry := testRunMemoryRegistry(t)
	provider := &runtimeOverflowProvider{
		failuresBeforeSuccess: 3,
	}
	rt, _ := testRuntimeWithProviderAndRegistry(t, provider, registry)
	rt.cfg.RunMemoryKeepRecentTokens = 1

	rec, err := rt.StartRun(context.Background(), "a prompt that keeps overflowing", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	blocked := waitForRunState(t, rt, rec.RunID, "user-alice", types.RunBlocked, 5*time.Second)
	if blocked.FinishedAt != nil {
		t.Fatalf("blocked run should not have finished_at")
	}
	if !isContextOverflowError(errors.New(blocked.Error)) {
		t.Fatalf("blocked error should preserve context-overflow evidence: %s", blocked.Error)
	}
}

func TestRuntimeManualRunMemoryCompaction(t *testing.T) {
	registry := testRunMemoryRegistry(t)
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{StopReason: "end_turn", Text: "manual compaction target", Model: "test-model"},
	)
	rt, s := testRuntimeWithProviderAndRegistry(t, provider, registry)
	rt.cfg.RunMemoryKeepRecentTokens = 1

	rec, err := rt.StartRun(context.Background(), "manual compaction prompt", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("state: got %s, want completed", done.State)
	}

	if err := rt.CompactRunMemory(context.Background(), rec.RunID, "user-alice", "manual_test"); err != nil {
		t.Fatalf("manual compact: %v", err)
	}
	entries, err := s.ListRunMemoryEntries(context.Background(), "user-alice", rec.RunID)
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	latest := entries[len(entries)-1]
	if latest.Kind != types.RunMemoryEntryCompaction {
		t.Fatalf("latest entry kind: got %s, want compaction", latest.Kind)
	}
	if latest.Reason != "manual_test" {
		t.Fatalf("compaction reason: got %q, want manual_test", latest.Reason)
	}
}

func TestChildRunUsesRunMemory(t *testing.T) {
	registry := testRunMemoryRegistry(t)
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{StopReason: "end_turn", Text: "parent done", Model: "test-model"},
		&ToolLoopResponse{StopReason: "end_turn", Text: "child done", Model: "test-model"},
	)
	rt, s := testRuntimeWithProviderAndRegistry(t, provider, registry)

	parent, err := rt.StartRun(context.Background(), "parent", "user-alice")
	if err != nil {
		t.Fatalf("start parent: %v", err)
	}
	waitForRunTerminalState(t, rt, parent.RunID, "user-alice", 5*time.Second)

	child, err := rt.StartChildRun(context.Background(), parent.RunID, "child objective", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
	})
	if err != nil {
		t.Fatalf("start child: %v", err)
	}
	done := waitForRunTerminalState(t, rt, child.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("child state: got %s, want completed", done.State)
	}

	entries, err := s.ListRunMemoryEntries(context.Background(), "user-alice", child.RunID)
	if err != nil {
		t.Fatalf("list child run memory: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("child run memory entries: got %d, want initial and final messages", len(entries))
	}
	if entries[0].Kind != types.RunMemoryEntryMessage || entries[0].Role != "user" {
		t.Fatalf("first child memory entry should be user message, got %+v", entries[0])
	}
}

type runtimeOverflowProvider struct {
	Provider
	failuresBeforeSuccess int32
	calls                 int32
}

func (p *runtimeOverflowProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	call := atomic.AddInt32(&p.calls, 1)
	if call <= p.failuresBeforeSuccess {
		return nil, fmt.Errorf("maximum context length exceeded")
	}
	return &ToolLoopResponse{
		StopReason: "end_turn",
		Text:       fmt.Sprintf("recovered with %d messages", len(req.Messages)),
		Usage:      TokenUsage{InputTokens: 3, OutputTokens: 2},
		Model:      "test-model",
	}, nil
}

func testRunMemoryRegistry(t *testing.T) *ToolRegistry {
	t.Helper()
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "echo",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return string(args), nil
		},
	}); err != nil {
		t.Fatalf("register echo: %v", err)
	}
	return registry
}

func waitForRunState(t *testing.T, rt *Runtime, runID, ownerID string, state types.RunState, timeout time.Duration) types.RunRecord {
	t.Helper()

	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := rt.GetRun(ctx, runID, ownerID)
		if err != nil {
			t.Fatalf("get run: %v", err)
		}
		if rec.State == state {
			return *rec
		}
		time.Sleep(25 * time.Millisecond)
	}

	rec, err := rt.GetRun(ctx, runID, ownerID)
	if err != nil {
		t.Fatalf("get run after timeout: %v", err)
	}
	t.Fatalf("timeout waiting for run %s state %s (got %s)", runID[:8], state, rec.State)
	return types.RunRecord{}
}
