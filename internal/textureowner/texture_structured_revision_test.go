package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax(t *testing.T) {
	_, handler := testAPISetup(t)

	createDocReq := textureRequest(t, http.MethodPost, "/api/texture/documents", textureCreateDocRequest{
		Title: "Structured API",
	})
	createDocW := httptest.NewRecorder()
	handler.HandleTextureCreateDocument(createDocW, createDocReq)
	if createDocW.Code != http.StatusCreated {
		t.Fatalf("create doc status = %d body=%s", createDocW.Code, createDocW.Body.String())
	}
	var created textureCreateDocResponse
	if err := json.NewDecoder(createDocW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create doc: %v", err)
	}

	bodyDoc, sourceEntities := runtimeStructuredRevisionFixture(t)
	createRevReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+created.DocID+"/revisions", textureCreateRevisionRequest{
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
	})
	createRevW := httptest.NewRecorder()
	handler.HandleTextureRevisions(createRevW, createRevReq)
	if createRevW.Code != http.StatusCreated {
		t.Fatalf("create structured revision status = %d body=%s", createRevW.Code, createRevW.Body.String())
	}
	var rev textureRevisionResponse
	if err := json.NewDecoder(createRevW.Body).Decode(&rev); err != nil {
		t.Fatalf("decode revision: %v", err)
	}
	if rev.Content != "Grounded[1]." {
		t.Fatalf("Content = %q, want derived projection", rev.Content)
	}
	if len(rev.BodyDoc) == 0 || len(rev.SourceEntities) == 0 {
		t.Fatalf("structured fields missing from response: body_doc=%s source_entities=%s", rev.BodyDoc, rev.SourceEntities)
	}
	if !strings.HasPrefix(rev.RevisionHash, types.StructuredRevisionHashScheme+":") {
		t.Fatalf("RevisionHash = %q, want structured prefix", rev.RevisionHash)
	}

	legacyReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+created.DocID+"/revisions", textureCreateRevisionRequest{
		Content:          "Bad {{source:legacy}} token",
		ParentRevisionID: rev.RevisionID,
	})
	legacyW := httptest.NewRecorder()
	handler.HandleTextureRevisions(legacyW, legacyReq)
	if legacyW.Code != http.StatusBadRequest {
		t.Fatalf("legacy syntax status = %d body=%s", legacyW.Code, legacyW.Body.String())
	}
}

func runtimeStructuredRevisionFixture(t *testing.T) (json.RawMessage, json.RawMessage) {
	t.Helper()
	doc := texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-node"},
			Content: []texturedoc.Node{{
				Type:  "paragraph",
				Attrs: map[string]any{"id": "p-1"},
				Content: []texturedoc.Node{
					{Type: "text", Text: "Grounded"},
					{
						Type: "source_ref",
						Attrs: map[string]any{
							"id":               "ref-1",
							"source_entity_id": "src-web",
							"display_mode":     "numbered_ref",
						},
					},
					{Type: "text", Text: "."},
				},
			}},
		},
	}
	entities := []texturedoc.SourceEntity{{
		SourceEntityID: "src-web",
		Target: texturedoc.SourceTarget{
			Kind: "web_url",
			URI:  "https://example.com/story",
		},
		Selectors: []texturedoc.SourceSelector{{
			Kind: sourcecontract.SelectorKindTextQuote,
			Data: map[string]any{"exact": "Grounded"},
		}},
		Display: texturedoc.SourceDisplay{
			Mode:  "numbered_ref",
			Title: "Example story",
		},
		Evidence: texturedoc.SourceEvidence{
			State:       sourcecontract.EvidenceStateConfirms,
			OpenSurface: sourcecontract.OpenSurfaceSource,
		},
		Provenance: texturedoc.SourceEntityProvenance{
			CreatedBy:    "runtime",
			SourceSystem: "test",
		},
	}}
	bodyDocJSON, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal body doc: %v", err)
	}
	sourceEntitiesJSON, err := json.Marshal(entities)
	if err != nil {
		t.Fatalf("marshal source entities: %v", err)
	}
	return bodyDocJSON, sourceEntitiesJSON
}

