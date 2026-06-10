package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

func TestWireAutonomousPublishTranscludesEditionAndDebounces(t *testing.T) {
	_, handler := testAPISetup(t)
	seedUniversalWireEditionFixture(t, handler)
	story := seedPlatformSourceNetworkVTextFixture(t, handler, "doc-publish-slice")
	ctx := context.Background()

	story, err := handler.rt.Store().GetDocument(ctx, story.DocID, story.OwnerID)
	if err != nil {
		t.Fatalf("reload story document: %v", err)
	}
	rev, err := handler.rt.Store().GetRevision(ctx, story.CurrentRevisionID, story.OwnerID)
	if err != nil {
		t.Fatalf("load story revision: %v", err)
	}

	rec := &types.RunRecord{
		OwnerID: universalWirePlatformOwnerID(),
		RunID:   "run-publish-slice",
		Metadata: map[string]any{
			"type":           "vtext_agent_revision",
			"request_intent": "universal_wire_processor_article_revision",
		},
	}
	handler.rt.wirePlatformPublisher = func(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) (*wirepublish.PublishVTextResponse, error) {
		return &wirepublish.PublishVTextResponse{
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
	if !strings.Contains(editionRev.Content, "vtext:"+story.DocID) {
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
	ref, _ := meta["platformd_publication_ref"].(map[string]any)
	if ref == nil || ref["route_path"] != "wire/madrid-dispatch" {
		t.Fatalf("expected platformd_publication_ref on revision metadata, got %+v", meta)
	}
}

func TestWirePlatformPublishFailsClosedWithoutEditionWhenPlatformdFails(t *testing.T) {
	_, handler := testAPISetup(t)
	seedUniversalWireEditionFixture(t, handler)
	story := seedPlatformSourceNetworkVTextFixture(t, handler, "doc-publish-fail")
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
	rec := &types.RunRecord{
		OwnerID: universalWirePlatformOwnerID(),
		RunID:   "run-publish-fail",
		Metadata: map[string]any{
			"request_intent": "universal_wire_processor_article_revision",
		},
	}
	handler.rt.wirePlatformPublisher = func(ctx context.Context, doc types.Document, rev types.Revision, rec *types.RunRecord) (*wirepublish.PublishVTextResponse, error) {
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
	if strings.Contains(editionRev.Content, "vtext:"+story.DocID) {
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
		Title:     "Seed brief only.vtext",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := handler.rt.Store().CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create story document: %v", err)
	}
	seedMeta, _ := json.Marshal(map[string]any{
		"source":                         "edit_vtext",
		"revision_role":                  vtextRevisionRoleInput,
		"input_origin":                   vtextInputOriginProcessorHandoff,
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
			"type":           "vtext_agent_revision",
			"request_intent":   "universal_wire_processor_article_revision",
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
