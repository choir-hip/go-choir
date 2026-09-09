package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestExecutionContextRoundTripAndIsolation(t *testing.T) {
	run := &types.RunRecord{RunID: "run-1"}
	ctx := WithExecutionContext(context.Background(), ExecutionContext{
		RunID: " run-1 ", ToolCallID: " call-parent ", AgentID: " agent-1 ", OwnerID: " owner-1 ", Profile: agentprofile.Texture,
		Role: " texture ", ChannelID: " channel-1 ", ComputerID: " autoputer-1 ", DesktopID: " desktop-1 ",
		OwnerEmail: " owner@example.com ", WorkingDir: " /workspace ", RunRecord: run,
	})
	got := ExecutionContextFrom(ctx)
	if got.RunID != "run-1" || got.ToolCallID != "call-parent" || got.AgentID != "agent-1" || got.OwnerID != "owner-1" || got.Profile != agentprofile.Texture || got.Role != agentprofile.Texture || got.ChannelID != "channel-1" || got.ComputerID != "autoputer-1" || got.DesktopID != "desktop-1" || got.OwnerEmail != "owner@example.com" || got.WorkingDir != "/workspace" || got.RunRecord != run {
		t.Fatalf("execution context = %#v", got)
	}
	if zero := ExecutionContextFrom(context.Background()); zero != (ExecutionContext{}) {
		t.Fatalf("missing execution context = %#v, want zero value", zero)
	}
}

func TestExecuteToolBatchParallelOrderEventsAndErrors(t *testing.T) {
	registry := NewToolRegistry()
	var active int32
	var peak int32
	for _, name := range []string{"slow", "fast", "fail"} {
		name := name
		if err := registry.Register(Tool{Name: name, Func: func(context.Context, json.RawMessage) (string, error) {
			current := atomic.AddInt32(&active, 1)
			for {
				old := atomic.LoadInt32(&peak)
				if current <= old || atomic.CompareAndSwapInt32(&peak, old, current) {
					break
				}
			}
			defer atomic.AddInt32(&active, -1)
			time.Sleep(20 * time.Millisecond)
			if name == "fail" {
				return "", errors.New("boom")
			}
			return name + "-output", nil
		}}); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	var invoked, completed map[string]map[string]any = map[string]map[string]any{}, map[string]map[string]any{}
	emit := func(kind types.EventKind, _ string, payload json.RawMessage) {
		var decoded map[string]any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Errorf("decode event: %v", err)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if kind == types.EventToolInvoked {
			invoked[decoded["call_id"].(string)] = decoded
		} else if kind == types.EventToolResult {
			completed[decoded["call_id"].(string)] = decoded
		}
	}
	results := ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "slow"}, {ID: "2", Name: "fast", Arguments: json.RawMessage(`{"x":1}`)}, {ID: "3", Name: "fail"},
	}, emit)
	if atomic.LoadInt32(&peak) < 2 {
		t.Fatalf("peak concurrency = %d, want parallel execution", peak)
	}
	if len(results) != 3 || results[0].CallID != "1" || results[1].CallID != "2" || results[2].CallID != "3" {
		t.Fatalf("ordered results = %#v", results)
	}
	if results[2].Output != "tool_error: boom" || !results[2].IsError {
		t.Fatalf("error result = %#v", results[2])
	}
	if len(invoked) != 3 || len(completed) != 3 || completed["3"]["is_error"] != true {
		t.Fatalf("events invoked=%#v completed=%#v", invoked, completed)
	}
}

func TestExecuteToolBatchSequentialPolicy(t *testing.T) {
	registry := NewToolRegistry()
	var active, peak int32
	fn := func(context.Context, json.RawMessage) (string, error) {
		current := atomic.AddInt32(&active, 1)
		if current > atomic.LoadInt32(&peak) {
			atomic.StoreInt32(&peak, current)
		}
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return `{"ok":true}`, nil
	}
	for _, name := range []string{"bash", "read_file"} {
		if err := registry.Register(Tool{Name: name, Func: fn}); err != nil {
			t.Fatal(err)
		}
	}
	results := ExecuteToolBatch(context.Background(), registry, []types.ToolCall{{ID: "1", Name: "bash"}, {ID: "2", Name: "read_file"}}, func(types.EventKind, string, json.RawMessage) {})
	if peak != 1 || results[0].IsError || results[1].IsError {
		t.Fatalf("peak=%d results=%#v", peak, results)
	}
}

