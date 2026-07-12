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
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/markdownstructure"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func textureAPISetup(t *testing.T) (*APIHandler, *store.Store) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "go-choir-m3-texture-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open texture api test store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_ = os.RemoveAll(promptRoot)
	})

	cfg := Config{
		SandboxID:           "sandbox-texture-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     2 * time.Second,
		SupervisionInterval: 5 * time.Second,
	}

	bus := events.NewEventBus()
	provider := NewStubProvider(2 * time.Second)
	rt := New(cfg, s, bus, provider)
	setTestDispatch(rt, s)

	return NewAPIHandler(rt), s
}

func textureReplaceAllResult(content string, baseRevisionIDs ...string) string {
	env := map[string]any{
		"kind":      "texture_edit",
		"operation": "replace_all",
		"content":   content,
	}
	if len(baseRevisionIDs) > 0 && strings.TrimSpace(baseRevisionIDs[0]) != "" {
		env["base_revision_id"] = strings.TrimSpace(baseRevisionIDs[0])
	}
	data, _ := json.Marshal(env)
	return string(data)
}

func textureStructuredApplyEditsResult(edits []textureStructuredEdit, baseRevisionIDs ...string) string {
	env := map[string]any{
		"kind":      "texture_edit",
		"operation": "apply_edits",
		"edits":     edits,
	}
	if len(baseRevisionIDs) > 0 && strings.TrimSpace(baseRevisionIDs[0]) != "" {
		env["base_revision_id"] = strings.TrimSpace(baseRevisionIDs[0])
	}
	data, _ := json.Marshal(env)
	return string(data)
}

func TestHandleInternalTextureProposalDeliveryRecordsAuthorInbox(t *testing.T) {
	t.Parallel()
	h, s := textureAPISetup(t)
	req := httptest.NewRequest(http.MethodPost, "/internal/texture/proposals", strings.NewReader(`{
		"owner_id":"author-1",
		"proposal_id":"readerprop-1",
		"publication_id":"pub-1",
		"publication_version_id":"pubver-1",
		"submitter_id":"reader-1",
		"delivery_id":"delivery-1"
	}`))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	h.HandleInternalTextureProposalDelivery(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusAccepted {
		t.Fatalf("delivery status: got %d body %s", w.Code, w.Body.String())
	}
	var resp internalTextureProposalDeliveryResponse
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

type textureEditToolProvider struct {
	provideriface.Provider
	result     string
	resultFunc func(prompt string) string
	delay      time.Duration
	choices    []string
	firstTools []provideriface.ToolDefinition
}

func newTextureEditToolProvider(result string) *textureEditToolProvider {
	return &textureEditToolProvider{
		Provider: NewStubProvider(1 * time.Millisecond),
		result:   result,
	}
}

func (p *textureEditToolProvider) ProviderName() string {
	return "texture-edit-tool"
}

func (p *textureEditToolProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
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
	emit(types.EventRunDelta, "execution", json.RawMessage(`{"text":"texture edit tool provider","provider":"texture-edit-tool"}`))
	return nil
}

func (p *textureEditToolProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	if p.firstTools == nil && toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		p.firstTools = append([]provideriface.ToolDefinition(nil), req.ToolDefinitions...)
	}
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
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "conductor handoff complete", Model: "test-model"}, nil
	}
	lastUser := extractLastUserMessage(req.Messages)
	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnTextureToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if messagesContainToolCall(req.Messages, "patch_texture") || messagesContainToolCall(req.Messages, "rewrite_texture") {
		if !strings.Contains(req.ToolChoice, "patch_texture") {
			if !messagesContainCoagentFollowUpDelivery(req.Messages) ||
				messagesToolCallCount(req.Messages, "patch_texture")+messagesToolCallCount(req.Messages, "rewrite_texture") >= 2 {
				return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "texture turn complete", Model: "test-model"}, nil
			}
		}
	}
	if (lastUser == "" && !strings.Contains(req.ToolChoice, "patch_texture")) || !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "texture turn complete", Model: "test-model"}, nil
	}
	prompt := toolLoopPromptContext(req)
	result := p.result
	if p.resultFunc != nil {
		result = p.resultFunc(prompt)
	}
	call, err := editTextureToolCallFromLegacyResult(prompt, result)
	if err != nil {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: result, Model: "test-model"}, nil
	}
	if strings.Contains(req.ToolChoice, "patch_texture") {
		call, err = requiredPatchTextureToolCallFromLegacyResult(prompt, result)
		if err != nil {
			return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: result, Model: "test-model"}, nil
		}
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls:  []types.ToolCall{call},
		Model:      "test-model",
	}, nil
}

type textureParkResidentProvider struct {
	provideriface.Provider
	choices []string
}

func (p *textureParkResidentProvider) ProviderName() string {
	return "texture-park-resident-provider"
}

func (p *textureParkResidentProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	task.Result = textureReplaceAllResult("provider execute fallback")
	emit(types.EventRunDelta, "execution", json.RawMessage(`{"text":"texture park resident provider","provider":"texture-park-resident-provider"}`))
	return nil
}

func (p *textureParkResidentProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	if !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "no texture tools", Model: "test-model"}, nil
	}
	prompt := req.System + "\n" + extractLastUserMessage(req.Messages)
	var result string
	switch {
	case !messagesContainToolCall(req.Messages, "patch_texture"):
		result = textureReplaceAllResult("Model-prior resident draft before worker evidence.")
	case messagesContainText(req.Messages, "A new grounded finding arrived") &&
		!messagesContainText(req.Messages, "Grounded update from parked resident actor."):
		result = textureReplaceAllResult("Grounded update from parked resident actor.")
	default:
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "resident actor idle", Model: "test-model"}, nil
	}
	call, err := editTextureToolCallFromLegacyResult(prompt, result)
	if err != nil {
		return nil, err
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls:  []types.ToolCall{call},
		Model:      "test-model",
	}, nil
}

type textureDecisionThenEditProvider struct {
	provideriface.Provider
	choices    []string
	firstTools []provideriface.ToolDefinition
	calls      int
}

func (p *textureDecisionThenEditProvider) ProviderName() string {
	return "texture-decision-then-edit"
}

func (p *textureDecisionThenEditProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	return NewStubProvider(1*time.Millisecond).Execute(ctx, task, emit)
}

