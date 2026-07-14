package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
	"github.com/yusefmosiah/go-choir/internal/workitem"
)

const retiredUniversalWireEditionSourcePath = "universal-wire/Wire.texture"

func TestWirePublicationSettlesFromCorpusdReceiptWithoutLocalEdition(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	story, rev := seedEligibleWirePublicationFixture(t, handler, "doc-publish-slice")
	seedLocalEditionSentinel(t, handler)
	if _, err := handler.rt.Store().CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   "traj-publish-slice",
		OwnerID:        universalWirePlatformOwnerID(),
		Kind:           types.TrajectoryKindPublication,
		SettlementRule: defaultSettlementRuleForKind(types.TrajectoryKindPublication),
	}); err != nil {
		t.Fatalf("create publication trajectory: %v", err)
	}
	storyResolution, err := handler.rt.Store().CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              universalWirePlatformOwnerID(),
		TrajectoryID:         "traj-publish-slice",
		Objective:            "resolve wire story candidate",
		Reason:               "processor opened a wire story Texture route",
		AuthorityProfile:     agentprofile.Texture,
		ObjectiveFingerprint: workitem.StoryResolutionFingerprint("traj-publish-slice", story.DocID),
		CreatedByRunID:       "run-publish-slice",
		Details:              map[string]any{"kind": "wire_story_resolution", "doc_id": story.DocID},
	})
	if err != nil {
		t.Fatalf("create story-resolution work item: %v", err)
	}
	rec := &types.RunRecord{
		OwnerID: universalWirePlatformOwnerID(),
		RunID:   "run-publish-slice",
		Metadata: map[string]any{
			"type":           "texture_agent_revision",
			"request_intent": "universal_wire_processor_article_revision",
			"trajectory_id":  "traj-publish-slice",
		},
	}
	handler.rt.wirePlatformPublisher = func(context.Context, types.Document, types.Revision, *types.RunRecord) (*wirepublish.PublishTextureResponse, error) {
		return &wirepublish.PublishTextureResponse{
			PublicationID:        "pub-wire-test",
			PublicationVersionID: "pubver-wire-test",
			RoutePath:            "/pub/texture/madrid-dispatch",
			SourceRevisionHash:   "revhash-wire-test",
		}, nil
	}
	handler.rt.maybeAutonomousPublishWireArticle(ctx, story, rev, rec)

	editionDoc, err := handler.rt.Store().GetDocument(ctx, "local-wire-edition-sentinel", universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load local edition sentinel: %v", err)
	}
	if editionDoc.CurrentRevisionID != "local-wire-edition-rev" {
		t.Fatalf("local edition head = %q, want unchanged sentinel", editionDoc.CurrentRevisionID)
	}
	if _, err := handler.rt.Store().GetRevision(ctx, "local-wire-edition-rev", universalWirePlatformOwnerID()); err != nil {
		t.Fatalf("local edition sentinel revision changed or disappeared: %v", err)
	}
	trajectory, err := handler.rt.Store().GetTrajectory(ctx, universalWirePlatformOwnerID(), "traj-publish-slice")
	if err != nil {
		t.Fatalf("load publication trajectory: %v", err)
	}
	if trajectory.Status != types.TrajectorySettled || trajectory.SettledAt == nil {
		t.Fatalf("trajectory = %+v, want settled", trajectory)
	}
	if trajectory.SubjectRefs["publish_ref"] != "corpusd_publication:pub-wire-test/pubver-wire-test" {
		t.Fatalf("publish_ref = %q", trajectory.SubjectRefs["publish_ref"])
	}
	if trajectory.SubjectRefs["edition_ref"] != "corpusd_route:/pub/texture/madrid-dispatch" {
		t.Fatalf("edition_ref = %q, want canonical corpusd route", trajectory.SubjectRefs["edition_ref"])
	}
	openItems, err := handler.rt.Store().ListWorkItemsByTrajectory(ctx, universalWirePlatformOwnerID(), "traj-publish-slice", true)
	if err != nil {
		t.Fatalf("list open work items: %v", err)
	}
	if len(openItems) != 0 {
		t.Fatalf("open work items = %+v, want none", openItems)
	}
	storyResolution, err = handler.rt.Store().GetWorkItem(ctx, universalWirePlatformOwnerID(), storyResolution.WorkItemID)
	if err != nil || storyResolution.Status != types.WorkItemCompleted {
		t.Fatalf("story-resolution item = %+v err=%v", storyResolution, err)
	}
}

