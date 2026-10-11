package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A Texture cell's choir.Updates() shows exactly the reports its turn can
// disposition. S1c (docs/problems/texture-probes-canonical-write-to-learn-schema-2026-10-11.md):
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

// A Texture cell's choir.ReadDoc() carries the head's outline: the block and
// node ids its edits address. Without it, a reactivated Texture run whose
// prompt outline described an older revision guessed ids by staging writes
// (docs/problems/texture-probes-canonical-write-to-learn-schema-2026-10-11.md,
// amendment).
func TestTextureCellDocSnapshotCarriesTheOutline(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{DocID: "doc-outline", OwnerID: "owner-1", Title: "AI news", CreatedAt: now, UpdatedAt: now}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s.CreateRevision(ctx, types.Revision{
		RevisionID: "rev-2", DocID: doc.DocID, OwnerID: doc.OwnerID, AuthorKind: types.AuthorAppAgent,
		Content: "Chips news", BodyDoc: runtimeTestTextureBodyDoc(t, doc.DocID, "rev-2", "Chips news"),
		Citations: json.RawMessage("[]"), CreatedAt: now,
	}); err != nil {
		t.Fatalf("create revision: %v", err)
	}
	rec := &types.RunRecord{RunID: "run-t", AgentID: "texture:" + doc.DocID, OwnerID: doc.OwnerID, ChannelID: doc.DocID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture}
	snap := textureDocSnapshotForRun(ctx, rt, rec)
	if snap == nil {
		t.Fatalf("no snapshot for the bound document")
	}
	if want := `paragraph id=p-doc-outline-rev-2 text="Chips news"`; !strings.Contains(snap.Outline, want) {
		t.Fatalf("snapshot outline missing %q: %q", want, snap.Outline)
	}
}