func (p *textureDecisionThenEditProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	if p.firstTools == nil && toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		p.firstTools = append([]provideriface.ToolDefinition(nil), req.ToolDefinitions...)
	}
	p.calls++
	switch p.calls {
	case 1:
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-decision",
				Name: "record_texture_decision",
				Arguments: json.RawMessage(`{
					"decision_kind":"no_worker_needed",
					"reason":"M3.2 staging proof: user supplied the needed content and requested no research or execution worker.",
					"evidence_refs":["staging-marker:M32_TEXTURE_DECISION_ROUTE_TEST"],
					"next_action":"Write the concise reader-facing Texture revision."
				}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		}, nil
	case 2:
		// Author a clean reader-facing revision that replaces the owner prompt V0,
		// rather than appending to it. The off-document decision rationale lives in
		// the prompt (now canonical V0) and must not be carried into the authored
		// document body, so the model rewrites the whole document with clean prose.
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-reader-edit",
				Name: "rewrite_texture",
				Arguments: json.RawMessage(`{
					"content":"M32_TEXTURE_DECISION_ROUTE_TEST\n\nThis marker is a deployed acceptance probe.",
					"rationale":"Author the clean reader-facing Texture revision from the owner prompt."
				}`),
			}},
			Usage: provideriface.TokenUsage{InputTokens: 1, OutputTokens: 1},
			Model: "test-model",
		}, nil
	default:
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
	}
}

func conductorSpawnTextureToolCall(prompt string) types.ToolCall {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "Create a working document."
	}
	title := buildInitialTextureTitle(prompt, "")
	args, _ := json.Marshal(map[string]any{
		"objective":       prompt,
		"role":            AgentProfileTexture,
		"initial_content": "# " + title + "\n\n" + prompt,
	})
	return types.ToolCall{ID: "spawn-texture-test-call", Name: "spawn_agent", Arguments: args}
}

type finalTextProvider struct {
	result string
}

func (p *finalTextProvider) ProviderName() string {
	return "final-text-provider"
}

func (p *finalTextProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
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

func messagesContainText(messages []json.RawMessage, text string) bool {
	for _, raw := range messages {
		if strings.Contains(string(raw), text) {
			return true
		}
		var msg struct {
			Content any `json:"content"`
		}
		if err := json.Unmarshal(raw, &msg); err == nil && strings.Contains(extractTextFromContent(msg.Content), text) {
			return true
		}
	}
	return false
}

func messagesContainCoagentFollowUpDelivery(messages []json.RawMessage) bool {
	return messagesContainText(messages, `"delivery_phase":"`+coagentPacketDeliveryMid+`"`) ||
		messagesContainText(messages, `"delivery_phase":"`+coagentPacketDeliveryFinal+`"`)
}

func messagesToolCallCount(messages []json.RawMessage, name string) int {
	count := 0
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
				count++
			}
		}
	}
	return count
}

func toolDefinitionsContain(defs []provideriface.ToolDefinition, name string) bool {
	for _, def := range defs {
		if def.Name == name {
			return true
		}
	}
	return false
}

func assertInitialTextureAutonomousSurface(t *testing.T, defs []provideriface.ToolDefinition) {
	t.Helper()
	if len(defs) <= 1 {
		t.Fatalf("initial Texture tool definitions = %+v, want full Texture affordance surface", defs)
	}
	for _, name := range []string{"patch_texture", "record_texture_decision", "spawn_agent", "request_super_execution"} {
		if !toolDefinitionsContain(defs, name) {
			t.Fatalf("initial Texture tool definitions = %+v, missing %s", defs, name)
		}
	}
}

func editTextureToolCallFromLegacyResult(prompt, raw string) (types.ToolCall, error) {
	var env struct {
		Kind           string                  `json:"kind"`
		BaseRevisionID string                  `json:"base_revision_id,omitempty"`
		Operation      string                  `json:"operation"`
		Content        string                  `json:"content,omitempty"`
		Edits          []textureStructuredEdit `json:"edits,omitempty"`
	}
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return types.ToolCall{}, err
	}
	if strings.TrimSpace(env.Kind) != "texture_edit" {
		return types.ToolCall{}, errors.New("not a texture edit result")
	}
	docID := extractPromptValue(prompt, `"doc_id":"`, `"`)
	if docID == "" {
		docID = extractPromptValue(prompt, "Current coordination channel: ", ".")
	}
	baseRevisionID := strings.TrimSpace(env.BaseRevisionID)
	if baseRevisionID == "" {
		baseRevisionID = extractPromptValue(prompt, "Current head revision: ", " ")
	}
	args := editTextureArgs{
		DocID:          docID,
		BaseRevisionID: baseRevisionID,
	}
	toolName := "patch_texture"
	switch strings.TrimSpace(env.Operation) {
	case "apply_edits":
		args.StructuredEdits = env.Edits
	default:
		toolName = "rewrite_texture"
		args.Operation = "replace_all"
		args.Content = env.Content
		args.Rationale = "test whole-document replacement"
	}
	data, err := json.Marshal(args)
	if err != nil {
		return types.ToolCall{}, err
	}
	return types.ToolCall{ID: "edit-texture-test-call", Name: toolName, Arguments: data}, nil
}

func requiredPatchTextureToolCallFromLegacyResult(prompt, raw string) (types.ToolCall, error) {
	call, err := editTextureToolCallFromLegacyResult(prompt, raw)
	if err != nil {
		return types.ToolCall{}, err
	}
	if call.Name == "patch_texture" {
		var args editTextureArgs
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return types.ToolCall{}, err
		}
		args.BaseRevisionID = ""
		data, err := json.Marshal(args)
		if err != nil {
			return types.ToolCall{}, err
		}
		call.Arguments = data
		return call, nil
	}
	var args editTextureArgs
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return types.ToolCall{}, err
	}
	content := strings.TrimSpace(args.Content)
	if content == "" {
		content = "Texture revision updated."
	}
	args.Operation = ""
	args.BaseRevisionID = ""
	args.Content = ""
	args.Rationale = ""
	args.StructuredEdits = []textureStructuredEdit{{
		Op:        "append_block",
		BlockType: "paragraph",
		Text:      content,
	}}
	data, err := json.Marshal(args)
	if err != nil {
		return types.ToolCall{}, err
	}
	call.Name = "patch_texture"
	call.Arguments = data
	return call, nil
}

func toolLoopPromptContext(req provideriface.ToolLoopRequest) string {
	var b strings.Builder
	b.WriteString(req.System)
	for _, raw := range req.Messages {
		var msg struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		}
		if err := json.Unmarshal(raw, &msg); err == nil {
			if text := strings.TrimSpace(extractTextFromContent(msg.Content)); text != "" {
				b.WriteString("\n")
				if msg.Role != "" {
					b.WriteString(msg.Role)
					b.WriteString(": ")
				}
				b.WriteString(text)
			}
		}
		b.WriteString("\n")
		b.Write(raw)
	}
	return b.String()
}

func extractCurrentCanonicalContentFromPrompt(s string) string {
	const marker = "Current canonical document content:\n---\n"
	start := strings.LastIndex(s, marker)
	if start < 0 {
		return ""
	}
	rest := s[start+len(marker):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
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

func TestTextureAPICreateDocument(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "My Document"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp textureCreateDocResponse
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

func TestTextureAPICreateDocumentAuth(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// No auth header.
	req := httptest.NewRequest(http.MethodPost, "/api/texture/documents",
		bytes.NewReader([]byte(`{"title":"test"}`)))
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestTextureCancelAgentRevisionCancelsTrajectoryAndLeavesMutationResumable(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
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
		SandboxID:    "sandbox-texture-test",
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
		RunID:            "run-cancel-child",
		AgentID:          "agent-vsuper-cancel",
		RequestedByRunID: "spawned-by-other-run",
		AgentProfile:     AgentProfileVSuper,
		AgentRole:        AgentProfileVSuper,
		OwnerID:          "user-1",
		SandboxID:        "sandbox-texture-test",
		State:            types.RunRunning,
		Prompt:           "Background candidate.",
		TrajectoryID:     trajectoryID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	graphChildDifferentTrajectory := types.RunRecord{
		RunID:            "run-cancel-graph-child-other-trajectory",
		AgentID:          "agent-other-trajectory",
		RequestedByRunID: parent.RunID,
		AgentProfile:     AgentProfileVSuper,
		AgentRole:        AgentProfileVSuper,
		OwnerID:          "user-1",
		SandboxID:        "sandbox-texture-test",
		State:            types.RunRunning,
		Prompt:           "Different trajectory despite spawned-by provenance.",
		TrajectoryID:     "traj-other-cancel-agent",
		CreatedAt:        now,
		UpdatedAt:        now,
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
		Objective:            "finish the pending Texture revision",
		ObjectiveFingerprint: "fp-cancel-texture-revision",
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

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+doc.DocID+"/cancel", nil)
	w := httptest.NewRecorder()
	h.HandleTextureCancelAgentRevision(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp textureCancelRevisionResponse
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

func TestTextureDeleteDocumentCancelsPendingActorTrajectory(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	trajectoryID := "traj-delete-cancels-texture"
	doc := types.Document{
		DocID:     "doc-delete-cancels-texture",
		OwnerID:   "user-1",
		Title:     "Delete Cancels Texture",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	textureRun := types.RunRecord{
		RunID:        "run-delete-cancels-texture",
		AgentID:      currentTextureAgentID(doc.DocID),
		ChannelID:    doc.DocID,
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		Prompt:       "Parked Texture revision actor.",
		TrajectoryID: trajectoryID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataTrajectoryID: trajectoryID,
		},
	}
	if err := s.CreateRun(ctx, textureRun); err != nil {
		t.Fatalf("create texture run: %v", err)
	}
	item, err := s.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              "user-1",
		TrajectoryID:         trajectoryID,
		Objective:            "finish Texture revision before delete",
		ObjectiveFingerprint: "fp-delete-cancels-texture",
		CreatedByRunID:       textureRun.RunID,
	})
	if err != nil {
		t.Fatalf("create work item: %v", err)
	}
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:     doc.DocID,
		RunID:     textureRun.RunID,
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create pending mutation: %v", err)
	}

	req := textureRequest(t, http.MethodDelete, "/api/texture/documents/"+doc.DocID, nil)
	w := httptest.NewRecorder()
	h.HandleTextureDocument(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if _, err := s.GetDocument(ctx, doc.DocID, "user-1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetDocument after delete err = %v, want ErrNotFound", err)
	}
	mutation, err := s.GetAgentMutationByRun(ctx, textureRun.RunID)
	if err != nil {
		t.Fatalf("get mutation: %v", err)
	}
	if mutation.State != "cancelled" {
		t.Fatalf("mutation state = %q, want cancelled", mutation.State)
	}
	cancelledRun, err := s.GetRun(ctx, textureRun.RunID)
	if err != nil {
		t.Fatalf("get cancelled run: %v", err)
	}
	if cancelledRun.State != types.RunCancelled {
		t.Fatalf("run state = %s, want cancelled", cancelledRun.State)
	}
	cancelledItem, err := s.GetWorkItem(ctx, "user-1", item.WorkItemID)
	if err != nil {
		t.Fatalf("get cancelled work item: %v", err)
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
}

// ----- Document list -----

func TestTextureAPIListDocuments(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create 2 documents.
	for _, title := range []string{"Doc A", "Doc B"} {
		req := textureRequest(t, http.MethodPost, "/api/texture/documents",
			map[string]string{"title": title})
		w := httptest.NewRecorder()
		h.HandleTextureCreateDocument(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create document: status = %d", w.Code)
		}
	}

	// List documents.
	req := textureRequest(t, http.MethodGet, "/api/texture/documents", nil)
	w := httptest.NewRecorder()
	h.HandleTextureListDocuments(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp textureListDocsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Documents) != 2 {
		t.Errorf("len(documents) = %d, want 2", len(resp.Documents))
	}
}

// ----- Document get -----

func TestTextureAPIGetDocument(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create a document.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var createResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&createResp)

	// Get the document.
	req = textureRequest(t, http.MethodGet, "/api/texture/documents/"+createResp.DocID, nil)
	w = httptest.NewRecorder()
	h.HandleTextureDocument(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp textureDocumentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.DocID != createResp.DocID {
		t.Errorf("DocID = %q, want %q", resp.DocID, createResp.DocID)
	}
}

// ----- Revision creation (user edit) -----

func TestTextureAPICreateRevisionUserEdit(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create a document.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create a user-authored revision. Public revision POSTs ignore
	// browser-supplied author labels and use the authenticated owner.
	revReq := textureCreateRevisionRequest{
		Content:     "Hello, world!",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var revResp textureRevisionResponse
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

func TestTextureAPICreateRevisionCanonicalizesAliasedImportedDocumentTitle(t *testing.T) {
	t.Parallel()
	h, s := textureAPISetup(t)
	ctx := context.Background()

	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "legacy-import.md"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create document status = %d, body: %s", w.Code, w.Body.String())
	}
	var docResp textureCreateDocResponse
	if err := json.NewDecoder(w.Body).Decode(&docResp); err != nil {
		t.Fatalf("decode document response: %v", err)
	}
	if err := s.UpsertDocumentAlias(ctx, "user-1", "imports/legacy-import.md", docResp.DocID, time.Now().UTC()); err != nil {
		t.Fatalf("UpsertDocumentAlias: %v", err)
	}

	revReq := textureCreateRevisionRequest{Content: "Imported projection first durable edit"}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create revision status = %d, body: %s", w.Code, w.Body.String())
	}

	doc, err := s.GetDocument(ctx, docResp.DocID, "user-1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if doc.Title != "legacy-import.texture" {
		t.Fatalf("document title = %q, want legacy-import.texture", doc.Title)
	}
	docID, err := s.GetDocumentAlias(ctx, "user-1", "imports/legacy-import.md")
	if err != nil {
		t.Fatalf("GetDocumentAlias original source: %v", err)
	}
	if docID != docResp.DocID {
		t.Fatalf("original source alias doc_id = %q, want %q", docID, docResp.DocID)
	}
}

func TestTextureAPIListRevisionsReturnsDurableVersionNumbersPastFifty(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Many Versions"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create document status = %d, body: %s", w.Code, w.Body.String())
	}
	var docResp textureCreateDocResponse
	if err := json.NewDecoder(w.Body).Decode(&docResp); err != nil {
		t.Fatalf("decode document response: %v", err)
	}

	parentID := ""
	var latest textureRevisionResponse
	for i := 0; i < 55; i++ {
		revReq := textureCreateRevisionRequest{
			Content:          fmt.Sprintf("Document body v%d", i),
			ParentRevisionID: parentID,
		}
		req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
		w = httptest.NewRecorder()
		h.HandleTextureRevisions(w, req)
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

	req = textureRequest(t, http.MethodGet, "/api/texture/documents/"+docResp.DocID+"/revisions?limit=10000", nil)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list revisions status = %d, body: %s", w.Code, w.Body.String())
	}
	var listResp textureListRevisionsResponse
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

	req = textureRequest(t, http.MethodGet, "/api/texture/documents/"+docResp.DocID, nil)
	w = httptest.NewRecorder()
	h.HandleTextureDocument(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get document status = %d, body: %s", w.Code, w.Body.String())
	}
	var getDocResp textureDocumentResponse
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

func TestTextureAPICreateRevisionIgnoresAppAgentAuthorFields(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create a document.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create a user revision first.
	revReq := textureCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	// Attempt to create an appagent revision through the public revision
	// endpoint. This must still be stored as a user-authored edit.
	revReq = textureCreateRevisionRequest{
		Content:     "AI-improved draft",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var revResp textureRevisionResponse
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

func TestTextureAPIIgnoresInvalidAuthorKind(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create a document.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Try to create a revision with "worker" author kind. Public callers
	// cannot select canonical authorship, so the request is accepted as a
	// normal user edit instead of exposing an author-kind validator.
	revReq := textureCreateRevisionRequest{
		Content:     "Worker content",
		AuthorKind:  "worker",
		AuthorLabel: "worker-1",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var revResp textureRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if revResp.AuthorKind != types.AuthorUser || revResp.AuthorLabel != "user-1" {
		t.Errorf("public revision author = %q/%q, want %q/user-1", revResp.AuthorKind, revResp.AuthorLabel, types.AuthorUser)
	}
}

// ----- History -----

func TestTextureAPIGetHistory(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create a document.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create revisions.
	revReq := textureCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	revReq = textureCreateRevisionRequest{
		Content:     "AI-improved",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	// Get history.
	req = textureRequest(t, http.MethodGet, "/api/texture/documents/"+docResp.DocID+"/history", nil)
	w = httptest.NewRecorder()
	h.HandleTextureHistory(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp textureHistoryResponse
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

func TestTextureAPIGetDiff(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create a document and revisions.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	revReq := textureCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	var rev1Resp textureRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&rev1Resp)

	revReq = textureCreateRevisionRequest{
		Content:     "AI-improved draft",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	var rev2Resp textureRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&rev2Resp)

	// Get diff.
	req = textureRequest(t, http.MethodGet,
		"/api/texture/diff?from="+rev1Resp.RevisionID+"&to="+rev2Resp.RevisionID, nil)
	w = httptest.NewRecorder()
	h.HandleTextureDiff(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp textureDiffResponse
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

func TestTextureAPIGetBlame(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create a document and revisions.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	revReq := textureCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	revReq = textureCreateRevisionRequest{
		Content:     "AI-improved draft",
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	var rev2Resp textureRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&rev2Resp)

	// Get blame.
	req = textureRequest(t, http.MethodGet,
		"/api/texture/revisions/"+rev2Resp.RevisionID+"/blame", nil)
	w = httptest.NewRecorder()
	h.HandleTextureBlame(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp textureBlameResponse
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

func TestTextureAPISnapshotDoesNotMutateHead(t *testing.T) {
	t.Parallel()
	h, s := textureAPISetup(t)

	// Create a document.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create two revisions.
	revReq := textureCreateRevisionRequest{
		Content:     "First draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	var rev1Resp textureRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&rev1Resp)

	revReq = textureCreateRevisionRequest{
		Content:     "Second draft",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	// View the first (historical) revision.
	req = textureRequest(t, http.MethodGet,
		"/api/texture/revisions/"+rev1Resp.RevisionID, nil)
	w = httptest.NewRecorder()
	h.HandleTextureRevision(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var snapshotResp textureRevisionResponse
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

// ----- Auth gating on texture endpoints -----

func TestTextureAPIAuthGating(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/texture/documents"},
		{http.MethodPost, "/api/texture/documents"},
		{http.MethodGet, "/api/texture/diff"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewReader(nil))
		w := httptest.NewRecorder()

		switch {
		case strings.HasPrefix(ep.path, "/api/texture/documents"):
			h.HandleTextureDocumentsRoot(w, req)
		case strings.HasPrefix(ep.path, "/api/texture/diff"):
			h.HandleTextureDiff(w, req)
		}

		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want %d", ep.method, ep.path, w.Code, http.StatusUnauthorized)
		}
	}
}

// ----- Citations and metadata -----

func TestTextureAPICitationsMetadataRoundTrip(t *testing.T) {
	t.Parallel()
	h, _ := textureAPISetup(t)

	// Create a document.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create a revision with citations and metadata.
	citations := []types.Citation{
		{ID: "c1", Type: "url", Value: "https://example.com", Label: "Example"},
	}
	citJSON, _ := json.Marshal(citations)
	metaJSON, _ := json.Marshal(map[string]any{"tags": []string{"draft"}})

	revReq := textureCreateRevisionRequest{
		Content:     "Document with citations",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
		Citations:   citJSON,
		Metadata:    metaJSON,
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)

	var revResp textureRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Get the revision back and check citations/metadata.
	req = textureRequest(t, http.MethodGet,
		"/api/texture/revisions/"+revResp.RevisionID, nil)
	w = httptest.NewRecorder()
	h.HandleTextureRevision(w, req)

	var getResp textureRevisionResponse
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

func textureAPISetupWithProvider(t *testing.T, provider provideriface.Provider, installTools bool) (*APIHandler, *store.Store, *Runtime) {
	return textureAPISetupWithProviderAndOptions(t, provider, installTools)
}

func textureAPISetupWithProviderAndOptions(t *testing.T, provider provideriface.Provider, installTools bool, opts ...RuntimeOption) (*APIHandler, *store.Store, *Runtime) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "go-choir-m3-texture-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open texture api test store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_ = os.RemoveAll(promptRoot)
	})

	cfg := Config{
		SandboxID:           "sandbox-texture-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: 5 * time.Second,
		TextureWakeDebounce: 250 * time.Millisecond,
	}

	bus := events.NewEventBus()
	rt := New(cfg, s, bus, provider, opts...)
	setTestDispatch(rt, s)
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

// textureAPISetupWithRuntime creates a test setup with a started runtime
// so that runs actually execute and complete.
func textureAPISetupWithRuntime(t *testing.T) (*APIHandler, *store.Store, *Runtime) {
	t.Helper()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Stubbed texture document revision."))
	provider.delay = 50 * time.Millisecond
	return textureAPISetupWithProvider(t, provider, true)
}

type fakeTextureWakeClock struct {
	mu     sync.Mutex
	timers []*fakeTextureWakeTimer
}

type fakeTextureWakeTimer struct {
	mu     sync.Mutex
	active bool
	fn     func()
}

func (c *fakeTextureWakeClock) afterFunc(_ time.Duration, fn func()) textureWakeTimer {
	timer := &fakeTextureWakeTimer{active: true, fn: fn}
	c.mu.Lock()
	c.timers = append(c.timers, timer)
	c.mu.Unlock()
	return timer
}

func (c *fakeTextureWakeClock) fireAll() {
	c.mu.Lock()
	timers := append([]*fakeTextureWakeTimer(nil), c.timers...)
	c.mu.Unlock()
	for _, timer := range timers {
		timer.fire()
	}
}

func (t *fakeTextureWakeTimer) Stop() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	wasActive := t.active
	t.active = false
	return wasActive
}

func (t *fakeTextureWakeTimer) fire() {
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

// TestScheduleTextureWorkerWakeLeadingCoalesce was deleted: the old timer-based
// debounce system (textureWakeKey, textureWakeMu, textureWakePending) was removed
// when scheduleTextureWorkerWake was rewritten to send an actor message via
// dispatchActor. The actor mailbox + park-resume semantics handle coalescing
// naturally; the leading+max-interval timer behavior no longer exists.

// TestCoagentUpdateTurnInjectorSupportsTexture asserts Texture uses the same
// warm-injection path as other durable actors so one logical Texture activation
// can deepen across multiple addressed findings packets.
func TestCoagentUpdateTurnInjectorSupportsTexture(t *testing.T) {
	provider := newTextureEditToolProvider(textureReplaceAllResult("noop"))
	_, _, rt := textureAPISetupWithProviderAndOptions(t, provider, true)

	textureRec := &types.RunRecord{
		RunID:        "run-texture",
		OwnerID:      "user-1",
		AgentID:      currentTextureAgentID("doc-x"),
		AgentProfile: AgentProfileTexture,
	}
	if rt.coagentUpdateTurnInjector(textureRec) == nil {
		t.Fatalf("Texture runs must warm-inject coagent packets")
	}

	superRec := &types.RunRecord{
		RunID:        "run-super",
		OwnerID:      "user-1",
		AgentID:      "super:user-1",
		AgentProfile: AgentProfileSuper,
	}
	if rt.coagentUpdateTurnInjector(superRec) == nil {
		t.Fatalf("super runs must keep warm injection")
	}

	researcherRec := &types.RunRecord{
		RunID:        "run-researcher",
		OwnerID:      "user-1",
		AgentID:      "researcher:abc",
		AgentProfile: AgentProfileResearcher,
	}
	if rt.coagentUpdateTurnInjector(researcherRec) == nil {
		t.Fatalf("researcher runs must keep warm injection")
	}
}

type revisionPromptEchoProvider struct {
	delay time.Duration
}

func (p *revisionPromptEchoProvider) ProviderName() string {
	return "revision-prompt-echo"
}

func (p *revisionPromptEchoProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
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
		task.Result = textureReplaceAllResult("Integrated latest user direction: Fresh user edit should survive.")
	} else {
		task.Result = textureReplaceAllResult("Stale output from the older document head.")
	}
	return nil
}

func (p *revisionPromptEchoProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	provider := &textureEditToolProvider{
		Provider: NewStubProvider(1 * time.Millisecond),
		delay:    p.delay,
		resultFunc: func(prompt string) string {
			if strings.Contains(prompt, "Fresh user edit should survive") {
				return textureReplaceAllResult("Integrated latest user direction: Fresh user edit should survive.")
			}
			return textureReplaceAllResult("Stale output from the older document head.")
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

func (p *stochasticWorkflowProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
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
	case AgentProfileTexture:
		task.Result = buildStochasticTextureResult(task.Prompt)
	default:
		task.Result = "stochastic workflow loop completed"
	}
	emit(types.EventRunDelta, "execution", json.RawMessage(`{"text":"stochastic workflow loop completed","provider":"stochastic-workflow"}`))
	return nil
}

func (p *stochasticWorkflowProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	if messagesContainToolCall(req.Messages, "spawn_agent") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow conductor handoff complete", Model: "test-model"}, nil
	}
	lastUser := extractLastUserMessage(req.Messages)
	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnTextureToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if messagesContainToolCall(req.Messages, "patch_texture") || messagesContainToolCall(req.Messages, "rewrite_texture") {
		if strings.Contains(req.ToolChoice, "patch_texture") {
			goto producePatch
		}
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow loop completed", Model: "test-model"}, nil
	}
	if (lastUser == "" && !strings.Contains(req.ToolChoice, "patch_texture")) || !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow loop completed", Model: "test-model"}, nil
	}
producePatch:
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
	prompt := toolLoopPromptContext(req)
	result := buildStochasticTextureResult(prompt)
	call, err := editTextureToolCallFromLegacyResult(prompt, result)
	if err != nil {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow loop completed", Model: "test-model"}, nil
	}
	if strings.Contains(req.ToolChoice, "patch_texture") {
		call, err = requiredPatchTextureToolCallFromLegacyResult(prompt, result)
		if err != nil {
			return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "stochastic workflow loop completed", Model: "test-model"}, nil
		}
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls:  []types.ToolCall{call},
		Model:      "test-model",
	}, nil
}

func buildStochasticTextureResult(prompt string) string {
	if strings.Contains(prompt, "CANCEL_RUN_MARKER") {
		return textureReplaceAllResult("CANCELLED SHOULD NOT MATERIALIZE")
	}
	var b strings.Builder
	b.WriteString("Stochastic texture revision.")
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
	return textureReplaceAllResult(b.String())
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
	req := textureRequest(t, http.MethodPost, "/api/texture/documents",
		map[string]string{"title": "Test Doc"})
	w := httptest.NewRecorder()
	h.HandleTextureCreateDocument(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create document: status = %d, body: %s", w.Code, w.Body.String())
	}
	var docResp textureCreateDocResponse
	_ = json.NewDecoder(w.Body).Decode(&docResp)

	// Create a user-authored revision.
	revReq := textureCreateRevisionRequest{
		Content:     "Hello, world!",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docResp.DocID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user revision: status = %d, body: %s", w.Code, w.Body.String())
	}
	var revResp textureRevisionResponse
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

func waitForTextureQuiescent(t *testing.T, rt *Runtime, s *store.Store, ownerID, docID string, minCheckpointSeq uint64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pending, err := s.GetPendingAgentMutationByDoc(context.Background(), docID, ownerID)
		if err != nil {
			t.Fatalf("get pending mutation: %v", err)
		}
		_, activeErr := s.GetLatestActiveRunByAgent(context.Background(), ownerID, "texture:"+docID)
		activeClear := errors.Is(activeErr, store.ErrNotFound)
		if activeErr != nil && !activeClear {
			t.Fatalf("get active texture run: %v", activeErr)
		}
		checkpointReady := minCheckpointSeq == 0
		if minCheckpointSeq > 0 {
			checkpoint, err := s.GetTextureControllerCheckpoint(context.Background(), docID, ownerID)
			if err != nil {
				t.Fatalf("get texture controller checkpoint: %v", err)
			}
			checkpointReady = checkpoint != nil && checkpoint.IntegratedMessageSeq >= int64(minCheckpointSeq)
		}
		if pending == nil && activeClear && checkpointReady {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	checkpoint, _ := s.GetTextureControllerCheckpoint(context.Background(), docID, ownerID)
	pending, _ := s.GetPendingAgentMutationByDoc(context.Background(), docID, ownerID)
	t.Fatalf("texture doc %s did not become quiescent within %v; pending=%+v checkpoint=%+v", docID, timeout, pending, checkpoint)
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
		if !isTextureWriteToolName(metadataString(meta, "source")) || metadataString(meta, "texture_edit_kind") != "texture_edit" {
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
	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content:          content,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "user",
		ParentRevisionID: doc.CurrentRevisionID,
	})
	w := httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user revision %q: status = %d, want %d; body: %s", content, w.Code, http.StatusCreated, w.Body.String())
	}
	var resp textureRevisionResponse
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

func revisionContentsContain(revs []types.Revision, text string) bool {
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, text) {
			return true
		}
	}
	return false
}

func waitForNoPendingWorkerUpdates(t *testing.T, s *store.Store, ownerID, agentID string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last []types.CoagentSourcePacket
	for time.Now().Before(deadline) {
		pending, err := s.ListPendingWorkerUpdates(context.Background(), ownerID, agentID, 10)
		if err != nil {
			t.Fatalf("list pending worker updates: %v", err)
		}
		if len(pending) == 0 {
			return
		}
		last = pending
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("pending updates for %s after %s = %+v, want none", agentID, timeout, last)
}

// TestTextureAgentRevisionCreatesCanonicalRevision verifies that submitting
// an agent revision prompt creates a canonical appagent-authored revision
// (VAL-ETEXT-003).
func TestTextureAgentRevisionCreatesCanonicalRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Submit an agent revision request.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it more formal"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp textureAgentRevisionResponse
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

func TestTextureSystemPromptSharesChoirCoreContext(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)

	rec := &types.RunRecord{
		RunID:        "run-texture-shared-prompt",
		AgentID:      "texture:doc-1",
		ChannelID:    "doc-1",
		OwnerID:      "user-alice",
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Prompt:       "What's the latest with AI?",
	}

	prompt, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatalf("systemPromptForRun: %v", err)
	}
	if !strings.Contains(prompt, "Choir is a multiagent writing, research, and execution system") {
		t.Fatalf("system prompt missing shared Choir context: %q", prompt)
	}
	if !strings.Contains(prompt, "Current UTC date/time:") || !strings.Contains(prompt, "Treat relative-date requests") {
		t.Fatalf("system prompt missing temporal grounding context: %q", prompt)
	}
	if !strings.Contains(prompt, "canonical writer of a versioned artifact, not a one-shot answerer") {
		t.Fatalf("system prompt missing Texture wake semantics: %q", prompt)
	}
	if !strings.Contains(prompt, "Current coordination channel: doc-1.") {
		t.Fatalf("system prompt missing coordination channel: %q", prompt)
	}
}

type textureMinimalEditProvider struct {
	provideriface.Provider
	choices    []string
	firstTools []provideriface.ToolDefinition
}

func (p *textureMinimalEditProvider) ProviderName() string {
	return "texture-minimal-edit"
}

func (p *textureMinimalEditProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	return NewStubProvider(1*time.Millisecond).Execute(ctx, task, emit)
}

func (p *textureMinimalEditProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	if p.firstTools == nil && toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		p.firstTools = append([]provideriface.ToolDefinition(nil), req.ToolDefinitions...)
	}
	if messagesContainToolCall(req.Messages, "spawn_agent") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "conductor handoff complete", Model: "test-model"}, nil
	}
	lastUser := extractLastUserMessage(req.Messages)
	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnTextureToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "texture turn complete", Model: "test-model"}, nil
	}
	// The canonical write is no longer terminal, so after writing once the model
	// intentionally ends the run rather than writing again.
	if messagesContainToolCall(req.Messages, "patch_texture") || messagesContainToolCall(req.Messages, "rewrite_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "texture turn complete", Model: "test-model"}, nil
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{{
			ID:        "call-minimal-edit",
			Name:      "patch_texture",
			Arguments: json.RawMessage(`{"edits":[{"op":"append_block","block_type":"paragraph","text":"M34_MINIMAL_EDIT_DEFAULTS\n\nThe Texture activation wrote a first visible revision from runtime-owned context."}]}`),
		}},
		Model: "test-model",
	}, nil
}

type textureWriteAndResearchProvider struct {
	provideriface.Provider
	choices    []string
	firstTools []provideriface.ToolDefinition
	wrote      bool
	spawned    bool
}

func (p *textureWriteAndResearchProvider) ProviderName() string {
	return "texture-write-and-research"
}

func (p *textureWriteAndResearchProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	return NewStubProvider(1*time.Millisecond).Execute(ctx, task, emit)
}

func (p *textureWriteAndResearchProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	if p.firstTools == nil && toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		p.firstTools = append([]provideriface.ToolDefinition(nil), req.ToolDefinitions...)
	}
	if messagesContainToolCall(req.Messages, "spawn_agent") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "handoff complete", Model: "test-model"}, nil
	}
	lastUser := extractLastUserMessage(req.Messages)
	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnTextureToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if p.wrote && !p.spawned && toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") {
		p.spawned = true
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{
				{
					ID:   "call-open-researcher",
					Name: "spawn_agent",
					Arguments: json.RawMessage(`{
						"role":"researcher",
						"objective":"Research the owner request and send concise evidence back to this Texture."
					}`),
				},
			},
			Model: "test-model",
		}, nil
	}
	if p.wrote || !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
	}
	p.wrote = true
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{{
			ID:        "call-write-working-revision",
			Name:      "patch_texture",
			Arguments: json.RawMessage(`{"edits":[{"op":"append_block","block_type":"paragraph","text":"FIRST_TURN_WRITE_AND_RESEARCH\n\nWorking revision while researcher evidence is pending."}]}`),
		}},
		Model: "test-model",
	}, nil
}

type textureAgenticResearchProvider struct {
	provideriface.Provider
	choices             []string
	wrote               bool
	spawned             bool
	sawPromptObligation bool
	sawGuardInstruction bool
}

func (p *textureAgenticResearchProvider) ProviderName() string {
	return "texture-agentic-research"
}

func (p *textureAgenticResearchProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	return NewStubProvider(1*time.Millisecond).Execute(ctx, task, emit)
}

func (p *textureAgenticResearchProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	if messagesContainText(req.Messages, "texture_model_prior_interim_needs_evidence_path") ||
		messagesContainText(req.Messages, "The latest canonical Texture revision is flagged model_prior_interim") {
		p.sawGuardInstruction = true
	}
	if messagesContainToolCall(req.Messages, "spawn_agent") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "research opened", Model: "test-model"}, nil
	}
	lastUser := extractLastUserMessage(req.Messages)
	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnTextureToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if strings.Contains(lastUser, "Probe morphisms (spawn_agent researcher) gather world knowledge") &&
		strings.Contains(lastUser, "Substantive world knowledge requires Probe (researcher) evidence") {
		p.sawPromptObligation = true
	}
	if p.wrote && !p.spawned && messagesContainToolCall(req.Messages, "patch_texture") {
		p.spawned = true
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-open-researcher-agentically",
				Name: "spawn_agent",
				Arguments: json.RawMessage(`{
					"role":"researcher",
					"objective":"Research what is going on with Anthropic and the US government. Send concise current evidence back to texture."
				}`),
			}},
			Model: "test-model",
		}, nil
	}
	if p.wrote || !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "premature interim-only completion", Model: "test-model"}, nil
	}
	p.wrote = true
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{{
			ID:        "call-write-agentic-working-revision",
			Name:      "patch_texture",
			Arguments: json.RawMessage(`{"edits":[{"op":"append_block","block_type":"paragraph","text":"AGENTIC_MODEL_PRIOR_V1\n\nThis is an interim model-prior question map. Current evidence is still unresolved."}]}`),
		}},
		Model: "test-model",
	}, nil
}

type textureInitialNoOpThenDraftProvider struct {
	provideriface.Provider
	choices                      []string
	attempts                     int
	sawNoOpError                 bool
	sawExactInitialPatchGuidance bool
	initialBlockID               string
}

func (p *textureInitialNoOpThenDraftProvider) ProviderName() string {
	return "texture-initial-noop-then-draft"
}

func (p *textureInitialNoOpThenDraftProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	return NewStubProvider(1*time.Millisecond).Execute(ctx, task, emit)
}

func (p *textureInitialNoOpThenDraftProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.choices = append(p.choices, req.ToolChoice)
	if p.initialBlockID == "" {
		p.initialBlockID = firstPromptOutlineParagraphID(req.System + "\n" + extractLastUserMessage(req.Messages))
	}
	for _, msg := range req.Messages {
		if strings.Contains(string(msg), "initial model-prior Texture revision must change prompt content") {
			p.sawNoOpError = true
		}
		if strings.Contains(string(msg), "call rewrite_texture instead") {
			p.sawExactInitialPatchGuidance = true
		}
	}
	if !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") || p.attempts >= 3 {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
	}
	p.attempts++
	if p.attempts == 1 {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-initial-missing-find",
				Name: "patch_texture",
				Arguments: json.RawMessage(`{
					"edits":[{
						"op":"replace",
						"find":"Missing exact section",
						"replace":"USEFUL_MODEL_PRIOR_V1"
					}]
				}`),
			}},
			Model: "test-model",
		}, nil
	}
	if p.attempts == 2 {
		blockID := p.initialBlockID
		if blockID == "" {
			blockID = "p-doc-initial-noop-retry-rev-doc-initial-noop-retry-v0-0"
		}
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-initial-noop",
				Name: "patch_texture",
				Arguments: json.RawMessage(fmt.Sprintf(`{
					"edits":[{
						"op":"update_block_text",
						"block_id":%q,
						"text":"Draft a short private note."
					}]
				}`, blockID)),
			}},
			Model: "test-model",
		}, nil
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "tool_use",
		ToolCalls: []types.ToolCall{{
			ID:        "call-initial-useful-draft",
			Name:      "patch_texture",
			Arguments: json.RawMessage(`{"edits":[{"op":"append_block","block_type":"paragraph","text":"USEFUL_MODEL_PRIOR_V1\n\nA short private note should name the audience, the decision to make, and the next concrete follow-up."}]}`),
		}},
		Model: "test-model",
	}, nil
}

func firstPromptOutlineParagraphID(text string) string {
	text = strings.ReplaceAll(text, `\n`, "\n")
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- paragraph id=") {
			continue
		}
		rest := strings.TrimPrefix(line, "- paragraph id=")
		if idx := strings.IndexAny(rest, " \t"); idx >= 0 {
			rest = rest[:idx]
		}
		return strings.TrimSpace(rest)
	}
	return ""
}

func textureAgentIDFromSystemPrompt(system string) string {
	const marker = "Current agent id: "
	for _, line := range strings.Split(system, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, marker) {
			continue
		}
		agentID := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, marker)), ".")
		if strings.HasPrefix(agentID, "texture:") {
			return agentID
		}
	}
	return ""
}

type textureResearchEvidenceLoopProvider struct {
	provideriface.Provider
	mu                 sync.Mutex
	choices            []string
	firstTools         []provideriface.ToolDefinition
	targetAgentID      string
	initialTextureDone bool
	researchUpdateDone bool
	wakeTextureDone    bool
}

func (p *textureResearchEvidenceLoopProvider) ProviderName() string {
	return "texture-research-evidence-loop"
}

func (p *textureResearchEvidenceLoopProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	return NewStubProvider(1*time.Millisecond).Execute(ctx, task, emit)
}

func (p *textureResearchEvidenceLoopProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	if p.targetAgentID == "" {
		p.targetAgentID = textureAgentIDFromSystemPrompt(req.System)
	}
	targetAgentID := p.targetAgentID
	p.choices = append(p.choices, req.ToolChoice)
	if p.firstTools == nil && toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		p.firstTools = append([]provideriface.ToolDefinition(nil), req.ToolDefinitions...)
	}
	p.mu.Unlock()

	lastUser := extractLastUserMessage(req.Messages)
	if p.initialTextureDone &&
		toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") &&
		!messagesContainToolCall(req.Messages, "spawn_agent") &&
		messagesContainToolCall(req.Messages, "patch_texture") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-spawn-researcher-after-guard",
				Name: "spawn_agent",
				Arguments: json.RawMessage(`{
					"role":"researcher",
					"objective":"Return one current-signal evidence packet to this Texture with update_coagent."
				}`),
			}},
			Model: "test-model",
		}, nil
	}

	if toolDefinitionsContain(req.ToolDefinitions, "update_coagent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		p.mu.Lock()
		alreadyUpdated := p.researchUpdateDone
		p.researchUpdateDone = true
		p.mu.Unlock()
		if alreadyUpdated {
			return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "research update already sent", Model: "test-model"}, nil
		}
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-research-update",
				Name: "update_coagent",
				Arguments: json.RawMessage(`{
						"schema_version":"coagent_source_packet.v1",
						"kind":"evidence_update",
						"agent_id":` + strconv.Quote(targetAgentID) + `,
						"summary":"Researcher evidence returned to Texture.",
						"claims":[{"text":"TEXTURE_CREATED_RESEARCH_EVIDENCE: public signal requires a cautious current-state revision.","source_ids":["src-current-signal"]}],
						"sources":[
							{
								"kind":"web_page",
								"source_id":"src-current-signal",
								"target":{"uri":"https://example.com/current-signal","title":"Current signal evidence"},
								"selectors":[{"kind":"text_quote","quote":"A current signal exists, but claims must stay scoped and cite worker evidence."}]
							}
						],
						"notes":["Use this evidence in the next Texture revision."]
				}`),
			}},
			Model: "test-model",
		}, nil
	}

	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnTextureToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
	}

	hasWorkerUpdate := messagesContainToolCall(req.Messages, "update_coagent") || messagesContainText(req.Messages, "coagent_update")
	p.mu.Lock()
	initialDone := p.initialTextureDone
	if !p.initialTextureDone {
		p.initialTextureDone = true
	}
	wakeDone := p.wakeTextureDone
	if initialDone && hasWorkerUpdate && !p.wakeTextureDone {
		p.wakeTextureDone = true
	}
	p.mu.Unlock()

	if !initialDone {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-initial-working-revision",
				Name:      "patch_texture",
				Arguments: json.RawMessage(`{"edits":[{"op":"append_block","block_type":"paragraph","text":"TEXTURE_INITIAL_WORKING_REVISION\n\nResearcher evidence has been requested and is pending."}]}`),
			}},
			Model: "test-model",
		}, nil
	}
	if hasWorkerUpdate && !wakeDone {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-evidence-backed-revision",
				Name:      "patch_texture",
				Arguments: json.RawMessage(`{"edits":[{"op":"append_block","block_type":"paragraph","text":"TEXTURE_V2_FROM_RESEARCH_EVIDENCE\n\nThe later revision incorporates TEXTURE_CREATED_RESEARCH_EVIDENCE and keeps current claims scoped to the worker packet."}]}`),
			}},
			Model: "test-model",
		}, nil
	}
	if messagesContainToolCall(req.Messages, "update_coagent") ||
		messagesContainToolCall(req.Messages, "spawn_agent") ||
		messagesContainToolCall(req.Messages, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
}

type textureSuperEvidenceLoopProvider struct {
	provideriface.Provider
	mu                 sync.Mutex
	choices            []string
	firstTools         []provideriface.ToolDefinition
	targetAgentID      string
	initialTextureDone bool
	superUpdateDone    bool
	wakeTextureDone    bool
}

func (p *textureSuperEvidenceLoopProvider) ProviderName() string {
	return "texture-super-evidence-loop"
}

func (p *textureSuperEvidenceLoopProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	return NewStubProvider(1*time.Millisecond).Execute(ctx, task, emit)
}

func (p *textureSuperEvidenceLoopProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	if p.targetAgentID == "" {
		p.targetAgentID = textureAgentIDFromSystemPrompt(req.System)
	}
	targetAgentID := p.targetAgentID
	p.choices = append(p.choices, req.ToolChoice)
	if p.firstTools == nil && toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		p.firstTools = append([]provideriface.ToolDefinition(nil), req.ToolDefinitions...)
	}
	p.mu.Unlock()

	lastUser := extractLastUserMessage(req.Messages)
	hasWorkerUpdate := messagesContainToolCall(req.Messages, "update_coagent") || messagesContainText(req.Messages, "coagent_update")
	if strings.Contains(lastUser, "model_prior_interim") &&
		toolDefinitionsContain(req.ToolDefinitions, "request_super_execution") &&
		!hasWorkerUpdate &&
		!messagesContainToolCall(req.Messages, "request_super_execution") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-request-super-after-guard",
				Name: "request_super_execution",
				Arguments: json.RawMessage(`{
					"objective":"Create artifacts/texture-super-proof.txt and report artifact/test evidence back to this Texture."
				}`),
			}},
			Model: "test-model",
		}, nil
	}
	if p.initialTextureDone &&
		toolDefinitionsContain(req.ToolDefinitions, "request_super_execution") &&
		!hasWorkerUpdate &&
		!messagesContainToolCall(req.Messages, "request_super_execution") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-request-super-after-v1",
				Name: "request_super_execution",
				Arguments: json.RawMessage(`{
					"objective":"Create artifacts/texture-super-proof.txt and report artifact/test evidence back to this Texture."
				}`),
			}},
			Model: "test-model",
		}, nil
	}

	if toolDefinitionsContain(req.ToolDefinitions, "update_coagent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		p.mu.Lock()
		alreadyUpdated := p.superUpdateDone
		p.superUpdateDone = true
		p.mu.Unlock()
		if alreadyUpdated {
			return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "super update already sent", Model: "test-model"}, nil
		}
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-super-update",
				Name: "update_coagent",
				Arguments: json.RawMessage(`{
						"schema_version":"coagent_source_packet.v1",
						"kind":"execution_result",
						"agent_id":` + strconv.Quote(targetAgentID) + `,
						"summary":"Super execution evidence returned to Texture.",
						"sources":[
							{"source_id":"src-super-artifact","kind":"file_artifact","target":{"uri":"file_artifact:artifacts/texture-super-proof.txt"}},
							{"source_id":"src-super-test","kind":"test_run","target":{"uri":"test_run:test -f artifacts/texture-super-proof.txt"}}
						],
						"actions":[{"type":"revise_texture","objective":"TEXTURE_CREATED_SUPER_EVIDENCE: execution proof is ready for Texture synthesis."}],
						"notes":["Use this execution evidence in the next Texture revision."]
					}`),
			}},
			Model: "test-model",
		}, nil
	}

	if toolDefinitionsContain(req.ToolDefinitions, "spawn_agent") && !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{conductorSpawnTextureToolCall(lastUser)},
			Model:      "test-model",
		}, nil
	}
	if !toolDefinitionsContain(req.ToolDefinitions, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
	}

	p.mu.Lock()
	initialDone := p.initialTextureDone
	if !p.initialTextureDone {
		p.initialTextureDone = true
	}
	wakeDone := p.wakeTextureDone
	if initialDone && hasWorkerUpdate && !p.wakeTextureDone {
		p.wakeTextureDone = true
	}
	p.mu.Unlock()

	if !initialDone {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-initial-execution-revision",
				Name:      "patch_texture",
				Arguments: json.RawMessage(`{"edits":[{"op":"append_block","block_type":"paragraph","text":"TEXTURE_INITIAL_EXECUTION_REVISION\n\nSuper execution evidence has been requested and is pending."}]}`),
			}},
			Model: "test-model",
		}, nil
	}
	if hasWorkerUpdate && !wakeDone {
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-super-evidence-backed-revision",
				Name:      "patch_texture",
				Arguments: json.RawMessage(`{"edits":[{"op":"append_block","block_type":"paragraph","text":"TEXTURE_V2_FROM_SUPER_EVIDENCE\n\nThe later revision incorporates TEXTURE_CREATED_SUPER_EVIDENCE and names the execution artifact as pending owner review."}]}`),
			}},
			Model: "test-model",
		}, nil
	}
	if messagesContainToolCall(req.Messages, "update_coagent") ||
		messagesContainToolCall(req.Messages, "request_super_execution") ||
		messagesContainToolCall(req.Messages, "patch_texture") {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model"}, nil
}

func TestTextureAgentRevisionCanEditUserProvidedTextWithoutWorkerHistory(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Hello, edited document.\n\nPolished structure."))

	h, s, _ := textureAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make the supplied text more formal."})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp textureAgentRevisionResponse
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
	if len(provider.choices) == 0 || provider.choices[0] != "required" {
		t.Fatalf("initial texture tool_choice = %#v, want generic required durable action for direct revise", provider.choices)
	}
	if len(provider.choices) != 2 {
		t.Fatalf("texture provider calls = %d choices=%#v, want a write turn plus a non-terminal continuation turn", len(provider.choices), provider.choices)
	}
	if provider.choices[1] != "" {
		t.Fatalf("continuation tool_choice = %q, want unconstrained after the non-terminal write", provider.choices[1])
	}
}

func TestDirectTextureReviseWritesWorkStateBeforeDelegatingResearch(t *testing.T) {
	t.Parallel()
	provider := &textureWriteAndResearchProvider{Provider: NewStubProvider(1 * time.Millisecond)}

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	rt.cfg.TextureActorParkIdle = time.Second
	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Research the current evidence, keep the owner-visible document honest while evidence is pending, and ask researcher for a grounded packet."})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp textureAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 2, 5*time.Second)
	var workStateRevision *types.Revision
	for i := range revs {
		if revs[i].AuthorKind == types.AuthorAppAgent && strings.Contains(revs[i].Content, "Working revision while researcher evidence is pending") {
			workStateRevision = &revs[i]
			break
		}
	}
	if workStateRevision == nil {
		t.Fatalf("direct revise did not write owner-visible work-state revision; revisions=%+v", revs)
	}
	if !strings.Contains(workStateRevision.Content, "pending") {
		t.Fatalf("work-state revision content = %q, want explicit pending-work state", workStateRevision.Content)
	}
	var researcher *types.RunRecord
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := rt.Store().ListRunsByOwner(context.Background(), "user-1", 100)
		if err != nil {
			t.Fatalf("list runs: %v", err)
		}
		for i := range runs {
			if runs[i].RequestedByRunID == resp.RunID && runs[i].AgentProfile == AgentProfileResearcher {
				researcher = &runs[i]
				break
			}
		}
		if researcher != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if researcher == nil {
		t.Fatalf("direct revise did not delegate researcher after work-state revision")
	}
	if len(provider.choices) < 2 || provider.choices[0] != "required" || provider.choices[1] != "" {
		t.Fatalf("texture provider choices = %#v, want generic durable action then unconstrained delegation turn", provider.choices)
	}
	if researcher.ChannelID != docID {
		t.Fatalf("researcher channel = %q, want doc channel %q; run=%+v", researcher.ChannelID, docID, *researcher)
	}
	sleepingRun := waitForStoredRunState(t, s, resp.RunID, types.RunPassivated, 5*time.Second)
	if got := metadataStringValue(sleepingRun.Metadata, "actor_sleep_state"); got != "idle" {
		t.Fatalf("texture run sleep state = %q, want idle; metadata=%+v", got, sleepingRun.Metadata)
	}
	mutation, err := s.GetAgentMutationByRun(context.Background(), resp.RunID)
	if err != nil {
		t.Fatalf("get sleeping texture mutation: %v", err)
	}
	if mutation == nil || mutation.State != "sleeping" {
		t.Fatalf("texture mutation = %+v, want sleeping after delegation", mutation)
	}
}

func TestInitialTextureRunWritesFirstAppagentRevisionThroughEdit(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("First Texture-authored working revision."))

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
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
		t.Fatalf("conductor did not create texture route: %+v", decision)
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
		t.Fatalf("initial texture state = %q, want completed", state)
	}
	initialRun, err := rt.GetRun(context.Background(), decision.InitialLoopID, "user-1")
	if err != nil {
		t.Fatalf("get initial texture run: %v", err)
	}
	if _, ok := initialRun.Metadata["requires_worker_grounding"]; ok {
		t.Fatalf("initial texture run should not carry requires_worker_grounding metadata: %+v", initialRun.Metadata)
	}

	revs, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("revision count = %d, want only v0/v1", len(revs))
	}
	foundTextureRevision := false
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "First Texture-authored working revision") {
			foundTextureRevision = true
		}
	}
	if !foundTextureRevision {
		t.Fatalf("expected first Texture-authored appagent revision, got %+v", revs)
	}
	mutation, err := s.GetAgentMutationByRun(context.Background(), decision.InitialLoopID)
	if err != nil {
		t.Fatalf("get initial mutation: %v", err)
	}
	if mutation == nil || mutation.State != "completed" {
		t.Fatalf("initial texture mutation = %+v, want completed mutation", mutation)
	}
}

func TestInitialTextureRunWritesBeforeSpawningResearcher(t *testing.T) {
	t.Parallel()
	provider := &textureWriteAndResearchProvider{Provider: NewStubProvider(1 * time.Millisecond)}

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Research current signals, write a working Texture, and ask researcher for evidence."}`, "user-1")
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
		t.Fatalf("conductor did not create texture route: %+v", decision)
	}
	if state := waitForTaskCompletion(t, h, decision.InitialLoopID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("initial texture state = %q, want completed", state)
	}
	if len(provider.choices) < 2 || provider.choices[0] != "" || provider.choices[1] != "" {
		t.Fatalf("initial texture tool choices = %#v, want unconstrained first paint and continuation", provider.choices)
	}
	assertInitialTextureAutonomousSurface(t, provider.firstTools)
	revs, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	var wroteRevision bool
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent && strings.Contains(rev.Content, "FIRST_TURN_WRITE_AND_RESEARCH") {
			wroteRevision = true
		}
	}
	if !wroteRevision {
		t.Fatalf("initial Texture turn did not write expected appagent revision: %+v", revs)
	}
	runs, err := rt.Store().ListRunsByOwner(context.Background(), "user-1", 100)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	var researcher *types.RunRecord
	for i := range runs {
		if runs[i].RequestedByRunID == decision.InitialLoopID && runs[i].AgentProfile == AgentProfileResearcher {
			researcher = &runs[i]
			break
		}
	}
	if researcher == nil {
		t.Fatalf("initial Texture turn did not spawn researcher; runs=%+v", runs)
	}
	if researcher.ChannelID != decision.DocID || trajectoryIDForRun(researcher) != submission.SubmissionID {
		t.Fatalf("researcher route = channel %q trajectory %q, want %s/%s; run=%+v", researcher.ChannelID, trajectoryIDForRun(researcher), decision.DocID, submission.SubmissionID, *researcher)
	}
}