func TestWirePublicationFailureCancelsClaimsWithoutCancellingActivation(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	const trajectoryID = "traj-publish-failure"
	story, rev := seedEligibleWirePublicationFixture(t, handler, "doc-publish-failure")
	if err := handler.rt.Store().UpdateDocument(ctx, story); err != nil {
		t.Fatalf("persist canonical story head: %v", err)
	}
	if _, err := handler.rt.Store().CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   trajectoryID,
		OwnerID:        universalWirePlatformOwnerID(),
		Kind:           types.TrajectoryKindPublication,
		SettlementRule: defaultSettlementRuleForKind(types.TrajectoryKindPublication),
	}); err != nil {
		t.Fatalf("create publication trajectory: %v", err)
	}
	storyResolution, err := handler.rt.Store().CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              universalWirePlatformOwnerID(),
		TrajectoryID:         trajectoryID,
		Objective:            "resolve wire story candidate",
		Reason:               "processor opened a wire story Texture route",
		AuthorityProfile:     agentprofile.Texture,
		ObjectiveFingerprint: workitem.StoryResolutionFingerprint(trajectoryID, story.DocID),
		CreatedByRunID:       "run-publish-failure",
		Details:              map[string]any{"kind": "wire_story_resolution", "doc_id": story.DocID},
	})
	if err != nil {
		t.Fatalf("create story-resolution work item: %v", err)
	}
	now := time.Now().UTC()
	rec := &types.RunRecord{
		RunID:        "run-publish-failure",
		AgentID:      "texture-publish-failure",
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
		OwnerID:      universalWirePlatformOwnerID(),
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "publish canonical wire revision",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			"type":           "texture_agent_revision",
			"request_intent": "universal_wire_processor_article_revision",
			"trajectory_id":  trajectoryID,
		},
	}
	if err := handler.rt.Store().CreateRun(ctx, *rec); err != nil {
		t.Fatalf("create publication activation: %v", err)
	}
	activationCtx, cancelActivation := context.WithCancel(context.Background())
	handler.rt.runningMu.Lock()
	handler.rt.running[rec.RunID] = cancelActivation
	handler.rt.runningMu.Unlock()
	t.Cleanup(func() {
		handler.rt.runningMu.Lock()
		delete(handler.rt.running, rec.RunID)
		handler.rt.runningMu.Unlock()
		cancelActivation()
	})

	publisherCalled := false
	handler.rt.wirePlatformPublisher = func(context.Context, types.Document, types.Revision, *types.RunRecord) (*wirepublish.PublishTextureResponse, error) {
		publisherCalled = true
		return nil, errors.New("platform publication failed")
	}
	handler.rt.maybeAutonomousPublishWireArticle(activationCtx, story, rev, rec)
	if !publisherCalled {
		t.Fatal("failing platform publisher was not called")
	}

	openItems, err := handler.rt.Store().ListWorkItemsByTrajectory(ctx, rec.OwnerID, trajectoryID, true)
	if err != nil {
		t.Fatalf("list open work items after publication failure: %v", err)
	}
	if len(openItems) != 0 {
		t.Fatalf("open work items after publication failure = %+v, want none", openItems)
	}
	allItems, err := handler.rt.Store().ListWorkItemsByTrajectory(ctx, rec.OwnerID, trajectoryID, false)
	if err != nil {
		t.Fatalf("list all work items after publication failure: %v", err)
	}
	publicationFingerprint := workitem.PublicationFingerprint(trajectoryID, rev.RevisionID)
	var publicationItem types.WorkItemRecord
	found := false
	for _, item := range allItems {
		if item.ObjectiveFingerprint == publicationFingerprint {
			publicationItem = item
			found = true
			break
		}
	}
	if !found || publicationItem.Status != types.WorkItemCancelled {
		t.Fatalf("publication work item = %+v found=%v, want cancelled", publicationItem, found)
	}
	storyResolution, err = handler.rt.Store().GetWorkItem(ctx, rec.OwnerID, storyResolution.WorkItemID)
	if err != nil {
		t.Fatalf("load story-resolution work item: %v", err)
	}
	if storyResolution.Status != types.WorkItemCancelled {
		t.Fatalf("story-resolution status = %s, want cancelled", storyResolution.Status)
	}
	trajectory, err := handler.rt.Store().GetTrajectory(ctx, rec.OwnerID, trajectoryID)
	if err != nil {
		t.Fatalf("load failed publication trajectory: %v", err)
	}
	if trajectory.Status != types.TrajectoryCancelled || trajectory.SettledAt != nil {
		t.Fatalf("trajectory = %+v, want cancelled and not settled", trajectory)
	}
	canonicalDoc, err := handler.rt.Store().GetDocument(ctx, story.DocID, story.OwnerID)
	if err != nil {
		t.Fatalf("load canonical story after publication failure: %v", err)
	}
	if canonicalDoc.CurrentRevisionID != rev.RevisionID {
		t.Fatalf("canonical story head = %q, want durable revision %q", canonicalDoc.CurrentRevisionID, rev.RevisionID)
	}
	canonicalRev, err := handler.rt.Store().GetRevision(ctx, rev.RevisionID, rev.OwnerID)
	if err != nil {
		t.Fatalf("load canonical revision after publication failure: %v", err)
	}
	if canonicalRev.DocID != rev.DocID || canonicalRev.Content != rev.Content {
		t.Fatalf("canonical revision changed after publication failure: %+v", canonicalRev)
	}
	storedRun, err := handler.rt.Store().GetRun(ctx, rec.RunID)
	if err != nil {
		t.Fatalf("load publication activation: %v", err)
	}
	if storedRun.State != types.RunRunning || storedRun.FinishedAt != nil {
		t.Fatalf("publication activation = %+v, want still running", storedRun)
	}
	select {
	case <-activationCtx.Done():
		t.Fatalf("publication activation was self-cancelled: %v", activationCtx.Err())
	default:
	}
}

