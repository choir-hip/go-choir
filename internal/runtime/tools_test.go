package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestNormalizeDelegateTargetValueAllowsSingleNoisyAllowedTarget(t *testing.T) {
	allowed := []string{AgentProfileResearcher}
	if got := normalizeDelegateTargetValue("researcher</parameter> </invoke>", allowed); got != AgentProfileResearcher {
		t.Fatalf("noisy researcher target = %q, want %q", got, AgentProfileResearcher)
	}
	if got := normalizeDelegateTargetValue("researcher and texture</invoke>", []string{AgentProfileResearcher, AgentProfileTexture}); got == AgentProfileResearcher || got == AgentProfileTexture {
		t.Fatalf("ambiguous noisy target recovered as %q, want unrecovered value", got)
	}
	if got := normalizeDelegateTargetValue("researcher please", allowed); got == AgentProfileResearcher {
		t.Fatalf("plain non-enum text recovered as %q, want allowlist rejection later", got)
	}
}

func TestFrozenCorpusEvalDisablesLiveSourceAcquisitionTools(t *testing.T) {
	rt, _ := testRuntime(t)
	run := &types.RunRecord{
		RunID:        "run-eval",
		OwnerID:      "user-alice",
		AgentProfile: AgentProfileResearcher,
		AgentRole:    AgentProfileResearcher,
		Metadata: map[string]any{
			"eval_kind":                    compactionRecallEvalKind,
			compactionRecallLiveSearchFlag: true,
		},
	}
	ctx := WithToolExecutionContext(context.Background(), run)
	registry := MustNewToolRegistry(
		newWebSearchTool(nil, rt),
		newSourceSearchTool(nil, rt),
		newFetchURLTool(nil, rt),
		newImportURLContentTool(rt),
		newImportDocumentContentTool(rt),
	)
	cases := []struct {
		tool string
		args json.RawMessage
		want string
	}{
		{"web_search", json.RawMessage(`{"query":"spend quota"}`), "web_search is disabled"},
		{"source_search", json.RawMessage(`{"query":"spend quota"}`), "source_search is disabled"},
		{"fetch_url", json.RawMessage(`{"url":"https://example.com"}`), "fetch_url is disabled"},
		{"import_url_content", json.RawMessage(`{"url":"https://example.com"}`), "URL imports are disabled"},
		{"import_document_content", json.RawMessage(`{"url":"https://example.com"}`), "live URL imports are disabled"},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			_, err := registry.Execute(ctx, tc.tool, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s error = %v, want %q", tc.tool, err, tc.want)
			}
		})
	}
}

// --- Tool Registry Tests ---

func TestToolRegistryRegister(t *testing.T) {
	registry := NewToolRegistry()

	tool := Tool{
		Name:        "read_file",
		Description: "Read a file from the sandbox filesystem",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "file contents", nil
		},
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("register: %v", err)
	}

	if registry.Size() != 1 {
		t.Errorf("size: got %d, want 1", registry.Size())
	}
}

func TestToolRegistryRegisterDuplicate(t *testing.T) {
	registry := NewToolRegistry()

	tool := Tool{
		Name: "duplicate",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", nil
		},
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("first register: %v", err)
	}

	if err := registry.Register(tool); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestToolRegistryRegisterValidateNoName(t *testing.T) {
	registry := NewToolRegistry()

	tool := Tool{
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", nil
		},
	}

	if err := registry.Register(tool); err == nil {
		t.Error("expected error for tool with no name")
	}
}

func TestToolRegistryRegisterValidateNilFunc(t *testing.T) {
	registry := NewToolRegistry()

	tool := Tool{
		Name: "nil_func",
	}

	if err := registry.Register(tool); err == nil {
		t.Error("expected error for tool with nil func")
	}
}

func TestToolRegistryExecute(t *testing.T) {
	registry := NewToolRegistry()

	echoTool := Tool{
		Name:        "echo",
		Description: "Echo back the input arguments",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return string(args), nil
		},
	}

	if err := registry.Register(echoTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	result, err := registry.Execute(context.Background(), "echo", json.RawMessage(`{"message":"hello"}`))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if result != `{"message":"hello"}` {
		t.Errorf("result: got %q, want %q", result, `{"message":"hello"}`)
	}
}

func TestToolRegistryExecuteNotFound(t *testing.T) {
	registry := NewToolRegistry()

	_, err := registry.Execute(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestToolRegistryExecuteError(t *testing.T) {
	registry := NewToolRegistry()

	failTool := Tool{
		Name: "fail",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", fmt.Errorf("tool failure")
		},
	}

	if err := registry.Register(failTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := registry.Execute(context.Background(), "fail", nil)
	if err == nil {
		t.Error("expected error from failing tool")
	}
}

func TestToolRegistryLookup(t *testing.T) {
	registry := NewToolRegistry()

	tool := Tool{
		Name: "findme",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "found", nil
		},
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("register: %v", err)
	}

	found, ok := registry.Lookup("findme")
	if !ok {
		t.Fatal("expected to find tool")
	}
	if found.Name != "findme" {
		t.Errorf("name: got %q, want findme", found.Name)
	}

	_, ok = registry.Lookup("nonexistent")
	if ok {
		t.Error("should not find nonexistent tool")
	}
}

func TestToolRegistryCatalog(t *testing.T) {
	registry := NewToolRegistry()

	tools := []Tool{
		{
			Name:        "read_file",
			Description: "Read a file from the sandbox filesystem",
			Func:        func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
		},
		{
			Name:        "list_files",
			Description: "List files in a directory within the sandbox",
			Func:        func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
		},
	}

	for _, tool := range tools {
		if err := registry.Register(tool); err != nil {
			t.Fatalf("register %s: %v", tool.Name, err)
		}
	}

	catalog := registry.Catalog()
	if catalog == "" {
		t.Error("catalog should not be empty")
	}

	// Catalog should list both tools.
	if !contains(catalog, "read_file") {
		t.Error("catalog should contain read_file")
	}
	if !contains(catalog, "list_files") {
		t.Error("catalog should contain list_files")
	}
}

func TestToolRegistryDefinitions(t *testing.T) {
	registry := NewToolRegistry()

	tool := Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string"},
			},
		},
		Func: func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("register: %v", err)
	}

	defs := registry.Definitions()
	if len(defs) != 1 {
		t.Fatalf("definitions: got %d, want 1", len(defs))
	}
	if defs[0].Name != "test_tool" {
		t.Errorf("name: got %q, want test_tool", defs[0].Name)
	}
	if defs[0].Description != "A test tool" {
		t.Errorf("description: got %q, want A test tool", defs[0].Description)
	}
}

func TestToolRegistryDefaultParameters(t *testing.T) {
	registry := NewToolRegistry()

	// Tool without parameters should get a default empty object schema.
	tool := Tool{
		Name: "no_params",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
	}

	if err := registry.Register(tool); err != nil {
		t.Fatalf("register: %v", err)
	}

	found, ok := registry.Lookup("no_params")
	if !ok {
		t.Fatal("expected to find tool")
	}
	if found.Parameters == nil {
		t.Error("parameters should not be nil (should have default empty object)")
	}
}