func TestTextureCurrentEventsPromptCanOpenProbePathWithoutCompletionGuard(t *testing.T) {
	t.Parallel()
	provider := &textureAgenticResearchProvider{Provider: NewStubProvider(1 * time.Millisecond)}

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"What's going on with Anthropic and the US government?"}`, "user-1")
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
		t.Fatalf("conductor did not create texture route: %+v", decision)
	}
	if state := waitForTaskCompletion(t, h, decision.InitialLoopID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("initial texture state = %q, want completed", state)
	}
	if provider.sawGuardInstruction {
		t.Fatalf("provider saw removed model-prior completion guard; choices=%#v", provider.choices)
	}
	if !provider.sawPromptObligation {
		t.Fatalf("provider never saw prompt-level Probe obligation; choices=%#v", provider.choices)
	}
	if !provider.spawned {
		t.Fatalf("provider did not open researcher by ordinary tool choice; choices=%#v", provider.choices)
	}
	if len(provider.choices) < 2 {
		t.Fatalf("texture choices = %#v, want at least conductor and Texture calls", provider.choices)
	}
	for _, choice := range provider.choices {
		if choice != "" {
			t.Fatalf("texture choices = %#v, want unconstrained conductor/Texture tool choices", provider.choices)
		}
	}
	events, err := rt.Store().ListEvents(context.Background(), decision.InitialLoopID, 100)
	if err != nil {
		t.Fatalf("list texture events: %v", err)
	}
	for _, event := range events {
		if event.Kind == types.EventRunRetry && event.Phase == "completion_guard" {
			t.Fatalf("unexpected completion guard retry event: %+v", event)
		}
		if strings.Contains(string(event.Payload), "texture_model_prior_interim_needs_evidence_path") {
			t.Fatalf("unexpected model-prior completion guard payload: %+v", event)
		}
	}
	revs, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	var v1 *types.Revision
	for i := range revs {
		if revs[i].AuthorKind == types.AuthorAppAgent && strings.Contains(revs[i].Content, "AGENTIC_MODEL_PRIOR_V1") {
			v1 = &revs[i]
			break
		}
	}
	if v1 == nil {
		t.Fatalf("missing agentic model-prior V1; revisions=%+v", revs)
	}
	meta := decodeRevisionMetadata(v1.Metadata)
	if !metadataBoolValue(meta, "model_prior_interim") || metadataStringValue(meta, "revision_grounding") != "model_prior" {
		t.Fatalf("V1 metadata not marked model-prior/interim: %+v", meta)
	}
	runs, err := rt.Store().ListRunsByOwner(context.Background(), "user-1", 100)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	var researcher *types.RunRecord
	for i := range runs {
		if runs[i].RequestedByRunID == decision.InitialLoopID && runs[i].AgentProfile == AgentProfileResearcher {
			researcher = &runs[i]
			break
		}
	}
	if researcher == nil {
		t.Fatalf("agentic Texture tool choice did not lead to researcher probe; runs=%+v", runs)
	}
	if researcher.ChannelID != decision.DocID || trajectoryIDForRun(researcher) != submission.SubmissionID {
		t.Fatalf("researcher route = channel %q trajectory %q, want %s/%s; run=%+v", researcher.ChannelID, trajectoryIDForRun(researcher), decision.DocID, submission.SubmissionID, *researcher)
	}
}

func TestTextureCreatedResearcherEvidenceWakesTextureV2(t *testing.T) {
	t.Parallel()
	provider := &textureResearchEvidenceLoopProvider{Provider: NewStubProvider(1 * time.Millisecond)}
	clock := &fakeTextureWakeClock{}

	h, s, rt := textureAPISetupWithProviderAndOptions(t, provider, true, withTextureWakeAfterFuncForTest(clock.afterFunc))
	rt.cfg.TextureActorParkIdle = time.Second
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"What's new in current infrastructure signals? Write a cautious working Texture and ask researcher for evidence."}`, "user-1")
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
		t.Fatalf("conductor did not create texture route: %+v", decision)
	}
	_ = waitForRevisionCount(t, s, decision.DocID, "user-1", 2, 5*time.Second)
	assertInitialTextureAutonomousSurface(t, provider.firstTools)

	var researcherRun *types.RunRecord
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := rt.Store().ListRunsByOwner(context.Background(), "user-1", 100)
		if err != nil {
			t.Fatalf("list runs: %v", err)
		}
		for i := range runs {
			if runs[i].RequestedByRunID == decision.InitialLoopID && runs[i].AgentProfile == AgentProfileResearcher {
				researcherRun = &runs[i]
				break
			}
		}
		if researcherRun != nil && researcherRun.State.Terminal() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if researcherRun == nil {
		t.Fatal("Texture-created researcher run was not found")
	}
	if researcherRun.State != types.RunCompleted {
		t.Fatalf("researcher state = %q, want completed; run=%+v", researcherRun.State, *researcherRun)
	}

	updates, err := s.ListWorkerUpdatesByTrajectory(context.Background(), "user-1", submission.SubmissionID, 20)
	if err != nil {
		t.Fatalf("list worker updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("worker updates = %+v, want exactly one researcher update", updates)
	}
	update := updates[0]
	if update.Role != AgentProfileResearcher ||
		update.AgentID != researcherRun.AgentID ||
		update.TargetAgentID != currentTextureAgentID(decision.DocID) ||
		update.ChannelID != decision.DocID ||
		update.MessageSeq == 0 {
		t.Fatalf("researcher update route = %+v, researcher=%+v", update, *researcherRun)
	}

	clock.fireAll()
	revs := waitForRevisionCount(t, s, decision.DocID, "user-1", 3, 5*time.Second)
	var evidenceRev *types.Revision
	for i := range revs {
		if revs[i].AuthorKind == types.AuthorAppAgent && strings.Contains(revs[i].Content, "TEXTURE_V2_FROM_RESEARCH_EVIDENCE") {
			evidenceRev = &revs[i]
			break
		}
	}
	if evidenceRev == nil {
		t.Fatalf("Texture did not create V2 from researcher evidence; revs=%+v", revs)
	}
	if evidenceRev.VersionNumber < 2 {
		t.Fatalf("evidence revision version = %d, want V2 or later; rev=%+v", evidenceRev.VersionNumber, *evidenceRev)
	}
	meta := decodeRevisionMetadata(evidenceRev.Metadata)
	consumed := metadataSlice(t, meta, "worker_updates_consumed")
	if len(consumed) != 1 {
		t.Fatalf("worker_updates_consumed length = %d, want 1; metadata=%+v", len(consumed), meta)
	}
	consumedUpdate := consumed[0].(map[string]any)
	if got := int64(consumedUpdate["seq"].(float64)); got != update.MessageSeq {
		t.Fatalf("consumed worker seq = %d, want %d", got, update.MessageSeq)
	}
	if got, _ := consumedUpdate["from_loop_id"].(string); got != researcherRun.RunID {
		t.Fatalf("consumed from_loop_id = %q, want %q", got, researcherRun.RunID)
	}
	doc, err := s.GetDocument(context.Background(), decision.DocID, "user-1")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != evidenceRev.RevisionID {
		t.Fatalf("document head = %q, want evidence revision %q", doc.CurrentRevisionID, evidenceRev.RevisionID)
	}
	sleepingRun := waitForStoredRunState(t, s, decision.InitialLoopID, types.RunPassivated, 5*time.Second)
	if ids := metadataStringSlice(sleepingRun.Metadata["worker_update_ids"]); !containsString(ids, update.UpdateID) {
		t.Fatalf("same Texture run worker_update_ids = %+v, want %s", ids, update.UpdateID)
	}
	runs, err := s.ListRunsByChannel(context.Background(), "user-1", decision.DocID, 20)
	if err != nil {
		t.Fatalf("list channel runs: %v", err)
	}
	var textureRevisionRuns []types.RunRecord
	for _, run := range runs {
		if agentProfileForRun(&run) == AgentProfileTexture && isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) {
			textureRevisionRuns = append(textureRevisionRuns, run)
		}
	}
	if len(textureRevisionRuns) != 1 || textureRevisionRuns[0].RunID != decision.InitialLoopID {
		t.Fatalf("texture revision runs = %+v, want only original durable thread %s", textureRevisionRuns, decision.InitialLoopID)
	}
}

func TestTextureCreatedSuperEvidenceWakesTextureV2(t *testing.T) {
	t.Parallel()
	provider := &textureSuperEvidenceLoopProvider{Provider: NewStubProvider(1 * time.Millisecond)}

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	rt.cfg.TextureActorParkIdle = time.Second
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Create a Texture that needs execution evidence. Ask super to produce a tiny artifact proof before finalizing."}`, "user-1")
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
		t.Fatalf("conductor did not create texture route: %+v", decision)
	}
	initialRevs := waitForRevisionCount(t, s, decision.DocID, "user-1", 2, 5*time.Second)
	if !revisionContentsContain(initialRevs, "TEXTURE_INITIAL_EXECUTION_REVISION") {
		t.Fatalf("initial Texture revision missing execution marker; revs=%+v", initialRevs)
	}
	assertInitialTextureAutonomousSurface(t, provider.firstTools)

	var superRun *types.RunRecord
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := rt.Store().ListRunsByOwner(context.Background(), "user-1", 100)
		if err != nil {
			t.Fatalf("list runs: %v", err)
		}
		for i := range runs {
			if runs[i].AgentProfile == AgentProfileSuper &&
				runs[i].AgentID == persistentSuperAgentID("user-1") &&
				trajectoryIDForRun(&runs[i]) == submission.SubmissionID {
				superRun = &runs[i]
				break
			}
		}
		if superRun != nil && superRun.State.Terminal() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if superRun == nil {
		t.Fatal("Texture-created persistent super run was not found")
	}
	if superRun.State != types.RunCompleted {
		t.Fatalf("super state = %q, want completed; run=%+v", superRun.State, *superRun)
	}
	if metadataStringValue(superRun.Metadata, "requested_by_profile") != AgentProfileTexture {
		t.Fatalf("super requested_by_profile = %q, want texture; metadata=%+v", metadataStringValue(superRun.Metadata, "requested_by_profile"), superRun.Metadata)
	}

	updates, err := s.ListWorkerUpdatesByTrajectory(context.Background(), "user-1", submission.SubmissionID, 20)
	if err != nil {
		t.Fatalf("list worker updates: %v", err)
	}
	var superUpdate *types.CoagentSourcePacket
	for i := range updates {
		if updates[i].Role == AgentProfileSuper && updates[i].TargetAgentID == currentTextureAgentID(decision.DocID) {
			superUpdate = &updates[i]
			break
		}
	}
	if superUpdate == nil {
		t.Fatalf("missing super update back to Texture; updates=%+v", updates)
	}
	if superUpdate.AgentID != superRun.AgentID ||
		superUpdate.ChannelID != decision.DocID ||
		superUpdate.MessageSeq == 0 ||
		len(coagentPacketSourceURIs(superUpdate.Packet, "file_artifact")) == 0 ||
		len(coagentPacketSourceURIs(superUpdate.Packet, "test_run")) == 0 {
		t.Fatalf("super update route/evidence = %+v, super=%+v", *superUpdate, *superRun)
	}

	revs := waitForRevisionCount(t, s, decision.DocID, "user-1", 3, 5*time.Second)
	var evidenceRev *types.Revision
	for i := range revs {
		if revs[i].AuthorKind == types.AuthorAppAgent && strings.Contains(revs[i].Content, "TEXTURE_V2_FROM_SUPER_EVIDENCE") {
			evidenceRev = &revs[i]
			break
		}
	}
	if evidenceRev == nil {
		t.Fatalf("Texture did not create V2 from super evidence; revs=%+v", revs)
	}
	if evidenceRev.VersionNumber < 2 {
		t.Fatalf("evidence revision version = %d, want V2 or later; rev=%+v", evidenceRev.VersionNumber, *evidenceRev)
	}
	meta := decodeRevisionMetadata(evidenceRev.Metadata)
	consumed := metadataSlice(t, meta, "worker_updates_consumed")
	if len(consumed) != 1 {
		t.Fatalf("worker_updates_consumed length = %d, want 1; metadata=%+v", len(consumed), meta)
	}
	consumedUpdate := consumed[0].(map[string]any)
	if got := int64(consumedUpdate["seq"].(float64)); got != superUpdate.MessageSeq {
		t.Fatalf("consumed worker seq = %d, want %d", got, superUpdate.MessageSeq)
	}
	if got, _ := consumedUpdate["from_loop_id"].(string); got != superRun.RunID {
		t.Fatalf("consumed from_loop_id = %q, want %q", got, superRun.RunID)
	}
}

func TestInitialTextureRunDefaultsMinimalEditContextFromActivation(t *testing.T) {
	t.Parallel()
	provider := &textureMinimalEditProvider{Provider: NewStubProvider(1 * time.Millisecond)}

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Write a short M3.4 visible first draft."}`, "user-1")
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
		t.Fatalf("conductor did not create texture route: %+v", decision)
	}
	if state := waitForTaskCompletion(t, h, decision.InitialLoopID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("initial texture state = %q, want completed", state)
	}
	if len(provider.choices) != 2 || provider.choices[0] != "" {
		t.Fatalf("texture provider choices = %#v, want unconstrained first paint plus a non-terminal continuation turn", provider.choices)
	}
	if provider.choices[1] != "" {
		t.Fatalf("continuation tool_choice = %q, want unconstrained after the non-terminal write", provider.choices[1])
	}
	assertInitialTextureAutonomousSurface(t, provider.firstTools)
	revs, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	var appRevision *types.Revision
	for i := range revs {
		if revs[i].AuthorKind == types.AuthorAppAgent {
			appRevision = &revs[i]
			break
		}
	}
	if appRevision == nil || !strings.Contains(appRevision.Content, "M34_MINIMAL_EDIT_DEFAULTS") {
		t.Fatalf("expected appagent revision from minimal edit context, got %+v", revs)
	}
	meta := decodeRevisionMetadata(appRevision.Metadata)
	if metadataString(meta, "source") != "patch_texture" ||
		metadataString(meta, "texture_edit_operation") != "apply_edits" ||
		metadataString(meta, "texture_edit_base_revision_id") == "" {
		t.Fatalf("appagent revision metadata = %+v, want defaulted patch_texture context", meta)
	}
	mutation, err := s.GetAgentMutationByRun(context.Background(), decision.InitialLoopID)
	if err != nil {
		t.Fatalf("get initial mutation: %v", err)
	}
	if mutation == nil || mutation.State != "completed" {
		t.Fatalf("initial texture mutation = %+v, want completed mutation", mutation)
	}
}

func TestInitialTextureDecisionPromptRejectsPrematureEditBeforeDecision(t *testing.T) {
	provider := &textureDecisionThenEditProvider{Provider: NewStubProvider(1 * time.Millisecond)}

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	prompt := "Create a short Texture document titled M32_TEXTURE_DECISION_ROUTE_TEST. The body should say this marker is a deployed acceptance probe. Keep the document reader-facing only. Because this task is fully supplied and requires no research or execution worker, record an off-document Texture decision note with decision_kind no_worker_needed, exact reason M3.2 staging proof: user supplied the needed content and requested no research or execution worker., evidence ref staging-marker:M32_TEXTURE_DECISION_ROUTE_TEST, next action Write the concise reader-facing Texture revision. Then write the concise reader-facing Texture revision."
	body, err := json.Marshal(map[string]string{"text": prompt})
	if err != nil {
		t.Fatalf("marshal prompt-bar request: %v", err)
	}
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", string(body), "user-1")
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
		t.Fatalf("conductor did not create texture route: %+v", decision)
	}
	if state := waitForTaskCompletion(t, h, decision.InitialLoopID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("initial texture state = %q, want completed", state)
	}

	decisions, err := s.ListTextureDecisionsByDocument(context.Background(), "user-1", decision.DocID, 10)
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if len(decisions) != 1 || decisions[0].DecisionKind != "no_worker_needed" {
		t.Fatalf("decisions = %+v, want one no_worker_needed record", decisions)
	}
	revs, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("revision count = %d, want user input plus one appagent revision", len(revs))
	}
	var appContent string
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent {
			appContent = rev.Content
		}
	}
	if !strings.Contains(appContent, "M32_TEXTURE_DECISION_ROUTE_TEST") {
		t.Fatalf("appagent revision content = %q, want marker", appContent)
	}
	if strings.Contains(appContent, "private reason leaked") ||
		strings.Contains(appContent, "M3.2 staging proof: user supplied the needed content") {
		t.Fatalf("appagent revision leaked private decision rationale: %q", appContent)
	}
	if len(provider.choices) < 2 ||
		provider.choices[0] != "" ||
		provider.choices[1] != "" {
		t.Fatalf("tool choices = %#v, want unconstrained first turn and free follow-up after decision record", provider.choices)
	}
	assertInitialTextureAutonomousSurface(t, provider.firstTools)
}

func TestTexturePromptSteersCurrentEventsToResearcherNotSuper(t *testing.T) {
	t.Parallel()
	current := types.Revision{
		DocID:      "doc-current-events",
		RevisionID: "rev-current-events",
		Content:    "what's going on with iran deal now",
		AuthorKind: types.AuthorAppAgent,
	}
	prompt := buildAgentRevisionRequest(current, nil, nil, textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "what's going on with iran deal now",
	}, "", false, nil, nil)

	for _, want := range []string{
		"For factual/current claims, keep the revision explicitly model-prior/interim and uncertain until researcher evidence arrives.",
		"Probe (researcher) gathers world-knowledge evidence",
		"Ordinary factual, current-events, web, or \"what is going on now\" questions are Probe work",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("current-events texture prompt missing %q:\n%s", want, prompt)
		}
	}
	assertNoForcedSemanticDelegation(t, prompt)
}

func TestTexturePromptNarrativeResearcherWordsDoNotSelectPolicyBranch(t *testing.T) {
	t.Parallel()
	current := types.Revision{
		DocID:      "doc-explicit-researcher",
		RevisionID: "rev-explicit-researcher",
		Content:    "Ask researcher for a concise finding and ask super for a tiny verification note.",
		AuthorKind: types.AuthorUser,
	}
	prompt := buildAgentRevisionRequest(current, nil, nil, textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "Ask researcher for a concise finding and ask super for a tiny verification note.",
	}, "", false, nil, nil)

	for _, forbidden := range []string{
		"The owner explicitly asked for researcher help.",
		"Probe (researcher) is the correct morphism class for that world-knowledge gap",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("narrative researcher words selected policy branch %q:\n%s", forbidden, prompt)
		}
	}
	assertNoForcedSemanticDelegation(t, prompt)
}

func TestTextureAgentRevisionAppliesStructuredEdit(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider("")
	provider.resultFunc = func(prompt string) string {
		blockID := firstPromptOutlineParagraphID(prompt)
		return textureStructuredApplyEditsResult([]textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: blockID,
			Text:    "Hello, edited document.",
		}, {
			Op:        "append_block",
			BlockType: "paragraph",
			Text:      "Evidence: structured worker update integrated.",
		}})
	}

	h, s, _ := textureAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Integrate the addressed worker update."})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp textureAgentRevisionResponse
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
	if meta["texture_edit_operation"] != "apply_edits" {
		t.Fatalf("texture_edit_operation = %v, want apply_edits; metadata=%+v", meta["texture_edit_operation"], meta)
	}
}

func TestTextureAgentRevisionIgnoresRawStubProviderResult(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithProvider(t, NewStubProvider(1*time.Millisecond), false)
	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Revise with the default stub provider."})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp textureAgentRevisionResponse
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

func TestTextureAgentRevisionIgnoresProviderFinalJSONEdit(t *testing.T) {
	t.Parallel()
	provider := &finalTextProvider{result: textureReplaceAllResult("FINAL JSON MUST NOT MATERIALIZE")}
	h, s, _ := textureAPISetupWithProvider(t, provider, true)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Return a legacy structured edit as final text."})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp textureAgentRevisionResponse
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

func TestTextureAgentRevisionRejectsMalformedEditTextureToolCall(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureStructuredApplyEditsResult([]textureStructuredEdit{
		{Op: "update_block_text", BlockID: "missing-block-id", Text: "replacement"},
	}))
	h, s, _ := textureAPISetupWithProvider(t, provider, true)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Apply an invalid edit."})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp textureAgentRevisionResponse
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

func TestTextureStaleAgentRevisionRejectsEditAfterUserEdit(t *testing.T) {
	t.Parallel()
	provider := &revisionPromptEchoProvider{delay: 250 * time.Millisecond}

	h, s, _ := textureAPISetupWithProvider(t, provider, true)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Produce a draft from the current document."})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var initialResp textureAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&initialResp); err != nil {
		t.Fatalf("decode initial agent revision response: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		rec, err := h.rt.GetRun(context.Background(), initialResp.RunID, "user-1")
		if err != nil {
			t.Fatalf("get initial texture run: %v", err)
		}
		if rec.State == types.RunRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("initial texture run never reached running state; last state=%q", rec.State)
		}
		time.Sleep(20 * time.Millisecond)
	}

	userEditReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content:          "Fresh user edit should survive.",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "user",
		ParentRevisionID: baseRevisionID,
	})
	userEditW := httptest.NewRecorder()
	h.HandleTextureRevisions(userEditW, userEditReq)
	if userEditW.Code != http.StatusCreated {
		t.Fatalf("create user redirect revision: status = %d, want %d; body: %s", userEditW.Code, http.StatusCreated, userEditW.Body.String())
	}
	var userEditResp textureRevisionResponse
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
			t.Fatalf("stale Texture write call created appagent revision: %+v", rev)
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

