package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func postOwnerInstruction(t *testing.T, handler *Handler, path, owner, requestID, content, head string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"client_request_id": requestID, "content": content, "expected_head_revision_id": head})
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	if owner != "" {
		req.Header.Set("X-Authenticated-User", owner)
	}
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, req)
	return response
}

func TestTextureOwnerInstructionAPIAuthOccurrenceReplayConflictAndPrivateProjection(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	path := "/api/texture/documents/" + start.InitialDocument.DocID + "/tell"
	if got := postOwnerInstruction(t, handler, path, "", "client-one", "keep this private", start.InitialRevision.RevisionID); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", got.Code)
	}
	if got := postOwnerInstruction(t, handler, path, "other-owner", "client-one", "keep this private", start.InitialRevision.RevisionID); got.Code != http.StatusNotFound {
		t.Fatalf("cross-owner status=%d body=%s", got.Code, got.Body.String())
	}
	first := postOwnerInstruction(t, handler, path, start.OwnerID, "client-one", "keep this private", start.InitialRevision.RevisionID)
	if first.Code != http.StatusAccepted || strings.Contains(first.Body.String(), "keep this private") {
		t.Fatalf("first instruction status=%d body=%s", first.Code, first.Body.String())
	}
	var firstResult textureOwnerInstructionResponse
	if err := json.Unmarshal(first.Body.Bytes(), &firstResult); err != nil || firstResult.Schema != textureOwnerInstructionSchemaV1 || firstResult.Replay ||
		firstResult.RequestID == "client-one" || firstResult.InstructionID == "" || firstResult.TargetWorkItemID != start.InitialWork.WorkItemID {
		t.Fatalf("first result=%+v err=%v", firstResult, err)
	}
	afterFirst, _ := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	replay := postOwnerInstruction(t, handler, path, start.OwnerID, "client-one", "keep this private", start.InitialRevision.RevisionID)
	var replayResult textureOwnerInstructionResponse
	_ = json.Unmarshal(replay.Body.Bytes(), &replayResult)
	if replay.Code != http.StatusAccepted || !replayResult.Replay || replayResult.InstructionID != firstResult.InstructionID {
		t.Fatalf("replay status=%d result=%+v body=%s", replay.Code, replayResult, replay.Body.String())
	}
	afterReplay, _ := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if afterReplay.SnapshotCursor != afterFirst.SnapshotCursor {
		t.Fatalf("instruction replay woke or mutated lifecycle: %d -> %d", afterFirst.SnapshotCursor, afterReplay.SnapshotCursor)
	}
	conflict := postOwnerInstruction(t, handler, path, start.OwnerID, "client-one", "changed", start.InitialRevision.RevisionID)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}
	second := postOwnerInstruction(t, handler, path, start.OwnerID, "client-two", "keep this private", start.InitialRevision.RevisionID)
	var secondResult textureOwnerInstructionResponse
	_ = json.Unmarshal(second.Body.Bytes(), &secondResult)
	if second.Code != http.StatusAccepted || secondResult.InstructionID == firstResult.InstructionID || secondResult.Replay {
		t.Fatalf("distinct occurrence status=%d result=%+v", second.Code, secondResult)
	}
	correct := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/correct", start.OwnerID, "client-correct", "replace prior direction", start.InitialRevision.RevisionID)
	var correctResult textureOwnerInstructionResponse
	_ = json.Unmarshal(correct.Body.Bytes(), &correctResult)
	if correct.Code != http.StatusAccepted || correctResult.Kind != "correct" {
		t.Fatalf("correct status=%d result=%+v", correct.Code, correctResult)
	}

	events := observationRequest(handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/events?after=0&limit=100", start.OwnerID)
	if events.Code != http.StatusOK || strings.Contains(events.Body.String(), "keep this private") || strings.Contains(events.Body.String(), "replace prior direction") ||
		!strings.Contains(events.Body.String(), firstResult.RequestID) || !strings.Contains(events.Body.String(), `"kind":"owner_instruction_queued"`) {
		t.Fatalf("private causal event page status=%d body=%s", events.Code, events.Body.String())
	}
}

func TestLifecycleReviseRoutesToOwnerInstructionNotProducerMailbox(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	body, _ := json.Marshal(map[string]string{
		"prompt": "revise privately", "client_request_id": "revise-occurrence", "expected_head_revision_id": start.InitialRevision.RevisionID,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", strings.NewReader(string(body)))
	req.Header.Set("X-Authenticated-User", start.OwnerID)
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, req)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), textureOwnerInstructionSchemaV1) || strings.Contains(response.Body.String(), "revise privately") {
		t.Fatalf("lifecycle revise status=%d body=%s", response.Code, response.Body.String())
	}
	pendingOwner, err := core.Store().ListPendingLifecycleOwnerInstructions(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, 10)
	if err != nil || len(pendingOwner) != 1 || pendingOwner[0].Content != "revise privately" {
		t.Fatalf("owner instruction pending=%+v err=%v", pendingOwner, err)
	}
	producerReports, err := core.Store().ListPendingLifecycleUpdates(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID, 10)
	if err != nil || len(producerReports) != 0 {
		t.Fatalf("lifecycle revise entered producer mailbox: %+v err=%v", producerReports, err)
	}
}

