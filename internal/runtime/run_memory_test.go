package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestBuildRunMemoryContextUsesLatestCompaction(t *testing.T) {
	entries := []types.RunMemoryEntry{
		{
			EntryID: "m1",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "user",
			Message: json.RawMessage(`{"role":"user","content":"old"}`),
		},
		{
			EntryID: "m2",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "assistant",
			Message: json.RawMessage(`{"role":"assistant","content":"keep"}`),
		},
		{
			EntryID:          "c1",
			Kind:             types.RunMemoryEntryCompaction,
			Summary:          "old user asked for durable memory",
			FirstKeptEntryID: "m2",
			TokensBefore:     100,
		},
		{
			EntryID: "m3",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "user",
			Message: json.RawMessage(`{"role":"user","content":"new"}`),
		},
	}

	messages := buildRunMemoryContext(entries)
	if len(messages) != 3 {
		t.Fatalf("messages: got %d, want 3", len(messages))
	}
	if !json.Valid(messages[0]) {
		t.Fatalf("summary message is not valid JSON: %s", messages[0])
	}
	if string(messages[1]) != string(entries[1].Message) {
		t.Fatalf("kept message: got %s, want %s", messages[1], entries[1].Message)
	}
	if string(messages[2]) != string(entries[3].Message) {
		t.Fatalf("post-compaction message: got %s, want %s", messages[2], entries[3].Message)
	}
}

func TestRunMemoryInitializeSeedsPriorActorSnapshot(t *testing.T) {
	_, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "user-alice"
	agentID := "coagent:memory-rewarm"

	prior := types.RunRecord{
		RunID:     "prior-memory-activation",
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		State:     types.RunPassivated,
		Prompt:    "remember durable actor context",
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentID: agentID,
		},
	}
	if err := s.CreateRun(ctx, prior); err != nil {
		t.Fatalf("create prior run: %v", err)
	}
	priorMessage, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:   prior.RunID,
		OwnerID: ownerID,
		AgentID: agentID,
		Kind:    types.RunMemoryEntryMessage,
		Role:    "user",
		Message: json.RawMessage(`{"role":"user","content":"remember sentinel actor memory"}`),
	})
	if err != nil {
		t.Fatalf("append prior memory message: %v", err)
	}
	if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:            prior.RunID,
		OwnerID:          ownerID,
		AgentID:          agentID,
		Kind:             types.RunMemoryEntryCompaction,
		Summary:          "sentinel compacted actor memory",
		FirstKeptEntryID: priorMessage.EntryID,
		TokensBefore:     42,
		Reason:           "threshold",
	}); err != nil {
		t.Fatalf("append prior memory compaction: %v", err)
	}
	olderFinishedAt := now.Add(-time.Minute)
	olderCompleted := types.RunRecord{
		RunID:      "older-completed-memory-activation",
		AgentID:    agentID,
		OwnerID:    ownerID,
		SandboxID:  "sandbox-test",
		State:      types.RunCompleted,
		Prompt:     "older completed context",
		Result:     "done",
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now.Add(-time.Minute),
		FinishedAt: &olderFinishedAt,
	}
	if err := s.CreateRun(ctx, olderCompleted); err != nil {
		t.Fatalf("create older completed run: %v", err)
	}
	if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:   olderCompleted.RunID,
		OwnerID: ownerID,
		AgentID: agentID,
		Kind:    types.RunMemoryEntryMessage,
		Role:    "user",
		Message: json.RawMessage(`{"role":"user","content":"older completed memory must not win by memory row time"}`),
	}); err != nil {
		t.Fatalf("append older completed memory: %v", err)
	}
	blocked := types.RunRecord{
		RunID:     "blocked-memory-activation",
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		State:     types.RunBlocked,
		Prompt:    "blocked unresolved context",
		Error:     "still unresolved",
		CreatedAt: now.Add(2 * time.Second),
		UpdatedAt: now.Add(2 * time.Second),
	}
	if err := s.CreateRun(ctx, blocked); err != nil {
		t.Fatalf("create blocked run: %v", err)
	}
	if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:   blocked.RunID,
		OwnerID: ownerID,
		AgentID: agentID,
		Kind:    types.RunMemoryEntryMessage,
		Role:    "user",
		Message: json.RawMessage(`{"role":"user","content":"blocked memory must not seed a new activation"}`),
	}); err != nil {
		t.Fatalf("append blocked memory: %v", err)
	}

	current := types.RunRecord{
		RunID:     "rewarm-memory-activation",
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		State:     types.RunPending,
		Prompt:    "new wake update",
		CreatedAt: now.Add(time.Second),
		UpdatedAt: now.Add(time.Second),
		Metadata: map[string]any{
			runMetadataAgentID: agentID,
		},
	}
	if err := s.CreateRun(ctx, current); err != nil {
		t.Fatalf("create current run: %v", err)
	}

	memory := newRunMemoryManager(s, &current, provideriface.Config{}, nil)
	messages, err := memory.initialize(ctx, []json.RawMessage{
		json.RawMessage(`{"role":"user","content":"new wake update"}`),
	})
	if err != nil {
		t.Fatalf("initialize run memory: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want actor snapshot + wake message: %+v", len(messages), messages)
	}
	if !strings.Contains(string(messages[0]), "sentinel compacted actor memory") {
		t.Fatalf("snapshot message missing prior memory: %s", messages[0])
	}
	if !strings.Contains(string(messages[0]), "remember sentinel actor memory") {
		t.Fatalf("snapshot message missing raw message retained by prior checkpoint: %s", messages[0])
	}
	if strings.Contains(string(messages[0]), "older completed memory") ||
		strings.Contains(string(messages[0]), "blocked memory") {
		t.Fatalf("snapshot message crossed wrong prior activation: %s", messages[0])
	}
	if !strings.Contains(string(messages[1]), "new wake update") {
		t.Fatalf("wake message missing: %s", messages[1])
	}

	entries, err := s.ListRunMemoryEntries(ctx, ownerID, current.RunID)
	if err != nil {
		t.Fatalf("list current entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("current entries = %+v, want snapshot + wake message", entries)
	}
	if entries[0].Kind != types.RunMemoryEntryCompaction || entries[0].Reason != "actor_rewarm" {
		t.Fatalf("first entry = %+v, want actor_rewarm compaction", entries[0])
	}
	if entries[0].Details["source_loop_id"] != prior.RunID {
		t.Fatalf("source_loop_id = %#v, want %s", entries[0].Details["source_loop_id"], prior.RunID)
	}
	if entries[1].Kind != types.RunMemoryEntryMessage {
		t.Fatalf("second entry = %+v, want wake message", entries[1])
	}
}