func TestTextureSeededStochasticWorkflowContracts(t *testing.T) {
	const ownerID = "user-1"
	const seed int64 = 20260430
	rng := rand.New(rand.NewSource(seed))
	provider := &stochasticWorkflowProvider{delay: 1500 * time.Millisecond}

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	conductorRun, err := rt.StartRunWithMetadata(context.Background(), "Build a toy evolution model and verify it.", ownerID, map[string]any{
		runMetadataAgentProfile:  AgentProfileConductor,
		runMetadataAgentRole:     AgentProfileConductor,
		"input_source":           "prompt_bar",
		"requested_app":          "texture",
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
		t.Fatalf("conductor decision missing durable texture ids: %+v", decision)
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

	initialReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+decision.DocID+"/revise",
		map[string]string{"prompt": "Start a long stochastic workflow."})
	initialW := httptest.NewRecorder()
	h.HandleTextureAgentRevision(initialW, initialReq)
	if initialW.Code != http.StatusAccepted {
		t.Fatalf("start initial texture revision: status = %d, want %d; body: %s", initialW.Code, http.StatusAccepted, initialW.Body.String())
	}
	var initialResp textureAgentRevisionResponse
	if err := json.NewDecoder(initialW.Body).Decode(&initialResp); err != nil {
		t.Fatalf("decode initial texture response: %v", err)
	}
	waitForRunRunning(t, rt, initialResp.RunID, ownerID, 5*time.Second)

	researchRun, err := rt.StartCoagentRun(context.Background(), initialResp.RunID, "Research toy model evidence", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    decision.DocID,
	})
	if err != nil {
		t.Fatalf("start researcher worker: %v", err)
	}
	superRun, err := rt.StartCoagentRun(context.Background(), initialResp.RunID, "Verify generated toy model", ownerID, map[string]any{
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
		seq, err := rt.ChannelCast(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(run)), decision.DocID, "texture:"+decision.DocID, "", from, role, content)
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
		t.Fatalf("initial stale texture state = %q, want completed", staleState)
	}
	mutation, err := s.GetAgentMutationByRun(context.Background(), initialResp.RunID)
	if err != nil {
		t.Fatalf("get initial mutation: %v", err)
	}
	if mutation == nil || mutation.State != "failed" {
		t.Fatalf("initial stale mutation = %+v, want failed no-write mutation", mutation)
	}
	waitForTextureQuiescent(t, rt, s, ownerID, decision.DocID, maxWorkerSeq, 20*time.Second)

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
		t.Fatalf("get initial texture run: %v", err)
	}
	if initialRun.RequestedByRunID != conductorRun.RunID {
		t.Fatalf("initial texture run parent = %q, want conductor run %q", initialRun.RequestedByRunID, conductorRun.RunID)
	}
	trajectoryID := trajectoryIDForRun(&initialRun)
	if trajectoryID != conductorRun.RunID {
		t.Fatalf("initial texture trajectory = %q, want conductor trajectory %q", trajectoryID, conductorRun.RunID)
	}
	events, err := s.ListEventsByTrajectory(context.Background(), ownerID, trajectoryID, 500)
	if err != nil {
		t.Fatalf("list stochastic trajectory events: %v", err)
	}
	hasChannelMessage := false
	hasTextureRevision := false
	for _, ev := range events {
		switch ev.Kind {
		case types.EventChannelMessage:
			hasChannelMessage = true
		case types.EventTextureDocumentRevisionCreated, types.EventTextureAgentRevisionCompleted:
			hasTextureRevision = true
		}
	}
	if !hasChannelMessage || !hasTextureRevision {
		t.Fatalf("trajectory events missing causality markers: channel=%v texture_revision=%v events=%+v", hasChannelMessage, hasTextureRevision, events)
	}

	cancelReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+decision.DocID+"/revise",
		map[string]string{"prompt": "CANCEL_RUN_MARKER"})
	cancelW := httptest.NewRecorder()
	h.HandleTextureAgentRevision(cancelW, cancelReq)
	if cancelW.Code != http.StatusAccepted {
		t.Fatalf("start cancellable texture revision: status = %d, want %d; body: %s", cancelW.Code, http.StatusAccepted, cancelW.Body.String())
	}
	var cancelResp textureAgentRevisionResponse
	if err := json.NewDecoder(cancelW.Body).Decode(&cancelResp); err != nil {
		t.Fatalf("decode cancellable texture response: %v", err)
	}
	waitForRunRunning(t, rt, cancelResp.RunID, ownerID, 5*time.Second)
	if err := rt.CancelRun(context.Background(), cancelResp.RunID, ownerID); err != nil {
		t.Fatalf("cancel texture run: %v", err)
	}
	cancelState := waitForTaskCompletion(t, h, cancelResp.RunID, 5*time.Second)
	if cancelState != types.RunCancelled {
		t.Fatalf("cancelled run state = %q, want cancelled", cancelState)
	}
	waitForTextureQuiescent(t, rt, s, ownerID, decision.DocID, maxWorkerSeq, 5*time.Second)
	revsAfterCancel, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, ownerID, 50)
	if err != nil {
		t.Fatalf("list revisions after cancellation: %v", err)
	}
	for _, rev := range revsAfterCancel {
		if strings.Contains(rev.Content, "CANCELLED SHOULD NOT MATERIALIZE") {
			t.Fatalf("cancelled texture output was materialized: %+v", rev)
		}
	}
}

func TestTextureWorkerMessageAutoWakeCreatesFollowUpRevision(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Integrated grounded findings into the next revision."))

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	userRevReq := textureCreateRevisionRequest{
		Content:     "Original draft.\n\nAdd a short section about recent model releases.",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
	}
	userRevReqBody := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", userRevReq)
	userRevW := httptest.NewRecorder()
	h.HandleTextureRevisions(userRevW, userRevReqBody)
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
	skippedSeq, err := rt.ChannelCast(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(noiseRun)), docID, "texture:"+docID, "", "auditor-1", "auditor", "This addressed note is not a worker update and must not drive synthesis.")
	if err != nil {
		t.Fatalf("post non-worker message: %v", err)
	}
	workerSeq, err := rt.ChannelCast(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(researchRun)), docID, "texture:"+docID, "", "researcher-1", "researcher", "Evidence: the latest public model releases shipped this week with stronger reasoning and tool use.")
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
		if agentProfileForRun(&runs[i]) == AgentProfileTexture && runs[i].RequestedByRunID == researchRun.RunID {
			wakeRun = &runs[i]
			break
		}
	}
	if wakeRun == nil {
		t.Fatalf("expected wake-driven texture run on channel %s, got %+v", docID, runs)
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

func TestTextureWorkerMessageAutoWakeBatchesRapidMessages(t *testing.T) {
	provider := newTextureEditToolProvider(textureReplaceAllResult("Integrated multiple grounded findings into one revision."))

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)

	userRevReq := textureCreateRevisionRequest{
		Content:     "Original draft.\n\nNeed the newest facts.",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
	}
	userRevReqBody := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", userRevReq)
	userRevW := httptest.NewRecorder()
	h.HandleTextureRevisions(userRevW, userRevReqBody)
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
		seq, err := rt.ChannelCast(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(researchRun)), docID, "texture:"+docID, "", "researcher-1", "researcher", content)
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
		if agentProfileForRun(&runs[i]) == AgentProfileTexture && runs[i].RequestedByRunID == researchRun.RunID {
			wakeRuns = append(wakeRuns, runs[i])
		}
	}
	if len(wakeRuns) != 1 {
		t.Fatalf("expected one debounced texture wake run, got %+v", wakeRuns)
	}
	if !strings.Contains(wakeRuns[0].Prompt, "Evidence A: the first grounded fact arrived.") {
		t.Fatalf("wake run prompt missing first worker message: %q", wakeRuns[0].Prompt)
	}
	if !strings.Contains(wakeRuns[0].Prompt, "Evidence B: the second grounded fact arrived.") {
		t.Fatalf("wake run prompt missing second worker message: %q", wakeRuns[0].Prompt)
	}
}

