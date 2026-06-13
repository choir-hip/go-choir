//go:build comprehensive

package runtime

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/markdownstructure"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func vtextAPISetup(t *testing.T) (*APIHandler, *store.Store) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "go-choir-m3-vtext-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open vtext api test store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_ = os.RemoveAll(promptRoot)
	})

	cfg := Config{
		SandboxID:           "sandbox-vtext-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     2 * time.Second,
		SupervisionInterval: 5 * time.Second,
	}

	bus := events.NewEventBus()
	provider := NewStubProvider(2 * time.Second)
	rt := New(cfg, s, bus, provider)

	return NewAPIHandler(rt), s
}

func vtextReplaceAllResult(content string, baseRevisionIDs ...string) string {
	env := map[string]any{
		"kind":      "vtext_edit",
		"operation": "replace_all",
		"content":   content,
	}
	if len(baseRevisionIDs) > 0 && strings.TrimSpace(baseRevisionIDs[0]) != "" {
		env["base_revision_id"] = strings.TrimSpace(baseRevisionIDs[0])
	}
	data, _ := json.Marshal(env)
	return string(data)
}

func vtextApplyEditsResult(edits []vtextTextEdit, baseRevisionIDs ...string) string {
	env := map[string]any{
		"kind":      "vtext_edit",
		"operation": "apply_edits",
		"edits":     edits,
	}
	if len(baseRevisionIDs) > 0 && strings.TrimSpace(baseRevisionIDs[0]) != "" {
		env["base_revision_id"] = strings.TrimSpace(baseRevisionIDs[0])
	}
	data, _ := json.Marshal(env)
	return string(data)
}

func TestHandleInternalVTextProposalDeliveryRecordsAuthorInbox(t *testing.T) {
	t.Parallel()
	h, s := vtextAPISetup(t)
	req := httptest.NewRequest(http.MethodPost, "/internal/vtext/proposals", strings.NewReader(`{
		"owner_id":"author-1",
		"proposal_id":"readerprop-1",
		"publication_id":"pub-1",
		"publication_version_id":"pubver-1",
		"submitter_id":"reader-1",
		"delivery_id":"delivery-1"
	}`))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	h.HandleInternalVTextProposalDelivery(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusAccepted {
		t.Fatalf("delivery status: got %d body %s", w.Code, w.Body.String())
	}
	var resp internalVTextProposalDeliveryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode delivery response: %v", err)
	}
	if resp.OwnerID != "author-1" || resp.DeliveryID != "delivery-1" || resp.TargetAgentID == "" || resp.ChannelID == "" {
		t.Fatalf("delivery response missing author routing: %+v", resp)
	}
	messages, err := s.ListChannelMessages(context.Background(), "author-1", resp.ChannelID, 0, 10)
	if err != nil {
		t.Fatalf("list author channel messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Role != "publication_proposal" || !strings.Contains(messages[0].Content, "proposal_id=readerprop-1") {
		t.Fatalf("author proposal message mismatch: %+v", messages)
	}
	if resp.RunID != "" {
		run, err := s.GetRun(context.Background(), resp.RunID)
		if err != nil {
			t.Fatalf("get proposal delivery run: %v", err)
		}
		if run.AgentProfile != AgentProfileSuper || run.OwnerID != "author-1" || !strings.Contains(run.Prompt, "readerprop-1") {
			t.Fatalf("proposal delivery run mismatch: %+v", run)
		}
	}
}

type vtextEditToolProvider struct {
	Provider
	result     string
	resultFunc func(prompt string) string
	delay      time.Duration
	choices    []string
}

func newVTextEditToolProvider(result string) *vtextEditToolProvider {
	return &vtextEditToolProvider{
		Provider: NewStubProvider(1 * time.Millisecond),
		result:   result,
	}
}

func (p *vtextEditToolProvider) ProviderName() string {
	return "vtext-edit-tool"
}

func (p *vtextEditToolProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	result := p.result
	if p.resultFunc != nil {
		result = p.resultFunc(task.Prompt)
	}
	task.Result = result
	emit(types.EventRunDelta, "execution", json.RawMessage(`{"text":"vtext edit tool provider","provider":"vtext-edit-tool"}`))
	return nil
}

func (p *vtextEditToolProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	if messagesContainToolCall(req.Messages, "spawn_agent") {
		return &ToolLoopResponse{StopReason: "end_turn", Text: "conductor handoff complete", Model: "test-model"}, nil
	}
	lastUser := extractLastUserMessage(req.Messages)
	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "edit_vtext") {
		return &ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnVTextToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if messagesContainToolCall(req.Messages, "edit_vtext") {
		return &ToolLoopResponse{StopReason: "end_turn", Text: "vtext turn complete", Model: "test-model"}, nil
	}
	if lastUser == "" || !toolDefinitionsContain(req.ToolDefinitions, "edit_vtext") {
		return &ToolLoopResponse{StopReason: "end_turn", Text: "vtext turn complete", Model: "test-model"}, nil
	}
	prompt := req.System + "\n" + lastUser
	result := p.result
	if p.resultFunc != nil {
		result = p.resultFunc(prompt)
	}
	call, err := editVTextToolCallFromLegacyResult(prompt, result)
	if err != nil {
		return &ToolLoopResponse{StopReason: "end_turn", Text: result, Model: "test-model"}, nil
	}
	return &ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls:  []types.ToolCall{call},
		Model:      "test-model",
	}, nil
}

func conductorSpawnVTextToolCall(prompt string) types.ToolCall {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "Create a working document."
	}
	title := buildInitialVTextTitle(prompt, "")
	args, _ := json.Marshal(map[string]any{
		"objective":       prompt,
		"role":            AgentProfileVText,
		"initial_content": "# " + title + "\n\n" + prompt,
	})
	return types.ToolCall{ID: "spawn-vtext-test-call", Name: "spawn_agent", Arguments: args}
}

type finalTextProvider struct {
	result string
}

func (p *finalTextProvider) ProviderName() string {
	return "final-text-provider"
}

func (p *finalTextProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	task.Result = p.result
	emit(types.EventRunDelta, "execution", json.RawMessage(`{"text":"final provider text","provider":"final-text-provider"}`))
	return nil
}

func messagesContainToolCall(messages []json.RawMessage, name string) bool {
	for _, raw := range messages {
		var msg struct {
			Content []map[string]any `json:"content"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		for _, block := range msg.Content {
			if blockType, _ := block["type"].(string); blockType != "tool_use" {
				continue
			}
			if toolName, _ := block["name"].(string); toolName == name {
				return true
			}
		}
	}
	return false
}

func toolDefinitionsContain(defs []ToolDefinition, name string) bool {
	for _, def := range defs {
		if def.Name == name {
			return true
		}
	}
	return false
}

func editVTextToolCallFromLegacyResult(prompt, raw string) (types.ToolCall, error) {
	var env struct {
		Kind           string          `json:"kind"`
		BaseRevisionID string          `json:"base_revision_id,omitempty"`
		Operation      string          `json:"operation"`
		Content        string          `json:"content,omitempty"`
		Edits          []vtextTextEdit `json:"edits,omitempty"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return types.ToolCall{}, err
	}
	if strings.TrimSpace(env.Kind) != "vtext_edit" {
		return types.ToolCall{}, errors.New("not a vtext edit result")
	}
	docID := extractPromptValue(prompt, `"doc_id":"`, `"`)
	if docID == "" {
		docID = extractPromptValue(prompt, "Current coordination channel: ", ".")
	}
	baseRevisionID := strings.TrimSpace(env.BaseRevisionID)
	if baseRevisionID == "" {
		baseRevisionID = extractPromptValue(prompt, "Current head revision: ", " ")
	}
	args := editVTextArgs{
		DocID:          docID,
		BaseRevisionID: baseRevisionID,
		Operation:      env.Operation,
		Content:        env.Content,
		Edits:          env.Edits,
	}
	data, err := json.Marshal(args)
	if err != nil {
		return types.ToolCall{}, err
	}
	return types.ToolCall{ID: "edit-vtext-test-call", Name: "edit_vtext", Arguments: data}, nil
}

func extractPromptValue(s, prefix, suffix string) string {
	start := strings.Index(s, prefix)
	if start < 0 {
		return ""
	}
	rest := s[start+len(prefix):]
	if suffix == "" {
		return strings.TrimSpace(rest)
	}
	end := strings.Index(rest, suffix)
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

// ----- Document creation -----

func TestVTextAPICreateDocument(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "My Document"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp vtextCreateDocResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DocID == "" {
		t.Error("DocID is empty")
	}
	if resp.Title != "My Document" {
		t.Errorf("Title = %q, want %q", resp.Title, "My Document")
	}
	if resp.OwnerID != "user-1" {
		t.Errorf("OwnerID = %q, want %q", resp.OwnerID, "user-1")
	}
}

func TestVTextAPICreateDocumentAuth(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// No auth header.
	req := httptest.NewRequest(http.MethodPost, "/api/vtext/documents",
		bytes.NewReader([]byte(`{"title":"test"}`)))
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestVTextCancelAgentRevisionCancelsTrajectoryAndLeavesMutationResumable(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	trajectoryID := "traj-cancel-agent"
	doc := types.Document{
		DocID:     "doc-cancel-agent",
		OwnerID:   "user-1",
		Title:     "Cancel Agent",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	parent := types.RunRecord{
		RunID:        "run-cancel-parent",
		AgentID:      "agent-super-cancel",
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		OwnerID:      "user-1",
		SandboxID:    "sandbox-vtext-test",
		State:        types.RunRunning,
		Prompt:       "Revise document.",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
			runMetadataAgentRole:    AgentProfileSuper,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	child := types.RunRecord{
		RunID:        "run-cancel-child",
		AgentID:      "agent-vsuper-cancel",
		ParentRunID:  "spawned-by-other-run",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		OwnerID:      "user-1",
		SandboxID:    "sandbox-vtext-test",
		State:        types.RunRunning,
		Prompt:       "Background candidate.",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	graphChildDifferentTrajectory := types.RunRecord{
		RunID:        "run-cancel-graph-child-other-trajectory",
		AgentID:      "agent-other-trajectory",
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		OwnerID:      "user-1",
		SandboxID:    "sandbox-vtext-test",
		State:        types.RunRunning,
		Prompt:       "Different trajectory despite spawned-by provenance.",
		TrajectoryID: "traj-other-cancel-agent",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
			runMetadataTrajectoryID: "traj-other-cancel-agent",
		},
	}
	for _, run := range []types.RunRecord{parent, child, graphChildDifferentTrajectory} {
		if err := s.CreateRun(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.RunID, err)
		}
	}
	item, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-1",
		TrajectoryID:         trajectoryID,
		Objective:            "finish the pending VText revision",
		ObjectiveFingerprint: "fp-cancel-vtext-revision",
		CreatedByRunID:       parent.RunID,
	})
	if err != nil {
		t.Fatalf("create trajectory work item: %v", err)
	}
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:               doc.DocID,
		RunID:               parent.RunID,
		OwnerID:             "user-1",
		State:               "pending",
		ScheduledMessageSeq: 7,
		CreatedAt:           now,
	}); err != nil {
		t.Fatalf("create pending mutation: %v", err)
	}

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+doc.DocID+"/cancel", nil)
	w := httptest.NewRecorder()
	h.HandleVTextCancelAgentRevision(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp vtextCancelRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if resp.Status != "cancelled" || !resp.Resumable || !containsString(resp.CancelledRunIDs, parent.RunID) || !containsString(resp.CancelledRunIDs, child.RunID) {
		t.Fatalf("unexpected cancel response: %+v", resp)
	}
	if containsString(resp.CancelledRunIDs, graphChildDifferentTrajectory.RunID) {
		t.Fatalf("cancel response = %+v, should not include spawned-by run on another trajectory", resp)
	}
	mutation, err := s.GetAgentMutationByRun(ctx, parent.RunID)
	if err != nil {
		t.Fatalf("get mutation: %v", err)
	}
	if mutation.State != "cancelled" {
		t.Fatalf("mutation state = %q, want cancelled", mutation.State)
	}
	cancelledItem, err := s.GetWorkItem(ctx, "user-1", item.WorkItemID)
	if err != nil {
		t.Fatalf("get work item: %v", err)
	}
	if cancelledItem.Status != types.WorkItemCancelled {
		t.Fatalf("work item status = %s, want cancelled", cancelledItem.Status)
	}
	trajectory, err := s.GetTrajectory(ctx, "user-1", trajectoryID)
	if err != nil {
		t.Fatalf("get trajectory: %v", err)
	}
	if trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("trajectory status = %s, want cancelled", trajectory.Status)
	}
	for _, runID := range []string{parent.RunID, child.RunID} {
		run, err := s.GetRun(ctx, runID)
		if err != nil {
			t.Fatalf("get run %s: %v", runID, err)
		}
		if run.State != types.RunCancelled {
			t.Fatalf("run %s state = %s, want cancelled", runID, run.State)
		}
	}
	graphChild, err := s.GetRun(ctx, graphChildDifferentTrajectory.RunID)
	if err != nil {
		t.Fatalf("get graph child: %v", err)
	}
	if graphChild.State != types.RunRunning {
		t.Fatalf("spawned-by child on other trajectory state = %s, want running", graphChild.State)
	}
}

// ----- Document list -----

func TestVTextAPIListDocuments(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create 2 documents.
	for _, title := range []string{"Doc A", "Doc B"} {
		req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
			map[string]string{"title": title})
		w := httptest.NewRecorder()
		h.HandleVTextCreateDocument(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create document: status = %d", w.Code)
		}
	}

	// List documents.
	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents", nil)
	w := httptest.NewRecorder()
	h.HandleVTextListDocuments(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp vtextListDocsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Documents) != 2 {
		t.Errorf("len(documents) = %d, want 2", len(resp.Documents))
	}
}

// ----- Document get -----

func TestVTextAPIGetDocument(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create a document.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var createResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&createResp)

	// Get the document.
	req = vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+createResp.DocID, nil)
	w = httptest.NewRecorder()
	h.HandleVTextDocument(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp vtextDocumentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DocID != createResp.DocID {
		t.Errorf("DocID = %q, want %q", resp.DocID, createResp.DocID)
	}
}

// ----- Revision creation (user edit) -----

func TestVTextAPICreateRevisionUserEdit(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create a document.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create a user-authored revision. Public revision POSTs ignore
	// browser-supplied author labels and use the authenticated owner.
	revReq := vtextCreateRevisionRequest{
		Content:     "Hello, world!",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var revResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if revResp.RevisionID == "" {
		t.Error("RevisionID is empty")
	}
	if revResp.AuthorKind != types.AuthorUser {
		t.Errorf("AuthorKind = %q, want %q", revResp.AuthorKind, types.AuthorUser)
	}
	if revResp.AuthorLabel != "user-1" {
		t.Errorf("AuthorLabel = %q, want %q", revResp.AuthorLabel, "user-1")
	}
	if revResp.VersionNumber != 0 {
		t.Errorf("VersionNumber = %d, want 0", revResp.VersionNumber)
	}
}

func TestVTextAPICreateRevisionCanonicalizesAliasedImportedDocumentTitle(t *testing.T) {
	t.Parallel()
	h, s := vtextAPISetup(t)
	ctx := context.Background()

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "legacy-import.md"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create document status = %d, body: %s", w.Code, w.Body.String())
	}
	var docResp vtextCreateDocResponse
	if err := json.NewDecoder(w.Body).Decode(&docResp); err != nil {
		t.Fatalf("decode document response: %v", err)
	}
	if err := s.UpsertDocumentAlias(ctx, "user-1", "imports/legacy-import.md", docResp.DocID, time.Now().UTC()); err != nil {
		t.Fatalf("UpsertDocumentAlias: %v", err)
	}

	revReq := vtextCreateRevisionRequest{Content: "Imported projection first durable edit"}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create revision status = %d, body: %s", w.Code, w.Body.String())
	}

	doc, err := s.GetDocument(ctx, docResp.DocID, "user-1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.Title != "legacy-import.vtext" {
		t.Fatalf("document title = %q, want legacy-import.vtext", doc.Title)
	}
	docID, err := s.GetDocumentAlias(ctx, "user-1", "imports/legacy-import.md")
	if err != nil {
		t.Fatalf("GetDocumentAlias original source: %v", err)
	}
	if docID != docResp.DocID {
		t.Fatalf("original source alias doc_id = %q, want %q", docID, docResp.DocID)
	}
}

func TestVTextAPIListRevisionsReturnsDurableVersionNumbersPastFifty(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Many Versions"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create document status = %d, body: %s", w.Code, w.Body.String())
	}
	var docResp vtextCreateDocResponse
	if err := json.NewDecoder(w.Body).Decode(&docResp); err != nil {
		t.Fatalf("decode document response: %v", err)
	}

	parentID := ""
	var latest vtextRevisionResponse
	for i := 0; i < 55; i++ {
		revReq := vtextCreateRevisionRequest{
			Content:          fmt.Sprintf("Document body v%d", i),
			ParentRevisionID: parentID,
		}
		req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
		w = httptest.NewRecorder()
		h.HandleVTextRevisions(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create revision %d status = %d, body: %s", i, w.Code, w.Body.String())
		}
		if err := json.NewDecoder(w.Body).Decode(&latest); err != nil {
			t.Fatalf("decode revision %d response: %v", i, err)
		}
		if latest.VersionNumber != i {
			t.Fatalf("revision %d VersionNumber = %d, want %d", i, latest.VersionNumber, i)
		}
		parentID = latest.RevisionID
	}

	req = vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docResp.DocID+"/revisions?limit=10000", nil)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list revisions status = %d, body: %s", w.Code, w.Body.String())
	}
	var listResp vtextListRevisionsResponse
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Revisions) != 55 {
		t.Fatalf("len(revisions) = %d, want 55", len(listResp.Revisions))
	}
	if listResp.Revisions[0].VersionNumber != 54 {
		t.Fatalf("latest VersionNumber = %d, want 54", listResp.Revisions[0].VersionNumber)
	}
	if listResp.Revisions[54].VersionNumber != 0 {
		t.Fatalf("oldest VersionNumber = %d, want 0", listResp.Revisions[54].VersionNumber)
	}

	req = vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docResp.DocID, nil)
	w = httptest.NewRecorder()
	h.HandleVTextDocument(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get document status = %d, body: %s", w.Code, w.Body.String())
	}
	var getDocResp vtextDocumentResponse
	if err := json.NewDecoder(w.Body).Decode(&getDocResp); err != nil {
		t.Fatalf("decode get document response: %v", err)
	}
	if getDocResp.RevisionCount != 55 {
		t.Fatalf("RevisionCount = %d, want 55", getDocResp.RevisionCount)
	}
	if getDocResp.CurrentVersionNumber != 54 {
		t.Fatalf("CurrentVersionNumber = %d, want 54", getDocResp.CurrentVersionNumber)
	}
}

// ----- Revision creation ignores browser appagent authorship -----

func TestVTextAPICreateRevisionIgnoresAppAgentAuthorFields(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create a document.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create a user revision first.
	revReq := vtextCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	// Attempt to create an appagent revision through the public revision
	// endpoint. This must still be stored as a user-authored edit.
	revReq = vtextCreateRevisionRequest{
		Content:     "AI-improved draft",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var revResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if revResp.AuthorKind != types.AuthorUser {
		t.Errorf("AuthorKind = %q, want %q", revResp.AuthorKind, types.AuthorUser)
	}
	if revResp.AuthorLabel != "user-1" {
		t.Errorf("AuthorLabel = %q, want %q", revResp.AuthorLabel, "user-1")
	}
}

// ----- Invalid browser author kind ignored -----

func TestVTextAPIIgnoresInvalidAuthorKind(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create a document.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Try to create a revision with "worker" author kind. Public callers
	// cannot select canonical authorship, so the request is accepted as a
	// normal user edit instead of exposing an author-kind validator.
	revReq := vtextCreateRevisionRequest{
		Content:     "Worker content",
		AuthorKind:  "worker",
		AuthorLabel: "worker-1",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var revResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if revResp.AuthorKind != types.AuthorUser || revResp.AuthorLabel != "user-1" {
		t.Errorf("public revision author = %q/%q, want %q/user-1", revResp.AuthorKind, revResp.AuthorLabel, types.AuthorUser)
	}
}

// ----- History -----

func TestVTextAPIGetHistory(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create a document.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create revisions.
	revReq := vtextCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	revReq = vtextCreateRevisionRequest{
		Content:     "AI-improved",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	// Get history.
	req = vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docResp.DocID+"/history", nil)
	w = httptest.NewRecorder()
	h.HandleVTextHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp vtextHistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(resp.Entries))
	}
	// Newest first. Public revision POSTs are always user-authored even if
	// the caller supplies appagent fields.
	if resp.Entries[0].AuthorKind != types.AuthorUser {
		t.Errorf("first entry AuthorKind = %q, want %q", resp.Entries[0].AuthorKind, types.AuthorUser)
	}
}

// ----- Diff -----

func TestVTextAPIGetDiff(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create a document and revisions.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	revReq := vtextCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	var rev1Resp vtextRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&rev1Resp)

	revReq = vtextCreateRevisionRequest{
		Content:     "AI-improved draft",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	var rev2Resp vtextRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&rev2Resp)

	// Get diff.
	req = vtextRequest(t, http.MethodGet,
		"/api/vtext/diff?from="+rev1Resp.RevisionID+"&to="+rev2Resp.RevisionID, nil)
	w = httptest.NewRecorder()
	h.HandleVTextDiff(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp vtextDiffResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.FromRevisionID != rev1Resp.RevisionID {
		t.Errorf("FromRevisionID = %q, want %q", resp.FromRevisionID, rev1Resp.RevisionID)
	}
	if resp.ToRevisionID != rev2Resp.RevisionID {
		t.Errorf("ToRevisionID = %q, want %q", resp.ToRevisionID, rev2Resp.RevisionID)
	}
}

// ----- Blame -----

func TestVTextAPIGetBlame(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create a document and revisions.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	revReq := vtextCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	revReq = vtextCreateRevisionRequest{
		Content:     "AI-improved draft",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	var rev2Resp vtextRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&rev2Resp)

	// Get blame.
	req = vtextRequest(t, http.MethodGet,
		"/api/vtext/revisions/"+rev2Resp.RevisionID+"/blame", nil)
	w = httptest.NewRecorder()
	h.HandleVTextBlame(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp vtextBlameResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RevisionID != rev2Resp.RevisionID {
		t.Errorf("RevisionID = %q, want %q", resp.RevisionID, rev2Resp.RevisionID)
	}
	if len(resp.Sections) == 0 {
		t.Error("no blame sections")
	}
}

// ----- Snapshot (view historical revision) -----

func TestVTextAPISnapshotDoesNotMutateHead(t *testing.T) {
	t.Parallel()
	h, s := vtextAPISetup(t)

	// Create a document.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create two revisions.
	revReq := vtextCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	var rev1Resp vtextRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&rev1Resp)

	revReq = vtextCreateRevisionRequest{
		Content:     "Second draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	// View the first (historical) revision.
	req = vtextRequest(t, http.MethodGet,
		"/api/vtext/revisions/"+rev1Resp.RevisionID, nil)
	w = httptest.NewRecorder()
	h.HandleVTextRevision(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var snapshotResp vtextRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&snapshotResp)
	if snapshotResp.Content != "First draft" {
		t.Errorf("snapshot content = %q, want %q", snapshotResp.Content, "First draft")
	}

	// Verify the document head is still the second revision.
	doc, err := s.GetDocument(req.Context(), docResp.DocID, "user-1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.CurrentRevisionID == rev1Resp.RevisionID {
		t.Error("viewing historical snapshot should not change document head")
	}
}

// ----- Auth gating on vtext endpoints -----

func TestVTextAPIAuthGating(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/vtext/documents"},
		{http.MethodPost, "/api/vtext/documents"},
		{http.MethodGet, "/api/vtext/diff"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewReader(nil))
		w := httptest.NewRecorder()

		switch {
		case strings.HasPrefix(ep.path, "/api/vtext/documents"):
			h.HandleVTextDocumentsRoot(w, req)
		case strings.HasPrefix(ep.path, "/api/vtext/diff"):
			h.HandleVTextDiff(w, req)
		}

		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want %d", ep.method, ep.path, w.Code, http.StatusUnauthorized)
		}
	}
}

// ----- Citations and metadata -----

func TestVTextAPICitationsMetadataRoundTrip(t *testing.T) {
	t.Parallel()
	h, _ := vtextAPISetup(t)

	// Create a document.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create a revision with citations and metadata.
	citations := []types.Citation{
		{ID: "c1", Type: "url", Value: "https://example.com", Label: "Example"},
	}
	citJSON, _ := json.Marshal(citations)
	metaJSON, _ := json.Marshal(map[string]any{"tags": []string{"draft"}})

	revReq := vtextCreateRevisionRequest{
		Content:     "Document with citations",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Citations:   citJSON,
		Metadata:    metaJSON,
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)

	var revResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Get the revision back and check citations/metadata.
	req = vtextRequest(t, http.MethodGet,
		"/api/vtext/revisions/"+revResp.RevisionID, nil)
	w = httptest.NewRecorder()
	h.HandleVTextRevision(w, req)

	var getResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	var gotCitations []types.Citation
	if err := json.Unmarshal(getResp.Citations, &gotCitations); err != nil {
		t.Fatalf("unmarshal citations: %v", err)
	}
	if len(gotCitations) != 1 || gotCitations[0].Value != "https://example.com" {
		t.Errorf("citations round-trip failed: %v", gotCitations)
	}

	var gotMeta map[string]any
	if err := json.Unmarshal(getResp.Metadata, &gotMeta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	tags, _ := gotMeta["tags"].([]interface{})
	if len(tags) != 1 {
		t.Errorf("metadata tags round-trip failed: %v", tags)
	}
}

// ----- Agent revision tests -----

func vtextAPISetupWithProvider(t *testing.T, provider Provider, installTools bool) (*APIHandler, *store.Store, *Runtime) {
	return vtextAPISetupWithProviderAndOptions(t, provider, installTools)
}

func vtextAPISetupWithProviderAndOptions(t *testing.T, provider Provider, installTools bool, opts ...RuntimeOption) (*APIHandler, *store.Store, *Runtime) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "go-choir-m3-vtext-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open vtext api test store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_ = os.RemoveAll(promptRoot)
	})

	cfg := Config{
		SandboxID:           "sandbox-vtext-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: 5 * time.Second,
		VTextWakeDebounce:   250 * time.Millisecond,
	}

	bus := events.NewEventBus()
	rt := New(cfg, s, bus, provider, opts...)
	if installTools {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("get working directory: %v", err)
		}
		if err := rt.InstallDefaultAgentTools(cwd); err != nil {
			t.Fatalf("install default agent tools: %v", err)
		}
	}

	// Start the runtime so runs execute.
	ctx := context.Background()
	rt.Start(ctx)
	t.Cleanup(func() { rt.Stop() })

	return NewAPIHandler(rt), s, rt
}

// vtextAPISetupWithRuntime creates a test setup with a started runtime
// so that runs actually execute and complete.
func vtextAPISetupWithRuntime(t *testing.T) (*APIHandler, *store.Store, *Runtime) {
	t.Helper()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Stubbed vtext document revision."))
	provider.delay = 50 * time.Millisecond
	return vtextAPISetupWithProvider(t, provider, true)
}

type fakeVTextWakeClock struct {
	mu     sync.Mutex
	timers []*fakeVTextWakeTimer
}

type fakeVTextWakeTimer struct {
	mu     sync.Mutex
	active bool
	fn     func()
}

func (c *fakeVTextWakeClock) afterFunc(_ time.Duration, fn func()) vtextWakeTimer {
	timer := &fakeVTextWakeTimer{active: true, fn: fn}
	c.mu.Lock()
	c.timers = append(c.timers, timer)
	c.mu.Unlock()
	return timer
}

func (c *fakeVTextWakeClock) fireAll() {
	c.mu.Lock()
	timers := append([]*fakeVTextWakeTimer(nil), c.timers...)
	c.mu.Unlock()
	for _, timer := range timers {
		timer.fire()
	}
}

func (t *fakeVTextWakeTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	wasActive := t.active
	t.active = false
	return wasActive
}

