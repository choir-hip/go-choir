package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type textureRetryToolLoopProvider struct {
	provideriface.Provider
	responses  []*provideriface.ToolLoopResponse
	requests   []provideriface.ToolLoopRequest
	beforeCall func(int, provideriface.ToolLoopRequest)
}

func (p *textureRetryToolLoopProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	index := len(p.requests)
	p.requests = append(p.requests, req)
	if p.beforeCall != nil {
		p.beforeCall(index, req)
	}
	if index >= len(p.responses) {
		return nil, fmt.Errorf("unexpected Texture retry provider call %d", index)
	}
	return p.responses[index], nil
}

func TestLifecycleReviseCommitsOwnerRevisionAndDispatchesWake(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	type dispatch struct {
		kind, content, source string
	}
	var dispatches []dispatch
	core.SetDispatchActor(func(_ context.Context, _, _, _, kind, content, _, source string) error {
		dispatches = append(dispatches, dispatch{kind: kind, content: content, source: source})
		return nil
	})

	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "revise-occurrence", "revise privately", start.InitialRevision.RevisionID)
	if response.Code != http.StatusAccepted {
		t.Fatalf("lifecycle revise status=%d body=%s", response.Code, response.Body.String())
	}
	var receipt textureOwnerRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil || receipt.Schema != textureOwnerRevisionSchemaV1 || receipt.Replay || receipt.DocID != start.InitialDocument.DocID || receipt.RevisionID == "" {
		t.Fatalf("lifecycle revise receipt=%+v err=%v", receipt, err)
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.HeadRevision.RevisionID != receipt.RevisionID || snapshot.HeadRevision.AuthorKind != types.AuthorUser {
		t.Fatalf("owner revision head=%+v", snapshot.HeadRevision)
	}
	var metadata map[string]any
	if err := json.Unmarshal(snapshot.HeadRevision.Metadata, &metadata); err != nil || metadata["input_origin"] != textureInputOriginUserPrompt || metadata["owner_prompt"] != "revise privately" {
		t.Fatalf("owner revision metadata=%s err=%v", snapshot.HeadRevision.Metadata, err)
	}
	pending, _, ok := store.PendingTextureOwnerRevision(snapshot)
	if !ok || pending.RevisionID != receipt.RevisionID {
		t.Fatalf("pending owner revision=%+v pending=%v", pending, ok)
	}
	if len(dispatches) != 1 || dispatches[0].kind != "coagent_result" || dispatches[0].source != "owner:"+start.OwnerID {
		t.Fatalf("revision wake dispatches=%+v", dispatches)
	}
	occurrence, err := agentcore.DecodeTextureActorOccurrence(dispatches[0].content)
	if err != nil || occurrence.Kind != agentcore.TextureActorOccurrenceDocumentRevision || occurrence.HeadRevisionID != receipt.RevisionID {
		t.Fatalf("revision occurrence=%+v err=%v", occurrence, err)
	}
}