func TestTextureWorkerMessageDebounceUsesFakeClock(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Integrated fake-clock worker findings."))
	clock := &fakeTextureWakeClock{}

	h, s, rt := textureAPISetupWithProviderAndOptions(t, provider, true, withTextureWakeAfterFuncForTest(clock.afterFunc))
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
		seq, err := rt.ChannelCast(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(researchRun)), docID, "texture:"+docID, "", "researcher-1", "researcher", content)
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
		if agentProfileForRun(&run) == AgentProfileTexture && run.RequestedByRunID == researchRun.RunID {
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

func TestTextureWorkerWakeRequeuesWhileMutationPending(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Integrated after pending mutation cleared."))
	clock := &fakeTextureWakeClock{}

	h, s, rt := textureAPISetupWithProviderAndOptions(t, provider, true, withTextureWakeAfterFuncForTest(clock.afterFunc))
	docID, _ := createDocWithUserRevision(t, h)

	blockingRunID := "blocking-texture-mutation"
	if err := s.CreateAgentMutation(context.Background(), store.AgentMutation{
		DocID:     docID,
		RunID:     blockingRunID,
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create pending mutation: %v", err)
	}

	researchRun, err := rt.StartRunWithMetadata(context.Background(), "Research while texture mutation is pending", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start research run: %v", err)
	}
	if _, err := rt.ChannelCast(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(researchRun)), docID, "texture:"+docID, "", "researcher-1", "researcher", "Evidence while a previous Texture mutation is still pending."); err != nil {
		t.Fatalf("post worker message: %v", err)
	}

	clock.fireAll()
	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs after blocked wake: %v", err)
	}
	for _, run := range runs {
		if agentProfileForRun(&run) == AgentProfileTexture && run.RequestedByRunID == researchRun.RunID {
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

func TestResearcherUpdateWakeUsesSameDebouncedPath(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Integrated persisted findings into the next revision."))
	provider.delay = 500 * time.Millisecond
	clock := &fakeTextureWakeClock{}

	h, s, rt := textureAPISetupWithProviderAndOptions(t, provider, true, withTextureWakeAfterFuncForTest(clock.afterFunc))
	docID, _ := createDocWithUserRevision(t, h)

	userRevReq := textureCreateRevisionRequest{
		Content:     "Original draft.\n\nNeed a sourced update.",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
	}
	userRevReqBody := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", userRevReq)
	userRevW := httptest.NewRecorder()
	h.HandleTextureRevisions(userRevW, userRevReqBody)
	if userRevW.Code != http.StatusCreated {
		t.Fatalf("second user revision: status = %d, want %d; body: %s", userRevW.Code, http.StatusCreated, userRevW.Body.String())
	}

	now := time.Now().UTC()
	textureRun := &types.RunRecord{
		RunID:        "texture-requester-findings-wake",
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
		AgentID:      "texture:" + docID,
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		ChannelID:    docID,
		State:        types.RunCompleted,
		Prompt:       "Own the document",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataChannelID:    docID,
			runMetadataAgentID:      "texture:" + docID,
			"doc_id":                docID,
		},
	}
	if err := s.CreateRun(context.Background(), *textureRun); err != nil {
		t.Fatalf("create texture requester run: %v", err)
	}
	researcherRun, err := rt.StartCoagentRun(context.Background(), textureRun.RunID, "Research the update", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start researcher run: %v", err)
	}

	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	raw, err := researcherRegistry.Execute(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(researcherRun)), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"finding stream 001",
		"agent_id":"texture:`+docID+`",
		"claims":[{"text":"A new release landed this week.","source_ids":["src-release"]}],
		"sources":[
			{
				"kind":"web_page",
				"source_id":"src-release",
				"target":{"uri":"https://example.com/release","title":"Release notes"},
				"selectors":[{"kind":"text_quote","quote":"The release notes describe the new capabilities."}]
			}
		],
		"notes":["Prefer a brief update in the next draft."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent: %v", err)
	}
	var findingResp struct {
		UpdateID string `json:"update_id"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &findingResp); err != nil {
		t.Fatalf("decode update_coagent: %v", err)
	}
	if findingResp.Status != "submitted" {
		t.Fatalf("update_coagent status = %q, want submitted", findingResp.Status)
	}
	if findingResp.UpdateID == "" {
		t.Fatalf("update_coagent returned empty update_id")
	}
	findingUpdate, err := s.GetWorkerUpdate(context.Background(), "user-1", findingResp.UpdateID)
	if err != nil {
		t.Fatalf("get coagent update %s: %v", findingResp.UpdateID, err)
	}
	if len(findingUpdate.Packet.Sources) != 1 {
		t.Fatalf("coagent update sources = %+v, want one durable source handle", findingUpdate.Packet.Sources)
	}
	evidenceID := findingUpdate.Packet.Sources[0].SourceID

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
		if agentProfileForRun(&runs[i]) == AgentProfileTexture &&
			metadataStringValue(runs[i].Metadata, "request_source") == "update_coagent" {
			wakeRun = &runs[i]
			break
		}
	}
	if wakeRun == nil {
		t.Fatalf("expected findings-driven texture wake run on channel %s, got %+v", docID, runs)
	}
	// Fresh Texture update wakes use the same durable mailbox-turn substrate as
	// resident and sleeping actors. The packet must not be smuggled in by
	// prepending cold prompt context before run memory initializes.
	if got := metadataStringValue(wakeRun.Metadata, "request_source"); got != "update_coagent" {
		t.Fatalf("wake run request_source = %q, want update_coagent", got)
	}
	if shouldPrependInitialCoagentUpdates(wakeRun) {
		t.Fatalf("Texture wake run should not be cold-prepend eligible; metadata=%+v", wakeRun.Metadata)
	}
	if !shouldAppendInitialCoagentMailboxTurns(wakeRun) {
		t.Fatalf("Texture wake run should append initial mailbox turns; metadata=%+v", wakeRun.Metadata)
	}
	memoryEntries, err := s.ListRunMemoryEntries(context.Background(), "user-1", wakeRun.RunID)
	if err != nil {
		t.Fatalf("list wake run memory: %v", err)
	}
	foundActivationMailboxTurn := false
	activationMailboxText := ""
	for _, entry := range memoryEntries {
		if entry.Kind != types.RunMemoryEntryMessage {
			continue
		}
		var msg struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}
		for _, part := range msg.Content {
			if strings.Contains(part.Text, "activation mailbox turn") &&
				strings.Contains(part.Text, `"delivery_phase":"activation_mailbox_turn"`) {
				foundActivationMailboxTurn = true
				activationMailboxText = part.Text
				break
			}
		}
		if foundActivationMailboxTurn {
			break
		}
	}
	if !foundActivationMailboxTurn {
		t.Fatalf("wake run memory missing activation mailbox turn: %+v", memoryEntries)
	}
	if !strings.Contains(activationMailboxText, evidenceID) {
		t.Fatalf("activation mailbox turn missing durable evidence handle %s: %s", evidenceID, activationMailboxText)
	}
	// Fix #2: a grounded integrate wake must write a revision before it can end
	// or take a terminal delegation action. The initial tool choice is "required"
	// (any tool, but must call one) so the model can choose patch_texture for
	// small deltas or rewrite_texture for full-document drafts.
	if got := initialTextureToolChoice(wakeRun); got != "required" {
		t.Fatalf("initialTextureToolChoice(wake run) = %q, want \"required\"", got)
	}
	if !strings.Contains(activationMailboxText, `"title":"Release notes"`) {
		t.Fatalf("activation mailbox turn missing packet source title: %s", activationMailboxText)
	}
}

func TestTextureRevisionRunParksAndConsumesUpdateWithoutColdWake(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider := &textureParkResidentProvider{Provider: NewStubProvider(time.Millisecond)}
	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	rt.cfg.TextureActorParkIdle = 2 * time.Second
	docID, _ := createDocWithUserRevision(t, h)
	doc, err := s.GetDocument(ctx, docID, "user-1")
	if err != nil {
		t.Fatalf("get doc: %v", err)
	}

	textureRun, err := rt.submitTextureAgentRevisionRun(ctx, doc, "user-1", textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "Draft immediately, then remain available for follow-up findings.",
	}, 0)
	if err != nil {
		t.Fatalf("submit texture revision run: %v", err)
	}
	if !metadataBoolValue(textureRun.Metadata, "actor_park_on_idle") {
		t.Fatalf("actor_park_on_idle = %v, want true; metadata=%+v", textureRun.Metadata["actor_park_on_idle"], textureRun.Metadata)
	}
	if got := metadataIntValue(textureRun.Metadata, "actor_park_idle_seconds"); got != 2 {
		t.Fatalf("actor_park_idle_seconds = %d, want 2", got)
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 2, 5*time.Second)
	if !revisionContentsContain(revs, "Model-prior resident draft before worker evidence.") {
		t.Fatalf("missing resident V1 revision; revisions=%+v", revs)
	}
	storedRun, err := rt.GetRun(ctx, textureRun.RunID, "user-1")
	if err != nil {
		t.Fatalf("get texture run: %v", err)
	}
	if storedRun.State != types.RunRunning {
		t.Fatalf("texture run state after V1 = %q, want running parked actor", storedRun.State)
	}

	researcherRun := types.RunRecord{
		RunID:        "researcher-parked-texture",
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
		AgentID:      "researcher:parked-texture",
		AgentProfile: AgentProfileResearcher,
		AgentRole:    AgentProfileResearcher,
		ChannelID:    docID,
		State:        types.RunRunning,
		Prompt:       "Send one grounded update.",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Metadata: map[string]any{
			runMetadataAgentID:      "researcher:parked-texture",
			runMetadataAgentProfile: AgentProfileResearcher,
			runMetadataAgentRole:    AgentProfileResearcher,
			runMetadataChannelID:    docID,
			runMetadataTrajectoryID: textureRun.RunID,
		},
	}
	if err := s.CreateRun(ctx, researcherRun); err != nil {
		t.Fatalf("create researcher run: %v", err)
	}
	raw, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&researcherRun)), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"parked finding 001",
		"agent_id":"texture:`+docID+`",
		"claims":[{"text":"A new grounded finding arrived from the parked resident test.","source_ids":["src-parked"]}],
		"sources":[
			{
				"kind":"web_page",
				"source_id":"src-parked",
				"target":{"uri":"https://example.com/parked","title":"Parked update evidence"},
				"selectors":[{"kind":"text_quote","quote":"A new grounded finding arrived."}]
			}
		],
		"notes":["The existing resident Texture actor should consume this without a cold wake run."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent: %v", err)
	}
	var updateResp struct {
		Status   string `json:"status"`
		UpdateID string `json:"update_id"`
	}
	if err := json.Unmarshal([]byte(raw), &updateResp); err != nil {
		t.Fatalf("decode update_coagent response: %v", err)
	}
	if updateResp.Status != "submitted" {
		t.Fatalf("update_coagent status = %q, want submitted", updateResp.Status)
	}
	if !strings.HasPrefix(updateResp.UpdateID, "upd-") {
		t.Fatalf("update_coagent update_id = %q, want runtime-owned id", updateResp.UpdateID)
	}

	revs = waitForRevisionCount(t, s, docID, "user-1", 3, 5*time.Second)
	if !revisionContentsContain(revs, "Grounded update from parked resident actor.") {
		t.Fatalf("missing parked resident V2 revision; revisions=%+v", revs)
	}
	if ids := metadataStringSlice(textureRun.Metadata["worker_update_ids"]); !containsString(ids, updateResp.UpdateID) {
		t.Fatalf("resident run worker_update_ids = %+v, want %s", ids, updateResp.UpdateID)
	}
	if !metadataBoolValue(textureRun.Metadata, runMetadataWorkerUpdatesInjected) {
		t.Fatalf("resident run %s missing %s metadata: %+v", textureRun.RunID, runMetadataWorkerUpdatesInjected, textureRun.Metadata)
	}
	waitForNoPendingWorkerUpdates(t, s, "user-1", "texture:"+docID, 2*time.Second)

	runs, err := s.ListRunsByChannel(ctx, "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list runs by channel: %v", err)
	}
	var textureRevisionRuns []types.RunRecord
	for _, run := range runs {
		if agentProfileForRun(&run) == AgentProfileTexture && isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) {
			textureRevisionRuns = append(textureRevisionRuns, run)
		}
	}
	if len(textureRevisionRuns) != 1 || textureRevisionRuns[0].RunID != textureRun.RunID {
		t.Fatalf("texture revision runs = %+v, want only resident run %s", textureRevisionRuns, textureRun.RunID)
	}
	if len(provider.choices) < 2 || provider.choices[0] != "" || provider.choices[1] != "" {
		t.Fatalf("provider choices = %#v, want unconstrained first patch and parked follow-up", provider.choices)
	}
}

func TestTextureIdlePassivatesAndResumesSameRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider := &textureParkResidentProvider{Provider: NewStubProvider(time.Millisecond)}
	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	rt.cfg.TextureActorParkIdle = time.Second
	docID, _ := createDocWithUserRevision(t, h)
	doc, err := s.GetDocument(ctx, docID, "user-1")
	if err != nil {
		t.Fatalf("get doc: %v", err)
	}

	textureRun, err := rt.submitTextureAgentRevisionRun(ctx, doc, "user-1", textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "Draft immediately, then sleep until follow-up findings arrive.",
	}, 0)
	if err != nil {
		t.Fatalf("submit texture revision run: %v", err)
	}
	revs := waitForRevisionCount(t, s, docID, "user-1", 2, 5*time.Second)
	if !revisionContentsContain(revs, "Model-prior resident draft before worker evidence.") {
		t.Fatalf("missing resident V1 revision; revisions=%+v", revs)
	}
	sleepingRun := waitForStoredRunState(t, s, textureRun.RunID, types.RunPassivated, 5*time.Second)
	if got := metadataStringValue(sleepingRun.Metadata, "actor_sleep_state"); got != "idle" {
		t.Fatalf("actor_sleep_state = %q, want idle; metadata=%+v", got, sleepingRun.Metadata)
	}
	mutation, err := s.GetAgentMutationByRun(ctx, textureRun.RunID)
	if err != nil {
		t.Fatalf("get sleeping mutation: %v", err)
	}
	if mutation == nil || mutation.State != "sleeping" {
		t.Fatalf("sleeping mutation = %+v, want state sleeping", mutation)
	}

	researcherRun := types.RunRecord{
		RunID:        "researcher-idle-resume-texture",
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
		AgentID:      "researcher:idle-resume-texture",
		AgentProfile: AgentProfileResearcher,
		AgentRole:    AgentProfileResearcher,
		ChannelID:    docID,
		State:        types.RunRunning,
		Prompt:       "Send one grounded update after Texture slept.",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Metadata: map[string]any{
			runMetadataAgentID:      "researcher:idle-resume-texture",
			runMetadataAgentProfile: AgentProfileResearcher,
			runMetadataAgentRole:    AgentProfileResearcher,
			runMetadataChannelID:    docID,
			runMetadataTrajectoryID: textureRun.RunID,
		},
	}
	if err := s.CreateRun(ctx, researcherRun); err != nil {
		t.Fatalf("create researcher run: %v", err)
	}
	raw, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&researcherRun)), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"idle resume finding",
		"agent_id":"texture:`+docID+`",
		"claims":[{"text":"A new grounded finding arrived from the parked resident test.","source_ids":["src-idle-resume"]}],
		"sources":[
			{
				"kind":"web_page",
				"source_id":"src-idle-resume",
				"target":{"uri":"https://example.com/idle-resume","title":"Idle resume evidence"},
				"selectors":[{"kind":"text_quote","quote":"A new grounded finding arrived."}]
			}
		],
		"notes":["The sleeping Texture actor should resume the same run and consume this without a cold wake run."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent after sleep: %v", err)
	}
	var updateResp struct {
		Status   string `json:"status"`
		UpdateID string `json:"update_id"`
	}
	if err := json.Unmarshal([]byte(raw), &updateResp); err != nil {
		t.Fatalf("decode update_coagent response: %v", err)
	}
	if updateResp.Status != "submitted" || !strings.HasPrefix(updateResp.UpdateID, "upd-") {
		t.Fatalf("update_coagent response = %+v, want submitted runtime-owned id", updateResp)
	}

	revs = waitForRevisionCount(t, s, docID, "user-1", 3, 5*time.Second)
	if !revisionContentsContain(revs, "Grounded update from parked resident actor.") {
		t.Fatalf("missing resumed resident revision; revisions=%+v", revs)
	}
	resumedRun := waitForStoredRunState(t, s, textureRun.RunID, types.RunPassivated, 5*time.Second)
	if ids := metadataStringSlice(resumedRun.Metadata["worker_update_ids"]); !containsString(ids, updateResp.UpdateID) {
		t.Fatalf("resumed run worker_update_ids = %+v, want %s", ids, updateResp.UpdateID)
	}
	if !metadataBoolValue(resumedRun.Metadata, "actor_reactivated_from_passivated") {
		t.Fatalf("resumed run missing same-run reactivation marker: %+v", resumedRun.Metadata)
	}

	runs, err := s.ListRunsByChannel(ctx, "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list runs by channel: %v", err)
	}
	var textureRevisionRuns []types.RunRecord
	for _, run := range runs {
		if agentProfileForRun(&run) == AgentProfileTexture && isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) {
			textureRevisionRuns = append(textureRevisionRuns, run)
		}
	}
	if len(textureRevisionRuns) != 1 || textureRevisionRuns[0].RunID != textureRun.RunID {
		t.Fatalf("texture revision runs = %+v, want only sleeping/resumed run %s", textureRevisionRuns, textureRun.RunID)
	}
	memoryEntries, err := s.ListRunMemoryEntries(ctx, "user-1", textureRun.RunID)
	if err != nil {
		t.Fatalf("list run memory: %v", err)
	}
	foundMailboxTurn := false
	for _, entry := range memoryEntries {
		if entry.Kind == types.RunMemoryEntryMessage && strings.Contains(string(entry.Message), "Choir coagent update packet") {
			foundMailboxTurn = true
			break
		}
	}
	if !foundMailboxTurn {
		t.Fatalf("run memory missing resumed mailbox turn: %+v", memoryEntries)
	}
	mutation, err = s.GetAgentMutationByRun(ctx, textureRun.RunID)
	if err != nil {
		t.Fatalf("get resumed sleeping mutation: %v", err)
	}
	if mutation == nil || mutation.State != "sleeping" {
		t.Fatalf("resumed mutation = %+v, want state sleeping", mutation)
	}
}

func TestTextureWakeStartsIntegrationForCompletedThreadHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider := newTextureEditToolProvider(textureReplaceAllResult("completed thread update integrated"))
	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	docID, _ := createDocWithUserRevision(t, h)
	doc, err := s.GetDocument(ctx, docID, "user-1")
	if err != nil {
		t.Fatalf("get doc: %v", err)
	}

	now := time.Now().UTC()
	completed := types.RunRecord{
		RunID:        "texture-existing-thread-history",
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
		AgentID:      "texture:" + docID,
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		ChannelID:    docID,
		State:        types.RunCompleted,
		Prompt:       "historical Texture thread activation",
		Result:       "delegated and ended before durable parking existed",
		CreatedAt:    now,
		UpdatedAt:    now,
		FinishedAt:   &now,
		Metadata: map[string]any{
			"type":                    textureAgentRevisionTaskType,
			runMetadataAgentID:        "texture:" + docID,
			runMetadataAgentProfile:   AgentProfileTexture,
			runMetadataAgentRole:      AgentProfileTexture,
			runMetadataChannelID:      docID,
			"doc_id":                  docID,
			"current_revision_id":     doc.CurrentRevisionID,
			"replacement_wake_legacy": true,
		},
	}
	if err := s.CreateRun(ctx, completed); err != nil {
		t.Fatalf("create completed texture thread: %v", err)
	}
	update := types.CoagentSourcePacket{
		UpdateID:      "update-existing-thread-history",
		OwnerID:       "user-1",
		AgentID:       "researcher:existing-thread",
		TargetAgentID: "texture:" + docID,
		ChannelID:     docID,
		TrajectoryID:  completed.RunID,
		Role:          AgentProfileResearcher,
		Packet:        testCoagentUpdatePacket("evidence_update", "existing thread update should not create a replacement run"),
		Content:       "Existing thread update should wait for a resident or sleeping Texture actor.",
		CreatedAt:     now.Add(time.Millisecond),
	}
	message := types.ChannelMessage{
		ChannelID:    update.ChannelID,
		FromAgentID:  update.AgentID,
		ToAgentID:    update.TargetAgentID,
		TrajectoryID: update.TrajectoryID,
		Role:         update.Role,
		Content:      update.Content,
		Timestamp:    update.CreatedAt,
	}
	if _, _, err := s.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("dispatch update: %v", err)
	}
	rec, err := rt.reconcileTextureAgentWake(ctx, "user-1", docID)
	if err != nil {
		t.Fatalf("reconcile texture wake: %v", err)
	}
	if rec == nil {
		t.Fatalf("reconcile returned no activation for completed thread history")
	}
	terminal := waitForRuntimeRunTerminal(t, rt, rec.RunID, "user-1", 5*time.Second)
	if terminal.State != types.RunCompleted {
		t.Fatalf("wake activation state = %q error=%q", terminal.State, terminal.Error)
	}
	runs, err := s.ListRunsByChannel(ctx, "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs: %v", err)
	}
	var textureRevisionRuns []types.RunRecord
	for _, run := range runs {
		if agentProfileForRun(&run) == AgentProfileTexture && isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) {
			textureRevisionRuns = append(textureRevisionRuns, run)
		}
	}
	if len(textureRevisionRuns) != 2 {
		t.Fatalf("texture revision runs = %+v, want historical thread plus D9 packet integration run", textureRevisionRuns)
	}
	if ids := metadataStringSlice(terminal.Metadata["worker_update_ids"]); !containsString(ids, update.UpdateID) {
		t.Fatalf("wake activation worker_update_ids = %+v, want %s", ids, update.UpdateID)
	}
	backlog, err := s.ListCoagentMailboxBacklog(ctx, "user-1", "texture:"+docID, 10)
	if err != nil {
		t.Fatalf("list coagent backlog: %v", err)
	}
	if len(backlog) != 0 {
		t.Fatalf("backlog = %+v, want pending packet consumed", backlog)
	}
}

func TestSubmitWorkerUpdateWakeUsesSameDebouncedPath(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Integrated structured super update into the next revision."))
	provider.delay = 500 * time.Millisecond
	clock := &fakeTextureWakeClock{}

	h, s, rt := textureAPISetupWithProviderAndOptions(t, provider, true, withTextureWakeAfterFuncForTest(clock.afterFunc))
	docID, _ := createDocWithUserRevision(t, h)

	userRevReq := textureCreateRevisionRequest{
		Content:     "Original draft.\n\nNeed execution artifacts and verification results.",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
	}
	userRevReqBody := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", userRevReq)
	userRevW := httptest.NewRecorder()
	h.HandleTextureRevisions(userRevW, userRevReqBody)
	if userRevW.Code != http.StatusCreated {
		t.Fatalf("second user revision: status = %d, want %d; body: %s", userRevW.Code, http.StatusCreated, userRevW.Body.String())
	}

	textureRun, err := rt.StartRunWithMetadata(context.Background(), "Own the document", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileTexture,
		runMetadataAgentRole:    AgentProfileTexture,
		runMetadataChannelID:    docID,
		runMetadataAgentID:      "texture:" + docID,
		"doc_id":                docID,
	})
	if err != nil {
		t.Fatalf("start texture run: %v", err)
	}
	waitForRunRunning(t, rt, textureRun.RunID, "user-1", 5*time.Second)
	superRun, err := rt.StartCoagentRun(context.Background(), textureRun.RunID, "Build and verify a toy artifact", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start super run: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := superRegistry.Execute(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(superRun)), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"execution_result",
		"summary":"super artifact 001",
		"agent_id":"texture:`+docID+`",
		"sources":[
			{"source_id":"src-artifact","kind":"file_artifact","target":{"uri":"file_artifact:artifacts/evolution-ca.html"}},
			{"source_id":"src-test","kind":"test_run","target":{"uri":"test_run:node artifacts/evolution-ca.verify.js passed"}}
		],
		"actions":[{"type":"revise_texture","objective":"Mention the generated visualization and verification result in the next version."}]
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
	if !containsString(coagentPacketSourceURIs(update.Packet), "file_artifact:artifacts/evolution-ca.html") {
		t.Fatalf("sources missing artifact: %+v", update.Packet.Sources)
	}
	if !strings.Contains(strings.Join(coagentPacketSourceURIs(update.Packet, "test_run"), "\n"), "evolution") {
		t.Fatalf("sources missing test: %+v", update.Packet.Sources)
	}

	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs: %v", err)
	}
	var wakeRun *types.RunRecord
	for i := range runs {
		if agentProfileForRun(&runs[i]) == AgentProfileTexture && runs[i].RequestedByRunID == superRun.RunID {
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

func TestTextureUpdateCoagentDuringActiveRevisionTriggersSameRunFollowUp(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Integrated content after the run completed."))
	provider.delay = 300 * time.Millisecond

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	rt.cfg.TextureActorParkIdle = time.Second
	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Produce the next draft now"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var initialResp textureAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&initialResp); err != nil {
		t.Fatalf("decode initial agent revision response: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	initialRunStarted := false
	for time.Now().Before(deadline) {
		rec, err := rt.GetRun(context.Background(), initialResp.RunID, "user-1")
		if err != nil {
			t.Fatalf("get initial texture run: %v", err)
		}
		if rec.State == types.RunRunning {
			initialRunStarted = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !initialRunStarted {
		t.Fatalf("initial texture run never reached running state before posting the late worker message")
	}

	researchRun, err := rt.StartRunWithMetadata(context.Background(), "Send one late finding", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start research run: %v", err)
	}
	raw, err := rt.ToolRegistryForProfile(AgentProfileResearcher).Execute(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(researchRun)), "update_coagent", json.RawMessage(`{
		"schema_version":"coagent_source_packet.v1",
		"kind":"evidence_update",
		"summary":"late finding",
		"agent_id":"texture:`+docID+`",
		"claims":[{"text":"Late finding: a sourced correction arrived while the texture run was already active.","source_ids":["src-late"]}],
		"sources":[
			{
				"kind":"web_page",
				"source_id":"src-late",
				"target":{"uri":"https://example.com/late","title":"Late finding evidence"},
				"selectors":[{"kind":"text_quote","quote":"Late finding: a sourced correction arrived while the texture run was already active."}]
			}
		],
		"notes":["The active Texture run should append this as a mailbox turn and continue in the same loop."]
	}`))
	if err != nil {
		t.Fatalf("update_coagent late worker message: %v", err)
	}
	var updateResp struct {
		Status   string `json:"status"`
		UpdateID string `json:"update_id"`
		Cursor   int64  `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(raw), &updateResp); err != nil {
		t.Fatalf("decode update_coagent response: %v", err)
	}
	if updateResp.Status != "submitted" || !strings.HasPrefix(updateResp.UpdateID, "upd-") || updateResp.Cursor == 0 {
		t.Fatalf("update_coagent response = %+v, want submitted runtime id and cursor", updateResp)
	}

	revs := waitForRevisionCount(t, s, docID, "user-1", 3, 8*time.Second)
	var appAgentContents []string
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent {
			appAgentContents = append(appAgentContents, rev.Content)
		}
	}
	if len(appAgentContents) == 0 {
		t.Fatalf("expected at least one appagent revision, got revisions: %+v", revs)
	}
	for _, content := range appAgentContents {
		if !strings.Contains(content, "Integrated content after the run completed.") {
			t.Fatalf("unexpected appagent revision content: %+v", appAgentContents)
		}
	}
	sleepingRun := waitForStoredRunState(t, s, initialResp.RunID, types.RunPassivated, 8*time.Second)

	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-1", docID, 20)
	if err != nil {
		t.Fatalf("list channel runs: %v", err)
	}
	var textureRevisionRuns []types.RunRecord
	for _, run := range runs {
		if agentProfileForRun(&run) == AgentProfileTexture && isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) {
			textureRevisionRuns = append(textureRevisionRuns, run)
		}
	}
	foundInitialRun := false
	for _, run := range textureRevisionRuns {
		if run.RunID == initialResp.RunID {
			foundInitialRun = true
		}
	}
	if !foundInitialRun {
		t.Fatalf("texture revision runs = %+v, want original actor run %s to remain in lineage", textureRevisionRuns, initialResp.RunID)
	}
	if ids := metadataStringSlice(sleepingRun.Metadata["worker_update_ids"]); !containsString(ids, updateResp.UpdateID) {
		t.Fatalf("same-run worker_update_ids = %+v, want %s", ids, updateResp.UpdateID)
	}

	var checkpoint *store.TextureControllerCheckpoint
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		checkpoint, err = s.GetTextureControllerCheckpoint(context.Background(), docID, "user-1")
		if err != nil {
			t.Fatalf("get controller checkpoint: %v", err)
		}
		if checkpoint != nil && checkpoint.IntegratedMessageSeq >= updateResp.Cursor {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if checkpoint == nil || checkpoint.IntegratedMessageSeq < updateResp.Cursor {
		t.Fatalf("checkpoint = %+v, want integrated seq >= %d", checkpoint, updateResp.Cursor)
	}

	mutation, err := s.GetAgentMutationByRun(context.Background(), initialResp.RunID)
	if err != nil {
		t.Fatalf("get sleeping mutation: %v", err)
	}
	if mutation == nil || mutation.State != "sleeping" {
		t.Fatalf("mutation = %+v, want sleeping resident mutation", mutation)
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

	prompt := buildAgentRevisionRequest(current, nil, nil, textureAgentRevisionRequest{
		Intent: "integrate_worker_findings",
	}, "", true, recent, nil)

	for _, want := range []string{
		"At least one recent worker message says a delegated worker is still active",
		"when Texture decides continuation is needed, request_super_execution",
		"continue the existing worker_run_id",
		"not start a duplicate worker",
		"Texture must not directly control worker/vsuper/co-super runs",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("active-worker texture prompt missing %q:\n%s", want, prompt)
		}
	}
	assertNoForcedSemanticDelegation(t, prompt)
}

func TestRestartRecoveryReactivatesInterruptedTextureRun(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(os.TempDir(), "go-choir-m3-texture-test")
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
		SandboxID: "sandbox-texture-test",
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
		ToAgentID:    "texture:" + doc.DocID,
		Role:         "researcher",
		Content:      "Durable finding: the corrected fact landed while the sandbox was about to restart.",
		Timestamp:    now.Add(3 * time.Second),
	}
	update := types.CoagentSourcePacket{
		UpdateID:      "update-restart-rewarm",
		OwnerID:       "user-1",
		AgentID:       message.FromAgentID,
		TargetAgentID: message.ToAgentID,
		ChannelID:     message.ChannelID,
		TrajectoryID:  message.TrajectoryID,
		Role:          message.Role,
		Packet:        newCoagentPacket("evidence_update", "restart rewarm finding", []types.CoagentPacketClaim{coagentClaim(message.Content)}, nil, nil, nil, nil),
		Content:       message.Content,
		CreatedAt:     message.Timestamp,
	}
	storedUpdate, _, err := s1.DispatchWorkerUpdate(ctx, update, message)
	if err != nil {
		t.Fatalf("dispatch worker update: %v", err)
	}
	message.Seq = storedUpdate.MessageSeq

	interruptedRun := types.RunRecord{
		RunID:            "texture-interrupted-restart",
		AgentID:          "texture:" + doc.DocID,
		AgentProfile:     AgentProfileTexture,
		AgentRole:        AgentProfileTexture,
		OwnerID:          "user-1",
		SandboxID:        "sandbox-texture-test",
		ChannelID:        doc.DocID,
		State:            types.RunRunning,
		Prompt:           "Integrate the durable finding",
		RequestedByRunID: researchRun.RunID,
		CreatedAt:        now.Add(4 * time.Second),
		UpdatedAt:        now.Add(4 * time.Second),
		Metadata: map[string]any{
			"type":                  "texture_agent_revision",
			"doc_id":                doc.DocID,
			"current_revision_id":   userRev.RevisionID,
			"scheduled_message_seq": message.Seq,
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataChannelID:    doc.DocID,
			runMetadataAgentID:      "texture:" + doc.DocID,
			runMetadataTrajectoryID: message.TrajectoryID,
		},
	}
	if err := s1.CreateRun(ctx, interruptedRun); err != nil {
		t.Fatalf("create interrupted texture run: %v", err)
	}
	if _, err := s1.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:   interruptedRun.RunID,
		OwnerID: interruptedRun.OwnerID,
		AgentID: interruptedRun.AgentID,
		Kind:    types.RunMemoryEntryMessage,
		Role:    "assistant",
		Message: json.RawMessage(`{"role":"assistant","content":"Stored V1 before restart; retain this actor context."}`),
	}); err != nil {
		t.Fatalf("append interrupted run memory: %v", err)
	}
	if err := s1.AppendEvent(ctx, &types.EventRecord{
		RunID:        interruptedRun.RunID,
		AgentID:      interruptedRun.AgentID,
		ChannelID:    interruptedRun.ChannelID,
		OwnerID:      interruptedRun.OwnerID,
		TrajectoryID: message.TrajectoryID,
		Timestamp:    now.Add(5 * time.Second),
		Kind:         types.EventRunProgress,
		Phase:        "tool_loop_budget_usage",
		Payload:      json.RawMessage(`{"provider_calls":3,"input_tokens":120,"output_tokens":40,"total_tokens":160}`),
	}); err != nil {
		t.Fatalf("append interrupted budget event: %v", err)
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
	provider := newTextureEditToolProvider(textureReplaceAllResult("Recovered after restart and integrated the durable finding."))
	provider.delay = 20 * time.Millisecond
	rt := New(Config{
		SandboxID:           "sandbox-texture-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     2 * time.Second,
		SupervisionInterval: 5 * time.Second,
		TextureWakeDebounce: 50 * time.Millisecond,
	}, s2, events.NewEventBus(), provider)
	setTestDispatch(rt, s2)
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
	waitForTextureQuiescent(t, rt, s2, "user-1", doc.DocID, uint64(message.Seq), 5*time.Second)
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
	if gotInterrupted.State != types.RunCompleted {
		t.Fatalf("interrupted run state = %q, want %q", gotInterrupted.State, types.RunCompleted)
	}
	if gotInterrupted.Error != "" {
		t.Fatalf("interrupted run error = %q, want empty", gotInterrupted.Error)
	}
	if !metadataBoolValue(gotInterrupted.Metadata, "actor_reactivated_from_passivated") ||
		!metadataBoolValue(gotInterrupted.Metadata, "actor_reactivate_existing_memory") {
		t.Fatalf("interrupted run metadata missing same-run reactivation markers: %+v", gotInterrupted.Metadata)
	}

	mutation, err := s2.GetAgentMutationByRun(ctx, interruptedRun.RunID)
	if err != nil {
		t.Fatalf("get interrupted mutation: %v", err)
	}
	if mutation == nil || mutation.State != "completed" {
		t.Fatalf("expected interrupted mutation to complete after same-run recovery, got %+v", mutation)
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
	var textureRevisionRuns []types.RunRecord
	for i := range runs {
		if agentProfileForRun(&runs[i]) == AgentProfileTexture && isTextureAgentRevisionTaskType(metadataStringValue(runs[i].Metadata, "type")) {
			textureRevisionRuns = append(textureRevisionRuns, runs[i])
		}
	}
	if len(textureRevisionRuns) != 1 || textureRevisionRuns[0].RunID != interruptedRun.RunID {
		t.Fatalf("texture revision runs after restart = %+v, want only same reactivated run %s", textureRevisionRuns, interruptedRun.RunID)
	}
	recoveredRun := textureRevisionRuns[0]
	if got := metadataStringValue(recoveredRun.Metadata, "actor_resume_source_loop_id"); got != interruptedRun.RunID {
		t.Fatalf("same-run resume source = %q, want %q; metadata=%+v", got, interruptedRun.RunID, recoveredRun.Metadata)
	}
	if got := metadataIntValue(recoveredRun.Metadata, "actor_budget_spent_provider_calls"); got != 3 {
		t.Fatalf("same-run spent provider calls = %d, want 3; metadata=%+v", got, recoveredRun.Metadata)
	}
	if got := metadataIntValue(recoveredRun.Metadata, "actor_budget_spent_input_tokens"); got != 120 {
		t.Fatalf("same-run spent input tokens = %d, want 120; metadata=%+v", got, recoveredRun.Metadata)
	}
	if got := metadataIntValue(recoveredRun.Metadata, "actor_budget_spent_output_tokens"); got != 40 {
		t.Fatalf("same-run spent output tokens = %d, want 40; metadata=%+v", got, recoveredRun.Metadata)
	}
	memoryEntries, err := s2.ListRunMemoryEntries(ctx, "user-1", recoveredRun.RunID)
	if err != nil {
		t.Fatalf("list recovered run memory: %v", err)
	}
	if len(memoryEntries) < 2 || memoryEntries[0].Kind != types.RunMemoryEntryMessage ||
		!strings.Contains(string(memoryEntries[0].Message), "Stored V1 before restart") {
		t.Fatalf("recovered memory entries = %+v, want prior actor message preserved first", memoryEntries)
	}
	foundMailboxTurn := false
	for _, entry := range memoryEntries {
		if entry.Kind == types.RunMemoryEntryMessage && strings.Contains(string(entry.Message), "Choir coagent update packet") {
			foundMailboxTurn = true
			break
		}
	}
	if !foundMailboxTurn {
		t.Fatalf("recovered memory entries missing appended mailbox turn: %+v", memoryEntries)
	}
	if ids := metadataStringSlice(recoveredRun.Metadata["worker_update_ids"]); !containsString(ids, storedUpdate.UpdateID) {
		t.Fatalf("same-run worker_update_ids = %+v, want %s", ids, storedUpdate.UpdateID)
	}
	checkpoint, err := s2.GetTextureControllerCheckpoint(ctx, doc.DocID, "user-1")
	if err != nil {
		t.Fatalf("get controller checkpoint after recovery: %v", err)
	}
	if checkpoint == nil || checkpoint.IntegratedMessageSeq != message.Seq {
		t.Fatalf("checkpoint after recovery = %+v, want integrated_message_seq=%d", checkpoint, message.Seq)
	}
}

func TestHandleTestTextureWorkerUpdateUsesStructuredToolPath(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Browser test structured worker update revision."))

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	rt.cfg.EnableTestAPIs = true

	docID, _ := createDocWithUserRevision(t, h)

	revReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Write the first draft"})
	revW := httptest.NewRecorder()
	h.HandleTextureAgentRevision(revW, revReq)
	if revW.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", revW.Code, http.StatusAccepted, revW.Body.String())
	}
	var revResp textureAgentRevisionResponse
	if err := json.NewDecoder(revW.Body).Decode(&revResp); err != nil {
		t.Fatalf("decode agent revision response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, revResp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("agent revision state = %q, want %q", state, types.RunCompleted)
	}

	req := textureRequest(t, http.MethodPost, "/api/test/texture/worker-update", map[string]any{
		"doc_id":         docID,
		"role":           "super",
		"schema_version": types.CoagentSourcePacketSchemaV1,
		"kind":           "execution_result",
		"summary":        "browser-worker-update-001",
		"sources": []map[string]any{
			{
				"source_id": "src-artifact",
				"kind":      "file_artifact",
				"target": map[string]any{
					"uri": "file_artifact:artifacts/evolution-ca.html",
				},
			},
			{
				"source_id": "src-test",
				"kind":      "test_run",
				"target": map[string]any{
					"uri": "test_run:node artifacts/evolution-ca.verify.js passed",
				},
			},
			{
				"source_id": "src-evidence",
				"kind":      "evidence",
				"target": map[string]any{
					"uri": "evidence:evidence-browser-001",
				},
			},
		},
		"actions": []map[string]any{{
			"type":      "revise_texture",
			"objective": "Mention the verified visualization in the next draft.",
		}},
	})
	w := httptest.NewRecorder()
	h.HandleTestTextureWorkerUpdate(w, req)
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
	if workerRun.RequestedByRunID != revResp.RunID {
		t.Fatalf("worker loop parent = %q, want texture run %q", workerRun.RequestedByRunID, revResp.RunID)
	}
	textureRun, err := s.GetRun(context.Background(), revResp.RunID)
	if err != nil {
		t.Fatalf("get texture loop: %v", err)
	}

	update, err := s.GetWorkerUpdate(context.Background(), "user-1", "browser-worker-update-001")
	if err != nil {
		t.Fatalf("get worker update: %v", err)
	}
	if update.Role != AgentProfileSuper || len(coagentPacketSourceURIs(update.Packet, "file_artifact")) != 1 || len(coagentPacketSourceURIs(update.Packet, "test_run")) != 1 || len(update.Packet.Actions) == 0 {
		t.Fatalf("unexpected structured update: %+v", update)
	}
	if update.TrajectoryID != trajectoryIDForRun(&workerRun) || update.TrajectoryID != trajectoryIDForRun(&textureRun) {
		t.Fatalf("worker update trajectory = %q, worker trajectory = %q, texture trajectory = %q", update.TrajectoryID, trajectoryIDForRun(&workerRun), trajectoryIDForRun(&textureRun))
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

func TestTextureAgentRevisionInheritsConductorTrajectoryFromRevisionMetadata(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider(textureReplaceAllResult("Conductor-linked texture revision."))

	h, s, _ := textureAPISetupWithProvider(t, provider, true)
	ctx := context.Background()

	now := time.Now().UTC()
	conductorRun := types.RunRecord{
		RunID:        "conductor-parent-001",
		AgentID:      "conductor:test",
		ChannelID:    "conductor-parent-001",
		AgentProfile: AgentProfileConductor,
		AgentRole:    AgentProfileConductor,
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
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
	revReq := textureCreateRevisionRequest{
		Content:          "User refined the conductor-framed working document.",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		Metadata:         metadata,
		ParentRevisionID: baseRevisionID,
	}
	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", revReq)
	w := httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create conductor-linked revision: status = %d, body: %s", w.Code, w.Body.String())
	}

	agentReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"intent": "revise"})
	agentW := httptest.NewRecorder()
	h.HandleTextureAgentRevision(agentW, agentReq)
	if agentW.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", agentW.Code, http.StatusAccepted, agentW.Body.String())
	}
	var resp textureAgentRevisionResponse
	if err := json.NewDecoder(agentW.Body).Decode(&resp); err != nil {
		t.Fatalf("decode agent revision response: %v", err)
	}

	textureRun, err := s.GetRun(ctx, resp.RunID)
	if err != nil {
		t.Fatalf("get texture run: %v", err)
	}
	if textureRun.RequestedByRunID != conductorRun.RunID {
		t.Fatalf("texture run parent = %q, want conductor %q", textureRun.RequestedByRunID, conductorRun.RunID)
	}
	if trajectoryIDForRun(&textureRun) != trajectoryIDForRun(&conductorRun) {
		t.Fatalf("texture trajectory = %q, want conductor trajectory %q", trajectoryIDForRun(&textureRun), trajectoryIDForRun(&conductorRun))
	}
	if state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("agent revision state = %q, want %q", state, types.RunCompleted)
	}
}

func TestTextureOpenFileResolvesCanonicalAlias(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)

	openReq := func(initialContent string) *httptest.ResponseRecorder {
		req := textureRequest(t, http.MethodPost, "/api/texture/files/open", map[string]string{
			"source_path":     "notes/ai-news.md",
			"title":           "ai-news.md",
			"initial_content": initialContent,
		})
		w := httptest.NewRecorder()
		h.HandleTextureRouter(w, req)
		return w
	}

	first := openReq("Initial file content")
	if first.Code != http.StatusCreated {
		t.Fatalf("first open file: status = %d, want %d; body: %s", first.Code, http.StatusCreated, first.Body.String())
	}
	var firstResp textureOpenFileResponse
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
	if doc.Title != "ai-news.texture" {
		t.Fatalf("opened document title = %q, want ai-news.texture", doc.Title)
	}

	second := openReq("Changed file bytes that should not fork a new doc")
	if second.Code != http.StatusOK {
		t.Fatalf("second open file: status = %d, want %d; body: %s", second.Code, http.StatusOK, second.Body.String())
	}
	var secondResp textureOpenFileResponse
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
	if importManifest["projection_kind"] != "texture" || importManifest["source_kind"] != "md" || importManifest["original_content_hash"] == "" {
		t.Fatalf("import manifest = %#v", importManifest)
	}
	if importManifest["original_content_id"] != firstResp.OriginalContentID {
		t.Fatalf("import manifest original_content_id = %q, want %q", importManifest["original_content_id"], firstResp.OriginalContentID)
	}
	originalItem, err := s.GetContentItem(context.Background(), "user-1", firstResp.OriginalContentID)
	if err != nil {
		t.Fatalf("GetContentItem original: %v", err)
	}
	if originalItem.FilePath != "notes/ai-news.md" || originalItem.MediaType != "text/markdown" || originalItem.AppHint != AgentProfileTexture {
		t.Fatalf("original content item = %#v", originalItem)
	}
	if originalItem.TextContent != "Initial file content" || originalItem.ContentHash == "" {
		t.Fatalf("original text/hash = %#v", originalItem)
	}
	migrationManifest, ok := meta["migration_manifest"].(map[string]any)
	if !ok {
		t.Fatalf("missing migration_manifest: %#v", meta)
	}
	if migrationManifest["migration_adapter"] != "markdown_to_texture_projection" || migrationManifest["source_gap_policy"] != "repairable_gap_no_invented_citations" {
		t.Fatalf("migration manifest = %#v", migrationManifest)
	}
	exportReq := textureRequest(t, http.MethodGet, "/api/texture/documents/"+firstResp.DocID+"/export?format=md", nil)
	exportW := httptest.NewRecorder()
	h.HandleTextureRouter(exportW, exportReq)
	if exportW.Code != http.StatusOK {
		t.Fatalf("export markdown: status = %d, want %d; body: %s", exportW.Code, http.StatusOK, exportW.Body.String())
	}
	var exported textureDocumentExportResponse
	if err := json.NewDecoder(exportW.Body).Decode(&exported); err != nil {
		t.Fatalf("decode export response: %v", err)
	}
	if exported.Format != "md" || exported.Filename != "ai-news.md" || exported.Content != "Initial file content" || exported.ContentHash == "" {
		t.Fatalf("export response = %#v", exported)
	}
}

func TestTexturePlainTextImportCarriesMigrationMetadataToFirstDurableRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	ctx := context.Background()

	initialContent := strings.Join([]string{
		"Plain text proposal",
		"",
		"Imported source text should become canonical Texture.",
	}, "\n")
	openReq := textureRequest(t, http.MethodPost, "/api/texture/files/open", map[string]string{
		"source_path":     "notes/plain-proposal.txt",
		"title":           "plain-proposal.txt",
		"initial_content": initialContent,
	})
	openW := httptest.NewRecorder()
	h.HandleTextureRouter(openW, openReq)
	if openW.Code != http.StatusCreated {
		t.Fatalf("open text file: status = %d, want %d; body: %s", openW.Code, http.StatusCreated, openW.Body.String())
	}
	var opened textureOpenFileResponse
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
	if doc.Title != "plain-proposal.texture" {
		t.Fatalf("opened document title = %q, want plain-proposal.texture", doc.Title)
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
	if v0ImportManifest["source_media_type"] != "text/plain" || v0ImportManifest["projection_kind"] != "texture" {
		t.Fatalf("v0 import_manifest = %#v", v0ImportManifest)
	}
	v0MigrationManifest, ok := v0Meta["migration_manifest"].(map[string]any)
	if !ok {
		t.Fatalf("v0 missing migration_manifest: %#v", v0Meta)
	}
	if v0MigrationManifest["source_kind"] != "text" ||
		v0MigrationManifest["source_media_type"] != "text/plain" ||
		v0MigrationManifest["migration_adapter"] != "plain_text_to_texture_projection" ||
		v0MigrationManifest["projection_kind"] != "texture" {
		t.Fatalf("v0 migration_manifest = %#v", v0MigrationManifest)
	}

	v1Content := initialContent + "\n\nFirst durable revision keeps import lineage."
	revReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+opened.DocID+"/revisions", textureCreateRevisionRequest{
		Content:          v1Content,
		ParentRevisionID: opened.CurrentRevisionID,
		Metadata:         json.RawMessage(`{"created_from":"plain_text_v1_user_edit"}`),
	})
	revW := httptest.NewRecorder()
	h.HandleTextureRevisions(revW, revReq)
	if revW.Code != http.StatusCreated {
		t.Fatalf("create text v1: status = %d, want %d; body: %s", revW.Code, http.StatusCreated, revW.Body.String())
	}
	var v1 textureRevisionResponse
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
	if v1Meta[canonicalTextureSourcePathMetadataKey] == nil {
		t.Fatalf("v1 missing %s: %#v", canonicalTextureSourcePathMetadataKey, v1Meta)
	}
	v1MigrationManifest := v1Meta["migration_manifest"].(map[string]any)
	if v1MigrationManifest["migration_adapter"] != "plain_text_to_texture_projection" {
		t.Fatalf("v1 migration_manifest = %#v", v1MigrationManifest)
	}

	sourcePath, err := s.GetDocumentAliasSourcePath(ctx, "user-1", opened.DocID)
	if err != nil {
		t.Fatalf("GetDocumentAliasSourcePath: %v", err)
	}
	if filepath.Ext(sourcePath) != ".texture" {
		t.Fatalf("latest alias source_path = %q, want .texture", sourcePath)
	}
	if docID, err := s.GetDocumentAlias(ctx, "user-1", "notes/plain-proposal.txt"); err != nil || docID != opened.DocID {
		t.Fatalf("original text alias docID = %q, err = %v, want %q", docID, err, opened.DocID)
	}

	exportReq := textureRequest(t, http.MethodGet, "/api/texture/documents/"+opened.DocID+"/export?format=md", nil)
	exportW := httptest.NewRecorder()
	h.HandleTextureRouter(exportW, exportReq)
	if exportW.Code != http.StatusOK {
		t.Fatalf("export text as markdown: status = %d, want %d; body: %s", exportW.Code, http.StatusOK, exportW.Body.String())
	}
	var exported textureDocumentExportResponse
	if err := json.NewDecoder(exportW.Body).Decode(&exported); err != nil {
		t.Fatalf("decode text export: %v", err)
	}
	if exported.Format != "md" || exported.Filename != "plain-proposal.md" || exported.Content != v1Content || exported.RevisionID != v1.RevisionID {
		t.Fatalf("exported text response = %#v", exported)
	}
}

