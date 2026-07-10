package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

func TestWireAutonomousPublishTranscludesEditionAndDebounces(t *testing.T) {
	_, handler := testAPISetup(t)
	seedUniversalWireEditionFixture(t, handler)
	story := seedPlatformSourceNetworkTextureFixture(t, handler, "doc-publish-slice")
	ctx := context.Background()

	story, err := handler.rt.Store().GetDocument(ctx, story.DocID, story.OwnerID)
	if err != nil {
		t.Fatalf("reload story document: %v", err)
	}
	rev, err := handler.rt.Store().GetRevision(ctx, story.CurrentRevisionID, story.OwnerID)
	if err != nil {
		t.Fatalf("load story revision: %v", err)
	}
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
		Objective:            "resolve wire story candidate to publication or explicit non-publication decision",
		Reason:               "processor opened a wire story Texture route",
		AuthorityProfile:     AgentProfileTexture,
		ObjectiveFingerprint: wireStoryResolutionWorkItemFingerprint("traj-publish-slice", story.DocID),
		CreatedByRunID:       "run-publish-slice",
		Details: map[string]any{
			"kind":   "wire_story_resolution",
			"doc_id": story.DocID,
		},
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
	handler.rt.wirePlatformPublisher = func(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) (*wirepublish.PublishTextureResponse, error) {
		return &wirepublish.PublishTextureResponse{
			PublicationID:        "pub-wire-test",
			PublicationVersionID: "pubver-wire-test",
			RoutePath:            "wire/madrid-dispatch",
			SourceRevisionHash:   "revhash-wire-test",
		}, nil
	}
	handler.rt.maybeAutonomousPublishWireArticle(ctx, story, rev, rec)

	editionDocID, err := handler.rt.Store().GetDocumentAlias(ctx, universalWirePlatformOwnerID(), universalWireEditionSourcePath)
	if err != nil {
		t.Fatalf("resolve wire edition alias: %v", err)
	}
	editionDoc, err := handler.rt.Store().GetDocument(ctx, editionDocID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load wire edition document: %v", err)
	}
	editionRev, err := handler.rt.Store().GetRevision(ctx, editionDoc.CurrentRevisionID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load wire edition revision: %v", err)
	}
	if !strings.Contains(editionRev.Content, "texture:"+story.DocID) {
		t.Fatalf("edition content missing transclusion for %s: %q", story.DocID, editionRev.Content)
	}
	if handler.rt.wirePublishDebouncer == nil {
		t.Fatal("expected publish debouncer after autonomous wire publication")
	}
	handler.rt.wirePublishDebouncer.mu.Lock()
	pending := len(handler.rt.wirePublishDebouncer.pendingDocIDs)
	handler.rt.wirePublishDebouncer.mu.Unlock()
	if pending != 1 {
		t.Fatalf("pending publish count = %d, want 1", pending)
	}
	rev, err = handler.rt.Store().GetRevision(ctx, story.CurrentRevisionID, story.OwnerID)
	if err != nil {
		t.Fatalf("reload story revision after publish: %v", err)
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	ref, _ := meta["corpusd_publication_ref"].(map[string]any)
	if ref == nil || ref["route_path"] != "wire/madrid-dispatch" {
		t.Fatalf("expected corpusd_publication_ref on revision metadata, got %+v", meta)
	}
	trajectory, err := handler.rt.Store().GetTrajectory(ctx, universalWirePlatformOwnerID(), "traj-publish-slice")
	if err != nil {
		t.Fatalf("load publication trajectory: %v", err)
	}
	if trajectory.Status != types.TrajectorySettled || trajectory.SettledAt == nil {
		t.Fatalf("publication trajectory = %+v, want settled with settled_at", trajectory)
	}
	if trajectory.SubjectRefs["publish_ref"] != "corpusd_publication:pub-wire-test/pubver-wire-test" {
		t.Fatalf("publish_ref = %q, want platform publication ref; trajectory=%+v", trajectory.SubjectRefs["publish_ref"], trajectory)
	}
	if !strings.HasPrefix(trajectory.SubjectRefs["edition_ref"], "texture_edition:") {
		t.Fatalf("edition_ref = %q, want edition ref; trajectory=%+v", trajectory.SubjectRefs["edition_ref"], trajectory)
	}
	openItems, err := handler.rt.Store().ListWorkItemsByTrajectory(ctx, universalWirePlatformOwnerID(), "traj-publish-slice", true)
	if err != nil {
		t.Fatalf("list open work items after publish: %v", err)
	}
	if len(openItems) != 0 {
		t.Fatalf("open work items after successful publish = %+v, want none", openItems)
	}
	storyResolution, err = handler.rt.Store().GetWorkItem(ctx, universalWirePlatformOwnerID(), storyResolution.WorkItemID)
	if err != nil {
		t.Fatalf("reload story-resolution work item: %v", err)
	}
	if storyResolution.Status != types.WorkItemCompleted {
		t.Fatalf("story-resolution work item status = %s, want completed", storyResolution.Status)
	}
}

func TestWirePlatformPublishFailsClosedWithoutEditionWhenCorpusdFails(t *testing.T) {
	_, handler := testAPISetup(t)
	seedUniversalWireEditionFixture(t, handler)
	story := seedPlatformSourceNetworkTextureFixture(t, handler, "doc-publish-fail")
	ctx := context.Background()
	story, err := handler.rt.Store().GetDocument(ctx, story.DocID, story.OwnerID)
	if err != nil {
		t.Fatalf("reload story document: %v", err)
	}
	rev, err := handler.rt.Store().GetRevision(ctx, story.CurrentRevisionID, story.OwnerID)
	if err != nil {
		t.Fatalf("load story revision: %v", err)
	}
	editionBefore, _ := handler.rt.Store().GetDocumentAlias(ctx, universalWirePlatformOwnerID(), universalWireEditionSourcePath)
	if _, err := handler.rt.Store().CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   "traj-publish-fail",
		OwnerID:        universalWirePlatformOwnerID(),
		Kind:           types.TrajectoryKindPublication,
		SettlementRule: defaultSettlementRuleForKind(types.TrajectoryKindPublication),
	}); err != nil {
		t.Fatalf("create publication trajectory: %v", err)
	}
	if _, err := handler.rt.Store().CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              universalWirePlatformOwnerID(),
		TrajectoryID:         "traj-publish-fail",
		Objective:            "resolve wire story candidate to publication or explicit non-publication decision",
		Reason:               "processor opened a wire story Texture route",
		AuthorityProfile:     AgentProfileTexture,
		ObjectiveFingerprint: wireStoryResolutionWorkItemFingerprint("traj-publish-fail", story.DocID),
		CreatedByRunID:       "run-publish-fail",
		Details: map[string]any{
			"kind":   "wire_story_resolution",
			"doc_id": story.DocID,
		},
	}); err != nil {
		t.Fatalf("create story-resolution work item: %v", err)
	}
	rec := &types.RunRecord{
		OwnerID: universalWirePlatformOwnerID(),
		RunID:   "run-publish-fail",
		Metadata: map[string]any{
			"request_intent": "universal_wire_processor_article_revision",
			"trajectory_id":  "traj-publish-fail",
		},
	}
	handler.rt.wirePlatformPublisher = func(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) (*wirepublish.PublishTextureResponse, error) {
		return nil, context.Canceled
	}
	handler.rt.maybeAutonomousPublishWireArticle(ctx, story, rev, rec)

	editionDocID, err := handler.rt.Store().GetDocumentAlias(ctx, universalWirePlatformOwnerID(), universalWireEditionSourcePath)
	if err != nil {
		t.Fatalf("resolve wire edition alias: %v", err)
	}
	if editionDocID != editionBefore {
		t.Fatalf("edition alias changed unexpectedly")
	}
	editionDoc, err := handler.rt.Store().GetDocument(ctx, editionDocID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load wire edition document: %v", err)
	}
	editionRev, err := handler.rt.Store().GetRevision(ctx, editionDoc.CurrentRevisionID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load wire edition revision: %v", err)
	}
	if strings.Contains(editionRev.Content, "texture:"+story.DocID) {
		t.Fatalf("edition should not transclude story when platform publish fails: %q", editionRev.Content)
	}
	if handler.rt.wirePublishDebouncer != nil {
		handler.rt.wirePublishDebouncer.mu.Lock()
		pending := len(handler.rt.wirePublishDebouncer.pendingDocIDs)
		handler.rt.wirePublishDebouncer.mu.Unlock()
		if pending != 0 {
			t.Fatalf("debouncer pending = %d, want 0 after failed platform publish", pending)
		}
	}
	trajectory, err := handler.rt.Store().GetTrajectory(ctx, universalWirePlatformOwnerID(), "traj-publish-fail")
	if err != nil {
		t.Fatalf("load publication trajectory: %v", err)
	}
	if trajectory.Status != types.TrajectoryLive || trajectory.SettledAt != nil {
		t.Fatalf("failed publication trajectory = %+v, want live without settled_at", trajectory)
	}
	if trajectory.SubjectRefs["publish_ref"] != "" || trajectory.SubjectRefs["edition_ref"] != "" {
		t.Fatalf("failed platform publish should not write trajectory refs: %+v", trajectory.SubjectRefs)
	}
	obligations, err := handler.rt.TrajectoryObligations(ctx, universalWirePlatformOwnerID(), "traj-publish-fail")
	if err != nil {
		t.Fatalf("trajectory obligations after failed publish: %v", err)
	}
	if got, want := len(obligations.OpenWorkItems), 2; got != want {
		t.Fatalf("open work items after failed publish = %+v, want %d items (story-resolution + in-flight publication)", obligations.OpenWorkItems, want)
	}
	if obligations.SettlementReady {
		t.Fatalf("failed publish should not be settlement-ready: %+v", obligations)
	}
}