func TestWirePublicationDoesNotBootstrapLocalEdition(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	story, rev := seedEligibleWirePublicationFixture(t, handler, "doc-no-edition")
	if _, err := handler.rt.Store().CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID: "traj-no-edition",
		OwnerID:      universalWirePlatformOwnerID(),
		Kind:         types.TrajectoryKindPublication,
	}); err != nil {
		t.Fatalf("create trajectory: %v", err)
	}
	rec := &types.RunRecord{OwnerID: universalWirePlatformOwnerID(), RunID: "run-no-edition", Metadata: map[string]any{
		"type": "texture_agent_revision", "request_intent": "universal_wire_processor_article_revision", "trajectory_id": "traj-no-edition",
	}}
	handler.rt.wirePlatformPublisher = func(context.Context, types.Document, types.Revision, *types.RunRecord) (*wirepublish.PublishTextureResponse, error) {
		return &wirepublish.PublishTextureResponse{PublicationID: "pub-2", PublicationVersionID: "pubver-2", RoutePath: "/pub/texture/no-edition"}, nil
	}
	handler.rt.maybeAutonomousPublishWireArticle(ctx, story, rev, rec)
	if _, err := handler.rt.Store().GetDocumentAlias(ctx, universalWirePlatformOwnerID(), retiredUniversalWireEditionSourcePath); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("local Wire.texture alias err = %v, want not found after successful corpusd publication", err)
	}
}

func seedEligibleWirePublicationFixture(t *testing.T, handler *APIHandler, docID string) (types.Document, types.Revision) {
	t.Helper()
	now := time.Now().UTC()
	doc := types.Document{DocID: docID, OwnerID: universalWirePlatformOwnerID(), Title: "Madrid dispatch.texture", CreatedAt: now, UpdatedAt: now}
	if err := handler.rt.Store().CreateDocument(context.Background(), doc); err != nil {
		t.Fatalf("create story document: %v", err)
	}
	metadata, _ := json.Marshal(map[string]any{
		"source": "edit_texture", "revision_role": textureRevisionRoleCanonical,
		"ingestion_handoff_cycle_id": "cycle-live", "ingestion_handoff_request_kind": "reconciler",
	})
	content := "# Madrid dispatch\n\nA complete canonical article paragraph."
	rev := types.Revision{
		RevisionID: "rev-" + docID, DocID: docID, OwnerID: doc.OwnerID,
		AuthorKind: types.AuthorAppAgent, AuthorLabel: "texture:" + docID,
		Content: content, BodyDoc: runtimeTestTextureBodyDoc(t, docID, "rev-"+docID, content),
		Citations: json.RawMessage("[]"), Metadata: metadata, CreatedAt: now,
	}
	if err := handler.rt.Store().CreateRevision(context.Background(), rev); err != nil {
		t.Fatalf("create story revision: %v", err)
	}
	doc.CurrentRevisionID = rev.RevisionID
	return doc, rev
}

func seedLocalEditionSentinel(t *testing.T, handler *APIHandler) {
	t.Helper()
	now := time.Now().UTC()
	doc := types.Document{DocID: "local-wire-edition-sentinel", OwnerID: universalWirePlatformOwnerID(), Title: "Wire.texture", CreatedAt: now, UpdatedAt: now}
	if err := handler.rt.Store().CreateDocument(context.Background(), doc); err != nil {
		t.Fatalf("create edition sentinel: %v", err)
	}
	content := "# retired local edition"
	rev := types.Revision{
		RevisionID: "local-wire-edition-rev", DocID: doc.DocID, OwnerID: doc.OwnerID,
		AuthorKind: types.AuthorAppAgent, AuthorLabel: "retired-local-edition",
		Content: content, BodyDoc: runtimeTestTextureBodyDoc(t, doc.DocID, "local-wire-edition-rev", content),
		Citations: json.RawMessage("[]"), Metadata: json.RawMessage(`{"source":"retired_local_edition"}`), CreatedAt: now,
	}
	if err := handler.rt.Store().CreateRevision(context.Background(), rev); err != nil {
		t.Fatalf("create edition sentinel revision: %v", err)
	}
	doc.CurrentRevisionID = rev.RevisionID
	if err := handler.rt.Store().UpdateDocument(context.Background(), doc); err != nil {
		t.Fatalf("advance sentinel head: %v", err)
	}
	if err := handler.rt.Store().UpsertDocumentAlias(context.Background(), doc.OwnerID, retiredUniversalWireEditionSourcePath, doc.DocID, now); err != nil {
		t.Fatalf("create edition sentinel alias: %v", err)
	}
}