func TestRunMemoryCompactionDoesNotSplitToolResultPair(t *testing.T) {
	entries := []types.RunMemoryEntry{
		{
			EntryID: "m1",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "user",
			Message: json.RawMessage(`{"role":"user","content":"please calculate"}`),
		},
		{
			EntryID: "m2",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "assistant",
			Message: json.RawMessage(`{"role":"assistant","content":[{"type":"tool_use","id":"call-1","name":"calculator","input":{"expr":"2+2"}}]}`),
		},
		{
			EntryID: "m3",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "user",
			Message: json.RawMessage(`{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-1","content":"4"}]}`),
		},
	}

	plan, ok := planRunMemoryCompaction(entries, 1, "threshold")
	if !ok {
		t.Fatalf("expected compaction plan")
	}
	if plan.FirstKeptEntryID != "m2" {
		t.Fatalf("first kept entry: got %q, want assistant tool_use m2", plan.FirstKeptEntryID)
	}

	withCompaction := append(entries, types.RunMemoryEntry{
		EntryID:          "c1",
		Kind:             types.RunMemoryEntryCompaction,
		Summary:          plan.Summary,
		FirstKeptEntryID: plan.FirstKeptEntryID,
		TokensBefore:     plan.TokensBefore,
	})
	messages := buildRunMemoryContext(withCompaction)
	if len(messages) != 3 {
		t.Fatalf("rebuilt messages: got %d, want summary + assistant + tool_result", len(messages))
	}
	if !assistantMessageHasToolUse(messages[1]) {
		t.Fatalf("expected assistant tool_use to precede tool_result: %s", messages[1])
	}
	if !isToolResultOnlyMessage(messages[2]) {
		t.Fatalf("expected tool_result after assistant tool_use: %s", messages[2])
	}
}

