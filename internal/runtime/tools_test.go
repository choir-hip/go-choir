package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

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

func TestExecuteToolsConductorVTextRouteSkipsOtherSpawn(t *testing.T) {
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
		{ID: "vtext", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"vtext","objective":"open document","initial_content":"# Draft"}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if len(executed) != 1 || executed[0] != AgentProfileVText {
		t.Fatalf("executed = %#v, want only vtext", executed)
	}
	if !results[0].IsError || !strings.Contains(results[0].Output, "vtext owns downstream") {
		t.Fatalf("research spawn result = %#v, want skipped downstream worker", results[0])
	}
	if results[1].IsError || results[1].Output != AgentProfileVText {
		t.Fatalf("vtext spawn result = %#v, want success", results[1])
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
	registerCountingTool("cast_agent")
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
		{ID: "cast-1", Name: "cast_agent", Arguments: json.RawMessage(`{"agent_id":"agent-impl","content":"proceed with exact evidence"}`)},
		{ID: "cast-2", Name: "cast_agent", Arguments: json.RawMessage(`{"agent_id":"agent-impl","content":"proceed with exact evidence"}`)},
		{ID: "export-1", Name: "publish_app_change_package", Arguments: json.RawMessage(`{"repo_path":"go-choir-candidate","base_sha":"base","snapshot_id":"snap"}`)},
		{ID: "export-2", Name: "publish_app_change_package", Arguments: json.RawMessage(`{"repo_path":"go-choir-candidate","base_sha":"base","snapshot_id":"snap"}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotCounts := map[string]int{
		"spawn_agent":                counts["spawn_agent"],
		"cast_agent":                 counts["cast_agent"],
		"publish_app_change_package": counts["publish_app_change_package"],
	}
	mu.Unlock()
	if gotCounts["spawn_agent"] != 2 || gotCounts["cast_agent"] != 1 || gotCounts["publish_app_change_package"] != 1 {
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

func TestExecuteToolsCoSuperSkipsDuplicateExportPatchset(t *testing.T) {
	registry := NewToolRegistry()
	var mu sync.Mutex
	exports := 0
	if err := registry.Register(Tool{
		Name: "publish_app_change_package",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			mu.Lock()
			exports++
			mu.Unlock()
			return "export ok", nil
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
		{ID: "export-1", Name: "publish_app_change_package", Arguments: json.RawMessage(`{"repo_path":"go-choir-candidate","base_sha":"base","snapshot_id":"snap"}`)},
		{ID: "export-2", Name: "publish_app_change_package", Arguments: json.RawMessage(`{"repo_path":"go-choir-candidate","base_sha":"base","snapshot_id":"snap"}`)},
	}

	results := executeTools(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotExports := exports
	mu.Unlock()
	if gotExports != 1 {
		t.Fatalf("exports executed = %d, want one", gotExports)
	}
	if results[0].IsError {
		t.Fatalf("first export = %#v, want success", results[0])
	}
	if !results[1].IsError || !strings.Contains(results[1].Output, "duplicate publish_app_change_package") {
		t.Fatalf("second export = %#v, want duplicate skip", results[1])
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

func TestExecuteToolsChainsRequiredWorkerDelegation(t *testing.T) {
	registry := NewToolRegistry()
	var delegatedArgs struct {
		WorkerSandboxURL string `json:"worker_sandbox_url"`
		WorkerID         string `json:"worker_id"`
		VMID             string `json:"vm_id"`
		Profile          string `json:"profile"`
		Objective        string `json:"objective"`
	}
	if err := registry.Register(Tool{
		Name: "request_worker_vm",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_requested","handle":{"purpose":"tiny UX copy edit","worker_id":"worker-1","vm_id":"vm-1","sandbox_url":"http://worker"},"next_required_tool":"delegate_worker_vm","next_required_args":{"worker_sandbox_url":"http://worker","worker_id":"worker-1","vm_id":"vm-1","profile":"vsuper"}}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name: "delegate_worker_vm",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			if err := json.Unmarshal(args, &delegatedArgs); err != nil {
				return "", err
			}
			return `{"status":"worker_run_completed","app_change_packages":[{"package_id":"package-worker-1","package_manifest_sha256":"manifest-sha"}]}`, nil
		},
	}); err != nil {
		t.Fatalf("register delegate_worker_vm: %v", err)
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
	if delegatedArgs.WorkerSandboxURL != "http://worker" || delegatedArgs.WorkerID != "worker-1" || delegatedArgs.VMID != "vm-1" {
		t.Fatalf("delegated args = %#v, want worker handle args", delegatedArgs)
	}
	if delegatedArgs.Profile != AgentProfileVSuper {
		t.Fatalf("delegated profile = %q, want %q", delegatedArgs.Profile, AgentProfileVSuper)
	}
	if !strings.Contains(delegatedArgs.Objective, "Full sweep objective goes here.") {
		t.Fatalf("delegated objective = %q, want parent prompt", delegatedArgs.Objective)
	}
	if !strings.Contains(results[0].Output, `"delegation_status":"worker_run_completed"`) ||
		!strings.Contains(results[0].Output, `"package-worker-1"`) {
		t.Fatalf("request output = %s, want chained delegation evidence", results[0].Output)
	}
	if strings.Join(emitted, ",") != "request_worker_vm,delegate_worker_vm" {
		t.Fatalf("emitted tool results = %#v, want request then delegate", emitted)
	}
}

func TestExecuteToolsPropagatesChainedWorkerDelegationBlocker(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{
		Name: "request_worker_vm",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_requested","next_required_tool":"delegate_worker_vm","next_required_args":{"worker_sandbox_url":"http://worker","worker_id":"worker-1","vm_id":"vm-1","profile":"vsuper"}}`, nil
		},
	}); err != nil {
		t.Fatalf("register request_worker_vm: %v", err)
	}
	if err := registry.Register(Tool{
		Name: "delegate_worker_vm",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return `{"status":"worker_run_incomplete","completion_blocker":"vsuper_completed_without_required_app_change_package","terminal_error":"worker vsuper completed without publish_app_change_package evidence","app_change_packages":[],"worker_event_summary":["tool.result delegate_worker_vm returned no package"]}`, nil
		},
	}); err != nil {
		t.Fatalf("register delegate_worker_vm: %v", err)
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
	if output["delegation_status"] != "worker_run_incomplete" {
		t.Fatalf("delegation_status = %v, want worker_run_incomplete\noutput=%s", output["delegation_status"], results[0].Output)
	}
	if output["completion_blocker"] != "vsuper_completed_without_required_app_change_package" {
		t.Fatalf("completion_blocker = %v, want required package blocker\noutput=%s", output["completion_blocker"], results[0].Output)
	}
	if output["delegation_incomplete"] != true {
		t.Fatalf("delegation_incomplete = %v, want true\noutput=%s", output["delegation_incomplete"], results[0].Output)
	}
	if _, ok := output["chained_delegation_output"].(map[string]any); !ok {
		t.Fatalf("missing chained_delegation_output map\noutput=%s", results[0].Output)
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
