package textureowner

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The Research more door POSTs /revise to the paper for a published story
// (docs/problems/story-expansion-request-no-reader-door-and-wrong-wake-framing-2026-10-20.md,
// P5: staging 2026-10-21 03:01 UTC answered 409 "lifecycle has no open desk
// target work item"). A story's trajectory has no work items by design
// (types.TrajectoryKindStory), so the handler's open-Texture-work target
// never exists for it. Ways the fix can fail, written before the code:
//   - F1: a live story whose texture:<doc> agent is live still answers 409,
//     or answers 202 but dispatches no wake to texture:<doc>, or the wake is
//     dropped downstream (the occurrence resolves invalid for lack of an
//     exact open work item) so no run is created for the reopened work;
//   - F2: the fix opens the door for every trajectory with no open Texture
//     work: a non-story trajectory whose Texture work was refused must
//     still answer 409 "lifecycle has no open desk target work item";
//   - F3: a story revise retried under the same client_request_id commits a
//     second revision or wakes the desk twice (the handler answers a retry
//     that carries the pre-commit head with 409, as for task trajectories;
//     the exact-replay path is the store's), or the revision id is not
//     uuid.NewSHA1(OID, owner\x00computer\x00trajectory\x00client_request_id),
//     the id the proxy recomputes (internal/proxy/platform_expand.go);
//   - F4: a story trajectory whose texture:<doc> agent is absent (or whose
//     doc, kind or status differs) is accepted anyway, so the wake would
//     have no desk to reach.

type storyReviseDispatch struct{ to, kind string }

// openOwnerStory opens a story the way management does (a Cast to
// texture:new) on the runtime's own computer, with no Texture run yet.
func openOwnerStory(t *testing.T, h *Handler, suffix string) (owner string, doc types.Document) {
	t.Helper()
	owner, computer := "owner-story-"+suffix, h.Core.TextureComputerID()
	management := agentprofile.Management + ":" + owner
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-open-" + suffix + ":cast:c1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "Harbor reopens " + suffix},
		Cast:      &types.CommitmentCast{Captures: []string{"cap-court"}, Publish: types.CastPublishAuto},
		Inbox:     types.StoryOpenInbox, SenderInbox: management, Addressee: types.StoryOpenInbox,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: management},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cell-records:open-" + suffix, Records: []types.CommitmentRecord{cast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := h.Store.CommitInboxRecords(t.Context(), req)
	if err != nil || len(receipt.Records) != 1 || !receipt.Records[0].OpenedStory {
		t.Fatalf("open story: %+v, %v", receipt, err)
	}
	doc, err = h.Store.GetLifecycleDocument(t.Context(), owner, computer, strings.TrimPrefix(receipt.Records[0].Inbox, agentprofile.Texture+":"))
	if err != nil {
		t.Fatalf("story document: %v", err)
	}
	return owner, doc
}

func TestOwnerReviseReachesLivingStoryTextureDesk(t *testing.T) {
	core, handler := testAPISetup(t)
	var dispatches []storyReviseDispatch
	var wakeContents []string
	core.SetDispatchActor(func(_ context.Context, _, _, to, kind, content, _, _ string) error {
		dispatches = append(dispatches, storyReviseDispatch{to: to, kind: kind})
		wakeContents = append(wakeContents, content)
		return nil
	})
	owner, doc := openOwnerStory(t, handler, "revise")
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), owner, doc.ComputerID, doc.TrajectoryID)
	if err != nil || snapshot.Trajectory.Kind != types.TrajectoryKindStory || len(snapshot.WorkItems) != 0 {
		t.Fatalf("story snapshot kind=%q work=%d err=%v", snapshot.Trajectory.Kind, len(snapshot.WorkItems), err)
	}
	path := "/api/texture/documents/" + doc.DocID + "/revise"

	// F1: accepted, and the Texture desk of the story is woken.
	first := postOwnerInstruction(t, handler, path, owner, "expand-1", "research the harbor more", doc.CurrentRevisionID)
	if first.Code != http.StatusAccepted {
		t.Fatalf("story revise status=%d body=%s", first.Code, first.Body.String())
	}
	woke := 0
	for _, d := range dispatches {
		if d.to == agentprofile.Texture+":"+doc.DocID && d.kind == "coagent_result" {
			woke++
		}
	}
	if woke != 1 {
		t.Fatalf("wakes to the story desk = %d, dispatches=%+v; want exactly 1", woke, dispatches)
	}

	// F1 downstream: the wake is not dropped. The occurrence resolves to the
	// reopened Texture work item, and the actor path's reconcile creates a run
	// for texture:<doc> carrying that item.
	workID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{"texture-owner-reopen", owner, doc.ComputerID, doc.TrajectoryID, "expand-1"}, "\x00"))).String()
	if len(wakeContents) != 1 {
		t.Fatalf("wake contents = %d, want 1", len(wakeContents))
	}
	agentID := agentprofile.Texture + ":" + doc.DocID
	occurrence, state, err := handler.ResolveTextureActorOccurrence(t.Context(), owner, doc.ComputerID, agentID, wakeContents[0])
	if err != nil || state != TextureActorOccurrencePending || occurrence.ResolvedTargetWorkItemID != workID {
		t.Fatalf("resolve occurrence work=%q state=%q err=%v; want pending on reopened work %s", occurrence.ResolvedTargetWorkItemID, state, err, workID)
	}
	run, err := handler.ReconcileActorOccurrenceWake(t.Context(), owner, doc.ComputerID, agentID, occurrence.ResolvedTargetWorkItemID, occurrence)
	if err != nil || run == nil || run.AgentID != agentID || metadataStringValue(run.Metadata, "lifecycle_work_item_id") != workID {
		t.Fatalf("reconcile run=%+v err=%v; want a run for %s on work %s", run, err, agentID, workID)
	}

	// F3: the revision id is the one the proxy recomputes, and a retry
	// replays without a second wake.
	want := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{owner, doc.ComputerID, doc.TrajectoryID, "expand-1"}, "\x00"))).String()
	if !strings.Contains(first.Body.String(), want) {
		t.Fatalf("revision id: body=%s want %s", first.Body.String(), want)
	}
	after, err := core.Store().GetLifecycleSnapshot(t.Context(), owner, doc.ComputerID, doc.TrajectoryID)
	if err != nil || after.HeadRevision.RevisionID != want || after.HeadRevision.AuthorKind != types.AuthorUser {
		t.Fatalf("story head=%+v err=%v; want the owner revision %s", after.HeadRevision, err, want)
	}
	before := len(dispatches)
	// The handler checks the expected head before the commit, so a retry
	// carrying the pre-commit head answers 409 "document head changed" (the
	// same as a task trajectory today); what matters is no second revision
	// and no second wake.
	retry := postOwnerInstruction(t, handler, path, owner, "expand-1", "research the harbor more", doc.CurrentRevisionID)
	again, err := core.Store().GetLifecycleSnapshot(t.Context(), owner, doc.ComputerID, doc.TrajectoryID)
	if retry.Code == http.StatusInternalServerError || len(dispatches) != before || err != nil || again.HeadRevision.RevisionID != want {
		t.Fatalf("retry status=%d body=%s dispatches %d -> %d head=%s err=%v; want no second revision and no second wake", retry.Code, retry.Body.String(), before, len(dispatches), again.HeadRevision.RevisionID, err)
	}
}

