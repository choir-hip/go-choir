package textureowner

import (
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/prompt-bar-minesweeper-demo-2026-10-10.md (F1): on a fresh
// computer the persistent management agent has never been registered, so a
// Texture turn that opens management failed "load exact target: record not
// found" and Texture could never delegate. Failure modes pinned:
//   - the opener refuses when the management agent is absent;
//   - the control targets anything but management:<owner>;
//   - the agent is not left registered for the delivery that follows.
func TestTextureOpensManagementOnAFreshComputer(t *testing.T) {
	core, handler := testAPISetup(t)
	installSynchronousTextureOwnerWake(t, core, handler)
	start := startObservationLifecycle(t, core.Store())
	first := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "fresh-owner-1", "build it", start.InitialRevision.RevisionID)
	if first.Code != http.StatusAccepted {
		t.Fatalf("revise status=%d body=%s", first.Code, first.Body.String())
	}
	management := agentprofile.Management + ":" + start.OwnerID
	if _, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, management); err == nil {
		t.Fatal("fixture: management agent already registered; the test needs a fresh computer")
	}
	agent, _ := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	snapshot, _ := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)

	args := editTextureArgs{ToolCallID: "open-management", WorkDisposition: string(types.WorkItemOpen), Controls: []textureControlArgs{{
		OpenPersistentManagement: true, Objective: "build and verify the game",
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "build and verify the game",
			Actions: []types.CoagentPacketAction{{Type: "run_tests", Objective: "verify the game headlessly",
				Safety: types.CoagentPacketActionSafety{MutationClass: "yellow", Network: "forbidden", FileMutation: "allowed"}}},
		},
	}}}
	controls, _, err := handler.textureTurnControls(t.Context(), &run, doc, snapshot, args)
	if err != nil {
		t.Fatalf("Texture could not open management on a fresh computer: %v", err)
	}
	if len(controls) != 1 || controls[0].TargetAgentID != management {
		t.Fatalf("controls = %+v, want one control to %s", controls, management)
	}
	if _, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, management); err != nil {
		t.Fatalf("management agent not registered after the opener: %v", err)
	}
}
