package textureowner

import (
	"context"
	"encoding/json"
	"errors"
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
	agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil || strings.TrimSpace(agent.ActiveRunID) == "" {
		t.Fatalf("lifecycle revise did not create/reuse resident activation: agent=%+v err=%v", agent, err)
	}

	// Replay while that lifecycle actor is resident. The old branch used
	// deliverOwnerRevisionToTextureActor -> DispatchWorkerUpdate here; replay must
	// remain an owner-instruction receipt and create no producer/legacy mailbox row.
	replayReq := httptest.NewRequest(http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", strings.NewReader(string(body)))
	replayReq.Header.Set("X-Authenticated-User", start.OwnerID)
	replayResponse := httptest.NewRecorder()
	handler.HandleTextureRouter(replayResponse, replayReq)
	var replay textureOwnerInstructionResponse
	if err := json.Unmarshal(replayResponse.Body.Bytes(), &replay); err != nil || replayResponse.Code != http.StatusAccepted || !replay.Replay {
		t.Fatalf("resident lifecycle revise replay status=%d response=%+v err=%v body=%s", replayResponse.Code, replay, err, replayResponse.Body.String())
	}
	pendingOwner, err = core.Store().ListPendingLifecycleOwnerInstructions(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, 10)
	if err != nil || len(pendingOwner) != 1 {
		t.Fatalf("resident replay duplicated owner instruction: %+v err=%v", pendingOwner, err)
	}
	producerReports, err := core.Store().ListPendingLifecycleUpdates(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID, 10)
	if err != nil || len(producerReports) != 0 {
		t.Fatalf("lifecycle revise entered producer mailbox: %+v err=%v", producerReports, err)
	}
	legacy, err := core.Store().ListCoagentMailboxBacklog(t.Context(), start.OwnerID, start.Agent.AgentID, 10)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("lifecycle revise entered legacy DispatchWorkerUpdate mailbox: %+v err=%v", legacy, err)
	}
	messages, err := core.Store().ListChannelMessages(t.Context(), start.OwnerID, start.InitialDocument.DocID, 0, 10)
	if err != nil || len(messages) != 0 {
		t.Fatalf("lifecycle revise emitted legacy mailbox message: %+v err=%v", messages, err)
	}
}

func TestLifecycleTextureWaitBlockNoChangeThenTellResumesSameResidentRun(t *testing.T) {
	for _, outcome := range []types.TextureTurnOutcome{types.TextureTurnWait, types.TextureTurnBlock, types.TextureTurnNoSemanticChange} {
		t.Run(string(outcome), func(t *testing.T) {
			core, handler := testAPISetup(t)
			core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
			start := startObservationLifecycle(t, core.Store())
			path := "/api/texture/documents/" + start.InitialDocument.DocID
			first := postOwnerInstruction(t, handler, path+"/tell", start.OwnerID, "first-"+string(outcome), "first instruction", start.InitialRevision.RevisionID)
			if first.Code != http.StatusAccepted {
				t.Fatalf("first tell status=%d body=%s", first.Code, first.Body.String())
			}
			agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
			if err != nil || agent.ActiveRunID == "" {
				t.Fatalf("resident agent=%+v err=%v", agent, err)
			}
			runID := agent.ActiveRunID
			run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, runID)
			if err != nil {
				t.Fatal(err)
			}
			inject := handler.coagentUpdateTurnInjector(&run)
			if messages, err := inject(false); err != nil || len(messages) != 1 || !strings.Contains(string(messages[0]), "first instruction") {
				t.Fatalf("first injection=%s err=%v", messages, err)
			}
			doc, _ := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
			if _, err := handler.applyTextureLifecycleTurn(t.Context(), &run, doc, editTextureArgs{ToolCallID: "first-transition-" + string(outcome), WorkDisposition: string(types.WorkItemOpen)}, outcome, types.Revision{}, store.TextureSourceGraphWriteSet{}, "durable first outcome"); err != nil {
				t.Fatalf("first %s transition: %v", outcome, err)
			}
			if err := core.Store().SleepAgentMutationAfterTextureTurn(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID, runID); err != nil {
				t.Fatalf("sleep first %s transition: %v", outcome, err)
			}
			stored, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, runID)
			if err != nil {
				t.Fatal(err)
			}
			stored.State, stored.UpdatedAt, stored.FinishedAt = types.RunPassivated, time.Now().UTC(), nil
			passivate := types.ReplaceLifecycleActivationRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "passivate-" + runID, TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: stored}
			passivate.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(passivate)
			if _, err := core.Store().ReplaceLifecycleActivation(t.Context(), passivate); err != nil {
				t.Fatalf("passivate resident run: %v", err)
			}

			second := postOwnerInstruction(t, handler, path+"/tell", start.OwnerID, "resume-"+string(outcome), "resume same run", start.InitialRevision.RevisionID)
			if second.Code != http.StatusAccepted {
				t.Fatalf("resume tell status=%d body=%s", second.Code, second.Body.String())
			}
			resumed, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, runID)
			if err != nil || resumed.RunID != runID || resumed.State != types.RunPending {
				t.Fatalf("same-run resume=%+v err=%v", resumed, err)
			}
			mutation, err := core.Store().GetAgentMutationByRun(t.Context(), start.OwnerID, start.ComputerID, runID)
			if err != nil || mutation == nil || mutation.State != "pending" {
				t.Fatalf("same-run mutation authority=%+v err=%v", mutation, err)
			}
			inject = handler.coagentUpdateTurnInjector(&resumed)
			if messages, err := inject(false); err != nil || len(messages) != 1 || !strings.Contains(string(messages[0]), "resume same run") {
				t.Fatalf("resumed injection=%s err=%v", messages, err)
			}
			doc, _ = core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
			if _, err := handler.applyTextureLifecycleTurn(t.Context(), &resumed, doc, editTextureArgs{ToolCallID: "resumed-transition-" + string(outcome), WorkDisposition: string(types.WorkItemOpen)}, types.TextureTurnWait, types.Revision{}, store.TextureSourceGraphWriteSet{}, "same resident run resumed"); err != nil {
				t.Fatalf("same-run resumed transition: %v", err)
			}
		})
	}
}

