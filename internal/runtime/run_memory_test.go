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
