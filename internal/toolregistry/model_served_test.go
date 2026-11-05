package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A run that fails over records the model that answered, not the one it
// started on, and the switch between them, in one model.served line written
// as the loop returns (docs/problems/model-provenance-unwired-2026-11-05.md).
func TestRunToolLoopRecordsServedModelAndFailover(t *testing.T) {
	provider := &modelFailingProvider{fail: map[string]error{"deepseek-v4.1-flash": fmt.Errorf("gateway: status 503 service unavailable")}}
	var served []types.ModelServedRecord
	emit := func(kind types.EventKind, _ string, payload json.RawMessage) {
		if kind != types.EventModelServed {
			return
		}
		var rec types.ModelServedRecord
		if err := json.Unmarshal(payload, &rec); err != nil {
			t.Fatalf("payload: %v", err)
		}
		served = append(served, rec)
	}
	_, _, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, emit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Source: "default_pool"}),
		WithToolLoopFallbackLLMConfigs([]provideriface.LLMSelection{{Provider: "chatgpt", Model: "gpt-6-luna"}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(served) != 1 {
		t.Fatalf("model.served lines = %d, want 1", len(served))
	}
	rec := served[0]
	if rec.Schema != types.ModelServedSchema || rec.SelectedModel != "deepseek-v4.1-flash" || rec.SelectedSource != "default_pool" {
		t.Fatalf("selection = %+v", rec)
	}
	if len(rec.Models) != 1 || rec.Models[0].Model != "gpt-6-luna" || rec.Models[0].Provider != "chatgpt" || rec.Models[0].Calls != 1 {
		t.Fatalf("models = %+v, want only luna, once", rec.Models)
	}
	if len(rec.Switches) != 1 || rec.Switches[0].FromModel != "deepseek-v4.1-flash" || rec.Switches[0].ToModel != "gpt-6-luna" || rec.Switches[0].Reason != "model_failover" {
		t.Fatalf("switches = %+v", rec.Switches)
	}
}

// A run that fails on every model still records the switch it made.
func TestRunToolLoopRecordsSwitchesOnFailure(t *testing.T) {
	provider := &modelFailingProvider{fail: map[string]error{
		"deepseek-v4.1-flash": fmt.Errorf("opencode provider: unsupported model"),
		"gpt-6-luna":          fmt.Errorf("model \"gpt-6-luna\" is not in the catalog for provider chatgpt"),
	}}
	var served []types.ModelServedRecord
	emit := func(kind types.EventKind, _ string, payload json.RawMessage) {
		if kind == types.EventModelServed {
			var rec types.ModelServedRecord
			_ = json.Unmarshal(payload, &rec)
			served = append(served, rec)
		}
	}
	_, _, err := RunToolLoop(context.Background(), provider, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, emit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}),
		WithToolLoopFallbackLLMConfigs([]provideriface.LLMSelection{{Provider: "chatgpt", Model: "gpt-6-luna"}}))
	if err == nil {
		t.Fatal("want the run to fail")
	}
	if len(served) != 1 || len(served[0].Models) != 0 || len(served[0].Switches) != 1 {
		t.Fatalf("served = %+v", served)
	}
}