func TestNewToolRegistryWithTools(t *testing.T) {
	tools := []Tool{
		{
			Name: "tool_a",
			Func: func(ctx context.Context, args json.RawMessage) (string, error) { return "a", nil },
		},
		{
			Name: "tool_b",
			Func: func(ctx context.Context, args json.RawMessage) (string, error) { return "b", nil },
		},
	}

	registry, err := NewToolRegistryWithTools(tools...)
	if err != nil {
		t.Fatalf("new with tools: %v", err)
	}

	if registry.Size() != 2 {
		t.Errorf("size: got %d, want 2", registry.Size())
	}
}

func TestMustNewToolRegistryPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for invalid tool")
		}
	}()

	MustNewToolRegistry(Tool{Name: ""})
}

func TestToolRegistryToolsSorted(t *testing.T) {
	registry := NewToolRegistry()

	// Register in non-alphabetical order.
	names := []string{"z_tool", "a_tool", "m_tool"}
	for _, name := range names {
		tool := Tool{
			Name: name,
			Func: func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
		}
		if err := registry.Register(tool); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	tools := registry.Tools()
	if len(tools) != 3 {
		t.Fatalf("tools: got %d, want 3", len(tools))
	}

	// Should be sorted alphabetically.
	if tools[0].Name != "a_tool" || tools[1].Name != "m_tool" || tools[2].Name != "z_tool" {
		t.Errorf("tools not sorted: got %s, %s, %s", tools[0].Name, tools[1].Name, tools[2].Name)
	}
}

// --- Tool Struct Tests ---

func TestToolValidate(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		wantErr bool
	}{
		{
			name:    "valid tool",
			tool:    Tool{Name: "valid", Func: func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil }},
			wantErr: false,
		},
		{
			name:    "empty name",
			tool:    Tool{Name: "", Func: func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil }},
			wantErr: true,
		},
		{
			name:    "nil func",
			tool:    Tool{Name: "nil_func"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tool.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToolDefinition(t *testing.T) {
	tool := Tool{
		Name:        "test",
		Description: "desc",
		Parameters:  map[string]any{"type": "object"},
		Func:        func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
	}

	def := tool.Definition()
	if def.Name != "test" {
		t.Errorf("name: got %q, want test", def.Name)
	}
	if def.Description != "desc" {
		t.Errorf("description: got %q, want desc", def.Description)
	}
	if def.Parameters["type"] != "object" {
		t.Errorf("parameters: got %v, want object", def.Parameters["type"])
	}
}

// --- executeTools Tests ---

func TestExecuteTools(t *testing.T) {
	registry := NewToolRegistry()

	echoTool := Tool{
		Name: "echo",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return string(args), nil
		},
	}
	if err := registry.Register(echoTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "echo", Arguments: json.RawMessage(`{"msg":"hello"}`)},
	}

	var emittedKinds []types.EventKind
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		emittedKinds = append(emittedKinds, kind)
	}

	results := executeTools(context.Background(), registry, calls, emit)

	if len(results) != 1 {
		t.Fatalf("results: got %d, want 1", len(results))
	}
	if results[0].CallID != "call-1" {
		t.Errorf("call_id: got %q, want call-1", results[0].CallID)
	}
	if results[0].Output != `{"msg":"hello"}` {
		t.Errorf("output: got %q, want echo result", results[0].Output)
	}
	if results[0].IsError {
		t.Error("should not be error")
	}

	// Should emit tool.invoked and tool.result events.
	if len(emittedKinds) != 2 {
		t.Fatalf("emitted events: got %d, want 2", len(emittedKinds))
	}
	if emittedKinds[0] != types.EventToolInvoked {
		t.Errorf("first event: got %q, want tool.invoked", emittedKinds[0])
	}
	if emittedKinds[1] != types.EventToolResult {
		t.Errorf("second event: got %q, want tool.result", emittedKinds[1])
	}
}

func TestExecuteToolsSkipsDuplicateTextureEditsInSameTurn(t *testing.T) {
	registry := NewToolRegistry()
	var executed int
	if err := registry.Register(Tool{
		Name: "patch_texture",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			executed++
			return `{"status":"stored","revision_id":"rev-2"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}

	run := &types.RunRecord{
		RunID:        "run-texture",
		OwnerID:      "owner-1",
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
	}
	results := executeTools(WithToolExecutionContext(context.Background(), run), registry, []types.ToolCall{
		{ID: "call-edit-1", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1"}`)},
		{ID: "call-edit-2", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"again"}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if executed != 1 {
		t.Fatalf("executed patch_texture %d times, want 1", executed)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].IsError {
		t.Fatalf("first edit result = %#v, want success", results[0])
	}
	if results[1].IsError || !strings.Contains(results[1].Output, "duplicate Texture write tool patch_texture") {
		t.Fatalf("second edit result = %#v, want non-error duplicate notice", results[1])
	}
}

func TestExecuteToolsDoesNotSkipTextureEditAfterFailedAttempt(t *testing.T) {
	registry := NewToolRegistry()
	var executed int
	if err := registry.Register(Tool{
		Name: "patch_texture",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			executed++
			if strings.Contains(string(args), "bad") {
				return "", fmt.Errorf("edit 0: find text not present")
			}
			return `{"status":"stored","revision_id":"rev-2"}`, nil
		},
	}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}

	run := &types.RunRecord{
		RunID:        "run-texture",
		OwnerID:      "owner-1",
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
	}
	results := executeTools(WithToolExecutionContext(context.Background(), run), registry, []types.ToolCall{
		{ID: "call-edit-1", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"bad"}`)},
		{ID: "call-edit-2", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"good"}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if executed != 2 {
		t.Fatalf("executed patch_texture %d times, want 2", executed)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if !results[0].IsError {
		t.Fatalf("first edit result = %#v, want error", results[0])
	}
	if results[1].IsError || !strings.Contains(results[1].Output, `"status":"stored"`) {
		t.Fatalf("second edit result = %#v, want stored success", results[1])
	}
}

func TestExecuteToolsSkipsDuplicateTextureResearcherSpawnInSameTurn(t *testing.T) {
	registry := NewToolRegistry()
	var executed []string
	if err := registry.Register(Tool{
		Name: "spawn_agent",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Role      string `json:"role"`
				Objective string `json:"objective"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			executed = append(executed, in.Role+":"+in.Objective)
			return in.Role, nil
		},
	}); err != nil {
		t.Fatalf("register spawn_agent: %v", err)
	}

	run := &types.RunRecord{
		RunID:        "run-texture",
		OwnerID:      "owner-1",
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
	}
	results := executeTools(WithToolExecutionContext(context.Background(), run), registry, []types.ToolCall{
		{ID: "research-1", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"researcher","channel_id":"doc-1","objective":"research current scores"}`)},
		{ID: "research-2", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"researcher","channel_id":"doc-1","objective":"research   current   scores"}`)},
		{ID: "research-3", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"researcher","channel_id":"doc-1","objective":"research injury notes"}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if len(executed) != 2 {
		t.Fatalf("executed spawns = %#v, want duplicate skipped but distinct objective allowed", executed)
	}
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if results[0].IsError || results[0].Output != AgentProfileResearcher {
		t.Fatalf("first spawn result = %#v, want success", results[0])
	}
	if results[1].IsError || !strings.Contains(results[1].Output, "duplicate texture researcher spawn") {
		t.Fatalf("second spawn result = %#v, want non-error duplicate notice", results[1])
	}
	if results[2].IsError || results[2].Output != AgentProfileResearcher {
		t.Fatalf("third spawn result = %#v, want distinct-objective success", results[2])
	}
}