func TestTextureImportedMarkdownRevisionUsesTextureProjectionAndPreservesCollapsedTable(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
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
	openReq := textureRequest(t, http.MethodPost, "/api/texture/files/open", map[string]string{
		"source_path":     "proposals/legal-cloud.md",
		"title":           "legal-cloud.md",
		"initial_content": initialContent,
	})
	openW := httptest.NewRecorder()
	h.HandleTextureRouter(openW, openReq)
	if openW.Code != http.StatusCreated {
		t.Fatalf("open markdown: status = %d, want %d; body: %s", openW.Code, http.StatusCreated, openW.Body.String())
	}
	var opened textureOpenFileResponse
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
	revReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+opened.DocID+"/revisions", textureCreateRevisionRequest{
		Content: collapsedDraft,
		Metadata: json.RawMessage(`{
			"source_path":"proposals/legal-cloud.md",
			"created_from":"browser_user_edit"
		}`),
	})
	revW := httptest.NewRecorder()
	h.HandleTextureRevisions(revW, revReq)
	if revW.Code != http.StatusCreated {
		t.Fatalf("create collapsed draft revision: status = %d, want %d; body: %s", revW.Code, http.StatusCreated, revW.Body.String())
	}
	var revision textureRevisionResponse
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
	if meta["texture_structure_stabilized"] != true {
		t.Fatalf("metadata did not record structure stabilization: %#v", meta)
	}
	canonicalPath, ok := meta[canonicalTextureSourcePathMetadataKey].(string)
	if !ok || filepath.Ext(canonicalPath) != ".texture" {
		t.Fatalf("%s = %#v, want .texture", canonicalTextureSourcePathMetadataKey, meta[canonicalTextureSourcePathMetadataKey])
	}
	sourcePath, err := s.GetDocumentAliasSourcePath(ctx, "user-1", opened.DocID)
	if err != nil {
		t.Fatalf("GetDocumentAliasSourcePath: %v", err)
	}
	if filepath.Ext(sourcePath) != ".texture" {
		t.Fatalf("latest alias source_path = %q, want .texture", sourcePath)
	}
	if docID, err := s.GetDocumentAlias(ctx, "user-1", "proposals/legal-cloud.md"); err != nil || docID != opened.DocID {
		t.Fatalf("original markdown alias docID = %q, err = %v, want %q", docID, err, opened.DocID)
	}
}

func TestTextureMarkdownStructureStabilizationRepairsMalformedTableTailRow(t *testing.T) {
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

	stabilized, changed := stabilizeTextureUserMarkdownStructures(parentContent, userContent)
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

func TestTextureMarkdownStructureStabilizationHandlesPartialTableContexts(t *testing.T) {
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
			stabilized, _ := stabilizeTextureUserMarkdownStructures(parentContent, strings.Join(tt.userLines, "\n"))
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

func TestTextureRestoreRevisionNormalizesMalformedTableTailRows(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	createDocReq := textureRequest(t, http.MethodPost, "/api/texture/documents", textureCreateDocRequest{
		Title: "Restore Table Tail Fixture",
	})
	w := httptest.NewRecorder()
	h.HandleTextureDocumentsRoot(w, createDocReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create document: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var doc textureCreateDocResponse
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
	sourceReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+doc.DocID+"/revisions", textureCreateRevisionRequest{
		Content: malformedSource,
		Metadata: json.RawMessage(`{
			"created_from":"historical_import",
			"source_entities":[{"entity_id":"src-restore-rule","label":"ABA Model Rule 1.6"}]
		}`),
	})
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, sourceReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create malformed source revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var sourceResp textureRevisionResponse
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
	currentReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+doc.DocID+"/revisions", textureCreateRevisionRequest{
		Content:          currentContent,
		ParentRevisionID: sourceResp.RevisionID,
		Metadata:         json.RawMessage(`{"created_from":"current_head"}`),
	})
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, currentReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create current revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	restoreReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+doc.DocID+"/restore", textureRestoreRevisionRequest{
		RevisionID: sourceResp.RevisionID,
		Mode:       "primary",
	})
	w = httptest.NewRecorder()
	h.HandleTextureRouter(w, restoreReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("restore revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var restoredResp textureRevisionResponse
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
	if meta["texture_structure_stabilized"] != true ||
		meta["texture_structure_stabilized_reason"] != "normalized_restored_markdown_table_rows" {
		t.Fatalf("restored metadata did not record normalization: %#v", meta)
	}
	entities := decodeTextureSourceEntities(meta["source_entities"])
	if len(entities) != 1 || entities[0].EntityID != "src-restore-rule" {
		t.Fatalf("restored source_entities = %#v", meta["source_entities"])
	}
}

func TestTextureMarkdownStructureStabilizationAllowsExplicitTableDeletion(t *testing.T) {
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

	stabilized, changed := stabilizeTextureUserMarkdownStructures(parentContent, userContent)
	if changed {
		t.Fatalf("stabilization changed explicit table deletion:\n%s", stabilized)
	}
	if strings.Contains(stabilized, "| Term | Definition |") {
		t.Fatalf("stabilization restored explicitly deleted table:\n%s", stabilized)
	}
}

func TestTextureMarkdownTableRowParserHandlesEscapedPipes(t *testing.T) {
	t.Parallel()
	cells := markdownstructure.TableRowCells(`| Term \| Alias | Definition with \| symbol |`)
	if len(cells) != 2 {
		t.Fatalf("cells = %#v, want 2 cells", cells)
	}
	if cells[0] != "Term | Alias" || cells[1] != "Definition with | symbol" {
		t.Fatalf("cells = %#v", cells)
	}
}

func TestTextureImportMarkdownLineageCreatesRevisionHistory(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)

	req := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud.md",
		Title:      "legal-cloud.md",
		Versions: []textureMarkdownLineageVersion{
			{
				Label:            "v44",
				SourceRevisionID: "git-a",
				Content:          "# Proposal\n\nGlossary table survives here.\n",
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
	h.HandleTextureRouter(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp textureMarkdownLineageImportResponse
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
	if doc.Title != "legal-cloud.texture" {
		t.Fatalf("imported lineage document title = %q, want legal-cloud.texture", doc.Title)
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
	if manifest["migration_adapter"] != "markdown_lineage_to_texture_revisions" || manifest["lineage_count"] != float64(3) || manifest["source_gap_policy"] != "repairable_gap_no_invented_citations" {
		t.Fatalf("migration manifest = %#v", manifest)
	}
	lineage, ok := manifest["version_lineage"].([]any)
	if !ok || len(lineage) != 3 {
		t.Fatalf("version lineage = %#v", manifest["version_lineage"])
	}
	if manifest["original_content_id"] == "" || manifest["original_content_hash"] == "" {
		t.Fatalf("missing original snapshot refs in manifest: %#v", manifest)
	}
	if _, ok := meta["source_gaps"]; ok {
		t.Fatalf("markdown lineage import retained legacy source_gaps metadata: %#v", meta["source_gaps"])
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
			if item.MediaType != "text/markdown" || item.AppHint != AgentProfileTexture || item.TextContent == "" || item.ContentHash == "" {
				t.Fatalf("snapshot content item = %#v", item)
			}
		}
	}
	if foundSnapshots != 3 {
		t.Fatalf("found snapshot content items = %d, want 3", foundSnapshots)
	}
}

func TestTextureImportMarkdownLineageResolvesCitationMarkers(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	entity := textureSourceEntity{
		EntityID: "src-aba-rule-16",
		Kind:     "source_service_item",
		Label:    "ABA Model Rule 1.6",
		Target: textureSourceEntityTarget{
			TargetKind: "source_service_item",
			ItemID:     "srcitem_aba_rule_16",
		},
		Selectors: []textureSourceEntitySelector{{
			SelectorKind: "text_quote",
			TextQuote:    "A lawyer shall not reveal information relating to the representation of a client.",
		}},
		Display: textureSourceEntityDisplay{
			InlineMode:       "embedded_excerpt",
			ExpandedMode:     "source_card",
			OpenSurface:      "source",
			DefaultCollapsed: false,
		},
		Evidence: textureSourceEntityEvidence{
			State:         "available",
			ResearchState: "represented",
		},
		Provenance: textureSourceEntityProvenance{
			CreatedBy:           "migration",
			RightsScope:         "source_service_projection",
			UntrustedSourceText: true,
		},
	}

	req := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath:     "proposals/legal-cloud-sourced.md",
		Title:          "legal-cloud-sourced.md",
		SourceEntities: []textureSourceEntity{entity},
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v44",
			Content: "# Proposal\n\nConfidentiality matters [1].\n",
			CitationResolutions: []textureCitationMarkerResolution{{
				Marker:   "[1]",
				EntityID: entity.EntityID,
			}},
		}},
	})
	w := httptest.NewRecorder()
	h.HandleTextureRouter(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp textureMarkdownLineageImportResponse
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
	if !strings.Contains(revs[0].Content, "Confidentiality matters [1].") || strings.Contains(revs[0].Content, "](source:") {
		t.Fatalf("resolved citation was not projected as native source ref: %q", revs[0].Content)
	}
	meta := decodeRevisionMetadata(revs[0].Metadata)
	if _, ok := meta["source_entities"]; ok {
		t.Fatalf("markdown lineage import retained legacy source_entities metadata: %#v", meta["source_entities"])
	}
	var structuredEntities []map[string]any
	if err := json.Unmarshal(revs[0].SourceEntities, &structuredEntities); err != nil {
		t.Fatalf("decode structured source_entities: %v", err)
	}
	if len(structuredEntities) != 1 || structuredEntities[0]["source_entity_id"] != entity.EntityID {
		t.Fatalf("structured source_entities = %#v", structuredEntities)
	}
	manifest := meta["migration_manifest"].(map[string]any)
	resolutions, ok := manifest["citation_resolutions"].([]any)
	if !ok || len(resolutions) != 1 {
		t.Fatalf("citation_resolutions = %#v", manifest["citation_resolutions"])
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

func TestTextureUserSaveAndAgentRevisePreserveSourcesAndTableShape(t *testing.T) {
	t.Parallel()
	provider := newTextureEditToolProvider("")
	provider.resultFunc = func(prompt string) string {
		return textureStructuredApplyEditsResult([]textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: firstPromptOutlineParagraphID(prompt),
			Text:    "A private legal cloud addresses this clearly.",
		}})
	}
	h, s, _ := textureAPISetupWithProvider(t, provider, true)
	ctx := context.Background()
	entity := textureSourceEntity{
		EntityID: "src-aba-rule-16",
		Kind:     "source_service_item",
		Label:    "ABA Model Rule 1.6",
		Target: textureSourceEntityTarget{
			TargetKind: "source_service_item",
			ItemID:     "srcitem_aba_rule_16",
		},
		Selectors: []textureSourceEntitySelector{{
			SelectorKind: "text_quote",
			TextQuote:    "A lawyer shall not reveal information relating to the representation of a client.",
		}},
		Display: textureSourceEntityDisplay{
			InlineMode:   "embedded_excerpt",
			ExpandedMode: "source_card",
			OpenSurface:  "source",
		},
		Evidence: textureSourceEntityEvidence{
			State:         "available",
			ResearchState: "represented",
		},
		Provenance: textureSourceEntityProvenance{
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
	importReq := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath:     "proposals/legal-cloud-sourced.md",
		Title:          "legal-cloud-sourced.md",
		SourceEntities: []textureSourceEntity{entity},
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v1",
			Content: parentContent,
			CitationResolutions: []textureCitationMarkerResolution{{
				Marker:   "[1]",
				EntityID: entity.EntityID,
			}},
		}},
	})
	w := httptest.NewRecorder()
	h.HandleTextureRouter(w, importReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var imported textureMarkdownLineageImportResponse
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
	userReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+imported.DocID+"/revisions", textureCreateRevisionRequest{
		Content:          userContent,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "owner",
		Metadata:         json.RawMessage(`{"created_from":"browser_user_edit"}`),
		ParentRevisionID: imported.CurrentRevisionID,
	})
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, userReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var userRevResp textureRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&userRevResp); err != nil {
		t.Fatalf("decode user revision response: %v", err)
	}
	userRev, err := s.GetRevision(ctx, userRevResp.RevisionID, "user-1")
	if err != nil {
		t.Fatalf("GetRevision user: %v", err)
	}
	userMeta := decodeRevisionMetadata(userRev.Metadata)
	userEntities := decodeTextureSourceEntities(userMeta["source_entities"])
	if len(userEntities) != 1 || userEntities[0].EntityID != entity.EntityID {
		t.Fatalf("user revision source_entities = %#v", userMeta["source_entities"])
	}
	userTables := extractMarkdownTableBlocks(userRev.Content)
	if len(userTables) != 1 || userTables[0].Text != parentTables[0].Text {
		t.Fatalf("user revision table changed:\nparent:\n%s\nuser:\n%s", parentTables[0].Text, userRev.Content)
	}

	reviseReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+imported.DocID+"/revise",
		map[string]string{"intent": "revise"})
	w = httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, reviseReq)
	if w.Code != http.StatusAccepted {
		t.Fatalf("revise status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}
	revs := waitForRevisionCount(t, s, imported.DocID, "user-1", 3, 5*time.Second)
	agentRev := revs[0]
	agentMeta := decodeRevisionMetadata(agentRev.Metadata)
	agentEntities := decodeTextureSourceEntities(agentMeta["source_entities"])
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

func TestTextureUserSaveRemovesDuplicateMarkdownTableSeparator(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	entity := textureSourceEntity{
		EntityID: "src-table-separator-proof",
		Kind:     "source_service_item",
		Label:    "Table Source",
		Target: textureSourceEntityTarget{
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
	importReq := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath:     "proposals/table-separator-proof.md",
		Title:          "table-separator-proof.md",
		SourceEntities: []textureSourceEntity{entity},
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v1",
			Content: parentContent,
			CitationResolutions: []textureCitationMarkerResolution{{
				Marker:   "[1]",
				EntityID: entity.EntityID,
			}},
		}},
	})
	w := httptest.NewRecorder()
	h.HandleTextureRouter(w, importReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var imported textureMarkdownLineageImportResponse
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
	userReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+imported.DocID+"/revisions", textureCreateRevisionRequest{
		Content:          userContent,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "owner",
		Metadata:         json.RawMessage(`{"created_from":"browser_user_edit"}`),
		ParentRevisionID: imported.CurrentRevisionID,
	})
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, userReq)
	if w.Code != http.StatusCreated {
		t.Fatalf("create user revision: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var userRevResp textureRevisionResponse
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
	if userMeta["texture_structure_stabilized"] != true {
		t.Fatalf("user metadata did not record table stabilization: %#v", userMeta)
	}
	userEntities := decodeTextureSourceEntities(userMeta["source_entities"])
	if len(userEntities) != 1 || userEntities[0].EntityID != entity.EntityID {
		t.Fatalf("user revision source_entities = %#v", userMeta["source_entities"])
	}
}

func TestTextureSourceGapRepairCreatesRevision(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)
	importReq := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-repairable.md",
		Title:      "legal-cloud-repairable.md",
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v44",
			Content: "# Proposal\n\nConfidentiality matters [1].\n",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleTextureRouter(importW, importReq)
	if importW.Code != http.StatusBadRequest || !strings.Contains(importW.Body.String(), "unresolved markdown citation marker [1]") {
		t.Fatalf("import unresolved marker status = %d body=%s", importW.Code, importW.Body.String())
	}
}

func TestTextureSourceGapRepairPreservesUnrepairedGaps(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)
	importReq := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-partial-repair.md",
		Title:      "legal-cloud-partial-repair.md",
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v44",
			Content: "Known [1]. Still unknown [2].",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleTextureRouter(importW, importReq)
	if importW.Code != http.StatusBadRequest || !strings.Contains(importW.Body.String(), "unresolved markdown citation marker [1]") {
		t.Fatalf("partial unresolved import status = %d body=%s", importW.Code, importW.Body.String())
	}
}

func TestTextureSourceGapRepairCanOmitNoSourceNeededMarker(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)
	importReq := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-no-source-needed.md",
		Title:      "legal-cloud-no-source-needed.md",
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v44",
			Content: "Ordinary framing sentence [2].",
			CitationResolutions: []textureCitationMarkerResolution{{
				Marker: "[2]",
				Action: "no_source_needed",
				Reason: "The sentence is structural framing, not a factual claim needing citation.",
			}},
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleTextureRouter(importW, importReq)
	if importW.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", importW.Code, http.StatusCreated, importW.Body.String())
	}
	var imported textureMarkdownLineageImportResponse
	if err := json.NewDecoder(importW.Body).Decode(&imported); err != nil {
		t.Fatalf("decode import response: %v", err)
	}
	if len(imported.Revisions) != 1 || imported.Revisions[0].Content != "Ordinary framing sentence." {
		t.Fatalf("imported revisions = %#v", imported.Revisions)
	}
	meta := decodeRevisionMetadata(imported.Revisions[0].Metadata)
	if _, ok := meta["source_gaps"]; ok {
		t.Fatalf("source_gaps should not be written after no-source-needed import: %#v", meta["source_gaps"])
	}
	if _, ok := meta["source_entities"]; ok {
		t.Fatalf("no-source-needed import should not invent source_entities metadata: %#v", meta["source_entities"])
	}
	manifest := meta["migration_manifest"].(map[string]any)
	resolutions, ok := manifest["citation_resolutions"].([]any)
	if !ok || len(resolutions) != 1 {
		t.Fatalf("citation_resolutions = %#v", manifest["citation_resolutions"])
	}
	item := resolutions[0].(map[string]any)
	if item["marker"] != "[2]" || item["action"] != "no_source_needed" || item["entity_id"] != nil {
		t.Fatalf("citation resolution manifest item = %#v", item)
	}
	if item["reason"] != "The sentence is structural framing, not a factual claim needing citation." {
		t.Fatalf("citation resolution reason = %#v", item["reason"])
	}
}

func TestTextureSourceGapRepairRejectsUnknownEntity(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)
	importReq := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-bad-repair.md",
		Title:      "legal-cloud-bad-repair.md",
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v44",
			Content: "Unknown [1].",
		}},
	})
	importW := httptest.NewRecorder()
	h.HandleTextureRouter(importW, importReq)
	if importW.Code != http.StatusBadRequest || !strings.Contains(importW.Body.String(), "unresolved markdown citation marker [1]") {
		t.Fatalf("unknown unresolved import status = %d body=%s", importW.Code, importW.Body.String())
	}
}

func TestTextureImportMarkdownLineageUsesExistingContentItems(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	oldContent := "# Proposal\n\nStored historical glossary.\n"
	latestContent := "# Proposal\n\nStored latest appendix table.\n"
	oldItem := types.ContentItem{
		ContentID:   "content-lineage-v44",
		OwnerID:     "user-1",
		SourceType:  "file_version",
		MediaType:   "text/markdown",
		AppHint:     AgentProfileTexture,
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
		AppHint:     AgentProfileTexture,
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

	req := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath: "proposals/legal-cloud-content-backed.md",
		Title:      "legal-cloud-content-backed.md",
		Versions: []textureMarkdownLineageVersion{
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
	h.HandleTextureRouter(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("import markdown lineage: status = %d, want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var resp textureMarkdownLineageImportResponse
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
	if !strings.Contains(oldest.Content, "Stored historical glossary.") {
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

func TestTextureImportMarkdownLineageRejectsMissingContentItem(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)
	req := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath: "proposals/missing-content-item.md",
		Title:      "missing-content-item.md",
		Versions: []textureMarkdownLineageVersion{{
			Label:         "v1",
			ContentItemID: "missing-content-item",
		}},
	})
	w := httptest.NewRecorder()
	h.HandleTextureRouter(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "content_item_id missing-content-item not found") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestTextureImportMarkdownLineageRejectsUnknownCitationEntity(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)
	req := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", textureMarkdownLineageImportRequest{
		SourcePath: "proposals/bad-sourced.md",
		Title:      "bad-sourced.md",
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v1",
			Content: "Claim [1].",
			CitationResolutions: []textureCitationMarkerResolution{{
				Marker:   "[1]",
				EntityID: "missing-source",
			}},
		}},
	})
	w := httptest.NewRecorder()
	h.HandleTextureRouter(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestTextureImportMarkdownLineageRejectsExistingAlias(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)
	body := textureMarkdownLineageImportRequest{
		SourcePath: "notes/duplicate.md",
		Title:      "duplicate.md",
		Versions: []textureMarkdownLineageVersion{{
			Label:   "v1",
			Content: "Initial version",
		}},
	}
	req := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", body)
	first := httptest.NewRecorder()
	h.HandleTextureRouter(first, req)
	if first.Code != http.StatusCreated {
		t.Fatalf("first import: status = %d, want %d; body: %s", first.Code, http.StatusCreated, first.Body.String())
	}

	secondReq := textureRequest(t, http.MethodPost, "/api/texture/markdown-lineage/import", body)
	second := httptest.NewRecorder()
	h.HandleTextureRouter(second, secondReq)
	if second.Code != http.StatusConflict {
		t.Fatalf("second import: status = %d, want %d; body: %s", second.Code, http.StatusConflict, second.Body.String())
	}
	var resp textureMarkdownLineageImportResponse
	if err := json.NewDecoder(second.Body).Decode(&resp); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if resp.Created || resp.ExistingDocID == "" {
		t.Fatalf("conflict response = %#v", resp)
	}
}

func TestTextureOpenFilePreservesDocxAndPDFOriginalArtifacts(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)

	openFile := func(sourcePath, title, initialContent string) textureOpenFileResponse {
		req := textureRequest(t, http.MethodPost, "/api/texture/files/open", map[string]string{
			"source_path":     sourcePath,
			"title":           title,
			"initial_content": initialContent,
		})
		w := httptest.NewRecorder()
		h.HandleTextureRouter(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("open %s: status = %d, want %d; body: %s", sourcePath, w.Code, http.StatusCreated, w.Body.String())
		}
		var resp textureOpenFileResponse
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
		resp      textureOpenFileResponse
		mediaType string
		appHint   string
		lossiness float64
		warning   string
		wantText  string
	}{
		{name: "docx", resp: docx, mediaType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", appHint: AgentProfileTexture, lossiness: 40, warning: "docx_projection_requires_style_adapter", wantText: "Extracted DOCX projection text"},
		{name: "pdf", resp: pdf, mediaType: "application/pdf", appHint: "pdf", lossiness: 80, warning: "pdf_projection_requires_extraction_adapter", wantText: "Extracted PDF projection text"},
	} {
		doc, err := s.GetDocument(context.Background(), tc.resp.DocID, "user-1")
		if err != nil {
			t.Fatalf("%s GetDocument: %v", tc.name, err)
		}
		if doc.Title != "legal-cloud-proposal.texture" {
			t.Fatalf("%s document title = %q, want legal-cloud-proposal.texture", tc.name, doc.Title)
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

func TestTextureOpenFileImportsDocxAndPDFBytesFromFilesRoot(t *testing.T) {
	h, s, _ := textureAPISetupWithRuntime(t)
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

	openFile := func(sourcePath string) textureOpenFileResponse {
		req := textureRequest(t, http.MethodPost, "/api/texture/files/open", map[string]string{
			"source_path": sourcePath,
			"title":       filepath.Base(sourcePath),
		})
		w := httptest.NewRecorder()
		h.HandleTextureRouter(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("open %s: status = %d, want %d; body: %s", sourcePath, w.Code, http.StatusCreated, w.Body.String())
		}
		var resp textureOpenFileResponse
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

func TestTextureEnsureManifestCreatesAliasAndFile(t *testing.T) {
	h, s, _ := textureAPISetupWithRuntime(t)
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)

	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/manifest", nil)
	w := httptest.NewRecorder()
	h.HandleTextureRouter(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ensure manifest: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp textureEnsureManifestResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ensure manifest response: %v", err)
	}
	if resp.DocID != docID {
		t.Fatalf("response doc_id = %q, want %q", resp.DocID, docID)
	}
	if resp.SourcePath == "" {
		t.Fatal("response source_path should not be empty")
	}
	if filepath.Ext(resp.SourcePath) != ".texture" {
		t.Fatalf("response source_path extension = %q, want .texture", filepath.Ext(resp.SourcePath))
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
	var shortcut textureShortcutFile
	if err := json.Unmarshal(bytes, &shortcut); err != nil {
		t.Fatalf("unmarshal shortcut file: %v\nraw=%s", err, string(bytes))
	}
	if shortcut.Kind != "texture" {
		t.Fatalf("shortcut kind = %q, want %q", shortcut.Kind, "texture")
	}
	if shortcut.DocID != docID {
		t.Fatalf("shortcut doc_id = %q, want %q", shortcut.DocID, docID)
	}
	if shortcut.SourcePath != resp.SourcePath {
		t.Fatalf("shortcut source_path = %q, want %q", shortcut.SourcePath, resp.SourcePath)
	}
}

func TestTextureShortcutFileKindPreservesLegacyTextureCompatibility(t *testing.T) {
	doc := types.Document{
		DocID:   "doc-shortcut-kind",
		OwnerID: "user-1",
		Title:   "Shortcut Kind.texture",
	}

	for _, tc := range []struct {
		name       string
		sourcePath string
		wantKind   string
	}{
		{name: "current texture", sourcePath: "shortcut-kind.texture", wantKind: "texture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !isTextureShortcutPath(tc.sourcePath) {
				t.Fatalf("%s should be recognized as a Texture shortcut", tc.sourcePath)
			}
			bytes, err := marshalTextureShortcutFile(doc, tc.sourcePath)
			if err != nil {
				t.Fatalf("marshalTextureShortcutFile: %v", err)
			}
			var shortcut textureShortcutFile
			if err := json.Unmarshal(bytes, &shortcut); err != nil {
				t.Fatalf("unmarshal shortcut file: %v\nraw=%s", err, string(bytes))
			}
			if shortcut.Kind != tc.wantKind {
				t.Fatalf("shortcut kind = %q, want %q", shortcut.Kind, tc.wantKind)
			}
			if shortcut.SourcePath != tc.sourcePath {
				t.Fatalf("shortcut source_path = %q, want %q", shortcut.SourcePath, tc.sourcePath)
			}
		})
	}
}

func TestTextureEnsureManifestReusesExistingAlias(t *testing.T) {
	h, s, _ := textureAPISetupWithRuntime(t)
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)

	docID, _ := createDocWithUserRevision(t, h)

	firstReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/manifest", nil)
	firstW := httptest.NewRecorder()
	h.HandleTextureRouter(firstW, firstReq)
	if firstW.Code != http.StatusOK {
		t.Fatalf("first ensure manifest: status = %d, want %d; body: %s", firstW.Code, http.StatusOK, firstW.Body.String())
	}
	var firstResp textureEnsureManifestResponse
	if err := json.NewDecoder(firstW.Body).Decode(&firstResp); err != nil {
		t.Fatalf("decode first ensure manifest response: %v", err)
	}

	secondReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/manifest", nil)
	secondW := httptest.NewRecorder()
	h.HandleTextureRouter(secondW, secondReq)
	if secondW.Code != http.StatusOK {
		t.Fatalf("second ensure manifest: status = %d, want %d; body: %s", secondW.Code, http.StatusOK, secondW.Body.String())
	}
	var secondResp textureEnsureManifestResponse
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

func TestTextureCreateRevisionRejectsStaleHead(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	headReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content:          "Latest head",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
	})
	headW := httptest.NewRecorder()
	h.HandleTextureRevisions(headW, headReq)
	if headW.Code != http.StatusCreated {
		t.Fatalf("create head revision: status = %d, want %d; body: %s", headW.Code, http.StatusCreated, headW.Body.String())
	}

	staleReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content:          "Stale write",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
	})
	staleW := httptest.NewRecorder()
	h.HandleTextureRevisions(staleW, staleReq)
	if staleW.Code != http.StatusConflict {
		t.Fatalf("stale create revision: status = %d, want %d; body: %s", staleW.Code, http.StatusConflict, staleW.Body.String())
	}
}

