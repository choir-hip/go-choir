package agentcore

// A Texture wake carries the earlier conversation append-only
// (docs/problems/texture-wake-misses-warm-cache-and-bedrock-400-2026-11-09.md
// section 1). The provider caches by leading bytes, so messages 0..N-1 of the
// second wake's request must equal the first wake's request plus its reply,
// byte for byte, with the new run context, prompt and frame after them.
//
// Ways this can fail, written before the code, each checked in code:
//
//  A. Two run-context turns (old facts, new facts) confuse the model. The
//     new turn follows the old; a resumed activation already holds this
//     shape (initialize appends a fresh run-context turn after replayed
//     memory), and the newest turn carries the current time and ids.
//  B. Role alternation: the carried tail ends with the prior run's final
//     assistant reply and the new turns are user messages. Where the tail
//     ends on a user message (an interrupted run) the seam is two user
//     messages, as it was before this change (the old carry kept trailing
//     non-context user turns too). The test asserts the normal seam.
//  C. tool_use/tool_result pairing at the seam: carryableRunMemoryTail still
//     drops orphan results and cuts at an unanswered tool_use (unchanged;
//     see TestTextureHoldTrajectoryCarriesTheEarlierConversationVerbatim).
//     A complete pair is carried whole (asserted here).
//  D. The carried compaction entry is re-rendered: its TokensBefore was
//     re-estimated at every hop, changing message 0. It now copies the
//     source's TokensBefore (asserted: rendered message byte-equal).
//  E. Another desk takes the same path: carryTexturePriorConversation is
//     gated on the document custodian profile
//     (TestTextureHoldTrajectoryFlagIgnoredOffTexture).
//  F. Carried frames keep their message role, never runtime_injection
//     (unchanged; asserted in the verbatim test).

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureCarryAppendKeepsTheFirstRequestAsAByteEqualPrefix(t *testing.T) {
	_, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Wake 1: a fresh run, no carry. Its request is the context messages.
	first := holdRun(t, s, "texture-append-1", holdAgent, now, nil)
	first.State = types.RunPending
	mem1 := newRunMemoryManager(s, &first, provideriface.Config{}, nil)
	req1, err := mem1.initialize(ctx, []json.RawMessage{
		runContextMessage(now, "FACTS-WAKE-1"),
		holdMessage(t, holdText("user", "wake one prompt")),
		holdMessage(t, holdText("user", "frame one")),
	})
	if err != nil {
		t.Fatal(err)
	}
	call := map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "tool_use", "id": "call-1", "name": "desk_go_eval", "input": map[string]any{"code": "x"}}}}
	res := map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "call-1", "content": "ok"}}}
	for _, m := range []any{call, res, holdText("assistant", "done wake one")} {
		raw := holdMessage(t, m)
		if err := mem1.appendMessage(ctx, runMemoryMessageRole(raw), raw); err != nil {
			t.Fatal(err)
		}
	}
	entries1, err := s.ListRunMemoryEntries(ctx, first.OwnerID, first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	sent1 := buildRunMemoryContext(entries1) // request 1 plus its assistant reply
	if len(sent1) != len(req1)+3 {
		t.Fatalf("setup: %d messages, want %d", len(sent1), len(req1)+3)
	}

	// Wake 2 carries it.
	second := holdRun(t, s, "texture-append-2", holdAgent, now.Add(10*time.Minute), map[string]any{
		runMetadataCarryActorConversation: true,
		runMetadataActorRewarmSource:      first.RunID,
	})
	second.State = types.RunPending
	req2, _ := initializeHoldRun(t, s, second)
	if len(req2) <= len(sent1) {
		t.Fatalf("second request has %d messages, first had %d", len(req2), len(sent1))
	}
	for i := range sent1 {
		if !bytes.Equal(req2[i], sent1[i]) {
			t.Fatalf("message %d changed:\nfirst:  %s\nsecond: %s", i, sent1[i], req2[i])
		}
	}
	if !strings.Contains(string(req2[0]), "FACTS-WAKE-1") {
		t.Fatalf("message 0 is not wake one's run context: %s", req2[0])
	}
	if runMemoryMessageRole(req2[len(sent1)-1]) != "assistant" || runMemoryMessageRole(req2[len(sent1)]) != "user" {
		t.Fatalf("seam breaks role alternation")
	}
	if !strings.Contains(string(req2[len(sent1)]), "NEW-RUN-FACTS") {
		t.Fatalf("new run context does not follow the carried conversation: %s", req2[len(sent1)])
	}
}

// Failure mode D: the carried checkpoint renders as the source rendered it.
func TestTextureCarryAppendCopiesTheCheckpointRenderingVerbatim(t *testing.T) {
	_, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	prior := holdRun(t, s, "texture-append-ckpt-1", holdAgent, now, nil)
	holdAppend(t, s, prior, types.RunMemoryEntryMessage, "user", runContextMessage(now, "OLD"), "")
	holdAppend(t, s, prior, types.RunMemoryEntryMessage, "user", holdMessage(t, holdText("user", "compacted away")), "")
	if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID: prior.RunID, OwnerID: prior.OwnerID, AgentID: prior.AgentID,
		Kind: types.RunMemoryEntryCompaction, Summary: "CHECKPOINT", TokensBefore: 123456,
	}); err != nil {
		t.Fatal(err)
	}
	holdAppend(t, s, prior, types.RunMemoryEntryMessage, "assistant", holdMessage(t, holdText("assistant", "after checkpoint")), "")
	pe, err := s.ListRunMemoryEntries(ctx, prior.OwnerID, prior.RunID)
	if err != nil {
		t.Fatal(err)
	}
	sent := buildRunMemoryContext(pe)

	cur := holdRun(t, s, "texture-append-ckpt-2", holdAgent, now.Add(time.Minute), map[string]any{
		runMetadataCarryActorConversation: true,
		runMetadataActorRewarmSource:      prior.RunID,
	})
	cur.State = types.RunPending
	req, _ := initializeHoldRun(t, s, cur)
	if len(req) <= len(sent) {
		t.Fatalf("carried request too short: %d vs %d", len(req), len(sent))
	}
	for i := range sent {
		if !bytes.Equal(req[i], sent[i]) {
			t.Fatalf("message %d changed:\nfirst:  %s\nsecond: %s", i, sent[i], req[i])
		}
	}
}
