package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func observationBodyDoc(text string) json.RawMessage {
	payload, _ := json.Marshal(map[string]any{
		"schema": "choir.texture_doc.v1",
		"doc": map[string]any{"type": "doc", "attrs": map[string]any{"id": "doc-root"}, "content": []any{
			map[string]any{"type": "paragraph", "attrs": map[string]any{"id": "p-root"}, "content": []any{map[string]any{"type": "text", "text": text}}},
		}},
	})
	return payload
}

func startObservationLifecycle(t *testing.T, s *store.Store) types.StartLifecycleRequest {
	t.Helper()
	req := types.StartLifecycleRequest{
		OwnerID: "user-1", ComputerID: "sandbox-test", CommandID: "start-observation",
		TrajectoryID: "trajectory-observation", Kind: types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://document/doc-observation", "doc_id": "doc-observation"},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-observation", Objective: "observe durable Texture versions"},
		InitialDocument: types.Document{DocID: "doc-observation", Title: "Observed Texture"},
		InitialRevision: types.Revision{RevisionID: "revision-observation-0", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Texture", BodyDoc: observationBodyDoc("initial")},
		Agent:           types.AgentRecord{AgentID: "texture:doc-observation", Profile: "texture", Role: "texture", ChannelID: "doc-observation"},
	}
	req.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	return req
}

func applyObservationSourceVersion(t *testing.T, s *store.Store, start types.StartLifecycleRequest) (types.Revision, store.TextureSourceGraphWriteSet) {
	t.Helper()
	packet := types.CoagentSourcePacketPayload{SchemaVersion: "v1", Kind: "result", Summary: "durable source update"}
	payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, "durable source update")
	if err != nil {
		t.Fatalf("payload digest: %v", err)
	}
	queue := types.QueueLifecycleUpdateRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "queue-observation",
		TrajectoryID: start.TrajectoryID, TargetAgentID: start.Agent.AgentID,
		ProducerAgentID: "researcher:observation", ProducerUpdateID: "producer-observation-1",
		UpdateID: "update-observation-1", Packet: packet, Content: "durable source update", PayloadDigest: payloadDigest,
	}
	queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s.QueueLifecycleUpdate(context.Background(), queue); err != nil {
		t.Fatalf("queue update: %v", err)
	}
	bodyDoc, sourceEntities := structuredTextureToolPayload(t)
	revision := types.Revision{
		RevisionID: "revision-observation-1", DocID: start.InitialDocument.DocID,
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, TrajectoryID: start.TrajectoryID,
		AuthorKind: types.AuthorAppAgent, AuthorLabel: "Texture", BodyDoc: bodyDoc, SourceEntities: sourceEntities,
	}
	graph, err := textureToolSourceGraphWriteSet(revision, materializedTextureEdit{BodyDoc: bodyDoc, SourceEntities: sourceEntities}, &types.RunRecord{
		RunID: "run-observation", OwnerID: start.OwnerID, SandboxID: start.ComputerID, TrajectoryID: start.TrajectoryID,
	})
	if err != nil {
		t.Fatalf("build source graph: %v", err)
	}
	apply := types.ApplyLifecycleUpdateRequest(queue)
	apply.CommandID = "apply-observation"
	apply.Disposition, apply.DispositionRef = types.UpdateIncorporated, revision.RevisionID
	apply.Revision = revision
	apply.CommandDigest, _ = store.ComputeApplyLifecycleUpdateWithSourceGraphDigest(apply, graph)
	result, err := s.ApplyLifecycleUpdateWithSourceGraph(context.Background(), apply, graph)
	if err != nil {
		t.Fatalf("apply source update: %v", err)
	}
	if result.Revision == nil {
		t.Fatal("apply returned no revision")
	}
	storedEntities, err := s.ListTextureSourceEntitiesForRevisionByScope(context.Background(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID, result.Revision.RevisionID)
	if err != nil {
		t.Fatalf("load stored source entities: %v", err)
	}
	storedRefs, err := s.ListTextureSourceRefsForRevisionByScope(context.Background(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID, result.Revision.RevisionID)
	if err != nil {
		t.Fatalf("load stored source refs: %v", err)
	}
	graph.SourceEntities, graph.SourceRefs = storedEntities, storedRefs
	return *result.Revision, graph
}

func observationRequest(handler *Handler, path, user string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if user != "" {
		request.Header.Set("X-Authenticated-User", user)
	}
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, request)
	return response
}

