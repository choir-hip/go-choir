package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// threadCompactionProvider answers the compaction request as scripted and
// everything else as a short run (one tool call, then a final text), and
// records every request.
type threadCompactionProvider struct {
	provideriface.Provider
	mu         sync.Mutex
	requests   []provideriface.ToolLoopRequest
	compaction func() (*provideriface.ToolLoopResponse, error)
	agentCalls int
}

func (p *threadCompactionProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()
	if isRunMemoryCompactionRequestForTest(req) {
		return p.compaction()
	}
	p.mu.Lock()
	p.agentCalls++
	n := p.agentCalls
	p.mu.Unlock()
	if n == 1 {
		return &provideriface.ToolLoopResponse{StopReason: "tool_use", Model: "test-model",
			ToolCalls: []types.ToolCall{{ID: "call-1", Name: "echo", Arguments: json.RawMessage(`{"text":"hi"}`)}}}, nil
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "finished", Model: "test-model"}, nil
}

func runThreadCompaction(t *testing.T, provider *threadCompactionProvider) (types.RunRecord, []types.RunMemoryEntry) {
	t.Helper()
	rt, s := testRuntimeWithProviderAndRegistry(t, provider, testRunMemoryRegistry(t))
	rt.cfg.RunMemoryContextThresholdTokens = 1
	rt.cfg.RunMemoryKeepRecentTokens = 1
	rec, err := rt.StartRun(context.Background(), "remember this through the tool call", "user-alice")
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	entries, err := s.ListRunMemoryEntries(context.Background(), "user-alice", rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	return done, entries
}

// The compaction request is the thread's own: the loop's system prompt and
// tools, its messages, then one instruction as the last user message.
func TestRunMemoryCompactionIsTheThreadsOwnRequest(t *testing.T) {
	t.Parallel()
	provider := &threadCompactionProvider{compaction: func() (*provideriface.ToolLoopResponse, error) {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Model: "test-model",
			Text: runMemoryCheckpointJSONForTest("thread checkpoint", []string{"echo"}, []string{"go on"})}, nil
	}}
	done, entries := runThreadCompaction(t, provider)
	if done.State != types.RunCompleted {
		t.Fatalf("state = %s", done.State)
	}
	var compactionReq, plain provideriface.ToolLoopRequest
	for _, r := range provider.requests {
		if isRunMemoryCompactionRequestForTest(r) {
			compactionReq = r
		} else if plain.System == "" {
			plain = r
		}
	}
	if compactionReq.System == "" || compactionReq.System != plain.System {
		t.Fatalf("compaction system prompt is not the thread's: %q", compactionReq.System)
	}
	if strings.Contains(compactionReq.System, "compactor") {
		t.Fatal("a compactor system prompt survived")
	}
	if len(compactionReq.ToolDefinitions) == 0 || len(compactionReq.ToolDefinitions) != len(plain.ToolDefinitions) {
		t.Fatalf("compaction request tools = %d, thread tools = %d", len(compactionReq.ToolDefinitions), len(plain.ToolDefinitions))
	}
	if compactionReq.ToolChoice != "" || compactionReq.CacheKey != plain.CacheKey || compactionReq.ConversationID != plain.ConversationID {
		t.Fatalf("compaction request is not the thread's: tool_choice %q cache %q conversation %q", compactionReq.ToolChoice, compactionReq.CacheKey, compactionReq.ConversationID)
	}
	if !strings.Contains(rawMessagesForTest(compactionReq.Messages), "remember this through the tool call") {
		t.Fatalf("compaction request does not carry the thread's messages: %s", rawMessagesForTest(compactionReq.Messages))
	}
	found := false
	for _, e := range entries {
		if e.Kind == types.RunMemoryEntryCompaction && e.Details["checkpoint_status"] == "llm_checkpoint" {
			found = true
		}
	}
	if !found {
		t.Fatal("no llm_checkpoint compaction entry")
	}
}

// A model error, an unparseable reply, or a tool call instead of text each
// give the deterministic compaction, and the run continues.
func TestRunMemoryCompactionFallsToDeterministicSummary(t *testing.T) {
	t.Parallel()
	cases := map[string]func() (*provideriface.ToolLoopResponse, error){
		"model error": func() (*provideriface.ToolLoopResponse, error) { return nil, fmt.Errorf("quota exhausted") },
		"not json": func() (*provideriface.ToolLoopResponse, error) {
			return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "I would summarize it as follows."}, nil
		},
		"tool call": func() (*provideriface.ToolLoopResponse, error) {
			return &provideriface.ToolLoopResponse{StopReason: "tool_use",
				ToolCalls: []types.ToolCall{{ID: "x", Name: "echo", Arguments: json.RawMessage(`{}`)}}}, nil
		},
	}
	for name, script := range cases {
		script := script
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			done, entries := runThreadCompaction(t, &threadCompactionProvider{compaction: script})
			if done.State != types.RunCompleted {
				t.Fatalf("state = %s, want completed", done.State)
			}
			for _, e := range entries {
				if e.Kind == types.RunMemoryEntryCompaction {
					if e.Details["checkpoint_status"] == "llm_checkpoint" {
						t.Fatal("expected the deterministic compaction")
					}
					return
				}
			}
			t.Fatal("no compaction entry")
		})
	}
}

// Compaction outside a loop has no thread to ask.
func TestRunMemoryCompactionWithoutThreadCallErrors(t *testing.T) {
	m := &runMemoryManager{rec: &types.RunRecord{}}
	if _, err := m.askThreadForCompaction(context.Background(), runMemoryCompactionPlan{}); err == nil {
		t.Fatal("want an error without a thread call")
	}
}
