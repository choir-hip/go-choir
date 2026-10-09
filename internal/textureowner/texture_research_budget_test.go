package textureowner

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/texture-research-loop-never-idles-2026-10-09.md.
// Failure modes: Texture opens research on every turn and each report wakes
// another turn, so an owner request never ends (a third opener must be
// dropped while the turn still commits); a later owner request must get a
// fresh budget; replaying a committed turn must not see its own openers as
// spent (the replay digest would change).

func researchOpenerArgs(toolCallID string) editTextureArgs {
	return editTextureArgs{ToolCallID: toolCallID, WorkDisposition: string(types.WorkItemOpen), Controls: []textureControlArgs{{
		OpenResearch: true, Objective: "more evidence",
		Packet: types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "question", Summary: "more evidence", Questions: []string{"What else?"}},
	}}}
}

func TestTextureResearchBudgetPerOwnerRequest(t *testing.T) {
	core, handler := testAPISetup(t)
	installSynchronousTextureOwnerWake(t, core, handler)
	start := startObservationLifecycle(t, core.Store())
	first := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "budget-owner-1", "research it", start.InitialRevision.RevisionID)
	if first.Code != http.StatusAccepted {
		t.Fatalf("revise status=%d body=%s", first.Code, first.Body.String())
	}
	agent, _ := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	snapshot, _ := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)

	var ownerInput time.Time
	for _, ev := range snapshot.Events {
		if ev.Kind == types.LifecycleArtifactHeadAdvanced && strings.HasPrefix(ev.CommandID, "owner-revise:") {
			ownerInput = ev.CreatedAt
		}
	}
	if ownerInput.IsZero() {
		t.Fatal("fixture: no owner-revise head event in the snapshot")
	}
	spent := snapshot
	spent.WorkItems = append(append([]types.WorkItemRecord(nil), snapshot.WorkItems...),
		types.WorkItemRecord{WorkItemID: "r1", AuthorityProfile: agentprofile.Research, Status: types.WorkItemOpen, CreatedAt: ownerInput.Add(time.Second)},
		types.WorkItemRecord{WorkItemID: "r2", AuthorityProfile: agentprofile.Research, Status: types.WorkItemOpen, CreatedAt: ownerInput.Add(2 * time.Second)},
		// Opened before this owner request: does not count.
		types.WorkItemRecord{WorkItemID: "r0", AuthorityProfile: agentprofile.Research, Status: types.WorkItemOpen, CreatedAt: ownerInput.Add(-time.Minute)},
	)

	controls, note, err := handler.textureTurnControls(t.Context(), &run, doc, spent, researchOpenerArgs("third-opener"))
	if err != nil {
		t.Fatalf("a spent budget must not fail the turn: %v", err)
	}
	if len(controls) != 0 || !strings.Contains(note, "research budget") {
		t.Fatalf("third research opener not dropped with a recorded reason: controls=%+v note=%q", controls, note)
	}

	// Under budget (only r0, before the owner request, plus one since): allowed.
	under := snapshot
	under.WorkItems = append(append([]types.WorkItemRecord(nil), snapshot.WorkItems...), spent.WorkItems[len(spent.WorkItems)-1], spent.WorkItems[len(spent.WorkItems)-3])
	controls, note, err = handler.textureTurnControls(t.Context(), &run, doc, under, researchOpenerArgs("second-opener"))
	if err != nil || len(controls) != 1 || note != "" {
		t.Fatalf("second research opener refused: controls=%d note=%q err=%v", len(controls), note, err)
	}

	// Replay: the turn's own deterministic work item is not counted.
	ownID := controls[0].TargetWorkItemID
	replayed := under
	replayed.WorkItems = append(append([]types.WorkItemRecord(nil), under.WorkItems...),
		types.WorkItemRecord{WorkItemID: ownID, AuthorityProfile: agentprofile.Research, Status: types.WorkItemOpen, CreatedAt: ownerInput.Add(3 * time.Second)})
	controls, note, err = handler.textureTurnControls(t.Context(), &run, doc, replayed, researchOpenerArgs("second-opener"))
	if err != nil || len(controls) != 1 || note != "" {
		t.Fatalf("replay of a committed opener saw its own work as spent: controls=%d note=%q err=%v", len(controls), note, err)
	}

	// A new owner request resets the budget.
	reset := spent
	reset.Events = append(append([]types.LifecycleEvent(nil), spent.Events...),
		types.LifecycleEvent{Kind: types.LifecycleArtifactHeadAdvanced, CommandID: "owner-revise:next", CreatedAt: ownerInput.Add(time.Minute)})
	controls, note, err = handler.textureTurnControls(t.Context(), &run, doc, reset, researchOpenerArgs("after-new-request"))
	if err != nil || len(controls) != 1 || note != "" {
		t.Fatalf("a new owner request did not reset the budget: controls=%d note=%q err=%v", len(controls), note, err)
	}
}