func TestLifecycleTextureWaitBlockNoChangeThenOwnerRevisionResumesSameResidentRun(t *testing.T) {
	for _, outcome := range []types.TextureTurnOutcome{types.TextureTurnWait, types.TextureTurnBlock, types.TextureTurnNoSemanticChange} {
		t.Run(string(outcome), func(t *testing.T) {
			core, handler := testAPISetup(t)
			installSynchronousTextureOwnerWake(t, core, handler)
			start := startObservationLifecycle(t, core.Store())
			path := "/api/texture/documents/" + start.InitialDocument.DocID + "/revise"
			first := postOwnerInstruction(t, handler, path, start.OwnerID, "first-"+string(outcome), "first owner revision", start.InitialRevision.RevisionID)
			if first.Code != http.StatusAccepted {
				t.Fatalf("first owner revision status=%d body=%s", first.Code, first.Body.String())
			}
			agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
			if err != nil || agent.ActiveRunID == "" {
				t.Fatalf("resident agent=%+v err=%v", agent, err)
			}
			run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
			if err != nil {
				t.Fatal(err)
			}
			if messages, err := handler.coagentUpdateTurnInjector(&run)(false); err != nil || len(messages) != 1 || !strings.Contains(string(messages[0]), "first owner revision") {
				t.Fatalf("first injection=%s err=%v", messages, err)
			}
			doc, err := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := handler.applyTextureLifecycleTurn(t.Context(), &run, doc, editTextureArgs{ToolCallID: "first-transition-" + string(outcome), WorkDisposition: string(types.WorkItemOpen)}, outcome, types.Revision{}, store.TextureSourceGraphWriteSet{}, "durable first outcome"); err != nil {
				t.Fatalf("first %s transition: %v", outcome, err)
			}
			if err := core.Store().SleepAgentMutationAfterTextureTurn(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID, run.RunID); err != nil {
				t.Fatalf("sleep first %s transition: %v", outcome, err)
			}
			stored, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, run.RunID)
			if err != nil {
				t.Fatal(err)
			}
			stored.State, stored.UpdatedAt, stored.FinishedAt = types.RunPassivated, time.Now().UTC(), nil
			passivate := types.ReplaceLifecycleActivationRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "passivate-" + run.RunID, TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: stored}
			passivate.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(passivate)
			if _, err := core.Store().ReplaceLifecycleActivation(t.Context(), passivate); err != nil {
				t.Fatalf("passivate resident run: %v", err)
			}
			second := postOwnerInstruction(t, handler, path, start.OwnerID, "resume-"+string(outcome), "resume same run", doc.CurrentRevisionID)
			if second.Code != http.StatusAccepted {
				t.Fatalf("resume owner revision status=%d body=%s", second.Code, second.Body.String())
			}
			resumed, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, run.RunID)
			if err != nil || resumed.RunID != run.RunID || (resumed.State != types.RunPending && resumed.State != types.RunPassivated) {
				t.Fatalf("same-run resume=%+v err=%v", resumed, err)
			}
			if messages, err := handler.coagentUpdateTurnInjector(&resumed)(false); err != nil || len(messages) != 1 || !strings.Contains(string(messages[0]), "resume same run") {
				t.Fatalf("resumed injection=%s err=%v", messages, err)
			}
		})
	}
}

