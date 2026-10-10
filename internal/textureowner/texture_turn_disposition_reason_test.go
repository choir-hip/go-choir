package textureowner

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// M11 rerun 10 (07:20:38 and 07:38:13Z): Texture named a producer report
// it could see and the reducer refused it as "not eligible to this
// activation" without saying why, so neither the desk nor the trace
// could tell which eligibility rule failed. Each ineligible case must name
// its reason.
func TestTextureDispositionRefusalNamesReason(t *testing.T) {
	rec := &types.RunRecord{RunID: "run-texture-b", AgentID: "texture-desk", Metadata: map[string]any{"scheduled_message_seq": 10}}
	report := func(mutate func(*types.CoagentSourcePacket)) types.CoagentSourcePacket {
		u := types.CoagentSourcePacket{
			UpdateID:           "assignment-report:report:sha256:abc",
			Disposition:        types.UpdatePending,
			Direction:          types.LifecyclePacketDirectionProducerReport,
			TargetAgentID:      "texture-desk",
			MessageSeq:         5,
			ProducerWorkItemID: "work-1",
		}
		mutate(&u)
		return u
	}
	cases := []struct {
		name   string
		update types.CoagentSourcePacket
		want   string
	}{
		{"unknown id", report(func(u *types.CoagentSourcePacket) { u.UpdateID = "other" }), "no update with this id"},
		{"already disposed", report(func(u *types.CoagentSourcePacket) { u.Disposition = types.UpdateDisposition("delivered") }), `disposition "delivered"`},
		{"other target", report(func(u *types.CoagentSourcePacket) { u.TargetAgentID = "management-desk" }), `targets "management-desk"`},
		{"after activation", report(func(u *types.CoagentSourcePacket) { u.MessageSeq = 11 }), "message seq 11 is after this activation's scheduled seq 10"},
		{"bound elsewhere", report(func(u *types.CoagentSourcePacket) { u.DeliveredToRunID = "run-texture-a" }), `bound to run "run-texture-a"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := types.LifecycleSnapshot{Updates: []types.CoagentSourcePacket{tc.update}}
			_, err := textureTurnPendingInbound(snapshot, rec, []textureUpdateDisposition{{UpdateID: "assignment-report:report:sha256:abc", Disposition: "delivered"}}, "")
			if err == nil {
				t.Fatal("expected a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("refusal %q does not name reason %q", err, tc.want)
			}
		})
	}
}
