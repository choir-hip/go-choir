//go:build comprehensive

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRuntimeRunMemoryThresholdCompaction(t *testing.T) {
	t.Parallel()
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
			Text:       runMemoryCheckpointJSONForTest("remember this through the tool call", []string{"echo tool completed"}, []string{"continue after compaction"}),
			Usage:      TokenUsage{InputTokens: 12, OutputTokens: 6},
			Model:      "test-model",
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
			if entry.Details["checkpoint_status"] != "llm_checkpoint" {
				t.Fatalf("checkpoint_status = %#v, want llm_checkpoint", entry.Details["checkpoint_status"])
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
	startIndex := -1
	completedIndex := -1
	for i, ev := range events {
		if ev.Kind == types.EventRunCompactionStarted && startIndex < 0 {
			startIndex = i
		}
		if ev.Kind == types.EventRunCompactionCompleted && completedIndex < 0 {
			completedIndex = i
		}
		kinds[ev.Kind] = true
	}
	if !kinds[types.EventRunCompactionStarted] || !kinds[types.EventRunCompactionCompleted] {
		t.Fatalf("missing compaction events: %+v", kinds)
	}
	if startIndex < 0 || completedIndex < 0 || startIndex > completedIndex {
		t.Fatalf("compaction event order start=%d completed=%d events=%+v", startIndex, completedIndex, events)
	}
}

func TestRuntimeRunMemoryOverflowRetriesOnceThenCompletes(t *testing.T) {
	t.Parallel()
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
	if got := atomic.LoadInt32(&provider.calls); got != 3 {
		t.Fatalf("provider calls: got %d, want 3 including LLM compaction", got)
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
	t.Parallel()
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
	t.Parallel()
	registry := testRunMemoryRegistry(t)
	provider := newMockToolLoopProvider(
		&ToolLoopResponse{StopReason: "end_turn", Text: "manual compaction target", Model: "test-model"},
		&ToolLoopResponse{StopReason: "end_turn", Text: runMemoryCheckpointJSONForTest("manual compaction prompt", []string{"manual run completed"}, []string{"resume from manual checkpoint"}), Model: "test-model"},
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
	if latest.Details["checkpoint_status"] != "llm_checkpoint" {
		t.Fatalf("checkpoint_status = %#v, want llm_checkpoint", latest.Details["checkpoint_status"])
	}
}

func TestChildRunUsesRunMemory(t *testing.T) {
	t.Parallel()
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

	child, err := rt.StartCoagentRun(context.Background(), parent.RunID, "child objective", "user-alice", map[string]any{
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

func TestRuntimeRunMemoryOverflowRecoveryRetrievesRawEntry(t *testing.T) {
	t.Parallel()
	registry := NewToolRegistry()
	provider := &runMemoryOverflowRetrievalProvider{
		sentinel: "RAW_ENTRY_SENTINEL_6b8c1f0e_exact",
	}
	rt, _ := testRuntimeWithProviderAndRegistry(t, provider, registry)
	if err := RegisterEvidenceTools(registry, rt); err != nil {
		t.Fatalf("register evidence tools: %v", err)
	}
	if err := RegisterRunMemoryTools(registry, rt); err != nil {
		t.Fatalf("register run-memory tools: %v", err)
	}
	rt.cfg.RunMemoryContextThresholdTokens = 1_000_000

	prompt := strings.Join([]string{
		"Diagnostic objective: recover from context overflow by retrieving the raw compacted entry.",
		strings.Repeat("summary-visible filler before the raw-only sentinel ", 20),
		"Exact raw-only value: " + provider.sentinel,
	}, "\n")
	rec, err := rt.StartRun(context.Background(), prompt, "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("state: got %s, want completed", done.State)
	}
	if done.Result != "retrieved raw sentinel" {
		t.Fatalf("result = %q", done.Result)
	}
	if got := atomic.LoadInt32(&provider.calls); got != 4 {
		t.Fatalf("provider calls = %d, want 4 including LLM compaction", got)
	}
}

type runMemoryOverflowRetrievalProvider struct {
	provideriface.Provider
	sentinel   string
	calls      int32
	agentCalls int32
}

func (p *runMemoryOverflowRetrievalProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	atomic.AddInt32(&p.calls, 1)
	if strings.Contains(req.System, "runtime run-memory compactor") {
		return &ToolLoopResponse{
			StopReason: "end_turn",
			Text:       runMemoryCheckpointJSONForTest("recover from overflow", []string{"raw sentinel compacted"}, []string{"call get_run_memory_entry for the compacted entry id"}),
			Usage:      TokenUsage{InputTokens: 12, OutputTokens: 6},
			Model:      req.Model,
		}, nil
	}
	call := atomic.AddInt32(&p.agentCalls, 1)
	raw := rawMessagesForTest(req.Messages)
	switch call {
	case 1:
		if !strings.Contains(raw, p.sentinel) {
			return nil, fmt.Errorf("first raw context missing sentinel before overflow")
		}
		return nil, fmt.Errorf("maximum context length exceeded")
	case 2:
		if strings.Contains(raw, p.sentinel) {
			return nil, fmt.Errorf("compacted retry context leaked exact sentinel")
		}
		entryID := extractRunMemoryEntryIDForTest(raw)
		if entryID == "" {
			return nil, fmt.Errorf("compacted retry context missing raw entry id: %s", raw)
		}
		return &ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-get-memory",
				Name:      "get_run_memory_entry",
				Arguments: json.RawMessage(fmt.Sprintf(`{"entry_id":%q}`, entryID)),
			}},
			Usage: TokenUsage{InputTokens: 10, OutputTokens: 5},
			Model: "test-model",
		}, nil
	case 3:
		if !strings.Contains(raw, p.sentinel) {
			return nil, fmt.Errorf("final context missing sentinel from retrieved raw entry")
		}
		return &ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "retrieved raw sentinel",
			Usage:      TokenUsage{InputTokens: 8, OutputTokens: 4},
			Model:      "test-model",
		}, nil
	default:
		return nil, fmt.Errorf("unexpected provider call %d", call)
	}
}

func extractRunMemoryEntryIDForTest(raw string) string {
	const marker = "entry_id="
	idx := strings.Index(raw, marker)
	if idx >= 0 {
		rest := raw[idx+len(marker):]
		return firstRunMemoryEntryTokenForTest(rest)
	}
	const listMarker = "Compacted raw entry ids available via get_run_memory_entry:"
	idx = strings.Index(raw, listMarker)
	if idx < 0 {
		return ""
	}
	rest := raw[idx+len(listMarker):]
	if dash := strings.Index(rest, "- "); dash >= 0 {
		rest = rest[dash+2:]
	}
	return firstRunMemoryEntryTokenForTest(rest)
}

func firstRunMemoryEntryTokenForTest(rest string) string {
	end := len(rest)
	for i, r := range rest {
		if r == ' ' || r == '\\' || r == '\n' || r == '\t' || r == '"' || r == '<' {
			end = i
			break
		}
	}
	return strings.TrimSpace(rest[:end])
}

func runMemoryCheckpointJSONForTest(objective string, completed, next []string) string {
	payload := runMemoryCheckpoint{
		CurrentObjective:       objective,
		ActiveTask:             "LLM run-memory compaction test",
		UserHardConstraints:    []string{"preserve retrieval handles"},
		CompletedWork:          completed,
		KeyDecisions:           []string{"use LLM checkpoint"},
		OpenObligations:        []string{"continue the run"},
		FailedAttempts:         []string{},
		SourceEvidenceHandles:  []string{},
		RawEntryHandles:        []string{"entry-from-test"},
		RawToolResultHandles:   []string{},
		FilesDocsResources:     []string{},
		BlockersUncertainties:  []string{},
		NextActions:            next,
		RetrievalInstructions:  []string{"call get_run_memory_entry when exact raw content matters"},
		ContinuationCheckpoint: "Continue from this LLM checkpoint.",
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

type runtimeOverflowProvider struct {
	provideriface.Provider
	failuresBeforeSuccess int32
	calls                 int32
	agentCalls            int32
}

func (p *runtimeOverflowProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	atomic.AddInt32(&p.calls, 1)
	if strings.Contains(req.System, "runtime run-memory compactor") {
		return &ToolLoopResponse{
			StopReason: "end_turn",
			Text:       runMemoryCheckpointJSONForTest("recover from overflow", []string{"provider overflow compacted"}, []string{"continue the run"}),
			Usage:      TokenUsage{InputTokens: 12, OutputTokens: 6},
			Model:      req.Model,
		}, nil
	}
	call := atomic.AddInt32(&p.agentCalls, 1)
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
