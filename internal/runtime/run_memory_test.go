package runtime

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

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
		AgentProfile: AgentProfileSuper,
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
	manager := newRunMemoryManager(nil, &types.RunRecord{}, Config{}, nil).
		withLLMCompactor(nil, LLMSelection{Model: "deepseek-v4-flash"}, 0)
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