func TestExecuteToolsProjectionReturnsCompactOutputAndPreservesDurableEvidence(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "projected",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return toolProjectionResultJSON(
				map[string]any{"summary": "compact result", "results": []string{"a"}},
				map[string]any{"raw": strings.Repeat("full evidence ", 20)},
				map[string]any{"type": "test_projection"},
			)
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	var resultPayload map[string]any
	results := executeTools(context.Background(), registry, []types.ToolCall{
		{ID: "call-projected", Name: "projected", Arguments: json.RawMessage(`{}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventToolResult {
			return
		}
		if err := json.Unmarshal(payload, &resultPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
	})

	if len(results) != 1 || results[0].IsError {
		t.Fatalf("results = %#v, want one successful projected result", results)
	}
	if strings.Contains(results[0].Output, "__choir_tool_projection") || strings.Contains(results[0].Output, "full evidence") {
		t.Fatalf("model-visible output leaked envelope/full evidence: %s", results[0].Output)
	}
	if !strings.Contains(results[0].Output, "compact result") {
		t.Fatalf("model-visible output = %s, want compact result", results[0].Output)
	}
	if resultPayload["full_output_sha256"] == "" || resultPayload["full_output"] == "" {
		t.Fatalf("result payload missing durable evidence fields: %#v", resultPayload)
	}
	if got := resultPayload["output"]; got != results[0].Output {
		t.Fatalf("event output = %#v, want model output %q", got, results[0].Output)
	}
	projection, ok := resultPayload["output_projection"].(map[string]any)
	if !ok || projection["type"] != "test_projection" {
		t.Fatalf("output_projection = %#v, want test_projection", resultPayload["output_projection"])
	}
}

func TestCompactWebSearchProjectionGuidesResearchFindingsCheckpoint(t *testing.T) {
	resp := &webSearchResponse{
		Query:    "nba update",
		Provider: "mock",
		Results: []map[string]any{{
			"title":    "NBA result",
			"url":      "https://example.com/nba",
			"snippet":  "A grounded update.",
			"provider": "mock",
		}},
	}
	model, _ := compactWebSearchProjection(map[string]any{"results": resp.Results}, resp, true)
	if _, ok := model["next_required_tool"]; ok {
		t.Fatalf("next_required_tool should be omitted from research projections: %#v", model["next_required_tool"])
	}
	instruction := fmt.Sprint(model["next_instruction"])
	if !strings.Contains(instruction, "before any additional search-only turn") {
		t.Fatalf("next_instruction = %q", instruction)
	}
	if strings.Contains(instruction, "provider health") {
		t.Fatalf("cadence instruction should not ask researcher to manage provider health: %q", instruction)
	}
	model, _ = compactWebSearchProjection(map[string]any{"results": resp.Results}, resp, false)
	if _, ok := model["next_required_tool"]; ok {
		t.Fatalf("next_required_tool should be omitted when checkpoint is not required: %#v", model["next_required_tool"])
	}
	fetchModel, _ := compactFetchURLProjection(map[string]any{"url": "https://example.com"}, "body", true)
	if _, ok := fetchModel["next_required_tool"]; ok {
		t.Fatalf("fetch next_required_tool should be omitted from research projections: %#v", fetchModel["next_required_tool"])
	}
}

func TestCompactWebSearchProjectionSurfacesGatewayOutage(t *testing.T) {
	resp := &webSearchResponse{
		Query:  "ai news",
		Outage: true,
		Code:   "search_outage",
		Error:  "search_outage",
		ProviderHealth: map[string]any{
			"brave": map[string]any{
				"state":              "cooling_down",
				"last_failure_class": "rate_limited",
			},
		},
		Attempts: []map[string]any{
			{"provider": "brave", "status": "rate_limited", "latency_ms": 12, "results": 0, "error": "429"},
		},
		Degraded: true,
	}
	model, metadata := compactWebSearchProjection(map[string]any{"outage": true}, resp, true)
	if model["search_outage"] != true {
		t.Fatalf("search_outage = %#v, want true", model["search_outage"])
	}
	if model["provider_health"] == nil {
		t.Fatalf("provider_health missing from outage projection")
	}
	if metadata["search_outage"] != true {
		t.Fatalf("metadata search_outage = %#v, want true", metadata["search_outage"])
	}
	instruction := fmt.Sprint(model["next_instruction"])
	if !strings.Contains(instruction, "precise blocker") {
		t.Fatalf("next_instruction = %q, want blocker guidance", instruction)
	}
}

func TestShouldRequireResearchFindingsAfterResearchToolBatches(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rec := &types.RunRecord{
		RunID:        "research-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileResearcher,
	}
	toolCtx := WithToolExecutionContext(ctx, rec)
	if !shouldRequireResearchFindingsAfterTool(toolCtx, rt) {
		t.Fatalf("first researcher search should require a findings checkpoint")
	}
	superRec := &types.RunRecord{
		RunID:        "super-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
	}
	if shouldRequireResearchFindingsAfterTool(WithToolExecutionContext(ctx, superRec), rt) {
		t.Fatalf("super search should not require update_coagent")
	}

	appendToolResult := func(eventID, tool string) {
		t.Helper()
		payload, err := json.Marshal(map[string]any{"tool": tool, "is_error": false})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		if err := s.AppendEvent(ctx, &types.EventRecord{
			EventID:   eventID,
			RunID:     rec.RunID,
			OwnerID:   rec.OwnerID,
			Timestamp: time.Now().UTC(),
			Kind:      types.EventToolResult,
			Phase:     "tool_call",
			Payload:   payload,
		}); err != nil {
			t.Fatalf("append %s event: %v", tool, err)
		}
	}

	appendToolResult("ev-web-search", "web_search")
	if shouldRequireResearchFindingsAfterTool(toolCtx, rt) {
		t.Fatalf("second researcher search before findings should not repeatedly require another first checkpoint")
	}
	appendToolResult("ev-submit-1", "update_coagent")
	if !shouldRequireResearchFindingsAfterTool(toolCtx, rt) {
		t.Fatalf("first research batch after a checkpoint should require the next findings update")
	}
	appendToolResult("ev-fetch-1", "fetch_url")
	if shouldRequireResearchFindingsAfterTool(toolCtx, rt) {
		t.Fatalf("additional research before the next findings update should not stack repeated required-tool reminders")
	}
	appendToolResult("ev-submit-2", "update_coagent")
	if !shouldRequireResearchFindingsAfterTool(toolCtx, rt) {
		t.Fatalf("another post-checkpoint research batch should require another findings update")
	}
	appendToolResult("ev-source-search", "source_search")
	if shouldRequireResearchFindingsAfterTool(toolCtx, rt) {
		t.Fatalf("source_search before the next findings update should not stack repeated required-tool reminders")
	}
}

func TestResearcherSourceSearchCallsSourceServiceAPI(t *testing.T) {
	ctx := context.Background()
	item := testSourceAPIItem()
	var sawSearch bool
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/search" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "rates" {
			t.Fatalf("source service query = %q, want rates", got)
		}
		if got := r.URL.Query().Get("max_results"); got != "5" {
			t.Fatalf("source service max_results = %q, want 5", got)
		}
		sawSearch = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SearchResponse{
			Query:    "rates",
			Provider: sourceapi.ProviderName,
			Results:  []sourceapi.ItemResult{item},
			Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")
	t.Setenv("SOURCE_SERVICE_DB_PATH", "")
	t.Setenv("SOURCECYCLED_DB_PATH", "")

	rt, _ := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	if _, ok := researcherRegistry.Lookup("source_search"); !ok {
		t.Fatalf("researcher missing source_search")
	}
	textureRegistry := rt.ToolRegistryForProfile(AgentProfileTexture)
	if _, ok := textureRegistry.Lookup("source_search"); ok {
		t.Fatalf("texture should not have source_search")
	}

	rec := &types.RunRecord{
		RunID:        "researcher-source-search-run",
		OwnerID:      "owner-source-search",
		AgentProfile: AgentProfileResearcher,
		AgentRole:    AgentProfileResearcher,
	}
	raw, err := researcherRegistry.Execute(WithToolExecutionContext(ctx, rec), "source_search", json.RawMessage(`{
		"query": "rates",
		"max_results": 5
	}`))
	if err != nil {
		t.Fatalf("source_search: %v", err)
	}
	var envelope struct {
		ModelOutput struct {
			Query           string `json:"query"`
			Provider        string `json:"provider"`
			ResultCount     int    `json:"result_count"`
			NextInstruction string `json:"next_instruction"`
			SourceIdentity  string `json:"source_identity"`
			Results         []struct {
				TargetKind      string   `json:"target_kind"`
				ItemID          string   `json:"item_id"`
				SourceID        string   `json:"source_id"`
				FetchID         string   `json:"fetch_id"`
				Title           string   `json:"title"`
				URL             string   `json:"url"`
				ContentHash     string   `json:"content_hash"`
				BodyKind        string   `json:"body_kind"`
				BodyLength      int      `json:"body_length"`
				ReaderSnapshot  bool     `json:"reader_snapshot"`
				EvidenceLevel   string   `json:"evidence_level"`
				VintagePolicy   string   `json:"vintage_policy"`
				LookaheadStatus string   `json:"lookahead_status"`
				Verticals       []string `json:"verticals"`
			} `json:"results"`
		} `json:"model_output"`
		DurableOutput struct {
			Results []map[string]any `json:"results"`
		} `json:"durable_output"`
		Projection struct {
			Type string `json:"type"`
		} `json:"projection"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("decode source_search projection: %v\nraw=%s", err, raw)
	}
	if !sawSearch {
		t.Fatalf("source_search did not call Source Service API")
	}
	if envelope.ModelOutput.Query != "rates" || envelope.ModelOutput.Provider != sourceapi.ProviderName {
		t.Fatalf("source_search model header = %+v", envelope.ModelOutput)
	}
	if envelope.ModelOutput.ResultCount != 1 || len(envelope.ModelOutput.Results) != 1 {
		t.Fatalf("source_search result count/model = %+v", envelope.ModelOutput)
	}
	got := envelope.ModelOutput.Results[0]
	if got.TargetKind != sourceapi.TargetKind || got.ItemID != item.ItemID || got.SourceID != item.SourceID || got.FetchID != item.FetchID {
		t.Fatalf("source identity = %+v, want item/source/fetch", got)
	}
	if got.ContentHash != item.ContentHash || got.EvidenceLevel != "official_release" || got.VintagePolicy != "release_snapshot" || got.LookaheadStatus != "no_lookahead" {
		t.Fatalf("source caveats/hash = %+v, want official caveats", got)
	}
	if got.BodyKind != item.BodyKind || got.BodyLength != item.BodyLength || got.ReaderSnapshot != item.ReaderSnapshot {
		t.Fatalf("source body classification = %+v, want %+v", got, item)
	}
	if got.Title != item.Title || got.URL != item.URL || len(got.Verticals) != 1 || got.Verticals[0] != "macro_policy" {
		t.Fatalf("source result projection = %+v", got)
	}
	if envelope.ModelOutput.NextInstruction == "" || !strings.Contains(envelope.ModelOutput.SourceIdentity, "source_service_item") {
		t.Fatalf("missing checkpoint/source identity guidance: %+v", envelope.ModelOutput)
	}
	if len(envelope.DurableOutput.Results) != 1 || envelope.Projection.Type != "source_search" {
		t.Fatalf("durable/projection output = %+v/%+v", envelope.DurableOutput, envelope.Projection)
	}
}

func TestResearcherSourceSearchWithoutConfiguredAPIIsUnavailable(t *testing.T) {
	t.Setenv("SOURCE_SERVICE_BASE_URL", "")
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")
	t.Setenv("SOURCE_SERVICE_DB_PATH", "")
	t.Setenv("SOURCECYCLED_DB_PATH", "")
	rt, _ := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	_, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "researcher-source-search-unconfigured",
		OwnerID:      "owner-source-search",
		AgentProfile: AgentProfileResearcher,
	}), "source_search", json.RawMessage(`{"query":"rates"}`))
	if err == nil || !strings.Contains(err.Error(), "source search client not configured") {
		t.Fatalf("source_search err = %v, want source search client not configured", err)
	}
}