func TestRunMemoryCompactionNamesRawRetrievalEntries(t *testing.T) {
	entries := []types.RunMemoryEntry{
		{
			EntryID: "m1",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "user",
			Message: json.RawMessage(`{"role":"user","content":"please calculate"}`),
		},
		{
			EntryID: "m2",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "assistant",
			Message: json.RawMessage(`{"role":"assistant","content":[{"type":"tool_use","id":"call-1","name":"calculator","input":{"expr":"2+2"}}]}`),
		},
		{
			EntryID: "m3",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "user",
			Message: json.RawMessage(`{"role":"user","content":[{"type":"tool_result","tool_use_id":"call-1","content":"4"}]}`),
		},
		{
			EntryID: "m4",
			Kind:    types.RunMemoryEntryMessage,
			Role:    "assistant",
			Message: json.RawMessage(`{"role":"assistant","content":"done"}`),
		},
	}

	plan, ok := planRunMemoryCompaction(entries, 1, "threshold")
	if !ok {
		t.Fatalf("expected compaction plan")
	}
	if len(plan.RawEntryIDs) == 0 {
		t.Fatalf("raw entry ids missing")
	}
	if !strings.Contains(plan.Summary, "entry_id=m1") {
		t.Fatalf("summary does not name raw entry ids:\n%s", plan.Summary)
	}
	foundToolResult := false
	for _, id := range plan.RawToolResultEntryIDs {
		if id == "m3" {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Fatalf("raw tool result ids = %#v, want m3", plan.RawToolResultEntryIDs)
	}
}

func TestRunMemoryCheckpointParsesAndRendersRetrievalHandles(t *testing.T) {
	checkpoint, err := parseRunMemoryCheckpoint(`{
		"current_objective":"finish provider conformance",
		"active_task":"prove LLM compaction",
		"user_hard_constraints":["no arbitrary max_tokens"],
		"completed_work":["wired DeepSeek"],
		"key_decisions":["use 70 percent threshold"],
		"open_obligations":["prove Node B"],
		"failed_attempts":["deterministic summary is not enough"],
		"source_evidence_handles":["trace-1"],
		"raw_entry_handles":["m1"],
		"raw_tool_result_handles":["m2"],
		"files_docs_resources":["docs/mission-llm-run-memory-compaction-v0.md"],
		"blockers_uncertainties":["staging proof missing"],
		"next_actions":["run product path"],
		"retrieval_instructions":["call get_run_memory_entry for m1 if exact text matters"],
		"continuation_checkpoint":"Continue from the provider conformance proof."
	}`)
	if err != nil {
		t.Fatalf("parse checkpoint: %v", err)
	}
	plan := runMemoryCompactionPlan{
		Reason:                "threshold",
		RawEntryIDs:           []string{"m1"},
		RawToolResultEntryIDs: []string{"m2"},
	}
	summary := renderRunMemoryCheckpointSummary(checkpoint, plan)
	for _, want := range []string{
		"Run memory LLM checkpoint",
		"finish provider conformance",
		"no arbitrary max_tokens",
		"get_run_memory_entry",
		"m1",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
	details := checkpointDetails(checkpoint)
	if got := details["current_objective"]; got != "finish provider conformance" {
		t.Fatalf("details current_objective = %#v", got)
	}
}

func TestRunMemoryCheckpointParserAcceptsScalarListFields(t *testing.T) {
	checkpoint, err := parseRunMemoryCheckpoint(`{
		"current_objective":"finish provider conformance",
		"active_task":"prove live compaction",
		"user_hard_constraints":"no arbitrary max_tokens",
		"raw_entry_handles":"entry-live-1",
		"raw_tool_result_handles":"entry-tool-live-1",
		"next_actions":"continue provider conformance",
		"retrieval_instructions":"call get_run_memory_entry for entry-live-1",
		"continuation_checkpoint":"Continue."
	}`)
	if err != nil {
		t.Fatalf("parse checkpoint: %v", err)
	}
	if len(checkpoint.UserHardConstraints) != 1 || checkpoint.UserHardConstraints[0] != "no arbitrary max_tokens" {
		t.Fatalf("constraints = %#v", checkpoint.UserHardConstraints)
	}
	if len(checkpoint.RawEntryHandles) != 1 || checkpoint.RawEntryHandles[0] != "entry-live-1" {
		t.Fatalf("raw entry handles = %#v", checkpoint.RawEntryHandles)
	}
}

func TestRunMemoryCompactionPromptIncludesObjectiveAndRetrievalInstructions(t *testing.T) {
	rec := &types.RunRecord{
		RunID:        "run-1",
		State:        types.RunRunning,
		AgentProfile: agentprofile.Super,
		Prompt:       "Run docs/mission-llm-run-memory-compaction-v0.md as MissionGradient.",
	}
	plan := runMemoryCompactionPlan{
		Reason:                "threshold",
		RawEntryIDs:           []string{"entry-user-1"},
		RawToolResultEntryIDs: []string{"entry-tool-1"},
		SummarizedEntries: []types.RunMemoryEntry{
			{
				EntryID: "entry-user-1",
				Seq:     1,
				Role:    "user",
				Message: json.RawMessage(`{"role":"user","content":"Do not use arbitrary max_tokens caps."}`),
			},
			{
				EntryID: "entry-tool-1",
				Seq:     2,
				Role:    "user",
				Message: json.RawMessage(`{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"provider conformance evidence"}]}`),
			},
		},
	}
	prompt := buildRunMemoryCompactionPrompt(rec, plan)
	for _, want := range []string{
		"current_objective",
		"Run docs/mission-llm-run-memory-compaction-v0.md",
		"entry-user-1",
		"entry-tool-1",
		"get_run_memory_entry",
		"Do not use arbitrary max_tokens caps",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestRunMemoryEffectiveThresholdUsesModelContextWindow(t *testing.T) {
	manager := newRunMemoryManager(nil, &types.RunRecord{}, provideriface.Config{}, nil).
		withLLMCompactor(nil, provideriface.LLMSelection{Model: "deepseek-v4-flash"}, 0)
	if got := manager.effectiveContextThresholdTokens(); got != 700000 {
		t.Fatalf("threshold = %d, want 700000", got)
	}
	manager.cfg.RunMemoryContextThresholdTokens = 160000
	if got := manager.effectiveContextThresholdTokens(); got != 160000 {
		t.Fatalf("explicit threshold = %d, want 160000", got)
	}
}

func TestContextOverflowErrorDetection(t *testing.T) {
	if !isContextOverflowError(errors.New("provider rejected request: maximum context length exceeded")) {
		t.Fatalf("expected maximum context length error to be detected")
	}
	if !isContextOverflowError(errors.New("prompt is too long for this model")) {
		t.Fatalf("expected prompt-too-long error to be detected")
	}
	if isContextOverflowError(errors.New("network timeout")) {
		t.Fatalf("unexpected context overflow detection")
	}
}