func TestTextureLifecycleObservationDurablePageResumeAndExactScope(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())

	first := observationRequest(handler, "/api/texture/documents/doc-observation/events?after=0&limit=1", start.OwnerID)
	if first.Code != http.StatusOK {
		t.Fatalf("first page status=%d body=%s", first.Code, first.Body.String())
	}
	var firstPage textureDurableEventPage
	if err := json.Unmarshal(first.Body.Bytes(), &firstPage); err != nil {
		t.Fatal(err)
	}
	if firstPage.Schema != textureObservationSchemaV1 || firstPage.DocID != start.InitialDocument.DocID ||
		firstPage.OwnerID != start.OwnerID || firstPage.ComputerID != start.ComputerID || firstPage.TrajectoryID != start.TrajectoryID ||
		len(firstPage.Events) != 1 || firstPage.NextCursor != firstPage.Events[0].Cursor || firstPage.Events[0].RevisionID != start.InitialRevision.RevisionID ||
		firstPage.Events[0].VersionNumber == nil || *firstPage.Events[0].VersionNumber != 0 || firstPage.Events[0].EventType != "version" {
		t.Fatalf("unexpected first durable page: %+v", firstPage)
	}

	revision, graph := applyObservationSourceVersion(t, core.Store(), start)
	resumed := observationRequest(handler, "/api/texture/documents/doc-observation/events?after="+strconv.FormatInt(firstPage.NextCursor, 10)+"&limit=100", start.OwnerID)
	if resumed.Code != http.StatusOK {
		t.Fatalf("resumed page status=%d body=%s", resumed.Code, resumed.Body.String())
	}
	var page textureDurableEventPage
	if err := json.Unmarshal(resumed.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(resumed.Body.String(), "durable source update") {
		t.Fatalf("public observation leaked raw actor packet chatter: %s", resumed.Body.String())
	}
	if page.NextCursor <= firstPage.NextCursor || page.Watermark != page.NextCursor || len(page.Events) < 2 {
		t.Fatalf("resumed page did not include mutations committed after first page: %+v", page)
	}
	var version *textureDurableEvent
	for i := range page.Events {
		if page.Events[i].RevisionID == revision.RevisionID {
			version = &page.Events[i]
			break
		}
	}
	if version == nil {
		t.Fatal("projected page has no applied version")
	}
	if version.EventType != "version" || version.ParentRevisionID != start.InitialRevision.RevisionID ||
		version.VersionNumber == nil || *version.VersionNumber != 1 || version.WorkState != "working" ||
		version.RequestID != "" || version.CommandID != "apply-observation" || version.UpdateID != "update-observation-1" || version.ControlID != "" ||
		len(version.SourceIdentities) != 1 {
		versionNumber := -1
		if version.VersionNumber != nil {
			versionNumber = *version.VersionNumber
		}
		t.Fatalf("projected source-aware version = %+v; version=%d parent=%q initial=%q", version, versionNumber, version.ParentRevisionID, start.InitialRevision.RevisionID)
	}
	identity := version.SourceIdentities[0]
	if identity.SourceRefCanonicalID != graph.SourceRefs[0].CanonicalID || identity.SourceRefVersionID != graph.SourceRefs[0].VersionID ||
		identity.SourceEntityCanonicalID != graph.SourceEntities[0].CanonicalID || identity.SourceEntityVersionID != graph.SourceEntities[0].VersionID ||
		identity.SourceEntityHash == "" || identity.SourceRefHash == "" || identity.DisplayMode == "" || identity.BodyNodeID == "" || identity.BodyNodePathHash == "" || len(identity.Selectors) == 0 || identity.OpenSurface == "" || !strings.Contains(identity.OpenPath, "/source-open?") {
		t.Fatalf("source identity = %+v", identity)
	}

	unauthenticated := observationRequest(handler, "/api/texture/documents/doc-observation/events", "")
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", unauthenticated.Code)
	}
	crossOwner := observationRequest(handler, "/api/texture/documents/doc-observation/events", "user-2")
	if crossOwner.Code != http.StatusNotFound {
		t.Fatalf("cross-owner status=%d body=%s", crossOwner.Code, crossOwner.Body.String())
	}
	expired := observationRequest(handler, "/api/texture/documents/doc-observation/events?after=999999", start.OwnerID)
	if expired.Code != http.StatusConflict {
		t.Fatalf("expired status=%d body=%s", expired.Code, expired.Body.String())
	}
	var expiredPage textureDurableEventPage
	_ = json.Unmarshal(expired.Body.Bytes(), &expiredPage)
	if !expiredPage.CursorExpired || !expiredPage.ReplayRequired || expiredPage.Watermark != page.Watermark {
		t.Fatalf("expired page = %+v", expiredPage)
	}

	sseRequest := httptest.NewRequest(http.MethodGet, "/api/texture/documents/doc-observation/stream?limit=100&once=1", nil)
	sseRequest.Header.Set("X-Authenticated-User", start.OwnerID)
	sseRequest.Header.Set("Last-Event-ID", strconv.FormatInt(firstPage.NextCursor, 10))
	sse := httptest.NewRecorder()
	handler.HandleTextureRouter(sse, sseRequest)
	if sse.Code != http.StatusOK || !strings.Contains(sse.Header().Get("Content-Type"), "text/event-stream") ||
		!strings.Contains(sse.Body.String(), "event: texture") || strings.Contains(sse.Body.String(), `"revision_id":"revision-observation-0"`) {
		t.Fatalf("Last-Event-ID resumed SSE status=%d headers=%v body=%s", sse.Code, sse.Header(), sse.Body.String())
	}
	if strings.Count(sse.Body.String(), "id: "+strconv.FormatInt(firstPage.NextCursor, 10)+"\n") != 0 {
		t.Fatalf("Last-Event-ID replayed acknowledged cursor: %s", sse.Body.String())
	}
}