func (t *fakeVTextWakeTimer) fire() {
	t.mu.Lock()
	if !t.active {
		t.mu.Unlock()
		return
	}
	t.active = false
	fn := t.fn
	t.mu.Unlock()
	fn()
}

type revisionPromptEchoProvider struct {
	delay time.Duration
}

func (p *revisionPromptEchoProvider) ProviderName() string {
	return "revision-prompt-echo"
}

func (p *revisionPromptEchoProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	if strings.Contains(task.Prompt, "Fresh user edit should survive") {
		task.Result = vtextReplaceAllResult("Integrated latest user direction: Fresh user edit should survive.")
	} else {
		task.Result = vtextReplaceAllResult("Stale output from the older document head.")
	}
	return nil
}

func (p *revisionPromptEchoProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	provider := &vtextEditToolProvider{
		Provider: NewStubProvider(1 * time.Millisecond),
		delay:    p.delay,
		resultFunc: func(prompt string) string {
			if strings.Contains(prompt, "Fresh user edit should survive") {
				return vtextReplaceAllResult("Integrated latest user direction: Fresh user edit should survive.")
			}
			return vtextReplaceAllResult("Stale output from the older document head.")
		},
	}
	return provider.CallWithTools(ctx, req)
}

type stochasticWorkflowProvider struct {
	delay time.Duration
}

func (p *stochasticWorkflowProvider) ProviderName() string {
	return "stochastic-workflow"
}

func (p *stochasticWorkflowProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	delay := p.delay
	if delay == 0 {
		delay = 90 * time.Millisecond
	}
	switch agentProfileForRun(task) {
	case AgentProfileConductor:
		delay = 10 * time.Millisecond
	case AgentProfileResearcher, AgentProfileSuper, AgentProfileCoSuper:
		delay = 5 * time.Millisecond
	}
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
	}

	switch agentProfileForRun(task) {
	case AgentProfileVText:
		task.Result = buildStochasticVTextResult(task.Prompt)
	default:
		task.Result = "stochastic workflow loop completed"
	}
	emit(types.EventRunDelta, "execution", json.RawMessage(`{"text":"stochastic workflow loop completed","provider":"stochastic-workflow"}`))
	return nil
}

func (p *stochasticWorkflowProvider) CallWithTools(ctx context.Context, req ToolLoopRequest) (*ToolLoopResponse, error) {
	if messagesContainToolCall(req.Messages, "spawn_agent") {
		return &ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow conductor handoff complete", Model: "test-model"}, nil
	}
	lastUser := extractLastUserMessage(req.Messages)
	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "edit_vtext") {
		return &ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnVTextToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if messagesContainToolCall(req.Messages, "edit_vtext") {
		return &ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow loop completed", Model: "test-model"}, nil
	}
	if lastUser == "" || !toolDefinitionsContain(req.ToolDefinitions, "edit_vtext") {
		return &ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow loop completed", Model: "test-model"}, nil
	}
	delay := p.delay
	if delay == 0 {
		delay = 90 * time.Millisecond
	}
	timer := time.NewTimer(delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return nil, ctx.Err()
	case <-timer.C:
	}
	prompt := req.System + "\n" + lastUser
	call, err := editVTextToolCallFromLegacyResult(prompt, buildStochasticVTextResult(prompt))
	if err != nil {
		return &ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow loop completed", Model: "test-model"}, nil
	}
	return &ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls:  []types.ToolCall{call},
		Model:      "test-model",
	}, nil
}

func buildStochasticVTextResult(prompt string) string {
	if strings.Contains(prompt, "CANCEL_RUN_MARKER") {
		return vtextReplaceAllResult("CANCELLED SHOULD NOT MATERIALIZE")
	}
	var b strings.Builder
	b.WriteString("Stochastic vtext revision.")
	if marker := latestUserEditMarker(prompt); marker != "" {
		b.WriteString("\nLatest user marker: ")
		b.WriteString(marker)
	}
	if strings.Contains(prompt, "Research findings ready") {
		b.WriteString("\nResearch integrated.")
	}
	if strings.Contains(prompt, "Super verification ready") {
		b.WriteString("\nSuper integrated.")
	}
	return vtextReplaceAllResult(b.String())
}

func latestUserEditMarker(prompt string) string {
	for i := 9; i >= 1; i-- {
		marker := "USER_EDIT_0" + string(rune('0'+i))
		if strings.Contains(prompt, marker) {
			return marker
		}
	}
	return ""
}

// createDocWithUserRevision is a test helper that creates a document and
// a user-authored revision, returning the doc ID and revision ID.
func createDocWithUserRevision(t *testing.T, h *APIHandler) (string, string) {
	t.Helper()

	// Create a document.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleVTextCreateDocument(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create document: status = %d, body: %s", w.Code, w.Body.String())
	}
	var docResp vtextCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create a user-authored revision.
	revReq := vtextCreateRevisionRequest{
		Content:     "Hello, world!",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user revision: status = %d, body: %s", w.Code, w.Body.String())
	}
	var revResp vtextRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&revResp)

	return docResp.DocID, revResp.RevisionID
}

func waitForRunRunning(t *testing.T, rt *Runtime, runID, ownerID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := rt.GetRun(context.Background(), runID, ownerID)
		if err != nil {
			t.Fatalf("get run %s: %v", runID, err)
		}
		if rec.State == types.RunRunning {
			return
		}
		if rec.State.Terminal() {
			t.Fatalf("run %s reached terminal state %q before running", runID, rec.State)
		}
		time.Sleep(10 * time.Millisecond)
	}
	rec, err := rt.GetRun(context.Background(), runID, ownerID)
	if err != nil {
		t.Fatalf("get run %s after timeout: %v", runID, err)
	}
	t.Fatalf("run %s did not reach running within %v; state=%q", runID, timeout, rec.State)
}

func waitForRevisionCount(t *testing.T, s *store.Store, docID, ownerID string, want int, timeout time.Duration) []types.Revision {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		revs, err := s.ListRevisionsByDoc(context.Background(), docID, ownerID, 20)
		if err == nil && len(revs) >= want {
			return revs
		}
		time.Sleep(20 * time.Millisecond)
	}
	revs, _ := s.ListRevisionsByDoc(context.Background(), docID, ownerID, 20)
	t.Fatalf("document %s did not reach %d revisions within %v; got %d", docID, want, timeout, len(revs))
	return nil
}

func waitForVTextQuiescent(t *testing.T, rt *Runtime, s *store.Store, ownerID, docID string, minCheckpointSeq uint64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pending, err := s.GetPendingAgentMutationByDoc(context.Background(), docID, ownerID)
		if err != nil {
			t.Fatalf("get pending mutation: %v", err)
		}
		_, activeErr := s.GetLatestActiveRunByAgent(context.Background(), ownerID, "vtext:"+docID)
		activeClear := errors.Is(activeErr, store.ErrNotFound)
		if activeErr != nil && !activeClear {
			t.Fatalf("get active vtext run: %v", activeErr)
		}
		checkpointReady := minCheckpointSeq == 0
		if minCheckpointSeq > 0 {
			checkpoint, err := s.GetVTextControllerCheckpoint(context.Background(), docID, ownerID)
			if err != nil {
				t.Fatalf("get vtext controller checkpoint: %v", err)
			}
			checkpointReady = checkpoint != nil && checkpoint.IntegratedMessageSeq >= int64(minCheckpointSeq)
		}
		if pending == nil && activeClear && checkpointReady {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	checkpoint, _ := s.GetVTextControllerCheckpoint(context.Background(), docID, ownerID)
	pending, _ := s.GetPendingAgentMutationByDoc(context.Background(), docID, ownerID)
	t.Fatalf("vtext doc %s did not become quiescent within %v; pending=%+v checkpoint=%+v", docID, timeout, pending, checkpoint)
}

func waitForWorkerUpdatesConsumed(t *testing.T, s *store.Store, docID, ownerID string, workerSeqs []uint64, timeout time.Duration) ([]types.Revision, map[int64]bool, bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastRevs []types.Revision
	lastConsumed := map[int64]bool{}
	lastBatched := false
	for time.Now().Before(deadline) {
		revs, err := s.ListRevisionsByDoc(context.Background(), docID, ownerID, 50)
		if err != nil {
			t.Fatalf("list revisions while waiting for worker consumption: %v", err)
		}
		consumedSeqs, batchedRevision := revisionWorkerConsumption(t, revs)
		allConsumed := true
		for _, seq := range workerSeqs {
			if !consumedSeqs[int64(seq)] {
				allConsumed = false
				break
			}
		}
		if allConsumed && batchedRevision {
			return revs, consumedSeqs, batchedRevision
		}
		lastRevs = revs
		lastConsumed = consumedSeqs
		lastBatched = batchedRevision
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for worker updates %v to be consumed; consumed=%+v batched=%v revs=%+v", workerSeqs, lastConsumed, lastBatched, lastRevs)
	return nil, nil, false
}

func revisionWorkerConsumption(t *testing.T, revs []types.Revision) (map[int64]bool, bool) {
	t.Helper()
	consumedSeqs := map[int64]bool{}
	batchedRevision := false
	for _, rev := range revs {
		if rev.AuthorKind != types.AuthorAppAgent {
			continue
		}
		meta := decodeRevisionMetadata(rev.Metadata)
		if metadataString(meta, "source") != "edit_vtext" || metadataString(meta, "vtext_edit_kind") != "vtext_edit" {
			continue
		}
		consumed := metadataSlice(t, meta, "worker_updates_consumed")
		if len(consumed) >= 2 {
			batchedRevision = true
		}
		for _, item := range consumed {
			entry, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("consumed worker metadata has type %T, want map", item)
			}
			seq, ok := entry["seq"].(float64)
			if !ok {
				t.Fatalf("consumed worker metadata missing seq: %+v", entry)
			}
			consumedSeqs[int64(seq)] = true
		}
	}
	return consumedSeqs, batchedRevision
}

func createUserRevisionFromCurrentHead(t *testing.T, h *APIHandler, s *store.Store, docID, ownerID, content string) string {
	t.Helper()
	doc, err := s.GetDocument(context.Background(), docID, ownerID)
	if err != nil {
		t.Fatalf("get document before user revision: %v", err)
	}
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", vtextCreateRevisionRequest{
		Content:          content,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "user",
		ParentRevisionID: doc.CurrentRevisionID,
	})
	w := httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user revision %q: status = %d, want %d; body: %s", content, w.Code, http.StatusCreated, w.Body.String())
	}
	var resp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode user revision response: %v", err)
	}
	return resp.RevisionID
}

func metadataSlice(t *testing.T, metadata map[string]any, key string) []any {
	t.Helper()
	raw, ok := metadata[key]
	if !ok {
		t.Fatalf("metadata missing %q: %+v", key, metadata)
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("metadata[%q] has type %T, want []any", key, raw)
	}
	return items
}

func metadataSeqContains(t *testing.T, metadata map[string]any, key string, seq uint64) bool {
	t.Helper()
	for _, item := range metadataSlice(t, metadata, key) {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("metadata[%q] item has type %T, want map[string]any", key, item)
		}
		got, ok := entry["seq"].(float64)
		if !ok {
			t.Fatalf("metadata[%q] item missing numeric seq: %+v", key, entry)
		}
		if int64(got) == int64(seq) {
			return true
		}
	}
	return false
}

// TestVTextAgentRevisionCreatesCanonicalRevision verifies that submitting
// an agent revision prompt creates a canonical appagent-authored revision
// (VAL-ETEXT-003).
func TestVTextAgentRevisionCreatesCanonicalRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Submit an agent revision request.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it more formal"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunID == "" {
		t.Error("RunID is empty")
	}
	if resp.DocID != docID {
		t.Errorf("DocID = %q, want %q", resp.DocID, docID)
	}

	// Wait for the task to complete and the revision to be created.
	state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("task state = %q, want %q", state, types.RunCompleted)
	}

	// Verify that a canonical appagent-authored revision was created.
	revs, err := s.ListRevisionsByDoc(context.Background(), docID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}

	// Should have 2 revisions: user + appagent.
	if len(revs) != 2 {
		t.Fatalf("len(revisions) = %d, want 2", len(revs))
	}

	// Find the appagent revision.
	var agentRev *types.Revision
	for i := range revs {
		if revs[i].AuthorKind == types.AuthorAppAgent {
			agentRev = &revs[i]
			break
		}
	}
	if agentRev == nil {
		t.Fatal("no appagent-authored revision found")
	}
	if agentRev.AuthorLabel != "appagent" {
		t.Errorf("AuthorLabel = %q, want %q", agentRev.AuthorLabel, "appagent")
	}
	if agentRev.Content == "" {
		t.Error("appagent revision content is empty")
	}

	// Document head should be the appagent revision.
	doc, err := s.GetDocument(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != agentRev.RevisionID {
		t.Errorf("document head = %q, want appagent revision %q", doc.CurrentRevisionID, agentRev.RevisionID)
	}
}

func TestVTextSystemPromptSharesChoirCoreContext(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)

	rec := &types.RunRecord{
		RunID:        "run-vtext-shared-prompt",
		AgentID:      "vtext:doc-1",
		ChannelID:    "doc-1",
		OwnerID:      "user-alice",
		AgentProfile: AgentProfileVText,
		AgentRole:    AgentProfileVText,
		Prompt:       "What's the latest with AI?",
	}

	prompt, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatalf("systemPromptForRun: %v", err)
	}
	if !strings.Contains(prompt, "You are one agent inside Choir, a multiagent writing, research, and execution system.") {
		t.Fatalf("system prompt missing shared Choir context: %q", prompt)
	}
	if !strings.Contains(prompt, "Current UTC date/time:") || !strings.Contains(prompt, "Treat relative-date requests") {
		t.Fatalf("system prompt missing temporal grounding context: %q", prompt)
	}
	if !strings.Contains(prompt, "VText is a durable document owner, not a one-shot answerer.") {
		t.Fatalf("system prompt missing vtext wake semantics: %q", prompt)
	}
	if !strings.Contains(prompt, "Current coordination channel: doc-1.") {
		t.Fatalf("system prompt missing coordination channel: %q", prompt)
	}
}

func TestVTextAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Hello, edited document.\n\nPolished structure."))

	h, s, _ := vtextAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make the supplied text more formal."})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("run state = %q, want %q", state, types.RunCompleted)
	}

	revs, err := s.ListRevisionsByDoc(context.Background(), docID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("revision count = %d, want 2", len(revs))
	}
	foundAppAgent := false
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Polished structure") {
			foundAppAgent = true
			break
		}
	}
	if !foundAppAgent {
		t.Fatalf("expected appagent revision over user-provided text, got %+v", revs)
	}
	if len(provider.choices) == 0 || provider.choices[0] != exactRequiredToolChoice("edit_vtext") {
		t.Fatalf("initial vtext tool_choice = %#v, want first choice %q", provider.choices, exactRequiredToolChoice("edit_vtext"))
	}
}

