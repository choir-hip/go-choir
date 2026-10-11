package textureowner

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// PD1 (docs/definitions/choir-pd1-persistent-research-desk-2026-10-11.md):
// one persistent research desk per document, reused by every Texture turn.
// Failure modes pinned: each opener mints a new research agent; a second
// opener in the same request opens a second desk (or is dropped) instead of
// following up on the request's research work; two openers in one turn open
// two desks; a later request on the document mints another agent instead of
// opening work on the same desk; research rounds per request are unbounded
// (texture-research-loop-never-idles-2026-10-09); replaying a committed turn
// counts its own controls as spent.
func researchOpenerArgs(toolCallID string) editTextureArgs {
	return editTextureArgs{ToolCallID: toolCallID, WorkDisposition: string(types.WorkItemOpen), Controls: []textureControlArgs{{
		OpenResearch: true, Objective: "more evidence",
		Packet: types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "question", Summary: "more evidence", Questions: []string{"What else?"}},
	}}}
}

func TestTextureUsesOnePersistentResearchDeskPerDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	installSynchronousTextureOwnerWake(t, core, handler)
	start := startObservationLifecycle(t, core.Store())
	resp := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "desk-owner-1", "research it", start.InitialRevision.RevisionID)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("revise status=%d body=%s", resp.Code, resp.Body.String())
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
		t.Fatal("fixture: no owner-revise head event")
	}
	desk := agentprofile.Research + ":" + doc.DocID

	// First opener on the document: creates the desk and its first work.
	first, note, err := handler.textureTurnControls(t.Context(), &run, doc, snapshot, researchOpenerArgs("first"))
	if err != nil || note != "" || len(first) != 1 {
		t.Fatalf("first opener: controls=%d note=%q err=%v", len(first), note, err)
	}
	if first[0].TargetAgentID != desk || first[0].OpenAgent == nil || first[0].OpenAgent.AgentID != desk || first[0].OpenWork == nil {
		t.Fatalf("first opener did not open the document's desk %s: %+v", desk, first[0])
	}

	// Two openers in one turn: one control.
	two := researchOpenerArgs("two-in-one")
	two.Controls = append(two.Controls, textureControlArgs{OpenResearch: true, Objective: "second branch",
		Packet: types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "question", Summary: "second branch", Questions: []string{"What about B?"}}})
	merged, _, err := handler.textureTurnControls(t.Context(), &run, doc, snapshot, two)
	if err != nil || len(merged) != 1 || !strings.Contains(strings.Join(merged[0].Packet.Questions, "|"), "What about B?") {
		t.Fatalf("two openers in one turn did not become one control carrying both: %+v err=%v", merged, err)
	}

	if err := core.Store().UpsertAgent(t.Context(), types.AgentRecord{AgentID: desk, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		Profile: agentprofile.Research, Role: agentprofile.Research, ChannelID: doc.DocID, CreatedAt: ownerInput, UpdatedAt: ownerInput}); err != nil {
		t.Fatal(err)
	}

	// The desk exists and has open work from this request: a second opener
	// follows up on that work.
	work := types.WorkItemRecord{WorkItemID: "w-desk", AuthorityProfile: agentprofile.Research, Status: types.WorkItemOpen,
		AssignedAgentID: desk, CreatedAt: ownerInput.Add(time.Second)}
	busy := snapshot
	busy.WorkItems = append(append([]types.WorkItemRecord(nil), snapshot.WorkItems...), work)
	busy.Events = append(append([]types.LifecycleEvent(nil), snapshot.Events...),
		types.LifecycleEvent{Kind: types.LifecycleControlQueued, WorkItemID: work.WorkItemID, UpdateID: "c-1", CreatedAt: ownerInput.Add(time.Second)})
	follow, note, err := handler.textureTurnControls(t.Context(), &run, doc, busy, researchOpenerArgs("second"))
	if err != nil || note != "" || len(follow) != 1 {
		t.Fatalf("second opener: controls=%d note=%q err=%v", len(follow), note, err)
	}
	if follow[0].TargetWorkItemID != work.WorkItemID || follow[0].TargetAgentID != desk || follow[0].OpenAgent != nil || follow[0].OpenWork != nil {
		t.Fatalf("second opener did not follow up on the request's research work: %+v", follow[0])
	}

	// A later owner request on the document: new work on the same desk.
	later := busy
	later.Events = append(append([]types.LifecycleEvent(nil), busy.Events...),
		types.LifecycleEvent{Kind: types.LifecycleArtifactHeadAdvanced, CommandID: "owner-revise:next", CreatedAt: ownerInput.Add(time.Minute)})
	next, _, err := handler.textureTurnControls(t.Context(), &run, doc, later, researchOpenerArgs("next-request"))
	if err != nil || len(next) != 1 || next[0].TargetAgentID != desk || next[0].OpenAgent != nil || next[0].OpenWork == nil {
		t.Fatalf("a later request did not open new work on the same desk: %+v err=%v", next, err)
	}

	// Rounds per request are bounded; replay does not count its own control.
	spent := busy
	for i := 2; i <= textureResearchRoundsPerOwnerRequest; i++ {
		spent.Events = append(spent.Events, types.LifecycleEvent{Kind: types.LifecycleControlQueued, WorkItemID: work.WorkItemID,
			UpdateID: "c-" + string(rune('0'+i)), CreatedAt: ownerInput.Add(time.Duration(i) * time.Second)})
	}
	dropped, note, err := handler.textureTurnControls(t.Context(), &run, doc, spent, researchOpenerArgs("over"))
	if err != nil || len(dropped) != 0 || !strings.Contains(note, "research rounds") {
		t.Fatalf("a research round past the bound was not dropped with a reason: %+v note=%q err=%v", dropped, note, err)
	}
	replayed := busy
	replayed.Events = append(append([]types.LifecycleEvent(nil), busy.Events...),
		types.LifecycleEvent{Kind: types.LifecycleControlQueued, WorkItemID: work.WorkItemID, UpdateID: follow[0].ControlID, CreatedAt: ownerInput.Add(2 * time.Second)})
	again, note, err := handler.textureTurnControls(t.Context(), &run, doc, replayed, researchOpenerArgs("second"))
	if err != nil || note != "" || len(again) != 1 || again[0].ControlID != follow[0].ControlID || again[0].TargetWorkItemID != work.WorkItemID {
		t.Fatalf("replay of a committed follow-up changed its controls: %+v note=%q err=%v", again, note, err)
	}
}