func TestTextureLifecycleCreateExactReplayAndChangedPayloadConflict(t *testing.T) {
	core, handler := testAPISetup(t)
	post := func(owner, title, content string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"client_request_id": "create-occurrence", "title": title, "initial_content": content})
		req := httptest.NewRequest(http.MethodPost, "/api/texture/lifecycle-documents", strings.NewReader(string(body)))
		if owner != "" {
			req.Header.Set("X-Authenticated-User", owner)
		}
		response := httptest.NewRecorder()
		handler.HandleTextureRouter(response, req)
		return response
	}
	if got := post("", "Title", "private initial"); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauth create=%d", got.Code)
	}
	first := post("user-1", "Title", "private initial")
	if first.Code != http.StatusCreated || strings.Contains(first.Body.String(), "private initial") {
		t.Fatalf("first create=%d %s", first.Code, first.Body.String())
	}
	var created textureLifecycleCreateResponse
	_ = json.Unmarshal(first.Body.Bytes(), &created)
	if created.Schema != "choir.texture_create.v1" || created.DocID == "" || created.RevisionID == "" || created.TrajectoryID == "" || created.Replay {
		t.Fatalf("created=%+v", created)
	}
	createdSnapshot, _ := core.Store().GetLifecycleSnapshot(t.Context(), "user-1", "sandbox-test", created.TrajectoryID)
	replay := post("user-1", "Title", "private initial")
	var replayed textureLifecycleCreateResponse
	_ = json.Unmarshal(replay.Body.Bytes(), &replayed)
	if replay.Code != http.StatusCreated || !replayed.Replay || replayed.DocID != created.DocID || replayed.TrajectoryID != created.TrajectoryID {
		t.Fatalf("replay=%d %+v", replay.Code, replayed)
	}
	replayedSnapshot, _ := core.Store().GetLifecycleSnapshot(t.Context(), "user-1", "sandbox-test", created.TrajectoryID)
	if replayedSnapshot.SnapshotCursor != createdSnapshot.SnapshotCursor {
		t.Fatalf("create replay woke or mutated lifecycle: %d -> %d", createdSnapshot.SnapshotCursor, replayedSnapshot.SnapshotCursor)
	}
	if changed := post("user-1", "Changed", "private initial"); changed.Code != http.StatusConflict {
		t.Fatalf("changed=%d %s", changed.Code, changed.Body.String())
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), "user-1", "sandbox-test", created.TrajectoryID)
	if err != nil || snapshot.Document.DocID != created.DocID || snapshot.HeadRevision.RevisionID != created.RevisionID || snapshot.HeadRevision.Content != "private initial" || len(snapshot.WorkItems) != 1 || snapshot.WorkItems[0].AssignedAgentID != created.TargetAgentID {
		t.Fatalf("created lifecycle snapshot=%+v err=%v", snapshot, err)
	}
}