func TestInitialVTextRunWritesFirstAppagentRevisionThroughEdit(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("First VText-authored working revision."))

	h, s, rt := vtextAPISetupWithProvider(t, provider, true)
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"What's new in AI?"}`, "user-1")
	w := httptest.NewRecorder()
	h.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt-bar status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var submission promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&submission); err != nil {
		t.Fatalf("decode prompt-bar response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, submission.SubmissionID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("conductor state = %q, want completed", state)
	}
	conductor, err := rt.GetRun(context.Background(), submission.SubmissionID, "user-1")
	if err != nil {
		t.Fatalf("get conductor run: %v", err)
	}
	var decision conductorDecision
	if err := json.Unmarshal([]byte(conductor.Result), &decision); err != nil {
		t.Fatalf("decode conductor decision: %v\n%s", err, conductor.Result)
	}
	if decision.DocID == "" || decision.InitialLoopID == "" {
		t.Fatalf("conductor did not create vtext route: %+v", decision)
	}
	if decision.CreateInitialVersion == nil || *decision.CreateInitialVersion {
		t.Fatalf("conductor create_initial_version = %v, want false", decision.CreateInitialVersion)
	}
	if decision.FramingRevisionID != "" {
		t.Fatalf("conductor framing revision = %q, want empty", decision.FramingRevisionID)
	}
	if decision.InitialRevisionID != decision.UserRevisionID {
		t.Fatalf("initial revision = %q, want user seed %q", decision.InitialRevisionID, decision.UserRevisionID)
	}
	if state := waitForTaskCompletion(t, h, decision.InitialLoopID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("initial vtext state = %q, want completed", state)
	}
	initialRun, err := rt.GetRun(context.Background(), decision.InitialLoopID, "user-1")
	if err != nil {
		t.Fatalf("get initial vtext run: %v", err)
	}
	if _, ok := initialRun.Metadata["requires_worker_grounding"]; ok {
		t.Fatalf("initial vtext run should not carry requires_worker_grounding metadata: %+v", initialRun.Metadata)
	}

	revs, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("revision count = %d, want only v0/v1", len(revs))
	}
	foundVTextRevision := false
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "First VText-authored working revision") {
			foundVTextRevision = true
		}
	}
	if !foundVTextRevision {
		t.Fatalf("expected first VText-authored appagent revision, got %+v", revs)
	}
	mutation, err := s.GetAgentMutationByRun(context.Background(), decision.InitialLoopID)
	if err != nil {
		t.Fatalf("get initial mutation: %v", err)
	}
	if mutation == nil || mutation.State != "completed" {
		t.Fatalf("initial vtext mutation = %+v, want completed mutation", mutation)
	}
}

func TestVTextPromptSteersCurrentEventsToResearcherNotSuper(t *testing.T) {
	t.Parallel()
	current := types.Revision{
		DocID:      "doc-current-events",
		RevisionID: "rev-current-events",
		Content:    "what's going on with iran deal now",
		AuthorKind: types.AuthorAppAgent,
	}
	prompt := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "what's going on with iran deal now",
	}, "", false, nil, nil)

	for _, want := range []string{
		"For factual/current claims, write a brief working revision with explicit uncertainty, then call spawn_agent with role=\"researcher\"",
		"Ordinary factual, current-events, web, or \"what is going on now\" questions are research work, not super work",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("current-events vtext prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestVTextPromptExplicitResearcherOverridesSuperFirstShortcut(t *testing.T) {
	t.Parallel()
	current := types.Revision{
		DocID:      "doc-explicit-researcher",
		RevisionID: "rev-explicit-researcher",
		Content:    "Ask researcher for a concise finding and ask super for a tiny verification note.",
		AuthorKind: types.AuthorUser,
	}
	prompt := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "Ask researcher for a concise finding and ask super for a tiny verification note.",
	}, "", false, nil, nil)

	want := "The owner explicitly asked for a researcher. After the brief working revision, call spawn_agent with role=\"researcher\" in this run; do not satisfy the researcher request by asking only super."
	if !strings.Contains(prompt, want) {
		t.Fatalf("explicit researcher vtext prompt missing %q:\n%s", want, prompt)
	}
}

func TestVTextAgentRevisionAppliesStructuredEdit(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextApplyEditsResult([]vtextTextEdit{
		{Op: "replace", Find: "Hello, world!", Replace: "Hello, edited document."},
		{Op: "append", Text: "Evidence: structured worker update integrated."},
	}))

	h, s, _ := vtextAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Integrate the addressed worker update."})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("run state = %q, want %q", state, types.RunCompleted)
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 2, 5*time.Second)
	var head types.Revision
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent {
			head = rev
			break
		}
	}
	if head.RevisionID == "" {
		t.Fatalf("missing appagent revision; revisions=%+v", revs)
	}
	if !strings.Contains(head.Content, "Hello, edited document.") || !strings.Contains(head.Content, "Evidence: structured worker update integrated.") {
		t.Fatalf("structured edits were not materialized into full document content: %q", head.Content)
	}
	meta := decodeRevisionMetadata(head.Metadata)
	if meta["vtext_edit_operation"] != "apply_edits" {
		t.Fatalf("vtext_edit_operation = %v, want apply_edits; metadata=%+v", meta["vtext_edit_operation"], meta)
	}
}

func TestVTextAgentRevisionIgnoresRawStubProviderResult(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithProvider(t, NewStubProvider(1*time.Millisecond), false)
	docID, _ := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Revise with the default stub provider."})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("run state = %q, want %q", state, types.RunCompleted)
	}
	revs, err := s.ListRevisionsByDoc(context.Background(), docID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("raw stub output created canonical revisions: got %d revisions %+v, want only the user revision", len(revs), revs)
	}
}

func TestVTextAgentRevisionIgnoresProviderFinalJSONEdit(t *testing.T) {
	t.Parallel()
	provider := &finalTextProvider{result: vtextReplaceAllResult("FINAL JSON MUST NOT MATERIALIZE")}
	h, s, _ := vtextAPISetupWithProvider(t, provider, true)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Return a legacy structured edit as final text."})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("run state = %q, want %q", state, types.RunCompleted)
	}
	doc, err := s.GetDocument(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != baseRevisionID {
		t.Fatalf("provider final text changed head to %q, want unchanged base %q", doc.CurrentRevisionID, baseRevisionID)
	}
	revs, err := s.ListRevisionsByDoc(context.Background(), docID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("provider final JSON created canonical revisions: got %d revisions %+v, want only the user revision", len(revs), revs)
	}
}

func TestVTextAgentRevisionRejectsMalformedEditVTextToolCall(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextApplyEditsResult([]vtextTextEdit{
		{Op: "replace", Find: "text that is not in the current document", Replace: "replacement"},
	}))
	h, s, _ := vtextAPISetupWithProvider(t, provider, true)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Apply an invalid edit."})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("run state = %q, want %q", state, types.RunCompleted)
	}
	doc, err := s.GetDocument(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != baseRevisionID {
		t.Fatalf("current revision = %q, want unchanged base %q", doc.CurrentRevisionID, baseRevisionID)
	}
}

func TestVTextStaleAgentRevisionRejectsEditAfterUserEdit(t *testing.T) {
	t.Parallel()
	provider := &revisionPromptEchoProvider{delay: 250 * time.Millisecond}

	h, s, _ := vtextAPISetupWithProvider(t, provider, true)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Produce a draft from the current document."})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var initialResp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&initialResp); err != nil {
		t.Fatalf("decode initial agent revision response: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		rec, err := h.rt.GetRun(context.Background(), initialResp.RunID, "user-1")
		if err != nil {
			t.Fatalf("get initial vtext run: %v", err)
		}
		if rec.State == types.RunRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("initial vtext run never reached running state; last state=%q", rec.State)
		}
		time.Sleep(20 * time.Millisecond)
	}

	userEditReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", vtextCreateRevisionRequest{
		Content:          "Fresh user edit should survive.",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "user",
		ParentRevisionID: baseRevisionID,
	})
	userEditW := httptest.NewRecorder()
	h.HandleVTextRevisions(userEditW, userEditReq)
	if userEditW.Code != http.StatusCreated {
		t.Fatalf("create user redirect revision: status = %d, want %d; body: %s", userEditW.Code, http.StatusCreated, userEditW.Body.String())
	}
	var userEditResp vtextRevisionResponse
	if err := json.NewDecoder(userEditW.Body).Decode(&userEditResp); err != nil {
		t.Fatalf("decode user redirect revision: %v", err)
	}

	state := waitForTaskCompletion(t, h, initialResp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("stale initial run state = %q, want %q", state, types.RunCompleted)
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 2, 8*time.Second)
	for _, rev := range revs {
		if strings.Contains(rev.Content, "Stale output from the older document head") {
			t.Fatalf("stale output was materialized as a revision: %+v", rev)
		}
		if rev.AuthorKind == types.AuthorAppAgent {
			t.Fatalf("stale edit_vtext call created appagent revision: %+v", rev)
		}
	}
	doc, err := s.GetDocument(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != userEditResp.RevisionID {
		t.Fatalf("document head = %q, want latest user revision %q", doc.CurrentRevisionID, userEditResp.RevisionID)
	}
	mutation, err := s.GetAgentMutationByRun(context.Background(), initialResp.RunID)
	if err != nil {
		t.Fatalf("get mutation: %v", err)
	}
	if mutation == nil || mutation.State != "failed" {
		t.Fatalf("stale edit mutation = %+v, want failed", mutation)
	}
}

func TestVTextSeededStochasticWorkflowContracts(t *testing.T) {
	const ownerID = "user-1"
	const seed int64 = 20260430
	rng := rand.New(rand.NewSource(seed))
	provider := &stochasticWorkflowProvider{delay: 1500 * time.Millisecond}

	h, s, rt := vtextAPISetupWithProvider(t, provider, true)
	conductorRun, err := rt.StartRunWithMetadata(context.Background(), "Build a toy evolution model and verify it.", ownerID, map[string]any{
		runMetadataAgentProfile:  AgentProfileConductor,
		runMetadataAgentRole:     AgentProfileConductor,
		"input_source":           "prompt_bar",
		"requested_app":          "vtext",
		"seed_prompt":            "Build a toy evolution model and verify it.",
		"initial_document_title": "Toy evolution model",
	})
	if err != nil {
		t.Fatalf("start conductor run: %v", err)
	}
	conductorDone := waitForRunTerminalState(t, rt, conductorRun.RunID, ownerID, 5*time.Second)
	if conductorDone.State != types.RunCompleted {
		t.Fatalf("conductor state = %q, want completed", conductorDone.State)
	}
	var decision struct {
		DocID          string `json:"doc_id"`
		UserRevisionID string `json:"user_revision_id"`
	}
	if err := json.Unmarshal([]byte(conductorDone.Result), &decision); err != nil {
		t.Fatalf("decode conductor decision: %v\nraw=%s", err, conductorDone.Result)
	}
	if decision.DocID == "" || decision.UserRevisionID == "" {
		t.Fatalf("conductor decision missing durable vtext ids: %+v", decision)
	}
	initialRevs, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, ownerID, 10)
	if err != nil {
		t.Fatalf("list initial revisions: %v", err)
	}
	if len(initialRevs) != 1 {
		t.Fatalf("initial revision count = %d, want 1 user seed revision", len(initialRevs))
	}
	if initialRevs[0].RevisionID != decision.UserRevisionID || initialRevs[0].AuthorKind != types.AuthorUser {
		t.Fatalf("initial revision = %+v, want user seed revision %s", initialRevs[0], decision.UserRevisionID)
	}

	initialReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+decision.DocID+"/revise",
		map[string]string{"prompt": "Start a long stochastic workflow."})
	initialW := httptest.NewRecorder()
	h.HandleVTextAgentRevision(initialW, initialReq)
	if initialW.Code != http.StatusAccepted {
		t.Fatalf("start initial vtext revision: status = %d, want %d; body: %s", initialW.Code, http.StatusAccepted, initialW.Body.String())
	}
	var initialResp vtextAgentRevisionResponse
	if err := json.NewDecoder(initialW.Body).Decode(&initialResp); err != nil {
		t.Fatalf("decode initial vtext response: %v", err)
	}
	waitForRunRunning(t, rt, initialResp.RunID, ownerID, 5*time.Second)

	researchRun, err := rt.StartChildRun(context.Background(), initialResp.RunID, "Research toy model evidence", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    decision.DocID,
	})
	if err != nil {
		t.Fatalf("start researcher worker: %v", err)
	}
	superRun, err := rt.StartChildRun(context.Background(), initialResp.RunID, "Verify generated toy model", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataChannelID:    decision.DocID,
	})
	if err != nil {
		t.Fatalf("start super worker: %v", err)
	}

	type scheduledAction struct {
		at   time.Duration
		name string
		fn   func()
	}
	var workerSeqs []uint64
	latestWorkerSeqByRole := map[string]uint64{}
	var userRevisionIDs []string
	postWorkerUpdate := func(run *types.RunRecord, from, role, content string) {
		t.Helper()
		seq, err := rt.ChannelCast(WithToolExecutionContext(context.Background(), run), decision.DocID, "vtext:"+decision.DocID, "", from, role, content)
		if err != nil {
			t.Fatalf("post worker update %q: %v", content, err)
		}
		workerSeqs = append(workerSeqs, seq)
		latestWorkerSeqByRole[role] = seq
	}
	addUserEdit := func(marker string) {
		t.Helper()
		revID := createUserRevisionFromCurrentHead(t, h, s, decision.DocID, ownerID, marker+"\n\nKeep this user direction in later synthesis.")
		userRevisionIDs = append(userRevisionIDs, revID)
	}
	jitter := func(maxMs int) time.Duration {
		return time.Duration(rng.Intn(maxMs)) * time.Millisecond
	}
	actions := []scheduledAction{
		{at: 10*time.Millisecond + jitter(6), name: "research-1", fn: func() {
			postWorkerUpdate(researchRun, "researcher-1", "researcher", "Research findings ready.\n\nFindings:\n- WORKER_RESEARCH_01: mutation and selection can be modeled with grid-local rules.")
		}},
		{at: 14*time.Millisecond + jitter(6), name: "super-1", fn: func() {
			postWorkerUpdate(superRun, "super-1", "super", "Super verification ready.\n\nTests:\n- WORKER_SUPER_01: generated model needs deterministic seed checks.")
		}},
		{at: 24*time.Millisecond + jitter(8), name: "user-1", fn: func() {
			addUserEdit("USER_EDIT_01")
		}},
		{at: 34*time.Millisecond + jitter(8), name: "research-2", fn: func() {
			postWorkerUpdate(researchRun, "researcher-1", "researcher", "Research findings ready.\n\nFindings:\n- WORKER_RESEARCH_02: fitness should depend on environment plus inherited variation.")
		}},
		{at: 44*time.Millisecond + jitter(8), name: "user-2", fn: func() {
			addUserEdit("USER_EDIT_02")
		}},
		{at: 52*time.Millisecond + jitter(8), name: "super-2", fn: func() {
			postWorkerUpdate(superRun, "super-1", "super", "Super verification ready.\n\nTests:\n- WORKER_SUPER_02: visualization output should expose generation and population counts.")
		}},
		{at: 68*time.Millisecond + jitter(8), name: "user-3", fn: func() {
			addUserEdit("USER_EDIT_03")
		}},
	}
	sort.Slice(actions, func(i, j int) bool {
		if actions[i].at == actions[j].at {
			return actions[i].name < actions[j].name
		}
		return actions[i].at < actions[j].at
	})
	start := time.Now()
	for _, action := range actions {
		if sleep := action.at - time.Since(start); sleep > 0 {
			time.Sleep(sleep)
		}
		action.fn()
	}
	if len(userRevisionIDs) != 3 {
		t.Fatalf("user revisions created = %d, want 3", len(userRevisionIDs))
	}
	if len(workerSeqs) != 4 {
		t.Fatalf("worker updates posted = %d, want 4", len(workerSeqs))
	}
	maxWorkerSeq := uint64(0)
	for _, seq := range workerSeqs {
		if seq > maxWorkerSeq {
			maxWorkerSeq = seq
		}
	}

	staleState := waitForTaskCompletion(t, h, initialResp.RunID, 10*time.Second)
	if staleState != types.RunCompleted {
		t.Fatalf("initial stale vtext state = %q, want completed", staleState)
	}
	mutation, err := s.GetAgentMutationByRun(context.Background(), initialResp.RunID)
	if err != nil {
		t.Fatalf("get initial mutation: %v", err)
	}
	if mutation == nil || mutation.State != "failed" {
		t.Fatalf("initial stale mutation = %+v, want failed no-write mutation", mutation)
	}
	waitForVTextQuiescent(t, rt, s, ownerID, decision.DocID, maxWorkerSeq, 20*time.Second)

	expectedConsumedSeqs := make([]uint64, 0, len(latestWorkerSeqByRole))
	for _, seq := range latestWorkerSeqByRole {
		expectedConsumedSeqs = append(expectedConsumedSeqs, seq)
	}
	revs, consumedSeqs, batchedRevision := waitForWorkerUpdatesConsumed(t, s, decision.DocID, ownerID, expectedConsumedSeqs, 20*time.Second)
	for _, rev := range revs {
		if strings.Contains(rev.Content, "Stale output") {
			t.Fatalf("stale output materialized in revision %+v", rev)
		}
		if strings.Contains(rev.Content, "CANCELLED SHOULD NOT MATERIALIZE") {
			t.Fatalf("cancelled output materialized in revision %+v", rev)
		}
	}
	for _, seq := range expectedConsumedSeqs {
		if !consumedSeqs[int64(seq)] {
			t.Fatalf("worker update seq %d was not recorded as consumed; consumed=%+v", seq, consumedSeqs)
		}
	}
	if !batchedRevision {
		t.Fatalf("expected at least one appagent revision to consume a debounced batch; revs=%+v", revs)
	}

	doc, err := s.GetDocument(context.Background(), decision.DocID, ownerID)
	if err != nil {
		t.Fatalf("get stochastic document: %v", err)
	}
	head, err := s.GetRevision(context.Background(), doc.CurrentRevisionID, ownerID)
	if err != nil {
		t.Fatalf("get stochastic head: %v", err)
	}
	for _, want := range []string{"USER_EDIT_03", "Research integrated.", "Super integrated."} {
		if !strings.Contains(head.Content, want) {
			t.Fatalf("head content missing %q:\n%s", want, head.Content)
		}
	}

	initialRun, err := s.GetRun(context.Background(), initialResp.RunID)
	if err != nil {
		t.Fatalf("get initial vtext run: %v", err)
	}
	if initialRun.ParentRunID != conductorRun.RunID {
		t.Fatalf("initial vtext run parent = %q, want conductor run %q", initialRun.ParentRunID, conductorRun.RunID)
	}
	trajectoryID := trajectoryIDForRun(&initialRun)
	if trajectoryID != conductorRun.RunID {
		t.Fatalf("initial vtext trajectory = %q, want conductor trajectory %q", trajectoryID, conductorRun.RunID)
	}
	events, err := s.ListEventsByTrajectory(context.Background(), ownerID, trajectoryID, 500)
	if err != nil {
		t.Fatalf("list stochastic trajectory events: %v", err)
	}
	hasChannelMessage := false
	hasVTextRevision := false
	for _, ev := range events {
		switch ev.Kind {
		case types.EventChannelMessage:
			hasChannelMessage = true
		case types.EventVTextDocumentRevisionCreated, types.EventVTextAgentRevisionCompleted:
			hasVTextRevision = true
		}
	}
	if !hasChannelMessage || !hasVTextRevision {
		t.Fatalf("trajectory events missing causality markers: channel=%v vtext_revision=%v events=%+v", hasChannelMessage, hasVTextRevision, events)
	}

	cancelReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+decision.DocID+"/revise",
		map[string]string{"prompt": "CANCEL_RUN_MARKER"})
	cancelW := httptest.NewRecorder()
	h.HandleVTextAgentRevision(cancelW, cancelReq)
	if cancelW.Code != http.StatusAccepted {
		t.Fatalf("start cancellable vtext revision: status = %d, want %d; body: %s", cancelW.Code, http.StatusAccepted, cancelW.Body.String())
	}
	var cancelResp vtextAgentRevisionResponse
	if err := json.NewDecoder(cancelW.Body).Decode(&cancelResp); err != nil {
		t.Fatalf("decode cancellable vtext response: %v", err)
	}
	waitForRunRunning(t, rt, cancelResp.RunID, ownerID, 5*time.Second)
	if err := rt.CancelRun(context.Background(), cancelResp.RunID, ownerID); err != nil {
		t.Fatalf("cancel vtext run: %v", err)
	}
	cancelState := waitForTaskCompletion(t, h, cancelResp.RunID, 5*time.Second)
	if cancelState != types.RunCancelled {
		t.Fatalf("cancelled run state = %q, want cancelled", cancelState)
	}
	waitForVTextQuiescent(t, rt, s, ownerID, decision.DocID, maxWorkerSeq, 5*time.Second)
	revsAfterCancel, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, ownerID, 50)
	if err != nil {
		t.Fatalf("list revisions after cancellation: %v", err)
	}
	for _, rev := range revsAfterCancel {
		if strings.Contains(rev.Content, "CANCELLED SHOULD NOT MATERIALIZE") {
			t.Fatalf("cancelled vtext output was materialized: %+v", rev)
		}
	}
}

func TestVTextWorkerMessageAutoWakeCreatesFollowUpRevision(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Integrated grounded findings into the next revision."))

	h, s, rt := vtextAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	userRevReq := vtextCreateRevisionRequest{
		Content:     "Original draft.\n\nAdd a short section about recent model releases.",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
	}
	userRevReqBody := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", userRevReq)
	userRevW := httptest.NewRecorder()
	h.HandleVTextRevisions(userRevW, userRevReqBody)
	if userRevW.Code != http.StatusCreated {
		t.Fatalf("second user revision: status = %d, want %d; body: %s", userRevW.Code, http.StatusCreated, userRevW.Body.String())
	}

	researchRun, err := rt.StartRunWithMetadata(context.Background(), "Ground the recent release claims", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start research run: %v", err)
	}
	noiseRun, err := rt.StartRunWithMetadata(context.Background(), "Send non-worker chatter", "user-1", map[string]any{
		runMetadataAgentProfile: "auditor",
		runMetadataAgentRole:    "auditor",
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start noise run: %v", err)
	}
	skippedSeq, err := rt.ChannelCast(WithToolExecutionContext(context.Background(), noiseRun), docID, "vtext:"+docID, "", "auditor-1", "auditor", "This addressed note is not a worker update and must not drive synthesis.")
	if err != nil {
		t.Fatalf("post non-worker message: %v", err)
	}
	workerSeq, err := rt.ChannelCast(WithToolExecutionContext(context.Background(), researchRun), docID, "vtext:"+docID, "", "researcher-1", "researcher", "Evidence: the latest public model releases shipped this week with stronger reasoning and tool use.")
	if err != nil {
		t.Fatalf("post worker message: %v", err)
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 3, 5*time.Second)
	var agentRev *types.Revision
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Integrated grounded findings") {
			revCopy := rev
			agentRev = &revCopy
			break
		}
	}
	if agentRev == nil {
		t.Fatalf("expected wake-driven appagent revision, got %+v", revs)
	}
	agentMeta := decodeRevisionMetadata(agentRev.Metadata)
	consumed := metadataSlice(t, agentMeta, "worker_updates_consumed")
	if len(consumed) != 1 {
		t.Fatalf("worker_updates_consumed length = %d, want 1; metadata=%+v", len(consumed), agentMeta)
	}
	consumedUpdate := consumed[0].(map[string]any)
	if got := int64(consumedUpdate["seq"].(float64)); got != int64(workerSeq) {
		t.Fatalf("consumed worker seq = %d, want %d", got, workerSeq)
	}
	if got, _ := consumedUpdate["from_loop_id"].(string); got != researchRun.RunID {
		t.Fatalf("consumed from_loop_id = %q, want %q", got, researchRun.RunID)
	}
	skipped := metadataSlice(t, agentMeta, "worker_updates_skipped")
	if len(skipped) != 1 {
		t.Fatalf("worker_updates_skipped length = %d, want 1; metadata=%+v", len(skipped), agentMeta)
	}
	skippedUpdate := skipped[0].(map[string]any)
	if got := int64(skippedUpdate["seq"].(float64)); got != int64(skippedSeq) {
		t.Fatalf("skipped worker seq = %d, want %d", got, skippedSeq)
	}
	if got, _ := skippedUpdate["reason"].(string); got != "ineligible_sender" {
		t.Fatalf("skipped reason = %q, want ineligible_sender", got)
	}
	if pending := metadataSlice(t, agentMeta, "worker_updates_pending"); len(pending) != 0 {
		t.Fatalf("worker_updates_pending = %+v, want empty", pending)
	}

	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs: %v", err)
	}
	var wakeRun *types.RunRecord
	for i := range runs {
		if agentProfileForRun(&runs[i]) == AgentProfileVText && runs[i].ParentRunID == researchRun.RunID {
			wakeRun = &runs[i]
			break
		}
	}
	if wakeRun == nil {
		t.Fatalf("expected wake-driven vtext run on channel %s, got %+v", docID, runs)
	}
	if !strings.Contains(wakeRun.Prompt, "Recent addressed worker messages") {
		t.Fatalf("wake run prompt missing worker message context: %q", wakeRun.Prompt)
	}
	if !strings.Contains(wakeRun.Prompt, "Evidence: the latest public model releases") {
		t.Fatalf("wake run prompt missing worker message content: %q", wakeRun.Prompt)
	}
	if !strings.Contains(wakeRun.Prompt, "User edit diff from previous canonical revision to current user-authored draft:") {
		t.Fatalf("wake run prompt missing user diff context: %q", wakeRun.Prompt)
	}
	if !strings.Contains(wakeRun.Prompt, "- added: Original draft.") {
		t.Fatalf("wake run prompt missing user diff content: %q", wakeRun.Prompt)
	}
}

func TestVTextWorkerMessageAutoWakeBatchesRapidMessages(t *testing.T) {
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Integrated multiple grounded findings into one revision."))

	h, s, rt := vtextAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	userRevReq := vtextCreateRevisionRequest{
		Content:     "Original draft.\n\nNeed the newest facts.",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
	}
	userRevReqBody := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", userRevReq)
	userRevW := httptest.NewRecorder()
	h.HandleVTextRevisions(userRevW, userRevReqBody)
	if userRevW.Code != http.StatusCreated {
		t.Fatalf("second user revision: status = %d, want %d; body: %s", userRevW.Code, http.StatusCreated, userRevW.Body.String())
	}

	researchRun, err := rt.StartRunWithMetadata(context.Background(), "Research the latest facts", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start research run: %v", err)
	}
	postWorkerMessage := func(content string) uint64 {
		t.Helper()
		seq, err := rt.ChannelCast(WithToolExecutionContext(context.Background(), researchRun), docID, "vtext:"+docID, "", "researcher-1", "researcher", content)
		if err != nil {
			t.Fatalf("post worker message %q: %v", content, err)
		}
		return seq
	}
	seqA := postWorkerMessage("Evidence A: the first grounded fact arrived.")
	seqB := postWorkerMessage("Evidence B: the second grounded fact arrived.")

	revs := waitForRevisionCount(t, s, docID, "user-1", 3, 5*time.Second)
	appAgentRevisions := 0
	var agentRev *types.Revision
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Integrated multiple grounded findings") {
			appAgentRevisions++
			revCopy := rev
			agentRev = &revCopy
		}
	}
	if appAgentRevisions != 1 {
		t.Fatalf("expected exactly one wake-driven appagent revision, got %d revisions: %+v", appAgentRevisions, revs)
	}
	if agentRev == nil {
		t.Fatalf("expected batched appagent revision, got %+v", revs)
	}
	agentMeta := decodeRevisionMetadata(agentRev.Metadata)
	consumed := metadataSlice(t, agentMeta, "worker_updates_consumed")
	if len(consumed) != 2 {
		t.Fatalf("worker_updates_consumed length = %d, want 2; metadata=%+v", len(consumed), agentMeta)
	}
	gotSeqs := []int64{
		int64(consumed[0].(map[string]any)["seq"].(float64)),
		int64(consumed[1].(map[string]any)["seq"].(float64)),
	}
	wantSeqs := []int64{int64(seqA), int64(seqB)}
	if gotSeqs[0] != wantSeqs[0] || gotSeqs[1] != wantSeqs[1] {
		t.Fatalf("consumed seqs = %+v, want %+v", gotSeqs, wantSeqs)
	}
	if pending := metadataSlice(t, agentMeta, "worker_updates_pending"); len(pending) != 0 {
		t.Fatalf("worker_updates_pending = %+v, want empty", pending)
	}

	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs: %v", err)
	}
	var wakeRuns []types.RunRecord
	for i := range runs {
		if agentProfileForRun(&runs[i]) == AgentProfileVText && runs[i].ParentRunID == researchRun.RunID {
			wakeRuns = append(wakeRuns, runs[i])
		}
	}
	if len(wakeRuns) != 1 {
		t.Fatalf("expected one debounced vtext wake run, got %+v", wakeRuns)
	}
	if !strings.Contains(wakeRuns[0].Prompt, "Evidence A: the first grounded fact arrived.") {
		t.Fatalf("wake run prompt missing first worker message: %q", wakeRuns[0].Prompt)
	}
	if !strings.Contains(wakeRuns[0].Prompt, "Evidence B: the second grounded fact arrived.") {
		t.Fatalf("wake run prompt missing second worker message: %q", wakeRuns[0].Prompt)
	}
}

func TestVTextWorkerMessageDebounceUsesFakeClock(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Integrated fake-clock worker findings."))
	clock := &fakeVTextWakeClock{}

	h, s, rt := vtextAPISetupWithProviderAndOptions(t, provider, true, withVTextWakeAfterFuncForTest(clock.afterFunc))
	docID, _ := createDocWithUserRevision(t, h)

	researchRun, err := rt.StartRunWithMetadata(context.Background(), "Research with fake clock", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start research run: %v", err)
	}
	postWorkerMessage := func(content string) uint64 {
		t.Helper()
		seq, err := rt.ChannelCast(WithToolExecutionContext(context.Background(), researchRun), docID, "vtext:"+docID, "", "researcher-1", "researcher", content)
		if err != nil {
			t.Fatalf("post worker message %q: %v", content, err)
		}
		return seq
	}
	seqA := postWorkerMessage("Fake-clock evidence A.")
	seqB := postWorkerMessage("Fake-clock evidence B.")

	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs before clock fires: %v", err)
	}
	for _, run := range runs {
		if agentProfileForRun(&run) == AgentProfileVText && run.ParentRunID == researchRun.RunID {
			t.Fatalf("wake run started before fake clock fired: %+v", run)
		}
	}

	clock.fireAll()
	revs := waitForRevisionCount(t, s, docID, "user-1", 2, 5*time.Second)
	var agentRev *types.Revision
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Integrated fake-clock worker findings") {
			revCopy := rev
			agentRev = &revCopy
			break
		}
	}
	if agentRev == nil {
		t.Fatalf("expected fake-clock wake revision, got %+v", revs)
	}
	agentMeta := decodeRevisionMetadata(agentRev.Metadata)
	consumed := metadataSlice(t, agentMeta, "worker_updates_consumed")
	if len(consumed) != 2 {
		t.Fatalf("worker_updates_consumed length = %d, want 2; metadata=%+v", len(consumed), agentMeta)
	}
	gotSeqs := []int64{
		int64(consumed[0].(map[string]any)["seq"].(float64)),
		int64(consumed[1].(map[string]any)["seq"].(float64)),
	}
	wantSeqs := []int64{int64(seqA), int64(seqB)}
	if gotSeqs[0] != wantSeqs[0] || gotSeqs[1] != wantSeqs[1] {
		t.Fatalf("consumed seqs = %+v, want %+v", gotSeqs, wantSeqs)
	}
}

func TestVTextWorkerWakeRequeuesWhileMutationPending(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Integrated after pending mutation cleared."))
	clock := &fakeVTextWakeClock{}

	h, s, rt := vtextAPISetupWithProviderAndOptions(t, provider, true, withVTextWakeAfterFuncForTest(clock.afterFunc))
	docID, _ := createDocWithUserRevision(t, h)

	blockingRunID := "blocking-vtext-mutation"
	if err := s.CreateAgentMutation(context.Background(), store.AgentMutation{
		DocID:     docID,
		RunID:     blockingRunID,
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create pending mutation: %v", err)
	}

	researchRun, err := rt.StartRunWithMetadata(context.Background(), "Research while vtext mutation is pending", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start research run: %v", err)
	}
	if _, err := rt.ChannelCast(WithToolExecutionContext(context.Background(), researchRun), docID, "vtext:"+docID, "", "researcher-1", "researcher", "Evidence while a previous VText mutation is still pending."); err != nil {
		t.Fatalf("post worker message: %v", err)
	}

	clock.fireAll()
	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs after blocked wake: %v", err)
	}
	for _, run := range runs {
		if agentProfileForRun(&run) == AgentProfileVText && run.ParentRunID == researchRun.RunID {
			t.Fatalf("wake run should wait for pending mutation to clear, got %+v", run)
		}
	}

	if err := s.DeferAgentMutation(context.Background(), blockingRunID); err != nil {
		t.Fatalf("defer blocking mutation: %v", err)
	}
	clock.fireAll()

	revs := waitForRevisionCount(t, s, docID, "user-1", 2, 5*time.Second)
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Integrated after pending mutation cleared.") {
			return
		}
	}
	t.Fatalf("expected requeued wake revision after pending mutation cleared, got %+v", revs)
}

func TestSubmitResearchFindingsWakeUsesSameDebouncedPath(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Integrated persisted findings into the next revision."))
	provider.delay = 500 * time.Millisecond
	clock := &fakeVTextWakeClock{}

	h, s, rt := vtextAPISetupWithProviderAndOptions(t, provider, true, withVTextWakeAfterFuncForTest(clock.afterFunc))
	docID, _ := createDocWithUserRevision(t, h)

	userRevReq := vtextCreateRevisionRequest{
		Content:     "Original draft.\n\nNeed a sourced update.",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
	}
	userRevReqBody := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", userRevReq)
	userRevW := httptest.NewRecorder()
	h.HandleVTextRevisions(userRevW, userRevReqBody)
	if userRevW.Code != http.StatusCreated {
		t.Fatalf("second user revision: status = %d, want %d; body: %s", userRevW.Code, http.StatusCreated, userRevW.Body.String())
	}

	vtextRun, err := rt.StartRunWithMetadata(context.Background(), "Own the document", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataChannelID:    docID,
		runMetadataAgentID:      "vtext:" + docID,
		"doc_id":                docID,
	})
	if err != nil {
		t.Fatalf("start vtext run: %v", err)
	}
	waitForRunRunning(t, rt, vtextRun.RunID, "user-1", 5*time.Second)
	researcherRun, err := rt.StartChildRun(context.Background(), vtextRun.RunID, "Research the update", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start researcher run: %v", err)
	}

	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	raw, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherRun), "update_coagent", json.RawMessage(`{
		"update_id":"finding-stream-001",
		"findings":["A new release landed this week."],
		"evidence":[
			{
				"kind":"web_page",
				"source_uri":"https://example.com/release",
				"title":"Release notes",
				"content":"The release notes describe the new capabilities."
			}
		],
		"notes":["Prefer a brief update in the next draft."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent: %v", err)
	}
	var findingResp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &findingResp); err != nil {
		t.Fatalf("decode update_coagent: %v", err)
	}
	if findingResp.Status != "submitted" {
		t.Fatalf("update_coagent status = %q, want submitted", findingResp.Status)
	}

	clock.fireAll()
	revs := waitForRevisionCount(t, s, docID, "user-1", 3, 5*time.Second)
	foundAppAgent := false
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Integrated persisted findings") {
			foundAppAgent = true
			break
		}
	}
	if !foundAppAgent {
		t.Fatalf("expected findings-driven appagent revision, got %+v", revs)
	}

	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs: %v", err)
	}
	var wakeRun *types.RunRecord
	for i := range runs {
		if agentProfileForRun(&runs[i]) == AgentProfileVText && runs[i].ParentRunID == researcherRun.RunID {
			wakeRun = &runs[i]
			break
		}
	}
	if wakeRun == nil {
		t.Fatalf("expected findings-driven vtext wake run on channel %s, got %+v", docID, runs)
	}
	evidenceID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("choir:research-finding:finding-stream-001:0")).String()
	if !strings.Contains(wakeRun.Prompt, evidenceID) {
		t.Fatalf("wake run prompt missing durable evidence handle %s: %q", evidenceID, wakeRun.Prompt)
	}
	evidence, err := s.GetEvidence(context.Background(), evidenceID, "user-1")
	if err != nil {
		t.Fatalf("evidence handle %s in wake prompt does not resolve: %v", evidenceID, err)
	}
	if evidence.Title != "Release notes" {
		t.Fatalf("evidence title = %q, want %q", evidence.Title, "Release notes")
	}
}

func TestSubmitWorkerUpdateWakeUsesSameDebouncedPath(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Integrated structured super update into the next revision."))
	provider.delay = 500 * time.Millisecond
	clock := &fakeVTextWakeClock{}

	h, s, rt := vtextAPISetupWithProviderAndOptions(t, provider, true, withVTextWakeAfterFuncForTest(clock.afterFunc))
	docID, _ := createDocWithUserRevision(t, h)

	userRevReq := vtextCreateRevisionRequest{
		Content:     "Original draft.\n\nNeed execution artifacts and verification results.",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
	}
	userRevReqBody := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", userRevReq)
	userRevW := httptest.NewRecorder()
	h.HandleVTextRevisions(userRevW, userRevReqBody)
	if userRevW.Code != http.StatusCreated {
		t.Fatalf("second user revision: status = %d, want %d; body: %s", userRevW.Code, http.StatusCreated, userRevW.Body.String())
	}

	vtextRun, err := rt.StartRunWithMetadata(context.Background(), "Own the document", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataChannelID:    docID,
		runMetadataAgentID:      "vtext:" + docID,
		"doc_id":                docID,
	})
	if err != nil {
		t.Fatalf("start vtext run: %v", err)
	}
	waitForRunRunning(t, rt, vtextRun.RunID, "user-1", 5*time.Second)
	superRun, err := rt.StartChildRun(context.Background(), vtextRun.RunID, "Build and verify a toy artifact", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start super run: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superRun), "update_coagent", json.RawMessage(`{
		"update_id":"super-artifact-001",
		"agent_id":"vtext:`+docID+`",
		"artifacts":["artifacts/evolution-ca.html"],
		"tests":["node artifacts/evolution-ca.verify.js passed"],
		"proposals":["Mention the generated visualization and verification result in the next version."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent: %v", err)
	}
	var updateResp struct {
		Status string `json:"status"`
		Cursor int64  `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(raw), &updateResp); err != nil {
		t.Fatalf("decode update_coagent: %v", err)
	}
	if updateResp.Status != "submitted" || updateResp.Cursor == 0 {
		t.Fatalf("update_coagent response = %+v, want submitted with cursor", updateResp)
	}

	clock.fireAll()
	revs := waitForRevisionCount(t, s, docID, "user-1", 3, 5*time.Second)
	var agentRev *types.Revision
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Integrated structured super update") {
			revCopy := rev
			agentRev = &revCopy
			break
		}
	}
	if agentRev == nil {
		t.Fatalf("expected structured-update appagent revision, got %+v", revs)
	}
	agentMeta := decodeRevisionMetadata(agentRev.Metadata)
	if !metadataSeqContains(t, agentMeta, "worker_updates_consumed", uint64(updateResp.Cursor)) {
		t.Fatalf("worker update seq %d was not consumed; metadata=%+v", updateResp.Cursor, agentMeta)
	}

	update, err := s.GetWorkerUpdate(context.Background(), "user-1", "super-artifact-001")
	if err != nil {
		t.Fatalf("get worker update: %v", err)
	}
	if len(update.Artifacts) != 1 || update.Artifacts[0] != "artifacts/evolution-ca.html" {
		t.Fatalf("artifacts = %+v", update.Artifacts)
	}
	if len(update.Tests) != 1 || !strings.Contains(update.Tests[0], "passed") {
		t.Fatalf("tests = %+v", update.Tests)
	}

	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs: %v", err)
	}
	var wakeRun *types.RunRecord
	for i := range runs {
		if agentProfileForRun(&runs[i]) == AgentProfileVText && runs[i].ParentRunID == superRun.RunID {
			wakeRun = &runs[i]
			break
		}
	}
	if wakeRun == nil {
		t.Fatalf("expected structured worker update wake run on channel %s, got %+v", docID, runs)
	}
	if !strings.Contains(wakeRun.Prompt, "artifacts/evolution-ca.html") || !strings.Contains(wakeRun.Prompt, "evolution-ca.verify.js passed") {
		t.Fatalf("wake run prompt missing structured worker update context: %q", wakeRun.Prompt)
	}
}