// P2a (docs/definitions/choir-protocol-p2-one-surface-2026-10-11.md): a
// Texture Cast to management folds to the persistent-management opener. When
// the request already has open management work on this trajectory, the
// opener continues it, as a research opener continues research work, because
// target_work_item_id is no longer on Texture's surface.
func TestTextureManagementCastContinuesTheRequestsOpenWork(t *testing.T) {
	core, handler := testAPISetup(t)
	installSynchronousTextureOwnerWake(t, core, handler)
	start := startObservationLifecycle(t, core.Store())
	resp := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "mgmt-owner-1", "build it", start.InitialRevision.RevisionID)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("revise status=%d body=%s", resp.Code, resp.Body.String())
	}
	agent, _ := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	snapshot, _ := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	ownerInput := latestOwnerInput(snapshot)
	if ownerInput.IsZero() {
		t.Fatal("fixture: no owner input")
	}
	cast := func(id string) editTextureArgs {
		return editTextureArgs{ToolCallID: id, WorkDisposition: string(types.WorkItemOpen), Controls: []textureControlArgs{{
			OpenPersistentManagement: true, Objective: "build the page", StagedAct: "cast",
			Packet: types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request",
				Summary: "build the page", Actions: []types.CoagentPacketAction{{Type: "run_tests", Objective: "verify the page",
				Safety: types.CoagentPacketActionSafety{MutationClass: "yellow", Network: "forbidden", FileMutation: "allowed"}}}},
		}}}
	}
	mgmt := agentprofile.Management + ":" + start.OwnerID

	opened, _, err := handler.textureTurnControls(t.Context(), &run, doc, snapshot, cast("open"))
	if err != nil || len(opened) != 1 || opened[0].TargetAgentID != mgmt || opened[0].OpenWork == nil || opened[0].StagedAct != "cast" {
		t.Fatalf("first management Cast did not open management work: %+v err=%v", opened, err)
	}

	work := types.WorkItemRecord{WorkItemID: "w-mgmt", TrajectoryID: doc.TrajectoryID, AuthorityProfile: agentprofile.Management,
		Status: types.WorkItemOpen, AssignedAgentID: mgmt, CreatedAt: ownerInput.Add(time.Second)}
	busy := snapshot
	busy.WorkItems = append(append([]types.WorkItemRecord(nil), snapshot.WorkItems...), work)
	follow, _, err := handler.textureTurnControls(t.Context(), &run, doc, busy, cast("follow"))
	if err != nil || len(follow) != 1 || follow[0].TargetWorkItemID != work.WorkItemID || follow[0].OpenWork != nil {
		t.Fatalf("a second management Cast in the request did not continue its open work: %+v err=%v", follow, err)
	}

	// A replay of the opening turn sees the work it opened and rebuilds the
	// same opener (same digest), rather than continuing its own work.
	own := snapshot
	ownWork := *opened[0].OpenWork
	ownWork.TrajectoryID, ownWork.CreatedAt = doc.TrajectoryID, ownerInput.Add(time.Second)
	own.WorkItems = append(append([]types.WorkItemRecord(nil), snapshot.WorkItems...), ownWork)
	replayed, _, err := handler.textureTurnControls(t.Context(), &run, doc, own, cast("open"))
	if err != nil || len(replayed) != 1 || replayed[0].OpenWork == nil || replayed[0].TargetWorkItemID != opened[0].TargetWorkItemID {
		t.Fatalf("replaying the opening turn continued its own work instead of rebuilding the opener: %+v err=%v", replayed, err)
	}

	// Management work from an earlier request is not continued.
	stale := snapshot
	old := work
	old.CreatedAt = ownerInput.Add(-time.Minute)
	stale.WorkItems = append(append([]types.WorkItemRecord(nil), snapshot.WorkItems...), old)
	fresh, _, err := handler.textureTurnControls(t.Context(), &run, doc, stale, cast("fresh"))
	if err != nil || len(fresh) != 1 || fresh[0].OpenWork == nil {
		t.Fatalf("management work from an earlier request was continued: %+v err=%v", fresh, err)
	}
}