func TestWireAutonomousPublishBootstrapsEditionWhenAliasMissing(t *testing.T) {
	_, handler := testAPISetup(t)
	// Deliberately do NOT seed the Universal Wire edition fixture: the
	// publication path must bootstrap the edition alias on first use.
	story := seedPlatformSourceNetworkTextureFixture(t, handler, "doc-bootstrap-edition")
	ctx := context.Background()

	story, err := handler.rt.Store().GetDocument(ctx, story.DocID, story.OwnerID)
	if err != nil {
		t.Fatalf("reload story document: %v", err)
	}
	rev, err := handler.rt.Store().GetRevision(ctx, story.CurrentRevisionID, story.OwnerID)
	if err != nil {
		t.Fatalf("load story revision: %v", err)
	}
	if _, err := handler.rt.Store().CreateTrajectoryIfAbsent(ctx, types.TrajectoryRecord{
		TrajectoryID:   "traj-bootstrap-edition",
		OwnerID:        universalWirePlatformOwnerID(),
		Kind:           types.TrajectoryKindPublication,
		SettlementRule: defaultSettlementRuleForKind(types.TrajectoryKindPublication),
	}); err != nil {
		t.Fatalf("create publication trajectory: %v", err)
	}
	if _, err := handler.rt.Store().CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              universalWirePlatformOwnerID(),
		TrajectoryID:         "traj-bootstrap-edition",
		Objective:            "resolve wire story candidate to publication or explicit non-publication decision",
		Reason:               "processor opened a wire story Texture route",
		AuthorityProfile:     AgentProfileTexture,
		ObjectiveFingerprint: wireStoryResolutionWorkItemFingerprint("traj-bootstrap-edition", story.DocID),
		CreatedByRunID:       "run-bootstrap-edition",
		Details: map[string]any{
			"kind":   "wire_story_resolution",
			"doc_id": story.DocID,
		},
	}); err != nil {
		t.Fatalf("create story-resolution work item: %v", err)
	}
	rec := &types.RunRecord{
		OwnerID: universalWirePlatformOwnerID(),
		RunID:   "run-bootstrap-edition",
		Metadata: map[string]any{
			"type":           "texture_agent_revision",
			"request_intent": "universal_wire_processor_article_revision",
			"trajectory_id":  "traj-bootstrap-edition",
		},
	}
	handler.rt.wirePlatformPublisher = func(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) (*wirepublish.PublishTextureResponse, error) {
		return &wirepublish.PublishTextureResponse{
			PublicationID:        "pub-bootstrap",
			PublicationVersionID: "pubver-bootstrap",
			RoutePath:            "wire/bootstrap-dispatch",
			SourceRevisionHash:   "revhash-bootstrap",
		}, nil
	}
	handler.rt.maybeAutonomousPublishWireArticle(ctx, story, rev, rec)

	// The edition alias must now exist and transclude the story.
	editionDocID, err := handler.rt.Store().GetDocumentAlias(ctx, universalWirePlatformOwnerID(), universalWireEditionSourcePath)
	if err != nil {
		t.Fatalf("edition alias was not bootstrapped: %v", err)
	}
	editionDoc, err := handler.rt.Store().GetDocument(ctx, editionDocID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load bootstrapped edition document: %v", err)
	}
	editionRev, err := handler.rt.Store().GetRevision(ctx, editionDoc.CurrentRevisionID, universalWirePlatformOwnerID())
	if err != nil {
		t.Fatalf("load bootstrapped edition revision: %v", err)
	}
	if !strings.Contains(editionRev.Content, "texture:"+story.DocID) {
		t.Fatalf("bootstrapped edition content missing transclusion for %s: %q", story.DocID, editionRev.Content)
	}

	// The Universal Wire API must now return the story.
	w := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/universal-wire/stories", "", "user-universal-wire")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/universal-wire/stories status = %d body=%s", w.Code, w.Body.String())
	}
	var resp universalWireStoriesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode stories response: %v", err)
	}
	if resp.Source != "universal-wire-edition-texture" {
		t.Fatalf("source = %q, want edition texture source after bootstrap", resp.Source)
	}
	if len(resp.Stories) != 1 {
		t.Fatalf("stories length = %d, want bootstrapped edition to expose the published story: %+v", len(resp.Stories), resp.Stories)
	}
	if resp.Stories[0].StoryTextureDoc != story.DocID {
		t.Fatalf("first story texture doc = %q, want %s", resp.Stories[0].StoryTextureDoc, story.DocID)
	}
}