func TestVTextWorkerMessageDuringActiveRevisionTriggersLaterFollowUp(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Integrated content after the run completed."))
	provider.delay = 300 * time.Millisecond

	h, s, rt := vtextAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Produce the next draft now"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var initialResp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&initialResp); err != nil {
		t.Fatalf("decode initial agent revision response: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	initialRunStarted := false
	for time.Now().Before(deadline) {
		rec, err := rt.GetRun(context.Background(), initialResp.RunID, "user-1")
		if err != nil {
			t.Fatalf("get initial vtext run: %v", err)
		}
		if rec.State == types.RunRunning {
			initialRunStarted = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !initialRunStarted {
		t.Fatalf("initial vtext run never reached running state before posting the late worker message")
	}

	researchRun, err := rt.StartRunWithMetadata(context.Background(), "Send one late finding", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start research run: %v", err)
	}
	lateSeq, err := rt.ChannelCast(WithToolExecutionContext(context.Background(), researchRun), docID, "vtext:"+docID, "", "researcher-1", "researcher", "Late finding: a sourced correction arrived while the vtext run was already active.")
	if err != nil {
		t.Fatalf("post late worker message: %v", err)
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 3, 8*time.Second)
	var appAgentContents []string
	var appAgentRevs []types.Revision
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent {
			appAgentContents = append(appAgentContents, rev.Content)
			appAgentRevs = append(appAgentRevs, rev)
		}
	}
	if len(appAgentContents) != 2 {
		t.Fatalf("expected two appagent revisions, got %d: %+v", len(appAgentContents), revs)
	}
	for _, content := range appAgentContents {
		if !strings.Contains(content, "Integrated content after the run completed.") {
			t.Fatalf("unexpected appagent revision content: %+v", appAgentContents)
		}
	}
	foundPending := false
	foundConsumed := false
	for _, rev := range appAgentRevs {
		meta := decodeRevisionMetadata(rev.Metadata)
		if metadataSeqContains(t, meta, "worker_updates_pending", lateSeq) {
			foundPending = true
		}
		if metadataSeqContains(t, meta, "worker_updates_consumed", lateSeq) {
			foundConsumed = true
		}
	}
	if !foundPending {
		t.Fatalf("expected one appagent revision to record late worker update %d as pending; revs=%+v", lateSeq, appAgentRevs)
	}
	if !foundConsumed {
		t.Fatalf("expected follow-up appagent revision to record late worker update %d as consumed; revs=%+v", lateSeq, appAgentRevs)
	}

	var wakeRun *types.RunRecord
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
		if err != nil {
			t.Fatalf("list channel runs: %v", err)
		}
		for i := range runs {
			if agentProfileForRun(&runs[i]) == AgentProfileVText && runs[i].ParentRunID == researchRun.RunID && runs[i].RunID != initialResp.RunID {
				wakeRun = &runs[i]
				break
			}
		}
		if wakeRun != nil && wakeRun.State.Terminal() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if wakeRun == nil {
		t.Fatalf("expected a follow-up vtext wake run after the active run completed")
	}
	if !strings.Contains(wakeRun.Prompt, "Late finding: a sourced correction arrived while the vtext run was already active.") {
		t.Fatalf("wake run prompt missing late worker message: %q", wakeRun.Prompt)
	}

	checkpoint, err := s.GetVTextControllerCheckpoint(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get controller checkpoint: %v", err)
	}
	if checkpoint == nil || checkpoint.IntegratedMessageSeq == 0 {
		t.Fatalf("expected controller checkpoint to advance after follow-up, got %+v", checkpoint)
	}

	pending, err := s.GetPendingAgentMutationByDoc(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get pending mutation: %v", err)
	}
	if pending != nil {
		t.Fatalf("expected no pending mutation after both revisions completed, got %+v", pending)
	}
}

func TestBuildAgentRevisionRequestRequiresSuperContinuationForActiveWorker(t *testing.T) {
	t.Parallel()
	current := types.Revision{
		DocID:      "doc-active-worker-dashboard",
		RevisionID: "rev-active-worker-dashboard",
		Content:    "Current dashboard.",
		AuthorKind: types.AuthorAppAgent,
	}
	recent := []ChannelMessage{{
		Role:    AgentProfileSuper,
		From:    "super:active-worker",
		Content: "Worker update ready.\n\nFindings:\n- delegate_worker_vm returned status \"worker_run_active\" with worker state \"running\".\n\nNotes:\n- active_worker_obligation=true\n- finish_ready=false\n\nRefs:\n- worker_run:worker-run-active",
	}}

	prompt := buildAgentRevisionRequest(current, nil, nil, vtextAgentRevisionRequest{
		Intent: "integrate_worker_findings",
	}, "", true, recent, nil)

	for _, want := range []string{
		"At least one recent worker message says a delegated worker is still active",
		"call request_super_execution",
		"continue the existing worker_run_id",
		"not start a duplicate worker",
		"VText must not directly control worker/vsuper/co-super runs",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("active-worker vtext prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestRestartRecoveryClearsInterruptedVTextMutationAndRelaunches(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(os.TempDir(), "go-choir-m3-vtext-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	ctx := context.Background()
	s1, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 1: %v", err)
	}

	now := time.Now().UTC()
	doc := types.Document{
		DocID:     "doc-restart-reconcile",
		OwnerID:   "user-1",
		Title:     "Restart reconcile",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s1.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	userRev := types.Revision{
		RevisionID:  "rev-user-restart",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "Base draft before the restart.",
		CreatedAt:   now.Add(1 * time.Second),
	}
	if err := s1.CreateRevision(ctx, userRev); err != nil {
		t.Fatalf("create user revision: %v", err)
	}

	researchRun := types.RunRecord{
		RunID:     "research-run-restart",
		OwnerID:   "user-1",
		SandboxID: "sandbox-vtext-test",
		ChannelID: doc.DocID,
		State:     types.RunCompleted,
		Prompt:    "Gather the missing fact",
		CreatedAt: now.Add(2 * time.Second),
		UpdatedAt: now.Add(2 * time.Second),
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileResearcher,
			runMetadataAgentRole:    AgentProfileResearcher,
			runMetadataChannelID:    doc.DocID,
		},
	}
	if err := s1.CreateRun(ctx, researchRun); err != nil {
		t.Fatalf("create research run: %v", err)
	}

	message := &types.ChannelMessage{
		ChannelID:    doc.DocID,
		TrajectoryID: "traj-restart-1",
		From:         "researcher",
		FromRunID:    researchRun.RunID,
		FromAgentID:  "researcher-1",
		ToAgentID:    "vtext:" + doc.DocID,
		Role:         "researcher",
		Content:      "Durable finding: the corrected fact landed while the sandbox was about to restart.",
		Timestamp:    now.Add(3 * time.Second),
	}
	if err := s1.AppendChannelMessage(ctx, message, "user-1"); err != nil {
		t.Fatalf("append channel message: %v", err)
	}

	interruptedRun := types.RunRecord{
		RunID:       "vtext-interrupted-restart",
		OwnerID:     "user-1",
		SandboxID:   "sandbox-vtext-test",
		ChannelID:   doc.DocID,
		State:       types.RunRunning,
		Prompt:      "Integrate the durable finding",
		ParentRunID: researchRun.RunID,
		CreatedAt:   now.Add(4 * time.Second),
		UpdatedAt:   now.Add(4 * time.Second),
		Metadata: map[string]any{
			"type":                  "vtext_agent_revision",
			"doc_id":                doc.DocID,
			"current_revision_id":   userRev.RevisionID,
			"scheduled_message_seq": message.Seq,
			runMetadataAgentProfile: AgentProfileVText,
			runMetadataAgentRole:    AgentProfileVText,
			runMetadataChannelID:    doc.DocID,
			runMetadataAgentID:      "vtext:" + doc.DocID,
			runMetadataTrajectoryID: message.TrajectoryID,
		},
	}
	if err := s1.CreateRun(ctx, interruptedRun); err != nil {
		t.Fatalf("create interrupted vtext run: %v", err)
	}
	if err := s1.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:               doc.DocID,
		RunID:               interruptedRun.RunID,
		OwnerID:             "user-1",
		State:               "pending",
		ScheduledMessageSeq: message.Seq,
		CreatedAt:           now.Add(4 * time.Second),
	}); err != nil {
		t.Fatalf("create interrupted mutation: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close store 1: %v", err)
	}

	s2, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 2: %v", err)
	}
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Recovered after restart and integrated the durable finding."))
	provider.delay = 20 * time.Millisecond
	rt := New(Config{
		SandboxID:           "sandbox-vtext-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     2 * time.Second,
		SupervisionInterval: 5 * time.Second,
		VTextWakeDebounce:   50 * time.Millisecond,
	}, s2, events.NewEventBus(), provider)
	if err := rt.InstallDefaultAgentTools(""); err != nil {
		t.Fatalf("install default agent tools after restart: %v", err)
	}

	t.Cleanup(func() {
		rt.Stop()
		_ = s2.Close()
		_ = os.RemoveAll(promptRoot)
	})
	rt.Start(ctx)

	revs := waitForRevisionCount(t, s2, doc.DocID, "user-1", 2, 5*time.Second)
	waitForVTextQuiescent(t, rt, s2, "user-1", doc.DocID, uint64(message.Seq), 5*time.Second)
	foundRecoveredRevision := false
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Recovered after restart and integrated the durable finding.") {
			foundRecoveredRevision = true
		}
	}
	if !foundRecoveredRevision {
		t.Fatalf("expected recovered appagent revision, got %+v", revs)
	}

	gotInterrupted, err := rt.GetRun(ctx, interruptedRun.RunID, "user-1")
	if err != nil {
		t.Fatalf("get interrupted run after restart: %v", err)
	}
	if gotInterrupted.State != types.RunPassivated {
		t.Fatalf("interrupted run state = %q, want %q", gotInterrupted.State, types.RunPassivated)
	}
	if gotInterrupted.Error != "" {
		t.Fatalf("interrupted run error = %q, want empty", gotInterrupted.Error)
	}

	mutation, err := s2.GetAgentMutationByRun(ctx, interruptedRun.RunID)
	if err != nil {
		t.Fatalf("get interrupted mutation: %v", err)
	}
	if mutation == nil || mutation.State != "stale_activation" {
		t.Fatalf("expected interrupted mutation to be stale after recovery, got %+v", mutation)
	}
	pending, err := s2.GetPendingAgentMutationByDoc(ctx, doc.DocID, "user-1")
	if err != nil {
		t.Fatalf("get pending mutation after recovery: %v", err)
	}
	if pending != nil {
		t.Fatalf("expected no pending mutation after recovery replay, got %+v", pending)
	}

	runs, err := s2.ListRunsByChannel(ctx, "user-1", doc.DocID, 20)
	if err != nil {
		t.Fatalf("list channel runs after restart: %v", err)
	}
	var recoveredRun *types.RunRecord
	for i := range runs {
		if agentProfileForRun(&runs[i]) == AgentProfileVText && runs[i].RunID != interruptedRun.RunID {
			recoveredRun = &runs[i]
			break
		}
	}
	if recoveredRun == nil {
		t.Fatalf("expected a replacement vtext run after restart, got %+v", runs)
	}
	if recoveredRun.ParentRunID != researchRun.RunID {
		t.Fatalf("replacement run parent = %q, want %q", recoveredRun.ParentRunID, researchRun.RunID)
	}
	if !strings.Contains(recoveredRun.Prompt, "Durable finding: the corrected fact landed while the sandbox was about to restart.") {
		t.Fatalf("replacement run prompt missing durable finding: %q", recoveredRun.Prompt)
	}

	checkpoint, err := s2.GetVTextControllerCheckpoint(ctx, doc.DocID, "user-1")
	if err != nil {
		t.Fatalf("get controller checkpoint after recovery: %v", err)
	}
	if checkpoint == nil || checkpoint.IntegratedMessageSeq != message.Seq {
		t.Fatalf("checkpoint after recovery = %+v, want integrated_message_seq=%d", checkpoint, message.Seq)
	}
}

func TestHandleTestVTextResearchFindingsUsesResearcherToolPath(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Browser test findings revision."))

	h, s, rt := vtextAPISetupWithProvider(t, provider, true)
	rt.cfg.EnableTestAPIs = true

	docID, _ := createDocWithUserRevision(t, h)

	revReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Write the first draft"})
	revW := httptest.NewRecorder()
	h.HandleVTextAgentRevision(revW, revReq)
	if revW.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", revW.Code, http.StatusAccepted, revW.Body.String())
	}
	var revResp vtextAgentRevisionResponse
	if err := json.NewDecoder(revW.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode agent revision response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, revResp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("agent revision state = %q, want %q", state, types.RunCompleted)
	}

	req := vtextRequest(t, http.MethodPost, "/api/test/vtext/research-findings", map[string]any{
		"doc_id":     docID,
		"finding_id": "browser-hook-001",
		"findings":   []string{"A sourced update arrived."},
		"notes":      []string{"Fold this into the next revision."},
	})
	w := httptest.NewRecorder()
	h.HandleTestVTextResearchFindings(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("test findings status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode findings response: %v", err)
	}
	if got, _ := resp["status"].(string); got != "submitted" {
		t.Fatalf("status = %q, want submitted", got)
	}
	if got, _ := resp["loop_id"].(string); strings.TrimSpace(got) == "" {
		t.Fatal("loop_id should not be empty")
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 3, 5*time.Second)
	found := false
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Browser test findings revision.") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected findings-driven revision, got %+v", revs)
	}
}

func TestHandleTestVTextWorkerUpdateUsesStructuredToolPath(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Browser test structured worker update revision."))

	h, s, rt := vtextAPISetupWithProvider(t, provider, true)
	rt.cfg.EnableTestAPIs = true

	docID, _ := createDocWithUserRevision(t, h)

	revReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Write the first draft"})
	revW := httptest.NewRecorder()
	h.HandleVTextAgentRevision(revW, revReq)
	if revW.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", revW.Code, http.StatusAccepted, revW.Body.String())
	}
	var revResp vtextAgentRevisionResponse
	if err := json.NewDecoder(revW.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode agent revision response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, revResp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("agent revision state = %q, want %q", state, types.RunCompleted)
	}

	req := vtextRequest(t, http.MethodPost, "/api/test/vtext/worker-update", map[string]any{
		"doc_id":       docID,
		"update_id":    "browser-worker-update-001",
		"role":         "super",
		"artifacts":    []string{"artifacts/evolution-ca.html"},
		"tests":        []string{"node artifacts/evolution-ca.verify.js passed"},
		"proposals":    []string{"Mention the verified visualization in the next draft."},
		"evidence_ids": []string{"evidence-browser-001"},
	})
	w := httptest.NewRecorder()
	h.HandleTestVTextWorkerUpdate(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("test worker update status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode worker update response: %v", err)
	}
	if got, _ := resp["status"].(string); got != "submitted" {
		t.Fatalf("status = %q, want submitted", got)
	}
	workerLoopID, _ := resp["loop_id"].(string)
	if strings.TrimSpace(workerLoopID) == "" {
		t.Fatal("loop_id should not be empty")
	}
	workerRun, err := s.GetRun(context.Background(), workerLoopID)
	if err != nil {
		t.Fatalf("get worker loop: %v", err)
	}
	if workerRun.ParentRunID != revResp.RunID {
		t.Fatalf("worker loop parent = %q, want vtext run %q", workerRun.ParentRunID, revResp.RunID)
	}
	vtextRun, err := s.GetRun(context.Background(), revResp.RunID)
	if err != nil {
		t.Fatalf("get vtext loop: %v", err)
	}

	update, err := s.GetWorkerUpdate(context.Background(), "user-1", "browser-worker-update-001")
	if err != nil {
		t.Fatalf("get worker update: %v", err)
	}
	if update.Role != AgentProfileSuper || len(update.Artifacts) != 1 || len(update.Tests) != 1 || len(update.Proposals) != 1 {
		t.Fatalf("unexpected structured update: %+v", update)
	}
	if update.TrajectoryID != trajectoryIDForRun(&workerRun) || update.TrajectoryID != trajectoryIDForRun(&vtextRun) {
		t.Fatalf("worker update trajectory = %q, worker trajectory = %q, vtext trajectory = %q", update.TrajectoryID, trajectoryIDForRun(&workerRun), trajectoryIDForRun(&vtextRun))
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 3, 5*time.Second)
	found := false
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "Browser test structured worker update revision.") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected worker-update-driven revision, got %+v", revs)
	}
}

func TestVTextAgentRevisionInheritsConductorTrajectoryFromRevisionMetadata(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Conductor-linked vtext revision."))

	h, s, _ := vtextAPISetupWithProvider(t, provider, true)
	ctx := context.Background()

	now := time.Now().UTC()
	conductorRun := types.RunRecord{
		RunID:        "conductor-parent-001",
		AgentID:      "conductor:test",
		ChannelID:    "conductor-parent-001",
		AgentProfile: AgentProfileConductor,
		AgentRole:    AgentProfileConductor,
		OwnerID:      "user-1",
		SandboxID:    "sandbox-vtext-test",
		State:        types.RunCompleted,
		Prompt:       "Create a research-backed working document.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileConductor,
			runMetadataAgentRole:    AgentProfileConductor,
			runMetadataAgentID:      "conductor:test",
			runMetadataChannelID:    "conductor-parent-001",
			runMetadataTrajectoryID: "trajectory-conductor-parent-001",
		},
	}
	if err := s.CreateRun(ctx, conductorRun); err != nil {
		t.Fatalf("create conductor parent run: %v", err)
	}

	docID, baseRevisionID := createDocWithUserRevision(t, h)
	metadata, _ := json.Marshal(map[string]any{
		"seed_prompt":       "Create a research-backed working document.",
		"conductor_loop_id": conductorRun.RunID,
	})
	revReq := vtextCreateRevisionRequest{
		Content:          "User refined the conductor-framed working document.",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		Metadata:         metadata,
		ParentRevisionID: baseRevisionID,
	}
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", revReq)
	w := httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create conductor-linked revision: status = %d, body: %s", w.Code, w.Body.String())
	}

	agentReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"intent": "revise"})
	agentW := httptest.NewRecorder()
	h.HandleVTextAgentRevision(agentW, agentReq)
	if agentW.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", agentW.Code, http.StatusAccepted, agentW.Body.String())
	}
	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(agentW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode agent revision response: %v", err)
	}

	vtextRun, err := s.GetRun(ctx, resp.RunID)
	if err != nil {
		t.Fatalf("get vtext run: %v", err)
	}
	if vtextRun.ParentRunID != conductorRun.RunID {
		t.Fatalf("vtext run parent = %q, want conductor %q", vtextRun.ParentRunID, conductorRun.RunID)
	}
	if trajectoryIDForRun(&vtextRun) != trajectoryIDForRun(&conductorRun) {
		t.Fatalf("vtext trajectory = %q, want conductor trajectory %q", trajectoryIDForRun(&vtextRun), trajectoryIDForRun(&conductorRun))
	}
	if state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("agent revision state = %q, want %q", state, types.RunCompleted)
	}
}

func TestVTextOpenFileResolvesCanonicalAlias(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)

	openReq := func(initialContent string) *httptest.ResponseRecorder {
		req := vtextRequest(t, http.MethodPost, "/api/vtext/files/open", map[string]string{
			"source_path":     "notes/ai-news.md",
			"title":           "ai-news.md",
			"initial_content": initialContent,
		})
		w := httptest.NewRecorder()
		h.HandleVTextRouter(w, req)
		return w
	}

	first := openReq("Initial file content")
	if first.Code != http.StatusCreated {
		t.Fatalf("first open file: status = %d, want %d; body: %s", first.Code, http.StatusCreated, first.Body.String())
	}
	var firstResp vtextOpenFileResponse
	if err := json.NewDecoder(first.Body).Decode(&firstResp); err != nil {
		t.Fatalf("decode first open file response: %v", err)
	}
	if !firstResp.Created {
		t.Fatalf("first open created = false, want true")
	}
	if firstResp.OriginalContentID == "" {
		t.Fatalf("first open original_content_id is empty")
	}
	doc, err := s.GetDocument(context.Background(), firstResp.DocID, "user-1")
	if err != nil {
		t.Fatalf("GetDocument opened file: %v", err)
	}
	if doc.Title != "ai-news.vtext" {
		t.Fatalf("opened document title = %q, want ai-news.vtext", doc.Title)
	}

	second := openReq("Changed file bytes that should not fork a new doc")
	if second.Code != http.StatusOK {
		t.Fatalf("second open file: status = %d, want %d; body: %s", second.Code, http.StatusOK, second.Body.String())
	}
	var secondResp vtextOpenFileResponse
	if err := json.NewDecoder(second.Body).Decode(&secondResp); err != nil {
		t.Fatalf("decode second open file response: %v", err)
	}
	if secondResp.Created {
		t.Fatalf("second open created = true, want false")
	}
	if secondResp.DocID != firstResp.DocID {
		t.Fatalf("second open doc_id = %q, want %q", secondResp.DocID, firstResp.DocID)
	}

	revs, err := s.ListRevisionsByDoc(context.Background(), firstResp.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("len(revisions) = %d, want 1", len(revs))
	}
	if revs[0].Content != "Initial file content" {
		t.Fatalf("initial aliased revision content = %q, want initial file content", revs[0].Content)
	}
	meta := decodeRevisionMetadata(revs[0].Metadata)
	if meta["created_from"] != "file_open" || meta["source_path"] != "notes/ai-news.md" {
		t.Fatalf("file-open metadata = %#v", meta)
	}
	importManifest, ok := meta["import_manifest"].(map[string]any)
	if !ok {
		t.Fatalf("missing import_manifest: %#v", meta)
	}
	if importManifest["projection_kind"] != "vtext" || importManifest["source_kind"] != "md" || importManifest["original_content_hash"] == "" {
		t.Fatalf("import manifest = %#v", importManifest)
	}
	if importManifest["original_content_id"] != firstResp.OriginalContentID {
		t.Fatalf("import manifest original_content_id = %q, want %q", importManifest["original_content_id"], firstResp.OriginalContentID)
	}
	originalItem, err := s.GetContentItem(context.Background(), "user-1", firstResp.OriginalContentID)
	if err != nil {
		t.Fatalf("GetContentItem original: %v", err)
	}
	if originalItem.FilePath != "notes/ai-news.md" || originalItem.MediaType != "text/markdown" || originalItem.AppHint != "vtext" {
		t.Fatalf("original content item = %#v", originalItem)
	}
	if originalItem.TextContent != "Initial file content" || originalItem.ContentHash == "" {
		t.Fatalf("original text/hash = %#v", originalItem)
	}
	migrationManifest, ok := meta["migration_manifest"].(map[string]any)
	if !ok {
		t.Fatalf("missing migration_manifest: %#v", meta)
	}
	if migrationManifest["migration_adapter"] != "markdown_to_vtext_projection" || migrationManifest["source_gap_policy"] != "repairable_gap_no_invented_citations" {
		t.Fatalf("migration manifest = %#v", migrationManifest)
	}
	exportReq := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+firstResp.DocID+"/export?format=md", nil)
	exportW := httptest.NewRecorder()
	h.HandleVTextRouter(exportW, exportReq)
	if exportW.Code != http.StatusOK {
		t.Fatalf("export markdown: status = %d, want %d; body: %s", exportW.Code, http.StatusOK, exportW.Body.String())
	}
	var exported vtextDocumentExportResponse
	if err := json.NewDecoder(exportW.Body).Decode(&exported); err != nil {
		t.Fatalf("decode export response: %v", err)
	}
	if exported.Format != "md" || exported.Filename != "ai-news.md" || exported.Content != "Initial file content" || exported.ContentHash == "" {
		t.Fatalf("export response = %#v", exported)
	}
}

func TestVTextPlainTextImportCarriesMigrationMetadataToFirstDurableRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	ctx := context.Background()

	initialContent := strings.Join([]string{
		"Plain text proposal",
		"",
		"Imported source text should become canonical VText.",
	}, "\n")
	openReq := vtextRequest(t, http.MethodPost, "/api/vtext/files/open", map[string]string{
		"source_path":     "notes/plain-proposal.txt",
		"title":           "plain-proposal.txt",
		"initial_content": initialContent,
	})
	openW := httptest.NewRecorder()
	h.HandleVTextRouter(openW, openReq)
	if openW.Code != http.StatusCreated {
		t.Fatalf("open text file: status = %d, want %d; body: %s", openW.Code, http.StatusCreated, openW.Body.String())
	}
	var opened vtextOpenFileResponse
	if err := json.NewDecoder(openW.Body).Decode(&opened); err != nil {
		t.Fatalf("decode open text response: %v", err)
	}
	if opened.OriginalContentID == "" {
		t.Fatalf("open text original_content_id is empty")
	}

	doc, err := s.GetDocument(ctx, opened.DocID, "user-1")
	if err != nil {
		t.Fatalf("GetDocument opened text: %v", err)
	}
	if doc.Title != "plain-proposal.vtext" {
		t.Fatalf("opened document title = %q, want plain-proposal.vtext", doc.Title)
	}
	v0, err := s.GetRevision(ctx, opened.CurrentRevisionID, "user-1")
	if err != nil {
		t.Fatalf("GetRevision v0: %v", err)
	}
	v0Meta := decodeRevisionMetadata(v0.Metadata)
	v0ImportManifest, ok := v0Meta["import_manifest"].(map[string]any)
	if !ok {
		t.Fatalf("v0 missing import_manifest: %#v", v0Meta)
	}
	if v0ImportManifest["source_media_type"] != "text/plain" || v0ImportManifest["projection_kind"] != "vtext" {
		t.Fatalf("v0 import_manifest = %#v", v0ImportManifest)
	}
	v0MigrationManifest, ok := v0Meta["migration_manifest"].(map[string]any)
	if !ok {
		t.Fatalf("v0 missing migration_manifest: %#v", v0Meta)
	}
	if v0MigrationManifest["source_kind"] != "text" ||
		v0MigrationManifest["source_media_type"] != "text/plain" ||
		v0MigrationManifest["migration_adapter"] != "plain_text_to_vtext_projection" ||
		v0MigrationManifest["projection_kind"] != "vtext" {
		t.Fatalf("v0 migration_manifest = %#v", v0MigrationManifest)
	}

	v1Content := initialContent + "\n\nFirst durable revision keeps import lineage."
	revReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+opened.DocID+"/revisions", vtextCreateRevisionRequest{
		Content:          v1Content,
		ParentRevisionID: opened.CurrentRevisionID,
		Metadata:         json.RawMessage(`{"created_from":"plain_text_v1_user_edit"}`),
	})
	revW := httptest.NewRecorder()
	h.HandleVTextRevisions(revW, revReq)
	if revW.Code != http.StatusCreated {
		t.Fatalf("create text v1: status = %d, want %d; body: %s", revW.Code, http.StatusCreated, revW.Body.String())
	}
	var v1 vtextRevisionResponse
	if err := json.NewDecoder(revW.Body).Decode(&v1); err != nil {
		t.Fatalf("decode text v1: %v", err)
	}
	if v1.VersionNumber != 1 || v1.ParentRevisionID != opened.CurrentRevisionID {
		t.Fatalf("v1 response = %#v", v1)
	}
	v1Meta := decodeRevisionMetadata(v1.Metadata)
	if v1Meta["import_manifest"] == nil || v1Meta["migration_manifest"] == nil {
		t.Fatalf("v1 did not carry import/migration metadata: %#v", v1Meta)
	}
	if v1Meta["canonical_vtext_source_path"] == nil {
		t.Fatalf("v1 missing canonical_vtext_source_path: %#v", v1Meta)
	}
	v1MigrationManifest := v1Meta["migration_manifest"].(map[string]any)
	if v1MigrationManifest["migration_adapter"] != "plain_text_to_vtext_projection" {
		t.Fatalf("v1 migration_manifest = %#v", v1MigrationManifest)
	}

	sourcePath, err := s.GetDocumentAliasSourcePath(ctx, "user-1", opened.DocID)
	if err != nil {
		t.Fatalf("GetDocumentAliasSourcePath: %v", err)
	}
	if filepath.Ext(sourcePath) != ".vtext" {
		t.Fatalf("latest alias source_path = %q, want .vtext", sourcePath)
	}
	if docID, err := s.GetDocumentAlias(ctx, "user-1", "notes/plain-proposal.txt"); err != nil || docID != opened.DocID {
		t.Fatalf("original text alias docID = %q, err = %v, want %q", docID, err, opened.DocID)
	}

	exportReq := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+opened.DocID+"/export?format=md", nil)
	exportW := httptest.NewRecorder()
	h.HandleVTextRouter(exportW, exportReq)
	if exportW.Code != http.StatusOK {
		t.Fatalf("export text as markdown: status = %d, want %d; body: %s", exportW.Code, http.StatusOK, exportW.Body.String())
	}
	var exported vtextDocumentExportResponse
	if err := json.NewDecoder(exportW.Body).Decode(&exported); err != nil {
		t.Fatalf("decode text export: %v", err)
	}
	if exported.Format != "md" || exported.Filename != "plain-proposal.md" || exported.Content != v1Content || exported.RevisionID != v1.RevisionID {
		t.Fatalf("exported text response = %#v", exported)
	}
}

