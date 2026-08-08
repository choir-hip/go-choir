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
	"time"

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
	now := time.Now().UTC()
	producer := types.AgentRecord{
		AgentID: "researcher:observation", OwnerID: start.OwnerID, ComputerID: start.ComputerID, SandboxID: start.ComputerID,
		Profile: "researcher", Role: "researcher", ChannelID: start.InitialDocument.DocID, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.UpsertAgent(context.Background(), producer); err != nil {
		t.Fatalf("upsert producer: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "open-observation-producer-work", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{WorkItemID: "work-observation-producer", Objective: "produce observed source update", AuthorityProfile: "researcher", AssignedAgentID: producer.AgentID},
	}
	open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
	opened, err := s.OpenLifecycleWork(context.Background(), open)
	if err != nil || opened.WorkItem == nil {
		t.Fatalf("open producer work: %+v, %v", opened.WorkItem, err)
	}
	producerRun := types.RunRecord{
		RunID: "run-observation-producer", AgentID: producer.AgentID, AgentProfile: producer.Profile, AgentRole: producer.Role,
		ChannelID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID, OwnerID: start.OwnerID, SandboxID: start.ComputerID,
		State: types.RunRunning, CreatedAt: now, UpdatedAt: now, Metadata: map[string]any{"lifecycle_work_item_id": opened.WorkItem.WorkItemID},
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "project-observation-producer",
		TrajectoryID: start.TrajectoryID, AgentID: producer.AgentID, Run: producerRun,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(context.Background(), project); err != nil {
		t.Fatalf("project producer: %v", err)
	}
	queue := types.QueueLifecycleUpdateRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "queue-observation",
		TrajectoryID: start.TrajectoryID, TargetAgentID: start.Agent.AgentID,
		ProducerAgentID: producer.AgentID, ProducerUpdateID: "producer-observation-1",
		UpdateID: "update-observation-1", Packet: packet, Content: "durable source update", PayloadDigest: payloadDigest,
		SourceRunID: producerRun.RunID, ChannelID: start.InitialDocument.DocID, Role: producer.Role,
		WorkItemID: opened.WorkItem.WorkItemID, WorkDisposition: types.WorkItemOpen,
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

func applyObservationTextureTurnWithInbound(t *testing.T, s *store.Store, start types.StartLifecycleRequest, inboundCount int) (types.LifecycleResult, types.ApplyTextureTurnRequest, int64) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	caller := types.RunRecord{
		RunID: "run-observation-texture", AgentID: start.Agent.AgentID, AgentProfile: "texture", AgentRole: "texture",
		ChannelID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID, OwnerID: start.OwnerID, SandboxID: start.ComputerID,
		State: types.RunRunning, CreatedAt: now, UpdatedAt: now, Metadata: map[string]any{"lifecycle_work_item_id": start.InitialWork.WorkItemID},
	}
	projectCaller := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "project-observation-texture",
		TrajectoryID: start.TrajectoryID, AgentID: caller.AgentID, Run: caller,
	}
	projectCaller.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(projectCaller)
	if _, err := s.ReplaceLifecycleActivation(ctx, projectCaller); err != nil {
		t.Fatalf("project Texture caller: %v", err)
	}

	producer := types.AgentRecord{
		AgentID: "researcher:observation-turn", OwnerID: start.OwnerID, ComputerID: start.ComputerID, SandboxID: start.ComputerID,
		Profile: "researcher", Role: "researcher", ChannelID: start.InitialDocument.DocID, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.UpsertAgent(ctx, producer); err != nil {
		t.Fatalf("upsert turn producer: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "open-observation-turn-producer", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-observation-turn-producer", Objective: "produce multiple reports", AuthorityProfile: "researcher",
			AssignedAgentID: producer.AgentID, CreatedByRunID: caller.RunID,
			Details: map[string]any{"requested_by_profile": "texture", "requested_by_agent_id": caller.AgentID, "requested_by_run_id": caller.RunID},
		},
	}
	open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
	opened, err := s.OpenLifecycleWork(ctx, open)
	if err != nil || opened.WorkItem == nil {
		t.Fatalf("open turn producer work: %+v, %v", opened.WorkItem, err)
	}
	producerRun := types.RunRecord{
		RunID: "run-observation-turn-producer", AgentID: producer.AgentID, AgentProfile: "researcher", AgentRole: "researcher",
		ChannelID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID, OwnerID: start.OwnerID, SandboxID: start.ComputerID,
		State: types.RunRunning, CreatedAt: now, UpdatedAt: now, RequestedByRunID: caller.RunID,
		Metadata: map[string]any{"lifecycle_work_item_id": opened.WorkItem.WorkItemID},
	}
	projectProducer := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "project-observation-turn-producer",
		TrajectoryID: start.TrajectoryID, AgentID: producer.AgentID, Run: producerRun,
	}
	projectProducer.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(projectProducer)
	if _, err := s.ReplaceLifecycleActivation(ctx, projectProducer); err != nil {
		t.Fatalf("project turn producer: %v", err)
	}

	inbound := make([]types.TextureTurnInboundDisposition, 0, inboundCount)
	for i := 1; i <= inboundCount; i++ {
		id := strconv.Itoa(i)
		packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "turn report " + id}
		content := "private turn report " + id
		payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
		if err != nil {
			t.Fatal(err)
		}
		queue := types.QueueLifecycleUpdateRequest{
			OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "queue-observation-turn-" + id,
			TrajectoryID: start.TrajectoryID, TargetAgentID: caller.AgentID, ProducerAgentID: producer.AgentID,
			ProducerUpdateID: "producer-observation-turn-" + id, UpdateID: "update-observation-turn-" + id,
			ChannelID: start.InitialDocument.DocID, Role: "researcher", SourceRunID: producerRun.RunID,
			Packet: packet, Content: content, PayloadDigest: payloadDigest, WorkItemID: opened.WorkItem.WorkItemID,
			WorkDisposition: types.WorkItemOpen,
		}
		queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
		if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
			t.Fatalf("queue turn report %d: %v", i, err)
		}
		inbound = append(inbound, types.TextureTurnInboundDisposition{
			TargetAgentID: caller.AgentID, ProducerAgentID: producer.AgentID, ProducerUpdateID: queue.ProducerUpdateID,
			UpdateID: queue.UpdateID, Disposition: types.UpdateIncorporated, ProducerWorkItemID: opened.WorkItem.WorkItemID,
			WorkDisposition: types.WorkItemOpen,
		})
	}
	trajectory, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, caller.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	beforeTurnCursor := trajectory.ReducerSeq
	turn := types.ApplyTextureTurnRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "apply-observation-texture-turn",
		DocumentID: doc.DocID, TrajectoryID: start.TrajectoryID, CallerAgentID: caller.AgentID, CallerRunID: caller.RunID,
		ExpectedLifecycleVersion: trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: agent.LifecycleVersion,
		ExpectedHeadRevisionID: doc.CurrentRevisionID, CallerWorkItemID: start.InitialWork.WorkItemID,
		CallerWorkDisposition: types.WorkItemOpen, Outcome: types.TextureTurnRevision,
		Revision: types.Revision{RevisionID: "revision-observation-turn", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Texture", BodyDoc: observationBodyDoc("one revision from multiple reports")},
		Reason:   "one semantic revision", Inbound: inbound,
	}
	turn.CommandDigest, err = store.ComputeApplyTextureTurnDigest(turn)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ApplyTextureTurn(ctx, turn)
	if err != nil {
		t.Fatalf("apply multi-inbound Texture turn: %v", err)
	}
	if result.Revision == nil {
		t.Fatal("multi-inbound Texture turn returned no revision")
	}
	return result, turn, beforeTurnCursor
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