func TestTextureLifecycleProjectionJoinsAtomicTurnAndControlIdentities(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	revision, _ := applyObservationSourceVersion(t, core.Store(), start)
	now := revision.CreatedAt
	turn, err := handler.projectTextureLifecycleEvent(context.Background(), types.Document{
		DocID: start.InitialDocument.DocID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, TrajectoryID: start.TrajectoryID,
	}, types.LifecycleEvent{
		EventID: "turn-command:1", OwnerID: start.OwnerID, ComputerID: start.ComputerID, TrajectoryID: start.TrajectoryID,
		Kind: types.LifecycleEventKind("texture_turn_committed"), ReducerSeq: 10, CommandID: "turn-command",
		CommandDigest: "sha256:turn", ArtifactRefs: []string{start.InitialDocument.DocID, revision.RevisionID},
		Reason: "private actor explanation must not project", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if turn.EventType != "version" || turn.RevisionID != revision.RevisionID || turn.VersionNumber == nil ||
		turn.CommandID != "turn-command" || turn.RequestID != "" || turn.ControlID != "" {
		t.Fatalf("atomic turn projection = %+v", turn)
	}
	control, err := handler.projectTextureLifecycleEvent(context.Background(), types.Document{
		DocID: start.InitialDocument.DocID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, TrajectoryID: start.TrajectoryID,
	}, types.LifecycleEvent{
		EventID: "turn-command:2", OwnerID: start.OwnerID, ComputerID: start.ComputerID, TrajectoryID: start.TrajectoryID,
		Kind: types.LifecycleEventKind("control_queued"), ReducerSeq: 11, CommandID: "turn-command", UpdateID: "control-one",
		WorkItemID: "target-work", CommandDigest: "sha256:turn", Reason: "private control chatter", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if control.EventType != "control" || control.ControlID != "control-one" || control.UpdateID != "control-one" ||
		control.WorkItemID != "target-work" || control.RequestID != "" || control.RevisionID != "" {
		t.Fatalf("control projection = %+v", control)
	}
	payload, _ := json.Marshal([]textureDurableEvent{turn, control})
	if strings.Contains(string(payload), "private actor explanation") || strings.Contains(string(payload), "private control chatter") {
		t.Fatalf("projection leaked raw actor chatter: %s", payload)
	}
}

func TestTextureSourceOpenPinsExactReferenceEntitySelectorAndHash(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	revision, graph := applyObservationSourceVersion(t, core.Store(), start)
	identity := textureSourceIdentityFromRecords(revision, graph.SourceRefs[0], graph.SourceEntities[0])

	opened := observationRequest(handler, identity.OpenPath, start.OwnerID)
	if opened.Code != http.StatusOK {
		t.Fatalf("source open status=%d body=%s", opened.Code, opened.Body.String())
	}
	var response textureSourceOpenResponse
	if err := json.Unmarshal(opened.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Schema != textureSourceOpenSchemaV1 || response.DocID != revision.DocID || response.RevisionID != revision.RevisionID ||
		response.SourceIdentity.SourceRefCanonicalID != graph.SourceRefs[0].CanonicalID ||
		response.SourceIdentity.SourceRefVersionID != graph.SourceRefs[0].VersionID ||
		response.SourceIdentity.SourceEntityCanonicalID != graph.SourceEntities[0].CanonicalID ||
		response.SourceIdentity.SourceEntityVersionID != graph.SourceEntities[0].VersionID ||
		response.SourceIdentity.SourceEntityHash != graph.SourceEntities[0].ContentHash || response.SourceIdentity.SourceRefHash != graph.SourceRefs[0].ContentHash || len(response.SourceIdentity.Selectors) == 0 ||
		response.SourceIdentity.OpenPath != identity.OpenPath {
		t.Fatalf("exact source open response = %+v", response)
	}
	for name, path := range map[string]string{
		"wrong ref version": strings.Replace(identity.OpenPath, graph.SourceRefs[0].VersionID, "wrong-version", 1),
		"wrong revision":    strings.Replace(identity.OpenPath, revision.RevisionID, start.InitialRevision.RevisionID, 1),
	} {
		if rejected := observationRequest(handler, path, start.OwnerID); rejected.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%s", name, rejected.Code, rejected.Body.String())
		}
	}
	if rejected := observationRequest(handler, identity.OpenPath, "user-2"); rejected.Code != http.StatusNotFound {
		t.Fatalf("cross-owner source open status=%d body=%s", rejected.Code, rejected.Body.String())
	}
}

func TestTextureLifecycleObservationSurvivesStoreRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	first, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	start := startObservationLifecycle(t, first)
	applyObservationSourceVersion(t, first, start)
	doc, err := first.GetLifecycleDocument(context.Background(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	handler := &Handler{Store: second}
	page, err := handler.textureLifecycleEventPage(context.Background(), doc, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Events) < 3 || page.NextCursor != page.Watermark {
		t.Fatalf("restarted durable projection = %+v", page)
	}
	for _, event := range page.Events {
		if event.RevisionID == "revision-observation-1" && len(event.SourceIdentities) == 1 {
			return
		}
	}
	t.Fatalf("restarted projection lost source-aware version: %+v", page)
}

func TestTextureLifecycleObservationRejectsWrongComputerScope(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	doc, err := core.Store().GetLifecycleDocument(context.Background(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	doc.ComputerID = "different-computer"
	_, err = handler.textureLifecycleEventPage(context.Background(), doc, 0, 100)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong computer page err=%v, want scope refusal", err)
	}
}