func TestVTextImportedMarkdownRevisionUsesVTextProjectionAndPreservesCollapsedTable(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	ctx := context.Background()

	initialContent := strings.Join([]string{
		"# Legal Cloud",
		"",
		"Appendix A: Glossary",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Work product | Durable output of professional work. |",
		"| Vector database | Storage optimized for similarity search. |",
		"",
		"Closing paragraph.",
	}, "\n")
	openReq := vtextRequest(t, http.MethodPost, "/api/vtext/files/open", map[string]string{
		"source_path":     "proposals/legal-cloud.md",
		"title":           "legal-cloud.md",
		"initial_content": initialContent,
	})
	openW := httptest.NewRecorder()
	h.HandleVTextRouter(openW, openReq)
	if openW.Code != http.StatusCreated {
		t.Fatalf("open markdown: status = %d, want %d; body: %s", openW.Code, http.StatusCreated, openW.Body.String())
	}
	var opened vtextOpenFileResponse
	if err := json.NewDecoder(openW.Body).Decode(&opened); err != nil {
		t.Fatalf("decode open response: %v", err)
	}

	collapsedDraft := strings.Join([]string{
		"# Legal Cloud",
		"",
		"Appendix A: Glossary",
		"",
		"TermDefinitionWork productDurable output of professional work.Vector databaseStorage optimized for similarity search.",
		"",
		"Closing paragraph with a user edit.",
	}, "\n")
	revReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+opened.DocID+"/revisions", vtextCreateRevisionRequest{
		Content: collapsedDraft,
		Metadata: json.RawMessage(`{
			"source_path":"proposals/legal-cloud.md",
			"created_from":"browser_user_edit"
		}`),
	})
	revW := httptest.NewRecorder()
	h.HandleVTextRevisions(revW, revReq)
	if revW.Code != http.StatusCreated {
		t.Fatalf("create collapsed draft revision: status = %d, want %d; body: %s", revW.Code, http.StatusCreated, revW.Body.String())
	}
	var revision vtextRevisionResponse
	if err := json.NewDecoder(revW.Body).Decode(&revision); err != nil {
		t.Fatalf("decode revision response: %v", err)
	}
	if !strings.Contains(revision.Content, "| Term | Definition |") ||
		!strings.Contains(revision.Content, "| Vector database | Storage optimized for similarity search. |") {
		t.Fatalf("revision did not preserve markdown table:\n%s", revision.Content)
	}
	if strings.Contains(revision.Content, "TermDefinitionWork product") {
		t.Fatalf("revision retained collapsed table artifact:\n%s", revision.Content)
	}
	meta := decodeRevisionMetadata(revision.Metadata)
	if meta["vtext_structure_stabilized"] != true {
		t.Fatalf("metadata did not record structure stabilization: %#v", meta)
	}
	canonicalPath, ok := meta["canonical_vtext_source_path"].(string)
	if !ok || filepath.Ext(canonicalPath) != ".vtext" {
		t.Fatalf("canonical_vtext_source_path = %#v, want .vtext", meta["canonical_vtext_source_path"])
	}
	sourcePath, err := s.GetDocumentAliasSourcePath(ctx, "user-1", opened.DocID)
	if err != nil {
		t.Fatalf("GetDocumentAliasSourcePath: %v", err)
	}
	if filepath.Ext(sourcePath) != ".vtext" {
		t.Fatalf("latest alias source_path = %q, want .vtext", sourcePath)
	}
	if docID, err := s.GetDocumentAlias(ctx, "user-1", "proposals/legal-cloud.md"); err != nil || docID != opened.DocID {
		t.Fatalf("original markdown alias docID = %q, err = %v, want %q", docID, err, opened.DocID)
	}
}

func TestVTextMarkdownStructureStabilizationRepairsMalformedTableTailRow(t *testing.T) {
	t.Parallel()
	parentContent := strings.Join([]string{
		"# Legal Cloud",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Agent | Multi-step worker. |",
		"| Work product | Durable output. |",
		"",
		"Closing paragraph.",
	}, "\n")
	userContent := strings.Join([]string{
		"# Legal Cloud",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Agent | Multi-step worker. |",
		"| Work product | Durable output.",
		"",
		"Closing paragraph with user edit.",
	}, "\n")

	stabilized, changed := stabilizeVTextUserMarkdownStructures(parentContent, userContent)
	if !changed {
		t.Fatalf("stabilization did not report a malformed table-row repair")
	}
	if !strings.Contains(stabilized, "| Work product | Durable output. |") {
		t.Fatalf("stabilized content did not restore final table delimiter:\n%s", stabilized)
	}
	if !strings.Contains(stabilized, "Closing paragraph with user edit.") {
		t.Fatalf("stabilized content dropped user edit:\n%s", stabilized)
	}
}

func TestVTextMarkdownStructureStabilizationHandlesPartialTableContexts(t *testing.T) {
	t.Parallel()
	parentContent := strings.Join([]string{
		"# Appendix",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Agent | Multi-step worker. |",
		"| Work product | Durable output. |",
		"| Vector database | Stores embeddings for retrieval. |",
		"",
		"---",
	}, "\n")
	tests := []struct {
		name      string
		userLines []string
		want      []string
	}{
		{
			name: "table row has no trailing delimiter before document end",
			userLines: []string{
				"# Appendix",
				"",
				"| Term | Definition |",
				"| --- | --- |",
				"| Agent | Multi-step worker. |",
				"| Work product | Durable output. |",
				"| Vector database | Stores embeddings for retrieval.",
			},
			want: []string{"| Vector database | Stores embeddings for retrieval. |"},
		},
		{
			name: "table row has no trailing delimiter before horizontal rule",
			userLines: []string{
				"# Appendix",
				"",
				"| Term | Definition |",
				"| --- | --- |",
				"| Agent | Multi-step worker. |",
				"| Work product | Durable output. |",
				"| Vector database | Stores embeddings for retrieval.",
				"",
				"---",
			},
			want: []string{"| Vector database | Stores embeddings for retrieval. |", "\n---"},
		},
		{
			name: "pipe-prefixed row remains part of table after a small blank gap",
			userLines: []string{
				"# Appendix",
				"",
				"| Term | Definition |",
				"| --- | --- |",
				"| Agent | Multi-step worker. |",
				"| Work product | Durable output. |",
				"",
				"| Vector database | Stores embeddings for retrieval.",
				"",
				"---",
			},
			want: []string{"| Work product | Durable output. |\n| Vector database | Stores embeddings for retrieval. |"},
		},
		{
			name: "bounded table cell edit preserves table identity",
			userLines: []string{
				"# Appendix",
				"",
				"| Term | Definition |",
				"| --- | --- |",
				"| Agent | Multi-step autonomous worker. |",
				"| Work product | Durable output. |",
				"| Vector database | Stores embeddings for retrieval. |",
				"",
				"---",
			},
			want: []string{"| Agent | Multi-step autonomous worker. |", "| Vector database | Stores embeddings for retrieval. |"},
		},
		{
			name: "unrelated edit preserves omitted appendix table",
			userLines: []string{
				"# Appendix",
				"",
				"Intro paragraph with a small owner edit.",
				"",
				"---",
			},
			want: []string{"Intro paragraph with a small owner edit.", "| Vector database | Stores embeddings for retrieval. |", "\n---"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stabilized, _ := stabilizeVTextUserMarkdownStructures(parentContent, strings.Join(tt.userLines, "\n"))
			for _, want := range tt.want {
				if !strings.Contains(stabilized, want) {
					t.Fatalf("stabilized content missing %q:\n%s", want, stabilized)
				}
			}
			if strings.Contains(stabilized, "TermDefinition") {
				t.Fatalf("stabilized content retained collapsed table artifact:\n%s", stabilized)
			}
		})
	}
}

func TestVTextRestoreRevisionNormalizesMalformedTableTailRows(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	createDocReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents", vtextCreateDocRequest{
		Title: "Restore Table Tail Fixture",
	})
	w := httptest.NewRecorder()
	h.HandleVTextDocumentsRoot(w, createDocReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create document: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var doc vtextCreateDocResponse
	if err := json.NewDecoder(w.Body).Decode(&doc); err != nil {
		t.Fatalf("decode document response: %v", err)
	}

	malformedSource := strings.Join([]string{
		"# Legal Cloud",
		"",
		"Appendix A: Glossary",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Agent | Multi-step worker. |",
		"| Vector database | Stores embeddings for retrieval. |",
		"",
		"| Work product | Durable output.",
		"",
		"End of proposal.",
	}, "\n")
	sourceReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+doc.DocID+"/revisions", vtextCreateRevisionRequest{
		Content: malformedSource,
		Metadata: json.RawMessage(`{
			"created_from":"historical_import",
			"source_entities":[{"entity_id":"src-restore-rule","label":"ABA Model Rule 1.6"}]
		}`),
	})
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, sourceReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create malformed source revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var sourceResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&sourceResp); err != nil {
		t.Fatalf("decode source revision response: %v", err)
	}
	sourceRev, err := s.GetRevision(ctx, sourceResp.RevisionID, "user-1")
	if err != nil {
		t.Fatalf("GetRevision source: %v", err)
	}
	sourceTables := extractMarkdownTableBlocks(sourceRev.Content)
	if len(sourceTables) != 1 || sourceTables[0].EndLine-sourceTables[0].StartLine+1 != 4 {
		t.Fatalf("source revision should retain the historical partial table shape:\n%s", sourceRev.Content)
	}

	currentContent := strings.Replace(malformedSource, "Legal Cloud", "Legal Cloud Current", 1)
	currentReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+doc.DocID+"/revisions", vtextCreateRevisionRequest{
		Content:          currentContent,
		ParentRevisionID: sourceResp.RevisionID,
		Metadata:         json.RawMessage(`{"created_from":"current_head"}`),
	})
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, currentReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create current revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	restoreReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+doc.DocID+"/restore", vtextRestoreRevisionRequest{
		RevisionID: sourceResp.RevisionID,
		Mode:       "primary",
	})
	w = httptest.NewRecorder()
	h.HandleVTextRouter(w, restoreReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("restore revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var restoredResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&restoredResp); err != nil {
		t.Fatalf("decode restored revision response: %v", err)
	}
	restored, err := s.GetRevision(ctx, restoredResp.RevisionID, "user-1")
	if err != nil {
		t.Fatalf("GetRevision restored: %v", err)
	}
	if !strings.Contains(restored.Content, "| Vector database | Stores embeddings for retrieval. |\n| Work product | Durable output. |") {
		t.Fatalf("restored revision did not normalize the table tail row:\n%s", restored.Content)
	}
	restoredTables := extractMarkdownTableBlocks(restored.Content)
	if len(restoredTables) != 1 || restoredTables[0].EndLine-restoredTables[0].StartLine+1 != 5 {
		t.Fatalf("restored revision table shape = %#v; content:\n%s", restoredTables, restored.Content)
	}
	meta := decodeRevisionMetadata(restored.Metadata)
	if meta["vtext_structure_stabilized"] != true ||
		meta["vtext_structure_stabilized_reason"] != "normalized_restored_markdown_table_rows" {
		t.Fatalf("restored metadata did not record normalization: %#v", meta)
	}
	entities := decodeVTextSourceEntities(meta["source_entities"])
	if len(entities) != 1 || entities[0].EntityID != "src-restore-rule" {
		t.Fatalf("restored source_entities = %#v", meta["source_entities"])
	}
}

func TestVTextMarkdownStructureStabilizationAllowsExplicitTableDeletion(t *testing.T) {
	t.Parallel()
	parentContent := strings.Join([]string{
		"# Appendix",
		"",
		"Intro paragraph.",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Agent | Multi-step worker. |",
		"| Work product | Durable output. |",
		"| Vector database | Stores embeddings for retrieval. |",
		"",
		"---",
	}, "\n")
	userContent := strings.Join([]string{
		"# Appendix",
		"",
		"Intro paragraph.",
		"",
		"---",
	}, "\n")

	stabilized, changed := stabilizeVTextUserMarkdownStructures(parentContent, userContent)
	if changed {
		t.Fatalf("stabilization changed explicit table deletion:\n%s", stabilized)
	}
	if strings.Contains(stabilized, "| Term | Definition |") {
		t.Fatalf("stabilization restored explicitly deleted table:\n%s", stabilized)
	}
}

func TestVTextMarkdownTableRowParserHandlesEscapedPipes(t *testing.T) {
	t.Parallel()
	cells := markdownstructure.TableRowCells(`| Term \| Alias | Definition with \| symbol |`)
	if len(cells) != 2 {
		t.Fatalf("cells = %#v, want 2 cells", cells)
	}
	if cells[0] != "Term | Alias" || cells[1] != "Definition with | symbol" {
		t.Fatalf("cells = %#v", cells)
	}
}

func TestVTextImportMarkdownLineageCreatesRevisionHistory(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud.md",
		Title:      "legal-cloud.md",
		Versions: []vtextMarkdownLineageVersion{
			{
				Label:            "v44",
				SourceRevisionID: "git-a",
				Content:          "# Proposal\n\nGlossary table survives here. [1]\n",
				CreatedAt:        "2026-06-04T10:00:00Z",
			},
			{
				Label:            "v47",
				SourceRevisionID: "git-b",
				Content:          "# Proposal\n\n| Term | Definition |\n| --- | --- |\n| Work product | Durable output |\n",
				CreatedAt:        "2026-06-04T11:00:00Z",
			},
			{
				Label:            "v49",
				SourceRevisionID: "git-c",
				Content:          "# Proposal\n\nLatest citations and conclusion.\n",
				CreatedAt:        "2026-06-04T12:00:00Z",
				Metadata:         json.RawMessage(`{"import_note":"owner selected current draft"}`),
			},
		},
	})
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if !resp.Created || resp.RevisionCount != 3 || len(resp.Revisions) != 3 || len(resp.OriginalContentIDs) != 3 {
		t.Fatalf("import response = %#v", resp)
	}
	if resp.Revisions[0].VersionNumber != 0 || resp.Revisions[1].VersionNumber != 1 || resp.Revisions[2].VersionNumber != 2 {
		t.Fatalf("response version numbers = %#v", resp.Revisions)
	}
	if resp.Revisions[0].ParentRevisionID != "" || resp.Revisions[1].ParentRevisionID != resp.Revisions[0].RevisionID || resp.Revisions[2].ParentRevisionID != resp.Revisions[1].RevisionID {
		t.Fatalf("response parent chain = %#v", resp.Revisions)
	}
	if resp.CurrentRevisionID != resp.Revisions[2].RevisionID {
		t.Fatalf("current revision = %q, want latest %q", resp.CurrentRevisionID, resp.Revisions[2].RevisionID)
	}

	docID, err := s.GetDocumentAlias(context.Background(), "user-1", "proposals/legal-cloud.md")
	if err != nil {
		t.Fatalf("GetDocumentAlias: %v", err)
	}
	if docID != resp.DocID {
		t.Fatalf("alias doc = %q, want %q", docID, resp.DocID)
	}
	doc, err := s.GetDocument(context.Background(), resp.DocID, "user-1")
	if err != nil {
		t.Fatalf("GetDocument imported lineage: %v", err)
	}
	if doc.Title != "legal-cloud.vtext" {
		t.Fatalf("imported lineage document title = %q, want legal-cloud.vtext", doc.Title)
	}
	revs, err := s.ListRevisionsByDoc(context.Background(), resp.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 3 {
		t.Fatalf("len(revisions) = %d, want 3", len(revs))
	}
	if revs[0].VersionNumber != 2 || !strings.Contains(revs[0].Content, "Latest citations") {
		t.Fatalf("latest stored revision = %#v", revs[0])
	}
	oldest := revs[2]
	meta := decodeRevisionMetadata(oldest.Metadata)
	manifest, ok := meta["migration_manifest"].(map[string]any)
	if !ok {
		t.Fatalf("missing migration_manifest: %#v", meta)
	}
	if manifest["migration_adapter"] != "markdown_lineage_to_vtext_revisions" || manifest["lineage_count"] != float64(3) || manifest["source_gap_policy"] != "repairable_gap_no_invented_citations" {
		t.Fatalf("migration manifest = %#v", manifest)
	}
	lineage, ok := manifest["version_lineage"].([]any)
	if !ok || len(lineage) != 3 {
		t.Fatalf("version lineage = %#v", manifest["version_lineage"])
	}
	if manifest["original_content_id"] == "" || manifest["original_content_hash"] == "" {
		t.Fatalf("missing original snapshot refs in manifest: %#v", manifest)
	}
	gaps, ok := meta["source_gaps"].([]any)
	if !ok || len(gaps) != 1 {
		t.Fatalf("source gaps = %#v", meta["source_gaps"])
	}
	gap := gaps[0].(map[string]any)
	if state := gap["evidence_state"].(map[string]any)["state"]; state != "candidate" {
		t.Fatalf("source gap evidence_state = %#v", gap["evidence_state"])
	}
	latestMeta := decodeRevisionMetadata(revs[0].Metadata)
	if latestMeta["source_metadata"].(map[string]any)["import_note"] != "owner selected current draft" {
		t.Fatalf("latest source metadata = %#v", latestMeta["source_metadata"])
	}

	items, err := s.ListContentItems(context.Background(), "user-1", 10)
	if err != nil {
		t.Fatalf("ListContentItems: %v", err)
	}
	var foundSnapshots int
	for _, item := range items {
		if item.SourceType == "file_version" && item.FilePath != "" && strings.HasPrefix(item.FilePath, "proposals/legal-cloud.md#") {
			foundSnapshots++
			if item.MediaType != "text/markdown" || item.AppHint != "vtext" || item.TextContent == "" || item.ContentHash == "" {
				t.Fatalf("snapshot content item = %#v", item)
			}
		}
	}
	if foundSnapshots != 3 {
		t.Fatalf("found snapshot content items = %d, want 3", foundSnapshots)
	}
}

func TestVTextImportMarkdownLineageResolvesCitationMarkers(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	entity := vtextSourceEntity{
		EntityID: "src-aba-rule-16",
		Kind:     "source_service_item",
		Label:    "ABA Model Rule 1.6",
		Target: vtextSourceEntityTarget{
			TargetKind: "source_service_item",
			ItemID:     "srcitem_aba_rule_16",
		},
		Selectors: []vtextSourceEntitySelector{{
			SelectorKind: "text_quote",
			TextQuote:    "A lawyer shall not reveal information relating to the representation of a client.",
		}},
		Display: vtextSourceEntityDisplay{
			InlineMode:       "embedded_excerpt",
			ExpandedMode:     "source_card",
			OpenSurface:      "source",
			DefaultCollapsed: false,
		},
		Evidence: vtextSourceEntityEvidence{
			State:         "available",
			ResearchState: "represented",
		},
		Provenance: vtextSourceEntityProvenance{
			CreatedBy:           "migration",
			RightsScope:         "source_service_projection",
			UntrustedSourceText: true,
		},
	}

	req := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath:     "proposals/legal-cloud-sourced.md",
		Title:          "legal-cloud-sourced.md",
		SourceEntities: []vtextSourceEntity{entity},
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v44",
			Content: "# Proposal\n\nConfidentiality matters [1]. Unresolved claim [2].\n",
			CitationResolutions: []vtextCitationMarkerResolution{{
				Marker:   "[1]",
				EntityID: entity.EntityID,
			}},
		}},
	})
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	revs, err := s.ListRevisionsByDoc(context.Background(), resp.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("len(revisions) = %d, want 1", len(revs))
	}
	if !strings.Contains(revs[0].Content, "[1](source:src-aba-rule-16)") {
		t.Fatalf("resolved citation was not rewritten as source link: %q", revs[0].Content)
	}
	if !strings.Contains(revs[0].Content, "Unresolved claim [2]") {
		t.Fatalf("unresolved citation marker should remain visible: %q", revs[0].Content)
	}
	meta := decodeRevisionMetadata(revs[0].Metadata)
	sourceEntities := decodeVTextSourceEntities(meta["source_entities"])
	if len(sourceEntities) != 1 || sourceEntities[0].EntityID != entity.EntityID {
		t.Fatalf("source_entities = %#v", meta["source_entities"])
	}
	manifest := meta["migration_manifest"].(map[string]any)
	resolutions, ok := manifest["citation_resolutions"].([]any)
	if !ok || len(resolutions) != 1 {
		t.Fatalf("citation_resolutions = %#v", manifest["citation_resolutions"])
	}
	gaps, ok := meta["source_gaps"].([]any)
	if !ok || len(gaps) != 1 {
		t.Fatalf("source_gaps = %#v", meta["source_gaps"])
	}
	gap := gaps[0].(map[string]any)
	if gap["marker"] != "[2]" {
		t.Fatalf("source gap = %#v", gap)
	}

	items, err := s.ListContentItems(context.Background(), "user-1", 10)
	if err != nil {
		t.Fatalf("ListContentItems: %v", err)
	}
	for _, item := range items {
		if item.FilePath == "proposals/legal-cloud-sourced.md#v44" {
			if !strings.Contains(item.TextContent, "Confidentiality matters [1]") {
				t.Fatalf("original snapshot should preserve raw markdown markers: %q", item.TextContent)
			}
			return
		}
	}
	t.Fatalf("missing original source snapshot item: %#v", items)
}

func TestVTextUserSaveAndAgentRevisePreserveSourcesAndTableShape(t *testing.T) {
	t.Parallel()
	provider := newVTextEditToolProvider("")
	provider.resultFunc = func(string) string {
		return vtextApplyEditsResult([]vtextTextEdit{{
			Op:      "replace",
			Find:    "A private legal cloud addresses this.",
			Replace: "A private legal cloud addresses this clearly.",
		}})
	}
	h, s, _ := vtextAPISetupWithProvider(t, provider, true)
	ctx := context.Background()
	entity := vtextSourceEntity{
		EntityID: "src-aba-rule-16",
		Kind:     "source_service_item",
		Label:    "ABA Model Rule 1.6",
		Target: vtextSourceEntityTarget{
			TargetKind: "source_service_item",
			ItemID:     "srcitem_aba_rule_16",
		},
		Selectors: []vtextSourceEntitySelector{{
			SelectorKind: "text_quote",
			TextQuote:    "A lawyer shall not reveal information relating to the representation of a client.",
		}},
		Display: vtextSourceEntityDisplay{
			InlineMode:   "embedded_excerpt",
			ExpandedMode: "source_card",
			OpenSurface:  "source",
		},
		Evidence: vtextSourceEntityEvidence{
			State:         "available",
			ResearchState: "represented",
		},
		Provenance: vtextSourceEntityProvenance{
			CreatedBy:   "migration",
			RightsScope: "source_service_projection",
		},
	}
	parentContent := strings.Join([]string{
		"# Proposal",
		"",
		"The core problem is confidentiality [1].",
		"",
		"A private legal cloud solves this.",
		"",
		"Appendix A: Glossary",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Agent | Multi-step worker. |",
		"| Work product | Durable output. |",
		"| Vector database | Stores embeddings for retrieval. |",
		"",
		"End of proposal.",
	}, "\n")
	importReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath:     "proposals/legal-cloud-sourced.md",
		Title:          "legal-cloud-sourced.md",
		SourceEntities: []vtextSourceEntity{entity},
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v1",
			Content: parentContent,
			CitationResolutions: []vtextCitationMarkerResolution{{
				Marker:   "[1]",
				EntityID: entity.EntityID,
			}},
		}},
	})
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, importReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var imported vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(w.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	parentRev, err := s.GetRevision(ctx, imported.CurrentRevisionID, "user-1")
	if err != nil {
		t.Fatalf("GetRevision parent: %v", err)
	}
	parentTables := extractMarkdownTableBlocks(parentRev.Content)
	if len(parentTables) != 1 {
		t.Fatalf("parent tables = %d, want 1:\n%s", len(parentTables), parentRev.Content)
	}

	userContent := strings.Replace(parentRev.Content, "A private legal cloud solves this.", "A private legal cloud addresses this.", 1)
	userReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/revisions", vtextCreateRevisionRequest{
		Content:          userContent,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "owner",
		Metadata:         json.RawMessage(`{"created_from":"browser_user_edit"}`),
		ParentRevisionID: imported.CurrentRevisionID,
	})
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, userReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var userRevResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&userRevResp); err != nil {
		t.Fatalf("decode user revision response: %v", err)
	}
	userRev, err := s.GetRevision(ctx, userRevResp.RevisionID, "user-1")
	if err != nil {
		t.Fatalf("GetRevision user: %v", err)
	}
	userMeta := decodeRevisionMetadata(userRev.Metadata)
	userEntities := decodeVTextSourceEntities(userMeta["source_entities"])
	if len(userEntities) != 1 || userEntities[0].EntityID != entity.EntityID {
		t.Fatalf("user revision source_entities = %#v", userMeta["source_entities"])
	}
	userTables := extractMarkdownTableBlocks(userRev.Content)
	if len(userTables) != 1 || userTables[0].Text != parentTables[0].Text {
		t.Fatalf("user revision table changed:\nparent:\n%s\nuser:\n%s", parentTables[0].Text, userRev.Content)
	}

	reviseReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/revise",
		map[string]string{"intent": "revise"})
	w = httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, reviseReq)
	if w.Code != http.StatusAccepted {
		t.Fatalf("revise status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	revs := waitForRevisionCount(t, s, imported.DocID, "user-1", 3, 5*time.Second)
	agentRev := revs[0]
	agentMeta := decodeRevisionMetadata(agentRev.Metadata)
	agentEntities := decodeVTextSourceEntities(agentMeta["source_entities"])
	if len(agentEntities) != 1 || agentEntities[0].EntityID != entity.EntityID {
		t.Fatalf("agent revision source_entities = %#v", agentMeta["source_entities"])
	}
	agentTables := extractMarkdownTableBlocks(agentRev.Content)
	if len(agentTables) != 1 || agentTables[0].Text != parentTables[0].Text {
		t.Fatalf("agent revision table changed:\nparent:\n%s\nagent:\n%s", parentTables[0].Text, agentRev.Content)
	}
	if !strings.Contains(agentRev.Content, "A private legal cloud addresses this clearly.") {
		t.Fatalf("agent edit did not apply:\n%s", agentRev.Content)
	}
}

func TestVTextUserSaveRemovesDuplicateMarkdownTableSeparator(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	entity := vtextSourceEntity{
		EntityID: "src-table-separator-proof",
		Kind:     "source_service_item",
		Label:    "Table Source",
		Target: vtextSourceEntityTarget{
			TargetKind: "source_service_item",
			ItemID:     "srcitem-table-separator-proof",
			SourceID:   "fixture:table",
			FetchID:    "fetch-table-separator-proof",
		},
	}
	parentContent := strings.Join([]string{
		"# Proposal",
		"",
		"A private legal cloud solves this [1].",
		"",
		"Appendix A: Glossary",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Agent | Multi-step worker. |",
		"| Work product | Durable output. |",
		"| Vector database | Stores embeddings for retrieval. |",
		"",
		"End of proposal.",
	}, "\n")
	importReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath:     "proposals/table-separator-proof.md",
		Title:          "table-separator-proof.md",
		SourceEntities: []vtextSourceEntity{entity},
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v1",
			Content: parentContent,
			CitationResolutions: []vtextCitationMarkerResolution{{
				Marker:   "[1]",
				EntityID: entity.EntityID,
			}},
		}},
	})
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, importReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var imported vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(w.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	parentRev, err := s.GetRevision(ctx, imported.CurrentRevisionID, "user-1")
	if err != nil {
		t.Fatalf("GetRevision parent: %v", err)
	}
	parentTables := extractMarkdownTableBlocks(parentRev.Content)
	if len(parentTables) != 1 {
		t.Fatalf("parent tables = %d, want 1:\n%s", len(parentTables), parentRev.Content)
	}

	userContent := strings.Replace(parentRev.Content, "A private legal cloud solves this", "A private legal cloud addresses this", 1)
	userContent = strings.Replace(userContent, "| --- | --- |\n| Agent |", "| --- | --- |\n| --- | --- |\n| Agent |", 1)
	userReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/revisions", vtextCreateRevisionRequest{
		Content:          userContent,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "owner",
		Metadata:         json.RawMessage(`{"created_from":"browser_user_edit"}`),
		ParentRevisionID: imported.CurrentRevisionID,
	})
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, userReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var userRevResp vtextRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&userRevResp); err != nil {
		t.Fatalf("decode user revision response: %v", err)
	}
	userRev, err := s.GetRevision(ctx, userRevResp.RevisionID, "user-1")
	if err != nil {
		t.Fatalf("GetRevision user: %v", err)
	}
	if !strings.Contains(userRev.Content, "A private legal cloud addresses this") {
		t.Fatalf("user prose edit was not preserved:\n%s", userRev.Content)
	}
	if strings.Count(userRev.Content, "| --- | --- |") != 1 {
		t.Fatalf("user revision kept duplicate separator:\n%s", userRev.Content)
	}
	userTables := extractMarkdownTableBlocks(userRev.Content)
	if len(userTables) != 1 || userTables[0].Text != parentTables[0].Text {
		t.Fatalf("user revision table changed:\nparent:\n%s\nuser:\n%s", parentTables[0].Text, userRev.Content)
	}
	userMeta := decodeRevisionMetadata(userRev.Metadata)
	if userMeta["vtext_structure_stabilized"] != true {
		t.Fatalf("user metadata did not record table stabilization: %#v", userMeta)
	}
	userEntities := decodeVTextSourceEntities(userMeta["source_entities"])
	if len(userEntities) != 1 || userEntities[0].EntityID != entity.EntityID {
		t.Fatalf("user revision source_entities = %#v", userMeta["source_entities"])
	}
}

func TestVTextSourceGapRepairCreatesRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	entity := vtextSourceEntity{
		EntityID: "src-aba-rule-16-repair",
		Kind:     "source_service_item",
		Label:    "ABA Model Rule 1.6",
		Target: vtextSourceEntityTarget{
			TargetKind: "source_service_item",
			ItemID:     "srcitem_aba_rule_16_repair",
		},
		Selectors: []vtextSourceEntitySelector{{
			SelectorKind: "text_quote",
			TextQuote:    "A lawyer shall not reveal information relating to the representation of a client.",
		}},
		Display: vtextSourceEntityDisplay{
			InlineMode:       "embedded_excerpt",
			ExpandedMode:     "source_card",
			OpenSurface:      "source",
			DefaultCollapsed: false,
		},
		Evidence: vtextSourceEntityEvidence{
			State:         "available",
			ResearchState: "represented",
		},
		Provenance: vtextSourceEntityProvenance{
			CreatedBy:           "source_gap_repair",
			RightsScope:         "source_service_projection",
			UntrustedSourceText: true,
		},
	}

	importReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-repairable.md",
		Title:      "legal-cloud-repairable.md",
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v44",
			Content: "# Proposal\n\nConfidentiality matters [1].\n",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleVTextRouter(importW, importReq)
	if importW.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", importW.Code, http.StatusCreated, importW.Body.String())
	}
	var imported vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(importW.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}

	repairReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/source-repairs", vtextSourceGapRepairRequest{
		BaseRevisionID: imported.CurrentRevisionID,
		SourceEntities: []vtextSourceEntity{
			entity,
		},
		CitationResolutions: []vtextCitationMarkerResolution{{
			Marker:   "[1]",
			EntityID: entity.EntityID,
		}},
	})
	repairW := httptest.NewRecorder()
	h.HandleVTextRouter(repairW, repairReq)
	if repairW.Code != http.StatusCreated {
		t.Fatalf("repair source gap: status = %d, want %d; body: %s", repairW.Code, http.StatusCreated, repairW.Body.String())
	}
	var repaired vtextRevisionResponse
	if err := json.NewDecoder(repairW.Body).Decode(&repaired); err != nil {
		t.Fatalf("decode repair response: %v", err)
	}
	if repaired.ParentRevisionID != imported.CurrentRevisionID || repaired.VersionNumber != 1 {
		t.Fatalf("repaired revision = %#v", repaired)
	}
	if !strings.Contains(repaired.Content, "[1](source:src-aba-rule-16-repair)") {
		t.Fatalf("repaired content = %q", repaired.Content)
	}
	if strings.Contains(repaired.Content, "[1](source:src-aba-rule-16-repair)(source:") {
		t.Fatalf("repaired content double-linked marker: %q", repaired.Content)
	}
	meta := decodeRevisionMetadata(repaired.Metadata)
	if meta["source"] != "vtext_source_gap_repair" || meta["base_revision_id"] != imported.CurrentRevisionID {
		t.Fatalf("repair metadata = %#v", meta)
	}
	if _, ok := meta["source_gaps"]; ok {
		t.Fatalf("source_gaps should be cleared after repair: %#v", meta["source_gaps"])
	}
	sourceEntities := decodeVTextSourceEntities(meta["source_entities"])
	if len(sourceEntities) != 1 || sourceEntities[0].EntityID != entity.EntityID {
		t.Fatalf("source_entities = %#v", meta["source_entities"])
	}
	if sourceEntities[0].Evidence.State != "confirms" || sourceEntities[0].Evidence.Relation != "confirms" {
		t.Fatalf("source entity evidence = %#v", sourceEntities[0].Evidence)
	}
	manifest := meta["source_repair_resolutions"].([]any)
	evidenceState := manifest[0].(map[string]any)["evidence_state"].(map[string]any)
	if evidenceState["state"] != "confirms" || evidenceState["target_id"] != entity.EntityID {
		t.Fatalf("source repair evidence_state = %#v", evidenceState)
	}
	revs, err := s.ListRevisionsByDoc(context.Background(), imported.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 2 || revs[0].RevisionID != repaired.RevisionID {
		t.Fatalf("stored revisions = %#v", revs)
	}
}

func TestVTextSourceGapRepairPreservesUnrepairedGaps(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)
	entity := vtextSourceEntity{
		EntityID: "src-rule-one",
		Kind:     "source_service_item",
		Label:    "Rule One",
		Target:   vtextSourceEntityTarget{TargetKind: "source_service_item", ItemID: "srcitem_rule_one"},
		Display:  vtextSourceEntityDisplay{InlineMode: "collapsed_citation", ExpandedMode: "source_card", OpenSurface: "source", DefaultCollapsed: true},
		Evidence: vtextSourceEntityEvidence{State: "available", ResearchState: "represented"},
		Provenance: vtextSourceEntityProvenance{
			CreatedBy:           "source_gap_repair",
			RightsScope:         "source_service_projection",
			UntrustedSourceText: true,
		},
	}
	importReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-partial-repair.md",
		Title:      "legal-cloud-partial-repair.md",
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v44",
			Content: "Known [1]. Still unknown [2].",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleVTextRouter(importW, importReq)
	if importW.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", importW.Code, http.StatusCreated, importW.Body.String())
	}
	var imported vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(importW.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}

	repairReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/source-repairs", vtextSourceGapRepairRequest{
		SourceEntities: []vtextSourceEntity{entity},
		CitationResolutions: []vtextCitationMarkerResolution{{
			Marker:   "[1]",
			EntityID: entity.EntityID,
		}},
	})
	repairW := httptest.NewRecorder()
	h.HandleVTextRouter(repairW, repairReq)
	if repairW.Code != http.StatusCreated {
		t.Fatalf("repair source gap: status = %d, want %d; body: %s", repairW.Code, http.StatusCreated, repairW.Body.String())
	}
	var repaired vtextRevisionResponse
	if err := json.NewDecoder(repairW.Body).Decode(&repaired); err != nil {
		t.Fatalf("decode repair response: %v", err)
	}
	if !strings.Contains(repaired.Content, "[1](source:src-rule-one)") || !strings.Contains(repaired.Content, "Still unknown [2]") {
		t.Fatalf("repaired content = %q", repaired.Content)
	}
	meta := decodeRevisionMetadata(repaired.Metadata)
	gaps, ok := meta["source_gaps"].([]any)
	if !ok || len(gaps) != 1 {
		t.Fatalf("source_gaps = %#v", meta["source_gaps"])
	}
	gap := gaps[0].(map[string]any)
	if gap["marker"] != "[2]" {
		t.Fatalf("remaining source gap = %#v", gap)
	}
	if state := gap["evidence_state"].(map[string]any)["state"]; state != "candidate" {
		t.Fatalf("remaining source gap evidence_state = %#v", gap["evidence_state"])
	}
}

func TestVTextSourceGapRepairCanOmitNoSourceNeededMarker(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)
	importReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-no-source-needed.md",
		Title:      "legal-cloud-no-source-needed.md",
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v44",
			Content: "Ordinary framing sentence [2].",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleVTextRouter(importW, importReq)
	if importW.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", importW.Code, http.StatusCreated, importW.Body.String())
	}
	var imported vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(importW.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}

	repairReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/source-repairs", vtextSourceGapRepairRequest{
		CitationResolutions: []vtextCitationMarkerResolution{{
			Marker: "[2]",
			Action: "no_source_needed",
			Reason: "The sentence is structural framing, not a factual claim needing citation.",
		}},
	})
	repairW := httptest.NewRecorder()
	h.HandleVTextRouter(repairW, repairReq)
	if repairW.Code != http.StatusCreated {
		t.Fatalf("repair source gap: status = %d, want %d; body: %s", repairW.Code, http.StatusCreated, repairW.Body.String())
	}
	var repaired vtextRevisionResponse
	if err := json.NewDecoder(repairW.Body).Decode(&repaired); err != nil {
		t.Fatalf("decode repair response: %v", err)
	}
	if repaired.Content != "Ordinary framing sentence." {
		t.Fatalf("repaired content = %q", repaired.Content)
	}
	meta := decodeRevisionMetadata(repaired.Metadata)
	if _, ok := meta["source_gaps"]; ok {
		t.Fatalf("source_gaps should be cleared after no-source-needed repair: %#v", meta["source_gaps"])
	}
	if _, ok := meta["source_entities"]; ok {
		t.Fatalf("no-source-needed repair should not invent source_entities: %#v", meta["source_entities"])
	}
	manifest, ok := meta["source_repair_resolutions"].([]any)
	if !ok || len(manifest) != 1 {
		t.Fatalf("source_repair_resolutions = %#v", meta["source_repair_resolutions"])
	}
	item := manifest[0].(map[string]any)
	if item["marker"] != "[2]" || item["action"] != "no_source_needed" || item["entity_id"] != nil {
		t.Fatalf("source repair manifest item = %#v", item)
	}
	if item["reason"] != "The sentence is structural framing, not a factual claim needing citation." {
		t.Fatalf("source repair reason = %#v", item["reason"])
	}
	evidenceState := item["evidence_state"].(map[string]any)
	if evidenceState["state"] != "no_source_needed" || evidenceState["reason"] != item["reason"] {
		t.Fatalf("no-source-needed evidence_state = %#v", evidenceState)
	}
}

func TestVTextSourceGapRepairRejectsUnknownEntity(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)
	importReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-bad-repair.md",
		Title:      "legal-cloud-bad-repair.md",
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v44",
			Content: "Unknown [1].",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleVTextRouter(importW, importReq)
	if importW.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", importW.Code, http.StatusCreated, importW.Body.String())
	}
	var imported vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(importW.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}

	repairReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/source-repairs", vtextSourceGapRepairRequest{
		CitationResolutions: []vtextCitationMarkerResolution{{
			Marker:   "[1]",
			EntityID: "missing-source",
		}},
	})
	repairW := httptest.NewRecorder()
	h.HandleVTextRouter(repairW, repairReq)
	if repairW.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", repairW.Code, http.StatusBadRequest, repairW.Body.String())
	}
	if !strings.Contains(repairW.Body.String(), "references unknown source entity missing-source") {
		t.Fatalf("body = %s", repairW.Body.String())
	}
}

func TestVTextSourceArtifactAttachmentCreatesMetadataOnlyRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	entity := vtextSourceEntity{
		EntityID: "src-public-rule",
		Kind:     "legal_source",
		Label:    "Public Rule",
		Target: vtextSourceEntityTarget{
			TargetKind:   "url",
			URL:          "https://example.com/rule",
			CanonicalURL: "https://example.com/rule",
		},
		Selectors: []vtextSourceEntitySelector{{
			SelectorKind: "text_quote",
			TextQuote:    "bounded excerpt",
		}},
		Display: vtextSourceEntityDisplay{
			InlineMode:       "embedded_excerpt",
			ExpandedMode:     "source_card",
			OpenSurface:      "source",
			DefaultCollapsed: true,
		},
		Evidence: vtextSourceEntityEvidence{
			State:         "available",
			ResearchState: "represented",
		},
		Provenance: vtextSourceEntityProvenance{
			CreatedBy:           "test",
			RightsScope:         "public_url_snapshot",
			UntrustedSourceText: true,
		},
	}
	importReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath:     "proposals/source-attachment.md",
		Title:          "source-attachment.md",
		SourceEntities: []vtextSourceEntity{entity},
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v1",
			Content: "A cited sentence [1](source:src-public-rule).\n\n| Term | Definition |\n| --- | --- |\n| Work product | Durable output |",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleVTextRouter(importW, importReq)
	if importW.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", importW.Code, http.StatusCreated, importW.Body.String())
	}
	var imported vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(importW.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	baseRev, err := s.GetRevision(ctx, imported.CurrentRevisionID, "user-1")
	if err != nil {
		t.Fatalf("base revision: %v", err)
	}
	now := time.Now().UTC()
	item := types.ContentItem{
		ContentID:    "content-public-rule",
		OwnerID:      "user-1",
		SourceType:   "text",
		MediaType:    "text/markdown",
		AppHint:      "vtext",
		Title:        "Readable Public Rule",
		SourceURL:    "https://example.com/rule",
		CanonicalURL: "https://example.com/rule",
		TextContent:  "Readable public source artifact for the cited rule.",
		ContentHash:  contentHash("Readable public source artifact for the cited rule."),
		Provenance:   json.RawMessage(`{"rights_scope":"public_source","untrusted_source_text":true}`),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.CreateContentItem(ctx, item); err != nil {
		t.Fatalf("create content item: %v", err)
	}

	attachReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/source-attachments", vtextSourceArtifactAttachmentRequest{
		BaseRevisionID: imported.CurrentRevisionID,
		Attachments: []vtextSourceArtifactAttachment{{
			EntityID:  "src-public-rule",
			ContentID: "content-public-rule",
		}},
	})
	attachW := httptest.NewRecorder()
	h.HandleVTextRouter(attachW, attachReq)
	if attachW.Code != http.StatusCreated {
		t.Fatalf("attach source artifact: status = %d, want %d; body: %s", attachW.Code, http.StatusCreated, attachW.Body.String())
	}
	var attached vtextRevisionResponse
	if err := json.NewDecoder(attachW.Body).Decode(&attached); err != nil {
		t.Fatalf("decode attached revision: %v", err)
	}
	if attached.Content != baseRev.Content {
		t.Fatalf("attachment changed content:\n got %q\nwant %q", attached.Content, baseRev.Content)
	}
	if attached.ParentRevisionID != imported.CurrentRevisionID || attached.VersionNumber != 1 {
		t.Fatalf("attached revision = %#v", attached)
	}
	meta := decodeRevisionMetadata(attached.Metadata)
	if meta["source"] != "vtext_source_artifact_attachment" || meta["base_revision_id"] != imported.CurrentRevisionID {
		t.Fatalf("attachment metadata = %#v", meta)
	}
	if meta["source_attachment_count"] != float64(1) {
		t.Fatalf("source_attachment_count = %#v", meta["source_attachment_count"])
	}
	entities := decodeVTextSourceEntities(meta["source_entities"])
	if len(entities) != 1 {
		t.Fatalf("source_entities = %#v", meta["source_entities"])
	}
	got := entities[0]
	if got.EntityID != "src-public-rule" || got.Target.ContentID != "content-public-rule" || got.Target.TargetKind != "content_item" {
		t.Fatalf("attached entity target = %#v", got.Target)
	}
	if got.Target.CanonicalURL != "https://example.com/rule" || got.Evidence.State != "available" || got.Provenance.RightsScope != "public_url_snapshot" {
		t.Fatalf("attached entity metadata = %#v", got)
	}
	if !strings.Contains(attached.Content, "| Term | Definition |") {
		t.Fatalf("table content not preserved: %q", attached.Content)
	}
}

func TestVTextSourceArtifactAttachmentRejectsEmptyContentItem(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	entity := vtextSourceEntity{
		EntityID: "src-empty",
		Kind:     "legal_source",
		Label:    "Empty Source",
		Target:   vtextSourceEntityTarget{TargetKind: "url", URL: "https://example.com/empty", CanonicalURL: "https://example.com/empty"},
		Display:  vtextSourceEntityDisplay{InlineMode: "collapsed_citation", ExpandedMode: "source_card", OpenSurface: "source", DefaultCollapsed: true},
		Evidence: vtextSourceEntityEvidence{State: "available", ResearchState: "represented"},
		Provenance: vtextSourceEntityProvenance{
			CreatedBy:           "test",
			RightsScope:         "public_url_snapshot",
			UntrustedSourceText: true,
		},
	}
	importReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath:     "proposals/source-attachment-empty.md",
		Title:          "source-attachment-empty.md",
		SourceEntities: []vtextSourceEntity{entity},
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v1",
			Content: "A cited sentence [1](source:src-empty).",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleVTextRouter(importW, importReq)
	if importW.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", importW.Code, http.StatusCreated, importW.Body.String())
	}
	var imported vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(importW.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	now := time.Now().UTC()
	if err := s.CreateContentItem(ctx, types.ContentItem{
		ContentID:   "content-empty",
		OwnerID:     "user-1",
		SourceType:  "text",
		MediaType:   "text/markdown",
		AppHint:     "vtext",
		Title:       "Empty Source",
		TextContent: "",
		ContentHash: contentHash(""),
		Provenance:  json.RawMessage(`{"rights_scope":"public_source"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("create content item: %v", err)
	}
	attachReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+imported.DocID+"/source-attachments", vtextSourceArtifactAttachmentRequest{
		Attachments: []vtextSourceArtifactAttachment{{
			EntityID:  "src-empty",
			ContentID: "content-empty",
		}},
	})
	attachW := httptest.NewRecorder()
	h.HandleVTextRouter(attachW, attachReq)
	if attachW.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", attachW.Code, http.StatusBadRequest, attachW.Body.String())
	}
	if !strings.Contains(attachW.Body.String(), "has no readable text_content") {
		t.Fatalf("body = %s", attachW.Body.String())
	}
}

func TestVTextImportMarkdownLineageUsesExistingContentItems(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	oldContent := "# Proposal\n\nStored historical glossary [1].\n"
	latestContent := "# Proposal\n\nStored latest appendix table.\n"
	oldItem := types.ContentItem{
		ContentID:   "content-lineage-v44",
		OwnerID:     "user-1",
		SourceType:  "file_version",
		MediaType:   "text/markdown",
		AppHint:     "vtext",
		Title:       "legal-cloud.md v44",
		FilePath:    "proposals/legal-cloud-content-backed.md#v44",
		TextContent: oldContent,
		ContentHash: contentHash(oldContent),
		Metadata:    json.RawMessage(`{"source":"fixture"}`),
		Provenance:  json.RawMessage(`{"created_from":"content_item_fixture"}`),
		CreatedAt:   now.Add(-time.Hour),
		UpdatedAt:   now.Add(-time.Hour),
	}
	latestItem := types.ContentItem{
		ContentID:   "content-lineage-v49",
		OwnerID:     "user-1",
		SourceType:  "file_version",
		MediaType:   "text/markdown",
		AppHint:     "vtext",
		Title:       "legal-cloud.md v49",
		FilePath:    "proposals/legal-cloud-content-backed.md#v49",
		TextContent: latestContent,
		ContentHash: contentHash(latestContent),
		Metadata:    json.RawMessage(`{"source":"fixture"}`),
		Provenance:  json.RawMessage(`{"created_from":"content_item_fixture"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.CreateContentItem(ctx, oldItem); err != nil {
		t.Fatalf("CreateContentItem old: %v", err)
	}
	if err := s.CreateContentItem(ctx, latestItem); err != nil {
		t.Fatalf("CreateContentItem latest: %v", err)
	}

	req := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-content-backed.md",
		Title:      "legal-cloud-content-backed.md",
		Versions: []vtextMarkdownLineageVersion{
			{
				Label:         "v44",
				ContentItemID: oldItem.ContentID,
			},
			{
				Label:         "v49",
				ContentItemID: latestItem.ContentID,
			},
		},
	})
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if got, want := resp.OriginalContentIDs, []string{oldItem.ContentID, latestItem.ContentID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("original_content_ids = %#v, want %#v", got, want)
	}
	revs, err := s.ListRevisionsByDoc(ctx, resp.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("len(revisions) = %d, want 2", len(revs))
	}
	if !strings.Contains(revs[0].Content, "Stored latest appendix table") {
		t.Fatalf("latest revision content = %q", revs[0].Content)
	}
	oldest := revs[1]
	if !strings.Contains(oldest.Content, "Stored historical glossary [1]") {
		t.Fatalf("oldest revision content = %q", oldest.Content)
	}
	meta := decodeRevisionMetadata(oldest.Metadata)
	manifest := meta["migration_manifest"].(map[string]any)
	if manifest["original_content_id"] != oldItem.ContentID || manifest["source_content_item_id"] != oldItem.ContentID {
		t.Fatalf("manifest content ids = %#v", manifest)
	}
	if manifest["original_content_path"] != oldItem.FilePath || manifest["original_content_source"] != "content_item" {
		t.Fatalf("manifest content source = %#v", manifest)
	}
	lineage, ok := manifest["version_lineage"].([]any)
	if !ok || len(lineage) != 2 {
		t.Fatalf("version_lineage = %#v", manifest["version_lineage"])
	}
	first := lineage[0].(map[string]any)
	if first["original_content_id"] != oldItem.ContentID || first["original_content_source"] != "content_item" {
		t.Fatalf("lineage first = %#v", first)
	}
	items, err := s.ListContentItems(ctx, "user-1", 10)
	if err != nil {
		t.Fatalf("ListContentItems: %v", err)
	}
	var matching int
	for _, item := range items {
		if strings.HasPrefix(item.FilePath, "proposals/legal-cloud-content-backed.md#") {
			matching++
		}
	}
	if matching != 2 {
		t.Fatalf("matching content-backed source items = %d, want existing two without duplicates; items=%#v", matching, items)
	}
}

func TestVTextImportMarkdownLineageRejectsMissingContentItem(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)
	req := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath: "proposals/missing-content-item.md",
		Title:      "missing-content-item.md",
		Versions: []vtextMarkdownLineageVersion{{
			Label:         "v1",
			ContentItemID: "missing-content-item",
		}},
	})
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "content_item_id missing-content-item not found") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestVTextImportMarkdownLineageRejectsUnknownCitationEntity(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)
	req := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", vtextMarkdownLineageImportRequest{
		SourcePath: "proposals/bad-sourced.md",
		Title:      "bad-sourced.md",
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v1",
			Content: "Claim [1].",
			CitationResolutions: []vtextCitationMarkerResolution{{
				Marker:   "[1]",
				EntityID: "missing-source",
			}},
		}},
	})
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestVTextImportMarkdownLineageRejectsExistingAlias(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)
	body := vtextMarkdownLineageImportRequest{
		SourcePath: "notes/duplicate.md",
		Title:      "duplicate.md",
		Versions: []vtextMarkdownLineageVersion{{
			Label:   "v1",
			Content: "Initial version",
		}},
	}
	req := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", body)
	first := httptest.NewRecorder()
	h.HandleVTextRouter(first, req)
	if first.Code != http.StatusCreated {
		t.Fatalf("first import: status = %d, want %d; body: %s", first.Code, http.StatusCreated, first.Body.String())
	}

	secondReq := vtextRequest(t, http.MethodPost, "/api/vtext/markdown-lineage/import", body)
	second := httptest.NewRecorder()
	h.HandleVTextRouter(second, secondReq)
	if second.Code != http.StatusConflict {
		t.Fatalf("second import: status = %d, want %d; body: %s", second.Code, http.StatusConflict, second.Body.String())
	}
	var resp vtextMarkdownLineageImportResponse
	if err := json.NewDecoder(second.Body).Decode(&resp); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if resp.Created || resp.ExistingDocID == "" {
		t.Fatalf("conflict response = %#v", resp)
	}
}

func TestVTextOpenFilePreservesDocxAndPDFOriginalArtifacts(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)

	openFile := func(sourcePath, title, initialContent string) vtextOpenFileResponse {
		req := vtextRequest(t, http.MethodPost, "/api/vtext/files/open", map[string]string{
			"source_path":     sourcePath,
			"title":           title,
			"initial_content": initialContent,
		})
		w := httptest.NewRecorder()
		h.HandleVTextRouter(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("open %s: status = %d, want %d; body: %s", sourcePath, w.Code, http.StatusCreated, w.Body.String())
		}
		var resp vtextOpenFileResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode open %s: %v", sourcePath, err)
		}
		if resp.OriginalContentID == "" {
			t.Fatalf("open %s original_content_id is empty", sourcePath)
		}
		return resp
	}

	docx := openFile("imports/legal-cloud-proposal.docx", "legal-cloud-proposal.docx", "Extracted DOCX projection text")
	pdf := openFile("imports/legal-cloud-proposal.pdf", "legal-cloud-proposal.pdf", "Extracted PDF projection text")

	for _, tc := range []struct {
		name      string
		resp      vtextOpenFileResponse
		mediaType string
		appHint   string
		lossiness float64
		warning   string
		wantText  string
	}{
		{name: "docx", resp: docx, mediaType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", appHint: "vtext", lossiness: 40, warning: "docx_projection_requires_style_adapter", wantText: "Extracted DOCX projection text"},
		{name: "pdf", resp: pdf, mediaType: "application/pdf", appHint: "pdf", lossiness: 80, warning: "pdf_projection_requires_extraction_adapter", wantText: "Extracted PDF projection text"},
	} {
		doc, err := s.GetDocument(context.Background(), tc.resp.DocID, "user-1")
		if err != nil {
			t.Fatalf("%s GetDocument: %v", tc.name, err)
		}
		if doc.Title != "legal-cloud-proposal.vtext" {
			t.Fatalf("%s document title = %q, want legal-cloud-proposal.vtext", tc.name, doc.Title)
		}
		item, err := s.GetContentItem(context.Background(), "user-1", tc.resp.OriginalContentID)
		if err != nil {
			t.Fatalf("%s GetContentItem: %v", tc.name, err)
		}
		if item.MediaType != tc.mediaType || item.AppHint != tc.appHint || item.FilePath == "" || item.ContentHash == "" {
			t.Fatalf("%s original item = %#v", tc.name, item)
		}
		if item.TextContent != tc.wantText {
			t.Fatalf("%s original text content = %q, want extracted projection %q", tc.name, item.TextContent, tc.wantText)
		}
		revs, err := s.ListRevisionsByDoc(context.Background(), tc.resp.DocID, "user-1", 10)
		if err != nil {
			t.Fatalf("%s ListRevisionsByDoc: %v", tc.name, err)
		}
		if len(revs) != 1 {
			t.Fatalf("%s len(revisions) = %d, want 1", tc.name, len(revs))
		}
		meta := decodeRevisionMetadata(revs[0].Metadata)
		importManifest, ok := meta["import_manifest"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing import_manifest: %#v", tc.name, meta)
		}
		if importManifest["original_content_id"] != tc.resp.OriginalContentID || importManifest["source_media_type"] != tc.mediaType || importManifest["lossiness_score"] != tc.lossiness {
			t.Fatalf("%s import manifest = %#v", tc.name, importManifest)
		}
		if importManifest["original_content_hash_state"] != "unavailable_until_binary_bytes_adapter" || importManifest["original_content_hash"] != "" || importManifest["original_identity_hash"] == "" {
			t.Fatalf("%s binary hash state = %#v", tc.name, importManifest)
		}
		warnings, ok := importManifest["warnings"].([]any)
		if !ok || len(warnings) != 1 || warnings[0] != tc.warning {
			t.Fatalf("%s warnings = %#v", tc.name, importManifest["warnings"])
		}
		if _, ok := meta["migration_manifest"]; ok {
			t.Fatalf("%s should not have markdown migration manifest: %#v", tc.name, meta)
		}
	}
}

