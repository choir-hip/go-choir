package textureowner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestTextureRevisionAPICommitsLifecycleBoundHeadThroughReducer(t *testing.T) {
	rt, handler := testAPISetup(t)
	start := types.StartLifecycleRequest{
		OwnerID: "user-1", ComputerID: rt.TextureSandboxID(), CommandID: "start-public-revision",
		TrajectoryID: "trajectory-public-revision", Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://document/public-revision", "doc_id": "document-public-revision"},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-public-revision", Objective: "revise artifact"},
		InitialDocument: types.Document{DocID: "document-public-revision", Title: "Public lifecycle revision"},
		InitialRevision: types.Revision{RevisionID: "revision-public-v0", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", Content: "Initial"},
		Agent:           types.AgentRecord{AgentID: "texture:document-public-revision", Profile: "texture", Role: "texture", ChannelID: "document-public-revision"},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := rt.Store().StartLifecycle(t.Context(), start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	request := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", textureCreateRevisionRequest{
		Content: "Owner-authored", ParentRevisionID: start.InitialRevision.RevisionID,
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
	if snapshot.HeadRevision.RevisionID != revision.RevisionID || snapshot.HeadRevision.Content != "Owner-authored" ||
		snapshot.Trajectory.ReducerSeq != 2 || snapshot.Trajectory.LifecycleVersion != 2 {
		t.Fatalf("unexpected lifecycle revision snapshot: %+v", snapshot)
	}
	replay := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", textureCreateRevisionRequest{
		Content: "Owner-authored", ParentRevisionID: start.InitialRevision.RevisionID,
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
}