func TestResidentTextureInjectsAndAtomicallyConsumesOrderedOwnerOccurrences(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	path := "/api/texture/documents/" + start.InitialDocument.DocID + "/tell"
	var wakes []string
	handler.wakeOwnerInstruction = func(_ context.Context, ownerID, docID, instructionID string) error {
		if ownerID != start.OwnerID || docID != start.InitialDocument.DocID || instructionID == "" {
			t.Fatalf("invalid owner wake scope owner=%q doc=%q instruction=%q", ownerID, docID, instructionID)
		}
		wakes = append(wakes, instructionID)
		return nil
	}
	first := postOwnerInstruction(t, handler, path, start.OwnerID, "resident-one", "first private owner instruction", start.InitialRevision.RevisionID)
	if first.Code != http.StatusAccepted || len(wakes) != 1 {
		t.Fatalf("first queue status=%d wakes=%v", first.Code, wakes)
	}
	if _, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID); err != nil {
		t.Fatalf("create resident Texture run: %v", err)
	}
	second := postOwnerInstruction(t, handler, path, start.OwnerID, "resident-two", "second private owner instruction", start.InitialRevision.RevisionID)
	if second.Code != http.StatusAccepted || len(wakes) != 2 || wakes[0] == wakes[1] {
		t.Fatalf("resident queue status=%d wakes=%v", second.Code, wakes)
	}
	// Exact replay and refusal must not enqueue another actor occurrence.
	if replay := postOwnerInstruction(t, handler, path, start.OwnerID, "resident-two", "second private owner instruction", start.InitialRevision.RevisionID); replay.Code != http.StatusAccepted {
		t.Fatalf("resident replay status=%d", replay.Code)
	}
	if refusal := postOwnerInstruction(t, handler, path, start.OwnerID, "resident-two", "changed", start.InitialRevision.RevisionID); refusal.Code != http.StatusConflict {
		t.Fatalf("resident refusal status=%d", refusal.Code)
	}
	if len(wakes) != 2 {
		t.Fatalf("replay/refusal emitted owner wake: %v", wakes)
	}
	agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil || agent.ActiveRunID == "" {
		t.Fatalf("resident agent=%+v err=%v", agent, err)
	}
	run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	inject := handler.coagentUpdateTurnInjector(&run)
	if inject == nil {
		t.Fatal("owner instruction injector unavailable")
	}
	messages, err := inject(false)
	if err != nil || len(messages) != 1 || !strings.Contains(string(messages[0]), "first private owner instruction") || !strings.Contains(string(messages[0]), "second private owner instruction") {
		t.Fatalf("injected messages=%s err=%v", messages, err)
	}
	instructionIDs := metadataStringSlice(run.Metadata[textureOwnerInstructionIDsMetadata])
	requestIDs := metadataStringSlice(run.Metadata[textureOwnerRequestIDsMetadata])
	if len(instructionIDs) != 2 || len(requestIDs) != 2 {
		t.Fatalf("authenticated metadata ids=%v requests=%v", instructionIDs, requestIDs)
	}
	doc, err := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := handler.applyTextureLifecycleTurn(t.Context(), &run, doc, editTextureArgs{ToolCallID: "owner-turn-tool", WorkDisposition: string(types.WorkItemOpen)}, types.TextureTurnWait, types.Revision{}, store.TextureSourceGraphWriteSet{}, "owner-directed wait")
	if err != nil || result.TextureTurn == nil || len(result.TextureTurn.OwnerInstructionIDs) != 2 || len(result.TextureTurn.CausalRequestIDs) != 2 || len(result.Events) == 0 || len(result.Events[0].RequestIDs) != 2 {
		t.Fatalf("owner turn result=%+v err=%v", result, err)
	}
	pending, err := core.Store().ListPendingLifecycleOwnerInstructions(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after owner turn=%+v err=%v", pending, err)
	}
}

func TestOwnerInstructionWakeSurvivesPassivationRaceAndBootReconcile(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	var actorDispatches []string
	core.SetDispatchActor(func(_ context.Context, _, _, _ string, kind, content, _, _ string) error {
		actorDispatches = append(actorDispatches, kind+":"+content)
		return nil
	})
	var wakes []string
	handler.wakeOwnerInstruction = func(_ context.Context, ownerID, docID, instructionID string) error {
		wakes = append(wakes, ownerID+":"+docID+":"+instructionID)
		return nil
	}
	path := "/api/texture/documents/" + start.InitialDocument.DocID + "/tell"
	queued := postOwnerInstruction(t, handler, path, start.OwnerID, "passivation-race", "instruction survives passivation", start.InitialRevision.RevisionID)
	if queued.Code != http.StatusAccepted || len(wakes) != 1 {
		t.Fatalf("queue status=%d wakes=%v", queued.Code, wakes)
	}
	run, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID)
	if err != nil || run == nil {
		t.Fatalf("initial reconcile run=%+v err=%v", run, err)
	}
	passivate := func(commandID string, rec types.RunRecord) {
		t.Helper()
		rec.State = types.RunPassivated
		rec.UpdatedAt = time.Now().UTC()
		req := types.ReplaceLifecycleActivationRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: commandID, TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: rec}
		req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
		if _, err := core.Store().ReplaceLifecycleActivation(t.Context(), req); err != nil {
			t.Fatal(err)
		}
	}
	passivate("owner-wake-passivation-race", *run)
	reactivated, err := handler.ReconcileActorWake(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil || reactivated == nil || reactivated.RunID != run.RunID {
		t.Fatalf("passivation-race reactivation=%+v err=%v", reactivated, err)
	}
	passivate("owner-wake-boot-passivation", *reactivated)
	handler.Start(t.Context())
	agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil || agent.ActiveRunID != run.RunID {
		t.Fatalf("boot reconciled agent=%+v err=%v dispatches=%v", agent, err, actorDispatches)
	}
	active, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	inject := handler.coagentUpdateTurnInjector(&active)
	messages, err := inject(false)
	if err != nil || len(messages) != 1 || !strings.Contains(string(messages[0]), "instruction survives passivation") {
		t.Fatalf("boot owner instruction injection=%s err=%v", messages, err)
	}
	if len(wakes) != 1 {
		t.Fatalf("reconcile emitted extra owner occurrence wake: %v", wakes)
	}
}