func TestTextureCreateRevisionRebasesAllowedStaleUserDraft(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	headReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content:          "Initial content.\n\nAgent-added latest head detail.",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
	})
	headW := httptest.NewRecorder()
	h.HandleTextureRevisions(headW, headReq)
	if headW.Code != http.StatusCreated {
		t.Fatalf("create newer head revision: status = %d, want %d; body: %s", headW.Code, http.StatusCreated, headW.Body.String())
	}
	var headResp textureRevisionResponse
	if err := json.NewDecoder(headW.Body).Decode(&headResp); err != nil {
		t.Fatalf("decode head response: %v", err)
	}

	staleReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content:          "Initial content.\n\nUser dirty draft detail.",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
		AllowRebase:      true,
		Metadata:         json.RawMessage(`{"autosaved":true}`),
	})
	staleW := httptest.NewRecorder()
	h.HandleTextureRevisions(staleW, staleReq)
	if staleW.Code != http.StatusCreated {
		t.Fatalf("rebased stale revision: status = %d, want %d; body: %s", staleW.Code, http.StatusCreated, staleW.Body.String())
	}
	var rebasedResp textureRevisionResponse
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

func TestTextureDocumentStreamSendsSnapshot(t *testing.T) {
	t.Parallel()
	h, s := textureAPISetup(t)
	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/stream", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleTextureDocumentStream(w, req)
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
	for _, ev := range parseTextureStreamEvents(t, w.Body.String()) {
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

func TestTextureDocumentResponseReportsPendingAgentMutation(t *testing.T) {
	t.Parallel()
	h, s := textureAPISetup(t)
	docID, _ := createDocWithUserRevision(t, h)
	if err := s.CreateAgentMutation(context.Background(), store.AgentMutation{
		DocID:     docID,
		RunID:     "run-texture-pending-ui",
		OwnerID:   "user-1",
		State:     "pending",
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create pending mutation: %v", err)
	}

	req := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID, nil)
	w := httptest.NewRecorder()
	h.HandleTextureDocument(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get document: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp textureDocumentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode document response: %v", err)
	}
	if !resp.AgentRevisionPending {
		t.Fatalf("agent_revision_pending = false, want true; response=%+v", resp)
	}
	if resp.AgentRevisionRunID != "run-texture-pending-ui" {
		t.Fatalf("agent_revision_run_id = %q, want run-texture-pending-ui", resp.AgentRevisionRunID)
	}
}

func TestTextureDocumentResponseReconcilesPendingMutationFromCurrentHead(t *testing.T) {
	t.Parallel()
	h, s := textureAPISetup(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)
	runID := "run-texture-head-already-written"
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
		"source":  "patch_texture",
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

	req := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID, nil)
	w := httptest.NewRecorder()
	h.HandleTextureDocument(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get document: status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp textureDocumentResponse
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

func TestTextureDiagnosisReportsCurrentRevisionVersion(t *testing.T) {
	t.Parallel()
	h, s := textureAPISetup(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)
	if err := s.CreateRevision(context.Background(), types.Revision{
		RevisionID:       "rev-diagnosis-v1",
		DocID:            docID,
		OwnerID:          "user-1",
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		Content:          "## Appendix\n\n| Owner | State |\n| --- | --- |\n| Legal cloud | Preserved |\n",
		Citations:        json.RawMessage("[]"),
		Metadata:         json.RawMessage(`{"source":"edit_texture"}`),
		ParentRevisionID: baseRevisionID,
		CreatedAt:        time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create second revision: %v", err)
	}

	req := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/diagnosis?limit=10", nil)
	w := httptest.NewRecorder()
	h.HandleTextureDiagnosis(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("diagnosis status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp textureDiagnosisResponse
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
	if structure.ContentHash == "" || structure.HeadingCount != 1 || structure.SourceMarkerCount != 0 {
		t.Fatalf("diagnosis structure counts/hash = %+v", structure)
	}
	if structure.TableCount != 1 || structure.TableRowCount != 3 || len(structure.Tables) != 1 {
		t.Fatalf("diagnosis table structure = %+v", structure)
	}
	if table := structure.Tables[0]; table.StartLine != 3 || table.EndLine != 5 || table.ColumnCount != 2 || table.RowCount != 3 || !table.HasSeparator || table.Signature == "" {
		t.Fatalf("diagnosis table signature = %+v", table)
	}
}

func TestTextureDiagnosisCanOmitRevisionContentForStructureEvidence(t *testing.T) {
	t.Parallel()
	h, s := textureAPISetup(t)
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

	req := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/diagnosis?limit=10&include_content=false", nil)
	w := httptest.NewRecorder()
	h.HandleTextureDiagnosis(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("diagnosis status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp textureDiagnosisResponse
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

func TestTextureDocumentStreamEmitsHeadChangeAfterAgentRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/stream", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleTextureDocumentStream(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	revReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it more formal"})
	revW := httptest.NewRecorder()
	h.HandleTextureAgentRevision(revW, revReq)
	if revW.Code != http.StatusAccepted {
		t.Fatalf("agent revision: status = %d, want %d; body: %s", revW.Code, http.StatusAccepted, revW.Body.String())
	}

	var resp textureAgentRevisionResponse
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
	for _, ev := range parseTextureStreamEvents(t, w.Body.String()) {
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

func TestTextureStreamEventMapsProgressSeparatelyFromStarted(t *testing.T) {
	t.Parallel()
	started, ok := textureStreamEventFromRecord(types.EventRecord{
		Kind:    types.EventTextureAgentRevisionStarted,
		Payload: json.RawMessage(`{"doc_id":"doc-1","loop_id":"run-1"}`),
	})
	if !ok || started.Kind != "synth_started" {
		t.Fatalf("started event = %+v, ok=%v, want synth_started", started, ok)
	}
	progress, ok := textureStreamEventFromRecord(types.EventRecord{
		Kind:    types.EventTextureAgentRevisionProgress,
		Payload: json.RawMessage(`{"doc_id":"doc-1","loop_id":"run-1"}`),
	})
	if !ok || progress.Kind != "synth_progress" {
		t.Fatalf("progress event = %+v, ok=%v, want synth_progress", progress, ok)
	}
	parked, ok := textureStreamEventFromRecord(types.EventRecord{
		Kind:    types.EventTextureAgentRevisionProgress,
		Payload: json.RawMessage(`{"doc_id":"doc-1","loop_id":"run-1","phase":"park_wait_started"}`),
	})
	if !ok || parked.Kind != "synth_completed" || parked.Phase != "park_wait_started" {
		t.Fatalf("park wait event = %+v, ok=%v, want synth_completed phase", parked, ok)
	}
}

func TestTextureStreamEventMapsTexturePassivationToSynthCompleted(t *testing.T) {
	t.Parallel()
	passivated, ok := textureStreamEventFromRecord(types.EventRecord{
		Kind:    types.EventRunPassivated,
		AgentID: "texture:doc-1",
		Payload: json.RawMessage(`{"doc_id":"doc-1","loop_id":"run-1","current_revision_id":"rev-1"}`),
	})
	if !ok || passivated.Kind != "synth_completed" {
		t.Fatalf("passivated event = %+v, ok=%v, want synth_completed", passivated, ok)
	}
	if passivated.DocID != "doc-1" || passivated.LoopID != "run-1" || passivated.CurrentRevisionID != "rev-1" {
		t.Fatalf("passivated stream event metadata = %+v", passivated)
	}

	if event, ok := textureStreamEventFromRecord(types.EventRecord{
		Kind:    types.EventRunPassivated,
		AgentID: "researcher:doc-1",
		Payload: json.RawMessage(`{"doc_id":"doc-1","loop_id":"run-1"}`),
	}); ok {
		t.Fatalf("non-Texture passivation mapped to Texture stream event: %+v", event)
	}
}

func TestTextureIdlePassivationEventCarriesDocumentStreamCompletionPayload(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	provider := &textureParkResidentProvider{Provider: NewStubProvider(time.Millisecond)}
	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	rt.cfg.TextureActorParkIdle = 25 * time.Millisecond
	docID, _ := createDocWithUserRevision(t, h)
	doc, err := s.GetDocument(ctx, docID, "user-1")
	if err != nil {
		t.Fatalf("get doc: %v", err)
	}

	textureRun, err := rt.submitTextureAgentRevisionRun(ctx, doc, "user-1", textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: "Draft once, then passivate.",
	}, 0)
	if err != nil {
		t.Fatalf("submit texture revision run: %v", err)
	}

	sleepingRun := waitForStoredRunState(t, s, textureRun.RunID, types.RunPassivated, 5*time.Second)
	revisionID := metadataStringValue(sleepingRun.Metadata, "current_revision_id")
	if revisionID == "" {
		t.Fatalf("passivated run missing current_revision_id: %+v", sleepingRun.Metadata)
	}

	var passivation *types.EventRecord
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		events, err := s.ListEvents(ctx, textureRun.RunID, 200)
		if err != nil {
			t.Fatalf("list events: %v", err)
		}
		for i := range events {
			if events[i].Kind == types.EventRunPassivated {
				ev := events[i]
				passivation = &ev
				break
			}
		}
		if passivation != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if passivation == nil {
		t.Fatalf("missing %s event for passivated Texture run", types.EventRunPassivated)
	}

	streamEvent, ok := textureStreamEventFromRecord(*passivation)
	if !ok || streamEvent.Kind != "synth_completed" {
		t.Fatalf("passivation stream event = %+v, ok=%v, want synth_completed", streamEvent, ok)
	}
	if streamEvent.DocID != docID || streamEvent.LoopID != textureRun.RunID || streamEvent.CurrentRevisionID != revisionID {
		t.Fatalf("passivation stream metadata = %+v, want doc=%s loop=%s revision=%s", streamEvent, docID, textureRun.RunID, revisionID)
	}
}

func TestTextureDocumentStreamEmitsHeadChangeAfterUserRevision(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	docID, baseRevisionID := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/stream", nil)
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.HandleTextureDocumentStream(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	createReq := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content:          "User-authored next head",
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      "alice",
		ParentRevisionID: baseRevisionID,
	})
	createW := httptest.NewRecorder()
	h.HandleTextureRevisions(createW, createReq)
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
	for _, ev := range parseTextureStreamEvents(t, w.Body.String()) {
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

func parseTextureStreamEvents(t *testing.T, body string) []textureDocumentStreamEvent {
	t.Helper()
	lines := strings.Split(body, "\n")
	events := make([]textureDocumentStreamEvent, 0)
	for _, line := range lines {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev textureDocumentStreamEvent
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			t.Fatalf("decode texture stream event: %v", err)
		}
		events = append(events, ev)
	}
	return events
}

// TestTextureAgentRevisionAuthRequired verifies that agent revision
// requires authentication (VAL-ETEXT-003: auth-gated).
func TestTextureAgentRevisionAuthRequired(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// No auth header.
	req := httptest.NewRequest(http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		bytes.NewReader([]byte(`{"prompt":"test"}`)))
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// TestTextureAgentRevisionPreservesUserAndAppAgentAttribution verifies
// that an end-to-end flow preserves both user and appagent attribution
// in history (VAL-CROSS-119).
func TestTextureAgentRevisionPreservesUserAndAppAgentAttribution(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Submit an agent revision.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Improve the text"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	var resp textureAgentRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)

	// Wait for completion.
	state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("task state = %q, want %q", state, types.RunCompleted)
	}

	// Make another user edit after the agent revision.
	revReq := textureCreateRevisionRequest{
		Content:     "User final edit",
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", revReq)
	w = httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
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

// TestTextureAgentRevisionNoWorkerAuthorship verifies that when subordinate
// workers might contribute to an appagent-driven change, the resulting
// canonical history attributes the change to the appagent, not to any
// worker identity (VAL-CROSS-120).
func TestTextureAgentRevisionNoWorkerAuthorship(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Submit an agent revision.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it better"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	var resp textureAgentRevisionResponse
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

// TestTextureAgentRevisionNoDuplicateOnRenewalRetry verifies that renewal
// and retry does not duplicate a canonical document mutation (VAL-CROSS-122).
func TestTextureAgentRevisionNoDuplicateOnRenewalRetry(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Submit an agent revision.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it concise"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	var resp1 textureAgentRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&resp1)

	// Simulate a renewal/retry by submitting the same request again
	// before the task completes. The idempotency check should return
	// the same task ID.
	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Make it concise"})
	w = httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)

	var resp2 textureAgentRevisionResponse
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

// TestTextureAgentRevisionDeliversOwnerRequestToResidentActor verifies the
// long-running-actor foreground path. A /revise while a resident (non-terminal)
// Texture actor holds a pending mutation must NOT complete that mutation (P1#1:
// the actor stays writable for further revisions), and must deliver the owner's
// new request to the actor as an addressed update instead of dropping it (P1#2:
// no lost foreground updates). An identical retry stays idempotent (VAL-CROSS-122).
func TestTextureAgentRevisionDeliversOwnerRequestToResidentActor(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	docID, baseRevisionID := createDocWithUserRevision(t, h)
	agentID := currentTextureAgentID(docID)
	runID := "run-resident-texture-actor"

	run := types.RunRecord{
		RunID:        runID,
		AgentID:      agentID,
		ChannelID:    docID,
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		Prompt:       "Revise the document.",
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataAgentID:      agentID,
			runMetadataChannelID:    docID,
			"doc_id":                docID,
		},
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create resident run: %v", err)
	}
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: runID, OwnerID: "user-1", State: "pending", CreatedAt: now,
	}); err != nil {
		t.Fatalf("create pending mutation: %v", err)
	}
	// An appagent head revision the resident actor already wrote (matching source
	// + loop_id) so the old reconcile path would have completed the mutation.
	headMeta, _ := json.Marshal(map[string]any{"source": "patch_texture", "loop_id": runID})
	if err := s.CreateRevision(ctx, types.Revision{
		RevisionID:       "rev-resident-head",
		DocID:            docID,
		OwnerID:          "user-1",
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		Content:          "Resident actor wrote V1.",
		Citations:        json.RawMessage("[]"),
		Metadata:         headMeta,
		ParentRevisionID: baseRevisionID,
		CreatedAt:        now,
	}); err != nil {
		t.Fatalf("create appagent head: %v", err)
	}

	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, textureRequest(t, http.MethodPost,
		"/api/texture/documents/"+docID+"/revise", map[string]string{"prompt": "Add a risks section"}))
	if w.Code != http.StatusAccepted {
		t.Fatalf("revise status = %d, want 202; body: %s", w.Code, w.Body.String())
	}
	var resp textureAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode revise response: %v", err)
	}
	if resp.RunID != runID {
		t.Fatalf("revise returned run %q, want resident run %q", resp.RunID, runID)
	}

	mutation, err := s.GetAgentMutationByRun(ctx, runID)
	if err != nil {
		t.Fatalf("get mutation: %v", err)
	}
	if mutation.State != "pending" {
		t.Fatalf("resident actor mutation state = %q, want pending (P1#1: must stay writable)", mutation.State)
	}

	updates, err := s.ListPendingWorkerUpdates(ctx, "user-1", agentID, 10)
	if err != nil {
		t.Fatalf("list pending updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("pending owner updates = %d, want 1 (P1#2: owner request must not be dropped)", len(updates))
	}
	if updates[0].Packet.Kind != "decision_request" || !strings.Contains(updates[0].Content, "Add a risks section") {
		t.Fatalf("delivered update = %+v, want owner_revision_request carrying the prompt", updates[0])
	}

	wRetry := httptest.NewRecorder()
	h.HandleTextureAgentRevision(wRetry, textureRequest(t, http.MethodPost,
		"/api/texture/documents/"+docID+"/revise", map[string]string{"prompt": "Add a risks section"}))
	if wRetry.Code != http.StatusAccepted {
		t.Fatalf("retry status = %d, want 202; body: %s", wRetry.Code, wRetry.Body.String())
	}
	updates, err = s.ListPendingWorkerUpdates(ctx, "user-1", agentID, 10)
	if err != nil {
		t.Fatalf("list pending updates after retry: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("pending owner updates after identical retry = %d, want 1 (VAL-CROSS-122 idempotency)", len(updates))
	}
}

// TestTextureAgentRevisionMutationCompletedOnlyOnce verifies that Texture write tools are
// the idempotency boundary for canonical appagent revisions (VAL-CROSS-122).
func TestTextureAppagentEditCanonicalizesAliasedMarkdownTitle(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)
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
		Metadata: buildFileOpenTextureMetadata(textureFileImportProjection{
			SourcePath:            "proposals/legacy-proposal.md",
			MediaType:             "text/markdown",
			ProjectionContent:     "# Legacy Proposal\n\nImported Markdown body.",
			ProjectionContentHash: contentHash("# Legacy Proposal\n\nImported Markdown body."),
			OriginalContentHash:   contentHash("# Legacy Proposal\n\nImported Markdown body."),
			ImportAdapter:         "texture_file_open_projection",
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
		AgentID:      "texture:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunCompleted,
		Prompt:       "Revise the imported markdown proposal.",
		CreatedAt:    now,
		UpdatedAt:    now,
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			"type":                "texture_agent_revision",
			"doc_id":              doc.DocID,
			"current_revision_id": base.RevisionID,
			runMetadataAgentID:    "texture:" + doc.DocID,
			runMetadataChannelID:  doc.DocID,
		},
	}
	rawArgs, err := json.Marshal(editTextureArgs{
		DocID:          doc.DocID,
		BaseRevisionID: base.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: "p-" + doc.DocID + "-" + base.RevisionID + "-0",
			Text:    "# Legacy Proposal\n\nImported Markdown body revised as canonical Texture.",
		}},
	})
	if err != nil {
		t.Fatalf("marshal edit args: %v", err)
	}
	if _, err := rt.ToolRegistryForProfile(AgentProfileTexture).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(run)), "patch_texture", rawArgs); err != nil {
		t.Fatalf("patch_texture: %v", err)
	}
	got, err := s.GetDocument(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if got.Title != "legacy-proposal.texture" {
		t.Fatalf("document title = %q, want legacy-proposal.texture", got.Title)
	}
	revs, err := s.ListRevisionsByDoc(ctx, doc.DocID, doc.OwnerID, 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 2 || revs[0].VersionNumber != 1 || !strings.Contains(revs[0].Content, "canonical Texture") {
		t.Fatalf("revisions = %+v, want appagent v1 canonical edit", revs)
	}
	meta := decodeRevisionMetadata(revs[0].Metadata)
	canonicalPath, ok := meta[canonicalTextureSourcePathMetadataKey].(string)
	if !ok || filepath.Ext(canonicalPath) != ".texture" {
		t.Fatalf("%s = %#v, want .texture", canonicalTextureSourcePathMetadataKey, meta[canonicalTextureSourcePathMetadataKey])
	}
	if meta["import_manifest"] == nil || meta["migration_manifest"] == nil {
		t.Fatalf("appagent v1 did not carry import/migration metadata: %#v", meta)
	}
	migrationManifest := meta["migration_manifest"].(map[string]any)
	if migrationManifest["migration_adapter"] != "markdown_to_texture_projection" {
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

func TestTextureAgentRevisionMutationCompletedOnlyOnce(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)

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

	// Create a completed task record with texture agent revision metadata.
	taskRec := &types.RunRecord{
		RunID:        "task-mutation-test",
		AgentID:      "texture:doc-mutation-test",
		ChannelID:    "doc-mutation-test",
		OwnerID:      "user-1",
		SandboxID:    "sandbox-texture-test",
		State:        types.RunCompleted,
		Prompt:       "Revise the document",
		Result:       textureReplaceAllResult("Revised content", "rev-user-1"),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			"type":                  "texture_agent_revision",
			"doc_id":                "doc-mutation-test",
			"current_revision_id":   "rev-user-1",
			runMetadataAgentID:      "texture:doc-mutation-test",
			runMetadataChannelID:    "doc-mutation-test",
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataAgentProfile: AgentProfileTexture,
		},
	}

	textureRegistry := rt.ToolRegistryForProfile(AgentProfileTexture)
	rawArgs, err := json.Marshal(editTextureArgs{
		DocID:          "doc-mutation-test",
		BaseRevisionID: "rev-user-1",
		Operation:      "replace_all",
		Content:        "Revised content",
		Rationale:      "test whole-document replacement",
	})
	if err != nil {
		t.Fatalf("marshal rewrite_texture args: %v", err)
	}
	if _, err := textureRegistry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(taskRec)), "rewrite_texture", rawArgs); err != nil {
		t.Fatalf("first rewrite_texture: %v", err)
	}
	if _, err := textureRegistry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(taskRec)), "rewrite_texture", rawArgs); err == nil {
		t.Fatal("duplicate rewrite_texture against the stale base revision should be rejected")
	}

	afterFirst, err := s.GetDocument(ctx, "doc-mutation-test", "user-1")
	if err != nil {
		t.Fatalf("get document after first rewrite: %v", err)
	}
	secondArgs, err := json.Marshal(editTextureArgs{
		DocID:          "doc-mutation-test",
		BaseRevisionID: afterFirst.CurrentRevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:        "append_block",
			BlockType: "paragraph",
			Text:      "Second same-run revision from later evidence.",
		}},
	})
	if err != nil {
		t.Fatalf("marshal second rewrite args: %v", err)
	}
	if _, err := textureRegistry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(taskRec)), "patch_texture", secondArgs); err != nil {
		t.Fatalf("second same-run patch_texture: %v", err)
	}

	// Call handleRunCompletion twice to simulate duplicate recovery processing.
	rt.handleRunCompletion(ctx, taskRec)
	rt.handleRunCompletion(ctx, taskRec)

	// Verify both legitimate same-run appagent revisions were created, while the
	// stale duplicate attempt produced no extra revision.
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
	if agentCount != 2 {
		t.Errorf("found %d appagent revisions, want 2 legitimate same-run revisions", agentCount)
	}
}