func testSourceAPIItem() sourceapi.ItemResult {
	return sourceapi.ItemResult{
		Rank:            1,
		TargetKind:      sourceapi.TargetKind,
		ItemID:          "srcitem_test_rates",
		SourceID:        "official:test",
		SourceType:      "rss",
		FetchID:         "fetch_test_rates",
		OriginalID:      "release-1",
		Title:           "Rate decision",
		Body:            "Rates held steady.",
		URL:             "https://example.test/release-1",
		CanonicalURL:    "https://example.test/release-1",
		PublishedAt:     "2026-06-04T12:00:00Z",
		FetchedAt:       "2026-06-04T12:01:00Z",
		Verticals:       []string{"macro_policy"},
		Language:        "en",
		Region:          "us",
		ContentHash:     "sha256-test-rates",
		BodyKind:        "reader_snapshot",
		BodyLength:      len("Rates held steady."),
		ReaderSnapshot:  true,
		EvidenceLevel:   "official_release",
		VintagePolicy:   "release_snapshot",
		LookaheadStatus: "no_lookahead",
		ReleaseDate:     "2026-06-04",
	}
}

func TestResearcherFailureSynthesizesCheckpointAfterSearch(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(""); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	ownerID := "owner-research-fallback"
	docID := "doc-research-fallback"
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "research fallback",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	parent := types.RunRecord{
		RunID:        "run-texture-parent",
		AgentID:      "texture:" + docID,
		ChannelID:    docID,
		OwnerID:      ownerID,
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		State:        types.RunCompleted,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileTexture,
			runMetadataAgentRole:    AgentProfileTexture,
			runMetadataAgentID:      "texture:" + docID,
			runMetadataChannelID:    docID,
		},
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create parent run: %v", err)
	}
	researcher := &types.RunRecord{
		RunID:            "run-researcher-failed",
		AgentID:          "researcher:fallback",
		ChannelID:        docID,
		RequestedByRunID: parent.RunID,
		OwnerID:          ownerID,
		AgentProfile:     AgentProfileResearcher,
		AgentRole:        AgentProfileResearcher,
		State:            types.RunFailed,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileResearcher,
			runMetadataAgentRole:    AgentProfileResearcher,
			runMetadataAgentID:      "researcher:fallback",
			runMetadataChannelID:    docID,
			"requested_by_profile":  AgentProfileTexture,
			"requested_by_agent_id": "texture:" + docID,
		},
	}
	if err := s.CreateRun(ctx, *researcher); err != nil {
		t.Fatalf("create researcher run: %v", err)
	}
	output := `{"query":"baseball last night","provider":"parallel","result_count":1,"results":[{"title":"Box score","url":"https://example.com/box","snippet":"A score."}]}`
	payload, _ := json.Marshal(map[string]any{
		"tool":     "web_search",
		"call_id":  "call-search",
		"is_error": false,
		"output":   output,
	})
	if err := s.AppendEvent(ctx, &types.EventRecord{
		EventID:   "event-web-search",
		RunID:     researcher.RunID,
		AgentID:   researcher.AgentID,
		OwnerID:   ownerID,
		ChannelID: docID,
		Timestamp: now,
		Kind:      types.EventToolResult,
		Phase:     "tool_call",
		Payload:   payload,
	}); err != nil {
		t.Fatalf("append search event: %v", err)
	}

	if err := rt.synthesizeResearcherUpdateOnFailure(ctx, researcher, fmt.Errorf("required next tool timed out")); err != nil {
		t.Fatalf("synthesize update: %v", err)
	}
	deliveries, err := s.ListPendingWorkerUpdates(ctx, ownerID, "texture:"+docID, 10)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("deliveries len = %d, want 1: %+v", len(deliveries), deliveries)
	}
	if !strings.Contains(deliveries[0].Content, "Runtime fallback") || !strings.Contains(deliveries[0].Content, "web_search") {
		t.Fatalf("delivery content = %q, want runtime web_search fallback", deliveries[0].Content)
	}
	events, err := s.ListEvents(ctx, researcher.RunID, 20)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if !hasSuccessfulToolResult(events, "update_coagent") {
		t.Fatalf("expected synthetic update_coagent tool result")
	}
}

