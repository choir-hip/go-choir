package textureowner

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Rerun 13 (docs/problems/m11-rerun-12-restore-refused-and-texture-reports-no-change-2026-10-10.md):
// Texture decided "wait_for_evidence" on an engineering report. The decision
// disposed the report, but the postcondition required this run's own revision
// to be the document head, so the activation deferred and its 29 buffered
// wakes were discarded. Failure modes pinned:
//   - a report disposed by a decision (no revision) reads as pending;
//   - a still-pending report reads as handled;
//   - a disposal that did not advance the report's identity is accepted;
//   - a report from another scope is accepted.
func TestProducerReportOccurrenceSettledByAnyDisposition(t *testing.T) {
	o := agentcore.TextureActorOccurrence{
		Version: agentcore.TextureActorOccurrenceVersion, Kind: agentcore.TextureActorOccurrenceProducerReport,
		OwnerID: "owner", ComputerID: "computer", TrajectoryID: "trajectory", DocumentID: "doc",
		TargetAgentID: "texture:doc", TargetWorkItemID: "work-texture", ProducerAgentID: "engineering:a",
		UpdateID: "report-1", ProducerUpdateID: "producer-1", ProducerWorkID: "work-a", MessageSeq: 8,
		LifecycleVersion: 1, ReducerSeq: 9,
	}
	report := func(disposition types.UpdateDisposition, version, seq int64) types.CoagentSourcePacket {
		return types.CoagentSourcePacket{
			OwnerID: "owner", ComputerID: "computer", TrajectoryID: "trajectory", ChannelID: "doc",
			TargetAgentID: "texture:doc", TargetWorkItemID: "work-texture", AgentID: "engineering:a",
			UpdateID: "report-1", ProducerUpdateID: "producer-1", ProducerWorkItemID: "work-a", MessageSeq: 8,
			Disposition: disposition, DispositionRef: "base-revision", LifecycleVersion: version, ReducerSeq: seq,
		}
	}
	if state, err := producerReportOccurrenceState(o, report(types.UpdatePending, 1, 9)); err != nil || state != TextureActorOccurrencePending {
		t.Fatalf("pending report = %v, %v; want pending", state, err)
	}
	if state, err := producerReportOccurrenceState(o, report(types.UpdateDelivered, 2, 12)); err != nil || state != TextureActorOccurrenceTerminal {
		t.Fatalf("report disposed by a decision = %v, %v; want terminal", state, err)
	}
	if _, err := producerReportOccurrenceState(o, report(types.UpdateDelivered, 1, 9)); err == nil {
		t.Fatal("a disposal that did not advance the report was accepted")
	}
	foreign := report(types.UpdateDelivered, 2, 12)
	foreign.TrajectoryID = "other"
	if _, err := producerReportOccurrenceState(o, foreign); err == nil {
		t.Fatal("a report from another trajectory was accepted")
	}
}