func TestEnsureUniversalWireEditionRepairsDanglingAlias(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	ownerID := universalWirePlatformOwnerID()
	const missingDocID = "missing-wire-edition-document"

	if err := handler.rt.Store().UpsertDocumentAlias(ctx, ownerID, universalWireEditionSourcePath, missingDocID, time.Now().UTC()); err != nil {
		t.Fatalf("seed dangling edition alias: %v", err)
	}
	editionDoc, editionRev, err := handler.rt.ensureUniversalWireEdition(ctx, ownerID)
	if err != nil {
		t.Fatalf("repair dangling edition alias: %v", err)
	}
	if editionDoc.DocID == "" || editionDoc.DocID == missingDocID {
		t.Fatalf("repaired edition doc id = %q, want a new live document", editionDoc.DocID)
	}
	if editionDoc.CurrentRevisionID != editionRev.RevisionID || editionRev.DocID != editionDoc.DocID {
		t.Fatalf("repaired edition document/revision mismatch: doc=%+v rev=%+v", editionDoc, editionRev)
	}
	gotAlias, err := handler.rt.Store().GetDocumentAlias(ctx, ownerID, universalWireEditionSourcePath)
	if err != nil {
		t.Fatalf("resolve repaired edition alias: %v", err)
	}
	if gotAlias != editionDoc.DocID {
		t.Fatalf("repaired edition alias = %q, want %q", gotAlias, editionDoc.DocID)
	}
}