func TestResearcherCompletionSynthesizesCheckpointAfterSavedEvidence(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(""); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	ownerID := "owner-research-completion-fallback"
	docID := "doc-research-completion-fallback"
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "research completion fallback",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	contentID := "content-openai-docs"
	if err := s.CreateEvidence(ctx, types.EvidenceRecord{
		EvidenceID: "ev-openai-docs",
		OwnerID:    ownerID,
		AgentID:    "researcher:completion-fallback",
		Kind:       "source_excerpt",
		SourceURI:  "https://developers.openai.com/api/docs/models/gpt-5.5",
		Title:      "OpenAI GPT-5.5 API docs",
		Content:    "OpenAI GPT-5.5 API docs excerpt.",
		Metadata:   json.RawMessage(`{"content_id":"` + contentID + `"}`),
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("create evidence: %v", err)
	}
	researcher := &types.RunRecord{
		RunID:            "run-researcher-completed",
		AgentID:          "researcher:completion-fallback",
		ChannelID:        docID,
		RequestedByRunID: "run-texture-parent",
		OwnerID:          ownerID,
		AgentProfile:     AgentProfileResearcher,
		AgentRole:        AgentProfileResearcher,
		State:            types.RunCompleted,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileResearcher,
			runMetadataAgentRole:    AgentProfileResearcher,
			runMetadataAgentID:      "researcher:completion-fallback",
			runMetadataChannelID:    docID,
			"requested_by_profile":  AgentProfileTexture,
			"requested_by_agent_id": "texture:" + docID,
		},
	}
	if err := s.CreateRun(ctx, *researcher); err != nil {
		t.Fatalf("create researcher run: %v", err)
	}
	output := `{"evidence_id":"ev-openai-docs","owner_id":"owner-research-completion-fallback","agent_id":"researcher:completion-fallback","kind":"source_excerpt","source_uri":"https://developers.openai.com/api/docs/models/gpt-5.5","title":"OpenAI GPT-5.5 API docs"}`
	payload, _ := json.Marshal(map[string]any{
		"tool":     "save_evidence",
		"call_id":  "call-save-evidence",
		"is_error": false,
		"output":   output,
	})
	if err := s.AppendEvent(ctx, &types.EventRecord{
		EventID:   "event-save-evidence",
		RunID:     researcher.RunID,
		AgentID:   researcher.AgentID,
		OwnerID:   ownerID,
		ChannelID: docID,
		Timestamp: now,
		Kind:      types.EventToolResult,
		Phase:     "tool_call",
		Payload:   payload,
	}); err != nil {
		t.Fatalf("append save evidence event: %v", err)
	}

	if err := rt.synthesizeResearcherUpdateOnCompletion(ctx, researcher); err != nil {
		t.Fatalf("synthesize completion update: %v", err)
	}
	deliveries, err := s.ListPendingWorkerUpdates(ctx, ownerID, "texture:"+docID, 10)
	if err != nil {
		t.Fatalf("list deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("deliveries len = %d, want 1: %+v", len(deliveries), deliveries)
	}
	if !strings.Contains(deliveries[0].Content, "Runtime fallback") || !strings.Contains(deliveries[0].Content, "save_evidence") {
		t.Fatalf("delivery content = %q, want save_evidence fallback", deliveries[0].Content)
	}
	if len(deliveries[0].EvidenceIDs) != 1 || deliveries[0].EvidenceIDs[0] != "ev-openai-docs" {
		t.Fatalf("delivery evidence ids = %+v, want saved evidence id", deliveries[0].EvidenceIDs)
	}
	sourceEntities := rt.evidenceSourceEntitiesFromWorkerUpdates(ctx, ownerID, deliveries)
	if len(sourceEntities) != 1 {
		t.Fatalf("source entities len = %d, want 1: %+v", len(sourceEntities), sourceEntities)
	}
	if sourceEntities[0].Target.ContentID != contentID {
		t.Fatalf("source entity content_id = %q, want %q", sourceEntities[0].Target.ContentID, contentID)
	}
	events, err := s.ListEvents(ctx, researcher.RunID, 20)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if !hasSuccessfulToolResult(events, "update_coagent") {
		t.Fatalf("expected synthetic update_coagent tool result")
	}
}

func TestExecuteToolsParallel(t *testing.T) {
	registry := NewToolRegistry()

	slowTool := Tool{
		Name: "slow",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "slow-result", nil
		},
	}
	fastTool := Tool{
		Name: "fast",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "fast-result", nil
		},
	}
	if err := registry.Register(slowTool); err != nil {
		t.Fatalf("register slow: %v", err)
	}
	if err := registry.Register(fastTool); err != nil {
		t.Fatalf("register fast: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "slow", Arguments: json.RawMessage(`{}`)},
		{ID: "call-2", Name: "fast", Arguments: json.RawMessage(`{}`)},
	}

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	results := executeTools(context.Background(), registry, calls, emit)

	// Results should be in the same order as the calls.
	if results[0].CallID != "call-1" {
		t.Errorf("result[0] call_id: got %q, want call-1", results[0].CallID)
	}
	if results[0].Output != "slow-result" {
		t.Errorf("result[0] output: got %q, want slow-result", results[0].Output)
	}
	if results[1].CallID != "call-2" {
		t.Errorf("result[1] call_id: got %q, want call-2", results[1].CallID)
	}
	if results[1].Output != "fast-result" {
		t.Errorf("result[1] output: got %q, want fast-result", results[1].Output)
	}
}

