package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// A Texture cell's choir.Updates() shows exactly the reports its turn can
// disposition. S1c (docs/archive/problems/texture-probes-canonical-write-to-learn-schema-2026-10-11.md):
// the frame showed a report that arrived after the activation was
// scheduled, the cell dispositioned it, and the commit refused the turn.
func TestTextureCellSeesOnlyUpdatesItsTurnCanDisposition(t *testing.T) {
	rec := &types.RunRecord{
		RunID: "run-1", AgentID: "texture:doc-1", AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		Metadata: map[string]any{"scheduled_message_seq": 6},
	}
	report := func(id string, seq int64, boundTo string) types.CoagentSourcePacket {
		return types.CoagentSourcePacket{
			UpdateID: id, MessageSeq: seq, DeliveredToRunID: boundTo,
			Direction: types.LifecyclePacketDirectionProducerReport, TargetAgentID: "texture:doc-1",
			Disposition: types.UpdatePending,
		}
	}
	cases := []struct {
		update types.CoagentSourcePacket
		shown  bool
	}{
		{report("at-scheduled", 6, ""), true},
		{report("bound-here", 4, "run-1"), true},
		{report("after-scheduled", 8, ""), false},
		{report("bound-elsewhere", 5, "run-0"), false},
	}
	for _, c := range cases {
		if got := cellUpdateVisible(rec, c.update); got != c.shown {
			t.Errorf("%s: shown=%v, want %v", c.update.UpdateID, got, c.shown)
		}
		if got := c.update.WithinTextureTurnBoundary(rec.RunID, 6); got != c.shown {
			t.Errorf("%s: the commit's rule says %v, the frame must agree", c.update.UpdateID, got)
		}
	}

	research := &types.RunRecord{RunID: "run-2", AgentID: "research:doc-1", AgentProfile: agentprofile.Research,
		Metadata: map[string]any{"scheduled_message_seq": 6}}
	late := report("late", 8, "")
	late.TargetAgentID = "research:doc-1"
	if !cellUpdateVisible(research, late) {
		t.Fatalf("the Texture consume boundary must not hide reports from other desks")
	}
}

// A Texture cell's choir.ReadDoc() carries the blocks its frame showed, by
// alias (Cut 2 2a). Ways it can fail: the snapshot is the head while the
// frame showed an older revision, so b2 names a different block than the
// model read; the cell marks read records its frame never showed; the frame
// is injected again when nothing changed.
func TestTextureCellWorksAgainstItsFrame(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{DocID: "doc-frame", OwnerID: "owner-1", Title: "AI news", CreatedAt: now, UpdatedAt: now}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	for _, r := range []types.Revision{
		{RevisionID: "rev-1", Content: "# AI news\n\nChips news"},
		{RevisionID: "rev-2", Content: "# AI news\n\nModels news\n\nChips news", ParentRevisionID: "rev-1"},
	} {
		r.DocID, r.OwnerID, r.AuthorKind, r.Citations, r.CreatedAt = doc.DocID, doc.OwnerID, types.AuthorUser, json.RawMessage("[]"), now
		if err := s.CreateRevision(ctx, r); err != nil {
			t.Fatalf("create revision: %v", err)
		}
	}
	rec := &types.RunRecord{RunID: "run-frame", AgentID: "texture:" + doc.DocID, OwnerID: doc.OwnerID, ChannelID: doc.DocID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, Metadata: map[string]any{"doc_id": doc.DocID}}
	if err := s.CreateRun(ctx, *rec); err != nil {
		t.Fatalf("create run: %v", err)
	}
	inject := rt.textureFrameInjector(rec)
	turns, err := inject(false)
	if err != nil || len(turns) != 1 {
		t.Fatalf("first frame = %d turns, %v", len(turns), err)
	}
	text := runMemoryUserMessageTexts(turns[0])[0]
	if !strings.Contains(text, "rev=rev-2") || !strings.Contains(text, "b2 Models news") || !strings.Contains(text, "b3 Chips news") {
		t.Fatalf("frame does not show the head by alias:\n%s", text)
	}
	if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{RunID: rec.RunID, OwnerID: rec.OwnerID, Kind: types.RunMemoryEntryMessage, Role: types.RunMemoryRoleRuntimeInjection, Message: turns[0]}); err != nil {
		t.Fatalf("persist frame: %v", err)
	}
	if again, _ := inject(false); len(again) != 0 {
		t.Fatalf("an unchanged frame was injected again")
	}
	// A model cannot forge a frame: only runtime injections count.
	if mark := textureFrameMarkFromMemory([]types.RunMemoryEntry{{Kind: types.RunMemoryEntryMessage, Role: "user", Message: turns[0]}}); mark.ok {
		t.Fatalf("a user message counted as a frame")
	}

	// The head moves (an owner edit) after the frame: the cell still works
	// against the revision its frame showed.
	if err := s.CreateRevision(ctx, types.Revision{RevisionID: "rev-3", DocID: doc.DocID, OwnerID: doc.OwnerID, AuthorKind: types.AuthorUser,
		Content: "# AI news\n\nOwner line\n\nModels news\n\nChips news", ParentRevisionID: "rev-2", Citations: json.RawMessage("[]"), CreatedAt: now}); err != nil {
		t.Fatalf("create head: %v", err)
	}
	snap := textureDocSnapshotForRun(ctx, rt, rec, cellRecords{})
	if snap == nil || snap.RevisionID != "rev-2" || len(snap.Blocks) != 3 || snap.Blocks[1].Alias != "b2" || snap.Blocks[1].Text != "Models news" {
		t.Fatalf("snapshot = %+v, want rev-2's blocks", snap)
	}

	// Records past the frame's through stay unread.
	view := []yaegikernel.InboxRecord{{Seq: 1, RecordID: "a"}, {Seq: 2, RecordID: "b"}}
	capped := capToTextureFrame(cellRecords{inbox: "texture:doc-frame", after: 0, through: 2, view: view}, textureFrameMark{revisionID: "rev-2", through: 1, ok: true})
	if capped.through != 1 || len(capped.view) != 1 || capped.view[0].RecordID != "a" {
		t.Fatalf("capped = %+v, want only the record the frame showed", capped)
	}
}
