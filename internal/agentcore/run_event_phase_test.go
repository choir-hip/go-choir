package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A run's progress events must keep the phase the tool loop emits them under
// ("tool_loop", "tool_loop_budget_usage", ...). The store keeps the payload,
// not EventRecord.Phase, so the phase rides the payload. Without it cost
// extraction (llmcost.ExtractCostEntries) and the Cut 0 per-turn token
// baseline read nothing from real rows.
//
// Ways this could fail, written first: the phase is dropped on persist; a
// payload that already names a phase is overwritten; a non-object payload is
// corrupted; tool events gain a phase they never had. (tool.invoked is a
// bus-only line since journal cut R3, so the tool event checked here is the
// stored tool.result.)
func TestRunEventEmitterPersistsPhase(t *testing.T) {
	rt, s := testRuntime(t)
	rec := &types.RunRecord{RunID: "run-phase", AgentID: "research:phase", OwnerID: "owner-1"}
	emit := rt.runEventEmitter(context.Background(), rec)

	turn, _ := json.Marshal(map[string]any{"iteration": 1, "model": "gpt-6-luna", "llm_model": "gpt-6-luna", "llm_provider": "chatgpt", "input_tokens": 1200, "output_tokens": 300})
	emit(types.EventRunProgress, "tool_loop", turn)
	own, _ := json.Marshal(map[string]any{"phase": "execution", "status": "completed", "model": "m", "tokens_in": 1, "tokens_out": 1})
	emit(types.EventRunProgress, "tool_loop", own)
	emit(types.EventRunProgress, "tool_loop", json.RawMessage(`"not an object"`))
	call, _ := json.Marshal(map[string]any{"tool": "desk_go_eval"})
	emit(types.EventToolResult, "tool_loop", call)

	evs, err := s.ListEvents(context.Background(), rec.RunID, 10)
	if err != nil || len(evs) != 4 {
		t.Fatalf("list events: %v (%d)", err, len(evs))
	}
	phase := func(i int) any {
		var p map[string]any
		_ = json.Unmarshal(evs[i].Payload, &p)
		return p["phase"]
	}
	if phase(0) != "tool_loop" {
		t.Fatalf("tool-loop turn lost its phase: %s", evs[0].Payload)
	}
	if phase(1) != "execution" {
		t.Fatalf("a payload's own phase must win: %s", evs[1].Payload)
	}
	if string(evs[2].Payload) != `"not an object"` {
		t.Fatalf("non-object payload changed: %s", evs[2].Payload)
	}
	if phase(3) != nil {
		t.Fatalf("tool events must not gain a phase: %s", evs[3].Payload)
	}
	if got := llmcost.ExtractCostEntries(evs); len(got) != 2 {
		t.Fatalf("cost extraction must read both model turns from stored rows, got %d", len(got))
	}
}
