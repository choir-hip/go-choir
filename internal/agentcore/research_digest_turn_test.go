package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The whole path, with the object-graph gates in ENFORCE as the product ships them: a
// real Texture turn on a story (a lifecycle trajectory that carries a main point) commits
// a Cast to its research desk; the research run that Cast wakes is built its digest from
// the conversation records alone, and the digest holds the trajectory's main point and
// the text of the head the turn committed. The deployed context lost its main point here
// (docs/problems/story-research-context-main-point-empty-under-enforce-2026-11-22.md).
func TestARealTextureTurnsCastGivesTheResearchRunItsMainPointAndTextUnderEnforce(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-turn", rt.cfg.ComputerID
	docID, trajectoryID := "doc-turn-1", "trajectory-turn-1"
	textureID, researchID := "texture:"+docID, "research:"+docID
	const mainPoint, bodyText = "Whether the harbor strike ends and on what terms", "Harbor workers struck on Monday; talks are set for Tuesday."
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "start-turn-1",
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindStory,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID, types.StoryRefMainPoint: mainPoint},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "texture-work-turn", Objective: "write the story", AssignedAgentID: textureID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectoryID, Title: "Harbor strike", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-turn-1", DocID: docID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: owner, Content: bodyText, CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: textureID, OwnerID: owner, ComputerID: computer, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start the story: %v", err)
	}
	caller := types.RunRecord{RunID: "texture-run-turn", OwnerID: owner, ComputerID: computer, AgentID: textureID, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, ChannelID: docID, TrajectoryID: trajectoryID, State: types.RunRunning,
		Metadata: map[string]any{"lifecycle_work_item_id": start.InitialWork.WorkItemID, "work_item_ids": []string{start.InitialWork.WorkItemID}}, CreatedAt: now, UpdatedAt: now}
	project := types.ReplaceLifecycleActivationRequest{OwnerID: owner, ComputerID: computer, CommandID: "project-turn", TrajectoryID: trajectoryID, AgentID: textureID, Run: caller}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{AgentID: researchID, OwnerID: owner, ComputerID: computer, Profile: agentprofile.Research, Role: agentprofile.Research, ChannelID: docID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, owner, computer, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	textureAgent, err := s.GetAgentByScope(ctx, owner, computer, textureID)
	if err != nil {
		t.Fatal(err)
	}
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-1:cast:tray-turn", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover the strike", Body: `{"kind":"question","summary":"the strike"}`},
		// A story a cell typed into its own Cast is overwritten by the turn's.
		Cast:  &types.CommitmentCast{Story: &types.CastStory{Text: "forged by a cell", MainPoint: "forged"}},
		Inbox: researchID, SenderInbox: textureID, Addressee: researchID,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: textureID},
	}
	turn := types.ApplyTextureTurnRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "turn-cast-research", DocumentID: docID, TrajectoryID: trajectoryID,
		CallerAgentID: textureID, CallerRunID: caller.RunID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedCallerLifecycleVersion: textureAgent.LifecycleVersion, ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID,
		CallerWorkItemID: start.InitialWork.WorkItemID, CallerWorkDisposition: types.WorkItemOpen, Outcome: types.TextureTurnWait, Reason: "wait for research",
		InboxRecords: []types.CommitmentRecord{cast},
	}
	turn.CommandDigest, _ = store.ComputeApplyTextureTurnDigest(turn)
	restore := objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)
	defer restore()
	if _, err := s.ApplyTextureTurn(ctx, turn); err != nil {
		t.Fatalf("the Texture turn that casts research: %v", err)
	}

	rt.researchDeps = &researchtools.Dependencies{Captures: digestSink{ctxSink: &ctxSink{urls: map[string]string{}}, text: map[string]string{}}}
	objectgraph.ResetReaderGateCounts()
	woken := briefRunRecord(owner, computer, docID, "run-research-turn")
	b := rt.researchBriefOpening(ctx, woken)
	if b == nil || b.digest == nil {
		t.Fatal("the research run the turn's Cast woke got no digest")
	}
	opening := b.openingText()
	for _, want := range []string{"Main point: " + mainPoint, bodyText, "Title: Harbor strike"} {
		if !strings.Contains(opening, want) {
			t.Errorf("the research run's message lacks %q:\n%s", want, opening)
		}
	}
	if strings.Contains(opening, "forged") {
		t.Errorf("a story a cell typed was delivered:\n%s", opening)
	}
	if n := textureCrossings(); n != 0 {
		t.Errorf("the digest read a Texture document or revision as the research desk (%d crossings): %v", n, objectgraph.ReaderGateCounts())
	}
}