func TestWireInputRevisionDoesNotAutonomousPublish(t *testing.T) {
	_, handler := testAPISetup(t)
	seedUniversalWireEditionFixture(t, handler)
	ctx := context.Background()
	now := time.Now().UTC()
	docID := "doc-input-only-publish"
	ownerID := universalWirePlatformOwnerID()
	doc := types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Seed brief only.texture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create story document: %v", err)
	}
	seedMeta, _ := json.Marshal(map[string]any{
		"source":                         "edit_texture",
		"revision_role":                  textureRevisionRoleInput,
		"input_origin":                   textureInputOriginProcessorHandoff,
		"artifact_kind":                  "source_brief",
		"ingestion_handoff_cycle_id":     "cycle-input-only",
		"ingestion_handoff_request_id":   "processor-input-only",
		"ingestion_handoff_request_kind": "processor",
	})
	rev := types.Revision{
		RevisionID:  "rev-input-only",
		DocID:       doc.DocID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "processor:processor-global",
		Content:     "Source brief: processor handoff seed only.",
		BodyDoc:     runtimeTestTextureBodyDoc(t, doc.DocID, "rev-input-only", "Source brief: processor handoff seed only."),
		Citations:   json.RawMessage("[]"),
		Metadata:    seedMeta,
		CreatedAt:   now,
	}
	if err := handler.rt.Store().CreateRevision(ctx, rev); err != nil {
		t.Fatalf("create input seed revision: %v", err)
	}
	doc, err := handler.rt.Store().GetDocument(ctx, doc.DocID, ownerID)
	if err != nil {
		t.Fatalf("reload story document: %v", err)
	}
	rev, err = handler.rt.Store().GetRevision(ctx, doc.CurrentRevisionID, ownerID)
	if err != nil {
		t.Fatalf("load input revision: %v", err)
	}

	editionDocID, err := handler.rt.Store().GetDocumentAlias(ctx, ownerID, universalWireEditionSourcePath)
	if err != nil {
		t.Fatalf("resolve wire edition alias: %v", err)
	}
	editionBefore, err := handler.rt.Store().GetDocument(ctx, editionDocID, ownerID)
	if err != nil {
		t.Fatalf("load edition before publish attempt: %v", err)
	}

	rec := &types.RunRecord{
		OwnerID: ownerID,
		Metadata: map[string]any{
			"type":           "texture_agent_revision",
			"request_intent": "universal_wire_processor_article_revision",
			"trajectory_id":  "traj-publish-skip",
		},
	}
	handler.rt.maybeAutonomousPublishWireArticle(ctx, doc, rev, rec)

	editionAfter, err := handler.rt.Store().GetDocument(ctx, editionDocID, ownerID)
	if err != nil {
		t.Fatalf("load edition after publish attempt: %v", err)
	}
	if editionAfter.CurrentRevisionID != editionBefore.CurrentRevisionID {
		t.Fatalf("input revision advanced edition head %q -> %q", editionBefore.CurrentRevisionID, editionAfter.CurrentRevisionID)
	}
	if handler.rt.wirePublishDebouncer != nil {
		handler.rt.wirePublishDebouncer.mu.Lock()
		pending := len(handler.rt.wirePublishDebouncer.pendingDocIDs)
		handler.rt.wirePublishDebouncer.mu.Unlock()
		if pending != 0 {
			t.Fatalf("input revision should not enqueue debouncer, pending=%d", pending)
		}
	}
}