func TestEditTextureInitialWorkingRevisionDoesNotSmuggleRequiredContinuation(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)
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
		RunID:        "run-texture-continuation",
		AgentID:      "texture:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		Prompt:       "Revise the document",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			"type":                  "texture_agent_revision",
			"doc_id":                doc.DocID,
			"current_revision_id":   userRev.RevisionID,
			"original_prompt":       "nba update",
			runMetadataAgentID:      "texture:" + doc.DocID,
			runMetadataChannelID:    doc.DocID,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataAgentProfile: AgentProfileTexture,
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

	registry := rt.ToolRegistryForProfile(AgentProfileTexture)
	editRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&run)), "rewrite_texture", json.RawMessage(`{
		"doc_id":"doc-initial-continuation",
		"base_revision_id":"rev-user-continuation",
		"rationale":"test whole-document replacement",
		"content":"# NBA update\n\nI am preparing a short working update and checking current evidence next."
	}`))
	if err != nil {
		t.Fatalf("rewrite_texture: %v", err)
	}
	var editResult map[string]any
	if err := json.Unmarshal([]byte(editRaw), &editResult); err != nil {
		t.Fatalf("decode edit result: %v", err)
	}
	if _, ok := editResult["next_required_tool"]; ok {
		t.Fatalf("rewrite_texture must not smuggle a required continuation; result=%s", editRaw)
	}

	spawnRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&run)), "spawn_agent", json.RawMessage(`{
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
		t.Fatalf("spawn_agent after completed edit must not require a second rewrite_texture; result=%s", spawnRaw)
	}
}

func TestTextureWorkerUpdateRevisionRejectsNoOpPatch(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)

	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "user-1"
	docID := "doc-worker-noop-test"
	agentID := currentTextureAgentID(docID)
	baseContent := "Current draft before researcher evidence."
	doc := types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Worker no-op guard",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	base := types.Revision{
		RevisionID:  "rev-worker-noop-v1",
		DocID:       docID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: "appagent",
		Content:     baseContent,
		CreatedAt:   now,
	}
	if err := s.CreateRevision(ctx, base); err != nil {
		t.Fatalf("create base revision: %v", err)
	}
	researchRun := types.RunRecord{
		RunID:        "run-worker-noop-researcher",
		OwnerID:      ownerID,
		SandboxID:    "sandbox-texture-test",
		AgentID:      "researcher-worker-noop",
		AgentProfile: AgentProfileResearcher,
		AgentRole:    AgentProfileResearcher,
		ChannelID:    docID,
		State:        types.RunCompleted,
		Prompt:       "Find the missing government evidence.",
		CreatedAt:    now.Add(time.Second),
		UpdatedAt:    now.Add(time.Second),
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileResearcher,
			runMetadataAgentRole:    AgentProfileResearcher,
			runMetadataChannelID:    docID,
			runMetadataAgentID:      "researcher-worker-noop",
			runMetadataTrajectoryID: "traj-worker-noop",
		},
	}
	if err := s.CreateRun(ctx, researchRun); err != nil {
		t.Fatalf("create researcher run: %v", err)
	}
	message := &types.ChannelMessage{
		ChannelID:    docID,
		TrajectoryID: "traj-worker-noop",
		From:         AgentProfileResearcher,
		FromRunID:    researchRun.RunID,
		FromAgentID:  researchRun.AgentID,
		ToAgentID:    agentID,
		Role:         AgentProfileResearcher,
		Content:      "Finding: Anthropic has grounded government evidence that must be reflected in the Texture revision.",
		Timestamp:    now.Add(2 * time.Second),
	}
	if err := s.AppendChannelMessage(ctx, message, ownerID); err != nil {
		t.Fatalf("append researcher message: %v", err)
	}

	runID := "run-worker-noop-texture"
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:               docID,
		RunID:               runID,
		OwnerID:             ownerID,
		State:               "pending",
		ScheduledMessageSeq: message.Seq,
		CreatedAt:           now.Add(3 * time.Second),
	}); err != nil {
		t.Fatalf("create agent mutation: %v", err)
	}
	textureRun := &types.RunRecord{
		RunID:        runID,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-texture-test",
		AgentID:      agentID,
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		ChannelID:    docID,
		State:        types.RunRunning,
		Prompt:       "Integrate the researcher evidence.",
		CreatedAt:    now.Add(3 * time.Second),
		UpdatedAt:    now.Add(3 * time.Second),
		Metadata: map[string]any{
			"type":                  textureAgentRevisionTaskType,
			"doc_id":                docID,
			"current_revision_id":   base.RevisionID,
			"request_source":        "update_coagent",
			"scheduled_message_seq": message.Seq,
			runMetadataAgentID:      agentID,
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataChannelID:    docID,
			runMetadataTrajectoryID: "traj-worker-noop",
		},
	}
	rawArgs, err := json.Marshal(editTextureArgs{
		DocID:          docID,
		BaseRevisionID: base.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: "p-" + docID + "-" + base.RevisionID + "-0",
			Text:    baseContent,
		}},
		Rationale: "No substantive content change intended.",
	})
	if err != nil {
		t.Fatalf("marshal no-op patch: %v", err)
	}
	if _, err := rt.ToolRegistryForProfile(AgentProfileTexture).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(textureRun)), "patch_texture", rawArgs); err == nil ||
		!strings.Contains(err.Error(), "worker update revision must change Texture content") {
		t.Fatalf("no-op worker update patch err = %v, want worker-update no-op guard", err)
	}
	revs, err := s.ListRevisionsByDoc(ctx, docID, ownerID, 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 1 || revs[0].RevisionID != base.RevisionID {
		t.Fatalf("revisions after rejected no-op = %+v, want only base revision", revs)
	}
	mutation, err := s.GetAgentMutationByRun(ctx, runID)
	if err != nil {
		t.Fatalf("get mutation: %v", err)
	}
	if mutation == nil || mutation.State != "pending" {
		t.Fatalf("mutation after rejected no-op = %+v, want pending", mutation)
	}
	checkpoint, err := s.GetTextureControllerCheckpoint(ctx, docID, ownerID)
	if err != nil {
		t.Fatalf("get checkpoint: %v", err)
	}
	if checkpoint != nil {
		t.Fatalf("checkpoint after rejected no-op = %+v, want nil", checkpoint)
	}
}

func TestInitialTextureRevisionRejectsNoOpPromptCopy(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)

	ctx := context.Background()
	now := time.Now().UTC()
	ownerID := "user-1"
	docID := "doc-initial-noop-test"
	agentID := currentTextureAgentID(docID)
	promptContent := "What's going on with Anthropic and the US government?"
	doc := types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Initial no-op guard",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	baseMetadata, err := json.Marshal(map[string]any{
		"input_origin": textureInputOriginUserPrompt,
		"seed_prompt":  promptContent,
	})
	if err != nil {
		t.Fatalf("marshal base metadata: %v", err)
	}
	base := types.Revision{
		RevisionID:  "rev-initial-noop-v0",
		DocID:       docID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     promptContent,
		Metadata:    baseMetadata,
		CreatedAt:   now,
	}
	if err := s.CreateRevision(ctx, base); err != nil {
		t.Fatalf("create base revision: %v", err)
	}

	runID := "run-initial-noop-texture"
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:     docID,
		RunID:     runID,
		OwnerID:   ownerID,
		State:     "pending",
		CreatedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("create agent mutation: %v", err)
	}
	textureRun := &types.RunRecord{
		RunID:        runID,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-texture-test",
		AgentID:      agentID,
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		ChannelID:    docID,
		State:        types.RunRunning,
		Prompt:       "Draft the first model-prior Texture revision.",
		CreatedAt:    now.Add(time.Second),
		UpdatedAt:    now.Add(time.Second),
		Metadata: map[string]any{
			"type":                  textureAgentRevisionTaskType,
			"doc_id":                docID,
			"current_revision_id":   base.RevisionID,
			"request_intent":        "initial_conductor_workflow",
			"seed_prompt":           promptContent,
			"input_origin":          textureInputOriginUserPrompt,
			runMetadataAgentID:      agentID,
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataChannelID:    docID,
			runMetadataTrajectoryID: "traj-initial-noop",
		},
	}
	rawArgs, err := json.Marshal(editTextureArgs{
		DocID:          docID,
		BaseRevisionID: base.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: "p-" + docID + "-" + base.RevisionID + "-0",
			Text:    promptContent,
		}},
		Rationale: "Store the initial draft.",
	})
	if err != nil {
		t.Fatalf("marshal initial no-op patch: %v", err)
	}
	if _, err := rt.ToolRegistryForProfile(AgentProfileTexture).Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(textureRun)), "patch_texture", rawArgs); err == nil ||
		!strings.Contains(err.Error(), "initial model-prior Texture revision must change prompt content") {
		t.Fatalf("no-op initial patch err = %v, want model-prior no-op guard", err)
	}
	revs, err := s.ListRevisionsByDoc(ctx, docID, ownerID, 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	if len(revs) != 1 || revs[0].RevisionID != base.RevisionID {
		t.Fatalf("revisions after rejected initial no-op = %+v, want only user V0", revs)
	}
	mutation, err := s.GetAgentMutationByRun(ctx, runID)
	if err != nil {
		t.Fatalf("get mutation: %v", err)
	}
	if mutation == nil || mutation.State != "pending" {
		t.Fatalf("mutation after rejected initial no-op = %+v, want pending", mutation)
	}
}

func TestInitialTextureNoOpPatchRetriesIntoUsefulDraft(t *testing.T) {
	t.Parallel()
	provider := &textureInitialNoOpThenDraftProvider{Provider: NewStubProvider(1 * time.Millisecond)}

	h, s, rt := textureAPISetupWithProvider(t, provider, true)
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Draft a short private note."}`, "user-1")
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
		t.Fatalf("conductor did not create texture route: %+v", decision)
	}
	if state := waitForTaskCompletion(t, h, decision.InitialLoopID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("initial texture state = %q, want completed", state)
	}
	if !provider.sawNoOpError {
		t.Fatalf("provider never saw initial no-op guard error; choices=%#v", provider.choices)
	}
	if provider.sawExactInitialPatchGuidance {
		t.Fatalf("provider saw exact initial patch failure guidance after unconstrained first paint; choices=%#v", provider.choices)
	}
	if len(provider.choices) < 4 ||
		provider.choices[0] != "" ||
		provider.choices[1] != "" ||
		provider.choices[2] != "" ||
		provider.choices[3] != "" {
		t.Fatalf("initial texture choices = %#v, want unconstrained retries and completion", provider.choices)
	}
	revs, err := s.ListRevisionsByDoc(context.Background(), decision.DocID, "user-1", 10)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	var appagentRevs []types.Revision
	for _, rev := range revs {
		if rev.AuthorKind == types.AuthorAppAgent {
			appagentRevs = append(appagentRevs, rev)
		}
	}
	if len(appagentRevs) != 1 {
		t.Fatalf("appagent revisions = %+v, want one useful V1 after retry", appagentRevs)
	}
	if !strings.Contains(appagentRevs[0].Content, "USEFUL_MODEL_PRIOR_V1") {
		t.Fatalf("stored V1 content = %q, want useful draft", appagentRevs[0].Content)
	}
}

func TestEditTextureExplicitResearcherDoesNotForceSpawnContinuation(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	doc := types.Document{
		DocID:     "doc-explicit-researcher-continuation",
		OwnerID:   "user-1",
		Title:     "Restart route proof",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	userRev := types.Revision{
		RevisionID:  "rev-user-explicit-researcher",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     "Ask researcher for route evidence, then ask super for a verification note.",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, userRev); err != nil {
		t.Fatalf("create user revision: %v", err)
	}
	run := types.RunRecord{
		RunID:        "run-texture-explicit-researcher",
		AgentID:      "texture:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		Prompt:       "Revise the document",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			"type":                        "texture_agent_revision",
			"doc_id":                      doc.DocID,
			"current_revision_id":         userRev.RevisionID,
			"original_prompt":             "Ask researcher for route evidence, then ask super for a verification note.",
			runMetadataExplicitResearcher: true,
			runMetadataAgentID:            "texture:" + doc.DocID,
			runMetadataChannelID:          doc.DocID,
			runMetadataAgentRole:          AgentProfileTexture,
			runMetadataAgentProfile:       AgentProfileTexture,
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

	registry := rt.ToolRegistryForProfile(AgentProfileTexture)
	editRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&run)), "rewrite_texture", json.RawMessage(`{
		"doc_id":"doc-explicit-researcher-continuation",
		"base_revision_id":"rev-user-explicit-researcher",
		"rationale":"test whole-document replacement",
		"content":"# Restart route proof\n\nWorking revision: researcher evidence and super verification are still pending."
	}`))
	if err != nil {
		t.Fatalf("rewrite_texture: %v", err)
	}
	var editResult map[string]any
	if err := json.Unmarshal([]byte(editRaw), &editResult); err != nil {
		t.Fatalf("decode edit result: %v", err)
	}
	if _, ok := editResult["next_required_tool"]; ok {
		t.Fatalf("rewrite_texture must not force researcher continuation; result=%s", editRaw)
	}
	if _, ok := registry.Lookup("spawn_agent"); !ok {
		t.Fatal("texture registry missing spawn_agent affordance")
	}
	if _, ok := registry.Lookup("request_super_execution"); !ok {
		t.Fatal("texture registry missing request_super_execution affordance")
	}
}

func TestEditTextureExplicitResearcherDoesNotForceSpawnAfterSuperBase(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	doc := types.Document{
		DocID:     "doc-explicit-researcher-after-super",
		OwnerID:   "user-1",
		Title:     "Restart route proof",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	superRev := types.Revision{
		RevisionID:  "rev-super-explicit-researcher",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorAppAgent,
		AuthorLabel: AgentProfileSuper,
		Content:     "Super artifact is ready; researcher evidence is still missing.",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, superRev); err != nil {
		t.Fatalf("create super revision: %v", err)
	}
	run := types.RunRecord{
		RunID:        "run-texture-explicit-researcher-after-super",
		AgentID:      "texture:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		TrajectoryID: "traj-explicit-researcher-after-super",
		Prompt:       "Revise the document",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			"type":                        "texture_agent_revision",
			"doc_id":                      doc.DocID,
			"current_revision_id":         superRev.RevisionID,
			"original_prompt":             "Ask researcher for route evidence, then ask super for a verification note.",
			runMetadataExplicitResearcher: true,
			runMetadataAgentID:            "texture:" + doc.DocID,
			runMetadataChannelID:          doc.DocID,
			runMetadataAgentRole:          AgentProfileTexture,
			runMetadataAgentProfile:       AgentProfileTexture,
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

	registry := rt.ToolRegistryForProfile(AgentProfileTexture)
	editRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&run)), "rewrite_texture", json.RawMessage(`{
		"doc_id":"doc-explicit-researcher-after-super",
		"base_revision_id":"rev-super-explicit-researcher",
		"rationale":"test whole-document replacement",
		"content":"# Restart route proof\n\nSuper evidence has arrived. Researcher evidence is still pending."
	}`))
	if err != nil {
		t.Fatalf("rewrite_texture: %v", err)
	}
	var editResult map[string]any
	if err := json.Unmarshal([]byte(editRaw), &editResult); err != nil {
		t.Fatalf("decode edit result: %v", err)
	}
	if _, ok := editResult["next_required_tool"]; ok {
		t.Fatalf("rewrite_texture must not force researcher continuation after super-authored base; result=%s", editRaw)
	}
}

func TestEditTextureExplicitResearcherFromBaseRevisionContentSurvivesWorkerPrompt(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	doc := types.Document{
		DocID:     "doc-explicit-researcher-base-content",
		OwnerID:   "user-1",
		Title:     "Restart route proof",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	userRev := types.Revision{
		RevisionID:  "rev-user-explicit-researcher-base-content",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     "Create a Texture document for the M3 route proof. Ask researcher for route evidence, then ask super for a verification note.",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, userRev); err != nil {
		t.Fatalf("create user revision: %v", err)
	}
	run := types.RunRecord{
		RunID:        "run-texture-explicit-researcher-base-content",
		AgentID:      "texture:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		TrajectoryID: "traj-explicit-researcher-base-content",
		Prompt:       "Revise from worker update.",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			"type":                  "texture_agent_revision",
			"doc_id":                doc.DocID,
			"current_revision_id":   userRev.RevisionID,
			"original_prompt":       "Revise from worker update.",
			runMetadataAgentID:      "texture:" + doc.DocID,
			runMetadataChannelID:    doc.DocID,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataAgentProfile: AgentProfileTexture,
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

	registry := rt.ToolRegistryForProfile(AgentProfileTexture)
	editRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&run)), "rewrite_texture", json.RawMessage(`{
		"doc_id":"doc-explicit-researcher-base-content",
		"base_revision_id":"rev-user-explicit-researcher-base-content",
		"rationale":"test whole-document replacement",
		"content":"# Restart route proof\n\nWorking revision from worker update."
	}`))
	if err != nil {
		t.Fatalf("rewrite_texture: %v", err)
	}
	var editResult map[string]any
	if err := json.Unmarshal([]byte(editRaw), &editResult); err != nil {
		t.Fatalf("decode edit result: %v", err)
	}
	if _, ok := editResult["next_required_tool"]; ok {
		t.Fatalf("base revision content must not force researcher continuation; result=%s", editRaw)
	}
}

func TestEditTextureExplicitResearcherFromSeedPromptSurvivesRequestIntent(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	doc := types.Document{
		DocID:     "doc-explicit-researcher-seed-prompt",
		OwnerID:   "user-1",
		Title:     "Restart route proof",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	userRev := types.Revision{
		RevisionID:  "rev-user-explicit-researcher-seed-prompt",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     "Initial worker-facing document state.",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, userRev); err != nil {
		t.Fatalf("create user revision: %v", err)
	}
	run := types.RunRecord{
		RunID:        "run-texture-explicit-researcher-seed-prompt",
		AgentID:      "texture:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		TrajectoryID: "traj-explicit-researcher-seed-prompt",
		Prompt:       "Revise from worker update.",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			"type":                  "texture_agent_revision",
			"doc_id":                doc.DocID,
			"current_revision_id":   userRev.RevisionID,
			"original_prompt":       "",
			"request_intent":        "integrate_worker_findings",
			"seed_prompt":           "Create a Texture document for the M3 route proof. Ask researcher for route evidence, then ask super for a verification note.",
			runMetadataAgentID:      "texture:" + doc.DocID,
			runMetadataChannelID:    doc.DocID,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataAgentProfile: AgentProfileTexture,
		},
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:               doc.DocID,
		RunID:               run.RunID,
		OwnerID:             doc.OwnerID,
		State:               "pending",
		ScheduledMessageSeq: 2,
		CreatedAt:           time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create mutation: %v", err)
	}

	registry := rt.ToolRegistryForProfile(AgentProfileTexture)
	editRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&run)), "rewrite_texture", json.RawMessage(`{
		"doc_id":"doc-explicit-researcher-seed-prompt",
		"base_revision_id":"rev-user-explicit-researcher-seed-prompt",
		"rationale":"test whole-document replacement",
		"content":"# Restart route proof\n\nResearcher finding: pending.\n\nSuper evidence has arrived."
	}`))
	if err != nil {
		t.Fatalf("rewrite_texture: %v", err)
	}
	var editResult map[string]any
	if err := json.Unmarshal([]byte(editRaw), &editResult); err != nil {
		t.Fatalf("decode edit result: %v", err)
	}
	if _, ok := editResult["next_required_tool"]; ok {
		t.Fatalf("seed prompt/request intent must not force researcher continuation; result=%s", editRaw)
	}
}

func TestEditTextureExplicitResearcherDoesNotDuplicateExistingResearcher(t *testing.T) {
	t.Parallel()
	_, s, rt := textureAPISetupWithRuntime(t)
	ctx := context.Background()
	doc := types.Document{
		DocID:     "doc-explicit-researcher-existing",
		OwnerID:   "user-1",
		Title:     "Restart route proof",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}
	userRev := types.Revision{
		RevisionID:  "rev-user-explicit-researcher-existing",
		DocID:       doc.DocID,
		OwnerID:     doc.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     "Ask researcher for route evidence.",
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, userRev); err != nil {
		t.Fatalf("create user revision: %v", err)
	}
	const trajectoryID = "traj-explicit-researcher-existing"
	researcherRun := types.RunRecord{
		RunID:        "run-existing-researcher",
		AgentID:      "researcher:existing",
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		TrajectoryID: trajectoryID,
		Prompt:       "Research route evidence",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileResearcher,
		AgentRole:    AgentProfileResearcher,
		Metadata: map[string]any{
			runMetadataAgentID:      "researcher:existing",
			runMetadataChannelID:    doc.DocID,
			runMetadataAgentRole:    AgentProfileResearcher,
			runMetadataAgentProfile: AgentProfileResearcher,
		},
	}
	if err := s.CreateRun(ctx, researcherRun); err != nil {
		t.Fatalf("create researcher run: %v", err)
	}
	run := types.RunRecord{
		RunID:        "run-texture-explicit-researcher-existing",
		AgentID:      "texture:" + doc.DocID,
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		TrajectoryID: trajectoryID,
		Prompt:       "Revise the document",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Metadata: map[string]any{
			"type":                        "texture_agent_revision",
			"doc_id":                      doc.DocID,
			"current_revision_id":         userRev.RevisionID,
			"original_prompt":             "Ask researcher for route evidence.",
			runMetadataExplicitResearcher: true,
			runMetadataAgentID:            "texture:" + doc.DocID,
			runMetadataChannelID:          doc.DocID,
			runMetadataAgentRole:          AgentProfileTexture,
			runMetadataAgentProfile:       AgentProfileTexture,
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

	registry := rt.ToolRegistryForProfile(AgentProfileTexture)
	editRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(&run)), "rewrite_texture", json.RawMessage(`{
		"doc_id":"doc-explicit-researcher-existing",
		"base_revision_id":"rev-user-explicit-researcher-existing",
		"rationale":"test whole-document replacement",
		"content":"# Restart route proof\n\nResearcher work is already open."
	}`))
	if err != nil {
		t.Fatalf("rewrite_texture: %v", err)
	}
	var editResult map[string]any
	if err := json.Unmarshal([]byte(editRaw), &editResult); err != nil {
		t.Fatalf("decode edit result: %v", err)
	}
	if _, ok := editResult["next_required_tool"]; ok {
		t.Fatalf("rewrite_texture duplicated existing researcher obligation; result=%s", editRaw)
	}
}

func TestTextureApplyEditsRejectsLegacyReplace(t *testing.T) {
	t.Parallel()
	current := types.Revision{
		RevisionID: "rev-1",
		Content:    "repeat\nkeep\nrepeat",
	}
	_, err := materializeTextureToolEdit(editTextureArgs{
		BaseRevisionID: "rev-1",
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:   "replace",
			Text: "changed",
		}},
	}, current)
	if err == nil || !strings.Contains(err.Error(), `op = "replace", want update_block_text`) {
		t.Fatalf("legacy replace err = %v, want structured-operation rejection", err)
	}
}

// TestTextureAgentRevisionProgressEvents verifies that progress events
// are emitted during agent revision execution with the doc_id so
// the frontend can correlate to the open document (VAL-ETEXT-004).
func TestTextureAgentRevisionProgressEvents(t *testing.T) {
	t.Parallel()
	h, s, _ := textureAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Subscribe to events before submitting the task.
	bus := s // We'll use the store to query events after completion.

	// Submit an agent revision.
	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"prompt": "Add more detail"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	var resp textureAgentRevisionResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)

	// Wait for completion.
	state := waitForTaskCompletion(t, h, resp.RunID, 5*time.Second)
	if state != types.RunCompleted {
		t.Fatalf("task state = %q, want %q", state, types.RunCompleted)
	}

	// Check that texture agent revision events were persisted.
	events, err := bus.ListEvents(context.Background(), resp.RunID, 200)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	// We should find texture.agent_revision.started and
	// texture.agent_revision.completed events.
	var foundStarted, foundCompleted bool
	for _, ev := range events {
		switch ev.Kind {
		case types.EventTextureAgentRevisionStarted:
			foundStarted = true
			// Verify the payload contains doc_id.
			var payload map[string]string
			if err := json.Unmarshal(ev.Payload, &payload); err == nil {
				if payload["doc_id"] != docID {
					t.Errorf("started event doc_id = %q, want %q", payload["doc_id"], docID)
				}
			}
		case types.EventTextureAgentRevisionCompleted:
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
		t.Error("missing texture.agent_revision.started event (VAL-ETEXT-004)")
	}
	if !foundCompleted {
		t.Error("missing texture.agent_revision.completed event (VAL-ETEXT-004)")
	}
}

// TestTextureAgentRevisionAcceptsReviseEventWithoutPrompt verifies that the
// frontend can submit a plain revise event and let the backend compile the
// effective texture request from document state.
func TestTextureAgentRevisionAcceptsReviseEventWithoutPrompt(t *testing.T) {
	t.Parallel()
	h, _, rt := textureAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"intent": "revise"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp textureAgentRevisionResponse
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

func TestTextureDiagnosisIncludesDocumentChannelRuns(t *testing.T) {
	t.Parallel()
	h, _, rt := textureAPISetupWithRuntime(t)
	docID, _ := createDocWithUserRevision(t, h)

	docRun, err := rt.StartRunWithMetadata(context.Background(), "diagnose this document", "user-1", map[string]any{
		runMetadataAgentProfile: AgentProfileTexture,
		runMetadataAgentRole:    AgentProfileTexture,
		runMetadataAgentID:      "texture:" + docID,
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

	req := textureRequest(t, http.MethodGet, "/api/texture/documents/"+docID+"/diagnosis?limit=3", nil)
	w := httptest.NewRecorder()
	h.HandleTextureRouter(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("diagnosis status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp textureDiagnosisResponse
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

func TestTextureAgentRevisionRegistersMediaSourceEntities(t *testing.T) {
	t.Setenv("CHOIR_DISABLE_YOUTUBE_TRANSCRIPT_FETCH", "1")
	previous := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() {
		sourcefetch.SetAllowPrivateNetworkForTests(previous)
	})
	h, s, rt := textureAPISetupWithRuntime(t)

	image := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer image.Close()

	docID, _ := createDocWithUserRevision(t, h)
	content := "Review these sources:\n\nhttps://www.youtube.com/watch?v=dQw4w9WgXcQ\n\n" + image.URL + "/cover.png"
	revReq := textureCreateRevisionRequest{
		Content:     content,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "alice",
	}
	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", revReq)
	w := httptest.NewRecorder()
	h.HandleTextureRevisions(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create media revision: status = %d body=%s", w.Code, w.Body.String())
	}

	req = textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"intent": "revise"})
	w = httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("revise status = %d body=%s", w.Code, w.Body.String())
	}
	var resp textureAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	run, err := rt.GetRun(context.Background(), resp.RunID, "user-1")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if _, ok := run.Metadata["media_source_refs"]; ok {
		t.Fatalf("media_source_refs should not be exposed in new run metadata: %#v", run.Metadata)
	}
	if _, ok := run.Metadata["media_source_research_required"]; ok {
		t.Fatalf("media_source_research_required should not be exposed in new run metadata: %#v", run.Metadata)
	}
	sourceEntities := decodeAvailableTextureSourceEntities(run.Metadata)
	if len(sourceEntities) != 2 {
		t.Fatalf("source_entities len = %d, want 2: %#v", len(sourceEntities), sourceEntities)
	}
	if strings.Contains(run.Prompt, "Detected durable media source refs") ||
		strings.Contains(run.Prompt, "Media source refs") ||
		!strings.Contains(run.Prompt, "Detected Texture source entities") ||
		!strings.Contains(run.Prompt, "call patch_texture with insert_source_ref using the listed entity_id/source_entity_id value") ||
		strings.Contains(run.Prompt, "Canonical inline Source Entity syntax is [label](source:ENTITY_ID)") {
		t.Fatalf("compiled prompt missing media source contract: %q", run.Prompt)
	}
	assertNoForcedSemanticDelegation(t, run.Prompt)
	entitiesByKind := map[string]textureSourceEntity{}
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
	dedupedEntities, added := rt.registerTextureMediaSourceEntities(context.Background(), "user-1", content, sourceEntities)
	if added || len(dedupedEntities) != 2 {
		t.Fatalf("re-register added=%v len=%d, want no new entities and len 2: %#v", added, len(dedupedEntities), dedupedEntities)
	}
	items, err := s.ListContentItems(context.Background(), "user-1", 20)
	if err != nil {
		t.Fatalf("list content items: %v", err)
	}
	if len(items) < 3 {
		t.Fatalf("content items = %d, want video, transcript status, and image refs: %#v", len(items), items)
	}
}

func TestTextureAgentRevisionPromotesResearcherContentRefsToSourceEntities(t *testing.T) {
	t.Parallel()
	h, s, rt := textureAPISetupWithRuntime(t)
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
		SandboxID: "sandbox-texture-test",
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
		ToAgentID:   "texture:" + docID,
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

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		map[string]string{"intent": "integrate_worker_findings"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("revise status = %d body=%s", w.Code, w.Body.String())
	}
	var resp textureAgentRevisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	run, err := rt.GetRun(ctx, resp.RunID, "user-1")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	sourceEntities := decodeAvailableTextureSourceEntities(run.Metadata)
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
	if !strings.Contains(run.Prompt, "Detected Texture source entities") ||
		!strings.Contains(run.Prompt, "content_id=content-cloud-audit") ||
		!strings.Contains(run.Prompt, "call patch_texture with insert_source_ref using the listed entity_id/source_entity_id value") ||
		strings.Contains(run.Prompt, "Canonical inline Source Entity syntax is [label](source:ENTITY_ID)") {
		t.Fatalf("compiled prompt missing content source entity contract: %q", run.Prompt)
	}
}

func TestMediaSourceRefToSourceEntityUsesTypedEvidenceStates(t *testing.T) {
	t.Parallel()
	candidate := mediaSourceRefToSourceEntity(textureMediaSourceRef{
		Kind:         "image",
		CanonicalURL: "https://example.com/pending-source.png",
	})
	if candidate.Evidence.State != "candidate" {
		t.Fatalf("candidate evidence state = %q, want candidate: %#v", candidate.Evidence.State, candidate)
	}

	unavailable := mediaSourceRefToSourceEntity(textureMediaSourceRef{
		Kind:                   "youtube",
		CanonicalURL:           "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		ContentID:              "content-youtube",
		TranscriptAvailability: "error",
	})
	if unavailable.Evidence.State != "unavailable" {
		t.Fatalf("unavailable evidence state = %q, want unavailable: %#v", unavailable.Evidence.State, unavailable)
	}

	existing := textureSourceEntity{
		EntityID: "src-candidate",
		Kind:     "image",
		Evidence: textureSourceEntityEvidence{State: "candidate"},
	}
	incoming := textureSourceEntity{
		EntityID: "src-candidate",
		Kind:     "image",
		Evidence: textureSourceEntityEvidence{State: "available"},
	}
	merged := mergeTextureSourceEntity(existing, incoming)
	if merged.Evidence.State != "available" {
		t.Fatalf("merged evidence state = %q, want available: %#v", merged.Evidence.State, merged)
	}
}

// TestTextureAgentRevisionDocumentNotFound verifies that requesting an
// agent revision for a non-existent document returns 404.
func TestTextureAgentRevisionDocumentNotFound(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)

	req := textureRequest(t, http.MethodPost, "/api/texture/documents/nonexistent/revise",
		map[string]string{"prompt": "test"})
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestTextureAgentRevisionWrongOwner verifies that requesting an agent
// revision for a document owned by another user returns 404.
func TestTextureAgentRevisionWrongOwner(t *testing.T) {
	t.Parallel()
	h, _, _ := textureAPISetupWithRuntime(t)

	docID, _ := createDocWithUserRevision(t, h)

	// Use a different user.
	req := httptest.NewRequest(http.MethodPost, "/api/texture/documents/"+docID+"/revise",
		bytes.NewReader([]byte(`{"prompt":"test"}`)))
	req.Header.Set("X-Authenticated-User", "user-2")
	w := httptest.NewRecorder()
	h.HandleTextureAgentRevision(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (wrong owner)", w.Code, http.StatusNotFound)
	}
}