func TestVTextOpenFileImportsDocxAndPDFBytesFromFilesRoot(t *testing.T) {
	h, s, _ := vtextAPISetupWithRuntime(t)
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)
	importsDir := filepath.Join(filesRoot, "imports")
	if err := os.MkdirAll(importsDir, 0o755); err != nil {
		t.Fatalf("create imports dir: %v", err)
	}
	docxBytes := buildMinimalDOCX(t, []string{"Proposal Title", "Opening paragraph"}, [][]string{
		{"Term", "Definition"},
		{"Work product", "Durable professional output"},
	})
	pdfBytes := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n3 0 obj\n<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>\nendobj\n4 0 obj\n<< /Length 72 >>\nstream\nBT\n/F1 12 Tf\n72 720 Td\n(Imported PDF sentence) Tj\n0 -20 Td\n(Second line) Tj\nET\nendstream\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF")
	if err := os.WriteFile(filepath.Join(importsDir, "brief.docx"), docxBytes, 0o644); err != nil {
		t.Fatalf("write docx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(importsDir, "brief.pdf"), pdfBytes, 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	openFile := func(sourcePath string) vtextOpenFileResponse {
		req := vtextRequest(t, http.MethodPost, "/api/vtext/files/open", map[string]string{
			"source_path": sourcePath,
			"title":       filepath.Base(sourcePath),
		})
		w := httptest.NewRecorder()
		h.HandleVTextRouter(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("open %s: status = %d, want %d; body: %s", sourcePath, w.Code, http.StatusCreated, w.Body.String())
		}
		var resp vtextOpenFileResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode open %s: %v", sourcePath, err)
		}
		return resp
	}

	docx := openFile("imports/brief.docx")
	pdf := openFile("imports/brief.pdf")

	docxRevs, err := s.ListRevisionsByDoc(context.Background(), docx.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("docx ListRevisionsByDoc: %v", err)
	}
	if len(docxRevs) != 1 {
		t.Fatalf("docx revisions = %d, want 1", len(docxRevs))
	}
	if !strings.Contains(docxRevs[0].Content, "Proposal Title") || !strings.Contains(docxRevs[0].Content, "| Term | Definition |") || !strings.Contains(docxRevs[0].Content, "| Work product | Durable professional output |") {
		t.Fatalf("docx projection content = %q", docxRevs[0].Content)
	}
	docxItem, err := s.GetContentItem(context.Background(), "user-1", docx.OriginalContentID)
	if err != nil {
		t.Fatalf("docx original item: %v", err)
	}
	if docxItem.ContentHash != contentHashBytes(docxBytes) || !strings.Contains(docxItem.TextContent, "Proposal Title") {
		t.Fatalf("docx original item hash/text = %#v", docxItem)
	}
	if selectors := selectorsFromContentMetadata(docxItem.Metadata); len(selectors) == 0 {
		t.Fatalf("docx original item missing selectors: %s", string(docxItem.Metadata))
	}
	docxManifest := decodeRevisionMetadata(docxRevs[0].Metadata)["import_manifest"].(map[string]any)
	if adapter, _ := docxManifest["import_adapter"].(string); adapter != "docx_ooxml_text_table_projection" && adapter != "docx_pandoc_markdown" {
		t.Fatalf("docx manifest adapter = %#v", docxManifest)
	}
	if docxManifest["original_content_hash_state"] != "available_from_original_bytes" {
		t.Fatalf("docx manifest = %#v", docxManifest)
	}
	if docxManifest["original_content_hash"] != "sha256:"+contentHashBytes(docxBytes) {
		t.Fatalf("docx original hash = %#v", docxManifest)
	}

	pdfRevs, err := s.ListRevisionsByDoc(context.Background(), pdf.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("pdf ListRevisionsByDoc: %v", err)
	}
	if len(pdfRevs) != 1 {
		t.Fatalf("pdf revisions = %d, want 1", len(pdfRevs))
	}
	if !strings.Contains(pdfRevs[0].Content, "Imported PDF sentence") || !strings.Contains(pdfRevs[0].Content, "Second line") {
		t.Fatalf("pdf projection content = %q", pdfRevs[0].Content)
	}
	pdfItem, err := s.GetContentItem(context.Background(), "user-1", pdf.OriginalContentID)
	if err != nil {
		t.Fatalf("pdf original item: %v", err)
	}
	if pdfItem.ContentHash != contentHashBytes(pdfBytes) || !strings.Contains(pdfItem.TextContent, "Imported PDF sentence") {
		t.Fatalf("pdf original item hash/text = %#v", pdfItem)
	}
	if selectors := selectorsFromContentMetadata(pdfItem.Metadata); len(selectors) == 0 {
		t.Fatalf("pdf original item missing selectors: %s", string(pdfItem.Metadata))
	}
	pdfManifest := decodeRevisionMetadata(pdfRevs[0].Metadata)["import_manifest"].(map[string]any)
	if adapter, _ := pdfManifest["import_adapter"].(string); adapter != "pdf_poppler_pdftotext" && adapter != "pdf_literal_text_projection_fallback" {
		t.Fatalf("pdf manifest adapter = %#v", pdfManifest)
	}
	if pdfManifest["original_content_hash_state"] != "available_from_original_bytes" {
		t.Fatalf("pdf manifest = %#v", pdfManifest)
	}
	if pdfManifest["original_content_hash"] != "sha256:"+contentHashBytes(pdfBytes) {
		t.Fatalf("pdf original hash = %#v", pdfManifest)
	}
}

