package textureowner

import (
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// (a) Texture: a turn whose cells were shown a window of the pending reports
// delivers only those. Without the restriction the consume-at-commit default
// delivered every pending report, including the ones past the 4 MiB frame
// that no cell had read.
func TestTextureTurnDeliversOnlyTheReportsItsCellsWereShown(t *testing.T) {
	rec := &types.RunRecord{RunID: "run-t", AgentID: "texture-desk", Metadata: map[string]any{"scheduled_message_seq": 100}}
	snapshot := types.LifecycleSnapshot{}
	for i := 0; i < 40; i++ {
		snapshot.Updates = append(snapshot.Updates, types.CoagentSourcePacket{
			UpdateID: fmt.Sprintf("u%03d", i), Disposition: types.UpdatePending,
			Direction: types.LifecyclePacketDirectionProducerReport, TargetAgentID: "texture-desk",
			MessageSeq: int64(i + 1), ProducerWorkItemID: "work-1",
		})
	}
	ids := func(in []types.TextureTurnInboundDisposition) map[string]bool {
		out := map[string]bool{}
		for _, d := range in {
			out[d.UpdateID] = true
		}
		return out
	}

	// Unrestricted (every report fit the frame): the default is unchanged.
	all, err := textureTurnPendingInbound(snapshot, rec, nil, "", nil, false)
	if err != nil || len(all) != 40 {
		t.Fatalf("unrestricted: %d delivered, %v", len(all), err)
	}

	// The cells were shown u000..u009: only those are delivered.
	shown := map[string]bool{}
	for i := 0; i < 10; i++ {
		shown[fmt.Sprintf("u%03d", i)] = true
	}
	first, err := textureTurnPendingInbound(snapshot, rec, nil, "", shown, true)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(first)
	if len(got) != 10 {
		t.Fatalf("delivered %d reports, want the 10 shown", len(got))
	}
	for id := range shown {
		if !got[id] {
			t.Fatalf("shown report %s was not delivered", id)
		}
	}
	if got["u010"] || got["u039"] {
		t.Fatalf("a report past the window was delivered unread: %v", got)
	}

	// The commit disposed the window; the rest stay pending and the next
	// turn's window starts at the oldest of them.
	for i := range snapshot.Updates {
		if got[snapshot.Updates[i].UpdateID] {
			snapshot.Updates[i].Disposition = types.UpdateDelivered
		}
	}
	nextShown := map[string]bool{}
	for i := 10; i < 20; i++ {
		nextShown[fmt.Sprintf("u%03d", i)] = true
	}
	second, err := textureTurnPendingInbound(snapshot, rec, nil, "", nextShown, true)
	if err != nil || len(ids(second)) != 10 || !ids(second)["u010"] || ids(second)["u020"] {
		t.Fatalf("the next turn delivered %v, %v", ids(second), err)
	}
}
