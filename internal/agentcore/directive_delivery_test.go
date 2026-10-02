package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestPersistentManagementDirectiveAdmissionAndUnboundInjection(t *testing.T) {
	update := types.CoagentSourcePacket{
		UpdateID: "directive:1:packet", OwnerID: "owner", ComputerID: "computer",
		AgentID: "research:desk", TargetAgentID: "management:owner",
		SourceRecordID: "directive:1", Direction: types.LifecyclePacketDirectionDirective,
		Disposition: types.UpdatePending, LifecycleVersion: 1,
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "directive", Summary: "note",
			Notes: []string{"directive:note"},
		},
		Content: "read commitment record directive:1",
	}
	rec := &types.RunRecord{
		RunID: "management-run", OwnerID: "owner", ComputerID: "computer",
		AgentID: "management:owner", AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management,
		Metadata: map[string]any{"request_source": "update_coagent"},
	}
	if !persistentManagementAdmissibleDirective(update) {
		t.Fatal("record-native directive was not admitted")
	}
	if !coagentUpdateDeliverableForRun(rec, update) {
		t.Fatal("unbound directive was not injectable to persistent Management")
	}

	missingRecord := update
	missingRecord.SourceRecordID = ""
	if persistentManagementAdmissibleDirective(missingRecord) {
		t.Fatal("directive without commitment record was admitted")
	}
	wrongDirection := update
	wrongDirection.Direction = types.LifecyclePacketDirectionControl
	if persistentManagementAdmissibleDirective(wrongDirection) {
		t.Fatal("non-directive packet was admitted as a directive")
	}
}