func buildMinimalDOCX(t *testing.T, paragraphs []string, table [][]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		t.Helper()
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create docx part %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write docx part %s: %v", name, err)
		}
	}
	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, paragraph := range paragraphs {
		body.WriteString(`<w:p><w:r><w:t>`)
		body.WriteString(escapeDocxText(paragraph))
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	body.WriteString(`<w:tbl>`)
	for _, row := range table {
		body.WriteString(`<w:tr>`)
		for _, cell := range row {
			body.WriteString(`<w:tc><w:p><w:r><w:t>`)
			body.WriteString(escapeDocxText(cell))
			body.WriteString(`</w:t></w:r></w:p></w:tc>`)
		}
		body.WriteString(`</w:tr>`)
	}
	body.WriteString(`</w:tbl></w:body></w:document>`)
	add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`)
	add("word/document.xml", body.String())
	if err := zw.Close(); err != nil {
		t.Fatalf("close docx zip: %v", err)
	}
	return buf.Bytes()
}

func escapeDocxText(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(text)
}

func TestVTextEnsureManifestCreatesAliasAndFile(t *testing.T) {
	h, s, _ := vtextAPISetupWithRuntime(t)
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)

	docID, _ := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/manifest", nil)
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ensure manifest: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp vtextEnsureManifestResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ensure manifest response: %v", err)
	}
	if resp.DocID != docID {
		t.Fatalf("response doc_id = %q, want %q", resp.DocID, docID)
	}
	if resp.SourcePath == "" {
		t.Fatal("response source_path should not be empty")
	}
	if filepath.Ext(resp.SourcePath) != ".vtext" {
		t.Fatalf("response source_path extension = %q, want .vtext", filepath.Ext(resp.SourcePath))
	}

	aliasedDocID, err := s.GetDocumentAlias(context.Background(), "user-1", resp.SourcePath)
	if err != nil {
		t.Fatalf("GetDocumentAlias: %v", err)
	}
	if aliasedDocID != docID {
		t.Fatalf("aliased doc_id = %q, want %q", aliasedDocID, docID)
	}

	bytes, err := os.ReadFile(filepath.Join(filesRoot, filepath.FromSlash(resp.SourcePath)))
	if err != nil {
		t.Fatalf("read manifest file: %v", err)
	}
	var shortcut vtextShortcutFile
	if err := json.Unmarshal(bytes, &shortcut); err != nil {
		t.Fatalf("unmarshal shortcut file: %v\nraw=%s", err, string(bytes))
	}
	if shortcut.Kind != "vtext" {
		t.Fatalf("shortcut kind = %q, want %q", shortcut.Kind, "vtext")
	}
	if shortcut.DocID != docID {
		t.Fatalf("shortcut doc_id = %q, want %q", shortcut.DocID, docID)
	}
	if shortcut.SourcePath != resp.SourcePath {
		t.Fatalf("shortcut source_path = %q, want %q", shortcut.SourcePath, resp.SourcePath)
	}
}

func TestVTextEnsureManifestReusesExistingAlias(t *testing.T) {
	h, s, _ := vtextAPISetupWithRuntime(t)
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)

	docID, _ := createDocWithUserRevision(t, h)

	firstReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/manifest", nil)
	firstW := httptest.NewRecorder()
	h.HandleVTextRouter(firstW, firstReq)
	if firstW.Code != http.StatusOK {
		t.Fatalf("first ensure manifest: status = %d, want %d; body: %s", firstW.Code, http.StatusOK, firstW.Body.String())
	}
	var firstResp vtextEnsureManifestResponse
	if err := json.NewDecoder(firstW.Body).Decode(&firstResp); err != nil {
		t.Fatalf("decode first ensure manifest response: %v", err)
	}

	secondReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/manifest", nil)
	secondW := httptest.NewRecorder()
	h.HandleVTextRouter(secondW, secondReq)
	if secondW.Code != http.StatusOK {
		t.Fatalf("second ensure manifest: status = %d, want %d; body: %s", secondW.Code, http.StatusOK, secondW.Body.String())
	}
	var secondResp vtextEnsureManifestResponse
	if err := json.NewDecoder(secondW.Body).Decode(&secondResp); err != nil {
		t.Fatalf("decode second ensure manifest response: %v", err)
	}
	if secondResp.SourcePath != firstResp.SourcePath {
		t.Fatalf("second source_path = %q, want %q", secondResp.SourcePath, firstResp.SourcePath)
	}

	sourcePath, err := s.GetDocumentAliasSourcePath(context.Background(), "user-1", docID)
	if err != nil {
		t.Fatalf("GetDocumentAliasSourcePath: %v", err)
	}
	if sourcePath != firstResp.SourcePath {
		t.Fatalf("stored source_path = %q, want %q", sourcePath, firstResp.SourcePath)
	}
}

func TestVTextCreateRevisionRejectsStaleHead(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	headReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", vtextCreateRevisionRequest{
		Content:          "Latest head",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
	})
	headW := httptest.NewRecorder()
	h.HandleVTextRevisions(headW, headReq)
	if headW.Code != http.StatusCreated {
		t.Fatalf("create head revision: status = %d, want %d; body: %s", headW.Code, http.StatusCreated, headW.Body.String())
	}

	staleReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", vtextCreateRevisionRequest{
		Content:          "Stale write",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
	})
	staleW := httptest.NewRecorder()
	h.HandleVTextRevisions(staleW, staleReq)
	if staleW.Code != http.StatusConflict {
		t.Fatalf("stale create revision: status = %d, want %d; body: %s", staleW.Code, http.StatusConflict, staleW.Body.String())
	}
}

func TestVTextCreateRevisionRebasesAllowedStaleUserDraft(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	headReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", vtextCreateRevisionRequest{
		Content:          "Initial content.\n\nAgent-added latest head detail.",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
	})
	headW := httptest.NewRecorder()
	h.HandleVTextRevisions(headW, headReq)
	if headW.Code != http.StatusCreated {
		t.Fatalf("create newer head revision: status = %d, want %d; body: %s", headW.Code, http.StatusCreated, headW.Body.String())
	}
	var headResp vtextRevisionResponse
	if err := json.NewDecoder(headW.Body).Decode(&headResp); err != nil {
		t.Fatalf("decode head response: %v", err)
	}

	staleReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", vtextCreateRevisionRequest{
		Content:          "Initial content.\n\nUser dirty draft detail.",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
		AllowRebase:      true,
		Metadata:         json.RawMessage(`{"autosaved":true}`),
	})
	staleW := httptest.NewRecorder()
	h.HandleVTextRevisions(staleW, staleReq)
	if staleW.Code != http.StatusCreated {
		t.Fatalf("rebased stale revision: status = %d, want %d; body: %s", staleW.Code, http.StatusCreated, staleW.Body.String())
	}
	var rebasedResp vtextRevisionResponse
	if err := json.NewDecoder(staleW.Body).Decode(&rebasedResp); err != nil {
		t.Fatalf("decode rebased response: %v", err)
	}
	if rebasedResp.ParentRevisionID != headResp.RevisionID {
		t.Fatalf("rebased parent = %q, want latest head %q", rebasedResp.ParentRevisionID, headResp.RevisionID)
	}
	for _, want := range []string{"Agent-added latest head detail.", "User dirty draft detail."} {
		if !strings.Contains(rebasedResp.Content, want) {
			t.Fatalf("rebased content missing %q:\n%s", want, rebasedResp.Content)
		}
	}
	meta := decodeRevisionMetadata(rebasedResp.Metadata)
	if got, _ := meta["rebased_from_revision_id"].(string); got != baseRevisionID {
		t.Fatalf("rebased_from_revision_id = %q, want %q; metadata=%+v", got, baseRevisionID, meta)
	}
	if got, _ := meta["rebase_onto_revision_id"].(string); got != headResp.RevisionID {
		t.Fatalf("rebase_onto_revision_id = %q, want %q; metadata=%+v", got, headResp.RevisionID, meta)
	}

	doc, err := s.GetDocument(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != rebasedResp.RevisionID {
		t.Fatalf("current head = %q, want rebased revision %q", doc.CurrentRevisionID, rebasedResp.RevisionID)
	}
}

func TestVTextDocumentStreamSendsSnapshot(t *testing.T) {
	t.Parallel()
	h, s := vtextAPISetup(t)
	docID, _ := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docID+"/stream", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleVTextDocumentStream(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if got := w.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}

	doc, err := s.GetDocument(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}

	var foundSnapshot bool
	for _, ev := range parseVTextStreamEvents(t, w.Body.String()) {
		if ev.Kind != "snapshot" {
			continue
		}
		foundSnapshot = true
		if ev.DocID != docID {
			t.Fatalf("snapshot doc_id = %q, want %q", ev.DocID, docID)
		}
		if ev.CurrentRevisionID != doc.CurrentRevisionID {
			t.Fatalf("snapshot current_revision_id = %q, want %q", ev.CurrentRevisionID, doc.CurrentRevisionID)
		}
		if ev.Pending {
			t.Fatal("snapshot should not report a pending mutation")
		}
	}
	if !foundSnapshot {
		t.Fatal("expected snapshot event in document stream")
	}
}

func TestVTextDocumentResponseReportsPendingAgentMutation(t *testing.T) {
	t.Parallel()
	h, s := vtextAPISetup(t)
	docID, _ := createDocWithUserRevision(t, h)
	if err := s.CreateAgentMutation(context.Background(), store.AgentMutation{
		DocID:     docID,
		RunID:     "run-vtext-pending-ui",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create pending mutation: %v", err)
	}

	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docID, nil)
	w := httptest.NewRecorder()
	h.HandleVTextDocument(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get document: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp vtextDocumentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode document response: %v", err)
	}
	if !resp.AgentRevisionPending {
		t.Fatalf("agent_revision_pending = false, want true; response=%+v", resp)
	}
	if resp.AgentRevisionRunID != "run-vtext-pending-ui" {
		t.Fatalf("agent_revision_run_id = %q, want run-vtext-pending-ui", resp.AgentRevisionRunID)
	}
}

func TestVTextDocumentResponseReconcilesPendingMutationFromCurrentHead(t *testing.T) {
	t.Parallel()
	h, s := vtextAPISetup(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)
	runID := "run-vtext-head-already-written"
	if err := s.CreateAgentMutation(context.Background(), store.AgentMutation{
		DocID:     docID,
		RunID:     runID,
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create pending mutation: %v", err)
	}
	meta, _ := json.Marshal(map[string]any{
		"source":  "edit_vtext",
		"loop_id": runID,
	})
	if err := s.CreateRevision(context.Background(), types.Revision{
		RevisionID:       "rev-appagent-current-head",
		DocID:            docID,
		OwnerID:          "user-1",
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		Content:          "Hello, edited document.",
		Citations:        json.RawMessage("[]"),
		Metadata:         meta,
		ParentRevisionID: baseRevisionID,
		CreatedAt:        time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create appagent head revision: %v", err)
	}

	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docID, nil)
	w := httptest.NewRecorder()
	h.HandleVTextDocument(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get document: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp vtextDocumentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode document response: %v", err)
	}
	if resp.AgentRevisionPending {
		t.Fatalf("agent_revision_pending = true after current head reconciliation; response=%+v", resp)
	}
	mutation, err := s.GetAgentMutationByRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("get mutation: %v", err)
	}
	if mutation.State != "completed" || mutation.RevisionID != "rev-appagent-current-head" {
		t.Fatalf("mutation not reconciled to completed current head: %+v", mutation)
	}
}

func TestVTextDiagnosisReportsCurrentRevisionVersion(t *testing.T) {
	t.Parallel()
	h, s := vtextAPISetup(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)
	if err := s.CreateRevision(context.Background(), types.Revision{
		RevisionID:       "rev-diagnosis-v1",
		DocID:            docID,
		OwnerID:          "user-1",
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		Content:          "## Appendix\n\n| Owner | State |\n| --- | --- |\n| Legal cloud | Preserved [source](source:src-legal-cloud) |\n",
		Citations:        json.RawMessage("[]"),
		Metadata:         json.RawMessage(`{"source":"edit_vtext"}`),
		ParentRevisionID: baseRevisionID,
		CreatedAt:        time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create second revision: %v", err)
	}

	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docID+"/diagnosis?limit=10", nil)
	w := httptest.NewRecorder()
	h.HandleVTextDiagnosis(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("diagnosis status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp vtextDiagnosisResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode diagnosis: %v", err)
	}
	if resp.Document == nil {
		t.Fatal("diagnosis missing document summary")
	}
	if resp.Document.CurrentRevisionID != "rev-diagnosis-v1" || resp.Document.CurrentVersionNumber != 1 || resp.Document.LastAuthorKind != string(types.AuthorAppAgent) {
		t.Fatalf("diagnosis document summary = %+v, want current v1 appagent head", resp.Document)
	}
	if len(resp.Revisions) == 0 || resp.Revisions[0].RevisionID != "rev-diagnosis-v1" || resp.Revisions[0].VersionNumber != 1 {
		t.Fatalf("diagnosis revisions = %+v, want latest v1 first", resp.Revisions)
	}
	if len(resp.RevisionStructures) == 0 || resp.RevisionStructures[0].RevisionID != "rev-diagnosis-v1" {
		t.Fatalf("diagnosis revision structures = %+v, want latest v1 first", resp.RevisionStructures)
	}
	structure := resp.RevisionStructures[0]
	if structure.ContentHash == "" || structure.HeadingCount != 1 || structure.SourceMarkerCount != 1 {
		t.Fatalf("diagnosis structure counts/hash = %+v", structure)
	}
	if structure.TableCount != 1 || structure.TableRowCount != 3 || len(structure.Tables) != 1 {
		t.Fatalf("diagnosis table structure = %+v", structure)
	}
	if table := structure.Tables[0]; table.StartLine != 3 || table.EndLine != 5 || table.ColumnCount != 2 || table.RowCount != 3 || !table.HasSeparator || table.Signature == "" {
		t.Fatalf("diagnosis table signature = %+v", table)
	}
}

func TestVTextDiagnosisCanOmitRevisionContentForStructureEvidence(t *testing.T) {
	t.Parallel()
	h, s := vtextAPISetup(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)
	if err := s.CreateRevision(context.Background(), types.Revision{
		RevisionID:       "rev-diagnosis-structure-only",
		DocID:            docID,
		OwnerID:          "user-1",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "owner",
		Content:          "| Term | Meaning |\n| --- | --- |\n| Work product | Durable output |\n",
		Citations:        json.RawMessage("[]"),
		Metadata:         json.RawMessage(`{"source":"owner_edit"}`),
		ParentRevisionID: baseRevisionID,
		CreatedAt:        time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create structure revision: %v", err)
	}

	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docID+"/diagnosis?limit=10&include_content=false", nil)
	w := httptest.NewRecorder()
	h.HandleVTextDiagnosis(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("diagnosis status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp vtextDiagnosisResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode diagnosis: %v", err)
	}
	if len(resp.Revisions) != 0 {
		t.Fatalf("diagnosis include_content=false returned full revisions: %+v", resp.Revisions)
	}
	if len(resp.RevisionStructures) == 0 {
		t.Fatalf("diagnosis include_content=false omitted structure summaries: %+v", resp)
	}
	if got := resp.RevisionStructures[0]; got.RevisionID != "rev-diagnosis-structure-only" || got.TableCount != 1 || got.ContentHash == "" {
		t.Fatalf("diagnosis structure-only summary = %+v", got)
	}
	if strings.Contains(w.Body.String(), "Work product") || strings.Contains(w.Body.String(), "Durable output") {
		t.Fatalf("diagnosis include_content=false leaked revision body: %s", w.Body.String())
	}
}

func TestVTextDocumentStreamEmitsHeadChangeAfterAgentRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	docID, _ := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docID+"/stream", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleVTextDocumentStream(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	revReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it more formal"})
	revW := httptest.NewRecorder()
	h.HandleVTextAgentRevision(revW, revReq)
	if revW.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", revW.Code, http.StatusAccepted, revW.Body.String())
	}

	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(revW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("task state = %q, want %q", state, types.RunCompleted)
	}

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	doc, err := s.GetDocument(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}

	var foundStarted, foundCompleted, foundRevisionCreated, foundHeadChanged bool
	for _, ev := range parseVTextStreamEvents(t, w.Body.String()) {
		switch ev.Kind {
		case "synth_started":
			foundStarted = true
		case "synth_completed":
			foundCompleted = true
		case "revision_created":
			foundRevisionCreated = true
			if ev.RevisionID == "" {
				t.Fatal("revision_created event missing revision_id")
			}
		case "head_changed":
			foundHeadChanged = true
			if ev.CurrentRevisionID != doc.CurrentRevisionID {
				t.Fatalf("head_changed current_revision_id = %q, want %q", ev.CurrentRevisionID, doc.CurrentRevisionID)
			}
		}
	}
	if !foundStarted {
		t.Fatal("expected synth_started event")
	}
	if !foundCompleted {
		t.Fatal("expected synth_completed event")
	}
	if !foundRevisionCreated {
		t.Fatal("expected revision_created event")
	}
	if !foundHeadChanged {
		t.Fatal("expected head_changed event")
	}
}

func TestVTextDocumentStreamEmitsHeadChangeAfterUserRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docID+"/stream", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleVTextDocumentStream(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	createReq := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", vtextCreateRevisionRequest{
		Content:          "User-authored next head",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
	})
	createW := httptest.NewRecorder()
	h.HandleVTextRevisions(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create revision: status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	doc, err := s.GetDocument(context.Background(), docID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}

	var foundRevisionCreated, foundHeadChanged bool
	for _, ev := range parseVTextStreamEvents(t, w.Body.String()) {
		switch ev.Kind {
		case "revision_created":
			foundRevisionCreated = true
			if ev.RevisionID == "" {
				t.Fatal("revision_created event missing revision_id")
			}
		case "head_changed":
			foundHeadChanged = true
			if ev.CurrentRevisionID != doc.CurrentRevisionID {
				t.Fatalf("head_changed current_revision_id = %q, want %q", ev.CurrentRevisionID, doc.CurrentRevisionID)
			}
		}
	}
	if !foundRevisionCreated {
		t.Fatal("expected revision_created event")
	}
	if !foundHeadChanged {
		t.Fatal("expected head_changed event")
	}
}

func parseVTextStreamEvents(t *testing.T, body string) []vtextDocumentStreamEvent {
	t.Helper()
	lines := strings.Split(body, "\n")
	events := make([]vtextDocumentStreamEvent, 0)
	for _, line := range lines {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev vtextDocumentStreamEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatalf("decode vtext stream event: %v", err)
		}
		events = append(events, ev)
	}
	return events
}

// TestVTextAgentRevisionAuthRequired verifies that agent revision
// requires authentication (VAL-ETEXT-003: auth-gated).
func TestVTextAgentRevisionAuthRequired(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// No auth header.
	req := httptest.NewRequest(http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		bytes.NewReader([]byte(`{"prompt":"test"}`)))
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// TestVTextAgentRevisionPreservesUserAndAppAgentAttribution verifies
// that an end-to-end flow preserves both user and appagent attribution
// in history (VAL-CROSS-119).
func TestVTextAgentRevisionPreservesUserAndAppAgentAttribution(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Submit an agent revision.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Improve the text"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	var resp vtextAgentRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)

	// Wait for completion.
	state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("task state = %q, want %q", state, types.RunCompleted)
	}

	// Make another user edit after the agent revision.
	revReq := vtextCreateRevisionRequest{
		Content:     "User final edit",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("user edit after agent: status = %d, body: %s", w.Code, w.Body.String())
	}

	// Get the history and verify both user and appagent attribution.
	entries, err := s.GetHistory(context.Background(), docID, "user-1", 10)
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(history) = %d, want 3", len(entries))
	}

	// History is newest-first.
	// Entry 0: latest user edit
	// Entry 1: appagent revision
	// Entry 2: initial user edit
	if entries[0].AuthorKind != types.AuthorUser {
		t.Errorf("entry 0 AuthorKind = %q, want %q", entries[0].AuthorKind, types.AuthorUser)
	}
	if entries[1].AuthorKind != types.AuthorAppAgent {
		t.Errorf("entry 1 AuthorKind = %q, want %q", entries[1].AuthorKind, types.AuthorAppAgent)
	}
	if entries[2].AuthorKind != types.AuthorUser {
		t.Errorf("entry 2 AuthorKind = %q, want %q", entries[2].AuthorKind, types.AuthorUser)
	}

	// Verify that the appagent revision has the correct label.
	if entries[1].AuthorLabel != "appagent" {
		t.Errorf("entry 1 AuthorLabel = %q, want %q", entries[1].AuthorLabel, "appagent")
	}
}

// TestVTextAgentRevisionNoWorkerAuthorship verifies that when subordinate
// workers might contribute to an appagent-driven change, the resulting
// canonical history attributes the change to the appagent, not to any
// worker identity (VAL-CROSS-120).
func TestVTextAgentRevisionNoWorkerAuthorship(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Submit an agent revision.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it better"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	var resp vtextAgentRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)

	// Wait for completion.
	state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("task state = %q, want %q", state, types.RunCompleted)
	}

	// Verify that no "worker" author kind exists in the history.
	entries, err := s.GetHistory(context.Background(), docID, "user-1", 10)
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	for _, entry := range entries {
		if entry.AuthorKind != types.AuthorUser && entry.AuthorKind != types.AuthorAppAgent {
			t.Errorf("found non-canonical author_kind %q in history — workers must not be canonical authors (VAL-CROSS-120)", entry.AuthorKind)
		}
	}
}

// TestVTextAgentRevisionNoDuplicateOnRenewalRetry verifies that renewal
// and retry does not duplicate a canonical document mutation (VAL-CROSS-122).
func TestVTextAgentRevisionNoDuplicateOnRenewalRetry(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Submit an agent revision.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it concise"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	var resp1 vtextAgentRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&resp1)

	// Simulate a renewal/retry by submitting the same request again
	// before the task completes. The idempotency check should return
	// the same task ID.
	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it concise"})
	w = httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)

	var resp2 vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode retry response: %v", err)
	}

	// The retry should return the same task ID (idempotent).
	if resp2.RunID != resp1.RunID {
		t.Errorf("retry returned different task ID: %q vs %q — should be idempotent (VAL-CROSS-122)", resp2.RunID, resp1.RunID)
	}

	// Wait for the task to complete.
	state := waitForTaskCompletion(t, h, resp1.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("task state = %q, want %q", state, types.RunCompleted)
	}

	// Verify only one appagent revision was created (no duplicate).
	revs, err := s.ListRevisionsByDoc(context.Background(), docID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}

	agentCount := 0
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent {
			agentCount++
		}
	}
	if agentCount != 1 {
		t.Errorf("found %d appagent revisions, want 1 — duplicate mutation detected (VAL-CROSS-122)", agentCount)
	}
}

// TestVTextAgentRevisionMutationCompletedOnlyOnce verifies that edit_vtext is
// the idempotency boundary for canonical appagent revisions (VAL-CROSS-122).
func TestVTextAppagentEditCanonicalizesAliasedMarkdownTitle(t *testing.T) {
	t.Parallel()
	_, s, rt := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	doc := types.Document{
		DocID:     "doc-legacy-md-agent",
		OwnerID:   "user-1",
		Title:     "legacy-proposal.md",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s.UpsertDocumentAlias(ctx, "user-1", "proposals/legacy-proposal.md", doc.DocID, now); err != nil {
		t.Fatalf("upsert document alias: %v", err)
	}
	base := types.Revision{
		RevisionID:  "rev-legacy-md-v0",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "# Legacy Proposal\n\nImported Markdown body.",
		Metadata: buildFileOpenVTextMetadata(vtextFileImportProjection{
			SourcePath:            "proposals/legacy-proposal.md",
			MediaType:             "text/markdown",
			ProjectionContent:     "# Legacy Proposal\n\nImported Markdown body.",
			ProjectionContentHash: contentHash("# Legacy Proposal\n\nImported Markdown body."),
			OriginalContentHash:   contentHash("# Legacy Proposal\n\nImported Markdown body."),
			ImportAdapter:         "vtext_file_open_projection",
			ImportAdapterVersion:  1,
		}, nil),
		CreatedAt: now,
	}
	if err := s.CreateRevision(ctx, base); err != nil {
		t.Fatalf("create base revision: %v", err)
	}
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:     doc.DocID,
		RunID:     "run-legacy-md-agent",
		OwnerID:   doc.OwnerID,
		State:     "pending",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create agent mutation: %v", err)
	}
	run := &types.RunRecord{
		RunID:        "run-legacy-md-agent",
		AgentID:      "vtext:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-vtext-test",
		State:        types.RunCompleted,
		Prompt:       "Revise the imported markdown proposal.",
		CreatedAt:    now,
		UpdatedAt:    now,
		AgentProfile: AgentProfileVText,
		AgentRole:    AgentProfileVText,
		Metadata: map[string]any{
			"type":                "vtext_agent_revision",
			"doc_id":              doc.DocID,
			"current_revision_id": base.RevisionID,
			runMetadataAgentID:    "vtext:" + doc.DocID,
			runMetadataChannelID:  doc.DocID,
		},
	}
	rawArgs, err := json.Marshal(editVTextArgs{
		DocID:          doc.DocID,
		BaseRevisionID: base.RevisionID,
		Operation:      "apply_edits",
		Edits: []vtextTextEdit{{
			Op:      "replace",
			Find:    "Imported Markdown body.",
			Replace: "Imported Markdown body revised as canonical VText.",
		}},
	})
	if err != nil {
		t.Fatalf("marshal edit args: %v", err)
	}
	if _, err := rt.ToolRegistryForProfile(AgentProfileVText).Execute(WithToolExecutionContext(ctx, run), "edit_vtext", rawArgs); err != nil {
		t.Fatalf("edit_vtext: %v", err)
	}
	got, err := s.GetDocument(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if got.Title != "legacy-proposal.vtext" {
		t.Fatalf("document title = %q, want legacy-proposal.vtext", got.Title)
	}
	revs, err := s.ListRevisionsByDoc(ctx, doc.DocID, doc.OwnerID, 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 2 || revs[0].VersionNumber != 1 || !strings.Contains(revs[0].Content, "canonical VText") {
		t.Fatalf("revisions = %+v, want appagent v1 canonical edit", revs)
	}
	meta := decodeRevisionMetadata(revs[0].Metadata)
	canonicalPath, ok := meta["canonical_vtext_source_path"].(string)
	if !ok || filepath.Ext(canonicalPath) != ".vtext" {
		t.Fatalf("canonical_vtext_source_path = %#v, want .vtext", meta["canonical_vtext_source_path"])
	}
	if meta["import_manifest"] == nil || meta["migration_manifest"] == nil {
		t.Fatalf("appagent v1 did not carry import/migration metadata: %#v", meta)
	}
	migrationManifest := meta["migration_manifest"].(map[string]any)
	if migrationManifest["migration_adapter"] != "markdown_to_vtext_projection" {
		t.Fatalf("appagent migration_manifest = %#v", migrationManifest)
	}
	sourcePath, err := s.GetDocumentAliasSourcePath(ctx, doc.OwnerID, doc.DocID)
	if err != nil {
		t.Fatalf("GetDocumentAliasSourcePath: %v", err)
	}
	if sourcePath != canonicalPath {
		t.Fatalf("latest alias source path = %q, want canonical path %q", sourcePath, canonicalPath)
	}
	if docID, err := s.GetDocumentAlias(ctx, doc.OwnerID, canonicalPath); err != nil || docID != doc.DocID {
		t.Fatalf("canonical alias docID = %q, err = %v, want %q", docID, err, doc.DocID)
	}
	if docID, err := s.GetDocumentAlias(ctx, doc.OwnerID, "proposals/legacy-proposal.md"); err != nil || docID != doc.DocID {
		t.Fatalf("original markdown alias docID = %q, err = %v, want %q", docID, err, doc.DocID)
	}
}

func TestVTextAgentRevisionMutationCompletedOnlyOnce(t *testing.T) {
	t.Parallel()
	_, s, rt := vtextAPISetupWithRuntime(t)

	ctx := context.Background()

	// Create a document manually.
	doc := types.Document{
		DocID:     "doc-mutation-test",
		OwnerID:   "user-1",
		Title:     "Mutation Test",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}

	// Create a user revision.
	rev := types.Revision{
		RevisionID:  "rev-user-1",
		DocID:       "doc-mutation-test",
		OwnerID:     "user-1",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Content:     "Original content",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, rev); err != nil {
		t.Fatalf("create revision: %v", err)
	}

	// Create an agent mutation record.
	mutation := store.AgentMutation{
		DocID:     "doc-mutation-test",
		RunID:     "task-mutation-test",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateAgentMutation(ctx, mutation); err != nil {
		t.Fatalf("create agent mutation: %v", err)
	}

	// Create a completed task record with vtext agent revision metadata.
	taskRec := &types.RunRecord{
		RunID:        "task-mutation-test",
		AgentID:      "vtext:doc-mutation-test",
		ChannelID:    "doc-mutation-test",
		OwnerID:      "user-1",
		SandboxID:    "sandbox-vtext-test",
		State:        types.RunCompleted,
		Prompt:       "Revise the document",
		Result:       vtextReplaceAllResult("Revised content", "rev-user-1"),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileVText,
		AgentRole:    AgentProfileVText,
		Metadata: map[string]any{
			"type":                  "vtext_agent_revision",
			"doc_id":                "doc-mutation-test",
			"current_revision_id":   "rev-user-1",
			runMetadataAgentID:      "vtext:doc-mutation-test",
			runMetadataChannelID:    "doc-mutation-test",
			runMetadataAgentRole:    AgentProfileVText,
			runMetadataAgentProfile: AgentProfileVText,
		},
	}

	vtextRegistry := rt.ToolRegistryForProfile(AgentProfileVText)
	rawArgs, err := json.Marshal(editVTextArgs{
		DocID:          "doc-mutation-test",
		BaseRevisionID: "rev-user-1",
		Operation:      "replace_all",
		Content:        "Revised content",
	})
	if err != nil {
		t.Fatalf("marshal edit_vtext args: %v", err)
	}
	if _, err := vtextRegistry.Execute(WithToolExecutionContext(ctx, taskRec), "edit_vtext", rawArgs); err != nil {
		t.Fatalf("first edit_vtext: %v", err)
	}
	if _, err := vtextRegistry.Execute(WithToolExecutionContext(ctx, taskRec), "edit_vtext", rawArgs); err == nil {
		t.Fatal("second edit_vtext should be rejected after mutation completion")
	}

	// Call handleRunCompletion twice to simulate duplicate recovery processing.
	rt.handleRunCompletion(ctx, taskRec)
	rt.handleRunCompletion(ctx, taskRec)

	// Verify only one appagent revision was created.
	revs, err := s.ListRevisionsByDoc(ctx, "doc-mutation-test", "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}

	agentCount := 0
	for _, r := range revs {
		if r.AuthorKind == types.AuthorAppAgent {
			agentCount++
		}
	}
	if agentCount != 1 {
		t.Errorf("found %d appagent revisions, want 1 — duplicate canonical revision detected (VAL-CROSS-122)", agentCount)
	}
}

func TestEditVTextInitialWorkingRevisionDoesNotSmuggleRequiredContinuation(t *testing.T) {
	t.Parallel()
	_, s, rt := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	doc := types.Document{
		DocID:     "doc-initial-continuation",
		OwnerID:   "user-1",
		Title:     "NBA update",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	userRev := types.Revision{
		RevisionID:  "rev-user-continuation",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     "nba update",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, userRev); err != nil {
		t.Fatalf("create user revision: %v", err)
	}
	run := types.RunRecord{
		RunID:        "run-vtext-continuation",
		AgentID:      "vtext:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-vtext-test",
		State:        types.RunRunning,
		Prompt:       "Revise the document",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileVText,
		AgentRole:    AgentProfileVText,
		Metadata: map[string]any{
			"type":                  "vtext_agent_revision",
			"doc_id":                doc.DocID,
			"current_revision_id":   userRev.RevisionID,
			"original_prompt":       "nba update",
			runMetadataAgentID:      "vtext:" + doc.DocID,
			runMetadataChannelID:    doc.DocID,
			runMetadataAgentRole:    AgentProfileVText,
			runMetadataAgentProfile: AgentProfileVText,
		},
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:     doc.DocID,
		RunID:     run.RunID,
		OwnerID:   doc.OwnerID,
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create mutation: %v", err)
	}

	registry := rt.ToolRegistryForProfile(AgentProfileVText)
	editRaw, err := registry.Execute(WithToolExecutionContext(ctx, &run), "edit_vtext", json.RawMessage(`{
		"doc_id":"doc-initial-continuation",
		"base_revision_id":"rev-user-continuation",
		"operation":"replace_all",
		"content":"# NBA update\n\nI am preparing a short working update and checking current evidence next."
	}`))
	if err != nil {
		t.Fatalf("edit_vtext: %v", err)
	}
	var editResult map[string]any
	if err := json.Unmarshal([]byte(editRaw), &editResult); err != nil {
		t.Fatalf("decode edit result: %v", err)
	}
	if _, ok := editResult["next_required_tool"]; ok {
		t.Fatalf("edit_vtext must not smuggle a required continuation; result=%s", editRaw)
	}

	spawnRaw, err := registry.Execute(WithToolExecutionContext(ctx, &run), "spawn_agent", json.RawMessage(`{
		"objective":"Research the user's NBA update request and send initial findings quickly.",
		"role":"researcher",
		"channel_id":"doc-initial-continuation"
	}`))
	if err != nil {
		t.Fatalf("spawn_agent after initial edit: %v", err)
	}
	var spawnResult map[string]any
	if err := json.Unmarshal([]byte(spawnRaw), &spawnResult); err != nil {
		t.Fatalf("decode spawn result: %v", err)
	}
	if _, ok := spawnResult["next_required_tool"]; ok {
		t.Fatalf("spawn_agent after completed edit must not require a second edit_vtext; result=%s", spawnRaw)
	}
}

func TestVTextApplyEditsRejectsAmbiguousReplace(t *testing.T) {
	t.Parallel()
	current := types.Revision{
		RevisionID: "rev-1",
		Content:    "repeat\nkeep\nrepeat",
	}
	_, err := materializeVTextToolEdit(editVTextArgs{
		BaseRevisionID: "rev-1",
		Operation:      "apply_edits",
		Edits: []vtextTextEdit{{
			Op:      "replace",
			Find:    "repeat",
			Replace: "changed",
		}},
	}, current)
	if err == nil || !strings.Contains(err.Error(), "matched 2 times") {
		t.Fatalf("ambiguous replace err = %v, want duplicate-match rejection", err)
	}

	got, err := materializeVTextToolEdit(editVTextArgs{
		BaseRevisionID: "rev-1",
		Operation:      "apply_edits",
		Edits: []vtextTextEdit{{
			Op:         "replace",
			Find:       "repeat",
			Replace:    "changed",
			ReplaceAll: true,
		}},
	}, current)
	if err != nil {
		t.Fatalf("replace_all edit: %v", err)
	}
	if got.Content != "changed\nkeep\nchanged" {
		t.Fatalf("content = %q, want all matches replaced", got.Content)
	}
}

// TestVTextAgentRevisionProgressEvents verifies that progress events
// are emitted during agent revision execution with the doc_id so
// the frontend can correlate to the open document (VAL-ETEXT-004).
func TestVTextAgentRevisionProgressEvents(t *testing.T) {
	t.Parallel()
	h, s, _ := vtextAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Subscribe to events before submitting the task.
	bus := s // We'll use the store to query events after completion.

	// Submit an agent revision.
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"prompt": "Add more detail"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	var resp vtextAgentRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)

	// Wait for completion.
	state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("task state = %q, want %q", state, types.RunCompleted)
	}

	// Check that vtext agent revision events were persisted.
	events, err := bus.ListEvents(context.Background(), resp.RunID, 200)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	// We should find vtext.agent_revision.started and
	// vtext.agent_revision.completed events.
	var foundStarted, foundCompleted bool
	for _, ev := range events {
		switch ev.Kind {
		case types.EventVTextAgentRevisionStarted:
			foundStarted = true
			// Verify the payload contains doc_id.
			var payload map[string]string
			if err := json.Unmarshal(ev.Payload, &payload); err == nil {
				if payload["doc_id"] != docID {
					t.Errorf("started event doc_id = %q, want %q", payload["doc_id"], docID)
				}
			}
		case types.EventVTextAgentRevisionCompleted:
			foundCompleted = true
			var payload map[string]string
			if err := json.Unmarshal(ev.Payload, &payload); err == nil {
				if payload["doc_id"] != docID {
					t.Errorf("completed event doc_id = %q, want %q", payload["doc_id"], docID)
				}
				if payload["revision_id"] == "" {
					t.Error("completed event missing revision_id")
				}
			}
		}
	}
	if !foundStarted {
		t.Error("missing vtext.agent_revision.started event (VAL-ETEXT-004)")
	}
	if !foundCompleted {
		t.Error("missing vtext.agent_revision.completed event (VAL-ETEXT-004)")
	}
}

// TestVTextAgentRevisionAcceptsReviseEventWithoutPrompt verifies that the
// frontend can submit a plain revise event and let the backend compile the
// effective vtext request from document state.
func TestVTextAgentRevisionAcceptsReviseEventWithoutPrompt(t *testing.T) {
	t.Parallel()
	h, _, rt := vtextAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"intent": "revise"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	task, err := rt.GetRun(context.Background(), resp.RunID, "user-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !strings.Contains(task.Prompt, "A revise event was triggered") {
		t.Fatalf("compiled prompt missing revise event context: %q", task.Prompt)
	}
	if !strings.Contains(task.Prompt, "Hello, world!") {
		t.Fatalf("compiled prompt missing current document content: %q", task.Prompt)
	}
}

func TestVTextDiagnosisIncludesDocumentChannelRuns(t *testing.T) {
	t.Parallel()
	h, _, rt := vtextAPISetupWithRuntime(t)
	docID, _ := createDocWithUserRevision(t, h)

	docRun, err := rt.StartRunWithMetadata(context.Background(), "diagnose this document", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      "vtext:" + docID,
		runMetadataChannelID:    "legacy-parent-channel",
		"doc_id":                docID,
	})
	if err != nil {
		t.Fatalf("start doc run: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := rt.StartRunWithMetadata(context.Background(), fmt.Sprintf("newer unrelated run %d", i), "user-1", map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
			runMetadataAgentRole:    AgentProfileSuper,
			runMetadataChannelID:    fmt.Sprintf("unrelated-%d", i),
		}); err != nil {
			t.Fatalf("start unrelated run %d: %v", i, err)
		}
	}

	req := vtextRequest(t, http.MethodGet, "/api/vtext/documents/"+docID+"/diagnosis?limit=3", nil)
	w := httptest.NewRecorder()
	h.HandleVTextRouter(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("diagnosis status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp vtextDiagnosisResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode diagnosis: %v", err)
	}
	for _, run := range resp.Runs {
		if run.RunID == docRun.RunID {
			return
		}
	}
	t.Fatalf("diagnosis omitted document run %s; runs=%+v", docRun.RunID, resp.Runs)
}

func TestVTextAgentRevisionRegistersMediaSourceRefs(t *testing.T) {
	t.Setenv("CHOIR_DISABLE_YOUTUBE_TRANSCRIPT_FETCH", "1")
	previous := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() {
		sourcefetch.SetAllowPrivateNetworkForTests(previous)
	})
	h, s, rt := vtextAPISetupWithRuntime(t)

	image := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer image.Close()

	docID, _ := createDocWithUserRevision(t, h)
	content := "Review these sources:\n\nhttps://www.youtube.com/watch?v=dQw4w9WgXcQ\n\n" + image.URL + "/cover.png"
	revReq := vtextCreateRevisionRequest{
		Content:     content,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revisions", revReq)
	w := httptest.NewRecorder()
	h.HandleVTextRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create media revision: status = %d body=%s", w.Code, w.Body.String())
	}

	req = vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"intent": "revise"})
	w = httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("revise status = %d body=%s", w.Code, w.Body.String())
	}
	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	run, err := rt.GetRun(context.Background(), resp.RunID, "user-1")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	refs := decodeVTextMediaSourceRefs(run.Metadata["media_source_refs"])
	if len(refs) != 2 {
		t.Fatalf("media_source_refs len = %d, want 2: %#v", len(refs), refs)
	}
	sourceEntities := decodeVTextSourceEntities(run.Metadata["source_entities"])
	if len(sourceEntities) != 2 {
		t.Fatalf("source_entities len = %d, want 2: %#v", len(sourceEntities), sourceEntities)
	}
	if !metadataBoolValue(run.Metadata, "media_source_research_required") {
		t.Fatalf("media_source_research_required not set: %#v", run.Metadata)
	}
	if !strings.Contains(run.Prompt, "Detected durable media source refs") ||
		!strings.Contains(run.Prompt, "Detected VText source entities") ||
		!strings.Contains(run.Prompt, "researcher-maintained source representations") ||
		!strings.Contains(buildVTextMediaSourceResearchObjective(refs, ""), "first call read_content_item") {
		t.Fatalf("compiled prompt missing media source contract: %q", run.Prompt)
	}
	byKind := map[string]vtextMediaSourceRef{}
	for _, ref := range refs {
		byKind[ref.Kind] = ref
	}
	if byKind["youtube"].VideoID != "dQw4w9WgXcQ" || byKind["youtube"].TranscriptAvailability != "unavailable" {
		t.Fatalf("youtube ref = %#v", byKind["youtube"])
	}
	if byKind["image"].MediaType != "image/png" || byKind["image"].ContentID == "" {
		t.Fatalf("image ref = %#v", byKind["image"])
	}
	entitiesByKind := map[string]vtextSourceEntity{}
	for _, entity := range sourceEntities {
		entitiesByKind[entity.Kind] = entity
	}
	if entitiesByKind["youtube_video"].Target.ContentID == "" ||
		entitiesByKind["youtube_video"].Display.OpenSurface != "video" ||
		entitiesByKind["youtube_video"].Evidence.TranscriptAvailability != "unavailable" {
		t.Fatalf("youtube source entity = %#v", entitiesByKind["youtube_video"])
	}
	if entitiesByKind["image"].Target.ContentID == "" ||
		entitiesByKind["image"].Display.OpenSurface != "image" ||
		entitiesByKind["image"].Evidence.State != "available" {
		t.Fatalf("image source entity = %#v", entitiesByKind["image"])
	}
	dedupedRefs, added := rt.registerVTextMediaSourceRefs(context.Background(), "user-1", content, map[string]any{
		"media_source_refs": refs,
	})
	if added || len(dedupedRefs) != 2 {
		t.Fatalf("re-register added=%v len=%d, want no new refs and len 2: %#v", added, len(dedupedRefs), dedupedRefs)
	}
	items, err := s.ListContentItems(context.Background(), "user-1", 20)
	if err != nil {
		t.Fatalf("list content items: %v", err)
	}
	if len(items) < 3 {
		t.Fatalf("content items = %d, want video, transcript status, and image refs: %#v", len(items), items)
	}
}

func TestVTextAgentRevisionPromotesResearcherContentRefsToSourceEntities(t *testing.T) {
	t.Parallel()
	h, s, rt := vtextAPISetupWithRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	item := types.ContentItem{
		ContentID:    "content-cloud-audit",
		OwnerID:      "user-1",
		SourceType:   "extracted_url",
		MediaType:    "text/html",
		AppHint:      "web",
		Title:        "Cloud auditability source",
		SourceURL:    "https://example.com/cloud-audit",
		CanonicalURL: "https://example.com/cloud-audit",
		TextContent:  "Cloud providers should preserve auditability and source-backed change records.",
		ContentHash:  "sha256-cloud-audit",
		Metadata:     json.RawMessage(`{}`),
		Provenance:   json.RawMessage(`{"source_url":"https://example.com/cloud-audit"}`),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.CreateContentItem(ctx, item); err != nil {
		t.Fatalf("CreateContentItem: %v", err)
	}

	docID, _ := createDocWithUserRevision(t, h)
	researchRun := types.RunRecord{
		RunID:     "run-content-source-research",
		OwnerID:   "user-1",
		SandboxID: "sandbox-vtext-test",
		ChannelID: docID,
		State:     types.RunCompleted,
		Metadata: map[string]any{
			runMetadataAgentRole: AgentProfileResearcher,
		},
	}
	if err := s.CreateRun(ctx, researchRun); err != nil {
		t.Fatalf("CreateRun researcher: %v", err)
	}
	message := &types.ChannelMessage{
		ChannelID:   docID,
		From:        "researcher",
		FromRunID:   researchRun.RunID,
		FromAgentID: "researcher-content-source",
		ToAgentID:   "vtext:" + docID,
		Role:        AgentProfileResearcher,
		Content: strings.Join([]string{
			"Coagent update ready.",
			"Role: researcher.",
			"Kind: findings.",
			"",
			"Findings:",
			"- The source supports this bounded claim: \"Cloud providers should preserve auditability.\" content_id:content-cloud-audit",
		}, "\n"),
		Timestamp: now,
	}
	if err := s.AppendChannelMessage(ctx, message, "user-1"); err != nil {
		t.Fatalf("AppendChannelMessage: %v", err)
	}

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		map[string]string{"intent": "integrate_worker_findings"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("revise status = %d body=%s", w.Code, w.Body.String())
	}
	var resp vtextAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	run, err := rt.GetRun(ctx, resp.RunID, "user-1")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	sourceEntities := decodeVTextSourceEntities(run.Metadata["source_entities"])
	if len(sourceEntities) != 1 {
		t.Fatalf("source_entities len = %d, want 1: %#v", len(sourceEntities), sourceEntities)
	}
	entity := sourceEntities[0]
	if entity.Kind != "content_item" ||
		entity.Target.ContentID != item.ContentID ||
		entity.Target.CanonicalURL != item.CanonicalURL ||
		entity.Display.OpenSurface != "source" ||
		entity.Evidence.ResearchState != "represented" ||
		len(entity.Selectors) != 1 ||
		entity.Selectors[0].SelectorKind != "text_quote" ||
		entity.Selectors[0].ContentHash != item.ContentHash {
		t.Fatalf("content source entity = %#v", entity)
	}
	if !strings.Contains(run.Prompt, "Detected VText source entities") ||
		!strings.Contains(run.Prompt, "content_id=content-cloud-audit") ||
		!strings.Contains(run.Prompt, "Canonical inline Source Entity syntax is [label](source:ENTITY_ID)") {
		t.Fatalf("compiled prompt missing content source entity contract: %q", run.Prompt)
	}
}

func TestMarkVTextMediaSourceRefsResearchState(t *testing.T) {
	t.Parallel()
	metadata := map[string]any{
		"media_source_research_required": true,
		"media_source_refs": []vtextMediaSourceRef{
			{Kind: "youtube", CanonicalURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", ResearchState: "pending"},
			{Kind: "image", CanonicalURL: "https://example.com/image.jpg", ResearchState: "pending"},
		},
		"source_entities": []vtextSourceEntity{
			{Kind: "youtube_video", EntityID: "src-one", Evidence: vtextSourceEntityEvidence{ResearchState: "pending"}},
			{Kind: "image", EntityID: "src-two", Evidence: vtextSourceEntityEvidence{ResearchState: "pending"}},
		},
	}
	markVTextMediaSourceRefsResearchState(metadata, "represented")
	refs := decodeVTextMediaSourceRefs(metadata["media_source_refs"])
	if len(refs) != 2 {
		t.Fatalf("refs len = %d, want 2", len(refs))
	}
	for _, ref := range refs {
		if ref.ResearchState != "represented" {
			t.Fatalf("research state = %q, want represented in %#v", ref.ResearchState, refs)
		}
	}
	if metadataBoolValue(metadata, "media_source_research_required") {
		t.Fatalf("media_source_research_required should be false after representation: %#v", metadata)
	}
	sourceEntities := decodeVTextSourceEntities(metadata["source_entities"])
	if len(sourceEntities) != 2 {
		t.Fatalf("source entities len = %d, want 2", len(sourceEntities))
	}
	for _, entity := range sourceEntities {
		if entity.Evidence.ResearchState != "represented" {
			t.Fatalf("source entity research state = %q, want represented in %#v", entity.Evidence.ResearchState, sourceEntities)
		}
	}
}

func TestMediaSourceRefToSourceEntityUsesTypedEvidenceStates(t *testing.T) {
	t.Parallel()
	candidate := mediaSourceRefToSourceEntity(vtextMediaSourceRef{
		Kind:         "image",
		CanonicalURL: "https://example.com/pending-source.png",
	})
	if candidate.Evidence.State != "candidate" {
		t.Fatalf("candidate evidence state = %q, want candidate: %#v", candidate.Evidence.State, candidate)
	}

	unavailable := mediaSourceRefToSourceEntity(vtextMediaSourceRef{
		Kind:                   "youtube",
		CanonicalURL:           "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		ContentID:              "content-youtube",
		TranscriptAvailability: "error",
	})
	if unavailable.Evidence.State != "unavailable" {
		t.Fatalf("unavailable evidence state = %q, want unavailable: %#v", unavailable.Evidence.State, unavailable)
	}

	existing := vtextSourceEntity{
		EntityID: "src-candidate",
		Kind:     "image",
		Evidence: vtextSourceEntityEvidence{State: "candidate"},
	}
	incoming := vtextSourceEntity{
		EntityID: "src-candidate",
		Kind:     "image",
		Evidence: vtextSourceEntityEvidence{State: "available"},
	}
	merged := mergeVTextSourceEntity(existing, incoming)
	if merged.Evidence.State != "available" {
		t.Fatalf("merged evidence state = %q, want available: %#v", merged.Evidence.State, merged)
	}
}

// TestVTextAgentRevisionDocumentNotFound verifies that requesting an
// agent revision for a non-existent document returns 404.
func TestVTextAgentRevisionDocumentNotFound(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)

	req := vtextRequest(t, http.MethodPost, "/api/vtext/documents/nonexistent/revise",
		map[string]string{"prompt": "test"})
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestVTextAgentRevisionWrongOwner verifies that requesting an agent
// revision for a document owned by another user returns 404.
func TestVTextAgentRevisionWrongOwner(t *testing.T) {
	t.Parallel()
	h, _, _ := vtextAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Use a different user.
	req := httptest.NewRequest(http.MethodPost, "/api/vtext/documents/"+docID+"/revise",
		bytes.NewReader([]byte(`{"prompt":"test"}`)))
	req.Header.Set("X-Authenticated-User", "user-2")
	w := httptest.NewRecorder()
	h.HandleVTextAgentRevision(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (wrong owner)", w.Code, http.StatusNotFound)
	}
}