func TestTextureLifecycleObservationProjectsOneVersionForMultiInboundTurnAcrossResumeAndReplay(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	turn, turnRequest, beforeTurnCursor := applyObservationTextureTurnWithInbound(t, core.Store(), start, 2)
	replay, err := core.Store().ApplyTextureTurn(context.Background(), turnRequest)
	if err != nil || !replay.Replay || replay.Revision == nil || replay.Revision.RevisionID != turn.Revision.RevisionID || len(replay.Events) != len(turn.Events) {
		t.Fatalf("multi-inbound turn replay = %+v, %v", replay, err)
	}
	for i := range turn.Events {
		if replay.Events[i].EventID != turn.Events[i].EventID || replay.Events[i].ReducerSeq != turn.Events[i].ReducerSeq || replay.Events[i].Kind != turn.Events[i].Kind {
			t.Fatalf("turn replay event %d changed: first=%+v replay=%+v", i, turn.Events[i], replay.Events[i])
		}
	}

	firstPath := "/api/texture/documents/doc-observation/events?after=" + strconv.FormatInt(beforeTurnCursor, 10) + "&limit=1"
	first := observationRequest(handler, firstPath, start.OwnerID)
	if first.Code != http.StatusOK {
		t.Fatalf("first turn page status=%d body=%s", first.Code, first.Body.String())
	}
	var firstPage textureDurableEventPage
	if err := json.Unmarshal(first.Body.Bytes(), &firstPage); err != nil {
		t.Fatal(err)
	}
	if len(firstPage.Events) != 1 || firstPage.Events[0].Kind != string(types.LifecycleTextureTurnCommitted) ||
		firstPage.Events[0].EventType != "version" || firstPage.Events[0].RevisionID != turn.Revision.RevisionID ||
		firstPage.Events[0].CommandID != "apply-observation-texture-turn" || firstPage.Events[0].CommandDigest == "" ||
		firstPage.Events[0].Cursor != beforeTurnCursor+1 || firstPage.NextCursor != firstPage.Events[0].Cursor {
		t.Fatalf("canonical turn version page = %+v", firstPage)
	}

	resumePath := "/api/texture/documents/doc-observation/events?after=" + strconv.FormatInt(firstPage.NextCursor, 10) + "&limit=100"
	resumed := observationRequest(handler, resumePath, start.OwnerID)
	replayed := observationRequest(handler, resumePath, start.OwnerID)
	if resumed.Code != http.StatusOK || replayed.Code != http.StatusOK || resumed.Body.String() != replayed.Body.String() {
		t.Fatalf("resumed API replay is unstable: first status=%d body=%s replay status=%d body=%s", resumed.Code, resumed.Body.String(), replayed.Code, replayed.Body.String())
	}
	var resumedPage textureDurableEventPage
	if err := json.Unmarshal(resumed.Body.Bytes(), &resumedPage); err != nil {
		t.Fatal(err)
	}
	if len(resumedPage.Events) != 2 || resumedPage.NextCursor != resumedPage.Watermark {
		t.Fatalf("resumed per-inbound transitions = %+v", resumedPage)
	}
	for i, event := range resumedPage.Events {
		if event.Kind != string(types.LifecycleUpdateApplied) || event.EventType != "lifecycle" || event.RevisionID != "" || event.VersionNumber != nil ||
			event.UpdateID != "update-observation-turn-"+strconv.Itoa(i+1) || event.CommandID != firstPage.Events[0].CommandID || event.CommandDigest != firstPage.Events[0].CommandDigest ||
			event.Cursor != firstPage.NextCursor+int64(i+1) {
			t.Fatalf("resumed inbound transition %d = %+v", i, event)
		}
	}

	full := observationRequest(handler, "/api/texture/documents/doc-observation/events?after="+strconv.FormatInt(beforeTurnCursor, 10)+"&limit=100", start.OwnerID)
	if full.Code != http.StatusOK {
		t.Fatalf("full turn page status=%d body=%s", full.Code, full.Body.String())
	}
	var fullPage textureDurableEventPage
	if err := json.Unmarshal(full.Body.Bytes(), &fullPage); err != nil {
		t.Fatal(err)
	}
	versions := 0
	for _, event := range fullPage.Events {
		if event.EventType == "version" {
			versions++
			if event.RevisionID != turn.Revision.RevisionID || event.Cursor != firstPage.Events[0].Cursor {
				t.Fatalf("unstable full-page version = %+v", event)
			}
		}
	}
	if versions != 1 {
		t.Fatalf("multi-inbound turn projected %d versions, want exactly one: %+v", versions, fullPage.Events)
	}
	if strings.Contains(full.Body.String(), "private turn report") {
		t.Fatalf("public API leaked inbound content: %s", full.Body.String())
	}

	sseRequest := httptest.NewRequest(http.MethodGet, "/api/texture/documents/doc-observation/stream?limit=100&once=1", nil)
	sseRequest.Header.Set("X-Authenticated-User", start.OwnerID)
	sseRequest.Header.Set("Last-Event-ID", strconv.FormatInt(firstPage.NextCursor, 10))
	sse := httptest.NewRecorder()
	handler.HandleTextureRouter(sse, sseRequest)
	if sse.Code != http.StatusOK || strings.Count(sse.Body.String(), "event: texture") != 2 || strings.Contains(sse.Body.String(), `"event_type":"version"`) ||
		!strings.Contains(sse.Body.String(), `"update_id":"update-observation-turn-1"`) || !strings.Contains(sse.Body.String(), `"update_id":"update-observation-turn-2"`) {
		t.Fatalf("resumed SSE replayed version or lost typed inbound causality: status=%d body=%s", sse.Code, sse.Body.String())
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
		Kind: types.LifecycleTextureTurnCommitted, ReducerSeq: 10, CommandID: "turn-command",
		CommandDigest: "sha256:turn", RequestID: "owner-request-causal", ArtifactRefs: []string{start.InitialDocument.DocID, revision.RevisionID},
		Reason: "private actor explanation must not project", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if turn.EventType != "version" || turn.RevisionID != revision.RevisionID || turn.VersionNumber == nil ||
		turn.CommandID != "turn-command" || turn.RequestID != "owner-request-causal" || turn.ControlID != "" {
		t.Fatalf("atomic turn projection = %+v", turn)
	}
	control, err := handler.projectTextureLifecycleEvent(context.Background(), types.Document{
		DocID: start.InitialDocument.DocID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, TrajectoryID: start.TrajectoryID,
	}, types.LifecycleEvent{
		EventID: "turn-command:2", OwnerID: start.OwnerID, ComputerID: start.ComputerID, TrajectoryID: start.TrajectoryID,
		Kind: types.LifecycleControlQueued, ReducerSeq: 11, CommandID: "turn-command", UpdateID: "control-one", RequestID: "owner-request-causal",
		WorkItemID: "target-work", CommandDigest: "sha256:turn", Reason: "private control chatter", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if control.EventType != "control" || control.ControlID != "control-one" || control.UpdateID != "control-one" ||
		control.WorkItemID != "target-work" || control.RequestID != "owner-request-causal" || control.RevisionID != "" {
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

func TestTextureObservationNoChangeTransitionsNeverReprojectOldVersion(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	doc, err := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []types.LifecycleEventKind{types.LifecycleTextureTurnCommitted, types.LifecycleControlDelivered, types.LifecycleOwnerInstructionQueued} {
		event, err := handler.projectTextureLifecycleEvent(t.Context(), doc, types.LifecycleEvent{
			EventID: "transition-" + string(kind), Kind: kind, ReducerSeq: 50, CommandID: "no-change",
			CommandDigest: "sha256:no-change", TrajectoryID: start.TrajectoryID,
			ArtifactRefs: []string{start.InitialDocument.DocID, start.InitialRevision.RevisionID}, CreatedAt: time.Now().UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if event.EventType == "version" || event.RevisionID != "" || event.VersionNumber != nil {
			t.Fatalf("%s duplicated old version: %+v", kind, event)
		}
		if event.Cursor != 50 || event.EventType == "" {
			t.Fatalf("%s lost typed cursor transition: %+v", kind, event)
		}
	}
}