func TestOwnerReviseNonStoryWithoutOpenTextureWorkStillRefused(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	dispatches := 0
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		dispatches++
		return nil
	})
	refuse := types.RefuseLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "refuse-f2",
		TrajectoryID: start.TrajectoryID, WorkItemID: start.InitialWork.WorkItemID, ActingAgentID: start.Agent.AgentID,
		RefusalRef: "refusal://f2", Reason: "F2",
	}
	refuse.CommandDigest, _ = store.ComputeRefuseLifecycleWorkDigest(refuse)
	if _, err := core.Store().RefuseLifecycleWork(t.Context(), refuse); err != nil {
		t.Fatal(err)
	}
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "f2", "no work", start.InitialRevision.RevisionID)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "lifecycle has no open desk target work item") || dispatches != 0 {
		t.Fatalf("non-story status=%d dispatches=%d body=%s", response.Code, dispatches, response.Body.String())
	}
}

// F4 cannot be built through the store: StartLifecycle and the story opening
// always create texture:<doc> (lifecycle start: "durable subject must be
// texture:<doc>"). The refusal is pinned on the decision itself, over
// hand-built snapshots.
func TestStoryTextureDeskLiveRefusesWithoutLiveTextureAgent(t *testing.T) {
	story := func(mutate func(*types.LifecycleSnapshot)) types.LifecycleSnapshot {
		snap := types.LifecycleSnapshot{
			Trajectory: types.TrajectoryRecord{Kind: types.TrajectoryKindStory, Status: types.TrajectoryLive, SubjectRefs: map[string]string{"doc_id": "doc-1"}},
			Agents:     []types.AgentRecord{{AgentID: "texture:doc-1", LifecycleVersion: 1}},
		}
		if mutate != nil {
			mutate(&snap)
		}
		return snap
	}
	cases := []struct {
		name string
		snap types.LifecycleSnapshot
		want bool
	}{
		{"live story with live desk", story(nil), true},
		{"texture agent absent", story(func(s *types.LifecycleSnapshot) { s.Agents = nil }), false},
		{"texture agent of another doc", story(func(s *types.LifecycleSnapshot) { s.Agents[0].AgentID = "texture:doc-2" }), false},
		{"texture agent never versioned", story(func(s *types.LifecycleSnapshot) { s.Agents[0].LifecycleVersion = 0 }), false},
		{"not a story", story(func(s *types.LifecycleSnapshot) { s.Trajectory.Kind = types.TrajectoryKindTask }), false},
		{"story of another doc", story(func(s *types.LifecycleSnapshot) { s.Trajectory.SubjectRefs["doc_id"] = "doc-2" }), false},
		{"settled story", story(func(s *types.LifecycleSnapshot) { s.Trajectory.Status = types.TrajectorySettled }), false},
	}
	for _, c := range cases {
		if got := storyTextureDeskLive(c.snap, "doc-1"); got != c.want {
			t.Errorf("%s: storyTextureDeskLive = %v, want %v", c.name, got, c.want)
		}
	}
}