func TestExecuteToolBatchTextureOneSuccessfulWrite(t *testing.T) {
	registry := NewToolRegistry()
	var attempts int
	if err := registry.Register(Tool{Name: "patch_texture", Func: func(context.Context, json.RawMessage) (string, error) {
		attempts++
		if attempts == 1 {
			return "", errors.New("stale")
		}
		return `{"status":"stored"}`, nil
	}}); err != nil {
		t.Fatal(err)
	}
	ctx := WithExecutionContext(context.Background(), ExecutionContext{Profile: agentprofile.Texture})
	results := ExecuteToolBatch(ctx, registry, []types.ToolCall{{ID: "1", Name: "patch_texture"}, {ID: "2", Name: "patch_texture"}, {ID: "3", Name: "patch_texture"}}, func(types.EventKind, string, json.RawMessage) {})
	if attempts != 2 || !results[0].IsError || results[1].IsError || results[2].IsError || !strings.Contains(results[2].Output, "one canonical document mutation") {
		t.Fatalf("attempts=%d results=%#v", attempts, results)
	}
}

func TestExecuteToolBatchSideEffectSkipPolicies(t *testing.T) {
	tests := []struct {
		name, profile, tool, args string
		notice                    bool
	}{
		{"super bash", agentprofile.Super, "bash", `{"command":"echo x"}`, false},
		{"cosuper bash", agentprofile.CoSuper, "bash", `{"command":"echo x"}`, false},
		{"super co-super spawn", agentprofile.Super, "spawn_agent", `{"profile":"co-super","slot":"implementation","channel_id":"c"}`, false},
		{"texture researcher", agentprofile.Texture, "spawn_agent", `{"profile":"researcher","channel_id":"c","objective":"find facts"}`, true},
		{"update", agentprofile.Researcher, "update_coagent", `{"summary":"x"}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewToolRegistry()
			var executions int
			if err := registry.Register(Tool{Name: tc.tool, Func: func(context.Context, json.RawMessage) (string, error) { executions++; return `{"ok":true}`, nil }}); err != nil {
				t.Fatal(err)
			}
			results := ExecuteToolBatch(WithExecutionContext(context.Background(), ExecutionContext{Profile: tc.profile}), registry, []types.ToolCall{{ID: "first", Name: tc.tool, Arguments: json.RawMessage(tc.args)}, {ID: "second", Name: tc.tool, Arguments: json.RawMessage(tc.args)}}, func(types.EventKind, string, json.RawMessage) {})
			if executions != 1 || results[0].IsError || results[1].IsError == tc.notice {
				t.Fatalf("executions=%d results=%#v", executions, results)
			}
		})
	}
}

func TestExecuteToolBatchConductorTextureOwnsRoute(t *testing.T) {
	registry := NewToolRegistry()
	var executed []string
	if err := registry.Register(Tool{Name: "spawn_agent", Func: func(_ context.Context, args json.RawMessage) (string, error) {
		var in struct {
			Profile string `json:"profile"`
		}
		_ = json.Unmarshal(args, &in)
		executed = append(executed, in.Profile)
		return `{"ok":true}`, nil
	}}); err != nil {
		t.Fatal(err)
	}
	results := ExecuteToolBatch(WithExecutionContext(context.Background(), ExecutionContext{Profile: agentprofile.Conductor}), registry, []types.ToolCall{
		{ID: "texture", Name: "spawn_agent", Arguments: json.RawMessage(`{"profile":"texture"}`)},
		{ID: "research", Name: "spawn_agent", Arguments: json.RawMessage(`{"profile":"researcher"}`)},
		{ID: "texture-2", Name: "spawn_agent", Arguments: json.RawMessage(`{"profile":"texture"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 1 || executed[0] != agentprofile.Texture || results[1].IsError || results[2].IsError {
		t.Fatalf("executed=%#v results=%#v", executed, results)
	}
}

func TestExecuteToolBatchProjectionAndCaps(t *testing.T) {
	registry := NewToolRegistry()
	durable := strings.Repeat("d", 513*1024)
	projected, _ := json.Marshal(map[string]any{"__choir_tool_projection": true, "model_output": "compact", "durable_output": durable, "projection": map[string]any{"kind": "proof"}})
	if err := registry.Register(Tool{Name: "projected", Func: func(context.Context, json.RawMessage) (string, error) { return string(projected), nil }}); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	results := ExecuteToolBatch(context.Background(), registry, []types.ToolCall{{ID: "1", Name: "projected"}}, func(kind types.EventKind, _ string, raw json.RawMessage) {
		if kind == types.EventToolResult {
			_ = json.Unmarshal(raw, &payload)
		}
	})
	if results[0].Output != "compact" || payload["full_output_len"].(float64) != float64(len(durable)) || payload["full_output_truncated"] != true || payload["full_output_sha256"] == "" {
		t.Fatalf("result=%#v payload=%#v", results[0], payload)
	}

	registry = NewToolRegistry()
	if err := registry.Register(Tool{Name: "large", Func: func(context.Context, json.RawMessage) (string, error) { return strings.Repeat("x", 101*1024), nil }}); err != nil {
		t.Fatal(err)
	}
	large := ExecuteToolBatch(context.Background(), registry, []types.ToolCall{{ID: "2", Name: "large"}}, func(types.EventKind, string, json.RawMessage) {})[0]
	if !strings.Contains(large.Output, "[output truncated — 103424 bytes total, showing first 102400 bytes]") {
		t.Fatalf("large output suffix missing: len=%d", len(large.Output))
	}
}

func TestExecuteToolBatchInstallsProviderToolCallID(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.Register(Tool{Name: "capture_call_id", Func: func(ctx context.Context, _ json.RawMessage) (string, error) {
		return ExecutionContextFrom(ctx).ToolCallID, nil
	}}); err != nil {
		t.Fatal(err)
	}
	results := ExecuteToolBatch(WithExecutionContext(context.Background(), ExecutionContext{ToolCallID: "stale"}), registry, []types.ToolCall{{ID: "provider-call-42", Name: "capture_call_id"}}, func(types.EventKind, string, json.RawMessage) {})
	if len(results) != 1 || results[0].IsError || results[0].Output != "provider-call-42" {
		t.Fatalf("results=%+v", results)
	}
}

func TestExecuteToolBatchAssignedCoSuperAdmissionGrammar(t *testing.T) {
	registry := NewToolRegistry()
	var executed []string
	_ = registry.Register(Tool{Name: "capsule_go_eval", Func: func(_ context.Context, args json.RawMessage) (string, error) {
		executed = append(executed, "eval")
		var in struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(args, &in)
		if in.Code == "fail" {
			return "", fmt.Errorf("compile error")
		}
		if in.Code == "stage_complete" {
			return `{"staged_intent_ids":["rlm:complete:1"]}`, nil
		}
		return `{"stdout":"ok"}`, nil
	}})
	_ = registry.Register(Tool{Name: "record_assignment_result", Func: func(_ context.Context, args json.RawMessage) (string, error) {
		executed = append(executed, "record")
		return `{"receipt":"ok"}`, nil
	}})
	_ = registry.Register(Tool{Name: "bash", Func: func(_ context.Context, _ json.RawMessage) (string, error) {
		executed = append(executed, "bash")
		return `{"exit_code":0}`, nil
	}})

	// 1. Two explicit terminals: statically refused, none run.
	executed = nil
	results := ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "record_assignment_result", Arguments: json.RawMessage(`{"result":"completed"}`)},
		{ID: "2", Name: "record_assignment_result", Arguments: json.RawMessage(`{"result":"failed"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 0 || !results[0].IsError || !results[1].IsError || !strings.Contains(results[0].Output, "at most one explicit terminal call") {
		t.Fatalf("two terminals not refused: executed=%v results=%+v", executed, results)
	}

	// 2. Two evals: statically refused, none run.
	executed = nil
	results = ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "capsule_go_eval", Arguments: json.RawMessage(`{"code":"1"}`)},
		{ID: "2", Name: "capsule_go_eval", Arguments: json.RawMessage(`{"code":"2"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 0 || !results[0].IsError || !results[1].IsError || !strings.Contains(results[0].Output, "at most one capsule_go_eval call") {
		t.Fatalf("two evals not refused: executed=%v results=%+v", executed, results)
	}

	// 3. Reversed companion order: statically refused, none run.
	executed = nil
	results = ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "record_assignment_result", Arguments: json.RawMessage(`{"result":"completed"}`)},
		{ID: "2", Name: "capsule_go_eval", Arguments: json.RawMessage(`{"code":"1"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 0 || !results[0].IsError || !results[1].IsError || !strings.Contains(results[0].Output, "cannot precede capsule_go_eval") {
		t.Fatalf("reversed order not refused: executed=%v results=%+v", executed, results)
	}

	// 4. Forbidden companion alongside explicit terminal: statically refused, none run.
	executed = nil
	results = ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "bash", Arguments: json.RawMessage(`{"cmd":"ls"}`)},
		{ID: "2", Name: "record_assignment_result", Arguments: json.RawMessage(`{"result":"completed"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 0 || !results[0].IsError || !results[1].IsError || !strings.Contains(results[0].Output, "companion tool calls are forbidden") {
		t.Fatalf("forbidden companion not refused: executed=%v results=%+v", executed, results)
	}

	// 5. Admitted shape (a): singleton terminal executes.
	executed = nil
	results = ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "record_assignment_result", Arguments: json.RawMessage(`{"result":"completed"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 1 || executed[0] != "record" || results[0].IsError {
		t.Fatalf("shape (a) failed: executed=%v results=%+v", executed, results)
	}

	// 6. Admitted shape (b): eval then terminal executes sequentially.
	executed = nil
	results = ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "capsule_go_eval", Arguments: json.RawMessage(`{"code":"1"}`)},
		{ID: "2", Name: "record_assignment_result", Arguments: json.RawMessage(`{"result":"completed"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 2 || executed[0] != "eval" || executed[1] != "record" || results[0].IsError || results[1].IsError {
		t.Fatalf("shape (b) failed: executed=%v results=%+v", executed, results)
	}

	// 7. Admitted shape (b) with eval failure: terminal skipped.
	executed = nil
	results = ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "capsule_go_eval", Arguments: json.RawMessage(`{"code":"fail"}`)},
		{ID: "2", Name: "record_assignment_result", Arguments: json.RawMessage(`{"result":"completed"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 1 || executed[0] != "eval" || !results[0].IsError || !results[1].IsError || !strings.Contains(results[1].Output, "prior capsule_go_eval failed") {
		t.Fatalf("eval failure did not skip terminal: executed=%v results=%+v", executed, results)
	}

	// 8. Admitted shape (b) with mailbox complete: eval completes mailbox and JSON terminal executes.
	executed = nil
	results = ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "1", Name: "capsule_go_eval", Arguments: json.RawMessage(`{"code":"stage_complete"}`)},
		{ID: "2", Name: "record_assignment_result", Arguments: json.RawMessage(`{"result":"completed"}`)},
	}, func(types.EventKind, string, json.RawMessage) {})
	if len(executed) != 2 || executed[0] != "eval" || executed[1] != "record" || results[0].IsError || results[1].IsError {
		t.Fatalf("eval+complete with JSON terminal failed: executed=%v results=%+v", executed, results)
	}
}