func TestDirectOwnerEditCommitsCurrentHeadAndWakesOnce(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	var dispatches []string
	core.SetDispatchActor(func(_ context.Context, _, _, _ string, kind, content, _, _ string) error {
		if kind == "coagent_result" {
			dispatches = append(dispatches, content)
		}
		return nil
	})
	before, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	requestBody := textureCreateRevisionRequest{
		Content: "direct owner correction", ParentRevisionID: start.InitialRevision.RevisionID,
		IdempotencyKey: "direct-owner-edit", ExpectedLifecycleVersion: before.Trajectory.LifecycleVersion,
	}
	request := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", requestBody)
	response := httptest.NewRecorder()
	handler.HandleTextureRevisions(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("direct edit status=%d body=%s", response.Code, response.Body.String())
	}
	var revision textureRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&revision); err != nil {
		t.Fatal(err)
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.HeadRevision.RevisionID != revision.RevisionID || snapshot.HeadRevision.AuthorKind != types.AuthorUser {
		t.Fatalf("direct edit head=%+v", snapshot.HeadRevision)
	}
	pending, _, ok := store.PendingTextureOwnerRevision(snapshot)
	if !ok || pending.RevisionID != revision.RevisionID {
		t.Fatalf("direct edit pending owner revision=%+v pending=%v", pending, ok)
	}
	if len(dispatches) != 1 {
		t.Fatalf("direct edit dispatches=%d", len(dispatches))
	}
	occurrence, err := agentcore.DecodeTextureActorOccurrence(dispatches[0])
	if err != nil || occurrence.Kind != agentcore.TextureActorOccurrenceDocumentRevision || occurrence.HeadRevisionID != revision.RevisionID {
		t.Fatalf("direct edit occurrence=%+v err=%v", occurrence, err)
	}
	replay := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", requestBody)
	replayResponse := httptest.NewRecorder()
	handler.HandleTextureRevisions(replayResponse, replay)
	if replayResponse.Code != http.StatusCreated || len(dispatches) != 1 {
		t.Fatalf("direct edit replay status=%d dispatches=%d body=%s", replayResponse.Code, len(dispatches), replayResponse.Body.String())
	}
}