func TestExecuteToolsSerializesHeavySideEffectTurns(t *testing.T) {
	registry := NewToolRegistry()
	var mu sync.Mutex
	var order []string
	register := func(name string) {
		t.Helper()
		if err := registry.Register(Tool{
			Name: name,
			Func: func(ctx context.Context, args json.RawMessage) (string, error) {
				mu.Lock()
				order = append(order, "start:"+name)
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				order = append(order, "end:"+name)
				mu.Unlock()
				return name + "-result", nil
			},
		}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	register("bash")
	register("read_file")

	results := executeTools(context.Background(), registry, []types.ToolCall{
		{ID: "call-1", Name: "bash", Arguments: json.RawMessage(`{}`)},
		{ID: "call-2", Name: "read_file", Arguments: json.RawMessage(`{}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if results[0].Output != "bash-result" || results[1].Output != "read_file-result" {
		t.Fatalf("results = %+v, want both tool outputs", results)
	}
	want := []string{"start:bash", "end:bash", "start:read_file", "end:read_file"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
}

func TestExecuteToolsError(t *testing.T) {
	registry := NewToolRegistry()

	failTool := Tool{
		Name: "fail",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", fmt.Errorf("tool failure")
		},
	}
	if err := registry.Register(failTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "fail", Arguments: json.RawMessage(`{}`)},
	}

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	results := executeTools(context.Background(), registry, calls, emit)

	if !results[0].IsError {
		t.Error("expected error result")
	}
	if results[0].Output == "" {
		t.Error("error output should contain error message")
	}
}

func TestExecuteToolsOutputTruncation(t *testing.T) {
	registry := NewToolRegistry()

	bigTool := Tool{
		Name: "big_output",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			// Return output larger than 100KB.
			result := make([]byte, 150*1024)
			for i := range result {
				result[i] = 'x'
			}
			return string(result), nil
		},
	}
	if err := registry.Register(bigTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "big_output", Arguments: json.RawMessage(`{}`)},
	}

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	results := executeTools(context.Background(), registry, calls, emit)

	// Output should be truncated to ~100KB + truncation notice.
	if len(results[0].Output) > 110*1024 {
		t.Errorf("output should be truncated, got %d bytes", len(results[0].Output))
	}
}

func TestExecuteToolsConductorTextureRouteSkipsOtherSpawn(t *testing.T) {
	registry := NewToolRegistry()
	executed := []string{}
	if err := registry.Register(Tool{
		Name: "spawn_agent",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Role string `json:"role"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			executed = append(executed, in.Role)
			return in.Role, nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "conductor-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileConductor,
	})
	calls := []types.ToolCall{
		{ID: "research", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"researcher","objective":"research"}`)},
		{ID: "texture", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"texture","objective":"open document","initial_content":"# Draft"}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if len(executed) != 1 || executed[0] != AgentProfileTexture {
		t.Fatalf("executed = %#v, want only texture", executed)
	}
	if results[0].IsError || !strings.Contains(results[0].Output, "texture owns downstream") {
		t.Fatalf("research spawn result = %#v, want non-error skipped downstream worker notice", results[0])
	}
	if results[1].IsError || results[1].Output != AgentProfileTexture {
		t.Fatalf("texture spawn result = %#v, want success", results[1])
	}
}

func TestExecuteToolsVSuperSkipsDuplicateCoordinationSideEffects(t *testing.T) {
	registry := NewToolRegistry()
	var mu sync.Mutex
	counts := map[string]int{}
	registerCountingTool := func(name string) {
		t.Helper()
		if err := registry.Register(Tool{
			Name: name,
			Func: func(ctx context.Context, args json.RawMessage) (string, error) {
				mu.Lock()
				counts[name]++
				mu.Unlock()
				return name + " ok", nil
			},
		}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	registerCountingTool("spawn_agent")
	registerCountingTool("update_coagent")
	registerCountingTool("publish_app_change_package")

	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "vsuper-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileVSuper,
	})
	calls := []types.ToolCall{
		{ID: "spawn-implementation-1", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"co-super","slot":"implementation","channel_id":"doc-1","objective":"implement"}`)},
		{ID: "spawn-implementation-2", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"co-super","slot":"implementation","channel_id":"doc-1","objective":"implement again"}`)},
		{ID: "spawn-verifier", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"co-super","slot":"verifier","channel_id":"doc-1","objective":"verify"}`)},
		{ID: "cast-1", Name: "update_coagent", Arguments: json.RawMessage(`{"agent_id":"agent-impl","content":"proceed with exact evidence"}`)},
		{ID: "cast-2", Name: "update_coagent", Arguments: json.RawMessage(`{"agent_id":"agent-impl","content":"proceed with exact evidence"}`)},
		{ID: "export-1", Name: "publish_app_change_package", Arguments: json.RawMessage(`{"repo_path":"Source/candidate","base_sha":"base","snapshot_id":"snap"}`)},
		{ID: "export-2", Name: "publish_app_change_package", Arguments: json.RawMessage(`{"repo_path":"Source/candidate","base_sha":"base","snapshot_id":"snap"}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotCounts := map[string]int{
		"spawn_agent":                counts["spawn_agent"],
		"update_coagent":             counts["update_coagent"],
		"publish_app_change_package": counts["publish_app_change_package"],
	}
	mu.Unlock()
	if gotCounts["spawn_agent"] != 2 || gotCounts["update_coagent"] != 1 || gotCounts["publish_app_change_package"] != 1 {
		t.Fatalf("executed counts = %+v, want spawn=2 cast=1 export=1", gotCounts)
	}
	for _, idx := range []int{1, 4, 6} {
		if !results[idx].IsError || !strings.Contains(results[idx].Output, "duplicate") {
			t.Fatalf("result[%d] = %#v, want duplicate skip error", idx, results[idx])
		}
	}
	for _, idx := range []int{0, 2, 3, 5} {
		if results[idx].IsError {
			t.Fatalf("result[%d] = %#v, want successful execution", idx, results[idx])
		}
	}
}

func TestExecuteToolsSuperSkipsDuplicateDelegateWorkerVM(t *testing.T) {
	registry := NewToolRegistry()
	var mu sync.Mutex
	executions := 0
	if err := registry.Register(Tool{
		Name: "delegate_worker_vm",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			mu.Lock()
			executions++
			mu.Unlock()
			return `{"status":"worker_run_completed","worker_id":"worker-1","app_change_packages":[{"package_id":"package-1"}]}`, nil
		},
	}); err != nil {
		t.Fatalf("register delegate_worker_vm: %v", err)
	}
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "super-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
	})
	args := json.RawMessage(`{"worker_sandbox_url":"http://worker","worker_id":"worker-1","vm_id":"vm-1","objective":"publish exactly one package","profile":"vsuper"}`)
	results := executeTools(ctx, registry, []types.ToolCall{
		{ID: "delegate-1", Name: "delegate_worker_vm", Arguments: args},
		{ID: "delegate-2", Name: "delegate_worker_vm", Arguments: args},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotExecutions := executions
	mu.Unlock()
	if gotExecutions != 1 {
		t.Fatalf("delegate executions = %d, want one", gotExecutions)
	}
	if results[0].IsError {
		t.Fatalf("first delegate = %#v, want success", results[0])
	}
	if !results[1].IsError || !strings.Contains(results[1].Output, "duplicate delegate_worker_vm") {
		t.Fatalf("second delegate = %#v, want duplicate skip", results[1])
	}
}

func TestExecuteToolsSuperSkipsDuplicateStartWorkerDelegation(t *testing.T) {
	registry := NewToolRegistry()
	var mu sync.Mutex
	executions := 0
	if err := registry.Register(Tool{
		Name: "start_worker_delegation",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			mu.Lock()
			executions++
			mu.Unlock()
			return `{"status":"worker_run_started","worker_id":"worker-1","worker_run_id":"run-1","app_change_packages":[]}`, nil
		},
	}); err != nil {
		t.Fatalf("register start_worker_delegation: %v", err)
	}
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "super-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
	})
	args := json.RawMessage(`{"worker_sandbox_url":"http://worker","worker_id":"worker-1","vm_id":"vm-1","objective":"run harmless worker proof","profile":"vsuper"}`)
	results := executeTools(ctx, registry, []types.ToolCall{
		{ID: "start-1", Name: "start_worker_delegation", Arguments: args},
		{ID: "start-2", Name: "start_worker_delegation", Arguments: args},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotExecutions := executions
	mu.Unlock()
	if gotExecutions != 1 {
		t.Fatalf("start executions = %d, want one", gotExecutions)
	}
	if results[0].IsError {
		t.Fatalf("first start = %#v, want success", results[0])
	}
	if results[1].IsError || !strings.Contains(results[1].Output, "duplicate_start_ignored") {
		t.Fatalf("second start = %#v, want non-error duplicate notice", results[1])
	}
}

func TestExplicitAppChangePackageConstraintsFromRunPrompt(t *testing.T) {
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "worker-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileVSuper,
		Prompt:       `Use app_id "human-proof-chyron-chyron-seq-123", visibility "unlisted", and include marker "chyron-seq-123".`,
	})
	if got := explicitAppChangePackageAppID(ctx); got != "human-proof-chyron-chyron-seq-123" {
		t.Fatalf("explicit app_id = %q", got)
	}
	if got := explicitAppChangePackageVisibility(ctx); got != "unlisted" {
		t.Fatalf("explicit visibility = %q", got)
	}
}

func TestDelegateRequiresAppChangePackageHonorsExplicitNegativeInstruction(t *testing.T) {
	tests := []struct {
		name      string
		objective string
		want      bool
	}{
		{
			name:      "positive package objective",
			objective: "commit the candidate checkout and publish_app_change_package for an owner-pullable package",
			want:      true,
		},
		{
			name:      "negative app change package objective",
			objective: "run a harmless ephemeral verification command, report the marker, and submit a precise worker update; do not edit Choir source and do not publish an AppChangePackage.",
			want:      false,
		},
		{
			name:      "negative tool name objective",
			objective: "collect evidence only; do not call publish_app_change_package for this probe",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := delegateRequiresAppChangePackage(AgentProfileVSuper, tt.objective); got != tt.want {
				t.Fatalf("delegateRequiresAppChangePackage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecuteToolsCoSuperSkipsDuplicateAppChangePackagePublish(t *testing.T) {
	registry := NewToolRegistry()
	var mu sync.Mutex
	publishes := 0
	if err := registry.Register(Tool{
		Name: "publish_app_change_package",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			mu.Lock()
			publishes++
			mu.Unlock()
			return "publish ok", nil
		},
	}); err != nil {
		t.Fatalf("register publish_app_change_package: %v", err)
	}
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "cosuper-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileCoSuper,
	})
	calls := []types.ToolCall{
		{ID: "export-1", Name: "publish_app_change_package", Arguments: json.RawMessage(`{"repo_path":"Source/candidate","base_sha":"base","snapshot_id":"snap"}`)},
		{ID: "export-2", Name: "publish_app_change_package", Arguments: json.RawMessage(`{"repo_path":"Source/candidate","base_sha":"base","snapshot_id":"snap"}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotPublishes := publishes
	mu.Unlock()
	if gotPublishes != 1 {
		t.Fatalf("publishes executed = %d, want one", gotPublishes)
	}
	if results[0].IsError {
		t.Fatalf("first publish = %#v, want success", results[0])
	}
	if !results[1].IsError || !strings.Contains(results[1].Output, "duplicate publish_app_change_package") {
		t.Fatalf("second publish = %#v, want duplicate skip", results[1])
	}
}

func TestExecuteToolsCoSuperSkipsDuplicateBashCommand(t *testing.T) {
	registry := NewToolRegistry()
	var mu sync.Mutex
	executions := 0
	if err := registry.Register(Tool{
		Name: "bash",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			mu.Lock()
			executions++
			mu.Unlock()
			return "bash ok", nil
		},
	}); err != nil {
		t.Fatalf("register bash: %v", err)
	}
	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "cosuper-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileCoSuper,
	})
	calls := []types.ToolCall{
		{ID: "bash-1", Name: "bash", Arguments: json.RawMessage(`{"command":"go test ./internal/platform","timeout_ms":60000}`)},
		{ID: "bash-2", Name: "bash", Arguments: json.RawMessage(`{"command":"go test ./internal/platform","timeout_ms":60000}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotExecutions := executions
	mu.Unlock()
	if gotExecutions != 1 {
		t.Fatalf("bash executions = %d, want one", gotExecutions)
	}
	if results[0].IsError {
		t.Fatalf("first bash = %#v, want success", results[0])
	}
	if !results[1].IsError || !strings.Contains(results[1].Output, "duplicate bash command") {
		t.Fatalf("second bash = %#v, want duplicate skip", results[1])
	}
}

func TestToolCommandEnvUsesPersistentScratchRoot(t *testing.T) {
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)

	env, err := toolCommandEnv("/tmp/ignored")
	if err != nil {
		t.Fatalf("toolCommandEnv: %v", err)
	}

	wantRoot := filepath.Join(filesRoot, ".choir-tool-cache")
	for _, key := range []string{"TMPDIR", "GOTMPDIR", "GOCACHE", "GOMODCACHE"} {
		value := envValue(env, key)
		if !strings.HasPrefix(value, wantRoot+string(os.PathSeparator)) {
			t.Fatalf("%s = %q, want under %q", key, value, wantRoot)
		}
		if _, err := os.Stat(value); err != nil {
			t.Fatalf("%s dir %q not prepared: %v", key, value, err)
		}
	}
}

func TestToolCommandEnvAddsConfiguredBrowserToolDirsToPath(t *testing.T) {
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)
	t.Setenv("PATH", "/usr/bin")
	t.Setenv("CHOIR_OBSCURA_BIN", "/nix/store/example-obscura/bin/obscura")
	t.Setenv("CHOIR_PLAYWRIGHT_BIN", "/nix/store/example-playwright/bin/playwright")

	env, err := toolCommandEnv("/tmp/ignored")
	if err != nil {
		t.Fatalf("toolCommandEnv: %v", err)
	}
	pathValue := envValue(env, "PATH")
	for _, want := range []string{
		"/usr/bin",
		"/nix/store/example-obscura/bin",
		"/nix/store/example-playwright/bin",
	} {
		if !pathListContains(pathValue, want) {
			t.Fatalf("PATH = %q, missing %q", pathValue, want)
		}
	}
}

func pathListContains(raw, want string) bool {
	for _, part := range filepath.SplitList(raw) {
		if part == want {
			return true
		}
	}
	return false
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func TestExecuteToolsDoesNotCapResearcherSearchBatch(t *testing.T) {
	registry := NewToolRegistry()
	var searches int32
	if err := registry.Register(Tool{
		Name: "web_search",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			atomic.AddInt32(&searches, 1)
			return "search result", nil
		},
	}); err != nil {
		t.Fatalf("register web_search: %v", err)
	}
	if err := registry.Register(Tool{
		Name: "fetch_url",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "fetch result", nil
		},
	}); err != nil {
		t.Fatalf("register fetch_url: %v", err)
	}

	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "researcher-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileResearcher,
	})
	calls := []types.ToolCall{
		{ID: "search-1", Name: "web_search", Arguments: json.RawMessage(`{"query":"ai news may 2026"}`)},
		{ID: "search-2", Name: "web_search", Arguments: json.RawMessage(`{"query":"openai may 2026"}`)},
		{ID: "search-3", Name: "web_search", Arguments: json.RawMessage(`{"query":"google ai may 2026"}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if got := atomic.LoadInt32(&searches); got != 3 {
		t.Fatalf("searches = %d, want 3", got)
	}
	for idx, result := range results {
		if result.IsError {
			t.Fatalf("result[%d] = %#v, want success", idx, result)
		}
	}
}

func TestExecuteToolsDoesNotHiddenChainWorkerDelegation(t *testing.T) {
	registry := NewToolRegistry()
	delegated := false
	if err := registry.Register(Tool{
		Name: "request_worker_vm",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_requested","handle":{"purpose":"tiny UX copy edit","worker_id":"worker-1","vm_id":"vm-1","sandbox_url":"http://worker"},"next_tool":"start_worker_delegation","start_args":{"worker_sandbox_url":"http://worker","worker_id":"worker-1","vm_id":"vm-1","profile":"vsuper"}}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name: "start_worker_delegation",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			delegated = true
			return `{"status":"worker_run_completed","app_change_packages":[{"package_id":"package-worker-1","package_manifest_sha256":"manifest-sha"}]}`, nil
		},
	}); err != nil {
		t.Fatalf("register start_worker_delegation: %v", err)
	}

	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "super-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
		Prompt:       "Full sweep objective goes here.",
	})
	var emitted []string
	calls := []types.ToolCall{
		{ID: "lease", Name: "request_worker_vm", Arguments: json.RawMessage(`{"purpose":"tiny UX copy edit"}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventToolResult {
			return
		}
		var event struct {
			Tool string `json:"tool"`
		}
		if err := json.Unmarshal(payload, &event); err == nil {
			emitted = append(emitted, event.Tool)
		}
	})

	if len(results) != 1 || results[0].IsError {
		t.Fatalf("results = %#v, want one successful request result", results)
	}
	if delegated {
		t.Fatalf("start_worker_delegation was hidden-chained; super must regain a model turn before starting worker work")
	}
	if !strings.Contains(results[0].Output, `"next_tool":"start_worker_delegation"`) ||
		strings.Contains(results[0].Output, `"delegation_status"`) ||
		strings.Contains(results[0].Output, `"package-worker-1"`) {
		t.Fatalf("request output = %s, want lease-only async guidance", results[0].Output)
	}
	if strings.Join(emitted, ",") != "request_worker_vm" {
		t.Fatalf("emitted tool results = %#v, want request only", emitted)
	}
}

func TestExecuteToolsDoesNotPropagateHiddenWorkerDelegationBlocker(t *testing.T) {
	registry := NewToolRegistry()
	delegated := false
	if err := registry.Register(Tool{
		Name: "request_worker_vm",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_requested","next_tool":"start_worker_delegation","start_args":{"worker_sandbox_url":"http://worker","worker_id":"worker-1","vm_id":"vm-1","profile":"vsuper"}}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name: "start_worker_delegation",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			delegated = true
			return `{"status":"worker_run_incomplete","completion_blocker":"vsuper_completed_without_required_app_change_package","terminal_error":"worker vsuper completed without publish_app_change_package evidence","app_change_packages":[],"worker_event_summary":["tool.result delegate_worker_vm returned no package"]}`, nil
		},
	}); err != nil {
		t.Fatalf("register start_worker_delegation: %v", err)
	}

	ctx := WithToolExecutionContext(context.Background(), &types.RunRecord{
		RunID:        "super-run",
		OwnerID:      "owner",
		AgentProfile: AgentProfileSuper,
		Prompt:       "Publish exactly one AppChangePackage.",
	})
	results := executeTools(ctx, registry, []types.ToolCall{
		{ID: "lease", Name: "request_worker_vm", Arguments: json.RawMessage(`{"purpose":"package experiment"}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})
	if len(results) != 1 || results[0].IsError {
		t.Fatalf("results = %#v, want one successful request result", results)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(results[0].Output), &output); err != nil {
		t.Fatalf("decode request output: %v\n%s", err, results[0].Output)
	}
	if delegated {
		t.Fatalf("start_worker_delegation was hidden-chained; blockers must come from explicit observe/finish calls")
	}
	if output["next_tool"] != "start_worker_delegation" {
		t.Fatalf("next_tool = %v, want start_worker_delegation\noutput=%s", output["next_tool"], results[0].Output)
	}
	for _, forbidden := range []string{"delegation_status", "completion_blocker", "delegation_incomplete", "chained_delegation_output"} {
		if _, ok := output[forbidden]; ok {
			t.Fatalf("hidden delegation field %q present in request output: %s", forbidden, results[0].Output)
		}
	}
}

func TestWorkerRunEventSummaryExposesSpawnAndChannelEvidence(t *testing.T) {
	spawnPayload, _ := json.Marshal(map[string]any{
		"tool":     "spawn_agent",
		"is_error": false,
		"output":   `{"profile":"co-super","role":"co-super","loop_id":"child-worker"}`,
	})
	channelPayload, _ := json.Marshal(map[string]any{
		"from_agent_id": "vsuper",
		"to_agent_id":   "co-super-worker",
		"role":          "worker",
		"content":       "verify the marker and report pass/fail",
	})
	events := []types.EventRecord{
		{Seq: 1, Kind: types.EventToolResult, Payload: spawnPayload},
		{Seq: 2, Kind: types.EventChannelMessage, Payload: channelPayload},
	}

	if got := collectWorkerSpawnProfiles(events); len(got) != 1 || got[0] != AgentProfileCoSuper {
		t.Fatalf("spawned profiles = %#v, want co-super", got)
	}
	if got := countWorkerChannelMessages(events); got != 1 {
		t.Fatalf("channel message count = %d, want 1", got)
	}
	summary := summarizeWorkerRunEvents(events)
	if len(summary) != 2 {
		t.Fatalf("summary length = %d, want 2: %#v", len(summary), summary)
	}
	if summary[0]["tool"] != "spawn_agent" || summary[1]["role"] != "worker" {
		t.Fatalf("summary = %#v, want spawn tool and worker channel role", summary)
	}
}

// --- buildSystemPromptWithTools Tests ---

func TestBuildSystemPromptWithTools(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name:        "test_tool",
		Description: "A test tool for the prompt",
		Func:        func(ctx context.Context, args json.RawMessage) (string, error) { return "", nil },
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	result := buildSystemPromptWithTools("Base prompt.", registry)
	if result == "Base prompt." {
		t.Error("should append tool catalog to base prompt")
	}
	if !contains(result, "test_tool") {
		t.Error("should include tool name in catalog")
	}
}

func TestBuildSystemPromptWithNilRegistry(t *testing.T) {
	result := buildSystemPromptWithTools("Base prompt.", nil)
	if result != "Base prompt." {
		t.Errorf("nil registry should return base prompt unchanged, got %q", result)
	}
}

func TestBuildSystemPromptWithEmptyRegistry(t *testing.T) {
	registry := NewToolRegistry()
	result := buildSystemPromptWithTools("Base prompt.", registry)
	if result != "Base prompt." {
		t.Errorf("empty registry should return base prompt unchanged, got %q", result)
	}
}

// --- Helper ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
