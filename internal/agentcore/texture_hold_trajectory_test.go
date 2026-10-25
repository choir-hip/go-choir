package agentcore

// Texture holds its trajectory (docs/design/texture-holds-the-trajectory-2026-10-25.md).
// A committed Texture wake starts a new run whose memory was seeded with the
// actor rewarm digest: tool_use inputs dropped, text cut to 350 characters,
// so the desk never saw its own casts or the owner's earlier directions.
// With actor_rewarm_carry_conversation (set by textureowner when
// texture.hold_trajectory is on) the rewarm carries the earlier run's
// checkpoint and messages verbatim. Ways it can fail, written before the
// code (the note's numbering):
//
//  1. Off changes the seed.
//  2. On, the earlier conversation is still lossy (a cast in a cell is lost).
//  3. A carried runtime injection counts as delivered in the new run.
//  4. The carry crosses actors.
//  5. The carried tail ends on a tool_use with no result.
//  6. Old run-context turns are carried.
//  7. A carried checkpoint is re-expanded or dropped.
//  8. A non-Texture run with the flag changes.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	holdOwner    = "user-hold"
	holdComputer = "autoputer-test"
	holdAgent    = "texture:doc-hold"
)

// holdCastText is longer than every digest cut, so only a verbatim carry
// shows its end.
var holdCastText = "Owner correction, replacing the rename: the Reader's manifest has aliases [] and PdfApp.svelte and EpubApp.svelte stay exactly as on the base. " +
	strings.Repeat("Everything else in the request stands. ", 40) + "END-OF-OWNER-WORDS"

func holdMessage(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func holdText(role, text string) map[string]any {
	return map[string]any{"role": role, "content": []any{map[string]string{"type": "text", "text": text}}}
}

func holdRun(t *testing.T, s *choirstore.Store, runID, agentID string, at time.Time, metadata map[string]any) types.RunRecord {
	t.Helper()
	md := map[string]any{runMetadataAgentID: agentID, "agent_profile": agentprofile.Texture}
	for k, v := range metadata {
		md[k] = v
	}
	rec := types.RunRecord{
		RunID: runID, AgentID: agentID, OwnerID: holdOwner, ComputerID: holdComputer,
		AgentProfile: md["agent_profile"].(string), State: types.RunPassivated, Prompt: "wake",
		CreatedAt: at, UpdatedAt: at, Metadata: md,
	}
	if err := s.CreateRun(context.Background(), rec); err != nil {
		t.Fatalf("create run %s: %v", runID, err)
	}
	return rec
}

func holdAppend(t *testing.T, s *choirstore.Store, rec types.RunRecord, kind types.RunMemoryEntryKind, role string, msg json.RawMessage, summary string) types.RunMemoryEntry {
	t.Helper()
	entry, err := s.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{
		RunID: rec.RunID, OwnerID: rec.OwnerID, AgentID: rec.AgentID, Kind: kind, Role: role, Message: msg, Summary: summary,
	})
	if err != nil {
		t.Fatalf("append memory: %v", err)
	}
	return entry
}

// seedEarlierTextureRun writes the memory a committed Texture wake leaves:
// its run-context turn, its request turn, the frame (a runtime injection), a
// cell that cast the owner's words, its result, and a last cell cut off
// before its result.
func seedEarlierTextureRun(t *testing.T, s *choirstore.Store, runID string, at time.Time) types.RunRecord {
	t.Helper()
	rec := holdRun(t, s, runID, holdAgent, at, nil)
	holdAppend(t, s, rec, types.RunMemoryEntryMessage, "user", runContextMessage(time.Date(2026, 10, 25, 5, 27, 0, 0, time.UTC), "OLD-RUN-FACTS"), "")
	holdAppend(t, s, rec, types.RunMemoryEntryMessage, "user", holdMessage(t, holdText("user", "Your document woke you. The owner's direction for this revision: keep the old readers.")), "")
	holdAppend(t, s, rec, types.RunMemoryEntryMessage, types.RunMemoryRoleRuntimeInjection, holdMessage(t, holdText("user", "FRAME-OF-EARLIER-RUN")), "")
	cast := map[string]any{"role": "assistant", "content": []any{map[string]any{
		"type": "tool_use", "id": "call-cast", "name": "desk_go_eval",
		"input": map[string]any{"code": `choir.Cast("management", "build the Reader", map[string]any{"kind": "execution_request", "summary": ` + "`" + holdCastText + "`" + `})`},
	}}}
	holdAppend(t, s, rec, types.RunMemoryEntryMessage, "assistant", holdMessage(t, cast), "")
	result := map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "call-cast", "content": "cast committed"}}}
	holdAppend(t, s, rec, types.RunMemoryEntryMessage, "user", holdMessage(t, result), "")
	dangling := map[string]any{"role": "assistant", "content": []any{map[string]any{
		"type": "tool_use", "id": "call-dangling", "name": "desk_go_eval", "input": map[string]any{"code": "DANGLING-CELL"},
	}}}
	holdAppend(t, s, rec, types.RunMemoryEntryMessage, "assistant", holdMessage(t, dangling), "")
	return rec
}