func TestTextureRevisionAPICommitsLifecycleBoundHeadThroughReducer(t *testing.T) {
	rt, handler := testAPISetup(t)
	var wakes int
	rt.SetDispatchActor(func(_ context.Context, _, _, _ string, kind, _ string, _, _ string) error {
		if kind == "coagent_result" {
			wakes++
		}
		return nil
	})
	start := types.StartLifecycleRequest{
		OwnerID: "user-1", ComputerID: rt.TextureComputerID(), CommandID: "start-public-revision",
		TrajectoryID: "trajectory-public-revision", Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://document/public-revision", "doc_id": "document-public-revision"},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-public-revision", Objective: "revise artifact"},
		InitialDocument: types.Document{DocID: "document-public-revision", Title: "Public lifecycle revision"},
		InitialRevision: types.Revision{RevisionID: "revision-public-v0", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", BodyDoc: observationBodyDoc("Initial")},
		Agent:           types.AgentRecord{AgentID: "texture:document-public-revision", Profile: "texture", Role: "texture", ChannelID: "document-public-revision"},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := rt.Store().StartLifecycle(t.Context(), start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	getRequest := textureRequest(t, http.MethodGet, "/api/texture/documents/"+start.InitialDocument.DocID, nil)
	getResponse := httptest.NewRecorder()
	handler.handleTextureGetDocument(getResponse, getRequest, start.InitialDocument.DocID)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get lifecycle document status = %d body=%s", getResponse.Code, getResponse.Body.String())
	}
	var document textureDocumentResponse
	if err := json.NewDecoder(getResponse.Body).Decode(&document); err != nil ||
		document.TrajectoryID != start.TrajectoryID {
		t.Fatalf("lifecycle document response omitted authority: %+v, %v", document, err)
	}
	updateRequest := textureRequest(t, http.MethodPut, "/api/texture/documents/"+start.InitialDocument.DocID, textureUpdateDocRequest{
		Title: "Renamed lifecycle artifact",
	})
	updateResponse := httptest.NewRecorder()
	handler.handleTextureUpdateDocument(updateResponse, updateRequest, start.InitialDocument.DocID)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("rename lifecycle document status = %d body=%s", updateResponse.Code, updateResponse.Body.String())
	}
	var renamed textureDocumentResponse
	if err := json.NewDecoder(updateResponse.Body).Decode(&renamed); err != nil || renamed.Title != "Renamed lifecycle artifact" {
		t.Fatalf("unexpected renamed lifecycle document: %+v, %v", renamed, err)
	}
	renamedSnapshot, err := rt.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("get renamed lifecycle snapshot: %v", err)
	}
	if renamedSnapshot.Document.Title != renamed.Title ||
		renamedSnapshot.Document.CurrentRevisionID != start.InitialRevision.RevisionID ||
		renamedSnapshot.Trajectory.ReducerSeq != 1 ||
		renamedSnapshot.Trajectory.LifecycleVersion != 1 {
		t.Fatalf("title projection mutated lifecycle authority: %+v", renamedSnapshot)
	}
	bodyDoc, sourceEntities := runtimeStructuredRevisionFixture(t)
	request := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", textureCreateRevisionRequest{
		BodyDoc: bodyDoc, SourceEntities: sourceEntities, ParentRevisionID: start.InitialRevision.RevisionID,
		IdempotencyKey: "public-revision-command", ExpectedLifecycleVersion: 1,
	})
	response := httptest.NewRecorder()
	handler.HandleTextureRevisions(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create lifecycle revision status = %d body=%s", response.Code, response.Body.String())
	}
	var revision textureRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&revision); err != nil {
		t.Fatalf("decode lifecycle revision: %v", err)
	}
	snapshot, err := rt.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("get lifecycle snapshot: %v", err)
	}
	if snapshot.HeadRevision.RevisionID != revision.RevisionID || snapshot.HeadRevision.Content == "" ||
		snapshot.Trajectory.ReducerSeq != 2 || snapshot.Trajectory.LifecycleVersion != 2 {
		t.Fatalf("unexpected lifecycle revision snapshot: %+v", snapshot)
	}
	pending, _, pendingOwnerRevision := store.PendingTextureOwnerRevision(snapshot)
	if !pendingOwnerRevision || pending.RevisionID != revision.RevisionID {
		t.Fatalf("pending owner revision=%+v pending=%v", pending, pendingOwnerRevision)
	}
	refs, refsErr := rt.Store().ListTextureSourceRefsForRevisionByScope(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID, revision.RevisionID)
	entities, entitiesErr := rt.Store().ListTextureSourceEntitiesForRevisionByScope(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID, revision.RevisionID)
	if refsErr != nil || entitiesErr != nil || len(refs) != 1 || len(entities) != 1 || wakes != 1 {
		t.Fatalf("joined source graph/wake refs=%+v entities=%+v wakes=%d errors=%v/%v", refs, entities, wakes, refsErr, entitiesErr)
	}
	replay := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", textureCreateRevisionRequest{
		BodyDoc: bodyDoc, SourceEntities: sourceEntities, ParentRevisionID: start.InitialRevision.RevisionID,
		IdempotencyKey: "public-revision-command", ExpectedLifecycleVersion: 1,
	})
	replayResponse := httptest.NewRecorder()
	handler.HandleTextureRevisions(replayResponse, replay)
	if replayResponse.Code != http.StatusCreated {
		t.Fatalf("replay lifecycle revision status = %d body=%s", replayResponse.Code, replayResponse.Body.String())
	}
	var replayed textureRevisionResponse
	if err := json.NewDecoder(replayResponse.Body).Decode(&replayed); err != nil || replayed.RevisionID != revision.RevisionID {
		t.Fatalf("unexpected lifecycle revision replay: %+v, %v", replayed, err)
	}
	if wakes != 1 {
		t.Fatalf("replay emitted Texture wake: %d", wakes)
	}
	conflictRequest := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", textureCreateRevisionRequest{
		Content: "conflicting reuse", ParentRevisionID: start.InitialRevision.RevisionID,
		IdempotencyKey: "public-revision-command", ExpectedLifecycleVersion: 1,
	})
	conflictResponse := httptest.NewRecorder()
	handler.HandleTextureRevisions(conflictResponse, conflictRequest)
	if conflictResponse.Code != http.StatusConflict || wakes != 1 {
		t.Fatalf("conflicting occurrence status=%d wakes=%d body=%s", conflictResponse.Code, wakes, conflictResponse.Body.String())
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "cancel-public-revision",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: revision.RevisionID, Reason: "finish public lifecycle",
	}
	cancel.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancel)
	cancelled, err := rt.Store().CancelLifecycleTrajectory(t.Context(), cancel)
	if err != nil {
		t.Fatalf("cancel lifecycle: %v", err)
	}
	postTerminalRequest := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", textureCreateRevisionRequest{
		Content: "Independent post-terminal edit", ParentRevisionID: revision.RevisionID,
		IdempotencyKey: "public-unbound-revision-command", ExpectedLifecycleVersion: cancelled.Trajectory.LifecycleVersion,
	})
	postTerminalResponse := httptest.NewRecorder()
	handler.HandleTextureRevisions(postTerminalResponse, postTerminalRequest)
	if postTerminalResponse.Code != http.StatusCreated {
		t.Fatalf("create post-terminal revision status = %d body=%s", postTerminalResponse.Code, postTerminalResponse.Body.String())
	}
	var postTerminalRevision textureRevisionResponse
	if err := json.NewDecoder(postTerminalResponse.Body).Decode(&postTerminalRevision); err != nil {
		t.Fatalf("decode post-terminal revision: %v", err)
	}
	terminalSnapshot, err := rt.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("get terminal lifecycle snapshot: %v", err)
	}
	if terminalSnapshot.HeadRevision.RevisionID != revision.RevisionID ||
		terminalSnapshot.Document.CurrentRevisionID != postTerminalRevision.RevisionID ||
		terminalSnapshot.CurrentDocumentHead == nil ||
		terminalSnapshot.CurrentDocumentHead.RevisionID != postTerminalRevision.RevisionID ||
		terminalSnapshot.Trajectory.TerminalArtifactHeadRef != revision.RevisionID {
		t.Fatalf("post-terminal document edit moved accepted lifecycle head: %+v", terminalSnapshot)
	}
	deleteRequest := textureRequest(t, http.MethodDelete, "/api/texture/documents/"+start.InitialDocument.DocID, nil)
	deleteResponse := httptest.NewRecorder()
	handler.handleTextureDeleteDocument(deleteResponse, deleteRequest, start.InitialDocument.DocID)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("archive lifecycle document status = %d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
}