func TestLifecycleTextureResearcherOpenerDerivesIdentitiesAndCommitsBeforeWake(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	start := startObservationLifecycle(t, core.Store())
	first := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/tell", start.OwnerID, "researcher-opener", "research exact gap", start.InitialRevision.RevisionID)
	if first.Code != http.StatusAccepted {
		t.Fatalf("tell status=%d body=%s", first.Code, first.Body.String())
	}
	agent, _ := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	inject := handler.coagentUpdateTurnInjector(&run)
	if _, err := inject(false); err != nil {
		t.Fatal(err)
	}
	doc, _ := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	args := editTextureArgs{ToolCallID: "open-researcher-tool", WorkDisposition: string(types.WorkItemOpen), Controls: []textureControlArgs{{
		OpenResearcher: true, Objective: "research exact gap",
		Packet: types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "question", Summary: "research exact gap", Questions: []string{"What evidence resolves it?"}},
	}}}
	snapshot, _ := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	controls, err := handler.textureTurnControls(t.Context(), &run, doc, snapshot, args)
	if err != nil || len(controls) != 1 || controls[0].OpenAgent == nil || controls[0].OpenWork == nil || !strings.HasPrefix(controls[0].TargetAgentID, "researcher:") || controls[0].OpenAgent.AgentID != controls[0].TargetAgentID || controls[0].OpenWork.WorkItemID != controls[0].TargetWorkItemID {
		t.Fatalf("runtime-derived Researcher opener=%+v err=%v", controls, err)
	}
	if _, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, controls[0].TargetAgentID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Researcher agent existed before atomic turn: %v", err)
	}
	result, err := handler.applyTextureLifecycleTurn(t.Context(), &run, doc, args, types.TextureTurnWait, types.Revision{}, store.TextureSourceGraphWriteSet{}, "research requested")
	if err != nil || result.TextureTurn == nil || len(result.Controls) != 1 {
		t.Fatalf("atomic Researcher runtime turn=%+v err=%v", result, err)
	}
	createdAgent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, controls[0].TargetAgentID)
	if err != nil || createdAgent.Profile != "researcher" || createdAgent.LifecycleVersion != 1 {
		t.Fatalf("created Researcher=%+v err=%v", createdAgent, err)
	}
	legacy, err := core.Store().ListPendingWorkerUpdates(t.Context(), start.OwnerID, controls[0].TargetAgentID, 10)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("Researcher first control leaked to legacy mailbox: %+v err=%v", legacy, err)
	}
	replay, err := handler.applyTextureLifecycleTurn(t.Context(), &run, doc, args, types.TextureTurnWait, types.Revision{}, store.TextureSourceGraphWriteSet{}, "research requested")
	if err != nil || !replay.Replay || len(replay.Controls) != 1 || replay.Controls[0].TargetAgentID != controls[0].TargetAgentID {
		t.Fatalf("Researcher runtime opener replay=%+v err=%v", replay, err)
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
