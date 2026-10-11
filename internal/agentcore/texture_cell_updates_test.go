package agentcore

import (
	"testing"

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