func TestLifecycleTextureResearcherOpenerDerivesIdentitiesAndCommitsBeforeWake(t *testing.T) {
	core, handler := testAPISetup(t)
	installSynchronousTextureOwnerWake(t, core, handler)
	start := startObservationLifecycle(t, core.Store())
	first := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "researcher-opener", "research exact gap", start.InitialRevision.RevisionID)
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
	if err != nil || len(controls) != 1 || controls[0].OpenAgent == nil || controls[0].OpenWork == nil || !strings.HasPrefix(controls[0].TargetAgentID, "research:") || controls[0].OpenAgent.AgentID != controls[0].TargetAgentID || controls[0].OpenWork.WorkItemID != controls[0].TargetWorkItemID {
		t.Fatalf("runtime-derived Researcher opener=%+v err=%v", controls, err)
	}
	openWork := controls[0].OpenWork
	if openWork.CreatedByRunID != run.RunID || openWork.Details["requested_by_profile"] != "texture" || openWork.Details["requested_by_agent_id"] != run.AgentID || openWork.Details["requested_by_run_id"] != run.RunID {
		t.Fatalf("Researcher work item lacks requester Texture binding: %+v", openWork)
	}
	if _, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, controls[0].TargetAgentID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("Researcher agent existed before atomic turn: %v", err)
	}
	result, err := handler.applyTextureLifecycleTurn(t.Context(), &run, doc, args, types.TextureTurnWait, types.Revision{}, store.TextureSourceGraphWriteSet{}, "research requested")
	if err != nil || result.TextureTurn == nil || len(result.Controls) != 1 {
		t.Fatalf("atomic Researcher runtime turn=%+v err=%v", result, err)
	}
	createdAgent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, controls[0].TargetAgentID)
	if err != nil || createdAgent.Profile != "research" || createdAgent.LifecycleVersion != 1 {
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

func TestLifecycleTextureSemanticControlErrorKeepsSameRunWritableForAtomicResearcherRetry(t *testing.T) {
	core, handler := testAPISetup(t)
	installSynchronousTextureOwnerWake(t, core, handler)
	start := startObservationLifecycle(t, core.Store())
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "researcher-retry", "research exact gap", start.InitialRevision.RevisionID)
	if response.Code != http.StatusAccepted {
		t.Fatalf("tell status=%d body=%s", response.Code, response.Body.String())
	}
	agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handler.coagentUpdateTurnInjector(&run)(false); err != nil {
		t.Fatal(err)
	}
	doc, err := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}

	registry := toolregistry.NewToolRegistry()
	if err := RegisterTools(registry, handler); err != nil {
		t.Fatal(err)
	}
	invalidArgs, _ := json.Marshal(map[string]any{
		"doc_id": doc.DocID, "base_revision_id": doc.CurrentRevisionID,
		"content":   "The evidence gap remains open while focused research begins.",
		"rationale": "Open the first focused evidence path.", "work_disposition": "open",
		"controls": []map[string]any{{
			"open_researcher": true, "objective": "research exact gap",
			"packet": map[string]any{"schema_version": types.CoagentSourcePacketSchemaV1, "kind": "research_request", "summary": "research exact gap", "questions": []string{"What evidence resolves it?"}},
		}},
	})
	validArgs, _ := json.Marshal(map[string]any{
		"doc_id": doc.DocID, "base_revision_id": doc.CurrentRevisionID,
		"content":   "The evidence gap remains open while focused research begins.",
		"rationale": "Open the first focused evidence path.", "work_disposition": "open",
		"controls": []map[string]any{{
			"open_researcher": true, "objective": "research exact gap",
			"packet": map[string]any{"schema_version": types.CoagentSourcePacketSchemaV1, "kind": "question", "summary": "research exact gap", "questions": []string{"What evidence resolves it?"}},
		}},
	})
	provider := &textureRetryToolLoopProvider{responses: []*provideriface.ToolLoopResponse{
		{StopReason: "tool_use", ToolCalls: []types.ToolCall{{ID: "tool-call-invalid-semantic-control", Name: "rewrite_texture", Arguments: invalidArgs}}, Model: "scripted-texture-retry"},
		{StopReason: "tool_use", ToolCalls: []types.ToolCall{{ID: "tool-call-valid-semantic-control", Name: "rewrite_texture", Arguments: validArgs}}, Model: "scripted-texture-retry"},
	}}
	observedInvalidNoEffect := false
	provider.beforeCall = func(index int, req provideriface.ToolLoopRequest) {
		if index != 1 {
			return
		}
		messages, _ := json.Marshal(req.Messages)
		if !strings.Contains(string(messages), `kind \"research_request\" is not supported`) || !strings.Contains(string(messages), "previous durable transition") {
			t.Fatalf("semantic tool error/reminder not fed into retry request: %s", messages)
		}
		mutation, mutationErr := core.Store().GetAgentMutationByRun(t.Context(), start.OwnerID, start.ComputerID, run.RunID)
		if mutationErr != nil || mutation == nil || mutation.State != "pending" || mutation.RevisionID != "" {
			t.Fatalf("correctable tool error poisoned mutation: mutation=%+v err=%v", mutation, mutationErr)
		}
		afterInvalid, snapshotErr := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
		if snapshotErr != nil {
			t.Fatal(snapshotErr)
		}
		if afterInvalid.HeadRevision.RevisionID != before.HeadRevision.RevisionID ||
			afterInvalid.Trajectory.LifecycleVersion != before.Trajectory.LifecycleVersion ||
			len(afterInvalid.Agents) != len(before.Agents) || len(afterInvalid.WorkItems) != len(before.WorkItems) || len(afterInvalid.Updates) != len(before.Updates) {
			t.Fatalf("invalid pre-commit control partially mutated lifecycle:\n before=%+v\n after=%+v", before, afterInvalid)
		}
		observedInvalidNoEffect = true
	}
	var retryEvent bool
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind == types.EventRunRetry && phase == "required_write_tool" && strings.Contains(string(payload), "required_write_tool_failed") {
			retryEvent = true
		}
	}
	execution := textureToolExecutionContext(&run)
	_, _, loopErr := toolregistry.RunToolLoop(
		toolregistry.WithExecutionContext(t.Context(), execution), provider, registry,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"research exact gap"}`)}, "Texture", 0, emit, nil,
		toolregistry.WithInitialToolChoice("required"),
		toolregistry.WithRequiredWriteTools("patch_texture", "rewrite_texture", "record_texture_decision"),
		toolregistry.WithPassivatingToolSuccesses("patch_texture", "rewrite_texture", "record_texture_decision"),
	)
	if !errors.Is(loopErr, toolregistry.ErrToolLoopPassivated) {
		t.Fatalf("same-run corrected tool loop error = %v, want passivation after commit", loopErr)
	}
	if len(provider.requests) != 2 || !observedInvalidNoEffect || !retryEvent {
		t.Fatalf("bounded correction proof incomplete: provider_calls=%d no_effect=%v retry_event=%v", len(provider.requests), observedInvalidNoEffect, retryEvent)
	}
	afterValid, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if afterValid.HeadRevision.RevisionID == before.HeadRevision.RevisionID ||
		len(afterValid.Agents) != len(before.Agents)+1 || len(afterValid.WorkItems) != len(before.WorkItems)+1 || len(afterValid.Updates) != len(before.Updates)+1 {
		t.Fatalf("corrected retry did not atomically commit revision/Researcher/work/control:\n before=%+v\n after=%+v", before, afterValid)
	}
	var researcher types.AgentRecord
	for _, candidate := range afterValid.Agents {
		if strings.HasPrefix(candidate.AgentID, "research:") {
			researcher = candidate
			break
		}
	}
	if researcher.AgentID == "" || researcher.Profile != "research" || researcher.LifecycleVersion != 1 {
		t.Fatalf("corrected retry Researcher = %+v", researcher)
	}
	mutation, err := core.Store().GetAgentMutationByRun(t.Context(), start.OwnerID, start.ComputerID, run.RunID)
	if err != nil || mutation == nil || mutation.State != "pending" || mutation.RevisionID != afterValid.HeadRevision.RevisionID {
		t.Fatalf("committed retry mutation projection = %+v err=%v", mutation, err)
	}
}

func TestTextureLifecycleCreateExactReplayAndChangedPayloadConflict(t *testing.T) {
	core, handler := testAPISetup(t)
	var dispatches []string
	core.SetDispatchActor(func(_ context.Context, _, _, _ string, kind, content, _, _ string) error {
		dispatches = append(dispatches, kind+":"+content)
		return nil
	})
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
	createdSnapshot, _ := core.Store().GetLifecycleSnapshot(t.Context(), "user-1", "autoputer-test", created.TrajectoryID)
	agent, err := core.Store().GetAgentByScope(t.Context(), "user-1", "autoputer-test", created.TargetAgentID)
	if err != nil || agent.ActiveRunID == "" {
		t.Fatalf("initial Texture activation agent=%+v err=%v", agent, err)
	}
	initialRun, err := core.Store().GetLifecycleRun(t.Context(), "user-1", "autoputer-test", agent.ActiveRunID)
	if err != nil || metadataStringValue(initialRun.Metadata, "lifecycle_work_item_id") != created.TargetWorkItemID ||
		metadataStringValue(initialRun.Metadata, "request_intent") != "apply_owner_revision" ||
		metadataStringValue(initialRun.Metadata, "request_source") != "" {
		t.Fatalf("initial Texture activation run=%+v err=%v", initialRun, err)
	}
	if len(dispatches) != 1 || !strings.HasPrefix(dispatches[0], "initial_dispatch:") {
		t.Fatalf("initial Texture dispatches=%v", dispatches)
	}
	replay := post("user-1", "Title", "private initial")
	var replayed textureLifecycleCreateResponse
	_ = json.Unmarshal(replay.Body.Bytes(), &replayed)
	if replay.Code != http.StatusCreated || !replayed.Replay || replayed.DocID != created.DocID || replayed.TrajectoryID != created.TrajectoryID {
		t.Fatalf("replay=%d %+v", replay.Code, replayed)
	}
	replayedSnapshot, _ := core.Store().GetLifecycleSnapshot(t.Context(), "user-1", "autoputer-test", created.TrajectoryID)
	if replayedSnapshot.SnapshotCursor != createdSnapshot.SnapshotCursor {
		t.Fatalf("create replay woke or mutated lifecycle: %d -> %d", createdSnapshot.SnapshotCursor, replayedSnapshot.SnapshotCursor)
	}
	if changed := post("user-1", "Changed", "private initial"); changed.Code != http.StatusConflict {
		t.Fatalf("changed=%d %s", changed.Code, changed.Body.String())
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), "user-1", "autoputer-test", created.TrajectoryID)
	if err != nil || snapshot.Document.DocID != created.DocID || snapshot.HeadRevision.RevisionID != created.RevisionID || snapshot.HeadRevision.Content != "private initial" || len(snapshot.WorkItems) != 1 || snapshot.WorkItems[0].AssignedAgentID != created.TargetAgentID {
		t.Fatalf("created lifecycle snapshot=%+v err=%v", snapshot, err)
	}
	if len(dispatches) != 1 {
		t.Fatalf("create replay/conflict redispatched initial work: %v", dispatches)
	}
}

func TestLifecycleTextureInitialWorkWakeRecoversCommittedStartExactlyOnce(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	var dispatches []string
	core.SetDispatchActor(func(_ context.Context, _, _, _ string, kind, content, _, _ string) error {
		dispatches = append(dispatches, kind+":"+content)
		return nil
	})

	run, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID)
	if err != nil || run == nil || run.AgentID != start.Agent.AgentID || run.TrajectoryID != start.TrajectoryID ||
		metadataStringValue(run.Metadata, "lifecycle_work_item_id") != start.InitialWork.WorkItemID ||
		metadataStringValue(run.Metadata, "current_revision_id") != start.InitialRevision.RevisionID ||
		metadataStringValue(run.Metadata, "request_intent") != "initial_owner_work" ||
		metadataStringValue(run.Metadata, "request_source") != "" {
		t.Fatalf("committed-start recovery run=%+v err=%v", run, err)
	}
	if len(dispatches) != 1 || !strings.HasPrefix(dispatches[0], "initial_dispatch:") {
		t.Fatalf("committed-start recovery dispatches=%v", dispatches)
	}
	if duplicate, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID); err != nil || duplicate != nil {
		t.Fatalf("committed-start replay=%+v err=%v", duplicate, err)
	}
	if len(dispatches) != 1 {
		t.Fatalf("committed-start replay redispatched: %v", dispatches)
	}
}

func TestLifecycleTextureConcurrentInitialWorkWakeKeepsSoleWinnerAuthoritative(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	var dispatchMu sync.Mutex
	var dispatches []string
	core.SetDispatchActor(func(_ context.Context, _, _, _ string, kind, content, _, _ string) error {
		dispatchMu.Lock()
		dispatches = append(dispatches, kind+":"+content)
		dispatchMu.Unlock()
		return nil
	})

	const callers = 40
	startGate := make(chan struct{})
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startGate
			_, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID)
			errs <- err
		}()
	}
	close(startGate)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent initial wake: %v", err)
		}
	}

	dispatchMu.Lock()
	gotDispatches := append([]string(nil), dispatches...)
	dispatchMu.Unlock()
	if len(gotDispatches) != 1 || !strings.HasPrefix(gotDispatches[0], "initial_dispatch:") {
		t.Fatalf("concurrent initial dispatches=%v", gotDispatches)
	}
	agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil || agent.ActiveRunID == "" {
		t.Fatalf("concurrent winner agent=%+v err=%v", agent, err)
	}
	if err := handler.ValidateActivationAuthority(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID, agent.ActiveRunID); err != nil {
		t.Fatalf("concurrent winner lost activation authority: %v", err)
	}
	runs, err := core.Store().ListLifecycleRunsByChannel(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var textureRuns int
	for i := range runs {
		if runs[i].AgentID == start.Agent.AgentID && isTextureAgentRevisionTaskType(metadataStringValue(runs[i].Metadata, "type")) {
			textureRuns++
		}
	}
	if textureRuns != 1 {
		t.Fatalf("concurrent initial Texture runs=%d all=%+v", textureRuns, runs)
	}
}

func TestLifecycleTextureSlowDocumentWakeDoesNotBlockUnrelatedDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	first := startObservationLifecycle(t, core.Store())
	second := first
	second.CommandID = "start-observation-other"
	second.TrajectoryID = "trajectory-observation-other"
	second.SubjectRefs = map[string]string{"artifact": "texture://document/doc-observation-other", "doc_id": "doc-observation-other"}
	second.InitialWork.WorkItemID = "work-observation-other"
	second.InitialWork.AssignedAgentID = "texture:doc-observation-other"
	second.InitialDocument.DocID = "doc-observation-other"
	second.InitialRevision.RevisionID = "revision-observation-other-0"
	second.Agent.AgentID = "texture:doc-observation-other"
	second.Agent.ChannelID = "doc-observation-other"
	second.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(second)
	if _, err := core.Store().StartLifecycle(t.Context(), second); err != nil {
		t.Fatal(err)
	}

	var dispatchMu sync.Mutex
	var dispatches []string
	core.SetDispatchActor(func(_ context.Context, _, _, toAgentID, kind, content, _, _ string) error {
		dispatchMu.Lock()
		dispatches = append(dispatches, toAgentID+":"+kind+":"+content)
		dispatchMu.Unlock()
		return nil
	})

	unlockFirst := handler.lockTextureWakeScope(first.OwnerID, first.ComputerID, first.InitialDocument.DocID)
	firstStarted := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		close(firstStarted)
		_, err := handler.ReconcileAgentWake(t.Context(), first.OwnerID, first.InitialDocument.DocID)
		firstDone <- err
	}()
	<-firstStarted

	secondRun, err := handler.ReconcileAgentWake(t.Context(), second.OwnerID, second.InitialDocument.DocID)
	if err != nil || secondRun == nil || secondRun.AgentID != second.Agent.AgentID {
		t.Fatalf("unrelated document wake blocked/failed: run=%+v err=%v", secondRun, err)
	}
	if err := handler.ValidateActivationAuthority(t.Context(), second.OwnerID, second.ComputerID, second.Agent.AgentID, secondRun.RunID); err != nil {
		t.Fatalf("unrelated document winner authority: %v", err)
	}
	dispatchMu.Lock()
	beforeRelease := append([]string(nil), dispatches...)
	dispatchMu.Unlock()
	if len(beforeRelease) != 1 || !strings.HasPrefix(beforeRelease[0], second.Agent.AgentID+":initial_dispatch:") {
		t.Fatalf("dispatches before releasing slow document=%v", beforeRelease)
	}

	unlockFirst()
	if err := <-firstDone; err != nil {
		t.Fatalf("slow document wake after release: %v", err)
	}
	dispatchMu.Lock()
	finalDispatches := append([]string(nil), dispatches...)
	dispatchMu.Unlock()
	if len(finalDispatches) != 2 {
		t.Fatalf("final independent dispatches=%v", finalDispatches)
	}
	handler.textureWakeLocksMu.Lock()
	remainingLocks := len(handler.textureWakeLocks)
	handler.textureWakeLocksMu.Unlock()
	if remainingLocks != 0 {
		t.Fatalf("wake lock registry leaked %d entries", remainingLocks)
	}
}

func TestResidentTextureInjectsAndConsumesCurrentOwnerRevision(t *testing.T) {
	core, handler := testAPISetup(t)
	installSynchronousTextureOwnerWake(t, core, handler)
	start := startObservationLifecycle(t, core.Store())
	path := "/api/texture/documents/" + start.InitialDocument.DocID + "/revise"
	first := postOwnerInstruction(t, handler, path, start.OwnerID, "resident-one", "superseded owner revision", start.InitialRevision.RevisionID)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first owner revision status=%d body=%s", first.Code, first.Body.String())
	}
	var firstReceipt textureOwnerRevisionResponse
	if err := json.NewDecoder(first.Body).Decode(&firstReceipt); err != nil {
		t.Fatal(err)
	}
	second := postOwnerInstruction(t, handler, path, start.OwnerID, "resident-two", "current owner revision", firstReceipt.RevisionID)
	if second.Code != http.StatusAccepted {
		t.Fatalf("second owner revision status=%d body=%s", second.Code, second.Body.String())
	}
	var secondReceipt textureOwnerRevisionResponse
	if err := json.NewDecoder(second.Body).Decode(&secondReceipt); err != nil {
		t.Fatal(err)
	}
	firstRevision, err := core.Store().GetLifecycleRevision(t.Context(), start.OwnerID, start.ComputerID, firstReceipt.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	firstOccurrence, err := agentcore.TextureDocumentRevisionOccurrence(firstRevision, "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	firstContent, err := agentcore.EncodeTextureActorOccurrence(firstOccurrence)
	if err != nil {
		t.Fatal(err)
	}
	if _, state, err := handler.ResolveTextureActorOccurrence(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID, firstContent); err != nil || state != TextureActorOccurrenceTerminal {
		t.Fatalf("superseded revision occurrence state=%q err=%v", state, err)
	}
	agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil || agent.ActiveRunID == "" {
		t.Fatalf("resident agent=%+v err=%v", agent, err)
	}
	run, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	if messages, err := handler.coagentUpdateTurnInjector(&run)(false); err != nil || len(messages) != 1 || !strings.Contains(string(messages[0]), "current owner revision") {
		t.Fatalf("current owner revision injection=%s err=%v", messages, err)
	}
	doc, err := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handler.applyTextureLifecycleTurn(t.Context(), &run, doc, editTextureArgs{ToolCallID: "consume-current-owner-revision", WorkDisposition: string(types.WorkItemOpen)}, types.TextureTurnWait, types.Revision{}, store.TextureSourceGraphWriteSet{}, "consume current owner revision"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, pending := store.PendingTextureOwnerRevision(snapshot); pending {
		t.Fatalf("current owner revision remained pending: %+v", snapshot)
	}
	if secondReceipt.RevisionID != snapshot.HeadRevision.RevisionID {
		t.Fatalf("consumed revision head=%s want=%s", snapshot.HeadRevision.RevisionID, secondReceipt.RevisionID)
	}
}

func TestOwnerRevisionWakeSurvivesPassivationRaceAndBootReconcile(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	var dispatches []string
	core.SetDispatchActor(func(_ context.Context, _, _, _ string, kind, content, _, _ string) error {
		dispatches = append(dispatches, kind+":"+content)
		return nil
	})
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "passivation-race", "owner revision survives passivation", start.InitialRevision.RevisionID)
	if response.Code != http.StatusAccepted || len(dispatches) != 1 {
		t.Fatalf("owner revision status=%d dispatches=%v body=%s", response.Code, dispatches, response.Body.String())
	}
	run, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID)
	if err != nil || run == nil {
		t.Fatalf("initial reconcile run=%+v err=%v", run, err)
	}
	passivate := func(commandID string, rec types.RunRecord) {
		t.Helper()
		rec.State, rec.UpdatedAt, rec.FinishedAt = types.RunPassivated, time.Now().UTC(), nil
		request := types.ReplaceLifecycleActivationRequest{
			OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: commandID,
			TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: rec,
		}
		request.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(request)
		if _, err := core.Store().ReplaceLifecycleActivation(t.Context(), request); err != nil {
			t.Fatal(err)
		}
	}
	passivate("owner-revision-passivation-race", *run)
	reactivated, err := handler.ReconcileActorWake(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil || reactivated == nil || reactivated.RunID != run.RunID {
		t.Fatalf("passivation-race reactivation=%+v err=%v", reactivated, err)
	}
	passivate("owner-revision-boot-passivation", *reactivated)
	if err := handler.Start(t.Context()); err != nil {
		t.Fatalf("boot reconcile: %v", err)
	}
	agent, err := core.Store().GetAgentByScope(t.Context(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil || agent.ActiveRunID != run.RunID {
		t.Fatalf("boot reconciled agent=%+v err=%v dispatches=%v", agent, err, dispatches)
	}
	active, err := core.Store().GetLifecycleRun(t.Context(), start.OwnerID, start.ComputerID, agent.ActiveRunID)
	if err != nil {
		t.Fatal(err)
	}
	if messages, err := handler.coagentUpdateTurnInjector(&active)(false); err != nil || len(messages) != 1 || !strings.Contains(string(messages[0]), "owner revision survives passivation") {
		t.Fatalf("boot owner revision injection=%s err=%v", messages, err)
	}
}