func initializeHoldRun(t *testing.T, s *choirstore.Store, rec types.RunRecord) ([]json.RawMessage, []types.RunMemoryEntry) {
	t.Helper()
	ctx := context.Background()
	memory := newRunMemoryManager(s, &rec, provideriface.Config{}, nil)
	messages, err := memory.initialize(ctx, []json.RawMessage{
		runContextMessage(time.Now(), "NEW-RUN-FACTS"),
		holdMessage(t, holdText("user", "Your document woke you. Intent: integrate_execution_findings.")),
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	entries, err := s.ListRunMemoryEntries(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	return messages, entries
}

func joinedMessages(messages []json.RawMessage) string {
	var b strings.Builder
	for _, m := range messages {
		b.Write(m)
		b.WriteString("\n")
	}
	return b.String()
}

// Failure modes 2, 3, 5 and 6.
func TestTextureHoldTrajectoryCarriesTheEarlierConversationVerbatim(t *testing.T) {
	_, s := testRuntime(t)
	now := time.Now().UTC()
	prior := seedEarlierTextureRun(t, s, "texture-hold-prior", now)
	current := holdRun(t, s, "texture-hold-current", holdAgent, now.Add(time.Second), map[string]any{
		runMetadataCarryActorConversation: true,
		runMetadataActorRewarmSource:      prior.RunID,
	})
	current.State = types.RunPending
	messages, entries := initializeHoldRun(t, s, current)
	all := joinedMessages(messages)

	if !strings.Contains(all, "END-OF-OWNER-WORDS") {
		t.Fatalf("the earlier cast is not carried whole:\n%s", all)
	}
	if !strings.Contains(all, "keep the old readers") || !strings.Contains(all, "FRAME-OF-EARLIER-RUN") {
		t.Fatalf("the earlier request turn or frame is not carried:\n%s", all)
	}
	if strings.Contains(all, "DANGLING-CELL") {
		t.Fatalf("a tool_use without its result was carried:\n%s", all)
	}
	if strings.Contains(all, "OLD-RUN-FACTS") {
		t.Fatalf("the earlier run-context turn was carried:\n%s", all)
	}
	if !strings.Contains(all, "NEW-RUN-FACTS") || !strings.Contains(string(messages[len(messages)-1]), "integrate_execution_findings") {
		t.Fatalf("the new wake's turns do not follow the carried conversation:\n%s", all)
	}
	if strings.Contains(all, "Actor memory snapshot") {
		t.Fatalf("the digest snapshot was seeded as well:\n%s", all)
	}
	for _, entry := range entries {
		if entry.Role == types.RunMemoryRoleRuntimeInjection {
			t.Fatalf("a carried entry kept the runtime_injection role: %+v", entry)
		}
	}
	// The carried frame is not this run's frame: the mark stays unset so the
	// runtime injects a fresh one.
	if mark := textureFrameMarkFromMemory(entries); mark.ok {
		t.Fatalf("carried memory set the new run's frame mark: %+v", mark)
	}
}

// Failure mode 1: the flag off seeds the digest as before.
func TestTextureHoldTrajectoryOffSeedsTheDigest(t *testing.T) {
	_, s := testRuntime(t)
	now := time.Now().UTC()
	prior := seedEarlierTextureRun(t, s, "texture-hold-off-prior", now)
	current := holdRun(t, s, "texture-hold-off-current", holdAgent, now.Add(time.Second), map[string]any{
		runMetadataActorRewarmSource: prior.RunID,
	})
	current.State = types.RunPending
	messages, entries := initializeHoldRun(t, s, current)
	if entries[0].Kind != types.RunMemoryEntryCompaction || entries[0].Reason != "actor_rewarm" ||
		!strings.Contains(entries[0].Summary, "Actor memory snapshot from prior activation "+prior.RunID) {
		t.Fatalf("off did not seed the digest: %+v", entries[0])
	}
	if strings.Contains(joinedMessages(messages), "END-OF-OWNER-WORDS") {
		t.Fatal("off carried the cast verbatim")
	}
}

// Failure mode 8: the flag is Texture's only.
func TestTextureHoldTrajectoryFlagIgnoredOffTexture(t *testing.T) {
	_, s := testRuntime(t)
	now := time.Now().UTC()
	agentID := "research:hold"
	prior := holdRun(t, s, "research-hold-prior", agentID, now, map[string]any{"agent_profile": agentprofile.Research})
	holdAppend(t, s, prior, types.RunMemoryEntryMessage, "user", holdMessage(t, holdText("user", "research memory")), "")
	current := holdRun(t, s, "research-hold-current", agentID, now.Add(time.Second), map[string]any{
		"agent_profile": agentprofile.Research, runMetadataCarryActorConversation: true,
	})
	current.State = types.RunPending
	_, entries := initializeHoldRun(t, s, current)
	if entries[0].Kind != types.RunMemoryEntryCompaction || !strings.Contains(entries[0].Summary, "Actor memory snapshot") {
		t.Fatalf("a research run with the flag did not seed the digest: %+v", entries[0])
	}
}

// Failure mode 4: a source run holding another agent's memory is refused;
// the newest run of this agent is carried instead.
func TestTextureHoldTrajectoryRefusesAnotherAgentsSource(t *testing.T) {
	_, s := testRuntime(t)
	now := time.Now().UTC()
	seedEarlierTextureRun(t, s, "texture-hold-own", now)
	foreign := holdRun(t, s, "texture-hold-foreign", "texture:doc-other", now.Add(2*time.Second), nil)
	holdAppend(t, s, foreign, types.RunMemoryEntryMessage, "user", holdMessage(t, holdText("user", "FOREIGN-DOCUMENT-MEMORY")), "")
	current := holdRun(t, s, "texture-hold-cross", holdAgent, now.Add(3*time.Second), map[string]any{
		runMetadataCarryActorConversation: true,
		runMetadataActorRewarmSource:      foreign.RunID,
	})
	current.State = types.RunPending
	messages, _ := initializeHoldRun(t, s, current)
	all := joinedMessages(messages)
	if strings.Contains(all, "FOREIGN-DOCUMENT-MEMORY") {
		t.Fatalf("another document's memory was carried:\n%s", all)
	}
	if !strings.Contains(all, "END-OF-OWNER-WORDS") {
		t.Fatalf("this agent's own earlier conversation was not carried instead:\n%s", all)
	}
}

// Failure mode 7: an earlier checkpoint is carried as this run's checkpoint,
// followed only by the messages after it (a result whose call was compacted
// away is not carried: a provider refuses it, failure mode 5's twin).
func TestTextureHoldTrajectoryCarriesTheCheckpointAsACheckpoint(t *testing.T) {
	_, s := testRuntime(t)
	now := time.Now().UTC()
	prior := holdRun(t, s, "texture-hold-ckpt-prior", holdAgent, now, nil)
	holdAppend(t, s, prior, types.RunMemoryEntryMessage, "user", holdMessage(t, holdText("user", "COMPACTED-AWAY")), "")
	holdAppend(t, s, prior, types.RunMemoryEntryCompaction, "", nil, "CHECKPOINT: the owner's newest direction governs; keep PdfApp.")
	orphan := map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "call-compacted", "content": "ORPHAN-RESULT"}}}
	holdAppend(t, s, prior, types.RunMemoryEntryMessage, "user", holdMessage(t, orphan), "")
	holdAppend(t, s, prior, types.RunMemoryEntryMessage, "user", holdMessage(t, holdText("user", "AFTER-CHECKPOINT")), "")
	current := holdRun(t, s, "texture-hold-ckpt-current", holdAgent, now.Add(time.Second), map[string]any{
		runMetadataCarryActorConversation: true,
		runMetadataActorRewarmSource:      prior.RunID,
	})
	current.State = types.RunPending
	messages, entries := initializeHoldRun(t, s, current)
	if entries[0].Kind != types.RunMemoryEntryCompaction || entries[0].Summary != "CHECKPOINT: the owner's newest direction governs; keep PdfApp." {
		t.Fatalf("the checkpoint was not carried as the first entry: %+v", entries[0])
	}
	all := joinedMessages(messages)
	if strings.Contains(all, "COMPACTED-AWAY") || strings.Contains(all, "ORPHAN-RESULT") || !strings.Contains(all, "AFTER-CHECKPOINT") {
		t.Fatalf("the carry re-expanded the checkpoint or lost its tail:\n%s", all)
	}
}
